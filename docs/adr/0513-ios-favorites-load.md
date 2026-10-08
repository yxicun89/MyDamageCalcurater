# ADR-0513: iOS の計算画面でお気に入りを攻撃側・防御側に読み込む

- 状態: 採用(受け入れ条件とテストまで。実装は未着手)
- 日付: 2026-10-04
- レーン: iOS(ダメージ計算レーンの iOS)
- 関連: ADR-0511(お気に入りの iOS 表示。§4「読み込む導線は別タスク」の続き)、ADR-0227(お気に入り API)、ADR-0327(Web のお気に入り画面)、
  ADR-0501「P6-2d」(構築から個体を呼び出す)・「P6-19」(防御側の特性)・「お気に入りの読み込みの受け入れ条件」、ADR-0509(持ち物の役割・メガ固定)、
  ADR-0507(画面レジストリ)、docs/ai-shared/decisions/2026-10-04-ios-favorites-load.md
- 対象外: 契約・API・Web の変更(`api/openapi.yaml`・Generated・services は触らない)。ダブル・テラスタル

## 背景

ADR-0511 は「お気に入りを計算に読み込む導線は今回やらない(Web レーンと揃える別タスク)」としていた。調べた結果、
**Web にも読み込みはまだ無い**(`web/src/screens/CalcScreen.tsx` にあるのは追加の `AddFavoriteButton` だけ。`CalcScreen.favorites.test.tsx` は追加の AC-6・AC-7 のみ。
`docs/plan.md` P5-3c の進行に「反映(お気に入りを計算に入れる)は P5-3d 候補」とあるだけ)。
したがって「Web に揃える」ことはできず、iOS の既存の「構築から呼び出す」(P6-2d)に揃えて先に作る。Web が後から作るときは、この ADR の規則(特に §3・§5)に合わせる。

保存されているもの(契約の `Individual`): 種族・性格・SP・特性・持ち物(+ ranks・status・テラスタイプ)。技は契約に無い。
防御側は、PR #598(ADR-0511)の判断で「無補正の性格・SP 0・画面で選んだ特性」の個体として保存している。

## 決定

1. **入口は2つ(攻撃側・防御側)。Menu ではなくシート**。計算画面の「構築から選ぶ」行(`teamSourceRow`)の直下に、同じ見た目(`glassCard`・幅いっぱい・タップ 36pt 以上)の独立した行を2つ置く。
   お気に入りは最大 100 件で `Menu` に向かないので、既存の技・種族ピッカーと同じシートにする。
   サービスが無い(`FeatureServices` に `FavoritesService` が登録されていない)ときは行を出さない(`FavoritePinSection` と同じ流儀の任意注入。`CalcFeature` の `requiredServices` に足さない)。
   識別子は追加のみ(§8)。
2. **シートの内容**: 見出しと注記(攻撃側「種族・性格・能力ポイント・特性・持ち物を読み込みます。技は変わりません。」/ 防御側「種族と特性だけを読み込みます。性格・能力ポイント・持ち物は使いません。」)の
   下に、お気に入りの行(サーバーの順。ラベルがあればラベル+種族名、無ければ種族名。マスタに無い種族の行も残し「不明なポケモン」)。
   **取得はシートを開いたとき1回**(`FavoriteLoadPickerViewModel.load()` を `.task` で1回。再読み込みボタンは新しい1回。古い応答は世代で捨てる。`FavoritesViewModel` と同じ規則。実装は包んでよい)。
   失敗・503・空はシートの中の案内だけ(取得の失敗を計算画面の `error` にしない。絶対ルール5)。行を選ぶとシートを閉じて `loadFavorite` を `scheduleLatest` で呼ぶ。
