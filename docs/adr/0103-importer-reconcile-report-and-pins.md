# ADR-0103: importer の照合と差分報告・取得元の版の固定(P2-2c)

- 状態: 採用(2026-09-22)。P2-2c の仕様。spec-writer 起草、implementer が実装、critic がレビュー
- 日付: 2026-09-22
- 関連: plan.md P2-2c、ADR-0101(§3 スナップショット・§5 習得技・§8 停止条件・§10 境界・§限界。本 ADR が拡張する)、
  ADR-0002(確定した方針・§3・追記 P2-1c の結論の件数と規則)、ADR-0005(データ駆動の効果定義)、ADR-0100(スキーマ)、
  CLAUDE.md 絶対ルール 2/4/6
- 番号: データレーンの帯(0100〜。COORDINATION.md)の4本目

## 背景

P2-2b(ADR-0101)で取得・変換・投入の仕組みはできたが、(1) 取得元の版が `PENDING-PIN-*` のままで実データを一度も通していない、
(2) calc と Showdown の差分は「止めるか・警告か」の最小限の Report しか無く、ADR-0002 追記 P2-1c の裁定(件数)が
実データで再現されているかを機械的に確かめる手段が無い、(3) 効果定義の網羅性・日本語名の欠落・Showdown だけの種族・
畳んだフォームと進化前からの習得技の扱いが未確認(ADR-0101 §限界)。P2-2c でこれらを「照合と差分報告」として一つにまとめ、
版を固定して実データで通す。

## 決定

### 1. 形:`Reconcile` = `Convert` + 照合。CLI は `Reconcile` を呼ぶ

```go
// package importer(services/pokedex/importer/reconcile.go)
func Reconcile(in Input) (Output, Reconciliation, error)
func FormatSummary(r Reconciliation) string                               // 人が読む要約(決定的)
func WriteReconciliation(dir string, r Reconciliation, now time.Time) error // 報告ファイルを書く(§2)
```

- `Reconcile` は内部で `Convert` を呼び、その Output・Report に照合結果を足す。`Convert` の API と挙動(ADR-0101 §12)は変えない
  (習得技の継承 §7 を除く)。
- エラー:
  - `Convert` が `ErrInvalidInput` / `ErrInvalidData` / `ErrInvalidEffect` → そのまま返す(Reconciliation はゼロ値)。
  - `Convert` が `ErrBlocked` → **照合できる部分(値の比較 §3・件数 §4 の calc/Showdown 側・裁定の照合 §5)は埋めて**
    `Partial = true`、`Output` はゼロ値、`ErrBlocked` を返す(止まったときこそ報告が要る)。Output に依る節(効果定義・日本語名・習得技)は空。
  - `Convert` は通ったが**裁定の照合 §5 が Blocker** → `Output` はゼロ値、`ErrBlocked`(`errors.Is` で判定できる形で包む)。
  - `in.Config.Reconcile` が無い → `ErrInvalidInput`(実データの取り込みでは照合を必須にする。`Convert` 単体は従来どおり不要)。
- CLI(`services/pokedex/cmd/import`)は `Convert` の代わりに `Reconcile` を呼び、止まったときも `WriteReconciliation` で報告を書き、
  `ErrBlocked` なら非 0 で終わる。`-dry-run` でも報告を書く。**照合の Blocker で止まったものは投入しない**。
  P2-2d の CronJob も同じ入口を使う(版が変わって裁定の件数が崩れたら自動では入らない)。
- すべて純粋(`WriteReconciliation` だけがファイルを書く)。ネットワークに触らない。

### 2. 報告の形式と置き場所(Git 管理外)

| ファイル | 中身 |
|---|---|
| `data/generated/reports/import-<UTC 時刻 20060102T150405Z>.json` | `Reconciliation` の JSON(機械可読の全部) |
| `data/generated/reports/latest.json` | 上と同じバイト列 |
| `data/generated/reports/latest-summary.txt` | `FormatSummary` の出力(件数・止めた理由・警告の種類別件数・裁定の照合結果) |

- P2-2b の `latest.json`(Report だけ)を `Reconciliation` に置き換える(Report は `Reconciliation.Report` として中に残る)。
- **中身は ID・件数・値(数値・タイプ ID・分類)だけ**。英語名・日本語名・説明文を入れない(ADR-0002「第三者データの一覧を
  docs やコミットに載せない」。個別の ID は `data/generated/` にだけ出す、は ADR-0002 追記 P2-1c のとおり)。
  テストで「架空データの英語名・日本語名が JSON にも要約にも現れない」ことを確かめる。
- 報告から docs・コミットメッセージ・ADR に転記してよいのは**件数だけ**(ADR-0002 の既存の書き方と同じ)。
- 決定的(同じ入力なら `Reconciliation` は `reflect.DeepEqual`。スライスは ID 順、map のキーは固定の集合)。

