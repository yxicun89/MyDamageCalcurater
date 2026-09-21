# ADR-0101: importer の取得・変換・投入(P2-2b)

- 状態: 提案(P2-2b の仕様。spec-writer 起草、implementer が実装、critic がレビュー)
- 日付: 2026-09-21
- 関連: plan.md P2-2b(P2-2c 照合と差分報告・P2-2d CronJob との境界は §10)、ADR-0002(確定した方針・追記 P2-1b・追記 P2-1c)、
  ADR-0005(データ駆動の効果定義)、ADR-0013(相性表はデータ)、ADR-0015(スキーマ・写像)、ADR-0012 / ADR-0015 §8(balance の read model は P2-3)、
  DECISIONS.md 2026-09-21(フォームの登録単位・CronJob 週1回・技の使用可否の既定案)、CLAUDE.md 絶対ルール 2/4/6
- 番号: データレーンの帯(0100〜。COORDINATION.md)の2本目。当初は 0017 だったが、main・他レーンと衝突したため 2026-09-22 に振り直した

## 背景

ADR-0015 で pokedex のスキーマと DB 行 → engine 型の写像ができた。P2-2b では、取得元(@smogon/calc 0.12.0 の Champions、
Showdown の champions mod、PokeAPI)から実データを手元に取り、ADR-0015 の行に変換し、pokedex の DB に冪等に投入する。
実データは Git に入れない(ADR-0002)ので、**取得・変換・投入の仕組みとその検証**だけをリポジトリに置き、検証は架空データで行う。

## 決定

### 1. 構成(取得 → スナップショット → 変換 → 投入)

```
[取得: ネットワーク]              [変換: 純粋]                 [投入: DB]
tools/importer (Node)   ──▶ data/generated/<source>/<version>/snapshot.json
                              + data/importer/*.json(Git 管理)
                              + data/local/name_ja_overrides.json(Git 管理外・任意)
                                        │ LoadInput
                                        ▼
                         services/pokedex/importer.Convert ──▶ Output(ADR-0015 の行)+ Report
                                                                      │ Run / Apply
                                                                      ▼
                                                               pokedex DB(1 トランザクション)
```

| パス | 内容 | Git |
|---|---|---|
| `tools/importer/` | **取得(ネットワークに触る唯一の場所)**。Node の抽出スクリプト。`package.json` は `"@smogon/calc": "0.12.0"`(完全固定。`package-lock.json` も追跡。check-publishable の E) | 追跡 |
| `services/pokedex/importer/` | package `importer`。**純粋な読み込み・変換**(`LoadInput` / `Decode*` / `Convert` / `NeedsImport` / `Checksum`)と**投入**(`Apply` / `AppliedVersions` / `Run`) | 追跡 |
| `services/pokedex/cmd/import/` | 投入の CLI(§11) | 追跡 |
| `data/importer/config.json` | 取得元の版の固定(`sources`)、除外する calc の内部フォーム・タイプ、日本語名の言語の優先順 | 追跡(版の metadata と英語識別子だけ) |
| `data/importer/effects.json` | 効果定義(§2) | 追跡 |
| `data/importer/regulations.json` | レギュレーション定義(§7) | 追跡 |
| `data/importer/examples/name_ja_overrides.example.json` | override の形式の例(架空データ) | 追跡 |
| `data/local/name_ja_overrides.json` | 日本語名の override(§6。**実データなので Git に入れない**。`.gitignore` に `data/local/` を足す) | 管理外 |
| `data/generated/<source>/<version>/snapshot.json` / `meta.json` | 正規化スナップショット(§3)と取得の記録 | 管理外(既存の .gitignore) |
| `data/generated/reports/` | 変換の報告(§8) | 管理外 |

- **Go の importer を `services/pokedex/` に置く理由**: (1) 変換結果を `services/internal/master` の写像に通したい(engine の型まで行けることを投入前に保証する)が、
  `internal` は `services` module の外(`tools` module)から import できない。(2) 書き込み先は pokedex の DB だけで、その書き手を pokedex の配下に置けば
  絶対ルール4の境界がディレクトリで見える。(3) migrations(`services/pokedex/db`)を同じ module から使える。
  `tools/importer/` はネットワーク取得(Node)に専念する。CLAUDE.md のリポジトリ構成の `tools/importer/ # マスタ取込(k8s CronJob)` は
  「取得」と読み替える(CLAUDE.md の更新はメインが判断する)。CronJob のイメージは両方を含める(P2-2d)。
