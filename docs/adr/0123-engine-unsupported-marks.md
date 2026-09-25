# ADR-0123: engine が正しく計算できない技・持ち物・特性に「未対応」の印を付け、サイコフィールドの先制技を無効にする(issue #271-b / #270 案 B)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #271(技。#233 を統合)、issue #270(持ち物・特性の既定案 B)、ADR-0121(技の機構のデータ)、
  ADR-0120(効果スキーマで表せない持ち物・特性の一覧)、ADR-0116(接地判定)、ADR-0106(特性による無効)、
  ADR-0011(WASM 境界)、ADR-0118(effects.json 2つの一致)

## 背景

engine は技を「威力・分類・タイプ・優先度」と持ち物・特性の効果定義から通常の式で計算する。
多段・威力変動・固定ダメージの技(ADR-0121 で機構としてデータに入った)や、効果スキーマで表せない
持ち物・特性(ADR-0120 の未対応一覧)は、黙って通常の技・補正なしとして数値を返していた。威力 0 の攻撃技は
ダメージ 0・確定数 0(倒せない)という、正しい結果に見える応答になっていた。

issue #271 は「未対応と分かる印」か「400 で拒否」を、#270 は「補正未対応の印」か「選択肢で区別」を ADR で決めるよう求めている。

## 決定

### 1. 拒否ではなく印を付ける(両 issue の既定案)

計算は従来どおり通常の式で行い、結果に印を付ける。400 で拒否すると画面で技・持ち物を選べなくなり、
「大体の目安」も得られない。数値そのものは印で変えない(既存の正常な入力のゴールデンは全件そのまま一致)。

### 2. 印の形(engine)

`DamageResult.Unsupported []UnsupportedMark`(印なしは nil)。`UnsupportedMark{Target, Reason, ID}`:

| Target | Reason | ID |
|---|---|---|
| `move` | 機構の値(ADR-0121 の 13 種)または `zero_power` | 技 ID |
| `attacker_item` / `attacker_ability` / `defender_item` / `defender_ability` | `unsupported_effect` | 持ち物・特性 ID |

