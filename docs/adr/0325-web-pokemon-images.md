# ADR-0325: Web のポケモン画像表示(P8-1c。manifest があれば <img>、無ければタイプ色エンブレム)

- 状態: 採用(2026-10-03。P8-1c Web 分を実装)
- 関連: ADR-0807(画像は gateway の `/images/*` で配信・manifest 契約)、ADR-0310(CSP)、ADR-0313(オフライン)、CLAUDE.md「画像は必須にしない」
- 番号: 0322 は ADR-0322-web-online-effects-enabled が使用済みのため 0325 とした。

## 決定(案)
1. **契約の読み方**: `GET /images/manifest.json`(同一オリジン)を、アプリ起動時に1回だけ fetch する。`{version:1, images:{<種族キー>:{thumb,detail}}}`。
   画像 URL は `/images/` + 相対パス。種族キーは Web の `Species.key`(`{図鑑番号4桁}-{フォルム3桁}`)と同じ。
2. **純粋関数** `web/src/images/pokemonImages.ts`: `parsePokemonImageManifest`(version 1・images が object・各 thumb/detail が string で
   `.webp` の安全な相対パスだけ採用。`..`・絶対パス・スキーム付き・`//`・バックスラッシュ・クエリ・空は**そのエントリを不採用**)、
   `pokemonImageUrl(manifest, key, size)`、`fetchPokemonImageManifest(fetch)`(失敗は例外にせず null)。プロトタイプのキー(`constructor` 等)は引かない。
3. **失敗はすべて「画像なし」**: 404・500・不正 JSON・HTML・version 違い・ネットワーク失敗・キー無し・画像の読み込み失敗(`onError`)は、
   エラー表示・alert・例外にせず既存のタイプ色エンブレムにする。Provider が無いときもエンブレム(既存画面・既存テストは無改修で通る)。
4. **部品**: `PokemonImagesProvider`(`manifest` を直接渡す口=テスト用/`fetch` を渡すと1回だけ取得する口。StrictMode でも1回)と
   `PokemonImage({speciesKey,size,fallback,className?})`(`<img loading="lazy" alt="" width height>`。装飾画像: 名前は隣のテキストで読める)。
   `fallback` に既存のエンブレム要素を渡す。使う画面は CalcScreen(攻撃側・防御側カード)と SpeedScreen(各行)。
   **判定画面(`web/src/judge/`)は判定レーンの持ち物なので編集しない**。判定がエンブレムの共通部品を使う形になった時点で、部品側の変更だけで追従できる
   (今回の調査では judge は `type-emblem` を使っていない)。ReverseScreen・BalanceScreen・TeamScreen にもエンブレムは無く、今回は対象外。
5. **thumb は一覧・カード、detail は詳細**。今の Web に「詳細」画面は無いので detail は URL 組み立てと将来用(使う画面ができたら足す)。
6. **CSP は変更不要**: 画像は同一オリジン `/images/*` なので `img-src 'self' data:` に収まる。`connect-src 'self'` も manifest の fetch に足りる。
7. **コンソールエラー**: nginx(コンテナ)・vite preview は `/images/*` に SPA フォールバック(index.html の 200)を返すため、manifest として読めず(JSON でない)
   エンブレムのままで、ブラウザのコンソールエラーも出ない(container.spec.ts の CSP テストは無改修で通る)。gateway が JSON の 404 を返す環境
   (k3d で画像なし)では、ブラウザが「Failed to load resource」を出しうる。これはアプリでは消せないので許容し、アプリ起因の例外・alert は出さない。
   nginx に `/images` の 404 を足さない(足すと CSP テストが赤くなる)。
8. **開発サーバー**: `vite dev` で画像を見るには `/images` を gateway へ転送する必要がある。`IMAGES_PROXY_TARGET`(Node 側の値。`VITE_` を付けない)を
   `vite.config.ts` に足し、設定したときだけ `/images` を転送する(既存の `POKEDEX_PROXY_TARGET` と同じ形)。未設定なら従来どおり(preview は SPA フォールバック)。
9. **オフライン(ADR-0313)**: 画像はオンラインの gateway 配信で、IndexedDB にキャッシュしない。オフラインでも同一オリジンの manifest を1回試し、
   取れなければ静かにエンブレム(計算モードによらず同じ。計算は画像に依存しない)。HTTP キャッシュ(ハッシュ付き immutable)に任せる。
10. **動き・a11y**: 常時動くアニメーションを入れない(フェードイン等もしない。prefers-reduced-motion の分岐は不要)。画像は装飾(alt 空)。
    CLS 対策として幅・高さを固定する(CSS はデザイントークンのみ)。画像の読み込み失敗でレイアウトがずれないよう、エンブレムと同じ寸法の枠に収める。

## テスト
`src/images/pokemonImages.test.ts`・`PokemonImage.test.tsx`・`src/screens/CalcScreen.images.test.tsx`・`src/speed/SpeedScreen.images.test.tsx`・
`src/App.images.test.tsx`・`e2e/images.spec.ts`。

## 実装時の追記(P8-1c)
- 定数名は `POKEMON_IMAGES_BASE_PATH`・`POKEMON_IMAGES_MANIFEST_PATH`。manifest は検証後 `Map`(プロトタイプのキーを引かない)で持つ。
- `PokemonImagesProvider` は `manifest` を渡すと取得しない。`fetch` を渡すと fetch 実装ごとの取得 Promise をモジュールで共有し、StrictMode でも1回。アンマウント後は state を更新しない。
- App は `imageFetch`(既定 `globalThis.fetch`)を Provider に渡す。
- `<img>` の寸法は属性の既定値(thumb 32 / detail 128)に加え、各画面の CSS クラス(エンブレムと同じ寸法)が決める。
- 起動時の `/images/manifest.json` の fetch が増えるため、「engine.wasm・API を読まない」ことを確かめる既存テスト(App.test.tsx 2件・App.routing.test.tsx 2件)は、`/images/` への fetch だけ数えないよう最小限直した(確かめる対象は変えていない)。
  e2e/images.spec.ts の HTML フォールバックのテストは、暖機の `/api/calc` の 404 コンソールログを数えないよう除外した。
