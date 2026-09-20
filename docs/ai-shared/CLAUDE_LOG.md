# Claude Log

## 2026-09-21 (チャットでのレビュー、Claude Code未起動)
### Done
- type_balance_architecture_review.md をレビュー、claude-review.md に記録
- CURRENT_STATE.md / DECISIONS.md の初版を作成

### Changed files
- docs/ai-shared/claude-review.md (new)
- docs/ai-shared/CURRENT_STATE.md (new)
- docs/ai-shared/DECISIONS.md (new)

### Decisions
- claude-review.md 参照

### Open issues
- なし(未決事項は全て決定済み)

### Next
- Claude Code は damage-calc の P1-3 を継続。balance 側には触れない

## 2026-09-21 (Claude Code: 宙に浮いたブランチの解消とブランチ運用ルール導入)
### Done
- fix/codex-workflow-golden を確認して保留判断(engine コア・API 契約に触れるため)。作業は 6d86382 として保全
- pokecalc-ai-shared の内容をリポジトリへ展開(docs/ai-shared/、docs/type-balance-design.md、CODEX_KICKOFF.md、AGENTS.md へ担当範囲を統合)
- AGENTS.md に「Git ブランチ運用」、CLAUDE.md にその要約と docs/ai-shared への参照を追記

### Decisions
- DECISIONS.md の「fix/codex-workflow-golden は保留」「Git ブランチ運用ルールを導入する」を参照

### Open issues
- (解決済み)fix/codex-workflow-golden はマージ済み。ただし engine の丸め順訂正・openapi の level 固定は独立レビュー未実施

### Later (同日)
- ユーザー指示により pokecalc-kit-v2/ と UNBLOCK_AND_MERGE_KICKOFF.md を削除。kit-v2 の差分
  (spec-writer/critic の Sonnet 化、deep-critic、ループ上限2回など)は採用しない判断で、現行の CLAUDE.md の運用を維持
- ユーザー判断で fix/codex-workflow-golden を main へマージ(保留を撤回)。AGENTS.md / CLAUDE.md の衝突を統合し、
  「Claude と Codex の作業を1ブランチで混ぜない」をブランチ運用に追記

### Next
- feat/claude-p1-engine を main から切り、P1-6([~])を critic でレビュー → [x] → P1-7 以降

## 2026-09-21 (Claude Code: マージコーディネーター役の整備)
### Done
- AGENTS.md に「共有ファイルの編集規約」5点、CLAUDE.md に「Codexブランチの取り込み手順」を追記。実際のマージは未実施
### Open issues
- docs/type-balance-test-strategy.md が未作成。規約に入れていない共有の書き込み先(plan.md / docs/adr の番号 / api/openapi.yaml / deploy/k8s/base / gateway ルーティング)の扱いは要判断

## 2026-09-21 (Claude Code: P1-6 の独立レビューと完了)
### Done
- feat/claude-p1-engine を main から作成。quick-scanner で充足状況を確認し、critic(opus)が独立レビューして PASS
- test-strategy.md を実装(gz+マニフェスト、暫定の種族集合、確定数の照合範囲)に合わせて更新。plan.md の P1-6 を [x]、P2-1 に再生成の依存を記録
### Open issues
- 任意の外部 Codex レビュー(scripts/codex-review.sh)は未実施(Codex の担当は TB 実装でレビュー担当ではない。レートリミットが理由とは確認していない)。軽微・任意の指摘6点は plan.md「改善要望」に記録
### Next
- P1-7 一括計算

## 2026-09-21 (Claude Code: P1-7 一括計算)
### Done
- scanner → spec-writer(ADR-0009 とテスト先行)→ implementer → critic(1回目 FAIL: 場・急所・攻撃側のパススルー未検証ほか)→ 修正 → critic 2回目 PASS(変異41種中38検出、2等価、1到達不能)
- engine/bulk.go(CalcBulk / DefenderPresetCatalog / DefaultDefenderPresets)、docs/adr/0009-bulk-calc-presets.md
### Open issues
- hb_boost / hd_boost の定義は人間の確認待ち。任意の外部 Codex レビューは未実施(上記と同じ扱い)
- P3-1 で openapi.yaml の description を先に直す(変化技は none/hp のみ、presets:[] は省略と同じ 等)
### Next
- P1-8 逆算
