# 開発計画と進行状況

凡例: `[ ]` 未着手 / `[~]` 作業中 / `[x]` 完了 / `[!]` ブロック中
作業する AI は、タスク開始時に `[~]`、完了時に `[x]` へ更新してからコミットする。
完了したタスクの経過(critic の往復・テスト件数・実装の詳細)と解決済みのブロッカーは [plan-archive.md](plan-archive.md) に移す。plan.md には、各タスクの1行の状態と未完了の受け入れ条件だけを置く。issue にした軽微指摘は issue を正とし、ここには書かない(二重管理しない)。

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
- [x] P0-1 `scripts/doctor.sh` を実行し不足ツールを報告
- [x] P0-2 Go workspace
- [x] P0-3 `api/openapi.yaml` の初版
- [x] P0-4 k3d クラスタ定義
- [x] P0-5 `scripts/codex-review.sh` の動作確認

### Phase 1 計算エンジン
- [x] P1-1 データモデル(種族・技・持ち物・特性・性格・フィールド状態・format)
- [x] P1-2 実数値計算(SP・性格・ランク)+ 全ポケモン網羅テスト(実数値)
- [x] P1-3 ダメージ計算コア(基本式・乱数16段階・一致・相性・急所・やけど)
- [x] P1-4 補正(天候・フィールド・壁・持ち物・特性)※マスタの補正定義から適用
- [x] P1-5 確定数/乱数n発の算出
- [x] P1-6 `tools/golden` でテストベクタ生成、`make test-golden` 全件一致
- [x] P1-7 一括計算(防御側の代表調整すべてに対する結果を一度に返す)
- [x] P1-8 逆算(観測ダメージ→調整候補、複数観測で絞り込み)+ 再現率テスト
- [x] P1-9 WASM ビルド(`make wasm`)と Go/WASM の結果一致テスト

### Phase R コーディング規約の策定とリファクタリング(2026-09-21 のユーザー依頼)
規約は [docs/coding-rules.md](coding-rules.md)(Claude Code と Codex 共通。公開できる状態を保つ・ハードコードしない・読みやすいコード)。Phase 1b より先に行う。
- [x] R-0 規約の策定と Codex レビュー
- [x] R-1 監査: 規約違反の洗い出し
- [ ] R-2 是正(挙動を変えない。`make test` / `make test-golden` / `make test-wasm` を維持。塊ごとに1コミット。詳細は audit-r1.md):
  - [x] R-2-1 シェルを `set -euo pipefail` に統一
  - [x] R-2-2 fixture の公式日本語名を架空名に置換
  - [x] R-2-3 `tools/golden/package.json` を `0.10.0` に完全固定、`.gitignore` に `*.wasm`
  - [x] R-2-4 ドメイン定数に名前を付けて集約
  - [x] R-2-5 小さな可読性の是正
  - [x] R-2-7 ADR の追記・修正
  - [x] R-2-8 Go の module path を公開用プレースホルダ
  - [ ] R-2-9 公開用クリーンコピーの作成(ユーザー決定: 履歴を書き換える。方式は**今のリポジトリと worktree には触れず、書き換えたコピーを別に作る**)。`scripts/make-public-copy.sh`(仮): 公開したいブランチをローカル clone → `git filter-repo` で作者名・メールを公開用 identity(`pokecalc-dev <noreply@example.com>`)に置換(`--mailmap` を既存の作者から動的に生成するので、実名をスクリプトに書かない)、履歴内の `github.com/<アカウント名>/pokecalc` を `example.com/pokecalc` に置換(正規表現)→ コピー側で `make check-publishable-full`(Git 作者の許可リストを作成)と履歴全体の検査(絶対パス・メール・秘密)。実行は**公開するとき**。前提ツール: `git-filter-repo`(`brew install git-filter-repo`。`make doctor` の任意ツールに追加)。元のリポジトリ(Codex の `pokecalc-codex-tb0`・`pokecalc-main` を含む)は変更しない
- [x] R-3 `make check-publishable`

