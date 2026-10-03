// 逆算の画面の登録(P4-4。ADR-0173)。
import { defineScreen, noClient } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { ReverseScreen } from "./ReverseScreen";

export default defineScreen({
  id: "reverse",
  segment: "reverse",
  label: appText.reverseTabLabel,
  order: 200,
  usesMaster: true,
  createClient: noClient,
  render: (env) => <ReverseScreen engine={env.engine} master={env.master} masterSearch={env.masterSearch} />,
});
