# ADR-0117: engine の入力の値域検査(種族値・タイプの重複・効果の補正値)

- 状態: 採用(2026-09-25。issue #255)
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #255、ADR-0108(件数の上限)、ADR-0010 §R(逆算)、ADR-0011(WASM 境界)

## 背景

ADR-0108 で件数の上限は入れたが、値の範囲は検査していなかった。WASM 境界は利用者のブラウザから
任意の種族値・効果を受け取るため、次が成功として通っていた(#255)。

- HP 種族値が巨大 → `koProbability` が HP+1 要素の配列を確保し、ネイティブでは数十 GB、WASM ではメモリ上限で回復できずに落ちる。
- 同じタイプを2つ持つ種族 → 相性を2回掛ける(抜群が4倍)。
- 負・0 の補正値 → ロールが非単調になる、全ロールが 1 に潰れる。


## 決定

### 1. 検査は `Individual.Validate` に集める

calc-svc・WASM・直接呼び出しのどれもが `Individual.Validate`(`CalcDamage` の入口と各境界の
`validateIndividual`)を通るため、そこに足す。エラーは既存の SP・ランクの検査と同じ素の `error` で、
境界が `invalid_input` に写す(新しい code は足さない)。

| 項目 | 範囲 | 定数 |
|---|---|---|
| 種族値(HP を含む全ステータス) | 1..255 | `MinBaseStat` / `MaxBaseStat`(types.go) |
| 種族のタイプ | 2つのときは互いに異なる | — |
| 持ち物の `DamageMod`・`PowerMod`・`BoostTypeMod`、特性の `StabMod`・`OffBoostTypeMod`・`ReduceSuperEffective` | 0(補正なし)または 1..2097152 | `MinEffectModifier` / `MaxEffectModifier`(damage.go) |
| 持ち物の `StatMods`、特性の `DefResistType` の値 | 1..2097152(0 は「補正なし」の意味を持たないので拒否) | 同上 |

種族値の上限 255 は本編の種族値が1バイトに収まることによる。pokedex のスキーマは `SMALLINT UNSIGNED`
で CHECK 制約を持たないため、engine の定数を単独の正にする(issue の既定案は「CHECK と同じ値」だが、
合わせる先が無い)。実データの取込対象の種族はすべてこの範囲に入る。

### 2. 補正値の上限は engine のいちばん広いクランプ(×512)に合わせる

issue の例は 1..65536(×16)だったが、上限は `powerModBounds` の上限 2097152(×512)にした。
理由: engine は各段階で `chainMods` の結果を `powerModBounds`(×0.01〜×512)・`statModBounds`
(×0.1〜×32)でクランプしており、既存の回帰テスト(issue #77 `TestChainModsClampAtCallSites`)が
×256 の補正でクランプの効き方を固定している。これより大きい値はどの段階でもクランプされて意味を持たず、
連鎖の途中の桁あふれだけを招くので、ここを上限にすると「意味のある入力」をすべて残したまま
桁あふれを防げる。`TestMaxEffectModifierMatchesWidestClamp` が両者を結ぶ。

下限は 1(×1/4096)。0・負は非単調・全ロール 1 を生むので拒否する。1 そのものは正の倍率として
単調な結果を返し、クランプの下限テスト(`StatMods`・`PowerMod` に 1)もこれを使うので残す。

## 影響

- 既存の正常な入力の結果は変わらない(ゴールデン・conformance・網羅テストは全件一致)。
- テストの fixture のうち、計算に使わないステータスを 0 のままにしていたもの(`mkIndiv` 等)は、
  範囲内の値で埋めるよう直した。検証している値は変えていない。
