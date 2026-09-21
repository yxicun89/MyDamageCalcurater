# ADR-0009: 一括計算の防御側プリセット定義と行の構成

- 状態: 採用(2026-09-21 改訂。プリセット定義はユーザー決定により**確定**。人間の確認待ちは解消)
- 日付: 2026-09-21(初版)/ 2026-09-21 改訂(P1-10)
- 関連: P1-7、P1-10、docs/requirements.md「相手側の一括表示」、api/openapi.yaml(`/api/calc/bulk`、`DefenderPreset`)、
  ADR-0002 §確定した方針、ADR-0005(データ駆動の効果定義)、ADR-0010(逆算の型ラベル)、
  docs/ai-shared/DECISIONS.md 2026-09-21「マスタデータ方針・防御プリセット・逆算・表示%のユーザー決定」、
  CLAUDE.md 絶対ルール1(API は openapi.yaml から)・2(engine は純粋)・6(テストを緩めない)・ドメイン規約(ハードコード禁止)

## 背景

要件は「相手側は入力させず、防御側の代表的な調整を並べて一括表示する」ことを求める。
対象は「無振り / H振り / HB振り(またはHD振り。技の分類で自動切替)/ H振り+B(D)補正 / HB特化(またはHD特化)」で、
持ち物の差し替え候補もトグルで比較できること。

初版では OpenAPI の enum `[none, hp, hb, hd, hb_boost, hd_boost]` に合わせ、`hb` / `hd` を
「H・B(D) を振り切り、さらに B(D) 上昇性格を掛けた最大耐久」と定義していた。
これは実戦の呼び名としては「HB特化」であって「HB振り」ではなく、**「性格補正なしで振り切っただけの型」を表す行が無かった**。
逆算(ADR-0010)は最初から「補正なし」と「上昇性格」を別の型として区別しているため、一括表示だけが両者を畳んでいる状態だった。

2026-09-21 のユーザー決定でこのずれを解消し、`hb` / `hd` を**補正なしの振り切り**に付け替え、
**最大耐久を `hb_full` / `hd_full` として新設**した。`hb_boost` / `hd_boost` は初版の仮定どおりで確定した(§2)。
本 ADR の以下の記述は改訂後の定義で、初版との差分は末尾の「変更履歴」にまとめる。

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

既定カタログ(8件。順序も規定。耐久が上がる順に並べる):

| 順 | Key | Label | SP | Nature | Applies |
|---|---|---|---|---|---|
| 0 | `none` | 無振り | なし | 無補正 | 全分類 |
| 1 | `hp` | H振り | hp:32 | 無補正 | 全分類 |
| 2 | `hb_boost` | H振り+B補正 | hp:32 | +def / -atk | 物理 |
| 3 | `hb` | HB振り | hp:32, def:32 | 無補正 | 物理 |
| 4 | `hb_full` | HB特化 | hp:32, def:32 | +def / -atk | 物理 |
| 5 | `hd_boost` | H振り+D補正 | hp:32 | +spd / -atk | 特殊 |
| 6 | `hd` | HD振り | hp:32, spd:32 | 無補正 | 特殊 |
| 7 | `hd_full` | HD特化 | hp:32, spd:32 | +spd / -atk | 特殊 |

上昇性格は `Nature{Plus: StatDef, Minus: StatAtk}` / `Nature{Plus: StatSpD, Minus: StatAtk}`(@smogon/calc の Bold / Calm)。
下降補正を `atk` に置くのは golden に合わせたもので、防御側の被ダメージ計算には影響しない。
`hb` / `hd` / `none` / `hp` の性格は無補正(Serious 相当。`Nature` のゼロ値)。

#### ADR-0010(逆算)の型ラベルとの対応

逆算は「H / B(D) の振り分け × 性格クラス」の全組合せを型(Archetype)として持つ。本カタログの8件は
その部分集合であり、**同じ調整は同じ呼び名**になっていなければならない。改訂後の対応:

| 本カタログ | ADR-0010 §5.3 のキー | ADR-0010 のラベル |
|---|---|---|
| `none` | `hnone-bnone-neutral` | 無振り |
| `hp` | `hfull-bnone-neutral` | H振り |
| `hb_boost` | `hfull-bnone-plus` | H振り+B補正 |
| `hb` | `hfull-bfull-neutral` | HB振り(無補正) |
| `hb_full` | `hfull-bfull-plus` | HB特化 |