### 3. calc と Showdown の値の全項目比較(`MoveDiffs` / `SpeciesDiffs`)

```go
type ValueDiff struct {
    Entity   string // "move" | "species"
    ID       string // 技 ID / Showdown の種族 ID
    Field    string // 下表
    Calc     string // 値の文字列表現(数値は10進、タイプは小文字の ID を "/" で連結、分類は小文字)
    Showdown string
    Severity string // "blocker" | "warning" | "info"
    Imported bool   // 取り込み対象か(技: calc が断片でなく Showdown で isNonstandard が null。種族: 対応があり Showdown で null、除外設定・HP 1 に当たらない)
}
```

| Entity | 比較する組 | Field | Severity(Imported のとき) |
|---|---|---|---|
| move | calc が断片でなく、Showdown に同じ ID がある(isNonstandard を問わない) | `type` | 変化技なら warning、攻撃技なら blocker(規則3) |
| | | `category`(calc の省略 = status) | blocker |
| | | `basePower` | blocker |
| | | `priority` | warning |
| species | calc の種族と対応する Showdown の種族(ADR-0101 §5 の対応規則) | `types` | blocker |
| | | `hp` / `atk` / `def` / `spa` / `spd` / `spe` | blocker |

- Imported でない組の差分は **info**(取り込まないので止めないが、網羅的に見せる)。
- Severity は ADR-0101 §4・§5 の Blocker/警告の区分と同じ(新しい停止条件を足さない)。**比較は Convert の判定と独立に全項目を並べる**
  (Convert は種族のタイプと種族値の食い違いを1件の `species-mismatch` にまとめるが、報告は項目ごと)。
- 比較しない項目: 命中・PP(calc が持たない。Showdown の値を採る)、特性(calc の種族は slot 0 しか持たずスナップショットに出していない)、
  持ち物・特性の値(名前と有無だけ。§4 の件数で見る)。
- 並びは Entity → ID → Field。

### 4. 件数の要約(`Summary`)

```go
type SetSummary struct{ Calc, Showdown, ShowdownStandard, Both, CalcOnly, ShowdownOnly, Imported int }
type Summary struct {
    Moves, Species, Items, Abilities SetSummary
    WarningCounts, BlockerCounts map[FindingKind]int // Reconciliation.Report の Kind 別件数
}
```

| 集合 | Calc | Showdown / ShowdownStandard | Both | CalcOnly | ShowdownOnly | Imported |
|---|---|---|---|---|---|---|
| Moves | 番兵 `(No Move)` を除く calc の技(断片を含む) | 全件 / isNonstandard が null | calc の ID ∩ Standard | calc の ID ∖ Standard | Standard ∖ calc の ID | `len(Output.Moves)` |
| Species | calc の種族 | 全件 / null | Showdown に対応がある calc の種族 | 対応が無い calc の種族(除外設定のもの) | null で、どの calc の種族とも対応しない Showdown の種族 | `len(Output.Species)` |
| Items | calc の持ち物 | 全件 / null | toID が一致 ∩ Standard | calc ∖ Standard | Standard ∖ calc | `len(Output.Items)` |
| Abilities | calc の特性の一覧 | 全件 / null | 同上 | 同上 | 同上 | `len(Output.Abilities)` |

- `Partial` のとき Imported は 0。

### 5. P2-1c の裁定の反映の確認(`Verdicts`)

ADR-0002 追記 P2-1c の結論(calc だけの 11 技は除外・Showdown だけの 1 技は使用可・変化技のタイプの食い違い 1 件は Showdown 側)を、
**件数と ID 集合の sha256** として `data/importer/config.json` に置き、毎回の取り込みで実データと照合する。

```json
"reconcile": {
  "effectHooks": ["onBasePower", "..."],
  "verdicts": {
    "basis": {"calc": "0.12.0", "showdown": "<裁定を行った Showdown の 40 桁 commit>"},
    "moves": {
      "calcOnlyExcluded":     {"count": 11, "idsSha256": "<64 桁 16 進>"},
      "showdownOnlyIncluded": {"count": 1,  "idsSha256": "<64 桁 16 進>"},
      "statusTypeMismatch":   {"count": 1,  "idsSha256": "<64 桁 16 進>"}
    }
  }
}
```

| key | 実データ側の ID 集合の定義 | 食い違ったとき |
|---|---|---|
| `calcOnlyExcluded` | 番兵を除く calc の技の ID ∖ 取り込む技の ID | **Blocker**(使える技の集合が裁定なしに変わった) |
| `showdownOnlyIncluded` | 取り込む技の ID ∖ 番兵を除く calc の技の ID | **Blocker**(同上) |
| `statusTypeMismatch` | 両方にあり(calc が断片でない)取り込む変化技のうちタイプが違うもの | 警告(ダメージに効かない) |

