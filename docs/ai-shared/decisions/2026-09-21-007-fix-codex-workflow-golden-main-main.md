## 2026-09-21: fix/codex-workflow-golden を main へマージし、以後は各自 main から作業する
Decision: 保留を撤回し、ユーザー判断で fix/codex-workflow-golden を main へ --no-ff マージ(ブランチは削除)。
Claude Code と Codex の作業を1つのブランチで混ぜない。相手の作業を使いたいときは、相手のブランチを一度 main に
マージしてから、各自が main から自分のブランチを切って作業する(AGENTS.md「Git ブランチ運用」に追記)。
Reason: 保留のまま Claude が続きの作業を積むと Codex の未レビュー作業と混ざる。一度 main に確定させて起点をそろえる方が
安全というユーザー判断。マージ後の main で make test / make test-golden(キャッシュ無し) / make lint / make build は成功。
Impact: AGENTS.md / CLAUDE.md の衝突は、Codex の共通ワークフロー(開始時と Git 運用・エージェントの使い分け・検証と終了時)と
main 側のブランチ運用・ai-shared 参照・Type Balance 担当範囲を両方残して統合。Codex 側の「feature/<内容> 等」は
新命名(feat/claude-... / feat/codex-...)に、「引き継ぎ文書に分ける」は「docs/ai-shared に書き、別途の引き継ぎ資料は作らない」に直した。
未確認: engine の丸め順訂正(ADR-0008)と openapi.yaml の level 固定は、実装者と別の reviewer による独立レビューを
まだ受けていない。P1-6 は [~] のまま、Claude Code 側で critic レビューを通してから [x] にする。
