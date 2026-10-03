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
- [x] P5-3b record-svc の残作業(実装: ADR-0220。base/record・gateway 配線・`record expire` と CronJob・NetworkPolicy・/metrics)(P5-3 の critic レビューより。2026-09-25): (1) `deploy/k8s/base/record` に Deployment・Service を追加し `GATEWAY_RECORD_URL` を配線、k3d で `/api/record/*` が届く(`scripts/up.sh` のイメージビルド対象に `record` の `server` を追加)。(2) ADR-0209 §4 の失効ジョブ(日次 CronJob。生イベント90日・お気に入り540日・devices 行30日・purge journal 90日。冪等・1回の上限あり)。team 側の同等ジョブも合わせて検討
- [x] P5-4 team-svc
- [x] P5-4b team-svc の残作業(実装: ADR-0220。P5-3b と同じ形)(P5-3b と対): (1) `deploy/k8s/base/team` に Deployment・Service を追加し `GATEWAY_TEAM_URL` を配線、k3d で `/api/team/*` が届く(`scripts/up.sh` に `team` を追加)。(2) ADR-0209 §4 の失効ジョブ(構築540日・devices 行30日・purge journal 90日を `TEAM_*` 環境変数で判定。冪等・1回の上限あり)。P5-3b と同じ形なので一緒に実装してよい
- [ ] P5-5 Web: 履歴・よく計算する相手・構築ビルダー(Showdown 形式のインポート/エクスポートを含む。requirements.md §2・ADR-0213 §4。ADR-0209 §8 の文言と「この端末のデータを削除」の UI を含む)。PR 単位に分割(Web レーン)。Showdown 形式の変換部は判定レーンが `web/src/team/showdownFormat.ts` で担当
  - [x] **P5-5a 構築ビルダーの骨格(PR-A1。2026-09-26)**: 構築の一覧・新規作成(名前だけ)・名前変更・削除。ADR-0309。詳細は plan-archive.md。
  - [x] **P5-5b メンバー編集(PR-A2)**: 6体の枠と個体(種族検索・技・持ち物・特性・性格・SP のグリッド・テラスタイプ)。
    マスタ(種族・技・持ち物・特性の名前解決)を使うのはここから。実装: `web/src/team/` の `TeamMemberEditor`・`TeamMemberFields`・
    `teamMember`(純粋関数)・`teamMemberOptions`。update 全置換・応答待ち・失敗時は下書き保持・SP は明示エラー。ADR-0316。
    Web: vitest 全件・typecheck・lint と e2e(`web/e2e/team.spec.ts`)
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
- [x] issue 309 判定画面の技を select(種族の learnset)に、調整をプリセット(無振り・最速・攻撃特化・HB/HD特化)に、SP6欄・ランク5欄を「詳細」に畳み、検証エラーを欄ごとに aria-invalid+文言で出す(ADR-0711。ADR-0705 §5 を置き換え)。critic PASS・PR #480

## AJ: 調整(ダメージ計算レーン。設計は ADR-0150。2026-10-01 ユーザー要望)

「1つのアプリで調整まで完結」させる。既存の逆算(観測→相手の SP 推定)・判定(SP 固定での勝敗)とは向きが違い、
**自分の SP を決める**機能。確認済みの方針(2026-10-01): 「16n」は **HP 実数値の 16n / 16n-1**、
探索は **engine 内の総当たり**(逆算と同じ。WASM でも動く)、画面は **新タブ「調整」に機能 2・3・4 をまとめる**、
効率の基準は **目標を満たす最小 SP と指数最大の両方**、順序は **engine → API → Web、iOS は後続**。

