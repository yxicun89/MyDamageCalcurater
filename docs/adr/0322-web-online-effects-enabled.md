# ADR-0322: Web のオンライン MasterSource で持ち物・特性の効果を有効にする(issue #211 の Web 分)

- 状態: 採用(2026-10-03)
- 関連: ADR-0218(公開 Item/Ability の省略可 effect。API 分)、ADR-0304 A-1(effect を持てないので無効化していた)、
  ADR-0313(IndexedDB キャッシュ)、ADR-0208(候補の件数の上限)、ADR-0320(メガストーン)

## 決定

1. **変換**: 公開 API の `effect` は DB の形(PascalCase)。`onlineSource.ts` の `mapItem`/`mapAbility` が engine の形(camelCase)に
   写す(トップレベルのフィールド名だけ。`StatMods`・`DefResistType` の中身のタイプ/ステータス ID は変えず、`DefAbsorbTypes` の
   値だけ入れ子のフィールド名も変える)。省略は `null`。値の検証はしない(サーバーが共通マスタで検証済み。ADR-0218 §2)。
   `web/src/master/exportSnapshot.ts` の逆変換(camelCase → PascalCase)と対になり、E2E フィクスチャはそれを再利用する。
2. **`capabilities.effects` は応答の中身で決める**: 「キーが無い」は「効果なし」と「効果を返さない古いサーバー」を区別できない
   (ADR-0218 §4)。`load()` が受けた持ち物に効果を持つものが**1件でもあれば true**、1件も無ければ false(従来どおり
   「持ち物の候補も比較」を無効にして注記を出す)。実データは効果あり 41/166 件なので、新しいサーバーで false にはならない。
   `ONLINE_MASTER_CAPABILITIES` は「古いサーバー」の基準値(effects false)として残す。
3. **候補の絞り込みと上限は既存のまま**: 防御側の候補は `defensiveItemCandidates`(効果データから選ぶ。効果を持たない持ち物は
   出ない)、通り数は `MAX_ITEM_VARIANTS`/`MAX_ITEM_CANDIDATES`(64)で切り詰め、切り詰めは既存の注記で表示する。
   使用可能な持ち物かどうかはサーバー(`searchItems` の使用可能集合)が決める。Web に持ち物のリストを持たない。
4. **IndexedDB キャッシュ**: スキーマ版は上げない。オンラインは毎回 `load()` で取り直すので古い effect 無しの応答は残らない。
   オフライン(キャッシュ)の capabilities は `effects: false` のままで、候補比較は無効のまま(別 issue。メガ持ち物の WASM 側検証は #505)。
   キャッシュに effect が入っても、オフラインの計算で持ち物・特性の効果が掛かるかは本件の対象外(従来から WASM は個体の effect を使う)。
5. **E2E**: pokedex フィクスチャは例データの effect を公開 API の形(PascalCase)で返す。calc-svc の例マスタ
   (`scripts/export-example-master.mjs`)には、フィクスチャが足す架空のメガストーンの持ち物だけを足す(効果ありの持ち物が
   候補比較で `itemVariants` に入るため、calc-svc がその ID を知らないと 400 になる。メガ種族は共通マスタの検証で
   基本種族・必要な持ち物の対を要求されるので足さない)。

## 却下した案

- **オンラインでは常に `effects: true`**: 効果を返さない古いサーバーで候補が空のまま「比較できる」ように見える。
- **`/api/pokedex` を別に叩いて効果の有無を探る**: 往復が増える。応答の中身で足りる。

## テスト

`web/src/master/onlineSource.test.ts`(変換・capabilities)、`web/e2e/support/pokedexFixture*.test.ts`(契約・往復)、
`web/e2e/online.spec.ts`(オンラインで候補比較をオンにすると行が増える)。
