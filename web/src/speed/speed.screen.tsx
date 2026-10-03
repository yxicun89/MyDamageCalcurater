// 素早さ比較の画面の登録(SP3・ADR-0604 §2。ADR-0173)。
// engine(WASM)・master(pokedex のマスタ)のどちらも使わない(ADR-0604 §5)。issue 308: マスタの読み込みに
// 失敗していても、このタブだけは使える(usesMaster: false)。
import { defineScreen } from "../app/screenDefinition";
import { appText } from "../i18n/ja";
import { createSpeedClient } from "./speedClient";
import { SpeedScreen } from "./SpeedScreen";

export default defineScreen({
  id: "speed",
  segment: "speed",
  label: appText.speedTabLabel,
  order: 400,
  usesMaster: false,
  // createSpeedClient 自体は fetch しない(素早さのタブを開くまで呼ばれない。SpeedScreen.tsx)。
  createClient: createSpeedClient,
  render: (_env, client) => <SpeedScreen speedClient={client} />,
});
