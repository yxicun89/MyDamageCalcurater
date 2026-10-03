# ADR-0807: API 契約・SQL から作る生成物を Git に置かず、使う前に生成する(iOS を含む)

- 状態: 採用
- 日付: 2026-10-03
- レーン: 運用(`0800〜`)
- 関連: ADR-0100 §1(sqlc)、ADR-0119(CI)、ADR-0301 §7(Web の生成型)、ADR-0415(iOS の balance 生成)、
  ADR-0500 §2(iOS の生成)、ADR-0604(speed の Web 型)、ADR-0705(judge の Web 型)、
  docs/ai-shared/COORDINATION.md「生成物を追跡から外したあとの取り込み」
- 置き換え: ADR-0100・ADR-0301 §7・ADR-0415・ADR-0500 §2・ADR-0604・ADR-0705 の「生成物はコミットする」
  (各 ADR に置き換えの追記を入れた)

## 背景

`api/openapi.yaml`・各レーンの `openapi.yaml`・pokedex の SQL から作る生成物(oapi-codegen・sqlc・
openapi-typescript・swift-openapi-generator)を Git にコミットしていた。API を変える PR が並ぶと、
生成物どうしが毎回衝突する(数千行の生成ファイルの衝突を手で解くことになる)。また再生成を忘れた PR が
main に入り、`make ios-gen-check` が赤になった(2026-10-03)。

ユーザー決定(2026-10-03): 生成物のコミットをやめ、ビルド時に生成する(生成物は ignore)。

## 決定

### 1. 生成物はすべて追跡しない(iOS を含む)

対象の一覧の正は `scripts/ensure-gen.sh list`:

| 生成物 | 入力 | 生成器(版の固定) | 生成 |
|---|---|---|---|
| `services/internal/api/openapi.gen.go` | `api/openapi.yaml` | oapi-codegen(`services/go.mod` の `tool`) | `make gen-go` |
| `services/{balance,speed,judge}/internal/api/openapi.gen.go` | 各 `services/<x>/api/openapi.yaml` | 同上 | `make <x>-gen` |
| `services/pokedex/internal/store/{db,models,pokedex.sql,querier}.go` | `services/pokedex/db/{sqlc.yaml,migrations,query}` | sqlc(`tools/go.mod` の `tool`) | `make gen-sql` |
| `web/src/api/{openapi,balance}.gen.ts`・`web/src/{speed,judge}/*.gen.ts` | 上の各 `openapi.yaml` | openapi-typescript・prettier(`web/package-lock.json`) | `make gen-ts` = `npm run gen` |
| `ios/PokeCalcKit/Sources/<ターゲット>/Generated/` | `ios/scripts/openapi-targets.sh` の各組 | swift-openapi-generator(`ios/tools/openapi-gen/Package.resolved`) | `make ios-gen` |

- `.gitignore` に入れ、`git rm --cached` で追跡から外す。iOS は `ios/PokeCalcKit/Sources/*/Generated` を
  まとめて無視し、生成対象の一覧(`ios/scripts/openapi-targets.sh`)を `openapi-gen.sh` と `ensure-gen.sh` の
  両方が読む。対象を1行足せば、生成・`gen-clean`・欠落の案内に自動で入る。
- `scripts/check-publishable.sh` は、これらが再び追跡されたら「追跡禁止」として失敗させる(`git add -f` の事故を防ぐ)。

### 2. 生成は使う側の前段で自動に走る(冪等・変更が無ければ速い)

- `make gen`(= `gen-go gen-sql gen-ts balance-gen speed-gen judge-gen`)と `make ios-gen` は、生成が要るときだけ
  生成器を呼ぶ。判定は `scripts/ensure-gen.sh stale <出力...> -- <入力...>`:
  `GEN_FORCE=1`・出力が無い・入力が無い・入力(ディレクトリは自身と配下すべて)のどれかが出力より新しい、の
  どれかなら生成する。ディレクトリ自身の更新時刻も比べるので、`migrations/` のファイルの削除・改名も検出する。
  macOS 標準の GNU Make 3.81 でも動くよう、複数出力の判定をスクリプトに寄せる。iOS は出力がディレクトリなので、
  生成のたびに `ios/.gen-stamps/<名前>`(Git 管理外)を更新し、それを出力として比べる。
