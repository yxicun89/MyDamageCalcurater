## 2026-09-22: PR #23(API P3-2 gateway)を main に統合(深夜。ユーザーの指示に基づく)
Decision: critic PASS(NG 2回のあと3回目)、make test 14 件・lint・build・check-publishable 0 件・make gen 差分なし、開いている他の PR 無し・Web/iOS/データのブランチに競合する変更無し、を確認してマージした。
Reason: ユーザーが同日「PR はテストとか諸々通っているならマージしていい。他のレーンの状況確認してから」と指示した(深夜のマージ条件より優先)。
Impact: Web(P4-5)・iOS は gateway 経由の API(ErrorCode に invalid_header、X-Device-Id / X-Session-Id は UUID)に追従する。
