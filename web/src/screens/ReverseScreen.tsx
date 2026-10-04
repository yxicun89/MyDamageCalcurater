// P4-4: 逆算画面(docs/design.md「画面: 逆算」、requirements.md「調整の推定(逆算)」、ADR-0300 §7、ADR-0010 §R)。
// engine には CalcEngine、マスタには MasterData を渡してもらう(App.tsx が CalcScreen と同じものを共有する)。
//
// 「自分」は常に既知側(engine.known)、「相手」は常に未知側(engine.unknownSpecies)。技も常に攻撃側(相手を
// 攻撃する側)の learnset から選ぶ: 与えたダメージ(side defender)では自分の learnset、受けたダメージ
// (side attacker)では相手の learnset になる(ADR-0010 §2)。
//
// 観測は行ごとに単位(%/HP)を持ち、無効な行が1つでもあれば engine を呼ばない(古い候補も出さない)。
// 空行は無視して送る観測から外す(ADR-0010 §R2)。観測の数値テキストの編集だけ 200ms の trailing debounce
// を挟み、確定した操作(選択・単位切り替え・行の追加や削除)は待たずに計算する(P4-18、issue 113、ADR-0300 §11)。

import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type AnimationEvent,
  type ReactElement,
  type ReactNode,
} from "react";
import {
  ATTACKER_PRESET_KEYS,
  DEFAULT_ATTACKER_PRESET,
  attackerPresetLabel,
  resolveAttackerPreset,
  type AttackerPresetKey,
} from "../domain/attackerPresets";
import {
  DEFAULT_DEFENDER_PRESET,
  defenderPresetForCategory,
  defenderPresetKeysFor,
  defenderPresetLabel,
  resolveDefenderPreset,
  type DefenderPresetKey,
} from "../domain/defenderPresets";
import { formatPercentRange } from "../domain/format";
import { damagingLearnsetMoves, firstDamagingMove, isStatusMove } from "../domain/moves";
import {
  canAddObservation,
  defaultObservationUnit,
  OBSERVATION_INPUT_DEBOUNCE_MS,
  parseObservation,
  type ObservationUnit,
} from "../domain/observations";
import { abilityNamesLabel } from "../domain/abilityLabels";
import { MAX_ITEM_CANDIDATES, MAX_OBSERVATIONS } from "../domain/requestLimits";
import {
  NO_ABILITY,
  buildIndividual,
  buildReverseRequest,
  defenderAbilityCandidates,
  selectableAbilities,
} from "../domain/requests";
import {
  itemIdAfterSpeciesChange,
  lockedOrChosenItem,
  megaItemLock,
  megaStoneItemIds,
  type MegaItemLock,
} from "../domain/mega";
import {
  itemAfterRoleChange,
  itemsForRole,
  itemsWithStoneLabels,
  megaStoneLabel,
  reverseMyItemRole,
} from "../domain/itemRoles";
import { reverseItemCandidates } from "../domain/reverseItems";
import {
  formatSPRanges,
  natureClassLabel,
  reverseAssumptionNote,
  reverseGuideNames,
  reverseItemLabel,
} from "../domain/reverseLabels";
import { splitUnsupportedMarks, unsupportedMarkLabels } from "../domain/unsupportedLabels";
import type {
  Ability,
  CalcEngine,
  EngineError,
  EngineResult,
  Item,
  Move,
  MoveCategory,
  Observation,
  ReverseResult,
  ReverseSide,
} from "../engine/types";
import {
  calcScreenText,
  megaItemText,
  masterOnlineText,
  requestLimitText,
  reverseResultText,
  reverseScreenText,
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
import { MegaItemReason } from "./MegaItemReason";
import { SpeciesSearchField } from "./SpeciesSearchField";
import { useSpeciesResolutions } from "./speciesResolution";
import { AbilitySelect, type AbilitySelectConfig } from "./AbilitySelect";
import { useMoveSort } from "../app/useMoveSort";
import { MoveOptions, MoveSortChips } from "./MoveSortControls";
import "./ReverseScreen.css";

/**
 * 「絞り込み」の演出(design.md「画面: 逆算」「観測を追加すると候補が絞られるアニメーション」)を
 * 出す観測数のしきい値。1件目の推定は絞り込みではないので対象外にする。
 */
const NARROWING_MIN_OBSERVATIONS = 2;

/**
 * 絞り込みの演出(design.md「画面: 逆算」: --duration-narrow 0.3秒)を、animationend が来なくても
 * (タブが裏にある等)必ず終わらせるまでの最大待ち時間。CSS の --duration-narrow(styles/tokens.css)を
 * 上回る値にする(animationend が実際に来るまでの余裕。CSS の秒数そのものを TS に複製しない。P4-9)。
 */
const NARROWING_ANIMATION_MAX_WAIT_MS = 1000;

/** 技を選んでいないときの、自分の調整の表示用の仮の分類(A/C 表記の既定は物理と同じ)。 */
const DEFAULT_MOVE_CATEGORY: MoveCategory = "physical";

/** 逆算画面(design.md「画面: 逆算」)。engine と master は呼び出し側が注入する(ADR-0300 §2・§3)。 */
export interface ReverseScreenProps {
  readonly engine: CalcEngine;
  readonly master: MasterData;
  /**
   * P4-16b(ADR-0304 A-10): 種族を都度引く口。`master.capabilities.speciesList` が false のとき、
   * 自分・相手のポケモンの選択をドロップダウンから検索欄に替えるために使う。省略は「検索できない」。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/**
 * 観測1行の画面の状態(text は入力文字列のまま持ち、送信時に parseObservation で検証する)。
 * id は行の並び替え・削除をまたいで安定した React の key に使う(行番号 n は表示用の1始まりの連番で、
 * 削除で詰まって変わるため key には使えない)。
 */
interface ObservationRow {
  readonly id: string;
  readonly text: string;
  readonly unit: ObservationUnit;
}

/**
 * 観測1行の初期値。id は行の追加・削除の識別(React の key)だけに使い、送信には使わない。
 * crypto.randomUUID は安全なコンテキスト(HTTPS・localhost)でしか使えず、LAN の HTTP で開くと落ちるため、
 * 画面ごとの連番を使う。
 */
function newObservationRow(id: number, unit: ObservationUnit): ObservationRow {
  return { id: String(id), text: "", unit };
}

/** 計算の状態(判別 union)。invalid は観測に不正な行がある間(古い候補を出さない)。 */
type Outcome =
  | { readonly status: "idle" }
  | { readonly status: "invalid" }
  | { readonly status: "loading" }
  | { readonly status: "status-move" }
  | { readonly status: "success"; readonly result: ReverseResult }
  | { readonly status: "error"; readonly error: EngineError };

/**
 * 直近に届いた calcReverse の応答と、それを生んだ入力(CalcScreen.tsx の CompletedCalc と同じ考え方。
 * この比較1つで、古い入力に対する応答を出さないことと、入力を変えた直後に古い候補を出さないことの
 * 両方を満たす)。observations は検証済みの配列の参照そのもので比較する(値ではなく参照)。
 */
interface CompletedReverse {
  readonly side: ReverseSide;
  readonly mySpecies: MasterSpecies;
  readonly theirsSpecies: MasterSpecies;
  readonly move: Move;
  readonly myItem: Item | null;
  readonly attackerPresetKey: AttackerPresetKey;
  readonly defenderPresetKey: DefenderPresetKey;
  readonly myAbility: Ability;
  readonly unknownAbilities: readonly Ability[];
  readonly observations: readonly Observation[];
  readonly result: EngineResult<ReverseResult>;
}

/** 種族が変わった後も、いま選んでいる技を引き継ぐか決める(CalcScreen.tsx の resolveMoveId と同じ考え方)。 */
function resolveMoveId(species: MasterSpecies | null, moves: readonly Move[], currentMoveId: string): string {
  if (species === null) {
    return "";
  }
  if (damagingLearnsetMoves(species, moves).some((move) => move.id === currentMoveId)) {
    return currentMoveId;
  }
  return firstDamagingMove(species, moves)?.id ?? "";
}

/** 逆算画面(design.md「画面: 逆算」、ADR-0300 §2・§7)。自分・相手・観測したダメージの側が揃うと自動で逆算する。 */
export function ReverseScreen({ engine, master, masterSearch }: ReverseScreenProps) {
  // P4-16b(ADR-0304 A-2・A-5・A-10・A-11): 使える機能。capabilities を省いたマスタは全部使える。
  const capabilities = masterCapabilities(master);
  const {
    speciesFor,
    abilitiesFor,
    movesFor,
    resolvedSpecies,
    resolvedAbilities,
    resolvedMoves,
    register: registerSpeciesResolution,
  } = useSpeciesResolutions();
  // P4-19(issue 110): 観測の上限に達した理由(role="status")の id。ボタンの aria-describedby から指す。
  const observationLimitReasonId = useId();
  const [side, setSide] = useState<ReverseSide>("defender");
  const [mySpeciesKey, setMySpeciesKey] = useState("");
  const [theirsSpeciesKey, setTheirsSpeciesKey] = useState("");
  const [myItemId, setMyItemId] = useState("");
  // ADR-0326: 観測した側の切り替えで役割に合わず外した自分の持ち物の名前(通知用)。選び直し・種族の変更で消す。
  const [myDroppedName, setMyDroppedName] = useState<string | null>(null);
  const [moveId, setMoveId] = useState("");
  // 特性の選択(issue 272、ADR-0311)。"" は自分では「種族の先頭」、相手では「おまかせ(種族の全特性)」。
  // 種族を変えたら "" に戻す(古い選択を引き継がない)。
  const [myAbilityId, setMyAbilityId] = useState("");
  const [theirsAbilityId, setTheirsAbilityId] = useState("");
  const [attackerPresetKey, setAttackerPresetKey] = useState<AttackerPresetKey>(DEFAULT_ATTACKER_PRESET);
  const [defenderPresetKey, setDefenderPresetKey] = useState<DefenderPresetKey>(DEFAULT_DEFENDER_PRESET);
  // 観測行の連番(newObservationRow)。0 は初期行が使う。
  const lastRowId = useRef(0);
  const nextRowId = (): number => {
    lastRowId.current += 1;
    return lastRowId.current;
  };
  const [observations, setObservations] = useState<ObservationRow[]>(() => [
    newObservationRow(0, defaultObservationUnit("defender")),
  ]);
  // P4-18(issue 113、ADR-0300 §11): 計算のトリガーに使う「計算用の鏡」。observations(即時側。表示・検証用)
  // とは別に持ち、テキスト編集(debounced)だけ OBSERVATION_INPUT_DEBOUNCE_MS 遅れて追いつく。参照が
  // observations と同じ間は「待機中でない」(debouncePending の判定に使う)。確定操作(immediate)は
  // 両方を同時に更新するので、常に同じ参照になる。
  const [requestRows, setRequestRows] = useState<ObservationRow[]>(observations);
  const observationDebounceTimerRef = useRef<number | null>(null);
  // 最新の観測。種族の解決(検索の応答。非同期)が届いた時点の flushObservationDebounce は、検索を始めたときの
  // 古いレンダーのクロージャで動くので、state の observations ではなくこの ref から最新を読む(さもないと、
  // 解決を待つ間に打った観測が requestRows に反映されず「計算中」のまま止まる。ADR-0313)。
  const latestObservationsRef = useRef<ObservationRow[]>(observations);
  const [completed, setCompleted] = useState<CompletedReverse | null>(null);
  // 観測を2件以上入れて届いた結果に「絞り込み」の演出を出す(design.md「画面: 逆算」)。
  // lastCompleted は直近に判定した completed(react-hooks/set-state-in-effect を避けるため、
  // effect ではなくレンダー本体で新しい completed かどうかを比べる。CalcScreen.tsx の koPulse と同じ形)。
  const [narrowingState, setNarrowingState] = useState<{
    readonly lastCompleted: CompletedReverse | null;
    readonly narrowing: boolean;
  }>({ lastCompleted: null, narrowing: false });

  const mySpecies = useMemo(
    () => speciesFor(master.species, mySpeciesKey),
    [master.species, mySpeciesKey, speciesFor],
  );
  const theirsSpecies = useMemo(
    () => speciesFor(master.species, theirsSpeciesKey),
    [master.species, theirsSpeciesKey, speciesFor],
  );
  // issue 515(ADR-0320): メガ種族の持ち物はメガストーンに固定する(毎レンダー種族から導く)。
  // メガストーンは単独の選択肢・持ち物候補に出さない(判別集合は、全件の一覧 + 検索で解決した種族から導く)。
  const stoneIds = useMemo(
    () => megaStoneItemIds([...master.species, ...resolvedSpecies]),
    [master.species, resolvedSpecies],
  );
  // ADR-0326: 自分の欄は観測した側の反対の役割、相手の候補は相手の側の役割(side)の持ち物だけ。固定は全件から引く。
  const myPickable = useMemo(
    () => itemsForRole(master.items, reverseMyItemRole(side), stoneIds),
    [master.items, side, stoneIds],
  );
  const theirsPickable = useMemo(
    () => itemsForRole(master.items, side, stoneIds),
    [master.items, side, stoneIds],
  );
  // 結果の候補の行・未対応の印は持ち物を ID から引く。メガストーンの英語名を出さない(ADR-0326 §4)。
  // 未対応の印の名前引き用。オンラインのマスタは moves・abilities が空なので、解決済みの分を足す。
  const markMoves = useMemo(() => [...master.moves, ...resolvedMoves], [master.moves, resolvedMoves]);
  const markAbilities = useMemo(
    () => [...master.abilities, ...resolvedAbilities],
    [master.abilities, resolvedAbilities],
  );
  const displayItems = useMemo(
    () => itemsWithStoneLabels(master.items, [mySpecies, theirsSpecies], stoneIds),
    [master.items, mySpecies, theirsSpecies, stoneIds],
  );
  const myLock = useMemo(() => megaItemLock(mySpecies, master.items), [mySpecies, master.items]);
  const theirsLock = useMemo(() => megaItemLock(theirsSpecies, master.items), [theirsSpecies, master.items]);
  const myItem = useMemo(
    () => lockedOrChosenItem(myLock, myPickable, myItemId),
    [myLock, myPickable, myItemId],
  );
  // 特性の選択肢と、calcReverse に渡す特性(issue 272、ADR-0311)。
  const myAbilityOptions = useMemo(
    () =>
      mySpecies === null ? [] : selectableAbilities(mySpecies, abilitiesFor(master.abilities, mySpeciesKey)),
    [mySpecies, master.abilities, mySpeciesKey, abilitiesFor],
  );
  const myAbility = useMemo(
    () => myAbilityOptions.find((ability) => ability.id === myAbilityId) ?? myAbilityOptions[0] ?? NO_ABILITY,
    [myAbilityOptions, myAbilityId],
  );
  const theirsAbilityOptions = useMemo(
    () =>
      theirsSpecies === null
        ? []
        : selectableAbilities(theirsSpecies, abilitiesFor(master.abilities, theirsSpeciesKey)),
    [theirsSpecies, master.abilities, theirsSpeciesKey, abilitiesFor],
  );
  const unknownAbilities = useMemo(
    () =>
      theirsSpecies === null
        ? []
        : defenderAbilityCandidates(
            theirsSpecies,
            theirsAbilityOptions,
            theirsAbilityId === "" ? null : theirsAbilityId,
          ),
    [theirsSpecies, theirsAbilityOptions, theirsAbilityId],
  );
  // 技は常に攻撃側(自分を攻撃側にする defender、相手を攻撃側にする attacker)の learnset から選ぶ。
  const moveSourceSpecies = side === "defender" ? mySpecies : theirsSpecies;
  const moveSourceKey = side === "defender" ? mySpeciesKey : theirsSpeciesKey;
  const moveOptions = useMemo(
    () =>
      moveSourceSpecies === null
        ? []
        : damagingLearnsetMoves(moveSourceSpecies, movesFor(master.moves, moveSourceKey)),
    [moveSourceSpecies, master.moves, moveSourceKey, movesFor],
  );
  // P4-17(ADR-0304 A-13): 技セレクトが使えるのは capabilities.moves が true、または今の攻撃側の技の
  // 候補が1件以上あるとき。案内の表示条件もこれと同じにする(CalcScreen.tsx と同じ考え方)。
  const movesAvailable = capabilities.moves || moveOptions.length > 0;
  const move = useMemo(
    () => moveOptions.find((candidate) => candidate.id === moveId) ?? null,
    [moveOptions, moveId],
  );
  // issue 275: 技を選んでいないときの自分の調整の表示用の仮の分類は DEFAULT_MOVE_CATEGORY(物理)。
  const presetCategory = move?.category ?? DEFAULT_MOVE_CATEGORY;
  // 技の分類が変わったとき、自分の耐久(防御側プリセット)を対になるプリセットへ自動で読み替える
  // (issue 275。選択肢が入れ替わってもラジオグループに必ず1つ checked が残るようにするため)。
  const effectiveDefenderPresetKey = defenderPresetForCategory(defenderPresetKey, presetCategory);
  // P4-19(issue 110、ADR-0208): 持ち物候補(itemCandidates)を組み立て、上限で絞り込んだかを画面に出す。
  // useEffect の依存に truncated を含む新しい配列を毎回作らないよう、ここで useMemo にする
  // (react-hooks/set-state-in-effect の無限ループを避ける)。
  // 相手がメガ種族のときは探索せず、メガストーン1件(引けなければ [null])に固定する。
  const itemCandidatesResult = useMemo(() => {
    if (theirsLock.kind === "locked") {
      return { candidates: [theirsLock.item], truncated: false };
    }
    if (theirsLock.kind === "missing" || move === null) {
      return { candidates: [null], truncated: false };
    }
    return reverseItemCandidates(side, theirsPickable, move);
  }, [side, theirsPickable, move, theirsLock]);

  const parsedObservations = useMemo(
    () => observations.map((row) => parseObservation(row.unit, row.text)),
    [observations],
  );
  const hasInvalidObservation = parsedObservations.some((parsed) => parsed.status === "invalid");
  const validObservations = useMemo(
    () => parsedObservations.flatMap((parsed) => (parsed.status === "valid" ? [parsed.observation] : [])),
    [parsedObservations],
  );

  // P4-18(issue 113): calcReverse を呼ぶための検証(requestRows 由来)。表示・aria-invalid の検証
  // (上の parsedObservations/hasInvalidObservation/validObservations)とは別に持つ: 打っている途中の
  // 値は表示にはすぐ反映するが、計算はデバウンス後の requestRows が追いつくまで始めない。
  const requestParsedObservations = useMemo(
    () => requestRows.map((row) => parseObservation(row.unit, row.text)),
    [requestRows],
  );
  const requestHasInvalidObservation = requestParsedObservations.some(
    (parsed) => parsed.status === "invalid",
  );
  const requestValidObservations = useMemo(
    () =>
      requestParsedObservations.flatMap((parsed) => (parsed.status === "valid" ? [parsed.observation] : [])),
    [requestParsedObservations],
  );
  // observations と requestRows の参照が違う間は、待機中のデバウンスがある(打ち終わりを待っている)。
  const debouncePending = observations !== requestRows;

  /**
   * 観測(observations)を書き換える唯一の通り道(P4-18、issue 113、ADR-0300 §11)。表示(observations)は
   * 常に即座に更新する。計算のトリガー(requestRows)は timing で分ける:
   *   - "immediate"(単位切り替え・行の追加/削除・対象側切り替え): 待機中のタイマーを解除し、即座に追いつかせる。
   *   - "debounced"(テキスト編集): 待機中のタイマーを解除し直し、OBSERVATION_INPUT_DEBOUNCE_MS 後に追いつかせる。
   */
  function replaceObservations(next: ObservationRow[], timing: "debounced" | "immediate"): void {
    latestObservationsRef.current = next;
    setObservations(next);
    if (observationDebounceTimerRef.current !== null) {
      window.clearTimeout(observationDebounceTimerRef.current);
      observationDebounceTimerRef.current = null;
    }
    if (timing === "immediate") {
      setRequestRows(next);
      return;
    }
    observationDebounceTimerRef.current = window.setTimeout(() => {
      observationDebounceTimerRef.current = null;
      setRequestRows(next);
    }, OBSERVATION_INPUT_DEBOUNCE_MS);
  }

  /**
   * 観測に触らない確定操作(種族・持ち物・技・プリセットの選択)の前に呼ぶ。待機中のデバウンスがあれば、
   * タイマーを解除して requestRows を最新の observations に合わせる(取りこぼさず、この操作と一緒に
   * 1回だけ計算する)。待機中のタイマーが無ければ何もしない(requestRows はすでに observations と同じ)。
   */
  function flushObservationDebounce(): void {
    if (observationDebounceTimerRef.current === null) {
      return;
    }
    window.clearTimeout(observationDebounceTimerRef.current);
    observationDebounceTimerRef.current = null;
    setRequestRows(latestObservationsRef.current);
  }

  function selectSide(nextSide: ReverseSide): void {
    setSide(nextSide);
    // 自分の持ち物が新しい役割に合わなければ外して通知する(固定のメガストーンは外さない)。
    const nextMyItem = itemAfterRoleChange({
      items: master.items,
      role: reverseMyItemRole(nextSide),
      currentItemId: myItem?.id ?? "",
    });
    setMyItemId(nextMyItem.itemId);
    setMyDroppedName(nextMyItem.dropped?.nameJa ?? null);
    // 対象側が変わると、観測したダメージの意味(与えた/受けた)が変わるので入力をやり直す(空の1行に戻す)。
    replaceObservations([newObservationRow(nextRowId(), defaultObservationUnit(nextSide))], "immediate");
    const sourceSpecies = nextSide === "defender" ? mySpecies : theirsSpecies;
    const sourceKey = nextSide === "defender" ? mySpeciesKey : theirsSpeciesKey;
    setMoveId((prev) => resolveMoveId(sourceSpecies, movesFor(master.moves, sourceKey), prev));
  }

  function selectMySpecies(key: string): void {
    flushObservationDebounce();
    setMySpeciesKey(key);
    setMyAbilityId("");
    changeMyItem(speciesFor(master.species, key));
    if (side === "defender") {
      const species = speciesFor(master.species, key);
      setMoveId((prev) => resolveMoveId(species, movesFor(master.moves, key), prev));
    }
  }

  function selectTheirsSpecies(key: string): void {
    flushObservationDebounce();
    setTheirsSpeciesKey(key);
    setTheirsAbilityId("");
    if (side === "attacker") {
      const species = speciesFor(master.species, key);
      setMoveId((prev) => resolveMoveId(species, movesFor(master.moves, key), prev));
    }
  }

  /**
   * P4-16b/P4-17(ADR-0304 A-10・A-13): 検索で自分の種族が解決したとき。resolution.species・
   * resolution.moves をそのまま使う(register の setState は非同期なので、直後に movesFor/speciesFor で
   * 引き直すと古い覚え書きのままになる。CalcScreen.tsx の handleAttackerResolved と同じ考え方)。
   */
  function handleMineResolved(resolution: MasterSpeciesResolution): void {
    flushObservationDebounce();
    registerSpeciesResolution(resolution);
    setMySpeciesKey(resolution.species.key);
    setMyAbilityId("");
    changeMyItem(resolution.species);
    if (side === "defender") {
      // movesFor(master.moves, key) は使わない(register の setState 直後はまだ古い覚え書きのまま)。
      // movesFor が最終的に返す形(master.moves + 解決で覚えた分)をここで直接組み立てる。
      setMoveId((prev) => resolveMoveId(resolution.species, [...master.moves, ...resolution.moves], prev));
    }
  }

  /** P4-16b/P4-17(ADR-0304 A-10・A-13): 検索で相手の種族が解決したとき。 */
  function handleTheirsResolved(resolution: MasterSpeciesResolution): void {
    flushObservationDebounce();
    registerSpeciesResolution(resolution);
    setTheirsSpeciesKey(resolution.species.key);
    setTheirsAbilityId("");
    if (side === "attacker") {
      setMoveId((prev) => resolveMoveId(resolution.species, [...master.moves, ...resolution.moves], prev));
    }
  }

  /** 自分の種族を変えたときの持ち物(メガは固定、メガから非メガへは未選択に戻す。issue 515)。 */
  function changeMyItem(next: MasterSpecies | null): void {
    setMyDroppedName(null);
    setMyItemId(
      itemIdAfterSpeciesChange({
        previous: mySpecies,
        next,
        items: master.items,
        currentItemId: myItem?.id ?? "",
      }),
    );
  }

  /** 自分の持ち物を選ぶ(確定操作。issue 113)。 */
  function selectMyItem(itemId: string): void {
    flushObservationDebounce();
    setMyItemId(itemId);
    setMyDroppedName(null);
  }

  /** 自分の特性を選ぶ(確定操作。issue 113、272)。 */
  function selectMyAbility(abilityId: string): void {
    flushObservationDebounce();
    setMyAbilityId(abilityId);
  }

  /** 相手の特性を選ぶ(確定操作。issue 113、272)。 */
  function selectTheirsAbility(abilityId: string): void {
    flushObservationDebounce();
    setTheirsAbilityId(abilityId);
  }

  /** 技を選ぶ(確定操作。issue 113)。 */
  function selectMove(nextMoveId: string): void {
    flushObservationDebounce();
    setMoveId(nextMoveId);
  }

  /** 自分の調整プリセットを選ぶ(確定操作。issue 113)。 */
  function selectAttackerPreset(key: AttackerPresetKey): void {
    flushObservationDebounce();
    setAttackerPresetKey(key);
  }

  /** 自分の耐久(防御側プリセット)を選ぶ(確定操作。issue 113、issue 275)。 */
  function selectDefenderPreset(key: DefenderPresetKey): void {
    flushObservationDebounce();
    setDefenderPresetKey(key);
  }

  function addObservation(): void {
    if (!canAddObservation(observations.length)) {
      return;
    }
    const row = newObservationRow(nextRowId(), defaultObservationUnit(side));
    replaceObservations([...observations, row], "immediate");
  }

  function removeObservation(index: number): void {
    replaceObservations(
      observations.filter((_row, rowIndex) => rowIndex !== index),
      "immediate",
    );
  }

  function updateObservationText(index: number, text: string): void {
    replaceObservations(
      observations.map((row, rowIndex) => (rowIndex === index ? { ...row, text } : row)),
      "debounced",
    );
  }

  function updateObservationUnit(index: number, unit: ObservationUnit): void {
    replaceObservations(
      observations.map((row, rowIndex) => (rowIndex === index ? { ...row, unit } : row)),
      "immediate",
    );
  }

  // 自分・相手・技(ダメージ技)・有効な観測が1件以上揃ったら calcReverse を呼ぶ(ADR-0300 §2・§7)。
  // setState は応答が届いたとき(.then のコールバック)だけで行う(react-hooks/set-state-in-effect)。
  // トリガーは requestRows 由来の値(requestHasInvalidObservation・requestValidObservations)を使う
  // (P4-18、issue 113): 打っている途中の値では始めず、デバウンス後・確定操作の後に始める。
  // AbortController は effect ごとに作り、cleanup(依存が変わった・アンマウント)で abort する
  // (古い計算に「もう要らない」を伝える。ADR-0300 §11)。
  useEffect(() => {
    if (
      mySpecies === null ||
      theirsSpecies === null ||
      move === null ||
      isStatusMove(move) ||
      requestHasInvalidObservation ||
      requestValidObservations.length === 0
    ) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();
    const { sp, nature } =
      side === "defender"
        ? resolveAttackerPreset(attackerPresetKey, move.category)
        : resolveDefenderPreset(effectiveDefenderPresetKey);
    const known = buildIndividual(mySpecies, {
      sp,
      nature,
      item: myItem,
      ability: myAbility,
    });
    const request = buildReverseRequest({
      side,
      known,
      unknownSpecies: theirsSpecies,
      move,
      typeChart: master.typeChart,
      itemCandidates: itemCandidatesResult.candidates,
      observations: requestValidObservations,
      unknownAbilities,
    });
    void engine.calcReverse(request, controller.signal).then((result) => {
      if (!cancelled) {
        setCompleted({
          side,
          mySpecies,
          theirsSpecies,
          move,
          myItem,
          attackerPresetKey,
          defenderPresetKey: effectiveDefenderPresetKey,
          myAbility,
          unknownAbilities,
          observations: requestValidObservations,
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
    side,
    mySpecies,
    theirsSpecies,
    move,
    myItem,
    attackerPresetKey,
    effectiveDefenderPresetKey,
    requestHasInvalidObservation,
    requestValidObservations,
    myAbility,
    unknownAbilities,
    itemCandidatesResult,
  ]);

  // 画面が消えるときは、待機中の観測デバウンスのタイマーを片付ける(回しっぱなしにしない。
  // 進行中の計算の abort は上の useEffect の cleanup が担う。issue 113)。
  useEffect(() => {
    return () => {
      if (observationDebounceTimerRef.current !== null) {
        window.clearTimeout(observationDebounceTimerRef.current);
        observationDebounceTimerRef.current = null;
      }
    };
  }, []);

  // 「絞り込み」の演出(design.md「画面: 逆算」)。新しい成功結果が届いたとき(completed の参照が
  // 変わったとき)だけ判定する。観測が1件だけの結果や、観測を減らして1件に戻った結果では付けない。
  if (completed !== null && completed !== narrowingState.lastCompleted) {
    const shouldNarrow =
      completed.result.ok &&
      completed.observations.length >= NARROWING_MIN_OBSERVATIONS &&
      !prefersReducedMotion();
    setNarrowingState({ lastCompleted: completed, narrowing: shouldNarrow });
  }
  const narrowing = narrowingState.narrowing;

  /** 絞り込みの is-narrowing を外す(animationend。バブリングで子要素と混ざらないよう currentTarget と比べる)。 */
  function handleNarrowingAnimationEnd(event: AnimationEvent<HTMLUListElement>): void {
    if (event.target === event.currentTarget) {
      setNarrowingState((prev) => ({ ...prev, narrowing: false }));
    }
  }

  // 絞り込みの演出は animationend が来なくても(タブが裏にある等)最大待ちで必ず外す(P4-9)。
  // lastCompleted(絞り込み直すたびに新しい参照になる)を依存に含めることで、animationend の前に
  // 入れ直して絞り込み直しても前のタイマーでは外さず、新しい最大待ちで掛け直す。視差効果を減らす設定・
  // 絞り込みでない結果では narrowing が false なのでタイマーを掛けない。
  useEffect(() => {
    if (!narrowing) {
      return;
    }
    const timer = window.setTimeout(() => {
      setNarrowingState((prev) => ({ ...prev, narrowing: false }));
    }, NARROWING_ANIMATION_MAX_WAIT_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [narrowingState.lastCompleted, narrowing]);

  let outcome: Outcome;
  if (mySpecies === null || theirsSpecies === null || move === null) {
    outcome = { status: "idle" };
  } else if (isStatusMove(move)) {
    outcome = { status: "status-move" };
  } else if (hasInvalidObservation) {
    outcome = { status: "invalid" };
  } else if (validObservations.length === 0) {
    outcome = { status: "idle" };
  } else if (
    // P4-18(issue 113): デバウンス待ち(debouncePending)の間は、まだ requestRows に届いていない
    // 入力があるので「計算中」を出す(表示は待たないが、その入力に対する結果はまだ無い)。
    debouncePending ||
    completed === null ||
    completed.side !== side ||
    completed.mySpecies !== mySpecies ||
    completed.theirsSpecies !== theirsSpecies ||
    completed.move !== move ||
    completed.myItem !== myItem ||
    completed.attackerPresetKey !== attackerPresetKey ||
    completed.defenderPresetKey !== effectiveDefenderPresetKey ||
    completed.myAbility !== myAbility ||
    completed.unknownAbilities !== unknownAbilities ||
    completed.observations !== requestValidObservations
  ) {
    outcome = { status: "loading" };
  } else {
    outcome = completed.result.ok
      ? { status: "success", result: completed.result.value }
      : { status: "error", error: completed.result.error };
  }

  return (
    <div className="reverse-screen">
      <SideSelector side={side} onChange={selectSide} />

      <div className="reverse-screen__cards">
        <ReverseCard
          regionLabel={reverseScreenText.myRegionLabel}
          cardLabel={reverseScreenText.mySpeciesLabel}
          speciesSelectLabel={reverseScreenText.mySpeciesLabel}
          itemSelectLabel={reverseScreenText.myItemLabel}
          species={mySpecies}
          speciesListAvailable={capabilities.speciesList}
          speciesList={master.species}
          masterSearch={masterSearch}
          items={myPickable}
          itemLock={myLock}
          droppedNotice={
            myDroppedName === null ? null : itemRoleText.droppedNotice(myDroppedName, reverseMyItemRole(side))
          }
          selectedSpeciesKey={mySpeciesKey}
          selectedItemId={myItem?.id ?? ""}
          onSpeciesChange={selectMySpecies}
          onSpeciesResolved={handleMineResolved}
          onItemChange={selectMyItem}
          abilitySelect={{
            ariaLabel: reverseScreenText.myAbilityLabel,
            options: myAbilityOptions,
            value: myAbility.id,
            onChange: selectMyAbility,
          }}
        >
          {side === "defender" && mySpecies !== null && (
            <MyPresetSelector
              category={presetCategory}
              value={attackerPresetKey}
              onChange={selectAttackerPreset}
            />
          )}
          {side === "attacker" && mySpecies !== null && (
            <MyDefenderPresetSelector
              category={presetCategory}
              value={effectiveDefenderPresetKey}
              onChange={selectDefenderPreset}
            />
          )}
        </ReverseCard>

        <ReverseCard
          regionLabel={reverseScreenText.theirRegionLabel}
          cardLabel={reverseScreenText.theirSpeciesLabel}
          speciesSelectLabel={reverseScreenText.theirSpeciesLabel}
          itemSelectLabel={undefined}
          species={theirsSpecies}
          speciesListAvailable={capabilities.speciesList}
          speciesList={master.species}
          masterSearch={masterSearch}
          items={theirsPickable}
          itemLock={theirsLock}
          droppedNotice={null}
          selectedSpeciesKey={theirsSpeciesKey}
          selectedItemId=""
          onSpeciesChange={selectTheirsSpecies}
          onSpeciesResolved={handleTheirsResolved}
          onItemChange={undefined}
          abilitySelect={{
            ariaLabel: reverseScreenText.theirAbilityLabel,
            options: theirsAbilityOptions,
            value: theirsAbilityId,
            onChange: selectTheirsAbility,
            autoOptionLabel: calcScreenText.anyAbilityOption,
          }}
        />
      </div>

      <MoveSelect
        moves={moveOptions}
        types={master.typeChart.types}
        value={moveId}
        onChange={selectMove}
        disabled={!movesAvailable}
      />
      {!movesAvailable && <p className="reverse-screen__notice">{masterOnlineText.movesUnavailable}</p>}
      {moveSourceSpecies !== null && capabilities.moves && moveOptions.length === 0 && (
        <p className="reverse-screen__notice">{calcScreenText.noDamagingMovesNotice}</p>
      )}
      {!capabilities.effects && (
        <p className="reverse-screen__notice">{masterOnlineText.itemCandidatesUnavailable}</p>
      )}
      {itemCandidatesResult.truncated && (
        <p className="reverse-screen__notice">
          {requestLimitText.itemCandidatesTruncated(MAX_ITEM_CANDIDATES)}
        </p>
      )}

      <div className="reverse-observations">
        {observations.map((row, index) => (
          <ObservationRowView
            key={row.id}
            n={index + 1}
            row={row}
            parsed={parsedObservations[index]}
            removable={index > 0}
            side={side}
            onTextChange={(text) => {
              updateObservationText(index, text);
            }}
            onUnitChange={(unit) => {
              updateObservationUnit(index, unit);
            }}
            onRemove={() => {
              removeObservation(index);
            }}
          />
        ))}
        <button
          type="button"
          className="reverse-observations__add"
          onClick={addObservation}
          disabled={!canAddObservation(observations.length)}
          aria-describedby={canAddObservation(observations.length) ? undefined : observationLimitReasonId}
        >
          {reverseScreenText.addObservationLabel}
        </button>
        {!canAddObservation(observations.length) && (
          <p id={observationLimitReasonId} role="status" className="reverse-screen__notice">
            {requestLimitText.observationLimitReached(MAX_OBSERVATIONS)}
          </p>
        )}
      </div>

      <ResultsSection
        outcome={outcome}
        items={displayItems}
        moves={markMoves}
        abilities={markAbilities}
        theirsHasAbilityChoice={theirsAbilityOptions.length > 1}
        narrowing={narrowing}
        onNarrowingAnimationEnd={handleNarrowingAnimationEnd}
      />
    </div>
  );
}

interface ReverseCardProps {
  /** 領域(カード)の見える見出しの語(「自分」「相手」)。 */
  readonly regionLabel: string;
  /** section の accessible name(既存のまま。「自分のポケモン」「相手のポケモン」)。 */
  readonly cardLabel: string;
  readonly speciesSelectLabel: string;
  /** 持ち物欄の accessible name。undefined なら持ち物欄を出さない(相手側カード)。 */
  readonly itemSelectLabel: string | undefined;
  readonly species: MasterSpecies | null;
  readonly speciesListAvailable: boolean;
  readonly speciesList: readonly MasterSpecies[];
  readonly masterSearch: MasterSpeciesSearch | undefined;
  /** 単独で選べる持ち物(メガストーンを除く)。 */
  readonly items: readonly Item[];
  /** メガシンカの持ち物固定(issue 515、ADR-0320)。持ち物欄が無い相手側カードは理由とストーン名を文で出す。 */
  readonly itemLock: MegaItemLock;
  /** 観測した側の切り替えで役割に合わず持ち物を外した通知(ADR-0326)。無ければ null。 */
  readonly droppedNotice: string | null;
  readonly selectedSpeciesKey: string;
  readonly selectedItemId: string;
  readonly onSpeciesChange: (key: string) => void;
  readonly onSpeciesResolved: (resolution: MasterSpeciesResolution) => void;
  readonly onItemChange: ((id: string) => void) | undefined;
  /** 特性セレクト(issue 272、ADR-0311)。選択肢が空なら出さない。 */
  readonly abilitySelect: AbilitySelectConfig;
  readonly children?: ReactNode;
}

/**
 * 自分側・相手側の共通カード(issue 304)。CalcScreen.tsx の SpeciesCard と同じ考え方: section の
 * accessible name は aria-labelledby で見える h2(regionLabel)から作り、既存の accessible name
 * (cardLabel。「自分のポケモン」等)は変えない。
 */
function ReverseCard({
  regionLabel,
  cardLabel,
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
}: ReverseCardProps) {
  const speciesSelectId = useId();
  const itemSelectId = useId();
  const itemReasonId = useId();
  const droppedNoticeId = useId();
  const itemDescribedBy =
    itemLock.kind === "none" ? (droppedNotice === null ? undefined : droppedNoticeId) : itemReasonId;
  return (
    // section の accessible name は今までどおり aria-label(cardLabel、「自分のポケモン」等。変えない)。
    // h2 は見える見出し(regionLabel、「自分」「相手」)を足すためだけに置く(issue 304)。
    <section className="reverse-card" aria-label={cardLabel}>
      <h2 className="reverse-card__region">{regionLabel}</h2>
      {speciesListAvailable ? (
        <>
          <label className="reverse-card__label" htmlFor={speciesSelectId}>
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
        />
      )}
      {/* P4-16b(ADR-0304 A-10): 検索中(まだ種族が解決していない)は持ち物欄も出さない
          (「入力前は候補を出さない」: カード内の role="option" は種族の検索候補だけにする)。 */}
      {itemSelectLabel !== undefined &&
        onItemChange !== undefined &&
        (speciesListAvailable || species !== null) && (
          <>
            <label className="reverse-card__label" htmlFor={itemSelectId}>
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
            <MegaItemReason id={itemReasonId} lock={itemLock} className="reverse-card__reason" />
            {droppedNotice !== null && (
              <p id={droppedNoticeId} role="status" className="reverse-card__reason">
                {droppedNotice}
              </p>
            )}
          </>
        )}
      {itemSelectLabel === undefined && itemLock.kind !== "none" && (
        <>
          <MegaItemReason id={itemReasonId} lock={itemLock} className="reverse-card__reason" />
          {itemLock.kind === "locked" && species !== null && (
            <p className="reverse-card__reason">
              {megaItemText.fixedItemName(megaStoneLabel(species, itemLock.item.nameJa))}
            </p>
          )}
        </>
      )}
      {(speciesListAvailable || species !== null) && (
        <AbilitySelect labelClassName="reverse-card__label" {...abilitySelect} />
      )}
      {children}
    </section>
  );
}

interface SideSelectorProps {
  readonly side: ReverseSide;
  readonly onChange: (side: ReverseSide) => void;
}

/** 観測したダメージの側(与えた/受けた)のラジオグループ。 */
function SideSelector({ side, onChange }: SideSelectorProps) {
  const groupName = useId();
  return (
    <div role="radiogroup" aria-label={reverseScreenText.sideGroupLabel} className="reverse-side">
      <label className="reverse-side__option">
        <input
          type="radio"
          name={groupName}
          checked={side === "defender"}
          onChange={() => {
            onChange("defender");
          }}
        />
        {reverseScreenText.sideDefenderLabel}
      </label>
      <label className="reverse-side__option">
        <input
          type="radio"
          name={groupName}
          checked={side === "attacker"}
          onChange={() => {
            onChange("attacker");
          }}
        />
        {reverseScreenText.sideAttackerLabel}
      </label>
    </div>
  );
}

interface MyPresetSelectorProps {
  readonly category: MoveCategory;
  readonly value: AttackerPresetKey;
  readonly onChange: (key: AttackerPresetKey) => void;
}

/** 自分の調整(攻撃側プリセットを再利用。domain/attackerPresets.ts、CalcScreen.tsx の AttackerPresetSelector と同じ形)。 */
function MyPresetSelector({ category, value, onChange }: MyPresetSelectorProps) {
  const groupName = useId();
  return (
    <div role="radiogroup" aria-label={reverseScreenText.myPresetGroupLabel} className="reverse-preset">
      {ATTACKER_PRESET_KEYS.map((key) => {
        const selected = key === value;
        return (
          <label
            key={key}
            className={`reverse-preset__option${selected ? " reverse-preset__option--selected" : ""}`}
          >
            <input
              type="radio"
              name={groupName}
              className="reverse-preset__input"
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

interface MyDefenderPresetSelectorProps {
  readonly category: MoveCategory;
  readonly value: DefenderPresetKey;
  readonly onChange: (key: DefenderPresetKey) => void;
}

/**
 * 自分の耐久(防御側プリセット。domain/defenderPresets.ts、issue 275)。「受けたダメージ」で
 * 自分が防御側のときに出す。MyPresetSelector と同じ形で、選択肢は技の分類で絞る。
 */
function MyDefenderPresetSelector({ category, value, onChange }: MyDefenderPresetSelectorProps) {
  const groupName = useId();
  return (
    <div role="radiogroup" aria-label={reverseScreenText.myPresetGroupLabel} className="reverse-preset">
      {defenderPresetKeysFor(category).map((key) => {
        const selected = key === value;
        return (
          <label
            key={key}
            className={`reverse-preset__option${selected ? " reverse-preset__option--selected" : ""}`}
          >
            <input
              type="radio"
              name={groupName}
              className="reverse-preset__input"
              checked={selected}
              onChange={() => {
                onChange(key);
              }}
            />
            {defenderPresetLabel(key)}
          </label>
        );
      })}
    </div>
  );
}

interface MoveSelectProps {
  readonly moves: readonly Move[];
  /** マスタのタイプ表の並び(タイプ順の群の並び)。 */
  readonly types: readonly string[];
  readonly value: string;
  readonly onChange: (moveId: string) => void;
  /**
   * P4-16b/P4-17(ADR-0304 A-5・A-13): 技の候補が1件も無いとき、欄は残すが disabled にする
   * (capabilities.moves 単体ではなく、呼び出し側が「いま攻撃側の技の候補があるか」で決めた値を渡す)。
   */
  readonly disabled?: boolean;
}

/** 技セレクタ(CalcScreen.tsx の MoveSelect と同じ表記)。並びはチップ群で選ぶ(既定は learnset の順)。 */
function MoveSelect({ moves, types, value, onChange, disabled = false }: MoveSelectProps) {
  const moveSelectId = useId();
  const [order] = useMoveSort();
  return (
    <>
      <label className="reverse-screen__label" htmlFor={moveSelectId}>
        {calcScreenText.moveLabel}
      </label>
      <MoveSortChips variant="reverse" />
      <select
        id={moveSelectId}
        className="reverse-screen__move"
        aria-label={calcScreenText.moveLabel}
        value={value}
        disabled={disabled}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        <MoveOptions moves={moves} types={types} order={order} value={value} />
      </select>
    </>
  );
}

interface ObservationRowViewProps {
  readonly n: number;
  readonly row: ObservationRow;
  readonly parsed: ReturnType<typeof parseObservation> | undefined;
  readonly removable: boolean;
  /** 観測した側(与えた = defender / 受けた = attacker)。説明の文(observationHintLabel)に使う。 */
  readonly side: ReverseSide;
  readonly onTextChange: (text: string) => void;
  readonly onUnitChange: (unit: ObservationUnit) => void;
  readonly onRemove: () => void;
}

/** 観測1行(入力・単位のラジオ・検証メッセージ・削除ボタン)。 */
function ObservationRowView({
  n,
  row,
  parsed,
  removable,
  side,
  onTextChange,
  onUnitChange,
  onRemove,
}: ObservationRowViewProps) {
  const groupName = useId();
  const inputId = useId();
  const hintId = useId();
  const messageId = useId();
  const invalid = parsed?.status === "invalid";
  const message =
    row.unit === "percent" ? reverseScreenText.percentInvalidMessage : reverseScreenText.damageInvalidMessage;
  const hint = reverseScreenText.observationHintLabel(side, row.unit);
  return (
    <div className="reverse-observation">
      <label className="reverse-observation__label" htmlFor={inputId}>
        {reverseScreenText.observationLabel(n)}
      </label>
      <input
        id={inputId}
        type="text"
        inputMode="numeric"
        aria-label={reverseScreenText.observationLabel(n)}
        className="reverse-observation__input"
        value={row.text}
        aria-invalid={invalid ? "true" : undefined}
        aria-describedby={invalid ? `${hintId} ${messageId}` : hintId}
        onChange={(event) => {
          onTextChange(event.target.value);
        }}
      />
      <div
        role="radiogroup"
        aria-label={reverseScreenText.observationUnitGroupLabel(n)}
        className="reverse-observation__unit"
      >
        <label className="reverse-observation__unit-option">
          <input
            type="radio"
            name={groupName}
            checked={row.unit === "percent"}
            onChange={() => {
              onUnitChange("percent");
            }}
          />
          {reverseScreenText.percentUnitLabel}
        </label>
        <label className="reverse-observation__unit-option">
          <input
            type="radio"
            name={groupName}
            checked={row.unit === "damage"}
            onChange={() => {
              onUnitChange("damage");
            }}
          />
          {reverseScreenText.damageUnitLabel}
        </label>
      </div>
      {/* critic指摘(issue 304): hint は入力欄の直後ではなく単位ラジオの後に置く(aria-describedby は
          DOM順に依存しないので支援技術への影響はない)。全幅行(grid-column: 1/-1)の hint が
          input と radiogroup の間に挟まると、grid の自動配置で両者が別々の行に分かれてしまうため。 */}
      <p id={hintId} className="reverse-observation__hint">
        {hint}
      </p>
      {invalid && (
        <p id={messageId} className="reverse-observation__message">
          {message}
        </p>
      )}
      {removable && (
        <button type="button" className="reverse-observation__remove" onClick={onRemove}>
          {reverseScreenText.removeObservationLabel(n)}
        </button>
      )}
    </div>
  );
}

interface ResultsSectionProps {
  readonly outcome: Outcome;
  readonly items: readonly Item[];
  /** 「未対応」の印(ADR-0123)の ID を表示名に解決するためのマスタ。 */
  readonly moves: readonly Move[];
  readonly abilities: readonly Ability[];
  /** 相手の種族の特性が2つ以上か(候補に特性名を出すかの判定。issue 272)。 */
  readonly theirsHasAbilityChoice: boolean;
  /** 観測を2件以上入れて届いた結果の「絞り込み」演出(design.md「画面: 逆算」)。 */
  readonly narrowing: boolean;
  readonly onNarrowingAnimationEnd: (event: AnimationEvent<HTMLUListElement>) => void;
}

/** 結果の表示(ADR-0300 §8: 返ってきた値を加工せずに表示する)。invalid は観測の不正行がある間、何も出さない。 */
function ResultsSection({
  outcome,
  items,
  moves,
  abilities,
  theirsHasAbilityChoice,
  narrowing,
  onNarrowingAnimationEnd,
}: ResultsSectionProps): ReactElement | null {
  switch (outcome.status) {
    case "idle":
    case "invalid":
      return null;
    case "loading":
      return (
        <div className="reverse-results" aria-busy="true">
          <p className="reverse-screen__notice">{calcScreenText.loadingNotice}</p>
        </div>
      );
    case "status-move":
      return <p className="reverse-screen__notice">{calcScreenText.statusMoveNotice}</p>;
    case "error":
      return (
        <p role="alert" className="reverse-screen__error">
          {outcome.error.message}
        </p>
      );
    case "success":
      return (
        <ReverseResultsList
          result={outcome.result}
          items={items}
          moves={moves}
          abilities={abilities}
          theirsHasAbilityChoice={theirsHasAbilityChoice}
          narrowing={narrowing}
          onNarrowingAnimationEnd={onNarrowingAnimationEnd}
        />
      );
    default: {
      // 判別 union の網羅性チェック(コーディング規約 §4 TypeScript「判別 union は網羅性を検査する」)。
      const exhaustive: never = outcome;
      throw new Error(`未知の Outcome: ${JSON.stringify(exhaustive)}`);
    }
  }
}

interface ReverseResultsListProps {
  readonly result: ReverseResult;
  readonly items: readonly Item[];
  readonly moves: readonly Move[];
  readonly abilities: readonly Ability[];
  readonly theirsHasAbilityChoice: boolean;
  readonly narrowing: boolean;
  readonly onNarrowingAnimationEnd: (event: AnimationEvent<HTMLUListElement>) => void;
}

/** 候補一覧(ADR-0300 §8: engine の順のまま、加工せずに表示)。防御側は H32 前提の注記を添える。 */
function ReverseResultsList({
  result,
  items,
  moves,
  abilities,
  theirsHasAbilityChoice,
  narrowing,
  onNarrowingAnimationEnd,
}: ReverseResultsListProps) {
  const assumptionNote = reverseAssumptionNote(result);
  const listClassName = `reverse-results__list${narrowing ? " is-narrowing" : ""}`;
  // issue 305: 観測を厳密に説明できる候補(exact)が1件も無いとき(exactCount 0 かつ候補が1件以上)。
  // 判定は engine が返した exactCount をそのまま使う(ADR-0300 §8: TS 側で再判定しない)。
  const hasNoExactCandidate = result.exactCount === 0 && result.candidates.length > 0;
  // issue 271 / issue 270(ADR-0123。iOS レーンの決定 DECISIONS.md 2026-09-25「未対応の印の表示」に揃える):
  // 全候補に共通する印は候補一覧の先頭に1回、残りはその候補だけに出す(CalcScreen.tsx の ResultsList と同じ形)。
  const { common: commonMarks, perRow: perCandidateMarks } = splitUnsupportedMarks(
    result.candidates.map((candidate) => candidate.unsupported),
  );
  const commonMarkLabels = unsupportedMarkLabels(commonMarks, moves, items, abilities);
  return (
    <div className="reverse-results">
      {commonMarkLabels.length > 0 && (
        <p role="status" className="reverse-results__unsupported-notice">
          <span
            aria-hidden="true"
            data-testid="unsupported-icon"
            className="reverse-results__unsupported-icon"
          >
            ⚠
          </span>
          <span>{unsupportedText.notice(commonMarkLabels)}</span>
        </p>
      )}
      {hasNoExactCandidate && (
        <p role="status" className="reverse-results__no-exact-notice">
          {reverseResultText.noExactCandidateNotice}
        </p>
      )}
      {assumptionNote !== null && <p className="reverse-results__assumption">{assumptionNote}</p>}
      <ul
        aria-label={reverseScreenText.resultsListLabel}
        className={listClassName}
        onAnimationEnd={onNarrowingAnimationEnd}
      >
        {result.candidates.map((candidate, index) => {
          const guideNames = reverseGuideNames(
            result.side,
            result.stat,
            candidate.natureClass,
            candidate.ranges,
          );
          // 全候補に共通する印は先頭の案内が担うので、この候補では残り(一部の候補だけにある印)だけ出す。
          const candidateMarkLabels = unsupportedMarkLabels(
            perCandidateMarks[index] ?? [],
            moves,
            items,
            abilities,
          );
          const abilityLabel = abilityNamesLabel(candidate.abilityIds, abilities, theirsHasAbilityChoice);
          return (
            <li
              key={`${candidate.natureClass}-${candidate.itemId}-${candidate.abilityId ?? ""}-${String(index)}`}
              className="reverse-results__row"
            >
              <span className="reverse-results__nature">
                {natureClassLabel(candidate.natureClass, result.stat)}
              </span>
              <span className="reverse-results__item">{reverseItemLabel(candidate.itemId, items)}</span>
              {abilityLabel !== null && <span className="reverse-results__ability">{abilityLabel}</span>}
              <span className="reverse-results__ranges">
                {formatSPRanges(result.stat, candidate.ranges)}
                {hasNoExactCandidate && (
                  <span className="reverse-results__reference-label">
                    {reverseResultText.referenceRangeLabel}
                  </span>
                )}
              </span>
              {guideNames.length > 0 && (
                <span className="reverse-results__guide">{guideNames.join("・")}</span>
              )}
              {!candidate.exact && (
                <span className="reverse-results__mismatch">{reverseResultText.closeCandidateLabel}</span>
              )}
              <span className="reverse-results__percent">
                <span className="reverse-results__percent-label">
                  {reverseResultText.predictedPercentLabel}
                </span>
                <span className="reverse-results__percent-value">{formatPercentRange(candidate)}</span>
              </span>
              {candidateMarkLabels.length > 0 && (
                <p className="reverse-results__unsupported">
                  <span
                    aria-hidden="true"
                    data-testid="unsupported-icon"
                    className="reverse-results__unsupported-icon"
                  >
                    ⚠
                  </span>
                  <span>{unsupportedText.rowLabel(candidateMarkLabels)}</span>
                </p>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
