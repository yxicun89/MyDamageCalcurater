// AJ6: 調整の画面(ADR-0319。plan.md「AJ: 調整」の AJ6)。
// 機能 2(火力指数・耐久指数・HP の 16n / 16n-1)・機能 3(SP 配分の提案)・機能 4(倒せる/耐える最小 SP)を
// 1画面にまとめ、機能 1(技を覚えるポケモン)は技の欄の横のボタンから開くパネルで出す。
// 計算はすべて calc-svc(API)が行い、Web は送って結果を出すだけ(ADR-0319 §1: API 専用の画面。WASM は使わない)。
// 「調整する」を押したときだけ API を呼ぶ(打鍵ごとに探索しない。ADR-0705 §7 と同じ)。
//
// 画面の構成(詳細は ADR-0319 §2・テストは adjust/AdjustScreen.test.tsx):
//   region「自分」          ポケモン・性格・特性・持ち物・技(+「覚えるポケモン」ボタン)・固定する能力ポイント 6 欄
//   region「調整の内容」    モードの radio 5 つと、モードごとの欄(耐久の基準・上限、攻撃の分類・上限・素早さの目標、「目標を指定する」)
//   region「相手」          ポケモン・調整(プリセット)・技(相手が攻撃する側のときだけ)。相手が要るモードだけ出す
//   region「目標」          発数・確率(プリセット)。相手が要るモードだけ出す
//   button「調整する」
//   region「調整の結果」    今の振り方の指数と 16n + モードの結果 + 未対応の印
//   region「この技を覚えるポケモン」 「覚えるポケモン」を押したときだけ出す

import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import type { components } from "../api/openapi.gen";
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
import { ADJUST_ITEM_ROLE_FILTER, itemsForRole } from "../domain/itemRoles";
import { megaStoneItemIds } from "../domain/mega";
import { isDamagingMove, learnsetMoves } from "../domain/moves";
import { BATTLE_LEVEL, MAX_SP_PER_STAT, MAX_SP_TOTAL, STAT_ORDER } from "../domain/requests";
import { unsupportedMarkLabels } from "../domain/unsupportedLabels";
import {
  REQUEST_ABORTED_CODE,
  type Ability,
  type Move,
  type MoveCategory,
  type StatKey,
} from "../engine/types";
import { adjustScreenText, unsupportedText, type AdjustModeKey } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";
import { SpeciesSearchField } from "../screens/SpeciesSearchField";
import "./AdjustScreen.css";
import type { AdjustClient, AdjustResult } from "./adjustClient";
import { adjustErrorMessage, formatChancePercent } from "./adjustFormat";
import {
  ADJUST_HITS_OPTIONS,
  ADJUST_NEUTRAL_MODIFIER,
  ADJUST_THRESHOLD_PRESETS,
  DEFAULT_ADJUST_HITS,
  DEFAULT_ADJUST_THRESHOLD_PERCENT,
  LEARNERS_PAGE_SIZE,
  firepowerModifier,
  natureIdForPreset,
} from "./adjustRequest";

type Schemas = components["schemas"];

const T = adjustScreenText;

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0319 §1)。 */
export interface AdjustScreenProps {
  readonly adjustClient: AdjustClient;
  /** 入力補助のマスタ(種族・性格・特性・持ち物・技の選択肢と名前。計算には使わない)。 */
  readonly master: MasterData;
  /** 種族を都度引く口(`master.capabilities.speciesList` が false のとき SpeciesSearchField で使う)。 */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** モードの表示順(radio の並び)。 */
const MODES: readonly AdjustModeKey[] = ["indices", "bulk", "offense", "minKo", "minSurvive"];

/** 能力ポイントの上限の既定(契約の省略と同じ 32)。 */
const DEFAULT_CEILING = String(MAX_SP_PER_STAT);

/** 耐久の基準の選択肢(契約の BulkFocus)。 */
const FOCUS_OPTIONS: readonly Schemas["BulkFocus"][] = ["physical", "special", "both"];

/** 攻撃の分類の選択肢。 */
const OFFENSE_CATEGORIES: readonly Schemas["MoveCategory"][] = ["physical", "special"];

/** 1つの種族の選択(名前・タイプ・覚える技・選べる特性)。オンラインでは resolveSpecies の結果から作る。 */
interface SpeciesChoice {
  readonly key: string;
  readonly name: string;
  readonly types: readonly string[];
  /** 覚える技のうちダメージ技だけ(変化技は指数・探索とも invalid_input になるため選ばせない)。 */
  readonly moves: readonly Move[];
  readonly abilities: readonly Ability[];
}

interface SelfState {
  readonly choice: SpeciesChoice | null;
  readonly natureId: string;
  readonly abilityId: string;
  readonly itemId: string;
  readonly moveId: string;
}

interface OpponentState {
  readonly choice: SpeciesChoice | null;
  readonly moveId: string;
}

/** 検査を通った1回分の送信(indices と、モードごとの操作)。 */
type Operation =
  | { readonly kind: "none" }
  | { readonly kind: "ko"; readonly request: Schemas["AdjustSearchRequest"] }
  | { readonly kind: "survive"; readonly request: Schemas["AdjustSearchRequest"] }
  | { readonly kind: "allocation"; readonly request: Schemas["AdjustAllocationRequest"] };

interface PreparedSubmit {
  readonly mode: AdjustModeKey;
  readonly indices: Schemas["AdjustIndicesRequest"];
  readonly operation: Operation;
  readonly hits: number;
  readonly hadGoal: boolean;
  /** 素早さの目標(実数値)があるか(offense の配分で、素早さの目標の可否を出すか)。 */
  readonly hasSpeedTarget: boolean;
}

/** モードの操作の結果(成功の値)。 */
type ModeResult =
  | { readonly kind: "none" }
  | { readonly kind: "ko"; readonly value: Schemas["AdjustKOResult"] }
  | { readonly kind: "survive"; readonly value: Schemas["AdjustSurviveResult"] }
  | { readonly kind: "allocation"; readonly value: Schemas["AdjustAllocationResult"] };

interface ResultView {
  readonly prepared: PreparedSubmit;
  readonly indices: Schemas["AdjustIndicesResult"];
  readonly modeResult: ModeResult;
  /** 未対応の印を、送信時点のマスタから引いた名前。 */
  readonly unsupportedLabels: readonly string[];
}

type ResultState =
  | { readonly status: "idle" }
  | { readonly status: "loading" }
  | { readonly status: "error"; readonly message: string }
  | { readonly status: "success"; readonly view: ResultView };

interface LearnersState {
  readonly moveId: string;
  readonly moveName: string;
  readonly items: readonly Schemas["SpeciesSummary"][];
  readonly loading: boolean;
  readonly errorMessage: string | null;
  readonly hasMore: boolean;
}

/** 整数として読めるか(空・小数・数でない文字は null)。 */
function parseInteger(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return null;
  }
  const value = Number(trimmed);
  return Number.isInteger(value) ? value : null;
}