初版では `hb`(= H32・B32・上昇性格)が「HB特化」を名乗り、ADR-0010 の `hfull-bfull-plus`(HB特化)と
同じ調整を別のキーが指していた。改訂後は `hb_full` が `hfull-bfull-plus` に対応し、
`hb` は `hfull-bfull-neutral` に対応する。**ラベルの食い違いは解消した**。
`hb` のラベルだけは要件書の表記に合わせて「HB振り」とし、ADR-0010 の「HB振り(無補正)」と括弧書きの有無で異なるが、
指す調整は同一である(逆算の表示は ADR-0010 側のラベルを使い、一括表示は本カタログの Label を使う)。
**ADR-0010 そのものは本改訂では変更しない**。逆算の再設計は P1-12 で別に行う。

**2026-09-21(P1-12)追記**: 逆算は上記の型(Archetype)自体を廃止した(ADR-0010 §R6)。
`ArchetypeOf` / `ReverseArchetypes` は削除され、逆算の結果は「性格クラス × 持ち物」ごとの
SP 範囲(`Ranges`)になった(ADR-0010 §R3)。したがって上表の「ADR-0010 §5.3 のキー」列は
**現在の逆算 API には存在しない**。この節は「本カタログのプリセットが H32 前提の探索空間
(ADR-0010 §R1: H=32 固定・B/D=0..32・性格は無補正/上昇の2通り)に収まっている」ことを示す
歴史的な対応表として残し、その被覆は `engine/bulk_test.go` の `TestDefenderPresetsInsideReverseSpace`
が固定する(旧 `ArchetypeOf` を使ったテストの置き換え)。

プリセットから作る防御側個体は次の形に固定する(ゴールデンのフィクスチャと一致させるため明示する):

- `Species` = 指定された防御側種族、`Level` = 50(`DefaultLevel`)、`Status` = `none`、`Ranks` = すべて 0
- `Ability` = ゼロ値(特性なし。特性込みの比較は M2 以降の拡張)
- `Item` = 後述の持ち物バリアント(無しは nil)

### 1-a. `Label`(表示名)の扱い(2026-09-21 ユーザー決定)

カタログの `Label`(日本語の表示名)を engine が持つのは、「画面に表示されていればよく、engine が持っている方が扱いやすいなら持ってよい」という判断による
(コーディング規約 §2「表示文言はクライアントの文言資源に」の例外として扱う)。`Label` は**既定の表示名**で、識別には使わない(識別は `Key`)。
クライアントは自分の文言資源で上書きしてよい。表示名が不要になった場合は、`Label` を外してクライアント側に移してよい(`Key` は変えない)。

### 2. `hb_boost` / `hd_boost` の意味(**確定**)と耐久の単調性

要件の「H振り+B(D)補正」は、**HP に 32 SP を振り、防御(特防)には SP を振らず、性格の上昇補正だけを掛けた型**。
すなわち `hb_boost` = SP{hp:32} + 性格 +def/-atk、`hd_boost` = SP{hp:32} + 性格 +spd/-atk。
初版ではこれを仮定として人間の確認待ちにしていたが、2026-09-21 のユーザー決定で**そのまま確定**した。
初版が挙げていた別解釈(「B に余り SP を全振り + 補正」= `hb_full`、「B に中途半端な量を振る」)は採らない。

#### 耐久の単調性(カタログ順が「耐久が上がる順」であることの根拠)

被ダメージは防御実数値だけで決まる(HP はダメージ量に影響しない)。engine の実数値は
`floor((種族値 + 20 + SP) × 性格補正)`、性格補正は整数演算 `×11/10`(ADR-0004、`engine/stats.go`)。
防御側の種族値を `B`、`D0 = B + 20` と置くと、同じ分類の中での防御実数値は:

| Key | 防御実数値 |
|---|---|
| `none` / `hp` | `D0` |
| `hb_boost` | `floor(11·D0/10)` |
| `hb` | `D0 + 32` |
| `hb_full` | `floor(11·(D0+32)/10)` |

