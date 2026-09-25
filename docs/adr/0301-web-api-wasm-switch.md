# ADR-0301: Web の API / WASM 切り替え(P4-5)— 実体 → ID の写像・生成型・モード・端末 ID

- 状態: 採用(Web レーン、2026-09-22。§4 の既定モードと §7 の gen-ts の配線は既定案で進行・ユーザー未確認。DECISIONS.md)
- 日付: 2026-09-22
- 関連: plan.md P4-5、ADR-0300(Web の構成)、ADR-0011 §10・§11(WASM 境界と API 契約の差分・遅延ロード)、
  ADR-0200(calc-svc の API 契約。`api/openapi.yaml`)、CLAUDE.md 絶対ルール1・技術規約(端末 ID とセッション ID)

## 背景

API レーンの P3-1(ADR-0200)で `api/openapi.yaml` が P1-12 以降の engine の形(一括の `defender`・`category`・逆算の範囲・
エラー code の共通語彙)に揃い、main に入った。ADR-0011 §10 が P4-5 に持ち越した「オンライン = ID を送る / オフライン = 解決済みの
実体を渡す」の2経路を Web に作る。gateway(P3-2)と pokedex-svc(P2-3)はまだ無い。

## 決定

### 1. 画面の入力は「解決済みの実体」のまま。API 実装が実体 → ID に写す

- 画面とリクエストの組み立て(`domain/requests.ts`)は変えない。`CalcEngine` の入出力は ADR-0300 §2 の WASM DTO 型のまま。
- 「ID → 実体」の解決は `MasterData` の読み込み時に済んでいる(画面は ID で選び、`MasterData` から実体を引く)。
  API 実装の `CalcEngine`(`createApiEngine`)は、受け取った実体から ID を取り出して API のリクエストにする(実体 → ID)。
  こうすると両経路が同じ型(DTO)を共有し、解決層は `MasterData` の1か所になる(ADR-0011 §10 の宿題)。
- API の型は `openapi-typescript` の生成型(`web/src/api/openapi.gen.ts`。手で編集しない)を正とし、写像の関数だけを手で書く。

### 2. 写像の表(WASM DTO ↔ API)

| WASM DTO(`engine/types.ts`) | API(`openapi.gen.ts`) | 写し方 |
|---|---|---|
| `Individual.species` | `Individual.speciesKey` | `species.key`(`{図鑑番号4桁}-{フォルム3桁}`) |
| `Individual.nature {plus, minus}`(`""` は無補正) | `Individual.natureId` | `MasterData.natures` から (plus, minus) が一致する性格。無補正は無補正性格を ID の昇順で並べた最初のもの(ADR-0200 §2 と同じ規則)。該当なしは `unknown_nature`(Web 側で失敗) |
| `Individual.ability` | `abilityId` | `ability.id`。`""` は `null` |
| `Individual.item` | `itemId` | `item.id`。`null` は `null` |
| `Individual.sp` / `ranks` / `teraType` / `status` / `level` | 同名 | そのまま(`teraType` の `""` は `null`) |
| `CalcRequest.move` | `CalcRequest.moveId` | `move.id` |
| `critical` | `options.critical` | そのまま |
| `status` / `field.weather` / `field.terrain` の `""` | 省略 | WASM は `""` を「なし」として受けるが、API の enum は `""` を拒否する(400 `invalid_enum`)。送らずに API の既定(`none`)に任せる |
| `BulkRequest.presets`(カスタムのプリセット定義) | (無し) | API の `presets` はプリセット名(`DefenderPreset`)だけなので表せない。fetch せずに `invalid_preset`(`presetKeys` は `presets` に写す) |
| `typeChart` | (無し) | 送らない。API は相性表をサーバーのマスタから引く |
| `BulkRequest.defenderSpecies` / `itemVariants` | `defenderSpeciesKey` / `itemVariants: (string\|null)[]` | `key` / `id`(`null` は `null`) |
| `ReverseRequest.known` / `unknownSpecies` / `itemCandidates` | `known` / `unknownSpeciesKey` / `itemCandidates` | 同上 |
| 応答 `CalcResult` | `CalcResult` | 同じ形。`category` は ADR-0200 §1-3 で API にも入った |
| 応答 `BulkRow.defender.nature {plus, minus}` | `NatureModifier {plus\|null, minus\|null}` + `natureId` | `null` → `""`。`natureId` は画面が使わないので捨てる |
| 応答 `ReverseCandidate` | 同名 + `natureId` | 同上 |
| エラー `{code, message}` | HTTP ステータス + `Error {code, message}` | `code` をそのまま運ぶ(語彙は ADR-0200 §1-6 で WASM と共通)。本文が読めない・通信できないときは Web 側の `engine_unavailable` |

