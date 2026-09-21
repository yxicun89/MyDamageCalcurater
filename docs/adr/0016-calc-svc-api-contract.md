# ADR-0016: calc-svc の API 契約とマスタ境界

- 状態: 採用(2026-09-21。P3-1 の設計。受け入れ条件とテストは spec-writer が先に書き、実装は implementer)
- 日付: 2026-09-21
- 関連: ADR-0009(一括計算)、ADR-0010 §9・§R1〜§R8(逆算)、ADR-0011 §3〜§5・§10・§13(WASM 境界)、
  ADR-0013(相性表はデータ)、ADR-0015(相性表 JSON を読む前例)、plan.md P3-1

## 背景

engine(P1-7〜P1-13)と WASM 境界(`engine/wasmapi`)は新しい形になったが、`api/openapi.yaml` は P1 以前のまま
(ADR-0010 §9・§R8、ADR-0011 §10 が P3-1 に持ち越した差分)。calc-svc を作るにあたり、契約を先に直し、
「同じ失敗は HTTP と WASM で同じ code」「同じ入力は同じ数値」を固定する必要がある。
マスタはデータレーンの共通マスタ(`services/internal/master`。P2-2a)がまだ main に無い。

## 決定

### 1. `api/openapi.yaml` の変更(絶対ルール1。`make gen` で `services/internal/api/openapi.gen.go` を再生成)

1. **一括計算** `/api/calc/bulk`: description と `BulkCalcRequest.presets` に次を書く。
   変化技は none/hp の2件のみ・行の順序はプリセット優先(presets × itemVariants)・`presets: []` は省略と同じ(既定セット)・
   重複は 400 `duplicate_preset`。`BulkCalcRow.preset` は enum のみ(engine のカスタム `Presets` は API に出さない)。
2. `BulkCalcRow` に必須の `defender: BulkDefender = {sp, nature: NatureModifier, natureId, stats}`(stats は実数値)。
   `NatureModifier = {plus: StatKey|null, minus: StatKey|null}`(両方 required・nullable。無補正は両方 null)。
3. `CalcResult` に必須の `category: MoveCategory`(WASM と同じ。Web が型を共有するため)。
   `minPercent` / `maxPercent` の description から `ObservedPercent` への言及を消し、観測%(`Observation.percent`)とは別概念と書く。
4. 数値(`type: number`)にはすべて `format: double`。生成型を float64 にし、float32 で `chancePercent` などが崩れないようにする。
   表示%は engine の tenths を 10 で割るだけ(float で近似しない)。
5. **逆算**を P1-12 の形にする(ADR-0010 §R8):
   - `ReverseRequest` = required `[format, side, known, unknownSpeciesKey, moveId, observations]` +
     `field` / `options` / `itemCandidates: [string|null]`(null は持ち物なし。省略・空は `[null]`)/ `maxCandidates`(0 は無制限)。
     `known` は side=defender なら自分=攻撃側、side=attacker なら自分=防御側。旧 `attacker` / `defenderSpeciesKey` は削除。
   - `Observation` = `{percent? 1..100, percentTenths? 1..1000, damage? ≥1, note?}` の**ちょうど1つ**。旧 `observedPercent` は削除。
   - `ReverseCandidate` = `{natureClass, nature, natureId, itemId, ranges:[SPRange], spCount, exact, mismatch, support, minPercent, maxPercent}`。
     旧 `presetLabel` / `sp` / `matchScore` / `rangePercent` は削除。
   - `ReverseResult` = `{side, stat, assumedHpSp, exactCount, candidates}`。順序は ADR-0010 §R4。
6. **エラー**: `Error.code` を enum `ErrorCode` にする。語彙は WASM 境界の `Code*` 定数と同じ文字列 +
   HTTP だけのもの(`missing_header`、`unknown_species|move|item|ability|nature`、`not_found`、`master_unavailable`、`upstream_unavailable`)。
   ステータスの対応: 入力の不正と ID 不明はすべて 400、`not_found` は 404、`internal` は 500、`master_unavailable` / `upstream_unavailable` は 503。
   ただし `type_chart_missing` / `invalid_type_chart` は **500**(critic 指摘 O3)。HTTP では相性表は常にマスタ(起動時に
   読み込んだ `Store`)から来るので、この2つはクライアントの入力起因では起こらずマスタ側の不備そのものになる
   (WASM 境界はリクエストに `typeChart` を乗せるので 400 のまま。ADR-0011)。
   calc の3操作に `'500'` / `'503'` を足す。ヘッダの UUID 形式の検証は gateway(P3-2)の仕事で、calc-svc は欠落・空だけを `missing_header` にする。
   生成ラッパ(`api.ServerInterfaceWrapper`)がヘッダの検証で 400 を返すとき、「欠落・空」(bind 失敗を含む)は `missing_header`、
   それ以外(同名ヘッダの重複指定)は `invalid_input` にする(critic 指摘 R1)。

