// JD5: 判定の画面(ADR-0705 §4〜§8、docs/judge-design.md §3 JD5)。
// 自分のポケモン1体(使う技込み)と相手候補1〜6件を入力し、judge-svc の
// POST /api/judge/v1/outspeed-and-ko を「判定する」で1回だけ呼ぶ(ADR-0705 §7)。
// 判定(素早さ・行動順・双方向の確定数)は judge-svc が決める。Web は engine(WASM)で計算し直さず、
// 「勝ち / 負け」にも丸めない(ADR-0700 §6-1・ADR-0704 §3 の立場を画面でも保つ)。
// master / masterSearch は**入力補助にだけ**使う(種族・性格・特性・持ち物の選択肢。ADR-0705 §5)。
// 技は ID の自由入力(ADR-0304 §3: ID から技を引く公開 API がまだ無い)。

import { useRef, useState } from "react";
import type { Ranks, StatKey, Stats } from "../engine/types";
import { MAX_SP_PER_STAT, MAX_SP_TOTAL } from "../domain/requests";
import { judgeErrorText, judgeScreenText } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { SpeciesSearchField } from "../screens/SpeciesSearchField";
import "./JudgeScreen.css";
import type { components } from "./judge.gen";
import type { JudgeClient } from "./judgeClient";

type Schemas = components["schemas"];

/** 画面の props(App.tsx が app/screens.tsx 経由で注入する。ADR-0705 §1)。 */
export interface JudgeScreenProps {
  readonly judgeClient: JudgeClient;
  /** 入力補助のマスタ(性格・特性・持ち物の選択肢、種族の一覧。判定の計算には使わない)。 */
  readonly master: MasterData;
  /**
   * 種族を都度引く口(ADR-0304 §1)。`master.capabilities.speciesList` が false のとき、
   * ポケモンのドロップダウンの代わりに SpeciesSearchField でこの口を使う。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 相手候補の上限(契約の defenders は 1〜6 件。ADR-0703 §1)。 */
export const MAX_DEFENDERS = 6;

/** SP の6欄(表示順)。 */
const SP_STATS: readonly StatKey[] = ["hp", "atk", "def", "spa", "spd", "spe"];

/** ランクのキー(HP を持たない。RankBlock と同じ順)。 */
type RankKey = keyof Ranks;

/** ランクの5欄(HP を持たない。RankBlock と同じ順)。 */
const RANK_STATS: readonly RankKey[] = ["atk", "def", "spa", "spd", "spe"];

/** ランクの範囲(CLAUDE.md ドメイン規約 / 契約の RankBlock と同じ -6..+6)。 */
const MIN_RANK = -6;
const MAX_RANK = 6;

/** 1体分の入力の状態(自分・候補で共通の形。ADR-0705 §4)。moveId は自分は request 直下、候補は候補の欄に使う。 */
interface IndividualFormState {
  readonly speciesKey: string;
  /** 選んだ種族の表示名(結果の行に出す。オンライン検索では master.species が空なのでここに持つ)。 */
  readonly speciesName: string;
  readonly natureId: string;
  readonly sp: Stats;
  /**
   * ランク(-6..+6)は文字列で持つ(type="text" の生の入力そのまま)。type="number" だと、負数を
   * 1文字ずつ打つ途中の "-" 単独をブラウザ(jsdom)が無効値として即座に "" へ戻してしまい、
   * user-event でのキー入力を模したテストで負数を入力できない。数への変換は parseRank で行う。
   */
  readonly ranks: Record<RankKey, string>;
  readonly abilityId: string;
  readonly itemId: string;
  readonly moveId: string;
}

/** 未入力の1体(既定値はすべて空・SP とランクは0。ADR-0705 §6)。 */
function emptyIndividual(): IndividualFormState {
  return {
    speciesKey: "",
    speciesName: "",
    natureId: "",
    sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
    ranks: { atk: "0", def: "0", spa: "0", spd: "0", spe: "0" },
    abilityId: "",
    itemId: "",
    moveId: "",
  };
}

/** 相手候補1件(React の key 用の id を持つ。BalanceScreen.tsx の MemberState と同じ考え方)。 */
interface CandidateState extends IndividualFormState {
  readonly id: number;
}

/** ランクの1欄を数へ変換する(空白のみ・数でなければ NaN)。 */
function parseRank(raw: string): number {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return Number.NaN;
  }
  return Number.parseInt(trimmed, 10);
}

