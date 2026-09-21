# 開発計画と進行状況

凡例: `[ ]` 未着手 / `[~]` 作業中 / `[x]` 完了 / `[!]` ブロック中
作業する AI は、タスク開始時に `[~]`、完了時に `[x]` へ更新してからコミットする。

## マイルストーン

| ID | ゴール | 人間の確認方法 |
|---|---|---|
| **M1** | **ブラウザで計算できる**(Mac上のk3d) | `make up` → http://localhost:8080 で計算・一括表示・逆算を触る |
| M2 | 計算の自動保存・よく使う・構築 | 計算後に履歴と「よく計算する相手」が出る |
| M3 | iPhone で使える | Xcode から実機に入れて Tailscale 経由で計算 |
| M4 | 運用(監視・SLO・GitOps) | Grafana でSLOダッシュボードを見る |
| +α | クラウド移行 / ダブル / 推薦ML | |

**キックオフでは M1 完了まで自動で進める。** M2 以降は人間が `/phase M2` などで開始する。

---

## M1: ブラウザで計算できる

### Phase 0 土台
- [x] P0-1 `scripts/doctor.sh` を実行し不足ツールを報告(入れられるものは brew で入れる)
- [x] P0-2 Go workspace(go.work)、Makefile、.gitignore、各ディレクトリの雛形
- [x] P0-3 `api/openapi.yaml` の初版(pokedex検索 / 計算 / 一括計算 / 逆算)と `make gen`
- [x] P0-4 k3d クラスタ定義(`deploy/k3d.yaml`)と `make up/down`、Kustomize base/overlays
- [x] P0-5 `scripts/codex-review.sh` の動作確認(Codex 未ログインならスキップして記録)

### Phase 1 計算エンジン
- [x] P1-1 データモデル(種族・技・持ち物・特性・性格・フィールド状態・format)
- [x] P1-2 実数値計算(SP・性格・ランク)+ 全ポケモン網羅テスト(実数値)
- [x] P1-3 ダメージ計算コア(基本式・乱数16段階・一致・相性・急所・やけど)
- [x] P1-4 補正(天候・フィールド・壁・持ち物・特性)※マスタの補正定義から適用
- [x] P1-5 確定数/乱数n発の算出
- [x] P1-6 `tools/golden` でテストベクタ生成、`make test-golden` 全件一致
- [x] P1-7 一括計算(防御側の代表調整すべてに対する結果を一度に返す)
- [x] P1-8 逆算(観測ダメージ→調整候補、複数観測で絞り込み)+ 再現率テスト
- [ ] P1-9 WASM ビルド(`make wasm`)と Go/WASM の結果一致テスト

### Phase 2 マスタデータ
- [ ] P2-1 **データソース調査**: チャンピオンズの使用可能ポケモン・技・持ち物の取得元を調べ ADR-0002 に記録
  - 確定後に、ゴールデンの種族集合(現在は gen9 参考集合1392種)を差し替えて `make golden-generate` で再生成し、`metadata.json` の `speciesScope` を更新する
  - HP=1 のヌケニンはゴールデンから除外している。チャンピオンズの実数値式(HP = 種族値+75+SP)では HP=76 になるため、扱いを決める
- [ ] P2-2 MySQL スキーマ(migrate)と importer
- [ ] P2-3 pokedex-svc(検索・詳細・持ち物/技一覧、日本語名で前方一致)

### Phase 3 API
- [ ] P3-1 calc-svc(起動時にマスタをメモリへ読み込み)
  - 一括計算(`/api/calc/bulk`)の対応: API の `presets`(enum 配列)→ engine の `PresetKeys`。`presets: []` と省略はどちらも既定セット
  - `api/openapi.yaml` の description を先に直して `make gen`(絶対ルール1): 「変化技は none/hp の2件のみ返す」「行の順序はプリセット優先(presets × itemVariants)」「`presets: []` は省略と同じ」。`BulkCalcRow.preset` は enum のみ(engine のカスタム `Presets` は API に出さない)
  - 逆算(`/api/calc/reverse`)の対応(ADR-0010 §9 の持ち越し。engine は `CalcReverse` 済み・API 未変更): `api/openapi.yaml` を先に直して `make gen`(絶対ルール1)。`ReverseRequest.attacker` / `defenderSpeciesKey` を `known` / `unknownSpeciesKey` に改名(`side=attacker` のとき既知側=自分=防御側)、`itemCandidates: [ItemId]` と `Observation.observedDamage` を追加、`observedPercent` は整数でなければ 400(engine には `int` を渡す)、`matchScore` の意味(1.0 = 全観測に完全一致する格子点がある)を description に書き `exact` を足す、`ReverseCandidate` に `archetypeKey` を足し `presetLabel` に `Archetype.Label` を入れる、`natureId` は engine の性格構造値(代表性格クラス)→ 性格 ID に calc-svc が写像する(例 +B/-A → Bold)、`rangePercent` は `MinPercent`/`MaxPercent` を写す
- [ ] P3-2 gateway(ルーティング・端末ID/セッションID・/assets・CORS)
- [ ] P3-3 契約テスト(OpenAPI 準拠)と k3d 上のスモークテスト