### 2. 性格 ID の写像(`natureId`)

engine は性格を構造値 `{Plus, Minus}` で持ち、ID を持たない。calc-svc がマスタの性格一覧から写す(`master.Store.NatureID`):

- 無補正(`engine.Nature.IsNeutral()`。plus == minus を含む)は、マスタの無補正性格を **ID の昇順**で並べた最初のもの。
- それ以外は (plus, minus) が一致する性格(複数あれば ID の昇順で最初)。
- 該当なしは `null`(エラーにしない。候補・行の提示を優先する)。

### 3. calc-svc の構成(`services` モジュール内。`services/go.mod` に `example.com/pokecalc/engine` を require + `replace ../engine`)

- `services/calc/internal/master`: **暫定のマスタ境界**。`Store` インターフェース(Species / Move / Item / Ability / Nature / NatureID / TypeChart)と、
  メモリ実装(`LoadSnapshot` + `LoadTypeChart` + `New`)。スナップショットの暫定スキーマは `services/calc/README.md`、架空データの例は
  `services/calc/testdata/master.example.json`。相性表は `testdata/golden/typechart.json` の schema(ADR-0015 と同じ検証: schemaVersion 1・
  未知のキー/フィールド・後続 JSON・effectiveness の欠落/空/null 行を拒否)。種族・技・持ち物・特性に相性表に無いタイプが現れたら
  `New` が `engine.ErrUnknownType` で拒否する(計算時の `unknown_type` を起動時に前倒しする)。
  ただし `teraType` はリクエスト由来の値であり `master.New` の起動時検査では防げないため、相性表に無ければ
  `engine.CalcDamage`(`validateAgainstTypeChart`)が都度チェックし、実行時に 400 `unknown_type` になる(critic 指摘 R6)。
  タイプ/ステータスキー/分類の綴りの検証は生成型 `api.PokeType` / `api.StatKey` / `api.MoveCategory` の `Valid()` を使い、
  独自の一覧を持たない(critic 指摘 O1)。`Store.Item` / `Store.Ability` は `*ItemEffect` / `*AbilityEffect`(内部の map を
  含む)もディープコピーして返す(critic 指摘 O2。呼び出し側が書き換えても Store に影響しないという doc の約束を Effect にも適用)。
  データレーンの `services/internal/master` が main に入ったら、`Store` の実装を差し替える(httpapi は `Store` にだけ依存する)。
- `services/calc/internal/httpapi`: 生成物 `api.ServerInterface` を実装する。流れは wasmapi と同じ
  「厳格デコード(未知フィールド拒否・末尾の余計なデータ拒否)→ 列挙の検証(個体ごとに status・teraType、リクエスト全体で
  format・weather・terrain)→ ID 解決 → engine の入力検証 → engine → 生成型への写し」。
  engine の sentinel → code の写像は wasmapi の `Code*` と同じ文字列を `api.ErrorCode` として**独立に**持つ(httpapi は
  `engine/wasmapi` に依存しない構成を保つため import はしない。実装当初の「`wasmapi.Code*` を import する」という記述は
  実態に合わせて訂正: critic 指摘 R2)。一致は `httpapi.TestWasmCodesAreValidErrorCodes` /
  `TestErrorCodeVocabularyMatchesWasm` で固定する。panic は回復して 500 `internal`(Go の内部情報を message に出さない)。
  sentinel に無い想定外のエラーは 500 `internal` の固定文だけを返し、詳細は `log/slog` にだけ残す(critic 指摘 O4)。
  **pokedex の5操作(searchSpecies / getSpecies / searchMoves / searchItems / listNatures)は生成ラッパ
  (`api.ServerInterfaceWrapper`。ヘッダの必須検証に加え q/limit/format などのクエリも解析する)を経由させず、
  `NewHandler` が直接 404 `not_found` を返す echo ハンドラを登録する**(calc の3操作だけがラッパ経由。critic 指摘 R1)。
  経由させると、ヘッダ欠落やクエリの型不一致(例 `limit=abc`)が `missing_header` / `invalid_json` 等に化けてしまい、
  「担当外の操作は常に `not_found`」という契約に反するため。
  echo の既定エラー(ルート無し・メソッド違い)も Error 形式にそろえる(ともに `not_found`)。
  リクエスト本文は 1MiB を上限にする(`http.MaxBytesReader`。超過は `invalid_json`)。`http.Server` に
  `ReadHeaderTimeout` 等のタイムアウトを設定する(critic 指摘 R7)。