- 取り込む技の ID は Convert の技の規則(ADR-0101 §4)の結果。Convert が種族で止まっても技の判定はできるので、`Partial` でも照合する。
- `idsSha256` = ID を昇順に並べ `"\n"` で連結した UTF-8 バイト列(末尾改行なし。空集合は空文字列)の sha256 の 16 進小文字。
  **件数だけでは同数の入れ替わりを見逃す**ので集合のハッシュも照合する。ハッシュは ID を明かさないので Git に置ける(ADR-0002 の方針に反しない)。
- 報告の `VerdictChecks []VerdictCheck{Key, ExpectedCount, ActualCount, ExpectedSHA256, ActualSHA256, IDs []string, OK bool}`
  (key 順)に実際の ID を出す(`data/generated` だけ)。食い違いは Finding `verdict-mismatch`(ID = key)。
- `basis` の各版が `sources` の同じキーの版と違えば警告 `verdict-basis-changed`(ID = source 名)。上流の版を上げたとき
  「裁定を持ち越している」ことを見えるようにする。件数・ハッシュが一致していれば止めない。
- **件数の期待値(11 / 1 / 1)は ADR-0002 追記 P2-1c の結論そのもの**で、`data/importer/config.json` にそれが入っていることを
  `make test` のテスト(`TestRepoConfigIsPinned`)で確かめる。裁定を更新するときは ADR-0002 に追記してから config を変える。
- 却下: 警告だけにする(CronJob が週1回で黙って使用可能集合を変える。P2-1c の「人間の裁定を待つ」に反する)/
  ID の一覧を config に置く(第三者の一覧をコミットすることになる)。

### 6. 効果定義の網羅性(`EffectCoverage`)

- 「補正を持つべき」の判定は **Showdown のコードを実行して得た持ち物・特性のイベントハンドラ名**から得る。
  - Showdown のスナップショットの `items[]` / `abilities[]` に `hooks`(その持ち物・特性のデータオブジェクトが持つ、`on` で始まる
    関数のプロパティ名の昇順。無ければ `[]`)を足す(ADR-0101 §3 の表の拡張)。
  - どのハンドラを「ダメージに効く」とみなすかは `config.json` の `reconcile.effectHooks`(Showdown のハンドラ名。`^on[A-Z][A-Za-z]*$`)。
    Go にハンドラ名も持ち物名もハードコードしない。初期値の候補: `onBasePower` `onSourceBasePower` `onAnyBasePower` `onModifyAtk` `onModifySpA`
    `onModifyDef` `onModifySpD` `onSourceModifyAtk` `onSourceModifySpA` `onModifyDamage` `onSourceModifyDamage` `onEffectiveness` `onModifySTAB`
    `onModifyType`(implementer が実データで見て調整してよい。変えたら本 ADR に追記)。
- 判定(取り込む持ち物・特性だけ):
  - hooks ∩ effectHooks が空でないのに `effects.json` に定義が無い → 警告 `effect-missing`。
  - `effects.json` に定義があるのに hooks ∩ effectHooks が空 → 警告 `effect-no-hook`(定義の付け間違いの疑い)。
- どちらも**止めない**(engine がまだ表せない補正は ADR-0005 の範囲で後から足す。件数で進み具合を見る)。
- 報告: `EffectCoverage{Items, Abilities CoverageStats}`、`CoverageStats{Imported, WithDamageHooks, Defined, Missing, NoHook int; MissingIDs, NoHookIDs []string}`。
- 却下: `testdata/golden/effects.json` を「持つべき」の正にする(engine が照合済みの物しか無く循環する)/ 持ち物の一覧を手で持つ
  (ハードコードと第三者の一覧のコミット)/ calc のコードの分岐を解析する(構造化されていない)。

### 7. 習得技の解決 — 採用: 進化前からは継がない(Champions のルール)

**結論(ユーザー決定)**: 習得技は ADR-0101 §5 の元の規則(自分の学習元、無ければ基本種(フォーム・メガの
base species)の学習元を1段だけ)のまま。**進化前(prevo)はたどらない**。M-C(champions mod)は
これが実際の Showdown の挙動であるため。

**検討した3案の経緯**(a→b→この結論の順に実データで確定した。実データ確認の前は既定案 a で仮置きしていた):

| 案 | 内容 | 結果 |
|---|---|---|
| a. 進化前をたどるだけ(世代を見ない) | `resolved(s)` に `resolved(prevo)` を無条件に足す | **却下**。§10 の1回目の標本確認(5件)で PS の `TeamValidator.checkCanLearn` と全件食い違った |
| b. フォーマットの `minSourceGen` 未満の学習元を除く | 学習元の符号(例 `"9M"` `"7L12"`)の先頭の数字を世代とし、`minSourceGen` 未満の学習元しか無い技は継承・自分の学習元のどちらでも除く | **却下**。実装して2回目の標本確認(新たな5件)をしたが、**依然全件食い違った**。原因は世代ではなく下記 |
| **採用: 進化前から継がない** | ADR-0101 §5 の元の規則のまま(自分の学習元、無ければ基本種を1段だけ) | **採用**。Showdown 本体のソースを読んで根本原因(mod 固有の無効化)を特定できたため |

