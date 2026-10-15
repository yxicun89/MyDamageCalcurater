# ADR-0542: iOS の素早さ画面から計算タブの連続技へ渡す(I-ios-21)

- 状態: 採用(2026-10-15。仕様とテスト先行。実装は plan `I-ios-21`)
- レーン: iOS
- 関連: ADR-0535(連続技。「今回は対象外」の 1 項目目をここで入れる)、ADR-0531(タブ・画面の持ち方。`AppNavigationModel`・`RetainedScreenHost`・
  `FeatureContext`・`ScreenLifetime`)、ADR-0538(素早さ画面の結論 1 行 + 想定カード + 折りたたみ)、ADR-0541(計算画面の作り直し)、
  ADR-0533(ランクのメーター。不変)、ADR-0534(API 不可時の表示。不変)、ADR-0359 追記(Web の段階 C2・I-web-32)、
  docs/plan/combo-calc.md(CB-6 段階 C・CB-8)、docs/ai-shared/decisions/2026-10-12-web-combo-screen.md、
  ADR-0501 追記章「素早さ画面から連続技へ(I-ios-21)」(受け入れ条件・テスト)

## 背景

Web は素早さ画面の「この相手で連続技を計算」で、計算画面の攻撃側 = 自分・防御側 = 相手を入れ、連続技の節を広げ、
素早さ画面の SP・性格補正・ランクを連続技の「自分の素早さ」へ入れる(スカーフは API に項目が無く反映せず 1 行で知らせる)。
受け渡しは App が持つ要求(`ComboOpenRequest`)の `token` を 1 ずつ進め、計算画面は `token` が変わったときだけ受け取る。

iOS は ADR-0535 で連続技の節を入れたが、タブをまたいで入力を渡す仕組みが無く、この導線を分けた。素早さ画面には既に
`SpeedComboHandoff`(自分・相手・今の振り・ランク・スカーフ)と、出さないままのリンク(`speedComboLink`)がある。

## 決めたこと

1. **ルートの token**(Web の `comboOpenRequest` と同じ流儀)。`AppNavigationModel` に `comboOpenRequest: ComboOpenRequest?`
   (`token` + `SpeedComboHandoff`)と `openCombo(_:featureID:)` を足す。`openCombo` は token を 1 進めて要求を置き、計算画面のタブ・区切りへ移る
   (逆算・調整を見ていても計算へ)。同じ値でも押すたびに進む(もう一度押すと入れ直せる)。タブ・区切りを移っても要求は変わらない
   (= 計算画面は入れ直さない)。タブに属さない画面 ID では何もしない。
   `RootView` は `FeatureContext` に `openCombo`(素早さ画面へ)と `comboOpenRequest`(計算画面のタブの根へ。積んだ画面の文脈には渡さない)を足す。
   押したときは計算画面のスタックの path を空にする(右上の「このアプリについて」などを積んでいても、計算画面の根と節を見せる)。
   `SpeedFeature` は `onOpenCombo` を渡す(これで `comboAvailable` が true になり、既存の `comboHandoff` がリンクを出す)。
2. **計算画面の入力の上書き規則**(`CalcViewModel.openFromSpeed(_:)`)。入れるのは攻撃側・防御側の**種族だけ**。
   - 種族が今と違う側は、ピッカーで選んだのと同じ規則(`selectAttacker` / `selectDefender`。技の選び直し・メガの持ち物の固定・防御側の特性を「指定なし」に戻す)。
   - 種族が今と同じ側は何も変えない(持ち物・特性・技・構築の個体を残す)。
   - 場・急所・やけど・攻撃側/防御側のランク・攻撃側の SP と性格補正(攻撃・特攻)・比較する持ち物は触らない。
   - 計算は変わる側があるときだけちょうど 1 回。両側とも同じなら計算しない。
   - 先頭ページの外の種族は詳細(`species(key:)`)を引いて入れる。データに無い種族の側は黙って変えない(エラーにしない。Web と同じ)。
   - 初期の読み込みの途中に届いても、読み込みが終わってから入れる(読み込みの既定に上書きされない)。
   - 取り消されたら何も書き換えない(片側だけ入らない)。画面の `.task` からは切り離した Task(`ComboHandoffRunner`)で最後まで走らせ、新しい要求が来たら古い Task を取り消してから始める(タブを離れても半端にならない。ADR-0531 の `.retained`)。
3. **連続技の節の上書き規則**(`ComboViewModel.applyHandoff(_:)`)。節を広げ(候補を読む)、「自分の素早さ」を `ComboHandoffPlan.selfSpeedInput` の値で**丸ごと上書き**する。
   - 今の振りがあるとき: SP・性格補正(素早さ画面の 上昇 / 補正なし / 下降 → 連続技の 上がる / 補正なし / 下がる)・ランク。0 や補正なしも入力として入れる。
   - 今の振りが無いとき: SP は未入力(個体の値)・性格補正は「計算画面の性格のまま」。ランクは素早さ画面に常に見えている値なので入れる(±0 も)。
     前の引き継ぎ・手で入れた値は残さない(Web は今の振りが無い要求で既定に戻す。iOS はランクだけ素早さ画面の値にする。下の「採らなかった案」)。
   - SP は攻撃側の残りの SP の上限に丸める(既存の `setSelfSpeedSP` と同じ)。性格の置き換えは既存の規則(ADR-0535 §7)。
   - 節の他の入力(追加の手・相手の振り・相手の技・相手の素早さの切り替え・素早さの場)は触らない。手がメインだけなら計算しない(開いただけでは計算しない)。
     計算中の条件(手が 2 つ以上 / 相手の技あり)なら 1 回計算し直す。
   - 計算画面に結果が無く節を出せないときは値を持っておき、結果が出たら開いた状態で出す(計算は送らない)。
