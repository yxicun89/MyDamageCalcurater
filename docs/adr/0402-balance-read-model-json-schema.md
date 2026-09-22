# ADR-0402: balance の read model の形を JSON Schema で公開する

- 状態: 採用(2026-09-22。タイプバランスレーンの判断。ユーザーの「残っている作業を進めて」による整備)
- 日付: 2026-09-22
- 関連: ADR-0014 §2・ADR-0401 §5(ポケモン)、ADR-0016 §3(技)、ADR-0017 §2(特性)、ADR-0015(相性表)、ADR-0100 §8(pokedex export)、
  DECISIONS.md 2026-09-22「pokedex export への依頼」

## 背景
balance が読む read model の形は ADR の本文と loader(`internal/master`)にしか書かれておらず、データレーンが `pokedex export` を作るときに
機械的に確かめる手段が無かった。

## 決定
1. 4つの read model の JSON Schema(draft 2020-12)を `services/balance/schema/` に置く:
   `pokemon-types.schema.json`・`moves.schema.json`・`abilities.schema.json`・`type-chart.schema.json`。
2. **形の意味の正は引き続き各 ADR**(ADR-0014/0015/0016/0017/0401)で、schema はそれを機械で確かめられる形にしたもの、loader は実装。
   三者がずれたら ADR に合わせて schema と loader を直す。データレーンの export は、この schema に合うことを確かめて出力する(推奨)。
3. schema で表せない制約(ID の重複禁止、同じ特性で同じ攻撃タイプへの immune / absorb の重複禁止、特性の係数を loader が約分して保持すること、
   整数値の小数 `2.0` を loader が拒否すること、`super_effective_multiplier` の `attackType: null` の扱いなど)は schema の `description` に書く。
4. テスト(`schema/schema_test.go`)で、(a) 各 example と同梱の相性表が schema に合うこと、(b) loader が拒否する代表的な入力を schema も拒否することを確かめる。
   検証ライブラリはテストだけで使う `github.com/santhosh-tekuri/jsonschema/v6`(2026-09-22 時点の最新 v6.0.3、Apache-2.0)。実行時のバイナリには入らない。

## 却下した案
- schema を正にして loader を生成する: Go の JSON Schema からのコード生成は制約の表現力が足りず、既存の loader の検証(重複・約分)を失う。
- 検証を実行時に行う: loader の検証で足りる。依存を実行時に増やさない。
