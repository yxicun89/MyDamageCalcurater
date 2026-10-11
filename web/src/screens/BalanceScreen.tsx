// P4-12a/b: タイプバランスの画面(docs/adr/0303-web-balance-screen.md、docs/type-balance-design.md §6 TB1〜TB5、
// ADR-0400、ADR-0401)。balance API のクライアント(props の client、ADR-0303 §1)へ、メンバー(最大6体。
// ポケモン・特性・技4つまで)を送り、防御相性(analyze)・攻撃範囲(coverage)を出す(P4-12a)。加えて、
// 仮想敵(threats。最大6体、メンバーと同じ入力)・おすすめタイプ(recommendations)を出す(P4-12b)。
// 倍率・集計・穴の判定は応答をそのまま出し、Web で計算し直さない(ADR-0303 §1・§7)。
// ADR-0421: 呼び出しの状態管理は balance/useTeamBalance、結果の表示は balance/BalancePanel に取り出した。
// この画面に残るのは入力欄(メンバー・仮想敵の選択)とその組み合わせだけ。

import { useState } from "react";
import type { BalanceClient } from "../api/balanceClient";
import { BalancePanel } from "../balance/BalancePanel";
import {
  emptyMember,
  MAX_MEMBERS,
  MemberFields,
  toBalanceInput,
  useMemberListActions,
  type MemberState,
} from "../balance/memberInputs";
import { isBalanceAvailable, useTeamBalance } from "../balance/useTeamBalance";
import { balanceScreenText, masterOnlineText } from "../i18n/ja";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";
import { Tile } from "../ui/Tile";
import { useSpeciesResolutions } from "./speciesResolution";
import "../balance/BalancePanel.css";

/** 画面の props(App.tsx が注入する。マスタは既存の画面と同じ MasterData。ADR-0303 §2・§3)。 */
export interface BalanceScreenProps {
  readonly master: MasterData;
  readonly client: BalanceClient;
  /**
   * P4-17b(ADR-0304 A-14): 種族を都度引く口。`capabilities.speciesList` が false のマスタでは、
   * 各枠(メンバー・仮想敵)のポケモン選択をドロップダウンから検索欄に替え、技・特性も
   * `resolveSpecies` と一緒に解決する(A-14.1・A-14.2)。省略は「検索できない」
   * (capabilities を省いたマスタ = 今までどおりドロップダウン)。
   */
  readonly masterSearch?: MasterSpeciesSearch;
}

/** タイプバランスの画面(ADR-0303 §2・§7)。 */
export function BalanceScreen({ master, client, masterSearch }: BalanceScreenProps) {
  // P4-17b(ADR-0304 A-14.1): 画面の可否は「入力の口があるか」で決める(判定は useTeamBalance と共有)。
  const capabilities = masterCapabilities(master);
  const balanceAvailable = isBalanceAvailable(master, masterSearch);

  // P4-17b(ADR-0304 A-14.2): 検索で解決した種族・特性・技の覚え書き。Map なので12枠(メンバー6・
  // 仮想敵6)で1つを共有できる(speciesResolution.ts は変更しない)。
  const { speciesFor, abilitiesFor, movesFor, register: registerSpeciesResolution } = useSpeciesResolutions();

  const [members, setMembers] = useState<MemberState[]>(() => [emptyMember(0)]);
  const memberActions = useMemberListActions(setMembers);

  // P4-12b: 仮想敵の枠(メンバーと同じ構造だが独立した状態。ADR-0303 §7)。
  const [threats, setThreats] = useState<MemberState[]>(() => [emptyMember(0)]);
  const threatActions = useMemberListActions(setThreats);

  /**
   * P4-17b(ADR-0304 A-14.2): 検索でメンバーの種族が解決したとき。枠は id で引く(critic 指摘):
   * 解決を待つ間に他の枠が削除されると index はもう違う枠を指しうるため。
   */
  function handleMemberResolved(id: number, resolution: MasterSpeciesResolution): void {
    registerSpeciesResolution(resolution);
    memberActions.resolveSpecies(id, resolution);
  }

  /** P4-17b(ADR-0304 A-14.2): 検索で仮想敵の種族が解決したとき(同上、id で引く)。 */
  function handleThreatResolved(id: number, resolution: MasterSpeciesResolution): void {
    registerSpeciesResolution(resolution);
    threatActions.resolveSpecies(id, resolution);
  }

  // ADR-0421: balance API の呼び出し(analyze・coverage・threats・recommendations)は useTeamBalance に任せる。
  const balance = useTeamBalance({
    members: members.map(toBalanceInput),
    threats: threats.map(toBalanceInput),
    client,
    master,
    masterSearch,
    movesFor,
  });

  return (
    <div className="balance-screen">
      {/* P4-17b(ADR-0304 A-14.1): ポケモン・技のどちらかを選ぶ口が無いときだけ画面ごと使えない案内を出し、
          入力は残すが全部 disabled にする(balance API は1本も呼ばない)。 */}
      {!balanceAvailable && (
        <p className="ui-notice ui-notice--info balance-screen__notice">
          {masterOnlineText.balanceUnavailable}
        </p>
      )}
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
            movesListAvailable={capabilities.moves}
            speciesListAvailable={capabilities.speciesList}
            masterSearch={masterSearch}
            speciesFor={speciesFor}
            abilitiesFor={abilitiesFor}
            movesFor={movesFor}
            onSelectSpecies={(speciesKey) => {
              memberActions.selectSpecies(master, index, speciesKey);
            }}
            onSpeciesResolved={(resolution) => {
              handleMemberResolved(member.id, resolution);
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
      <Tile
        empty
        className="balance-screen__add"
        label={balanceScreenText.addMemberLabel}
        onClick={memberActions.add}
        disabled={!balanceAvailable || members.length >= MAX_MEMBERS}
      />

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
            movesListAvailable={capabilities.moves}
            speciesListAvailable={capabilities.speciesList}
            masterSearch={masterSearch}
            speciesFor={speciesFor}
            abilitiesFor={abilitiesFor}
            movesFor={movesFor}
            onSelectSpecies={(speciesKey) => {
              threatActions.selectSpecies(master, index, speciesKey);
            }}
            onSpeciesResolved={(resolution) => {
              handleThreatResolved(threat.id, resolution);
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
      <Tile
        empty
        className="balance-screen__add"
        label={balanceScreenText.addThreatLabel}
        onClick={threatActions.add}
        disabled={!balanceAvailable || threats.length >= MAX_MEMBERS}
      />

      <BalancePanel state={balance} master={master} speciesFor={speciesFor} abilitiesFor={abilitiesFor} />
    </div>
  );
}
