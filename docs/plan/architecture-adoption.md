# ARCH: ソフトウェアアーキテクチャの正式採用(調査と計画)

- 状態: **調査・計画のみ(未決)**。コードは変えていない。ADR は採用が決まってから書く(下の「ADR の番号」)。
- 日付: 2026-10-11(基準: `origin/main` 4b97544c)
- 発端: ユーザーの意向(2026-10-11)「人間があとで確認しやすくなるので、正式にソフトウェアアーキテクチャを採用したい」。
  (A) このリポジトリを改造する / (B) 新しいリポジトリへ移す、のどちらかを含めて、すぐ作業せず調査して計画を立てる。
- 守る前提: ADR-0012 のサービス境界、CLAUDE.md の絶対ルール(API は `api/openapi.yaml` 先行・`engine/` は純粋・サービスは自分の DB だけ)、
  レーン制(COORDINATION.md)、1 タスク = 1 PR。

## 0. 結論(先に)

| 問い | 推奨 |
|---|---|
| A か B か | **A(このリポジトリで段階的に)**。B は Git 履歴・PR 番号・ADR・ガード・GitOps・進行中の PR をすべて切り離す費用が大きく、得られるもの(きれいな初期状態)は A でも段階的に得られる(§4) |
| Go サービスの方式 | **案 0「現状維持 + 明文化」**(今の形 = 純粋な核〈engine・各サービスの内側〉+ ポートとアダプタ、に名前を付けて規約と検査にする)。コードを動かすのは、規約にはっきり反する 2 か所(G1・G5)だけ。ユースケース層(`internal/app`)は**最初から計画に入れず**、「1 つのハンドラが他のポートを 2 つ以上順に呼ぶ」などの機械で測れる条件を満たしたサービスだけ、そのレーンが次に触るときに足す(案 1 は案 0 の拡張として残す)。7 案を同じ 5 つの基準で比べた結果と、変えなくてよい根拠は §3.1 |
| Web | **Feature-Sliced Design を簡略化した 4 段**(`app` → `screens`(画面) → `features`(機能) → `shared`(domain・master・engine・api・ui・i18n))。下から上・横の import を lint で禁止(§3.1) |
| iOS | **今の MVVM + Service プロトコル(=ポート)を明文化**。生成 API を import してよいのは `API*.swift` だけ、をテストで固定。ターゲット分割は後回し(§3.2) |
| 機械検査 | **既存の「import を go/parser で読むテスト」(ADR-0105・失効ジョブ)の流儀を共通化した小さな検査ツール**を `tools/` に置き、`make lint` で回す。**今ある違反は「許可リスト(期限つき)」で凍結**し、段階ごとに 1 行ずつ消す(§5) |
| 進め方 | **1 段階 = 1 サービス(または Web/iOS の 1 区切り)= 1 PR = そのレーン**。最初の PR は規約と検査だけ(コードは動かさない)(§6) |

ユーザーが決めること は §8「未決の質問」。

## 1. 用語(この文書での言い方)

- **内側 / 外側**: 内側はポケモンの計算・判定・分析そのもの(ドメイン)。外側は HTTP・DB・NATS・ファイル・他サービスの呼び出し。**依存は外 → 内の一方向**(内側は外側を import しない)。
- **ポート**: 内側が「こういう機能がほしい」と宣言するインターフェース(例: `Provider`・`Store`・`EventPublisher`)。
- **アダプタ**: ポートを実装する外側のコード(例: `internal/master` の read model ローダ、`internal/store` の TiDB 実装、`internal/client` の HTTP 呼び出し)。
- **ユースケース(アプリ層)**: 1 つの API 操作の手順(入力の検証 → マスタの解決 → 計算 → イベント発行)。HTTP の形式(JSON・ステータスコード)は知らない。

## 2. 現状調査

### 2.1 全体