**なぜ案bも食い違ったか(根本原因)**: Showdown 本体のソース(`dex-species.js` の `learnsetParent`)に次の行がある。

```js
} else if (species.prevo) {
  if (this.dex.currentMod.startsWith("champions")) return null;   // champions は進化前をたどらない
  species = this.get(species.prevo);
  ...
```

**champions mod は、進化前をたどる学習元の継承を世代を問わず常に無効化している**(coded 上の断定。世代の
新旧に依らない)。実データを確認したところ、標準種族の直接の学習元はほぼ全件が世代9の学習元を持つため
(400種族・15,764件の学習元を確認して世代9未満は0件)、案bの世代フィルタ自体は実質ほぼ効果が無く、
真因は「世代情報が足りない」ことではなく「この mod は進化前を継がない」ことだった。

**採用した規則(ADR-0101 §5 の元の規則。変更なし)**:

- `resolved(s)` = `s` 自身の学習元(`learnsets[s.id]`)があればそれ(`minSourceGen` 以上の世代のものだけ)、
  無ければ基本種(フォーム・メガの `baseSpecies`。**進化前ではない**)の学習元を1段だけ。
- `minSourceGen` による世代の絞り込みは、進化前の継承と無関係に**常に**効く(PS の `checkCanLearn` は
  進化前をたどらない場合でも学習元の世代を見るため)。実データでは自分の学習元は基本的に世代9を含むため
  実質ほとんど効かないが、将来のデータで古い世代しか無い学習元が現れたときのために残す。
- 取り込まない技は落とす(従来どおり)。

**進化前をたどる継承(案b)はコードに残すが、既定では使わない(データ駆動)**:

- どちらの規則を使うかは `regulations.json` の `inheritFromPrevo`(真偽値)で選ぶ。**Go のコードに
  `"champions"` などの mod 名をハードコードしない**。M-C は `false`。
- `true` のときは、案bの実装(`resolved(s)` に `s.prevo` があれば `resolved(prevo の種族)` を何段でも足す。
  `minSourceGen` はそのまま適用)をそのまま使う。将来、進化前をたどる別のレギュレーションが要るときのために
  実装・テストを残す(却下せず残した理由: 実装済みで動作確認済みのコードを削除する積極的な理由が無く、
  `inheritFromPrevo` 1個のフラグ分岐で済み複雑さの増分が小さいため)。
- `prevo` の名前が Showdown の種族に無い・循環する場合の `ErrInvalidData` は、**`inheritFromPrevo: true` の
  ときだけ**検査する(false のときは prevo を見ないので検査もしない。壊れた `prevo` があっても無視する)。
- 複数のレギュレーションで `inheritFromPrevo` の値が割れていたら `ErrInvalidData`(`minSourceGen` の一致条件と同じ考え方。
  v1 は1つの Showdown スナップショットに対して単一の習得技テーブルしか持たないため。§9)。
- スナップショットの拡張(据え置き。有用なため残す):
  - `species[]` に `prevo`(Showdown の `species.prevo`。名前。無ければ `""`)。使わなくても実データの構造そのままで
    無害。将来 `inheritFromPrevo: true` を使うときに要る。
  - `learnsets` は**全種族の分**を、`{種族ID: {技ID: 学習した最大世代}}` の形で出す(ADR-0101 §3 の注記を置き換える)。
    学習元の符号(Showdown の `learnset[moveId]` が持つ文字列の配列。例 `["9M","7L12"]`)の**先頭の数字が世代**。
    1つの技に複数の学習元があれば最大の世代だけを残す。
  - `regulations.json` の各レギュレーション定義に `inheritFromPrevo`(真偽値)・`minSourceGen`(整数。Showdown の
    `TeamValidator` の `minSourceGen` に対応)を足す。
- 報告: `Learnsets.Inherited`(継承で増えた `(species_key, move_id)` の行数)、
  `Learnsets.InheritedBySpecies map[species_key]int`。M-C(`inheritFromPrevo: false`)では常に 0。
- 却下(変わらず): Node 側で PS の関数に解決させる(`getFullLearnset` 等の API が版で変わり、規則を Go のテストで
  固定できない。検証の突き合わせに使うのはよい)。
- 限界: `inheritFromPrevo: true` の経路(現状未使用)は、案bの近似のまま(学習元の種類・`learnsetDomain` までは
  見ない。上の「案bも食い違った」の記載のとおり、この経路を実際に使う場合は mod 側の `learnsetParent` の
  挙動を個別に確認する必要がある)。

### 8. 畳んだフォームの習得技の差・Showdown だけの種族・日本語名

