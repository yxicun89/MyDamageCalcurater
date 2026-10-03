## 2026-09-22: PR #14(API P3-1 calc-svc・依存の最新化)を main に統合
Decision: 他レーンの状況(開いている PR は #14 のみ、Web・iOS の未マージの変更は api/openapi.yaml・services と競合しない)を確認し、main を取り込んで再検証(make test 12 件・lint・build・check-publishable 0 件・make gen 差分なし)してからマージした。
Reason: ユーザーが「テストとか諸々通っているならマージしていい。他のレーンの状況確認してから」と回答した。
Impact: Web(P4-5)・iOS は新しい api/openapi.yaml(category・BulkCalcRow.defender・逆算の新形・ErrorCode)に追従できる。
