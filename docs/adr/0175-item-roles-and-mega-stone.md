# ADR-0175: 持ち物の役割(攻撃側/防御側)とメガストーンの判定を pokedex で1か所だけ導いて公開する

- 状態: 採用
- 日付: 2026-10-03
- レーン: データ(pokedex・API 契約)。Web・iOS の絞り込みと表示は各レーン
- 関連: ADR-0002 決定 10、ADR-0105 §3(検索 API・使用可能集合)、ADR-0120(効果データの網羅)、ADR-0123(未対応の印)、
  ADR-0128(read model の dataVersion)、ADR-0218(公開 API の effect)、ADR-0313(Web のオフラインキャッシュ)、
  ADR-0320(Web のメガ持ち物固定)、ADR-0324 §2(メガストーンの日本語名は生成しない)、docs/mega-evolution-spec.md §4-3

## 背景(2026-10-03 のユーザー報告)

1. メガストーンが英語表記で選択肢に出る(上流に日本語名の無い新しいメガストーン 40 件。ADR-0324 §2 で名前は生成しない)。
2. 攻撃側にオボンのみ、ルカリオにボーマンダナイトのような、その側で意味の無い持ち物を選べる。
3. 持ち物はすべて日本語で見せたい。

メガストーンを持ち物の選択肢から外し(メガ種族を選ぶと自動で固定。ADR-0320)、固定中の表示を
「<基本種名>のメガストーン」という日本語の文言にすれば、英語のストーン名は画面に出ない(名前を推測しない)。
その側で意味のある持ち物だけを選べるようにするには、持ち物ごとの「役割」が要る。

Web はいま `megaStoneItemIds`(`web/src/domain/mega.ts`)でストーンを自前に導いている。役割まで各クライアント
(Web のオンライン・オフライン、iOS)が効果データから再導出すると、規則が分かれる。

## 決定

### 1. 役割の導出規則(1つの純粋関数)

`services/internal/master.ItemRoles(effect *engine.ItemEffect, isMegaStone bool) []ItemRole` を唯一の正とする。
値は `attacker`(攻撃側で持つとダメージが変わる)と `defender`(防御側で持つとダメージが変わる)。

| ItemEffect の項目 | 役割 | engine の根拠(その側の持ち物だけを読む箇所) |
|---|---|---|
| `StatMods[atk]`・`StatMods[spa]` | attacker | `offensiveStatMod`(`in.Attacker.Item`)。物理は atk、特殊は spa(`damage.go` の atkKey) |
| `StatMods[def]`・`StatMods[spd]` | defender | `defensiveStatMod`(`in.Defender.Item`)。物理は def、特殊は spd(defKey) |
| `StatMods[hp]`・`StatMods[spe]` | なし | ダメージ計算で読まない |
| `DamageMod` | attacker | `otherModifiers`(`in.Attacker.Item`)。`OnlySuperEffective` は条件だけ |
| `PowerMod` | attacker | `powerModifier`(`in.Attacker.Item`)。`PowerCategory` は条件だけ |
| `BoostType` + `BoostTypeMod` | attacker | `powerModifier`(`in.Attacker.Item`。`BoostTypeMod != 0` のときだけ掛かる) |
| `ResistBerryType` | defender | `otherModifiers`(`in.Defender.Item`) |
| `UnsupportedAttacker` | attacker | `unsupportedMarks` の `attacker_item`(ADR-0123) |
| `UnsupportedDefender` | defender | `unsupportedMarks` の `defender_item`(ADR-0123) |
| `OnlySuperEffective`・`PowerCategory` だけ | なし | 修飾子。単独では補正を生まない |

- 補正値は「中立でない」ときだけ数える: `StatMods` の値・`DamageMod`・`PowerMod`・`BoostTypeMod` が 0 でも 4096(×1.0)でもないこと。
  4096 は掛けてもダメージが変わらず、0 は engine が「補正なし」と読む値。`BoostType` は `BoostTypeMod` が中立でないときだけ数える
  (engine は `BoostTypeMod != 0` のときだけ掛ける)。
- 両方に当てはまるものは両方(並びは常に `attacker` → `defender`)。
- 効果データが無い(`effect == nil`)持ち物は空配列(どちらの役割も持たない = 計算の持ち物の選択肢に出ない)。
- **メガストーンは常に空配列**(効果データがあっても)。メガストーンは種族の選択で固定される持ち物で、単独で選ぶものではない(§2)。
- 戻り値は nil ではなく空配列(JSON で `[]`)。
- 計算の意味との整合: 規則の各行が「その側に持たせると、持たせないときと比べて Rolls が変わるか未対応の印が付く」と一致することを、
  engine を実際に呼ぶテスト(`services/internal/master/item_role_test.go` の `TestItemRolesMatchEngineDamage`)で確かめる。
  計算・ゴールデンは変えない(engine は不変)。

役割はダメージ計算の役割に限る。素早さ(`StatMods[spe]`)など他の画面の役割は、必要になったら別の値として足す(本 ADR の対象外)。

### 2. メガストーンの判定

持ち物 I がメガストーン ⇔ `species` のいずれかの行が `is_mega = 1 AND required_item_id = I`。
名前の表を持たず、マスタから導く。**使用可能集合(レギュレーション)で絞らない**(使えないメガのストーンも、単独で選ぶ持ち物ではない)。
pokedex の `SearchItems` の SQL(`EXISTS` の列 `is_mega_stone`)が唯一の判定の場所。

> 追記(2026-10-04・ADR-0140・issue #607): 判定を「`items.is_mega_stone`(取得元 Showdown の持ち物データ `megaStone` と
> メガ種族の要求から importer が導く。migration 000012)が真 **または** 上の EXISTS」に改めた。唯一の判定の場所は引き続き `SearchItems` の SQL。

