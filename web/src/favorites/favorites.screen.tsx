// お気に入り(手動ピン留め)の画面の登録(P5-3c・ADR-0327。ADR-0323)。
// マスタを使わない(種族名の解決をしない)ので、マスタの読み込みに失敗していても開ける。API 専用でオンラインのときだけ動く。
import { defineScreen } from "../app/screenDefinition";
import { favoritesScreenText } from "../i18n/favorites";
import { FavoritesScreen } from "./FavoritesScreen";

export default defineScreen({
  id: "favorites",
  segment: "favorites",
  label: favoritesScreenText.tabLabel,
  order: 650,
  usesMaster: false,
  // 記録 API のクライアントは App が持つ(オンライン限定で env.recordClient として渡す。ADR-0317)。
  createClient: () => undefined,
  render: (env) => <FavoritesScreen recordClient={env.recordClient} reloadToken={env.favoritesReloadToken} />,
});
