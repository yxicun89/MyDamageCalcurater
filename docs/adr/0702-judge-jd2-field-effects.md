# ADR-0702: 判定(素早さ×ダメージ連動)JD2 場の効果(トリックルーム・追い風)

- 状態: 採用(2026-09-23)
- 日付: 2026-09-23
- 関連: ADR-0700(JD0 基盤。契約の分割方針・上流の事情を漏らさない立場)、ADR-0701(JD1 の request/response・
  素早さの求め方・こだわりスカーフの丸め)、docs/judge-design.md §3 JD2、
  ADR-0002(ゴールデンテストの照合先 @smogon/calc 0.12.0 の完全固定)、
  ADR-0600 §3(スカーフ ×1.5 をサービスが持つ前例)、ADR-0602(同速を真偽に丸めない前例)、
  CLAUDE.md ドメイン規約(4096 基準の固定小数と五捨五超入。float で近似しない)

## 背景

JD1 で `POST /api/judge/v1/outspeed-and-ko` が「抜けるか + 倒せるか」を返せるようになった。
JD2 は judge-design.md §3 の JD2、**場の効果(トリックルーム・追い風)**を同じ endpoint に足す。

どちらも**ダメージには一切関与せず、行動順にだけ効く**。calc-svc が場の効果として理解するのは
weather / terrain / screens だけで(judge の `FieldState` はそれを転送するためだけに存在する)、
トリックルーム・追い風を送る先が無い。したがって JD2 で決めるのは「judge だけが解釈する欄をどう足すか」と、
「追い風とこだわりスカーフをどの順でどう丸めるか」の 2 点になる。

後者は CLAUDE.md のドメイン規約(4096 基準の固定小数と五捨五超入)に直接かかる決定なので、
**推測せず @smogon/calc 0.12.0(ADR-0002 が固定した版)の実装を読んで確かめた**(§2 の出典)。

## 決定

### 1. `speedField` を request の新しい欄として足す(`field` とは分ける)

`OutspeedAndKoRequest` に省略可の `speedField` を足す。

| 欄 | 既定 | 意味 |
|---|---|---|
| `trickRoom` | false | トリックルームがかかっているか |
| `attackerTailwind` | false | 自分の側に追い風がかかっているか |
| `defenderTailwind` | false | 相手の側に追い風がかかっているか |

- **`field`(calc-svc へ転送する)には入れない**。calc-svc に理解できない欄を転送すると、
  calc-svc の `DisallowUnknownFields` に当たって 400 になるか、黙って捨てられて「指定したのに効かない」になる。
  どちらも呼び出し側から原因が見えない。judge が自分で解釈する値は judge の欄に置く
  (ADR-0701 §7「契約は judge が自分で持つ」と同じ立場)。
- **`speedField` は calc-svc に送らない**。judge が解釈して使い切る。
- `speedField` 自体の省略も、個々の欄の省略も、すべて false(場の効果なし)として扱う。
  したがって `speedField` を送らない request の応答は JD1 と 1 ビットも変わらない(後方互換)。
- 未知の欄・真偽値でない値は、既存の body 検査(`DisallowUnknownFields` + 型)に載って
  400 `invalid_request` になる。検査順(ADR-0701 §5)は変えない — `speedField` は上流を 1 回も
  呼ばない段階で読み終わる。

### 2. 追い風は実数値を ×2 する / 補正は 4096 基準で連結してから 1 回だけ丸める

**出典(実際に読んで確認した)**: `@smogon/calc` 0.12.0 の `dist/mechanics/util.js` の `getFinalSpeed`。
Champions 世代(`Generations.get(0)`、`dist/mechanics/champions.js`)も `computeFinalStats(..., 'spe')` 経由で
同じ `getFinalSpeed` を通る。要点は次の 3 つ:

1. ランク補正済みの素早さに対し、効いている補正を**配列に集める**。
   追い風 `speedMods.push(8192)`(= ×2)、こだわりスカーフ `speedMods.push(6144)`(= ×1.5)。
   **配列に積む順は追い風が先、持ち物が後**。
