# ADR-0222: ダブルの計算(壁・全体技)を engine に入れる(issue #232 案B のダブル分・#288)

- 状態: 提案(spec 段階。テストと期待値まで。実装・critic は後続)
- 日付: 2026-10-03
- 関連: ADR-0002(ゴールデンの oracle。追記 P2-1b で Champions 世代へ)、ADR-0004(補正の適用位置)、
  ADR-0005(補正の対象範囲。「ダブル固有補正は未対応」を本 ADR で置き換える)、ADR-0011(WASM 境界)、
  ADR-0123・ADR-0215(未対応の印。target・reason は enum にしない)、ADR-0160(PR #497。テラスタイプと
  format=double の未対応の印)、ADR-0200(calc-svc の契約)、issue #232・#288(技の対象のマスタ化はデータレーン)

## 背景

engine は `Format` を読まず、ダブルでもシングルの数値を返していた(issue #232・#288)。
ユーザー決定(2026-10-03):

- **ポケモンチャンピオンズにテラスタルは無い**。teraType は計算に使わない(issue #232 案B のテラス部分は取り下げ)
- PR #497(ADR-0160。teraType・format=double に「未対応の印」を付け、数値は変えない)を先にマージし、
  その上にダブルの計算を載せる

計算は 4096 基準の固定小数・五捨五超入のまま、ゴールデン(@smogon/calc 0.12.0 の Champions 世代
`Generations.get(0)`)と全件一致させる。known_diffs は足さない。

## 1. テラスタル

ポケモンチャンピオンズにテラスタルは無い(ユーザー確認 2026-10-03)。teraType は計算に使わない。
API・WASM が teraType を受け取ったときの扱いは #497(ADR-0160)の未対応の印(target `attacker_tera_type` /
`defender_tera_type`)に従う。その扱いの見直し(黙って無視する・400 で拒否する・印を消す)は §4 の未決事項。
本 ADR のテスト・ゴールデンはテラスを持たない(ダブルのベクタもテラス無し)。

参考(実装しない): oracle の Champions 世代は teraType を指定すると、攻撃側のタイプ一致(テラス = 元のタイプ → ×2.0、
テラスだけ一致 → ×1.5、てきおうりょくは ×2.25 / ×2.0、テラス≠元タイプの技は ×1.5)と、タイプを持つかの判定
(接地・サイコフィールドの先制技・すなあらし/ゆきの防御補正)に反映する。防御側のタイプ相性には反映せず
(gen9 の mechanics は反映する)、テラス 60 威力の下限・Tera Blast・ステラは無い。ゲームにテラスタルが無いので、
これらは実装しない。

## 2. oracle がダブルで反映するもの(総当たりで確認)

入力の意味: 形式は `new Field({gameType: 'Singles' | 'Doubles'})`。`champions.js` が `gameType` を読むのは次の2か所だけ。
技の対象は `Move.target`。oracle の Champions のデータで攻撃技に target があるのは全体技の 34 技だけ(他は省略 = `any`)。
gen9 のデータとの差は Astral Barrage 1 技(Champions は省略 = 単体扱い、gen9 は allAdjacentFoes)。

`tools/golden/generate.mjs` の `doubleCase` が各行を「対照と比べて変わる/変わらない」まで assert している
(oracle の版・データが変わったら生成が止まる)。

| # | 項目 | oracle(Champions 世代) | 根拠(dist/mechanics) | engine |
|---|---|---|---|---|
| D1 | 防御側の壁(リフレクター・ひかりのかべ・オーロラベール) | ダブル ×2732/4096(シングル ×2048)。急所は無視。リフレクター+ベールでも1回 | champions.js calculateFinalModsChampions | 実装 |
| D2 | 全体技(target が allAdjacent・allAdjacentFoes) | 基礎ダメージに `pokeRound(base×3072/4096)`。**天候・急所より前** | calculateBaseDamageChampions | 実装 |
| D3 | 全体技 × 壁 | 両方掛かる(0.75 × 2732/4096 ≒ 0.5 なので、ロールがシングルの壁と一致する組もある) | 同上 | 実装 |
| D4 | 対象の数 | 1対1の計算なので、ダブルの全体技は常に ×0.75(1体にしか当たらない場面は表せない) | 同上 | oracle どおり(見せ方は §4) |
| D5 | 味方の効果(てだすけ・フレンドガード) | Field の側のフラグで持つ | champions.js L452・L772 | 対象外 |
| D6 | 攻撃側の壁 | ダメージに効かない | — | そのまま |
| D7 | おやこあい × 全体技 | 子の攻撃を出さない | champions.js L251 | engine はおやこあいを持たない(対象外) |
| D8 | 全体技・無効相性 | 0 のまま。ふゆう等の特性による無効も同じ | — | そのまま |

(予備調査で「ダブルの壁は約 0.5」とした値は、全体技 Earthquake の ×0.75 と重なったもの。単体技は 2732/4096。)

## 3. 決定

### 3.1 engine

- `Move.Target MoveTarget` を足す。値は `""`(不明)・`single`・`spread` だけ。他は `ErrUnknownMoveTarget`
  (形式に関係なく拒否)。spread は oracle の allAdjacent・allAdjacentFoes に当たる(マスタへの写し方はデータレーン)。
- ダブルの壁は `2732`(シングル・形式未指定は `ModifierHalf`)。全体技は `Format == double && Target == spread` のとき
  基礎ダメージの式の直後に `pokeRound(base, 3072)`(天候・急所の前)。
- **形式**: `""`(ゼロ値)と `single` はシングル。engine に直接届いた未知の形式(HTTP・WASM は enum で拒否する)は
  エラーにせず、ダブルの補正を掛けない(シングルと同じ数値)。印は #497(ADR-0160)の「安全側の印」(target `format`)に任せる
  (`ErrUnknownFormat` は作らない。#497 のテスト「未知の形式は安全側で印」と矛盾させないため)。
- **技の対象が不明**(`Target == ""`)の**ダブルの攻撃技**は、全体技の補正を掛けず(単体扱い。壁は掛ける)、技の印
  `{target: move, reason: move_target_unknown, id: 技ID}` を付ける。新しい `UnsupportedReason`
  `UnsupportedMoveTargetUnknown`。技の印の並びは 機構(昇順)→ zero_power → move_target_unknown。
  シングル・形式未指定・変化技には付けない。マスタが技の対象を持つまで(issue #288)、calc-svc のダブルは全攻撃技にこの印が付く
  (黙って「全体技でない」数値を正しいように見せない。ADR-0123 の方式)。
- テラスタイプは計算に使わない(§1)。

### 3.2 一括計算・逆算

- `CalcBulk`・`CalcReverse` は `Format`・`Move.Target` をそのまま `CalcDamage` に渡す(既に渡している)。
- ダブルでも行・候補の構成は single と同じで、壁・全体技だけが効く(行は増やさない)。

### 3.3 境界(wasmapi・calc-svc・API)

- wasmapi の `move` に `target`(省略 = `""`)を足す。`""`・`single`・`spread` 以外は `invalid_enum`(calc・bulk・reverse)。
- calc-svc の変換は変更なし(format は既に engine に渡している)。技の対象はマスタの `engine.Move.Target`(現状は常に `""`)。
- `api/openapi.yaml` は説明だけを変える(`Format` と `UnsupportedMark.reason` の move_target_unknown)。形・enum は変えない。
  teraType の説明は変えない(#497 の担当)。`make gen`・`make ios-gen` 済み(`ios-gen-check` 緑)。
- 画面(Web・iOS の印の文言)は対象外。各クライアントは未知の reason を汎用の文言で出す(ADR-0215)。

### 3.4 ゴールデン

- Champions oracle に2ファイルを足す: `doubles.json`(33 件。§2 の各行と対照)・`doubles-random.jsonl.gz`
  (3000 件。seed 0x44424c45 = metadata の `doublesRandomSeed`。メインの random・legacy-random とは別の乱数列)。
  既存ファイル(fixed・random・attack-species・defense-species・stats-species・typechart・legacy-effects)は
  **バイト列不変**(再生成の前後で sha256 一致を確認)。形式 double と技の対象は doubles* にだけ現れる
  (`TestGoldenDoublesFieldsOnlyInDoublesFiles`)。doubles* は `goldenChampionsDamageFiles` には入れない
  (#497 の `TestGoldenInputsHaveNoTeraOrDouble` がそのリストの全ベクタにシングルを要求するため)。
- ランダムの全体技プールは、技名を名指しする処理が champions.js に無い 15 技(Earthquake・Bulldoze はグラスフィールドで半減、
  Misty Explosion・Eruption・Water Spout・Explosion・Self-Destruct は除く)。Earthquake は固定ベクタで地形なしで使う。
- known_diffs.yaml には何も足さない。

## 4. 未決事項(人間の確認)

- Q1. ダブルで相手が1体だけのときの全体技(engine は常に ×0.75 = D4)を画面でどう見せるか(注記を出すか・切り替えを置くか)。
- Q2. teraType を calc に送られたときの扱い(テラスタルが無いので、#497 の未対応の印のままにするか、黙って無視するか、
  400 で拒否するか、印を消すか。iOS の構築メンバーは teraType を送っている = ADR-0160 §背景)。

## 5. #497(ADR-0160)との関係と、マージ後の作業(実装者)

本ブランチは #497 が無くても単体でテストが通る形にしてある(印の期待は技の印 target=move だけを比べ、format・tera の印には
依存しない)。#497 をマージした後、ダブルを計算に反映したので次を同じ PR で直す:

- engine `unsupportedMarks` の format の印: `Format == double` には付けない(計算に反映したため)。未知の形式
  (`""`・single・double 以外)の安全側の印は残す。テラスの印は ADR-0160 のまま。
- #497 のテストのうち「format=double は印が付き数値は不変」を前提にするもの(`engine/unsupported_tera_format_test.go`・
  `engine/wasmapi/unsupported_tera_format_test.go`・`services/calc/internal/httpapi/unsupported_tera_format_test.go`・
  `services/judge/internal/client/calc_test.go`・vectors の `*/tera-double-marks`)の期待値を直す: format=double の印を外し、
  技の対象が不明なダブルの攻撃技には先頭に技の印 move_target_unknown が付く(仮実装に #497 を当てて確認した失敗は
  `TestUnsupportedTeraAndFormatMarks`・`...PropagateToBulk`・`...PropagateToReverse`・`TestWasmTeraAndFormatMarks`・
  `...InBulkAndReverse`・`TestCalcTeraAndFormatMarks`・`TestBulkAndReverseTeraAndFormatMarks`・`TestTeraAndFormatParityWithWasm`。
  どれも印の期待の差だけ)。数値不変を前提にするテストはダブルの数値に直す。理由はコミットメッセージに書き、
  未知の形式・テラスの印の期待は残す。本 ADR のテストと `TestGoldenInputsHaveNoTeraOrDouble` は #497 を当てても通る。
- ADR-0160 に「format=double の印は ADR-0222 で外した」と追記する。`api/openapi.yaml` の `Format` の説明は
  #497 と本 ADR の両方が書き換えるので、本 ADR の内容(double は計算に反映)に寄せて1つにまとめる。
  `UnsupportedMark.target` の説明の format は「未知の形式だけ」に直す。
- `engine/wasmapi/testdata/vectors.json` の末尾は両方が追記するので、両方のベクタを残してマージする。
- ADR-0005 の「ダブル固有補正は未対応」、ADR-0011 の「format は渡すだけ」、`TestCalcBulkFormatDouble` の注記を本 ADR を指すように直す。

## 6. 対象外・後続

- 技の対象のマスタ化(`MasterMove`・スキーマ・importer・pokedex の応答。issue #288・DECISIONS.md 2026-10-03)はデータレーン。
  Showdown の `target` を allAdjacent・allAdjacentFoes → spread、それ以外 → single に写し、oracle のデータとの差
  (Astral Barrage)を reconcile で扱う。入ったら calc-svc のダブルの技の印が消える。判定画面の「ダブル」を戻すのもその後。
- 味方の効果(てだすけ・フレンドガード等)、Web・iOS の印の文言。