/** 技の分類(変化技は攻撃の分類として使えないので null)。 */
function attackCategoryOf(move: Move | undefined): Schemas["MoveCategory"] | null {
  return move === undefined || move.category === "status" ? null : move.category;
}

function findMove(choice: SpeciesChoice | null, moveId: string): Move | undefined {
  return choice?.moves.find((move) => move.id === moveId);
}

/** 異なる由来の印を、対象・理由・ID の組で1件にまとめる。 */
function uniqueMarks(marks: readonly Schemas["UnsupportedMark"][]): Schemas["UnsupportedMark"][] {
  const seen = new Set<string>();
  const unique: Schemas["UnsupportedMark"][] = [];
  for (const mark of marks) {
    const key = `${mark.target}\u0000${mark.reason}\u0000${mark.id}`;
    if (!seen.has(key)) {
      seen.add(key);
      unique.push(mark);
    }
  }
  return unique;
}

/** 調整の画面(ADR-0319)。 */
export function AdjustScreen({ adjustClient, master, masterSearch }: AdjustScreenProps) {
  const speciesListAvailable = masterCapabilities(master).speciesList;

  const [mode, setMode] = useState<AdjustModeKey>("indices");
  const [self, setSelf] = useState<SelfState>({
    choice: null,
    natureId: "",
    abilityId: "",
    itemId: "",
    moveId: "",
  });
  const [fixedSp, setFixedSp] = useState<Record<StatKey, string>>({
    hp: "0",
    atk: "0",
    def: "0",
    spa: "0",
    spd: "0",
    spe: "0",
  });
  const [ceilings, setCeilings] = useState<Record<StatKey, string>>({
    hp: DEFAULT_CEILING,
    atk: DEFAULT_CEILING,
    def: DEFAULT_CEILING,
    spa: DEFAULT_CEILING,
    spd: DEFAULT_CEILING,
    spe: DEFAULT_CEILING,
  });
  const [focus, setFocus] = useState<Schemas["BulkFocus"]>("both");
  const [offenseCategoryOverride, setOffenseCategoryOverride] = useState<Schemas["MoveCategory"] | null>(
    null,
  );
  const [minSpeed, setMinSpeed] = useState("");
  const [useGoal, setUseGoal] = useState(false);
  const [opponent, setOpponent] = useState<OpponentState>({ choice: null, moveId: "" });
  const [attackerPreset, setAttackerPreset] = useState<AttackerPresetKey>(DEFAULT_ATTACKER_PRESET);
  const [defenderPreset, setDefenderPreset] = useState<DefenderPresetKey>(DEFAULT_DEFENDER_PRESET);
  const [hits, setHits] = useState(DEFAULT_ADJUST_HITS);
  const [thresholdPercent, setThresholdPercent] = useState(DEFAULT_ADJUST_THRESHOLD_PERCENT);
  const [result, setResult] = useState<ResultState>({ status: "idle" });
  const [learners, setLearners] = useState<LearnersState | null>(null);

  // 送信ごとの連番と取り消し(古い応答で上書きしない。ADR-0319 §4)。
  const submitSeqRef = useRef(0);
  const submitAbortRef = useRef<AbortController | null>(null);
  const learnersSeqRef = useRef(0);
  const learnersAbortRef = useRef<AbortController | null>(null);

  function abortSubmit(): void {
    submitAbortRef.current?.abort();
    submitAbortRef.current = null;
  }

  function abortLearners(): void {
    learnersAbortRef.current?.abort();
    learnersAbortRef.current = null;
  }

  // 画面を閉じたら計算中の呼び出しを取り消す。
  useEffect(() => {
    return () => {
      abortSubmit();
      abortLearners();
    };
  }, []);

  // ---- 導出する値 ----

  const needsOpponent =
    mode === "minKo" || mode === "minSurvive" || ((mode === "bulk" || mode === "offense") && useGoal);
  const opponentAttacks = mode === "minSurvive" || (mode === "bulk" && useGoal);
  // ADR-0326: 持ち物は攻撃・防御のどちらかの役割を持つものだけ(メガストーンは出さない。調整にメガの固定は無い)。
  const selectableItems = useMemo(
    () => itemsForRole(master.items, ADJUST_ITEM_ROLE_FILTER, megaStoneItemIds(master.species)),
    [master.items, master.species],
  );
  const selfMove = findMove(self.choice, self.moveId);
  const opponentMove = findMove(opponent.choice, opponent.moveId);
  const offenseCategory: Schemas["MoveCategory"] =
    offenseCategoryOverride ?? attackCategoryOf(selfMove) ?? "physical";
  /** 相手が受けるときの防御側プリセットの絞り込みに使う分類(自分の技、無ければ攻撃の分類)。 */
  const defenderCategory: MoveCategory =
    attackCategoryOf(selfMove) ?? (mode === "offense" ? offenseCategory : "physical");
  const attackerCategory: MoveCategory = attackCategoryOf(opponentMove) ?? "physical";
  const effectiveDefenderPreset = defenderPresetForCategory(defenderPreset, defenderCategory);
  const offenseStat: StatKey = offenseCategory === "special" ? "spa" : "atk";

  const fixedTotal = STAT_ORDER.reduce((sum, stat) => sum + (parseInteger(fixedSp[stat]) ?? 0), 0);

  // ---- 入力の更新 ----

  function choiceFromSpecies(
    key: string,
    name: string,
    types: readonly string[],
    moves: readonly Move[],
    abilities: readonly Ability[],
  ): SpeciesChoice {
    return { key, name, types, moves: moves.filter(isDamagingMove), abilities };
  }

  function chooseFromList(key: string): SpeciesChoice | null {
    const species = master.species.find((candidate) => candidate.key === key);
    return species === undefined
      ? null
      : choiceFromSpecies(
          species.key,
          species.nameJa,
          species.types,
          learnsetMoves(species, master.moves),
          master.abilities,
        );
  }

  function selectSelfSpecies(choice: SpeciesChoice | null): void {
    setSelf((current) => ({
      ...current,
      choice,
      moveId: choice?.moves.some((move) => move.id === current.moveId) === true ? current.moveId : "",
    }));
    setOffenseCategoryOverride(null);
  }

  function selectSelfMove(moveId: string): void {
    setSelf((current) => ({ ...current, moveId }));
    setOffenseCategoryOverride(null);
  }

  function selectOpponentSpecies(choice: SpeciesChoice | null): void {
    setOpponent((current) => ({
      choice,
      moveId: choice?.moves.some((move) => move.id === current.moveId) === true ? current.moveId : "",
    }));
  }

  // ---- 送信 ----

  /** 送信前の検査と request の組み立て(ADR-0319 §4・§5)。違反なら理由(message)を返す。 */
  function prepare(): PreparedSubmit | { readonly message: string } {
    if (self.choice === null || self.natureId === "") {
      return { message: T.selfRequiredMessage };
    }
    const sp: Record<StatKey, number> = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };
    for (const stat of STAT_ORDER) {
      const value = parseInteger(fixedSp[stat]);
      if (value === null || value < 0 || value > MAX_SP_PER_STAT) {
        return { message: T.spRangeMessage(MAX_SP_PER_STAT) };
      }
      sp[stat] = value;
    }
    if (fixedTotal > MAX_SP_TOTAL) {
      return { message: T.spTotalMessage(MAX_SP_TOTAL) };
    }

    const individual: Schemas["Individual"] = {
      speciesKey: self.choice.key,
      level: BATTLE_LEVEL,
      natureId: self.natureId,
      sp,
    };
    if (self.abilityId !== "") {
      individual.abilityId = self.abilityId;
    }
    if (self.itemId !== "") {
      individual.itemId = self.itemId;
    }

    const indices: Schemas["AdjustIndicesRequest"] = { individual };
    if (selfMove !== undefined) {
      indices.moveId = selfMove.id;
      const modifier = firepowerModifier(self.choice.types, selfMove.type);
      if (modifier !== ADJUST_NEUTRAL_MODIFIER) {
        indices.modifier = modifier;
      }
    }

    const base = { mode, indices, hits, hadGoal: false, hasSpeedTarget: false };
    if (mode === "indices") {
      return { ...base, operation: { kind: "none" } };
    }

    // 回す能力の上限(固定 SP 以上・0..32)。
    function ceilingFor(stats: readonly StatKey[]): Schemas["AdjustCeiling"] | { readonly message: string } {
      const ceiling: Schemas["AdjustCeiling"] = {};
      for (const stat of stats) {
        const value = parseInteger(ceilings[stat]);
        if (value === null || value < 0 || value > MAX_SP_PER_STAT) {
          return { message: T.spRangeMessage(MAX_SP_PER_STAT) };
        }
        if (value < sp[stat]) {
          return { message: T.ceilingBelowFixedMessage };
        }
        ceiling[stat] = value;
      }
      return ceiling;
    }

    function opponentIndividual(
      kind: "attacker" | "defender",
    ): Schemas["Individual"] | { readonly message: string } {
      if (opponent.choice === null) {
        return { message: T.opponentRequiredMessage };
      }
      const resolved =
        kind === "attacker"
          ? resolveAttackerPreset(attackerPreset, attackerCategory)
          : resolveDefenderPreset(effectiveDefenderPreset);
      const natureId = natureIdForPreset(master.natures, resolved.nature);
      if (natureId === null) {
        return { message: T.natureNotFoundMessage };
      }
      return { speciesKey: opponent.choice.key, level: BATTLE_LEVEL, natureId, sp: { ...resolved.sp } };
    }

    function searchRequest(
      attacker: Schemas["Individual"],
      defender: Schemas["Individual"],
      moveId: string,
    ): Schemas["AdjustSearchRequest"] {
      const request: Schemas["AdjustSearchRequest"] = { format: "single", attacker, defender, moveId, hits };
      if (thresholdPercent !== DEFAULT_ADJUST_THRESHOLD_PERCENT) {
        request.thresholdPercent = thresholdPercent;
      }
      return request;
    }

    if (mode === "minKo") {
      if (selfMove === undefined) {
        return { message: T.selfMoveRequiredMessage };
      }
      const defender = opponentIndividual("defender");
      if ("message" in defender) {
        return defender;
      }
      return {
        ...base,
        operation: { kind: "ko", request: searchRequest(individual, defender, selfMove.id) },
      };
    }

    if (mode === "minSurvive") {
      if (opponent.choice === null) {
        return { message: T.opponentRequiredMessage };
      }
      if (opponentMove === undefined) {
        return { message: T.opponentMoveRequiredMessage };
      }
      const attacker = opponentIndividual("attacker");
      if ("message" in attacker) {
        return attacker;
      }
      return {
        ...base,
        operation: { kind: "survive", request: searchRequest(attacker, individual, opponentMove.id) },
      };
    }

    // bulk / offense(配分)。
    const ceiling = mode === "bulk" ? ceilingFor(["hp", "def", "spd"]) : ceilingFor([offenseStat, "spe"]);
    if ("message" in ceiling) {
      return ceiling;
    }
    const request: Schemas["AdjustAllocationRequest"] = { self: individual, mode, ceiling, minSpeed: 0 };
    let hasSpeedTarget = false;
    if (mode === "bulk") {
      request.focus = focus;
    } else {
      request.offenseCategory = offenseCategory;
      const raw = minSpeed.trim();
      const target = raw === "" ? 0 : parseInteger(raw);
      if (target === null || target < 0) {
        return { message: T.minSpeedMessage };
      }
      request.minSpeed = target;
      hasSpeedTarget = target > 0;
    }
    if (useGoal) {
      const goalBase = { format: "single", hits } as const;
      let goal: Schemas["AdjustAllocGoal"];
      if (mode === "bulk") {
        if (opponent.choice === null) {
          return { message: T.opponentRequiredMessage };
        }
        if (opponentMove === undefined) {
          return { message: T.opponentMoveRequiredMessage };
        }
        const attacker = opponentIndividual("attacker");
        if ("message" in attacker) {
          return attacker;
        }
        goal = { ...goalBase, opponent: attacker, moveId: opponentMove.id };
      } else {
        if (selfMove === undefined) {
          return { message: T.selfMoveRequiredMessage };
        }
        if (attackCategoryOf(selfMove) !== offenseCategory) {
          return { message: T.categoryMismatchMessage };
        }
        const defender = opponentIndividual("defender");
        if ("message" in defender) {
          return defender;
        }
        goal = { ...goalBase, opponent: defender, moveId: selfMove.id };
      }
      if (thresholdPercent !== DEFAULT_ADJUST_THRESHOLD_PERCENT) {
        goal.thresholdPercent = thresholdPercent;
      }
      request.goal = goal;
    }
    return { ...base, operation: { kind: "allocation", request }, hadGoal: useGoal, hasSpeedTarget };
  }

  /** モードの操作を1つ呼ぶ(呼び出しは同期的に始まる)。 */
  async function runOperation(operation: Operation, signal: AbortSignal): Promise<AdjustResult<ModeResult>> {
    switch (operation.kind) {
      case "none":
        return { ok: true, value: { kind: "none" } };
      case "ko": {
        const response = await adjustClient.minSpToKo(operation.request, signal);
        return response.ok ? { ok: true, value: { kind: "ko", value: response.value } } : response;
      }
      case "survive": {
        const response = await adjustClient.minSpToSurvive(operation.request, signal);
        return response.ok ? { ok: true, value: { kind: "survive", value: response.value } } : response;
      }
      case "allocation": {
        const response = await adjustClient.allocation(operation.request, signal);
        return response.ok ? { ok: true, value: { kind: "allocation", value: response.value } } : response;
      }
    }
  }

  function allMoves(): readonly Move[] {
    return [...master.moves, ...(self.choice?.moves ?? []), ...(opponent.choice?.moves ?? [])];
  }

  function allAbilities(): readonly Ability[] {
    return [...master.abilities, ...(self.choice?.abilities ?? []), ...(opponent.choice?.abilities ?? [])];
  }

  async function handleSubmit(): Promise<void> {
    abortSubmit();
    submitSeqRef.current += 1;
    const seq = submitSeqRef.current;

    const prepared = prepare();
    if ("message" in prepared) {
      setResult({ status: "error", message: prepared.message });
      return;
    }

    const controller = new AbortController();
    submitAbortRef.current = controller;
    setResult({ status: "loading" });

    // indices とモードの操作を並行して呼び、両方そろってから結果を出す(ADR-0319 §4)。
    const [indicesResponse, operationResponse] = await Promise.all([
      adjustClient.indices(prepared.indices, controller.signal),
      runOperation(prepared.operation, controller.signal),
    ]);
    if (submitSeqRef.current !== seq || controller.signal.aborted) {
      return;
    }
    const failure = !indicesResponse.ok ? indicesResponse : !operationResponse.ok ? operationResponse : null;
    if (failure !== null) {
      if (failure.error.code !== REQUEST_ABORTED_CODE) {
        setResult({ status: "error", message: adjustErrorMessage(failure.error.code) });
      }
      return;
    }
    if (!indicesResponse.ok || !operationResponse.ok) {
      return;
    }
    const modeResult = operationResponse.value;
    const marks = modeResult.kind === "none" ? [] : modeResult.value.unsupported;
    setResult({
      status: "success",
      view: {
        prepared,
        indices: indicesResponse.value,
        modeResult,
        unsupportedLabels: unsupportedMarkLabels(
          uniqueMarks(marks),
          allMoves(),
          master.items,
          allAbilities(),
        ),
      },
    });
  }

  // ---- 技を覚えるポケモン ----

  async function loadLearners(
    moveId: string,
    moveName: string,
    base: readonly Schemas["SpeciesSummary"][],
  ): Promise<void> {
    abortLearners();
    learnersSeqRef.current += 1;
    const seq = learnersSeqRef.current;
    const controller = new AbortController();
    learnersAbortRef.current = controller;
    setLearners({ moveId, moveName, items: base, loading: true, errorMessage: null, hasMore: false });

    const response = await adjustClient.moveLearners(
      moveId,
      { limit: LEARNERS_PAGE_SIZE, offset: base.length },
      controller.signal,
    );
    if (learnersSeqRef.current !== seq || controller.signal.aborted) {
      return;
    }
    if (!response.ok) {
      if (response.error.code !== REQUEST_ABORTED_CODE) {
        setLearners({
          moveId,
          moveName,
          items: base,
          loading: false,
          errorMessage: adjustErrorMessage(response.error.code),
          // 続きの読み込みの失敗は、もう一度押せるように残す(最初の1ページの失敗は開き直す)。
          hasMore: base.length > 0,
        });
      }
      return;
    }
    setLearners({
      moveId,
      moveName,
      items: [...base, ...response.value],
      loading: false,
      errorMessage: null,
      hasMore: response.value.length === LEARNERS_PAGE_SIZE,
    });
  }

  function openLearners(move: Move | undefined): void {
    if (move !== undefined) {
      void loadLearners(move.id, move.nameJa, []);
    }
  }

  const loading = result.status === "loading";
  const errorMessage = result.status === "error" ? result.message : null;
  const resultHeadingId = useId();
  const selfHeadingId = useId();
  const modeHeadingId = useId();
  const opponentHeadingId = useId();
  const goalHeadingId = useId();
  const fixedSpHintId = useId();
  const minSpeedHintId = useId();

  const abilityOptions = self.choice?.abilities ?? (speciesListAvailable ? master.abilities : []);

  return (
    <div className="adjust-screen">
      <section aria-labelledby={selfHeadingId} className="adjust-screen__region">
        <h2 id={selfHeadingId}>{T.selfRegionLabel}</h2>
        <div className="adjust-screen__fields">
          <SpeciesField
            visibleLabel={T.speciesFieldLabel}
            name={T.selfSpeciesLabel}
            choice={self.choice}
            onSelect={selectSelfSpecies}
            chooseFromList={chooseFromList}
            speciesListAvailable={speciesListAvailable}
            master={master}
            masterSearch={masterSearch}
            fromResolution={(resolution) =>
              choiceFromSpecies(
                resolution.species.key,
                resolution.species.nameJa,
                resolution.species.types,
                resolution.moves,
                resolution.abilities,
              )
            }
          />
          <LabeledSelect
            visibleLabel={T.natureFieldLabel}
            name={T.selfNatureLabel}
            value={self.natureId}
            onChange={(natureId) => {
              setSelf((current) => ({ ...current, natureId }));
            }}
          >
            <option value="">{T.naturePlaceholder}</option>
            {master.natures.map((nature) => (
              <option key={nature.id} value={nature.id}>
                {nature.nameJa}
              </option>
            ))}
          </LabeledSelect>
          <LabeledSelect
            visibleLabel={T.abilityFieldLabel}
            name={T.selfAbilityLabel}
            value={self.abilityId}
            onChange={(abilityId) => {
              setSelf((current) => ({ ...current, abilityId }));
            }}
          >
            <option value="">{T.unselectedOption}</option>
            {abilityOptions.map((ability) => (
              <option key={ability.id} value={ability.id}>
                {ability.nameJa}
              </option>
            ))}
          </LabeledSelect>
          <LabeledSelect
            visibleLabel={T.itemFieldLabel}
            name={T.selfItemLabel}
            value={self.itemId}
            onChange={(itemId) => {
              setSelf((current) => ({ ...current, itemId }));
            }}
          >
            <option value="">{T.unselectedOption}</option>
            {selectableItems.map((item) => (
              <option key={item.id} value={item.id}>
                {item.nameJa}
              </option>
            ))}
          </LabeledSelect>
          <MoveField
            visibleLabel={T.moveFieldLabel}
            name={T.selfMoveLabel}
            moves={self.choice?.moves ?? []}
            moveId={self.moveId}
            onChange={selectSelfMove}
            learnersButtonName={T.selfLearnersButtonName}
            onOpenLearners={() => {
              openLearners(selfMove);
            }}
          />
        </div>

        <fieldset className="adjust-screen__fixed-sp">
          <legend>{T.fixedSpGroupLabel}</legend>
          <p id={fixedSpHintId} className="adjust-screen__hint">
            {T.fixedSpHint}
          </p>
          <div className="adjust-screen__fields">
            {STAT_ORDER.map((stat) => (
              <NumberField
                key={stat}
                label={T.fixedSpLabel(stat)}
                value={fixedSp[stat]}
                describedBy={fixedSpHintId}
                onChange={(value) => {
                  setFixedSp((current) => ({ ...current, [stat]: value }));
                }}
              />
            ))}
          </div>
          <p>{T.fixedSpTotal(fixedTotal, MAX_SP_TOTAL)}</p>
        </fieldset>
      </section>

      <section aria-labelledby={modeHeadingId} className="adjust-screen__region">
        <h2 id={modeHeadingId}>{T.modeRegionLabel}</h2>
        <div role="radiogroup" aria-label={T.modeGroupLabel} className="adjust-screen__modes">
          {MODES.map((candidate) => (
            <label key={candidate} className="adjust-screen__choice">
              <input
                type="radio"
                name="adjust-mode"
                checked={mode === candidate}
                onChange={() => {
                  setMode(candidate);
                }}
              />
              {T.modeLabel[candidate]}
            </label>
          ))}
        </div>

        {mode === "bulk" && (
          <div className="adjust-screen__fields">
            <LabeledSelect
              visibleLabel={T.focusLabel}
              name={T.focusLabel}
              value={focus}
              onChange={(value) => {
                setFocus(value as Schemas["BulkFocus"]);
              }}
            >
              {FOCUS_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {T.focusOption[option]}
                </option>
              ))}
            </LabeledSelect>
            {(["hp", "def", "spd"] as const).map((stat) => (
              <CeilingField key={stat} stat={stat} ceilings={ceilings} onChange={setCeilings} />
            ))}
          </div>
        )}

        {mode === "offense" && (
          <div className="adjust-screen__fields">
            <LabeledSelect
              visibleLabel={T.offenseCategoryLabel}
              name={T.offenseCategoryLabel}
              value={offenseCategory}
              onChange={(value) => {
                setOffenseCategoryOverride(value as Schemas["MoveCategory"]);
              }}
            >
              {OFFENSE_CATEGORIES.map((option) => (
                <option key={option} value={option}>
                  {T.offenseCategoryOption[option as "physical" | "special"]}
                </option>
              ))}
            </LabeledSelect>
            {([offenseStat, "spe"] as const).map((stat) => (
              <CeilingField key={stat} stat={stat} ceilings={ceilings} onChange={setCeilings} />
            ))}
            <NumberField
              label={T.minSpeedLabel}
              value={minSpeed}
              describedBy={minSpeedHintId}
              onChange={setMinSpeed}
            />
            <p id={minSpeedHintId} className="adjust-screen__hint">
              {T.minSpeedHint}
            </p>
          </div>
        )}

        {(mode === "bulk" || mode === "offense") && (
          <label className="adjust-screen__choice">
            <input
              type="checkbox"
              checked={useGoal}
              onChange={(event) => {
                setUseGoal(event.target.checked);
              }}
            />
            {T.useGoalLabel}
          </label>
        )}
      </section>

      {needsOpponent && (
        <section aria-labelledby={opponentHeadingId} className="adjust-screen__region">
          <h2 id={opponentHeadingId}>{T.opponentRegionLabel}</h2>
          <div className="adjust-screen__fields">
            <SpeciesField
              visibleLabel={T.speciesFieldLabel}
              name={T.opponentSpeciesLabel}
              choice={opponent.choice}
              onSelect={selectOpponentSpecies}
              chooseFromList={chooseFromList}
              speciesListAvailable={speciesListAvailable}
              master={master}
              masterSearch={masterSearch}
              fromResolution={(resolution) =>
                choiceFromSpecies(
                  resolution.species.key,
                  resolution.species.nameJa,
                  resolution.species.types,
                  resolution.moves,
                  resolution.abilities,
                )
              }
            />
            {opponentAttacks ? (
              <LabeledSelect
                visibleLabel={T.presetFieldLabel}
                name={T.opponentPresetLabel}
                value={attackerPreset}
                onChange={(value) => {
                  setAttackerPreset(value as AttackerPresetKey);
                }}
              >
                {ATTACKER_PRESET_KEYS.map((key) => (
                  <option key={key} value={key}>
                    {attackerPresetLabel(key, attackerCategory)}
                  </option>
                ))}
              </LabeledSelect>
            ) : (
              <LabeledSelect
                visibleLabel={T.presetFieldLabel}
                name={T.opponentPresetLabel}
                value={effectiveDefenderPreset}
                onChange={(value) => {
                  setDefenderPreset(value as DefenderPresetKey);
                }}
              >
                {defenderPresetKeysFor(defenderCategory).map((key) => (
                  <option key={key} value={key}>
                    {defenderPresetLabel(key)}
                  </option>
                ))}
              </LabeledSelect>
            )}
            {opponentAttacks && (
              <MoveField
                visibleLabel={T.moveFieldLabel}
                name={T.opponentMoveLabel}
                moves={opponent.choice?.moves ?? []}
                moveId={opponent.moveId}
                onChange={(moveId) => {
                  setOpponent((current) => ({ ...current, moveId }));
                }}
                learnersButtonName={T.opponentLearnersButtonName}
                onOpenLearners={() => {
                  openLearners(opponentMove);
                }}
              />
            )}
          </div>
        </section>
      )}

      {needsOpponent && (
        <section aria-labelledby={goalHeadingId} className="adjust-screen__region">
          <h2 id={goalHeadingId}>{T.goalRegionLabel}</h2>
          <div className="adjust-screen__fields">
            <LabeledSelect
              visibleLabel={T.hitsLabel}
              name={T.hitsLabel}
              value={String(hits)}
              onChange={(value) => {
                setHits(Number(value));
              }}
            >
              {ADJUST_HITS_OPTIONS.map((option) => (
                <option key={option} value={String(option)}>
                  {T.hitsOption(option)}
                </option>
              ))}
            </LabeledSelect>
            <LabeledSelect
              visibleLabel={T.thresholdLabel}
              name={T.thresholdLabel}
              value={String(thresholdPercent)}
              onChange={(value) => {
                setThresholdPercent(Number(value));
              }}
            >
              {ADJUST_THRESHOLD_PRESETS.map((option) => (
                <option key={option} value={String(option)}>
                  {T.thresholdOption(option)}
                </option>
              ))}
            </LabeledSelect>
          </div>
        </section>
      )}

      {errorMessage !== null && (
        <p role="alert" className="adjust-screen__error">
          {errorMessage}
        </p>
      )}

      <button
        type="button"
        className="adjust-screen__submit"
        onClick={() => {
          void handleSubmit();
        }}
      >
        {T.submitLabel}
      </button>

      <section aria-labelledby={resultHeadingId} aria-busy={loading} className="adjust-screen__region">
        <h2 id={resultHeadingId}>{T.resultRegionLabel}</h2>
        {result.status === "success" ? (
          <ResultBody view={result.view} />
        ) : loading ? (
          <p className="adjust-screen__notice">{T.loadingNotice}</p>
        ) : result.status === "idle" ? (
          <p className="adjust-screen__notice">{T.emptyResultNotice}</p>
        ) : null}
      </section>

      {learners !== null && (
        <section aria-label={T.learnersRegionLabel} className="adjust-screen__region">
          <h2>{T.learnersHeading(learners.moveName)}</h2>
          {learners.errorMessage !== null && (
            <p role="alert" className="adjust-screen__error">
              {learners.errorMessage}
            </p>
          )}
          {learners.items.length > 0 && (
            <ul className="adjust-screen__learners">
              {learners.items.map((species) => (
                <li key={species.key}>{species.nameJa}</li>
              ))}
            </ul>
          )}
          {learners.loading && <p className="adjust-screen__notice">{T.learnersLoading}</p>}
          {!learners.loading && learners.errorMessage === null && learners.items.length === 0 && (
            <p className="adjust-screen__notice">{T.learnersEmpty}</p>
          )}
          {learners.hasMore && !learners.loading && (
            <button
              type="button"
              onClick={() => {
                void loadLearners(learners.moveId, learners.moveName, learners.items);
              }}
            >
              {T.learnersMore}
            </button>
          )}
        </section>
      )}
    </div>
  );
}

