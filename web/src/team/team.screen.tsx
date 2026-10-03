// 構築ビルダーの画面の登録(P5-5 PR-A1・ADR-0309 §1・§2。ADR-0323)。
// PR-A2 のメンバー編集で種族・技・持ち物・特性の名前解決にマスタが要るため usesMaster: true。
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { createTeamClient } from "./teamClient";
import { TeamScreen } from "./TeamScreen";

export default defineScreen({
  id: "team",
  segment: "team",
  label: appText.teamTabLabel,
  order: 600,
  usesMaster: true,
  // createTeamClient 自体は fetch しない(構築のタブを開くまで呼ばれない。TeamScreen.tsx)。
  createClient: createTeamClient,
  render: (env, client) => (
    <TeamScreen
      teamClient={client}
      master={env.master}
      masterSearch={env.masterSearch}
      reloadToken={env.reloadToken}
    />
  ),
});