- **畳んだフォーム**(`form-folded`): 畳んだフォームの `resolved`(取り込む技に絞る)と代表の `resolved` を比べ、違えば警告
  `form-learnset-diff`(ID = 畳んだフォーム、Detail = 代表)と `FoldedLearnsets []FoldedLearnsetDiff{ID, Representative, OnlyInFolded, OnlyInRepresentative []string}`。
  v1 の取り込みは代表のものだけのまま(ADR-0101 §5)。実データで差が出たら扱いを決める(件数を ADR に追記)。
- **Showdown だけの種族**: isNonstandard が null で calc に対応が無い Showdown の種族を全部、理由付きで出す
  (`ShowdownOnlySpecies []ShowdownOnlySpecies{ID, Num, Reason, Representative}`、ID 順)。Reason:
  `folded`(代表に畳んだ。Representative = 代表)/ `different-performance`(同じ num の取り込む種族と性能が違う。`species-showdown-only`)/
  `num-not-in-calc`(同じ図鑑番号の種族を calc が持たない)/ `unresolvable`(formeOrder に無い等で中間表現にできない。Convert は黙って飛ばしている)。
  ADR-0002 §3 の「Showdown 側のみ 35」の内訳をここで確かめる。
- **日本語名**(PokeAPI との照合): `Names map[string]NameStats`(キー `species` `moves` `items` `abilities` `types`)、
  `NameStats{Override, PokeAPI, FallbackEn int; ByLanguage map[string]int; FallbackIDs []string}`。`ByLanguage` は PokeAPI で採った名前の言語
  (`config.nameJaLanguages` の順で最初に見つかったもの)の件数。欠落件数 = `FallbackEn`。

### 9. 停止条件と警告(ADR-0101 §8 への追加)

| 区分 | 条件 | 動き |
|---|---|---|
| `ErrInvalidInput` | `config.reconcile` が無い(Reconcile のとき)、`reconcile` の形式違反(未知フィールド・count が負・sha256 が64桁16進でない・effectHooks が空または形式違反・basis のキーが sources に無い・basis の版が空/`PENDING`)、レギュレーションの `minSourceGen` が1未満(§7) | 止める |
| `ErrInvalidData` | `inheritFromPrevo: true` のときだけ: `prevo` が無い種族を指す・循環。複数のレギュレーションで `minSourceGen` / `inheritFromPrevo` の値が違う(§7) | 止める |
| `ErrBlocked` | ADR-0101 §8 の既存の条件 + 裁定の照合 `calcOnlyExcluded` / `showdownOnlyIncluded` の食い違い | 止める。Output は空、報告は書く |
| 警告 | `verdict-mismatch`(`statusTypeMismatch`)、`verdict-basis-changed`、`effect-missing`、`effect-no-hook`、`form-learnset-diff` | 続ける |

- 追加の FindingKind: `verdict-mismatch` `verdict-basis-changed` `effect-missing` `effect-no-hook` `form-learnset-diff`
  (`KindVerdictMismatch` `KindVerdictBasisChanged` `KindEffectMissing` `KindEffectNoHook` `KindFormLearnsetDiff`)。
  Convert の Warnings と合わせて `Reconciliation.Report` に入れる(ソート規則は Convert と同じ)。

### 10. 取得元の版の固定と実データでの確認(P2-2c の完了条件)

**固定の方針**

| source | 固定する版 | 選び方 |
|---|---|---|
| calc | `0.12.0`(npm。済み) | 変えない |
| showdown | `smogon/pokemon-showdown` の 40 桁 commit | **P2-1c の裁定を行った commit(短縮 `f10d679`、2026-09-20)の完全な hash**。裁定の件数がその版で確認済みなので、取り込むデータ = 裁定したデータになる。最新の HEAD への追従は P2-2d の運用(上げたら照合 §5 が通るまで入らない) |
| showdown mod | `regulations.json` の `showdownMod` | 上の commit で Reg M-C のフォーマットが使う mod 名(ADR-0002 §3 の記録では `champions`)を `config/formats.ts` で確認して入れる |
| pokeapi | `PokeAPI/pokeapi` の 40 桁 commit | 固定作業の時点の `master` の HEAD(PokeAPI は Champions の技数値に使わず名前だけなので、裁定との結び付きは無い) |

- 記録場所: `data/importer/config.json` の `sources` と `reconcile.verdicts.basis`、`data/importer/regulations.json` の `showdownMod`。
  取得日時・tarball の sha256 は `data/generated/.../meta.json`(Git 管理外)。**版そのもの(英語の ID と 16 進)は本 ADR にも書いてよい**
  (第三者のデータではなく版の metadata。ADR-0002 の「Git に置くのは…データの生成元/版の metadata」に当たる)。固定したら §12 に追記する。
- `TestRepoConfigIsPinned`(`make test`)がコミットされた config について、PENDING が無い・commit の形式・`reconcile` があり件数が 11/1/1・
  ハッシュが全 0 でない、を確かめる。

