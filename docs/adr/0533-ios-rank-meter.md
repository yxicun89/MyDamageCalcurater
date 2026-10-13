# ADR-0533: iOS の能力ランク入力を 13 段のメーターにする

- 状態: 採用(2026-10-12。ユーザー要望のラウンド 4。ec 経由)
- 関連: ADR-0357(Web のメーター)、`docs/ai-shared/decisions/2026-10-11-web-rank-meter.md`、ADR-0501「能力ランクのメーター」章(iOS のテストの変更)
- 担当: iOS(Web の見た目・色の意味・段数・読み上げに揃える)

## 決定

1. **共通 SwiftUI 部品 `RankMeterView`**(`ios/PokeCalc/RankMeterView.swift`)。13 段(左 6 = 下降・中央 0・右 6 = 上昇)。
   純粋ロジック(段↔値、ドラッグ位置→段、増減のクランプ、符号付き文字、読み上げ、識別子)は `RankMeterLogic`(PokeCalcCore)に置き XCTest で固定する。
2. **色**: 上昇 = `brandPrimary`、下降 = `warning`、0 の目印 = `textPrimary`(太い縦棒)、未到達の段の枠 = `textSecondary`。赤は使わない。値は `+2` / `-3` / `±0` の文字でも併記する。
3. **操作**: トラックのタップ(`SpatialTapGesture`)・ドラッグで最寄りの段。ドラッグは UIKit のパン(`RankHorizontalPan`)で、最初の動きが水平優勢のときだけ始め、縦が優勢なら始めない(ScrollView の縦スクロールを奪わない。Web の `touch-action: pan-y` 相当。判定は `RankMeterLogic.isHorizontalDrag`)、− / + ボタン(36pt 以上、両端で無効)。
   VoiceOver ではトラック全体を 1 つの adjustable 要素にし(`accessibilityAdjustableAction` で ±1)、label は欄のステータス名(「攻撃」)、value は「+2」(0 は「±0」)。
   Web の `aria-valuetext`「攻撃 +2」は `RankMeterLogic.spokenText` に持つ(label + value を続けて読むと同じ読み上げになる)。
   − / + の名前は「<欄の名前>を上げる / 下げる」(例「防御側のランクを上げる」)。
4. **識別子**(欄ごとに安定): `rankMeter-<field>` / `rankMeterMinus-<field>` / `rankMeterPlus-<field>`。
   field は `calcAttacker`・`calcDefender`・`speed`・`judgeAttacker-<stat>`・`judgeCandidate<n>-<stat>`。旧 `calc{Attacker,Defender}Rank*`・`speedRank*`・`<p>Rank*-<stat>` は無くなった。
5. **適用**: 計算の詳細(攻撃側 = 技の分類で攻撃/特攻、防御側 = 防御/特防)、素早さ(カスタム)、判定(5 ステータス × 各個体。判定は非表示のままだがコードにある)。
   ViewModel・契約・エンジンは変えない(値は従来どおり -6〜+6 の整数)。
6. **文字サイズ**: アクセシビリティの文字サイズではメーターを幅いっぱいに置き、− / 値 / + を下の行に並べる(横にはみ出さない)。
7. **動き**: 常時アニメーションなし。操作したときだけ `MotionToken.pressDuration` の短い反応で、「視差効果を減らす」では動かさない。
8. 計算の見える値は従来どおり「A +1」「B ±0」(`RankLabel`。どのステータスを触っているかが分かる)。素早さ・判定は符号付きの数だけ。

## 影響

iOS の計算・素早さ・判定の画面とそのテスト。API・engine・Web は変えない。
未使用になった `JudgeLabels.rankIncrement/rankDecrement` は削除。`CalcConditionLabels.rank*Increment/Decrement` は `DefenderRankLabelsTests` が固定する文言なので残す。

## 追記: 素早さ画面の新レイアウト(2026-10-12)

素早さ画面は speed レーンが「相手を選んで先に動く最低ライン」の構成に作り直した(#757)。自分の側のランク行は、新しい `SpeedSelfSection` の中で同じ `RankMeterView`(field `speed`)に置き換えた。識別子・読み上げは本文のとおり。

追記(ゲートの失敗と対処。2026-10-12): `SpeedMatchupLargeTextUITests.testSpeedMatchupInputNoHorizontalOverflowAtAX5` が「画面に出なかった要素」で落ちた。原因は本物で、ランクのメーターの `DragGesture` が ScrollView の縦スクロールを奪い、画面中央(slow swipe の開始点)がメーターに当たると 30 回スワイプしても進まなかった。対処はアプリ側: ドラッグを `RankHorizontalPan`(`UIGestureRecognizerRepresentable` の UIPanGestureRecognizer。最初の速度が水平優勢のときだけ begin。判定は `RankMeterLogic.isHorizontalDrag`)に替え、縦は始まらせない。期待は変えていない。`CalcDefenderRanksUITests` の縦スワイプ試験に「メーターの上から始めても画面がスクロールする」検査を追加。他の3件(`AdjustGoalsUITests.testUnavailableGoals…`・`AdjustScreenUITests.testUnsupportedMarkNoticeIsShown`・`CalcAttackerStatsUITests.testSelectingSpecialMove…`)は単独では再現せず、修正後の上記クラス一式(48 件)と合わせて成功した(同じ原因の可能性が高い:スクロールが進まない)。

## 追記: ±0(未入力)の見た目(2026-10-13。Web の ADR-0357 追記 / `docs/ai-shared/decisions/2026-10-13-web-rank-meter-zero.md` に揃える)

ユーザーの画面確認で「±0 のとき枠だけで寂しい」。決定 2 の「未到達の段の枠 = `textSecondary`」を改める。
- 値の入っていない段のうち、左 6(下降側)は `warning`、右 6(上昇側)は `brandPrimary` を薄く塗る(塗り 16%・枠 70%。トークンの `opacity` で表す。色リテラルは使わない)。値が入った段は従来どおり濃い塗り。
- 中央の 0 は太い縦棒(`textPrimary`)に、左右 2pt の余白を足して強調。
- 見た目の決定は純粋ロジック `RankMeterLogic.segmentFill`(`SegmentFill`: solidUp/solidDown/tintUp/tintDown/zeroMarker)と定数 `tintFillOpacity`/`tintStrokeOpacity` に切り出し、XCTest で固定。
- 無効時はメーター全体の不透明度を下げる(`disabledChipOpacity`)。常時アニメーションなし。
- 右の見える値ラベルは符号付き(計算は「A ±0」、素早さは「±0」。従来どおり)。読み上げ(label「攻撃」・value「±0」)・adjustable・識別子・タップ 37pt は不変。
