# ADR-0224: テラスタルをオプションの機能として計算に反映する(指定したときだけ)

- 状態: 採用(critic PASS。2026-10-03)
- 日付: 2026-10-03
- 関連: ADR-0002(ゴールデンの oracle。追記 P2-1b で Champions 世代へ)、ADR-0005(補正の対象範囲。「テラスタルは対象外」を
  本 ADR で置き換える)、ADR-0011(WASM 境界)、ADR-0123・ADR-0215(未対応の印。target・reason は enum にしない)、
  ADR-0160(PR #497。テラスタイプ・format=double の未対応の印)、ADR-0222(ダブル。§1 の「teraType は計算に使わない」を
  本 ADR で置き換える)、issue #232

## 背景

ポケモンチャンピオンズ本編にテラスタルは無い(ユーザー確認 2026-10-03。ADR-0222 §1)。それでも、
ユーザー決定(2026-10-03)は次のとおり:

> テラスタルは機能だけ追加しておいて、オプションで選択できるでいい。ダブルはなんでもいい(現状のままでよい)。

engine は `Individual.TeraType` を受け取るが計算に反映せず、指定すると「未対応」の印を付けていた(ADR-0160)。
iOS の構築メンバーは `teraType` を持ち、calc にそのまま送っている(ADR-0160 §背景)。

## 決定

既定(`teraType` 省略・空)は従来どおり(テラスタル無し)。指定したときだけ「テラスタル済み」として計算に反映し、
@smogon/calc 0.12.0 の Champions 世代(`Generations.get(0)`。絶対ルール3の照合の正)と同じ結果にする。
oracle は `Pokemon.teraType` を指定するだけでテラスタル済みと扱う(別のフラグは無い)ので、engine も同じにする。
計算は 4096 基準の固定小数・五捨五超入のまま(float にしない)。known_diffs は足さない。

### 1. oracle が反映するもの(総当たりで確認した表)

oracle の実装(0.12.0 の `dist/`):

- タイプ一致 `util.getStabMod`: `4096 + (元タイプ=技 ? 2048) + (テラス=技 ? 2048)
  + (てきおうりょく かつ hasType(技) ? (テラスが元タイプ ? 1024 : 2048))`
- `Pokemon.hasType(t)`: テラス中(ステラ以外)は `teraType === t` だけを見る。テラス無しは元のタイプ
- `Pokemon.hasOriginalType(t)`: 常に元のタイプ
- `champions.js` の相性は `defender.types`(元のタイプ)で引く。テラスの語は champions.js に1つも無い
  (= テラス一致技の威力 60 下限・Tera Blast・ステラの補正は Champions 世代に無い)

`tools/golden/generate.mjs` の `teraCase` で、各行を「テラスを外した対照」と比べ、oracle のダメージが変わる/変わらないを
生成のたびに確かめている(前提が黙って崩れたら生成が止まる)。下の数値は Lv50・個体値31・SP0・まじめ・特性/持ち物なし
(`tera.json` の同名ケース)のダメージ範囲(対照 → テラス)。

