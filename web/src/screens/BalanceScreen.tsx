// P4-12a: タイプバランスの画面(docs/adr/0303-web-balance-screen.md、docs/type-balance-design.md §6 TB1/TB2)。
// balance API のクライアント(props の client、ADR-0303 §1)へ、メンバー(最大6体。ポケモン・特性・技4つまで)を
// 送り、防御相性(analyze)・攻撃範囲(coverage)を出す。倍率・集計は応答をそのまま出し、Web で計算し直さない
// (ADR-0303 §1)。ポケモンを選んだメンバーが1人もいなければどちらも呼ばない。攻撃技(変化技でない技)を
// 選んだメンバーが1人もいなければ coverage は呼ばない。

import { useEffect, useRef, useState } from "react";
import type { components } from "../api/balance.gen";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import { coverageMultiplierLabel, defenseMultiplierLabel } from "../domain/balanceLabels";
import { learnsetMoves } from "../domain/moves";
import { balanceScreenText, typeNameJa } from "../i18n/ja";
import type { MasterData } from "../master/types";
import "./BalanceScreen.css";

type Schemas = components["schemas"];
type AnalyzeMembers = Schemas["AnalyzeRequest"]["members"];
type CoverageMembers = Schemas["CoverageRequest"]["members"];

/** 画面の props(App.tsx が注入する。マスタは既存の画面と同じ MasterData。ADR-0303 §2・§3)。 */
export interface BalanceScreenProps {
  readonly master: MasterData;
  readonly client: BalanceClient;
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

/** タイプバランスの画面(ADR-0303 §2)。 */
export function BalanceScreen({ master, client }: BalanceScreenProps) {
  const [members, setMembers] = useState<MemberState[]>(() => [emptyMember(0)]);
  const nextIdRef = useRef(1);

  function updateMember(index: number, patch: (member: MemberState) => MemberState): void {
    setMembers((current) => current.map((member, i) => (i === index ? patch(member) : member)));
  }

  function addMember(): void {
    setMembers((current) => {
      if (current.length >= MAX_MEMBERS) {
        return current;
      }
      const id = nextIdRef.current;
      nextIdRef.current += 1;
      return [...current, emptyMember(id)];
    });
  }

  function removeMember(index: number): void {
    setMembers((current) => (current.length <= 1 ? current : current.filter((_, i) => i !== index)));
  }

  function selectSpecies(index: number, speciesKey: string): void {
    const species = master.species.find((candidate) => candidate.key === speciesKey);
    updateMember(index, (member) => ({
      ...member,
      speciesKey,
      abilityId: species?.abilities[0] ?? "",
      moveIds: Array.from({ length: MOVE_SLOTS }, () => ""),
    }));
  }

  function selectAbility(index: number, abilityId: string): void {
    updateMember(index, (member) => ({ ...member, abilityId }));
  }

  function selectMove(index: number, slot: number, moveId: string): void {
    updateMember(index, (member) => ({
      ...member,
      moveIds: member.moveIds.map((current, i) => (i === slot ? moveId : current)),
    }));
  }

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

  const [completedAnalyze, setCompletedAnalyze] = useState<Completed<Schemas["AnalyzeResponse"]> | null>(
    null,
  );
  const [completedCoverage, setCompletedCoverage] = useState<Completed<Schemas["CoverageResponse"]> | null>(
    null,
  );

  // ポケモンを選んだメンバーが1人もいなければ呼ばない。setState は .then の中だけで行う(CalcScreen.tsx と
  // 同じ作法)。応答が届く前に入力が変わったら(cancelled)、古い応答は無視する。
  useEffect(() => {
    if (analyzeMembers.length === 0) {
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
  }, [client, analyzeKey]);

  // 攻撃技(変化技でない技)を選んだメンバーが1人もいなければ呼ばない。
  useEffect(() => {
    if (coverageKey === "") {
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
  }, [client, coverageKey]);

  // idle・loading は入力(analyzeMembers・coverageKey)から毎レンダー導出する(CalcScreen.tsx と同じ作法)。
  const analyzeState: RequestState<Schemas["AnalyzeResponse"]> =
    analyzeMembers.length === 0 ? { status: "idle" } : deriveRequestState(analyzeKey, completedAnalyze);
  const coverageState: RequestState<Schemas["CoverageResponse"]> =
    coverageKey === "" ? { status: "idle" } : deriveRequestState(coverageKey, completedCoverage);

  const loading = analyzeState.status === "loading" || coverageState.status === "loading";
  const alertMessage =
    analyzeState.status === "error"
      ? analyzeState.error.message
      : coverageState.status === "error"
        ? coverageState.error.message
        : null;

  return (
    <div className="balance-screen">
      <div className="balance-screen__members">
        {members.map((member, index) => (
          <MemberFields
            key={member.id}
            n={index + 1}
            member={member}
            master={master}
            removable={members.length > 1}
            onSelectSpecies={(speciesKey) => {
              selectSpecies(index, speciesKey);
            }}
            onSelectAbility={(abilityId) => {
              selectAbility(index, abilityId);
            }}
            onSelectMove={(slot, moveId) => {
              selectMove(index, slot, moveId);
            }}
            onRemove={() => {
              removeMember(index);
            }}
          />
        ))}
      </div>
      <button type="button" onClick={addMember} disabled={members.length >= MAX_MEMBERS}>
        {balanceScreenText.addMemberLabel}
      </button>

      {loading && <p className="balance-screen__notice">{balanceScreenText.loadingNotice}</p>}
      {alertMessage !== null && (
        <p role="alert" className="balance-screen__error">
          {alertMessage}
        </p>
      )}

      {analyzeState.status === "success" && (
        <>
          <DefenseTable master={master} response={analyzeState.value} />
          <TeamSummaryTable response={analyzeState.value} />
        </>
      )}
      {coverageState.status === "success" && <CoverageTable response={coverageState.value} />}
    </div>
  );
}

interface MemberFieldsProps {
  readonly n: number;
  readonly member: MemberState;
  readonly master: MasterData;
  readonly removable: boolean;
  readonly onSelectSpecies: (speciesKey: string) => void;
  readonly onSelectAbility: (abilityId: string) => void;
  readonly onSelectMove: (slot: number, moveId: string) => void;
  readonly onRemove: () => void;
}

/** 1メンバー分の入力(ポケモン・特性・技1〜4)。fieldset + legend が role=group とその accessible name になる。 */
function MemberFields({
  n,
  member,
  master,
  removable,
  onSelectSpecies,
  onSelectAbility,
  onSelectMove,
  onRemove,
}: MemberFieldsProps) {
  const species = master.species.find((candidate) => candidate.key === member.speciesKey) ?? null;
  const moves = species === null ? [] : learnsetMoves(species, master.moves);
  return (
    <fieldset className="balance-member">
      <legend>{balanceScreenText.memberGroupLabel(n)}</legend>
      <select
        aria-label={balanceScreenText.speciesLabel}
        value={member.speciesKey}
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
        <button type="button" onClick={onRemove} className="balance-member__remove">
          {balanceScreenText.removeMemberLabel(n)}
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
