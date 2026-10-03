# ADR-0171: API 契約・SQL から作る生成物を Git に置かず、使う前に生成する(iOS だけは追跡を続ける)

- 状態: 採用
- 日付: 2026-10-03
- レーン: 運用(専用の ADR 帯が無いため、ADR-0119 と同じくデータ帯 `0100〜` の空きを使う)
- 関連: ADR-0100 §1(sqlc)、ADR-0119(CI)、ADR-0500 §2(iOS の生成)、ADR-0604 §2(speed の Web 型)、
  docs/ai-shared/COORDINATION.md「生成物を追跡から外したあとの取り込み」

## 背景

`api/openapi.yaml`・各レーンの `openapi.yaml`・pokedex の SQL から作る生成物(oapi-codegen・sqlc・
openapi-typescript・swift-openapi-generator)を Git にコミットしていた。API を変える PR が並ぶと、
生成物どうしが毎回衝突する(数千行の生成ファイルの衝突を手で解くことになる)。また再生成を忘れた PR が
main に入り、`make ios-gen-check` が赤になった(2026-10-03)。

ユーザー決定(2026-10-03): 生成物のコミットをやめ、ビルド時に生成する。

## 決定

### 1. Go・TypeScript の生成物は追跡しない

対象(`scripts/ensure-gen.sh list` が一覧の正):

| 生成物 | 入力 | 生成器(版の固定) |
|---|---|---|
| `services/internal/api/openapi.gen.go` | `api/openapi.yaml` | oapi-codegen(`services/go.mod` の `tool`) |
| `services/{balance,speed,judge}/internal/api/openapi.gen.go` | 各 `services/<x>/api/openapi.yaml` | 同上 |
| `services/pokedex/internal/store/{db,models,pokedex.sql,querier}.go` | `services/pokedex/db/{migrations,query}` | sqlc(`tools/go.mod` の `tool`) |
| `web/src/api/{openapi,balance}.gen.ts`・`web/src/{speed,judge}/*.gen.ts` | 上の各 `openapi.yaml` | openapi-typescript・prettier(`web/package-lock.json`) |

`.gitignore` に入れ、`git rm --cached` で追跡から外す。`scripts/check-publishable.sh` は、これらが再び
追跡されたら「追跡禁止」として失敗させる(`git add -f` の事故を防ぐ)。

### 2. 生成は使う側の前段で自動に走る(冪等・変更が無ければ速い)

- `make gen`(= `gen-go gen-sql gen-ts balance-gen speed-gen judge-gen`)は、出力が無いか、入力
  (仕様・設定・生成器の版を固定するファイル)のどれかが出力より新しいときだけ生成器を呼ぶ
  (`scripts/ensure-gen.sh stale`。macOS 標準の GNU Make 3.81 でも動くよう、複数出力の判定をスクリプトに寄せる)。
  強制は `make gen GEN_FORCE=1`、削除は `make gen-clean`。
- Makefile: `test`・`lint`・`build`・`staticcheck`・`test-services`・各レーンの `*-test`/`*-lint`/`*-build`・
  `test-db`・`test-db-docker`・`test-nats`・`e2e`・`dev`・`up`・`deploy-latest`・`*-docker-build`・
  `*-docker-push`・`*-registry-push`・`*-k3d-deploy*`・`web-e2e-online`・`web-e2e-balance` 等、
  Go のパッケージをビルドするターゲットは該当する生成を前提条件に持つ。
- Web: `web/package.json` の `gen`(`web/scripts/gen-api-types.mjs`)を `predev`・`prebuild`・`pretypecheck`・
  `prelint`・`pretest`・`pretest:wasm`・`pree2e*` で呼ぶ。`cd web && npm ci && npm run build` だけでも成立する。
- Docker: Go のイメージ(calc・gateway・pokedex・record・team・balance・speed・judge)は、ホストで生成した
  ファイルをビルドコンテキストから受け取る(Makefile のターゲットが先に `make gen` する)。イメージ内で
  生成器(特に cgo を要する sqlc)を毎回ビルドするのは遅いため。無いときは Dockerfile の `RUN test -f …` が
  「make gen を実行してから」と案内して止まる。Web のイメージは `npm run build` の `prebuild` が
  イメージ内で生成する(仕様の YAML をコンテキストから COPY する)。
- CI(`.github/workflows/ci.yml`): `make test` の前に `make gen` を明示のステップとして走らせる
  (「openapi.yaml から生成してビルド・テストできる」ことを毎回確かめる。コミットとの差分検査は無くなる)。

### 3. `go build`/`go test` を直接叩いたときの案内

