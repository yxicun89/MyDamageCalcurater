---
name: phase
description: docs/plan.md のマイルストーンまたはタスクをワークフロー(scanner→spec→implement→critic)で自動実行する。例 /phase M1、/phase P1-3
---
# /phase

対象: $ARGUMENTS(省略時は plan.md の最初の未完了タスク)

対象に含まれる未完了タスクを上から順に、1つずつ次の手順で処理する。

1. plan.md のタスクを `[~]` にする
2. `quick-scanner` に影響範囲を調べさせる
3. `spec-writer` に受け入れ条件とテストを書かせる
4. `implementer` に実装させる
5. `critic` にレビューさせる。FAIL なら指摘を渡して 4 に戻る(最大3回)
6. engine・逆算・DBスキーマ・API契約のタスクなら `scripts/codex-review.sh` を実行し、重大な指摘があれば 4 に戻る(Codex が無ければスキップ)
7. plan.md を `[x]` にし、1タスク1コミット(`P1-3: ダメージ計算コア` の形式)
8. 3回ループしても通らなければ `[!]` にして「ブロッカー」に記録し、依存しない次のタスクへ進む

マイルストーンの最後に `/verify` を実行し、人間向けの動作確認手順を `docs/verify-<M>.md` に書いて止まる。
人間の確認が必要なこと(CLAUDE.md 参照)に当たったら、その時点で止まって依頼する。
