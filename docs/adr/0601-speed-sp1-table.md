# ADR-0601: 素早さ比較 SP1 素早さの表

- 状態: 採用(2026-09-22。§1 はユーザー決定・回答。§2 以降は素早さレーンの判断)
- 日付: 2026-09-22
- 関連: docs/speed-design.md §5、ADR-0600(計算コア・read model・API の共通事項)

## 決定

### 1. ユーザー決定・回答(2026-09-22)
- 使用可能な各ポケモンについて 6 行(無振り / 準速 / 最速 / 最速スカーフ / 最速+1 / 最速+2)。最初から速い順。道具・ランクで絞り込める。
- 同じ実数値は同速としてまとめて表示。表に載せるのは既定のレギュレーションの使用可能集合(read model の Roster 全体)。

### 2. 行の型(プリセット)
「入力の作り方の型」(ADR-0009 と同じ考え方。マスタではない)として、speed のコアが 1 か所に順序付きで持つ。ID は API の enum と同じ。

| ID | 表示(クライアントの文言資源) | SP | 性格 | ランク | スカーフ |
|---|---|---|---|---|---|
| `uninvested` | 無振り | 0 | neutral | 0 | なし |
| `neutral-max` | 準速 | 32(`engine.MaxSPPerStat`) | neutral | 0 | なし |
| `max` | 最速 | 32 | plus | 0 | なし |
| `max-scarf` | 最速スカーフ | 32 | plus | 0 | あり |
| `max-plus1` | 最速+1 | 32 | plus | +1 | なし |
| `max-plus2` | 最速+2 | 32 | plus | +2 | なし |

表示名は API に含めない(文言はクライアントの文言資源。coding-rules §2)。値は ADR-0600 §3 の `speed.Speed` で計算する。

### 3. 並びと同速
- 値ごとに 1 つの「段」(tier)にまとめ、段は素早さの**降順**。
- 段の中は pokemonId の昇順(図鑑番号順)、同じポケモンなら §2 の表の順。
- 同速の段は `entries` が 2 件以上の段として表す(クライアントはこれで「同速」と表示できる)。

### 4. 絞り込み
- 道具・ランクの絞り込みは「どの行(プリセット)を出すか」で表す: クエリ `presets`(カンマ区切り。form・explode: false)。
  省略時は 6 行すべて。例: スカーフだけ → `presets=max-scarf`、ランクなし → `presets=uninvested,neutral-max,max,max-scarf`。
- 指定の順は結果に影響しない(段の中の順は常に §2 の順)。未知の ID・重複・空(`presets=`)は 400 `invalid_request`。

### 5. API
`GET /api/speed/v1/table?presets=...` → 200
```json
{"regulationId": "example", "presets": ["uninvested", "..."],
 "tiers": [{"speed": 150, "entries": [{"pokemonId": "9001-000", "nameJa": "...", "types": ["fire"], "baseSpeed": 100, "preset": "max-scarf"}]}]}
```
- `presets` は実際に使った行の ID(§2 の順)。
- ヘッダー(400)→ クエリ(400)→ read model 未設定(503 `master_unavailable`)→ 200。provider のエラー・計算のエラー(read model が検証済みなので通常起きない)は 500 固定文言。

### 6. 細部
- コアは `Presets()`(定義の複製)・`NormalizePresets`(空 `ErrNoPresets`・未知 `ErrUnknownPreset`・重複 `ErrDuplicatePreset`。§2 の順に並べ直す)・`BuildTable` を公開する。
  HTTP はクエリを read model より先に検査する(provider が nil でもクエリが不正なら 400)。
- OpenAPI の `presets` クエリは minItems 1・uniqueItems、`SpeedTier.speed` は minimum 1。
- テストの期待値は ADR-0600 §6 と同じ限定的な例外として、SP1 の表(コア・HTTP)でも §2 の行と ADR-0600 §3 の式から手で導いた値を使う(過程をコメントに書く)。

### 7. 404 / 405 の応答(SP0 critic の軽微)
契約に無いパス・メソッドへの 404 / 405 は Echo の既定の応答(`{"message": ...}`)のままにする(balance と同じ)。クライアントは契約にあるパスだけを呼ぶので、
ErrorCode を増やしてまで揃える利点が無い。

## 却下した案
- 表を全件のフラットな行で返し、同速のまとめをクライアントに任せる: 同速の定義(値が同じ)をサーバーとクライアントの 2 か所に書くことになる。
- 道具・ランクを別々のクエリ(`item=scarf&rank=1`)にする: 6 行の組み合わせは固定で、行の選択で過不足なく表せる。
