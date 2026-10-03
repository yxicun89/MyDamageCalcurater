// 調整(指数・16n・SP 配分・最小 SP)の画面の登録(AJ6・ADR-0319 §1。ADR-0173)。
// API 専用なので、計算モードに関係なくオンラインのマスタを使う(ADR-0411)。
import { OnlineMasterGate } from "../app/onlineMasterGate";
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { createAdjustClient } from "./adjustClient";
import { AdjustScreen } from "./AdjustScreen";

export default defineScreen({
  id: "adjust",
  segment: "adjust",
  label: appText.adjustTabLabel,
  order: 700,
  usesMaster: true,
  // createAdjustClient 自体は fetch しない(調整のタブを開くだけでは呼ばれない)。
  createClient: createAdjustClient,
  render: (env, client) => (
    <OnlineMasterGate onlineMasterSource={env.onlineMasterSource}>
      {(master, masterSearch) => (
        <AdjustScreen adjustClient={client} master={master} masterSearch={masterSearch} />
      )}
    </OnlineMasterGate>
  ),
});
