## 2026-10-04: お気に入りの calc の Web の保存形と復元規則(Web レーンから iOS〈ios-6f〉へ)
Decision: Web はピン留め時の計算画面の入力を `FavoriteInput.calc`(CalcRequest)に保存し、一覧の「「{見出し}」を計算に使う」で
計算タブに戻して一括計算を走らせる(ADR-0333)。
- Web が保存する calc: format=single / attacker = 種族・特性(実際に使う ID)・持ち物・SP は atk/spa のみ・natureId・ranks(atk/spa のみ、0 なら省略)・status(burn のみ)/
  defender = 種族の無振り個体(sp 全 0・無補正の natureId)+ 特性(おまかせは省略)・持ち物・ranks(def/spd のみ)/ moveId /
  field = weather・terrain・defenderScreens(既定は省略)/ options.critical(true のときだけ)。attackerScreens・teraType は作らない。
- individual は calc.attacker と同じ。label は「{攻撃側}→{防御側}({技})」(30 コードポイントで切る)。
- 復元: Web は一括計算(bulk。防御側はプリセット5行)なので、calc.defender の SP・性格・テラス等は使わず、Web で表せない項目は「反映していません」と案内する。
  iOS は calc 全体を戻して calcDamage(単発)を呼ぶ想定のままでよい(Web が保存した calc は無振りの防御側として計算される)。
  この差(Web = bulk / iOS = calcDamage)は意図したもの。
- 引けない種族・技・持ち物・特性は、引けた部分だけ戻して項目を明示する(お気に入りは消さない)。メガの持ち物は固定を優先して知らせる。
- 文言: 行のボタン「「{見出し}」を計算に使う」、計算画面の案内「「{見出し}」の計算を開きました」、旧お気に入り(calc なし)の案内「攻撃側だけ」。iOS も同じ語にする。
Reason: F-09(原文14)。Web と iOS で同じお気に入りを開けるようにし、差(一括計算 / 単発計算)を明記する。
Impact: iOS は calc を作るとき Web と同じ省略規則に揃えると、同じ計算の重複判定(ADR-0227)が端末をまたいで効く(必須ではない)。
