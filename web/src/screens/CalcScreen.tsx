// P4-2/P4-3: 計算画面(docs/design.md「画面: ダメージ計算」、ADR-0300 §2・§5・§6)。
// engine には CalcEngine(差し替え口)、マスタには MasterData(いまは架空の例データ)を渡してもらう。
// 攻撃側は「技」の直後の「攻撃」「特攻」の2ブロック(プリセット・SP の数値・性格補正。
// domain/attackerStatInputs.ts、既定は無振り・補正なし)で決め、選んだ技が使う方を強調する(ADR-0329)。
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
import { ATTACKER_PRESET_KEYS, attackerPresetLabel } from "../domain/attackerPresets";
import {
  ATTACK_STATS,
  DEFAULT_ATTACKER_STAT_INPUTS,
  NATURE_MODIFIERS,
  attackStatFor,
  isModifierSelectable,
  matchingPreset,
  presetInput,
  resolveAttackerStats,
  type AttackerStatIssue,
  type AttackStat,
  type AttackStatInput,
  type AttackerStatInputs,
  type NatureModifier,
} from "../domain/attackerStatInputs";
import {
  DEFAULT_CALC_CONDITIONS,
  conditionRequestParts,
  defenderRankStatFor,
  rankStatFor,
  type CalcConditions,
} from "../domain/calcConditions";
import { abilityNamesLabel } from "../domain/abilityLabels";
import { formatEffectiveness, formatKO, formatMoveCategory, formatPercentRange } from "../domain/format";
import {
  itemIdAfterSpeciesChange,
  lockedOrChosenItem,
  megaItemLock,
  megaStoneItemIds,
  type MegaItemLock,
} from "../domain/mega";
import { itemAfterRoleChange, itemsForRole, itemsWithStoneLabels, megaStoneLabel } from "../domain/itemRoles";
import { damagingLearnsetMoves, firstDamagingMove, isStatusMove } from "../domain/moves";
import { MAX_ITEM_VARIANTS } from "../domain/requestLimits";
import {
  buildBulkRequest,
  buildIndividual,
  NO_ABILITY,
  defenderAbilityCandidates,
  defenderItemVariants,
  defensiveItemCandidates,
  selectableAbilities,
} from "../domain/requests";
import { splitUnsupportedMarks, unsupportedMarkLabels } from "../domain/unsupportedLabels";
import type {
  Ability,
  BulkResult,
  BulkRow,
  CalcEngine,
  EngineError,
  EngineResult,
  Item,
  Move,
  MoveCategory,
} from "../engine/types";
import {
  attackerStatText,
  calcScreenText,
  frequentOpponentsText,
  isTypeId,
  masterOnlineText,
  megaItemText,
  requestLimitText,
  typeNameJa,
  unsupportedText,
} from "../i18n/ja";
import { itemRoleText } from "../i18n/items";
import { masterCapabilities } from "../master/capabilities";
import type {
  MasterData,
  MasterSpecies,
  MasterSpeciesResolution,
  MasterSpeciesSearch,
} from "../master/types";
import { prefersReducedMotion } from "../ui/motion";
import { AddFavoriteButton } from "../favorites/AddFavoriteButton";
import type { RecordClient } from "../record/recordClient";
import { useFrequentOpponents } from "../record/useFrequentOpponents";
import { resolveNatureId } from "../api/apiEngine";
import { favoriteInputOf } from "../favorites/favoriteInput";
import { MegaItemReason } from "./MegaItemReason";
import { SpeciesSearchField } from "./SpeciesSearchField";
import { useSpeciesResolutions } from "./speciesResolution";
import { PokemonImage } from "../images/PokemonImage";
import { AbilitySelect, type AbilitySelectConfig } from "./AbilitySelect";
import { CalcConditionsPanel } from "./CalcConditionsPanel";
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
  return `${row.preset}-${row.itemId}-${row.abilityId ?? ""}`;
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
  /**
   * P5-5c(ADR-0317 §2): 「よく計算する相手」を引く口。App は計算モードがオンラインのときだけ渡す。
   * 省略は「出さない」(失敗・0件・オフラインと同じく黙って非表示。計算には影響しない)。
   */
  readonly recordClient?: RecordClient;
  /** P5-3c(ADR-0327): 攻撃側をお気に入りに追加できたとき(App がお気に入りタブの一覧を取り直させる)。 */
  readonly onFavoriteAdded?: () => void;
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
  readonly attackerStatInputs: AttackerStatInputs;
  readonly attackerAbility: Ability;
  readonly defenderAbilities: readonly Ability[];
  readonly conditions: CalcConditions;
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
  if (damagingLearnsetMoves(species, moves).some((move) => move.id === currentMoveId)) {
    return currentMoveId;
  }
  return firstDamagingMove(species, moves)?.id ?? "";
}

