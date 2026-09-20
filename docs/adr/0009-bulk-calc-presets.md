# ADR-0009: 一括計算の防御側プリセット定義と行の構成

- 状態: 採用(ただし §2 の `hb_boost` / `hd_boost` の定義は**人間の確認が必要**)
- 日付: 2026-09-21
- 関連: P1-7、docs/requirements.md「相手側の一括表示」、api/openapi.yaml(`/api/calc/bulk`、`DefenderPreset`)、
  ADR-0005(データ駆動の効果定義)、CLAUDE.md 絶対ルール2(engine は純粋)・ドメイン規約(ハードコード禁止)

## 背景

要件は「相手側は入力させず、防御側の代表的な調整を並べて一括表示する」ことを求める。
対象は「無振り / H振り / HB特化(またはHD特化。技の分類で自動切替)/ H振り+B(D)補正」で、
持ち物の差し替え候補もトグルで比較できること。

OpenAPI は `DefenderPreset` を `[none, hp, hb, hd, hb_boost, hd_boost]` の enum として既に定義しているが、
**各プリセットの SP 配分・性格はどの文書にも定義が無い**。唯一の既存の実体は
ゴールデン生成器 `tools/golden/generate.mjs` の防御側網羅ベクタで、次の4つを使っている
(`make test-golden` 全件一致済み、すなわち外部実装 @smogon/calc と照合済みの定義):

| golden の名前 | SP | 性格 |
|---|---|---|
| `zero` | なし | 無補正(Serious) |
| `h` | hp:32 | 無補正 |
| `hb` | hp:32, def:32 | Bold(+def / -atk) |
| `hd` | hp:32, spd:32 | Calm(+spd / -atk) |

一括計算の呼び出し側は **WASM(バックエンド無しのブラウザ、P1-9/P4-5)と calc-svc(P3-1)の2つ**で、
どちらでも同じ結果にならなければならない。

## 決定

### 1. プリセット定義は「引数で受け取る」。engine は既定値を純粋関数として持つ

`CalcBulk` は防御側プリセットを **データ(`[]DefenderPreset`)として引数で受け取る**。
`DefenderPreset` は `Key / Label / SP(Stats) / Nature(Plus,Minus) / Applies(物理・特殊・全分類)` だけを持ち、
性格 ID や持ち物 ID などマスタのキーを含まない構造値とする。

同時に engine は既定カタログを **純粋関数** `DefenderPresetCatalog()` /
`DefaultDefenderPresets(category)` として公開し、`Presets` 未指定時はこれを使う。

根拠:

- **ハードコード規約に反しない**。禁止されているのは「持ち物・技・ポケモンのリスト」= マスタデータの複製である。
  プリセットは SP の配り方という *入力の作り方の型* であり、対応するマスタテーブルは存在しない(P2 のスキーマにも無い)。
  engine は依然として持ち物・特性の効果値を `ItemEffect` / `AbilityEffect` として外から受け取る(ADR-0005)。
- **WASM と calc-svc で結果が一致する**。既定値を calc-svc がマスタ DB から読む形にすると、
  オフライン WASM は同じ定義を別経路(別のデータファイル)で持つことになり、二重管理と不一致の温床になる。
  engine 側に1つ持てば、両方の呼び出し側が同じ既定で動く。
- **上書き可能なので将来の変更に耐える**。P2 でチャンピオンズの SP 規則が変わった、あるいは
  プリセットをマスタ管理したくなった場合は、calc-svc が `Presets` に定義を詰めて渡せばよい。
  engine 側の変更は不要で、既定値はフォールバックとして残る。
- 既定値の置き場は **engine のコード(`engine/bulk.go` の純粋関数)**とし、データファイルは作らない。
  外部ファイルにすると WASM に同梱する仕組みが別途必要になり、純粋性の利点が消えるため。

既定カタログ(順序も規定。耐久が上がる順に並べる):

| Key | Label | SP | Nature | Applies |
|---|---|---|---|---|
| `none` | 無振り | なし | 無補正 | 全分類 |
| `hp` | H振り | hp:32 | 無補正 | 全分類 |
| `hb_boost` | H振り+B補正 | hp:32 | +def / -atk | 物理 |
| `hb` | HB特化 | hp:32, def:32 | +def / -atk | 物理 |
| `hd_boost` | H振り+D補正 | hp:32 | +spd / -atk | 特殊 |
| `hd` | HD特化 | hp:32, spd:32 | +spd / -atk | 特殊 |