| # | 条件 | 例(攻撃側 → 防御側 / 技) | oracle | 反映 |
|---|---|---|---|---|
| T1 | 攻撃側: テラス = 元タイプ = 技 | リザードン テラスほのお → カビゴン / かえんほうしゃ | 51-61 → 68-82(×1.5 → ×2.0) | する |
| T2 | 攻撃側: テラス ≠ 元タイプ、技は元タイプ | リザードン テラスみず / かえんほうしゃ。ガブリアス テラスドラゴン / だいちのちから | 不変(×1.5 のまま) | する(変わらないことを含む) |
| T3 | 攻撃側: テラスだけが技と一致 | リザードン テラスでんき / 10まんボルト | 34-41 → 51-61(×1.0 → ×1.5) | する |
| T4 | 攻撃側: どれも不一致 | リザードン テラスみず / 10まんボルト | 不変 | する(変わらない) |
| T5 | てきおうりょく | テラス=元=技 68-82 → 76-92(×2.0 → ×2.25)、テラスだけ一致 34-41 → 68-82(×2.0)、テラス≠元の元タイプ技 68-82 → 51-61(×1.5)、複合でテラス=もう一方の元タイプ(ガブリアス テラスドラゴン / だいちのちから)54-64 → 40-48(×1.5)、不一致は不変 | 左のとおり | する |
| T6 | 攻撃側の接地(フィールドの威力補正) | ピカチュウ テラスひこう / エレキフィールド 10まんボルト 36-43 → 28-34(浮く)。リザードン テラスほのお 34-41 → 45-53(元ひこうでも接地)。テラスでんき 34-41 → 67-79(接地+一致)。リザードン テラスひこう・ふゆう+テラスノーマルは不変 | 左のとおり | する |
| T7 | 防御側の接地(ミストのドラゴン半減) | カビゴン テラスひこう / ドラゴンクロー 33-39 → 64-76。アーマーガア テラスはがね 21-26 → 11-13 | 左のとおり | する |
| T8 | サイコフィールドの先制技(接地した防御側に当たらない) | カビゴン テラスひこう / でんこうせっか 0 → 34-42(当たる)。アーマーガア テラスノーマル 12-15 → 0(当たらない) | 左のとおり | する |
| T9 | すなあらし(いわの特防 ×1.5) | カビゴン テラスいわ / かえんほうしゃ 51-61 → 34-42。バンギラス テラスほのお 18-22 → 27-33。物理技は不変 | 左のとおり | する |
| T10 | ゆき(こおりの防御 ×1.5) | カビゴン テラスこおり / のしかかり 75-88 → 51-60。ユキノオー テラスくさ 45-54 → 67-79。特殊技は不変 | 左のとおり | する |
| T11 | 防御側のタイプ相性 | カビゴン テラスゴースト / のしかかり 75-88(無効にならない)。ガブリアス テラスフェアリー / ドラゴンクロー 116-140。カビゴン テラスひこう / だいちのちから 40-48。テラスくさ / かえんほうしゃ 51-61。たつじんのおび(抜群判定)も元タイプ | **不変** | **しない**(oracle の癖。§2) |

ゴールデンは T1〜T11 の各行(`tera/t1/…`〜`tera/t11/…`)・組合せ(`tera/combo/…`)・18タイプの総当たり
(`tera/all/{atk,def,both}/<type>/…`)を `tera.json`(180件)に、ランダム 3000 件(seed `0x54455241` "TERA"、
シングルのみ、層の下限は生成器と `TestGoldenTeraCoverage` の両方で守る)を `tera-random.jsonl.gz` に出す。
既存 9 ファイル(fixed・random・attack/defense/stats-species・legacy-effects・doubles・doubles-random・typechart)と
`effects.json` のバイト列は不変(sha256 を再生成の前後で確認。`metadata.json` だけが files・seed・除外理由の追記で変わる)。
生成は2回実行して全ファイルの sha256 が一致すること(決定性)を確認した。

### 2. 防御側のテラスはタイプ相性に反映しない(oracle の癖を再現する)

本編 SV では防御側のテラスはタイプ相性を置き換える(@smogon/calc の gen9 mechanics も反映する)。
Champions 世代の oracle は反映しない。照合の正は oracle(絶対ルール3)なので、engine も相性は元のタイプで引く。
ただし「そのタイプを持つか」(T7〜T10)は oracle どおりテラスタイプで見る(反映する)。

これは**ゲームの実際の仕様と食い違う**点なので、防御側に teraType が指定されたときは ADR-0160 の未対応の印
`{target: defender_tera_type, reason: unsupported_effect, id: <テラスタイプ>}` を残す(「この数値は本編のテラスタルと
違いうる」の明示)。known_diffs には足さない(oracle と一致しているので差分ではない)。

### 3. 印(ADR-0160 との整合)

| 指定 | 数値 | 印 |
|---|---|---|
| 攻撃側の teraType | §1 のとおり反映 | **付けない**(ADR-0160 の `attacker_tera_type` を外す。定数と契約上の値は互換のため残す) |
| 防御側の teraType | 「そのタイプを持つか」だけ反映、相性は元タイプ | `defender_tera_type` を残す(変化技にも付く。ADR-0160 §2 のまま) |
| 逆算の既知側 | 既知側の teraType は上と同じ(攻撃側なら印なし、防御側なら印あり) | 同上 |
| 一括計算のプリセットの防御側・逆算の探索側 | テラス無し | なし |

並びは ADR-0160 のまま(技 → 攻撃側の持ち物 → 攻撃側の特性 → 防御側の持ち物 → 防御側の特性 → 防御側のテラス → 未知の形式)。

