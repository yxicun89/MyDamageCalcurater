// 構築編集の中の「タイプバランス」の節(ADR-0421 §1・§2)。6 枠の下書きをそのまま入力にし、空の枠は無視して 1 体からでも出す。
// balance が使えない・失敗しても構築の編集と保存は止めない: 呼び出しの状態は useTeamBalance が持ち、
// エラーは BalancePanel の中(role="alert")にだけ出る。この節は例外を投げない。

import type { ReactNode } from "react";
import type { BalanceClient } from "../api/balanceClient";
import { BalancePanel } from "../balance/BalancePanel";
import { isBalanceAvailable, useTeamBalance } from "../balance/useTeamBalance";
import { masterOnlineText, teamBalanceText } from "../i18n/ja";
import type { MasterData, MasterSpeciesSearch } from "../master/types";
import type { SpeciesResolutions } from "../screens/speciesResolution";
import { balanceInputFromDraft } from "./teamBalanceInput";
import type { MemberDraft } from "./teamMember";

/** 入力の変更から送るまでの待ち(ADR-0421 §2)。 */
export const TEAM_BALANCE_DEBOUNCE_MS = 300;

/** おすすめタイプの上位の数(ADR-0422 §1)。 */
export const TEAM_BALANCE_RECOMMENDATIONS_LIMIT = 5;

export interface TeamBalanceSectionProps {
  readonly drafts: readonly MemberDraft[];
  readonly client: BalanceClient;
  readonly master: MasterData;
  readonly masterSearch: MasterSpeciesSearch | undefined;
  readonly resolutions: SpeciesResolutions;
  /** おすすめのポケモンのタイルを押したとき(構築の最初の空き枠に入れる。TeamMemberEditor が持つ)。 */
  readonly onPickPokemon: (speciesKey: string) => void;
  /** 空き枠があるか(無ければタイルは押せない)。 */
  readonly canPick: boolean;
  /** テストで待ちを短くするための口(既定は 300ms)。 */
  readonly debounceMs?: number;
}

export function TeamBalanceSection({
  drafts,
  client,
  master,
  masterSearch,
  resolutions,
  onPickPokemon,
  canPick,
  debounceMs = TEAM_BALANCE_DEBOUNCE_MS,
}: TeamBalanceSectionProps): ReactNode {
  const members = drafts.map(balanceInputFromDraft);
  const state = useTeamBalance({
    members,
    threats: [],
    client,
    master,
    masterSearch,
    movesFor: resolutions.movesFor,
    debounceMs,
    recommendationsLimit: TEAM_BALANCE_RECOMMENDATIONS_LIMIT,
  });
  const available = isBalanceAvailable(master, masterSearch);
  const hasMember = members.some((member) => member.speciesKey !== "");

  return (
    <section aria-labelledby="team-balance-heading" className="team-balance">
      <h3 id="team-balance-heading" className="team-balance__heading">
        {teamBalanceText.heading}
      </h3>
      {!available && <p className="ui-notice ui-notice--info">{masterOnlineText.balanceUnavailable}</p>}
      {available && !hasMember && <p className="team-balance__empty">{teamBalanceText.empty}</p>}
      {available && hasMember && (
        <BalancePanel
          state={state}
          master={master}
          speciesFor={resolutions.speciesFor}
          abilitiesFor={resolutions.abilitiesFor}
          onPickPokemon={onPickPokemon}
          canPick={canPick}
        />
      )}
    </section>
  );
}
