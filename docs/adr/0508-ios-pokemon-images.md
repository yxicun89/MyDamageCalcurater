# ADR-0508: iOS のポケモン画像は「manifest にあれば AsyncImage、無ければタイプ色エンブレム」(P8-1c)

- 状態: 採用(2026-10-03。spec-writer。実装は後続)
- 関連: ADR-0807(gateway の `/images/*`・manifest の形。決定 3〜6)・ADR-0507(機能レジストリ・CoreServices)・ADR-0501(P8-1c の受け入れ条件章)・ADR-0002(公式画像を Git に入れない)・CLAUDE.md「画像は必須にしない」

## 決定

1. **型は PokeCalcCore に置く**(`ImageCatalog.swift`): `ImageManifest`(decode。version 1 以外・不正 JSON・形違いは throw せず `.empty` = 画像なし。未知の欄は無視。
   1エントリの型違いは他のエントリを巻き込まない)、`PokeImageURL`(基点 URL + `/images/` + 相対パス。空・先頭 `/`・`..`・scheme 付き・`//host`・`?` `#` `\`・制御文字は nil)、
   `ImageCatalog` プロトコル(`imageURL(speciesKey:size:) async -> URL?`。throw しない)、`RemoteImageCatalog`(actor。manifest を最初の引き当てで1回だけ取得)、
   取得手段 `ImageManifestFetching`(テストで差し替え。実体 `URLSessionImageManifestFetcher`)、`NoImageCatalog`、`MockImageCatalog`。
   openapi・Generated には載せない(ADR-0807 決定 5)。
2. **再取得の方針は「起動時1回・手動更新なし」**。成功も失敗も保持し、失敗しても再取得しない(画面を遅くしない・通信失敗のたびに待たせない)。
   manifest が変わった(`make assets` をやり直した)ときはアプリを再起動する。ハッシュ付きファイル名なので画像本体のキャッシュは URLSession 任せでよい。
3. **X-Device-Id は付けない**(ADR-0807 決定 4。画像取得に端末 ID は不要。`<img>` が送れないのと同じ契約)。基点 URL は `AppConfiguration` の API の baseURL と同じ。
   ATS の例外は足さない(http は非修飾ホスト名・`.local` のみ。P6-16)。API モードで基点が https/許可ホストなら画像 URL も同じ条件で通る。
4. **注入は `CoreServices` に `images: any ImageCatalog` を1つ足す**。画像は複数の画面(計算・逆算・検索シート・タイプバランス・構築)が共有する「画面ではないもの」なので `FeatureServices` ではなく core。
   `.ready(core:features:)` の形は変えない。`CoreServices` の追加は既定値付き(`NoImageCatalog()`)にして既存の呼び出し(プレビュー・テスト)を壊さない。
   モック接続は `MockImageCatalog(environment:)`(`POKECALC_MOCK_IMAGES=1` で画像あり。**既定は画像なし**)、API 接続は `RemoteImageCatalog(baseURL:, fetcher: URLSessionImageManifestFetcher())`。
   View へは SwiftUI の Environment(`\.imageCatalog`。既定 `NoImageCatalog`)で渡し、`SpeciesEmblemView` の呼び出し側(3か所)に引数を足さない。
5. **表示は `SpeciesImageView`(App 側)**: `speciesKey`・サイズ種別・フォールバックの `SpeciesEmblemView` を受け取る。manifest にキーがあれば `AsyncImage`、
   読み込み中・失敗・キー無しはエンブレムのまま(判定は純粋関数 `SpeciesImageDisplay`)。画像は装飾(名前はテキストが読む)なので label は付けない。
   **フェードイン等のアニメーションは付けない**(`AsyncImage` の transaction/animation を使わない。CLAUDE.md「常時アニメーション無し」)。
   画像は `SpeciesEmblemView` と同じ直径の円に `scaledToFill` で切り抜き、文字サイズ(AX5)で大きくならない固定サイズ。
6. **識別子**: 画像の読み込みに成功したときだけ `speciesImage-<speciesKey>` を持つ要素を出す(エンブレムの identifier は増やさない)。XCUITest は「枠がある/無い」だけを見て画像の中身は検査しない。
   `accessibilityHidden(true)` の要素は XCUITest から見えないため、画像は「label 無しの画像要素」(`.accessibilityElement(children: .ignore)`+identifier)として公開する。
   VoiceOver が空の要素として読む場合は `Image(decorative:)` を使い、identifier は外側のコンテナに付けて隠さない(実装者が実機/シミュレータで確認する)。
7. **どこに出すか**: thumb を (a) 計算・逆算の種族ヘッダー(`SpeciesHeaderMenuLabel`)、(b) 種族検索の行(`MasterSearchRow.species`。よく使う相手の行も同じ)、(c) タイプバランスのメンバーカード(`BalanceMemberCard`)の
   `SpeciesEmblemView` 置き換え先とする(いずれも `SpeciesEmblemView` が使われている3か所)。**detail は後続**: 現状の iOS に1体を大きく見せる詳細画面は無く、足すと画面設計の判断が要る。
   detail の取得 API(`ImageCatalog`・`PokeImageSize.detail`)は先に作ってテストするが、View には出さない。
8. **失敗は画面を壊さない**: manifest の 404・不正・version 違い・通信失敗・危険なパスはすべて画像なし。画像本体の 404・デコード失敗も AsyncImage の failure でエンブレム。
   計算・検索は画像の取得を待たない(manifest 取得はバックグラウンド。絶対ルール 5 と同趣旨)。
9. **テストの画像は架空**: モックは `data:` URL の数十バイトの PNG(コードで生成)。実 Pokémon の画像・公式画像・バイナリ画像ファイルはコミットしない(ADR-0002)。

## P8-1c の受け入れ条件
docs/adr/0501-ios-screen-acceptance.md 末尾「P8-1c の受け入れ条件」を正とする。