// ---- 入力の部品 ----

interface LabeledSelectProps {
  /** 見えるラベル(短い語)。accessible name はこれを含む(SC 2.5.3)。 */
  readonly visibleLabel: string;
  readonly name: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly children: ReactNode;
}

/** select と、for で結んだ見えるラベル(select の中の option の文字をラベルに混ぜない)。 */
function LabeledSelect({ visibleLabel, name, value, onChange, children }: LabeledSelectProps) {
  const id = useId();
  return (
    <div className="adjust-screen__field">
      <label htmlFor={id}>{visibleLabel}</label>
      <select
        id={id}
        aria-label={name}
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        {children}
      </select>
    </div>
  );
}

interface NumberFieldProps {
  readonly label: string;
  readonly value: string;
  readonly describedBy?: string;
  readonly onChange: (value: string) => void;
}

/** 数値欄(type=number。見えるラベルと accessible name は同じ語)。 */
function NumberField({ label, value, describedBy, onChange }: NumberFieldProps) {
  const id = useId();
  return (
    <div className="adjust-screen__field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="number"
        inputMode="numeric"
        value={value}
        aria-describedby={describedBy}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      />
    </div>
  );
}

interface CeilingFieldProps {
  readonly stat: StatKey;
  readonly ceilings: Record<StatKey, string>;
  readonly onChange: (next: Record<StatKey, string>) => void;
}

