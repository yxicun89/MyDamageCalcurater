// JD5: 判定の画面(ADR-0705 §4〜§8、docs/judge-design.md §3 JD5)。
// 自分のポケモン1体(使う技込み)と相手候補1〜6件を入力し、judge-svc の
// POST /api/judge/v1/outspeed-and-ko を「判定する」で1回だけ呼ぶ(ADR-0705 §7)。
// 判定(素早さ・行動順・双方向の確定数)は judge-svc が決める。Web は engine(WASM)で計算し直さず、
// 「勝ち / 負け」にも丸めない(ADR-0700 §6-1・ADR-0704 §3 の立場を画面でも保つ)。
// master / masterSearch は**入力補助にだけ**使う(種族・性格・特性・持ち物の選択肢。ADR-0705 §5)。
// 技は選んだポケモンの覚える技(learnset)から選び、調整はプリセットで入れる。SP6欄・ランク5欄は「詳細」に畳む(issue 309・ADR-0711)。

import { useEffect, useId, useRef, useState } from "react";
import type { Move, Ranks } from "../engine/types";
import { unsupportedMarkName } from "../domain/unsupportedLabels";
import { formatMoveCategory } from "../domain/format";
import {
  JUDGE_ITEM_ROLE_FILTER,
  itemsForRole,
  itemsWithStoneLabels,
  megaStoneLabel,
} from "../domain/itemRoles";
import { megaItemLock, megaStoneItemIds } from "../domain/mega";
import { MAX_SP_PER_STAT } from "../domain/requests";
import { calcScreenText, judgeErrorText, judgeScreenText, unsupportedText } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import { MegaItemReason } from "../screens/MegaItemReason";
import { SpeciesSearchField } from "../screens/SpeciesSearchField";
import "./JudgeScreen.css";
import {
  applyPreset,
  chooseMove,
  chooseSpecies,
  editNature,
  editSp,
  emptyIndividual,
  type IndividualFormState,
  JUDGE_PRESET_KEYS,
  JUDGE_STATUS_KEYS,
  type JudgeStatusKey,
  judgePresetLabel,
  moveCategoryOf,
  parseRank,
  RANK_STATS,
  type RankKey,
  SP_STATS,
} from "./individualForm";
import type { components } from "./judge.gen";
import type { JudgeClient } from "./judgeClient";
import { type BodyErrors, hasDetailsError, validateBodies } from "./judgeValidation";

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

/** 相手候補1件(React の key 用の id を持つ。BalanceScreen.tsx の MemberState と同じ考え方)。 */
interface CandidateState extends IndividualFormState {
  readonly id: number;
}

/** 検証エラーの置き場の識別子(自分 / 候補の id)。 */
const ATTACKER_KEY = "attacker";
function candidateKey(id: number): string {
  return `candidate-${String(id)}`;
}

/** 検証エラー無し(欄ごとの誤りの置き場。体の識別子 → 誤り)。 */
const NO_FIELD_ERRORS: ReadonlyMap<string, BodyErrors> = new Map();

/** 検査を通った(= 全欄が -6..+6 の整数の)ランクだけを渡す前提。 */
function isZeroRanks(ranks: Record<RankKey, string>): boolean {
  return RANK_STATS.every((stat) => parseRank(ranks[stat]) === 0);
}

/** 検査を通ったランクを数の RankBlock にする(検査が範囲・整数であることを保証済み)。 */
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
 * SP の入力欄の onChange から整数を読む。空・数でなければ 0 にする(欄を空にしたら 0 に戻る。
 * SP は 0 以上なので "-" 単独の問題が無く、type="number" のまま使える)。
 */