- [x] AJ0 ADR-0150(指数の定義・16n ライン・「効率」の定義・探索の入出力)。指数式は spec-writer が既存の通説と照合して確定する
- [x] AJ1 engine: 火力指数・耐久指数(物理 H×B / 特殊 H×D)・HP の 16n / 16n-1 ライン(現在の HP SP と、次の/前のラインまでの SP 差)。純粋関数。テーブル駆動テスト
- [x] AJ2 engine: 倒せる/耐える最小 SP の探索(機能 4)。自分・相手・技・場を渡し、「確定 n 発で倒せる最小の A/C SP」「確定で耐える最小の H/B(D) SP」を返す。乱数込みの確率しきい値(既定 100% = 確定、指定可)。常時最大振りにしない
- [x] AJ3 engine: SP 配分の提案(機能 3)。固定 SP(能力ごと)+残り SP を「耐久側(H/B/D の配分を使用者が決める。素早さは見ない)」「攻撃側(S と A/C の効率配分)」から選ぶ。結果は最小 SP の組と、指数最大の組。合計 66・各 32 の制約を守る
- [x] AJ4 `api/openapi.yaml` に調整 API を追加し `make gen`、WASM 境界(`engine/wasmapi`)に露出、calc-svc。計算はステートレス(絶対ルール 5)
- [x] AJ5 技の逆引き(機能 1): `GET /api/pokedex/moves/{key}/learners`(技→覚えるポケモン。既定のレギュレーションの使用可能集合で絞る)。pokedex の `learnsets` を逆に引く。ページング・上限は ADR-0105 の前例に倣う
- [x] AJ6 Web: 新タブ「調整」(指数・16n 表示、固定 SP、耐久側/攻撃側の選択、最小 SP の提示)。機能 1 は技選択から開けるポケモン一覧。送信ボタンでだけ呼ぶ(打鍵ごとに探索しない)
- [x] AJ7 iOS 版(Web で確認後。別タスクで切る)

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
- **失効ジョブ(record-expire・team-expire)を k3d の実データへ初めて向ける承認**(ADR-0209 の「人間の確認」・ADR-0220 未決事項 0。P5-3b・P5-4b): 既定案は「承認までは local・cloud とも CronJob を `suspend: true` のまま(自動で実データを消さない)」。承認後に `deploy/k8s/overlays/local/cronjob-expire-suspend-patch.yaml` と kustomization の `patches` を外す。承認前の確認は手動 Job(`kubectl -n pokecalc create job --from=cronjob/record-expire record-expire-manual-...`。docs/runbooks/api.md §8)。作業は止まらない。
- **P4-5 の Safari 実機確認**(仕様ブロッカーではない。作業は止めない。Chrome は 2026-09-22 に確認済み): `make web-dev` で開き、Safari で計算・逆算が動くこと、`.wasm` の MIME type(`application/wasm`)・`WebAssembly.instantiateStreaming`(失敗時は arrayBuffer にフォールバック)・キャッシュ・初回ロード(約4.6MB / gzip 1.3MB)・メモリを確認する。加えて issue #333: 375px 幅未満でタブ列を左端までスクロールし、先頭の「計算」タブが読める・押せること(`justify-content: safe center` の Safari 対応)。確認できるまで P4-5 は「実装・自動テスト済み、Safari 実機未確認」として扱う。

- **観測%の丸め方**(逆算の入力側。ADR-0010 §R2): 実機の相手 HP の減少は整数%で表示される。丸め方(切り捨てか四捨五入か)だけが未確認。確認できるまでは、どの丸めでも真値を落とさない区間で照合する(P1-12 で実装済み。作業は止まらない)。確認できたら `Observation.Matches` の1か所で区間を狭める。確認方法の例: HP が分かっている自分のポケモンで、実点数のダメージと画面の%を見比べる。