`none / hp / hb / hd` の SP と性格は上表のとおり golden の定義と一致させる(P1-6 で外部実装と照合済みのため)。
下降補正を `atk` に置くのは golden(Bold / Calm)に合わせたもので、防御側の被ダメージ計算には影響しない。

プリセットから作る防御側個体は次の形に固定する(ゴールデンのフィクスチャと一致させるため明示する):

- `Species` = 指定された防御側種族、`Level` = 50(`DefaultLevel`)、`Status` = `none`、`Ranks` = すべて 0
- `Ability` = ゼロ値(特性なし。特性込みの比較は M2 以降の拡張)
- `Item` = 後述の持ち物バリアント(無しは nil)

### 2. `hb_boost` / `hd_boost` の意味(**人間の確認が必要**)

要件の「H振り+B(D)補正」を、**HP に 32 SP を振り、防御(特防)には SP を振らず、性格の上昇補正だけを掛けた型**
と解釈する。すなわち `hb_boost` = SP{hp:32} + 性格 +def/-atk、`hd_boost` = SP{hp:32} + 性格 +spd/-atk。

根拠: 「H振り」+「B(D)補正」という語の素直な読みであり、`hp`(補正なし)と `hb`(振り切り+補正)の
中間の耐久を表す型として意味がある。SP 合計は 32 で上限 66 以内、性格補正は HP 以外に掛かるので規約にも反しない。

ただし次の別解釈も成立しうるため、**この定義は仮定であり、人間の確認を受けるまで確定扱いにしない**
(CLAUDE.md「人間の確認が必要なこと」に準じる):

- 「H振り + B に余り SP(66-32=34 → 上限32)を全振り + 補正」= 実質 `hb` と同じ
- 「H振り + B に中途半端な量(例: 16)を振って補正」= 調整先の値が決まらないと定まらない

確認が取れて定義が変わる場合は、本 ADR と `engine/bulk_test.go` の
`TestDefenderPresetCatalogDefinitions` を同時に更新する(期待値の変更理由はコミットメッセージに書く)。
テストを緩めて両方の解釈を通す、はしない(絶対ルール6)。

### 3. プリセットの選択元・`presets` 省略時の既定セットと変化技の扱い

使用するプリセットは次の優先順で決める(`selectPresets`)。

| `Presets` | `PresetKeys` | 使用するプリセット |
|---|---|---|
| 空 | 空 | `DefaultDefenderPresets(Move.Category)`(下記の既定セット) |
| 空 | あり | 既定カタログ `DefenderPresetCatalog()` から `PresetKeys` の順に選ぶ |
| あり | 空 | `Presets` をそのまま(定義の順) |
| あり | あり | **`Presets` を検索元にして** `PresetKeys` の順に選ぶ(カタログは見ない) |

- `PresetKeys` と `Presets` の両方が与えられたら **`Presets` を検索元にする**。カタログに同じキーがあっても
  `Presets` の定義が使われ、`Presets` に無いキーはカタログにあっても `ErrUnknownPreset`(カタログへフォールバックしない)。
  `Presets` に含まれても `PresetKeys` で選ばれなかった定義は行にならない。
- **`PresetKeys` のみなら既定カタログから選ぶ**。
- 検索元(`Presets`、または既定カタログ)の中でキーが重複していれば、`PresetKeys` が重複していない場合でも
  `ErrDuplicatePreset`(§5)。
- 行の順序は `PresetKeys` を指定したときは `PresetKeys` の順、指定しないときは `Presets` / 既定セットの順。

`Presets` も `PresetKeys` も空のとき、`DefaultDefenderPresets(Move.Category)` を使う。

- 物理技: `none, hp, hb_boost, hb`
- 特殊技: `none, hp, hd_boost, hd`
- 変化技(`status`)および分類が空・未知の値: `none, hp` の2件のみ

変化技は B / D のどちらの耐久も結果に影響しないため、B 系・D 系を並べても意味がない。
0ダメージの行が4つ並ぶより2つに畳む。行を返さない(空)のではなく `none, hp` を返すのは、
画面が「行が無い」状態を特別扱いしなくて済むようにするため。

