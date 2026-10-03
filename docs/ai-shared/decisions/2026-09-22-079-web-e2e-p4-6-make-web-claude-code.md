## 2026-09-22: Web の E2E(P4-6)とルートの make への組み込み(Web レーン、Claude Code)
Decision: (1) ユーザー決定どおり、`web/Makefile` が `test: web-test` / `lint: web-lint` / `build: web-build` を足した。依存が無い・lockfile より古いときは自動で `npm ci`。
(2) E2E は `make web-e2e`(オフライン = WASM。`/api` を遮断しても計算できること、engine.wasm は初回の計算まで読まないこと)と
`make web-e2e-online`(例データを書き出して calc-svc を `go run` で起動し、オンラインとオフラインの結果の行が一致すること)。chromium のみ。
(3) **提案(API レーン宛て、既定案)**: ルートの `make e2e`(`scripts/e2e.sh`。P3-3 の k3d スモーク)の最後で `make web-e2e` を呼ぶ。
gateway 経由のオンライン E2E(`VITE_API_BASE_URL` を gateway に向ける)は P3-3 の後に Web レーンが足す。
Reason: P4-6 の完了条件と、ユーザー回答(2026-09-22)の反映。
Impact: 他のレーンのルートの `make test` / `lint` / `build` で Web のテスト・型検査・ビルドも走る(初回は npm ci ぶん遅い)。