順序が崩れうるのは `hb_boost` と `hb` の境目だけで、
`floor(11·D0/10) ≤ D0 + 32 ⟺ D0 ≤ 329 ⟺ B ≤ 309`、
厳密不等号 `floor(11·D0/10) < D0 + 32` は `D0 < 320 ⟺ B ≤ 299` で成り立つ。
**種族値は仕様上 255 が上限**(1バイト値。実在最大は B=230 のツボツボ)なので、`B ≤ 299` は全種族で常に真。
したがって上の表の順序は**全種族で厳密な単調減少**であり、単調性検査は
「同じ分類の中でダメージが厳密に減る」形で書ける。検査を `>=` に緩める必要はない(絶対ルール6)。

ただし **`>` だけを書くと成立条件を見失う**ため、テストは次の2段構えにする:

1. 代表フィクスチャ(1種族)で `none == hp > hb_boost > hb > hb_full`(`hd` 系も同様)を検査する
2. 種族値 `B ∈ 1..255` を総当たりし、同じ厳密順序が全域で成り立つことを検査する
   (`B = 299 / 300` 付近の境界も併せて固定し、「255 上限だから成り立つ」という根拠を
   テスト側にも残す。上限を超える値で崩れることは検査せず、崩れる閾値だけをコメントで示す)

`none` と `hp` は防御実数値が等しいのでダメージは**等しい**(HP だけが増える)。
ここは `>=` ではなく `==` で固定する。

### 2-a. 定義が変わるときに同時に直すもの

本カタログを変えたら、次をすべて同じコミットで更新する(片方だけ変えるとゴールデンが落ちる):

- `engine/bulk.go` の `DefenderPresetCatalog()` / `DefaultDefenderPresets()` / `PresetKey` 定数
- `engine/bulk_test.go` の `TestDefenderPresetCatalogDefinitions` / `TestDefaultDefenderPresetsByCategory` / 単調性検査
- `tools/golden/generate.mjs` の防御側網羅ベクタ(§6)と `engine/bulk_golden_test.go` の件数・対応表
- `api/openapi.yaml` の `DefenderPreset` enum と description(`make gen`)
- `engine/wasmapi/testdata/vectors.json` の bulk ベクタ
- `docs/test-strategy.md` の防御側網羅(プリセット数と件数)、`docs/adr/0011-wasm-boundary.md` の JSON 例、`api/openapi.yaml` の `/api/calc/bulk` の説明文、`docs/design.md` の結果行のモック
- 将来 calc-svc(P3-1)では、`DefenderPreset` enum と engine の `PresetKey` の対応を検査するテストを置く(現在は両者の一致を自動で守るテストがない)

期待値の変更理由はコミットメッセージに書く。テストを緩めて通す、はしない(絶対ルール6)。

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

- 物理技: `none, hp, hb_boost, hb, hb_full`(5件)
- 特殊技: `none, hp, hd_boost, hd, hd_full`(5件)
- 変化技(`status`)および分類が空・未知の値: `none, hp` の2件のみ

既定セットはカタログから `Applies` で絞ったものであり、カタログ順を保つ。
改訂で物理・特殊の既定セットが 4 件から **5 件**に増えた(`hb_full` / `hd_full` の新設分)。

変化技は B / D のどちらの耐久も結果に影響しないため、B 系・D 系を並べても意味がない。
0ダメージの行が5つ並ぶより2つに畳む。行を返さない(空)のではなく `none, hp` を返すのは、
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

### 6. ゴールデン照合はカタログ8件すべてを覆う

初版では外部照合(@smogon/calc)されていたのはカタログ6件のうち4件(`none/hp/hb/hd`、ただし当時の
`hb`/`hd` は上昇性格込みの定義)だけで、`hb_boost` / `hd_boost` には対応ベクタが無かった。

改訂後のカタログは**8件すべてが「SP と性格だけ」で定義される**ので、すべて外部実装で照合できる。
`tools/golden/generate.mjs` の防御側網羅ベクタを4種から8種へ拡張する:

