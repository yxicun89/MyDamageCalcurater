// 判定(抜けて倒せるか・返り討ちに遭うか)の画面の登録(JD5・ADR-0705 §1。ADR-0323)。
// issue 276(ADR-0411): API 専用なので、計算モードに関係なくオンラインのマスタを使う。
import { OnlineMasterGate } from "../app/onlineMasterGate";
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { createJudgeClient } from "./judgeClient";
import { JudgeScreen } from "./JudgeScreen";

export default defineScreen({
  id: "judge",
  segment: "judge",
  label: appText.judgeTabLabel,
  order: 500,
  usesMaster: true,
  hidden: true, // ADR-0330: 目的を作り直すまでタブから外す(コードは残す。再表示はこの行を消す)
  // createJudgeClient 自体は fetch しない(判定のタブを開くだけでは呼ばれない。JudgeScreen.tsx)。
  createClient: createJudgeClient,
  render: (env, client) => (
    <OnlineMasterGate onlineMasterSource={env.onlineMasterSource}>
      {(master, masterSearch) => (
        <JudgeScreen judgeClient={client} master={master} masterSearch={masterSearch} />
      )}
    </OnlineMasterGate>
  ),
});
