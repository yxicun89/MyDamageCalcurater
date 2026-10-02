# ADR-0160: テラスタイプと format=double を拒否せず「未対応」の印を付ける(issue #232)

- 状態: 採用
- 日付: 2026-10-02
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #232、ADR-0123(未対応の印)、ADR-0215(target・reason を enum にしない)、ADR-0005(M1 の対象外)、
  ADR-0011(WASM 境界。「受け取るが engine は未使用」)、ADR-0708(judge の印の中継)、requirements.md(ダブルは後続 Phase)

## 背景

engine は `Individual.TeraType` と `Format=double` を受け取るが計算に反映せず(テラスの STAB・防御タイプの置換、
ダブルの壁 2/3・全体技 0.75 はどれも未実装。ADR-0005)、結果にもその印が無い。指定したのに黙って
「テラスなし・シングル」の数値が返る。

issue #232 の既定案 A は「400 で拒否」だが、iOS の構築メンバーは `teraType` を持ち、`APIPokeCalcService` が
calc にそのまま送っている。拒否にすると構築からの計算がすべて失敗する(破壊的変更)。案 B(正しく計算)は、
ポケモンチャンピオンズにテラスタル・ダブルがあるか / いつ対応するかという人間の判断が要る。

## 決定

### 1. 拒否せず、既存の「未対応」の印を付ける(案 C)

ADR-0123 と同じ方式にする。計算は従来どおり(テラスなし・シングルの式)で行い、結果に印を付ける。
数値は印で変えない(ゴールデンは全件そのまま一致。ゴールデンのベクタは元からシングル・テラスなしだけ)。
400 で拒否しない。表に無いテラスタイプは従来どおり `ErrUnknownType`(検証が先。ADR-0013)。

### 2. 印の形

| 条件 | target | reason | id |
|---|---|---|---|
| `Attacker.TeraType != ""` | `attacker_tera_type` | `unsupported_effect` | テラスタイプの ID(例 `fire`) |
| `Defender.TeraType != ""` | `defender_tera_type` | `unsupported_effect` | テラスタイプの ID |
| `Format` が `""` でも `single` でもない | `format` | `unsupported_effect` | `Format` の値(例 `double`) |

- **reason は新設せず `unsupported_effect` を使う**。意味は「効果を計算に反映していない」で、テラスタル・ダブルの
  効果にもそのまま当てはまる。Web・iOS は `unsupported_effect` のとき理由の括弧を省き、対象名と ID だけを出す
  (ADR-0501 P6-17・`web/src/i18n/ja.ts`)ので、新しい reason を足すより既存の表示に自然に乗る。
- **target は新設する**(ADR-0215 で enum にしていないので破壊的変更にならない)。古いクライアントは未知の target を
  「項目「fire」」「項目「double」」のように汎用の語 + ID で出す(印を捨てない・応答全体を失敗にしない)。
- **並び**: 既存の印(技 → 攻撃側の持ち物 → 攻撃側の特性 → 防御側の持ち物 → 防御側の特性)の後に
  攻撃側のテラス → 防御側のテラス → 形式。既存の印の並びは変えない。
- **条件を絞らない**: テラスが元のタイプと同じ・技と違うタイプなど、テラスタルの規則上は結果が同じになる入力にも付ける。
  ゲーム(ポケモンチャンピオンズ)にテラスタルがあるか・規則が本編と同じかが未確定で、「通常の式と同じ」と
  engine が言えない(ADR-0123 §3 の「式から言える場合だけ外す」に当たらない)。また、指定した値が無視されたことを
  利用者に伝える目的にも合う。
- **変化技でも付ける**: ADR-0123 §2 本文は「変化技は印を付けない」(技の印)だが、持ち物・特性の印は
  既存コードでは変化技にも付く(技の印だけが外れる)。テラス・形式の印も後者と同じく変化技に付ける
  (指定が無視されたことを伝える目的は変化技でも同じ)。
- **iOS での重複**: 攻撃側・防御側のテラスが同じタイプだと、`target` が違っても iOS の `UnsupportedPlacement` の
  重複除去(ID 単位)で1件にまとまる(クラッシュはしない。§5 の iOS 側の作業で解消する)。
- **engine に直接届いた未知の形式**(HTTP・WASM は enum で拒否するので直接呼び出しだけ)も安全側で印を付ける
  (ADR-0123 §3 の未知の機構と同じ)。

### 3. 伝達

印は `unsupportedMarks`(`CalcDamage`)で付けるので、一括計算の各行・逆算の各候補にも同じ印が付く
(CalcDamage の合成。ADR-0123 §2)。

- 一括計算: 攻撃側のテラスと形式。防御側はプリセットから組み立てるのでテラスを持たない。特性のまとめ(ADR-0126)は
  全特性で同じ印なので割れない。
- 逆算: 既知側のテラスと形式。探索側(相手)はテラスを持たない。
- WASM(`engine/wasmapi`)・calc-svc の写し(`unsupportedFrom`)は target・reason を文字列で写すだけなので変更不要。
  Go/WASM 一致のベクタに `calc` / `bulk` / `reverse` の `*/tera-double-marks` を足した(tag `tera` / `double`)。
- judge は calc-svc の印をそのまま中継する(ADR-0708)。judge は `teraType` を受け取らないので、届くのは `format` の印だけ。

### 4. 契約

`api/openapi.yaml` の形は変えない。`UnsupportedMark`(target・reason・id)・`Format`・`Individual.teraType` の
description に既知の値と意味を追記し、`make gen`・`make ios-gen` で生成物(コメントだけ)を更新した。

### 5. 表示(各レーンの後続作業)

Web(`unsupportedTargetLabel`)・iOS(`UnsupportedTarget`・`UnsupportedMarkLabel`)に新しい target の語
(例: 攻撃側のテラスタイプ / 防御側のテラスタイプ / 対戦形式)と、ID の表示名(タイプ名・「ダブル」)の解決を足すのは
各クライアントレーンの作業。足すまでは ADR-0215 の汎用表示で安全に出る。

## 結果

- テラス・ダブルを指定した結果が、黙って正しい結果のように見えなくなる。iOS の構築からの計算は壊れない。
- 数値・ゴールデンは不変。`TestCalcBulkFormatDouble` が注記していた「Format が CalcDamage へ素通しされているかを
  検証できない」問題は、format の印で検証できるようになった。
- テラス・ダブルを正しく計算する(案 B)ときは、実装した側から印の条件を外し、本 ADR に追記する。