function CeilingField({ stat, ceilings, onChange }: CeilingFieldProps) {
  return (
    <NumberField
      label={T.ceilingLabel(stat)}
      value={ceilings[stat]}
      onChange={(value) => {
        onChange({ ...ceilings, [stat]: value });
      }}
    />
  );
}

interface SpeciesFieldProps {
  readonly visibleLabel: string;
  readonly name: string;
  readonly choice: SpeciesChoice | null;
  readonly onSelect: (choice: SpeciesChoice | null) => void;
  readonly chooseFromList: (key: string) => SpeciesChoice | null;
  readonly speciesListAvailable: boolean;
  readonly master: MasterData;
  readonly masterSearch?: MasterSpeciesSearch;
  readonly fromResolution: (resolution: MasterSpeciesResolution) => SpeciesChoice;
}

/** 種族の欄。一覧が使えるときは select、使えないときは検索欄(SpeciesSearchField)。 */
function SpeciesField({
  visibleLabel,
  name,
  choice,
  onSelect,
  chooseFromList,
  speciesListAvailable,
  master,
  masterSearch,
  fromResolution,
}: SpeciesFieldProps) {
  if (!speciesListAvailable) {
    return (
      <SpeciesSearchField
        label={name}
        masterSearch={masterSearch}
        onResolved={(resolution) => {
          onSelect(fromResolution(resolution));
        }}
      />
    );
  }
  return (
    <LabeledSelect
      visibleLabel={visibleLabel}
      name={name}
      value={choice?.key ?? ""}
      onChange={(key) => {
        onSelect(chooseFromList(key));
      }}
    >
      <option value="">{T.speciesPlaceholder}</option>
      {master.species.map((species) => (
        <option key={species.key} value={species.key}>
          {species.nameJa}
        </option>
      ))}
    </LabeledSelect>
  );
}

