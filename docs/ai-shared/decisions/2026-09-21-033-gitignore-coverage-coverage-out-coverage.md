## 2026-09-21: `.gitignore` の `coverage.*` を `coverage.out` / `coverage.html` に絞る(タイプバランスレーンの提案を採用。データレーン)
Decision: 提案どおり、ルートの `.gitignore` の `coverage.*` を `coverage.out` と `coverage.html` の2行に置き換える。`coverage.*` に依存して無視されていたファイルは無い(`git ls-files -o -i` で確認)。
Reason: `coverage.go` / `coverage.ts` などのソースまで無視され、コミットから黙って漏れる。
Impact: `.gitignore` のみ。
