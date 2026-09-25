# ADR-0126: 一括計算の防御側・逆算の相手側に特性の候補を渡す(issue #272 の engine 側)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #272、ADR-0009(一括計算のプリセット。§1 の「Ability = ゼロ値」を改める)、ADR-0010(逆算。§R1 の「特性は未知側ゼロ値固定」を改める)、ADR-0011(WASM 境界)、ADR-0106(無効・吸収の特性)、ADR-0208(件数の上限)

## 背景

1対1の `CalcDamage` は防御側の特性(無効・吸収・軽減)を正しく計算する(ADR-0106)。一方で一括計算は
`DefenderPreset.Defender` が特性をゼロ値に固定し、逆算は相手の個体をゼロ値の特性で組み立てていた。
そのため、たとえば地面無効の特性しか持たない種族に地面技を一括計算すると、全行にダメージと確定数が出て、
同じ防御側を1対1で計算した結果(0・immune)と食い違っていた。計算画面は一括計算だけを呼ぶので、
画面では防御側の特性が一度も効かない。

engine はマスタを持たない(ADR-0005・ADR-0011)ので、「その種族がどの特性を持ち、どれが効くか」は
呼び出し側が解決済みの `Ability` として渡す必要がある。

## 検討した案(既定の選び方)

- (A) 種族の1番目の特性だけで計算する。1番目以外(隠れ特性など)で無効になる種族を黙って誤る。
- (B) 呼び出し側が特性を1つ指定する(issue の既定案)。選ばないかぎり今までと同じで、画面の既定を
  「1番目」にすると (A) と同じ誤りが残る。
- (C) 種族の全特性で行を分ける。多くの種族では特性が結果を変えないので、行が2〜3倍に増えるだけになる。
- (D) 特性の候補(0〜3件)を受け取り、**計算結果が完全に同じになる特性は1行にまとめ、違うときだけ行を分ける**。
  まとめは効果の定義を読んで推測せず、全行の結果の一致(`reflect.DeepEqual`)で決める。**採用**。

(D) なら、呼び出し側が種族の全特性を渡しても、特性が効かない技では行数が今までと同じで、
効く技(地面技に対する地面無効など)のときだけ「どの特性ならどうなるか」が並ぶ。利用者が特性を選び忘れても
無効・軽減の可能性が黙って消えない。1つだけ渡せば (B) と同じ。

## 決定

1. `BulkInput.DefenderAbilities []Ability` と `ReverseInput.UnknownAbilities []Ability` を足す。
   - 0〜`MaxAbilityCandidates`(= 3。通常特性2つ + 隠れ特性1つ)件。空・nil は「特性なし」の1通りで、
     issue #272 以前と同じ結果になる(後方互換)。
   - ID が空・ID の重複・件数超過・種族が持たない特性(`Species.Abilities` が空でないときだけ検査)は
     `ErrInvalidAbilityCandidates`。
2. 候補の各特性で計算し、結果が完全に同じ特性を1つにまとめる。代表は先に渡したもの(渡した順を保つ)。
   - 一括計算: 全プリセット × 全持ち物の結果がすべて同じならまとめる。行の並びは プリセット → 特性 → 持ち物。
   - 逆算: 全性格クラス × 全持ち物 × SP 0..32 の結果がすべて同じならまとめる。定義順は 性格クラス → 特性 → 持ち物
     (ADR-0010 §R4 の全順序の最後の「定義順」がこれになる)。特性は SP のような探索の次元ではなく、渡したものだけを試す。
   - 全候補(全特性)でダメージが 0 のときだけ `ErrMoveDealsNoDamage`(ADR-0117 §3)。
3. 行 `BulkRow` と候補 `ReverseCandidate` に `Ability`(計算に使った代表)と `AbilityIDs`(結果が同じ特性の ID。
   代表が先頭)を足す。特性を渡さなかったときは `Ability` がゼロ値、`AbilityIDs` が nil。
   一括計算の行の `Defender.Ability` も代表の特性になる。
4. 攻撃側の特性は、一括計算の `Attacker` と逆算の `Known` がもともと `Individual.Ability` を持っているので、
   engine の口は既にある(画面が種族の1番目を黙って入れているのは Web・iOS の問題)。受けたダメージの逆算
   (`side=attacker`)では相手 = 攻撃側の特性が `UnknownAbilities` になる。
5. WASM 境界(`engine/wasmapi`): `calcBulk` に `defenderAbilities`、`calcReverse` に `unknownAbilities`
   (いずれも `Ability` の配列。`calc` の `ability` と同じ形)を足す。応答の行・候補に `abilityId` と `abilityIds` を
   足すが、**特性を送らなかったときは出さない**(`omitempty`)。これで Web がまだ送らない間は応答がバイト単位で従来と同じ。
   件数超過(境界で DTO 変換の前に見る)と `ErrInvalidAbilityCandidates` は `invalid_input` に写す(新しいコードは足さない)。

## 他レーンへの依頼(既定案)

- API(`api/openapi.yaml`): 既に採用済みの `BulkCalcRequest.defenderOverride.abilityId`(DECISIONS.md 2026-09-25
  「issue #274/#272 の防御側の詳細」)と整合させる。
  - `defenderOverride.abilityId` があれば `DefenderAbilities` にその1件を渡す(種族の特性に無い ID は 400 `invalid_input`、
    マスタに無い ID は 400 `unknown_ability`)。
  - **無ければ calc-svc が種族の全特性をマスタから解決して `DefenderAbilities` に渡す**(本 ADR の (D) により、
    特性が効かない技では行は増えない。特性が1つしかない種族は必ずその特性が効く = issue の境界値の受け入れ条件)。
  - 逆算は `ReverseRequest.unknownAbilityId`(任意・1つ)を足し、同じ既定(無ければ種族の全特性)にする。
  - 行 `BulkCalcRow` と `ReverseCandidate` に `abilityId`(string)と `abilityIds`(string[])を足す。
    省略時の既定を変えると HTTP の応答の行数が特性の効く技で増えるので、契約の description に明記する。
- Web・iOS: 計算・逆算の画面で攻撃側・防御側(相手側)の特性を選べるようにする。攻撃側の既定は種族の1番目で、画面に表示する。
  防御側・相手側の既定は「指定なし = 種族の全特性」を送り、行・候補ごとに `abilityIds` を表示する。WASM を使う Web は
  種族の特性をマスタから解決して `defenderAbilities` / `unknownAbilities` に渡す。

## 影響

- 計算量: 一括計算・逆算とも最大で特性の数(3)倍の `CalcDamage`。逆算は 2 × 3 × 64 × 33 = 12,672 回が上限。
- ゴールデン: 既存の数値は不変(特性を渡さない経路は従来と同じ)。`engine/bulk_ability_golden_test.go` が
  fixed.json の「防御側に効果つきの特性があるケース」を一括計算で oracle と照合する。
- 残る限界: 逆算の特性は候補として試すだけで、観測から特性そのものを絞る表示(候補の一致度で分かる)は画面側の仕事。
