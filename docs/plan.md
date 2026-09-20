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
- [ ] P0-1 `scripts/doctor.sh` を実行し不足ツールを報告(入れられるものは brew で入れる)
- [ ] P0-2 Go workspace(go.work)、Makefile、.gitignore、各ディレクトリの雛形
- [ ] P0-3 `api/openapi.yaml` の初版(pokedex検索 / 計算 / 一括計算 / 逆算)と `make gen`
- [ ] P0-4 k3d クラスタ定義(`deploy/k3d.yaml`)と `make up/down`、Kustomize base/overlays
- [ ] P0-5 `scripts/codex-review.sh` の動作確認(Codex 未ログインならスキップして記録)

### Phase 1 計算エンジン
- [ ] P1-1 データモデル(種族・技・持ち物・特性・性格・フィールド状態・format)
- [ ] P1-2 実数値計算(SP・性格・ランク)+ 全ポケモン網羅テスト(実数値)
- [ ] P1-3 ダメージ計算コア(基本式・乱数16段階・一致・相性・急所・やけど)
- [ ] P1-4 補正(天候・フィールド・壁・持ち物・特性)※マスタの補正定義から適用
- [ ] P1-5 確定数/乱数n発の算出
- [ ] P1-6 `tools/golden` でテストベクタ生成、`make test-golden` 全件一致
- [ ] P1-7 一括計算(防御側の代表調整すべてに対する結果を一度に返す)
- [ ] P1-8 逆算(観測ダメージ→調整候補、複数観測で絞り込み)+ 再現率テスト
- [ ] P1-9 WASM ビルド(`make wasm`)と Go/WASM の結果一致テスト

### Phase 2 マスタデータ
- [ ] P2-1 **データソース調査**: チャンピオンズの使用可能ポケモン・技・持ち物の取得元を調べ ADR-0002 に記録
- [ ] P2-2 MySQL スキーマ(migrate)と importer
- [ ] P2-3 pokedex-svc(検索・詳細・持ち物/技一覧、日本語名で前方一致)

### Phase 3 API
- [ ] P3-1 calc-svc(起動時にマスタをメモリへ読み込み)
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

## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)
