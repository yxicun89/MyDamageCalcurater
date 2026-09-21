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

## 2026-09-21 (Claude Code: P1-8 逆算)
### Done
- 先に docs の Codex 役割の記述を訂正(969d335)。scanner → spec-writer(ADR-0010 とテスト先行)→ implementer → critic 2回(FAIL→FAIL)→ 修正 → 指示された修正をメインが確認して完了
- engine/reverse.go(CalcReverse: 格子の総当たり×性格クラス×持ち物 → 型に畳み込み)、docs/adr/0010-reverse-estimation.md
- Recall@5(固定シード・1,000ケース・1,392種): defender 92.9%/95.6%、attacker 95.0%/95.5%(基準 80%/95%)。Recall 基準本体は allspecies タグ(make test-all-species)、make test には小標本の Smoke
### Open issues
- 2回観測の余裕が 0.5〜0.6pt。順序規則(ADR-0010 §6.3)を触ると基準を割る
- 表示 % の丸め(round-half-up)は仮定・人間の確認待ち。API 契約との差は plan.md P3-1 に持ち越し
- 任意の外部 Codex レビューは未実施
### Next
- P1-9 WASM

## 2026-09-21 (Claude Code: P1-9 WASM)
### Done
- scanner 省略 → spec-writer(ADR-0011 とテスト先行)→ implementer → critic 1回目 FAIL(受け入れ条件3点が無検証)→ 修正 → メインが確認して完了。engine 本体は無変更
- engine/wasmapi(DTO・検証・エラー写像。純粋)、engine/cmd/wasm(syscall/js の登録のみ)、engine/cmd/wasmexpect、scripts/wasm.sh、scripts/wasm-conformance.mjs、make wasm / make test-wasm
- Go/WASM が 32 ベクタ × 2周でバイト一致。engine.wasm 4.63 MB(gzip 1.32 MB)、逆算の最悪 16 ms(Node)
### Open issues
- ブラウザでの実動作は未確認(Node のみ)。P4-5 で人間が確認(plan.md ブロッカー)
- ADR-0011 §10 の API 契約差分は P3-1 / P4-5 に持ち越し(plan.md)
- CLAUDE.md のリポジトリ構成表に engine/wasmapi/ と engine/cmd/wasmexpect/ が無い(critic の提案。ユーザー確認待ち)
- 任意の外部 Codex レビューは未実施
### Next
- P2-1 データソース調査

## 2026-09-21 (Claude Code: P2-1 データソース調査)
### Done
- 調査担当エージェントが ADR-0002(暫定)を作成。メインが主要事実を再現確認(@smogon/calc 0.12.0 は MIT・Champions 世代あり・持ち物166件・こだわり系/とつげきチョッキ/進化の輝石なし・ヌケニンなし)
### Open issues
- ADR-0002 の人間の確認事項10項目(plan.md ブロッカー)。requirements.md の逆算の持ち物候補(こだわり系)が現行の集合と食い違う
- 調査担当の報告のうち、再現確認していない点(PokeAPI・Showdown・Serebii・Bulbapedia の内容)は ADR 上も未確認/取得(要約経由)として区別されている
### Next
- 人間の確認後に P2-2

## 2026-09-21 (Claude Code: ユーザー決定の反映)
### Done
- ユーザーの決定(マスタ方針・プリセット再定義・逆算の再設計・表示%・WASM・構成表)を docs に反映: ADR-0002 確定、requirements.md(こだわり系除外・プリセット定義・逆算・小数第1位%)、.gitignore(data/generated/)、README(第三者データ非配布)、CLAUDE.md(構成表・データ/レギュレーション規約)、plan.md(Phase 1b・P2-1b・ブロッカー整理)、DECISIONS.md
### Open issues
- testdata/golden と非コミット方針の関係(ユーザー確認待ち)。実機観測%の丸め規則。技の食い違い。メガ石対応・フォーム・更新運用
### Next
- P1-10 防御プリセットの再定義

## 2026-09-21 (Claude Code: コーディング規約 v2 と P1-10)
### Done
- docs/coding-rules.md を起草 → Codex に read-only でレビューさせ「要修正」→ v2 に反映 → 再レビュー依頼(未回答)。CLAUDE.md / AGENTS.md から参照。plan.md に Phase R を追加
- P1-10: spec-writer → implementer → critic(FAIL: ドキュメント3点のみ、コード・テスト・ゴールデンは全項目合格・変異生存ゼロ)→ 私がドキュメントを修正(ADR-0011 の例・test-strategy.md・openapi の説明・design.md・ADR-0009 §2-a)して完了
### Open issues
- Codex の v2 再レビューが未回答。LICENSE の方針が未定。main が進んでいる(pokecalc-main worktree)ため、feat/claude-p1-engine の main へのマージ時に確認が要る
### Next
- Phase R

## 2026-09-21 (Claude Code: Phase R の是正とユーザー回答の反映)
### Done
- R-2-1(シェル)・R-2-2(fixture の公式名を架空名に)・R-2-3(oracle の完全固定・.gitignore)・R-2-4(ドメイン定数の命名)を、挙動不変(golden の sha256・件数不変)で実施しコミット
- 監査へのユーザー回答を反映: ADR-0013(タイプ相性表をデータ化)、ADR-0009 §1-a(Label)、ADR-0002 の第三者データ抜粋を削除、規約の module path・LICENSE・データ化の線引き、plan.md に P1-13 / R-2-8 / R-2-9
### Open issues
- R-2-9(履歴の書き換え)は Codex の worktree と main worktree への影響があるため、実行前にユーザーと調整。Codex の docs/type-balance-design.md にレートリミット等の記述が残る(Codex 担当のため未編集)
### Next
- R-2-5(実装中)→ R-2-8 → R-2-9 → R-3 → P1-13

