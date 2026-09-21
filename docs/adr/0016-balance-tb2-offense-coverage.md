# ADR-0016: balance TB2 攻撃範囲の契約と技の取得

- 状態: 採用(2026-09-21。§1 の3点はユーザー回答。それ以外はタイプバランスレーンの既定案)
- 日付: 2026-09-21
- 関連: docs/type-balance-design.md §6 TB2、ADR-0014(TB1。read model・判定順・エラーの流儀)、ADR-0015(相性表)、ADR-0002、ADR-0012

## 決定

### 1. ユーザー回答(2026-09-21)
- **有効打 = 等倍以上**(×1 以上)。いまひとつ(×1/2)と無効(×0)は有効打に数えない。抜群(×2)は別に数える。
- **防御側は 18 の単タイプ**。複合タイプの評価は TB4(仮想敵診断)で扱う。
- **技はメンバーごとに技 ID を最大4つ**送る。技のタイプと分類(物理/特殊/変化)は balance 側の技の read model から引く。

### 2. API
- 新しい endpoint `POST /api/balance/v1/team-balance/coverage`(防御の analyze とは別のデータ構造。設計書 §6 TB2)。
  ヘッダー・body 上限・メンバー 1〜6・pokemonId の形式は analyze と同じ。
- request: `members: [{pokemonId, moveIds: [moveId, ...]}]`。`moveIds` は 0〜4 件、同じメンバー内で重複は 400。
  moveId の形式は `^[a-z0-9]+(-[a-z0-9]+)*$`・最大 40 文字(英小文字 ID。共通マスタの技 ID 形式が決まったら合わせる)。
- response:
  - `members[i]`: `pokemonId`、`moveIds`(request の順)、`attackTypes`(変化技を除いた技のタイプ。重複なし・正準順)、
    `coverage`(防御タイプ 18 件・正準順: `defenseType`、`bestMultiplier`、`effective`、`superEffective`)
  - `teamCoverage`(18 件・正準順): `defenseType`、`bestMultiplier`、`effectiveMembers`(×1 以上を取れるメンバー数)、
    `superEffectiveMembers`(×2 を取れるメンバー数)
  - `bestMultiplier` はその防御タイプに対する最大の倍率(`"0" "1/2" "1" "2"`)。攻撃技が1つも無ければ `null` で、`effective` / `superEffective` は false。
- **重複計上しない**(設計書 §6): 同じメンバーが同じタイプの技を複数持っても `attackTypes` は1回。チーム集計はメンバー単位で数える(技の数ではない)。
- 変化技(category `status`)は除外する(設計書 §6)。

### 3. 技の read model(ADR-0014 §2 と同じ流儀の temporary adapter)
- 純粋コアに `MoveProvider`(moveId → type・category)。未登録は `ErrUnknownMove`(`UnknownMoveError{MoveID}`)。
- adapter は環境変数 `BALANCE_MOVES_PATH` の JSON を起動時に1回読む:
  `{"schemaVersion":1,"moves":[{"moveId":"move-9001","type":"fire","category":"special"}]}`。
  検証: schemaVersion 1、`moves` は1件以上、moveId の形式と重複なし、type は 18 タイプ、category は physical/special/status、未知フィールド・後続 JSON は不正 → 起動失敗。
  空文字は未設定扱い。
- Git には**架空データの example** だけ(`move-9001` 以降。実在の技の名前・ID を使わない)。local overlay は ConfigMap でマウントする。
- 実データは後で `data/generated/` のスナップショットを読む adapter に置き換える(ADR-0014 と同じ。DECISIONS.md のユーザー決定)。

### 4. 判定順とエラー
ヘッダー(400)→ body(400/413)→ ポケモンの read model 未設定または技の read model 未設定(503 `master_unavailable`)→
pokemonId の解決(422 `unknown_pokemon`)→ moveId の解決(422 `unknown_move`、message は `unknown moveId: <ID>`)→ 200。
それ以外の内部エラーは 500 `internal_error`(固定文言)。pokemonId も解決するのは、ID の検証を analyze とそろえ、将来 STAB 等でタイプを使うため。

### 5. 範囲外(このステージで扱わない)
- タイプ相性表どおりでない技(相手のタイプで倍率が変わる技、2タイプを持つ技、固定ダメージ技など)の特例。技の効果データが要るので TB3 以降。
- STAB・特性・テラスタル。

## 却下した案
- analyze の response に攻撃範囲を足す: 設計書 §6「防御タイプ分析とは別のデータ構造」に反し、技の read model が無いと防御分析まで 503 になる。
- 技のタイプを request で送る: ユーザー回答(ID を送る)と TB1 の方針に反する。
