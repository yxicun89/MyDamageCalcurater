# AGENTS.md (Codex 向け)

このリポジトリのルールは `CLAUDE.md` が正です。必ず先に読んでください。

Codex の役割は次の2つです。

1. 別モデルによる **レビュー**(`scripts/codex-review.sh` からの依頼)
2. **タイプバランスチェッカー**(`services/balance/`)の実装

## 共有状態(docs/ai-shared/)

Claude Code と Codex は記憶を共有しない。共有記憶は `docs/ai-shared/` だけ。

1. 作業開始時: `docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md` を読む(使い方は `README_AI_SHARED.md`)
2. 作業終了時: 自分のログ(`CODEX_LOG.md`)に追記し、`CURRENT_STATE.md` の自分の担当欄を更新する

## レビュー担当としてのルール

- レビューでは `CLAUDE.md` の「絶対ルール」「ドメイン規約」違反を最優先で指摘する
- 特に見る点: ダメージ計算の丸め順序、4096基準の補正、SP換算、境界値(0/32/66)、逆算の探索漏れ、サービス間のDB越境
- 指摘は「重大 / 重要 / 軽微」に分類し、ファイルと行を示す
- 依頼がない限りファイルを変更しない

## 実装担当としてのルール(タイプバランスチェッカー)

### 担当範囲(厳守)

- **担当**: `services/balance/`(タイプバランスチェッカー)とその Deployment/Service/Kustomize
- **担当外・変更禁止**: `engine/`, `services/pokedex/`, `services/calc/`, `services/record/`, `services/team/`, `web/`, `ios/`, `api/openapi.yaml` の damage 関連エンドポイント
  - pokedex-svc の API は **呼ぶだけ**。実装やスキーマの変更はしない
  - 変更が必要だと思ったら実装せず `docs/ai-shared/DECISIONS.md` に提案を書いて止まる
- ダメージ計算アプリ側の未完了タスクを「引き継ぎ」として実装しない

### 最初に読むもの(この順で)

1. `docs/ai-shared/CURRENT_STATE.md`
2. `docs/ai-shared/DECISIONS.md`
3. `docs/type-balance-design.md`(設計書。実装の唯一の起点)
4. `docs/ai-shared/claude-review.md`(Claude によるレビューと修正指摘)
5. 必要なときだけ `docs/ai-shared/CLAUDE_LOG.md`

### 規約

- `services/balance/internal/balance/` は純粋 Go(HTTP・DB・Kubernetes に依存しない)
- 倍率は float ではなく整数表現(claude-review.md 参照)
- TB1 の時点から `EffectSource`(タイプ由来/特性由来)を型に持たせる
- マスタデータは pokedex-svc の REST API から取得する。DB には直接繋がない
- 認証なし。pokecalc と同じ端末ID/セッションIDの流儀に合わせる
- manifest は Kustomize(`services/balance/deploy/k8s/base` + `overlays/local`)
- Argo CD Application は balance 専用に分ける。Sync は最初 manual

### 完了条件

- ユニットテストが通ること(`go test ./services/balance/...`)
- k3d 上で `/api/balance/v1/team-balance/analyze` が疎通すること
- セッション終了時に `docs/ai-shared/CODEX_LOG.md` と `CURRENT_STATE.md` の自分の担当欄を更新すること

## Git ブランチ運用

- Claude Code: `feat/claude-<phase名>`(例 `feat/claude-p1-engine`)。Phase 単位で切る。
  その Phase の `/verify` が通ったら main へマージしてブランチを削除する
- Codex: `feat/codex-<stage名>`(例 `feat/codex-tb0-foundation`)。TB ステージ単位で切る。
  そのステージのテストが全件通ったら main へマージしてブランチを削除する
- 単発の修正: `fix/claude-...` / `fix/codex-...`。そのセッション内でマージまで完了させる。
  次のセッションに持ち越さない
- 1つのブランチに複数の Phase/ステージ分の作業を積み上げない
- レートリミットや上限で片方が触れなくなることがある。もう片方が引き継ぐことはしない。
  ブランチが宙に浮いた場合は、内容が明確なら担当外でも完了させてよい
  (判断基準は「ルール違反や設計変更を含まないか」)。
  不明瞭なら `DECISIONS.md` に保留として記録し、元の担当が次にそのステージへ着手する際に
  新規タスクとして扱う(引き継ぎ資料は作らない)
