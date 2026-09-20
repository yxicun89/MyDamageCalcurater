# pokecalc

ポケモンチャンピオンズ向けのダメージ計算・構築ビルダー。
純粋 Go の計算 engine を API とブラウザ WASM から使う構成を開発中です。

現状は engine コアと OpenAPI・環境の土台まで実装されています。
ブラウザ画面、計算 API のサービス実装、逆算、WASM は未完了です。
`make up` だけで計算画面が使える状態には達していません。
進捗と次の作業は [docs/plan.md](docs/plan.md) を確認してください。

## 開発を再開する

Claude Code / Codex のどちらでも、最初に [CLAUDE.md](CLAUDE.md)、
[AGENTS.md](AGENTS.md)、[docs/plan.md](docs/plan.md) を読みます。
要件・テスト戦略・デザイン・関連 ADR と現在の差分を照合してから変更します。

```sh
git status --short --branch
git diff
git diff --cached
git log -8 --oneline
make doctor
```

main 上なら既存の未コミット変更を保持して `git switch -c fix/<内容>` 等で作業ブランチを作ります。
既存の適切な作業ブランチはそのまま使います。reset/clean 等による既存変更の破棄は禁止です。

Claude Code は `.claude/skills/` の `/phase <タスクID>`、`/improve`、`/verify` を使用できます。
Codex は同じタスク ID を指定して [共通ワークフロー](docs/development-workflow.md) に従います。
役割定義は `.codex/agents/`。単純な作業はメインだけ、重要な計算変更は独立レビューを挟みます。
`KICKOFF.md` は当初の M1 開始指示です。既存リポジトリを再初期化する手順として使わないでください。

## ツールと検証

Go の要求バージョンは `go.work` と各 `go.mod`、Node 依存は各 `package.json` を参照します。
`make doctor` が前提ツールを確認します。Docker/k3d/kubectl/Helm はクラスタを使う検証に、
TiDB 用 tiup は M2、Xcode の実機関連設定は M3 で必要です。

```sh
make test
make test-golden
make test-all-species
make lint
make build
```

ゴールデンテストは `tools/golden` の外部計算実装から生成したベクタと照合します。
期待値の再生成時は同ディレクトリで `npm ci` により lockfile に従って依存を導入し、
ルートから `make golden-generate` を実行します。通常の Go 側の照合はコミット済みベクタを使います。
[テスト戦略](docs/test-strategy.md) も参照してください。

`make lint` は Go の整形・vet と shell/Node の構文、`make build` は実装済み Go モジュールを確認します。
モジュールごとに調べる場合は次を使います。

```sh
(cd engine && go vet ./... && go build ./...)
(cd services && go vet ./... && go build ./...)
(cd tools && go vet ./... && go build ./...)
```

変更した Go ファイルは `gofmt` します。`make fmt` は広い範囲を変更するため、既存差分がある場合は
変更ファイルに絞ってください。API 変更は `api/openapi.yaml` が先で、その後 `make gen` です。

`make dev` / `make e2e` / `make wasm` / `make ios-test` / `make import` / `make assets` や
TypeScript・sqlc の生成は未実装部分があります。Makefile・スクリプトの内容を確認してください。
正常終了でも未実装メッセージやテスト 0 件を合格として記録しません。
Web の package.json とアプリが整備されたら、そこに定義された test/lint/typecheck/build を確認します。

## 構成

| パス | 役割 |
|---|---|
| `api/openapi.yaml` | API 契約の唯一の正 |
| `engine/` | 純粋 Go の計算ロジックとテスト |
| `services/` | Go サービス用モジュール、OpenAPI 生成コード |
| `tools/golden/` / `testdata/golden/` | 外部実装によるテストベクタ生成・照合データ |
| `web/` / `ios/` | クライアントの予定配置先 |
| `deploy/` / `scripts/` | k3d/Kustomize と開発補助 |
| `.claude/` / `.codex/` | 各ツールのエージェント・ワークフロー設定 |
| `docs/` | 要件、設計、テスト戦略、進捗、ADR、引き継ぎ |

Claude からの任意の外部 Codex レビューには `scripts/codex-review.sh` を使います。
ログイン等でスキップしたレビューを実施済みとは扱いません。Codex 主導時は別の reviewer が確認し、
このスクリプトによる再帰起動は行いません。