- Go モジュール: `engine`・`services`(calc・gateway・pokedex・record・team・internal)・`services/balance`・`services/speed`・`services/judge`・`tools`(`go.work`)。
  `apps/wishlist` は別アプリで、本計画の対象外。
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
| G6 | 重要 | **HTTP 層がドメインの規則を持つ**(SP の上限・レベルの検証) | `services/record/internal/httpapi/favorites.go:239-244, 376-377`(`engine.DefaultLevel`・`engine.MaxSPPerStat` で検証) |
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

## 3. 選択肢と根拠

### 3.1 Go サービス

#### 候補(7 案)

| 案 | 中身 |
|---|---|
| **0 現状維持 + 明文化** | 今の形に名前を付けて規約と検査にするだけ。名前は「純粋な核 + ポートとアダプタ」(engine・各サービスの内側が核。HTTP・DB・NATS・他サービスはアダプタ)。これは Gary Bernhardt の「Functional Core, Imperative Shell」(核は副作用なし、殻が I/O を受け持つ)と、Cockburn のヘキサゴナルを合わせた形で、**engine と balance・speed は既にこの形**。コードは規約に反する 2 か所(G1・G5)だけ直す |
| 1 案 0 + 必要な所だけユースケース層 | 案 0 に加え、条件(下の「推奨」に書く)を満たすサービス(いま該当するのは judge・record。calc・pokedex は境界線上)に `internal/app` を足し、手順をハンドラから移す |
| 2 クリーンアーキテクチャ(一律) | 全サービスを entities / use cases / interface adapters / frameworks の 4 層に分ける |
| 3 レイヤード(表示・ドメイン・データ) | 上の層が下の層に依存する 3 層 |
| 4 バーティカルスライス | 層ではなく「API 操作 1 つ = フォルダ 1 つ」(ハンドラ・処理・DB の呼び出しを 1 か所に)。操作の間で共有を減らす |
| 5 モジュラーモノリス | サービスを 1 プロセスに寄せ、モジュール境界をコードで守る |
| 6 DDD の戦術パターン | 集約・リポジトリ・ドメインイベント・値オブジェクトでドメインを組む |

#### 同じ 5 つの基準での比較

◎ 良い / ○ 普通 / △ 弱い / ✕ 合わない。「費用」は ◎ ほど安い。

| 案 | 開発のしやすさ | AI 実装との相性(規則が機械で検査でき、置き場所に迷わない) | 多レーンの競合 | 人間が確認しやすいか | 費用 |
|---|---|---|---|---|---|
| **0 現状維持 + 明文化** | ◎ 書き方が今と同じ | ○ 依存の向きは検査できる。ただし「手順をどこに書くか」は決まらないので、AI はハンドラに足し続けやすい | ◎ ファイルを動かさない(S0 は新規ファイルだけ) | ○ 規約・図・検査で全体は追える。judge の 944 行のハンドラなど読みにくい所は残る | ◎ |
| 1 案 0 + 必要な所だけユースケース層 | ○ 該当サービスだけ型の詰め替えが増える | ◎ 「手順は app、変換は httpapi」が規則にでき、検査もできる | ○ 1 サービス = 1 PR に閉じる | ◎ 1 操作の手順が 1 関数で読める | ○ |
| 2 クリーンアーキテクチャ(一律) | △ 薄いサービス(gateway・team・record)でも層と型が増える | ◎ 規則ははっきり | ✕ 全サービスのファイルが動く | ○ 層は揃うが、追う間接参照が増える | ✕ |
| 3 レイヤード | ○ 分かりやすい | ○ 上下の規則は検査しやすい | △ ポートを外して作り直す所が出る | ○ | △ |
| 4 バーティカルスライス | ○ 1 操作が 1 フォルダで閉じる | ○ 規則は単純だが、操作の間の共有部品(変換・エラー整形)の置き場所で迷う | ✕ 全ハンドラのファイルを分け直す | ○ | ✕ |
| 5 モジュラーモノリス | △ 運用は軽いが、いまの k8s・GitOps の構成を作り直す | △ | ✕ | △ | ✕ 学習目的のマイクロサービス構成(CLAUDE.md)・ADR-0012 と逆 |
| 6 DDD の戦術パターン | △ 計算が中心で状態の変化が少ないドメインには道具が重い | △ 用語と型が多く、AI の出力が揺れやすい | △ | △ | ✕ |

