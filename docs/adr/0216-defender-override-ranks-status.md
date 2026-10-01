# ADR-0216: 一括計算の防御側にランク・状態異常を一律で上書きする(issue #274/#272 の API レーン担当分の残り)

- 状態: 採用(critic PASS 2026-10-02)
- 日付: 2026-10-02
- 関連: ADR-0009(一括計算のプリセット。防御側は Ranks=0・Status=none で組み立てる)、ADR-0126(engine の特性候補)、
  ADR-0214(`defenderOverride.abilityId`。この ADR は同じオブジェクトに残りの2項目を足す)、ADR-0208・ADR-0108
  (件数の上限と HTTP/WASM の検査順)、ADR-0011(WASM 境界)、ADR-0123(未対応の印。防御ランク無視の機構)、
  ADR-0003(engine の test-first と独立 critic)、DECISIONS.md 2026-09-25「issue #274/#272 の防御側の詳細」
  (`defenderOverride: { abilityId?, ranks?: RankBlock, status?: StatusCondition }` を採用済み)

## 背景

DECISIONS.md 2026-09-25 で、iOS レーンの提案どおり `BulkCalcRequest.defenderOverride` に `abilityId`・`ranks`・
`status` を足すことを採用した。`abilityId` は ADR-0214 で実装済み。残りの `ranks`・`status` は engine 側の
変更(`BulkInput` へのオーバーライド)を伴うため別タスクとして残っていた(docs/plan.md「issue #274/#272 の
API レーン担当分の残り」)。iOS・Web の「詳細」で「防御側のランク(B/D)」「防御側の状態異常」を選べるようにするための契約。

## 決定

### 1. engine: `BulkInput.DefenderOverride`(ゼロ値 = 上書きしない)

```go
// engine/bulk.go
var ErrInvalidDefenderOverride = errors.New("防御側の上書きが不正")

type DefenderOverride struct {
    Ranks  Ranks  // 各 -6..+6。ゼロ値はランク 0(プリセットと同じ)
    Status Status // "" と StatusNone は「状態異常なし」。それ以外は既知の Status だけ
}

type BulkInput struct {
    ...
    DefenderOverride DefenderOverride
}
```

- **適用位置**: `DefenderPreset.Defender(species, item)` で防御側を組み立て、特性(ADR-0126)を載せた後、
  `CalcDamage` を呼ぶ前に `Ranks` と `Status` を上書きする。全プリセット × 全持ち物 × 全特性の行に同じ値を当てる。
  特性のまとめ(結果の一致判定)も上書き後の結果で行う。`BulkRow.Defender` は上書き後の個体(Ranks・Status を含む)。
- **合成**: プリセット(既定カタログ・呼び出し側のカスタムとも)は SP・性格だけを決め、Ranks=0・Status=none で
  組み立てる(ADR-0009)。上書きはそれを置き換える(上書きが勝つ)。SP・性格・持ち物・特性・種族は変えない。
  ランクはブロック全体を置き換える(プリセット側が常に 0 なので項目ごとの合成と結果は同じ)。
- **ゼロ値は従来とバイト同一**: `DefenderOverride{}`・`{Status: StatusNone}` は今までの `CalcBulk` と
  `reflect.DeepEqual` で同じ結果。`Status: ""` は `StatusNone` に正規化して `BulkRow.Defender` に載せる。
- **検証**: ランクのどれか(使わない側の能力を含む)が -6..+6 の外、または `Status` が既知の7値と "" 以外なら
  `ErrInvalidDefenderOverride` を返し、部分的な行を返さない。件数上限(ADR-0108)の検査より後、計算より前。
  `CalcDamage` の `Individual.Validate` に任せない理由: そのエラーは sentinel ではなく、境界で `internal` に
  なってしまうため。
- `DefenderPreset.Defender` 自体は変えない(逆算の探索空間・プリセットのテストに影響させない)。

### 2. HTTP 契約(`api/openapi.yaml`)

`DefenderOverride` に `ranks: RankBlock` と `status: StatusCondition` を足す(`Individual` の同名フィールドと同じ
`$ref`。生成される Go・TS・Swift の型も `Individual` と同じ形になる)。応答(`BulkCalcRow`・`BulkDefender`)は
変えない: 上書きは要求の全行に一律なので、クライアントは自分が送った値を知っている。`BulkDefender.stats` は
従来どおりランク補正前の実数値。

| 入力 | HTTP | WASM |
|---|---|---|
| ランクが -6..+6 の外(どの能力でも) | 400 `invalid_input` | `invalid_input` |
| 未知の `status` | 400 `invalid_enum`(`Individual.status` と同じ) | `invalid_enum` |
| `ranks.hp` など契約に無いキー | 400 `unknown_field` | `unknown_field` |
| ランクが整数でない・文字列 | 400 `invalid_json` | `invalid_json` |
| 件数上限違反と上書きの不正が重なる | 件数上限の `invalid_input` が先 | 同じ |