- **ネットワークと純粋な変換を分ける**: Go の importer は実行時にネットワークへ出ない。`make test` は架空データの変換だけを検証する。
- engine は変更しない(絶対ルール2)。

### 2. 効果定義(item_effects / ability_effects)の出どころ

- calc も Showdown も PokeAPI も 4096 基準の係数を構造化データとして持たない(ADR-0002 §8)。
  **リポジトリで人が管理する `data/importer/effects.json` を本番の効果定義の正**とする。
- 形式(厳格。未知のフィールドは拒否):

```json
{"schemaVersion": 1,
 "items":     {"<持ち物ID>": <効果オブジェクト>},
 "abilities": {"<特性ID>":   <効果オブジェクト>}}
```

  - キーは技・持ち物・特性の ID 形式(`^[a-z0-9]+$`。calc / Showdown の `toID`)。値は ADR-0015 §6 と同じ形(engine のフィールド名。例 `{"DamageMod":5324}`)で、
    `master.DecodeItemEffect` / `DecodeAbilityEffect` の厳格な検証を**変換時に**、取り込むタイプ相性表で通す(小数・負・未知フィールド・空・表に無いタイプは失敗)。
    投入する値は `master.Encode*Effect` の正準形。
  - **中身は英語 ID と 4096 基準の整数だけ**。名前・説明文・日本語・画像などの第三者の表現は入れない。
  - 取り込む持ち物・特性に無い ID の定義は投入せず警告(`effect-unused`)にする。将来のレギュレーションで復活する持ち物の定義を残せる(ADR-0002 追記 P2-1b §2)。
  - 網羅性の検査(補正を持つべき持ち物に定義があるか)は P2-2c。
- **ADR-0002 の「実マスタを Git に入れない」との関係**: この定義ファイルは「どの持ち物が使えるか」「名前」を持たず、
  engine が実装しているゲームの仕組みの係数(4096 基準の整数)を英語 ID に対応づけるだけで、`testdata/golden/effects.json`(既にコミット済み。
  engine のテストに必須)と同じ種類のデータである。ADR-0002「追加の回答」の基準(数値と英語識別子だけで第三者の文章・画像・日本語名を含まない)に当てはまるので
  **コミットする**。ただし全持ち物の一覧にはしない(補正を持つものだけ。補正なしは「行が無い」で表す)。判断を変える場合は ADR-0002 と本 ADR を更新する(人間の確認事項 1)。
- 初期内容: implementer は `testdata/golden/effects.json` の定義を ID(`toID`)に直したものから始めてよい(engine が照合済みの係数)。
  Champions で使えない持ち物の定義は `effect-unused` の警告になるだけで害は無い。

### 3. 正規化スナップショット(取得元ごと)

- 置き場所: `data/generated/<source>/<version>/snapshot.json`。`<version>` は `config.json` の `sources` の値と一致し、`^[A-Za-z0-9._-]+$`(パスの外に出ない)。
  スナップショットの `version` フィールドも同じ値でなければ読み込みを拒否する。
- `snapshot.json` は**取得元のデータだけ**を持ち、取得日時などの揺れる値を入れない(同じデータなら同じバイト列 → 同じ sha256 → 週1回の再取得で差分が無ければ取り込まない)。
  取得日時・上流の URL・ダウンロードした tarball の sha256 などは同じディレクトリの `meta.json` に書く(importer は読まない)。
- 抽出は**取得元の表現をなるべくそのまま**写す(型名 `"Fire"`・分類 `"Special"`・名前 `"Test Flame"`)。ID 化(`toID`)・小文字化・規則の適用は Go 側で行う(テストできる側に寄せる)。
  例外は下記の注記のとおり(Showdown の継承の解決・`accuracy: true`)。
- すべて `schemaVersion: 1`。デコードは未知のフィールド・`schemaVersion` / `source` の食い違い・後続データを拒否する(`ErrInvalidInput`)。