/** 計算画面(design.md「画面: ダメージ計算」、ADR-0300 §2・§6)。攻撃側・防御側・技が揃うと自動で計算する。 */
export function CalcScreen({ engine, master, masterSearch, recordClient, onFavoriteAdded }: CalcScreenProps) {
  // P4-16b(ADR-0304 A-2・A-9・A-10): 使える機能。capabilities を省いたマスタ(オフライン相当)は全部使える。
  const capabilities = masterCapabilities(master);
  // 検索で解決した種族・特性の覚え書き(capabilities.speciesList が true のときは常に空のまま。ADR-0304 A-10)。
  const {
    speciesFor,
    abilitiesFor,
    movesFor,
    resolvedSpecies,
    register: registerSpeciesResolution,
  } = useSpeciesResolutions();
  const compareReasonId = useId();
  const frequentOpponents = useFrequentOpponents(recordClient, master, masterSearch);
  const [attackerKey, setAttackerKey] = useState("");
  const [defenderKey, setDefenderKey] = useState("");
  const [attackerItemId, setAttackerItemId] = useState("");
  const [defenderItemId, setDefenderItemId] = useState("");
  // ADR-0326: 攻守入れ替えで役割に合わず外した持ち物の名前(通知用)。選び直し・種族の変更で消す。
  const [attackerDroppedName, setAttackerDroppedName] = useState<string | null>(null);
  const [defenderDroppedName, setDefenderDroppedName] = useState<string | null>(null);
  const [moveId, setMoveId] = useState("");
  const [compareItems, setCompareItems] = useState(false);
  // 特性の選択(issue 272、ADR-0311)。"" は攻撃側では「種族の先頭」、防御側では「おまかせ(種族の全特性)」。
  // 種族を変えたら "" に戻す(古い選択を引き継がない)。
  const [attackerAbilityId, setAttackerAbilityId] = useState("");
  const [defenderAbilityId, setDefenderAbilityId] = useState("");
  // 「詳細」の条件(issue 274、ADR-0312)。攻守入れ替え・種族・技の変更では消さない。
  const [conditions, setConditions] = useState<CalcConditions>(DEFAULT_CALC_CONDITIONS);
  // 攻撃側の「攻撃」「特攻」の入力(SP の文字列・性格補正)。技・種族・攻守入れ替えでは変えない
  // (ADR-0329 §6、ADR-0312 §6 と同じ寿命)。プリセットの選択状態は持たず、値から毎レンダー導く。
  const [attackerStatInputs, setAttackerStatInputs] = useState<AttackerStatInputs>(
    DEFAULT_ATTACKER_STAT_INPUTS,
  );
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
    () => speciesFor(master.species, attackerKey),
    [master.species, attackerKey, speciesFor],
  );
  const defenderSpecies = useMemo(
    () => speciesFor(master.species, defenderKey),
    [master.species, defenderKey, speciesFor],
  );
  // issue 515(ADR-0320): メガ種族の持ち物はメガストーンに固定する。固定は毎レンダー種族から導き、表示と要求に使う。
  // メガストーンは単独の選択肢・候補に出さない(判別集合は、全件の一覧 + 検索で解決した種族から導く)。
  const stoneIds = useMemo(
    () => megaStoneItemIds([...master.species, ...resolvedSpecies]),
    [master.species, resolvedSpecies],
  );
  // ADR-0326: 攻撃側は attacker、防御側(と持ち物の比較候補)は defender の持ち物だけ。固定は全件から引く。
  const attackerPickable = useMemo(
    () => itemsForRole(master.items, "attacker", stoneIds),
    [master.items, stoneIds],
  );
  const defenderPickable = useMemo(
    () => itemsForRole(master.items, "defender", stoneIds),
    [master.items, stoneIds],
  );
  // 結果の行・未対応の印は持ち物を ID から引く。メガストーンの英語名を出さない(ADR-0326 §4)。
  const displayItems = useMemo(
    () => itemsWithStoneLabels(master.items, [attackerSpecies, defenderSpecies], stoneIds),
    [master.items, attackerSpecies, defenderSpecies, stoneIds],
  );
  const attackerLock = useMemo(
    () => megaItemLock(attackerSpecies, master.items),
    [attackerSpecies, master.items],
  );
  const defenderLock = useMemo(
    () => megaItemLock(defenderSpecies, master.items),
    [defenderSpecies, master.items],
  );
  const attackerItem = useMemo(
    () => lockedOrChosenItem(attackerLock, attackerPickable, attackerItemId),
    [attackerLock, attackerPickable, attackerItemId],
  );
  const defenderItem = useMemo(
    () => lockedOrChosenItem(defenderLock, defenderPickable, defenderItemId),
    [defenderLock, defenderPickable, defenderItemId],
  );
  // 防御側がメガ種族のときは持ち物の候補を比較しない(メガストーン1件に固定)。
  const compareDisabledByMega = defenderLock.kind !== "none";
  const effectiveCompareItems = compareItems && !compareDisabledByMega;
  const attackerMoves = useMemo(
    () =>
      attackerSpecies === null
        ? []
        : damagingLearnsetMoves(attackerSpecies, movesFor(master.moves, attackerKey)),
    [attackerSpecies, master.moves, attackerKey, movesFor],
  );
  // P4-17(ADR-0304 A-13): 技セレクトが使えるのは capabilities.moves が true、または攻撃側の技の候補が
  // 1件以上あるとき(種族が解決済みで learnset が1件以上ある)。案内の表示条件もこれと同じにする。
  const movesAvailable = capabilities.moves || attackerMoves.length > 0;
  const move = useMemo(
    () => attackerMoves.find((candidate) => candidate.id === moveId) ?? null,
    [attackerMoves, moveId],
  );
  // P4-19(issue 110、ADR-0208): 防御側の持ち物の通り(itemVariants)を組み立て、上限で絞り込んだかを
  // 画面に出す。useEffect の依存に truncated を含む新しい配列を毎回作らないよう、ここで useMemo にする
  // (react-hooks/set-state-in-effect の無限ループを避ける)。
  const itemVariantsResult = useMemo(() => {
    const candidates = move === null ? [] : defensiveItemCandidates(defenderPickable, move);
    return defenderItemVariants({ selectedItem: defenderItem, compare: effectiveCompareItems, candidates });
  }, [defenderPickable, move, defenderItem, effectiveCompareItems]);

  // 特性の選択肢と、calcBulk に渡す特性(issue 272、ADR-0311)。
  const attackerAbilityOptions = useMemo(
    () =>
      attackerSpecies === null
        ? []
        : selectableAbilities(attackerSpecies, abilitiesFor(master.abilities, attackerKey)),
    [attackerSpecies, master.abilities, attackerKey, abilitiesFor],
  );
  const attackerAbility = useMemo(
    () =>
      attackerAbilityOptions.find((ability) => ability.id === attackerAbilityId) ??
      attackerAbilityOptions[0] ??
      NO_ABILITY,
    [attackerAbilityOptions, attackerAbilityId],
  );
  const defenderAbilityOptions = useMemo(
    () =>
      defenderSpecies === null
        ? []
        : selectableAbilities(defenderSpecies, abilitiesFor(master.abilities, defenderKey)),
    [defenderSpecies, master.abilities, defenderKey, abilitiesFor],
  );
  const defenderAbilities = useMemo(
    () =>
      defenderSpecies === null
        ? []
        : defenderAbilityCandidates(
            defenderSpecies,
            defenderAbilityOptions,
            defenderAbilityId === "" ? null : defenderAbilityId,
          ),
    [defenderSpecies, defenderAbilityOptions, defenderAbilityId],
  );

  // ADR-0329 §3〜§5: 攻撃側の入力を要求の SP・性格にする(不正・性格が解決できないときは ok=false)。
  // 技の分類が決まるまでは物理扱い。
  const attackerStats = useMemo(
    () => resolveAttackerStats(attackerStatInputs, master.natures, move?.category ?? DEFAULT_MOVE_CATEGORY),
    [attackerStatInputs, master.natures, move],
  );

  // P5-3c(ADR-0327 §2): お気に入りに入れる攻撃側(種族・性格・SP・持ち物)。入力が不正なら入れない(ADR-0329 §7)。
  const favoriteInput = useMemo(() => {
    if (attackerSpecies === null || !attackerStats.ok) {
      return null;
    }
    const natureId = resolveNatureId(master.natures, attackerStats.nature);
    if (natureId === undefined) {
      return null;
    }
    return favoriteInputOf({
      label: attackerSpecies.nameJa,
      speciesKey: attackerSpecies.key,
      natureId,
      sp: attackerStats.sp,
      itemId: attackerItem?.id ?? null,
    });
  }, [attackerSpecies, attackerStats, master.natures, attackerItem]);

  function selectAttacker(key: string): void {
    setAttackerKey(key);
    setAttackerAbilityId("");
    setAttackerDroppedName(null);
    const species = speciesFor(master.species, key);
    changeAttackerItem(species);
    setMoveId((prev) => resolveMoveId(species, movesFor(master.moves, key), prev));
  }

  /**
   * P4-16b/P4-17(ADR-0304 A-10・A-13): 検索で攻撃側の種族が解決したとき。resolution.species・
   * resolution.moves をそのまま使う(register の setState は非同期なので、直後に movesFor/speciesFor で
   * 引き直すと古い覚え書きのままになる。既存の種族の扱いと同じ理由)。
   */
  function handleAttackerResolved(resolution: MasterSpeciesResolution): void {
    registerSpeciesResolution(resolution);
    setAttackerKey(resolution.species.key);
    setAttackerAbilityId("");
    setAttackerDroppedName(null);
    changeAttackerItem(resolution.species);
    // movesFor(master.moves, key) は使わない(register の setState 直後はまだ古い覚え書きのまま)。
    // movesFor が最終的に返す形(master.moves + 解決で覚えた分)をここで直接組み立てる。
    setMoveId((prev) => resolveMoveId(resolution.species, [...master.moves, ...resolution.moves], prev));
  }

  /** P4-16b(ADR-0304 A-10): 検索で防御側の種族が解決したとき(防御側は技を持たないので moveId は変えない)。 */
  function handleDefenderResolved(resolution: MasterSpeciesResolution): void {
    registerSpeciesResolution(resolution);
    setDefenderKey(resolution.species.key);
    setDefenderAbilityId("");
    setDefenderDroppedName(null);
    setDefenderItemId(
      itemIdAfterSpeciesChange({
        previous: defenderSpecies,
        next: resolution.species,
        items: master.items,
        currentItemId: defenderItem?.id ?? "",
      }),
    );
  }

  function selectDefender(key: string): void {
    setDefenderKey(key);
    setDefenderAbilityId("");
    setDefenderDroppedName(null);
    setDefenderItemId(
      itemIdAfterSpeciesChange({
        previous: defenderSpecies,
        next: speciesFor(master.species, key),
        items: master.items,
        currentItemId: defenderItem?.id ?? "",
      }),
    );
  }

  /** 攻撃側の種族を変えたときの持ち物(メガは固定、メガから非メガへは未選択に戻す)。 */
  function changeAttackerItem(next: MasterSpecies | null): void {
    setAttackerItemId(
      itemIdAfterSpeciesChange({
        previous: attackerSpecies,
        next,
        items: master.items,
        currentItemId: attackerItem?.id ?? "",
      }),
    );
  }

  function chooseAttackerItem(id: string): void {
    setAttackerItemId(id);
    setAttackerDroppedName(null);
  }

  function chooseDefenderItem(id: string): void {
    setDefenderItemId(id);
    setDefenderDroppedName(null);
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
    const newAttackerMoves = movesFor(master.moves, defenderKey);
    setAttackerKey(defenderKey);
    setDefenderKey(attackerKey);
    setAttackerAbilityId("");
    setDefenderAbilityId("");
    // 持ち物は種族に付いて動く(固定したメガストーンも、入れ替え先で同じ種族の固定として導かれる)。
    // 役割に合わなくなった持ち物は外して通知する(ADR-0326。固定のメガストーンは外さない)。
    const nextAttacker = itemAfterRoleChange({
      items: master.items,
      role: "attacker",
      currentItemId: defenderItem?.id ?? "",
    });
    const nextDefender = itemAfterRoleChange({
      items: master.items,
      role: "defender",
      currentItemId: attackerItem?.id ?? "",
    });
    setAttackerItemId(nextAttacker.itemId);
    setDefenderItemId(nextDefender.itemId);
    setAttackerDroppedName(nextAttacker.dropped?.nameJa ?? null);
    setDefenderDroppedName(nextDefender.dropped?.nameJa ?? null);
    setMoveId((prev) => resolveMoveId(newAttackerSpecies, newAttackerMoves, prev));

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
  // issue 248(issue 113 で ReverseScreen に入れたのと同じ形): AbortController は effect ごとに作り、
  // cleanup(依存が変わった・アンマウント)で abort する(古い計算に「もう要らない」を伝え、gateway 側の
  // 取り消し伝播〈issue 113〉を活かす。オンラインでないときは calcBulk 側が signal を無視するだけ)。
  useEffect(() => {
    if (
      attackerSpecies === null ||
      defenderSpecies === null ||
      move === null ||
      move.category === "status" ||
      !attackerStats.ok
    ) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();
    const { sp, nature } = attackerStats;
    const parts = conditionRequestParts(conditions);
    const attackerIndividual = buildIndividual(attackerSpecies, {
      sp,
      nature,
      item: attackerItem,
      ability: attackerAbility,
      ...(parts.status === undefined ? {} : { status: parts.status }),
      ...(parts.ranks === undefined ? {} : { ranks: parts.ranks }),
    });
    const { variants: itemVariants } = itemVariantsResult;
    const request = buildBulkRequest({
      attacker: attackerIndividual,
      defenderSpecies,
      move,
      typeChart: master.typeChart,
      itemVariants,
      defenderAbilities,
      ...(parts.critical === undefined ? {} : { critical: parts.critical }),
      ...(parts.field === undefined ? {} : { field: parts.field }),
      ...(parts.defenderOverride === undefined ? {} : { defenderOverride: parts.defenderOverride }),
    });
    // calcBulk は EngineResult(ok/not ok)で成否を運び、reject しない契約(ADR-0011 §5)。
    // それでも floating promise を残さないよう void で明示する。
    void engine.calcBulk(request, controller.signal).then((result) => {
      if (!cancelled) {
        setCompleted({
          attackerSpecies,
          defenderSpecies,
          move,
          attackerItem,
          defenderItem,
          compareItems: effectiveCompareItems,
          attackerStatInputs,
          attackerAbility,
          defenderAbilities,
          conditions,
          result,
        });
      }
    });
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [
    engine,
    master,
    attackerSpecies,
    defenderSpecies,
    move,
    attackerItem,
    defenderItem,
    effectiveCompareItems,
    attackerStats,
    attackerStatInputs,
    attackerAbility,
    defenderAbilities,
    conditions,
    itemVariantsResult,
  ]);

  // idle・status-move は選ばれている入力から直接決まる。completed が無い、または今の入力と違う入力の
  // 応答(初回の読み込み中・入力を変えた直後)は loading にし、古い行を出さない(ADR-0300 §8)。
  let outcome: Outcome;
  if (attackerSpecies === null || defenderSpecies === null || move === null) {
    outcome = { status: "idle" };
  } else if (isStatusMove(move)) {
    outcome = { status: "status-move" };
  } else if (!attackerStats.ok) {
    // 攻撃側の入力が不正なときは計算せず、古い行も出さない(理由は攻撃側の入力のすぐ下に出す。ADR-0329 §5)。
    outcome = { status: "idle" };
  } else if (
    completed === null ||
    completed.attackerSpecies !== attackerSpecies ||
    completed.defenderSpecies !== defenderSpecies ||
    completed.move !== move ||
    completed.attackerItem !== attackerItem ||
    completed.defenderItem !== defenderItem ||
    completed.compareItems !== effectiveCompareItems ||
    completed.attackerStatInputs !== attackerStatInputs ||
    completed.attackerAbility !== attackerAbility ||
    completed.defenderAbilities !== defenderAbilities ||
    completed.conditions !== conditions
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
          species={attackerSpecies}
          speciesListAvailable={capabilities.speciesList}
          speciesList={master.species}
          masterSearch={masterSearch}
          items={attackerPickable}
          itemLock={attackerLock}
          droppedNotice={
            attackerDroppedName === null ? null : itemRoleText.droppedNotice(attackerDroppedName, "attacker")
          }
          selectedSpeciesKey={attackerKey}
          selectedItemId={attackerItem?.id ?? ""}
          onSpeciesChange={selectAttacker}
          onSpeciesResolved={handleAttackerResolved}
          onItemChange={chooseAttackerItem}
          abilitySelect={{
            ariaLabel: calcScreenText.attackerAbilityLabel,
            options: attackerAbilityOptions,
            value: attackerAbility.id,
            onChange: setAttackerAbilityId,
          }}
          isSwapping={swapping}
          onSwapAnimationEnd={endSwapAnimation}
          activeHoloClearRef={activeHoloClearRef}
        >
          {recordClient !== undefined && (
            <AddFavoriteButton recordClient={recordClient} input={favoriteInput} onAdded={onFavoriteAdded} />
          )}
        </SpeciesCard>
        <button type="button" className="calc-screen__swap" onClick={swap}>
          {calcScreenText.swapButtonLabel}
        </button>
        <SpeciesCard
          regionLabel={calcScreenText.defenderRegionLabel}
          speciesSelectLabel={calcScreenText.defenderPokemonLabel}
          itemSelectLabel={calcScreenText.defenderItemLabel}
          species={defenderSpecies}
          speciesListAvailable={capabilities.speciesList}
          speciesList={master.species}
          masterSearch={masterSearch}
          items={defenderPickable}
          itemLock={defenderLock}
          droppedNotice={
            defenderDroppedName === null ? null : itemRoleText.droppedNotice(defenderDroppedName, "defender")
          }
          selectedSpeciesKey={defenderKey}
          selectedItemId={defenderItem?.id ?? ""}
          onSpeciesChange={selectDefender}
          onSpeciesResolved={handleDefenderResolved}
          onItemChange={chooseDefenderItem}
          abilitySelect={{
            ariaLabel: calcScreenText.defenderAbilityLabel,
            options: defenderAbilityOptions,
            value: defenderAbilityId,
            onChange: setDefenderAbilityId,
            autoOptionLabel: calcScreenText.anyAbilityOption,
          }}
          isSwapping={swapping}
          onSwapAnimationEnd={endSwapAnimation}
          activeHoloClearRef={activeHoloClearRef}
        />
      </div>

      <MoveSelect moves={attackerMoves} value={moveId} onChange={setMoveId} disabled={!movesAvailable} />
      {!movesAvailable && <p className="calc-screen__notice">{masterOnlineText.movesUnavailable}</p>}
      {attackerSpecies !== null && capabilities.moves && attackerMoves.length === 0 && (
        <p className="calc-screen__notice">{calcScreenText.noDamagingMovesNotice}</p>
      )}

      {attackerSpecies !== null && (
        <AttackerStatBlocks
          inputs={attackerStatInputs}
          usedStat={attackStatFor(move?.category ?? null)}
          issues={attackerStats.ok ? [] : attackerStats.issues}
          onChange={setAttackerStatInputs}
        />
      )}

      <label className="calc-screen__compare">
        <input
          type="checkbox"
          checked={effectiveCompareItems}
          disabled={!capabilities.effects || compareDisabledByMega}
          aria-describedby={compareDisabledByMega ? compareReasonId : undefined}
          onChange={(event) => {
            setCompareItems(event.target.checked);
          }}
        />
        {calcScreenText.compareItemCandidatesLabel}
      </label>
      {compareDisabledByMega && (
        <p id={compareReasonId} className="calc-screen__notice">
          {megaItemText.compareDisabledReason}
        </p>
      )}
      {!capabilities.effects && (
        <p className="calc-screen__notice">{masterOnlineText.itemCandidatesUnavailable}</p>
      )}
      {itemVariantsResult.truncated && (
        <p className="calc-screen__notice">{requestLimitText.itemCandidatesTruncated(MAX_ITEM_VARIANTS)}</p>
      )}

      <CalcConditionsPanel
        conditions={conditions}
        onChange={setConditions}
        rankStat={rankStatFor(move?.category ?? null)}
        defenderRankStat={defenderRankStatFor(move?.category ?? null)}
      />

      <ResultsSection
        outcome={outcome}
        items={displayItems}
        moves={master.moves}
        abilities={master.abilities}
        moveType={move?.type}
        defenderHasAbilityChoice={defenderAbilityOptions.length > 1}
        pulsingKeys={pulsingKeys}
        onKoAnimationEnd={handleKoAnimationEnd}
      />

      {frequentOpponents.length > 0 && (
        <div role="group" aria-label={frequentOpponentsText.groupLabel} className="calc-screen__frequent">
          {frequentOpponents.map((chip) => (
            <button
              key={chip.key}
              type="button"
              className="calc-screen__frequent-chip"
              onClick={() => {
                if (chip.resolution === undefined) {
                  selectDefender(chip.key);
                } else {
                  handleDefenderResolved(chip.resolution);
                }
              }}
            >
              {chip.nameJa}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

interface SpeciesCardProps {
  readonly regionLabel: string;
  readonly speciesSelectLabel: string;
  readonly itemSelectLabel: string;
  /** 選ばれている種族の実体(ドロップダウン・検索のどちらで選んでも、呼び出し側が解決して渡す)。 */
  readonly species: MasterSpecies | null;
  /** P4-16b(ADR-0304 A-10): 種族の一覧が使えるか。false ならドロップダウンの代わりに検索欄を出す。 */
  readonly speciesListAvailable: boolean;
  readonly speciesList: readonly MasterSpecies[];
  /** 検索口(speciesListAvailable が false のときに使う。省略は「検索できない」)。 */
  readonly masterSearch: MasterSpeciesSearch | undefined;
  /** 単独で選べる持ち物(メガストーンを除く)。 */
  readonly items: readonly Item[];
  /** メガシンカの持ち物固定(issue 515、ADR-0320)。none 以外は持ち物欄を disabled にして理由を添える。 */
  readonly itemLock: MegaItemLock;
  /** 攻守入れ替えで役割に合わず持ち物を外した通知(ADR-0326)。無ければ null。 */
  readonly droppedNotice: string | null;
  readonly selectedSpeciesKey: string;
  readonly selectedItemId: string;
  readonly onSpeciesChange: (key: string) => void;
  /** 検索で種族が解決したとき(speciesListAvailable が false のときに使う)。 */
  readonly onSpeciesResolved: (resolution: MasterSpeciesResolution) => void;
  readonly onItemChange: (id: string) => void;
  /** 特性セレクト(issue 272、ADR-0311)。選択肢が空なら出さない。 */
  readonly abilitySelect: AbilitySelectConfig;
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
  species,
  speciesListAvailable,
  speciesList,
  masterSearch,
  items,
  itemLock,
  droppedNotice,
  selectedSpeciesKey,
  selectedItemId,
  onSpeciesChange,
  onSpeciesResolved,
  onItemChange,
  abilitySelect,
  children,
  isSwapping,
  onSwapAnimationEnd,
  activeHoloClearRef,
}: SpeciesCardProps) {
  const primaryType = species?.types[0];
  const holo = useHoloCard(activeHoloClearRef);
  const className = ["calc-card", isSwapping ? "is-swapping" : "", holo.isHolo ? "is-holo" : ""]
    .filter((part) => part !== "")
    .join(" ");
  // 領域(カード)の見える見出し(h2)。accessible name はこの見出しの文字から作る(SC 2.5.3)。
  // 2枚のカードを並べて出すので、id は useId() で発行し固定文字列にしない。
  const regionHeadingId = useId();
  const speciesSelectId = useId();
  const itemSelectId = useId();
  const itemReasonId = useId();
  const droppedNoticeId = useId();
  const itemDescribedBy =
    itemLock.kind === "none" ? (droppedNotice === null ? undefined : droppedNoticeId) : itemReasonId;
  return (
    <section
      className={className}
      aria-labelledby={regionHeadingId}
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
      <h2 id={regionHeadingId} className="calc-card__region">
        {regionLabel}
      </h2>
      {speciesListAvailable ? (
        <>
          <label className="calc-card__label" htmlFor={speciesSelectId}>
            {calcScreenText.pokemonFieldLabel}
          </label>
          <select
            id={speciesSelectId}
            aria-label={speciesSelectLabel}
            value={selectedSpeciesKey}
            onChange={(event) => {
              onSpeciesChange(event.target.value);
            }}
          >
            <option value="" hidden>
              {calcScreenText.speciesPlaceholderOption}
            </option>
            {speciesList.map((candidate) => (
              <option key={candidate.key} value={candidate.key}>
                {candidate.nameJa}
              </option>
            ))}
          </select>
        </>
      ) : (
        <SpeciesSearchField
          label={speciesSelectLabel}
          masterSearch={masterSearch}
          onResolved={onSpeciesResolved}
          selectedNameJa={species?.nameJa ?? null}
        />
      )}
      {/* P4-16b(ADR-0304 A-10): 検索中(まだ種族が解決していない)は持ち物欄も出さない
          (attackerCard 内の role="option" は種族の検索候補だけにする。「入力前は候補を出さない」)。 */}
      {(speciesListAvailable || species !== null) && (
        <>
          <label className="calc-card__label" htmlFor={itemSelectId}>
            {calcScreenText.itemFieldLabel}
          </label>
          <select
            id={itemSelectId}
            aria-label={itemSelectLabel}
            value={selectedItemId}
            disabled={itemLock.kind !== "none"}
            aria-describedby={itemDescribedBy}
            onChange={(event) => {
              onItemChange(event.target.value);
            }}
          >
            <option value="">{calcScreenText.noItemOption}</option>
            {itemLock.kind === "locked" && species !== null ? (
              <option value={itemLock.item.id}>{megaStoneLabel(species, itemLock.item.nameJa)}</option>
            ) : (
              items.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.nameJa}
                </option>
              ))
            )}
          </select>
          <MegaItemReason id={itemReasonId} lock={itemLock} className="calc-card__reason" />
          {droppedNotice !== null && (
            <p id={droppedNoticeId} role="status" className="calc-card__reason">
              {droppedNotice}
            </p>
          )}
          <AbilitySelect labelClassName="calc-card__label" {...abilitySelect} />
        </>
      )}
      {species !== null && primaryType !== undefined && (
        <div className="calc-card__info">
          <PokemonImage
            speciesKey={species.key}
            size="thumb"
            className="calc-card__image"
            fallback={
              <span
                className="calc-card__emblem"
                data-testid="type-emblem"
                style={{ backgroundColor: `var(--type-${primaryType})` }}
              />
            }
          />
          <h3 className="calc-card__name">{species.nameJa}</h3>
          <ul className="calc-card__types">
            {species.types.map((type) => (
              <li
                key={type}
                className="calc-card__type"
                style={{
                  backgroundColor: `var(--type-${type}, var(--border-hairline))`,
                  color: `var(--type-${type}-ink, var(--text-primary))`,
                }}
              >
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

interface AttackerStatBlocksProps {
  readonly inputs: AttackerStatInputs;
  /** 選択中の技の分類が使うブロック(強調する)。技が未選択なら null。 */
  readonly usedStat: AttackStat | null;
  readonly issues: readonly AttackerStatIssue[];
  readonly onChange: (next: AttackerStatInputs) => void;
}

/**
 * 「攻撃」「特攻」の2ブロック(ADR-0329 §1)。常に両方出し、選んだ技が使う方の見出しに「(この技で使用)」と
 * aria-current を付ける(色だけに頼らない)。値は親が持ち、ここでは変えた1ブロック分を返すだけ。
 */
function AttackerStatBlocks({ inputs, usedStat, issues, onChange }: AttackerStatBlocksProps) {
  const spInvalid = (stat: AttackStat) => issues.some((issue) => issue.kind === "sp" && issue.stat === stat);
  const natureInvalid = issues.some((issue) => issue.kind === "nature");
  return (
    <div className="calc-attack-stats">
      {ATTACK_STATS.map((stat) => (
        <AttackerStatBlock
          key={stat}
          stat={stat}
          inputs={inputs}
          used={usedStat === stat}
          spInvalid={spInvalid(stat)}
          onChange={(next) => {
            onChange({ ...inputs, [stat]: next });
          }}
        />
      ))}
      {natureInvalid && (
        <p role="alert" className="calc-screen__error">
          {attackerStatText.natureUnresolved}
        </p>
      )}
    </div>
  );
}

interface AttackerStatBlockProps {
  readonly stat: AttackStat;
  readonly inputs: AttackerStatInputs;
  readonly used: boolean;
  readonly spInvalid: boolean;
  readonly onChange: (next: AttackStatInput) => void;
}

function AttackerStatBlock({ stat, inputs, used, spInvalid, onChange }: AttackerStatBlockProps) {
  // ラジオの name・説明の id は画面内で一意にする(同じ部品を複数置いてもグループが混ざらないように)。
  const presetName = useId();
  const natureName = useId();
  const reasonId = useId();
  const errorId = useId();
  const input = inputs[stat];
  const name = attackerStatText.statName[stat];
  const selectedPreset = matchingPreset(input);
  const presetCategory = stat === "spa" ? "special" : "physical";
  const presetSelectable = (key: (typeof ATTACKER_PRESET_KEYS)[number]) =>
    isModifierSelectable(inputs, stat, presetInput(key).modifier);
  const anyDisabled =
    ATTACKER_PRESET_KEYS.some((key) => !presetSelectable(key)) ||
    NATURE_MODIFIERS.some((modifier) => !isModifierSelectable(inputs, stat, modifier));
  return (
    <fieldset
      className={`calc-attack-stat${used ? " calc-attack-stat--used" : ""}`}
      aria-current={used ? "true" : undefined}
    >
      <legend className="calc-attack-stat__legend">
        {used ? `${name}${attackerStatText.usedSuffix}` : name}
      </legend>
      <div
        role="radiogroup"
        aria-label={attackerStatText.presetGroupLabel(name)}
        aria-describedby={anyDisabled ? reasonId : undefined}
        className="calc-preset"
      >
        {ATTACKER_PRESET_KEYS.map((key) => {
          const selected = key === selectedPreset;
          return (
            <label
              key={key}
              className={`calc-preset__option${selected ? " calc-preset__option--selected" : ""}`}
            >
              <input
                type="radio"
                name={presetName}
                className="calc-preset__input"
                checked={selected}
                disabled={!presetSelectable(key)}
                onChange={() => {
                  onChange(presetInput(key));
                }}
              />
              {attackerPresetLabel(key, presetCategory)}
            </label>
          );
        })}
        {selectedPreset === null && (
          <span className="calc-attack-stat__custom">{attackerStatText.custom}</span>
        )}
      </div>
      <label className="calc-attack-stat__sp">
        {attackerStatText.spLabel(name)}
        <input
          type="text"
          inputMode="numeric"
          className="calc-attack-stat__sp-input"
          aria-invalid={spInvalid ? "true" : undefined}
          aria-describedby={spInvalid ? errorId : undefined}
          value={input.spText}
          onChange={(event) => {
            onChange({ ...input, spText: event.target.value });
          }}
        />
      </label>
      {spInvalid && (
        <p id={errorId} role="alert" className="calc-screen__error">
          {attackerStatText.spInvalid(name)}
        </p>
      )}
      <div
        role="radiogroup"
        aria-label={attackerStatText.natureGroupLabel(name)}
        aria-describedby={anyDisabled ? reasonId : undefined}
        className="calc-preset"
      >
        {NATURE_MODIFIERS.map((modifier) => (
          <NatureModifierOption
            key={modifier}
            name={natureName}
            modifier={modifier}
            checked={input.modifier === modifier}
            disabled={!isModifierSelectable(inputs, stat, modifier)}
            onSelect={() => {
              onChange({ ...input, modifier });
            }}
          />
        ))}
      </div>
      {anyDisabled && (
        <p id={reasonId} className="calc-screen__notice">
          {attackerStatText.sameDirectionReason}
        </p>
      )}
    </fieldset>
  );
}

interface NatureModifierOptionProps {
  readonly name: string;
  readonly modifier: NatureModifier;
  readonly checked: boolean;
  readonly disabled: boolean;
  readonly onSelect: () => void;
}

/** 性格補正のピル(上昇・補正なし・下降)。 */
function NatureModifierOption({ name, modifier, checked, disabled, onSelect }: NatureModifierOptionProps) {
  return (
    <label className={`calc-preset__option${checked ? " calc-preset__option--selected" : ""}`}>
      <input
        type="radio"
        name={name}
        className="calc-preset__input"
        checked={checked}
        disabled={disabled}
        onChange={onSelect}
      />
      {attackerStatText.modifierLabel[modifier]}
    </label>
  );
}

interface MoveSelectProps {
  readonly moves: readonly Move[];
  readonly value: string;
  readonly onChange: (moveId: string) => void;
  /**
   * P4-16b/P4-17(ADR-0304 A-5・A-13): 技の候補が1件も無いとき、欄は残すが disabled にする
   * (capabilities.moves 単体ではなく、呼び出し側が「いま攻撃側の技の候補があるか」で決めた値を渡す)。
   */
  readonly disabled?: boolean;
}

/** 技セレクタ。learnset の順のまま、分類と威力(変化技は威力を出さない)を併記する。 */
function MoveSelect({ moves, value, onChange, disabled = false }: MoveSelectProps) {
  const moveSelectId = useId();
  return (
    <>
      <label className="calc-screen__label" htmlFor={moveSelectId}>
        {calcScreenText.moveLabel}
      </label>
      <select
        id={moveSelectId}
        className="calc-screen__move"
        aria-label={calcScreenText.moveLabel}
        value={value}
        disabled={disabled}
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
    </>
  );
}

interface ResultsSectionProps {
  readonly outcome: Outcome;
  readonly items: readonly Item[];
  /** 「未対応」の印(ADR-0123)の ID を表示名に解決するためのマスタ。 */
  readonly moves: readonly Move[];
  readonly abilities: readonly Ability[];
  /** ダメージバーの色に使う、選ばれている技のタイプ(design.md: バーは技のタイプ色)。 */
  readonly moveType: string | undefined;
  /** 防御側の種族の特性が2つ以上か(行に特性名を出すかの判定。issue 272)。 */
  readonly defenderHasAbilityChoice: boolean;
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
  moves,
  abilities,
  moveType,
  defenderHasAbilityChoice,
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
          moves={moves}
          abilities={abilities}
          moveType={moveType}
          defenderHasAbilityChoice={defenderHasAbilityChoice}
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
  readonly moves: readonly Move[];
  readonly abilities: readonly Ability[];
  readonly moveType: string | undefined;
  readonly defenderHasAbilityChoice: boolean;
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
function ResultsList({
  result,
  items,
  moves,
  abilities,
  moveType,
  defenderHasAbilityChoice,
  pulsingKeys,
  onKoAnimationEnd,
}: ResultsListProps) {
  const firstRow = result.rows[0];
  const barColor =
    moveType === undefined || moveType === ""
      ? "var(--text-secondary)"
      : `var(--type-${moveType}, var(--text-secondary))`;
  // issue 271 / issue 270(ADR-0123。iOS レーンの決定 DECISIONS.md 2026-09-25「未対応の印の表示」に揃える):
  // 全行に共通する印は結果の先頭に1回、残りはその行だけに出す(technicalな target で決め打ちせず、
  // 印の内容〈target・reason・id〉が全行にあるかで判定する。ADR-0300 §8: TS 側で数値・判定を加工しない、
  // ここは「どこに出すか」の割り振りだけを行う)。
  const { common: commonMarks, perRow: perRowMarks } = splitUnsupportedMarks(
    result.rows.map((row) => row.result.unsupported),
  );
  const commonMarkLabels = unsupportedMarkLabels(commonMarks, moves, items, abilities);
  return (
    <div className="calc-results">
      {commonMarkLabels.length > 0 && (
        <p role="status" className="calc-results__unsupported-notice">
          <span aria-hidden="true" data-testid="unsupported-icon" className="calc-results__unsupported-icon">
            ⚠
          </span>
          <span>{unsupportedText.notice(commonMarkLabels)}</span>
        </p>
      )}
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
          const abilityLabel = abilityNamesLabel(row.abilityIds, abilities, defenderHasAbilityChoice);
          const barValue = Math.min(row.result.maxPercent, DAMAGE_BAR_MAX_PERCENT);
          const koKey = koRowKey(row);
          const koClassName = `calc-results__ko${pulsingKeys.has(koKey) ? " is-pulsing" : ""}`;
          // 全行に共通する印は先頭の案内が担うので、この行では残り(一部の行だけにある印)だけ出す。
          const rowMarkLabels = unsupportedMarkLabels(perRowMarks[index] ?? [], moves, items, abilities);
          return (
            // preset・itemId・abilityId の組は行内で一意ではない場合がある(同じ preset で持ち物違い)ため index も足す。
            <li
              key={`${row.preset}-${row.itemId}-${row.abilityId ?? ""}-${String(index)}`}
              className="calc-results__row"
            >
              <span className="calc-results__preset">{row.presetLabel}</span>
              <span className="calc-results__item">{itemLabel}</span>
              {abilityLabel !== null && <span className="calc-results__ability">{abilityLabel}</span>}
              <span className="calc-results__percent">{formatPercentRange(row.result)}</span>
              <span className={koClassName} onAnimationEnd={onKoAnimationEnd(koKey)}>
                {formatKO(row.result.ko)}
              </span>
              <div aria-hidden="true" data-testid="damage-bar" className="calc-results__bar">
                <div
                  data-testid="damage-bar-fill"
                  className="calc-results__bar-fill"
                  style={{ width: `${String(barValue)}%`, backgroundColor: barColor }}
                />
              </div>
              {rowMarkLabels.length > 0 && (
                <p className="calc-results__unsupported">
                  <span
                    aria-hidden="true"
                    data-testid="unsupported-icon"
                    className="calc-results__unsupported-icon"
                  >
                    ⚠
                  </span>
                  <span>{unsupportedText.rowLabel(rowMarkLabels)}</span>
                </p>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