- `MasterData` に性格の一覧 `natures: {id, nameJa, plus, minus}[]` を足す(API の `Nature` と同じ形)。例データにも架空の性格を置く。

### 3. 端末 ID とセッション ID

- 全リクエストに `X-Device-Id`(端末 UUID。`localStorage` に保存して使い回す)と `X-Session-Id`(ページを開くたびに新しい UUID)を付ける(CLAUDE.md 技術規約)。
- UUID は `crypto.getRandomValues` から v4 の形で作る。`crypto.randomUUID` は安全なコンテキスト(HTTPS・localhost)でしか使えず、
  LAN の HTTP で開くと落ちるため。`localStorage` が使えない環境(プライベートブラウズ等)では、端末 ID もページごとの値で代える(失敗させない)。

### 4. モードの切り替え(既定はオフライン)

- ヘッダーに「オフライン(WASM)/ オンライン(API)」の切り替えを置く。選択は `localStorage` に覚える(端末ごとの好み)。
- **既定はオフライン**。オンラインで計算するには、サーバーのマスタと Web のマスタの ID が一致している必要があるが、
  いまの Web のマスタは架空の例データで、pokedex-svc(P2-3)からマスタを読む `MasterSource` はまだ作れない。
  pokedex-svc と gateway(P3-2)が揃った時点で、オンラインのときは API からマスタを読む `MasterSource` に切り替え、既定を見直す。
- API の基点 URL は環境変数 `VITE_API_BASE_URL`(既定は同じオリジン `/`。gateway が `/api` と Web の両方を配る想定。requirements.md §4)で1か所で読む。
  開発時は Vite の `server.proxy` で `/api` を calc-svc(または gateway)に送れるようにする(`API_PROXY_TARGET`。`VITE_` 接頭辞を付けず、クライアントのバンドルに入れない)。
- **WASM の遅延ロード(ADR-0011 §11 の宿題)**: オンラインのときは WASM を読まない。オフラインで最初に計算したときだけ読む(ADR-0300 §2 のまま)。
  オンラインで API に届かないとき、自動でオフラインに切り替えることはしない(どちらで計算したかが分からなくなるため)。
  エラー(`engine_unavailable`)を出し、切り替えは利用者が選ぶ。
- **取り消した計算は `engine_unavailable` にしない**(2026-09-24 追記。P4-18、issue 113、ADR-0300 §11)。
  画面が新しい入力で先行の要求を `AbortSignal` で取り消したときは、`fetch` の失敗を通信不能と同じ扱いにせず、
  `request_aborted`(`web/src/engine/types.ts` の `REQUEST_ABORTED_CODE`)を返す。自動フォールバックをしない方針は変えない。
- **契約外の 2xx も `engine_unavailable` にする**(2026-09-24 追記。P4-21、issue 67)。
  HTTP 200 で JSON として読めても、本文が契約(`api/openapi.yaml`)の応答の形でなければ、写像関数
  (`mapCalcResult` など)が `result.rolls` や `result.rows.map(...)` で例外になり、「計算は reject しない」
  という画面の前提(`CalcScreen.tsx` は `.then` しか登録しない)を破る。`createApiEngine` は成功応答を
  実行時に検証し、契約外なら `engine_unavailable` の `{ok: false}` を返す(§4 の「応答が読めない」に含める)。
  - **検証の範囲は写像関数が読むフィールド**。読むフィールドは、存在すること・JS 上の種類(数値 / 真偽値 /
    文字列 / 配列 / オブジェクト)が合うことを要求し、入れ子(`ko`・`rows[]`・`rows[].defender.nature`・
    `candidates[].ranges[]`)も同じ規則で再帰的に見る。こうすると、成功で返る DTO に `undefined` が入らない。
  - **列挙の値そのもの・数値の範囲・配列の件数は検査しない**(`category` が未知の文字列でも成功)。
    サーバーが語彙を増やしたときに Web が壊れないため。余分なフィールドも成功のまま(前方互換)。
  - **写像が `??` で既定値を補うフィールド(`ko.chancePercent`・`itemId`・`nature.plus/minus`)と、写像が
    捨てるフィールド(`natureId`)は、欠落・`null` を許す**。サーバー側が `omitempty` で省いた応答を、
    表示に影響しない項目のせいで落とさないため。
  - 本文が読めたうえでの契約違反は、`signal` が abort 済みでも `engine_unavailable` にする(通信は成立して
    おり、取り消しが原因ではないため。古い応答は画面が捨てる)。

### 5. 架空の例データの ID を契約の形に合わせる

- 種族の `key` は API の `SpeciesKey`(`^[0-9]{4}-[0-9]{3}$`。Shared Interfaces の Pokemon ID)に合わせ、`9001-000` の形にする
  (ADR-0300 §3 の「ID は `example-` で始める」を種族について改める。技・持ち物・特性・性格の ID は `example-` のまま)。
