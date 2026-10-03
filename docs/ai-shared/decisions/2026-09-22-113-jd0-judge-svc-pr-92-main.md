## 2026-09-22: 判定 JD0(judge-svc 基盤)を PR #92 で main に統合
Decision: ADR-0700(基盤・上流の呼び方・エラー正規化・受け入れ条件8件)と、docs/judge-design.md §4 の未決事項5件の決定・`services/judge/` の実装(internal/client・internal/httpapi・cmd/api・deploy/k8s・Dockerfile・scripts/smoke.sh)を PR #92 で main に統合した。critic は3回目で PASS(1・2回目 NG はいずれも上流エラー文面への URL/host:port/ホスト名の漏洩。`transportFailureReason()` を固定語彙への分類に変更して解消)。
Reason: `make test`・`make lint`・`make build`(ルート)が緑、critic PASS、他レーンの範囲外変更なし(COORDINATION.md の共有ファイル規約の範囲内)を確認してマージした。
Impact: 判定レーンのブランチを `feat/judge-jd1` に切り替えた(JD0 の `feat/judge-jd0` は削除)。次は JD1(`POST /api/judge/v1/outspeed-and-ko`)。
