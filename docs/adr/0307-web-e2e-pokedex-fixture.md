# ADR-0307: `web-e2e-online` の pokedex フィクスチャ(PR2)

- 状態: 採用(Web レーン、2026-09-25)
- 日付: 2026-09-25
- 関連: ADR-0301 §5 追記(この問題を「未解決」として残した経緯)、ADR-0304(Web のオンライン MasterSource)、
  ADR-0306(ルートの `make e2e` は k3d 無しで3件を常時実行する)、ADR-0002(実マスタを Git に置かない)、
  plan.md「`make e2e` の `web-e2e-online` 修復・PR2」、CLAUDE.md 絶対ルール1・4

## 背景

`make e2e`(ADR-0306)が常時実行する `web-e2e-online` が、PR1(`fix/web-online-e2e-master-export`)で
calc-svc が起動できるようになったあとも緑にならず、`web/e2e/online.spec.ts` の2件がどちらもタイムアウトする。
原因は PR1 の対象(`toCalcSnapshot` の契約追従)とは別の、P4-16(ADR-0304)以降の設計不整合が2つある。

1. **オンラインのマスタの出どころ**。`web/src/main.tsx` はオンラインモードの `MasterSource` を
   `createOnlineMasterSource`(pokedex-svc の公開 API `/api/pokedex/*` を読む)に固定で切り替える。しかし
   `playwright.online.config.ts` は calc-svc だけを起動し、calc-svc は担当外の pokedex の操作を意図的に
   404 で返す(`services/calc/internal/httpapi/server.go` の `registerPokedexNotFoundRoutes`。ADR-0301 §5 追記の R1)。
   このため `onlineSource.load()` が `GET /api/pokedex/items` で reject し、画面は
   「マスタデータの読み込みに失敗しました」のまま止まる。
2. **種族の選び方**。`ONLINE_MASTER_CAPABILITIES.speciesList` は常に `false` なので、オンラインの種族の
   スロットは `<select>` ではなく検索欄(`SpeciesSearchField`。ADR-0304 A-4・A-10)になる。
   `web/e2e/support/calcPage.ts` の `selectMatchup` は `selectOption`(`<select>` 前提)のままで噛み合わない。

