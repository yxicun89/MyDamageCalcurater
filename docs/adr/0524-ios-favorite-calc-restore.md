# ADR-0524: iOS のお気に入りから計算の入力を復元して結果をすぐ出す — F-09

- 状態: 採用(2026-10-10)
- 関連: docs/usability-round2.md F-09(原文14)、ADR-0333(Web)、docs/ai-shared/decisions/2026-10-04-web-favorite-calc-restore.md・
  2026-10-04-api-favorite-calc-team-name.md、ADR-0228(`FavoriteInput.calc` / `Favorite.calc`)、ADR-0227(冪等・重複)、
  ADR-0511・ADR-0513(お気に入りの一覧・読み込み #613)、ADR-0519(履歴の復元。`loadHistoryCalc`)、ADR-0521(F-12 の部品)、CLAUDE.md 絶対ルール5

## 決定

### 1. 保存(計算画面の「攻撃側をお気に入りに追加」)

- `FavoritesService.addFavorite(label:individual:calc:)`(`calc` は `CalcHistoryCalc?`。2 引数版は `calc: nil` の便宜 extension)。
  `Favorite.calc` も同じ型(履歴の `CalcHistoryCalc` = `CalcRequest` の写像を共通で使う)。
- `CalcViewModel.attackerFavoritePin()` が今の計算入力から `FavoritePinTarget(label, individual, calc)` を作る。
  Web の表(ADR-0333 §1)と同じ規則で、API へ送るときに**既定のままの条件はキーごと省く**(`generatedFavoriteCalc`):

  | 計算画面の状態 | `calc` | 備考 |
  |---|---|---|
  | 攻撃側 種族・性格・SP・特性・持ち物・ランク・やけど | `attacker` | 今の計算要求の個体そのまま(プリセット/構築どちらも)。特性なし・持ち物なしは省く。ランクが全 0 なら `ranks` を省く。状態異常なしは `status` を省く |
  | 防御側 種族・特性 | `defender` | 個体は「無振り」(SP 全 0・無補正の性格)。特性は「おまかせ」なら省く |
  | 防御側 ランク | `defender.ranks` | 全 0 なら省く |
  | 防御側 持ち物 | `defender.itemId` | **メガ(持ち物がストーンに固定)のときだけ**そのストーン。iOS の防御側は単一の持ち物ではなく「比較する持ち物」の複数選択なので、それ以外は保存しない |
  | 技 | `moveId` | |
  | 天候・フィールド・防御側の壁 | `field` | 既定は `field` ごと省く。none・壁なしの項目も省く |
  | 急所 | `options.critical` | true のときだけ |
  | 形式 | `format` | 常に `single` |
  | 結果・比較する持ち物 | 保存しない | 復元時に計算し直す |

  `individual` は `calc.attacker` と同じ。`label` は Web と同じ「{攻撃側}→{防御側}({技})」(`FavoriteLabel.normalize` が 30 コードポイントで切る)。
  防御側の無補正の性格が見つからない・技が無いときは `calc` なし(従来の追加)。名前が引けないときは見出しなし(種族名で出る)。SP 不正・性格が決まらない・技未選択は
  従来どおり追加ボタンが何もしない(個体そのものを作れないため。`attackerIndividualForFavorite` と同じ)。
- 防御側のお気に入り追加は従来のまま(`calc` なし。防御側の個体だけ)。
- 重複・冪等(ADR-0227/0228): 同じ内容は 200 `created:false`、`calc` が違えば別のお気に入り。モックも `calc` を含めて同一判定する。

### 2. 復元(お気に入り画面の行)

- `calc` を持つ行に主ボタン「計算に使う」(アクセシビリティ名「「{見出し}」を計算に使う」= Web の `useLabel`。識別子 `favoriteUseButton-<id>`)。
  タップで履歴の行と同じく計算画面を push し、入力を復元して結果をすぐ出す。外す導線は変えない。
- 復元の実体は `CalcViewModel.loadFavoriteCalc(_:)` = `loadHistoryCalc` と同じ処理(`restoreCalc`。攻撃側の出どころの印だけ `FavoriteLoad.sourceTeamID` + お気に入り ID)。
  復元できるもの: 攻撃側の個体(種族・性格・SP・特性・持ち物・ランク・やけど・テラス)、技、防御側の種族・特性(種族が持たなければ落とす)・ランク、
  天候・フィールド・防御側の壁・急所。メガの持ち物固定は画面の既存規則が自動で当たる。
- **復元しないもの(画面の文言 `FavoritesLabels.restoreLimitNote` と本 ADR に明記。ADR-0519 と同じ限界)**: 防御側の性格・能力ポイント・持ち物
  (iOS は防御側を種族と代表調整の一覧で計算し、持ち物は比較の複数選択のため)、攻撃側の壁(シングルでは効かない)、ダブル形式(シングルで開く)。
  Web が保存した calc の防御側は無振りなので、結果の「無振り」の行は Web と一致する。
- 失敗: マスタに無い技・種族(マスタ更新後の `unknown_move` 等)は**計算の失敗として表示し、入力を書き換えない**(既存の `handleInputFailure`)。
  失敗時は「開きました」の案内を出さない(帯だけ)。お気に入り API が落ちても計算は使える(絶対ルール5。一覧の失敗は節の中だけ)。
- 文言(Web と同じ語): 行の案内 / 計算画面に「「{見出し}」の計算を開きました」+ 上の限界の1文。
- `calc` を持たない旧お気に入り: 行に「個体だけ…計算画面の『お気に入りから読み込む』で使えます」の案内(`legacyFavoriteHint`)を出し、
  従来の読み込み導線(#613。個体を入力欄に読み込む)は変えない(テストも変更なし)。

### 3. 部品・画面

行は F-12 の部品(`PillButtonStyle(.primary)`・`PopLabel`)。タップ 36pt 以上・AX 文字サイズで縦積み・lineLimit 不使用・トークンのみ。
モック: `POKECALC_MOCK_FAVORITES=calc`(402 復元できる〈9003→9001・とくしゅA・雨〉/ 401 技がマスタに無い / 400 calc なしの旧お気に入り)。

## Web・既存との違い

- Web は一括計算の画面で、防御側の SP・性格は使わず「反映していません」と案内する。iOS も一括計算(防御側 = 種族 + 代表調整)なので同じ限界。
  Web 決定ファイルの「iOS は calcDamage(単発)を呼ぶ想定」は、iOS の計算画面が `calcBulk` のため採らない(履歴の復元と同じ)。
- Web はボタン文言「攻撃側をお気に入りに追加」を変えない方針で、iOS も追加ボタンの語は変えない。

## 既存テストへの影響

`StubFavoritesService`(テスト補助)のメソッドを 3 引数版(`calc:` 付き)に移し、`calc` の記録 `addCalcs` を足した(2 引数の呼び出しは extension 経由で同じ)。
既存テストの期待値の変更なし。

## 受け入れ条件(テストで固定)

`CalcViewModelFavoriteCalcTests`(calc の作り方・見出し・条件・保存→復元の往復・1回だけ計算・出どころの印・calc なしは何もしない・失敗は入力を変えない)、
`FavoriteCalcRestoreTests`(本文のキー省略・条件つき・calc なしの本文は従来・一覧の写像・モックの同一判定と calc シナリオ・ピン留め・行・文言)、
`FavoriteCalcRestoreUITests`(一覧の導線・復元して結果・失敗の表示・追加→一覧→開く・AX5)。
