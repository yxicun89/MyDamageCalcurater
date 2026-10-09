# ADR-0518: iOS の計算画面に攻撃側の SP 数値入力・性格補正を足し、技の選択肢をダメージ技に絞る(F-01 / I-ios-1・I-ios-5)

- 状態: 採用(spec-writer。2026-10-05。受け入れ条件とテストが先。実装は後続)
- 日付: 2026-10-05
- レーン: iOS
- 関連: ADR-0329(Web の攻撃側 SP・性格・2 ブロック)、ADR-0328(Web の技の絞り込み)、ADR-0500 §6(攻撃側プリセット)、
  ADR-0501「P6-2d」(構築から呼ぶ)、ADR-0509(持ち物の役割・メガ固定)、
  `docs/ai-shared/decisions/2026-10-04-web-calc-attacker-input-flow.md`、docs/usability-round2.md F-01
- 番号: iOS 帯の最新(0517)の次。0513(お気に入り読み込み)は別ブランチで使用中のため、衝突を避けて 0518 を使う

## 背景

ユーザー要望(2026-10-04): 攻撃側の SP を 0〜32 の数値で入れ、性格補正も選びたい。技の選択肢はダメージを与える技だけにしたい。
Web は完了している(ADR-0329・ADR-0328)。iOS を同じ仕様に揃える。engine・API 契約・services は変えない(丸めは engine のまま)。

iOS の既存: 攻撃側は `AttackerPreset`(無振り・特化・振り。ピルの行 `attackerPreset-*`)だけで、SP は「技の関連ステータスに 0 か 32」、
性格は一覧から規則で選ぶ。技の選択肢 `moveOptions`(learnset ∩ 検索結果)は変化技を含み、変化技だけを覚える種族では変化技を選ぶ。
逆算(`ReverseViewModel`)と調整(`AdjustViewModel`)の技ピッカーは**すでにダメージ技だけ**に絞ってある。判定は対象外(Web も対象外)。

## 決定

### 1. 状態・要求(Web に揃える)

- 攻撃側の入力は「攻撃」「特攻」の 2 ブロック。ブロックごとに `{ spText: 文字列, modifier: 上昇/補正なし/下降 }`(`AttackStatInput`)。
  既定は両方 `"0"`・補正なし。値は技・種族(攻撃側/防御側)・攻守入れ替え・構築の呼び出しで消さない(`load()` だけが既定に戻す)。
- 要求の SP は H・B・D・S が 0、攻撃と特攻は**両方**載せる(技が使わない側もそのまま。お気に入りの個体も同じ)。合計は最大 64 で 66 を超えない。
- 性格は**マスタの性格一覧から**解決する(`AttackerStatRules.resolveNature`。Web の `resolveAttackerNature` と同じ規則):
  両方補正なし → 一覧の最初の無補正(従来の `AttackerPreset.build` と同じ。既定の要求を変えない)/ 同じ向きは解決失敗 /
  技が使う側が上昇でもう一方が補正なしなら、マイナスをもう一方に置く性格(物理 = +A/−C、特殊 = +C/−A)を優先 /
  A・C の両方に合う性格のうち **ID の昇順で最初** / 無ければ技が使う側だけを合わせる(使う側が補正なしなら無補正)/ それも無ければ失敗。
  ID の昇順は文字列(Unicode スカラー)の昇順。
- 不正入力は**丸めず**明示エラー(計算しない・古い結果も出さない・古い応答も捨てる)。SP は 10 進整数 0〜32 のみ
  (前後の空白は無視・先頭の 0 は可・空欄は 0。符号・小数点・指数・全角・33 以上は不正)。技が使わない側が不正でも計算しない。
  性格が解決できないときも同じ(`error` に `natureUnavailable` を立てる。メッセージは `AttackerStatLabels.natureUnresolved`)。
  SP の不正は `error` を立てず、入力欄の近くに理由を出す(`attackerSPError-*`)。直せば計算し直す。
  タスク文の「範囲外の丸め」は採らない(Web が「黙って丸めず明示エラー」に決めたため。画面の表示と送る値のずれを作らない)。
- 同じ向き(上昇と上昇・下降と下降)は入力時に制約する: もう一方が上昇なら、こちらの「上昇」と「特化」プリセットを選べなくし、理由の一文を添える。
  もう一方の値を黙って書き換えない。

### 2. 画面の範囲(今回は最小。大きな再構成は F-12 と一緒)

- **置き場所**: 「技セレクタ」と「詳細」の間に「攻撃」「特攻」の 2 ブロックを**常に**出す(Web の「技 → 攻撃/特攻」の順に近づく)。
  design.md「数値の直接入力は詳細を開いたときだけ」の例外(Web と同じ。利用者の要望。ピルで打たずに済む点は変わらない)。
  design.md は実装 PR で同時に更新する(計算画面の図と箇条書き)。