- **公開のタイミング**(R-2-9・LICENSE・issue #328): 公開するときに、クリーンコピーの作成と、第三者データを含まない状態の確認、LICENSE の決定を行う。それまでは今のリポジトリで開発を続ける。

## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)
- [ ] お気に入り(手動ピン留め)のCRUD API(requirements.md §2「あれば便利(マストではない)」)。
  `favorites`テーブル・保持期間(540日)・全削除時の件数カウントはADR-0209で設計・実装済みだが、
  作成・削除・一覧のAPI自体は未着手(ADR-0209にも「recordにお気に入りのCRUDを足すときに検証する」と
  将来課題として記述されている)。Webレーンからの問い合わせ(2026-09-25。P5-5着手時)で未実装であることを
  確認・回答済み。着手するかどうかはユーザー判断待ち(急ぎではない)
- [x] issue #271/#270(データレーンからの依頼。ADR-0121 §4・ADR-0123 §7。DECISIONS.md 2026-09-25)の API レーン
  担当分: `api/openapi.yaml` に `MasterMove.mechanisms: string[]`(必須・昇順・通常の技は空配列)と
  `CalcResult`(`BulkCalcRow.result` も同じ型)・`ReverseCandidate` への `unsupported: UnsupportedMark[]`
  (必須・印なしは `[]`)を追加(`make gen`)。pokedex-svc の内部マスタ export に `ListMoveMechanisms` を
  配線(SQL の並びに頼らずこの層で昇順ソート)、calc-svc は `sharedmaster.MoveRow.Mechanisms` にそのまま渡す
  だけ(検証は既存の `MoveMechanismsOf` が担当)。calc-svc の応答変換(`calcResultFrom`・`reverseResultFrom`)
  に `unsupportedFrom`(`engine/wasmapi` と同じ変換)を配線し、HTTP/WASM パリティテストの
  `dropEmptyUnsupported`(印を比較対象から除外する暫定処置)を削除して印も比べるようにした。
  Web の例データ(`exportSnapshot.ts`)に `mechanisms: []` を追加(Web は `unsupported` をまだ受け取らない
  設計のまま。`mapCalcResult` 等の明示的フィールド写像により自動的に弾かれる。issue #67 の前方互換どおり)。
  データレーン・Web レーン・iOS レーンへ連絡済み(iOS は生成物の再生成が必要)
- [x] issue #274/#272(iOS レーンからの提案。DECISIONS.md 2026-09-25)の API レーン担当分のうち **abilityId**:
  `BulkCalcRequest.defenderOverride.abilityId`(ADR-0126・ADR-0214)。データレーンが engine 側
  (`BulkInput.DefenderAbilities`・`ReverseInput.UnknownAbilities`。PR #402)を実装済みで、API レーンは
  `defenderOverride.abilityId`(既に採用済みの概念)をその1件として渡す配線と、`ReverseRequest.unknownAbilityId`
  (新規)・`BulkCalcRow`/`ReverseCandidate` への `abilityId`/`abilityIds`(必須)を実装。省略時は種族の全特性
  (最大3件。4件目は Showdown の特殊枠 `"S"` として落とす。ADR-0105 §5 と同じ判断)を解決して渡すため、
  1つしか特性を持たない種族は必ずその特性が効くようになる(issue の境界値の受け入れ条件を満たす)。
  一括計算・逆算の行数/候補数の上限(ADR-0208)が特性分岐で最大3倍まで増えうることを openapi.yaml と
  ADR-0208 に追記。critic レビュー予定。**残り(ranks・status の上書き)は別タスクとして残す**(このタスクの
  スコープ外。abilityId とは独立に追加できる)。入ったら iOS・Web へ連絡(生成物の再生成・追従は各レーン)
- [x] issue #272 の Web 分(ADR-0311): 計算・逆算画面に特性セレクトを追加。攻撃側(自分)は種族の特性から選び(既定は先頭)、
  防御側(相手)は「おまかせ(種族の全特性)」+各特性から選ぶ(おまかせは先頭3件を `defenderAbilities` /
  `unknownAbilities` に渡す)。WASM は候補をそのまま、API 実装は候補がちょうど1件のときだけ
  `defenderOverride.abilityId` / `unknownAbilityId` で送る。行・候補の `abilityId` / `abilityIds` を DTO に写し、
  特性で分かれた行は特性名つきの別行、まとめられた行は名前を並べて表示。`MAX_ABILITY_CANDIDATES` は
  `domain/requestLimits.ts`、選択肢・候補の組み立ては `domain/requests.ts`。iOS は別レーン
- [x] issue #274 の Web 分(ADR-0312): 計算画面に「詳細」(既定は閉じる disclosure)を追加。急所・やけど(攻撃側 burn のみ)・
  天候・フィールド・防御側の壁(3つ独立)・攻撃側のランク(選択中の技の分類で A か C を ±1、-6..+6)を入力でき、
  触った分だけ `critical` / `attacker.status` / `field` / `attacker.ranks` を要求に載せる(既定は従来とバイト同一)。
  要求の形は `domain/calcConditions.ts`、文言は `calcConditionsText`、部品は `screens/CalcConditionsPanel.tsx`。
  条件は攻守入れ替え・種族・技の変更で消さず、変えた直後は古い結果を出さない。iOS は別レーン
- [x] issue #274/#272 の API レーン担当分の残り(ADR-0216): `BulkCalcRequest.defenderOverride` に `ranks: RankBlock` /
  `status: StatusCondition` を追加(全行の防御側に一律で上書き)。engine は `BulkInput.DefenderOverride`
  (`ErrInvalidDefenderOverride`。件数上限の後・計算の前に検証し、results・rows の両ループで特性の直後に当てる)、
  wasmapi は `defenderOverride{ranks,status}`、calc-svc は `parseStatusCondition`+`ranksFromBlock`。範囲外のランクは
  400 `invalid_input`、未知の status は `invalid_enum`。防御側の状態異常は今のダメージ式に効かない(結果不変)。
  逆算には足していない(ADR-0216 §4)。Web・iOS 生成物は再生成済み
- [x] issue #284(ユーザー決定。DECISIONS.md 2026-09-25「ユーザー決定 4 件」#2)balance・speed・judge も
  gateway の後ろにまとめる: `services/gateway/internal/httpapi/routing.go` に `routeBalance`/`routeSpeed`/
  `routeJudge` と `prefixBalance`/`prefixSpeed`/`prefixJudge`(record・team と同じ前方一致・末尾スラッシュ
  必須・不一致は404の規則)を追加し、`requiresHeaderCheck` に3つとも加えて `/api/{balance,speed,judge}/*`
  にも端末ID・セッションIDの検証(ADR-0202 §4)を課すようにした(issue #236 で判明していた「Traefik 直結だと
  gateway の検証を経由しない」穴をこれで塞ぐ。balance/speed/judge 自身が持つ複製の検証〈issue #236〉は
  二重になるが害はなく、削除するかどうかは各レーンの判断のまま残す)。`server.go` に `Config.BalanceURL`/
  `SpeedURL`/`JudgeURL`(nilなら503 `upstream_unavailable`)と対応する `ReverseProxy` を追加、`main.go` に
  `GATEWAY_BALANCE_URL`/`GATEWAY_SPEED_URL`/`GATEWAY_JUDGE_URL` を追加。CORS の許可メソッドは変更なし
  (balance・speed・judge の契約〈`services/{balance,speed,judge}/api/openapi.yaml`〉はいずれも GET/POST の
  みで確認済み)。`deploy/k8s` にはbalance/speed/judge自体のDeployment/Serviceがまだ無く、gatewayの
  deployment.yamlへの実際のURL配線も record・team(P5-3b/P5-4b)と同じく別タスクとして残す(コードのみ
  今回のスコープ)。新規 `balance_speed_judge_routing_test.go` で3サービス共通のルーティング・404境界・
  ヘッダ検証・実転送を固定、既存の `TestUnroutedPathsAreNotFound` から `/api/balance/defense`(今は503に
  変わるため404の例として不適切)を削除、ADR-0202 §3 の表と関連ADR行を更新(ADR-0012の「balanceは独自の
  Ingress」の記述を更新し、ADR-0606〈issue #236 のspeed側〉への参照を追加)。
  **critic 1回目FAIL(重要2件)→修正**: (1) `/api/{balance,speed,judge}/healthz`(完全一致)がgatewayで
  ヘッダ検証必須になっており、3サービスの契約(`publicHealth`)・ADR-0600・ADR-0700の「ヘッダ不要」と食い違う
  時限爆弾だった→ `requiresHeaderCheck` にpathを渡し完全一致だけ例外にする修正+テーブル駆動テスト4本を追加。
  (2) `services/gateway/README.md` のルーティング表がrecord/team/balance/speed/judge抜けの古いままだった
  →5サービス分の行・環境変数を追加。軽微2件(ingress.yaml・manifest_test.goの「独自Ingress」コメントに
  補足、`main_test.go` の `TestEnvNames`/`TestLoadConfig`/`TestLoadConfigRejects` にrecord/team/balance/
  speed/judgeの5URLを追加してURL取り違えmutationのすり抜けを閉じた)も反映。critic 2回目レビュー予定。
  タイプバランス・素早さ・判定レーンへ、直結Ingressを撤去できる旨を連絡予定
- [x] issue #110(セキュリティ。Codex レビュー)の API レーン担当分: `POST /api/calc/bulk`・`/api/calc/reverse` の候補・観測配列に件数上限が無く、1MiB未満の小さな本文で計算量を増幅できた(2,000×2,000 で約9.4秒)。契約(`maxItems`/`uniqueItems`/`maximum`。ADR-0208)を追加し、calc-svc の生成ラッパは検証しないため(実測確認済み)自前検証をID解決・engine呼び出しより前に実装。critic PASS、実HTTPで境界値と再現手順の解消(0.9ms・engine未到達)を確認。engine/wasmapi(データレーン)・Web・iOSへの追従は DECISIONS.md に既定案付きで依頼(issue はレーンの完了までクローズしない)
- [x] issue #110 のデータレーン担当分: `engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限(presets 8・itemVariants 64・itemCandidates 64・observations 16・maxCandidates 0..128)を追加(ADR-0108)。HTTP を経由しない直接呼び出し・WASM でも計算量を増幅できないようにした。wasmapi は DTO 変換より前に同じ検査を重ねて置き、複数の違反が重なっても HTTP と同じ `invalid_input` が先に出るようにした(parity)。`MaxCandidates` の負の値は、従来「無制限」扱いだったのを ADR-0208 の契約(`minimum: 0`)に合わせて拒否するよう変更(既存テストの期待値を更新。理由は ADR-0108 決定4)。critic PASS(1往復)。Web・iOS の追従(観測16件でUI無効化・持ち物候補64件超の扱い)は ADR-0208 §4 のまま未着手
- [x] issue #148(クラウド公開前のアクセス境界・認証方針。ユーザー決定「私設サービスを維持する」)の API レーン担当分: `deploy/k8s/overlays/cloud` から gateway の Ingress を削除 patch で除去し、public Ingress/LoadBalancer/NodePort/externalIPs/hostNetwork/hostPort が無いことを構造検査+`kubectl kustomize`実描画検査の2層で固定(ADR-0210)。TLS 終端は gateway/クラスタの Ingress では行わず Tailscale(`tailscale serve`)に任せる方針を決定。端末IDが認証として機能しないこと・CORSが到達制御でないことの回帰テストを追加(`TestDeviceIDIsNotAuthentication`・`TestCORSIsNotAccessControl`・`TestContractHasNoAuthentication`)。`base`のgateway Ingress本体は local(k3d)専用として残し、先頭コメントで明記。ADR-0209 §1(クラウド公開へ進む判断)は「公開しない」で確定した旨を追記。critic PASS。運用(tailnet ACL・失効手順のrunbook)・Web/iOS(接続先をtailnet名に)への依頼はDECISIONS.mdに既定案付きで記録(issue はレーンの完了までクローズしない)
- [x] issue #106(データ・運用レーン。Codex レビュー)手動 import Job(`make import-k8s`)と定期 CronJob が同時実行できる問題: `concurrencyPolicy: Forbid` は同じ CronJob が作る Job 同士にしか効かず、`kubectl create job --from=cronjob/...` が作る独立した手動 Job とは排他しないため、共有 PVC(`pokedex-import-cache`)上の取得キャッシュ・DB 投入が競合しうる実バグだった。`tools/importer/cronjob.sh` に busybox の `flock`(非ブロッキング)を `fetch.mjs` 呼び出しより前に追加し、取得〜投入の全工程をアプリ側で排他(ADR-0109)。ロック取得失敗は既存の終了コード規約どおり終了コード1(再試行可能)にし、`cronjob-import.yaml`(podFailurePolicy・concurrencyPolicy とも既存のまま)・`services/pokedex/cmd/import`(Go CLI)・Makefile は無変更。2プロセス同時起動の統合テスト(`cronjob_lock_test.go`)を追加し、Docker(Linux・busybox flock)で実際にロックが機能することを確認済み(macOS はローカルに flock が無いため自動 Skip)。critic PASS。k3d での手動確認手順は docs/runbooks/data.md §6 に追記し、2026-09-23 に実クラスタで実施: 2つの手動 Job を同時作成し、片方が「別の import が実行中」のログで即座に終了コード1、`backoffLimit` の再試行で成功したことを確認(秘密は出力に含まれない)
- [x] issue #104(データ・運用レーン。Codex レビュー)pokedex の DB 資格情報を用途別の最小権限へ分離する: server(検索API)・importer(CronJob)・migrate(Job)がすべて root 相当の同じ資格情報(Secret `mysql-auth`/`pokedex-dsn`)を使っており、公開 HTTP Pod が侵害されると DDL・ユーザー管理まで可能だった実リスクだった。`pokedex_reader`(SELECT専用)・`pokedex_importer`(SELECT/INSERT/UPDATE/DELETE)・`pokedex_migrator`(+ CREATE/ALTER/DROP/INDEX/REFERENCES)の3ロールを作り、server/importer/migrateそれぞれに最小限のDSNだけを渡す(ADR-0110)。`services/pokedex/db.Provision`が冪等・ローテーション対応で3ユーザーを作成・GRANT(接続前に正規表現でパスワード・ユーザー名・DB名・権限を検証し、root自身を対象にする入力は拒否)。`services/pokedex/cmd/migrate`の`up`は`POKEDEX_PROVISION_DSN`があるときだけプロビジョニングしてから実際のmigrationを行う(無ければ後方互換で直接migration。ローカルmake dev/make test-dbは対象外)。`scripts/up.sh`は新規クラスタで4DSNを一度に作成、既存クラスタは無いキーだけ`kubectl patch`で追記(値をargv/ログに出さない設計に修正)。critic PASS(1往復。指摘は軽微5件、うちroot保護の抜け穴を塞ぐテスト追加・check-publishable.shの過剰な許可パターン修正・up.shのエラー握り潰し修正の3件を反映)。**実クラスタ(k3d)で実際に`make up`を実行し、`SHOW GRANTS`で3ユーザーの権限がADR決定1と過不足なく一致することを確認済み**。既存クラスタからの無停止移行(Secretへのキー追記のみ)も実地確認済み
- [x] issue #109(データ・運用レーン。Codex レビュー)pokedex HTTPサーバーにタイムアウトとgraceful shutdownを追加する: `services/pokedex/cmd/pokedex/main.go`の`runServe`が`http.ListenAndServe`を直接呼ぶだけでタイムアウト(ReadHeaderTimeout等)を一切設定せず、SIGINT/SIGTERMも購読しないため、遅い・不完全な接続がリソースを無期限に保持し、Kubernetesのrollout・node drainで処理中リクエストが即座に打ち切られていた実リスクだった(calc/gateway/balance/judgeは既に対応済みでpokedexだけが欠けていた)。`services/balance`と同じ値(readHeaderTimeout=5s・readTimeout=10s・writeTimeout=15s・idleTimeout=60s・maxHeaderBytes=16KiB・shutdownTimeout=10s)で`newHTTPServer`/`serve`/`runServe`の3層に分離(ADR-0111)。既存の`run(args) int`(サブコマンド振り分け)との名前衝突を`runServe`への改名と`runServeCmd`の新設で解消。`deployment.yaml`に`terminationGracePeriodSeconds: 30`を追加し、main.goの`shutdownTimeout`定数より長いことをハードコードせず不等式でmanifestテストに固定。critic PASS(1往復。指摘なし)。ヘッダ未完了接続の切断(生TCP接続で実測)・shutdown中のin-flightリクエスト完了・DSN非露出をすべてテストで固定し、`-race`・`-count=3`でも安定を確認。**実クラスタ(k3d)でpokedexを再ビルド・再デプロイし、`terminationGracePeriodSeconds`が実際に30になっていること・`api-smoke`が正常応答することを確認済み**
- [x] issue #112(データ・運用レーン。Codex レビュー)pokedexのDB接続プールに上限と寿命を設定する: `services/pokedex/cmd/pokedex/main.go`が`sql.Open`後に`SetMaxOpenConns`等を一度も呼ばず(Go標準の既定は無制限)、突発的な同時要求がそのままMySQL接続数に転嫁され、1 Podでも接続枠を占有しうる実リスクだった。4環境変数(`POKEDEX_DB_MAX_OPEN_CONNS`=10・`POKEDEX_DB_MAX_IDLE_CONNS`=5・`POKEDEX_DB_CONN_MAX_IDLE_TIME`=5m・`POKEDEX_DB_CONN_MAX_LIFETIME`=30m)を追加し、`services/pokedex/db.OpenPool`(プール生成を1か所に集約。P7-1のメトリクス化に備える)経由で適用(ADR-0112)。検証(open/idleは正の整数・idle<=open・durationは正値)は`sql.Open`より前、エラー文にDSNを含めない。`export`サブコマンドは`(PoolConfig).ForExport()`で`MaxOpenConns=1`に上書き(逐次処理の実態に合わせる)。`deployment.yaml`に4環境変数を既定値のまま明示し、`docs/runbooks/data.md`にreplica数を増やすときの接続予算の注記を追加。critic PASS(1往復。軽微指摘1件〈idle==openの境界値テスト追加〉を反映)。実MySQLで同時クエリがMaxOpenConnsを超えないこと(直列化の実測込み)を確認。**実クラスタ(k3d)でpokedexを再ビルド・再デプロイし、4環境変数が実際に設定されていること・`api-smoke`が正常応答することを確認済み**
- [x] issue #69(データ/APIレーン)技・持ち物検索の並びがOpenAPI契約と一致しない: `api/openapi.yaml` の `searchMoves`/`searchItems` の description が「並びは ID 順」としていたが、`services/pokedex/db/query/pokedex.sql` の `SearchMoves`/`SearchItems` は導入時(P2-3)から一貫して `ORDER BY <table>.name_ja, <table>.id`(日本語名の照合順序が正。ADR-0105 §3 に「技・持ち物は name_ja, id」と既に明記されており、SQL 側もこれに一致していた)。つまり誤っていたのは契約の説明文だけで、SQL・ADR は無変更(新規 ADR は不要)。`searchSpecies`(`dex_no, form`。SpeciesKey が固定幅ゼロ埋めのためこれは文字列としての ID 順と一致)と `listNatures`(`ORDER BY id`)は元から契約どおりで対象外。契約の description を実態(名前順・同順位は ID)に訂正して `make gen`・`make ios-gen`(絶対ルール1。iOS 生成物は getMove〈P3-7〉分も含めて追従していなかったため合わせて解消)。DB 層(`db.TestSearchMovesAndItemsOrderIsNameJaNotID`。実 MySQL で確認、`-tags mysql`)と httpapi 層(`TestSearchMovesAndItemsPreserveGivenOrderAndLimitCutsThatOrder`。ハンドラが並べ替えず、`limit` がその並びの先頭から切ることを固定。ID 順に並べ替えてから切ると集合自体が変わることを変異テストで確認済み)の両方にテストを追加。critic PASS(1往復)
- [x] issue #73(API/データレーン)OpenAPIとengineの防御プリセット集合を同期検査する: `api/openapi.yaml` の `DefenderPreset` enum と `engine.DefenderPresetCatalog()` は1対1対応が前提(`services/calc/internal/httpapi/convert.go` の `presetKeysFrom` は変換テーブルを持たず契約の列挙値をそのまま `engine.PresetKey` に型変換するだけ)だが、それを固定するテストが無かった。`services/calc/internal/httpapi/preset_sync_test.go`(`TestDefenderPresetEnumMatchesEngineCatalog`)を追加: ハードコードした一覧同士を比較する既存の `vocabulary_test.go` の流儀ではなく、契約(埋め込まれた spec。`loadContract` 経由の kin-openapi)から `DefenderPreset` の enum を直接読み、`engine.DefenderPresetCatalog()` のキー集合・順序(契約の description が「耐久が上がる順」と明記。ADR-0009 §1)と比較する(ハードコードした一覧は「足し忘れ」自体を検出できないため避けた)。変異テストで両方向(engineだけに追加・openapi.yamlだけから削除・件数一致のまま重複させて列としてだけ崩す)を実際に検知することを確認済み(確認後 revert)。critic 1回目FAIL(件数不一致を黙ってskipすると重複を見逃す穴。修正: 件数不一致を明示的な失敗にしてから列を比較)→ 修正 → 2回目相当でPASS
- [x] issue #113(Web/iOS/APIレーン)入力変更時の古い計算要求を抑止・キャンセルする、のAPIレーン連携分(「クライアントのcancel伝播」): Web/iOSは自レーン分(200ms debounce・AbortSignal/Task cancel)を完了済み(PR #174・iOS側コミット)だったが、実測で調べたところ gateway 側に見落としがあった。クライアントが要求を中断すると Go の `http.Server` が `r.Context()` を `context.Canceled` で終えるが、`services/gateway/internal/httpapi/proxy.go` の `ReverseProxy.ErrorHandler` はこれを区別せず「上流に到達できない」WARN ログを出し 503 `upstream_unavailable` を返していた(実際は上流もgatewayも正常で、クライアントが単に離脱しただけ)。`errors.Is(err, context.Canceled)` のときだけ特別扱いし、WARN ログを出さず(Debug に留める)応答も書かない(相手は既に居ない)ように修正。自前のタイムアウト(`net/http: timeout awaiting response headers`)は別のエラー文言になるため混同しないことを実測で確認。`TestClientCancelIsNotUpstreamUnavailable`(wall-clock sleep 不使用。フェイクRoundTripper版・実`http.Transport`版の両方)を追加、変異テストで実効性を確認。ADR-0202 §5 に追記(AC-G10)。**限界**: gateway→calc-svcへのcontextキャンセル伝播自体は効くが、calc-svcのハンドラ・engineはcontextを見ない(engineを純粋に保つ絶対ルール2)ため、issue本文の「calc-svc CPU消費も止める」は本修正の範囲では未達成(中断された逆算は完走する。ADR-0208の上限で最悪計算量は有界なので実害は限定的。詳細はDECISIONS.md)。issue本文の受け入れ条件・対象範囲はいずれもWeb/iOS固有かgateway/calcのtimeout値等を明示的に除外しており、この限界を残したままissue #113はWeb/iOS/APIすべてのレーン分が完了としてクローズ可
- [x] P4-17(Web/APIレーン)技のID解決の欠落を解消(ADR-0304 §3): `GET /api/pokedex/moves/batch?ids=...`(`getMovesByIds`)を新設。ADR-0304が当初推していた案A(`getSpecies.learnset`をID配列からMove実体配列に変える)は採らなかった。理由: iOS(M3)が`SpeciesDetail.learnset`を`string[]`のまま前提にした機能(CalcViewModel・ReverseViewModel・TeamEditViewModelがlearnsetをID集合として扱い、検索結果との積集合を取る)を既に出荷済みで、案Aはその完成済み機能を壊す破壊的変更になり「契約変更が小さい方」の基準に反すると判断。新設したエンドポイントは既存のlearnsetを無変更のまま、1回の呼び出しでIDの配列をMove実体の配列に解決する。ids 1〜64件(ADR-0208の前例。生成ラッパは配列のmaxItemsを検証しないため`services/pokedex/internal/httpapi/search.go`で自前検査)、見つからないIDは黙って省く、応答順はidsと同じ(DBのIN句は順序を保証しないためハンドラで並べ替え)、`getMove`と同様に既定のレギュレーションで絞らない。ルーティング(`/moves/batch`が`/moves/:key`に食われないこと)を含めテスト済み(`TestGetMovesByIds`)。ADR-0105 §3・ADR-0304 §3に追記。**ids 64件超の実データ確認はまだ行っていない**(1種族のlearnsetが64件を超える場合はWeb側で分割呼び出しが必要。詳細はADR-0304 §3・DECISIONS.md)。critic 1回目FAIL(コメント・ドキュメントの事実誤り3件。実装・テストへの指摘なし)→ 修正・推奨事項も反映 → 2回目FAIL(1: 64件で1回に収まるという未検証の約束をしていた。実データ確認できず〈k3dクラスタ停止中〉、分割呼び出し前提に書き換えて対応 2: calc-svcのエラーコードのコメントが実際と不一致〈missing_headerが正・invalid_inputは誤り〉 3: 契約のmaxItemsとGo定数64の同期テストが無かった。`api.GetSwagger()`から契約のmaxItemsを読んで期待値にする`contractQueryParamMaxItems`ヘルパーを追加し、変異テスト〈契約だけ32に変更〉で同期の実効性を確認)→ 修正済み → **3回目PASS**(重要2件を反映してcommit: Web欄のP4-17の行が「未回答」のまま古くなっていたのを解決済みに更新、`TestGetMovesByIds`に未検証だった3つの契約どおりの挙動〈マスタ未投入→200 []・重複ids→重複したまま返る・空要素は黙って省く〉のテストを追加。軽微なコメントの言い回しの訂正も反映)。Webレーンへ実装完了を連絡(learnsetの解決に使ってP4-17の技オンライン未対応を解消できる。64件超は分割呼び出しが必要である旨も伝える)
- MySQL の manifest に MYSQL_DATABASE が無く、初回起動時に pokedex DB が自動作成されない実バグを発見(データレーンが k3d に初めて実デプロイした際に発生)。deploy/k8s/overlays/local/mysql/statefulset.yaml に MYSQL_DATABASE: pokedex を追加し、layout_test.go に検知テストを追加して修正(2026-09-22)。**新規クラスタでは直るが、この修正前にすでに初期化済みの PVC は MYSQL_DATABASE の効果を受けない**(コンテナ起動時にしか実行されない仕様のため)。既存の PVC に対しては CREATE DATABASE を手動実行するしかない。docs/runbooks/data.md に一言注記するとよい
- P2-3 の critic の軽微(2026-09-22。4件。#74): 未反映は `check-publishable.sh` の `B_KEYVALUE_ALLOW` を self-test の基準リポジトリにも播く、の1件。
  pokedex 側の3件は対応済み(2026-09-25): readmodel の `maxCatalogAbilityCount` を balance の schema の maxItems と直接比べる同期テスト(balance の loader 側の定数はタイプバランスレーン) /
  nature-mismatch の Blocker の Detail に `make import-fetch` を含む復旧案内 / `TestPublicInputValidation` の 400 応答を契約検証(kin-openapi)に通す

- P2-2d の critic の軽微(2026-09-22): `cronjob_layout_test.go` の「消さない」検査を secret・statefulset・configmap にも広げる / `make lint` が kubectl に依存する(kubectl の無い環境では失敗する)/ upstream の `checkedAt` が未来でも fresh 扱い / **コンテナの中で取得スクリプト(Showdown の build 等)を実際に流した記録が無い。初回の `make import-k8s` で確かめる**

- [x] issue #76(データレーン)`services/pokedex/db/mysql_test.go` に「species_abilities.slot = 4 が入る」ことを確かめるケースを足す(P2-2c の critic の軽微。000005 は使い捨てコンテナで手動確認済みだったが自動テストが無かった): `TestConstraintsRejectInvalidRows` のスロット5拒否・特性重複拒否は負方向だけだったため、`TestSpeciesAbilitiesSlot4RoundTrip` を追加し slot 4 への挿入成功と読み戻し(`SELECT ... ORDER BY slot`)を固定。確認・ロールバックはトランザクション内(コミットして残すと、他テストの `freshDB` が呼ぶ `DownAll` が migration 000005 の down〈CHECK を 1..3 へ戻す〉で失敗するため)。migration 000005 の CHECK を一時的に `(1,2,3)` に戻して新テストが失敗することを確認した上で revert(退行検知の実効性を確認)。`-tags mysql`(使い捨て MySQL コンテナ、pin 済み `mysql:9.7.2`)・`make test`・`make lint` 成功

- [x] `scripts/check-publishable.sh --self-test` の既存の失敗2件を MT-2 で修正し、`make lint` に自己テストを追加(2026-09-22)

P1-6 独立レビューで出た軽微・任意の指摘(コードは未変更。次の engine タスクに合わせて対応を検討):
- `engine/golden_test.go`: `speciesCount` の下限アサート追加(現在は 0 だけ検査。少数種で再生成しても通ってしまう)
- `engine/golden_test.go`: `DamageInput` の json タグ明示または `DisallowUnknownFields`(フィールド改名でフィクスチャ値が黙ってゼロ値になる)
- [x] `Makefile`: `go vet -tags golden` を lint に追加 / `golden-generate` は説明どおり `npm ci` を実行するか未導入で明示的に失敗させる(#77。vet は allspecies も。`golden-generate` は `npm ci` してから生成)
- `tools/golden/package.json`: `^0.10.0` を `0.10.0` に完全固定
- [x] `engine/damage.go` `chainMods`: @smogon/calc はクランプ(41/410〜131072/2097152)を持つ。現在の補正集合では到達しないが、補正追加時に再確認(#77。同じクランプを実装し境界テストを追加)
- ゴールデン未カバー: リフレクターとオーロラベールの同時成立、`Effectiveness` / `STAB` の直接照合(L1 では確認済み。壁の同時成立は #77 で L1 の回帰テストを追加)
- [x] issue #276(担当: タイプバランス・Web。ADR-0411): API 専用の画面(タイプバランス・判定)は計算モードに関係なくオンラインのマスタを使う
      (`web/src/app/withOnlineMaster.tsx`。読めなければ日本語の案内と再試行)。balance のエラーはコードを日本語の文言に写像し、
      英語の message を出さない(`balanceErrorText`)。ヘッダーの切替の名前を「ダメージ計算の実行場所」に変更。契約・生成物の変更なし。
      判定のエラー補助行(サーバー message)は未対応(別 issue 候補)
- [x] issue #322(担当: API。ADR-0204 追記): calc-svc のマスタ本文の上限を 16MiB から 4MiB に下げた
      (`master.MaxExportBytes`。実マスタは見積り 0.5〜1MB で数倍の余裕)。上限ちょうど・+1(ErrInvalidMaster)のテストと、
      「基礎 32MiB + 4 × 上限 ≤ limits.memory の 8 割」を固定する `TestMasterBodyLimitFitsMemoryLimit` を追加。契約・生成物の変更なし。
      実マスタの export の実測(pokedex 起動が必要)と Linux コンテナでの RSS は未実施

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
- [x] issue #328 の Web 分(ADR-0314): アプリ下部のフッター(`<footer>`、main の外)に「このアプリについて」リンクを置き、
  `/about`(ADR-0300 §1 のパス連動。タブには入れない)で非公式の注記とデータの出典4件を出す。文言は `aboutText`
  (iOS の `AboutText` と一字一句同じ)、画面は `AboutScreen.tsx`。タブ列は出さず他タブは hidden で DOM に残す(入力を保つ)。
  axe(`@axe-core/playwright` 4.13.0)の検査を `e2e/a11y-about.spec.ts` に追加。マスタ・engine は使わない
- [x] issue #274 の Web 分の残り(ADR-0315): 「詳細」に防御側のランク(選択中の技の分類で B か D を ±1、-6..+6、def / spd は別保持)を追加。
  触った分だけ `defenderOverride.ranks`(5項目)を要求に載せ(既定は従来とバイト同一)、API は特性(#272)と同じ `defenderOverride` に合成、
  WASM は素通し(特性は従来どおり `defenderAbilities`)。条件の置き場は `domain/calcConditions.ts`。防御側の状態異常は式に効かないので出さない。iOS は別レーン