2. 配列を `chainMods(speedMods, 410, 131172)` で **1 つの補正にまとめる**。
   1 ステップは `M = (M * mod + 2048) >> 12`(4096 基準の**切り上げ寄り**の丸め)。
3. まとめた補正を **1 回だけ** 適用する: `speed = pokeRound(speed * M / 4096)`。
   `pokeRound` は `num % 1 > 0.5 ? ceil : floor` = **五捨五超入**(ちょうど .5 は切り捨て)。

したがって judge の素早さは次の順で求める(ADR-0701 §2 の順序を拡張する)。

```
実数値(engine.RealStats)
  → ランク補正(engine.EffectiveStat)
  → 素早さ補正を 4096 基準で連結(追い風 8192 → こだわりスカーフ 6144)
  → 連結した補正を 1 回だけ五捨五超入で適用
```

- 連結の 1 ステップは `(M*mod + engine.Modifier4096/2) / engine.Modifier4096`(切り上げ寄り)、
  最終適用は ADR-0701 §2 と同じ `(v*M + engine.Modifier4096/2 - 1) / engine.Modifier4096`(五捨五超入)。
  **丸めの向きが 2 か所で違うのは原典どおり**で、誤記ではない。
- engine の `chainMods` / `pokeRound` は非公開(小文字)なので judge から呼べない。
  ADR-0600 §3 / ADR-0701 §2 が「engine に無い素早さの補正は judge が名前付き定数と式で持つ」と決めた
  のと同じ扱いで、judge 側に同じ式を置く。定数名は `tailwindSpeedModifier = 8192` /
  既存の `scarfSpeedModifier = 6144`、基準は `engine.Modifier4096` を使い、4096 を直書きしない。
- 上下限(`chainMods` の 410/131172、速度の 0/10000)は**入れない**。judge が持つ補正は現状 8192 と 6144 の
  2 つだけで、連結結果は 4096〜12288、素早さの最大は種族値 255・ランク +6・追い風 + スカーフでも 4000 台に収まり、
  どの上下限にも当たらない。当たらない分岐を書くと、テストで踏めないコードが残る。

**なぜこの丸め方でないといけないか(このタスクの技術的な要)**: 「スカーフを掛けて丸めてから追い風で ×2」
にすると答えがずれる。実数値 91(種族値 71・SP 0・無補正)にスカーフと追い風が両方乗る場合:

| やり方 | 計算 | 結果 |
|---|---|---|
| **連結して 1 回丸める(採用)** | M = 12288、`pokeRound(91 × 12288 / 4096)` = 91 × 3 | **273** |
| スカーフを先に丸めてから ×2 | `pokeRound(91 × 1.5)` = 136 → ×2 | 272 ✗ |
| 追い風 ×2 を先に(丸め不要)→ スカーフ | 182 → `pokeRound(182 × 1.5)` | 273 |

追い風は ×2 ちょうどで整数を保つため、「追い風を先に掛ける逐次適用」は採用案と常に一致する。
それでも**連結**の形で書くのは、(a) 原典の機構そのままであること、(b) すいすい・葉緑素(8192)、
はやあし(6144)、スロースタート(2048)、麻痺(別枠)のように ×2 でない補正を後の段階で足すときに、
逐次適用だと丸め誤差が出てから気づくことになるため。

### 3. トリックルームは実数値を変えず、比較の向きだけ反転する

- `attackerSpeed` / `defenderSpeed`(レスポンス)は**トリックルームの影響を受けない**。
  ゲームの仕様どおり、トリックルームは「行動順が遅い方から」になるだけで素早さの値自体は変わらない。
  画面には「何対何で」を出したいので(ADR-0701 §1)、実数値をいじって表示を嘘にしない。
