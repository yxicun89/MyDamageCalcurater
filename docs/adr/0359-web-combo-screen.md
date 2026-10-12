# ADR-0359: 計算画面の連続技(CB-6 段階 B)

- 状態: 採用(2026-10-12)
- レーン: Web
- 関連: docs/plan/combo-calc.md(§1〜§5・AC-19)、ADR-0253(`POST /api/calc/combo`)、ADR-0179 追記 CB-5(WASM の calcCombo)、ADR-0357(ランクメーターの色の意味)、ADR-0123(未対応の印)、ADR-0300 §8(返ってきた値を加工しない)

## 決めたこと

1. **置き場所は案 A**。計算画面の結果の下に「連続技」の節(`ComboSection`)。手がメインの 1 つだけのときは「次の技」ボタンだけにし、押したときだけ広げる。
2. **入力を二重に持たない(UC-5)**。攻撃側・場・条件・ランク・メインの技・多段の回数は計算画面の値を props で受け取り、連続技専用の欄は作らない。
   防御側の振りは、計算画面の結果の行(`BulkRow` = 振り × 持ち物 × 特性)から「相手の振り」で選ぶ(`domain/comboView.ts` の `buildComboDefenders`)。
   メインの急所は「詳細」の急所(`conditions.critical`)と同じ値を読み書きする。
3. **手の持ち方**。画面の状態は「メイン(技欄の技)+ 追加の手(技 ID・急所)」の並び。並べ替えは上へ/下へ、メインは外せない、最大は `MAX_COMBO_STEPS`(3)。
   攻撃側を変えて覚えなくなった追加の手は、状態を書き換えず表示と要求から外す(毎レンダー導く)。追加の手の多段の回数は指定しない(既定の回数)。
4. **計算**は `calcComboVia(engine, ComboRequest)`。手が 2 件以上、または相手の技を選んだときだけ。要求は `useMemo` の参照を持ち、応答がその要求のものでなければ
   「計算中」として古い結果を出さない。effect の cleanup で abort し古い応答を捨てる(計算画面の calcBulk と同じ流儀。タイマーのデバウンスは入れない)。
5. **失敗の扱い**。`calcCombo` を持たない engine・失敗は、節の中だけに `role="alert"`(絶対ルール 5)。`request_aborted` は表示しない。
6. **表示**は engine / API の値の書式化だけ(`domain/comboView.ts`)。倒す確率は `total.ko.displayChancePercent`(確定は `guaranteed`、表示 0 は「倒せない」)、
   耐える確率は `opponentAttack.fast`(相手が先とした悪い方)を主にし、0 / 100 を予約する丸め(`formatSurviveChance`)。素早さは `speed.firstSelfSpeed → lastSelfSpeed`、
   相手の素早さが分からないときは最後の手の `line`(範囲の向きのまま言葉にするので、トリックルーム中も同じ関数で済む)。
7. **印**。連続技だけの理由 3 種(choice_locked・defender_full_hp_assumed・speed_undetermined)は `i18n/combo.ts` が専用の語を持つ。
   既存の `unsupportedText.reason` は契約の理由 15 種の過不足を検査するテストがあるため足さず、それ以外の印は単発と同じ `unsupportedMarkLabel` を使う。
8. **色**は既存トークンだけ。ランクチップの上昇は `--brand-primary`、下降は `--warning`/`--warning-soft`。合計バーは `--brand-primary`。色リテラルは書かない。
9. 素早さの種族の例(speed API の reach)と素早さ画面からの導線は段階 C。今回は線の数値だけ。

## 影響

- Web のみ(API・engine の変更なし)。既存のテストは変えていない(fake の `CalcEngine` は `calcCombo` が省略可のまま)。
- iOS(CB-8)は docs/ai-shared/decisions/2026-10-12-web-combo-screen.md の項目・語に揃える。