- `services/calc/cmd/calc`: 環境変数 `CALC_ADDR`(既定 `:8080`)・`CALC_MASTER_PATH`・`CALC_TYPECHART_PATH`。起動時に両方を読み、失敗したら非ゼロ終了
  (フォールバックの既定データを持たない。ADR-0013)。`GET /healthz` は `200 {"status":"ok"}` で、**openapi には載せない運用エンドポイント**
  (クライアントの API ではなく k8s の probe 用。gateway は外に出さない)。計算はイベント保存に依存しない(絶対ルール5。P5-2 までイベント発行自体が無い)。

### 4. 仕様の細部(spec-writer が決めたこと)

- **`side` と `presets` は列挙で先に弾かない**。wasmapi は `side` / `presetKeys` を境界で列挙検証せず engine の sentinel に一本化している
  (ADR-0011 §4)。HTTP で `Valid()` を先に使うと未知の side が `invalid_enum`、未知の preset が `invalid_enum` になり WASM
  (`invalid_reverse_side` / `unknown_preset`)と食い違う。「同じ失敗は同じ code」を優先し、HTTP も engine に渡して
  `invalid_reverse_side` / `unknown_preset` にする。`side` の欠落も `invalid_reverse_side`。
  それ以外の列挙(`format`・`weather`・`terrain`・`status`・`teraType`)は生成型の `Valid()` で `invalid_enum`。
- **必須フィールドの欠落**は、そのフィールドの検証で落ちる: `format` の欠落は `invalid_enum`(空は列挙外。WASM は `""` を single とみなすが、
  HTTP では required)、`moveId` の欠落は `unknown_move`、`observations` の欠落は `no_observation`、`side` の欠落は上のとおり。
- **観測はキーの有無で数える**(HTTP だけ)。`percent: 0` は「指定したが範囲外」なので `{percent: 0, damage: 50}` は `invalid_observation`
  (契約上 percent は 1..100)。WASM はキーの有無を区別できず 0 を未指定とみなすが、失敗の食い違いではない(WASM では成功する入力で、契約違反)。
- `maxCandidates` が負なら 400 `invalid_input`(契約上 minimum 0。engine は負を無制限とみなすが、契約違反を黙って通さない)。
- **メソッド違い**(例 `GET /api/calc`)も 404 `not_found`。ErrorCode にメソッド違いの語彙を持たず、「その操作は無い」として扱う。
- `KOChance.chancePercent` は契約上 optional だが、calc-svc は WASM と同じく**常に返す**(engine の生値。確定・倒せないときは 0)。
- 持ち物なしは `itemId: null`(engine の空文字を nullable に写す)、無補正は `nature: {plus: null, minus: null}`。WASM は `""` のまま
  (パリティテストはこの表現の違いだけを正規化して比べる)。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-1 | 契約が OpenAPI として妥当で、検証ヘルパーが空振りしない(null の性格補正が通り、category 欠落・語彙外の code が落ちる) | `httpapi.TestContractDocumentIsValid` / `TestContractHelperIsNotVacuous` |
