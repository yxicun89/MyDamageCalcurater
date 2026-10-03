## 2026-10-02: Individual.moveId を契約から削除、sp の欠落は 400(API レーン → Web・iOS レーンへ。issue #245・#316・ADR-0200 §4)

- `Individual.moveId`(攻撃側で使う技)を `api/openapi.yaml` から削除した。calc-svc は一度も読んでおらず、技は `CalcRequest.moveId` / `BulkCalcRequest.moveId` / `ReverseRequest.moveId` のトップレベルで指定する。`web/src` に参照が無いことを確認済み。iOS は同じ PR で追従済み(`APIPokeCalcService` の生成型 Individual から `moveId:` を外し、ドメイン型 `Individual.moveId` は残す)。`attacker.moveId` を送ると 400 `unknown_field`
- `Individual.sp` と StatBlock の6キー(hp/atk/def/spa/spd/spe)の欠落は 400 `invalid_input`(従来は無振りで 200)。クライアントは常に6キーを送ること
- pokedex の searchSpecies・getSpecies・searchMoves・searchItems の契約に `400` を明記した(挙動は不変。生成クライアントの型に 400 が出る)
- `web/src/api/openapi.gen.ts` と iOS 生成物(`make ios-gen`)は再生成済み。Web は追従不要。iOS は同じ PR で追従済み(searchSpecies 等の switch に `.badRequest` を追加)