/** validationMessage を通った(= 全欄が -6..+6 の整数の)ランクだけを渡す前提。 */
function isZeroRanks(ranks: Record<RankKey, string>): boolean {
  return RANK_STATS.every((stat) => parseRank(ranks[stat]) === 0);
}

/** 検査を通ったランクを数の RankBlock にする(validationMessage が範囲・整数であることを保証済み)。 */
function toRankBlock(ranks: Record<RankKey, string>): Ranks {
  return {
    atk: parseRank(ranks.atk),
    def: parseRank(ranks.def),
    spa: parseRank(ranks.spa),
    spd: parseRank(ranks.spd),
    spe: parseRank(ranks.spe),
  };
}

/**
 * 数値入力の onChange から整数を読む。空・数でなければ null にし、呼び出し側は state を更新しない
 * (SP は 0 以上なので "-" 単独の問題が無く、type="number" のまま使える)。
 */
function parseIntFieldChange(raw: string): number | null {
  if (raw.trim() === "") {
    return null;
  }
  const parsed = Number.parseInt(raw, 10);
  return Number.isNaN(parsed) ? null : parsed;
}

/** Individual(自分)を、省略可の欄は選んだときだけ持つ形で組み立てる(ADR-0705 §6: 最小の request)。 */
function buildIndividual(state: IndividualFormState): Schemas["Individual"] {
  const individual: Schemas["Individual"] = {
    speciesKey: state.speciesKey,
    natureId: state.natureId,
    sp: { ...state.sp },
  };
  if (!isZeroRanks(state.ranks)) {
    individual.ranks = toRankBlock(state.ranks);
  }
  if (state.abilityId !== "") {
    individual.abilityId = state.abilityId;
  }
  if (state.itemId !== "") {
    individual.itemId = state.itemId;
  }
  return individual;
}

/** DefenderCandidate(候補)。Individual と同じ欄 + その候補が使う技(前後の空白を落とす)。 */
function buildDefender(state: IndividualFormState): Schemas["DefenderCandidate"] {
  return { ...buildIndividual(state), moveId: state.moveId.trim() };
}

interface BuildRequestInput {
  readonly format: Schemas["Format"];
  readonly attacker: IndividualFormState;
  readonly defenders: readonly IndividualFormState[];
  readonly speedField?: Schemas["SpeedField"];
}

/** request 全体の組み立て(ADR-0705 §6)。field は JD5 では送らない(却下した案)。 */
function buildRequest(input: BuildRequestInput): Schemas["OutspeedAndKoRequest"] {
  const request: Schemas["OutspeedAndKoRequest"] = {
    format: input.format,
    attacker: buildIndividual(input.attacker),
    defenders: input.defenders.map(buildDefender),
    moveId: input.attacker.moveId.trim(),
  };
  if (input.speedField !== undefined) {
    request.speedField = input.speedField;
  }
  return request;
}

/**
 * 送信前の検査(ADR-0705 §7・受け入れ条件7)。契約の範囲と同じ検査を、呼ぶ前にクライアント側で行う。
 * 違反していれば理由を返す(null は「送ってよい」)。
 */
function validationMessage(
  attacker: IndividualFormState,
  defenders: readonly IndividualFormState[],
): string | null {
  const all = [attacker, ...defenders];
  for (const individual of all) {
    if (
      individual.speciesKey.trim() === "" ||
      individual.natureId.trim() === "" ||
      individual.moveId.trim() === ""
    ) {
      return judgeScreenText.requiredMessage;
    }
  }
  for (const individual of all) {
    for (const stat of SP_STATS) {
      const value = individual.sp[stat];
      if (value < 0 || value > MAX_SP_PER_STAT) {
        return judgeScreenText.spRangeMessage(MAX_SP_PER_STAT);
      }
    }
    const total = SP_STATS.reduce((sum, stat) => sum + individual.sp[stat], 0);
    if (total > MAX_SP_TOTAL) {
      return judgeScreenText.spTotalMessage(MAX_SP_TOTAL);
    }
  }
  for (const individual of all) {
    for (const stat of RANK_STATS) {
      const value = parseRank(individual.ranks[stat]);
      if (Number.isNaN(value) || value < MIN_RANK || value > MAX_RANK) {
        return judgeScreenText.rankRangeMessage;
      }
    }
  }
  return null;
}