| golden のベクタ名 | SP | @smogon/calc の nature | 備考 |
|---|---|---|---|
| `none` | なし | Serious | 旧 `zero` |
| `hp` | hp:32 | Serious | 旧 `h` |
| `hb_boost` | hp:32 | Bold | 新規 |
| `hb` | hp:32, def:32 | Serious | 新規(旧 `hb` とは別物) |
| `hb_full` | hp:32, def:32 | Bold | 旧 `hb` に相当 |
| `hd_boost` | hp:32 | Calm | 新規 |
| `hd` | hp:32, spd:32 | Serious | 新規(旧 `hd` とは別物) |
| `hd_full` | hp:32, spd:32 | Calm | 旧 `hd` に相当 |

- **ベクタ名は engine の `PresetKey` と同一文字列にする**。初版の `zero`/`h` という別名は、
  対応表を1つ挟むぶん取り違えの余地があった。名前が一致していれば対応表は「存在検査」で済む。
- 1グループ(種族 × 攻撃側アンカー)の件数は 4 → **8**。`defense-species.jsonl.gz` の総件数は
  `speciesCount × 5(アンカー) × 4` = `speciesCount × 20` から **`speciesCount × 40`** になる。
  `engine/golden_test.go` と `engine/bulk_golden_test.go` の件数期待値を同時に更新する。
- SP → 努力値の換算 `max(0, 8×SP−4)` は変えない(SP32 → EV252)。
- **「外部照合されているのはどのプリセットか」をテストで固定する**。
  改訂後は「カタログの全キーが golden のベクタ名として存在すること」を検査でき、
  外部照合されないプリセットが将来こっそり増えることを防げる。
- 再生成(`make golden-generate`)と `metadata.json` の更新は実装時に行う。
  期待値・件数が変わる理由(本 ADR のプリセット再定義)はコミットメッセージに書く(絶対ルール6)。
- oracle を `@smogon/calc@0.12.0` Champions へ切り替えるのは **P2-1b の別タスク**で、本改訂では 0.10.0 / gen9 のまま。

## 却下・保留

- **プリセット定義を calc-svc がマスタ DB から読む形**: WASM オフライン(P4-5)で同じ定義を別経路で持つ必要があり、
  二重管理になる。将来必要になれば `Presets` 引数から注入できるので、いま採用する理由が無い。
- **行ごとの部分エラー**: 比較表としての性質から全体エラーにする。逆算(P1-8)で候補ごとの成否が必要になったら再検討。
- **特性込みのプリセット**(例: 防御側「しんかのきせき」「あついしぼう」): M1 では持ち物バリアントのみ。
  `Ability` を `DefenderPreset` に足せば拡張できる構造にしてある。
- **ダブル固有補正**: `Format` は `DamageInput` にそのまま渡すだけで、一括計算側では何もしない(ADR-0005 の未対応範囲)。

## 影響

- `engine/bulk.go` に実装。`CalcBulk` は `CalcDamage` の合成にすぎず、独自のダメージ計算をしてはならない
  (テストで各行と `CalcDamage` の完全一致を検証する)。**ダメージ計算そのもの(`damage.go` / `stats.go`)は
  本改訂で変更しない**。変わるのはカタログ(SP・性格・ラベル・既定セット)と、それに依存する契約・テストだけ。
- **`api/openapi.yaml` の変更が要る**(初版の「変更は不要」は撤回)。絶対ルール1に従い openapi.yaml を先に直して `make gen`:
  - `DefenderPreset` enum に `hb_full` / `hd_full` を追加し、`[none, hp, hb_boost, hb, hb_full, hd_boost, hd, hd_full]`
    とカタログ順で並べる(enum の並びは生成コードの定数順にしか影響しないが、カタログ順と一致させて読み違いを防ぐ)。
  - `DefenderPreset` の description に各キーの SP・性格を書く(定義がどこにも書かれていない、という初版の問題の再発防止)。
  - `BulkCalcRequest.presets` の description を「省略時は技の分類に応じた既定セット
    (物理 5 件 / 特殊 5 件 / 変化技 2 件)」に更新する。
  - `BulkCalcRow.presetLabel` の例を「HB特化」から、新定義で誤解の無い表記へ直す。
- API の `presets`(`DefenderPreset` enum の配列)は engine の `PresetKeys` に対応する。engine の `Presets`(完全定義)は
  API には現れない。calc-svc が enum を engine のカタログ / `PresetKeys` に解決する(P3-1)。
