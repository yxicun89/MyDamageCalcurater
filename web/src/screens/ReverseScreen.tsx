// P4-4: 逆算画面(docs/design.md「画面: 逆算」、requirements.md「調整の推定(逆算)」、ADR-0300 §7、ADR-0010 §R)。
// engine には CalcEngine、マスタには MasterData を渡してもらう(App.tsx が CalcScreen と同じものを共有する)。
//
// 「自分」は常に既知側(engine.known)、「相手」は常に未知側(engine.unknownSpecies)。技も常に攻撃側(相手を
// 攻撃する側)の learnset から選ぶ: 与えたダメージ(side defender)では自分の learnset、受けたダメージ
// (side attacker)では相手の learnset になる(ADR-0010 §2)。
//
// 観測は行ごとに単位(%/HP)を持ち、無効な行が1つでもあれば engine を呼ばない(古い候補も出さない)。
// 空行は無視して送る観測から外す(ADR-0010 §R2)。デバウンスはしない(入力のたびに再計算する)。

import { useEffect, useId, useMemo, useRef, useState, type AnimationEvent, type ReactElement } from "react";
import {
  ATTACKER_PRESET_KEYS,
  DEFAULT_ATTACKER_PRESET,
  attackerPresetLabel,
  resolveAttackerPreset,
  type AttackerPresetKey,
} from "../domain/attackerPresets";
import { formatMoveCategory, formatPercentRange } from "../domain/format";
import { firstDamagingMove, learnsetMoves } from "../domain/moves";
import { defaultObservationUnit, parseObservation, type ObservationUnit } from "../domain/observations";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildIndividual,
  buildReverseRequest,
  defaultAbility,
} from "../domain/requests";
import { reverseItemCandidates } from "../domain/reverseItems";
import {
  formatSPRanges,
  natureClassLabel,
  reverseAssumptionNote,
  reverseGuideNames,
  reverseItemLabel,
} from "../domain/reverseLabels";
import type {
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
import { calcScreenText, reverseResultText, reverseScreenText } from "../i18n/ja";
import type { MasterData, MasterSpecies } from "../master/types";
import { prefersReducedMotion } from "../ui/motion";
import "./ReverseScreen.css";

/**
 * 「絞り込み」の演出(design.md「画面: 逆算」「観測を追加すると候補が絞られるアニメーション」)を
 * 出す観測数のしきい値。1件目の推定は絞り込みではないので対象外にする。
 */
const NARROWING_MIN_OBSERVATIONS = 2;

/** 技を選んでいないときの、自分の調整の表示用の仮の分類(A/C 表記の既定は物理と同じ)。 */
const DEFAULT_MOVE_CATEGORY: MoveCategory = "physical";

/** 逆算画面(design.md「画面: 逆算」)。engine と master は呼び出し側が注入する(ADR-0300 §2・§3)。 */
export interface ReverseScreenProps {
  readonly engine: CalcEngine;
  readonly master: MasterData;
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
  readonly observations: readonly Observation[];
  readonly result: EngineResult<ReverseResult>;
}

/** 種族が変わった後も、いま選んでいる技を引き継ぐか決める(CalcScreen.tsx の resolveMoveId と同じ考え方)。 */
function resolveMoveId(species: MasterSpecies | null, moves: readonly Move[], currentMoveId: string): string {
  if (species === null) {
    return "";
  }
  if (learnsetMoves(species, moves).some((move) => move.id === currentMoveId)) {
    return currentMoveId;
  }
  return firstDamagingMove(species, moves)?.id ?? "";
}

/** 逆算画面(design.md「画面: 逆算」、ADR-0300 §2・§7)。自分・相手・観測したダメージの側が揃うと自動で逆算する。 */
export function ReverseScreen({ engine, master }: ReverseScreenProps) {
  const [side, setSide] = useState<ReverseSide>("defender");
  const [mySpeciesKey, setMySpeciesKey] = useState("");
  const [theirsSpeciesKey, setTheirsSpeciesKey] = useState("");
  const [myItemId, setMyItemId] = useState("");
  const [moveId, setMoveId] = useState("");
  const [attackerPresetKey, setAttackerPresetKey] = useState<AttackerPresetKey>(DEFAULT_ATTACKER_PRESET);
  // 観測行の連番(newObservationRow)。0 は初期行が使う。
  const lastRowId = useRef(0);
  const nextRowId = (): number => {
    lastRowId.current += 1;
    return lastRowId.current;
  };
  const [observations, setObservations] = useState<ObservationRow[]>(() => [
    newObservationRow(0, defaultObservationUnit("defender")),
  ]);
  const [completed, setCompleted] = useState<CompletedReverse | null>(null);
  // 観測を2件以上入れて届いた結果に「絞り込み」の演出を出す(design.md「画面: 逆算」)。
  // lastCompleted は直近に判定した completed(react-hooks/set-state-in-effect を避けるため、
  // effect ではなくレンダー本体で新しい completed かどうかを比べる。CalcScreen.tsx の koPulse と同じ形)。
  const [narrowingState, setNarrowingState] = useState<{
    readonly lastCompleted: CompletedReverse | null;
    readonly narrowing: boolean;
  }>({ lastCompleted: null, narrowing: false });

  const mySpecies = useMemo(
    () => master.species.find((species) => species.key === mySpeciesKey) ?? null,
    [master.species, mySpeciesKey],
  );
  const theirsSpecies = useMemo(
    () => master.species.find((species) => species.key === theirsSpeciesKey) ?? null,
    [master.species, theirsSpeciesKey],
  );
  const myItem = useMemo(
    () => master.items.find((item) => item.id === myItemId) ?? null,
    [master.items, myItemId],
  );
  // 技は常に攻撃側(自分を攻撃側にする defender、相手を攻撃側にする attacker)の learnset から選ぶ。
  const moveSourceSpecies = side === "defender" ? mySpecies : theirsSpecies;
  const moveOptions = useMemo(
    () => (moveSourceSpecies === null ? [] : learnsetMoves(moveSourceSpecies, master.moves)),
    [moveSourceSpecies, master.moves],
  );
  const move = useMemo(
    () => moveOptions.find((candidate) => candidate.id === moveId) ?? null,
    [moveOptions, moveId],
  );

  const parsedObservations = useMemo(
    () => observations.map((row) => parseObservation(row.unit, row.text)),
    [observations],
  );
  const hasInvalidObservation = parsedObservations.some((parsed) => parsed.status === "invalid");
  const validObservations = useMemo(
    () => parsedObservations.flatMap((parsed) => (parsed.status === "valid" ? [parsed.observation] : [])),
    [parsedObservations],
  );

  function selectSide(nextSide: ReverseSide): void {
    setSide(nextSide);
    // 対象側が変わると、観測したダメージの意味(与えた/受けた)が変わるので入力をやり直す(空の1行に戻す)。
    setObservations([newObservationRow(nextRowId(), defaultObservationUnit(nextSide))]);
    const sourceSpecies = nextSide === "defender" ? mySpecies : theirsSpecies;
    setMoveId((prev) => resolveMoveId(sourceSpecies, master.moves, prev));
  }

  function selectMySpecies(key: string): void {
    setMySpeciesKey(key);
    if (side === "defender") {
      const species = master.species.find((candidate) => candidate.key === key) ?? null;
      setMoveId((prev) => resolveMoveId(species, master.moves, prev));
    }
  }

  function selectTheirsSpecies(key: string): void {
    setTheirsSpeciesKey(key);
    if (side === "attacker") {
      const species = master.species.find((candidate) => candidate.key === key) ?? null;
      setMoveId((prev) => resolveMoveId(species, master.moves, prev));
    }
  }

  function addObservation(): void {
    const row = newObservationRow(nextRowId(), defaultObservationUnit(side));
    setObservations((prev) => [...prev, row]);
  }

  function removeObservation(index: number): void {
    setObservations((prev) => prev.filter((_row, rowIndex) => rowIndex !== index));
  }

  function updateObservationText(index: number, text: string): void {
    setObservations((prev) => prev.map((row, rowIndex) => (rowIndex === index ? { ...row, text } : row)));
  }

  function updateObservationUnit(index: number, unit: ObservationUnit): void {
    setObservations((prev) => prev.map((row, rowIndex) => (rowIndex === index ? { ...row, unit } : row)));
  }

  // 自分・相手・技(ダメージ技)・有効な観測が1件以上揃ったら calcReverse を呼ぶ(ADR-0300 §2・§7)。
  // setState は応答が届いたとき(.then のコールバック)だけで行う(react-hooks/set-state-in-effect)。
  useEffect(() => {
    if (
      mySpecies === null ||
      theirsSpecies === null ||
      move === null ||
      move.category === "status" ||
      hasInvalidObservation ||
      validObservations.length === 0
    ) {
      return;
    }
    let cancelled = false;
    const { sp, nature } =
      side === "defender"
        ? resolveAttackerPreset(attackerPresetKey, move.category)
        : { sp: ZERO_SP, nature: NEUTRAL_NATURE };
    const known = buildIndividual(mySpecies, {
      sp,
      nature,
      item: myItem,
      ability: defaultAbility(mySpecies, master.abilities),
    });
    const request = buildReverseRequest({
      side,
      known,
      unknownSpecies: theirsSpecies,
      move,
      typeChart: master.typeChart,
      itemCandidates: reverseItemCandidates(side, master.items, move),
      observations: validObservations,
    });
    void engine.calcReverse(request).then((result) => {
      if (!cancelled) {
        setCompleted({
          side,
          mySpecies,
          theirsSpecies,
          move,
          myItem,
          attackerPresetKey,
          observations: validObservations,
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
    side,
    mySpecies,
    theirsSpecies,
    move,
    myItem,
    attackerPresetKey,
    hasInvalidObservation,
    validObservations,
  ]);

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

  let outcome: Outcome;
  if (mySpecies === null || theirsSpecies === null || move === null) {
    outcome = { status: "idle" };
  } else if (move.category === "status") {
    outcome = { status: "status-move" };
  } else if (hasInvalidObservation) {
    outcome = { status: "invalid" };
  } else if (validObservations.length === 0) {
    outcome = { status: "idle" };
  } else if (
    completed === null ||
    completed.side !== side ||
    completed.mySpecies !== mySpecies ||
    completed.theirsSpecies !== theirsSpecies ||
    completed.move !== move ||
    completed.myItem !== myItem ||
    completed.attackerPresetKey !== attackerPresetKey ||
    completed.observations !== validObservations
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
        <section className="reverse-card" aria-label={reverseScreenText.mySpeciesLabel}>
          <select
            aria-label={reverseScreenText.mySpeciesLabel}
            value={mySpeciesKey}
            onChange={(event) => {
              selectMySpecies(event.target.value);
            }}
          >
            <option value="" hidden />
            {master.species.map((species) => (
              <option key={species.key} value={species.key}>
                {species.nameJa}
              </option>
            ))}
          </select>
          <select
            aria-label={reverseScreenText.myItemLabel}
            value={myItemId}
            onChange={(event) => {
              setMyItemId(event.target.value);
            }}
          >
            <option value="">{calcScreenText.noItemOption}</option>
            {master.items.map((item) => (
              <option key={item.id} value={item.id}>
                {item.nameJa}
              </option>
            ))}
          </select>
          {side === "defender" && mySpecies !== null && (
            <MyPresetSelector
              category={move?.category ?? DEFAULT_MOVE_CATEGORY}
              value={attackerPresetKey}
              onChange={setAttackerPresetKey}
            />
          )}
        </section>

        <section className="reverse-card" aria-label={reverseScreenText.theirSpeciesLabel}>
          <select
            aria-label={reverseScreenText.theirSpeciesLabel}
            value={theirsSpeciesKey}
            onChange={(event) => {
              selectTheirsSpecies(event.target.value);
            }}
          >
            <option value="" hidden />
            {master.species.map((species) => (
              <option key={species.key} value={species.key}>
                {species.nameJa}
              </option>
            ))}
          </select>
        </section>
      </div>

      <MoveSelect moves={moveOptions} value={moveId} onChange={setMoveId} />

      <div className="reverse-observations">
        {observations.map((row, index) => (
          <ObservationRowView
            key={row.id}
            n={index + 1}
            row={row}
            parsed={parsedObservations[index]}
            removable={index > 0}
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
        <button type="button" onClick={addObservation}>
          {reverseScreenText.addObservationLabel}
        </button>
      </div>

      <ResultsSection
        outcome={outcome}
        items={master.items}
        narrowing={narrowing}
        onNarrowingAnimationEnd={handleNarrowingAnimationEnd}
      />
    </div>
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

interface MoveSelectProps {
  readonly moves: readonly Move[];
  readonly value: string;
  readonly onChange: (moveId: string) => void;
}

/** 技セレクタ(CalcScreen.tsx の MoveSelect と同じ表記)。learnset の順のまま出す。 */
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

interface ObservationRowViewProps {
  readonly n: number;
  readonly row: ObservationRow;
  readonly parsed: ReturnType<typeof parseObservation> | undefined;
  readonly removable: boolean;
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
  onTextChange,
  onUnitChange,
  onRemove,
}: ObservationRowViewProps) {
  const groupName = useId();
  const messageId = useId();
  const invalid = parsed?.status === "invalid";
  const message =
    row.unit === "percent" ? reverseScreenText.percentInvalidMessage : reverseScreenText.damageInvalidMessage;
  return (
    <div className="reverse-observation">
      <input
        type="text"
        inputMode="numeric"
        aria-label={reverseScreenText.observationLabel(n)}
        className="reverse-observation__input"
        value={row.text}
        aria-invalid={invalid ? "true" : undefined}
        aria-describedby={invalid ? messageId : undefined}
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
  /** 観測を2件以上入れて届いた結果の「絞り込み」演出(design.md「画面: 逆算」)。 */
  readonly narrowing: boolean;
  readonly onNarrowingAnimationEnd: (event: AnimationEvent<HTMLUListElement>) => void;
}

/** 結果の表示(ADR-0300 §8: 返ってきた値を加工せずに表示する)。invalid は観測の不正行がある間、何も出さない。 */
function ResultsSection({
  outcome,
  items,
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
  readonly narrowing: boolean;
  readonly onNarrowingAnimationEnd: (event: AnimationEvent<HTMLUListElement>) => void;
}

/** 候補一覧(ADR-0300 §8: engine の順のまま、加工せずに表示)。防御側は H32 前提の注記を添える。 */
function ReverseResultsList({ result, items, narrowing, onNarrowingAnimationEnd }: ReverseResultsListProps) {
  const assumptionNote = reverseAssumptionNote(result);
  const listClassName = `reverse-results__list${narrowing ? " is-narrowing" : ""}`;
  return (
    <div className="reverse-results">
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
          return (
            <li
              key={`${candidate.natureClass}-${candidate.itemId}-${String(index)}`}
              className="reverse-results__row"
            >
              <span className="reverse-results__nature">
                {natureClassLabel(candidate.natureClass, result.stat)}
              </span>
              <span className="reverse-results__item">{reverseItemLabel(candidate.itemId, items)}</span>
              <span className="reverse-results__ranges">{formatSPRanges(result.stat, candidate.ranges)}</span>
              {guideNames.length > 0 && (
                <span className="reverse-results__guide">{guideNames.join("・")}</span>
              )}
              {!candidate.exact && (
                <span className="reverse-results__mismatch">{reverseResultText.closeCandidateLabel}</span>
              )}
              <span className="reverse-results__percent">{formatPercentRange(candidate)}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
