# ADR-0321: wasmapi(オフライン計算)にメガ種族の持ち物規則を入れ、calc-svc と揃える(issue #505)

- 状態: 採用
- 日付: 2026-10-03
- レーン: データ(engine/wasmapi)
- 関連: issue #505・#315、ADR-0011(WASM 境界)、ADR-0200 §4 追記(calc-svc の規則)、ADR-0320(Web のメガ持ち物固定)、ADR-0002 決定 10

## 背景

calc-svc は、メガ種族(`isMega`)に `requiredItemId` 以外の持ち物を持たせた計算を 400 `invalid_input` で拒否する(ADR-0200 §4 追記)。
一方 `engine/wasmapi` の種族 DTO は `isMega`・`requiredItemId` を持たず、オフライン計算は同じ入力を計算してしまう。
正しい入力の結果は一致し、違うのは不正入力の扱いだけだった。

## 決定

1. **規則は calc-svc と同じ**: メガ種族は `requiredItemId` の持ち物か持ち物なし(null・省略)だけを受け付ける。別の持ち物は `invalid_input`。
   `requiredItemId` が無い(null・省略・空)メガ種族は、どの持ち物も持てない。`isMega` が false の種族は何も検査しない。新しい code は足さない。
2. **メッセージも同じ文言**(種族キー・持ち物 ID・入力の場所を含む)。入力の場所のラベルは calc-svc と同じ:
   calc は `攻撃側`・`防御側`、bulk は `攻撃側` と `itemVariants[i]`(防御側がメガ種族のとき)、reverse は `既知の側` と `itemCandidates[i]`(推定側がメガ種族のとき)。
   候補に1つでも違反があれば要求全体を拒否する(黙って除外しない)。
3. **検証の位置**: DTO 変換と `Individual.Validate`(SP・ランク等)の後、engine 呼び出しの前。SP 超過などが先に報告される(calc-svc の「ID 解決・SP 検査の後」と同じ段)。
4. **データの持ち方**: `speciesDTO` に `isMega`(bool。省略は false)と `requiredItemId`(string。null・省略は空)を足す。`engine.Species` には渡さない
   (engine は変更しない。検証は wasmapi 境界の `mega.go` だけ)。メガ種族・メガストーンの一覧は持たず、リクエストの種族が持つ値だけを見る
   (CLAUDE.md: マスタをハードコードしない)。値の出どころはマスタの `MasterSpecies.isMega`・`requiredItemId`(公開 API に既にある)。
5. **parity を固定する**: `services/calc/internal/httpapi/mega_parity_test.go`(HTTP と wasmapi に同じ入力を渡し、成否・code・メッセージの一致を見る)と、
   Go/WASM 一致テスト(`make test-wasm`)の `engine/wasmapi/testdata/vectors.json` に、メガの成功ベクタ3本(calc・bulk・reverse)と、拒否ベクタ3本を足す。
   拒否ベクタは `expectError`(エラー code)を持ち、成功ベクタ用の一致テストの対象から外す。`TestErrorVectorsReturnExpectedCode` が code を、
   `scripts/wasm-conformance.mjs` がネイティブ Go と WASM のバイト一致を見る。

## Web への影響(未対応・次の作業)

Web の `toEngineSpecies`(`web/src/domain/requests.ts`)は ADR-0320 により `isMega`・`requiredItemId` を境界へ渡さない(当時の境界が `unknown_field` で拒否するため)。
この ADR で境界は両方を受け付けるようになったが、Web が渡すまではオフライン計算に規則は効かない(従来どおり)。Web は計算・逆算の持ち物欄をメガ種族ではストーンに
固定しているので通常の操作では起きない入力であり、渡す変更(`toEngineSpecies` と ADR-0320 の対応テスト `requests.mega.test.ts` の更新)は Web レーンの次の作業とする。

## 結果

- 通常種族の応答は変わらない(既存ベクタ・ゴールデンは不変)。
- 追加の engine 変更なし。
