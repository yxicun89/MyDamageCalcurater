## 2026-10-02: 公開 API の Item / Ability に省略可の effect を足した(API レーン → Web・iOS レーンへ。issue #211・ADR-0218)

- `searchItems` の各持ち物と `getSpecies.abilities` の各特性は、効果を持てば `effect`(`MasterEffect`)を伴う。効果を持たなければ**キーごと省く**(null にしない)。「効果を持つ持ち物だけ」の検索条件は無い(`effect` の有無で絞る)
- `effect` の JSON は DB の形(`DamageMod` などの PascalCase。ADR-0005)で、Web の engine 型(`damageMod` などの camelCase)とは違う。Web は `onlineSource.ts` の `mapItem` / `mapAbility` で変換が要る
- 返す前に共通マスタ(`DecodeItemEffect` / `DecodeAbilityEffect`)で厳格に検証し、応答に載る行に不正があれば 503 `master_unavailable`。Web の E2E フィクスチャ(`pokedexFixture.ts`)は Web レーンが effect を返すようにし、`pokedexFixture.contract.test.ts` の `not.toHaveProperty("effect")` を反転させる
- 「キーが無い」は「効果なし」と「effect を返さない古いサーバー」を区別しない。Web の IndexedDB マスタキャッシュ(#210)には effect の無い古い応答が残る可能性があるので、Web レーンが版の切り替えを扱う。ONLINE_MASTER_CAPABILITIES.effects を true にする判断も Web レーン
- iOS の生成物は API レーンで再生成済み(`make ios-gen-check` 緑)。Web の `openapi.gen.ts` も再生成済み
