# ARCH: ソフトウェアアーキテクチャの正式採用(調査と計画)

- 状態: **方式はユーザーが決定(2026-10-11): ヘキサゴナル(ポートとアダプタ)を名前つきで正式採用する。A(このリポジトリを段階的に改造)も決定**。
  **S0 実装済み(2026-10-11。ADR-0811・coding-rules・`tools/archcheck`・`make lint`)**。§8 は全問決定済み。
  **S3(calc)完了(2026-10-12。S3a〈計算・一括・逆算〉と S3b〈調整。データレーンが代行〉。ADR-0252。calc は archcheck の未移行の一覧から外れた)**。S1(G1 の解消)も 2026-10-12 完了。
  **S5(pokedex)実装済み(2026-10-12。ADR-0180。G7 解消。pokedex は未移行の一覧から外れた)**。
  **S4(record)実装済み(2026-10-12。ADR-0254。データレーンが代行。record は archcheck の未移行の一覧から外れた。G6 解消)**。
  **S6(team)完了(2026-10-12。ADR-0255。データレーンが代行。team は archcheck の未移行の一覧から外れた)**。残りは S2・S8・S9。
- 日付: 2026-10-11(基準: `origin/main`)
- 発端: ユーザーの意向(2026-10-11)「人間があとで確認しやすくなるので、正式にソフトウェアアーキテクチャを採用したい」。
- 方式の比較(利点・欠点の表): [../architecture-comparison.md](../architecture-comparison.md)(PR #711)。この文書は調査の詳細・決定の理由・段階計画を持つ。
- 守る前提: ADR-0012 のサービス境界、CLAUDE.md の絶対ルール(API は `api/openapi.yaml` 先行・`engine/` は純粋・サービスは自分の DB だけ)、
  レーン制(COORDINATION.md)、1 タスク = 1 PR。

## 0. 結論(先に)

| 問い | 決定・推奨 |
|---|---|
| A か B か | **決定: A(このリポジトリを段階的に改造)**(ユーザー回答 2026-10-11)。理由は §4 |
| Go サービスの方式 | **決定: ヘキサゴナル(ポートとアダプタ)を名前つきで正式採用**。全サービスで `internal/domain`(純粋な核)/ `internal/app`(ユースケース層)/ `internal/adapters`(httpapi・store・client・events ほか)の**フォルダの形をそろえる**。小さなサービスの app は薄くてよい。**新しい機能(コンボ計算〈ブランチ `docs/combo-calc-spec` の `docs/plan/combo-calc.md`〉など)は最初からこの形で作る**(§3.1・§5.1) |
| 機械検査 | `tools/archcheck` は **adapters → app → domain の向きだけ**を検査する(内側は外側を import しない)。形をそろえたサービスから順に検査の対象に入る(§5.1) |
| 進め方 | **1 サービス = 1 PR = そのサービスのレーン**。大きいサービス(judge・calc・record・pokedex)から。最初の PR(S0)は規約・ADR・検査の骨組みだけで、コードは動かさない(§6) |
| Web | Feature-Sliced Design を簡略化した 4 段(`app` → `screens` → `features` → `shared`)。ディレクトリ名は変えず、向きを ESLint で検査(§3.2・§5.2) |
| iOS | 今の MVVM + Service プロトコルを明文化(Service プロトコル = ポート、`API*`・`Mock*` = アダプタ。ヘキサゴナルと同じ考え)。生成 API を import してよいのは `API*.swift` だけ、をテストで固定(§3.2・§5.3) |

残りの未決は §8「未決の質問」。

## 1. 用語(この文書での言い方)

- **内側 / 外側**: 内側はポケモンの計算・判定・分析そのもの(ドメイン)。外側は HTTP・DB・NATS・ファイル・他サービスの呼び出し。**依存は外 → 内の一方向**(内側は外側を import しない)。
- **ポート**: 内側が「こういう機能がほしい」と宣言するインターフェース(例: `Provider`・`Store`・`EventPublisher`)。
- **アダプタ**: ポートを実装する外側のコード(例: `internal/master` の read model ローダ、`internal/store` の TiDB 実装、`internal/client` の HTTP 呼び出し)。
- **ユースケース(アプリ層)**: 1 つの API 操作の手順(入力の検証 → マスタの解決 → 計算 → イベント発行)。HTTP の形式(JSON・ステータスコード)は知らない。

## 2. 現状調査

### 2.1 全体

- Go モジュール: `engine`・`services`(calc・gateway・pokedex・record・team・internal)・`services/balance`・`services/speed`・`services/judge`・`tools`(`go.work`)。
  `apps/wishlist` は別アプリで、本計画の段階(S1〜)には入れない。ただしユーザーの意向(2026-10-12「このプロジェクト全てで」)により、同じ規則(domain / app / adapters・外 → 内の一方向)に Wishlist レーンが移行済み(`apps/wishlist/docs/arch-spec.md`。向きは `apps/wishlist/api/internal/archtest` が検査し、`tools/archcheck` の対象ではない)。
- 各サービスの中は、ほぼ同じ形: `cmd/<名前>`(起動・設定)、`internal/httpapi`(生成した ServerInterface の実装)、`internal/store`(DB)、`internal/events`(NATS)、`internal/master`(マスタの読み込み)。
  名前は揃っているが、**どの層が何に依存してよいかは文書にも検査にも無い**。
- 既にある機械検査: `make lint`(gofmt・go vet・staticcheck・plan・公開前検査)。import の向きの検査は 2 か所だけ:
  `services/pokedex/internal/storetest/no_prod_import_test.go`(storetest を本番コードから import しない。ADR-0105)と
  `services/record/internal/expire/expire_test.go` / `services/team/internal/expire/expire_test.go`(失効ジョブが他サービス・httpapi を import しない)。
  **engine の純粋さ(I/O を持ち込まない)は今は人のレビューだけで守られている**。Web の ESLint・iOS にも境界の検査は無い。

### 2.2 守られている所(本番コード。テストは除く)

| 規約 | 根拠(ファイル) |
|---|---|
| engine は他の何も import しない | `engine/*.go` の import に `example.com/pokecalc/...` が無い。WASM 境界は `engine/wasmapi` → `engine` の一方向 |
| balance・speed は「純粋な内側 + アダプタ」の形 | `services/balance/internal/balance`・`services/speed/internal/speed` は HTTP・DB を import しない。`internal/master`(read model ローダ)が内側の `Provider` を実装する向き(`master` → `balance`/`speed`)。ADR-0012 決定 7 の provider 境界どおり |
| サービスが他サービスの DB に触らない | DB に触るのは pokedex・record・team の `internal/store` だけで、それぞれ自分の DSN |
| サービス間は API / イベント経由 | judge → calc・pokedex は HTTP(`services/judge/internal/client`)。record・team は NATS のイベント(`services/internal/calcevents`)を購読 |
| HTTP の外にポートがある所 | calc のイベント発行は `httpapi.EventPublisher`(`services/calc/internal/httpapi/server.go:42`)。record・team の `store.Store`(`store.go`)はインターフェース |
| 共有パッケージは各サービスを import しない | `services/internal/*` は `engine` と `services/internal/api` だけを import |

### 2.3 破れている所・曖昧な所

重さ: **重要** = 規約として決めたら違反になる / **軽微** = 形の揃えの問題。

| # | 重さ | 内容 | 場所 |
|---|---|---|---|
| G1 | 重要 | **サービスのコードが別サービスのパッケージを import**(DB は分かれているが、コードが pokedex に依存) | `services/record/cmd/migrate/main.go:26`・`services/team/cmd/migrate/main.go:26` が `services/pokedex/db`(`RoleGrant`・プロビジョニング)を使う。共有なら `services/internal/dbmigrate` に置くべきもの |
| G2 | 重要 | **ユースケースが HTTP 層にある**(入力の検証 → マスタ解決 → engine 呼び出し → イベント発行が 1 関数) | `services/calc/internal/httpapi/server.go` `CalcDamage`(221〜287 行)・`CalcBulk`・`CalcReverse`、`adjust.go`(468 行)、`convert.go`(377 行) |
| G3 | 重要 | 同上。**他サービス 2 つを順に呼ぶ手順がハンドラの中**(約 200 行) | `services/judge/internal/httpapi/outspeed.go` `outspeedAndKo`(250 行〜。ファイル 944 行) |
| G4 | 重要 | **ポートが無く、具体的な HTTP クライアントに直接依存** | `services/judge/internal/httpapi/server.go:27` `Dependencies{Pokedex *client.Pokedex; Calc *client.Calc}` |
| G5 | 重要 | **内側がアダプタの型に依存**(向きが逆) | `services/judge/internal/speedeffects/decode.go:153` `BuildTable(master client.MasterEffects)`、`cache.go:14` |
| G6 | 重要 | **HTTP 層がドメインの規則を持つ**(SP の上限・レベルの検証)〔S4 で解消: `services/record/internal/domain/favorite.go`〕 | `services/record/internal/httpapi/favorites.go:239-244, 376-377`(`engine.DefaultLevel`・`engine.MaxSPPerStat` で検証) |
| G7 | 重要 | **HTTP 層が DB(sqlc)を直接呼ぶ**。検索の組み立てもハンドラ側 | `services/pokedex/internal/httpapi` → `internal/store`・`internal/readtx`。`search.go`(595 行)・`master.go`(331 行) |
| G8 | 軽微 | ハンドラから store を直接呼ぶ(ただし `store.Store` はインターフェース。処理が薄いので今は許容範囲) | `services/team/internal/httpapi/server.go`(543 行)・`services/record/internal/httpapi` |
| G9 | 軽微 | **マスタ/イベントの型が HTTP の生成型(`services/internal/api`)に依存** | `services/calc/internal/master`(→ `services/internal/api`)、`services/internal/calcevents`(イベントの中身に `req.Attacker` など API の型をそのまま入れる。`server.go:280`) |
| G10 | 軽微 | ポートの置き場所がまちまち(使う側に置く / 実装側に置く) | `calc/internal/master/master.go:22 Store`(実装側)と `calc/internal/httpapi EventPublisher`(使う側)、`balance` の `Provider`(内側) |
| G11 | 軽微 | 同じ部品の複製(別モジュールのため) | `httpguard`・`httpmetrics` が `services/internal`・`services/balance/internal`・`services/speed/internal`・`services/judge/internal` に 4 つ |
| G12 | 軽微(テストのみ) | テスト用の部品が他サービスをまたぐ | calc・pokedex・record・team の `cmd/*_test.go` → `services/gateway/deploytest`、`services/gateway/internal/httpapi` のテスト → `services/calc/calctest` |

### 2.4 Web(`web/src`)

構成: `app/`(骨組み・画面の登録 ADR-0323)、`screens/`(計算・逆算・タイプバランスの画面 **と**、他の画面も使う部品)、
機能ごとのディレクトリ(`adjust/`・`judge/`・`speed/`・`team/`・`favorites/`・`record/`・`deviceData/`)、
共有(`domain/`・`master/`・`engine/`(WASM の包み)・`api/`・`ui/`・`i18n/`・`images/`)。
「機能ごとのディレクトリ + 共有」という形は既にあり、Feature-Sliced に近い。破れている所:

| # | 内容 | 場所 |
|---|---|---|
| W1 | **機能が `screens/` の部品を import**(`screens/` が「画面」と「共有部品」を兼ねている) | `judge/`・`adjust/`・`team/`・`speed/` → `screens/SpeciesSearchField`・`MovePicker`・`MegaItemReason`・`AbilitySelect`・`ToggleChip`・`speciesResolution` |
| W2 | 機能 → 機能 | `favorites/` → `record/recordClient` |
| W3 | 共有 → 骨組み(下 → 上) | `api/clientIds.ts:7` → `app/storage` |
| W4 | 共有 → 機能 | `i18n/favorites.ts:3` → `favorites/favoriteCalc` |
| W5 | ドメイン → 通信の生成型 | `domain/balanceLabels.ts:7` → `api/balance.gen` |

`web/eslint.config.js` に import の境界の規則は無い。

### 2.5 iOS(`ios/`)

- `PokeCalcKit`(Swift Package): `PokeCalcAPI`・`PokeCalcSpeedAPI`・`PokeCalcBalanceAPI`・`PokeCalcJudgeAPI`(生成物だけ)、`PokeCalcDesign`(デザイントークン)、
  `PokeCalcCore`(ドメインの型・`PokeCalcService` などのプロトコル・`API*Service`(生成クライアントを使う実装)・`Mock*Service`・ViewModel。約 90 ファイルが 1 ターゲット)。
- アプリ本体 `ios/PokeCalc/`: SwiftUI の View だけ(画面ごとのフォルダは無く平置き)と `Features/FeatureRegistry.swift`(ADR-0507)。
- **形はすでに MVVM + ポート(Service プロトコル)+ アダプタ(API*/Mock*)**。生成 API を import しているのは `API*.swift` の 8 ファイルだけで、規約どおり。
  ただし **同じターゲットの中なので、ViewModel が生成型を import しても止まらない**(慣習で守っているだけ)。

## 3. 方式の決定と根拠

### 3.1 Go サービス: ヘキサゴナル(ポートとアダプタ)を採用

候補(現状維持 + 明文化・ヘキサゴナル・クリーンアーキテクチャ・オニオン・レイヤード・縦割り・モジュラーモノリス)の利点・欠点の比較は
[../architecture-comparison.md](../architecture-comparison.md)(PR #711)にある。ここには、ヘキサゴナルに決めた理由と、この文書の調査との対応だけを書く。

**ヘキサゴナルを選ぶ理由**

1. **核を守る目的に合う**。`engine/` の本番コードは `example.com/pokecalc/...` を 1 つも import しない(約 8,800 行)。WASM でも同じコードを使う。
   ヘキサゴナルは「核を中心に置き、HTTP・DB・他サービスは外側の付け替えられる部品」という形で、この性質をそのまま名前にできる。
2. **今のコードに近い**。`services/balance`・`services/speed` は、純粋な内側(`internal/balance`・`internal/speed`)と、内側の `Provider` を実装するアダプタ(`internal/master`)に既に分かれている(§2.2)。
   移行は「フォルダの名前をそろえ、手順を app に寄せる」作業が中心になる。
3. **向きの検査が単純**。規則は「外 → 内の一方向」だけ(adapters → app → domain)。`tools/archcheck` の規則も 1 種類で済み、AI が守りやすく、人間も結果を読みやすい。
4. **「現状維持 + 明文化」では足りない所がある**。名前と形がそろっていないと、人間が確認するときの基準が弱い。また手順の置き場所が決まっていないため、
   `services/judge/internal/httpapi/outspeed.go`(944 行)のように手順がハンドラに溜まる(G3)。HTTP 層(`internal/httpapi`)は全サービスで 44 ファイル・約 9,700 行あり、engine(約 8,800 行)より大きい。

**クリーンアーキテクチャ・オニオンを選ばない理由**: どちらも層の数と決まりが多い。層ごとに型の変換コードと規則が増えるため、**AI が実装で手間取りやすい**と判断した
(層が増えるほど変換と規則が増える、という理由からの判断で、計測した値ではない)。ヘキサゴナルでもユースケース層(app)は置くので、
「1 つの操作の手順が 1 か所で読める」という利点は得られる。

**DDD について**: DDD(ドメインの言葉でモデルを作る考え方)は**アーキテクチャの方式とは独立で、どの方式の上でも再現できる**。
このプロジェクトのドメインの言葉(種族・技・特性・実数値・確定数)は既に `engine/` と `docs/glossary.md` にそろっている。
戦術パターン(集約・リポジトリなど)まで入れるのは、複雑さが engine の計算ルールにあり永続化の業務ルールが薄いため、過剰と判断する。

根拠(出典):
- Alistair Cockburn「Hexagonal Architecture」(2005): https://alistair.cockburn.us/hexagonal-architecture/
- Gary Bernhardt「Boundaries」(Functional Core, Imperative Shell。純粋な核と I/O の殻): https://www.destroyallsoftware.com/talks/boundaries
- Robert C. Martin「The Clean Architecture」(2012): https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- Martin Fowler「PresentationDomainDataLayering」: https://martinfowler.com/bliki/PresentationDomainDataLayering.html
- Jimmy Bogard「Vertical Slice Architecture」(2018): https://www.jimmybogard.com/vertical-slice-architecture/
- Go 公式「Organizing a Go module」(`internal/`・`cmd/` の使い方。層は定めていない): https://go.dev/doc/modules/layout
- Three Dots Labs「How to implement Clean Architecture in Go」(Go で段階的にアプリ層を抜き出す実例): https://threedots.tech/post/introducing-clean-architecture/ ・ https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example
- Martin Fowler「MonolithFirst」: https://martinfowler.com/bliki/MonolithFirst.html

#### Go の機械検査の道具

| 候補 | 中身 | 利点 | 費用 |
|---|---|---|---|
| **自前の小さな検査(推奨)** `tools/archcheck` | `go/parser` で import だけ読み、adapters → app → domain の向きに反する import を見つける。`make lint` から全モジュールを 1 回で検査 | **前例がある**(ADR-0105 の storetest・失効ジョブのテスト)。依存の追加が無い(版の固定・更新の手間が増えない)。生成物が無くても動く(ビルド不要)。エラー文を日本語で「何を・どこへ移すか」まで書ける(AI が直しやすい)。5 つのモジュールを同じ規則で見られる | 自前の保守(100〜200 行程度) |
| go-arch-lint | YAML で「部品(component)と、依存してよい相手(mayDependOn)」を宣言 | ヘキサゴナル・オニオン向けに作られている。規則が表として読める | 新しいツールの導入と版の固定。go.work の複数モジュールでの動かし方を確かめる必要 |
| golangci-lint の depguard | ファイルの glob ごとに allow / deny の一覧 | 広く使われている | 今は golangci-lint を使っていない(staticcheck を直接)。導入すると lint 全体の仕組みが変わる |

- go-arch-lint: https://github.com/fe3dback/go-arch-lint
- depguard: https://github.com/OpenPeeDeeP/depguard
- 参考(依存の規則を CI の検査にする例): https://dev.to/gabrielanhaia/the-dependency-rule-written-as-a-ci-check-in-go-51co

### 3.2 クライアント(Web・iOS)

| 対象 | 方式 | 中身 | 合い方 |
|---|---|---|---|
| Web | 現状維持(文書だけ) | 今のディレクトリの役割を README に書くだけ。検査は足さない | △ 費用は最小だが、W1〜W5 の向きの崩れが今後も増える(検査が無いので AI が気づけない) |
| Web | **Feature-Sliced Design の簡略版(推奨)** | 本来は app / pages / widgets / features / entities / shared の 6 層。上の層は下の層だけ import、同じ層の横は禁止。ここでは `app` / `screens` / `features` / `shared` の 4 段に縮める | ◎ 機能ごとのディレクトリが既にある。画面の登録(ADR-0323)は「app が画面を集める」形でそのまま合う |
| Web | bulletproof-react の構成 | `features/` 同士の import を禁止し、`shared → features → app` の一方向。ESLint の `import/no-restricted-paths` で強制 | ○ 規則は同じ考え。ディレクトリ名を大きく変える分だけ費用が大きい |
| Web(検査) | eslint-plugin-boundaries / ESLint 標準の `no-restricted-imports` | 前者は「要素の種類」ごとの依存規則。後者は依存追加なしでパスの禁止だけ書ける | 最初は `no-restricted-imports`(依存追加なし)で足りる。規則が増えたら boundaries を検討 |
| iOS | **現状維持 + 明文化:MVVM + Service プロトコル(推奨)** | View → ViewModel → Service プロトコル(ポート)← API*/Mock*(アダプタ)。Apple の SwiftUI のデータの扱い(Observable なモデルを View が観察)に沿う | ◎ 既にこの形。生成 API を import しているのは `API*.swift` の 8 ファイルだけで、変える理由が無い |
| iOS | ターゲット分割(Domain / Data / Presentation を別 SwiftPM ターゲット) | コンパイラが import を強制 | ○ 強いが、`PokeCalcCore` の約 90 ファイルを動かすので全レーンの iOS 作業と競合する。後回し |

Web の推奨(Feature-Sliced の簡略版)も、**中身は「現状維持 + 明文化 + 検査」**: ディレクトリ名は変えず、今の `app/`・`screens/`・機能のディレクトリ・共有のディレクトリを 4 段に割り当てて、向きを lint で検査するだけ。
コードを動かすのは向きが崩れている W1〜W5 だけ。「文書だけの現状維持」を推さないのは、Web は機能を足すレーンが多く(判定・素早さ・調整・構築・タイプバランス)、検査が無いと崩れが増えるため(W1 は 4 レーン分の import)。

- Feature-Sliced Design: https://feature-sliced.design/docs/get-started/overview
- bulletproof-react「Project Structure」: https://github.com/alan2207/bulletproof-react/blob/master/docs/project-structure.md
- eslint-plugin-boundaries: https://github.com/javierbrea/eslint-plugin-boundaries
- Apple「Managing model data in your app」: https://developer.apple.com/documentation/swiftui/managing-model-data-in-your-app

## 4. A(このリポジトリを改造)と B(新しいリポジトリへ移行)

| 観点 | A: このリポジトリで段階的に | B: 新リポジトリへ移行 |
|---|---|---|
| Git 履歴 | 残る(約 1,900 コミット)。`git blame` で「なぜこうしたか」を ADR・PR へ辿れる | 切れる(履歴ごと移すなら A と同じで、新しくする意味が薄い) |
| PR・issue | PR(#700 台まで)・issue の番号と参照(ADR・decisions に多数)がそのまま有効 | 参照が全部旧リポジトリを指す。リンクの張り替えか、旧リポジトリを読み取り専用で残す |
| CI・ガード | `.github/workflows/ci.yml`・`scripts/ai-guard/bash-guard.sh`・`scripts/pr-merge.sh`・`.claude/`・`.codex/` をそのまま使える | すべて移して動作確認。ガード・権限の変更は**人間のマージ**が要る(CLAUDE.md)ので、移行中は AI が自走できない期間ができる |
| GitOps | balance・speed・judge の Argo CD Application(`repoURL` は placeholder)と AppProject、PENDING の Argo CD の認証(PAT)をそのまま | 新リポジトリの認証登録・Application の差し替え・Sync の確認を人間が行う |
| 進行中の PR | 開いている PR(2026-10-11 時点で #675・#698〜#706 の 7 件)と、各レーンのブランチがそのまま続く | 全部を旧リポジトリで閉じるか、新リポジトリへ作り直す。**競合の再発どころか、全レーンの作業がいったん止まる** |
| ADR・plan・ai-shared | 番号帯(ADR-0805・COORDINATION の表)・`docs/plan/`(ADR-0172)・`state/` をそのまま使う | 移すか作り直す。番号の続きをどうするかを決め直す |
| データ | 生成物・実マスタは Git に無い(ADR-0002・ADR-0807)ので影響なし | 同じく影響なし(数少ない B の利点が無い点) |
| 移行の手間 | 規約と検査の追加 + サービスごとの小さなリファクタリング(§6) | 上の全部の移し替え + 同じリファクタリング |
| リスク | 長く続くと「途中の形」が混ざる → **未移行のサービスの一覧を archcheck が持ち、残りの数を `make lint` が表示する**ことで見える化する | 移行中に 2 つのリポジトリが並ぶ(どちらが正か迷う)。権限・CI・GitOps の設定漏れ |

**決定: A**(ユーザー回答 2026-10-11)。B で得られる「最初から規約どおりの状態」は、A でも「規約と検査を先に入れ、未移行のサービスを一覧で管理し、1 サービスずつ減らす」ことで得られる。
これは古い仕組みを少しずつ置き換える Strangler Fig の考えを、リポジトリの中で小さく行うもの(https://martinfowler.com/bliki/StranglerFigApplication.html )。

## 5. 規約(S0 で ADR-0811 と `docs/coding-rules.md` に移す)

### 5.1 Go(全サービス共通の形)

```
services/<svc>/
  cmd/<名前>/              起動・設定・組み立て(すべてを import してよい唯一の場所)
  internal/domain/         純粋な核。engine・services/internal/master・標準ライブラリだけ。HTTP・DB・NATS・時刻の取得を持たない
  internal/app/            ユースケース層。1 つの操作の手順(検証 → 解決 → 計算 → 保存・通知)。
                           必要なポート(インターフェース)をここで宣言する。小さなサービスでは薄くてよい
  internal/adapters/
    httpapi/               入口。生成型 ⇄ app の型の変換・HTTP のエラー整形だけ
    store/                 DB(sqlc・TiDB・MySQL)。app のポートを実装
    client/                他サービスの API の呼び出し。app のポートを実装
    events/                NATS の発行・購読
    master/ など            read model の読み込みほか、外部とのやりとり
  internal/api/            oapi-codegen の生成物(場所は今のまま。§8 Q8)。外側として扱う
```

- **3 つのフォルダ(`domain`・`app`・`adapters`)は全サービスでそろえる**。核が engine そのものでサービス側に domain の中身が無い場合(gateway・calc の一部)や、
  app がほとんど無い場合(gateway)も、フォルダを置き `doc.go` に「このサービスの核は engine」「転送だけなので app は薄い」のように役割を書く(§8 Q7)。
- ポートは**使う側**で宣言する(app が使うものは app、domain が使うもの〈例 balance の `Provider`〉は domain)。
- **新しい機能・新しいサービスは最初からこの形**で作る(例: コンボ計算。ブランチ `docs/combo-calc-spec` の `docs/plan/combo-calc.md`)。
- 共有パッケージ(`services/internal/`)の扱い: `master` は内側(domain・app から使ってよい)。`api`・`calcevents`・`httpguard`・`httpmetrics`・`reqlog`・`version`・`dbmigrate` は外側(adapters・cmd だけが使う)。

**`tools/archcheck` が検査するのは向きだけ**(adapters → app → domain。内側は外側を import しない):

| 置き場所 | import してはいけないもの |
|---|---|
| `internal/domain` | 同じサービスの `internal/app`・`internal/adapters`・`internal/api`、外側の共有パッケージ、echo・DB ドライバ・NATS |
| `internal/app` | 同じサービスの `internal/adapters`・`internal/api`、外側の共有パッケージ、echo・DB ドライバ・NATS |
| `internal/adapters` | (制限なし。app・domain を使う) |

- **検査の対象は「形をそろえたサービス」から**。`tools/archcheck` は `internal/domain`・`internal/app`・`internal/adapters` がそろったサービスだけを検査し、
  まだそろっていないサービスは「未移行の一覧」(初版は全サービス)に載せる。一覧は段階の PR ごとに 1 行ずつ減り、**一覧に無いサービス(= 新しいサービス)で形がそろっていなければ失敗**する。
  `make lint` は一覧の残りの数を表示する。一覧が空になったら一覧の仕組みごと消す。
- engine の純粋さ・サービス間の import の禁止(G1)は、この検査には入れない(CLAUDE.md の既存の規則とレビューで守る。G1 は S1 で直す)。検査を足すかは後で判断する(§9)。

### 5.2 Web

| 段 | ディレクトリ | import してよい先 |
|---|---|---|
| app | `app/`・`App.tsx`・`main.tsx` | すべて |
| screens(画面) | `screens/`(計算・逆算・タイプバランス)・各機能の `*.screen.tsx` | features・shared |
| features(機能) | `adjust/`・`judge/`・`speed/`・`team/`・`favorites/`・`record/`・`deviceData/` | shared だけ(**機能 → 機能は禁止**) |
| shared | `domain/`・`master/`・`engine/`・`api/`・`ui/`・`i18n/`・`images/`・`styles/` | shared の中だけ。`domain/` は `api/*.gen` を import しない |

検査: `web/eslint.config.js` に `no-restricted-imports`(依存の追加なし)。既知の違反(W1〜W5)はファイル単位の `eslint-disable` ではなく、設定の中の例外一覧として凍結する。

### 5.3 iOS

- `PokeCalcCore` の中で、生成モジュール(`PokeCalcAPI` ほか)を import してよいのは `API*.swift` だけ。ViewModel・`*DomainTypes.swift` は Service プロトコルと自分の型だけを使う。
- 検査: `PokeCalcCoreTests` に「ソースを読んで import 行を確かめる」テストを 1 本(`swift test` で macOS でも走る。Xcode 不要)。
- View は `ios/PokeCalc/` に置き、Service を直接呼ばず ViewModel を通す(今の形)。

## 6. 段階の計画

**原則**
- **1 サービス = 1 PR = そのサービスのレーン**。大きいサービス(judge・calc・record・pokedex)から。
- 振る舞いは変えない。既存のテスト・契約テスト・ゴールデン・e2e がそのまま通ることが受け入れ条件。機能追加と同じ PR に混ぜない。
- **競合を避ける工夫**:
  - 着手前に `gh pr list` でそのサービスを触っている PR が無いことを確かめる。あれば先にそれを入れてから始める。
  - 着手する前日までに `docs/ai-shared/decisions/` に「<サービス> の形をそろえる PR を出す」と 1 件足し、他レーンが同じサービスを触る PR を控えられるようにする。
  - ファイルの移動は `git mv` で行い、移動と中身の変更を同じコミットに混ぜない(同じ PR の中で「移動のコミット」→「app への寄せのコミット」)。git が名前の変更として追えるので、他のブランチは `git merge origin/main` で取り込める。
  - PR は短く保ち、CI が緑になったらその日のうちにマージする(長く開いたままにしない)。
  - Web の共有部品の移動(W1)は一時的な再エクスポートを残し、他レーンは自分の次の PR で import を差し替える。
- 担当レーンが移すのは自分のサービスだけ。他レーンのファイルは触らない。

| 段階 | 担当レーン | 内容 | 受け入れ条件(機械で確かめる) |
|---|---|---|---|
| **S0 規約・ADR・検査の骨組み**(**完了 2026-10-11**) | データ(§8 Q2 決定) | ADR-0811(ヘキサゴナル採用)・`docs/coding-rules.md` に §5.1 の形と図・`tools/archcheck`(向きの検査と未移行の一覧。初版は全サービスが一覧に載る)・`make lint` に 1 行。**コードは動かさない** | `make lint` が通る。archcheck のテストで「domain → adapters の import」「app → adapters の import」「一覧に無いのに形がそろっていないサービス」がそれぞれ失敗する |
| S1 越境の解消(G1)〔2026-10-12 完了。import の検査は足さない〕 | データ → API | `RoleGrant`・プロビジョニングを `services/pokedex/db` から `services/internal/dbmigrate` へ。record・team の migrate の import を差し替える | record・team の `cmd/migrate` が `services/pokedex/...` を import しない。`make test`(`-tags mysql`・`tidb` を含む)が通る |
| S2 judge | 判定 | `internal/judge`・`speedeffects` の純粋な部分 → `domain`。`outspeedAndKo` の手順 → `app`(`Pokedex`・`Calc` のポートを app で宣言)。`httpapi`・`client`・`httpguard`・`httpmetrics` → `adapters`。G3〜G5 を解消 | judge が未移行の一覧から消え、archcheck が通る。app のテストが HTTP なしで fake のポートで走る。既存の judge のテストが全件通る |
| S3 calc(**完了 2026-10-12。前半 S3a〈計算・一括・逆算〉・後半 S3b〈調整 `adjust.go` → app。データレーンが代行。I-api-2〉。ADR-0252。critic 軽微指摘〈未知の失敗の写し・逆算の締め切りテスト・domain の単体テスト・docs/impl のパス〉は 2026-10-15 に整理。I-api-6**) | API | 入力の検証(`validateIndividual`・`checkMegaItem` など)→ `domain`。計算・一括・逆算・調整の手順 → `app`(イベント発行は app のポート)。`httpapi`・`events`・`master` → `adapters`。G2・G9・G10 を解消 | 契約テスト・`calctest`・`make test-golden`・k3d スモークが通る。調整(AJ)の PR が無い時に。大きければ「計算・一括・逆算」と「調整」の 2 PR に分けてよい |
| S4 record(**完了 2026-10-12・ADR-0254。データレーンが代行**) | API | SP 上限などの検証 → `domain`。お気に入り・履歴の手順と失効ジョブ → `app`。`httpapi`・`store`・`events` → `adapters`。G6 を解消 | `-tags tidb` を含むテストが通る。失効ジョブの import 検査(`expire_test.go`)を新しいパスに合わせ、検査の意味は弱めない。M2 の PR が record に無い時に(§8 Q5) |
| S5 pokedex(**完了 2026-10-12・ADR-0180**) | データ | 検索の組み立て → `app`(正規化など純粋な部分は `domain`)。`httpapi`・`store`・`readtx`・`readmodel` の I/O → `adapters`。G7 を解消。importer は §8 Q6 | `-tags mysql` のテストが通る。storetest の import 検査(ADR-0105)を新しいパスに合わせる |
| S6 team(**完了 2026-10-12・ADR-0255。データレーンが代行。I-api-4**) | API | record と同じ形(app は薄い) | `-tags tidb` を含むテストが通る |
| S7 gateway(**完了 2026-10-12・ADR-0811**) | API(データレーンが代行) | `httpapi` → `adapters`。`domain`・`app` は `doc.go` だけ(転送だけ) | gateway のテスト・k3d スモークが通る |
| S8 balance | タイプバランス | `internal/balance` → `domain`。`master`・`httpapi`・`httpguard`・`httpmetrics` → `adapters`。app は薄い | `go test ./services/balance/...` が通る。GitOps の overlay は image だけなので影響なし |
| S9 speed | 素早さ | balance と同じ | `go test ./services/speed/...` が通る |
| W0〜W2 Web | Web(W1 の import 差し替えは各レーン) | W0: `eslint.config.js` に §5.2 の規則と例外一覧。W1: `screens/` の共有部品を `ui/` へ(再エクスポートを残す)。W2: W2〜W5 を解消 | `npm run lint`(eslint + prettier)・vitest・Playwright が通る。最後に例外一覧が空 |
| I0 iOS | iOS | §5.3 のテスト 1 本と README の 1 節 | `swift test` と `make ios-test` が通る |

**順番の理由**: S0 で「新しいサービス・新しい機能は最初から形がそろう」状態を先に作る(コンボ計算などはここから)。S1 は小さく、S4・S6 の前提。
大きいサービスは、手順がハンドラに一番溜まっていて担当が 1 レーンで閉じる judge で型を決め、calc・record・pokedex がそれをなぞる。
小さいサービス(team・gateway・balance・speed)は形をそろえるだけで、ほぼ移動だけになる。Web・iOS は Go と独立に進められる(担当レーンが違うので同時でよい)。

**レーンへの影響のまとめ**: S0 以後、全レーンの PR は `make lint` の archcheck を通る必要がある(移行済みのサービスでは向きの違反が失敗、未移行のサービスは一覧で通る、新しいサービスは最初から形が必要)。
各レーンが自分のサービスの段階を 1 PR ずつ行う。

## 7. ADR の番号

- **ADR-0811: ヘキサゴナル(ポートとアダプタ)の採用**(横断の帯 `08xx`。S0 で書く)。2026-10-11 の時点で main の最新は ADR-0810、リモートのブランチに `08xx` の新規 ADR は無い。
  書く直前に `ls docs/adr | grep '^08'` と、`git branch -r` の各ブランチで `git diff --name-only --diff-filter=A origin/main...<ブランチ> -- docs/adr` を見て重複を確かめる(`0804` が 2 つある前例があるため)。
- 各サービスの移行で設計の判断が要るとき(例: calc の検証をどこまで domain に寄せるか)は、そのレーンの帯から番号を取る(COORDINATION.md の表)。2026-10-11 時点の目安:
  データ 0179〜 / API 0252〜 / Web 0356〜 / タイプバランス 0423〜(進行中 0422)/ iOS 0531〜(進行中 0529・0530)/ 素早さ 0610〜 / 判定 0715〜。
  判断が要らない移行は ADR を作らず、ADR-0811 の「段階の記録」に 1 行足す(§8 Q9)。

## 8. 質問と決定(全問決定済み)

**ユーザー決定(2026-10-11): Q1 は A、Q2〜Q10 はすべて各項の(推奨)のとおり。** 以下は決定の記録(先頭が採用した案)。

1. **A か B か** — **決定済み: A(このリポジトリを段階的に改造)**(ユーザー回答 2026-10-11)。
2. **S0(規約・ADR・検査の骨組み)をどのレーンが持つか**(決定済み)
   - (推奨)データレーン(M4・運用・横断の持ち主。COORDINATION.md)。
   - 新しく「アーキテクチャ」レーンを作る(`state/` と作業ディレクトリが 1 つ増える)。
   - API レーン(Go サービスの大半を持つ)。
3. **Go の機械検査の道具**(決定済み)
   - (推奨)自前の `tools/archcheck`(go/parser。依存追加なし。前例あり)。
   - go-arch-lint(YAML で宣言。新しい道具を固定版で導入)。
   - golangci-lint + depguard(lint の仕組みごと置き換え)。
4. **大きいサービスの中の順番**(決定済み)
   - (推奨)judge → calc → record → pokedex(手順が一番溜まっていて 1 レーンで閉じる judge で型を決める)。
   - calc → judge → record → pokedex(利用者に一番近い計算から)。
   - pokedex → calc → judge → record(マスタの提供元から)。
5. **record・team の移行と M2(必須。ADR-0137)の関係**(決定済み)
   - (推奨)M2 の PR が record・team に無い時に行う(M2 を止めず、移行も後回しにしない)。
   - M2 の必須が終わるまで record・team の移行を後ろに回す(「大きいサービスから」の順は崩れる)。
   - M2 より先に移行する。
6. **バッチの置き場所(pokedex の importer・record/team の失効ジョブ)**(決定済み)
   - (推奨)失効ジョブは各サービスの `app`(手順)+ `adapters/store`。importer(`services/pokedex/importer`)は今回の対象外にし、S5 の後で同じ形にするかを判断する。
   - importer も S5 で同時に移す(PR が大きくなる)。
7. **domain・app の中身が無いサービスのフォルダ**(決定済み)
   - (推奨)フォルダを置き、`doc.go` に役割を書く(形が全サービスでそろい、archcheck の判定も単純)。
   - 中身が無ければフォルダを置かない(archcheck に例外を持たせる)。
8. **oapi-codegen の生成物(`internal/api`)の場所**(決定済み)
   - (推奨)今のまま(`scripts/ensure-gen.sh`・`.gitignore`・Makefile の生成先を動かさない。外側として扱う)。
   - `internal/adapters/httpapi/api` へ移す(生成の設定と共有ファイルを全レーンで変える)。
9. **サービスごとの ADR**(決定済み)
   - (推奨)ADR-0811 の 1 本 + 判断が要る移行だけレーンの帯で ADR。判断の要らない移行は 0811 の「段階の記録」に 1 行。
   - サービスごとに必ず ADR。
10. **iOS のターゲット分割**(決定済み)
    - (推奨)今はしない。テスト(I0)で足り、分割は iOS の大きな作業の切れ目で改めて判断する。
    - S0 と同じ時期に分ける。

## 9. この調査で見ていないこと

- `tools/`(importer・golden)・k8s の構成は対象外。`apps/wishlist` は Wishlist レーンが同じ規則で移行済み(2026-10-12。§2.1)。
- engine の純粋さ(`time.Now()`・乱数などの関数単位)とサービス間の import の禁止は、archcheck に入れていない(§5.1)。入れるかは全サービスの移行の後に判断する。
- 移行によるパスの変更で `docs/impl/`(実装の対応表)の記述が古くなる。各サービスの移行 PR で該当行を直す。