### 3. 公開 API(`api/openapi.yaml`)

- `Item` に省略可の2項目を足す(`required` は `[id, nameJa]` のまま。破壊的変更にしない)。
  - `roles: ItemRole[]`(`ItemRole` = `attacker` | `defender`。Go の定数名は `ItemRoleAttacker`/`ItemRoleDefender`)。pokedex-svc は常に返す(空配列可)。
  - `isMegaStone: boolean`。pokedex-svc は常に返す。
- `searchItems` にクエリ `role` は**足さない**。理由: (1) 役割は効果 JSON から Go で導くので、SQL の `LIMIT` の前に絞れない
  (列を持たせると migration・importer・dataVersion の変更が要る)。(2) 持ち物は使用可能集合で 166 件で、クライアントは1回で全件取る(ADR-0304 §1)。
  (3) オフライン(キャッシュ)では項目で絞るしかないので、項目だけにすればオンラインとオフラインの絞り込みが同じ1つの式になる。
- `SpeciesDetail` に省略可の2項目を足す。pokedex-svc は常にキーを返す(メガでなければ null。`requiredItemId` と同じ扱い)。
  - `baseSpeciesKey: string | null`(メガシンカ前の種族キー)。
  - `baseSpeciesNameJa: string | null`(メガシンカ前の種族の日本語名)。固定中の表示「<基本種名>のメガストーン」を、
    基本種を別に引かずに(オフラインのキャッシュに基本種が無くても)組み立てるため。
- 調査結果: 実データで `GET /api/pokedex/species/0448-001` の `baseSpeciesKey` が null に見えたのは、マスタの欠落ではなく、
  `SpeciesDetail` に項目が無く pokedex-svc が出していなかったため(DB の `species.base_species_key` には入っている。内部 API の
  `MasterSpecies.baseSpeciesKey` は出ている)。

### 4. クライアント(Web・iOS)の使い方(各レーンが実装する)

- 持ち物の選択肢 = `roles` にその側の役割を含む持ち物。ストーンは `roles` が空なので、この1つの式で外れる(`isMegaStone` を別に見なくてよい)。
- `isMegaStone` は表示・固定の補助(固定中の持ち物の判別、`megaStoneItemIds` の置き換え)。
- メガ種族を選んだときの固定中の表示は、ストーンの `nameJa` ではなく文言資源の「{基本種名}のメガストーン」に `baseSpeciesNameJa` を入れる。
  `baseSpeciesNameJa` が null(古いサーバー・不整合)のときは名前を推測せず「メガストーン」だけを出す。
- 固定に使う持ち物の検索は、絞り込む前の全件から引く(ストーンは選択肢に出ないが、固定と計算の要求には要る)。
- `roles` が無い(古いサーバー・古いキャッシュ・例データ)ときは役割で絞らない(従来の一覧のまま)。クライアントで効果から再導出しない。
  Web はキャッシュのスキーマ版(`MASTER_CACHE_SCHEMA_VERSION`)を上げて、`roles` の無い古いキャッシュを捨てる。
- 逆算・判定の持ち物候補も、その側の `roles` で絞る。

### 5. read model・内部 API(変えない)

- `pokedex export`(balance・speed の read model 6 ファイル)には持ち物が無く、balance・speed は持ち物を使わない。持ち物の欄は足さない。
  6 ファイルの形・dataVersion・checksum は変わらない(loader の `DisallowUnknownFields`・schema の `additionalProperties: false` に影響しない)。
- 内部 API `getMasterExport` の `MasterItem` にも足さない(calc-svc は役割を使わず、loader は未知のフィールドを拒否する)。
- オフラインの再現は、Web の IndexedDB キャッシュが `searchItems` の応答(`roles`・`isMegaStone` を含む)をそのまま保存することで行う。

### 6. 異常系

- 使用可能集合: `searchItems` は従来どおり既定のレギュレーションの持ち物だけを返す。`roles`・`isMegaStone` はレギュレーションに依らない。
- 役割が空の持ち物(効果なし・ストーン)も `searchItems` には返す(図鑑としての一覧・固定に使う)。選択肢から外すのはクライアント。
- 効果 JSON が検証を通らない持ち物は、従来どおり 503 `master_unavailable`(役割を導く前に失敗する)。

## 検証

- 規則: `services/internal/master/item_role_test.go`(全項目 × 攻撃/防御のテーブル駆動・中立値・修飾子だけ・両方・効果なし・ストーン、
  および engine を実際に呼んで規則と計算が一致することの確認)。
- 契約: `services/pokedex/internal/httpapi/item_roles_test.go`(`searchItems` の `roles`・`isMegaStone` が常にキーを持つ・値・
  使用可能集合の外のメガのストーン・契約の enum と `master.AllItemRoles` の一致・新しい項目が `required` に無い(互換)・
  `getSpecies` の `baseSpeciesKey`・`baseSpeciesNameJa`・内部 API の `MasterItem` に項目が無い)。
- read model: `services/pokedex/internal/readmodel/readmodel_item_roles_test.go`(6 ファイルに持ち物の役割が出ない)。
- 実 MySQL: `services/pokedex/db/item_mega_stone_mysql_test.go`(`SearchItems` の `is_mega_stone`)。

## 影響

- `api/openapi.yaml` に省略可の項目を足すだけ(既存クライアントは無視でき、古いサーバーの応答も新しい契約に合う)。
- `services/pokedex/db/query/pokedex.sql` の `SearchItems` に列が1つ増える(sqlc の生成物が変わる)。migration は無い。
- engine・ゴールデン・計算 API は変わらない。