### Phase 1b 決定の反映(2026-09-21 のユーザー決定。ADR-0002 の確定方針・DECISIONS.md 参照)
- [x] P1-10 防御プリセットの再定義
- [x] P1-13 タイプ相性表のデータ化
- [x] P1-11 表示%の分離
- [x] P1-12 逆算の再設計

### Phase 2 マスタデータ
- [x] P2-1 **データソース調査**
- [x] P2-1b ゴールデンの oracle を `@smogon/calc@0.12.0` の Champions へ切り替え
- [x] P2-1c 技の使用可否の調査
- [x] P2-2 MySQL スキーマ
- [x] P2-3 pokedex-svc

### Phase 3 API
- [x] P2-3b 無効・吸収の特性
- [x] P3-1 calc-svc(起動時にマスタをメモリへ読み込み)
- [x] P3-2 gateway(ルーティング・端末ID/セッションID・/assets・CORS)
- [x] P3-3 契約テスト(OpenAPI 準拠)と k3d 上のスモークテスト
- [x] P3-4 calc-svc のマスタを pokedex-svc の内部 API
- [x] P3-5 gateway の `GATEWAY_WEB_URL`
- [x] P3-6 calc・gateway を pokedex-svc につなぐ
- [x] P3-7 `GET /api/pokedex/moves/{key}`

### Phase 4 Web
- [x] P4-1 デザイントークン(docs/design.md)を CSS 変数に実装
- [x] P4-2 計算画面(左右カード・持ち物・技・結果の一括表示・攻守入れ替え)
- [x] P4-3 プリセット選択
- [x] P4-4 逆算画面(観測ダメージ入力→候補リスト)
- [x] P4-5 API / WASM 切り替え。実装・自動テスト済み(ADR-0301)。Chrome・Safari とも人間が確認済み〈2026-10-03〉
- [x] P4-6 Playwright E2E
- [x] P4-7 **M1 完了報告**

- [x] P4-8 design.md「動き」の演出

- [x] P4-9 P4-8 の軽微な改善

- [x] P4-10 URL で画面を切り替える
- [x] P4-11 Web をコンテナで動かす
- [x] P4-12 タイプバランスの画面 `/balance`
- ~~P4-13 素早さ比較の画面~~ → 取り消し(ユーザー決定 2026-09-22。素早さレーンの SP3 のまま)
- [x] P4-14 手順書の書き方の改善
- [x] P4-15 `make gen-ts` の再生成
- [x] P4-16 Web のオンライン MasterSource の基盤
- [x] P4-16b Web のオンライン MasterSource の画面側
- [x] P4-16c P4-16b の critic 指摘で見送った残り
- [x] P4-17 技の ID 解決
- [x] P4-17b BalanceScreen
- [x] P4-18
- [x] P4-21 Codex コードレビューの次点 issue
- [x] P4-19 issue #110
- [ ] P4-20 issue #148(クラウド公開前のアクセス境界。ADR-0210。API レーン分は PR #157 で完了): Web 側のコード変更は不要(`apiBaseUrl()` の既定は同一オリジン、CORS は gateway 側)。残りは、運用側が到達経路(Tailscale Operator か subnet router + tailscale serve)を選んだあと、MagicDNS 名を `VITE_API_BASE_URL` にデプロイ時設定する運用作業(issue #285)
- [x] P4-22 issue #72
- [x] `make e2e` の `web-e2e-online` 修復・PR1
- [x] `make e2e` の `web-e2e-online` 修復・PR2
- [x] issue #71 の Web 側
- [x] issue #333(375px幅でタブの名前が1文字ずつ縦に折り返す)
- [x] issue #306
- [x] issue #275
- [x] issue #304
- [x] issue #308
- [x] issue #305
- [x] issue #248
- [x] issue #218
- [x] issue #271 / issue #270
- [ ] 判定画面(JD5 `JudgeScreen`)の「未対応」の印への追従(issue #271 / #270 の判定レーン分。**上の
  Web レーンの PR の対象外**)。judge の契約は計算・逆算と別の形(`attackerKoUnsupported` /
  `defenderKoUnsupported`。ADR-0708 §1・`web/src/judge/judge.gen.ts`)なので、別タスクとして進める。
  文言(`unsupportedText`)と表示の作法(色だけに頼らない・`role="status"` の案内)は上のものを再利用する。

