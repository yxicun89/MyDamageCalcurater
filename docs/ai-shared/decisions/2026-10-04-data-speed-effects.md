## 2026-10-04: 素早さに効く特性・持ち物をマスタの正規化データにした(データレーン → 判定レーン。issue #235 第2段・ADR-0139)
Decision: 素早さ補正を持つ特性7件(雨・晴れ・砂・雪・エレキフィールドで ×2、状態異常で ×1.5 かつまひの半減を受けない、持ち物を失った後に ×2)と
持ち物1件(常に ×0.5。こだわりスカーフは既存の扱いのまま対象外)を、`data/importer/effects.json` の `speedItems`・`speedAbilities` 節に定義し、
効果 JSON の `SpeedMods`(`[{Condition, Modifier}]`。Modifier は 4096 基準の整数、配列の順は評価の優先順で成立した最初の要素だけを掛ける)・
`IgnoresParalysisSpeedDrop`(特性だけ)として取り込む。条件の語彙は engine の `SpeedCondition`
(`always`・`weather_sun`・`weather_rain`・`weather_sand`・`weather_snow`・`terrain_electric`・`has_status`・`item_lost`)。
Reason: 判定レーンの依頼(2026-10-02。judge は ID の switch を持たず、データで素早さを反映する)。
Impact:
- **判定レーンへ**: 内部 API `/internal/pokedex/master` の `MasterItem.effect`・`MasterAbility.effect`(契約は自由形式のまま。API レーンへの依頼は不要)に
  `SpeedMods`・`IgnoresParalysisSpeedDrop` が入る。語彙は engine から import できる。judge 側の NetworkPolicy・設定・デコード、状態異常の入力は判定レーン
- ダメージ計算・ゴールデン・read model(balance・speed の6ファイル)は変わらない。持ち物の役割(ADR-0175)で素早さだけの持ち物は roles が空
- **デプロイ順**: アプリ(calc-svc のデコーダ・Web の WASM の境界)を先にロールアウトし、その後に取り込み → `make master-release`。逆順だと旧アプリが新しい項目を拒否する
