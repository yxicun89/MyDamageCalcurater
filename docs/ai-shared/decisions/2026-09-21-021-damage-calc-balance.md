## 2026-09-21: damage-calc と balance は兄弟のドメインモノリスとする
Decision: 「1機能ドメイン=1サービス、サービス内部はモノリス」とし、damage-calc と balance の
相互 API 依存を作らない。pokemon/type/move/ability/damage-formula 単位の新サービスは作らない。
Reason: Kubernetes を理由に責務を細分化せず、個人開発で管理可能な複雑さを保つため。
Impact: 既存の未実装 pokedex-svc を balance の必須ランタイム依存にはしない。既存文書の
「balance-svc は pokedex-svc API を呼ぶ」は本決定で置き換える。詳細は ADR-0012。
