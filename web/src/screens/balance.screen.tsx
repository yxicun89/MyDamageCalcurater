// タイプバランスの画面の登録(P4-12a・ADR-0303 §2。ADR-0323)。
// issue 276(ADR-0411): API 専用なので、計算モードに関係なくオンラインのマスタを使う。
import { createBalanceClient } from "../api/balanceClient";
import { OnlineMasterGate } from "../app/onlineMasterGate";
import { NAV_GROUPS } from "../app/navGroups";
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { BalanceScreen } from "./BalanceScreen";

export default defineScreen({
  id: "balance",
  segment: "balance",
  label: appText.balanceTabLabel,
  order: 300,
  group: NAV_GROUPS.team,
  usesMaster: true,
  // API 専用(オフラインでは動かない。ADR-0358)。
  onlineOnly: true,
  // ADR-0421: タイプバランスは構築編集の中に統合した。ナビ・URL・マウントから外す(旧 /balance は構築へ寄せる。app/routes.ts)。
  // 画面の部品はテストの足場として残し、整理は段階 4 で行う。再表示はこの行を消すだけ。
  hidden: true,
  // createBalanceClient 自体は fetch しない(メンバーを選ぶまで呼ばれない。BalanceScreen.tsx)。
  createClient: createBalanceClient,
  render: (env, client) => (
    <OnlineMasterGate onlineMasterSource={env.onlineMasterSource}>
      {(master, masterSearch) => (
        <BalanceScreen master={master} client={client} masterSearch={masterSearch} />
      )}
    </OnlineMasterGate>
  ),
});