### 4. 持ち物バリアントと行の順序

- `ItemVariants []*Item`。**nil または空のときは「素の1通り」**(`[]*Item{nil}` と同じ、持ち物なし)。
- 持ち物の解決(ID → `ItemEffect`)は呼び出し側(calc-svc / WASM のブリッジ)が行い、
  engine は解決済みの `*Item` を受け取ってそのまま防御側個体に載せる。engine に持ち物一覧は持ち込まない(ADR-0005)。
- 行の順序は **プリセット優先(preset-major)**: `for preset { for item { row } }`。
  行数は `len(presets) × len(itemVariants)`。同じ調整の持ち物違いが隣り合い、画面のトグル比較に素直に対応する。
- 行は渡された `*Item` のポインタをそのまま保持し、`ItemID` には `Item.ID`(nil なら空文字)を入れる。
  OpenAPI の `BulkCalcRow.itemId`(nullable)に素直に写せる。

### 5. 不正入力はエラーにする(部分成功にしない)

一括計算は「同じ攻撃に対する比較表」であり、一部の行だけ欠けた表は読み手を誤らせる。
次はいずれも `CalcBulk` 全体をエラーにする(行ごとのエラーフィールドは持たない):

| 条件 | エラー |
|---|---|
| `PresetKeys` に定義に無いキー | `ErrUnknownPreset` |
| プリセットのキーが重複(`Presets` 内 / `PresetKeys` 内) | `ErrDuplicatePreset` |
| プリセットの SP が 1ステータス 32 超 / 負 / 合計 66 超 | `ErrInvalidPreset` |
| プリセットの性格補正が HP を指す / `Key` が空 | `ErrInvalidPreset` |
| 攻撃側個体・防御側種族が不正(`Individual.Validate` 相当) | `CalcDamage` と同じエラー |

`Label` が空のときはエラーにせず `Key` を表示名として使う(表示の都合であり入力の誤りではないため)。
重複を許さないのは、行の順序と同一性(「この調整の行」)が一意でないと画面・逆算・テストが破綻するため。

## 却下・保留

- **プリセット定義を calc-svc がマスタ DB から読む形**: WASM オフライン(P4-5)で同じ定義を別経路で持つ必要があり、
  二重管理になる。将来必要になれば `Presets` 引数から注入できるので、いま採用する理由が無い。
- **行ごとの部分エラー**: 比較表としての性質から全体エラーにする。逆算(P1-8)で候補ごとの成否が必要になったら再検討。
- **特性込みのプリセット**(例: 防御側「しんかのきせき」「あついしぼう」): M1 では持ち物バリアントのみ。
  `Ability` を `DefenderPreset` に足せば拡張できる構造にしてある。
- **ダブル固有補正**: `Format` は `DamageInput` にそのまま渡すだけで、一括計算側では何もしない(ADR-0005 の未対応範囲)。

## 影響

- `engine/bulk.go` に実装。`CalcBulk` は `CalcDamage` の合成にすぎず、独自のダメージ計算をしてはならない
  (テストで各行と `CalcDamage` の完全一致を検証する)。
- `api/openapi.yaml` の変更は不要。既存の `DefenderPreset` enum・`BulkCalcRequest` / `BulkCalcRow` で表現できる。
  P3-1 で calc-svc を作るときに、enum → `DefenderPreset` 定義の対応(engine のカタログ)と行の順序を実装する。
- API の `presets`(`DefenderPreset` enum の配列)は engine の `PresetKeys` に対応する。engine の `Presets`(完全定義)は
  API には現れない。calc-svc が enum を engine のカタログ / `PresetKeys` に解決する(P3-1)。
- API の `presets: []`(空配列)と省略は engine では区別されず(`PresetKeys` の長さ 0)、どちらも技の分類に応じた既定セットになる。
- `tools/golden/generate.mjs` の防御側プリセット定義(`zero/h/hb/hd`)は本 ADR のカタログと同じ値でなければならない。
  片方を変えたらもう片方も変える(`engine/bulk_golden_test.go` が検出する)。
- `hb_boost` / `hd_boost` はゴールデンに対応ベクタが無い。定義が確定したら
  `tools/golden/generate.mjs` に追加して外部照合する(P2-1 の再生成に合わせる)。
