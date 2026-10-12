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

## 追記(critic の指摘への対応と持ち越し)

- 連続技は防御側を**満 HP** として計算する(「対戦の状態」の残り HP は渡さない。印 `defender_full_hp_assumed` はエンジンが付ける)。
- 連続技の節は、計算画面の一括計算(防御側の選択肢の出どころ)が成功しているときだけ出る。単発が失敗中は連続技も出ない。
- 計算中の案内は `role="status"` にした。手の削除・閉じるの後のフォーカス移動、`li` の名前の扱い、防御側の選択が消えたときの通知は段階 C で直す。

## 追記: 段階 C(2026-10-12。C1 素早さの種族の例・C2 素早さ画面からの導線・C3 a11y)

**C1 素早さの種族の例**
- 連続技の結果で相手の素早さが不明(`opponentSpeedKnown` が偽)のとき、最後の手の自分の素早さ(`speed.lastSelfSpeed`)で `speedClient.reach({mode: "raw", value})` を呼ぶ。
  呼び出しは `web/src/speed/speedClient.ts` のクライアント 1 か所(新しい経路は作らない)。ComboSection 内の `SpeedExample` が持ち、値が変わるたびに引き直す。古い応答は捨てる(求めた素早さが今の値と違えば出さない。cleanup の cancelled。`reach` は signal を取らないので abort はしない)。
- 出すのは 6 プリセットのうち**準速**(`neutral-max`。SP 32・補正なし)の行だけ。「準速 c 族以上には抜かれる(例: 速い側で最も遅い 1 体)」(`faster.min`・`slowestFaster`)、「準速 a 族までは抜ける(例: 遅い側で最も速い 1 体)」(`slower.max`・`fastestSlower`)、同速の区間があれば「準速 b 族とは同速」(区間は 〜 でつなぐ)。空の区間の行は出さない(1〜3 行)。区間と名前は API の値のまま(Web で計算し直さない。`domain/comboView.ts` の `speedExampleLines`)。
- **出さない場合**: speedClient が無い(オフライン)・reach が失敗・準速の行が無い・トリックルーム中(`reach` は向きを入れ替えないので「抜ける」が逆になる。ADR-0610 §1)。注記も出さない。連続技の結果・単発の計算は壊さない(絶対ルール 5)。
- speedClient は `calc.screen.tsx` の `createClient: createSpeedClient` で作り(作るだけで通信しない)、`env.recordClient` がある(= オンライン。App が計算モードがオンラインのときだけ渡す)ときだけ CalcScreen → ComboSection に渡す。ScreenEnvironment にモードの項目は足さない。

**C2 素早さ画面 → 連続技**
- `ScreenEnvironment` に `onOpenCombo`(素早さ画面へ)と `comboOpenRequest`(計算画面へ。`app/comboOpenRequest.ts` の `ComboOpenRequest` = `SpeedComboRequest` + token)を足した。App は `openFavoriteInCalc` と同じ流儀で token を進めて計算タブへ移る。`speed.screen.tsx` は `onOpenCombo` を渡す(ADR-0610 §7 の宿題を解消)。マスタ読み込み失敗中の素早さ画面には渡さない(リンクを出さない)。
- ポケモン ID は計算画面の種族の key と同じ形(`9001-000`)なので、そのまま `selectAttacker` / `selectDefender`(オンラインは `masterSearch.resolveSpecies` → `handleAttackerResolved` / `handleDefenderResolved`)で入れる。引けないポケモンは入れない(安全側)。token が変わったときだけ適用し、同じ token の再描画では利用者の変更を消さない。入れ終えたら連続技の節を広げる(`openToken`。「次の技」を押した状態)。
- 要求の `speed`(SP・性格・ランク・スカーフ)は**反映しない**。計算画面の攻撃側の入力は攻撃・特攻だけで、素早さの SP・性格・ランクの入力欄が無い(連続技は攻撃側の素早さ SP 0 で計算する)。反映するには計算画面に素早さの入力を足す必要があり、別タスク(持ち越し)。

**C3 a11y**
- 追加の手を外した後はフォーカスを「追加の技」のピッカーへ(ピッカーが使えないときは最後の手の操作ボタン)、「閉じる」の後は「次の技」ボタンへ移す(消えたボタンにフォーカスを残さない)。描画後の effect で消費する。
- 選んでいた防御側の振りが一括計算の更新で無くなり先頭へ戻ったとき、`role="status"` で「選んでいた相手の振りがなくなったので、先頭の振りに戻しました」と知らせる。
- 手の `li` の `aria-label` は変えていない(既存テストが listitem 名で引く)。