### 4. engine の仕様

- `hasType(個体, t)`: TeraType が空でなければ `TeraType == t`、空なら元のタイプ。接地(`isGrounded`)・
  すなあらし/ゆき(`weatherDefenseMod`)・サイコフィールドの先制技(`blockedByPsychicTerrain` → `isGrounded`)が使う
- 元のタイプの判定(oracle の `hasOriginalType`)を別に持ち、タイプ一致と相性(`Species.Types`)はこちら
- タイプ一致補正: `4096 + (元タイプ=技 ? 2048) + (テラス=技 ? 2048) + てきおうりょくの加算`。
  てきおうりょくは効果データ `AbilityEffect.StabMod`(テラス無しのときの一致補正。8192)から加算分 `StabMod − ModifierStab`
  を導き、「技のタイプを持つ(hasType)」ときだけ足す。テラスが元タイプのときは加算分の半分(oracle の +1024)。
  特性名・倍率をコードに直書きしない。`DamageResult.STAB` は補正値が 4096 でないとき true
- Format(ダブル)とは独立(ダブルでもシングルと同じ規則。ダブルの壁・全体技はそのまま)
- CalcBulk: 攻撃側の TeraType を全行へ素通し。プリセットの防御側はテラス無し
- CalcReverse: 既知側の TeraType を素通し。探索側(相手)はテラス無し
- 検証: 表に無いテラスタイプは従来どおり `ErrUnknownType`(ADR-0013。WASM 境界・calc-svc の検証も変えない)

### 5. API 契約

`api/openapi.yaml` の `Individual.teraType` と `UnsupportedMark`(target・reason の説明)の **説明だけ**を変える
(スキーマ・必須・enum は変えない)。`make gen`(Go・TS)と `ios/scripts/openapi-gen.sh` を実行済み。

## #497(ADR-0160)のテストとの整合(期待値の変更一覧)

テストは消さず、弱めない。攻撃側の印が消える・防御側の印は残る方向に期待を直し、数値の反映を確かめる検査を足した。

| ファイル | テスト | 変更 |
|---|---|---|
| `engine/unsupported_tera_format_test.go` | `TestUnsupportedTeraAndFormatMarks` | 攻撃側テラスの3ケースの期待を「印なし」へ。数値の不変は「テラスで数値が変わらない入力」だけに限定し、変わる入力(テラス=技)は ×1.5 の rolls を要求。両側テラスのケースは防御側の印だけ。攻撃側の印が1つでも付いたら失敗する検査を追加 |
| 同 | `TestUnsupportedTeraAndFormatMarksOnStatusMove` | 変化技の印を防御側のテラス(ゴースト)で確かめる(攻撃側テラスも入れたまま、その印は付かないことを含めて比較) |
| 同 | `TestUnsupportedTeraAndFormatPropagateToBulk` | 期待を `[move_target_unknown]` へ(攻撃側テラスの印なし)。数値が single と同じことは維持 |
| 同 | `TestUnsupportedTeraAndFormatPropagateToReverse` | 既知の攻撃側テラス: 印なし。既知の防御側テラス: 印あり(変更なし) |
| 同 | `TestUnsupportedTeraUnknownTypeStillRejected`・`TestUnsupportedTeraAndFormatDoNotSplitAbilityGroups` | 変更なし |
| `engine/double_test.go` | `TestDoubleIgnoresTeraType` → `TestDoubleTeraTypeSameRuleAsSingle` | 「数値が変わらない」から「元からノーマルの個体と同じ数値(タイプ一致)で、防御側テラスは相性に効かない」へ |
| `engine/bulk_test.go` | `TestCalcBulkPassesThroughAllOptionsCombined` | 「TeraType は現状ダメージに影響しない」を、剥がすと全行が変わる(strips に追加)・テラス=元タイプ×てきおうりょくで最大ダメージが上がる、へ |
| `engine/wasmapi/unsupported_tera_format_test.go` | `TestWasmTeraAndFormatMarks` | 攻撃側 teraType の期待を `[]` へ。組合せケースから攻撃側の印を外す |
| 同 | `TestWasmTeraAndFormatDoNotChangeNumbers` | 攻撃側を「テラス≠元タイプ・元タイプの技」(T2。数値不変)に変更。防御側ゴースト・double の数値不変は維持 |
| 同 | `TestWasmAttackerTeraChangesNumbers`(新規) | 攻撃側 teraType が境界を通って ×2.0 になる(てきおうりょくと同じ rolls)・印なし |
| 同 | `TestWasmTeraAndFormatMarksInBulkAndReverse` | 攻撃側の印を外す。逆算 side=attacker(既知の防御側テラス)の印ありを追加 |
| 同 | `TestVectorsTeraAndDoubleProduceMarks` | 「tag tera ならテラスの印がある」を「攻撃側の印は無く、防御側テラスを持つ入力には防御側の印(ID 一致)がある」へ |
| `engine/wasmapi/vectors_test.go` | `TestVectorsCoverRequiredScenarios` | 必須タグに `teraStab`・`teraHasType` を追加 |
| `engine/wasmapi/testdata/vectors.json` | `*/tera-double-marks` | request は不変。source の説明だけ更新。テラスを反映する新ベクタ6件を追加(下記) |
| `services/calc/internal/httpapi/unsupported_tera_format_test.go` | `TestCalcTeraAndFormatMarks` | 攻撃側テラスの期待を `[]`、両側テラス+ダブルは `[move_target_unknown, defender_tera_type]` |
| 同 | `TestBulkAndReverseTeraAndFormatMarks` | 期待を `[move_target_unknown]` へ。逆算 side=attacker(既知の防御側テラス)の印ありを追加 |
| 同 | `TestTeraAndFormatParityWithWasm` | 印の件数を calc 3→2(防御側テラスと技の対象不明)、reverse 2→1 |
| 同 | `TestCalcAttackerTeraReflectedInNumbers`(新規) | calc-svc で teraType が engine まで届き、T1 で rolls が変わり T2 で変わらない・engine 直呼びと一致 |
| `services/judge/internal/client/calc_test.go` | — | 変更なし(judge は teraType を送らない) |
| `engine/golden_tera_format_scope_test.go` | `TestGoldenInputsHaveNoTeraOrDouble` | 変更なし(既存ファイルにテラスが無いことを引き続き守る。tera* は対象外のリスト) |

