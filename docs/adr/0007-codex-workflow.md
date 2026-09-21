# ADR-0007: Claude Code と Codex で共有する開発ワークフロー

- 状態: 採用
- 日付: 2026-09-21

## 背景

Claude Code と Codex の両方で開発を進められるよう、実装だけでなく既存の役割分担・検証・Git 運用を Codex にも適用する。
既存の `.claude/agents/` は scanner → spec-writer → implementer → critic の直列構成で、
重要タスクでは外部 Codex レビューを追加する。ADR-0003 では engine コアを親が test-first で実装し、
critic が独立レビューする適応を認めている。これらを無視した全面置換は行わない。

## 決定

- `CLAUDE.md` をドメイン・技術規約の正として保持し、Git・共通運用を `AGENTS.md` に集約する。
  Claude 固有の定義・skills は残し、両方が `docs/development-workflow.md` を参照する。
- Codex に scanner / spec_writer / implementer / reviewer / verifier の役割を用意する。
  architect は必要時に spec_writer が兼任し、役割数だけ子を毎回起動しない。
- Codex の仕様担当は読み取り専用で受け入れ条件・テストを設計し、実装担当または親が
  テストを先に書く。共有作業ツリーへの書き込みを指定担当に限定する。
- 重要タスクの独立レビューと spec/test-first を保持する。軽微な作業は親だけでよい。
  独立した調査、固定した差分のレビューと検証に限り通常最大 2 子を並列利用する。
- モデル ID は固定せず継承する。単純調査は low、通常作業は medium。
  高推論は必要な理由を事前説明する臨時の選択に留める。
- カスタム役割は現行公式形式の `.codex/agents/*.toml`、同時実行制限は `.codex/config.toml` に置く。
  フォーマットフックと広いコマンド許可はコピーしない。整形・lint・build を明示的に検証する。
- 終了コードだけで検証成功にしない。未実装ターゲット、テスト 0 件、外部レビュー skip を区別する。
- 新規 Skills は追加せず共通手順を文書で再利用する。進捗・引き継ぎは plan と関連文書に残す。

## 影響

ADR-0003 の既存 engine 実装方針は維持する。サブエージェント省略を軽微な作業へ広げる点と、
Codex の仕様担当が直接編集しない点は本 ADR による適応である。
独立レビューと別モデルレビューは同義ではなく、実施した種類を正しく記録する。
Codex から外部 Codex レビュースクリプトを再帰的に呼び出さない。

Git は既存変更を保持して作業ブランチ上で進める。1タスク1コミット、plan 更新の原則は維持し、
依頼されていない破棄・push はしない。設定ファイルの構文検証と実際のクライアント読込は別々に確認する。

## 参照

- [既存ワークフロー適応 ADR-0003](0003-kickoff-workflow-and-tooling.md)
- [共通開発手順](../development-workflow.md)
- [OpenAI 公式 Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)(2026-09-21 確認)