| source | version | 中身(JSON のキー) | 抽出の注記 |
|---|---|---|---|
| `calc` | npm の版(`0.12.0`) | `generation`、`types`(型名。順序 = sort_order の元)、`typeChart`(型名→型名→×2 コード。無い組は等倍)、`species[]{name,types,baseStats{hp,atk,def,spa,spd,spe}}`、`moves[]{name,type?,category?,basePower,priority}`(**type / category は calc が省略するとき省略**)、`items[]`(名前)、`abilities[]`(名前) | `Generations.get(0)`。抽出スクリプトは `node_modules` の版が `config.json` の版と一致しなければ止まる(tools/golden と同じ)。calc の種族は特性を slot 0 しか持たないので出さない |
| `showdown` | GitHub の commit(40 桁) | `mod`、`species[]{id,name,num,baseSpecies,forme,baseForme,types,baseStats,abilities{"0","1","H"},requiredItem,formeOrder,isNonstandard}`、`moves[]{id,name,type,category,basePower,accuracy,pp,priority,isNonstandard}`、`items[]{id,name,isNonstandard}`、`abilities[]{id,name,isNonstandard}`、`learnsets{種族ID:[技ID]}` | Showdown の `Dex.mod(<mod>)` を**実行して**継承(`inherit`)を解決した実効値を出す(読むだけでは誤る。ADR-0002 却下案)。`accuracy: true`(必中)は `0` にする。learnsets は `isNonstandard` が null の種族の分だけ |
| `pokeapi` | PokeAPI リポジトリ(`PokeAPI/pokeapi`)の commit | `species[]`(pokemon-species)、`forms[]`(既定でないフォーム。pokemon-form の `pokemon_name`)、`moves[]`、`items[]`、`abilities[]`、`types[]`。いずれも `{slug, names{<PokeAPI の言語 identifier>: 名前}}`。言語は `ja-Hrkt` と `ja` だけ出す | commit を固定した `data/v2/csv/*.csv`(約 14 ファイル)を raw で取得する。REST を約 1,200 回叩かず、版が commit で固定でき、公平利用(キャッシュ・頻度)に沿う |

- Showdown の取得方法(implementer が選ぶ。条件は「版の正は `config.json` の commit だけ」「Showdown のコードを実行して実効値を出す」):
  既定案は codeload の tarball を `data/generated/.cache/` に落とし、sha256 を `meta.json` に記録し、Showdown のビルド(`node build`)後に `Dex` を使う。
- ネットワークの礼儀: 逐次・リクエスト間に待ち時間、`data/generated/.cache/<source>/<version>/` に生の取得物をキャッシュし、同じ版は再取得しない。

### 4. 技の変換(ADR-0002 追記 P2-1c の規則を実装する)

ID は `toID(名前)`(小文字英数字以外を落とす)。calc の技は `type` が無いものを「断片」とし、**calc に無いもの**として扱う。

| calc | Showdown(実効の `isNonstandard`) | 結果 | 報告 |
|---|---|---|---|
| あり | null | 取り込む | 下の値の照合 |
| あり | 非 null | 除外(規則1) | 警告 `move-excluded` |
| あり | 無い(番兵 `(No Move)` を含む) | 除外(規則1) | 警告 `move-excluded` |
| 無い・断片 | null | Showdown の値で取り込む(規則2) | 警告 `move-showdown-only` |
| 断片 | 非 null・無い | 除外(規則1) | 警告 `move-excluded` |

両方にある技の値:
- **タイプ**: Showdown を採る(規則3)。calc と違えば `move-type-mismatch`。**変化技なら警告、攻撃技なら Blocker**(取り込みを止めて人間の裁定を待つ)。
- **分類・威力**: calc の `category` 省略は Status(規則4)。calc と Showdown で違えば `move-value-mismatch` の **Blocker**(ダメージに効く。P2-1c では 0 件)。値は calc を採る。
- **命中・PP**: Showdown(calc は持たない)。**優先度**: Showdown。calc と違えば警告 `move-value-mismatch`(ダメージに効かない)。
- 変化技の威力は 0。`name_en` は Showdown の名前。
- 規則5(覚えるポケモンが 0 の技)は既定案どおりマスタに置き、選べるかは習得技で決める(`learnsets ∩ regulation_moves`。ADR-0015)。

### 5. 種族・フォーム・メガ

- **種族集合は calc の Champions 世代**(P2-1b と同じ): calc の全種族から、(a) `config.json` の `excludeCalcSpecies`(calc の名前。現時点で内部フォーム `Aegislash-Both` のみ)と、
  (b) HP 種族値 1 を除く(いずれも警告 `species-excluded`)。`excludeCalcSpecies` の名前が calc に無ければ `ErrInvalidData`(改名を黙って素通りしない)。
