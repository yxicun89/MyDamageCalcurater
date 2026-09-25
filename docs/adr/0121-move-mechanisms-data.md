# ADR-0121: 技の機構(多段・固定ダメージ・威力変動 等)をマスタのデータとして持つ(issue #271-a)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #271(#233 を統合。#289 の「フィールド固有の技の処理」も引き継ぐ)、ADR-0005(M1 の対象外)、
  ADR-0107(技の追加効果。同じ「行が無い = なし」の形)、ADR-0115(黙って落とさない)、ADR-0116(接地判定。
  フィールド固有の技の処理は #271 へ)、ADR-0101 §3(取得元の表現のまま出し、分類は Go 側)

## 背景

engine は技を「威力・分類・タイプ・優先度」だけで計算する。多段技・威力が条件で変わる技・固定ダメージ技・
別の能力値を使う技などは、黙って誤ったダメージ(威力 0 の技はダメージ 0 =「倒せない」)になる
(issue #271: 攻撃技の約 17%)。原因は、それらを区別する情報がマスタに無いこと(#233)。

engine が「未対応」と印を付けるにも、計算できるものを正しく計算するにも、まず技ごとの機構がデータとして要る。
本 ADR はデータ側(#271-a)だけを決める。engine での扱い(未対応の印・一部の機構の実装)は次の作業(D16)で決める。

## 調査(取得元に何があるか)

- @smogon/calc 0.12.0 は技の特殊な処理を**技名の分岐**でコードに持ち、データには出ない。
- Showdown(ピン留めした commit・champions mod)の技データには、機構を機械的に読み取れる材料がある:
  - プロパティ: `multihit`(回数か [最小, 最大])・`damage`(数値か "level")・`ohko`・`willCrit`・
    `overrideOffensiveStat`・`overrideOffensivePokemon`・`overrideDefensiveStat`・`ignoreDefensive`
  - 関数(ハンドラ)の名前: `basePowerCallback`・`damageCallback`・`onBasePower`・`onModifyType`・`onEffectiveness` 等
  - 天候・フィールドの状態のハンドラが技 ID を文字列で名指ししている箇所(フィールドで特定の技の威力が変わる処理)。
    技のデータには現れないので、状態のハンドラのソースの文字列リテラルを技 ID と突き合わせて拾う。
- 技名のハードコードはしない(CLAUDE.md)。分類の規則は Showdown のプロパティ名・イベント名だけで書く。

## 決定

### 1. 表の形: `move_mechanisms(move_id, mechanism)`(migration 000008)

1つの技が複数の機構を持つ(例: 多段かつ威力が変わる)ので、単一の enum 列ではなく learnsets と同じ
「(技, 機構) の組を1行」にする。主キー `(move_id, mechanism)`、`moves` への外部キー ON DELETE CASCADE、
`mechanism` は CHECK で値の一覧を固定する。**行が無い = 通常の技**(威力・分類・タイプから通常の式で計算できる)。
変化技は機構を持たない(他表の列を見る CHECK は書けないので importer と `services/internal/master` が守る)。
down は DROP TABLE だけ(行が入った状態でも通る)。

値の一覧の正は `services/internal/master` の `AllMoveMechanisms`。migration の CHECK と一致することを
`services/pokedex/db` の layout テストで確かめる。

### 2. 機構の一覧と判定の規則(攻撃技だけ)

| 機構 | 意味 | 判定材料(Showdown) |
|---|---|---|
| `multi_hit` | 複数回当たる | `multihit` |
| `fixed_damage` | ダメージを計算式でなく技の処理で決める | `damage`・`damageCallback` |
| `ohko` | 一撃必殺 | `ohko` |
| `variable_power` | 条件で威力が変わる | `basePowerCallback`・`onBasePower`。**威力 0 で登録された攻撃技**(固定ダメージ・一撃必殺でないもの)も |
| `alt_offense_stat` | 攻撃側の参照する能力値・ポケモンが違う | `overrideOffensiveStat`・`overrideOffensivePokemon` |
| `alt_defense_stat` | 防御側の参照する能力値が違う | `overrideDefensiveStat` |
| `always_crit` | 必ず急所 | `willCrit` |
| `ignore_defense_ranks` | 防御側のランク変化を無視 | `ignoreDefensive` |
| `type_change` | 条件でタイプが変わる | `onModifyType` |
| `effectiveness_change` | 相性の求め方が違う | `onEffectiveness` |
| `priority_change` | 条件で優先度が変わる | `onModifyPriority` |
| `field_specific` | 天候・フィールドがこの技を名指しして処理を変える | 状態のハンドラ(`onBasePower`・`onModifyDamage`・`onWeatherModifyDamage` 等)が技 ID を名指し |
| `move_specific` | 技固有の処理があり、ダメージに効くかを名前から決められない | `onModifyMove`・`onTryHit`・`onPrepareHit` |

ダメージの量に効かないと確かめたハンドラ(使えるか・失敗するか・当たった後・溜め 等: `onTry`・`onTryMove`・
`onHit`・`onAfterHit`・`onAfterSubDamage`・`onAfterMove`・`onAfterMoveSecondarySelf`・`onMoveFail`・
`onDisableMove`・`onModifyTarget`・`onTryImmunity`・`beforeTurnCallback`・`beforeMoveCallback`・
`priorityChargeCallback`)は機構にしない。状態のハンドラでは `onTryAddVolatile`(名指しが技でなく同名の状態)を除く。

**分類表に無いハンドラ名**(取得元の更新で増えたもの)は、黙って通常の技にせず安全側
(技のハンドラは `move_specific`、状態のハンドラは `field_specific`)に分類し、警告
`move-mechanism-unknown-hook` を出す(ADR-0115)。取り込みは止めない。分類表(`convert_move_mechanisms.go`)に足して消す。

取得元の形が想定外(`multihit` が1回・範囲が逆、`damage` が 0 や未知の文字列 等)なら `ErrInvalidData` で止める。

### 3. 取得と投入の流れ

- `tools/importer/fetch-showdown.mjs` が技ごとに `mechanism`(判定材料を取得元の表現のまま)を出す。
- `services/pokedex/importer` が `mechanism` を**必須**としてデコードする。無い(古い取得物)と全技が黙って
  通常の技になるので、デコードで拒否する(`tools/importer` で取り直す)。
- 分類(`convert_move_mechanisms.go`)→ `Output.MoveMechanisms` → `Apply` が投入(`DeleteMoveMechanisms` /
  `InsertMoveMechanism`)。`ListMoveMechanisms` は sqlc に用意する。
- 照合の要約(`latest-summary.txt`・dry-run の出力)に `moveMechanisms: attack=<攻撃技数> withMechanism=<機構を持つ技数>`
  と機構ごとの技数を出す(技の ID は出さない)。

### 4. 出口(この ADR の範囲)

- `services/internal/master` の `MoveRow.Mechanisms` を `master.Move` で検証する(未知の値・重複・変化技の機構は拒否)。
  `engine.Move` にはまだ載せない(D16)。
- balance/speed の read model(`moves.json`)には足さない(ADR-0107 決定7 と同じ理由: balance の読み込みは未知のキーを拒否する)。
- 内部 API `/internal/pokedex/master` の `MasterMove` への追加は `api/openapi.yaml` の変更で、API レーンの持ち物
  (COORDINATION.md)。API レーンに `MasterMove.mechanisms: string[]`(昇順・通常の技は空配列)の追加を依頼する。
  それまで calc-svc には届かない。

### 5. 機構にしないもの(意図して)

- **サイコフィールドの先制技の無効**: 技固有ではなく「優先度 > 0 の技 × 接地した防御側」の一般規則。優先度は既に
  `moves.priority` にあるので engine 側(D16)で扱える。条件で優先度が変わる技だけ `priority_change` を付ける。
- **使えるか・失敗するか**(条件を満たさないと失敗する技): 成功した場合のダメージは通常の式で正しい。
- **溜めターンの能力上昇**(溜めの間に自分の能力が上がる技): 上がった後のランクを利用者が入れれば通常の式で正しい。
- **`ignoreImmunity`**: ピン留めした取得元では、遅れて当たる技の開始処理のためだけに使われ、ダメージの相性には効かない。

## 検証(実データの dry-run。技の ID は書かない)

- 攻撃技 335(calc の Champions 世代の 334 + Showdown だけの 1)。機構を持つ技 93。
  機構ごと: variable_power 41・move_specific 20・multi_hit 14・fixed_damage 9・ohko 4・type_change 4・always_crit 3・
  alt_offense_stat 2・effectiveness_change 2・field_specific 2・ignore_defense_ranks 2・alt_defense_stat 1・priority_change 1。
  既存の件数・警告・verdict は変わらない。未知のハンドラの警告は 0。
- oracle(@smogon/calc 0.12.0 Champions 世代)との突き合わせ: 各攻撃技を、同じ威力・タイプ・分類の通常の技と
  ランダムな条件(種族・SP・性格・ランク・天候・フィールド・壁・急所・状態・HP・相手の持ち物)で比べ、結果が違う技 82 件
  (条件なしで常に違う 42・条件次第 40)を得た(issue の 56 件より条件の幅が広い)。
  - 64 件は機構を持つ。
  - 残り 18 件はすべて §5 の「機構にしないもの」: 優先度 > 0 でサイコフィールドに無効化される 15、溜めターンの能力上昇 2、
    条件を満たさないと失敗する 1。
  - 機構を持つが oracle では違いが出ない 29 件: 多くは oracle が状態(受けたダメージ・行動順・倒れた味方の数・
    重力 等)を持たないため比べられないもの(固定ダメージ・一撃必殺・行動順で威力が変わる技 等)。
    安全側の過検出(`move_specific` だが実際はダメージに効かない: 命中だけ変える技・味方への処理)は 5 件。
    engine でどう見せるか(「未対応」か「注意」か)は D16 で決める。

## 結果

- 良い点: 誤ったダメージになりうる技をデータで区別できる。技名を持たず、取得元の更新で増えた処理も警告付きで安全側に倒る。
- 注意: 既存の取得物(`data/generated/showdown/<commit>/snapshot.json`)には `mechanism` が無いので、このマージ後は
  `tools/importer` で Showdown の取得をやり直す(同じ commit。キャッシュ済みのソースを使うのでネットワーク不要。他の値は変わらない)。
  取り直した取得物は、この変更を含まない古いブランチの importer では未知のフィールドとして拒否される。
- `move_specific` の過検出 5 件は、ハンドラのソースを読まないと区別できない。必要になったら ADR を追記して規則を細かくする。

**実装時の追記(2026-09-25。API レーン)**: §4 の依頼どおり `api/openapi.yaml` の `MasterMove` に
`mechanisms: string[]`(必須・昇順・通常の技は空配列)を追加した。`services/pokedex/internal/httpapi/master.go`
が `ListMoveMechanisms` を move_id ごとにまとめ、SQL の `ORDER BY` に頼らずここで明示的にソートしてから返す
(契約の「昇順」保証をこの層で持つ)。`services/calc/internal/master/export.go` の `buildMoves` はこれを
`sharedmaster.MoveRow.Mechanisms` にそのまま渡すだけで、検証・`engine.Move.Mechanisms` への変換は既存の
`services/internal/master.MoveMechanismsOf`(データレーンが本 ADR で実装済み)がそのまま行う。
Web(`web/src/master/exportSnapshot.ts` の `CalcSnapshotMove`)の例データにも `mechanisms: []` を追加した
(例データは機構を持つ技を含まない)。iOS 側の生成物(swift-openapi-generator)は iOS レーンでの
`make ios-gen` 相当の再生成が必要(このタスクでは未実施。DECISIONS.md 参照)。
