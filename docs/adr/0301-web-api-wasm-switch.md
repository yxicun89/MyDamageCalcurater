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

### 5. 架空の例データの ID を契約の形に合わせる

- 種族の `key` は API の `SpeciesKey`(`^[0-9]{4}-[0-9]{3}$`。Shared Interfaces の Pokemon ID)に合わせ、`9001-000` の形にする
  (ADR-0300 §3 の「ID は `example-` で始める」を種族について改める。技・持ち物・特性・性格の ID は `example-` のまま)。
- ローカルでオンラインを試すため、Web の例データを calc-svc のスナップショット形式(`services/calc/README.md`)に書き出す
  スクリプト `web/scripts/export-example-master.mjs` を置く(出力は `data/generated/` 相当の .gitignore 済みの場所。生成物はコミットしない)。
  calc-svc をその出力で起動すれば、Web の例データの ID がそのまま通る(P4-6 の E2E でも使う)。

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
- **ブラウザでの実機確認(Chrome・Safari の MIME・instantiateStreaming・キャッシュ・メモリ)**: 人間の作業(plan.md P4-5 の小項目、ブロッカーに記載)。

## 影響

- `web/src/api/`(生成型と API 実装)、`MasterData.natures`、例データの種族キー、ヘッダーのモード切り替え、`web/scripts/export-example-master.mjs`。
- ルートの `Makefile` の `gen-ts`。API レーンは `make gen` の前に `make web-install` が要る(DECISIONS.md に記載)。
