## 2026-10-03: iOS の画面は機能レジストリで登録する(iOS レーン → iOS に画面を足す全レーンへ。ADR-0507)
Decision: iOS アプリの画面は `ios/PokeCalc/Features/<名前>Feature.swift`(`AppFeature`)で登録し、`RootView.swift`・`AppEnvironment.swift` は編集しない。
画面だけが使うサービスは `registerServices` で `FeatureServices` に型で登録し、`context.services.resolve((any XxxService).self)` で引く。
`AppEnvironment` は `.ready(core: CoreServices, features: FeatureServices)` に固定した。共有ファイルの編集は `Features/FeatureRegistry.swift` の配列の末尾への1行だけ(任意で `ios/scripts/sim-run.sh` の `case` 1行)。
Reason: `.ready` の位置引数と RootView のボタン・遷移・起動時の else-if を全レーンが書き換え、PR が毎回衝突していた(ユーザー決定「画面レジストリ化」)。
Impact: 未マージで `RootView`・`AppEnvironment` を編集しているブランチ(例: 判定画面 #511)は、main を merge して両ファイルは main 側を採り、自分の画面を
`AppFeature` に移して `FeatureRegistry` に1行足す(手順と判定画面の例は ADR-0507「他のブランチの移行手順」)。入口の識別子・環境変数の名前は従来どおり。
