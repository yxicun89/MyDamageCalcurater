# ADR-0600: 素早さ比較 SP0 基盤

- 状態: 採用(2026-09-22。§1 はユーザー決定・回答。§2 以降は素早さレーンの判断)
- 日付: 2026-09-22
- 関連: docs/speed-design.md、ADR-0002(マスタの扱い)、ADR-0004(4096 基準・五捨五超入)、ADR-0012(サービス境界)、ADR-0014(balance の read model の形)

## 背景
ユーザーの要望で、素早さ比較を 3 つ目の独立サービスとして作る(DECISIONS.md 2026-09-22)。SP0 では、以後の段階(表・自分の位置・画面)が
乗る基盤として、サービスの骨格・素早さの計算コア・暫定の read model・最小の API を決める。

## 決定

### 1. ユーザー決定・回答(2026-09-22)
- 独立サービス `services/speed/`、Web の独立タブ。表は各ポケモン 6 行(無振り / 準速 / 最速 / 最速スカーフ / 最速+1 / 最速+2)。
- 右の入力は「無振り / 準速 / 最速」+ スカーフ on/off の最小の選択。オプションで SP 0〜32・性格の補正 3 通り・ランク -6〜+6・スカーフ、または実数値の直接入力。
- 同じ実数値は同速としてまとめて表示。表に載せるのは既定のレギュレーションの使用可能集合。
- Web の骨組み(`web/`)が無い間は、`web/src/speed/` の画面部品とテストだけ先に作り、タブの登録は骨組みができてから 1 項目足す。

### 2. サービスの骨格
- Go モジュール `example.com/pokecalc/services/speed`(独立モジュール。`go.work` に `use ./services/speed`)。
  engine へは `replace example.com/pokecalc/engine => ../../engine` で依存し、`GOWORK=off` でもビルド・テストできるようにする。
- 構成は balance に倣う: `cmd/api`(設定は環境変数を 1 か所で読む)/ `internal/speed`(純粋なコア。I/O なし)/ `internal/httpapi`(Echo v5)/
  `internal/master`(read model の読み込み)/ `internal/api`(oapi-codegen の生成物。`make speed-gen`)/ `api/openapi.yaml`(speed 自身の契約)。
- Makefile は `services/speed/Makefile`(ターゲットは `speed-` 接頭辞)。ルートの `test` / `lint` / `build` に前提条件として含める(balance と同じ)。
- Docker イメージは engine を含むため、**ビルドコンテキストはリポジトリのルート**(`docker build -f services/speed/Dockerfile .`)。
  ベースイメージは balance と同じ golang 1.27-alpine の digest 固定 → scratch、非 root。
- Kustomize は `deploy/k8s/base`(Deployment・Service・Ingress `/api/speed`)と `overlays/local`(架空データの read model を ConfigMap でマウント)。
  **GitOps の overlay と Argo CD Application は SP4 で作る**(イメージの digest はレジストリに初めて置いたときに決まり、digest 固定の overlay を先に作れないため)。

### 3. 素早さの計算コア
入力: 素早さ種族値(1〜255)・素早さ SP(0〜`engine.MaxSPPerStat`)・性格の補正(`minus` / `neutral` / `plus`)・ランク(-6〜+6)・スカーフの有無。

1. 実数値: `engine.RealStats`。性格は `plus` = 素早さ上昇(下降は攻撃)、`minus` = 素早さ下降(上昇は攻撃)、`neutral` = `engine.NatureNeutral`。
   素早さ以外の補正は素早さの値に影響しないので、相手のステータスは何でもよい。
2. ランク: `engine.EffectiveStat(in, engine.StatSpe)`(engine の式・floor)。
3. こだわりスカーフ: ランク補正の**後**に、4096 基準の補正 6144(×1.5)を五捨五超入で掛ける: `v × 6144 / 4096` の端数が 0.5 を超えれば切り上げ、0.5 ちょうど以下は切り捨て
   (= floor((v × 6144 + 2047) / 4096))。Showdown の `Pokemon.getStat`(ランクを floor で適用)→ `ModifySpe` イベント(`chainModify(1.5)` → `modify` の五捨五超入)の順。
   例: 実数値 201 → 301(301.5 は切り捨て)、200 → 300。
