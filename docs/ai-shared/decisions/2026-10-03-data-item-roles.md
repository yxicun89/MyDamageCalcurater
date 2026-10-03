## 2026-10-03: 持ち物の役割(roles)とメガストーン判定を pokedex が1か所で導いて公開する(データレーン → Web・iOS レーン。ADR-0175)
Decision: `GET /api/pokedex/items` の各 `Item` に `roles`(`attacker`/`defender` の配列。空でも常に出す)と `isMegaStone`(常に出す)を足した。`GET /api/pokedex/species/{key}` の `SpeciesDetail` に `baseSpeciesKey`・`baseSpeciesNameJa`(メガでなければ null。キーは常に出す)を足した。
役割の規則は `services/internal/master.ItemRoles` が唯一の正(補正が 0 でも 4096 でもないときだけ数える。メガストーン・効果なしは空)。`role` の検索条件は足さない(クライアントが `roles` で絞る。オフラインも同じ式)。
Reason: メガストーンの英語名が選択肢に出る・攻撃側に防御用の持ち物を選べる、というユーザー報告(2026-10-03)。規則を各クライアントで再導出すると分かれる。
Impact:
- **Web・iOS レーン**: 持ち物の選択肢 = その側の役割を `roles` に含むもの。`roles` が無い(古いサーバー・キャッシュ)ときは絞らない。固定中の表示は「{baseSpeciesNameJa}のメガストーン」(null なら「メガストーン」)。Web はキャッシュのスキーマ版を上げる。詳細は ADR-0175 §4。
- engine・ゴールデン・read model 6 ファイル・内部 API は不変。
