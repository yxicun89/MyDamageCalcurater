// CB-6 段階 B(ADR-0359、docs/plan/combo-calc.md §4-3 案 A): 計算画面の結果の下の「連続技」。
// 手が 1 つ(メインの技だけ)のときは「次の技」ボタンだけを見せ、押すと広げる。計算は calcComboVia(engine, ComboRequest)。
// 攻撃側・防御側・場・条件・ランクは計算画面の値をそのまま受け取る(別の入力欄を作らない = UC-5)。
// 返ってきた値は加工せずに表示する(ADR-0300 §8)。失敗してもこの節の中だけに案内し、単発の計算結果は壊さない(絶対ルール 5)。

import { useEffect, useId, useMemo, useRef, useState, type ReactElement } from "react";
import {
  DEFAULT_SELF_SPEED,
  MAX_COMBO_STEPS,
  applySelfSpeed,
  buildComboRequest,
  canBeOpponentMove,
  clampSelfSpeedSp,
  maxSelfSpeedSp,
  type SelfSpeedInput,
  type SelfSpeedNature,
} from "../domain/combo";
import {
  comboKoText,
  formatSurviveChance,
  rankChipsOf,
  speedExampleLines,
  speedLineText,
  type ComboDefenderOption,
  type RankChip,
} from "../domain/comboView";
import { formatPercent, formatPercentRange } from "../domain/format";
import { unsupportedMarkLabel } from "../domain/unsupportedLabels";
import {
  COMBO_UNSUPPORTED_CODE,
  REQUEST_ABORTED_CODE,
  calcComboVia,
  type Ability,
  type CalcEngine,
  type ComboOpponentSpeed,
  type ComboResult,
  type ComboSpeedField,
  type ComboStepRequest,
  type EngineResult,
  type Field,
  type Individual,
  type Item,
  type Move,
  type TypeChart,
  type UnsupportedMark,
} from "../engine/types";
import { comboScreenText, isComboMarkReason } from "../i18n/combo";
import { unsupportedText } from "../i18n/ja";
import type { SpeedClient, SpeedResult } from "../speed/speedClient";
import type { components as SpeedComponents } from "../speed/speed.gen";
import { Icon } from "../ui/Icon";
import { RankMeter } from "../ui/RankMeter";
import { MovePicker } from "./MovePicker";
import { ToggleChip } from "./ToggleChip";
import "./ComboSection.css";

/** 画面が持つ 1 手(メイン = 計算画面の技。追加の手 = 技ピッカーで選んだ技)。 */
type ComboEntry =
  | { readonly kind: "main"; readonly key: string }
  | { readonly kind: "extra"; readonly key: string; readonly moveId: string; readonly critical: boolean };

const MAIN_ENTRY: ComboEntry = { kind: "main", key: "main" };

/** 直近に届いた応答と、それを生んだ要求(この参照が今の要求と違えば古い応答として捨てる)。 */
interface CompletedCombo {
  readonly request: ReturnType<typeof buildComboRequest>;
  readonly result: EngineResult<ComboResult>;
}

export interface ComboSectionProps {
  readonly engine: CalcEngine;
  readonly typeChart: TypeChart;
  /** 技ピッカーのタイプ順の並び(マスタのタイプ表)。 */
  readonly types: readonly string[];
  /** 攻撃側(計算画面の値。ランク・やけどを含む)。揃っていなければ null。 */
  readonly attacker: Individual | null;
  /** メインの技(計算画面の技欄)。無ければ null。 */
  readonly mainMove: Move | null;
  /** メインの技の多段の回数(計算画面の指定。無ければ null)。 */
  readonly mainHits: number | null;
  /** メインの急所(計算画面の条件「急所」と同じ値)。 */
  readonly mainCritical: boolean;
  readonly onMainCriticalChange: (critical: boolean) => void;
  /** 追加の手の候補(攻撃側の覚える技のうち、攻撃技またはランクを変える変化技)。 */
  readonly extraCandidates: readonly Move[];
  /** 相手の技の候補(防御側の覚える攻撃技)。 */
  readonly opponentCandidates: readonly Move[];
  /** 相手の振りの選択肢(計算画面の一括計算の行。空なら節を出さない)。 */
  readonly defenders: readonly ComboDefenderOption[];
  /** 場(天候・フィールド・壁)。全手で同じ。 */
  readonly field: Field | undefined;
  /** 印の名前引き用のマスタ。 */
  readonly moves: readonly Move[];
  readonly items: readonly Item[];
  readonly abilities: readonly Ability[];
  /**
   * 素早さの種族の例(段階 C1)を引く speed API の口。オフライン(計算モード)では渡されず、そのときは例を出さない。
   */
  readonly speedClient?: SpeedClient;
  /**
   * 素早さ画面から「連続技を開く」要求を受けた印(段階 C2)。値が変わったとき(null → 数値を含む)に節を広げる。
   */
  readonly openToken?: number | null;
  /**
   * 素早さ画面の「今の振り」(段階 C2 の続き・I-web-32)。openToken が変わったときに自分の素早さの入力へ反映する。
   * scarf は連続技に指定する項目が無いので反映せず、案内だけ出す。
   */
  readonly openSpeed?: OpenSpeed | null;
}