/** 送信1回の状態(判別 union)。idle は「判定する」をまだ押していない。 */
type SubmitState =
  | { readonly status: "idle" }
  | { readonly status: "validationError"; readonly message: string }
  | { readonly status: "loading" }
  | {
      readonly status: "success";
      readonly value: Schemas["OutspeedAndKoResponse"];
      /** 送信時点の候補(結果の行に種族名を添えるため。応答の届く前に入力が変わっても使わない)。 */
      readonly defenders: readonly IndividualFormState[];
    }
  | { readonly status: "error"; readonly error: { readonly code: string; readonly message: string } };

/** 画面に出すエラー(見出し + 補助のサーバーの message。ADR-0705 §8)。 */
interface DisplayError {
  readonly heading: string;
  /** サーバーの message が見出しと違うときだけ、補助の行として持つ(同じ文言は二重に出さない)。 */
  readonly detail: string | null;
}

const JUDGE_ERROR_HEADINGS: Readonly<Record<string, string>> = judgeErrorText;

function displayError(state: SubmitState): DisplayError | null {
  if (state.status === "validationError") {
    return { heading: state.message, detail: null };
  }
  if (state.status !== "error") {
    return null;
  }
  const heading = JUDGE_ERROR_HEADINGS[state.error.code];
  if (heading === undefined) {
    return { heading: state.error.message, detail: null };
  }
  return { heading, detail: heading === state.error.message ? null : state.error.message };
}

function koText(ko: Schemas["KOChance"]): string {
  if (ko.hits === 0) {
    return judgeScreenText.koNone;
  }
  return ko.guaranteed
    ? judgeScreenText.koGuaranteed(ko.hits)
    : judgeScreenText.koRandom(ko.hits, ko.displayChancePercent);
}

/** 素早さの比較(同速は outspeeds の false と区別する。ADR-0700 §6-1)。 */
function speedComparisonLabel(matchup: Schemas["Matchup"]): string {
  if (matchup.speedTie) {
    return judgeScreenText.speedTieLabel;
  }
  return matchup.outspeeds ? judgeScreenText.outspeedsTrueLabel : judgeScreenText.outspeedsFalseLabel;
}

/** 行動順(決まらないときは断定しない。ADR-0704 §2)。 */
function turnOrderLabel(matchup: Schemas["Matchup"]): string {
  if (matchup.turnOrderTie) {
    return judgeScreenText.turnOrderTieLabel;
  }
  return matchup.attackerMovesFirst
    ? judgeScreenText.attackerMovesFirstLabel
    : judgeScreenText.defenderMovesFirstLabel;
}