function parseSpFieldChange(raw: string): number {
  const parsed = Number.parseInt(raw, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
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
  if (state.status !== "none") {
    individual.status = state.status;
  }
  return individual;
}

/** DefenderCandidate(候補)。Individual と同じ欄 + その候補が使う技(前後の空白を落とす)。 */
function buildDefender(state: IndividualFormState): Schemas["DefenderCandidate"] {
  return { ...buildIndividual(state), moveId: state.moveId };
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
    moveId: input.attacker.moveId,
  };
  if (input.speedField !== undefined) {
    request.speedField = input.speedField;
  }
  return request;
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
      /** 送信時点の自分側の技(印の技名の解決用。結果のあとに種族を変えても名前が変わらないように)。 */
      readonly attackerMoves: readonly Move[];
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

/** 素早さに反映した補正・反映していない入力の行(空の欄は出さない。ADR-0710)。 */
function speedNotes(matchup: Schemas["Matchup"]): string[] {
  const t = judgeScreenText;
  const sides = [
    { side: t.speedSideSelf, applied: matchup.attackerSpeedApplied, ignored: matchup.attackerSpeedIgnored },
    {
      side: t.speedSideOpponent,
      applied: matchup.defenderSpeedApplied,
      ignored: matchup.defenderSpeedIgnored,
    },
  ];
  const notes: string[] = [];
  for (const { side, applied, ignored } of sides) {
    if (applied.length > 0) {
      notes.push(
        t.speedAppliedNote(
          side,
          applied.map((name) => t.speedFactorLabel[name]),
        ),
      );
    }
    if (ignored.length > 0) {
      notes.push(
        t.speedIgnoredNote(
          side,
          ignored.map((name) => t.speedIgnoredLabel[name]),
        ),
      );
    }
  }
  return notes;
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
  // 送信時の検証エラー(欄ごと)。送信のたびに作り直す。
  const [fieldErrors, setFieldErrors] = useState<ReadonlyMap<string, BodyErrors>>(NO_FIELD_ERRORS);

  // 送信ごとの連番(古い応答を無視する。ADR-0705 §7・受け入れ条件8)。
  const submitSeqRef = useRef(0);
  // 候補の React key 発行用(削除で index がずれても入力を取り違えない。BalanceScreen.tsx と同じ作法)。
  const nextCandidateIdRef = useRef(1);

  function updateAttacker(update: IndividualUpdate): void {
    setAttacker(update);
  }

  function updateCandidate(index: number, update: IndividualUpdate): void {
    setCandidates((current) =>
      current.map((entry, i) => (i === index ? { ...update(entry), id: entry.id } : entry)),
    );
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
    // 誤りの文言の「相手候補n」が番号のずれで古くなるので、いったん消す(次の送信で作り直す)。
    setFieldErrors(NO_FIELD_ERRORS);
  }

  async function handleSubmit(): Promise<void> {
    const validation = validateBodies([
      { key: ATTACKER_KEY, who: judgeScreenText.attackerWhoLabel, state: attacker },
      ...candidates.map((candidate, index) => ({
        key: candidateKey(candidate.id),
        who: judgeScreenText.candidateGroupLabel(index + 1),
        state: candidate,
      })),
    ]);
    setFieldErrors(validation.errors);
    if (validation.firstMessage !== null) {
      setSubmitState({ status: "validationError", message: validation.firstMessage });
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
        ? { status: "success", value: result.value, defenders: candidates, attackerMoves: attacker.moves }
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
          errors={fieldErrors.get(ATTACKER_KEY)}
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
              onChange={(update) => {
                updateCandidate(index, update);
              }}
              errors={fieldErrors.get(candidateKey(candidate.id))}
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
          <ResultList
            defenders={submitState.defenders}
            matchups={submitState.value.matchups}
            master={master}
            attackerMoves={submitState.attackerMoves}
          />
        ) : submitState.status === "loading" ? (
          <p className="judge-screen__notice">{judgeScreenText.loadingNotice}</p>
        ) : (
          <p className="judge-screen__notice">{judgeScreenText.emptyResultNotice}</p>
        )}
      </section>
    </div>
  );
}

/** 1体分の入力を、いまの値から次の値へ更新する関数(非同期の解決結果が古い値を上書きしないよう、値ではなく関数で渡す)。 */
type IndividualUpdate = (current: IndividualFormState) => IndividualFormState;

