## 2026-09-21: balance の API 契約はサービスローカル OpenAPI を正とする
Decision: balance の契約は `services/balance/api/openapi.yaml` を正とし、oapi-codegen で型を生成する。
ルート `api/openapi.yaml` は damage/gateway の契約として Codex は変更しない。
Reason: damage-calc と balance を兄弟のドメインモノリスとして分離しつつ、仕様先行と生成型の規律を維持するため。
Impact: CLAUDE.md の「API はルート OpenAPI が唯一の正」は damage/gateway の範囲に限定して読み替える。
balance の契約変更はサービス内 spec → 生成 → テストの順で行う。詳細は ADR-0012。