3. **何を設定するか**(`CalcViewModel.loadFavorite(_:side:)`。マスタ照合は純粋関数 `FavoriteLoad.plan`):
   - 攻撃側: 種族・性格+SP(組)・特性・持ち物。**技は変えない**(いまの技が新しい learnset にあれば残し、無ければ既定の規則=最初のダメージ技。`selectTeamIndividual` と同じ `reselectMove`)。
     出どころは構築から呼ぶ処理と同じ `.team(TeamIndividualSelection)` のスナップショットに写す(`teamID = FavoriteLoad.sourceTeamID`・`memberID` = お気に入りの id・
     `displayName` = ラベル ?? 種族名)。**新しい `BuildSource` の case は足さない**(`attackerPreset` は nil になり、ピルの選択表示が外れる。ADR-0501「P6-2d」§2 と同じ)。
     構築の行の表示は既存のフォールバック(構築が見つからなければ表示名だけ)にそのまま乗る。
   - 防御側: 種族と特性だけ(性格・SP・持ち物は使わない。攻撃側として保存した個体を防御側に読んでも同じ)。
   - **ranks・status・天候・急所・やけど・壁などの画面の計算条件は変えず、お気に入りが持つ ranks・status も持ち込まない**(`selectTeamIndividual` の `individualForRequest` が ranks/status を落とすのと同じ。
     防御側も `selectDefender` と同じで defenderRanks・持ち物の比較は残す)。テラスタイプは `individualForRequest` が通すとおりスナップショットに残る(画面に UI は無い)。
4. **マスタに無いものは黙って落とさない**(読めた分だけ設定して `favoriteLoadNotice` で案内。全部読めなければ何も変えず計算もしない):
   - 種族が `species(key:)` で `not_found` → `.speciesMissing`(何も変えない・計算しない)。通信失敗など他の失敗 → `.unavailable`(何も変えない・計算しない・画面の `error` にしない)。キャンセルは案内なし。
   - 性格が `natureOptions` に無い → 性格と SP の組を使わず(`.nature`)、出どころはいまのプリセットのまま。種族・特性・持ち物は設定する。
   - 特性が種族の `abilities` に無い → `nil`(`.ability`)。持ち物が `itemOptions` に無い → `nil`(`.item`)。
   - `FavoriteLoadNotice.partial([...])` は `FavoriteLoadDropped` の宣言順(nature, item, ability)。防御側で「使わない」項目は落としたことにしない。
5. **持ち物の役割・メガ固定(ADR-0509)**: メガ種族は持ち物をストーンに固定する(保存が nil なら案内なし・保存が別の持ち物なら `.item`)。ストーンがマスタに無い(`.missing`)なら nil(保存があれば `.item`)。
   非メガ種族のメガストーンは `.item` で落とす。**役割(ItemRoleFilter)に反するだけの持ち物は残して案内もしない**(ADR-0509 §2「選択済みの値は残す」。`attackerItemOptions` は `keeping:` で残す)。
   防御側がメガのときは、詳細を読んだ結果(ストーン固定)を**同じ1回の計算に反映**する(2回目の計算をしない)。
6. **計算と通信の回数**: 読み込みは計算ちょうど1回(全部読めなかったときは0回)。`species(key:)` は新しい種族につき1回(攻撃側は `reloadAttackerMoveOptions` の詳細、
   防御側は `loadDefenderDetail` の詳細を使い回す。重ねて読まない)。防御側は読み込みで `defenderAbilityOptions`・`defenderAbilityOptionsSpeciesKey` を整合させるので、
   View の `.task(id: defenderSpeciesKey)` の `loadDefenderAbilityOptions()`(P6-19)は何もしない(`species(key:)` も計算も増えない)。攻撃側も `attackerAbilityOptions` を新しい種族に整合させる。
   世代は `beginInput()` で守る(追い越された古い読み込みは状態・案内・計算・エラーのどれも変えない。古い計算の失敗も出さない)。
7. **案内の寿命と文言**: `favoriteLoadNotice` は、新しい `loadFavorite` の開始・次の入力操作(`beginInput()`)・`dismissFavoriteLoadNotice()` で消える。
   文言は `FavoritesLabels` の追加(`FavoriteLoad.swift` の extension。既存の固定文言は変えない)に集約し、`FavoriteLoadNotice.text` が返す。サーバーの code・英語の message は出さない。
   計算画面では、2つの入口の下(または結果の上)に1つの `Text`(`favoriteLoadNotice`)として出す。常時アニメなし・`lineLimit` なし・design.md のトークンのみ。
