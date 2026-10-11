// 構築(メンバー)と仮想敵から balance API を呼ぶ状態管理(ADR-0421 §2、元は ADR-0303 §9・ADR-0400・ADR-0401)。
// ポケモンを選んだメンバーが1人もいなければ analyze/coverage/recommendations のいずれも呼ばない。
// 攻撃技(変化技でない技)を選んだメンバーが1人もいなければ coverage は呼ばない。
// threats はパーティ1体以上・仮想敵1体以上そろったときだけ呼ぶ。
// 4つの呼び出しは互いに独立(1つの遅延・エラーが他の表示を消さない)。古い応答は無視し、入力が変わったら「計算中」に戻す。

import { useEffect, useState } from "react";
import type { BalanceClient, BalanceResult } from "../api/balanceClient";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import type { SpeciesResolutions } from "../screens/speciesResolution";
import type { BalanceInputMember, RequestState, Schemas, TeamBalanceState } from "./types";

type AnalyzeMembers = Schemas["AnalyzeRequest"]["members"];
type CoverageMembers = Schemas["CoverageRequest"]["members"];
type ThreatsPokemon = Schemas["ThreatsRequestPokemon"];
type RecommendationsMembers = Schemas["RecommendationsRequest"]["members"];

export interface UseTeamBalanceOptions {
  /** 構築の枠(speciesKey が "" の枠は無視する)。 */
  readonly members: readonly BalanceInputMember[];
  /** 仮想敵の枠(入力欄の番号を残すため、空の枠も含めて渡す。空は無視する)。 */
  readonly threats: readonly BalanceInputMember[];
  readonly client: BalanceClient;
  readonly master: MasterData;
  /** 種族を都度引く口(ADR-0304 A-14)。balance を呼べるかの判定に使う。 */
  readonly masterSearch?: MasterSpeciesSearch;
  /** 検索で解決した分も含めて、メンバーが実際に引ける技を返す(A-14.3)。攻撃技の有無の判定に使う。 */
  readonly movesFor: SpeciesResolutions["movesFor"];
  /** 入力の変更から送るまでの待ち(ミリ秒)。0 なら待たない。 */
  readonly debounceMs?: number;
}

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

/**
 * メンバー1件を threats/recommendations の ID だけの形にする(ThreatsRequestPokemon と
 * RecommendationsRequestMember は同じ形。ADR-0401 §6)。重複した技は1つにし、特性は選んでいるときだけ送る。
 */
function toIdOnlyPokemon(member: BalanceInputMember): ThreatsPokemon {
  const moveIds = dedupeMoveIds(member.moveIds);
  return member.abilityId === null || member.abilityId === ""
    ? { pokemonId: member.speciesKey, moveIds }
    : { pokemonId: member.speciesKey, moveIds, abilityId: member.abilityId };
}

/**
 * 1本の呼び出し。key が "" なら呼ばない。setState は .then の中だけで行い、応答が届く前に入力が変わったら
 * (cancelled)古い応答は無視する。debounceMs が正なら、入力が変わってから待って送る(待つ間に変わったら送らない)。
 */