4. **スカーフ**: API の `ComboRequest` に指定する項目が無い(持ち物は 1 つ)ので反映しない。素早さ画面でスカーフを指定していたら、
   自分の素早さの下に 1 行(`PopNoticeView(.info)`、Web と同じ語「素早さ画面で指定したこだわりスカーフは、連続技の計算では反映できません。」、識別子 `comboScarfIgnored`)。
   自分の素早さ(SP・性格補正・ランク)を変えたら消す。次の引き継ぎがスカーフなしでも消す。
   引き継いで開いたときは「説明」ボタン(`PopHelpButton`、識別子 `comboHandoffHelpButton`)を出し、奥に 1 行 1 文の箇条(`comboHandoffHelpLine-<n>`):
   「自分と相手のポケモン、素早さのSP・性格補正・ランクを引き継ぎます」「こだわりスカーフとまひは、連続技に指定する項目が無いので引き継ぎません」
   「追い風とトリックルームは、この節の「素早さの場」で選びます」。
5. **目が行くように**(Web は節を広げるだけ。iOS は VoiceOver を足す): 受け取るたびに `appliedHandoffCount` を 1 進め、View はその変化で
   (a) `ScrollViewReader` で節へ送る(視差効果を減らすでは動きなし)、(b) `@AccessibilityFocusState` で節の見出しへフォーカス、
   (c) `AccessibilityNotification.Announcement`「素早さ画面の自分と相手で連続技を開きました」を 1 回出す。常時の動きは無い。
6. **素早さ画面のボタン**: 選んだ相手の想定カードの直下(出力 1 の最後・「これ以上」の上)に「この相手で連続技を計算」
   (`PopLabel` + `PopSymbol.damage`・`PillButtonStyle(.secondary)`・36pt 以上・`lineLimit` 無し)。自分と相手を選んだときだけ出す(既存の `comboHandoff`)。
   識別子 `speedComboLink`・語 `SpeedMatchupLabels.comboLink` は既存のものを使う。
7. 守ること: パレット K のトークンのみ・タップ 36pt 以上(アイコンだけのボタンは 37pt)・`lineLimit` 無し・常時アニメ無し・AX5 で横にはみ出さない・
   docs/glossary.md の禁止語なし・既存の識別子と読み上げ名は付け替えず新規のみ・ランク(ADR-0533)と API 不可時の表示(ADR-0534)は変えない。
   API・engine・サービスは変えない(`api/openapi.yaml` 不変)。直近の入力・履歴・お気に入りに自分の素早さを保存しない(Web と同じ)。

## 採らなかった案

- **今の振りが無いときにランクも未入力へ戻す(Web と完全に同じ)**: iOS の素早さ画面はランクを今の振りと別に常に出し、`SpeedComboHandoff` も
  `attackerRank` を別に持つ。見えている値を黙って捨てない方を採った。±0 を入れても個体のランク(計算画面で素早さのランクを触る口は無い)と同じ結果になる。
- **追い風・トリックルーム・まひも引き継ぐ**: 連続技には素早さの場(トリックルーム・追い風)があるが、Web は引き継がない。両 UI で揃え、「説明」で節の「素早さの場」へ案内する。
  まひは連続技に項目が無い。
- **計算画面を積む画面(`.pushed`)で開く**(履歴・お気に入りと同じ): 連続技の節はダメージタブの計算画面の入力(UC-5)を使うので、タブの計算画面へ入れる。
  積む画面にすると、計算タブの入力と別の計算画面が 2 つになる。
- **スカーフを素早さのランク +1 相当などに置き換えて反映する**: 1.5 倍はランクの倍率と一致せず、計算結果を偽る。API の項目が入るまで持ち越し(Web と同じ)。
- **素早さ画面の想定(相手の振り)を連続技の相手の振りへ入れる**: 連続技の相手の振りは計算画面の結果の行(防御側の調整)から選ぶ形で、素早さの想定とは軸が違う。Web も入れない。

## 影響

- `PokeCalcCore`: `ComboHandoff.swift`(新規。`ComboOpenRequest`・`ComboHandoffPlan`)、`AppNavigation.swift`・`CalcViewModel.swift`・`ComboViewModel.swift`・`ComboText.swift` に追加。
- アプリ: `RootView.swift`・`Features/AppFeature.swift`(`FeatureContext`)・`Features/CalcFeature.swift`・`Features/SpeedFeature.swift`・`CalcScreenView.swift`・
  `ComboSectionView.swift`・`ComboResultView.swift`(自分の素早さ)・`SpeedScreenView.swift`・`SpeedMatchupOutput.swift`。
- 既存のテストは変えない(ADR-0501 追記章の表)。