- **calc ↔ Showdown の対応**: `toID(calc 名) == Showdown の id`、または `toID(calc 名) == toID(Showdown の name + "-" + baseForme)`
  (calc の `Aegislash-Shield` ↔ Showdown の `Aegislash`(baseForme `Shield`) のような既定フォームの表記違い。名前の対応表を持たない)。
  対応が無い calc の種族(除外の設定にも無い)は `ErrInvalidData`。対応した Showdown の `isNonstandard` が null でなければ除外して警告(技の規則1に揃える)。
- **値**: タイプ・種族値は calc を採り、Showdown と違えば `species-mismatch` の **Blocker**(ダメージに効く)。特性は Showdown の `abilities`(`"0"`→slot 1、`"1"`→2、`"H"`→3。calc は slot 0 しか持たない)。
  `"S"`(特殊な特性。Zygarde 系統など一部の種族にだけ現れる)は `species_abilities.slot` が 1..3 の3枠しかない(ADR-0015 §3)ため `"H"` と同じ slot 3 に置く
  (実装時の確認: 実データで `H` と `S` が同じ種族に同時に現れることは無い前提。両方揃った場合は slot の重複として `master.Species` の写像(§9 の投入前チェック)で検出して止める)。
  `showdown_id` は Showdown の id、`name_en` は Showdown の name、`dex_no` は Showdown の `num`。
- **フォルム番号 `{図鑑番号4桁}-{フォルム3桁}`**: 基本種(Showdown の name が `baseSpecies` の種族)の `formeOrder` における添字(基本種は 0)。
  見た目違いで畳んだフォームの添字も**欠番として数える**(Showdown が formeOrder に追記しても既存の番号が動かない)。
  `formeOrder` に無いフォームは `ErrInvalidData`。採番表をリポジトリに持たない(ADR-0002 §P2-2 草案の「採番表」を置き換える。第三者の一覧をコミットしないため)。
  **番号の安定性は投入時に検査**する: DB に既にある `showdown_id` の key が変わる投入は `ErrKeyChanged` で止め、DB を変えない(team-svc 等が保存した key を壊さない)。
- **見た目だけ違うフォームの畳み込み**(DECISIONS 2026-09-21): 同じ `num` の中で、性能(タイプの並び・種族値・特性の slot と ID)が同じものは、
  フォルム番号が最小のもの(代表)だけを取り込み、他は警告 `form-folded`(Detail に代表)にする。calc にあるかどうかに依らない。
  Showdown だけにあり(`isNonstandard` が null)、同じ num のどの取り込む種族とも性能が違うもの(戦闘中だけのフォーム等)は取り込まず警告 `species-showdown-only`(P2-2c で照合)。
- **メガ**: Showdown の `forme` が `Mega` で始まる種族は `is_mega = 1`、`base_species_key` = 基本種の key、`required_item_id` = `toID(requiredItem)`。
  `requiredItem` が空、またはその持ち物を取り込まない場合は `ErrInvalidData`(ADR-0015 のメガの整合と、「集合のメガなら持ち物も同じ集合」の行をまたぐ検査)。
  メガ以外の `requiredItem` は保存しない(スキーマに列が無い)。
- **習得技**: Showdown の `learnsets[showdown_id]`。無ければ `learnsets[toID(baseSpecies)]`(メガ・フォームは基本種を継ぐ)。取り込まない技は落とす。
  畳んだフォームの習得技は代表のものだけ(v1。差があれば P2-2c で報告)。
- **特性・タイプ・持ち物**:
  - 特性 = 取り込む種族の特性スロットに現れる ID(Showdown の abilities に null で存在すること。calc の一覧に無ければ警告 `ability-showdown-only`。calc の一覧は特性の正ではなく参考情報なので取り込みは止めない)。
  - タイプ = calc の `types` から `config.json` の `excludeTypes`(calc の型名。`???` と `Stellar`。calc に無い名前は `ErrInvalidData`)を除いたもの。
    sort_order は calc の並び順の 1 始まり。種族・技・効果が除外したタイプを使えば `ErrInvalidData`。相性表は除外したタイプの組を落とす(無い組は等倍)。
  - 持ち物 = 技と同じ規則(両方にあり Showdown で null → 取り込む。Showdown だけで null → 取り込み警告 `item-showdown-only`。calc だけ、または Showdown で非 null・無い → 除外警告 `item-excluded`)。
    技と違い値の食い違いは無い(名前と有無だけ)ので Blocker は無い。