**実データでの確認の手順(implementer。ネットワークは下の最小回数だけ。取得物は `data/generated/` で Git 管理外)**

1. 版を決める: `https://api.github.com/repos/smogon/pokemon-showdown/commits/f10d679`(1 回)で完全な hash と日付を確認、
   `git ls-remote https://github.com/PokeAPI/pokeapi refs/heads/master`(1 回)で HEAD。`config.json` / `regulations.json` に入れる。
   `reconcile.verdicts` はいったん件数 11/1/1・ハッシュ 64 桁の `0` で置く。
2. `make import-fetch`(calc の npm・Showdown の tarball 1 本・PokeAPI の CSV 約 14 本。キャッシュされ再取得しない)。
3. 未検証点を確かめる(結果は件数だけ本 ADR §12 に書く):
   - PS のビルド: `npm ci --omit=dev && node build` で `dist/sim/dex.js` ができるか。できなければ `npm ci`(dev 込み)に直す。
   - `Dex.mod(<mod>)` が M-C の mod を指すか(種族 392・技のうち null 515・持ち物 166 前後。ADR-0002 §3 の件数と比べる)。
   - `dex.learnsets.get(id)` が同期/非同期のどちらでも動くか、mod の learnsets が使われているか(継承の前後の行数)。
   - `dex.items.get` / `dex.abilities.get` の返す物に `on*` の関数が残っているか(`hooks` が空でない持ち物の件数)。
   - `species.prevo` が名前で入っているか。
   - PokeAPI の CSV の列名(`languages.csv` の `identifier`、`pokemon_form_names.csv` の `pokemon_name`、各 `*_names.csv` の `local_language_id` / `name`)。
     各カテゴリの件数が 0 でないこと、`Names` の `FallbackEn` の件数。
   - 習得技の継承: PS の `TeamValidator`(M-C のフォーマット)で、継承で増えた技を持つ種族を 5 種ほど標本に `checkCanLearn` 相当の判定と
     Go の結果が一致するか。一致しなければ §7 の既定を止めて報告する。
4. `make import-dry-run` → `data/generated/reports/latest.json` の `verdictChecks` で ID と件数を確認し(件数 11/1/1 と ID の性質が
   ADR-0002 追記 P2-1c の記述どおりか)、`actualSha256` を config に写す。もう一度 `make import-dry-run` が 0 で終わり、
   `verdict-mismatch` が無いこと。
5. 件数(Summary・警告の種類別・Names・EffectCoverage・ShowdownOnlySpecies の理由別・Learnsets.Inherited)を §12 に書く。個別の ID・名前は書かない。

### 11. Go の API の追加(テストが前提にする名前)

```go
// Config に追加(JSON キー "reconcile"。無くても DecodeConfig は通る。有れば厳格に検証)
type Config struct{ /* 既存 */ ; Reconcile *ReconcileConfig }
type ReconcileConfig struct{ EffectHooks []string; Verdicts Verdicts }            // "effectHooks", "verdicts"
type Verdicts struct{ Basis map[string]string; Moves MoveVerdicts }              // "basis", "moves"
type MoveVerdicts struct{ CalcOnlyExcluded, ShowdownOnlyIncluded, StatusTypeMismatch VerdictCount }
type VerdictCount struct{ Count int; IDsSHA256 string }                          // "count", "idsSha256"

// スナップショットに追加
type ShowdownSpecies struct{ /* 既存 */ ; Prevo string }    // "prevo"
type ShowdownItem struct{ /* 既存 */ ; Hooks []string }     // "hooks"
type ShowdownAbility struct{ /* 既存 */ ; Hooks []string }  // "hooks"
// ShowdownSnapshot.Learnsets の型を変更(§7): 種族ID → 技ID → 学習した最大世代
type ShowdownSnapshot struct{ /* 既存 */ ; Learnsets map[string]map[string]int }

// RegulationDef に追加(§7。Go に mod 名や世代の数値をハードコードしないための入力)
type RegulationDef struct{ /* 既存 */ ; InheritFromPrevo bool; MinSourceGen int } // "inheritFromPrevo", "minSourceGen"

type Reconciliation struct {
    SchemaVersion int                // 1
    Sources       map[string]string  // Config.Sources の写し
    Partial       bool
    Summary       Summary
    MoveDiffs, SpeciesDiffs []ValueDiff
    VerdictChecks []VerdictCheck
    EffectCoverage EffectCoverage
    Names         map[string]NameStats
    ShowdownOnlySpecies []ShowdownOnlySpecies
    FoldedLearnsets []FoldedLearnsetDiff
    Learnsets     LearnsetStats      // {Inherited int; InheritedBySpecies map[string]int}
    Report        Report             // Convert の Warnings/Blockers + 照合の指摘
}
```

JSON のキーは camelCase のタグを付ける(`Report` / `Finding` にタグを足して `warnings` / `kind` 等にしてもよい)。

