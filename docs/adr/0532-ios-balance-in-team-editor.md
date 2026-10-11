# ADR-0532: iOS の構築編集に「タイプバランス」を組み込み、別画面と入口を廃止する

- 状態: 採用(2026-10-12。iOS レーン。ADR-0421 段階 2'・4 と ADR-0422 段階 C の iOS 分)
- 関連: ADR-0421(構築の中のバランス)、ADR-0422(おすすめの改善・タイル・相性の色)、ADR-0415(iOS のタイプバランス画面)、
  ADR-0522(構築の 6 枠)、ADR-0531(ナビゲーション案 B。`FeatureEntry.link`)、ADR-0529(画像)、Web の I-web-21〜23

## 背景
ADR-0421 で、タイプバランスは構築編集の中の節にし、別画面を廃止することになった。iOS では構築一覧の行(`openBalanceScreen`)から
別画面(`BalanceScreenView`)を開き、1 体ずつ追加するか「構築から読み込む」で入れていた。ADR-0422 でおすすめを上位 5 個にし、
該当ポケモンのタイル(タップで空き枠へ)と相性の色(耐性=水色系・無効=灰)を足すことになった。Web は I-web-21〜23 で実装済み。

## 決定

### 1. 構築編集の中の節(`TeamBalanceSection`)
- `TeamEditView` の 6 枠の下に「タイプバランス」の節を置く。入力は下書き(`TeamEditViewModel.balanceParty` = 種族を選んだ枠だけを枠の順)。
  保存しなくても結果が出る。下書きが変わるたびに `.task(id: balanceParty)` で `BalanceViewModel.setParty` に渡す
  (同じ種族・特性・技なら送り直さない。古い応答の無視・待ちは BalanceViewModel の中。構築側はそれを知らない)。
- 待ちは `BalanceViewModel.teamRefreshDebounce`(300ms。ADR-0421 §2)。おすすめは従来どおり 1 秒の専用の待ち(サーバーの重い計算の同時数を絞るため。ADR-0415 §8)。
- 0 体は案内の一言(Web と同じ「ポケモンを選ぶと、タイプの強み・弱みがここに出ます」)だけで、balance を呼ばない。
- 中身は従来の結果ビューを節として再利用: 防御相性・チームの集計・攻撃範囲(`BalanceResultsView`)、仮想敵(`BalanceThreatsView`。入力欄つき・保存しない)、
  おすすめタイプ(`BalanceRecommendationsView`)、技範囲チェッカー(`BalanceMoveRangeView`)。
- エラーは節の中だけ(各部の ErrorBanner)。どれかが失敗したら節に「再計算」(`BalanceViewModel.retry(party:)`: 下書きが今のメンバーと違えば引き直し、同じなら送り直す)。
  文言は `BalanceErrorText`(禁止語 API・マスタ・リクエストを含まない。`GlossaryTests`)。構築の編集・保存は balance と無関係に動く。
- 画面を離れるときの `cancel()` は呼ばない: BalanceViewModel は編集画面と同じ寿命で、タブの切り替え(ADR-0531 の保持)で隠れても計算を止めない。

### 2. 別画面と入口の廃止
- `BalanceFeature`(`.link` 登録)・`BalanceScreenView` を削除。`BalanceService` の登録は `TeamListFeature.registerServices` に移す
  (`requiredServices` には入れない。balance が無くても構築は開ける)。
- `FeatureEntry.link(in:)`・`FeatureLink`・`FeatureContext.links` は使う画面が無くなったので削除(必要になったら ADR-0531 の形で戻す)。
- `BalanceViewModel.loadTeam` は `setParty` と同じ働きなので削除し、テストは `setParty` に書き換えた(検査は同じ)。
  `addMember`・`removeMember` などメンバーの操作は、ViewModel の既存テスト(計算の独立・世代・debounce の検査)の入口として残す。
- 文言 `addMemberLabel`・`loadTeamLabel`・`emptyNotice`・`emptyTeamLoadNotice` は使う所が無くなったので削除。

### 3. おすすめ(ADR-0422 §1・§2)
- 構築の節は `limit: 5`(`BalanceViewModel.teamRecommendationsLimit`。`init(recommendationsLimit:)`。既定 nil=送らないは従来のまま)。
- 契約 0.10.0 の `skewedAttackTypes` を `BalanceRecommendations.skewedAttackTypes` に読む(無ければ空。知らないタイプは decode エラー)。
  今は表示しない(Web も出していない。除外はサーバーが済ませる)。
- 候補ごとに該当ポケモンのタイル(`SpeciesImageView` = 画像か、無ければタイプ色のエンブレム + 名前)。1 候補で 8 個まで(`BalancePickTiles`)、
  残りは「もっと見る」/「閉じる」。並びは応答のまま。タップで `TeamEditViewModel.addMember`(最初の空き枠)。空き枠が無ければ押せず、
  「空き枠がありません。枠を外すと入れられます」を出す(`hasEmptySlot`)。バランス側は枠の構造を知らない(`BalancePick` で構築側が渡す)。
- タイルは遅延しない列の並び(`BalanceTileColumns`。最小幅を `@ScaledMetric` で広げ、AX5 では 1 列)。タップ領域は 36pt 以上。

### 4. 相性の色(ADR-0422 §3)
- `BalanceMatchupTone`(PokeCalcCore): 防御は category から、攻撃範囲は倍率から(値の範囲で判定し直さない)。
  弱点・抜群 = `danger`(赤)、耐性・いまひとつ = `matchupResist`(水色系)、無効 = `matchupImmune`(灰)、等倍・攻撃技なし = 文字の既定色。緑は使わない。
- 色だけにしない: 印(上向き・下向き・×・横線。Web の G-05 と同じ)と文字(「×2 弱点」)を並べる。印は読み上げない。

### 5. UI テスト用の架空の応答
- モック構成の既定は従来どおり `UnavailableBalanceService`(利用者に架空の相性表を見せない。ADR-0415 §4)。
- `POKECALC_MOCK_BALANCE=fixture` のときだけ `MockBalanceService`(架空の応答。名前は「テスト」で始める)。結果・タイル・満杯の XCUITest が使う。
  形は `MockBalanceServiceTests` が固定する。

## 受け入れ条件
1. 構築編集で 1 体選ぶだけで、節に防御相性・集計が出る(保存しなくてよい)。枠を外すと追随する。空の枠・技 0 個でも落ちない。
2. balance が使えなくても、構築の編集・保存ができ、エラーは節の中だけ(再計算つき)。
3. 構築一覧・各タブに別画面への入口が無い。
4. おすすめは limit 5。タイルは 1 候補 8 個 +「もっと見る」。タップで最初の空き枠に入る。満杯なら押せず理由を出す。
5. 弱点・抜群は赤、耐性・いまひとつは水色系、無効は灰。印と文字を併記。
6. `skewedAttackTypes` を読める(無い応答は空)。

## 代替案と不採用の理由
- 別画面を残して構築から開く: ADR-0421 の決定(手間が二重)に反する。
- 技範囲チェッカーを外す: Web の節には無いが、iOS の既存機能を黙って減らさない(構築の枠を使わない入力なので節の末尾に置いた)。外すなら別に決める。
- タイルを `LazyVGrid` で並べる: 画面外のタイルが作られず、XCUITest がスクロールしないと引けない。件数は最大でも 5 候補 × 全件なので遅延は要らない。
- 構築の節でも `cancel()` を onDisappear で呼ぶ: タブの保持(ADR-0531)で隠れて戻ると、同じ下書きでは送り直さないので結果が欠けたままになりうる。