### 6. 日本語名

- 優先順位: **override > PokeAPI > 英語名**。`name_ja_source` はそれぞれ `override` / `pokeapi` / `fallback_en`。英語名へのフォールバックと、どの行にも当たらない override は警告
  (`name-fallback` / `override-unused`。欠落件数の報告に使う)。
- PokeAPI の言語: `config.json` の `nameJaLanguages` の順(**既定 `["ja", "ja-Hrkt"]`**。ユーザー決定 2026-09-21。
  ADR-0002 決定2 の暫定案(`ja` 優先)に合わせる。順は `config.json` の値だけで決まり、Go 側にハードコードした既定は無い)。
- 突き合わせ: PokeAPI の `slug` を `toID` したものと、こちらの ID(種族は `showdown_id`)を比べる。種族は既定フォーム(Showdown の `forme` が空)なら `species`、それ以外は `forms` を引く。
  タイプの英語名は calc の型名(例 `Grass`)。
- override: `data/local/name_ja_overrides.json`(**Git に入れない**。中身が実在の日本語名=第三者データのため)。無ければ空として扱う。
  形式 `{"schemaVersion":1,"species":{<showdown_id>:名前},"moves":{},"items":{},"abilities":{},"types":{}}`(空の値は `ErrInvalidInput`)。
  `data/generated/` と分けるのは、生成物の掃除で人手の成果物を消さないため。k8s の CronJob への渡し方は P2-2d。

### 7. レギュレーション

- `data/importer/regulations.json`(Git 管理。人が管理する定義で、M-C はここにだけ現れる。コードに書かない):
  `{"schemaVersion":1,"regulations":[{"id","nameJa","isDefault","startsOn","endsOn","showdownMod"}]}`(日付は `YYYY-MM-DD` か空)。
- v1 の集合の作り方: 定義の `showdownMod` が Showdown スナップショットの `mod` と一致することを要求し(違えば `ErrInvalidData`)、
  **そのスナップショットの組から取り込んだ種族・技・持ち物**をそのレギュレーションの集合にする。特性の集合 = 集合の種族の特性スロット。
  v1 は定義1件(スナップショットの mod は1つ)。M-D 等を足すときは、mod ごとのスナップショットを読み、集合を mod ごとに作る形へ拡張する(スキーマは既に対応済み)。
- 既定のレギュレーションが2件以上なら `ErrInvalidData`(DB の UNIQUE と同じ規則を投入前に)。

### 8. 停止条件と報告(P2-2b は最小限。詳細な照合と報告は P2-2c)

| 区分 | 条件 | 動き |
|---|---|---|
| `ErrInvalidInput` | ファイルが無い・読めない・形式違反(未知フィールド・schemaVersion・source・版の食い違い・ID/日付の形式・showdown/pokeapi の版が commit 形式でない・`PENDING` で始まる版/mod=未固定のプレースホルダ) | 止める |
| `ErrInvalidData` | 入力同士が矛盾(対応の無い calc の種族、実在しない除外名、formeOrder に無いフォーム、メガの持ち物が無い、除外したタイプの使用(種族・技のどちらも)、mod の食い違い、既定が複数、Output が engine の型に写像できない=§1 の投入前チェック) | 止める |
| `master.ErrInvalidEffect` | 効果定義の値が不正 | 止める |
| `ErrBlocked` | **人間の裁定が要る食い違い**: 攻撃技のタイプ(規則3)、技の分類・威力、種族のタイプ・種族値 | 止める。`Report.Blockers` に列挙し、`Output` は空で返す(部分的な結果を投入に回さない) |
| 警告 | 変化技のタイプ、除外・補完した技/持ち物、特性が calc の一覧に無い、畳んだフォーム、Showdown だけの種族、除外した種族、英語名フォールバック、未使用の override・効果定義、優先度の違い | 続ける。`Report.Warnings` |