interface MoveFieldProps {
  readonly visibleLabel: string;
  readonly name: string;
  readonly moves: readonly Move[];
  readonly moveId: string;
  readonly onChange: (moveId: string) => void;
  readonly learnersButtonName: string;
  readonly onOpenLearners: () => void;
}

/** 技の欄と「覚えるポケモン」ボタン(技を選ぶまで押せない)。 */
function MoveField({
  visibleLabel,
  name,
  moves,
  moveId,
  onChange,
  learnersButtonName,
  onOpenLearners,
}: MoveFieldProps) {
  return (
    <div className="adjust-screen__move">
      <LabeledSelect visibleLabel={visibleLabel} name={name} value={moveId} onChange={onChange}>
        <option value="">{T.movePlaceholder}</option>
        {moves.map((move) => (
          <option key={move.id} value={move.id}>
            {move.nameJa}
          </option>
        ))}
      </LabeledSelect>
      <button type="button" aria-label={learnersButtonName} disabled={moveId === ""} onClick={onOpenLearners}>
        {T.learnersButtonLabel}
      </button>
    </div>
  );
}

// ---- 結果 ----

function ResultBody({ view }: { readonly view: ResultView }) {
  const { prepared, modeResult } = view;
  return (
    <>
      {view.unsupportedLabels.length > 0 && (
        <p className="adjust-screen__notice">{unsupportedText.notice(view.unsupportedLabels)}</p>
      )}
      <IndicesView indices={view.indices} />
      {modeResult.kind === "ko" && (
        <p>
          {modeResult.value.feasible
            ? T.koFeasible(
                modeResult.value.stat,
                modeResult.value.sp,
                prepared.hits,
                formatChancePercent(modeResult.value.chancePercent),
              )
            : T.koInfeasible(
                modeResult.value.stat,
                modeResult.value.sp,
                prepared.hits,
                formatChancePercent(modeResult.value.chancePercent),
              )}
        </p>
      )}
      {modeResult.kind === "survive" && (
        <p>
          {modeResult.value.feasible
            ? T.surviveFeasible(
                modeResult.value.stat,
                modeResult.value.hpSp,
                modeResult.value.statSp,
                prepared.hits,
                formatChancePercent(modeResult.value.chancePercent),
              )
            : T.surviveInfeasible(
                modeResult.value.stat,
                modeResult.value.hpSp,
                modeResult.value.statSp,
                prepared.hits,
                formatChancePercent(modeResult.value.chancePercent),
              )}
        </p>
      )}
      {modeResult.kind === "allocation" && (
        <AllocationView
          allocation={modeResult.value}
          hadGoal={prepared.hadGoal}
          hasSpeedTarget={prepared.hasSpeedTarget}
        />
      )}
    </>
  );
}