生成物を持つ各パッケージに手書きの `gen_required.go` を1つ置く。生成物の識別子を1つ参照するだけのファイルで、
生成物が無いとこのファイルの位置で `undefined: …` になり、ファイル冒頭のコメント
(「リポジトリのルートで make gen を実行する。ADR-0171」)に辿り着ける。
`scripts/ensure-gen.sh check` は生成物の欠落を一覧にして同じ案内を出す(README・AGENTS.md の手順にも書く)。

### 4. 既存の検査の置き換え

- `check-publishable.sh --full` の「`make gen` の前後で作業ツリーの差分が変わらない」は成立しなくなる
  (生成物が無視されるため差分に出ない)。「`make gen` が成功し、`ensure-gen.sh check` が通る」に置き換える。
- `services/pokedex/db/layout_test.go` の「sqlc の生成物がある」検査は残す(`make test` の前段で生成される)。
  失敗時の案内文だけ「コミットする」から「make gen を実行する」へ直す。
- `api.GetSwagger()` を使う契約テストは、生成が前段に走るのでそのまま成立する。

### 5. iOS(`ios/PokeCalcKit/Sources/PokeCalcAPI/Generated/`)だけは追跡を続ける

検討した案と結果:

- **SwiftPM のビルドプラグイン(swift-openapi-generator の `OpenAPIGenerator`)で毎ビルド生成する**:
  `Sources/PokeCalcAPI/openapi.yaml` を `api/openapi.yaml` へのシンボリックリンクにし、生成設定を同じ
  ディレクトリに置き、手書きの空でない Swift ファイルを1つ足す形で、`swift build` は通ることを確かめた
  (2026-10-03、Xcode 27・swift-openapi-generator 1.13.1)。しかし `xcodebuild`(= `make ios-test-unit` など)は
  「Plugin "OpenAPIGenerator" ... must be enabled before it can be used」で失敗した。プラグインの信頼は
  Xcode の画面で人が一度「Trust & Enable」するか、`xcodebuild` に検証を省くフラグを渡す必要がある。
  前者はクローン・マシンごとの手作業になり CLI からの初回ビルドが必ず壊れる。後者はプラグインの安全確認を
  省く操作で、AI エージェントの自動実行でも安全確認の回避として拒否された(2026-10-03 の検証時)。
  どちらも採らない。
- **Xcode の Run Script / スキームの Pre-action で生成する**: パッケージのターゲットはアプリのビルドフェーズより
  先にビルドされるため Run Script では間に合わない。Pre-action は失敗してもビルドを止めないうえ、
  Xcode が自動で作る `PokeCalcKit-Package` スキームには付けられない。採らない。

したがって iOS の生成物は追跡を続け、衝突だけを機械的に解く:

- `.gitattributes` で Generated の Swift に `merge=pokecalc-ios-gen` を付ける。merge ドライバは clone ごとの
  `git config` が要るので `scripts/setup-git.sh` で登録する(未登録の clone では従来どおり通常の 3-way マージ)。
- ドライバ(`ios/scripts/merge-generated.sh`)は、`git merge` 中なら両側と merge-base の `api/openapi.yaml`・
  生成設定を `git merge-file` で合成し、衝突が無ければその合成結果から生成した内容で解決する
  (同じ仕様からの生成結果をキャッシュし、9ファイルで生成器を1回だけ呼ぶ)。git はドライバ実行時点で
  作業ツリーをまだ更新していない(merge-ort はマージ結果をメモリ上で作ってから書き出す。実験で確認)ため、
  作業ツリーの `api/openapi.yaml` は使えない。仕様自体が衝突したとき・rebase/cherry-pick のときなど合成元が
  分からないときは、こちら側の内容を残して衝突のままにし、「仕様の衝突を解いてから `make ios-gen`」と案内する。
- 生成し忘れの検出は従来どおり `make ios-gen-check`(`make ios-test` に含む)。

## 代償

- 生成器の初回ビルド(特に cgo の sqlc)が新しいクローン・CI の最初の `make` で数分かかる(Go のビルド
  キャッシュが効けば以降は速い)。CI の時間も同じだけ増える。
- `go build ./...` を `make gen` 無しで叩くと失敗する(案内は出る)。IDE も初回は `make gen` が要る。
- 生成器の版を上げると、全員の手元で再生成が走る(版を固定するファイルを入力に含めているため。正しい挙動)。
- iOS だけ方式が違う(追跡+マージドライバ)。ドライバは `scripts/setup-git.sh` を流した clone でだけ効き、
  macOS(swift)が無い環境・rebase では手で `make ios-gen` する。iOS の生成物の衝突は残りうる。
- 未マージの PR が生成物を編集していると、この変更のマージ後に「削除と変更」の衝突になる。
  移行手順は docs/ai-shared/COORDINATION.md に書く(`git rm --cached` → `make gen`)。