- `Convert` は止めたときも `Report` を返す。CLI は `data/generated/reports/import-<UTC時刻>.json` と `latest.json` に書く(Report の JSON。Git 管理外)。
- Report・Output は**決定的**(スライスは ID / key 順。map の反復順に依らない)。
- P2-2c の範囲: calc と Showdown の差分の網羅的な報告(値の全項目・件数の要約)、P2-1c の裁定の反映の確認、効果定義の網羅性、PokeAPI の図鑑との照合。

### 9. 版と冪等な投入

- `LoadInput(root)` は `root`(本番は `data/`)から読み、`[]SourceVersion{Source, Version, Checksum}` を source 順で返す。source は7つ:
  `calc` / `showdown` / `pokeapi`(version = 固定した版)、`importer-config` / `effects` / `regulations` / `name-overrides`(version = `local`。
  override が無ければ version `none`・checksum は空の sha256)。checksum はファイルのバイト列の sha256(16 進)。
- `NeedsImport(applied, incoming)`: incoming の検証(重複・source の形式 `^[a-z0-9]+(-[a-z0-9]+)*$`・version 非空・checksum 形式・空でない。違反は `ErrInvalidInput`)の後、
  **source の集合・各 version・各 checksum がすべて一致するときだけ false**(取り込まない)。version だけ変わった場合も取り込む(版の記録を正しく保つ)。
- `Apply(ctx, db, out, versions, now)`: 1 トランザクションで、(1) 既存の `showdown_id → key` と out を比べ、key が変わるものがあれば `ErrKeyChanged`、
  (2) マスタ・レギュレーションの全テーブルを out の内容に置き換え、(3) `data_versions` を versions に置き換える(`imported_at = now`)。
  失敗したら全部戻す。自己参照の外部キー(`species.base_species_key`)があるので、削除はメガを先・挿入はメガを後にする。SQL は sqlc(`services/pokedex/db/query/` に追加)で書く。
- `Run(ctx, db, out, versions, now, force)`: `AppliedVersions` → `NeedsImport`(force なら常に投入)→ `Apply`。取り込んだら true。
- 冪等性: 同じ Output で2回 `Apply` しても全テーブルの中身は同じ。版が同じなら `Run` は何も書かない(`imported_at` も変えない)。

### 10. P2-2c / P2-2d / P2-3 との境界

- P2-2c: 差分の網羅的な照合と報告、裁定の反映の確認(§8)。P2-2b の Report の種類(FindingKind)は P2-2c で増やしてよい。
- P2-2d: CronJob(週1回)と、上流の最新版の検出・`config.json` の版の更新の運用、override の k8s への渡し方。
- P2-3: balance 向け read model の出力(`pokedex export`)。P2-2b は出力しない。

### 11. コマンド

- `services/pokedex/cmd/import`: `-data <dir>`(既定は `../data`。Makefile から services ディレクトリで実行するため)、`-dry-run`(変換と報告だけ。DB に触らない)、
  `-force`。DSN は `POKEDEX_DATABASE_DSN`。`ErrBlocked` なら報告を書いて非 0 で終わる。
- Makefile:
  - `import-fetch`: `tools/importer` で `npm ci` と抽出(**ネットワークが要る**。版は `data/importer/config.json`)
  - `import`: `go run ./pokedex/cmd/import -data ../data`(`POKEDEX_DATABASE_DSN` が無ければ失敗。既存の「P2 で実装」の表示を置き換える)
  - `import-dry-run`: DB なしで変換と報告
  - `test-db`: 対象を `./pokedex/...` に広げる(importer の `-tags mysql` のテストを含める)。`db` と `importer` の両パッケージが同じ
    `POKEDEX_TEST_DSN` の DB へ `DownAll`/`Up`/投入を行うため、`go test` の既定のパッケージ並列実行のままだと競合する
    (実装時に確認: デッドロック・重複キーで失敗した)。`-p 1` を付けてパッケージ間を直列にする
  - `lint`: `node --check tools/importer/*.mjs` を足す
- **ネットワークに触るもの・実データを使うものは `make test` に入れない**。

### 12. Go の API(テストが前提にする名前。package `example.com/pokecalc/services/pokedex/importer`)