calc-svc は `parseStatusCondition` と同じ検証で `status` を、`ranks` は engine の
`ErrInvalidDefenderOverride` を `invalid_input` に写す(`engineSentinels` に1行)か、engine に渡す前に同じ検査を
する(どちらでもよいが、code と検査順は上の表に合わせる)。新しいエラーコードは足さない(ADR-0208 §2 と同じ方針)。

### 3. 防御側の状態異常は、今のダメージ式に効かない(事実の記録)

engine のダメージ式が状態異常を見るのは `burnModifier`(**攻撃側**がやけど・物理技・こんじょう等でない →
攻撃 0.5 倍)だけで、防御側の `Status` を参照する箇所は無い(`engine/damage.go`)。防御側の状態で変わるもの
(ふしぎなうろこの防御 1.5 倍、たたりめ等の威力2倍)は効果スキーマにも技の機構にも無い。ゴールデン
(`testdata/golden/` の fixed・random・attack-species・defense-species・legacy-effects の全件。2026-10-02 確認)にも
防御側の状態異常を持つケースは無い。

したがって `defenderOverride.status` は**現時点では結果を1バイトも変えない**(テストで固定する)。それでも契約に
入れる理由:

- DECISIONS.md 2026-09-25 で採用済みの形で、iOS・Web の「詳細」が状態異常の選択を持つ前提になっている。
- 後で防御側の状態を見る技・特性に対応したとき(別 ADR)、契約を変えずに効くようになる。そのときは
  `TestCalcBulkDefenderStatusDoesNotChangeDamage` 等を意図して更新し、その ADR に理由を書く。

クライアントが「防御側をやけどにしたのにダメージが変わらない」と誤解しないよう、画面で状態異常を出すかどうかは
各レーンが判断する(この ADR は UI を決めない)。

### 4. 逆算(`ReverseRequest`・`ReverseInput`)には足さない

- `side=attacker`(被ダメ観測)では防御側は既知の自分で、`known.ranks`・`known.status`(`Individual`)で既に渡せる。
- `side=defender`(与ダメ観測)では防御側は探索対象の相手で、engine は Ranks=0・Status=none で組み立てる。
  相手のランクを固定したい用途(「てっぺきの後に殴った観測」)はありうるが、どのクライアントからも依頼が無く、
  `defenderOverride` という名前は `side` によって既知/未知が入れ替わる逆算では意味が曖昧になる。足すなら
  `unknownAbilityId` と同じく「相手側」の名前(例 `unknownRanks`/`unknownStatus`。`side=attacker` では相手の
  やけど・攻撃ランクとして効く)で、別 ADR で決める。
- 今回は `ReverseRequest` に `defenderOverride` を送ると従来どおり 400 `unknown_field` になることをテストで固定する。

### 5. WASM 境界(`engine/wasmapi`)

`calcBulk` のリクエストに任意の `defenderOverride: { ranks?: {atk,def,spa,spd,spe}, status?: string }` を足す。
特性は従来どおり `defenderAbilities`(解決済みの実体)で渡すので、WASM の `defenderOverride` は `abilityId` を
持たない(送れば `unknown_field`)。応答の形は変えない。検査順は ADR-0108 決定3 と同じく、件数上限 → DTO 変換
(`status` の enum)→ ランク範囲 → engine。

## 検証(受け入れ条件)

1. engine: ゼロ値の上書きは従来の `CalcBulk` と `reflect.DeepEqual` で同じ(`engine/bulk_defender_override_test.go`)。
2. engine: 上書きは全行に当たり、各行は上書き後の防御側の `CalcDamage` と完全一致。SP・性格・持ち物・特性・行の順序と数は不変。
3. engine: 物理は防御・特殊は特防のランクだけが効き、±6 の境界まで単調。急所は正の防御ランクを無視。
4. engine: 防御側の状態異常(6種)は結果を変えないが `BulkRow.Defender.Status` に載る。
5. engine: ランク範囲外・未知の状態異常は `ErrInvalidDefenderOverride`(部分的な行なし)。
6. ゴールデン: `fixed.json`・`random.jsonl.gz` の「防御側にランクがあるケース」を一括計算(1プリセット+上書き)で
   oracle と全件一致(`engine/bulk_override_golden_test.go`、`make test-golden`)。ゴールデンのファイルは変えない。
7. HTTP: 省略・空・ゼロ値は応答がバイト同一。指定時は `engine.CalcBulk`(上書き付き)と一致し、`abilityId` と併用できる。
   エラーは上の表どおり。逆算は `defenderOverride` を `unknown_field` で拒否。
8. パリティ: 上の表の失敗は HTTP と WASM で同じ code、同じ上書きなら行が一致(`parity_test.go`)。

## 影響

- engine・wasmapi・calc-svc・`api/openapi.yaml`・生成物(Go・TS・Swift)。生成物はいずれも任意フィールドの追加で、
  既存の呼び出しはそのままコンパイルできる(Swift の `DefenderOverride.init` は既定値 nil の引数が増えるだけ)。
- ゴールデンのベクタ・known_diffs は変えない。
- Web・iOS の画面(「詳細」に防御側のランク B/D・状態異常を足すか)は各レーンの判断。急ぎではない(iOS レーン明記)。