## 限界

- 値の比較は calc と Showdown の2者だけ。gen9 からの変更点の一覧(ADR-0002 §3 の「16 件」)は見ない。
- 効果定義の網羅性は「Showdown がハンドラを持つか」という近似。ハンドラがあっても条件付き(天気・状態)で engine が表せない物は
  `effect-missing` として残り続ける(件数で見る)。
- 習得技は ADR-0101 §5 の元の規則(自分の学習元、無ければ基本種を1段だけ)のまま。進化前(prevo)からは継がない
  (§7。M-C の実際の挙動)。`inheritFromPrevo: true` の経路(現状未使用)を使う場合は、学習元の種類・`learnsetDomain`
  までは見ない近似のまま(§7 の限界に記載)。
- ハッシュによる裁定の照合は、上流が同じ集合を保ったまま値だけ変えたことは見ない(値は §3 の比較と ADR-0101 §8 の Blocker が見る)。

## 人間の確認事項(既定案で進める。実データの結果で覆る場合だけ確認する)

1. **習得技の解決**(§7)。既定案は「進化前をたどるだけ(世代を見ない)」→ 実データ確認で食い違い(標本5件) →
   「案b(`minSourceGen` で世代も絞る)」を実装 → **実データで再検証してもなお標本5件が食い違い**、Showdown 本体の
   ソース(`learnsetParent`)を読んで根本原因(champions mod は進化前からの継承を世代を問わず無効化している)を特定した。
   **結論(ユーザー決定): 進化前からは継がない(ADR-0101 §5 の元の規則のまま)**。データ駆動のフラグ
   (`regulations.json` の `inheritFromPrevo`。M-C は `false`)で切り替えられるようにし、案bの実装は
   `inheritFromPrevo: true` の経路として残した。§7・§10・§12 に経緯と実データの再検証結果を記載。**解決済み**。
2. **裁定の件数・集合の食い違いで取り込みを止める**(§5)。既定: 止める(Blocker)。CronJob が止まりやすくなるが、使用可能集合の黙った変化を防ぐ方を採る。
   → §10 の実データ確認で確認済み(件数・ハッシュとも一致。下記)。

## 12. 固定した版と実データでの件数(implementer が §10 の手順の後に追記する)

**固定した版**(§10 の表のとおり。値そのものは第三者データではなく版の metadata):

| source | 版 |
|---|---|
| calc | `0.12.0`(変更なし) |
| showdown | `f10d6798f2ba5af92e55892c8c7063ca7b53c18a`(P2-1c の裁定を行った commit。短縮 `f10d679`、2026-09-20) |
| showdown mod(regulations.json の M-C) | `champions`(`config/formats.ts` の `[Gen 9 Champions] VGC 2026 Reg M-C` で確認) |
| pokeapi | `575291cdb197a7e3a320297be276c9de4ef8401a`(固定作業時点の `master` の HEAD) |

**`make import-dry-run` の結果**(`data/generated/reports/latest.json` / `latest-summary.txt`。件数だけ、個別の ID・名前は無し):

- `partial: false`、Blocker 0 件。裁定の照合3区分とも `ok: true`(件数・ハッシュとも一致。calcOnlyExcluded 11 / showdownOnlyIncluded 1 / statusTypeMismatch 1。ADR-0002 追記 P2-1c の結論どおり)。
- Summary: moves(calc 525・Showdown 938・使用可 515・両方 514・calc だけ 11・Showdown だけ 1・取り込み 515)、
  species(calc 359・Showdown 1518・使用可 392・両方 358・calc だけ 1・Showdown だけ 34・取り込み 349)、
  items(calc 166・Showdown 583・使用可 166・両方 166・取り込み 166)、abilities(calc 215・Showdown 321・使用可 316・両方 214・calc だけ 1・Showdown だけ 102・取り込み 216)。
  種族・技の calc の件数は ADR-0002 §3 の記録(種族359・技526含む番兵→525除く・持ち物166・特性215)と一致する。
- 警告の内訳: `ability-showdown-only` 1、`effect-missing` 79(持ち物35・特性44。§6 は近似で止めない指標)、`effect-unused` 5、`form-folded` 34、
  `move-excluded` 12、`move-showdown-only` 1、`move-type-mismatch` 1、`move-value-mismatch` 10(すべて priority の警告)、`name-fallback` 162、
  `species-excluded` 1、`species-mega-base-dependency` 1(使用可能なメガの基本種を依存行として取り込んだ件数)。
  `verdict-mismatch` / `verdict-basis-changed` は無し。
