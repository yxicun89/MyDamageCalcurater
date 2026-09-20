# CLAUDE.md

自分用ポケモン ダメージ計算 & 構築ビルダー(ポケモンチャンピオンズ対応)。学習目的のマイクロサービス構成。

Git・ブランチ・役割分担・検証・引き継ぎは [AGENTS.md](AGENTS.md) の共通運用にも従う。
Claude Code / Codex の手順対応は [docs/development-workflow.md](docs/development-workflow.md) を参照。
このファイルの絶対ルール・ドメイン規約・技術規約を共通の正として維持する。

## 最初に読むもの(この順で)

1. `docs/plan.md` — 進行状況とタスク。**作業の起点は常にここ**
2. `docs/requirements.md` — 要件の正
3. `docs/test-strategy.md` — テストの正
4. `docs/design.md` — 画面・ビジュアルの正
5. `docs/adr/` — 過去の設計判断
6. `docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md` — Claude Code と Codex の共有状態(運用は [AGENTS.md](AGENTS.md))

## リポジトリ構成

```
api/openapi.yaml        # API契約の唯一の正(仕様先行)
engine/                 # 計算エンジン(純粋Go、I/O・外部依存なし)
engine/cmd/wasm/        # ブラウザ用WASMビルド
services/gateway/       # Echo。クライアントの唯一の入口、/assets も配信
services/pokedex/       # マスタ参照(MySQL)
services/calc/          # 計算(ステートレス)
services/record/        # 計算イベント・お気に入り(TiDB: record DB)
services/team/          # 構築(TiDB: team DB)
tools/importer/         # マスタ取込(k8s CronJob)
tools/golden/           # @smogon/calc からテストベクタ生成(Node)
tools/assets/           # 画像変換・アップロード
testdata/golden/        # 生成済みテストベクタ(コミットする)
web/                    # Vite + React + TypeScript
ios/                    # SwiftUI アプリ
deploy/k8s/             # Kustomize(base / overlays/local / overlays/cloud)
scripts/                # 環境チェック・補助スクリプト
docs/
```

## 絶対ルール

1. **API変更は必ず `api/openapi.yaml` から**。手書きでハンドラやクライアントの型を作らない。変更後は `make gen`
2. **`engine/` は純粋に保つ**。DB・HTTP・ファイル・時刻・乱数の外部取得を持ち込まない
3. **計算ロジックを変更したら `make test-golden` が全件一致すること**。既知の差分は `testdata/golden/known_diffs.yaml` にADR付きで登録したものだけ許容
4. **サービスは自分のDBにだけ触る**。他サービスのデータはAPIかイベント経由
5. **計算はイベント保存に依存しない**。record/team/TiDB/NATSが落ちても計算APIは成功を返す
6. **テストを消したり弱めたりして通さない**。期待値の変更は理由をコミットメッセージに書く
7. 設計判断をしたら `docs/adr/` に1ファイル追加する
8. 1タスク = 1コミット。`docs/plan.md` のチェックを更新してからコミット

## ドメイン規約

- 対象: ポケモンチャンピオンズ、シングル優先。engine の入力には最初から `format`(single/double)を持たせる
- Lv50、個体値31固定。能力ポイント(SP)は1ステータス最大32・合計66
  - HP = 種族値 + 75 + SP / その他 = floor((種族値 + 20 + SP) × 性格補正)
- ゴールデンテストでは SP を努力値 `max(0, 8×SP−4)` に換算して @smogon/calc(gen9)と照合
- ダメージ計算は4096基準の固定小数と五捨五超入。float で近似しない
- 逆算(調整推定)は engine 内の総当たり探索(WASMでも動かすため)
- 持ち物・技・ポケモンのリストをコードにハードコードしない。マスタデータから引く
- 画像は必須にしない。無ければタイプ色エンブレムで成立すること
- 常時動くアニメーションを入れない。演出は操作時のみ

## 技術規約

- Go: Echo、oapi-codegen(echo サーバー生成)、sqlc、golang-migrate。テーブル駆動テスト
- Web: Vite + React + TS、openapi-typescript、Playwright
- iOS: SwiftUI、swift-openapi-generator、XCTest。シミュレータで `xcodebuild test`
- 認証なし。クライアントは端末ID(UUID)とセッションIDを全リクエストに付与
- 設定は環境変数。ローカルとクラウドの差分は Kustomize overlay で吸収

## コマンド(実装状況は Makefile と README を確認)