- 強制は `GEN_FORCE=1`、削除は `make gen-clean`(iOS も消す)。
- Makefile: Go のパッケージをビルドするターゲット(`test`・`lint`・`build`・`staticcheck`・`test-services`・
  `test-db*`・`test-nats`・`migrate-*`・`import*`・`pokedex-export`・`up`・`dev`・`e2e`・`deploy-latest`・
  各レーンの `*-test`/`*-lint`/`*-build`/`*-docker-build`・`*-k3d-deploy*`・`api-docker-build`・
  `web-e2e-online`・`web-e2e-balance`)は該当する生成を前提条件に持つ。`make ios-*` は `ios-gen` を前提に持つ
  (`ios-gen-check` を除く。これは一時ディレクトリへの生成と手元の生成物を比べる検査)。
- 公開用のイメージを push するターゲット(`balance-`/`speed-`/`judge-docker-push`・`*-registry-push`・
  `pokedex-registry-push`)は、手元の生成物の鮮度(更新時刻)に頼らず、push の前に `GEN_FORCE=1` で作り直す。
- Web: `web/package.json` の `gen`(`web/scripts/gen-api-types.mjs`)を `predev`・`prebuild`・`pretypecheck`・
  `prelint`・`pretest`・`pretest:wasm`・`pree2e*` で呼ぶ。`cd web && npm ci && npm run build` だけでも成立する。
- Docker: Go のイメージ(calc・gateway・pokedex・record・team・balance・speed・judge)は、ホストで生成した
  ファイルをビルドコンテキストから受け取る(Makefile のターゲットが先に生成する)。イメージ内で生成器
  (特に cgo を要する sqlc)を毎回ビルドするのは遅いため。無いときは Dockerfile の `RUN test -f …` が
  「make gen を実行してから」と案内して止まる。Web のイメージは `npm run build` の `prebuild` がイメージ内で
  生成する(仕様の YAML をコンテキストから COPY する)。
- CI(`.github/workflows/ci.yml`): `make test` の前に `make gen` を明示のステップとして走らせ、所要時間を
  ログに出す(`time`)。「openapi.yaml・SQL から生成してビルド・テストできる」ことを毎回確かめる。
  コミットとの差分検査は無くなる。iOS は従来どおり CI の対象外(ADR-0119)。

### 3. 生成物が無いときの案内

- Go: 生成物を持つ各パッケージに手書きの `gen_required.go` を置く。生成物の識別子を1つ参照するだけのファイルで、
  生成物が無いとこのファイルの位置で `undefined: …` になり、冒頭のコメント(「リポジトリのルートで make gen」)に辿り着ける。
- iOS: 生成物を持つ各ターゲットに手書きの `GenRequired.swift` を置く(`Client` を参照する)。生成物が無いと
  Xcode でも `cannot find type 'Client' in scope` がこのファイルに出て、冒頭のコメント(「make ios-gen」)に辿り着ける。
  SwiftPM はソースの無いターゲットを作れないため、このファイルが無いと「target is empty」という分かりにくい
  エラーになる。**Xcode で直接開く前に `make ios-gen` を1回流す**(ios/README.md・docs/runbooks/ios.md。
  Go の `go build` を直接使う前の `make gen` と同じ扱い)。新しい生成対象を足すときは、そのターゲットにも置く。
- `scripts/ensure-gen.sh check`(Go・TS)/`check-ios`(iOS)が欠落を一覧にして案内する。`scripts/up.sh`・`dev.sh`・
  `k3d-deploy-latest.sh` は最初に `check` を流す。

### 4. 既存の検査の置き換え

- `check-publishable.sh --full` の「`make gen` の前後で作業ツリーの差分が変わらない」は成立しなくなる
  (生成物が無視されるため差分に出ない)。「`make gen` が成功し、`ensure-gen.sh check` が通る」に置き換える。
