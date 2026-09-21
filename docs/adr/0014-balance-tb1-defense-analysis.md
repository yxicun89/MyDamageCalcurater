# ADR-0014: balance TB1 の防御タイプバランスの契約とポケモンタイプの取得

- 状態: 採用(タイプバランスレーン。人間の判断を要する未決事項は下の「未決」に分けた)
- 日付: 2026-09-21
- 関連: docs/type-balance-design.md §6 TB1・§7・§8、ADR-0002(マスタの取得元)、ADR-0012(サービス境界と共通マスタ)、
  ADR-0013(タイプ相性表はデータ)、docs/ai-shared/claude-review.md

## 背景

TB1 は最大6体のパーティについて、18 の攻撃タイプそれぞれに対する各メンバーの防御倍率と、攻撃タイプごとのチーム集計を返す。
TB0 の契約は request を `pokemonId` のみにしている(claude-review.md「API request: pokemonId のみ」)。
このため balance は `pokemonId` からタイプを引く必要があるが、次の制約がある。

- ADR-0012: balance は pokedex-svc・damage-calc を実行時に呼ばない。共通マスタはビルド時生成物またはローカル read model として使う。
- ADR-0002: 実 Pokémon マスタ・生成済みスナップショットは Git にコミットしない(`data/generated/` は Git 管理外)。
- 共通マスタのスナップショットの schema は P2-2(ダメージ計算レーン)で設計中で、まだ無い。

## 決定

### 1. request は `pokemonId` のみのまま(TB0 の契約を維持)
タイプを request に含める案は採らない(claude-review.md の回答を維持。クライアントごとにタイプが食い違うのを防ぐ)。

### 2. ポケモンのタイプは provider 境界の後ろで引く
- 純粋コア(`internal/balance`)に `PokemonTypeProvider`(pokemonId → 1〜2 タイプ)の interface を置く。
  未登録の ID は `ErrUnknownPokemon`。TB0 の `TypeChartProvider` と同じ形。
- TB1 の adapter は **balance ローカルの read model ファイル**(JSON)を起動時に1回だけ読む。パスは環境変数
  `BALANCE_POKEMON_TYPES_PATH`。形式:
  ```json
  {"schemaVersion": 1, "pokemon": [{"pokemonId": "9001-000", "types": ["fire", "flying"]}]}
  ```
  検証: `schemaVersion` は 1、ID は `NNNN-NNN` で重複なし、タイプは 1〜2 個・18 タイプのいずれか・重複なし。
  不正なファイルは起動時エラーで終了する(黙って一部だけ使わない)。
- 環境変数が未設定のときは provider を持たずに起動し、analyze は **503 `master_unavailable`** を返す(health は 200)。
  未登録の `pokemonId` は **422 `unknown_pokemon`**。
- この read model は temporary であり、共通マスタのスナップショット schema(P2-2)が決まったら adapter だけを差し替える。
  Git に置くのは schema と**架空データの example**(ID 9001-000 以降、実在ポケモンの ID・名前を使わない)だけ。
  local overlay の k3d smoke は、その example を ConfigMap でマウントして 200 を確認する。

### 3. 倍率の分類と集計の定義
メンバーごとの倍率(TB0 の整数表現。4 = 等倍)を次の6分類にする。

| 倍率 | category | 表示の例(UI) |
|---|---|---|
| 16 | `quad_weak` | ×4 弱点 |
| 8 | `weak` | ×2 弱点 |
| 4 | `neutral` | ×1 等倍 |
| 2 | `resist` | ×1/2 耐性 |
| 1 | `quad_resist` | ×1/4 耐性 |
| 0 | `immune` | ×0 無効 |

攻撃タイプごとのチーム集計(設計書 §6 の5項目):

- `weak`(弱点持ち数)= ×2 と ×4 の人数。`quadWeak`(4倍弱点持ち数)= ×4 の人数で、`weak` の内数。
- `resist`(耐性持ち数)= ×1/2 と ×1/4 の人数。無効は含めない。
- `immune`(無効持ち数)= ×0 の人数。
- `neutral`(等倍数)= ×1 の人数。
- 不変条件: `weak + resist + immune + neutral = メンバー数`。

総合点・ランキング・独自スコアは作らない(設計書 §6)。

### 4. response の形
- 順序を安定させるため map ではなく配列にする。members は request の順、各配列のタイプは正準順(normal … fairy)。
- 倍率は文字列 `"0" "1/4" "1/2" "1" "2" "4"`(設計書 §7「表示用文字列へ変換してよい」)と `category` を併記する。
- 各倍率に `source`(`type` / `ability`)を付ける。TB1 では常に `type`(claude-review.md 指摘3、TB3 のための型)。
- 同じ `pokemonId` の重複は許す(分析の入力として禁止する理由がない。種族の重複可否はレギュレーション依存で balance の責務外)。

### 5. 細部(spec-writer が挙げた未決の確定。レーン内の実装判断)
1. response の各メンバーの `types` は read model の順のまま返す(並べ替えない)。「正準順」は `defense` と `teamSummary` の攻撃タイプの並びを指す。
2. read model の `pokemon` が空配列、またはキーが無いものは不正(起動時エラー)。空のマスタで起動して全件 422 になるのを防ぐ。
3. `BALANCE_POKEMON_TYPES_PATH` が空文字のときは未設定と同じ扱い(Kubernetes の env で空値が入る場合と区別しない)。
4. 判定順は ヘッダー(400)→ body(400/413)→ provider 未設定(503)→ ID 解決(422)→ 200。不正な request には provider の有無によらず 400 を返す。
5. 上記以外の内部エラー(相性表の欠落・相性表や provider の想定外の失敗)は **500 `internal_error`**(message は固定文言。内部の詳細を返さない)。OpenAPI に 500 を追加する。
6. 422 の message には、見つからなかった `pokemonId`(クライアント自身の入力)を含める。

## 未決(人間の判断が要るもの。既定案で進めている)
- 共通マスタの配布方法(GitOps でデプロイした balance に、Git 管理外の実データをどう渡すか)。既定案: P2-2 の設計で決まる方法に合わせ、
  それまでは local overlay の example と、利用者が用意したファイルのマウントで運用する。

## 却下した案
- request にタイプを含める: クライアント間でマスタが分岐する。TB0 の契約(レビュー済み)を変える理由がない。
- balance に実データを同梱する: ADR-0002 に反する。
- 起動時に pokedex-svc から取得する: ADR-0012 に反する(実行時の依存)。
