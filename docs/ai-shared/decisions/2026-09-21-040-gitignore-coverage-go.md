## 2026-09-21: ルートの .gitignore の `coverage.*` を Go のカバレッジ出力だけに絞る提案(タイプバランスレーンから。既定案)
Decision(提案): `.gitignore` の `coverage.*` は `coverage.go` / `coverage.ts` などのソースも無視してしまう(TB2 で `services/balance/internal/balance/coverage.go` が黙ってコミットから漏れかけた)。
既定案: `coverage.*` を `coverage.out` と `coverage.html`(と各ツールの実際の出力名)に置き換える。持ち主はルートの共有ファイルなのでデータレーンが判断する。
Reason: `git status` に出ないため、テストはローカルで通るのに clone すると壊れる状態になる。Web レーンの `coverage.ts` 等でも起きうる。
Impact: タイプバランスレーンは回避のため本体を `offense.go` にした(変更不要)。他のレーンは、新しいファイルが `git status` に出ることを確かめてから commit する。