/** 素早さ画面から渡る自分の素早さ(SpeedComboRequest.speed と同じ形。性格は素早さの補正)。 */
export interface OpenSpeed {
  readonly sp: number;
  readonly nature: Exclude<SelfSpeedNature, "keep">;
  readonly rank: number;
  readonly scarf: boolean;
}

/** 連続技の印 1 件の文言。連続技だけの理由は専用の語、それ以外は単発の印と同じ書式。 */
function comboMarkLabel(
  mark: UnsupportedMark,
  moves: readonly Move[],
  items: readonly Item[],
  abilities: readonly Ability[],
): string {
  if (!isComboMarkReason(mark.reason)) {
    return unsupportedMarkLabel(mark, moves, items, abilities);
  }
  const known = Object.hasOwn(unsupportedText.target, mark.target);
  const target = known
    ? unsupportedText.target[mark.target as keyof typeof unsupportedText.target]
    : unsupportedText.unknownTarget;
  const name = unsupportedMarkLabelName(mark, moves, items, abilities);
  return `${target}「${name}」(${comboScreenText.markReason[mark.reason]})`;
}

/** 印の対象の表示名(マスタに無ければ ID のまま)。 */
function unsupportedMarkLabelName(
  mark: UnsupportedMark,
  moves: readonly Move[],
  items: readonly Item[],
  abilities: readonly Ability[],
): string {
  switch (mark.target) {
    case "move":
      return moves.find((move) => move.id === mark.id)?.nameJa ?? mark.id;
    case "attacker_item":
    case "defender_item":
      return items.find((item) => item.id === mark.id)?.nameJa ?? mark.id;
    case "attacker_ability":
    case "defender_ability":
      return abilities.find((ability) => ability.id === mark.id)?.nameJa ?? mark.id;
    default:
      return mark.id;
  }
}

function RankChips({ chips }: { readonly chips: readonly RankChip[] }): ReactElement | null {
  if (chips.length === 0) {
    return null;
  }
  return (
    <span className="calc-combo__chips">
      {chips.map((chip) => (
        <span key={chip.text} className={`ui-badge calc-combo__chip calc-combo__chip--${chip.direction}`}>
          {chip.text}
        </span>
      ))}
    </span>
  );
}

