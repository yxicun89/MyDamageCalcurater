# ADR-0204: calc-svc のマスタを pokedex-svc の内部 API から受け取る

- 状態: 提案(2026-09-22。ユーザー決定に基づく API レーンの設計。受け入れ条件とテストは spec-writer が先に書き、実装は implementer)
- 日付: 2026-09-22
- 関連: ADR-0002(実マスタをコミットしない)、ADR-0005(データ駆動の効果定義)、ADR-0012 §6(共通マスタのための実行時依存)、
  ADR-0013(相性表はデータ)、ADR-0100(pokedex のスキーマと写像)、ADR-0101(importer)、ADR-0200(calc-svc の契約と暫定マスタ境界)、
  ADR-0202(gateway のルーティング)、ADR-0203(k3d デプロイとスモーク)、CLAUDE.md 絶対ルール 1・4・5

## 背景

calc-svc(ADR-0200)は起動時に「暫定スナップショット」(calc-svc 独自の JSON)と相性表 JSON を読む。
データレーンの共通マスタ(`services/internal/master`。DB の行 → engine の型の写像)が main に入り、
正本の pokedex の DB(ADR-0100)から calc-svc へマスタを渡す経路を決める必要がある。
ユーザー決定(2026-09-22): 「calc-svc は起動時に pokedex-svc の内部エンドポイントからマスタ一式(行の形の JSON)を取得し、
services/internal/master の写像でメモリに載せる。gateway には出さない。マスタの正本は pokedex の DB 1つ(絶対ルール4)。
pokedex-svc(P2-3)ができるまでは同じ形式のファイルと架空データで作る。取得できない間は 503 master_unavailable。
契約は API レーンが openapi に定義し、実装はデータレーン」。

ADR-0012 §6 は「共通マスタのためだけに新しい実行時サービス間依存を追加しない」としていた。この ADR は、calc-svc → pokedex-svc
の起動時の取得についてだけ、ユーザー決定によりその例外を置く(balance の read model 方針は変えない)。
依存は起動時の1回だけで、取得後は pokedex-svc が落ちても計算は続く(絶対ルール5 の精神。計算は record/team/NATS にも依存しない)。

## 決定

### 1. 契約(`api/openapi.yaml`。`make gen` で Go・TS を再生成)

- タグ `internal`(新設): サービス間の内部 API。gateway は公開しない。端末ID/セッションID は要らない。
- `GET /internal/pokedex/master`(operationId `getMasterExport`)。200 は `MasterExport`、503 は `Error`(`master_unavailable`。
  DB に未投入・DB に接続できない)。パラメータ(ヘッダ)を持たない。
- `MasterExport` = required `[schemaVersion, dataVersion, types, typeChart, species, moves, items, abilities, natures]`。
  形は DB の行(共通マスタの `TypeRow` / `TypeChartRow` / `SpeciesRow` + `SpeciesAbilityRow` / `MoveRow` / `ItemRow` / `AbilityRow`)に対応する:

| フィールド | 要素の形 | 対応 |
|---|---|---|
| `schemaVersion` | integer(enum [1]) | この形の版 |
| `dataVersion` | string(minLength 1) | pokedex の data_versions に由来する版の識別子。calc-svc は記録とログに使うだけ |
| `types` | `{id: PokeType, sortOrder, nameJa}` | types |
| `typeChart` | `{attackType: PokeType, defenseType: PokeType, code: 0/1/2/4}` | type_chart。等倍は省略可(ADR-0013) |
| `species` | `{key, dexNo, form, showdownId, nameJa, type1, type2 (nullable), baseStats: StatBlock, isMega, baseSpeciesKey (nullable), requiredItemId (nullable), abilities: [{slot, abilityId}]}` | species + species_abilities |
| `moves` | `{id, nameJa, type, category, power, priority}` | moves(計算に使う列だけ) |
| `items` / `abilities` | `{id, nameJa, effect: object \| null}` | items + item_effects / abilities + ability_effects |
| `natures` | `{id, nameJa, plus: StatKey \| null, minus: StatKey \| null}` | natures(未作成。§5 の提案) |