- `outspeeds` の**意味は「自分が先に動くか」のまま**保つ。JD1 はトリックルームを扱えなかったので
  この意味が単純な `>` と一致していただけで、JD2 で向きを反転させることは再定義ではなく完成である。

  ```
  outspeeds = trickRoom ? attackerSpeed < defenderSpeed : attackerSpeed > defenderSpeed
  ```

- `speedTie` は `attackerSpeed == defenderSpeed` のままで、**トリックルームでは反転しない**。
  同速はゲームでもトリックルームの有無に関わらず行動順がランダムで、「どちらが先か」を断定できない点が
  変わらないため。`outspeeds` と `speedTie` が同時に true にならない不変条件も保たれる。

### 4. コアの形(`internal/judge` は純粋なまま)

- **追い風は `Individual` の欄にする**(`Tailwind bool`)。追い風は対象側の素早さそのものを変えるので、
  既存の `Scarf bool` と同じ「その個体の素早さに乗る補正」であり、`Speed(Individual) (int, error)` の
  引数を増やさずに済む(JD1 の `TestSpeed` の表は 1 行も変わらない)。
- **トリックルームは `SpeedField{TrickRoom bool}`** にし、`CompareSpeed(attacker, defender Individual,
  field SpeedField) (SpeedComparison, error)` の第 3 引数で渡す。トリックルームは片側の値ではなく
  **両者の比較**に効くので、`Individual` に置くと「attacker と defender で食い違う」不正な状態を表せてしまう。
- 契約の `SpeedField`(3 欄)とコアの `SpeedField`(1 欄)で中身が違うのは意図どおり。契約は
  呼び出し側から見た「場の効果」をひとまとめにし、コアは効き方(側に乗るか・比較に効くか)で分ける。
  変換は `internal/httpapi` が行う(`speedField.attackerTailwind → attacker.Tailwind`)。
- `CompareSpeed` は**引数を足す形で 1 本のまま**にし、場の効果なし版を別関数にしない。2 本あると
  呼び出し側が古い方を呼んでトリックルームを黙って落とす事故が起きる。既存のテストは呼び出しに
  `SpeedField{}` を足すだけで、**期待値は 1 つも変えていない**(CLAUDE.md 絶対ルール 6 に触れない)。
- `internal/judge` は引き続き I/O を持たない。上流の呼び出しは 1 本も増えない(JD2 で新しい上流依存は無い)。

## 受け入れ条件(JD2)

1. `services/judge/api/openapi.yaml` の `OutspeedAndKoRequest` に省略可の `speedField` があり、
   `SpeedField`(`trickRoom` / `attackerTailwind` / `defenderTailwind`、すべて `boolean` の `default: false`)が
   `FieldState` とは**別のスキーマ**として定義されている。`make judge-gen` の生成物が最新。
2. 追い風は対象側の**実数値を ×2** し、`attackerSpeed` / `defenderSpeed` に反映される。
   `attackerTailwind` は自分だけ、`defenderTailwind` は相手だけに効く。
3. 素早さ補正は 4096 基準で連結してから **1 回だけ**五捨五超入する。とくに実数値 91 にスカーフと追い風が
   両方乗るとき **273**(各補正ごとに丸める実装なら 272)になる。スカーフ単独・追い風単独の値は
   JD1 から変わらない(91 → 136 / 182)。
4. トリックルームは `attackerSpeed` / `defenderSpeed` を**変えない**。`outspeeds` だけが
   `attackerSpeed < defenderSpeed` に反転し、`speedTie` は反転しない。
   `outspeeds` と `speedTie` が同時に true になることは無い。
5. `speedField` を省略した request の応答は JD1 と同一(`speedField: {}` や全欄 false も同じ)。
6. `speedField` は calc-svc に**送らない**。calc-svc へ送る body に `speedField` キーが現れず、
   `field` の転送(ADR-0701 受け入れ条件 6)も変わらない。
7. `speedField` に未知の欄・真偽値でない値がある request は、上流を 1 回も呼ばずに
   400 `invalid_request`(検査順は ADR-0701 §5 のまま)。