並びは 技(理由の昇順、`zero_power` は最後)→ 攻撃側の持ち物 → 攻撃側の特性 → 防御側の持ち物 → 防御側の特性。
一括計算は各行の `Result`、逆算は各候補の `Unsupported`(SP によらない)に同じ印が付く(どちらも CalcDamage の合成)。
変化技は印を付けない(ダメージを持たないので 0 が正しい。issue #271 の異常系)。

### 3. 技の印の条件(過検出を減らす)

`engine.Move.Mechanisms` を持たせ(マスタ → `services/internal/master.Move`、WASM の `move.mechanisms` の両経路)、
機構ごとに次の条件で印を付ける。どれも「この入力なら通常の式と同じ結果になる」ことが式から言える場合だけ印を外す。

| 機構 | 印を付ける条件 |
|---|---|
| `always_crit` | 入力が急所なし(急所ありなら同じ) |
| `ignore_defense_ranks` | 防御側の、技の分類で使う側(物理は防御・特殊は特防)のランクが 0 でない |
| `priority_change` | フィールドがサイコ(engine が優先度を使うのは §5 の判定だけ)。防御側が浮いていて判定に関係しないときも付ける(安全側の過検出) |
| `field_specific` | 天候かフィールドがある(engine が持つ場の状態はこの2つだけで、無ければ名指しの処理は起きない) |
| それ以外の 9 種 | 常に |
| 未知の値(engine に届いた場合) | 常に(安全側。WASM の入力は `invalid_enum` で拒否する) |

威力 0 の攻撃技は機構に関係なく `zero_power` を付ける(issue #271 の境界値)。

値の一覧の正を `services/internal/master` から engine(`engine.AllMoveMechanisms`)へ移した。master は
`MoveMechanism = engine.MoveMechanism` の別名で、migration の CHECK との一致テスト(ADR-0121)はそのまま効く。

**正しく計算する(任意)の範囲**: この ADR では印だけにする。`always_crit` を強制急所にする等は数値が変わる判断で、
@smogon/calc の技ごとの処理と照合するベクタが要る(生成器の代表技は機構なしの技だけ)。よく使う機構から
別 issue で1つずつ実装し、実装したら印の条件から外す。

### 4. 持ち物・特性の印は効果定義のデータに持つ(#270 案 B)

効果定義の JSON に印の項目 `UnsupportedAttacker` / `UnsupportedDefender`(true だけ)を足す
(`engine.ItemEffect` / `engine.AbilityEffect` の同名フィールド。WASM では `unsupportedAttacker` / `unsupportedDefender`)。
持ち物・特性の一覧をコードに持たない(CLAUDE.md)。effects.json は本番(`data/importer/effects.json`)と
ゴールデン(`testdata/golden/effects.json`)の両方に同じ内容で入れる(ADR-0118)。

- **側を分ける**: 防御側が持っても効かない攻撃用の特性(威力を上げる特性 等)に印を付けると、相手の通常の特性を
  選ぶだけで印が出てしまう。そこで「その側で持つとダメージが変わる」側だけに印を付ける。
- **側は oracle で決めて照合する**: `tools/golden/generate.mjs` の網羅調査(ADR-0120 §3)を側ごとに分け、
  印の集合が `unsupported-effects.json` と一致すること、印の側が oracle でダメージが変わった側と一致することを確かめる
  (崩れると生成器が止まる)。印だけの定義はベクタで使わない。持ち物 5(攻撃側 2・防御側 4)、特性 51
  (攻撃側 35・防御側 19)。
- **調査に使う技を広げた**(独立レビューの指摘): ADR-0120 の調査は先制度 0・単発・反動なしの代表技だけで、
  技の性質が条件の効果(先制技を防ぐ特性・反動技を強める特性 等)を取りこぼし、印が付かなかった。
  調査にだけ、先制技・反動技・連続技・威力 60 以下・非接触の物理技・接触・パンチ・かみつき・音・波動・弾・切る技・
  追加効果のある技の代表 23 技を足し(oracle の champions.js が条件に使う `move.flags` / `priority` / `recoil` /
  `hits` / `bp` / `secondaries` を網羅。どれかが欠けると生成器が止まる)、条件にサイコフィールド + 雨を足した。
  新たに見つかったのは持ち物 1・特性 6(先制技を防ぐ特性 2、反動技の威力を上げる特性 1、受けると場やランクが
  変わり連続技の2発目以降に効く特性 3、サイコフィールドで効く持ち物 1)。`unsupported-effects.json` に理由付きで足した。
  ベクタには使わないので既存の期待値は変わらない。先制技にする特性(Gale Wings)は oracle がサイコフィールドの判定の
  後で優先度を変えるため、oracle でもダメージが変わらず、印は付けない。
- **本番のデータでは印と補正を混ぜない**(生成器が確かめる)。engine と `services/internal/master` は形として混在を
  受け付ける(補正は掛けたうえで印も付ける)。
- **importer の網羅性の指標は変えない**: 印だけの定義は補正を計算していないので、`effect-missing` / `effect-no-hook`
  では「定義なし」と数える(ADR-0103 §6)。実データの dry-run の出力は変更前と同一。
- 効果定義の JSON は内部 API(`MasterEffect`)では不透明なオブジェクトなので、calc-svc の engine にも契約の変更なしで届く。

### 5. サイコフィールドの先制技(ADR-0121 §5 の持ち越し)

優先度 > 0 の攻撃技 × サイコフィールド × 防御側が接地(ADR-0116 の `isGrounded`)なら、ダメージ 0・
`Nullified = psychic_terrain`(新しい値)。@smogon/calc 0.12.0 champions.js と同じ規則・同じ順(タイプ由来の無効 →
特性による無効・吸収 → サイコフィールド)。ゴールデンに `psychic-priority/*` 6 件(接地した防御側 3 件は 0、
ひこうタイプ・浮く特性の防御側・別のフィールドの 3 件は通常)を足し、生成器で当たる/当たらないを oracle でも確かめる。
`NullifyKind` の意味を「特性による」から「タイプ相性以外の理由」に広げた(WASM・HTTP の応答には `Nullified` を出していない)。

### 6. 応答への出し方

- **WASM**(`engine/wasmapi`): calc の結果・bulk の各行の結果・reverse の各候補に `unsupported`(常に配列。印なしは `[]`)。
  入力は `move.mechanisms`(未知の値は `invalid_enum`、重複は `invalid_input`)と効果の `unsupportedAttacker` /
  `unsupportedDefender`。Go/WASM 一致のベクタに2件足した。Web はまだ表示しない。
- **calc-svc**: `api/openapi.yaml` は API レーンの持ち物なので契約は変えない。印は engine の結果にあるが応答には出ない。
  HTTP と WASM のパリティテストは、HTTP に印の項目が入るまで WASM の空の `unsupported` だけを外して比べる
  (空でない印が出たら失敗させる)。API レーンへの依頼は §7。

### 7. API レーンへの依頼(既定案)

1. `MasterMove.mechanisms: string[]`(ADR-0121 §4 の依頼の再掲。昇順・通常の技は空配列・値は下の 13 種)。
   calc-svc は `services/internal/master.Move` に `MoveRow.Mechanisms` を渡すだけで engine に届く。
2. 応答に印: `CalcResponse`(と `BulkRow.result`・`ReverseCandidate`)に `unsupported: UnsupportedMark[]`(必須・印なしは `[]`)。
   ```yaml
   UnsupportedMark:
     type: object
     required: [target, reason, id]
     properties:
       target: { type: string, enum: [move, attacker_item, attacker_ability, defender_item, defender_ability] }
       reason:
         type: string
         enum: [alt_defense_stat, alt_offense_stat, always_crit, effectiveness_change, field_specific, fixed_damage,
                ignore_defense_ranks, move_specific, multi_hit, ohko, priority_change, type_change, variable_power,
                zero_power, unsupported_effect]
       id: { type: string }
   ```
   写しは WASM の `unsupportedFrom`(`engine/wasmapi/dto.go`)と同じ。入ったらパリティテストの `dropEmptyUnsupported` を消す。

## 結果

- 良い点: 多段・威力変動・固定ダメージの技と、表せない持ち物・特性を選んだ結果が、黙って正しい結果のように見えなくなる
  (WASM は今すぐ、calc-svc は契約の追加後)。サイコフィールドの先制技は oracle と一致する。
- 既存の正常な入力の数値は変わらない(ゴールデン全件一致。追加は `psychic-priority/*` の 6 件だけ)。
- 注意: `move_specific` の過検出(ADR-0121 の 5 件)にも印が付く。印の条件を細かくするときは本 ADR に追記する。
- 印を見せる画面(Web・iOS)は各レーンの作業。
