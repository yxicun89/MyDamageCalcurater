// P4-2/P4-3: 計算画面(docs/design.md「画面: ダメージ計算」、ADR-0300 §2・§5・§6)。
// engine には CalcEngine(差し替え口)、マスタには MasterData(いまは架空の例データ)を渡してもらう。
// 攻撃側は攻撃側プリセット(domain/attackerPresets.ts、既定は無振り)の Key を選び、SP・性格は
// 今の技の分類から導出する(P4-3、ADR-0300 §5)。
// 返ってきた値は加工せずに表示する(ADR-0300 §8)。技の相性・確定数の言葉も engine の値をそのまま使う。

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type AnimationEvent,
  type CSSProperties,
  type PointerEvent,
  type ReactElement,
  type ReactNode,
  type RefObject,
} from "react";
import {
  ATTACKER_PRESET_KEYS,
  DEFAULT_ATTACKER_PRESET,
  attackerPresetLabel,
  resolveAttackerPreset,
  type AttackerPresetKey,
} from "../domain/attackerPresets";
import { formatEffectiveness, formatKO, formatMoveCategory, formatPercentRange } from "../domain/format";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import {
  buildBulkRequest,
  buildIndividual,
  defaultAbility,
  defenderItemVariants,
  defensiveItemCandidates,
} from "../domain/requests";
import type {
  BulkResult,
  BulkRow,
  CalcEngine,
  EngineError,
  EngineResult,
  Item,
  Move,
  MoveCategory,
} from "../engine/types";
import { calcScreenText, isTypeId, typeNameJa } from "../i18n/ja";
import type { MasterData, MasterSpecies, MasterSpeciesSearch } from "../master/types";
import { prefersReducedMotion } from "../ui/motion";
import "./CalcScreen.css";

/**
 * 攻守入れ替えの演出(design.md「動き」: カードが入れ替わる(0.35秒))を、animationend が来なくても
 * (タブが裏にある等)必ず終わらせるまでの最大待ち時間。CSS の --duration-swap(styles/tokens.css)を
 * 少し上回る値にする(animationend が実際に来るまでの余裕。CSS の秒数そのものを TS に複製しない)。
 */
const SWAP_ANIMATION_MAX_WAIT_MS = 1000;

/**
 * 確定数バッジの弾み(design.md「動き」: --duration-pulse 0.4秒)を、animationend が来なくても
 * (タブが裏にある等)必ず終わらせるまでの最大待ち時間。CSS の --duration-pulse を上回る値にする
 * (animationend が実際に来るまでの余裕。CSS の秒数そのものを TS に複製しない。P4-9)。
 */
const KO_PULSE_ANIMATION_MAX_WAIT_MS = 1000;

/** ホロ効果のカード内の位置(0〜100%)。 */
interface HoloPosition {
  readonly xPercent: number;
  readonly yPercent: number;
}

/** ホロ効果の CSS カスタムプロパティ(--holo-x/--holo-y)込みのインラインスタイル。 */
interface HoloStyle extends CSSProperties {
  readonly "--holo-x"?: string;
  readonly "--holo-y"?: string;
}

/** 値を [0, 100] に収める(design.md「動き」: ホロ効果はカード内の位置(%)に連動)。 */
function clampPercent(value: number): number {
  return Math.min(100, Math.max(0, value));
}

/** 確定数バッジの弾みを、行ごとに追跡するためのキー(preset・itemId の組。CalcScreen.tsx の行の識別と同じ考え方)。 */
function koRowKey(row: BulkRow): string {
  return `${row.preset}-${row.itemId}`;
}

/**
 * 同時に光るカードを1枚だけにするための共有 ref(CalcScreen が1つ作り、両方のカードの useHoloCard に渡す)。
 * 値は「今光っているカードを消す関数」。state ではなく ref にするのは、これが変わったときに
 * CalcScreen まで再レンダーする必要が無いため(ホロの状態はカードの中に閉じる。CalcScreen.holoRender.test.tsx)。
 */
type ActiveHoloClearRef = RefObject<(() => void) | null>;

interface HoloCard {
  readonly isHolo: boolean;
  readonly style: HoloStyle | undefined;
  readonly onPointerMove: (event: PointerEvent<HTMLElement>) => void;
  readonly onPointerLeave: () => void;
}

