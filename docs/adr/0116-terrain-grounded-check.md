# ADR-0116: フィールド(地形)の補正に接地判定を入れる(issue #231)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #231(#289 を統合)、ADR-0005(M1 は常に接地とみなす → 本 ADR で解消)、ADR-0106(特性による無効・吸収)、
  ADR-0013(相性表はデータ)、ADR-0002(ゴールデンの oracle)、#271(技固有の処理の「未対応の印」)

## 背景

`engine/modifiers.go` の `terrainDamageMod` は地形と技タイプだけを見ており、ひこうタイプや特性ふゆうで
浮いている個体にもエレキ/グラス/サイコフィールドの ×1.3、ミストフィールドのドラゴン ×0.5 を掛けていた。
ゴールデンの生成器(`tools/golden/generate.mjs`)は地形ありのケースからひこうタイプを意図的に除外していたため、
この誤りは照合で見つからなかった。

oracle(@smogon/calc 0.12.0 の Champions 世代 `dist/mechanics/champions.js`)の規則:

```js
// util.js
function isGrounded(pokemon, field) {
  return field.isGravity || pokemon.hasItem('Iron Ball') ||
    (!pokemon.hasType('Flying') && !pokemon.hasAbility('Levitate', 'Eelevate') && !pokemon.hasItem('Air Balloon'));
}
// champions.js(威力の補正 bpMods)
if (isGrounded(attacker, field)) { Electric×Electric / Grassy×Grass / Psychic×Psychic → 5325 }
if (isGrounded(defender, field)) { Misty×Dragon / Grassy×(Bulldoze|Earthquake) → 2048 }
```

## 決定

1. **接地判定は engine の純粋関数 `isGrounded(Individual)`**(`engine/modifiers.go`)。
   「ひこうタイプでない かつ 特性の効果が `Airborne` でない」なら接地。威力を上げる3つは攻撃側、
   ミストのドラゴン半減は防御側の接地を見る(oracle と同じ)。
2. **「浮いている」は特性の効果データ `AbilityEffect.Airborne`(bool)で表す**。名前(ふゆう)をコードに書かない
   (CLAUDE.md)。本番の定義 `data/importer/effects.json` とゴールデンのアダプタ `testdata/golden/effects.json` の
   ふゆうに `"Airborne": true` を足した。
   - `DefImmuneTypes` に ground を含むかで代用しない(issue の既定案からの変更)。地面の無効と浮いていることは別の性質で、
     地面技を吸収する どしょく(`DefAbsorbTypes`)は接地しているため、代理にすると将来の特性で誤る。明示の項目にすれば
     地面の無効(`DefImmuneTypes`)と接地(`Airborne`)を独立に書ける。ふゆうは両方を持つ。
   - 境界: 共通マスタ `services/internal/master/effects.go` は `Airborne` を `true` のリテラルだけ受ける(`IgnoresBurn` と同じ。
     false は省略で表す)。WASM の DTO は `airborne`、Web の型は `airborne?: boolean`。
     OpenAPI の `MasterEffect` は効果 JSON をそのまま通す(`additionalProperties: true`)ので契約の変更は無い。
3. **ひこうタイプの判定は `TypeFlying`(タイプ ID)で行う**。天候の防御補正が `TypeRock`・`TypeIce` を名指しするのと同じ
   「ゲーム機構が名指しするタイプ」(ADR-0013 §2)。相性表をコードに戻す話ではないので、
   `TestNoTypeChartTableInEngineSource` の禁止リストから `TypeFlying` を外し、`TestTypeChartFixtureCoversRuleTypes` の
   ルールが参照するタイプに足した。
4. **テラスタイプは見ない**。oracle の `hasType` はテラス中ならテラスタイプを見るが、engine は一致判定・相性も素の
   `Species.Types` で計算している(ADR-0005、#232・#315)。テラスの扱いは接地判定も含めてそちらでまとめて変える。
5. **ゴールデン**: 生成器から「地形ありならひこうタイプを除外」を外した(固定ケースの assert と、Champions の random・
   legacy-random の種族プール)。乱数列は同じで、プールだけが変わるので random.jsonl.gz(10,000 件)と
   legacy-effects.jsonl.gz(3,851 件)を再生成した。地形ありのうち、ひこうの攻撃側 787 件・防御側 828 件(random)、
   309 件・306 件(legacy)。固定ケースに `terrain-grounding/*` 11 件(ひこう・ふゆうの攻撃側/防御側と、
   反対側だけが浮いている対照)を足し、ふゆうの `levitate/combined` にミストフィールドを足した。
   `engine/golden_immunity_test.go` の「P2-3b で変わらないファイル」の sha256 は再生成後の値に更新した。
   known_diffs には何も足していない(全件一致)。接地判定を外した engine では固定 7 件・random 等が不一致になることを確認した。

## 対象外(引き続き未モデル化)

- **じゅうりょく・くろいてっきゅう**(必ず接地): Field にじゅうりょくが無く、効果定義にくろいてっきゅうも無い。
- **ふうせん**(Air Balloon): Champions 世代に在るが、浮くだけでなく地面技を無効にする持ち物(oracle `champions.js` L186)。
  持ち物による無効は ADR-0106 の結果(`DamageResult.Nullified`。特性による無効・吸収)の外にあり、`ItemEffect` に
  「浮く」だけを足すと地面技が当たってしまう。持ち物の無効をモデル化するときに `ItemEffect` へ足す(効果データの追加)。
- **でんじふゆう・テレキネシス・ねをはる** 等の場の状態(issue #231 の対象外)。
- **フィールド固有の技の処理**: グラスフィールドの じしん・じならし半減、サイコフィールドの先制技無効、
  ワイドフォース・だいちのはどう 等(#289 から引き継いだ内容。#271 の「未対応の印」で扱う)。
  ゴールデンの代表技にはこれらが無い(metadata.json の exclusions)。

## 影響

- ひこうタイプ・ふゆうの個体が絡む地形ありの計算結果が変わる(これまでの値が誤り)。calc-svc と WASM は同じ engine を使う。
- pokedex の効果定義はコミット済みの `data/importer/effects.json` から入るので、次の `make import` でふゆうに `Airborne` が入る。
  入るまでは DB のふゆうに `Airborne` が無く、calc-svc の結果だけが旧挙動のまま(DB の再取込が要る)。
- 型バランスの read model(`services/pokedex/internal/readmodel`)は防御相性の効果だけを写すので、`Airborne` は出さない。
