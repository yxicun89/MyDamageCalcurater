# pokecalc

ポケモンチャンピオンズ向けのダメージ計算・構築ビルダー。純粋 Go の計算 engine を、API(Go のマイクロサービス)と
ブラウザ WASM の両方から使います。

## 何が動くか

- ブラウザ(Web): ダメージ計算・逆算・タイプバランス・素早さ比較・判定・構築ビルダー。オフライン(WASM。架空の例データ)とオンライン(API。実データ)の両方
- サービス: gateway・calc・pokedex(マスタ取込を含む)・balance・speed・judge・record・team(k3d。各レーンの overlay は docs/runbooks/)。`make dev` が起動するのは calc と gateway だけ
- iOS アプリ(SwiftUI。`make ios-test`)
- 未実装: `make assets`(画像の変換・配信。終了コード 2)と、k3d の有無で変わる E2E の一部(クラスタ分は `make up` が前提)
- 進捗と残りの作業は [docs/plan.md](docs/plan.md)(状態の正)。要件・テスト・設計の入口は [docs/README.md](docs/README.md)

## 起動する

最短(バックエンド不要。WASM で計算):

```sh
cd "$(git rev-parse --show-toplevel)"
make doctor
./scripts/setup-git.sh
make web-install
make web-dev
```

`./scripts/setup-git.sh` は clone ごとに1回です(iOS の API 生成物の衝突を再生成で解く merge ドライバを登録する)。
API 契約・SQL から作る Go・TypeScript の生成物は Git に置きません。`make` の各ターゲットと Web の
`npm run dev`・`build`・`test` 等が先に生成します(ADR-0171)。

表示された URL をブラウザで開きます。k3d(`make up` → `http://localhost:8080`)・iOS・DB を使う手順は
[docs/verify-m1.md](docs/verify-m1.md) と [docs/runbooks/](docs/runbooks/) を上から実行してください。

## データの扱い(第三者の著作物)

- 第三者由来の Pokémon マスタデータ(種族値・技・持ち物・日本語名など)、生成済みスナップショット、公式画像は、本リポジトリでは配布しません。
  `data/generated/` は `.gitignore` の対象です。
- 実行には、所定の schema に従ったデータを利用者側で用意する必要があります(importer と schema はこのリポジトリにあります)。
- Git 管理するのは、ソースコード、importer、schema、架空データを使った example、README、データの生成元と版を示す metadata です。
- 方針と経緯は [ADR-0002](docs/adr/0002-master-data-source.md) を参照してください。

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

main 上なら既存の未コミット変更を保持して `git switch -c feat/claude-<phase名>` / `feat/codex-<stage名>` 等(命名は AGENTS.md「Git ブランチ運用」)で作業ブランチを作ります。
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
期待値の再生成はルートから `make golden-generate` を実行します(lockfile に従う `npm ci` も行います)。通常の Go 側の照合はコミット済みベクタを使います。
[テスト戦略](docs/test-strategy.md) も参照してください。

`make lint` は Go の整形・vet と shell/Node の構文、`make build` は実装済み Go モジュールを確認します。
モジュールごとに調べる場合は次を使います。

```sh
make gen                                          # 生成物は Git に置かない。go を直接使う前に1回(ADR-0171)
(cd engine && go vet ./... && go build ./...)
(cd services && go vet ./... && go build ./...)   # balance・speed・judge は別モジュール。make lint / make build が検査する
(cd tools && go vet ./... && go build ./...)
```

変更した Go ファイルは `gofmt` します。`make fmt` は広い範囲を変更するため、既存差分がある場合は
変更ファイルに絞ってください。API 変更は `api/openapi.yaml` が先で、その後 `make gen` です。
`gen_required.go` で `undefined: …` のコンパイルエラーが出たら生成物がありません(`scripts/ensure-gen.sh check` が欠けている
ファイルを一覧にします)。iOS の生成物だけは追跡を続けるので、`make ios-gen` の結果をコミットします。

`make e2e` は常時3件(`web-e2e`・`web-e2e-online`・`web-e2e-balance`。k3d クラスタ不要)の Playwright を必ず実行し、
kubectl の現在のコンテキストが `k3d-$CLUSTER` のときだけ既存クラスタが要る3件(`api-smoke`・`web-k3d-smoke`・`web-k3d-e2e`)
を追加で実行します。クラスタが無ければスキップして成功しますが、`E2E_REQUIRE_K3D=1` を付けるとスキップせず失敗で
終わります(リリース前などクラスタ分まで確かめたいとき用)。詳細は [ADR-0306](docs/adr/0306-root-e2e-wiring.md)。

正常終了でも未実装メッセージやテスト 0 件を合格として記録しません(未実装は `make assets` のみ)。

## 構成

| パス | 役割 |
|---|---|
| `api/openapi.yaml` | ダメージ計算・gateway・pokedex 等の API 契約(balance・speed・judge は `services/<名前>/api/openapi.yaml`。DECISIONS 2026-09-21) |
| `engine/` | 純粋 Go の計算ロジックとテスト |
| `services/` | Go サービス用モジュール、OpenAPI 生成コード |
| `tools/golden/` / `testdata/golden/` | 外部実装によるテストベクタ生成・照合データ |
| `web/` / `ios/` | クライアント(Vite + React + TypeScript / SwiftUI) |
| `deploy/` / `scripts/` | k3d/Kustomize と開発補助 |
| `.claude/` / `.codex/` | 各ツールのエージェント・ワークフロー設定 |
| `docs/` | 要件、設計、テスト戦略、進捗、ADR、引き継ぎ |