```
make doctor       # 前提ツールの確認(scripts/doctor.sh)
make up / down    # k3d クラスタ作成+全デプロイ / 削除
make dev          # k8s を使わずローカルで全サービス起動(高速な開発ループ用)
make gen          # OpenAPI / sqlc のコード生成
make test         # 実装済みユニットテスト(engine/services。Webは後続)
make lint         # Go整形/vet、shell/Node構文
make build        # 実装済みGoモジュールのビルド
make golden-generate # 外部実装の期待値を再生成(先にtools/goldenでnpm ci)
make test-golden  # engine のゴールデンテスト
make test-all-species # 全ポケモン網羅テスト
make e2e          # k3d 上のスモーク + Playwright
make ios-test     # iOS シミュレータでテスト
make wasm         # engine を WASM にビルドして web/public へ
make import       # マスタデータ取込
make assets       # 画像を WebP 2サイズに変換して MinIO へ
```

## Claude Code 固有の開発ワークフロー

Claude Code では既存の `/phase` スキルを使用し、通常タスクを以下の順で進める。
軽微な作業はメインのみでよい。engine コアの test-first 実装と独立 critic レビューは
ADR-0003 の適応を維持する。Codex では ADR-0007 と共通ワークフローの対応を使う。

| ステップ | エージェント | モデル | 役割 |
|---|---|---|---|
| 1 | quick-scanner | haiku | 関連コード・ADR・要件を探索し影響範囲を要約 |
| 2 | spec-writer | opus | 受け入れ条件とテストを先に書く |
| 3 | implementer | sonnet | テストを通す最小実装 |
| 4 | critic | opus | ルール違反・テスト漏れ・越境をレビュー。NGなら3へ(最大3回) |
| 5 | Codex(任意) | GPT系 | engine・逆算・DB設計など重要タスクのみ `scripts/codex-review.sh` で別モデルレビュー |

- 同じ失敗で3回ループしたら止まり、`docs/plan.md` の「ブロッカー」に書いて次のタスクへ
- 完了条件: `make test` 成功 / engine 変更時は `make test-golden` 成功 / plan.md 更新 / 必要ならADR
- 改善要望は `/improve`、全体確認は `/verify`
- 上表のモデルは既存 Claude agent 定義の割り当て。Codex のモデルへ機械的に置換しない
- `.claude/settings.json` の gofmt フックとは別に、明示的な整形・lint・build の結果を確認する
- レビューのスキップや未実装ターゲットの正常終了を成功と数えない。詳細は共通ワークフローを参照
- ブランチ: Phase ごとに `feat/claude-<phase名>` を切り、`/verify` 通過後に main へマージしてブランチを削除する。
  詳細は AGENTS.md「Git ブランチ運用」(Codex は `feat/codex-<stage名>`、詰まったブランチは引き継がず保留として記録)

### Codexブランチの取り込み手順

Claude Code は Codex ブランチを main に取り込むマージコーディネーターを務める。
「Codex のブランチを main に取り込んで」と指示されたときだけ、次の手順で行う。
Codex が完了を報告する前に、自分から取り込まない。共有ファイルの編集規約は AGENTS.md を参照。

1. 対象ブランチ(`feat/codex-*`)の内容を確認する(`git log main..<branch>`、`git diff main...<branch>`、
   Codex が報告したテスト結果)
2. `docs/type-balance-test-strategy.md` に沿ったテストが通っていることを確認する。
   通っていなければマージせず報告する(この文書が未作成の間は確認できないため、マージせず報告する)
3. ディレクトリが競合していないことを再確認する。Codex の変更が `services/balance/` と共有ファイルの
   編集規約が許す範囲に収まっているか(`git diff --name-only main...<branch>` で確認)。範囲外があれば報告して止まる
4. `git switch main && git merge --no-ff --no-commit feat/codex-<stage名>`
   (`--no-commit` にするのは、手順5〜6の追記をマージコミットに含め、main に `go.work` や Makefile が
   参照するだけで実体の無い中間状態を作らないため)
5. 未追記なら、`go.work` の `use` に `./services/balance` を追記し、ルート `Makefile` に
   `include services/balance/Makefile` の1行を追記する
6. `docs/ai-shared/DECISIONS.md` の末尾に「このマージで何を統合したか」を1行追記する(既存エントリは編集しない)
7. `make test` / `make lint` / `make build` を実行して成功を確認し、マージコミットを作る
8. マージ後、`docs/ai-shared/CURRENT_STATE.md` の `## Type Balance Checker` 欄を更新する(Branch・Status・Next)

コンフリクトが出た場合:

- `docs/ai-shared/CURRENT_STATE.md` / `DECISIONS.md` だけなら、両者の内容を残す形で解決する
- それ以外のファイルでコンフリクトが出たら、自動解決せず、内容を報告して止まる。
  担当ディレクトリが正しく分かれていれば通常は起きないため、起きたこと自体を異常のサインとして扱う

## 人間の確認が必要なこと(自動で進めない)

- Xcode の署名チームの設定と実機インストール
- Codex / 外部サービスのログイン
- `known_diffs.yaml` への追加(ADRを書いた上で、次の人間レビューで承認)
- クラスタ削除・DBのデータ削除