/**
 * ホロ効果(design.md「動き」)。状態はこのカードの中だけに持ち、CalcScreen を再レンダーしない
 * (CalcScreen.holoRender.test.tsx「ホロの pointermove で画面全体を描き直さない」)。
 * 位置の反映は requestAnimationFrame で1フレームに1回へ間引く(1回の予約につき最後の位置だけ反映する。
 * P4-9)。ポインタが離れた・画面から消えたら、予約中のフレームを取り消す。mouse・pen 以外(touch)では
 * 何もしない(design.md「動き」: ホロはポインタ操作のときだけ)。
 */
function useHoloCard(activeClearRef: ActiveHoloClearRef): HoloCard {
  const [position, setPosition] = useState<HoloPosition | null>(null);
  const frameIdRef = useRef<number | null>(null);
  const pendingPositionRef = useRef<HoloPosition | null>(null);

  // このカードを消す関数(常に同じ参照にする。activeClearRef との比較に使うため)。
  const clear = useCallback((): void => {
    if (frameIdRef.current !== null) {
      cancelAnimationFrame(frameIdRef.current);
      frameIdRef.current = null;
    }
    pendingPositionRef.current = null;
    setPosition(null);
  }, []);

  // 画面から消えたら、予約中のフレームを取り消す(回しっぱなしにしない)。このカードが今光っている
  // カードだったときは、共有 ref も片付ける(消えたカードの clear を後から呼ばないように)。
  useEffect(() => {
    return () => {
      if (frameIdRef.current !== null) {
        cancelAnimationFrame(frameIdRef.current);
        frameIdRef.current = null;
      }
      if (activeClearRef.current === clear) {
        activeClearRef.current = null;
      }
    };
  }, [activeClearRef, clear]);

  function applyPendingPosition(): void {
    frameIdRef.current = null;
    const next = pendingPositionRef.current;
    pendingPositionRef.current = null;
    if (next === null) {
      return;
    }
    // 光り始めるとき: 前に光っていたカード(自分以外)があれば消す(同時に光るのは1枚だけ)。
    // カードを移るときはブラウザが先に pointerleave を送り、前のカードの予約は取り消されている前提
    // (同じフレーム内で A→B→A と動いて leave が来ない場合は、予約順で B が光ることがある)。
    if (activeClearRef.current !== clear) {
      activeClearRef.current?.();
      activeClearRef.current = clear;
    }
    setPosition(next);
  }

  function onPointerMove(event: PointerEvent<HTMLElement>): void {
    if (event.pointerType !== "mouse" && event.pointerType !== "pen") {
      return;
    }
    if (prefersReducedMotion()) {
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    pendingPositionRef.current = {
      xPercent: clampPercent(((event.clientX - rect.left) / rect.width) * 100),
      yPercent: clampPercent(((event.clientY - rect.top) / rect.height) * 100),
    };
    if (frameIdRef.current === null) {
      frameIdRef.current = requestAnimationFrame(applyPendingPosition);
    }
  }

  return {
    isHolo: position !== null,
    style:
      position === null
        ? undefined
        : {
            "--holo-x": `${String(Math.round(position.xPercent))}%`,
            "--holo-y": `${String(Math.round(position.yPercent))}%`,
          },
    onPointerMove,
    onPointerLeave: clear,
  };
}

/** 技を選んでいないときの、攻撃側プリセット表示用の仮の分類(A/C 表記の既定は物理と同じ)。 */
const DEFAULT_MOVE_CATEGORY: MoveCategory = "physical";

/** 計算画面(design.md「画面: ダメージ計算」)。engine と master は呼び出し側が注入する(ADR-0300 §2・§3)。 */
export interface CalcScreenProps {
  readonly engine: CalcEngine;
  readonly master: MasterData;
  /**
   * P4-16b(ADR-0304 A-10): 種族を都度引く口。`master.capabilities.speciesList` が false のとき、
   * ポケモンの選択をドロップダウンから検索欄に替えるために使う。省略は「検索できない」
   * (capabilities を省いたマスタ = 今までどおりドロップダウン)。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 計算の状態(判別 union)。idle は入力が揃っていない、status-move は変化技を選んでいる。 */
type Outcome =
  | { readonly status: "idle" }
  | { readonly status: "loading" }
  | { readonly status: "status-move" }
  | { readonly status: "success"; readonly result: BulkResult }
  | { readonly status: "error"; readonly error: EngineError };

/**
 * 直近に届いた calcBulk の応答と、それを生んだ入力(このオブジェクトの参照・値が今の入力と
 * 1つでも違えば、応答は古い入力に対するものとみなし loading 扱いにする。CalcScreen.test.tsx
 * 「古い計算の応答が後から届いても、新しい入力の結果を上書きしない」と、入力変更直後に
 * 古い行を出さないことの両方をこの1つの比較で満たす)。
 */
interface CompletedCalc {
  readonly attackerSpecies: MasterSpecies;
  readonly defenderSpecies: MasterSpecies;
  readonly move: Move;
  readonly attackerItem: Item | null;
  readonly defenderItem: Item | null;
  readonly compareItems: boolean;
  readonly attackerPresetKey: AttackerPresetKey;
  readonly result: EngineResult<BulkResult>;
}

/**
 * 攻撃側の技を、種族が変わった後も引き継ぐか決める(CalcScreen.test.tsx「攻撃側を変えたとき…」)。
 * 新しい攻撃側が今の技をそのまま覚えていればそれを使い、覚えていなければ最初のダメージ技に戻す。
 */
function resolveMoveId(species: MasterSpecies | null, moves: readonly Move[], currentMoveId: string): string {
  if (species === null) {
    return "";
  }
  if (learnsetMoves(species, moves).some((move) => move.id === currentMoveId)) {
    return currentMoveId;
  }
  return firstDamagingMove(species, moves)?.id ?? "";
}

/** 計算画面(design.md「画面: ダメージ計算」、ADR-0300 §2・§6)。攻撃側・防御側・技が揃うと自動で計算する。 */
export function CalcScreen({ engine, master }: CalcScreenProps) {
  const [attackerKey, setAttackerKey] = useState("");
  const [defenderKey, setDefenderKey] = useState("");
  const [attackerItemId, setAttackerItemId] = useState("");
  const [defenderItemId, setDefenderItemId] = useState("");
  const [moveId, setMoveId] = useState("");
  const [compareItems, setCompareItems] = useState(false);
  // 攻撃側プリセットの Key だけを持ち、攻撃側・技・攻守入れ替えでは変えない(ADR-0300 §5、
  // CalcScreen.test.tsx「攻撃側のプリセット(P4-3)」)。表示名・SP・性格は今の技の分類から毎レンダー導出する。
  const [attackerPresetKey, setAttackerPresetKey] = useState<AttackerPresetKey>(DEFAULT_ATTACKER_PRESET);
  // calcBulk の応答だけを state に持つ。idle・status-move・loading は入力から毎レンダー導出する
  // (effect の中で同期的に setState すると react-hooks/set-state-in-effect に引っかかるため)。
  const [completed, setCompleted] = useState<CompletedCalc | null>(null);

  // 確定数が変わった瞬間にバッジを弾ませる(design.md「動き」)。直近に処理した completed と、
  // 行のキー(koRowKey)ごとの確定数の文字を state に持つ。新しい completed が来たときだけ
  // (レンダー中に前回の completed と比べて)更新する(react-hooks/set-state-in-effect を避けるため、
  // effect ではなくレンダー本体で行う。React の「レンダー中に state を調整する」パターン)。
  const [koPulse, setKoPulse] = useState<{
    readonly lastCompleted: CompletedCalc | null;
    readonly koTexts: ReadonlyMap<string, string>;
    readonly pulsingKeys: ReadonlySet<string>;
  }>({ lastCompleted: null, koTexts: new Map(), pulsingKeys: new Set() });

  // 攻守入れ替えの演出(design.md「動き」)。animationend で外すほか、来なかったときのタイマーでも外す。
  const [swapping, setSwapping] = useState(false);
  const swapFallbackTimerRef = useRef<number | null>(null);

  // ホロ効果(design.md「動き」、P4-9)。状態はカード(useHoloCard)の中に持つ。ここでは
  // 「同時に光るのは1枚だけ」を保つための共有 ref だけを持つ(ref なので、これが変わっても
  // CalcScreen までは再レンダーしない。CalcScreen.holoRender.test.tsx)。
  const activeHoloClearRef = useRef<(() => void) | null>(null);

  const attackerSpecies = useMemo(
    () => master.species.find((species) => species.key === attackerKey) ?? null,
    [master.species, attackerKey],
  );
  const defenderSpecies = useMemo(
    () => master.species.find((species) => species.key === defenderKey) ?? null,
    [master.species, defenderKey],
  );
  const attackerItem = useMemo(
    () => master.items.find((item) => item.id === attackerItemId) ?? null,
    [master.items, attackerItemId],
  );
  const defenderItem = useMemo(
    () => master.items.find((item) => item.id === defenderItemId) ?? null,
    [master.items, defenderItemId],
  );
  const attackerMoves = useMemo(
    () => (attackerSpecies === null ? [] : learnsetMoves(attackerSpecies, master.moves)),
    [attackerSpecies, master.moves],
  );
  const move = useMemo(
    () => attackerMoves.find((candidate) => candidate.id === moveId) ?? null,
    [attackerMoves, moveId],
  );

  function selectAttacker(key: string): void {
    setAttackerKey(key);
    const species = master.species.find((candidate) => candidate.key === key) ?? null;
    setMoveId((prev) => resolveMoveId(species, master.moves, prev));
  }

  /** タイマーが残っていれば止める(2回目の入れ替えで前のタイマーが後から発火しないように)。 */
  function clearSwapFallbackTimer(): void {
    if (swapFallbackTimerRef.current !== null) {
      window.clearTimeout(swapFallbackTimerRef.current);
      swapFallbackTimerRef.current = null;
    }
  }

  /** 攻守入れ替えの演出を終える(animationend か、フォールバックのタイマーから呼ぶ)。 */
  function endSwapAnimation(): void {
    clearSwapFallbackTimer();
    setSwapping(false);
  }

  function swap(): void {
    const newAttackerSpecies = defenderSpecies;
    setAttackerKey(defenderKey);
    setDefenderKey(attackerKey);
    setAttackerItemId(defenderItemId);
    setDefenderItemId(attackerItemId);
    setMoveId((prev) => resolveMoveId(newAttackerSpecies, master.moves, prev));

    if (!prefersReducedMotion()) {
      setSwapping(true);
      clearSwapFallbackTimer();
      swapFallbackTimerRef.current = window.setTimeout(endSwapAnimation, SWAP_ANIMATION_MAX_WAIT_MS);
    }
  }

  // アンマウント時にフォールバックのタイマーを片付ける(回しっぱなしにしない)。
  useEffect(() => {
    return () => {
      clearSwapFallbackTimer();
    };
  }, []);

  // 確定数が変わった瞬間にバッジを弾ませる(design.md「動き」)。新しい成功結果が届いたときだけ判定する
  // (completed の参照が変わるのは calcBulk の応答が届いたときだけ)。
  if (completed !== null && completed !== koPulse.lastCompleted) {
    if (completed.result.ok) {
      const reduceMotion = prefersReducedMotion();
      const nextKoTexts = new Map(koPulse.koTexts);
      const changedKeys = new Set<string>();
      for (const row of completed.result.value.rows) {
        const key = koRowKey(row);
        const koText = formatKO(row.result.ko);
        const previousText = koPulse.koTexts.get(key);
        if (!reduceMotion && previousText !== undefined && previousText !== koText) {
          changedKeys.add(key);
        }
        nextKoTexts.set(key, koText);
      }
      setKoPulse({ lastCompleted: completed, koTexts: nextKoTexts, pulsingKeys: changedKeys });
    } else {
      setKoPulse((prev) => ({ ...prev, lastCompleted: completed, pulsingKeys: new Set() }));
    }
  }
  const pulsingKeys = koPulse.pulsingKeys;

  /** 確定数バッジの is-pulsing を外す(animationend。バブリングで子要素のアニメーションと混ざらないよう currentTarget と比べる)。 */
  function handleKoAnimationEnd(key: string) {
    return (event: AnimationEvent<HTMLSpanElement>): void => {
      if (event.target !== event.currentTarget) {
        return;
      }
      setKoPulse((prev) => {
        if (!prev.pulsingKeys.has(key)) {
          return prev;
        }
        const next = new Set(prev.pulsingKeys);
        next.delete(key);
        return { ...prev, pulsingKeys: next };
      });
    };
  }

  // 確定数バッジの弾みは animationend が来なくても(タブが裏にある等)最大待ちで必ず外す(P4-9)。
  // 弾み始めるたび(pulsingKeys が変わるたび)に掛け直す(前のタイマーが次の弾みを打ち切らないように)。
  // 視差効果を減らす設定では pulsingKeys が常に空なので、このタイマーも動かない。
  useEffect(() => {
    if (pulsingKeys.size === 0) {
      return;
    }
    const keysToClear = pulsingKeys;
    const timer = window.setTimeout(() => {
      setKoPulse((prev) => {
        if (prev.pulsingKeys.size === 0) {
          return prev;
        }
        const next = new Set(prev.pulsingKeys);
        for (const key of keysToClear) {
          next.delete(key);
        }
        return { ...prev, pulsingKeys: next };
      });
    }, KO_PULSE_ANIMATION_MAX_WAIT_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [pulsingKeys]);

  // 攻撃側・防御側・ダメージ技が揃ったら calcBulk を呼ぶ(ADR-0300 §2・§6)。
  // setState は応答が届いたとき(.then のコールバック)だけで行い、effect の本体では呼ばない
  // (react-hooks/set-state-in-effect)。入力が変わるたびに実行し直し、古い応答が新しい表示を
  // 上書きしないよう cancelled で無視する。idle・status-move は下の outcome で入力から直接導出する。
  useEffect(() => {
    if (attackerSpecies === null || defenderSpecies === null || move === null || move.category === "status") {
      return;
    }
    let cancelled = false;
    const { sp, nature } = resolveAttackerPreset(attackerPresetKey, move.category);
    const attackerIndividual = buildIndividual(attackerSpecies, {
      sp,
      nature,
      item: attackerItem,
      ability: defaultAbility(attackerSpecies, master.abilities),
    });
    const candidates = defensiveItemCandidates(master.items, move);
    const itemVariants = defenderItemVariants({
      selectedItem: defenderItem,
      compare: compareItems,
      candidates,
    });
    const request = buildBulkRequest({
      attacker: attackerIndividual,
      defenderSpecies,
      move,
      typeChart: master.typeChart,
      itemVariants,
    });
    // calcBulk は EngineResult(ok/not ok)で成否を運び、reject しない契約(ADR-0011 §5)。
    // それでも floating promise を残さないよう void で明示する。
    void engine.calcBulk(request).then((result) => {
      if (!cancelled) {
        setCompleted({
          attackerSpecies,
          defenderSpecies,
          move,
          attackerItem,
          defenderItem,
          compareItems,
          attackerPresetKey,
          result,
        });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [
    engine,
    master,
    attackerSpecies,
    defenderSpecies,
    move,
    attackerItem,
    defenderItem,
    compareItems,
    attackerPresetKey,
  ]);

  // idle・status-move は選ばれている入力から直接決まる。completed が無い、または今の入力と違う入力の
  // 応答(初回の読み込み中・入力を変えた直後)は loading にし、古い行を出さない(ADR-0300 §8)。
  let outcome: Outcome;
  if (attackerSpecies === null || defenderSpecies === null || move === null) {
    outcome = { status: "idle" };
  } else if (move.category === "status") {
    outcome = { status: "status-move" };
  } else if (
    completed === null ||
    completed.attackerSpecies !== attackerSpecies ||
    completed.defenderSpecies !== defenderSpecies ||
    completed.move !== move ||
    completed.attackerItem !== attackerItem ||
    completed.defenderItem !== defenderItem ||
    completed.compareItems !== compareItems ||
    completed.attackerPresetKey !== attackerPresetKey
  ) {
    outcome = { status: "loading" };
  } else {
    outcome = completed.result.ok
      ? { status: "success", result: completed.result.value }
      : { status: "error", error: completed.result.error };
  }

  return (
    <div className="calc-screen">
      <div className="calc-screen__cards">
        <SpeciesCard
          regionLabel={calcScreenText.attackerRegionLabel}
          speciesSelectLabel={calcScreenText.attackerPokemonLabel}
          itemSelectLabel={calcScreenText.attackerItemLabel}
          speciesList={master.species}
          items={master.items}
          selectedSpeciesKey={attackerKey}
          selectedItemId={attackerItemId}
          onSpeciesChange={selectAttacker}
          onItemChange={setAttackerItemId}
          isSwapping={swapping}
          onSwapAnimationEnd={endSwapAnimation}
          activeHoloClearRef={activeHoloClearRef}
        >
          {attackerSpecies !== null && (
            <AttackerPresetSelector
              category={move?.category ?? DEFAULT_MOVE_CATEGORY}
              value={attackerPresetKey}
              onChange={setAttackerPresetKey}
            />
          )}
        </SpeciesCard>
        <button type="button" className="calc-screen__swap" onClick={swap}>
          {calcScreenText.swapButtonLabel}
        </button>
        <SpeciesCard
          regionLabel={calcScreenText.defenderRegionLabel}
          speciesSelectLabel={calcScreenText.defenderPokemonLabel}
          itemSelectLabel={calcScreenText.defenderItemLabel}
          speciesList={master.species}
          items={master.items}
          selectedSpeciesKey={defenderKey}
          selectedItemId={defenderItemId}
          onSpeciesChange={setDefenderKey}
          onItemChange={setDefenderItemId}
          isSwapping={swapping}
          onSwapAnimationEnd={endSwapAnimation}
          activeHoloClearRef={activeHoloClearRef}
        />
      </div>

      <MoveSelect moves={attackerMoves} value={moveId} onChange={setMoveId} />

      <label className="calc-screen__compare">
        <input
          type="checkbox"
          checked={compareItems}
          onChange={(event) => {
            setCompareItems(event.target.checked);
          }}
        />
        {calcScreenText.compareItemCandidatesLabel}
      </label>

      <ResultsSection
        outcome={outcome}
        items={master.items}
        moveType={move?.type}
        pulsingKeys={pulsingKeys}
        onKoAnimationEnd={handleKoAnimationEnd}
      />
    </div>
  );
}

interface SpeciesCardProps {
  readonly regionLabel: string;
  readonly speciesSelectLabel: string;
  readonly itemSelectLabel: string;
  readonly speciesList: readonly MasterSpecies[];
  readonly items: readonly Item[];
  readonly selectedSpeciesKey: string;
  readonly selectedItemId: string;
  readonly onSpeciesChange: (key: string) => void;
  readonly onItemChange: (id: string) => void;
  /** カードの中に足す追加要素(攻撃側プリセットの選択。防御側カードは渡さない)。 */
  readonly children?: ReactNode;
  /** 攻守入れ替えの演出中か(design.md「動き」)。 */
  readonly isSwapping: boolean;
  readonly onSwapAnimationEnd: () => void;
  /** ホロ効果(design.md「動き」、P4-9): 同時に光るのは1枚だけにするための、カード間で共有する ref。 */
  readonly activeHoloClearRef: ActiveHoloClearRef;
}

/** 攻撃側・防御側の共通カード: ポケモン・持ち物の選択と、選んだ種族の名前・タイプ・エンブレム。 */
function SpeciesCard({
  regionLabel,
  speciesSelectLabel,
  itemSelectLabel,
  speciesList,
  items,
  selectedSpeciesKey,
  selectedItemId,
  onSpeciesChange,
  onItemChange,
  children,
  isSwapping,
  onSwapAnimationEnd,
  activeHoloClearRef,
}: SpeciesCardProps) {
  const species = speciesList.find((candidate) => candidate.key === selectedSpeciesKey) ?? null;
  const primaryType = species?.types[0];
  const holo = useHoloCard(activeHoloClearRef);
  const className = ["calc-card", isSwapping ? "is-swapping" : "", holo.isHolo ? "is-holo" : ""]
    .filter((part) => part !== "")
    .join(" ");
  return (
    <section
      className={className}
      aria-label={regionLabel}
      style={holo.style}
      onAnimationEnd={(event) => {
        // バブリングで子要素のアニメーション(バッジの弾み等)と混ざらないよう currentTarget と比べる。
        if (event.target === event.currentTarget) {
          onSwapAnimationEnd();
        }
      }}
      onPointerMove={holo.onPointerMove}
      onPointerLeave={holo.onPointerLeave}
    >
      <select
        aria-label={speciesSelectLabel}
        value={selectedSpeciesKey}
        onChange={(event) => {
          onSpeciesChange(event.target.value);
        }}
      >
        <option value="" hidden />
        {speciesList.map((candidate) => (
          <option key={candidate.key} value={candidate.key}>
            {candidate.nameJa}
          </option>
        ))}
      </select>
      <select
        aria-label={itemSelectLabel}
        value={selectedItemId}
        onChange={(event) => {
          onItemChange(event.target.value);
        }}
      >
        <option value="">{calcScreenText.noItemOption}</option>
        {items.map((item) => (
          <option key={item.id} value={item.id}>
            {item.nameJa}
          </option>
        ))}
      </select>
      {species !== null && primaryType !== undefined && (
        <div className="calc-card__info">
          <span
            className="calc-card__emblem"
            data-testid="type-emblem"
            style={{ backgroundColor: `var(--type-${primaryType})` }}
          />
          <h2 className="calc-card__name">{species.nameJa}</h2>
          <ul className="calc-card__types">
            {species.types.map((type) => (
              <li key={type} className="calc-card__type" style={{ color: `var(--type-${type})` }}>
                {/* マスタ由来の type は相性表の18種に限らないので、型ガードで確かめ、
                    未知の ID はそのまま出す(未知データで画面を壊さない)。 */}
                {isTypeId(type) ? typeNameJa[type] : type}
              </li>
            ))}
          </ul>
        </div>
      )}
      {children}
    </section>
  );
}

interface AttackerPresetSelectorProps {
  readonly category: MoveCategory;
  readonly value: AttackerPresetKey;
  readonly onChange: (key: AttackerPresetKey) => void;
}

/**
 * 攻撃側プリセットのピル型ラジオグループ(design.md「入力はタップで選ぶ」、ADR-0300 §5)。
 * 選んでいるのは Key で、表示名は今の技の分類(category)から導出する
 * (CalcScreen.test.tsx「A特化のまま特殊技に替えると…」)。
 */
function AttackerPresetSelector({ category, value, onChange }: AttackerPresetSelectorProps) {
  // ラジオの name は画面内で一意にする(同じ部品を複数置いてもグループが混ざらないように)。
  const groupName = useId();
  return (
    <div role="radiogroup" aria-label={calcScreenText.attackerPresetGroupLabel} className="calc-preset">
      {ATTACKER_PRESET_KEYS.map((key) => {
        const selected = key === value;
        return (
          <label
            key={key}
            className={`calc-preset__option${selected ? " calc-preset__option--selected" : ""}`}
          >
            <input
              type="radio"
              name={groupName}
              className="calc-preset__input"
              checked={selected}
              onChange={() => {
                onChange(key);
              }}
            />
            {attackerPresetLabel(key, category)}
          </label>
        );
      })}
    </div>
  );
}

interface MoveSelectProps {
  readonly moves: readonly Move[];
  readonly value: string;
  readonly onChange: (moveId: string) => void;
}

/** 技セレクタ。learnset の順のまま、分類と威力(変化技は威力を出さない)を併記する。 */
function MoveSelect({ moves, value, onChange }: MoveSelectProps) {
  return (
    <select
      aria-label={calcScreenText.moveLabel}
      value={value}
      onChange={(event) => {
        onChange(event.target.value);
      }}
    >
      {moves.map((move) => (
        <option key={move.id} value={move.id}>
          {move.nameJa}
          {calcScreenText.moveOptionSeparator}
          {formatMoveCategory(move.category)}
          {move.category === "status"
            ? ""
            : `${calcScreenText.moveOptionSeparator}${calcScreenText.movePowerLabel}${String(move.power)}`}
        </option>
      ))}
    </select>
  );
}

interface ResultsSectionProps {
  readonly outcome: Outcome;
  readonly items: readonly Item[];
  /** ダメージバーの色に使う、選ばれている技のタイプ(design.md: バーは技のタイプ色)。 */
  readonly moveType: string | undefined;
  /** 確定数が変わって弾ませる行のキー(koRowKey)の集合(design.md「動き」)。 */
  readonly pulsingKeys: ReadonlySet<string>;
  readonly onKoAnimationEnd: (key: string) => (event: AnimationEvent<HTMLSpanElement>) => void;
}

/**
 * 計算結果の表示(ADR-0300 §8: 返ってきた値を加工せずに表示する)。loading は、新しい入力に対する
 * 応答をまだ待っている間、古い行を出さないための表示(CalcScreen.test.tsx「入力を変えたら…」)。
 */
function ResultsSection({
  outcome,
  items,
  moveType,
  pulsingKeys,
  onKoAnimationEnd,
}: ResultsSectionProps): ReactElement | null {
  switch (outcome.status) {
    case "idle":
      return null;
    case "loading":
      return (
        <div className="calc-results" aria-busy="true">
          <p className="calc-screen__notice">{calcScreenText.loadingNotice}</p>
        </div>
      );
    case "status-move":
      return <p className="calc-screen__notice">{calcScreenText.statusMoveNotice}</p>;
    case "error":
      return (
        <p role="alert" className="calc-screen__error">
          {outcome.error.message}
        </p>
      );
    case "success":
      return (
        <ResultsList
          result={outcome.result}
          items={items}
          moveType={moveType}
          pulsingKeys={pulsingKeys}
          onKoAnimationEnd={onKoAnimationEnd}
        />
      );
    default: {
      // 判別 union の網羅性チェック(コーディング規約 §4 TypeScript「判別 union は網羅性を検査する」)。
      // eslint の switch-exhaustiveness-check に加え、実行時にも未知の状態を検出する。
      const exhaustive: never = outcome;
      throw new Error(`未知の Outcome: ${JSON.stringify(exhaustive)}`);
    }
  }
}

interface ResultsListProps {
  readonly result: BulkResult;
  readonly items: readonly Item[];
  readonly moveType: string | undefined;
  readonly pulsingKeys: ReadonlySet<string>;
  readonly onKoAnimationEnd: (key: string) => (event: AnimationEvent<HTMLSpanElement>) => void;
}

/** 最大100%の一括表示予算(design.md のダメージバー)。100%を超える分は頭打ちにする。 */
const DAMAGE_BAR_MAX_PERCENT = 100;

/**
 * 行一覧(ADR-0300 §8: engine の順のまま、加工せずに表示)。技の相性(effectiveness)は調整(preset)や
 * 持ち物のバリアントが変わっても同じ値になる(防御側の種族・技のタイプだけで決まる)ため、行ごとに
 * 繰り返さず、結果全体の先頭行の値を1回だけ表示する。
 */
function ResultsList({ result, items, moveType, pulsingKeys, onKoAnimationEnd }: ResultsListProps) {
  const firstRow = result.rows[0];
  const barColor =
    moveType === undefined || moveType === "" ? "var(--text-secondary)" : `var(--type-${moveType})`;
  return (
    <div className="calc-results">
      {firstRow !== undefined && (
        <p className="calc-results__effectiveness">
          <strong>{formatEffectiveness(firstRow.result.effectiveness)}</strong>
        </p>
      )}
      <ul aria-label={calcScreenText.resultsListLabel} className="calc-results__list">
        {result.rows.map((row, index) => {
          const itemLabel =
            row.itemId === ""
              ? calcScreenText.noItemRowLabel
              : (items.find((item) => item.id === row.itemId)?.nameJa ?? row.itemId);
          const barValue = Math.min(row.result.maxPercent, DAMAGE_BAR_MAX_PERCENT);
          const koKey = koRowKey(row);
          const koClassName = `calc-results__ko${pulsingKeys.has(koKey) ? " is-pulsing" : ""}`;
          return (
            // preset・itemId の組は行内で一意ではない場合がある(同じ preset で持ち物違い)ため index も足す。
            <li key={`${row.preset}-${row.itemId}-${String(index)}`} className="calc-results__row">
              <span className="calc-results__preset">{row.presetLabel}</span>
              <span className="calc-results__item">{itemLabel}</span>
              <span className="calc-results__percent">{formatPercentRange(row.result)}</span>
              <span className={koClassName} onAnimationEnd={onKoAnimationEnd(koKey)}>
                {formatKO(row.result.ko)}
              </span>
              <div
                role="meter"
                aria-valuemin={0}
                aria-valuemax={DAMAGE_BAR_MAX_PERCENT}
                aria-valuenow={barValue}
                className="calc-results__bar"
              >
                <div
                  className="calc-results__bar-fill"
                  style={{ width: `${String(barValue)}%`, backgroundColor: barColor }}
                />
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