function HpLineItem({
  label,
  point,
}: {
  readonly label: string;
  readonly point: Schemas["HPLinePoint"] | null;
}) {
  return (
    <li>{point === null ? T.hpLineNone(label) : T.hpLinePoint(label, point.hp, point.sp, point.spDelta)}</li>
  );
}

function IndicesView({ indices }: { readonly indices: Schemas["AdjustIndicesResult"] }) {
  const headingId = useId();
  const lines = indices.hpLines;
  return (
    <section aria-labelledby={headingId} className="adjust-screen__subregion">
      <h3 id={headingId}>{T.indicesHeading}</h3>
      <p>{T.statsLine(indices.stats)}</p>
      <p>{T.indexLine(T.firepowerIndexLabel, indices.firepowerIndex ?? T.firepowerIndexNone)}</p>
      <p>{T.indexLine(T.physicalBulkLabel, indices.physicalBulkIndex)}</p>
      <p>{T.indexLine(T.specialBulkLabel, indices.specialBulkIndex)}</p>
      <p className="adjust-screen__hint">{T.indexNote}</p>
      <h4>{T.hpLineHeading}</h4>
      <p>{T.hpCurrent(lines.hp, T.hpLineKindLabel[lines.current])}</p>
      <ul className="adjust-screen__lines">
        <HpLineItem label={T.next16nLabel} point={lines.next16n} />
        <HpLineItem label={T.prev16nLabel} point={lines.prev16n} />
        <HpLineItem label={T.next16nMinus1Label} point={lines.next16nMinus1} />
        <HpLineItem label={T.prev16nMinus1Label} point={lines.prev16nMinus1} />
      </ul>
    </section>
  );
}