| AC-2 | `/api/calc` の成功は `engine.CalcDamage` の写し(rolls・min/maxDamage・defenderHP・effectiveness・stab・category・ko、表示%は tenths÷10)。本文の moveId が attacker.moveId より優先。Store 以外に依存しない | `TestCalcDamageMatchesEngine` / `TestCalcDamageMoveIDTakesPrecedence` / `TestCalcDamageNeedsOnlyStore` |
| AC-3 | `/api/calc/bulk`: 省略と `[]` が同じ既定セット、変化技は none/hp、指定順、preset-major、itemVariants の null、defender{sp,nature,natureId,stats}、重複は duplicate_preset | `TestCalcBulkDefaultPresets` / `TestCalcBulkRowOrderIsPresetMajor` / `TestCalcBulkNatureIDMapping` / `TestCalcBulkErrors` |
| AC-4 | `/api/calc/reverse`: side=defender / attacker の成功が `engine.CalcReverse` の写し(順序・ranges・spCount・exact・mismatch・support・表示%・natureClass・natureId・assumedHpSp・exactCount)、観測・side・ID の不正 | `TestCalcReverseMatchesEngine` / `TestCalcReverseNatureIDs` / `TestCalcReverseErrors` |
| AC-5 | エラーの共通語彙とステータス、検証の順序(wasmapi と同じ段の順: 構文 → format の列挙 → 個体ごとに列挙[status/teraType]と ID 解決 → 入力検証[Validate]は最後。critic 指摘 R5) | `TestCalcDamageErrorVocabulary` / `TestCalcDamageValidationOrder` |
| AC-6 | ヘッダの欠落・空は 400 missing_header(3操作)。UUID 形式は見ない | `TestMissingHeaders` |
| AC-7 | pokedex ルート・未知のルート・メソッド違いは 404 not_found(Error 形式)、panic は 500 internal(内部情報を出さない) | `TestPokedexRoutesAreNotFound` / `TestUnknownRoutesAreNotFound` / `TestPanicIsRecoveredAsInternal` |
| AC-8 | `GET /healthz` は 200 `{"status":"ok"}` | `TestHealthz` |
| AC-9 | WASM とのパリティ: 同じ失敗は同じ code、calc・bulk・reverse の成功は同じ数値(critic 指摘 R3 で reverse を追加) | `TestErrorCodeParityWithWasm` / `TestCalcResultParityWithWasm` / `TestBulkResultParityWithWasm` / `TestReverseResultParityWithWasm` |
| AC-10 | 起動: 設定の読み込み(必須・既定)、例のマスタで起動して計算できる、壊れたファイルで起動しない、ctx の終了で止まる | `cmd/calc.TestLoadConfig` / `TestNewHandlerServesExampleMaster` / `TestStartupFailsOnBadFiles` / `TestRunStopsOnContextCancel` |
| AC-M1〜M5 | マスタ境界: ロードと参照・コピーを返す・例のファイル・スキーマ違反の拒否・相性表の読み込みと拒否・New の整合性検査・NatureID の写像 | `master.TestLoadSnapshotAndLookup` ほか `services/calc/internal/master/master_test.go` |

契約の検証(test-strategy.md L4)は httpapi の各ケースで kin-openapi(`openapi3filter` + `routers/legacy`)を使う。
P3-3 で gateway 経由のものを足す。

## 却下した案

- **列挙をすべて生成型の `Valid()` で先に検証する**(side・presets を含む): WASM と code が食い違う(§4)。
- **`CalcResult.category` を落とす**(Web は要求から分類を知っている): WASM の結果に category があり、Web が型を共有するときに
  片方だけに無いフィールドを作る。1フィールドで済むので足す。
- **natureId が見つからなければ 500 / 400**: 候補・行の提示が止まる。性格は表示の補助なので null にする。
- **データレーンの `services/internal/master` を待つ**: API レーンが止まる。`Store` インターフェースで差し替え可能にし、暫定の実装を置く。
- **`/healthz` を openapi に載せる**: クライアントの API ではなく、生成クライアントに運用エンドポイントを混ぜたくない。
- **メソッド違いに 405 と新しい code を足す**: 語彙は §1-6 の一覧に留める(API レーンの既定案。ユーザー未確認)。「操作が無い」は not_found で表せる。

## 影響

- Web(P4-5)は生成型の変更(`category`・`BulkCalcRow.defender`・逆算の形・`ErrorCode`)に追従する。
- gateway(P3-2)は `upstream_unavailable`(503)と UUID 形式の検証を担う。
- ADR-0010 §9・§R8 と ADR-0011 §10 の P3-1 への持ち越しは、この ADR で解消する(Web 側の追従は P4-5)。
- 共通マスタ(P2-2a)が入ったら `services/calc/internal/master` を差し替え、暫定スキーマと例のファイルを削除する。