案 2〜6 は、少なくとも 1 つの基準で ✕ が付き、案 0・1 を上回る基準が無い。**実際の選択は案 0 と案 1 のどちらを「計画として」採るか**。

#### 推奨: 案 0(現状維持 + 明文化)。案 1 は条件つきの拡張として残す

変えなくてよい根拠(コード上の事実):

1. **核はすでに純粋で、正しさは別の仕組みで守られている**。`engine/` の本番コードは `example.com/pokecalc/...` を 1 つも import しない(約 8,800 行)。
   計算の正しさはゴールデンテスト(@smogon/calc と全件一致。CLAUDE.md 絶対ルール 3)が守る。ここを層で包み直しても、確認のしやすさはほとんど増えない。
2. **balance・speed は、すでに推奨の形**。`services/balance/internal/balance`・`services/speed/internal/speed` は HTTP・DB を import せず、`internal/master` が内側の `Provider` を実装する(§2.2)。案 0 はこれを「正解の例」として文書にするだけで済む。
3. **サービスの多くは「変換 + engine 呼び出し」の薄い殻**。gateway は転送、team は保存と読み出し、record も大半は同じ(`store.Store` はインターフェース済み。検証が溜まった `favorites.go` だけは C4)。ここにユースケース層を足すと、同じ値を型から型へ詰め替えるコードが増えるだけになる(案 2 の弱点)。
4. **競合の原因は構造ではなく共有ファイルだった**。これまでの PR の競合は `CURRENT_STATE.md`・`DECISIONS.md`・`docs/plan.md` に集中していた(ADR-0805・ADR-0172 の記録)。コードの層を変えても競合は減らず、むしろ全サービスを動かす間は増える。
5. **Go 公式の構成ガイドは層を定めていない**。`internal/`・`cmd/` の使い方だけを示し、規模に合わせて育てる方針(https://go.dev/doc/modules/layout )。今の構成はこれに沿っている。
6. **検査の前例がある**。import の向きのテストは既に 3 本ある(§2.1)ので、それを共通化すれば「明文化」が飾りでなく機械で守られる。

一方で、案 0 のままでは残る弱点(案 1 を捨てない理由):

- `services/judge/internal/httpapi/outspeed.go`(944 行)の `outspeedAndKo` は、デコード・2 サービスの順次呼び出し・判定・エラー整形が 1 関数にある(G3)。
- HTTP 層(`internal/httpapi`)は全サービスで 44 ファイル・約 9,700 行あり、engine(約 8,800 行)より大きい。500 行を超えるファイルが 5 つある。
- AI は「手順の置き場所」が決まっていないと、既存のハンドラに足し続ける。

そこで **案 1 のユースケース層は「計画の段階」にはせず、規約に機械で測れる条件**(例: 「1 つのハンドラ関数が他サービスのクライアント・store・publisher のうち 2 つ以上を呼ぶ」、または「`internal/httpapi` の 1 ファイルが 600 行を超える」)を書き、
**条件を満たしたサービスを、そのレーンが次にそのサービスを触るときに直す**。いまの調査では judge(pokedex と calc を順に呼ぶ・944 行)と record(`favorites.go` 624 行)が条件を満たし、calc(`server.go` 482 行・`adjust.go` 468 行。publisher と store を呼ぶ)と pokedex(`search.go` 595 行)は境界線上(§6 の C1〜C4)。
こうすると、案 0 の安さ・競合の少なさを保ったまま、案 1 の読みやすさが要る所にだけ費用を払える。

> 案 1 を最初から計画に入れる(C1〜C4 を日程に入れる)のが良いのは、「人間が 1 操作の手順を追いやすい」ことを費用より優先する場合。§8 Q2 で選べるようにしてある。

根拠:
- Alistair Cockburn「Hexagonal Architecture」(2005): https://alistair.cockburn.us/hexagonal-architecture/
- Gary Bernhardt「Boundaries」(Functional Core, Imperative Shell。2012): https://www.destroyallsoftware.com/talks/boundaries
- Robert C. Martin「The Clean Architecture」(2012): https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- Martin Fowler「PresentationDomainDataLayering」: https://martinfowler.com/bliki/PresentationDomainDataLayering.html
- Jimmy Bogard「Vertical Slice Architecture」(2018): https://www.jimmybogard.com/vertical-slice-architecture/
- Go 公式「Organizing a Go module」: https://go.dev/doc/modules/layout
- Three Dots Labs「How to implement Clean Architecture in Go」(段階的にアプリ層を抜き出す実例): https://threedots.tech/post/introducing-clean-architecture/ ・ https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example
- Martin Fowler「MonolithFirst」: https://martinfowler.com/bliki/MonolithFirst.html

#### Go の機械検査の候補

| 候補 | 中身 | 利点 | 費用 |
|---|---|---|---|
| **自前の小さな検査(推奨)** `tools/archcheck` | `go/parser` で import だけ読み、規則(どのディレクトリが何を import してはいけないか)と許可リストに照らす。`make lint` から全モジュールを 1 回で検査 | **前例がある**(ADR-0105 の storetest・失効ジョブのテスト)。依存の追加が無い(版の固定・更新の手間が増えない)。生成物が無くても動く(`go list` と違いビルド不要)。エラー文を日本語で「何を・どこへ移すか」まで書ける(AI が直しやすい)。5 つのモジュールを同じ規則で見られる | 自前の保守(100〜200 行程度)。表現力は下の 2 つより弱い |
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
| リスク | 長く続くと「途中の形」が混ざる → **許可リストに期限を付け、残りを `make lint` が数える**ことで見える化する | 移行中に 2 つのリポジトリが並ぶ(どちらが正か迷う)。権限・CI・GitOps の設定漏れ |

**推奨: A**。B で得られる「最初から規約どおりの状態」は、A でも「規約と検査を先に入れ、違反を許可リストで凍結し、段階ごとに減らす」ことで得られる。
これは古い仕組みを少しずつ置き換える Strangler Fig の考えを、リポジトリの中で小さく行うもの(https://martinfowler.com/bliki/StranglerFigApplication.html )。

## 5. 規約の案(採用が決まったら ADR と `docs/coding-rules.md` に移す)

### 5.1 Go(全サービス共通)

```
services/<svc>/
  cmd/<名前>/        起動・設定・組み立て(すべてを import してよい唯一の場所)
  internal/<ドメイン>/  内側(例 balance・speed・judge)。engine と標準ライブラリだけ
  internal/app/       ユースケース(§3.1 の条件を満たしたサービスだけ。案 1)。内側とポート(自分で宣言)だけ
  internal/httpapi/   入口のアダプタ。生成型 ⇄ app/内側の型の変換・HTTP のエラー整形だけ
  internal/store/ client/ master/ events/   出口のアダプタ。ポートを実装する
```

検査する規則(`tools/archcheck` の初版):

| ID | 規則 |
|---|---|
| R1 | `engine/`(`engine/cmd` を除く)は `example.com/pokecalc/...` と I/O の標準パッケージ(`net/*`・`os`・`database/*`・`time` の現在時刻・`math/rand` 等)を import しない。※ `time`・`os` は関数単位の検査が要るので、初版は import の一覧で止め、例外は許可リストに理由を書く |
| R2 | `services/<A>/...` は `services/<B>/...`(B ≠ A)を import しない(本番コード)。共有は `services/internal/` に置く |
| R3 | `services/internal/...` は `services/<svc>/...` を import しない |
| R4 | 内側(`internal/balance`・`internal/speed`・`internal/judge` など、サービスごとに 1 行で宣言)は、同じサービスの `httpapi`・`store`・`client`・`events`・`master`・`api`(生成型)を import しない |
| R5 | `internal/app` は `httpapi`・`echo` を import しない |
| R6 | 出口のアダプタ(`store`・`client`・`events`)は `httpapi` を import しない |
| R7 | テスト専用の部品(`storetest`・`calctest`・`deploytest`)は本番コードから import しない(ADR-0105 の検査を一般化) |

許可リスト(初版で凍結する既知の違反): G1(record・team の migrate → `pokedex/db`)、G5(judge `speedeffects` → `client`)。
G2・G3・G6・G7 は案 0 の規則(R1〜R7)には当たらない(httpapi に手順や検証があること自体は禁止しない)。ユースケース層を足すかどうかは、§3.1 の条件を `tools/archcheck` が**警告として表示**し(失敗にはしない)、そのレーンが判断する。
許可リストの各行には **理由・担当レーン・解消する段階**を書き、`make lint` は行数を表示する(0 になったら許可リストの仕組みごと消す)。

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

## 6. 段階の計画(推奨案)

**原則**: 1 段階 = 1 サービス(または Web/iOS の 1 区切り)= 1 PR = そのサービスのレーン。振る舞いは変えない(既存のテスト・契約テスト・ゴールデン・e2e がそのまま通ることが受け入れ条件)。
機能追加とリファクタリングを同じ PR に混ぜない。着手前に `gh pr list` でそのサービスを触っている PR が無いことを確かめ、あれば先にそれを入れてから始める。

**必ず行う段階(案 0)**

| 段階 | 担当レーン | 内容 | 受け入れ条件(機械で確かめる) | 競合を避ける工夫 |
|---|---|---|---|---|
| **S0 規約と検査の骨組み** | 横断(§8 Q4。既定はデータレーン〈運用の持ち主〉) | 横断 ADR(08xx)・`docs/coding-rules.md` に規約の節と構成図・`tools/archcheck`(R1〜R7 と、案 1 の条件の警告)・許可リスト・`make lint` に追加。**コードは動かさない** | `make lint` が通り、許可リストを 1 行消すと失敗する(検査が効いていることのテスト)。R1〜R7 それぞれに「違反の例で失敗する」テスト | 新しいファイルの追加と Makefile の lint に 1 行だけ。どのレーンのコードにも触らない |
| S1 越境の解消(G1) | データ(`services/internal/dbmigrate`)→ API(record・team の migrate) | `RoleGrant`・プロビジョニングを `services/internal/dbmigrate` へ移す。`pokedex/db` からは同じ名前で参照を残す | 許可リストの G1 の 2 行が消える。`make test`(`-tags mysql`・`tidb` を含む)が通る | データレーンの PR で移動と互換の参照を入れ、API レーンが record・team の import を差し替える(2 PR。順番が逆でも壊れない) |
| S2 向きの逆転の解消(G5) | 判定 | `speedeffects` が `client.MasterEffects` ではなく自分の型を受け取り、変換は `client` 側へ | 許可リストの G5 が消える。judge のテストが全件通る | 判定レーンだけのディレクトリ。数ファイルの変更 |
| S3 balance・speed | タイプバランス・素早さ | 内側の宣言を archcheck に 1 行(既に合格の形) | 変更なしで R4 が通る | 設定 1 行だけ |
| W0 Web の規則 | Web | `eslint.config.js` に §5.2 の規則と例外一覧(W1〜W5) | `npm run lint`(eslint + prettier)が通る。例外一覧を消すと失敗する | 設定ファイルだけ |
| W1 Web の共有部品 | Web(移動)→ 判定・素早さ・調整・構築(import の差し替え) | `screens/` の共有部品(`SpeciesSearchField`・`MovePicker` ほか)を `ui/`(または `shared/`)へ移し、`screens/` 側に**一時的な再エクスポート**を残す | 例外一覧の W1 が消える。Playwright・vitest が通る | 再エクスポートがあるので、他レーンは自分の次の PR で import を差し替えればよい(一斉に直さない)。全部差し替わったら再エクスポートを消す |
| W2 Web の残り | Web | W2〜W5(`api → app`・`i18n → favorites`・`domain → api.gen`・`favorites → record`) | 例外一覧が空 | 1 PR |
| I0 iOS の規則 | iOS | §5.3 のテスト 1 本と README の 1 節 | `swift test` と `make ios-test` が通る | テストの追加だけ |

**条件を満たしたら行う段階(案 1 の拡張。計画の日程には入れない)**

条件(§3.1): 1 つのハンドラ関数が他サービスのクライアント・store・publisher のうち 2 つ以上を呼ぶ、または `internal/httpapi` の 1 ファイルが 600 行を超える。`tools/archcheck` が警告で知らせ、**そのレーンが次にそのサービスを触る PR の前に、別の PR で**行う。

| 段階 | 担当レーン | いまの状態 | 内容 | 受け入れ条件 |
|---|---|---|---|---|
| C1 judge | 判定 | **条件を満たす**(`outspeed.go` 944 行・pokedex と calc を順に呼ぶ) | `internal/app`(`OutspeedAndKo`)を作り手順を移す。`Pokedex`・`Calc` のポートを app に宣言し `client` が実装(G3・G4) | 警告が消える。app のテストが HTTP なしで fake のポートで走る。既存の `httpapi` のテストが全件通る |
| C2 calc | API | 境界線上(`server.go` 482 行・`adjust.go` 468 行。store と publisher を呼ぶ) | `internal/app`(計算・一括・逆算・調整)。イベント発行は app のポート経由(G2・G10) | 契約テスト・`calctest`・`make test-golden`・k3d スモークが通る。調整(AJ)の PR が無い時に。adjust と calc を 2 PR に分けてよい |
| C3 pokedex | データ | 境界線上(`search.go` 595 行で 600 行の手前。外部呼び出しは store だけ。ただし検索の組み立てが HTTP 層にある) | 検索の組み立てを `readmodel` 側か `internal/app` へ。httpapi は sqlc を直接呼ばない(G7) | `-tags mysql` のテストが通る。importer は対象外 |
| C4 record | API | 条件を満たす(`favorites.go` 624 行)。SP 上限などの検証が HTTP 層にある(G6) | 検証の規則をドメイン側の関数へ。薄い CRUD は今のまま | favorites の検証のテストが HTTP なしで走る。**M2 の必須(ADR-0137)が終わってから** |

**順番の理由**: S0 で「今より悪くならない」状態を先に作る(以後の全レーンの PR が自動で検査される)。S1・S2 は規約にはっきり反する所で、小さい。
Web・iOS は Go と独立に進められる(担当レーンが違うので同時でよい)。条件つきの段階はレーンの都合で行い、どの段階もほかの段階を待たない。

**レーンへの影響のまとめ**: S0 以後、全レーンの PR は `make lint` の archcheck を通る必要がある(新しい違反だけが失敗し、既存の違反は許可リストで通る。案 1 の条件は警告だけ)。
各レーンが直すのは自分のサービスの段階だけで、他レーンのファイルは触らない(W1 の import 差し替えは各レーンが自分の次の PR で行う)。

## 7. ADR の番号(採用が決まってから書く)

- **横断の ADR(方式の採用・規約・archcheck)**: 運用・横断の帯 `08xx`。2026-10-11 の時点で main の最新は ADR-0810、リモートのブランチに `08xx` の新規 ADR は無い → **ADR-0811 の予定**。
  書く直前に `ls docs/adr | grep '^08'` と、`git branch -r` の各ブランチで `git diff --name-only --diff-filter=A origin/main...<ブランチ> -- docs/adr` を見て重複を確かめる(`0804` が 2 つある前例があるため)。
- **段階ごとの ADR**: 各レーンの帯から、そのレーンの最新の次(COORDINATION.md の表)。2026-10-11 時点の目安:
  データ 0179〜(main 最新 0178)/ API 0252〜(0251)/ Web 0356〜(0355)/ タイプバランス 0423〜(0421 + 進行中 0422)/ iOS 0531〜(進行中 0529・0530)/ 素早さ 0610〜(0609)/ 判定 0715〜(0714)。
  小さな段階(S2・S3・W2・I0 など)は ADR を作らず、横断 ADR の「段階の記録」に 1 行足すだけでもよい(§8 Q5)。

## 8. 未決の質問(ユーザーが決めること。推奨を先頭に)

1. **A か B か**
   - (推奨)A: このリポジトリで段階的に。
   - B: 新リポジトリへ移行(§4 の費用。進行中の PR を全部止める)。
2. **Go の方式**
   - (推奨)案 0「現状維持 + 明文化」。直すのは G1・G5 だけ。ユースケース層は条件を満たしたサービスだけ、そのレーンの都合で(C1〜C4)。
   - 案 1 を計画として: judge・calc・pokedex・record のユースケース層を日程に入れて進める(1 操作の手順を追いやすくすることを費用より優先する場合)。
   - 案 0 から条件つきの拡張も外す(明文化と検査だけ。judge の 944 行のハンドラなどは残る)。
   - 案 2〜6(クリーンアーキテクチャ一律・レイヤード・バーティカルスライス・モジュラーモノリス・DDD)。§3.1 の比較でどれも ✕ がある。
3. **Go の機械検査の道具**
   - (推奨)自前の `tools/archcheck`(go/parser。依存追加なし。前例あり)。
   - go-arch-lint(YAML で宣言。新しい道具を固定版で導入)。
   - golangci-lint + depguard(lint の仕組みごと置き換え)。
4. **S0(規約と検査の骨組み)をどのレーンが持つか**
   - (推奨)データレーン(M4・運用・横断の持ち主。COORDINATION.md)。
   - 新しく「アーキテクチャ」レーンを作る(`state/` と作業ディレクトリが 1 つ増える)。
   - API レーン(Go サービスの大半を持つ)。
5. **段階ごとの ADR**
   - (推奨)横断 ADR(0811)1 本 + 構造が変わる段階(S1・W1・C1〜C4)だけレーンの帯で ADR。
   - すべての段階で ADR。
   - 横断 ADR 1 本だけ(段階は PR と plan で追う)。
6. **iOS のターゲット分割(I1)**
   - (推奨)今はしない。検査のテスト(I0)で足り、分割は iOS の大きな作業の切れ目で改めて判断する。
   - S0 と同じ時期に分ける。
7. **M2 との優先順位**
   - (推奨)必ず行う段階(S0〜S3・W0〜W2・I0)は M2 と並行(他レーンを止めない小さな PR)。条件つきの C2・C4 は M2 の必須が終わってから。
   - M2 の必須が終わるまで全段階を始めない。
   - M2 より先に全段階を進める。
8. **許可リストの期限**
   - (推奨)期限は日付ではなく「解消する段階」で書き、段階の PR で必ず消す。行数を `make lint` が表示する。
   - 日付の期限を付け、過ぎたら `make lint` を失敗させる(放置は防げるが、関係ないレーンの PR が突然赤くなる)。

## 9. この調査で見ていないこと

- `apps/wishlist`(別アプリ)・`tools/`(importer・golden)・k8s の構成は対象外。
- `time.Now()`・乱数などの関数単位の純粋さ(R1 の注記)は import だけでは検査できない。必要なら S0 で `go/ast` の呼び出し検査を足すかを決める。
- 案 1 の条件(呼ぶポートの数・600 行)の値は現状からの見積もり。S0 の ADR で決め直す。