8. **識別子(追加のみ。既存は不変)**:

   | identifier | 要素 |
   |---|---|
   | `attackerFavoriteSourceButton` / `defenderFavoriteSourceButton` | 計算画面の入口(`teamSourceRow` の直下。サービスがあるときだけ) |
   | `favoriteLoadSheet` | シート全体(`.contain`。**子が1つだけの `.contain` は識別子を畳む**ので見出し・注記・一覧など子を2つ以上に) |
   | `favoriteLoadNote` | 注記(攻撃側/防御側) |
   | `favoriteLoadRow-<favoriteId>` | 行(ボタン。タップ 36pt 以上) |
   | `favoriteLoadEmpty` / `favoriteLoadError` / `favoriteLoadRetry` / `favoriteLoadClose` | 空・失敗の案内・再読み込み・閉じる |
   | `favoriteLoadNotice` | 計算画面に出す読み込み結果の案内 |
9. **モック**: `MockFavoritesScenario.loadable`(`POKECALC_MOCK_FAVORITES=loadable`)を追加。304「読み込める」(9002-000・攻撃上昇・SP 0/32/0/0/2/32・特性 test-ability-beta・持ち物 test-item-berry)・
   303「一部だけ」(9004-000・特性と持ち物がマスタに無い)・302「種族なし」(9999-000)・301(ラベルなし・9003-000・特性 test-ability-gamma)。既存のシナリオの既定は変えない。
10. **Web との関係**: Web は未実装(上記)。Web が実装するときは §3(攻撃側=個体・防御側=種族と特性・ranks 等は持ち込まない)・§4(黙って落とさない)に揃える。この ADR は Web の仕様を決めない。

## 代替案

- **お気に入りを `Menu` にする**: 最大 100 件で現実的でない(技・種族と同じ理由)。却下。
- **新しい `BuildSource` の case(`.favorite`)を足す**: `buildRequest` と `individualForRequest` の分岐が増え、ピル/構築の排他の規則と二重になる。スナップショットは `.team` と同じ値なので `.team` に写す。
  代償は、構築の行の見かけ(構築が無いのでフォールバックで表示名だけ)が「構築から選ぶ」行に出ること。実装時に見た目が紛らわしければ `TeamSourceMenuRow` に由来の表示を足す(識別子は変えない)。
- **ranks・status も復元する**: 画面の計算条件(攻撃側のランク・やけど)を黙って書き換えると、前の結果と食い違う。構築から呼ぶ処理に揃えて復元しない。
- **取得をシートでなく画面の起動時に1回**: 使わない人にも通信する・古くなる。開いたとき1回(ADR-0511 §2 の「画面を開くたび」と同じ考え方)。
- **全部読めなくても計算する**: 変えていない状態での計算は意味がなく、結果が増えて紛らわしい。何も変えないときは計算しない。

## 結果・影響

- 計算画面の View を足す(入口2行・シート・案内)。`CalcViewModel` に `favoriteLoadNotice`・`loadFavorite`・`dismissFavoriteLoadNotice` を足す。Core に `FavoriteLoad.swift`(純粋関数・シート VM・文言)を足す。
- 既存の経路(プリセット・構築から呼ぶ・防御側の選択・防御側の特性の `.task`)は変えない。既存の識別子・テストは不変。
- 契約・API・Web・engine は変更なし。

## 追記(2026-10-04): 実装後の補足(critic 指摘)
- 「取得に失敗したときは何も変えない」の保証は、**種族の詳細(`species(key:)`)の取得まで**。詳細を取得して種族・性格・SP・特性・持ち物を反映したあとに、技の再選択(`reselectMove`)が失敗した場合は、通常の種族変更(`selectAttackerSpecies`)と同じく、種族だけが変わった状態で入力エラーとして終わる(技は現在の技を優先して再選択する)。
- 性格と SP が読めず落としたとき(`plan.natureId == nil`)は、出どころ(`attackerBuildSource`)を直前のままにする(仕様どおり)。直前が別の構築の `.team` だった場合は、その性格・SP が新しい種族に適用される。
- 出どころを `.team(teamID: "favorite")` にするため、構築選択行は `teamOptions` に無い選択を受け取る。構築の再選択・再保存とは衝突しない。