- API の `presets: []`(空配列)と省略は engine では区別されず(`PresetKeys` の長さ 0)、どちらも技の分類に応じた既定セットになる。
- **P3-1(calc-svc の `/api/calc/bulk`)への持ち越し**: 既定セットが 4 → 5 件になったので、
  行数を 4 前提で書いた実装・テスト・画面を作らないこと。enum → `PresetKey` の変換は
  `hb_full` / `hd_full` を含む8件を扱う。docs/plan.md の P3-1 の `/api/calc/bulk` 項目もこの前提で読むこと
  (plan.md 本体の更新は P1-10 のコミットでは行わない)。
- `tools/golden/generate.mjs` の防御側プリセット定義は本 ADR のカタログと同じ値でなければならない。
  片方を変えたらもう片方も変える(`engine/bulk_golden_test.go` が検出する)。§6 を参照。
- `engine/wasmapi`(ADR-0011): 境界は `presetKeys` を列挙検証せず engine の sentinel に委ねているので、
  `hb_full` / `hd_full` は**境界側の変更なしで通る**はず。これを `testdata/vectors.json` の bulk ベクタで固定する
  (ADR-0011 §7 のとおりネイティブ側の期待値はコミットせず、engine から都度生成して照合する)。
- Web / iOS のクライアント型は openapi から生成されるため、`make gen` 後に enum が増える。
  行数を固定値で持っている箇所があれば直す(M1 時点では未実装)。

## 変更履歴

### 2026-09-21 改訂(P1-10): 防御プリセットの再定義

ユーザー決定(docs/ai-shared/DECISIONS.md 2026-09-21 最終エントリ、ADR-0002 §確定した方針、
docs/requirements.md「相手側の一括表示」)により、カタログを 6 件から 8 件に再定義した。

| Key | 初版の定義 | 改訂後の定義 | 変化 |
|---|---|---|---|
| `none` | SP なし・無補正 | 同じ | 不変 |
| `hp` | hp:32・無補正 | 同じ | 不変 |
| `hb` | hp:32, def:32・**+def/-atk**(ラベル「HB特化」) | hp:32, def:32・**無補正**(ラベル「HB振り」) | **変更**(性格補正を外した) |
| `hd` | hp:32, spd:32・**+spd/-atk**(ラベル「HD特化」) | hp:32, spd:32・**無補正**(ラベル「HD振り」) | **変更**(性格補正を外した) |
| `hb_boost` | hp:32・+def/-atk(**仮定**・人間の確認待ち) | 同じ(**確定**) | 定義は不変、状態が確定に |
| `hd_boost` | hp:32・+spd/-atk(**仮定**・人間の確認待ち) | 同じ(**確定**) | 定義は不変、状態が確定に |
| `hb_full` | — | hp:32, def:32・+def/-atk(ラベル「HB特化」) | **新設**(初版の `hb` がここへ移った) |
| `hd_full` | — | hp:32, spd:32・+spd/-atk(ラベル「HD特化」) | **新設**(初版の `hd` がここへ移った) |

要点:

- **初版の `hb` / `hd`(性格補正込みの最大耐久)は消えていない。キーが `hb_full` / `hd_full` に移った**。
  同じ名前のキーが別の調整を指すようになったので、`hb` / `hd` を含む保存済みデータ・ベクタ・
  画面の期待値は読み替えが必要になる(M1 時点では永続化していないため影響は engine とテストに閉じる)。
- 「性格補正なしで振り切っただけ」の型が新たに表現できるようになり、逆算(ADR-0010 §5.3 の
  `hfull-bfull-neutral` / `hfull-bfull-plus`)と一括表示のラベルが1対1で対応するようになった。
- `hb_boost` / `hd_boost` の人間の確認待ち(初版 §2)は**解消**。docs/plan.md ブロッカー節の
  該当項目もこの改訂で閉じられる。
- ゴールデン照合の範囲が 4 プリセット → **8 プリセット(カタログ全件)**に広がった(§6)。
- `api/openapi.yaml` の `DefenderPreset` enum に `hb_full` / `hd_full` を追加する必要が生じた。
  初版「`api/openapi.yaml` の変更は不要」は撤回(§影響)。