interface AllocationViewProps {
  readonly allocation: Schemas["AdjustAllocationResult"];
  readonly hadGoal: boolean;
  readonly hasSpeedTarget: boolean;
}

function AllocationView({ allocation, hadGoal, hasSpeedTarget }: AllocationViewProps) {
  return (
    <>
      <p>{T.remainingLabel(allocation.remaining)}</p>
      <PlanView
        heading={T.maxIndexHeading}
        plan={allocation.maxIndex}
        hadGoal={hadGoal}
        hasSpeedTarget={hasSpeedTarget}
      />
      {allocation.minSp !== null ? (
        <PlanView
          heading={T.minSpHeading}
          plan={allocation.minSp}
          hadGoal={hadGoal}
          hasSpeedTarget={hasSpeedTarget}
        />
      ) : (
        !hadGoal && <p className="adjust-screen__notice">{T.minSpNotRequested}</p>
      )}
    </>
  );
}

interface PlanViewProps {
  readonly heading: string;
  readonly plan: Schemas["AdjustAllocPlan"];
  readonly hadGoal: boolean;
  readonly hasSpeedTarget: boolean;
}

function PlanView({ heading, plan, hadGoal, hasSpeedTarget }: PlanViewProps) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="adjust-screen__subregion">
      <h3 id={headingId}>{heading}</h3>
      <p>{T.planSpLine(plan.sp)}</p>
      <p>{T.planTotal(plan.totalSp)}</p>
      <p>{T.statsLine(plan.stats)}</p>
      <p>{T.indexLine(T.physicalBulkLabel, plan.physicalBulk)}</p>
      <p>{T.indexLine(T.specialBulkLabel, plan.specialBulk)}</p>
      {hadGoal && (
        <p>
          {plan.goalMet
            ? T.goalMet(formatChancePercent(plan.chancePercent))
            : T.goalNotMet(formatChancePercent(plan.chancePercent))}
        </p>
      )}
      {hasSpeedTarget && <p>{plan.speedMet ? T.speedMet : T.speedNotMet}</p>}
    </section>
  );
}