### Phase 4 Web
- [ ] P4-1 デザイントークン(docs/design.md)を CSS 変数に実装
- [ ] P4-2 計算画面(左右カード・持ち物・技・結果の一括表示・攻守入れ替え)
- [ ] P4-3 プリセット選択(自分側: A特化/A振り/無振り)
- [ ] P4-4 逆算画面(観測ダメージ入力→候補リスト)
- [ ] P4-5 API / WASM 切り替え(WASM ならバックエンド無しで動く)
- [ ] P4-6 Playwright E2E(主要フロー)
- [ ] P4-7 **M1 完了報告**: 動作確認手順を `docs/verify-m1.md` に書く

## M2: 保存・構築
- [ ] P5-1 TiDB(tiup playground で開発、k3d は TiDB Operator 最小構成)
- [ ] P5-2 NATS JetStream と calc-svc からのイベント発行(失敗しても計算は成功)
- [ ] P5-3 record-svc(保存・よく使う集計: 頻度×時間減衰)
- [ ] P5-4 team-svc(構築 CRUD、Showdown 形式入出力)
- [ ] P5-5 Web: 履歴・よく計算する相手・構築ビルダー

## M3: iOS
- [ ] P6-1 Xcode プロジェクト、swift-openapi-generator、デザイントークン
- [ ] P6-2 計算画面・逆算・構築
- [ ] P6-3 シミュレータテスト(`make ios-test`)
- [ ] P6-4 Tailscale serve の手順書 → **人間が実機インストール**

## M4: 運用
- [ ] P7-1 kube-prometheus-stack / Loki、各サービスのメトリクス
- [ ] P7-2 SLO(計算API p99 < 100ms、可用性)とダッシュボード
- [ ] P7-3 ArgoCD(GitOps)
- [ ] P7-4 MySQL/TiDB バックアップと復元テスト

## ブロッカー
(ここに止まった理由と試したことを書く)

**【人間の確認待ち】`hb_boost` / `hd_boost`(H振り+B(D)補正)の定義**(P1-7、ADR-0009 §2)
- requirements.md の「H振り+B(D)補正」に SP 配分の定義が無いため、「H振り(hp:32)+防御(特防)を上げる性格補正のみ・SP は振らない」と仮定して実装した。確定扱いにしていない。
- 別解釈: 残り SP を B/D に全振り(実質 hb/hd と同じになり既定セットが1件減る)/ 中途半端な SP 量。
- 確認できたら、次を同時に更新する: ADR-0009 §1 の表と §2 / `engine/bulk.go` のカタログ(`hb_boost` `hd_boost` の行)/ `engine/bulk_test.go` の `TestDefenderPresetCatalogDefinitions` と `TestDefaultDefenderPresetsByCategory`・単調性検査 /
  `tools/golden/generate.mjs`(defense 部分にベクタ追加)→ `make golden-generate` → `testdata/golden/metadata.json` と `engine/bulk_golden_test.go` の件数期待値(現在は4件/グループ前提)/ `api/openapi.yaml` の `presetLabel` 例示。
- `none/hp/hb/hd` の4種は golden で外部照合済みで、この確認の影響を受けない。テストを緩めて両解釈を通すことはしない(絶対ルール6)。

**【人間の確認待ち】表示 % の丸め(round-half-up)**(P1-8、ADR-0010 §3)
- ポケモンチャンピオンズの表示%が四捨五入なのか切り捨てなのか、小数第1位まで出るのかは未確認。「round-half-up の整数%」と仮定して実装した。確定扱いにしていない。実機が切り捨てだと、1 ポイントずれて完全一致が消える観測が出る(ADR-0010 §8)。
- 確認できたら、次を同時に更新する: `engine/reverse.go` の `DisplayPercent`(1関数。engine の丸めはここだけ)/ ADR-0010 §3 の式と §7 の Recall 実測値 / `engine/reverse_test.go` の `TestDisplayPercentRounding` の期待値(理由をコミットメッセージに書く)。Recall テストは `DisplayPercent` を使って観測を作るので、関数の差し替えだけで追従する。しきい値(80%/95%)・シード・ケース数は動かさない(絶対ルール6)。
- 丸めの確定で Recall が基準を割った場合も、しきい値・シード・ケース数ではなく ADR-0010 §6.3 の順序規則側を直す(2回観測の余裕は現状 0.5〜0.6 ポイントで薄い)。

## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)

P1-6 独立レビューで出た軽微・任意の指摘(コードは未変更。次の engine タスクに合わせて対応を検討):
- `engine/golden_test.go`: `speciesCount` の下限アサート追加(現在は 0 だけ検査。少数種で再生成しても通ってしまう)
- `engine/golden_test.go`: `DamageInput` の json タグ明示または `DisallowUnknownFields`(フィールド改名でフィクスチャ値が黙ってゼロ値になる)
- `Makefile`: `go vet -tags golden` を lint に追加 / `golden-generate` は説明どおり `npm ci` を実行するか未導入で明示的に失敗させる
- `tools/golden/package.json`: `^0.10.0` を `0.10.0` に完全固定
- `engine/damage.go` `chainMods`: @smogon/calc はクランプ(41/410〜131072/2097152)を持つ。現在の補正集合では到達しないが、補正追加時に再確認
- ゴールデン未カバー: リフレクターとオーロラベールの同時成立、`Effectiveness` / `STAB` の直接照合(L1 では確認済み)
