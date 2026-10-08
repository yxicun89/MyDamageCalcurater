## 2026-10-03: iOS のポケモン画像(P8-1c)は ImageCatalog + SpeciesImageView、既定は画像なし(iOS レーン → タイプバランスレーン・全レーン。ADR-0508)
Decision: manifest(ADR-0808)にキーがあれば AsyncImage、無ければ既存のタイプ色エンブレム。`ImageCatalog`(PokeCalcCore。起動時に1回取得・失敗も保持・throw しない)を `CoreServices.images`(既定 NoImageCatalog)に足す。
モックの既定は画像なし(`POKECALC_MOCK_IMAGES=1` で架空 PNG の data URL)。thumb は種族ヘッダー・検索行・タイプバランスのカードに出し、detail は iOS に詳細画面が無いので後続。X-Device-Id は付けない。
Reason: 画像は必須にしない(CLAUDE.md)。画像が無い現状で既存の全テストが通ること(AC-X)を最優先にするため。
Impact: spec-writer が受け入れ条件(ADR-0501 末尾)と単体 30 件・XCUITest 5 件・足場(`ImageCatalog.swift`)を追加済み。実装は後続。api/openapi.yaml・Generated は不変。
