# ADR-0108: engine / engine/wasmapi の候補・観測件数の上限(issue #110 追従)

- 状態: 採用(2026-09-23。issue #110・ADR-0208 §4 のデータレーン依頼分。critic レビュー予定)
- 日付: 2026-09-23
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #110、ADR-0208(API レーンの契約・calc-svc の検証)、ADR-0009(一括計算)、
  ADR-0010 §R(逆算)、ADR-0011(WASM 境界)

## 背景

ADR-0208 §4 は、`services/calc/internal/httpapi` に置いた件数上限の検証(HTTP 経由の入力に対する防御)
を、データレーンにも同じ値で依頼している。理由は、HTTP を経由しない直接呼び出し(ネイティブ Go の
呼び出し元・テスト)と WASM(ブラウザ)は calc-svc の検証を一切通らないため、`engine.CalcBulk` /
`engine.CalcReverse` に上限が無いままだと、issue #110 が指摘した計算量の増幅がそのまま残るため。

## 決定

### 1. 上限の値は ADR-0208 §1 と完全に一致させる

| 定数(`engine` パッケージ) | 値 | 対応する ADR-0208 の契約 |
|---|---|---|
| `MaxBulkPresets`(bulk.go) | 8 | `BulkCalcRequest.presets` の `maxItems` |
| `MaxBulkItemVariants`(bulk.go) | 64 | `BulkCalcRequest.itemVariants` の `maxItems` |
| `MaxReverseItemCandidates`(reverse.go) | 64 | `ReverseRequest.itemCandidates` の `maxItems` |
| `MaxReverseObservations`(reverse.go) | 16 | `ReverseRequest.observations` の `maxItems` |
| `MaxReverseMaxCandidates`(reverse.go) | 128 | `ReverseRequest.maxCandidates` の `maximum` |

値を2箇所(HTTP 契約と engine)に書くのは重複だが、`services/calc` と `engine` は別 Go module
(`go.work` で束ねているだけ)で、契約定数を共有パッケージにするほどの理由が無いため、
コメントで対応関係を明記して手で揃える(ADR-0208 §1 の値が変わったらここも変える)。

### 2. `CalcBulk` は `Presets` と `PresetKeys` の両方の件数を見る

HTTP 層(ADR-0208)は `BulkCalcRequest.presets`(= engine の `PresetKeys` に変換される。
`presetKeysFrom`)だけを検査すればよいが、`engine.BulkInput` は `Presets`(カスタム定義。
API には出ない、直接呼び出し専用の入口)と `PresetKeys` の両方を公開 API として持つ
(ADR-0009 §3)。直接呼び出し・WASM はどちらの入口も通れるため、両方に
`MaxBulkPresets` を適用する。`selectPresets` が実際に何行になるかを計算する前に検査するので、
`Presets` を使う経路(`PresetKeys` が空で `Presets` をそのまま行として使う)も、
`PresetKeys` で選ぶ経路も、どちらも 8 件で頭打ちにする。

### 3. 検証の位置: 各関数の最初(選択・検証ロジックより前)。wasmapi は DTO 変換より前

`CalcBulk` は `selectPresets`(重複検査・カタログ引き当て)より前、`CalcReverse` は
`Side` の検証の直後・`validateObservations`(観測1件ごとの内容検証)より前に置く。
ADR-0208 §3 の「巨大な入力に対して以降の一切の追加の仕事をしない」という考え方を、
engine 内の以降の処理(`CalcDamage` の呼び出しループ)にもそのまま適用する。

`engine/wasmapi`(`bulkRequest.run` / `reverseRequest.run`)でも、同じ件数チェックを
DTO の変換(`itemsToEngine`・プリセットの `nature`/`applies` の解析・観測の変換)より前に
重ねて置いた(独立レビュー指摘)。理由: DTO 変換は1件ずつ全件を検証してから初めて
`engine.CalcBulk`/`CalcReverse` に届くため、変換だけを先に置くと「件数超過」と「未知の enum」
のような複数の違反が同じリクエストに重なったとき、HTTP(件数を先に見る。ADR-0208 §3)と
WASM(先に変換のエラーが出る)で code が食い違いうる。件数チェックを DTO 変換より前に
置くことで、単一の違反はもちろん複数の違反が重なった場合も HTTP と同じ `invalid_input` が
先に出るようにした。`engine.CalcBulk`/`CalcReverse` 自身の検査(決定1〜3)は、
wasmapi を経由しない直接呼び出し(ネイティブ Go)のための独立した防御として残す
(二重チェックだが、届く経路が異なるため両方必要)。

### 4. `MaxCandidates` の負の値は「無制限」ではなく不正にする(既存挙動の変更)

これまで `CalcReverse` は `MaxCandidates <= 0` を「無制限」として扱っていた
(`if in.MaxCandidates > 0 && in.MaxCandidates < n { n = in.MaxCandidates }`)。
`engine/reverse_test.go` の `TestReverseOrderDeterministic` に
「`MaxCandidates=-1` は無制限」という既存テストがあった。

ADR-0208 §1 の契約は `minimum: 0` で、負の値は 400 `invalid_input`(0 だけが「無制限」)。
直接呼び出し・WASM で HTTP と異なる結論(負を許す)になると、同じ入力が経路によって
成功したり失敗したりするため、**engine 側も負を不正にする**(0 は従来どおり無制限のまま)。

既存テストは仕様変更として書き換えた(CLAUDE.md 絶対ルール6。期待値の変更理由をここに記録):
削除ではなく「負は `ErrInvalidMaxCandidates` になる」という新しい主張に置き換えた。

### 5. 新しい sentinel は5つ、いずれも `wasmapi` では `invalid_input` に写す

`ErrTooManyPresets` / `ErrTooManyItemVariants`(bulk.go)、
`ErrTooManyItemCandidates` / `ErrTooManyObservations` / `ErrInvalidMaxCandidates`(reverse.go)。

ADR-0208 §2 の「新しい ErrorCode は足さない」という判断を engine/wasmapi 側にも合わせ、
`engine/wasmapi/wasmapi.go` の `errorResponse` の sentinel 一覧に5つとも `CodeInvalidInput` へ
写す対応を追加した。Go の sentinel(engine 内部の識別子)自体は、他の失敗と区別してテスト・
呼び出し側が `errors.Is` で判別できるよう分けたままにする(HTTP の code を増やすことと、
engine 内部でエラーの種類を区別できることは別の話)。

### 6. ベンチマークは追加しない(ADR-0208 の実測を流用)

ADR-0208 は HTTP 層で `BenchmarkCalcBulkAtLimit` / `BenchmarkCalcReverseAtLimit` を計測済み
(上限ちょうどで bulk 約 3.0ms、reverse 約 25ms)。HTTP ハンドラは検証を通過した後
`engine.CalcBulk` / `CalcReverse` をそのまま呼ぶだけなので、計測されているコストは
engine 本体のものと同一である。engine 側に同じ内容のベンチマークを重複して置く価値は薄いため、
上限ちょうどで**成功する**ことを固定するテスト(`TestCalcBulkAtLimitSucceeds`・
`TestReverseValidationErrors/上限ちょうどの件数・範囲は受け付ける`)だけを足す。

## 受け入れ条件

1. `engine.CalcBulk` は `len(Presets) > 8`・`len(PresetKeys) > 8`・`len(ItemVariants) > 64` を
   それぞれ専用の sentinel で拒否する。上限ちょうど(8 プリセット × 64 持ち物 = 512 行)は成功する。
2. `engine.CalcReverse` は `len(ItemCandidates) > 64`・`len(Observations) > 16`・
   `MaxCandidates < 0 || MaxCandidates > 128` をそれぞれ専用の sentinel で拒否する。
   上限ちょうど(2性格クラス × 64持ち物 = 128候補、観測16件、MaxCandidates=128)は成功する。
3. `MaxCandidates = 0` は従来どおり無制限(受け入れ条件2で新たに拒否されるのは負と129以上だけ)。
4. `engine/wasmapi` の `CalcBulk`/`CalcReverse` が同じ上限超過を `invalid_input` として返す
   (HTTP と同じ code。ADR-0208 §2 との parity)。
5. `make test-golden` は不変(ダメージ計算そのものは変更していない)。

## 却下した案

- **上限を `engine` と `services/calc` で共有する定数パッケージに切り出す**: 2つの Go module を
  またぐ依存を新設することになり、値が2〜3個ずれても実害が小さい割に構成が複雑になる。
  コメントでの対応関係の明記で足りると判断した。
- **`MaxCandidates` の負を今までどおり「無制限」のままにする**: HTTP 経由と直接呼び出し/WASM で
  結論が変わってしまう(決定4)。

## 影響

- 変更: `engine/bulk.go`・`engine/reverse.go`(上限の追加のみ。ダメージ計算は不変)、
  `engine/wasmapi/wasmapi.go`(sentinel → `CodeInvalidInput` の対応追加)。
- 既存テストの変更: `engine/reverse_test.go` の `TestReverseOrderDeterministic`
  (`MaxCandidates=-1` の期待値を「無制限」から「`ErrInvalidMaxCandidates`」に変更。決定4)。
- Web・iOS レーンへの依頼(ADR-0208 §4 のとおり、この ADR では実装しない):
  観測 UI を16件で無効化、持ち物候補が64件を超える場合の扱いを決める。