## M2: 保存・構築

**P5-1〜P5-4 の前提(先に決めた設計。issue #103・ADR-0209「M2 保存データの保持・削除・端末 ID 境界」に従う)**:
端末 ID は認証ではなくデータの分割キー / 生の計算イベントは作成から90日・構築とお気に入りは `max(devices.last_seen_at, 行.updated_at)` から540日で失効 /
端末単位の全削除はサービスごとに1本(`DELETE /api/record/device-data`・`DELETE /api/team/device-data`。冪等・`partial` の繰り返し)/
削除の墓石(`devices.purged_at`)で JetStream の遅延イベントの復活を防ぐ。受け入れ条件は ADR-0209 の AC-D / AC-P / AC-R / AC-L。

- [ ] P5-1 TiDB(ADR-0211。範囲は `devices`・purge journal の2表とプロビジョニングまで): スキーマ/migrate CLI(PR #205)と TidbCluster・TidbInitializer のマニフェストは実装済み。**残り**: 共有 k3d への実適用(AC-T3・AC-T8: TidbCluster・TidbInitializer が Ready/Completed になること)と、`tnir/mysqlclient`(amd64 専用)の Apple Silicon での起動可否の確認。purge journal の DB 外の保存先は P7-4 が決めるまで未充足(ADR-0209 追記・ADR-0211 §6)
- [x] P5-2 NATS JetStream と calc-svc からのイベント発行
- [x] P5-3 record-svc
- [ ] P5-3b record-svc の残作業(P5-3 の critic レビューより。2026-09-25): (1) `deploy/k8s/base/record` に Deployment・Service を追加し `GATEWAY_RECORD_URL` を配線、k3d で `/api/record/*` が届く(`scripts/up.sh` のイメージビルド対象に `record` の `server` を追加)。(2) ADR-0209 §4 の失効ジョブ(日次 CronJob。生イベント90日・お気に入り540日・devices 行30日・purge journal 90日。冪等・1回の上限あり)。team 側の同等ジョブも合わせて検討
- [x] P5-4 team-svc
- [ ] P5-4b team-svc の残作業(P5-3b と対): (1) `deploy/k8s/base/team` に Deployment・Service を追加し `GATEWAY_TEAM_URL` を配線、k3d で `/api/team/*` が届く(`scripts/up.sh` に `team` を追加)。(2) ADR-0209 §4 の失効ジョブ(構築540日・devices 行30日・purge journal 90日を `TEAM_*` 環境変数で判定。冪等・1回の上限あり)。P5-3b と同じ形なので一緒に実装してよい
- [ ] P5-5 Web: 履歴・よく計算する相手・構築ビルダー(Showdown 形式のインポート/エクスポートを含む。requirements.md §2・ADR-0213 §4。ADR-0209 §8 の文言と「この端末のデータを削除」の UI を含む)。PR 単位に分割(Web レーン)。Showdown 形式の変換部は判定レーンが `web/src/team/showdownFormat.ts` で担当
  - [x] 
  - [ ] **P5-5b メンバー編集(PR-A2)**: 6体の枠と個体(種族検索・技・持ち物・特性・性格・SP のグリッド・テラスタイプ)。
    マスタ(種族・技・持ち物・特性の名前解決)を使うのはここから
  - [ ] **P5-5c 履歴・よく計算する相手・端末データの削除(PR-A3 以降)**: record-svc の API と ADR-0209 §8 の文言
- [x] P5-6 技の追加効果

- [x] issue #219(Web 配信にセキュリティヘッダが無い)

- [x] issue #332 の Web 分

## M3: iOS
- [x] P6-1 Xcode プロジェクト、swift-openapi-generator、デザイントークン
- [x] P6-2 計算画面・逆算・構築
- [x] P6-2d 構築から個体を呼び出す配線
- [x] P6-3 シミュレータテスト
- [x] P6-4 Tailscale serve の手順書 `docs/runbooks/ios-device-install.md` を作成 →
- [x] P6-5 issue #113
- [x] P6-6 issue #110
- [x] P6-9 issue #68 の残り
- [x] P6-10
- [x] P6-7 ADR-0209 §8 の文言と「この端末のデータを削除」の UI
- [x] P6-11 issue #334
- [x] P6-12 issue #71 の iOS 側
- [x] P6-13 issue #274
- [x] P6-14 最大の文字サイズ
- [x] P6-15 P6-14 の残り
- [x] P6-16 issue #250
- [x] P6-8 issue #99
- [x] P6-17 issue #271/#270 の iOS 側
- [x] P6-18 issue #328
- [x] P6-19 issue #272 の iOS 側

## TB: タイプバランスチェッカー(タイプバランスレーン。設計は docs/type-balance-design.md)
- [x] TB0 基盤(型・相性コア・HTTP・Docker/Kustomize・Argo CD・単体テスト)
- [x] TB1 防御タイプバランス
- [x] TB1b 相性表を P1-13 のデータ
- [x] TB2 攻撃範囲(ADR-0016)
- [x] TB3 特性
- [x] TB4 仮想敵診断(ADR-0400)
- [x] TB5 おすすめタイプと該当ポケモン
- [x] TB 整備(2026-09-22)
- [x] TB 実データの配線
- [x] TB6 技範囲チェッカー
- [x] Codexレビュー issue #105 対応
- [x] 全体レビュー issue #263・#292 対応
- [x] issue #298

### ブロッカー(タイプバランスレーン)
(なし。Argo CD の実同期は 2026-09-22 に解消)

## SP: 素早さ比較(素早さレーン。設計は docs/speed-design.md。2026-09-22 ユーザー要望)
ユーザーの仕様(確定):
- 3つ目の機能・サービスとして独立して作る(`services/speed/`。ダメージ計算・タイプバランスと並ぶ)。Web に独立した画面(タブ)を置く。iOS は後で
- 画面は左右に配置する。**左 = 全体の表**(最初から速い順に並べた表)、**右 = 自分のポケモン**。自分の実数値が左の表のどこに入るかを視覚的に示す
- 左の表は、使用可能な各ポケモンについて **6 行**: 無振り / 準速(素早さ SP 32・補正なし)/ 最速(SP 32・素早さ上昇性格)/ 最速+こだわりスカーフ / 最速+1(ニトロチャージ等)/ 最速+2(こうそくいどう等)。道具・ランクで絞り込める
- 右の入力は**最小の選択で計算できる**こと: ポケモンを選び、「無振り / 準速 / 最速」を選んで、こだわりスカーフの on/off を切り替えるだけ。**オプション**で好きな数値でも算出できる
- 実数値は Champions の式(その他 = floor((種族値 + 20 + SP) × 性格補正))、Lv50・個体値31固定。スカーフ・ランクの掛け方は engine / Showdown の規則に従う
- 2026-09-22 ユーザー回答(確定): 右のオプションは素早さ SP 0〜32・性格の補正3通り・ランク -6〜+6・スカーフ on/off を自由に選ぶか、実数値を直接入力して位置だけを見る。同じ実数値は同速としてまとめて表示(同速の中は図鑑番号順)。表に載せるのは既定のレギュレーションの使用可能集合。Web の骨組み(`web/`)が無い間は `web/src/speed/` の画面部品とテストだけ先に作り、タブ登録は骨組みができてから1項目足す
- [x] SP0 基盤
- [x] SP1 素早さの表
- [x] SP2 自分のポケモンの位置
- [x] SP3 Web の素早さ画面
- [x] SP4 pokedex の read model
- [x] SP5 GitOps
- [x] SP6 追い風・まひ・トリックルーム(ADR-0607。契約 0.5.0・コア・HTTP・Web。iOS は対象外)
- [x] issue #237 gitops overlay の read model

## JD: 判定(判定レーン。設計は docs/judge-design.md。2026-09-22 ユーザー要望)
「ニトチャ+メイン技で素早さ抜ける+そのポケモンを倒せるか」を1回の入力で確認する。engine を直接呼び、pokedex-svc と calc-svc の公開 API だけに依存する(speed-svc には依存しない)。
- [x] JD0 基盤(ディレクトリ構成・pokedex-svc/calc-svc への HTTP クライアント・ヘルスチェック)
- [x] JD1 抜けるか+倒せるかの最小構成
- [x] JD2〜JD5 の範囲・順序をユーザーに確認
- [x] JD2 場の効果(トリックルーム・追い風)
- [x] JD3 複数の相手候補を一度に判定
- [x] JD4 相手の技を含めた返り討ち判定
- [x] JD5 Web の画面
- [x] issue #234 moveId/natureId の形式検証が無く、制御文字などが上流 URL にそのまま埋め込まれ、503
- [x] issue #213(重大度 high)上流(pokedex-svc/calc-svc)が遅いと judge は1リクエスト全体の期限を持たず
- [x] issue #329(重大度 low)SP 合計超過(67)の拒否を確かめる回帰テストが無く、`validateSP` の
- [x] issue #257(重大度 low)`services/judge/scripts/smoke.sh` が healthz しか叩かず、k3d 上で
- [x] issue #260
- [x] issue #260 のタイプバランス分
- [x] 判定の応答に calc-svc の「未対応」の印を中継する

## AJ: 調整(ダメージ計算レーン。設計は ADR-0150。2026-10-01 ユーザー要望)

「1つのアプリで調整まで完結」させる。既存の逆算(観測→相手の SP 推定)・判定(SP 固定での勝敗)とは向きが違い、
**自分の SP を決める**機能。確認済みの方針(2026-10-01): 「16n」は **HP 実数値の 16n / 16n-1**、
探索は **engine 内の総当たり**(逆算と同じ。WASM でも動く)、画面は **新タブ「調整」に機能 2・3・4 をまとめる**、
効率の基準は **目標を満たす最小 SP と指数最大の両方**、順序は **engine → API → Web、iOS は後続**。

- [x] AJ0 ADR-0150(指数の定義・16n ライン・「効率」の定義・探索の入出力)。指数式は spec-writer が既存の通説と照合して確定する
- [x] AJ1 engine: 火力指数・耐久指数(物理 H×B / 特殊 H×D)・HP の 16n / 16n-1 ライン(現在の HP SP と、次の/前のラインまでの SP 差)。純粋関数。テーブル駆動テスト
- [x] AJ2 engine: 倒せる/耐える最小 SP の探索(機能 4)。自分・相手・技・場を渡し、「確定 n 発で倒せる最小の A/C SP」「確定で耐える最小の H/B(D) SP」を返す。乱数込みの確率しきい値(既定 100% = 確定、指定可)。常時最大振りにしない
- [x] AJ3 engine: SP 配分の提案(機能 3)。固定 SP(能力ごと)+残り SP を「耐久側(H/B/D の配分を使用者が決める。素早さは見ない)」「攻撃側(S と A/C の効率配分)」から選ぶ。結果は最小 SP の組と、指数最大の組。合計 66・各 32 の制約を守る
- [ ] AJ4 `api/openapi.yaml` に調整 API を追加し `make gen`、WASM 境界(`engine/wasmapi`)に露出、calc-svc。計算はステートレス(絶対ルール 5)
- [ ] AJ5 技の逆引き(機能 1): `GET /api/pokedex/moves/{key}/learners`(技→覚えるポケモン。既定のレギュレーションの使用可能集合で絞る)。pokedex の `learnsets` を逆に引く。ページング・上限は ADR-0105 の前例に倣う
- [ ] AJ6 Web: 新タブ「調整」(指数・16n 表示、固定 SP、耐久側/攻撃側の選択、最小 SP の提示)。機能 1 は技選択から開けるポケモン一覧。送信ボタンでだけ呼ぶ(打鍵ごとに探索しない)
- [ ] AJ7 iOS 版(Web で確認後。別タスクで切る)

## DOC: 文書(全レーン。docs/coding-rules.md §8。2026-09-22 ユーザー要望)
各レーンが自分の範囲の README(何をするか・mermaid の構成図・ディレクトリ・コマンド・関連 ADR。80 行以内)と、動かして確かめられるレーンは手順書(`docs/runbooks/<レーン>.md`。AGENTS.md「手順書の書き方」に従う)を書く。全体図は `docs/architecture.md`。
- [x] DOC-data
- [x] DOC-api: `services/calc/README.md`・`services/gateway/README.md` を §8 の形に、手順書
- [x] DOC-web: `web/README.md`、手順書
- [x] DOC-tb: `services/balance/README.md` を §8 の形に、手順書 `docs/runbooks/balance.md`
- [x] DOC-speed
- [x] DOC-ios: `ios/README.md`
- [x] DOC-arch

## M4: 運用
- [x] P7-1 kube-prometheus-stack / Loki、各サービスのメトリクス
  - [x] issue #293 の残り(2026-10-02): 一次切り分けの runbook(observability.md §7)と `make k8s-render` に base/observability
- [x] issue #299(タイムアウトの連鎖。ADR-0801): calc・balance・speed にハンドラ全体の締め切り(writeTimeout − 1 秒)と同時実行の上限(超過は待たせず 503 + Retry-After)、judge・pokedex に上限、k3d の Traefik に有限のタイムアウト(`scripts/up.sh` が適用)。Docker 負荷試験(同時 120 で EOF 0 件)はメインでの実地確認。issue #330(httpmetrics の複製のずれ検出)は先行コミット a8db4fa で解消済み
- [x] P7-2 SLO(計算API p99 < 100ms、可用性)とダッシュボード
- [~] P7-3 ArgoCD(GitOps): balance は Argo CD 管理。speed・judge の実クラスタ適用は人間確認待ち(CURRENT_STATE.md)。残りは issue #292・#263・#237(NetworkPolicy の balance・speed → mysql は base に反映済み=ADR-0412 追記。残りは共有クラスタへの apply の人間確認と pokedex の実 digest 確定)
- [ ] P7-4 MySQL/TiDB バックアップと復元テスト(ADR-0209 §9 を要件に含める: バックアップに `devices`〈墓石〉を含める /
  purge journal(#5b。世代取得後の削除要求。保持90日)をバックアップ世代と別に保持し復元時に再適用 /
  Ready の前に墓石の再適用・purge journal の再適用・失効ジョブの強制実行 / JetStream は再生しない / 世代30日。
  受け入れ条件は AC-B1〜B3・AC-B2b)

## 後続: 要件との対応(issue #286。M1〜M4 の後。担当レーン付き)
requirements.md の項目のうち、計画に無かったものをここに置く。着手の順・可否はユーザー判断(急ぎではない)。
- [ ] P5-3c お気に入り(手動ピン留め)の作成・削除・一覧 API と画面(requirements.md §2「あれば便利」。担当: API レーン→ Web・iOS。`favorites` の表・保持期間・全削除の件数は ADR-0209 で実装済みで、API・画面が未着手。ADR-0209 の「record にお気に入りの CRUD を足すときに検証する」を併せて行う)
- [ ] P6-20 iOS の Showdown 形式のインポート/エクスポート(requirements.md §2 は必須。P6-2 で後回しにしたまま。担当: iOS レーン。P5-4 の後。Web は P5-5・team-svc は P5-4)
- [ ] P8-1 ポケモン画像の配信(任意。M1 の後。requirements.md「ポケモン画像」: MinIO・gateway の画像パス・`manifest.json`・`make assets`・無ければタイプ色のエンブレム。担当: 運用(deploy・scripts)+ API + Web。gateway の予約パス `/assets/*` は未設定で常に 404 なので `/images/` に移す〈issue #286 所見1〉。`make assets` は実装まで終了コード 2 のスタブ)
- 公開時の名称・画像の差し替え構造(requirements.md「知財」): **後回し**。公開のタイミング(R-2-9・LICENSE・issue #328。ブロッカー節)と同時に決める。画像は P8-1 でキー(`{図鑑番号4桁}-{フォルム3桁}`)による差し替え構造になる

## ブロッカー
解決済みの記録は [plan-archive.md](plan-archive.md)。未解決のものだけをここに置く(issue があるものは issue を正とする)。


**【人間の確認待ち】**
- **公開のタイミング**(R-2-9・LICENSE・issue #328): 公開するときに、クリーンコピーの作成と、第三者データを含まない状態の確認、LICENSE の決定を行う。それまでは今のリポジトリで開発を続ける。

## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)
- [x] issue #271/#270
- [x] issue #274/#272
- [x] issue #272 の Web 分
- [x] issue #274 の Web 分
- [x] issue #274/#272 の API レーン担当分の残り
- [x] issue #284
- [x] issue #110
- [x] issue #110 のデータレーン担当分
- [x] issue #148
- [x] issue #106
- [x] issue #104
- [x] issue #109
- [x] issue #112
- [x] issue #69(データ/APIレーン)技・持ち物検索の並びがOpenAPI契約と一致しない
- [x] issue #73(API/データレーン)OpenAPIとengineの防御プリセット集合を同期検査する
- [x] issue #113(Web/iOS/APIレーン)入力変更時の古い計算要求を抑止・キャンセルする、のAPIレーン連携分(「クライアントのcancel伝播」)
- [x] P4-17(Web/APIレーン)技のID解決の欠落を解消(ADR-0304 §3)
- [x] issue #276
- [x] issue #211(API レーン分。ADR-0218): 公開 API の Item / Ability に省略可の `effect` を足した(searchItems・getSpecies.abilities。共通マスタで厳格に検証し、不正は 503 master_unavailable。内部 API は変更なし。critic PASS)

- [x] issue #236 の balance 分(ADR-0413。X-Device-Id/X-Session-Id を gateway と同じ正準 UUID 検証に。openapi 0.8.0)
- [x] issue #210 の Web 分(ADR-0313): 既定の計算モードをオンラインに変更(ユーザー決定 2026-10-01。保存済みのモードは尊重)。
  オンラインで取得した持ち物・性格と、解決した種族・特性・技を IndexedDB に保存(`master/cache/`。`MasterCacheStore`・
  スキーマ版つき。書き込み・読み出しの失敗は握りつぶす)し、オフラインはそのキャッシュだけから読む(オンラインを呼ばない・
  架空データを出さない。空・壊れ・版違いは `appText.masterCacheEmptyError` の案内+再試行)。`main.tsx` は例データをやめて
  `createCachedMasterSources` に差し替え。オフラインでは持ち物の候補比較は選べない(公開 API に効果データが無い)。
  実装中に見つけた退行も直した: 攻守入れ替えで種族の検索欄の名前が追従しない(`SpeciesSearchField.selectedNameJa`)、
  種族の解決待ちの間に打った逆算の観測が計算に反映されない(`ReverseScreen` の `latestObservationsRef`)。
  E2E は pokedex フィクスチャでオンライン→オフラインを確かめる(コンテナは CSP の下の WASM 計算を `container.spec.ts` だけで確認)
- [ ] issue #274/#272 の API レーン担当分の残り: `defenderOverride.ranks: RankBlock` / `defenderOverride.status:
  StatusCondition`(全行に一律で上書き)。abilityId(上記)とは独立に追加できる。engine 側の変更
  (`BulkInput`/`ReverseInput` へのオーバーライド追加。プリセット解決後・計算前に当てる)を伴うため
  ADR-0003 の test-first + 独立 critic の対象。優先度は低い(iOS レーンから「急ぎではない」と明記済み)