- `MoveDiffs` 11 件(すべて severity `warning`。type 1 件・priority 10 件で `move-type-mismatch` / `move-value-mismatch` の内訳と一致)、`SpeciesDiffs` 0 件(blocker 無し)。
- `ShowdownOnlySpecies` 33 件(`folded` 23・`num-not-in-calc` 10・`different-performance` 0・`unresolvable` 0)。
  Summary の `species.showdownOnly`(34)と1件差があるが、これは定義の違いによる想定内の差(§8 の一覧は「取り込まれず理由が付く」もの、
  Summary は「calc の種族と直接対応しない」もの。畳み込みの代表に Showdown 由来の種族がなり、その組に calc 確認済みの種族が
  含まれる場合、その代表は取り込まれるが calc と直接には対応しない=Summary では ShowdownOnly、§8 の一覧では除外理由が無いので載らない)。
  ADR-0002 §3 の「Showdown 側のみ 35(未確認)」は今回の精査で 33〜34 と判明(前回の見積りが「未確認」だったための差で、裁定のやり直しは不要)。
- `Names` の欠落(FallbackEn): species 119 / moves 0 / items 40 / abilities 3 / types 0(件数のみ。個別 ID は `data/generated/` にだけ出る)。
- `Learnsets.Inherited`(経緯): 案a相当 1861 行(251 種族)→ 案b(世代で絞る)実装後 42 行(35 種族)→
  **最終(進化前から継がない。§7 採用)実装後は 0 行**(`inheritFromPrevo: false` では常に 0。実データで確認済み)。

**未検証点の確認結果**(§10 の手順3):

- PS のビルド: `npm ci --omit=dev && node build` は失敗せず成功した(スクリプトの変更不要だった)。ただし `dist/sim/dex.js` の動的 import の
  named export の見え方が版によって違ったため、`fetch-showdown.mjs` に `default` / `module.exports` を両方受ける処理を足した。
- `Dex.mod('champions')`: 種族 1518 件(標準 392 件。ADR-0002 §3 の 392 と一致)、技 954 件(標準 515 件。525 と共通 514 件も一致)。
- `dex.learnsets.get` は非推奨気味で、進化前を含む全種族を対象にするため `dex.species.getLearnsetData` に切り替えた(全種族分の learnsets を出す。ADR-0101 §3 の更新どおり)。
- `hooks`(`on` で始まる関数プロパティ名)は取り込む持ち物・特性のうち相当数(持ち物42件・特性49件)がダメージに効くハンドラを持つ一方、
  対応する効果定義(`data/importer/effects.json`)がまだ無いもの(`effect-missing` 79 件)が多い。v1 の既知の未整備として件数で残す(§6 は止めない設計どおり)。
- `species.prevo` は名前で正しく入っていた(取得できた)。
- PokeAPI の CSV の列名は ADR-0101 §3 の想定どおり(`languages.csv` の `identifier`、`pokemon_form_names.csv` の `pokemon_name`、
  各 `*_names.csv` の `local_language_id` / `name`)で、列名の変更は不要だった。
- **習得技の解決の標本突き合わせ(PS の `TeamValidator.checkCanLearn`)**: 3回行った。
  1. 進化前をたどるだけ(案a): 5件中5件が食い違った(Go は学習可、PS は学習不可)。
  2. `minSourceGen` で世代も絞る(案b): 新たな標本5件を取ったが、依然5件中5件が食い違った。Showdown 本体のソース
     (`dex-species.js` の `learnsetParent`)を読んで根本原因(champions mod は進化前からの継承を世代を問わず無効化)を特定。
  3. **進化前から継がない(§7 採用)に直した後**: 案1・案2で食い違った標本5件と、
     自分の学習元だけの新しい標本5件の、あわせて **10件で PS の `checkCanLearn` と全件一致**した。**解決済み**。

**実データで判明し、規則を直した点**(ADR-0101 §5 に追記済み。ADR-0103 の範囲内):

- 特性スロット `"S"` が `"H"` と同じ種族に実在した → `species_abilities.slot` を 1..4 の4枠に広げ、`"S"` を slot 4 に写す
  (DB のマイグレーション・`master.Species` の検証・importer の写像を変更。ADR-0101 §5 参照)。
- 使用可能なメガの基本種が Showdown で `isNonstandard: "Past"`(calc に対応が無い)場合がある → その基本種を
  `base_species_key` の外部キーを満たすためだけの依存行として取り込み、レギュレーションの使用可能集合には入れない。
- フォームを持たない基本種で Showdown が `formeOrder` を省略することがある → その場合は `form = 0` を許す。
- 固定した calc 0.12.0 の Champions 世代に `Stellar` 型が無かった → `config.json` の `excludeTypes` から `Stellar` を外した
  (残すと calc に無いタイプ名として `ErrInvalidData` になり止まっていた。除外が要るのは `???` だけ。ADR-0101 §5 参照)。
- `species_abilities.slot` の CHECK(1..4)は、main に取り込み済みの `000002_create_master.up.sql` を書き換えず、
  `000005_widen_species_ability_slot.up/down.sql` で ALTER TABLE により広げた(ADR-0100 参照)。