```go
var ErrInvalidInput, ErrInvalidData, ErrBlocked, ErrKeyChanged error

type FindingKind string // 値はケバブケース(例 "move-type-mismatch")
const (
    KindMoveExcluded, KindMoveShowdownOnly, KindMoveTypeMismatch, KindMoveValueMismatch FindingKind = ...
    KindItemExcluded, KindItemShowdownOnly FindingKind = ... // 持ち物(技と同じ規則。critic 指摘で追加)
    KindSpeciesMismatch, KindSpeciesShowdownOnly, KindSpeciesExcluded, KindFormFolded FindingKind = ...
    KindAbilityShowdownOnly FindingKind = ... // 特性が calc の一覧に無い(critic 指摘で追加)
    KindNameFallback, KindOverrideUnused, KindEffectUnused FindingKind = ...
)
type Finding struct{ Kind FindingKind; ID, Detail string } // ID は技・持ち物・特性 ID、種族は showdown_id(calc だけのものは toID(calc 名))
type Report struct{ Warnings, Blockers []Finding }

// スナップショット・入力(JSON キーは §3 の表。フィールド名はその UpperCamel。BaseStats は HP/Atk/Def/SpA/SpD/Spe)
type BaseStats struct{ HP, Atk, Def, SpA, SpD, Spe int }
type CalcSnapshot struct{ SchemaVersion int; Source, Version string; Generation int; Types []string
    TypeChart map[string]map[string]int; Species []CalcSpecies; Moves []CalcMove; Items, Abilities []string }
type CalcSpecies struct{ Name string; Types []string; BaseStats BaseStats }
type CalcMove struct{ Name, Type, Category string; BasePower, Priority int } // Type/Category は "" = 省略
type ShowdownSnapshot struct{ SchemaVersion int; Source, Version, Mod string; Species []ShowdownSpecies
    Moves []ShowdownMove; Items []ShowdownItem; Abilities []ShowdownAbility; Learnsets map[string][]string }
type ShowdownSpecies struct{ ID, Name string; Num int; BaseSpecies, Forme, BaseForme string; Types []string
    BaseStats BaseStats; Abilities map[string]string; RequiredItem string; FormeOrder []string; IsNonstandard *string }
type ShowdownMove struct{ ID, Name, Type, Category string; BasePower, Accuracy, PP, Priority int; IsNonstandard *string } // Accuracy 0 = 必中
type ShowdownItem struct{ ID, Name string; IsNonstandard *string }
type ShowdownAbility struct{ ID, Name string; IsNonstandard *string }
type PokeAPISnapshot struct{ SchemaVersion int; Source, Version string; Species, Forms, Moves, Items, Abilities, Types []PokeAPIName }
type PokeAPIName struct{ Slug string; Names map[string]string }
type NameOverrides struct{ SchemaVersion int; Species, Moves, Items, Abilities, Types map[string]string }
type EffectsFile struct{ SchemaVersion int; Items, Abilities map[string]json.RawMessage }
type RegulationsFile struct{ SchemaVersion int; Regulations []RegulationDef }
type RegulationDef struct{ ID, NameJa string; IsDefault bool; StartsOn, EndsOn, ShowdownMod string }
type Config struct{ SchemaVersion int; Sources map[string]string; ExcludeCalcSpecies, ExcludeTypes, NameJaLanguages []string }
type Input struct{ Calc CalcSnapshot; Showdown ShowdownSnapshot; PokeAPI PokeAPISnapshot; Overrides NameOverrides
    Effects EffectsFile; Regulations RegulationsFile; Config Config }

func DecodeCalcSnapshot([]byte) (CalcSnapshot, error)        // 以下すべて失敗は ErrInvalidInput
func DecodeShowdownSnapshot([]byte) (ShowdownSnapshot, error)
func DecodePokeAPISnapshot([]byte) (PokeAPISnapshot, error)
func DecodeNameOverrides([]byte) (NameOverrides, error)
func DecodeEffectsFile([]byte) (EffectsFile, error)
func DecodeRegulationsFile([]byte) (RegulationsFile, error)
func DecodeConfig([]byte) (Config, error)                     // nameJaLanguages が空なら失敗
func LoadInput(root string) (Input, []SourceVersion, error)   // root/importer/*.json, root/generated/<source>/<version>/snapshot.json, root/local/name_ja_overrides.json(任意)

// 出力(ADR-0015 の行。スライスは ID / key 順)
type TypeRow struct{ master.TypeRow; NameJaSource string }
type NamedRow struct{ ID, NameJa, NameJaSource, NameEn string }            // abilities / items
type MoveRow struct{ ID, NameJa, NameJaSource, NameEn, Type, Category string; Power, Accuracy, PP, Priority int } // Accuracy 0 = NULL
type SpeciesRow struct{ master.SpeciesRow; NameJaSource string; Abilities []master.SpeciesAbilityRow }
type EffectRow struct{ ID string; Effect []byte }                          // 正準形
type LearnsetRow struct{ SpeciesKey, MoveID string }
type RegulationRow struct{ ID, NameJa string; IsDefault bool; StartsOn, EndsOn string }
type RegulationMemberRow struct{ RegulationID, MemberID string }
type Output struct{ Types []TypeRow; TypeChart []master.TypeChartRow; Abilities, Items []NamedRow; Moves []MoveRow
    Species []SpeciesRow; ItemEffects, AbilityEffects []EffectRow; Learnsets []LearnsetRow; Regulations []RegulationRow
    RegulationSpecies, RegulationMoves, RegulationItems, RegulationAbilities []RegulationMemberRow }
func Convert(in Input) (Output, Report, error)

type SourceVersion struct{ Source, Version, Checksum string }
func Checksum(raw []byte) string
func NeedsImport(applied, incoming []SourceVersion) (bool, error)
func Apply(ctx context.Context, db *sql.DB, out Output, versions []SourceVersion, now time.Time) error
func AppliedVersions(ctx context.Context, db *sql.DB) ([]SourceVersion, error)
func Run(ctx context.Context, db *sql.DB, out Output, versions []SourceVersion, now time.Time, force bool) (bool, error)
```

