// タイプバランスの画面の登録(P4-12a・ADR-0303 §2。ADR-0323)。
// issue 276(ADR-0411): API 専用なので、計算モードに関係なくオンラインのマスタを使う。
import { createBalanceClient } from "../api/balanceClient";
import { OnlineMasterGate } from "../app/onlineMasterGate";
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { BalanceScreen } from "./BalanceScreen";

export default defineScreen({
  id: "balance",
  segment: "balance",
  label: appText.balanceTabLabel,
  order: 300,
  usesMaster: true,
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
