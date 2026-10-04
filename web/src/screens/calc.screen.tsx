// 計算の画面の登録(P4-2。ADR-0323)。
import { defineScreen, noClient } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { CalcScreen } from "./CalcScreen";

export default defineScreen({
  id: "calc",
  segment: "calc",
  label: appText.calcTabLabel,
  order: 100,
  usesMaster: true,
  createClient: noClient,
  render: (env) => (
    <CalcScreen
      engine={env.engine}
      master={env.master}
      masterSearch={env.masterSearch}
      recordClient={env.recordClient}
      onFavoriteAdded={env.onFavoriteAdded}
    />
  ),
});