- スカーフの補正は engine に無い(Champions に無い持ち物で、ダメージ計算の engine の対象外。CLAUDE.md)。ユーザーの仕様で表の行として必要なので、
  **speed のコアが名前付き定数 1 か所で持つ**(`scarfSpeedModifier = 6144`。出典はこの節)。engine の非公開の `pokeRound` は複製せず、
  上の 1 式だけを speed に置く。engine に素早さの補正を足すかは、データレーンに DECISIONS.md で提案する(足されたら speed はそれを呼ぶ)。
- 表の 6 行(docs/speed-design.md §5)は「入力の作り方の型」(ADR-0009 と同じ考え方)で、マスタではないので speed のコアが 1 か所に持つ(SP1)。
  「無振り」は準速と同じく性格補正なし(SP 0)とする。
- まひ・おいかぜ・トリックルーム・特性(すいすい等)は SP0〜SP2 の対象外。

### 4. 暫定の read model(架空データ)
環境変数 `SPEED_POKEMON_PATH` の JSON。未設定なら起動はし、`/healthz` は 200、ポケモンを使う API は 503 `master_unavailable`(balance と同じ)。
設定されているのに読めない・不正なら起動を失敗させる(一部だけ使わない)。

```json
{"schemaVersion": 1, "regulationId": "example", "pokemon": [
  {"pokemonId": "9001-000", "nameJa": "テストカソウドリ", "types": ["fire", "flying"], "baseSpeed": 100}
]}
```
- `schemaVersion` は 1 だけ。`regulationId` は `^[a-z0-9]+(-[a-z0-9]+)*$`。このファイル全体が 1 つのレギュレーションの使用可能集合。
- `pokemon` は 1 件以上。`pokemonId` は `NNNN-NNN` で重複不可。`nameJa` は空でない。`types` は 1〜2 個・重複なし・英小文字の ID(表示用。
  18 タイプの一覧はコードに持たず形式だけ検査する)。`baseSpeed` は 1〜255。未知のフィールドは拒否。
- 例は `services/speed/testdata/pokemon.example.json`(架空の ID `9xxx`・架空の名前)。local overlay のコピーは Go のテストで同期を検査する(balance と同じ)。
- SP4 で pokedex の read model(データレーン P2-3)の adapter に差し替える。コアは `PokemonProvider` インターフェースだけに依存する。

### 5. API(SP0)
- `GET /healthz`・`GET /api/speed/healthz` → 200 `{"status":"ok"}`。
- `GET /api/speed/v1/pokemon` → 200 `{"regulationId": "...", "pokemon": [{"pokemonId","nameJa","types","baseSpeed"}]}`(pokemonId の昇順)。
  ヘッダー `X-Device-Id`・`X-Session-Id` が必須(欠落・空は 400 `invalid_request`。CLAUDE.md の技術規約・balance と同じ)。read model 未設定は 503 `master_unavailable`。
- エラーは `{"code","message"}`。コードは OpenAPI の enum を正とする。想定外のエラーは 500 の固定文言。
- 表(SP1)・自分の位置(SP2)の endpoint は各段階の ADR で足す。

### 6. テストの期待値
- test-strategy.md の「期待値を手計算しない」は、ダメージ計算を外部実装(@smogon/calc)と照合する原則。実数値とランクの式は engine のゴールデンテストで担保済みで、
  speed はそれを呼ぶだけ。speed が自前で持つのはスカーフの 1 式(§3)と適用の順だけなので、その単体テストは**式の出典(§3)から手で導いた少数の値**で書く
  (201→301・ランク後に掛かること等。計算過程をコメントに書く)。この限定的な例外は SP0 のコアの単体テストだけに適用する。
- read model の `types` は `^[a-z]+$`、`nameJa` は前後の空白を除いて空でないこと。500 の文言は `"internal error"` 固定(balance と同じ)。

## 却下した案
- 実数値の式を speed に複製する: ドメイン式の正は engine(CLAUDE.md・coding-rules §2)。
- engine にスカーフを足す: engine はデータレーンの範囲で、Champions に無い効果を engine のダメージ計算に入れない方針。提案に留める。
- ルートの `api/openapi.yaml` に speed を足す: balance と同じく独立サービスの契約はサービス内に置く(ADR-0012)。