interface IndividualFieldsProps {
  readonly value: IndividualFormState;
  readonly onChange: (update: IndividualUpdate) => void;
  /** 送信時の検証エラー(この体の欄ごと)。無ければ undefined。 */
  readonly errors: BodyErrors | undefined;
  readonly master: MasterData;
  /** capabilities.speciesList(ADR-0304 §1)。false ならドロップダウンの代わりに検索欄を出す。 */
  readonly speciesListAvailable: boolean;
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 技セレクタの option の表示(「技名・分類・威力n」。変化技は威力を出さない。計算画面と同じ書式)。 */
function moveOptionText(move: Move): string {
  const separator = calcScreenText.moveOptionSeparator;
  const power =
    move.category === "status" ? "" : `${separator}${calcScreenText.movePowerLabel}${String(move.power)}`;
  return `${move.nameJa}${separator}${formatMoveCategory(move.category)}${power}`;
}

/** 誤りの文言の段落(欄の aria-describedby が指す先)。 */
function ErrorText({ id, message }: { readonly id: string; readonly message: string | null }) {
  if (message === null) {
    return null;
  }
  return (
    <p id={id} className="judge-individual__error">
      {message}
    </p>
  );
}

/**
 * 1体分の入力(種族・性格・特性・持ち物・技・調整プリセット。SP6欄・ランク5欄は「詳細」)。自分側・候補側の
 * 両方で使う共通部品(ADR-0705 §9: CalcScreen の個体編集フォームは使い回さず、判定に要る欄だけをここに作る)。
 */
function IndividualFields({
  value,
  onChange,
  errors,
  master,
  speciesListAvailable,
  masterSearch,
}: IndividualFieldsProps) {
  const uid = useId();
  // issue 515・ADR-0320: メガ種族の持ち物はメガストーンに固定する(固定は選んだ種族から毎回導く)。
  // メガストーンは単独の選択肢に出さない(判別集合は、全件の一覧 + この体で選んだ種族から導く)。
  const itemLock = megaItemLock(value.species, master.items);
  const pickableItems = itemsForRole(
    master.items,
    JUDGE_ITEM_ROLE_FILTER,
    megaStoneItemIds(value.species === null ? master.species : [...master.species, value.species]),
  );
  const itemReasonId = `${uid}-item-reason`;
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const natures = master.natures;
  const category = moveCategoryOf(value);
  const speciesErrorId = `${uid}-species-error`;
  const natureErrorId = `${uid}-nature-error`;
  const moveErrorId = `${uid}-move-error`;
  const spRangeErrorId = `${uid}-sp-range-error`;
  const spTotalErrorId = `${uid}-sp-total-error`;
  const rankErrorId = `${uid}-rank-error`;

  // 誤りが「詳細」の中の欄なら、送信のたびに開いて見えるようにする(開閉は体ごとに独立)。
  useEffect(() => {
    if (errors !== undefined && hasDetailsError(errors) && detailsRef.current !== null) {
      detailsRef.current.open = true;
    }
  }, [errors]);

  function describedBy(...ids: (string | false)[]): string | undefined {
    const present = ids.filter((id): id is string => id !== false);
    return present.length > 0 ? present.join(" ") : undefined;
  }

  return (
    <div className="judge-individual">
      {speciesListAvailable ? (
        <label className="judge-individual__field">
          <span>{judgeScreenText.speciesLabel}</span>
          <select
            aria-label={judgeScreenText.speciesLabel}
            aria-invalid={errors?.species != null}
            aria-describedby={describedBy(errors?.species != null && speciesErrorId)}
            value={value.speciesKey}
            onChange={(event) => {
              const species = master.species.find((candidate) => candidate.key === event.target.value);
              if (species !== undefined) {
                onChange((current) => chooseSpecies(current, species, master.moves, natures, master.items));
              }
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
            // 検索で解決した種族の技(resolveSpecies が返す learnset の実体)から選ぶ。
            onChange((current) =>
              chooseSpecies(
                current,
                resolution.species,
                [...master.moves, ...resolution.moves],
                natures,
                master.items,
              ),
            );
          }}
        />
      )}
      <ErrorText id={speciesErrorId} message={errors?.species ?? null} />

      <label className="judge-individual__field">
        <span>{judgeScreenText.natureLabel}</span>
        <select
          aria-label={judgeScreenText.natureLabel}
          aria-invalid={errors?.nature != null}
          aria-describedby={describedBy(errors?.nature != null && natureErrorId)}
          value={value.natureId}
          onChange={(event) => {
            const natureId = event.target.value;
            onChange((current) => editNature(current, natureId, natures));
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
      <ErrorText id={natureErrorId} message={errors?.nature ?? null} />

      <label className="judge-individual__field">
        <span>{judgeScreenText.abilityLabel}</span>
        <select
          aria-label={judgeScreenText.abilityLabel}
          value={value.abilityId}
          onChange={(event) => {
            const abilityId = event.target.value;
            onChange((current) => ({ ...current, abilityId }));
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
          value={
            itemLock.kind === "locked" ? itemLock.item.id : itemLock.kind === "missing" ? "" : value.itemId
          }
          disabled={itemLock.kind !== "none"}
          aria-describedby={itemLock.kind === "none" ? undefined : itemReasonId}
          onChange={(event) => {
            const itemId = event.target.value;
            onChange((current) => ({ ...current, itemId }));
          }}
        >
          <option value="">{judgeScreenText.unselectedOption}</option>
          {itemLock.kind === "locked" && value.species !== null ? (
            <option value={itemLock.item.id}>{megaStoneLabel(value.species)}</option>
          ) : (
            pickableItems.map((item) => (
              <option key={item.id} value={item.id}>
                {item.nameJa}
              </option>
            ))
          )}
        </select>
      </label>
      <MegaItemReason id={itemReasonId} lock={itemLock} className="judge-individual__reason" />

      <label className="judge-individual__field">
        <span>{judgeScreenText.statusLabel}</span>
        <select
          aria-label={judgeScreenText.statusLabel}
          value={value.status}
          onChange={(event) => {
            const status = event.target.value as JudgeStatusKey;
            onChange((current) => ({ ...current, status }));
          }}
        >
          {JUDGE_STATUS_KEYS.map((key) => (
            <option key={key} value={key}>
              {judgeScreenText.statusOptionLabel[key]}
            </option>
          ))}
        </select>
      </label>

      <label className="judge-individual__field">
        <span>{judgeScreenText.moveLabel}</span>
        <select
          aria-label={judgeScreenText.moveLabel}
          aria-invalid={errors?.move != null}
          aria-describedby={describedBy(errors?.move != null && moveErrorId)}
          disabled={value.moves.length === 0}
          value={value.moveId}
          onChange={(event) => {
            const moveId = event.target.value;
            onChange((current) => chooseMove(current, moveId, natures));
          }}
        >
          {value.moves.map((move) => (
            <option key={move.id} value={move.id}>
              {moveOptionText(move)}
            </option>
          ))}
        </select>
      </label>
      {value.speciesKey !== "" && value.moves.length === 0 && (
        <p className="judge-individual__hint">{judgeScreenText.moveUnavailableNotice}</p>
      )}
      <ErrorText id={moveErrorId} message={errors?.move ?? null} />

      <div
        role="radiogroup"
        aria-label={judgeScreenText.presetGroupLabel}
        className="judge-individual__presets"
      >
        {JUDGE_PRESET_KEYS.map((key) => (
          <label key={key} className="judge-individual__field">
            <input
              type="radio"
              name={`${uid}-preset`}
              checked={value.presetKey === key}
              onChange={() => {
                onChange((current) => applyPreset(current, key, natures));
              }}
            />
            {judgePresetLabel(key, category)}
          </label>
        ))}
      </div>

      {/* SP6欄とランク5欄だけを畳む。閉じても値は state に残り、送信に使う。 */}
      {/* details の暗黙の group ロールは外す(相手候補の fieldset の group と数えが混ざらないように) */}
      <details ref={detailsRef} role="none" className="judge-individual__details">
        <summary>{judgeScreenText.detailsSummaryLabel}</summary>
        <div className="judge-individual">
          {SP_STATS.map((stat) => {
            const rangeInvalid = errors?.spRangeStats.has(stat) === true;
            const totalInvalid = errors?.spTotal != null;
            return (
              <label key={stat} className="judge-individual__field">
                <span>{judgeScreenText.spLabel(stat)}</span>
                <input
                  type="number"
                  aria-label={judgeScreenText.spLabel(stat)}
                  aria-invalid={rangeInvalid || totalInvalid}
                  aria-describedby={describedBy(
                    rangeInvalid && spRangeErrorId,
                    totalInvalid && spTotalErrorId,
                  )}
                  min={0}
                  max={MAX_SP_PER_STAT}
                  value={value.sp[stat]}
                  onChange={(event) => {
                    const parsed = parseSpFieldChange(event.target.value);
                    onChange((current) => editSp(current, { ...current.sp, [stat]: parsed }, natures));
                  }}
                />
              </label>
            );
          })}
          <ErrorText id={spRangeErrorId} message={errors?.spRange ?? null} />
          <ErrorText id={spTotalErrorId} message={errors?.spTotal ?? null} />

          {RANK_STATS.map((stat) => {
            const invalid = errors?.rankStats.has(stat) === true;
            return (
              <label key={stat} className="judge-individual__field">
                <span>{judgeScreenText.rankLabel(stat)}</span>
                {/*
                 * type="number" だと jsdom は "-" 単独(負数を打っている途中)を無効値として即座に ""
                 * へ戻してしまい、キー入力を1文字ずつ追う実際の操作で負数を打てない。ランクは -6..+6 を
                 * 送るため text + inputMode="numeric" にし、入力の生の文字列をそのまま state に持つ
                 * (数への変換・範囲検査は parseRank / judgeValidation.ts で行う)。
                 */}
                <input
                  type="text"
                  inputMode="numeric"
                  pattern="-?[0-9]*"
                  aria-label={judgeScreenText.rankLabel(stat)}
                  aria-invalid={invalid}
                  aria-describedby={describedBy(invalid && rankErrorId)}
                  value={value.ranks[stat]}
                  onChange={(event) => {
                    const raw = event.target.value;
                    onChange((current) => ({ ...current, ranks: { ...current.ranks, [stat]: raw } }));
                  }}
                />
              </label>
            );
          })}
          <ErrorText id={rankErrorId} message={errors?.rankRange ?? null} />
        </div>
      </details>
    </div>
  );
}

interface ResultListProps {
  /** 送信時点の候補(defenders と同じ順)。judge は種族名を返さないので、ここから引く。ADR-0705 §8。 */
  readonly defenders: readonly IndividualFormState[];
  readonly matchups: readonly Schemas["Matchup"][];
  /** 印の ID を表示名に解決するマスタ(issue 271)。 */
  readonly master: MasterData;
  /** 自分側の技(種族検索で解決した learnset の実体。マスタに無い技の名前解決用)。 */
  readonly attackerMoves: readonly Move[];
}

type KoDirection = "attackerKo" | "defenderKo";

/** 確定数の直下に出す「未対応」の注意文(issue 271。印が空なら null)。 */
function koUnsupportedNote(
  direction: KoDirection,
  marks: readonly Schemas["UnsupportedMark"][],
  moves: readonly Move[],
  master: MasterData,
): string | null {
  if (marks.length === 0) {
    return null;
  }
  // メガストーンの英語名を出さない(ADR-0326 §4)。
  const items = itemsWithStoneLabels(master.items, master.species, megaStoneItemIds(master.species));
  const labels = marks.map((mark) => {
    const name = unsupportedMarkName(mark, moves, items, master.abilities);
    const reason =
      mark.reason === "unsupported_effect"
        ? ""
        : `(${Object.hasOwn(unsupportedText.reason, mark.reason) ? unsupportedText.reason[mark.reason as keyof typeof unsupportedText.reason] : unsupportedText.unknownReason})`;
    return `${judgeScreenText.koUnsupportedTargetLabel(direction, mark.target)}「${name === "" ? mark.id : name}」${reason}`;
  });
  return judgeScreenText.koUnsupportedNote(direction, labels);
}

/**
 * 判定結果の一覧(defenders と同じ順・同じ件数で出す。ADR-0705 §8)。行は defenderIndex で対応づけ、
 * 応答の並びは信用しない(取り違えを検出するテストがある。ADR-0703 §2)。
 */
function ResultList({ defenders, matchups, master, attackerMoves }: ResultListProps) {
  const byDefenderIndex = new Map(matchups.map((matchup) => [matchup.defenderIndex, matchup]));
  return (
    <ul className="judge-screen__matchups">
      {defenders.map((defender, index) => {
        const matchup = byDefenderIndex.get(index);
        if (matchup === undefined) {
          return null;
        }
        const moves = [...master.moves, ...attackerMoves, ...defender.moves];
        const attackerNote = koUnsupportedNote("attackerKo", matchup.attackerKoUnsupported, moves, master);
        const defenderNote = koUnsupportedNote("defenderKo", matchup.defenderKoUnsupported, moves, master);
        return (
          <li
            key={index}
            className="judge-screen__matchup"
            data-testid="judge-matchup"
            data-defender-index={String(matchup.defenderIndex)}
          >
            <p className="judge-screen__matchup-species">{defender.speciesName || defender.speciesKey}</p>
            <p>{judgeScreenText.speedLabel(matchup.attackerSpeed, matchup.defenderSpeed)}</p>
            {speedNotes(matchup).map((note) => (
              <p key={note} className="judge-screen__speed-note">
                {note}
              </p>
            ))}
            <p>{speedComparisonLabel(matchup)}</p>
            <p>{judgeScreenText.priorityLabel(matchup.attackerMovePriority, matchup.defenderMovePriority)}</p>
            <p>{turnOrderLabel(matchup)}</p>
            <p>
              {judgeScreenText.attackerKoLabel} {koText(matchup.attackerKo)}
            </p>
            {attackerNote !== null && (
              <div
                role="note"
                className="judge-screen__ko-unsupported"
                data-testid="judge-unsupported-attacker-ko"
              >
                {attackerNote}
              </div>
            )}
            <p>
              {judgeScreenText.defenderKoLabel} {koText(matchup.defenderKo)}
            </p>
            {defenderNote !== null && (
              <div
                role="note"
                className="judge-screen__ko-unsupported"
                data-testid="judge-unsupported-defender-ko"
              >
                {defenderNote}
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