8. `make judge-test` / `judge-lint` / `judge-build` が通り、`internal/judge` は I/O を持たないまま。

## テストの期待値

- 上流は JD1 と同じ `httptest.Server` の架空の応答(架空の種族 `9001-000` / `9002-000`、架空の技・性格)。
  実マスタ・実データは使わない。`t.Cleanup` の登録順も JD0/JD1 と同じ。
- 素早さの期待値は §2 の式からの手計算。丸めの向きが分かるケースを必ず置く:
  - 種族値 71・SP 0・無補正 → 実数値 **91**。スカーフのみ → **136**(JD1 と同じ)、追い風のみ → **182**、
    **スカーフ + 追い風 → 273**(各補正ごとに丸めると 272 になる)。
  - 種族値 73 → 実数値 **93**。スカーフのみ → 139、スカーフ + 追い風 → **279**(逐次なら 278)。
- 確定数(`ko`)は JD1 どおり calc-svc の値の転記だけを見る(judge では検証しない)。
- judge は engine の式を複製しないので `make test-golden` の対象は増えない(ADR-0700・ADR-0701 と同じ)。

## 却下した案

- **トリックルーム・追い風を既存の `field` に入れる**: calc-svc へ転送する欄なので、calc-svc が知らない欄が
  混ざる。転送前に judge が抜き取る形にすると、「`field` に入れたのに calc に行く欄と行かない欄がある」という
  説明しにくい契約になる。§1 のとおり欄を分ける。
- **トリックルームで `attackerSpeed` / `defenderSpeed` を反転・加工して返す**(例: 10000 から引く):
  ゲームの仕様に無い値を画面に出すことになり、ADR-0701 §1 が `attackerSpeed` を返す理由(なぜその判定なのかを
  画面が出せる)を壊す。
- **トリックルーム中は `outspeeds` の意味を「素早さが大きいか」に固定し、反転は呼び出し側に任せる**:
  同じ真偽値を画面ごとに別の意味で読むことになり、Web と iOS で解釈が割れる。judge は「先に動くか」という
  1 つの問いに答える(judge-design.md §1)。
- **トリックルーム中は `speedTie` も反転する / 同速時に `outspeeds` を true にする**: 同速はどちらが先か
  決まらない。ADR-0700 §6-1・ADR-0602 の「同速を真偽値 1 つに丸めない」と矛盾する。
- **各補正を順に掛けてそのつど五捨五超入する**: §2 の表のとおり実数値 91 で 272 と 273 に割れる。
  @smogon/calc 0.12.0 の `getFinalSpeed` は連結してから 1 回だけ丸める。
- **`chainMods` の上下限(410 / 131172)と速度の上限(10000)も移植する**: §2 のとおり現状の補正では
  到達せず、テストで踏めない分岐が残る。補正が増えて到達しうるようになった段階で足す。
- **`CompareSpeed` を 2 引数のまま残し `CompareSpeedInField` を別に足す**: §4 のとおり、古い方を呼んで
  トリックルームを黙って落とす事故が起きる。引数を足す変更は既存テストの期待値を変えない。
- **追い風を `SpeedField` に入れてコアでも 3 欄にする**: `Speed(Individual)` にどちら側かを渡す引数が要り、
  `Individual.Scarf` と同じ性質の値が 2 か所に分かれる。
- **麻痺(素早さ ×1/2)も一緒に入れる**: ADR-0701 §2 が `status` を JD2 に送ったのは事実だが、麻痺は
  素早さ補正の連結の**後**に別枠で掛かる(`getFinalSpeed` の `hasStatus('par')` は `pokeRound` の後)。
  さらに `status` はダメージ側(やけどの攻撃半減)にも効くので calc-svc への転送も要り、JD2 の
  「場の効果」とは別の決定になる。judge-design.md §3 の JD2 の定義(トリックルーム・追い風)に揃え、
  `status` は JD3 以降で `field` 転送とあわせて決める。