- ローカルでオンラインを試すため、Web の例データを calc-svc のスナップショット形式(`services/calc/README.md`)に書き出す
  スクリプト `web/scripts/export-example-master.mjs` を置く(出力は `data/generated/` 相当の .gitignore 済みの場所。生成物はコミットしない)。
  calc-svc をその出力で起動すれば、Web の例データの ID がそのまま通る(P4-6 の E2E でも使う)。

#### 5 追記(2026-09-25。`make e2e` の `web-e2e-online` 修復。ブランチ `fix/web-online-e2e-master-export`)

calc-svc の契約が、この節を書いた時点(2026-09-22)の暫定スキーマ(`services/calc/README.md` の
schemaVersion 1、相性表は `CALC_TYPECHART_PATH` で別出し)から、ADR-0204 の `MasterExport`
(`api/openapi.yaml`。相性表を本体に含める・`showdownId`/`type1`/`type2`/`abilities[{slot,abilityId}]`・
`MasterMove.effect`・効果のキーは PascalCase)に変わった。`web/playwright.online.config.ts` が
廃止済みの `CALC_TYPECHART_PATH` を渡し続け、`toCalcSnapshot()` の出力も旧スキーマのままだったため、
calc-svc が起動できず `make e2e` の `web-e2e-online` が壊れていた(issue 無し。plan.md 2026-09-25 着手分)。

- `web/src/master/exportSnapshot.ts`(`toCalcSnapshot`)を `MasterExport` の形に合わせて書き直した:
  `dataVersion`(`"example"` を含む固定文字列。呼ぶたびに変わらない)・`types`(`typeChart.types` から
  `{id, sortOrder, nameJa}`。`nameJa` は `i18n/ja.ts` の `typeNameJa` から引く)・`typeChart`(18×18=324件、
  等倍の既定は `code: 2`)を本体に足し、`species` は `type1`/`type2`(`types[0]`/`types[1] ?? null`)・
  `showdownId`(後述)・`isMega: false`・`baseSpeciesKey: null`・`requiredItemId: null`・
  `abilities: [{slot, abilityId}]` の形にし、`moves` に `effect: null`(例データに追加効果は無い)を足した。
- `items`/`abilities` の効果(`ItemEffect`/`AbilityEffect`)は Web の DTO の camelCase のキーのまま
  持っていたが、共通マスタ(`services/internal/master/effects.go` の `itemEffectFields`/`abilityEffectFields`/
  `absorbEffectFields`)は PascalCase を大文字小文字区別で照合するため、書き出し時に変換する
  (`toPascalCaseEffect`)。タイプ/ステータス ID をキーに持つ辞書(`statMods`・`defResistType`・
  `defAbsorbTypes` の外側のキー)は値であって変換対象のフィールド名ではないので変換しない。
  `defAbsorbTypes` の値(`AbsorbEffect`)のように、値自身がさらにフィールド名を持つオブジェクトの辞書は
  同じ規則を再帰的に適用する(例データには無いが、将来の入れ子の効果に備える)。
- 技・持ち物・特性の ID(`web/src/master/example/{moves,items,abilities}.ts`)からハイフンを除いた
  (`example-move-tackle` → `examplemovetackle`)。共通マスタの ID の形式(`services/internal/master/typechart.go`
  の `codeIDPattern = ^[a-z0-9]+$`)がハイフンを許さないため。**種族の `key` はこの節の元の決定どおり
  `SpeciesKey`(`9001-000`。ハイフン必須)のままで変更しない**。`showdownId`(ADR-0204 で新設された必須
  フィールド)は `key` からハイフンを除いた値(`"9001-000"` → `"9001000"`)にする。**性格の ID は
  `example-` のままで変更しない**(`buildNatures` は ID の形式を検査しない。calc-svc 側の契約どおり)。
- `web/playwright.online.config.ts` から `CALC_TYPECHART_PATH` と `testdata/golden/typechart.json` への
  参照を削除し、calc-svc には `CALC_MASTER_PATH` だけを渡す(ADR-0204 §「相性表は本体」に合わせる)。