- `effect` は item_effects / ability_effects の JSON をそのまま(`MasterEffect` = `type: object, additionalProperties: true, nullable: true`)。
  形の正は共通マスタの `DecodeItemEffect` / `DecodeAbilityEffect`(キーは engine のフィールド名 `DamageMod` 等。ADR-0005)。
- 使用可能集合(レギュレーション)で絞らない(計算は全件を扱える必要がある。絞り込みは pokedex の検索の仕事)。
- `nameEn` は含めない(calc に不要)。**`showdownId` は含める**(下の「spec-writer が決めたこと」)。

### 2. calc-svc のマスタ境界(`services/calc/internal/master`)

- 暫定スナップショット形式(`LoadSnapshot` / `LoadTypeChart` / `New` / `Snapshot` と README の暫定スキーマ)を**廃止**し、入力を生成型
  `api.MasterExport` に一本化する。`Store` インターフェースと `NatureID` の規則(ADR-0200 §2)は変えない。
- `FromExport(api.MasterExport) (*MemoryStore, error)`: 共通マスタの写像(`TypeChart` → `Species` / `Move` / `Item` / `Ability`)で engine の型にする。
  - 性格は calc-svc 側で検証する(ID が空・重複、plus/minus が StatKey でない・HP を指す)。
  - 種類ごとの ID の重複、参照先の欠落(種族の特性 → abilities、メガの `baseSpeciesKey` → species、`requiredItemId` → items)は
    ロード失敗(DB では外部キーが保証するが、ファイル方式でも同じ規則にする)。
  - 失敗はすべて `ErrInvalidMaster` で包み、共通マスタの `ErrInvalidRow` / `ErrInvalidEffect` と `engine.ErrInvalidTypeChart` も
    `errors.Is` で判別できるようにする。部分的な Store を返さない。入力の map を Store と共有しない。
  - `MemoryStore.DataVersion()` で `dataVersion` を返す(ログ用。`Store` インターフェースには足さない)。
- `DecodeExport(io.Reader)`: 厳格デコード(未知のフィールド・後続のデータ・必須のトップレベルの欠落/null を拒否)。
  効果定義の数値は字面のまま保つ(`json.Decoder.UseNumber` 等。`5324.0` を `5324` に丸めて共通マスタの整数検査をすり抜けさせない)。
- 入手元は `Source` インターフェース(`Fetch(ctx) (api.MasterExport, error)`):
  - `FileSource{Path}`: JSON ファイル(k3d の local overlay と `make dev`)。
  - `NewHTTPSource(baseURL, timeout)`: `GET {baseURL}/internal/pokedex/master`。URL は http/https の絶対 URL(クエリ不可)、
    タイムアウトは正。接続できない・タイムアウト・200 以外は `ErrMasterUnavailable`、本文の不正は `ErrInvalidMaster`、
    呼び出し側の ctx の終了は ctx のエラー。端末ID/セッションID は付けない。

### 3. 起動(`services/calc/cmd/calc`)と準備状態(`services/calc/internal/httpapi`)

- 環境変数は `CALC_MASTER_URL`(pokedex-svc のベース URL)と `CALC_MASTER_PATH`(ファイル)の**ちょうど1つ**(空は未設定と同じ)。
  両方・どちらも無しは起動エラー。**`CALC_TYPECHART_PATH` は廃止**し、設定されていたら起動エラーにする(古い設定に気づかせる)。
- ファイル方式: 起動時に読み、失敗なら非ゼロ終了(従来どおり)。`/readyz` は最初から 200。
- URL 方式: HTTP サーバはすぐ起動し、バックグラウンドで取得を再試行する(指数バックオフ。既定 0.5 秒から倍々、上限 30 秒。
  1回のタイムアウト 10 秒。ctx の終了で止まる)。取得・検証の失敗はどちらも再試行する(pokedex-svc の投入中・修正後に自然に回復する)。
  - 取得できるまで: calc の3操作は 503 `master_unavailable`、`GET /healthz` は 200(liveness)、新設の `GET /readyz` は
    503 `master_unavailable`(readiness)。取得後: `/readyz` は 200 `{"status":"ok"}`。
  - **取得後の再取得はしない**。マスタの更新(importer の CronJob)は calc-svc の再起動(`kubectl rollout restart`)で反映する。
    理由: 計算中にマスタが入れ替わると、同じ入力の結果が途中で変わる。版の切り替えを起動の単位にそろえる方が追いやすい。