function useBalanceCall<T>(
  client: BalanceClient,
  key: string,
  enabled: boolean,
  debounceMs: number,
  call: () => Promise<BalanceResult<T>>,
): Completed<T> | null {
  const [completed, setCompleted] = useState<Completed<T> | null>(null);
  useEffect(() => {
    if (!enabled || key === "") {
      return;
    }
    let cancelled = false;
    const run = (): void => {
      void call().then((result) => {
        if (!cancelled) {
          setCompleted({ key, result });
        }
      });
    };
    let timer: ReturnType<typeof setTimeout> | undefined;
    if (debounceMs > 0) {
      timer = setTimeout(run, debounceMs);
    } else {
      run();
    }
    return () => {
      cancelled = true;
      if (timer !== undefined) {
        clearTimeout(timer);
      }
    };
    // key が送る内容そのものを表すので、これ(と client・可否・待ち)だけを見る。call は毎レンダー新しくなるため含めない。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, key, enabled, debounceMs]);
  return completed;
}

/**
 * P4-17b(ADR-0304 A-14.1): balance を使えるかは「一覧がそろっているか」ではなく「入力の口があるか」で決める。
 * capabilities.moves が永続的に false(A-13.1)なオンラインでも、種族を検索で選べば技は resolveSpecies が
 * 一緒に解決するため、検索口があれば技を選ぶ口もある(speciesList が true でドロップダウンを使っているときは
 * resolveSpecies が走らないので、検索口があっても技は届かない)。
 */
export function isBalanceAvailable(
  master: MasterData,
  masterSearch: MasterSpeciesSearch | undefined,
): boolean {
  const capabilities = masterCapabilities(master);
  const speciesInputAvailable = capabilities.speciesList || masterSearch !== undefined;
  const moveInputAvailable = capabilities.moves || (!capabilities.speciesList && masterSearch !== undefined);
  return speciesInputAvailable && moveInputAvailable;
}

/** タイプバランスの4つの呼び出し(analyze・coverage・threats・recommendations)の状態を返す。 */
export function useTeamBalance({
  members,
  threats,
  client,
  master,
  masterSearch,
  movesFor,
  debounceMs = 0,
}: UseTeamBalanceOptions): TeamBalanceState {
  // P4-16b(ADR-0304 A-9): 使えなければ balance API を1本も呼ばない(誤解を招く診断を出さない)。
  const balanceAvailable = isBalanceAvailable(master, masterSearch);

  const filledMembers = members.filter((member) => member.speciesKey !== "");

  // analyze へ送るメンバー(ポケモンを選んだメンバーだけ、枠の順)。abilityId は無ければ省く。
  // analyzeKey は種族・特性が変わったときだけ変わる文字列で、技の変更では変わらない
  // (技を選んでも analyze を送り直さない)。
  const analyzeMembers: AnalyzeMembers = filledMembers.map((member) =>
    member.abilityId === null || member.abilityId === ""
      ? { pokemonId: member.speciesKey }
      : { pokemonId: member.speciesKey, abilityId: member.abilityId },
  );
  const analyzeKey = analyzeMembers.length === 0 ? "" : JSON.stringify(analyzeMembers);

  // P4-17b(ADR-0304 A-14.3): 参照する一覧を「選んだメンバーが実際に引ける技」にする(master.moves +
  // 検索で解決した分)。オンラインでは master.moves が空なので、master.moves だけを見ると実体不明の ID だらけになる。
  const moveById = new Map(
    members.flatMap((member) => movesFor(master.moves, member.speciesKey)).map((move) => [move.id, move]),
  );
  // coverage を呼ぶかどうかだけの判定(倍率・分類の計算はしない)。実体が分からない ID は攻撃技と見なさない
  // (fail-closed。A-14.3)。
  const hasDamagingMove = members.some((member) =>
    member.moveIds.some((moveId) => {
      const move = moveById.get(moveId);
      return move !== undefined && move.category !== "status";
    }),
  );
  const coverageMembers: CoverageMembers = filledMembers.map((member) => ({
    pokemonId: member.speciesKey,
    moveIds: dedupeMoveIds(member.moveIds),
  }));
  const coverageKey = hasDamagingMove ? JSON.stringify(coverageMembers) : "";

  // パーティのうちポケモンを選んだメンバー(threats・recommendations で共通。ADR-0400 §1・ADR-0401 §6)。
  const idOnlyMembers: readonly ThreatsPokemon[] = filledMembers.map(toIdOnlyPokemon);

  // threats はパーティ1体以上・仮想敵1体以上そろったときだけ呼ぶ(ADR-0400 §1)。key は両方の内容を含む。
  // slotNumbers は途中の枠を空けたまま後ろだけ埋めても、結果の見出しが入力欄の番号とずれないようにするため。
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

  // recommendations はパーティ1体以上そろえば呼ぶ(仮想敵は入力に含めない。ADR-0303 §7)。
  const recommendationsMembers: RecommendationsMembers = [...idOnlyMembers];
  const recommendationsKey =
    recommendationsMembers.length === 0 ? "" : JSON.stringify(recommendationsMembers);

  const completedAnalyze = useBalanceCall(client, analyzeKey, balanceAvailable, debounceMs, () =>
    client.analyze(analyzeMembers),
  );
  const completedCoverage = useBalanceCall(client, coverageKey, balanceAvailable, debounceMs, () =>
    client.coverage(coverageMembers),
  );
  const completedThreats = useBalanceCall(client, threatsKey, balanceAvailable, debounceMs, () =>
    client.threats(idOnlyMembers, threatsThreats),
  );
  const completedRecommendations = useBalanceCall(
    client,
    recommendationsKey,
    balanceAvailable,
    debounceMs,
    () => client.recommendations(recommendationsMembers),
  );

  // idle・loading は入力から毎レンダー導出する(CalcScreen.tsx と同じ作法)。
  return {
    analyze: analyzeKey === "" ? { status: "idle" } : deriveRequestState(analyzeKey, completedAnalyze),
    coverage: coverageKey === "" ? { status: "idle" } : deriveRequestState(coverageKey, completedCoverage),
    threats: threatsKey === "" ? { status: "idle" } : deriveRequestState(threatsKey, completedThreats),
    recommendations:
      recommendationsKey === ""
        ? { status: "idle" }
        : deriveRequestState(recommendationsKey, completedRecommendations),
    threatSlotNumbers,
  };
}