- `make ios-gen-check` は「コミット済みの生成物との一致」から「手元の生成物との一致」に意味が変わる
  (`make ios-test` は先に `ios-gen` を流すので、手での編集や生成器の非決定性を検出する役になる)。
- `services/pokedex/db/layout_test.go` の「sqlc の生成物がある」検査は残す(`make test` の前段で生成される)。
  失敗時の案内文だけ「コミットする」から「make gen を実行する」へ直す。
- `api.GetSwagger()` を使う契約テストは、生成が前段に走るのでそのまま成立する。

## 却下した案(iOS)

- **iOS だけ追跡を残し、衝突を merge ドライバ(合成した仕様からの再生成)で解く**: 一度実装したが却下した。
  - 生成対象の追加に追従できない。ドライバが生成対象(仕様・設定・出力先の組)を自分で持つため、
    main で増えた `PokeCalcBalanceAPI`(ADR-0415)に対応できていなかった。
  - 生成器の版を取り違える。ドライバは「こちら側」の生成器で合成した仕様から作るので、相手側が生成器の版を
    上げていると、どちらの側とも違う組み合わせの生成物ができうる。
  - clone ごとの `scripts/setup-git.sh` に依存する。流していない clone では黙って通常の 3-way マージに戻る。
  - rebase・cherry-pick・GitHub 上のマージでは効かない(git はドライバに相手のコミットを渡さず、
    作業ツリーの仕様もドライバ実行時点ではまだマージ前)。
  - ユーザー決定(生成物は ignore)と食い違い、Go・TS と iOS で運用が分かれる。
- **SwiftPM のビルドプラグイン(swift-openapi-generator の `OpenAPIGenerator`)で毎ビルド生成する**:
  `Sources/PokeCalcAPI/openapi.yaml` を `api/openapi.yaml` へのシンボリックリンクにする形で `swift build` は通った
  (2026-10-03、Xcode 27・swift-openapi-generator 1.13.1)が、`xcodebuild`(= `make ios-test-unit`)は
  「Plugin "OpenAPIGenerator" ... must be enabled before it can be used」で失敗した。プラグインの信頼は Xcode の画面で
  clone・マシンごとに人が「Trust & Enable」するか、`xcodebuild` に検証を省くフラグを渡す必要がある。前者は
  CLI からの初回ビルドが必ず壊れ、後者はプラグインの安全確認を省く操作で、AI エージェントの自動実行でも
  安全確認の回避として拒否された。
- **Xcode の Run Script / スキームの Pre-action で生成する**: パッケージのターゲットはアプリのビルドフェーズより
  先にビルドされるため Run Script では間に合わない。Pre-action は失敗してもビルドを止めないうえ、Xcode が自動で
  作る `PokeCalcKit-Package` スキームには付けられない。

採用した形(`make ios-*` の前段で `ios-gen`、Xcode 直接は先に `make ios-gen` を1回)は、Go の
「`make` は自動、`go build` 直接は先に `make gen`」と同じ扱いで、ユーザー決定に揃う。

## 代償

- 生成器の初回ビルド(cgo の sqlc・swift-openapi-generator)が、新しいクローン・CI の最初の実行で数分かかる。
  以降は Go・SwiftPM のビルドキャッシュが効く(CI は `actions/setup-go` の cache)。
- `go build ./...`・Xcode を `make gen`/`make ios-gen` 無しで使うと失敗する(案内は出る)。IDE も初回は生成が要る。
- 生成器の版を上げると、全員の手元で再生成が走る(版を固定するファイルを入力に含めているため。正しい挙動)。
- 更新時刻で判定するため、生成物を手で書き換えても再生成されない(手で編集しない約束は従来どおり)。
  疑わしいときは `GEN_FORCE=1`。公開用イメージの push は常に強制で作り直す。
- 未マージの PR が生成物を編集していると、この変更のマージ後に「削除と変更」の衝突になる。
  移行手順は docs/ai-shared/COORDINATION.md に書く(`git rm -r --cached` → `make gen GEN_FORCE=1`)。
