# ADR-0120: 効果スキーマで表せる持ち物・特性をすべて効果定義に足し、表せないものを一覧で固定する(issue #270)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #270、ADR-0101(effects.json が本番の正)、ADR-0106(特性による無効・吸収)、ADR-0116(接地判定)、
  ADR-0117(効果の補正値の範囲)、ADR-0118(effects.json 2つの一致)、ADR-0002(ゴールデンの oracle)、
  #271(技の「未対応の印」)、#272(画面で特性を選べない)

## 背景

効果定義(`data/importer/effects.json`)は持ち物 11・特性 14 件だけで、定義の無い持ち物・特性は
`Effect=nil`(補正なし)として黙って計算されていた。importer の網羅性の報告(ADR-0103 §6)でも、
ダメージに効くハンドラを持つのに定義の無いもの(`effect-missing`)が 92 件あった。
そのうちタイプ強化の持ち物・半減きのみなどは、今の効果スキーマ(`engine.ItemEffect` / `engine.AbilityEffect`)で
そのまま表せる。

## 決定

1. **issue の既定案 A を採る**: 今のスキーマで oracle(@smogon/calc 0.12.0 の Champions 世代
   `dist/mechanics/champions.js`)と同じ補正になるものを、すべて効果定義に足す。値は oracle の実装から取る。
   - タイプ強化の持ち物 17 件: `BoostType` + `BoostTypeMod: 4915`(`getItemBoostType` の持ち物。威力の補正 bpMods)
   - ノーマルジュエル: `BoostType: normal` + `BoostTypeMod: 5325`(`<Type> Gem` の bpMods。1回の計算では消費を考えない)
   - 半減きのみ 16 件: `ResistBerryType`(`getBerryResistType`。抜群のとき、ノーマルは常に ×0.5)
   - Fire Mane: `OffBoostType: fire` + `6144`(攻撃実数値の補正 atMods)
   - Heatproof: `DefResistType {fire: 2048}`、Purifying Salt: `DefResistType {ghost: 2048}`(atMods。Thick Fat と同じ段)
   - Eelevate: `DefImmuneTypes [ground]` + `Airborne`(oracle は地面の無効も `isGrounded` も Levitate と同じに扱う)
   本番の定義と `testdata/golden/effects.json` を同じ内容で更新する(ADR-0118 のテストが一致を要求する)。
2. **1種ずつゴールデンで照合する**: `tools/golden/generate.mjs` が、タイプ・相性で効く効果(`BoostType` /
   `ResistBerryType` / `OffBoostType` / `DefResistType` / `ReduceSuperEffective`)ごとに、定義から
   「効く」(`effects/<id>/…/apply`)と「効かない対照」(`…/control`)を `fixed.json` に作る。
   半減きのみは 抜群(効く)/等倍(効かない)、タイプ強化は 同タイプ(効く)/別タイプ(効かない)。
   生成器は、同じ条件で効果を外したときの oracle のダメージと比べ、効く方が変わり対照が変わらないことを確かめる。
   Eelevate は ADR-0106 の無効ケースと ADR-0116 の接地ケースに足した。
   Champions の random のプールには足さない(乱数列が動き、既存の期待値がすべて変わるため)。
   engine の `TestGoldenCoversEveryChampionsEffect` が、legacy 以外の全定義が `fixed.json` にあること
   (タイプ・相性で効くものは効く/対照の両方)を確かめる。
3. **表せないものを一覧で固定する(回帰テスト)**: 生成器が Champions 世代の全持ち物・全特性を1つずつ攻撃側/防御側に持たせ、
   持たせないときとダメージが変わるものを数える(代表技 36 × 攻撃側 2 × 防御側 9 × 3条件。条件は
   通常 / すなあらし・急所・攻撃側やけどでHP1/3 / 晴れ・グラスフィールド・両者まひ)。
   「ダメージが変わるもの − 定義済み」が `tools/golden/unsupported-effects.json`(ID と理由)と一致しなければ止まる。
   逆に、定義があるのにダメージが変わらないものがあっても止まる。
   新しい効果の型を engine に足したら、一覧から外して定義に移す。
4. **取込時の補正値の上限**: `services/internal/master/effects.go` の `decodePositiveInt` に engine の
   `MaxEffectModifier`(×512)と同じ上限を入れる(PR #359 のレビューの軽微指摘)。上限を超える定義を取り込むと、
   その持ち物・特性を選んだ計算が毎回入力エラーになるため、取込の時点で止める。

## 表せないもの(未対応一覧)

正は `tools/golden/unsupported-effects.json`(理由付き)。調査条件でダメージが変わった持ち物 45・特性 62 のうち、
持ち物 4・特性 45 がここに残る。

- 持ち物: airballoon(持ち物による無効・浮遊)、grassyseed(フィールドでランク変化)、ironball(接地させる)、
  lightball(種族の条件)
- 特性:
  - 技タイプの変更: aerilate, dragonize, pixilate, refrigerate, liquidvoice, libero, protean
  - 相性・無効の変更(技の性質や天候): scrappy, bulletproof, soundproof, cloudnine, megasol, dryskin(ADR-0106 限界1)
  - 技の性質による補正: ironfist, megalauncher, strongjaw, sharpness, toughclaws, sheerforce, punkrock, technician,
    auraguard, fluffy, analytic
  - HP・状態異常・天候・フィールド・性別・急所の条件: blaze, overgrow, torrent, swarm, multiscale, guts, marvelscale,
    sandforce, solarpower, grasspelt, rivalry, sniper, battlearmor, shellarmor
  - 特性の実数値補正・威力段階のタイプ補正(スキーマに項目が無い): hugepower, purepower, hustle, furcoat,
    steelyspirit, fairyaura
  - 多段: parentalbond

## 対象外・限界

- **「補正未対応」の印(issue の既定案 B)はこの ADR では決めない**。応答に印を付けるか選択肢で区別するかは API/画面の
  変更で、技の未対応の印(#271)と同じ仕組みでまとめて決める。`unsupported-effects.json` はその印の元データに使える。
- 調査条件に無い条件(ダブル・天候以外の場の状態・特定の技だけで効く効果など)でだけ効くものは、この一覧に現れない。
  importer の `effect-missing`(Showdown のハンドラ基準)は別の基準の網羅性の指標として残る。
- ノーマルジュエル・Eelevate は Showdown のデータ上、importer の `effectHooks` に含めたハンドラを持たないため
  `effect-no-hook` の警告になる(Levitate と同じ)。定義は oracle で照合済みなので残す。