/** 判定の画面(ADR-0705 §4)。 */
export function JudgeScreen({ judgeClient, master, masterSearch }: JudgeScreenProps) {
  const capabilities = masterCapabilities(master);

  const [format, setFormat] = useState<Schemas["Format"]>("single");
  const [attacker, setAttacker] = useState<IndividualFormState>(emptyIndividual);
  const [candidates, setCandidates] = useState<CandidateState[]>(() => [{ ...emptyIndividual(), id: 0 }]);
  const [trickRoom, setTrickRoom] = useState(false);
  const [attackerTailwind, setAttackerTailwind] = useState(false);
  const [defenderTailwind, setDefenderTailwind] = useState(false);
  const [submitState, setSubmitState] = useState<SubmitState>({ status: "idle" });

  // 送信ごとの連番(古い応答を無視する。ADR-0705 §7・受け入れ条件8)。
  const submitSeqRef = useRef(0);
  // 候補の React key 発行用(削除で index がずれても入力を取り違えない。BalanceScreen.tsx と同じ作法)。
  const nextCandidateIdRef = useRef(1);

  function updateAttacker(patch: Partial<IndividualFormState>): void {
    setAttacker((current) => ({ ...current, ...patch }));
  }

  function updateCandidate(index: number, patch: Partial<IndividualFormState>): void {
    setCandidates((current) => current.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
  }

  function addCandidate(): void {
    setCandidates((current) => {
      if (current.length >= MAX_DEFENDERS) {
        return current;
      }
      const id = nextCandidateIdRef.current;
      nextCandidateIdRef.current += 1;
      return [...current, { ...emptyIndividual(), id }];
    });
  }

  function removeCandidate(index: number): void {
    setCandidates((current) => (current.length <= 1 ? current : current.filter((_, i) => i !== index)));
  }

  async function handleSubmit(): Promise<void> {
    const message = validationMessage(attacker, candidates);
    if (message !== null) {
      setSubmitState({ status: "validationError", message });
      return;
    }
    const speedFieldActive = trickRoom || attackerTailwind || defenderTailwind;
    const request = buildRequest({
      format,
      attacker,
      defenders: candidates,
      speedField: speedFieldActive ? { trickRoom, attackerTailwind, defenderTailwind } : undefined,
    });

    const seq = submitSeqRef.current + 1;
    submitSeqRef.current = seq;
    setSubmitState({ status: "loading" });

    const result = await judgeClient.outspeedAndKo(request);
    // 古い応答は無視する(このあとで別の送信が始まっていたら seq が進んでいる)。
    if (submitSeqRef.current !== seq) {
      return;
    }
    setSubmitState(
      result.ok
        ? { status: "success", value: result.value, defenders: candidates }
        : { status: "error", error: result.error },
    );
  }

  const loading = submitState.status === "loading";
  const error = displayError(submitState);

  return (
    <div className="judge-screen">
      <section aria-label={judgeScreenText.attackerRegionLabel} className="judge-screen__region">
        <label className="judge-individual__field">
          <span>{judgeScreenText.formatLabel}</span>
          <select
            aria-label={judgeScreenText.formatLabel}
            value={format}
            onChange={(event) => {
              setFormat(event.target.value as Schemas["Format"]);
            }}
          >
            <option value="single">{judgeScreenText.formatOption.single}</option>
            <option value="double">{judgeScreenText.formatOption.double}</option>
          </select>
        </label>

        <IndividualFields
          value={attacker}
          onChange={updateAttacker}
          master={master}
          speciesListAvailable={capabilities.speciesList}
          masterSearch={masterSearch}
        />

        <fieldset className="judge-screen__speed-field">
          <legend>{judgeScreenText.speedFieldGroupLabel}</legend>
          <label className="judge-individual__field">
            <input
              type="checkbox"
              checked={trickRoom}
              onChange={(event) => {
                setTrickRoom(event.target.checked);
              }}
            />
            {judgeScreenText.trickRoomLabel}
          </label>
          <label className="judge-individual__field">
            <input
              type="checkbox"
              checked={attackerTailwind}
              onChange={(event) => {
                setAttackerTailwind(event.target.checked);
              }}
            />
            {judgeScreenText.attackerTailwindLabel}
          </label>
          <label className="judge-individual__field">
            <input
              type="checkbox"
              checked={defenderTailwind}
              onChange={(event) => {
                setDefenderTailwind(event.target.checked);
              }}
            />
            {judgeScreenText.defenderTailwindLabel}
          </label>
          <p className="judge-screen__notice">{judgeScreenText.defenderTailwindNotice}</p>
        </fieldset>
      </section>

      <section aria-label={judgeScreenText.defendersRegionLabel} className="judge-screen__region">
        {candidates.map((candidate, index) => (
          <fieldset key={candidate.id} className="judge-screen__candidate">
            <legend>{judgeScreenText.candidateGroupLabel(index + 1)}</legend>
            <IndividualFields
              value={candidate}
              onChange={(patch) => {
                updateCandidate(index, patch);
              }}
              master={master}
              speciesListAvailable={capabilities.speciesList}
              masterSearch={masterSearch}
            />
            <button
              type="button"
              className="judge-screen__remove"
              disabled={candidates.length <= 1}
              onClick={() => {
                removeCandidate(index);
              }}
            >
              {judgeScreenText.removeCandidateLabel(index + 1)}
            </button>
          </fieldset>
        ))}
        <button type="button" onClick={addCandidate} disabled={candidates.length >= MAX_DEFENDERS}>
          {judgeScreenText.addCandidateLabel}
        </button>
        {candidates.length >= MAX_DEFENDERS && (
          <p className="judge-screen__notice">{judgeScreenText.maxCandidatesNotice(MAX_DEFENDERS)}</p>
        )}
      </section>

      {error !== null && (
        <p role="alert" className="judge-screen__error">
          {error.heading}
          {error.detail !== null && <span className="judge-screen__error-detail"> {error.detail}</span>}
        </p>
      )}

      <button
        type="button"
        className="judge-screen__submit"
        disabled={loading}
        onClick={() => {
          void handleSubmit();
        }}
      >
        {judgeScreenText.submitLabel}
      </button>

      <section aria-label={judgeScreenText.resultRegionLabel} className="judge-screen__region">
        {submitState.status === "success" ? (
          <ResultList defenders={submitState.defenders} matchups={submitState.value.matchups} />
        ) : submitState.status === "loading" ? (
          <p className="judge-screen__notice">{judgeScreenText.loadingNotice}</p>
        ) : (
          <p className="judge-screen__notice">{judgeScreenText.emptyResultNotice}</p>
        )}
      </section>
    </div>
  );
}

interface IndividualFieldsProps {
  readonly value: IndividualFormState;
  readonly onChange: (patch: Partial<IndividualFormState>) => void;
  readonly master: MasterData;
  /** capabilities.speciesList(ADR-0304 §1)。false ならドロップダウンの代わりに検索欄を出す。 */
  readonly speciesListAvailable: boolean;
  readonly masterSearch?: MasterSpeciesSearch;
}

/**
 * 1体分の入力(種族・性格・SP・ランク・特性・持ち物・技 ID)。自分側・候補側の両方で使う共通部品
 * (ADR-0705 §9: CalcScreen の個体編集フォームは使い回さず、判定に要る欄だけをここに作る)。
 */
function IndividualFields({
  value,
  onChange,
  master,
  speciesListAvailable,
  masterSearch,
}: IndividualFieldsProps) {
  return (
    <div className="judge-individual">
      {speciesListAvailable ? (
        <label className="judge-individual__field">
          <span>{judgeScreenText.speciesLabel}</span>
          <select
            aria-label={judgeScreenText.speciesLabel}
            value={value.speciesKey}
            onChange={(event) => {
              const speciesKey = event.target.value;
              const species = master.species.find((candidate) => candidate.key === speciesKey);
              onChange({ speciesKey, speciesName: species?.nameJa ?? "" });
            }}
          >
            <option value="" hidden />
            {master.species.map((species) => (
              <option key={species.key} value={species.key}>
                {species.nameJa}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <SpeciesSearchField
          label={judgeScreenText.speciesLabel}
          masterSearch={masterSearch}
          onResolved={(resolution) => {
            onChange({ speciesKey: resolution.species.key, speciesName: resolution.species.nameJa });
          }}
        />
      )}

      <label className="judge-individual__field">
        <span>{judgeScreenText.natureLabel}</span>
        <select
          aria-label={judgeScreenText.natureLabel}
          value={value.natureId}
          onChange={(event) => {
            onChange({ natureId: event.target.value });
          }}
        >
          <option value="" hidden />
          {master.natures.map((nature) => (
            <option key={nature.id} value={nature.id}>
              {nature.nameJa}
            </option>
          ))}
        </select>
      </label>

      <label className="judge-individual__field">
        <span>{judgeScreenText.abilityLabel}</span>
        <select
          aria-label={judgeScreenText.abilityLabel}
          value={value.abilityId}
          onChange={(event) => {
            onChange({ abilityId: event.target.value });
          }}
        >
          <option value="">{judgeScreenText.unselectedOption}</option>
          {master.abilities.map((ability) => (
            <option key={ability.id} value={ability.id}>
              {ability.nameJa}
            </option>
          ))}
        </select>
      </label>

      <label className="judge-individual__field">
        <span>{judgeScreenText.itemLabel}</span>
        <select
          aria-label={judgeScreenText.itemLabel}
          value={value.itemId}
          onChange={(event) => {
            onChange({ itemId: event.target.value });
          }}
        >
          <option value="">{judgeScreenText.unselectedOption}</option>
          {master.items.map((item) => (
            <option key={item.id} value={item.id}>
              {item.nameJa}
            </option>
          ))}
        </select>
      </label>

      {SP_STATS.map((stat) => (
        <label key={stat} className="judge-individual__field">
          <span>{judgeScreenText.spLabel(stat)}</span>
          <input
            type="number"
            aria-label={judgeScreenText.spLabel(stat)}
            min={0}
            max={MAX_SP_PER_STAT}
            value={value.sp[stat]}
            onChange={(event) => {
              const parsed = parseIntFieldChange(event.target.value);
              if (parsed !== null) {
                onChange({ sp: { ...value.sp, [stat]: parsed } });
              }
            }}
          />
        </label>
      ))}

      {RANK_STATS.map((stat) => (
        <label key={stat} className="judge-individual__field">
          <span>{judgeScreenText.rankLabel(stat)}</span>
          {/*
           * type="number" だと jsdom は "-" 単独(負数を打っている途中)を無効値として即座に ""
           * へ戻してしまい、キー入力を1文字ずつ追う実際の操作で負数を打てない。ランクは -6..+6 を
           * 送るため text + inputMode="numeric" にし、入力の生の文字列をそのまま state に持つ
           * (数への変換・範囲検査は validationMessage / parseRank で行う)。
           */}
          <input
            type="text"
            inputMode="numeric"
            pattern="-?[0-9]*"
            aria-label={judgeScreenText.rankLabel(stat)}
            value={value.ranks[stat]}
            onChange={(event) => {
              onChange({ ranks: { ...value.ranks, [stat]: event.target.value } });
            }}
          />
        </label>
      ))}

      <label className="judge-individual__field">
        <span>{judgeScreenText.moveIdLabel}</span>
        <input
          type="text"
          aria-label={judgeScreenText.moveIdLabel}
          value={value.moveId}
          onChange={(event) => {
            onChange({ moveId: event.target.value });
          }}
        />
      </label>
      <p className="judge-individual__hint">{judgeScreenText.moveIdHint}</p>
    </div>
  );
}

interface ResultListProps {
  /** 送信時点の候補(defenders と同じ順)。judge は種族名を返さないので、ここから引く。ADR-0705 §8。 */
  readonly defenders: readonly IndividualFormState[];
  readonly matchups: readonly Schemas["Matchup"][];
}

/**
 * 判定結果の一覧(defenders と同じ順・同じ件数で出す。ADR-0705 §8)。行は defenderIndex で対応づけ、
 * 応答の並びは信用しない(取り違えを検出するテストがある。ADR-0703 §2)。
 */
function ResultList({ defenders, matchups }: ResultListProps) {
  const byDefenderIndex = new Map(matchups.map((matchup) => [matchup.defenderIndex, matchup]));
  return (
    <ul className="judge-screen__matchups">
      {defenders.map((defender, index) => {
        const matchup = byDefenderIndex.get(index);
        if (matchup === undefined) {
          return null;
        }
        return (
          <li
            key={index}
            className="judge-screen__matchup"
            data-testid="judge-matchup"
            data-defender-index={String(matchup.defenderIndex)}
          >
            <p className="judge-screen__matchup-species">{defender.speciesName || defender.speciesKey}</p>
            <p>{judgeScreenText.speedLabel(matchup.attackerSpeed, matchup.defenderSpeed)}</p>
            <p>{speedComparisonLabel(matchup)}</p>
            <p>{judgeScreenText.priorityLabel(matchup.attackerMovePriority, matchup.defenderMovePriority)}</p>
            <p>{turnOrderLabel(matchup)}</p>
            <p>
              {judgeScreenText.attackerKoLabel} {koText(matchup.attackerKo)}
            </p>
            <p>
              {judgeScreenText.defenderKoLabel} {koText(matchup.defenderKo)}
            </p>
          </li>
        );
      })}
    </ul>
  );
}