export function ComboSection({
  engine,
  typeChart,
  types,
  attacker,
  mainMove,
  mainHits,
  mainCritical,
  onMainCriticalChange,
  extraCandidates,
  opponentCandidates,
  defenders,
  field,
  moves,
  items,
  abilities,
  speedClient,
  openToken = null,
  openSpeed = null,
}: ComboSectionProps): ReactElement | null {
  const headingId = useId();
  const sectionRef = useRef<HTMLElement>(null);
  // 手の削除・閉じるの後に、消えたボタンの代わりにフォーカスを移す先(段階 C3)。描画後の effect で消費する。
  const pendingFocusRef = useRef<"after-remove" | "after-close" | null>(null);
  const [handledOpenToken, setHandledOpenToken] = useState<number | null>(null);
  const speedFieldBodyId = useId();
  const nextKeyRef = useRef(1);
  const [open, setOpen] = useState(false);
  const [rawEntries, setRawEntries] = useState<readonly ComboEntry[]>([MAIN_ENTRY]);
  const [defenderKey, setDefenderKey] = useState("");
  const [opponentMoveId, setOpponentMoveId] = useState("");
  const [opponentSpeedFromDefender, setOpponentSpeedFromDefender] = useState(false);
  const [speedFieldOpen, setSpeedFieldOpen] = useState(false);
  const [speedField, setSpeedField] = useState<ComboSpeedField>({});
  const [selfSpeed, setSelfSpeed] = useState<SelfSpeedInput>(DEFAULT_SELF_SPEED);
  const [scarfIgnored, setScarfIgnored] = useState(false);
  const [completed, setCompleted] = useState<CompletedCombo | null>(null);

  // 素早さ画面からの要求(段階 C2): token が変わったときだけ広げる(レンダー中の state 調整。計算画面の復元と同じ考え方)。
  if (openToken !== null && openToken !== handledOpenToken) {
    setHandledOpenToken(openToken);
    setOpen(true);
    // speed の無い要求は入力を既定(個体の値のまま)に戻す。前の要求の値を残さない。
    setSelfSpeed(
      openSpeed === null
        ? DEFAULT_SELF_SPEED
        : { sp: openSpeed.sp, nature: openSpeed.nature, rank: openSpeed.rank },
    );
    setScarfIgnored(openSpeed?.scarf === true);
  }

  const moveById = useMemo(() => new Map(extraCandidates.map((move) => [move.id, move])), [extraCandidates]);
  // 攻撃側を変えて覚えなくなった追加の手は外れる(状態は触らず、毎レンダー導く)。
  const entries = useMemo(
    () => rawEntries.filter((entry) => entry.kind === "main" || moveById.has(entry.moveId)),
    [rawEntries, moveById],
  );
  const opponentMove = opponentCandidates.find((candidate) => candidate.id === opponentMoveId);
  const defender = defenders.find((option) => option.key === defenderKey) ?? defenders[0];

  const moveOfEntry = (entry: ComboEntry): Move | undefined =>
    entry.kind === "main" ? (mainMove ?? undefined) : moveById.get(entry.moveId);

  const steps = useMemo((): ComboStepRequest[] | null => {
    if (mainMove === null) {
      return null;
    }
    const built: ComboStepRequest[] = [];
    for (const entry of entries) {
      if (entry.kind === "main") {
        built.push({
          move: mainMove,
          ...(mainCritical ? { critical: true } : {}),
          ...(mainHits === null ? {} : { hits: mainHits }),
        });
        continue;
      }
      const move = moveById.get(entry.moveId);
      if (move !== undefined) {
        built.push({ move, ...(entry.critical ? { critical: true } : {}) });
      }
    }
    return built;
  }, [entries, mainMove, mainCritical, mainHits, moveById]);

  const shouldCalc = (steps?.length ?? 0) >= 2 || opponentMove !== undefined;
  // 自分の素早さの入力を攻撃側の個体に重ねる(既定のままなら同じ個体。I-web-32)。
  const comboAttacker = useMemo(
    () => (attacker === null ? null : applySelfSpeed(attacker, selfSpeed)),
    [attacker, selfSpeed],
  );
  const request = useMemo(() => {
    if (!shouldCalc || steps === null || comboAttacker === null || defender === undefined) {
      return null;
    }
    const opponentSpeed: ComboOpponentSpeed | undefined = opponentSpeedFromDefender
      ? "from_defender"
      : undefined;
    return buildComboRequest({
      attacker: comboAttacker,
      defender: defender.individual,
      steps,
      typeChart,
      ...(field === undefined ? {} : { field }),
      ...(opponentMove === undefined ? {} : { opponentMove }),
      ...(opponentSpeed === undefined ? {} : { opponentSpeed }),
      speedField,
    });
  }, [
    shouldCalc,
    steps,
    comboAttacker,
    defender,
    typeChart,
    field,
    opponentMove,
    opponentSpeedFromDefender,
    speedField,
  ]);

  // 要求が変わるたびに計算し直す。古い応答は cancelled で無視し、古い計算には abort を伝える(計算画面の calcBulk と同じ流儀)。
  useEffect(() => {
    if (request === null) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();
    void calcComboVia(engine, request, controller.signal).then((result) => {
      if (!cancelled) {
        setCompleted({ request, result });
      }
    });
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [engine, request]);

  // 手の削除・閉じるの後のフォーカス(段階 C3)。hooks なので早期 return より前に置く。
  useEffect(() => {
    const pending = pendingFocusRef.current;
    if (pending === null) {
      return;
    }
    pendingFocusRef.current = null;
    const root = sectionRef.current;
    if (root === null) {
      return;
    }
    const target =
      pending === "after-close"
        ? root.querySelector<HTMLElement>(".calc-combo__open")
        : (root.querySelector<HTMLElement>(".calc-combo__add button:not(:disabled)") ??
          root.querySelector<HTMLElement>(".calc-combo__step:last-child button:not(:disabled)"));
    target?.focus();
  });

  if (attacker === null || mainMove === null || defenders.length === 0) {
    return null;
  }

  const expanded = open || entries.length > 1 || opponentMove !== undefined;
  const canAdd = entries.length < MAX_COMBO_STEPS;

  function addStep(moveId: string): void {
    if (moveId === "" || !canAdd) {
      return;
    }
    const key = `extra-${String(nextKeyRef.current)}`;
    nextKeyRef.current += 1;
    setRawEntries([...entries, { kind: "extra", key, moveId, critical: false }]);
  }

  function moveEntry(index: number, delta: -1 | 1): void {
    const target = index + delta;
    if (target < 0 || target >= entries.length) {
      return;
    }
    const next = [...entries];
    const [picked] = next.splice(index, 1);
    if (picked !== undefined) {
      next.splice(target, 0, picked);
    }
    setRawEntries(next);
  }

  function removeEntry(index: number): void {
    pendingFocusRef.current = "after-remove";
    setRawEntries(entries.filter((_, i) => i !== index));
  }

  function setExtraCritical(key: string, critical: boolean): void {
    setRawEntries(
      entries.map((entry) => (entry.kind === "extra" && entry.key === key ? { ...entry, critical } : entry)),
    );
  }

  function updateSelfSpeed(patch: Partial<SelfSpeedInput>): void {
    setSelfSpeed({ ...selfSpeed, ...patch });
    setScarfIgnored(false);
  }

  function updateSpeedField(patch: Partial<ComboSpeedField>): void {
    setSpeedField({ ...speedField, ...patch });
  }

  // 選んでいた振りが無くなり、先頭に戻って計算しているとき(段階 C3)。
  const defenderWasReset = defenderKey !== "" && !defenders.some((option) => option.key === defenderKey);

  const current =
    request !== null && completed !== null && completed.request === request ? completed.result : null;

  return (
    <section ref={sectionRef} className="calc-combo" aria-labelledby={headingId}>
      <h2 id={headingId} className="calc-combo__heading">
        {comboScreenText.sectionLabel}
      </h2>
      {!expanded ? (
        <button
          type="button"
          className="ui-button ui-button--secondary calc-combo__open"
          onClick={() => {
            setOpen(true);
          }}
        >
          <Icon name="plus" size={16} />
          {comboScreenText.addStepButton}
        </button>
      ) : (
        <>
          <ul aria-label={comboScreenText.stepsListLabel} className="calc-combo__steps">
            {entries.map((entry, index) => {
              const move = moveOfEntry(entry);
              if (move === undefined) {
                return null;
              }
              const critical = entry.kind === "main" ? mainCritical : entry.critical;
              return (
                <li
                  key={entry.key}
                  aria-label={`${comboScreenText.stepOrdinal(index)} ${move.nameJa}`}
                  className="calc-combo__step"
                >
                  <div className="calc-combo__step-head">
                    <span className="calc-combo__ordinal">{comboScreenText.stepOrdinal(index)}</span>
                    <span className="calc-combo__move">{move.nameJa}</span>
                    {entry.kind === "main" && (
                      <span className="ui-badge calc-combo__main-badge">{comboScreenText.mainStepBadge}</span>
                    )}
                    <RankChips chips={rankChipsOf(move)} />
                  </div>
                  <div className="calc-combo__step-actions">
                    {move.category !== "status" && (
                      <ToggleChip
                        type="checkbox"
                        label={comboScreenText.critical(index)}
                        checked={critical}
                        onChange={(checked) => {
                          if (entry.kind === "main") {
                            onMainCriticalChange(checked);
                          } else {
                            setExtraCritical(entry.key, checked);
                          }
                        }}
                      />
                    )}
                    <button
                      type="button"
                      className="ui-button ui-button--secondary calc-combo__icon-button"
                      aria-label={comboScreenText.moveUp(move.nameJa)}
                      disabled={index === 0}
                      onClick={() => {
                        moveEntry(index, -1);
                      }}
                    >
                      <Icon name="up" size={16} />
                    </button>
                    <button
                      type="button"
                      className="ui-button ui-button--secondary calc-combo__icon-button"
                      aria-label={comboScreenText.moveDown(move.nameJa)}
                      disabled={index === entries.length - 1}
                      onClick={() => {
                        moveEntry(index, 1);
                      }}
                    >
                      <Icon name="down" size={16} />
                    </button>
                    {entry.kind === "extra" && (
                      <button
                        type="button"
                        className="ui-button ui-button--secondary calc-combo__icon-button"
                        aria-label={comboScreenText.removeStep(move.nameJa)}
                        onClick={() => {
                          removeEntry(index);
                        }}
                      >
                        <Icon name="trash" size={16} />
                      </button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>

          {canAdd ? (
            <div className="calc-combo__add">
              <MovePicker
                label={comboScreenText.addPickerLabel}
                labelClassName="calc-screen__label"
                triggerClassName="calc-screen__move"
                moves={extraCandidates}
                types={types}
                value=""
                onChange={addStep}
                showUnselected
                disabled={extraCandidates.length === 0}
              />
            </div>
          ) : (
            <p className="calc-screen__notice">{comboScreenText.maxStepsNotice(MAX_COMBO_STEPS)}</p>
          )}
          {entries.length === 1 && opponentMove === undefined && (
            <button
              type="button"
              className="ui-button ui-button--secondary calc-combo__close"
              onClick={() => {
                pendingFocusRef.current = "after-close";
                setOpen(false);
              }}
            >
              {comboScreenText.closeButton}
            </button>
          )}

          <div className="calc-combo__inputs">
            {defenderWasReset && (
              <p role="status" className="calc-screen__notice">
                {comboScreenText.defenderReset}
              </p>
            )}
            {defenders.length > 1 && (
              <label className="calc-combo__field">
                <span className="calc-screen__label">{comboScreenText.defenderLabel}</span>
                <select
                  aria-label={comboScreenText.defenderLabel}
                  value={defender?.key ?? ""}
                  onChange={(event) => {
                    setDefenderKey(event.target.value);
                  }}
                >
                  {defenders.map((option) => (
                    <option key={option.key} value={option.key}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <MovePicker
              label={comboScreenText.opponentMoveLabel}
              labelClassName="calc-screen__label"
              triggerClassName="calc-screen__move"
              moves={opponentCandidates.filter(canBeOpponentMove)}
              types={types}
              value={opponentMove?.id ?? ""}
              onChange={setOpponentMoveId}
              noneLabel={comboScreenText.opponentMoveNone}
              disabled={opponentCandidates.length === 0}
            />
            <ToggleChip
              type="checkbox"
              label={comboScreenText.opponentSpeedFromDefender}
              checked={opponentSpeedFromDefender}
              onChange={setOpponentSpeedFromDefender}
            />
            <fieldset className="calc-combo__self-speed">
              <legend className="calc-screen__label">{comboScreenText.selfSpeed.legend}</legend>
              <label className="calc-combo__field">
                <span className="calc-screen__label">{comboScreenText.selfSpeed.spLabel}</span>
                <input
                  type="number"
                  inputMode="numeric"
                  min={0}
                  max={maxSelfSpeedSp(attacker)}
                  step={1}
                  value={selfSpeed.sp ?? attacker.sp.spe}
                  onChange={(event) => {
                    const parsed = Number.parseInt(event.target.value, 10);
                    updateSelfSpeed({ sp: clampSelfSpeedSp(attacker, Number.isNaN(parsed) ? 0 : parsed) });
                  }}
                />
              </label>
              <label className="calc-combo__field">
                <span className="calc-screen__label">{comboScreenText.selfSpeed.natureLabel}</span>
                <select
                  value={selfSpeed.nature}
                  onChange={(event) => {
                    updateSelfSpeed({ nature: event.target.value as SelfSpeedNature });
                  }}
                >
                  {(["keep", "plus", "neutral", "minus"] as const).map((choice) => (
                    <option key={choice} value={choice}>
                      {comboScreenText.selfSpeed.natureOptions[choice]}
                    </option>
                  ))}
                </select>
              </label>
              <RankMeter
                label={comboScreenText.selfSpeed.rankLabel}
                statName={comboScreenText.selfSpeed.rankStatName}
                value={selfSpeed.rank ?? attacker.ranks?.spe ?? 0}
                onChange={(rank) => {
                  updateSelfSpeed({ rank });
                }}
                decreaseLabel={comboScreenText.selfSpeed.rankDownLabel}
                increaseLabel={comboScreenText.selfSpeed.rankUpLabel}
              />
              {selfSpeed.nature !== "keep" && (
                <p className="calc-screen__notice">{comboScreenText.selfSpeed.natureNote}</p>
              )}
              {scarfIgnored && (
                <p className="calc-screen__notice" role="status">
                  {comboScreenText.selfSpeed.scarfIgnored}
                </p>
              )}
            </fieldset>
            <button
              type="button"
              className="ui-button ui-button--secondary calc-conditions__toggle"
              aria-expanded={speedFieldOpen}
              aria-controls={speedFieldBodyId}
              onClick={() => {
                setSpeedFieldOpen(!speedFieldOpen);
              }}
            >
              <Icon name="open" size={16} />
              {comboScreenText.speedFieldToggle}
            </button>
            <div id={speedFieldBodyId}>
              {speedFieldOpen && (
                <div className="calc-conditions__group">
                  <ToggleChip
                    type="checkbox"
                    label={comboScreenText.trickRoom}
                    checked={speedField.trickRoom === true}
                    onChange={(trickRoom) => {
                      updateSpeedField({ trickRoom });
                    }}
                  />
                  <ToggleChip
                    type="checkbox"
                    label={comboScreenText.selfTailwind}
                    checked={speedField.selfTailwind === true}
                    onChange={(selfTailwind) => {
                      updateSpeedField({ selfTailwind });
                    }}
                  />
                  <ToggleChip
                    type="checkbox"
                    label={comboScreenText.opponentTailwind}
                    checked={speedField.opponentTailwind === true}
                    onChange={(opponentTailwind) => {
                      updateSpeedField({ opponentTailwind });
                    }}
                  />
                </div>
              )}
            </div>
          </div>

          {request !== null && current === null && (
            <p className="calc-screen__notice" role="status">
              {comboScreenText.loading}
            </p>
          )}
          {current !== null && !current.ok && current.error.code !== REQUEST_ABORTED_CODE && (
            <p role="alert" className="calc-screen__error">
              {current.error.code === COMBO_UNSUPPORTED_CODE
                ? current.error.message
                : comboScreenText.failed(current.error.message)}
            </p>
          )}
          {current?.ok === true && (
            <ComboResultView
              result={current.value}
              stepMoves={entries.map((entry) => moveOfEntry(entry))}
              moves={moves}
              items={items}
              abilities={abilities}
              speedClient={speedField.trickRoom === true ? undefined : speedClient}
            />
          )}
          <p className="calc-screen__notice calc-combo__note">{comboScreenText.note}</p>
        </>
      )}
    </section>
  );
}

interface ComboResultViewProps {
  readonly result: ComboResult;
  /** 各手の技(結果の steps と同じ順)。ランクチップに使う。 */
  readonly stepMoves: ReadonlyArray<Move | undefined>;
  readonly moves: readonly Move[];
  readonly items: readonly Item[];
  readonly abilities: readonly Ability[];
  /** 素早さの種族の例を引く口(無い・トリックルーム中は例を出さない)。 */
  readonly speedClient: SpeedClient | undefined;
}

/** 最大100%の一括表示予算(単発のダメージバーと同じ)。 */
const TOTAL_BAR_MAX_PERCENT = 100;

function ComboResultView({
  result,
  stepMoves,
  moves,
  items,
  abilities,
  speedClient,
}: ComboResultViewProps): ReactElement {
  const { total, speed, opponentAttack } = result;
  const lastStep = result.steps.at(-1);
  const markLabels = result.unsupported.map((mark) => comboMarkLabel(mark, moves, items, abilities));
  const barValue = Math.min(total.maxPercent, TOTAL_BAR_MAX_PERCENT);
  return (
    <div className="calc-results calc-combo__results">
      {markLabels.length > 0 && (
        <p role="status" className="calc-results__unsupported-notice">
          <span aria-hidden="true" className="calc-results__unsupported-icon">
            ⚠
          </span>
          <span>{unsupportedText.notice(markLabels)}</span>
        </p>
      )}
      <ol aria-label={comboScreenText.resultStepsListLabel} className="calc-combo__result-steps">
        {result.steps.map((step, index) => {
          const move = stepMoves[index];
          const orderLabel =
            step.order === "depends_on_opponent_speed" ? null : comboScreenText.orderLabel[step.order];
          return (
            <li key={`${String(index)}-${step.moveId}`} className="calc-combo__result-step">
              <span className="calc-combo__ordinal">{comboScreenText.stepOrdinal(index)}</span>
              <span className="calc-combo__move">{move?.nameJa ?? step.moveId}</span>
              {move !== undefined && <RankChips chips={rankChipsOf(move)} />}
              <span className="calc-results__percent">
                {step.damage === null ? comboScreenText.noDamage : formatPercentRange(step.damage)}
              </span>
              {orderLabel !== null && <span className="calc-combo__order">{orderLabel}</span>}
            </li>
          );
        })}
      </ol>
      <p className="calc-combo__total">
        {comboScreenText.total(formatPercent(total.minPercent), formatPercent(total.maxPercent))}
      </p>
      <div aria-hidden="true" data-testid="combo-total-bar" className="calc-results__bar">
        <div
          data-testid="combo-total-bar-fill"
          className="calc-results__bar-fill calc-combo__total-fill"
          style={{ width: `${String(barValue)}%` }}
        />
      </div>
      <p className="ui-badge calc-combo__ko">{comboKoText(result.steps.length, total.ko)}</p>
      <p className="calc-combo__speed">
        {comboScreenText.speedSummary(speed.firstSelfSpeed, speed.lastSelfSpeed)}
      </p>
      {speed.opponentSpeedKnown ? (
        <p className="calc-screen__notice">{comboScreenText.opponentSpeed(speed.opponentSpeed)}</p>
      ) : (
        <>
          {lastStep !== undefined && <p className="calc-screen__notice">{speedLineText(lastStep.line)}</p>}
          {speedClient !== undefined && (
            <SpeedExample speedClient={speedClient} selfSpeed={speed.lastSelfSpeed} />
          )}
        </>
      )}
      {opponentAttack !== null && (
        <div className="calc-combo__opponent">
          <p className="ui-badge calc-combo__survive">
            {comboScreenText.survive(formatSurviveChance(opponentAttack.fast.surviveChancePercent))}
          </p>
          <p className="calc-screen__notice">
            {comboScreenText.actions(opponentAttack.minActions, opponentAttack.maxActions)}
          </p>
          {opponentAttack.fast.actions[0] !== undefined && (
            <p className="calc-screen__notice">
              {comboScreenText.perHit(formatPercentRange(opponentAttack.fast.actions[0].damage))}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

/** 直近に届いた reach の応答と、それを求めた自分の素早さ(今の値と違えば古い応答として捨てる)。 */
interface SettledReach {
  readonly selfSpeed: number;
  readonly result: SpeedResult<SpeedComponents["schemas"]["ReachResponse"]>;
}

/**
 * 素早さの種族の例(段階 C1)。相手の素早さが不明のときだけ描かれ、最後の手の自分の素早さで speed API の reach(raw)を引く。
 * 失敗・古い応答は何も出さない(連続技の結果を壊さない)。
 */
function SpeedExample({
  speedClient,
  selfSpeed,
}: {
  readonly speedClient: SpeedClient;
  readonly selfSpeed: number;
}): ReactElement | null {
  const [settled, setSettled] = useState<SettledReach | null>(null);

  useEffect(() => {
    let cancelled = false;
    // reach は失敗も SpeedResult で返し reject しない(speedClient.ts)。万一の reject は例を出さないだけにする。
    void speedClient
      .reach({ mode: "raw", value: selfSpeed })
      .then((result) => {
        if (!cancelled) {
          setSettled({ selfSpeed, result });
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [speedClient, selfSpeed]);

  if (settled === null || settled.selfSpeed !== selfSpeed || !settled.result.ok) {
    return null;
  }
  const lines = speedExampleLines(settled.result.value.lines);
  if (lines.length === 0) {
    return null;
  }
  return (
    <ul aria-label={comboScreenText.speedExample.listLabel} className="calc-combo__speed-example">
      {lines.map((line) => (
        <li key={line} className="calc-screen__notice">
          {line}
        </li>
      ))}
    </ul>
  );
}