## 却下案

- **Go の importer を `tools/importer` に置く**: `services/internal/master` を import できず、写像の規則を二重に持つことになる。
- **PokeAPI の REST を全件叩く**: 約 1,200 回。版が固定できず、公平利用に反しやすい。CSV を commit 固定で取る。
- **フォルム番号の採番表をリポジトリに置く**(ADR-0002 §P2-2 草案): 第三者の種族・フォームの一覧をコミットすることになる。formeOrder の添字 + 投入時の安定性検査で代える。
- **効果定義を `data/generated` 側(Git 管理外)に置く**: 人が管理する唯一の正で再生成できず、engine のテストと同じ種類の数値なのに失われうる。
- **override を Git に置く**: 実在の日本語名の一覧になる(ADR-0002)。
- **差分の自動解決(攻撃技のタイプの食い違いで calc か Showdown を自動で採る)**: P2-1c の規則3(人間の裁定を待つ)に反する。
- **投入を UPSERT だけにする(消さない)**: レギュレーションから外れた行が残り、集合が過去の版と混ざる。全置き換えを 1 トランザクションで行う。

## 限界

- 抽出スクリプト(Node)の出力が本 ADR の形であることは、`make import-fetch` を実データで実行して Go の `LoadInput` が通ることでしか確かめていない(Node 側の単体テストは無い)。
- PokeAPI の CSV に Champions 固有の名前(新特性・メガ石・メガフォーム)がどこまで入っているかは未確認(ADR-0002 の確認は REST)。欠落は override で補う。
- formeOrder の添字による採番は Showdown が並びを入れ替えると番号が変わる。投入時の `ErrKeyChanged` で検出して止めるが、直すのは人間。
- 畳んだフォームの習得技の差は見ていない(P2-2c)。
- 習得技の継承(進化前のタマゴ技など。Pokemon Showdown のバリデータは習得可否の判定で進化前の学習元もたどる)は
  `learnsets[showdown_id]`(無ければ baseSpecies)をそのまま使う v1 の実装では追っていない。実データで
  実際に抜けが出るかは P2-2c で確認する(架空データの fixture では進化系統を作っていないため未検証)。

## 人間の確認事項 → ユーザー回答 2026-09-21(DECISIONS.md 参照)で確定

1. **効果定義ファイル `data/importer/effects.json` をコミットする** → 確定(コミットする。英語 ID と 4096 基準の整数だけ。testdata/golden/effects.json と同じ扱い)。
2. **日本語名の言語の優先順** → 確定(`ja` → `ja-Hrkt`。ADR-0002 決定2 の暫定案どおり。§6 を参照。`config.json` の並びで変えられる)。
3. **`data/importer/regulations.json` の日本語のレギュレーション名(例「レギュレーション M-C」相当)をコミットする** → 確定(コミットする。ゲームの固有表現ではなく利用者のラベルとして扱う)。