追加したベクタ(Go/WASM 一致。`make test-wasm`): `calc/tera-stab-adaptability`(T5)・`calc/tera-only-stab`(T3)・
`calc/tera-defender-sand`(T9・防御側の印)・`calc/tera-attacker-flying-terrain`(T6)・`bulk/tera-stab`(T3)・
`reverse/tera-known-defender-sand`(T9・逆算 side=attacker)。

## 影響

- **既存クライアントの結果が変わる**: iOS の構築メンバーは teraType を calc に送っている。攻撃側にテラスを持つ
  メンバーの計算は、これまでの「テラス無しの数値+攻撃側テラスの印」から「テラスのタイプ一致・接地を反映した数値・印なし」に
  変わる(例: テラス=元タイプの技は ×1.5 → ×2.0、テラスだけ一致の技は ×1.0 → ×1.5、元タイプでない技を持つ
  てきおうりょくは ×2.0 → ×1.5)。防御側にテラスを持つメンバーは、接地・すなあらし/ゆきだけ数値が変わりうり、
  防御側テラスの印は残る。Web は WASM が同じ engine を使うので同じ変化になる
- 判定(judge)は teraType を送らないので変化なし
- 入力 UI(テラスの選択)は本 ADR の対象外。Web・iOS レーンが追従する(`docs/ai-shared/decisions/` に連絡)
- `docs/impl/damage-engine.md` の TeraType の行と、`web/src/engine/types.ts` の印の説明(attacker_tera_type)は
  実装に合わせて更新する(前者は実装者、後者は Web レーン)
- ADR-0005 の対象外リスト・ADR-0222 §1・ADR-0160 に本 ADR を指す追記を入れた

## 未決事項

- Q1. 防御側テラスの相性を本編 SV どおり反映するか(oracle と食い違うので known_diffs と人間の承認が要る)。
  既定案: 反映しない(本 ADR)。必要になったら別 ADR で gen9 の mechanics を照合先にする案を検討する
- Q2. ダブル × テラスのゴールデン照合(ユーザーは「なんでもいい」)。engine の規則は形式と独立なので、
  既定案: 照合しない(単体テストで形式に依らないことだけを固定)
