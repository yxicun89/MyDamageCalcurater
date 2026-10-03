## 2026-09-24: issue #73(OpenAPIとengineの防御プリセット集合を同期検査する)を修正(API レーン)
Decision: `api/openapi.yaml` の `DefenderPreset` enum と `engine.DefenderPresetCatalog()`(`engine/bulk.go`)は
1対1対応が前提(`services/calc/internal/httpapi/convert.go` の `presetKeysFrom` は変換テーブルを持たず、契約の
列挙値をそのまま `engine.PresetKey` に型変換するだけ)だが、これを固定する回帰テストが無かった
(実際のズレは無かった。issue #73 が問題にしていたのは「テストの欠落」自体)。
`services/calc/internal/httpapi/preset_sync_test.go`(`TestDefenderPresetEnumMatchesEngineCatalog`)を追加。
同パッケージの既存 `vocabulary_test.go`(`wasmapi.Code*` の一覧を手で列挙し、コメントで「新しい code を足したら
ここにも足すこと」と注意喚起する流儀)は**意図的に踏襲しなかった**: 手で列挙した一覧は自分自身の陳腐化
(足し忘れ)を検出できないため、契約(埋め込まれた spec。`contract_test.go` の `loadContract` を再利用)から
`DefenderPreset` の enum を直接読み、`engine.DefenderPresetCatalog()` のキー集合・件数・順序
(契約の description が「耐久が上がる順」と明記。ADR-0009 §1 は8件・順序も規定)と比較する方式にした。
`engine/bulk.go`・`api/openapi.yaml` は無変更(新規 ADR も不要。ADR-0009 §1 が既に決定済みの内容を機械検査で
固定しただけ)。
critic 1回目 FAIL: 件数不一致を `if len(a) == len(b) { 順序比較 }` で黙って skip していたため、集合としては
一致するが列としては崩れている変異(例: `PresetHP` の行を2重にして9件にする。集合は8件のopenapi enumと一致
してしまう)を見逃す穴があった。修正: 件数不一致を明示的な失敗にしてから列を比較するよう変更し、重複変異・
順序入れ替え変異の両方を実際に検知することを確認(確認後 revert)。2回目相当で PASS。
Reason: issue #73。片方だけにプリセットを追加・削除しても通常の生成・ユニットテストでは同期漏れを検出できず、
「APIが受け付けるがengineが解決できない」「engineのプリセットをAPIから指定できない」状態を作り得た。
Impact: `docs/plan.md` の改善要望に issue #73 の行を追加。データレーンへの追加対応は無し(engine は無変更、
実バグではなく回帰テストの欠落だった)。issue #73 はこの PR のマージでクローズしてよい。
