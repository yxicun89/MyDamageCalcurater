// 計算の画面の登録(P4-2。ADR-0323)。
import { NAV_GROUPS } from "../app/navGroups";
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { createSpeedClient } from "../speed/speedClient";
import { CalcScreen } from "./CalcScreen";

export default defineScreen({
  id: "calc",
  segment: "calc",
  label: appText.calcTabLabel,
  order: 100,
  group: NAV_GROUPS.damage,
  usesMaster: true,
  // 素早さの種族の例(CB-6 段階 C1)で speed API を使う。作るだけで通信しない(連続技の結果が出るまで呼ばれない)。
  createClient: createSpeedClient,
  render: (env, speedClient) => (
    <CalcScreen
      engine={env.engine}
      master={env.master}
      masterSearch={env.masterSearch}
      recordClient={env.recordClient}
      onFavoriteAdded={env.onFavoriteAdded}
      restoreRequest={env.favoriteRestore}
      comboRequest={env.comboOpenRequest}
      // オフラインでは speed API を呼ばない(recordClient と同じく、オンラインのときだけ App が渡す)。
      speedClient={env.recordClient === undefined ? undefined : speedClient}
    />
  ),
});