**未解決(このタスクの範囲外として持ち越し)**: 上記の修正で calc-svc 自体は正しい `MasterExport` で
起動できることを確認した(`GET /healthz` が 200)が、`web/e2e/online.spec.ts` はまだ緑にならない。
`main.tsx` の「オンライン」モードは P4-16(ADR-0304)以降、`MasterSource` を `createOnlineMasterSource`
(pokedex-svc の公開 API `/api/pokedex/*` を読む)に固定で切り替える。`web-e2e-online` は calc-svc だけを
`go run`(pokedex-svc は起動しない)で立てる設計(2026-09-22 の DECISIONS.md の決定、この ADR の §5 の
前段)だが、calc-svc は担当外の pokedex の7操作を意図的に 404 で返す(`services/calc/internal/httpapi/server.go`
の `registerPokedexNotFoundRoutes`。critic 指摘 R1、P3-1 から変更なし)。このため `onlineSource.load()`
が `GET /api/pokedex/items` 等で reject し(`curl` で 404 を確認済み)、画面は「マスタデータの読み込みに
失敗しました」のまま止まり、`e2e/online.spec.ts` の2件がどちらもタイムアウトする。加えて、たとえ
items/natures が読めても `ONLINE_MASTER_CAPABILITIES.speciesList` が常に `false` のため `CalcScreen` は
種族の選択をプルダウンから `SpeciesSearchField`(検索入力)に切り替える設計であり、`e2e/support/calcPage.ts`
の `selectMatchup`(`<select>` への `selectOption`)とも噛み合わない。P4-16 が「オンライン」の意味を
(このADRの§5が前提にしていた「Webの例データ + calc-svcのみ」から)「pokedex-svcの公開APIから読む」へ
グローバルに変えたことで、`make e2e` が長らくスタブだった(issue #72・P4-22 で2026-09-25に初めて実行される
ようになった)間に生じていた既存の不整合と判断する。calc-svc に pokedex 相当のルートを持たせるのは
R1 の決定に反するため不可、pokedex-svc(MySQL 必須)を e2e に足すのは「k3d 不要」の設計(ADR-0306)に反する。
Web 側だけで完結する対処(例: e2e 専用の軽量な pokedex フィクスチャサーバーを別に置く、または
`web-e2e-online` の検証範囲を「オンラインは常に例データ + calc-svc」に戻す設計変更)が要るが、
どちらも新しい設計判断(と、それに伴う `e2e/online.spec.ts` 自体の書き換え)を要するため、このタスクの
「`toCalcSnapshot` を契約に合わせる」という範囲には含めず、別タスクとして残す。

### 6. テスト

- 単体: 写像(実体 → ID、応答 → DTO、`natureId` の選び方、エラーの写し)、UUID と端末 ID の保存、モードの保存。`fetch` は fake。
- 型: `openapi.gen.ts` の型で写像の関数を書き、`typecheck` が契約とのずれを検出する(生成型を手で複製しない)。
- 実サーバーに対する結合(calc-svc を例データで起動して同じ画面操作が両モードで同じ結果になる)は P4-6 の E2E に置く。

### 7. `make gen-ts`

- ルートの `Makefile` の `gen-ts`(Web の持ち物)を実装する: `openapi-typescript`(7.13.0。完全固定)で `web/src/api/openapi.gen.ts` を生成し、Prettier で整形する。
  生成物はコミットする(Go の `openapi.gen.go` と同じ。`make check-publishable-full` が `make gen` の差分で陳腐化を検出する)。
- `web/node_modules` が無ければ `gen-ts` は**失敗**する(黙ってスキップしない)。`api/openapi.yaml` を変えるレーン(API)は、一度 `make web-install` が要る。
- `openapi-typescript` の peer は `typescript ^5` だが、`typescript` は ESLint 用の 6 系の別名(ADR-0300 §1)なので、`overrides` でそれに合わせる。

## 却下・保留

- **画面を ID ベースにして WASM 側で実体に解決する**: WASM 経路に解決層がもう1つ要り、二重になる。
- **API に届かないとき自動で WASM にフォールバック**: どちらの結果か分からなくなる。明示の切り替えにした。
- **生成型を使わず手で API の型を書く**: 絶対ルール1に反する。
- **応答の検証を `try/catch` だけで済ませる**(2026-09-24、issue 67): 写像関数自身のバグも同じ `engine_unavailable` に
  握りつぶしてしまい、原因が分からなくなる(coding-rules §3「エラーは握りつぶさない」)。
- **JSON Schema のバリデータ(ajv 等)を `openapi.yaml` から生成して使う**(同): 契約の写しを手で書かずに済むが、
  依存の追加と `make gen` の配線が要る。検証したい範囲が「写像が読むフィールド」に限られる今は手書きの型ガードにし、
  balance も含めて検証が広がるようなら改めて検討する。
- **ブラウザでの実機確認(Chrome・Safari の MIME・instantiateStreaming・キャッシュ・メモリ)**: 人間の作業(plan.md P4-5 の小項目、ブロッカーに記載)。

## 影響

- `web/src/api/`(生成型と API 実装)、`MasterData.natures`、例データの種族キー、ヘッダーのモード切り替え、`web/scripts/export-example-master.mjs`。
- ルートの `Makefile` の `gen-ts`。API レーンは `make gen` の前に `make web-install` が要る(DECISIONS.md に記載)。
