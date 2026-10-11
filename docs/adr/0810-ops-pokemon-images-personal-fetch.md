# ADR-0810: ポケモン画像を個人利用で取得する道具を足す(ADR-0808 の「取得ツールは作らない」を改める)

- 状態: 採用(2026-10-11。ユーザー決定。usability-round3 G-06 のデータ側)
- 関連: ADR-0808(変換・配信・manifest)・ADR-0002(画像を Git に入れない)・ADR-0101(importer)・ADR-0325(Web の表示)・ADR-0508(iOS の表示)

## 背景
ADR-0808 は権利を理由に取得ツールを作らず、手元に置いた画像を変換する前提だった。利用者は個人利用のみ(商用公開の予定はない)で、
ポケモンの画像・アイコンを使うと決めた。ただし画像そのものは Git に入れない(ADR-0002・ADR-0808 の入出力先 `data/generated/` は維持)。

## 決定
1. **取得ツールを作る**。`tools/assets/fetch.mjs`(Node・追加依存なし。`make assets-fetch`)。出力は `data/generated/images/src/{key}.png`。以降は ADR-0808 のまま(`make assets` → `dist/` → gateway `/images/*`)。
2. **入手元は GitHub raw の `smogon/sprites`(主)+ Pokémon Showdown 本体の sprites(補助)**。ユーザー要望は「GitHub 上のスプライト集の raw を優先」(大量アクセスを避ける)。
   - 主: `https://raw.githubusercontent.com/smogon/sprites/master/src/champions/s{名前}[-o{フォーム}].png`、無ければ `src/dex/`(Showdown の sprites の元の画像リポジトリ。128px 透過 PNG)。
     ファイル名は `s` + 名前(英数字以外は語の間なら `_`、端なら消す。`Mr. Mime` → `smr_mime`)+ フォームは `-o` + 同じ規則(`Mega-X` → `-omega_x`)。`-s` 付きは色違いなので使わない。
   - 補助: `play.pokemonshowdown.com/sprites/home/{基本種id}-{フォームid}.png`(192px)→ `gen5/`。
   - 理由: 名前もフォームも取り込み済みの Showdown スナップショットから機械的に組める。Champions の新しいメガ(Showdown 本体の home に無かった `raichu-megay`・`falinks-mega` を含む)が `src/champions` に載っている。
   - 実測(2026-10-11、少数): `champions/scharizard-omega_x.png`・`sfloette-omega.png`・`smeganium-omega.png`・`sraichu-omega_y.png`・`sfalinks-omega.png` が 200。Champions に無い種族(例 ほうおう)は `dex/` が 200。
     Showdown 本体だけの実測は 349 種族の 1/4 で home 78・gen5 のみ 8・無し 2 / 88 件。
   - PokeAPI(official-artwork 475px)は画質が良いが、pokemon id(フォームは 10000 番台)への対応表を別に作る必要があるため採用しない。必要になれば入手元に足せる。
3. **key と URL はデータから**: key は pokedex importer と同じ規則(図鑑番号 + 基本種の `formeOrder` の位置)で Showdown スナップショットから作り、取得対象は取り込み済みマスタ(`data/generated/readmodel/pokemon-types.json` の `pokemonId`)に限る。ポケモン名はコードに書かない。
4. **作法**: User-Agent を名乗る・直列(同時 1 本。並列にしない)・各リクエストの後に 500ms 待つ・429/5xx は間隔を広げて 2 回まで再試行・PNG の先頭バイトを検査。既存の `{key}.png` は取り直さない(冪等)。`--limit N`(未取得のうち N 件)・`--dry-run`(計画だけ)。
   失敗した key は理由付きで一覧に出すが終了コードは 0(画像なしでも動く。ADR-0808)。マスタ未取り込みなど入力が無いときだけ非 0。
5. **k3d への載せ方は ADR-0808 追記のまま**(`make assets-fetch` → `make assets` → `make images-k3d`)。クラウド overlay には触らない。
6. **robots.txt と利用規約の確認(2026-10-11)**: `play.pokemonshowdown.com/robots.txt` は 200 で中身が空(Disallow なし)。sprites は Cloudflare 経由の静的ファイル(`cache-control: max-age=691200`)。
   sprites を載せる Smogon の `smogon/sprites` リポジトリ(GitHub)にライセンスの記載は無く、画像の権利は Game Freak / Nintendo / The Pokémon Company にある。サイトに sprites 専用の利用規約は見つからなかったため、
   個人の手元利用に留め、再配布しない・負荷をかけない(直列・待ち・既存スキップ・全件でも約 350 key×最大 4 URL。再実行は 0 リクエスト)運用とする。`raw.githubusercontent.com` の robots.txt は無い(404)。
   `raw.githubusercontent.com` は raw の静的配信で、`smogon/sprites` は GitHub 上に画像の実体を持つ(配布物は `src/` の PNG。リポジトリにライセンスの記載は無い)。
   GitHub の API は使わない(raw の URL を直接引く)。
7. 利用条件: 画像は Game Freak / Nintendo / The Pokémon Company の著作物。個人の手元での利用に限り、再配布・公開をしない(Git に入れない・クラウドへ載せない)。公開する日が来たら画像は別の入手元か自作に差し替える(manifest が無ければエンブレムに戻る)。

## テスト
`tools/assets/fetch.test.mjs`(架空データと偽の fetch。ネットワークに出ない): URL・key の組み立て、冪等、失敗の報告、フォールバック、`--limit`/dry-run、同時実行数と待ち。`make test-tools` に登録。
