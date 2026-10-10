## 2026-10-10: 技の機構の段階3(I-data-7。データレーンから Web・iOS・API・record へ)
Decision: ADR-0144 の受け入れ条件と失敗するテストを置いた(実装はこの後)。残り HP・多段の回数を「対戦の状態」`battleState` として
`POST /api/calc`(`CalcRequest.battleState`)と WASM の `calc` に省略可で足し、残り HP で決まる 8 技・なげつける・分類の切り替えを計算に入れる
(印が残る技 31 → 21)。持ち物のなげつける威力を取り込み(migration 000017 `items.fling_power`・内部 API `MasterItem.flingPower`・公開 API `Item.flingPower`)、
くろいてっきゅうの接地を効果データ(`Grounds`)で表す。対戦の履歴が要る 15 技・フォルム依存 3・じゅうりょく・確率・わるあがきは印のまま。
Reason: ユーザー方針(2026-10-10。データレーンの推奨を採用)。入力の形は @smogon/calc 0.12.0 の curHP / hits と同じ実数値・回数(割合の丸めを契約に持ち込まない)。
Impact:
- 公開 API: 省略可の `CalcRequest.battleState`(`attackerCurrentHp`・`defenderCurrentHp` は 1..最大 HP の実数値、`hits` は範囲の多段技の最小..最大)と
  `Item.flingPower`。応答は変わらない(確定数が残り HP で数えた値になる。`defenderHP`・表示%は最大 HP のまま)。値域外は 400 `invalid_input`。
  一括・逆算・調整には足さない。API レーンの記録(state/api.md)の方針(省略可のキーの追加・生成型だけで読める)と矛盾しない。
- デプロイ順: migrate(000017)→ アプリ(pokedex-svc・calc-svc・Web の WASM)→ `make import-fetch` → 取り込み → master-release。
  000017 はマージ前に main の最新の版を確かめ、先に別の版が入っていたら繰り下げる。

### Web レーンへの依頼(画面。計算の写し `onlineSource.ts` の flingPower と WASM はデータレーンの PR で入れる)

**実装済み(2026-10-11。データレーンが代行。PR は後で。ADR-0340)。** 逸脱(critic 指摘で確定。iOS はこの規則に揃える): 計算画面は行ごとに防御側の SP が違う一括計算なので、
**防御側の残りHPは割合(%)だけの入力**(整数 1〜100。空・100 は満タンで送らない)。行ごとの実数値は `max(1, floor(行の最大HP × % / 100))`(行の最大 HP = 一括の結果の `defenderHP`)。
換算結果が行の最大 HP 以上ならその行には付けない。要約は「防御側 HP n%」。攻撃側は実数値のまま。状態を指定したときだけ各行を `battleState` つきの `calc` で計算し直す(同時 4 件まで)。
お気に入りの保存は実数値: 無振り(SP 0)の最大 HP(種族値 + 75)の行に換算して保存し、復元は `ceil(HP × 100 / 無振りの最大HP)`(上限 99)の % に戻す(行が特定できない近似)。
iOS が `calcDamage`(単発・防御側が1つ)なら、防御側の最大 HP が1つに決まるので実数値入力でもよい(その場合の保存・復元は実数値のまま)。

最終形(`battleState` を WASM の `calc` に渡す。オンラインの `POST /api/calc` も同じ形):

| 項目 | 置き場所 | 既定 | 入力・検証 | 文言案 |
|---|---|---|---|---|
| 攻撃側の残り HP | 計算画面の「対戦の状態」(折りたたみ。既定は閉じる) | 空 = 満タン(送らない) | 数値 1..攻撃側の最大 HP(実数値)。横に「/ 最大」と割合(小数第1位・切り捨て)を出す。範囲外は送らずに欄の下に誤りを出す | 見出し「攻撃側の残りHP」・補足「空なら満タンで計算します」・誤り「1〜{最大}で入力してください」 |
| 防御側の残り HP | 同上 | 空 = 満タン | 同上(防御側の最大 HP)。割合で入れたい人向けに「%で入力」の切り替えを置くなら、実数値への換算は `max(1, floor(最大 × % / 100))` で、送るのは実数値 | 見出し「防御側の残りHP」・補足「確定数は残りHPから数えます(%表示は最大HPに対する値のまま)」 |
| 多段の回数 | 技の下(技の `mechanismParams.multiHit` が `min < max` のときだけ出す) | 「既定」(送らない) | 選択肢 既定・min..max 回 | ラベル「回数」・既定の表示「既定(通常 {min+1} 回/スキルリンク等 {max} 回)」 |

- 種族・SP・性格・持ち物の変更で最大 HP が下がり、残り HP が最大を超えたら、残り HP を最大に合わせて「最大HP({n})に合わせました」と出す(黙って送らない)。
- 技を変えたら回数は「既定」に戻す。多段でない技・回数が固定の技では欄を出さず、送らない(送ると 400 / invalid_input)。
- 状態を入れて計算した結果には「対戦の状態: 攻撃側 HP {a}/{A}・防御側 HP {d}/{D}・{n} 回」を結果の近くに出す(何を前提にした数値か分かるように)。
- お気に入りに保存するとき `battleState` は**含めてよい**(record-svc が保存して返す。下の「record レーンへの依頼」の節)。保存した計算・履歴から開いた計算は、返された残り HP・回数を欄に復元する。
- テスト: 欄の表示条件(範囲の多段だけ)・既定は送らない・範囲外は送らない・最大の変更で丸める・お気に入りの保存・履歴から開いたときの復元。

### iOS レーンへの依頼

- `swift-openapi-generator` の再生成(省略可のキーなので既存のコードは壊れない)。計算は HTTP なので計算の写しは不要。
- 画面は Web と同じ最終形(上の表・文言・検証)。`CalcRequest.battleState` に入れて送る。お気に入りの保存にも含めてよい(record-svc が保存して返す)。

### API レーンへの連絡

- `CalcRequest.battleState` と `CalcBattleState` を足した(api/openapi.yaml)。calc-svc の検証(1 未満も `invalid_input`)と engine への写しはデータレーンの PR で入れる。
- 計算イベント(`calcevents.CalcDetail`)には指定があれば `battleState` を載せる(空のオブジェクトは載せない。record-svc の対応と同じ PR。下の依頼)。

### record レーンへの依頼(データレーンが実装済み)

- 2026-10-10 追記: ユーザー指示でデータレーンの同じ PR に入れた。record-svc はお気に入り(`FavoriteInput.calc`)で `battleState` を受けて保存し、返す
  (ADR-0228 の正規化: 省略・null・`{}` は省略のまま。指定したキーだけを保存。値域は `CalcBattleState` と同じ〈残り HP は 1 以上・回数は 1..10〉で、
  最大 HP との照合はしない〈マスタが要るため〉。範囲外は 400 `invalid_input`、未知のキーは 400 `unknown_field`)。
  計算履歴は calc-svc の計算イベント `CalcDetail` に `battleState`(omitempty)を足し、record-svc が同じ正規化で返す。
- **デプロイ順**: イベントの形の変更なので、record-svc(受け手)を先に入れ替えてから calc-svc。古い record-svc は未知のキー `battleState` を含む
  イベントを読めない恐れがある。DB のスキーマは変えない(お気に入りの snapshot・イベントの payload は JSON)。
- Web・iOS は、お気に入りの保存で `battleState` を外さなくてよい(Web への依頼の表の当初案「外す」は取り消し)。保存した計算・履歴から開いた計算は、
  保存した残り HP・回数を復元する。
