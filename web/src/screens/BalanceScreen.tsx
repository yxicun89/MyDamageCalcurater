// P4-12a/b: タイプバランスの画面(docs/adr/0303-web-balance-screen.md、docs/type-balance-design.md §6 TB1〜TB5、
// ADR-0400、ADR-0401)。balance API のクライアント(props の client、ADR-0303 §1)へ、メンバー(最大6体。
// ポケモン・特性・技4つまで)を送り、防御相性(analyze)・攻撃範囲(coverage)を出す(P4-12a)。加えて、
// 仮想敵(threats。最大6体、メンバーと同じ入力)・おすすめタイプ(recommendations)を出す(P4-12b)。
// 倍率・集計・穴の判定は応答をそのまま出し、Web で計算し直さない(ADR-0303 §1・§7)。
// ポケモンを選んだメンバーが1人もいなければ analyze/coverage/recommendations のいずれも呼ばない。
// 攻撃技(変化技でない技)を選んだメンバーが1人もいなければ coverage は呼ばない。
// threats はパーティ1体以上・仮想敵1体以上そろったときだけ呼ぶ(ADR-0303 §7)。
// 4つの呼び出しは互いに独立(1つの遅延・エラーが他の表示を消さない。ADR-0303 §9)。

import { useEffect, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import type { components } from "../api/balance.gen";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import {
  coverageMultiplierLabel,
  defenseMultiplierLabel,
  matchupMultiplierLabel,
  multiplierLabel,
  safeLabel,
  superEffectiveLabel,
} from "../domain/balanceLabels";
import { learnsetMoves } from "../domain/moves";
import { balanceScreenText, masterOnlineText, typeNameJa } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import "./BalanceScreen.css";

type Schemas = components["schemas"];
type AnalyzeMembers = Schemas["AnalyzeRequest"]["members"];
type CoverageMembers = Schemas["CoverageRequest"]["members"];
type ThreatsPokemon = Schemas["ThreatsRequestPokemon"];
type RecommendationsMembers = Schemas["RecommendationsRequest"]["members"];

/** 画面の props(App.tsx が注入する。マスタは既存の画面と同じ MasterData。ADR-0303 §2・§3)。 */
export interface BalanceScreenProps {
  readonly master: MasterData;
  readonly client: BalanceClient;
  /**
   * P4-16b(ADR-0304 A-9・A-10): 種族を都度引く口。この画面は
   * `capabilities.speciesList` と `capabilities.moves` が両方そろうまで使えない(A-9)ので、
   * 今は受け取るだけで使わない。技が戻る P4-17 で、各枠の検索欄に使う。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** 技の枠数(services/balance/api/openapi.yaml の moveIds の maxItems)。 */
const MOVE_SLOTS = 4;

/** メンバーの最大数(services/balance/api/openapi.yaml の members の maxItems)。 */
const MAX_MEMBERS = 6;

/** 画面が持つメンバー1件の状態。moveIds は枠(技1〜技4)の順のまま、空欄は ""。 */
interface MemberState {
  readonly id: number;
  readonly speciesKey: string;
  readonly abilityId: string;
  readonly moveIds: readonly string[];
}

function emptyMember(id: number): MemberState {
  return { id, speciesKey: "", abilityId: "", moveIds: Array.from({ length: MOVE_SLOTS }, () => "") };
}

/** balance 呼び出し1本の状態(判別 union)。idle は送るメンバーが無い(呼んでいない)。 */
type RequestState<T> =
  | { readonly status: "idle" }
  | { readonly status: "loading" }
  | { readonly status: "success"; readonly value: T }
  | { readonly status: "error"; readonly error: { readonly code: string; readonly message: string } };

/** 直近に届いた応答と、それを生んだ入力の key(CalcScreen.tsx の CompletedCalc と同じ考え方)。 */
interface Completed<T> {
  readonly key: string;
  readonly result: BalanceResult<T>;
}

/**
 * key が今の入力(currentKey)と一致する応答が届いていれば成功/失敗、まだなら loading にする
 * (入力から毎レンダー導出する。effect の中で同期的に setState すると react-hooks/set-state-in-effect に
 * 引っかかるため、CalcScreen.tsx と同じく effect は .then の中でだけ setState する)。
 */
function deriveRequestState<T>(currentKey: string, completed: Completed<T> | null): RequestState<T> {
  if (completed === null || completed.key !== currentKey) {
    return { status: "loading" };
  }
  return completed.result.ok
    ? { status: "success", value: completed.result.value }
    : { status: "error", error: completed.result.error };
}

/** 枠の順のまま、空欄と重複した技を除く(CoverageRequestMember.moveIds の uniqueItems)。 */
function dedupeMoveIds(moveIds: readonly string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const moveId of moveIds) {
    if (moveId === "" || seen.has(moveId)) {
      continue;
    }
    seen.add(moveId);
    result.push(moveId);
  }
  return result;
}

function findSpeciesName(master: MasterData, pokemonId: string): string {
  return master.species.find((candidate) => candidate.key === pokemonId)?.nameJa ?? pokemonId;
}

function findAbilityName(master: MasterData, abilityId: string): string {
  return master.abilities.find((candidate) => candidate.id === abilityId)?.nameJa ?? abilityId;
}

/**
 * P4-12b: メンバー1件を threats/recommendations の ID だけの形にする(ThreatsRequestPokemon と
 * RecommendationsRequestMember は同じ形。ADR-0401 §6)。重複した技は1つにし、特性は選んでいるときだけ送る。
 */
function toIdOnlyPokemon(member: MemberState): ThreatsPokemon {
  const moveIds = dedupeMoveIds(member.moveIds);
  return member.abilityId === ""
    ? { pokemonId: member.speciesKey, moveIds }
    : { pokemonId: member.speciesKey, moveIds, abilityId: member.abilityId };
}

/** メンバーの枠(最大6体)を操作する共通のハンドラの形(P4-12b: 自分のパーティと仮想敵で同じ操作を使う)。 */
interface MemberListActions {
  readonly add: () => void;
  readonly remove: (index: number) => void;
  readonly selectSpecies: (master: MasterData, index: number, speciesKey: string) => void;
  readonly selectAbility: (index: number, abilityId: string) => void;
  readonly selectMove: (index: number, slot: number, moveId: string) => void;
}

/**
 * メンバーの枠(最大6体)を操作する共通のハンドラ(P4-12b: 自分のパーティと仮想敵で同じ操作を使う)。
 * ID 発行用の ref はこの中だけで持つ(render 中に ref を外へ渡さない。react-hooks/refs)。
 * 各操作はアロー関数のプロパティにする(オブジェクトのメソッドとして切り離して渡しても this に依存しない。
 * @typescript-eslint/unbound-method)。
 */
function useMemberListActions(setList: Dispatch<SetStateAction<MemberState[]>>): MemberListActions {
  const nextIdRef = useRef(1);
  return {
    add: (): void => {
      setList((current) => {
        if (current.length >= MAX_MEMBERS) {
          return current;
        }
        const id = nextIdRef.current;
        nextIdRef.current += 1;
        return [...current, emptyMember(id)];
      });
    },
    remove: (index: number): void => {
      setList((current) => (current.length <= 1 ? current : current.filter((_, i) => i !== index)));
    },
    selectSpecies: (master: MasterData, index: number, speciesKey: string): void => {
      const species = master.species.find((candidate) => candidate.key === speciesKey);
      setList((current) =>
        current.map((member, i) =>
          i === index
            ? {
                ...member,
                speciesKey,
                abilityId: species?.abilities[0] ?? "",
                moveIds: Array.from({ length: MOVE_SLOTS }, () => ""),
              }
            : member,
        ),
      );
    },
    selectAbility: (index: number, abilityId: string): void => {
      setList((current) => current.map((member, i) => (i === index ? { ...member, abilityId } : member)));
    },
    selectMove: (index: number, slot: number, moveId: string): void => {
      setList((current) =>
        current.map((member, i) =>
          i === index
            ? { ...member, moveIds: member.moveIds.map((current2, j) => (j === slot ? moveId : current2)) }
            : member,
        ),
      );
    },
  };
}

/** タイプバランスの画面(ADR-0303 §2・§7)。 */
export function BalanceScreen({ master, client }: BalanceScreenProps) {
  // P4-16b(ADR-0304 A-9): speciesList・moves が両方そろうまで画面ごと使えない(技が空のまま
  // threats/recommendations を呼ぶと誤解を招く診断になるため。effects はこの判定に入れない)。
  const capabilities = masterCapabilities(master);
  const balanceAvailable = capabilities.speciesList && capabilities.moves;
  const [members, setMembers] = useState<MemberState[]>(() => [emptyMember(0)]);
  const memberActions = useMemberListActions(setMembers);

  // P4-12b: 仮想敵の枠(メンバーと同じ構造だが独立した状態。ADR-0303 §7)。
  const [threats, setThreats] = useState<MemberState[]>(() => [emptyMember(0)]);
  const threatActions = useMemberListActions(setThreats);

  // analyze へ送るメンバー(ポケモンを選んだメンバーだけ、枠の順)。abilityId は空なら省く。
  // analyzeKey は種族・特性が変わったときだけ変わる文字列で、技の変更では変わらない
  // (技を選んでも analyze を送り直さない)。
  const analyzeMembers: AnalyzeMembers = members
    .filter((member) => member.speciesKey !== "")
    .map((member) =>
      member.abilityId === ""
        ? { pokemonId: member.speciesKey }
        : { pokemonId: member.speciesKey, abilityId: member.abilityId },
    );
  const analyzeKey = JSON.stringify(analyzeMembers);

  const moveById = new Map(master.moves.map((move) => [move.id, move]));
  // coverage を呼ぶかどうかだけの判定(倍率・分類の計算はしない。balance 側の「変化技は除外」の定義が
  // 変わっても、ここは「呼ぶ価値があるか」の目安に過ぎず、応答自体は balance-svc が返す値をそのまま使う)。
  const hasDamagingMove = members.some((member) =>
    member.moveIds.some((moveId) => moveId !== "" && moveById.get(moveId)?.category !== "status"),
  );
  const coverageMembers: CoverageMembers = members
    .filter((member) => member.speciesKey !== "")
    .map((member) => ({ pokemonId: member.speciesKey, moveIds: dedupeMoveIds(member.moveIds) }));
  const coverageKey = hasDamagingMove ? JSON.stringify(coverageMembers) : "";

  // パーティのうちポケモンを選んだメンバー(threats・recommendations で共通。ADR-0400 §1・ADR-0401 §6)。
  const idOnlyMembers: readonly ThreatsPokemon[] = members
    .filter((member) => member.speciesKey !== "")
    .map(toIdOnlyPokemon);

  // P4-12b: threats はパーティ1体以上・仮想敵1体以上そろったときだけ呼ぶ(ADR-0400 §1)。
  // key は両方の内容を含むので、どちらを変えても loading に戻る。
  // slotNumbers は「仮想敵n」の入力欄の番号(1始まり)を、送った順(=応答の順)のまま持ち回る。
  // 途中の枠を空けたまま後ろだけ埋めても、結果の見出しが入力欄の番号とずれないようにするため。
  const filledThreats = threats
    .map((threat, slotIndex) => ({ threat, slotNumber: slotIndex + 1 }))
    .filter((entry) => entry.threat.speciesKey !== "");
  const threatsThreats: readonly ThreatsPokemon[] = filledThreats.map((entry) =>
    toIdOnlyPokemon(entry.threat),
  );
  const threatSlotNumbers: readonly number[] = filledThreats.map((entry) => entry.slotNumber);
  const threatsKey =
    idOnlyMembers.length === 0 || threatsThreats.length === 0
      ? ""
      : JSON.stringify({ members: idOnlyMembers, threats: threatsThreats });

  // P4-12b: recommendations はパーティ1体以上そろえば呼ぶ(仮想敵は入力に含めない。ADR-0303 §7)。
  const recommendationsMembers: RecommendationsMembers = [...idOnlyMembers];
  const recommendationsKey =
    recommendationsMembers.length === 0 ? "" : JSON.stringify(recommendationsMembers);

  const [completedAnalyze, setCompletedAnalyze] = useState<Completed<Schemas["AnalyzeResponse"]> | null>(
    null,
  );
  const [completedCoverage, setCompletedCoverage] = useState<Completed<Schemas["CoverageResponse"]> | null>(
    null,
  );
  const [completedThreats, setCompletedThreats] = useState<Completed<Schemas["ThreatsResponse"]> | null>(
    null,
  );
  const [completedRecommendations, setCompletedRecommendations] = useState<Completed<
    Schemas["RecommendationsResponse"]
  > | null>(null);

  // ポケモンを選んだメンバーが1人もいなければ呼ばない。setState は .then の中だけで行う(CalcScreen.tsx と
  // 同じ作法)。応答が届く前に入力が変わったら(cancelled)、古い応答は無視する。
  useEffect(() => {
    // P4-16b(ADR-0304 A-9): speciesList・moves のどちらかが使えなければ、balance API を1本も呼ばない
    // (誤解を招く診断を出さない)。
    if (!balanceAvailable || analyzeMembers.length === 0) {
      return;
    }
    let cancelled = false;
    void client.analyze(analyzeMembers).then((result) => {
      if (!cancelled) {
        setCompletedAnalyze({ key: analyzeKey, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // analyzeKey が種族・特性の内容そのものを表すので、これだけを見る(members を直接見ると
    // 技だけの変更でも実行され直してしまう)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, analyzeKey, balanceAvailable]);

  // 攻撃技(変化技でない技)を選んだメンバーが1人もいなければ呼ばない。
  useEffect(() => {
    if (!balanceAvailable || coverageKey === "") {
      return;
    }
    let cancelled = false;
    void client.coverage(coverageMembers).then((result) => {
      if (!cancelled) {
        setCompletedCoverage({ key: coverageKey, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // coverageKey が送る内容そのものを表す(空なら呼ばない)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, coverageKey, balanceAvailable]);

  // P4-12b: パーティ1体以上・仮想敵1体以上そろわなければ呼ばない(ADR-0400 §1)。
  useEffect(() => {
    if (!balanceAvailable || threatsKey === "") {
      return;
    }
    let cancelled = false;
    void client.threats(idOnlyMembers, threatsThreats).then((result) => {
      if (!cancelled) {
        setCompletedThreats({ key: threatsKey, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // threatsKey が送る内容そのものを表す(空なら呼ばない)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, threatsKey, balanceAvailable]);

  // P4-12b: パーティ1体以上そろえば呼ぶ(仮想敵の入力では呼び直さない。ADR-0303 §7)。
  useEffect(() => {
    if (!balanceAvailable || recommendationsKey === "") {
      return;
    }
    let cancelled = false;
    void client.recommendations(recommendationsMembers).then((result) => {
      if (!cancelled) {
        setCompletedRecommendations({ key: recommendationsKey, result });
      }
    });
    return () => {
      cancelled = true;
    };
    // recommendationsKey が送る内容そのものを表す(空なら呼ばない。仮想敵は含まない)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, recommendationsKey, balanceAvailable]);

  // idle・loading は入力から毎レンダー導出する(CalcScreen.tsx と同じ作法)。
  const analyzeState: RequestState<Schemas["AnalyzeResponse"]> =
    analyzeMembers.length === 0 ? { status: "idle" } : deriveRequestState(analyzeKey, completedAnalyze);
  const coverageState: RequestState<Schemas["CoverageResponse"]> =
    coverageKey === "" ? { status: "idle" } : deriveRequestState(coverageKey, completedCoverage);
  const threatsState: RequestState<Schemas["ThreatsResponse"]> =
    threatsKey === "" ? { status: "idle" } : deriveRequestState(threatsKey, completedThreats);
  const recommendationsState: RequestState<Schemas["RecommendationsResponse"]> =
    recommendationsKey === ""
      ? { status: "idle" }
      : deriveRequestState(recommendationsKey, completedRecommendations);

  const loading = analyzeState.status === "loading" || coverageState.status === "loading";

  return (
    <div className="balance-screen">
      {/* P4-16b(ADR-0304 A-9): speciesList・moves のどちらかが使えないときは画面ごと使えない案内を出し、
          入力は残すが全部 disabled にする(balance API は1本も呼ばない)。 */}
      {!balanceAvailable && <p className="balance-screen__notice">{masterOnlineText.balanceUnavailable}</p>}
      <div className="balance-screen__members">
        {members.map((member, index) => (
          <MemberFields
            key={member.id}
            groupLabel={balanceScreenText.memberGroupLabel(index + 1)}
            removeLabel={balanceScreenText.removeMemberLabel(index + 1)}
            member={member}
            master={master}
            removable={members.length > 1}
            disabled={!balanceAvailable}
            onSelectSpecies={(speciesKey) => {
              memberActions.selectSpecies(master, index, speciesKey);
            }}
            onSelectAbility={(abilityId) => {
              memberActions.selectAbility(index, abilityId);
            }}
            onSelectMove={(slot, moveId) => {
              memberActions.selectMove(index, slot, moveId);
            }}
            onRemove={() => {
              memberActions.remove(index);
            }}
          />
        ))}
      </div>
      <button
        type="button"
        onClick={memberActions.add}
        disabled={!balanceAvailable || members.length >= MAX_MEMBERS}
      >
        {balanceScreenText.addMemberLabel}
      </button>

      <div className="balance-screen__members">
        {threats.map((threat, index) => (
          <MemberFields
            key={threat.id}
            groupLabel={balanceScreenText.threatGroupLabel(index + 1)}
            removeLabel={balanceScreenText.removeThreatLabel(index + 1)}
            member={threat}
            master={master}
            removable={threats.length > 1}
            disabled={!balanceAvailable}
            onSelectSpecies={(speciesKey) => {
              threatActions.selectSpecies(master, index, speciesKey);
            }}
            onSelectAbility={(abilityId) => {
              threatActions.selectAbility(index, abilityId);
            }}
            onSelectMove={(slot, moveId) => {
              threatActions.selectMove(index, slot, moveId);
            }}
            onRemove={() => {
              threatActions.remove(index);
            }}
          />
        ))}
      </div>
      <button
        type="button"
        onClick={threatActions.add}
        disabled={!balanceAvailable || threats.length >= MAX_MEMBERS}
      >
        {balanceScreenText.addThreatLabel}
      </button>

      {loading && <p className="balance-screen__notice">{balanceScreenText.loadingNotice}</p>}
      {analyzeState.status === "error" && (
        <p role="alert" className="balance-screen__error">
          {analyzeState.error.message}
        </p>
      )}
      {coverageState.status === "error" && (
        <p role="alert" className="balance-screen__error">
          {coverageState.error.message}
        </p>
      )}

      {analyzeState.status === "success" && (
        <>
          <DefenseTable master={master} response={analyzeState.value} />
          <TeamSummaryTable response={analyzeState.value} />
        </>
      )}
      {coverageState.status === "success" && <CoverageTable response={coverageState.value} />}

      {threatsState.status === "loading" && (
        <p className="balance-screen__notice">{balanceScreenText.threatsLoadingNotice}</p>
      )}
      {threatsState.status === "error" && (
        <p role="alert" className="balance-screen__error">
          {threatsState.error.message}
        </p>
      )}
      {threatsState.status === "success" &&
        threatsState.value.threats.map((threat, index) => (
          <ThreatSection
            key={`${threat.pokemonId}-${String(index)}`}
            n={threatSlotNumbers[index] ?? index + 1}
            master={master}
            threat={threat}
          />
        ))}

      {recommendationsState.status === "loading" && (
        <p className="balance-screen__notice">{balanceScreenText.recommendationsLoadingNotice}</p>
      )}
      {recommendationsState.status === "error" && (
        <p role="alert" className="balance-screen__error">
          {recommendationsState.error.message}
        </p>
      )}
      {recommendationsState.status === "success" && (
        <RecommendationsSection master={master} response={recommendationsState.value} />
      )}
    </div>
  );
}

interface MemberFieldsProps {
  /** fieldset の legend(role=group の accessible name。「メンバーn」「仮想敵n」)。 */
  readonly groupLabel: string;
  /** 削除ボタンの文言(「メンバーnを削除」「仮想敵nを削除」)。 */
  readonly removeLabel: string;
  readonly member: MemberState;
  readonly master: MasterData;
  readonly removable: boolean;
  /** P4-16b(ADR-0304 A-9): 画面が使えないとき(speciesList・moves のどちらかが false)、欄を全部 disabled にする。 */
  readonly disabled: boolean;
  readonly onSelectSpecies: (speciesKey: string) => void;
  readonly onSelectAbility: (abilityId: string) => void;
  readonly onSelectMove: (slot: number, moveId: string) => void;
  readonly onRemove: () => void;
}

/**
 * 1枠分の入力(ポケモン・特性・技1〜4)。fieldset + legend が role=group とその accessible name になる。
 * 自分のパーティ(メンバー)と仮想敵の両方で使う共通コンポーネント(ADR-0303 §7)。
 */
function MemberFields({
  groupLabel,
  removeLabel,
  member,
  master,
  removable,
  disabled,
  onSelectSpecies,
  onSelectAbility,
  onSelectMove,
  onRemove,
}: MemberFieldsProps) {
  const species = master.species.find((candidate) => candidate.key === member.speciesKey) ?? null;
  const moves = species === null ? [] : learnsetMoves(species, master.moves);
  return (
    <fieldset className="balance-member">
      <legend>{groupLabel}</legend>
      <select
        aria-label={balanceScreenText.speciesLabel}
        value={member.speciesKey}
        disabled={disabled}
        onChange={(event) => {
          onSelectSpecies(event.target.value);
        }}
      >
        <option value="" hidden />
        {master.species.map((candidate) => (
          <option key={candidate.key} value={candidate.key}>
            {candidate.nameJa}
          </option>
        ))}
      </select>
      <select
        aria-label={balanceScreenText.abilityLabel}
        value={member.abilityId}
        disabled={disabled}
        onChange={(event) => {
          onSelectAbility(event.target.value);
        }}
      >
        {species?.abilities.map((abilityId) => (
          <option key={abilityId} value={abilityId}>
            {findAbilityName(master, abilityId)}
          </option>
        ))}
      </select>
      {Array.from({ length: MOVE_SLOTS }, (_, slot) => slot).map((slot) => (
        <select
          key={slot}
          aria-label={balanceScreenText.moveLabel(slot + 1)}
          value={member.moveIds[slot] ?? ""}
          disabled={disabled}
          onChange={(event) => {
            onSelectMove(slot, event.target.value);
          }}
        >
          <option value="">{balanceScreenText.noMoveOption}</option>
          {moves.map((move) => (
            <option key={move.id} value={move.id}>
              {move.nameJa}
            </option>
          ))}
        </select>
      ))}
      {removable && (
        <button type="button" onClick={onRemove} disabled={disabled} className="balance-member__remove">
          {removeLabel}
        </button>
      )}
    </fieldset>
  );
}

interface DefenseTableProps {
  readonly master: MasterData;
  readonly response: Schemas["AnalyzeResponse"];
}

/** 防御相性の表(メンバー × 18タイプ)。列の並びは応答の順に従う(タイプの順をコードに持たない)。 */
function DefenseTable({ master, response }: DefenseTableProps) {
  const headerTypes = response.teamSummary.map((entry) => entry.attackType);
  return (
    <table aria-label={balanceScreenText.defenseTableLabel}>
      <thead>
        <tr>
          <th scope="col">{balanceScreenText.memberColumnLabel}</th>
          {headerTypes.map((type) => (
            <th key={type} scope="col" style={{ borderBottomColor: `var(--type-${type})` }}>
              {typeNameJa[type]}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {response.members.map((member, index) => (
          <tr key={`${member.pokemonId}-${String(index)}`}>
            <th scope="row">{findSpeciesName(master, member.pokemonId)}</th>
            {headerTypes.map((type) => {
              const entry = member.defense.find((candidate) => candidate.attackType === type);
              return (
                <td key={type}>
                  {entry === undefined ? "" : defenseMultiplierLabel(entry.multiplier, entry.category)}
                </td>
              );
            })}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** チームの集計(攻撃タイプごとの 弱点・うち×4・耐性・無効・等倍 の人数)。 */
function TeamSummaryTable({ response }: { readonly response: Schemas["AnalyzeResponse"] }) {
  return (
    <table aria-label={balanceScreenText.teamSummaryTableLabel}>
      <thead>
        <tr>
          <th scope="col">{balanceScreenText.attackTypeColumnLabel}</th>
          <th scope="col">{balanceScreenText.weakColumnLabel}</th>
          <th scope="col">{balanceScreenText.quadWeakColumnLabel}</th>
          <th scope="col">{balanceScreenText.resistColumnLabel}</th>
          <th scope="col">{balanceScreenText.immuneColumnLabel}</th>
          <th scope="col">{balanceScreenText.neutralColumnLabel}</th>
        </tr>
      </thead>
      <tbody>
        {response.teamSummary.map((entry) => (
          <tr key={entry.attackType}>
            <th scope="row" style={{ borderLeftColor: `var(--type-${entry.attackType})` }}>
              {typeNameJa[entry.attackType]}
            </th>
            <td>{entry.weak}</td>
            <td>{entry.quadWeak}</td>
            <td>{entry.resist}</td>
            <td>{entry.immune}</td>
            <td>{entry.neutral}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

/** 攻撃範囲(防御タイプごとの 最大倍率・有効・抜群 の人数)。 */
function CoverageTable({ response }: { readonly response: Schemas["CoverageResponse"] }) {
  return (
    <table aria-label={balanceScreenText.coverageTableLabel}>
      <thead>
        <tr>
          <th scope="col">{balanceScreenText.defenseTypeColumnLabel}</th>
          <th scope="col">{balanceScreenText.bestMultiplierColumnLabel}</th>
          <th scope="col">{balanceScreenText.effectiveColumnLabel}</th>
          <th scope="col">{balanceScreenText.superEffectiveColumnLabel}</th>
        </tr>
      </thead>
      <tbody>
        {response.teamCoverage.map((entry) => (
          <tr key={entry.defenseType}>
            <th scope="row" style={{ borderLeftColor: `var(--type-${entry.defenseType})` }}>
              {typeNameJa[entry.defenseType]}
            </th>
            <td>{coverageMultiplierLabel(entry.bestMultiplier)}</td>
            <td>{entry.effectiveMembers}</td>
            <td>{entry.superEffectiveMembers}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// ---- P4-12b: 仮想敵(threats)・おすすめタイプ(recommendations)の表示(ADR-0400・ADR-0401) ----

/** タイプの一覧を表示用の文字列にする(「・」区切り、空なら「なし」。ADR-0303 §7)。 */
function typeListText(types: readonly Schemas["TypeId"][]): string {
  return types.length === 0
    ? balanceScreenText.noneLabel
    : types.map((type) => typeNameJa[type]).join(balanceScreenText.listSeparator);
}

interface ThreatSectionProps {
  readonly n: number;
  readonly master: MasterData;
  readonly threat: Schemas["ThreatResult"];
}

/**
 * 仮想敵1体分の結果(region)。行=自分のメンバー、列=受ける倍率・与える倍率・安全・抜群(ADR-0400 §2)。
 * safe/superEffective は応答の真偽値をそのまま語にする(倍率から判定し直さない)。
 */
function ThreatSection({ n, master, threat }: ThreatSectionProps) {
  return (
    <section
      className="balance-screen__threat"
      aria-label={balanceScreenText.threatRegionLabel(n, findSpeciesName(master, threat.pokemonId))}
    >
      <table aria-label={balanceScreenText.threatMatchupTableLabel}>
        <thead>
          <tr>
            <th scope="col">{balanceScreenText.memberColumnLabel}</th>
            <th scope="col">{balanceScreenText.incomingColumnLabel}</th>
            <th scope="col">{balanceScreenText.outgoingColumnLabel}</th>
            <th scope="col">{balanceScreenText.safeColumnLabel}</th>
            <th scope="col">{balanceScreenText.superEffectiveColumnLabel}</th>
          </tr>
        </thead>
        <tbody>
          {threat.matchups.map((matchup, index) => (
            <tr key={`${matchup.pokemonId}-${String(index)}`}>
              <th scope="row">{findSpeciesName(master, matchup.pokemonId)}</th>
              <td>{matchupMultiplierLabel(matchup.incoming)}</td>
              <td>{matchupMultiplierLabel(matchup.outgoing)}</td>
              <td>{safeLabel(matchup.safe)}</td>
              <td>{superEffectiveLabel(matchup.superEffective)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p>{balanceScreenText.safeMembersLabel(threat.safeMembers)}</p>
      <p>{balanceScreenText.superEffectiveMembersLabel(threat.superEffectiveMembers)}</p>
    </section>
  );
}

interface RecommendationsSectionProps {
  readonly master: MasterData;
  readonly response: Schemas["RecommendationsResponse"];
}

/** おすすめタイプの結果(region)。防御・攻撃範囲の穴、候補の表、特性で補える表を出す(ADR-0401)。 */
function RecommendationsSection({ master, response }: RecommendationsSectionProps) {
  return (
    <section
      className="balance-screen__recommendations"
      aria-label={balanceScreenText.recommendationsRegionLabel}
    >
      <p>{balanceScreenText.defenseHolesLabel(typeListText(response.defenseHoles))}</p>
      <p>{balanceScreenText.offenseHolesLabel(typeListText(response.offenseHoles))}</p>
      <CandidatesTable candidates={response.candidates} />
      <AbilityOptionsTable master={master} abilityOptions={response.abilityOptions} />
    </section>
  );
}

/** おすすめタイプの候補の表(タイプ・ふさぐ防御の穴・ふさぐ攻撃範囲の穴・該当ポケモン。応答の順)。 */
function CandidatesTable({ candidates }: { readonly candidates: readonly Schemas["TypeCandidate"][] }) {
  return (
    <table aria-label={balanceScreenText.candidatesTableLabel}>
      <thead>
        <tr>
          <th scope="col">{balanceScreenText.typesColumnLabel}</th>
          <th scope="col">{balanceScreenText.defenseCoveredColumnLabel}</th>
          <th scope="col">{balanceScreenText.offenseCoveredColumnLabel}</th>
          <th scope="col">{balanceScreenText.pokemonColumnLabel}</th>
        </tr>
      </thead>
      <tbody>
        {candidates.map((candidate, index) => {
          const pokemonText =
            candidate.pokemon.length === 0
              ? balanceScreenText.noneLabel
              : candidate.pokemon
                  .map((pokemon) => pokemon.nameJa ?? pokemon.pokemonId)
                  .join(balanceScreenText.listSeparator);
          return (
            <tr key={`${candidate.types.join("-")}-${String(index)}`}>
              <th scope="row">{typeListText(candidate.types)}</th>
              <td>{typeListText(candidate.defenseCovered)}</td>
              <td>{typeListText(candidate.offenseCovered)}</td>
              <td>{pokemonText}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

interface AbilityOptionsTableProps {
  readonly master: MasterData;
  readonly abilityOptions: readonly Schemas["AbilityOption"][];
}

/** 特性で補えるポケモンの表(防御の穴ごとに 攻撃タイプ・該当ポケモン。ADR-0401 §4)。 */
function AbilityOptionsTable({ master, abilityOptions }: AbilityOptionsTableProps) {
  return (
    <table aria-label={balanceScreenText.abilityOptionsTableLabel}>
      <thead>
        <tr>
          <th scope="col">{balanceScreenText.attackTypeColumnLabel}</th>
          <th scope="col">{balanceScreenText.pokemonColumnLabel}</th>
        </tr>
      </thead>
      <tbody>
        {abilityOptions.map((option) => {
          const pokemonText =
            option.pokemon.length === 0
              ? balanceScreenText.noneLabel
              : option.pokemon
                  .map((pokemon) =>
                    balanceScreenText.abilityOptionEntryLabel(
                      pokemon.nameJa ?? pokemon.pokemonId,
                      findAbilityName(master, pokemon.abilityId),
                      multiplierLabel(pokemon.multiplier),
                    ),
                  )
                  .join(balanceScreenText.listSeparator);
          return (
            <tr key={option.attackType}>
              <th scope="row">{typeNameJa[option.attackType]}</th>
              <td>{pokemonText}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