どちらも、`make e2e` が長らくスタブだった(issue #72・P4-22 で 2026-09-25 に初めて実際に動き始めた)間に
静かに積み上がっていた既存の不整合で、PR1 の範囲外として持ち越されていた。

## 決定

### 1. E2E 専用の pokedex フィクスチャを Web 側に置く

`web/e2e/support/pokedexFixture.ts`(応答を作る純粋関数)と `web/e2e/support/pokedexFixtureServer.mjs`
(HTTP の待ち受け)を新設し、Web の架空の例データ(`web/src/master/example/`)から pokedex-svc の
公開エンドポイント(`GET /api/pokedex/{items,natures,species,species/{key},moves/batch}`)の応答を返す。

**なぜこの形か**(却下した案):

- **calc-svc に pokedex 相当のルートを持たせる**: ADR-0301 §5 追記の R1(担当外の操作は 404)に反する。
  「サービスは自分のDBにだけ触る」(CLAUDE.md 絶対ルール4)の境界を E2E の都合で崩さない。
- **pokedex-svc を `web-e2e-online` に足す**: MySQL とマスタ投入が要り、「k3d 不要で常時実行する3件」という
  ADR-0306 の設計に反する(本物の pokedex-svc との結合は `web-k3d-e2e` の担当で、そちらは既にある)。
- **オンラインの意味を「例データ + calc-svc」に戻す**: ADR-0304 の設計(オンライン = 公開 API から読む)を
  E2E のために作り変えることになり、本番と違うものを確かめることになる。
- **`page.route()` でブラウザ側からモックする**: 本番ビルド(`vite preview`)が実際に配る経路を通らず、
  プロキシの配線(この不整合の一部)を確かめられない。

フィクスチャは**本番のコードではなく E2E の支え**なので `web/e2e/support/` に置き、`web/src/` には入れない。
アプリのバンドルからは参照されない。

### 2. 応答を作る部分は純粋関数にし、契約テストで固定する

`handlePokedexRequest(master, request) -> {status, body}` は HTTP を知らない純粋関数にする。理由は、
フィクスチャが本物の公開 API の契約(`api/openapi.yaml`)から静かにズレると「E2E は緑なのに本番は壊れている」
という最悪の状態になるため。`web/e2e/support/pokedexFixture.contract.test.ts` が毎回、生成型
(`web/src/api/openapi.gen.ts`。`make gen` が `api/openapi.yaml` から作る)と突き合わせる:

- 型レベル: 期待するキーの一覧を `satisfies readonly (keyof Schemas[...])[]` で宣言し、契約側にキーが増えたら
  `AssertNever<Exclude<...>>` が typecheck を落とす。
- 実行時: 応答の各要素を型ガードに通し、キー集合が**ちょうど**であることを確かめる。例データが持つ
  `Item.effect`・`Ability.effect`・`MasterSpecies.learnset` が契約に無い場所へ漏れるのを捕まえる。

さらに `pokedexFixture.onlineSource.test.ts` が、本番の読み取り側(`createOnlineMasterSource`)を
このフィクスチャにつないで `load` / `searchSpecies` / `resolveSpecies` が成立することを確かめる
(`fetch` だけ差し替え、HTTP は挟まない)。vitest がこの2ファイルを拾えるよう
`vitest.config.ts` の `include` に `e2e/support/**/*.test.ts` を足し、逆に Playwright が拾わないよう
`playwright.config.ts` に `testMatch: ["**/*.spec.ts"]` を明示する。

### 3. E2E 専用ゆえの簡略化の範囲

本物の pokedex-svc + gateway を再現しない。**正常系が契約のスキーマを満たすこと**を最優先にし、次まで作る:

- 端末 ID / セッション ID ヘッダの欠落は 400 `missing_header`、UUID でなければ 400 `invalid_header`
  (UUID の検証は本来 gateway の担当。ADR-0202)。
- `limit` の範囲外・非整数は 400 `invalid_input`、`SpeciesKey` の形式違反は 400 `invalid_input`、
  未知の種族は 404 `not_found`、`ids` の欠落・0件・65件超は 400 `invalid_input`。

作らないもの(E2E で通らないため): レギュレーションによる使用可能集合の絞り込み、`format` クエリ、
`searchMoves` / `getMove` など Web のオンラインが呼ばない操作、405 Method Not Allowed
(GET 以外と未知のパスはまとめて 404 `not_found` にする)、`master_unavailable` / `upstream_unavailable`。

データは架空の例データだけで、実マスタ・生成済みスナップショットは持たない(CLAUDE.md ドメイン規約・ADR-0002)。

### 4. プロキシは `POKEDEX_PROXY_TARGET` でパスごとに振り分ける

`web/vite.config.ts` に `POKEDEX_PROXY_TARGET` を足し、`/api/pokedex` を `/api`(calc)より**前**に置く。
Vite のプロキシは定義順の前方一致で選ぶので、`/api` が先だと pokedex への要求が calc に行ってしまう
(P4-12a の `BALANCE_PROXY_TARGET` と同じ既存パターン。`/api/balance` の扱いは変えない)。
`playwright.online.config.ts` はフィクスチャを3つ目の `webServer` として起動し、`vite preview` に
`POKEDEX_PROXY_TARGET` を渡す。ポートは `e2e/support/serverConfig.ts` の `POKEDEX_FIXTURE_PORT`。

### 5. `online.spec.ts` の検査内容

- 種族は `selectMatchupBySearch`(検索欄に入力 → 候補をクリック)で選ぶ。オフライン用の `selectMatchup`
  (`<select>` 前提)は変更しない(オフラインの35件の無回帰)。
- 「持ち物の候補も比較」はオンラインでは押せない(公開 API に効果データが無く `effects: false`。ADR-0304 A-1)。
  従来この比較テストはこれをオンにしていたが、P4-16 以降は成立しない。**disabled であることを確かめたうえで**
  既定の5行を比較する形に改める(オフライン側では enabled であることも確かめる。検査は増やして減らさない)。
- マスタが `/api/pokedex/*` から 200 で読めていること(= プロキシの振り分けが効いていること)を確かめる
  テストを1件足す。

## 影響

- `make e2e` の `web-e2e-online` が実際に緑になる(完了条件)。`web-e2e`(オフライン35件)・`web-e2e-balance` は無変更。
- `services/`・`api/openapi.yaml`・`engine/` は無変更。API 契約は変えない(`make gen` は不要)。
- フィクスチャは本物の pokedex-svc の代用ではない。本物との結合は `web-k3d-e2e`(k3d 上)が引き続き担当する。
- 契約が変わったときは、生成型の更新で契約テストが落ちてフィクスチャの更新漏れに気づける。