- 各ブロック: 見出し(選んだ技が使う側に「(この技で使用)」を文字で足す。色だけに頼らない。技が無ければ強調しない)/
  SP の数値欄(`TextField`。数値キーボード。ラベル「攻撃のSP」「特攻のSP」)/ 性格補正の 3 択(上昇・補正なし・下降。`Menu` は使わない。ボタンの並び)/
  プリセットと一致しないときの「カスタム」の印。
- **プリセットのピルの行は今の場所・今の識別子のまま**(`attackerPreset-none|aFull|aMax`。既存 XCUITest が依存。位置を動かさない)。
  行は「選んだ技が使う側のブロック」のプリセットとして働く(`selectAttackerPreset(_:)` は使う側のブロックに値を入れる。ラベルは A/C 切り替えのまま)。
  選択状態は保持せず、使う側のブロックの値から導く(一致しなければ選択なし)。
  使わない側のブロックのプリセットは今回は出さない(数値・補正で調整できる)。ブロックごとのプリセットは F-12 の再構成で扱う(Web は各ブロックに持つ。差は意図的な縮小)。
- 「構築から選ぶ」(`.team`): 個体を呼んでいる間は**その個体の SP・性格で計算**する(入力欄の値は使わない・書き換えない・不正でも止めない)。
  入力欄・補正・ピルのどれを操作しても構築の選択は外れ、入力欄の値で計算する(個体の特性も外す。`selectAttackerPreset` と同じ)。
  個体の値を入力欄へ写すことは今回しない(後続。入力欄の下に `teamSourceNotice` の注記を出す)。
  お気に入りから呼ぶ処理(ADR-0513。別ブランチ)がマージされたら、同じ規則(呼んでいる間は個体の値・入力欄の操作で外れる)にそろえる。
- 数値キーボードに確定キーが無いので、キーボードのツールバーに「完了」(`calcKeyboardDone`)を置く。常時動くアニメーションは入れない。

### 3. 技の絞り込み(Web ADR-0328 に揃える)

- 計算画面の `moveOptions` を「learnset ∩ 検索結果 ∩ 変化技でない(`category != .status`)」にする(判定は `CalcMoveRules` の 1 か所。技名・ID を直書きしない)。
  既定の技は最初のダメージ技。種族を替えて選択中の技が選択肢に無ければ最初のダメージ技に選び直す。
- **構築から呼んだ個体の技が変化技だけ**でも、計算画面は変化技を選ばない(既定のダメージ技にする。個体の SP・性格は使う)。
- **ダメージ技を 1 つも覚えない種族**: 技欄は空(`moveId` は空・`selectedMove` は nil)、`noDamagingMovesNotice` を出し、計算要求を送らない
  (`error` は立てない。マスタの不具合ではないため)。入力は受け付ける。種族を替えれば計算が走る。
- `selectMove(id:)` は選択肢に無い技(変化技を含む)を無視する(計算しない)。
- **status-move の安全網**: 選択中の技が変化技のとき(保存データ・将来の入力経路)は計算要求を送らず `statusMoveNotice` を出す
  (`isStatusMoveSelected`)。現在の UI・VM の経路からは到達しないので、Web と同じく画面の自動テストは無く、判定 `CalcMoveRules.isStatusMove` を単体で固定する。
- 逆算・調整は変更なし(すでにダメージ技だけ)。判定画面は対象外(Web も別。判定レーンの持ち物)。
  **構築編集の技選択(4 技)は絞らない**(構築は変化技も持つ)。`TeamMemberConverter` の「最初の非変化技」の選び方は変えない。

### 4. 文言(Core の `AttackerStatLabels` に集約。Web の `ja.ts` と同じ語)

「攻撃」「特攻」/「(この技で使用)」/「攻撃の調整」「特攻の調整」/「攻撃のSP」「特攻のSP」/「攻撃の性格補正」「特攻の性格補正」/「上昇」「補正なし」「下降」/「カスタム」/
「攻撃のSPは0〜32の整数で入力してください」(特攻も同様)/「この性格補正の組み合わせに当たる性格が、データにありません」/
「攻撃と特攻の両方を上昇、または両方を下降にすることはできません」/「変化技はダメージを計算しません」/
「このポケモンはダメージを与える技を覚えないため、計算できません」/「完了」。iOS だけの語: `teamSourceNotice`。
Web の申し送り(2026-10-04)の「マスタにありません」は、出荷済みの `ja.ts`(「データにありません」。平易な語への変更)に合わせて後者を採る。

### 5. 識別子(追加のみ。既存は不変)