- httpapi は `NewDeferredHandler(current StoreFunc)` を持つ(`StoreFunc` は準備中なら nil を返す)。`NewHandler(store)` は従来どおりで、
  `/readyz` は常に 200。`/readyz` も `/healthz` と同じく openapi に載せない運用エンドポイント。
- calc-svc は `GET /internal/pokedex/master` を提供しない(生成物 `api.ServerInterface` を満たすメソッドだけ置き、ルートに登録しない)。

### 4. k3d / dev

- local overlay は引き続き**ファイル方式**。`services/calc/testdata/master.example.json` を MasterExport の形(架空データ + 相性表の行)に置き換え、
  `deploy/k8s/overlays/local/api/master.example.json` はそのコピー(一致テストは維持)。`typechart.example.json` と ConfigMap `calc-typechart` は削除。
- base の calc Deployment の readinessProbe を `/readyz` に(liveness は `/healthz` のまま)。
- gateway は `/internal/*` を 404 `not_found` のまま通さない(ルーティングは前方一致の許可リスト。ADR-0202 §3)。テストとスモークで固定する。
- `scripts/dev.sh` は `CALC_MASTER_PATH` だけを渡す。

### 5. データレーンへの提案(実装はデータレーン)

- pokedex-svc(P2-3)で `GET /internal/pokedex/master` を実装する。クラスタ内の Service だけで公開し、Ingress には出さない。
  DB に未投入なら 503 `master_unavailable`。
- `natures` テーブル(`id, name_ja, plus, minus`)を追加し、`/api/pokedex/natures` と内部 API の両方で使う。
- k3d で calc の local overlay を URL 方式(`CALC_MASTER_URL=http://pokedex`)に切り替えるのは、pokedex-svc のデプロイ後に API レーンが行う。

### spec-writer が決めたこと

- **`showdownId` を MasterExport の種族に含める**(依頼の案では「calc に不要なので含めない」)。共通マスタの `Species` が
  `ShowdownID` の形式(`^[a-z0-9]+$`)を検証するため、含めないと calc-svc が偽の値を作るしかなく、写像の検証を黙って弱めることになる。
  共通マスタは変更禁止なので、契約の側に1フィールド足す方を選んだ。
- 技・持ち物・特性の ID は共通マスタの形式に合わせる(例のファイル・calctest・smoke.sh の `test-beam` → `testbeam` など)。
  性格の ID の形式は calc-svc では検査しない(pokedex のスキーマが未定のため)が、例のファイルは同じ形にそろえた。
- 参照先の欠落(特性・メガの元種族・メガストーン)をロード失敗にする(§2)。
- `CALC_TYPECHART_PATH` が設定されていたら起動エラー(§3)。
- `/readyz` の 503 の本文は Error 形式(`master_unavailable`)。
- 再試行の既定値(0.5 秒・上限 30 秒・1回 10 秒)は環境変数にしない(使われていない設定項目を作らない。テストは config を直接組む)。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-C1 | 契約: `GET /internal/pokedex/master` がヘッダ無しで契約に合い、例のファイルと偽の上流の本文(200・503)が MasterExport / Error に準拠する。検証は空振りしない | `master.TestExampleExportMatchesContract` / `TestHTTPSourceFixturesMatchContract` / `TestMasterExportContractIsNotVacuous` |
| AC-M1 | FromExport で engine の型が引ける(相性表・単タイプ・メガ・効果・性格・dataVersion)。コピーを返し、入力と共有しない | `TestFromExportAndLookup` / `TestFromExportTypeChart` / `TestLookupUnknownIDs` / `TestLookupReturnsCopies` / `TestLookupReturnsCopiesOfEffects` / `TestFromExportDoesNotAliasInput` |
| AC-M2 | 不正はロード失敗(ErrInvalidMaster + 共通マスタ・engine の sentinel) | `TestFromExportRejectsInvalid` |
| AC-M3 | 例のファイルが読める。厳格デコード。効果の数値を字面のまま渡す | `TestExampleExportLoads` / `TestDecodeExportRejectsInvalid` / `TestDecodeExportKeepsEffectLiterals` |
| AC-M4 | FileSource・HTTPSource(パス・ヘッダ無し・非 200・壊れた本文・未知のフィールド・接続拒否・タイムアウト・ctx) | `TestFileSource` / `TestNewHTTPSourceRejectsInvalidConfig` / `TestHTTPSourceFetchesExport` / `TestHTTPSourceFailures` / `TestHTTPSourceConnectionRefused` / `TestHTTPSourceTimeout` / `TestHTTPSourceHonorsContext` |
| AC-M5 | NatureID の規則は ADR-0200 §2 のまま | `TestNatureID` / `TestNatureIDNoNeutralInMaster` |
| AC-R1〜R4 | 準備中は3操作と /readyz が 503 master_unavailable(契約どおり)、/healthz は 200。準備後は NewHandler と同じ。NewHandler の /readyz は 200。calc-svc は内部 API を 404 | `httpapi.TestMasterUnavailableWhileNotReady` / `TestProbesWhileNotReady` / `TestDeferredHandlerBecomesReady` / `TestDeferredHandlerMatchesNewHandler` / `TestReadyzWithLoadedStore` / `TestMasterExportRouteIsNotFound` |
| AC-B1〜B5 | 設定(ちょうど1つ・URL の検証・CALC_TYPECHART_PATH の拒否)、バックオフ、ファイル方式の起動と失敗、URL 方式の 503→200・失敗の再試行・再取得しない・ctx で止まる・上流なしでも起動 | `cmd/calc.TestEnvNames` / `TestDefaultMasterFetchSettings` / `TestLoadConfig` / `TestBackoffDelay` / `TestNewHandlerServesExampleMaster` / `TestStartupFailsOnBadFiles` / `TestRunStopsOnContextCancel` / `TestURLModeBecomesReadyAfterUpstream` / `TestURLModeRetriesOnInvalidExport` / `TestURLModeStopsRetryingOnCancel` / `TestRunURLModeStartsWithoutUpstream` |
| AC-D1 | k8s: readiness は /readyz、base にマスタの設定なし、local はファイル方式(CALC_MASTER_PATH だけ・コピー一致・相性表のコピーなし)、dev.sh に CALC_TYPECHART_PATH なし | `TestManifestCalcWorkload` / `TestManifestCalcBaseHasNoLocalData` / `TestManifestCalcLocalDataFromOverlayCopies` / `TestManifestCalcLocalHasNoTypeChartCopy` / `deploytest.TestDevScript` |
| AC-G1 | gateway は `/internal/*` を 404 not_found にし、どの上流にも届けない(スモークでも確認) | `gateway/internal/httpapi.TestUnroutedPathsAreNotFound` / `deploytest.TestSmokeScriptPassesAgainstGatewayAndCalc` |

## 却下した案

- **calc-svc が pokedex の DB に直接つなぐ**: 絶対ルール4 に反する。
- **pokedex が read model のファイルを出力し calc-svc がそれを読む(ADR-0100 §8 の balance と同じ形)**: ユーザー決定で API にした。
  ファイル方式は k3d local と dev 用に残す(同じ形なので切り替えは環境変数だけ)。
- **リクエストごとに pokedex-svc を引く**: 計算が pokedex-svc の可用性に依存する(絶対ルール5 の精神に反する)。
- **定期的に再取得する**: 計算中にマスタが変わる。更新は再起動の単位にそろえる(§3)。
- **URL 方式でも起動時に取得できなければ終了する(CrashLoop)**: pokedex-svc と同時に起動する通常の流れで再起動を繰り返す。
  readiness で宛先から外す方が状態が読みやすい。
- **`showdownId` を含めず calc-svc が偽の値を入れる**: 共通マスタの検証を黙って弱める(上の「spec-writer が決めたこと」)。

## 影響

- Web: `web/src/api/openapi.gen.ts` に内部 API と MasterExport の型が増える(生成物だけ。手書きコードは変わらない)。
- iOS: `ios/PokeCalcKit/Sources/PokeCalcAPI/Generated/` は `make ios-gen` で再生成が要る(`getMasterExport` が増える)。
  内部 API をクライアントに出したくなければ iOS の生成設定で `internal` タグを除外する(iOS レーンの判断)。
- データレーン: §5 の提案(内部 API の実装・natures テーブル)。
- ADR-0200 §3 の暫定マスタ境界は、この ADR で置き換える。