`attackerStatBlocks`(2 ブロックを包む `.contain`)/ `attackerStatBlock-atk|spa`(各ブロックの `.contain`)/ `attackerStatHeading-atk|spa`(見出し。ラベルに「(この技で使用)」)/
`attackerSPField-atk|spa`(`TextField`。value が文字列)/ `attackerSPError-atk|spa`(不正の理由)/ `attackerNature-<atk|spa>-<up|neutral|down>`(ボタン。選択は isSelected、無効は isEnabled == false)/
`attackerNatureGroup-atk|spa`/ `attackerCustomMark-atk|spa`(カスタムのときだけ存在)/ `attackerSameDirectionReason-atk|spa`(無効な選択肢があるブロックにだけ)/
`attackerStatTeamNotice`(構築の個体を呼んでいる間)/ `calcStatusMoveNotice`・`calcNoDamagingMovesNotice`(案内)/ `calcKeyboardDone`(キーボードの「完了」)。
**子が 1 つだけの `.contain` は識別子を畳むので、`.contain` のコンテナは子を 2 つ以上にする**(ブロックは見出し・欄・選択肢で 2 つ以上ある)。

### 6. 見た目・操作

デザイントークンのみ・`lineLimit` なし(折り返す)・タップ範囲 36pt 以上・`Menu` 禁止・常時アニメ無し。AX5(`dynamicTypeSize.isAccessibilitySize`)では性格補正の 3 択を縦に積む
(プリセットのピルの行と同じ考え方)。SP 欄は AX5 でも幅に収まる。

## 既存テストへの影響(仕様変更。実装者が理由つきで期待値を直す。削除・弱めない)

spec-writer は既存テストを触っていない(実装前は通る)。実装で次が落ちる。理由は仕様変更(Web の ADR-0328 §1・ADR-0329 §6 と同じ)で、期待値の変更理由をコミットメッセージに書く:

| テスト | 理由 |
|---|---|
| `CalcViewModelTests.testMoveOptionsAreAttackerLearnsetOnlyInLearnsetOrder` | 変化技が選択肢に出ない([アルファ専用, 特殊]) |
| `CalcViewModelTests.testAttackerWithOnlyStatusMovesFallsBackToFirstLearnsetMove` | 変化技へフォールバックしない(技なし・計算しない)。「変化技は atk に振る」の確認は `AttackerPreset` 単体(`AttackerPresetTests`)が持つ |
| `CalcViewModelTests.testEachInputChangeCallsCalcBulkExactlyOnceWithTheRightShape` | 特殊技へ替えても攻撃の値が残る(両方の SP を載せる。値は技で消えない) |
| `CalcViewModelDefenderRanksTests`(183 行付近)・`CalcViewModelConditionsTests`(256 行付近)の「変化技を選んで…」 | UI・VM から変化技を選べない。判定 `CalcMoveRules.isStatusMove` と `AttackerPreset.relevantStat`/`RankLabel` の単体に寄せる |
| `CalcViewModelTeamIndividualTests`(246 行付近、変化技だけの個体) | 計算画面も変化技を選ばない(既定のダメージ技) |
| `testSelectingSpecialMoveShowsCLetterPresetLabel` ほか `CalcScreenUITests` | 変更不要の見込み(ピルの識別子・位置・ラベルは不変)。位置が変わって通り越すときは操作だけ直す |

そのほか `selectAttackerPreset` を使う既存テストは、「使う側のブロックに値を入れる」で従来の要求と同じになる(物理・特殊とも)ので通る見込み。実装後に全件を流して確かめる。

## 検討した代替案

- **ブロックごとにプリセットのピルを 3 つずつ持つ(Web と同じ)**: 画面が縦に大きく伸び、既存の XCUITest(ピルの位置・`attackerPreset-*`)を壊す。F-12 の再構成で扱う。
- **SP の数値入力を「詳細」の中に置く**: 要望の「常に試せる」に反し、design.md の例外を作らずに済むだけで価値が無い。Web も常時表示。
- **入力値をステッパーで丸める**: 画面の表示と送る値がずれる。Web も不正は明示エラー。
- **構築の個体の値を入力欄へ写す**: 保存された SP が合計 66・攻撃/特攻以外を含むため、入力欄(攻撃・特攻だけ)に写すと情報が落ちる。注記で十分。
- **性格を `{plus, minus}` の構造値で送る**: 実在しない性格になり、API は natureId に写せない(Web と同じ理由で採らない)。

## 結果

- 利用者は攻撃・特攻の SP を 0〜32 の任意の値で、性格補正つきで試せる。選択肢にダメージ技だけが出る。
- 既定のままの要求は従来と同一(テストで固定)。engine・API・ゴールデン・services は変えない。
- 大きな再構成(カードの並べ替え・ブロックごとのプリセット・個体の値の写し込み)は後続(F-12)。
