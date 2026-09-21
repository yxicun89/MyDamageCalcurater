# ADR-0012: damage-calc / balance のサービス境界と共通マスタ

- 状態: 採用
- 日付: 2026-09-21

## 背景

既存構想には gateway、pokedex、calc、record、team のサービス分割案がある一方、実装済みなのは
純粋 Go の damage engine が中心で、pokedex-svc とマスタ importer は未実装である。
balance の TB0 を未実装 API の完成まで止めると、独立して検証できる基盤まで進められない。

## 決定

1. 機能ドメインの主要な実行単位は `damage-calc` と `balance` の2つとする。
2. 各サービス内部はモノリスとし、pokemon/type/move/ability/damage-formula 単位へ分割しない。
3. damage-calc と balance は兄弟サービスとし、互いの API へ直接依存しない。
4. ポケモン・タイプ・技・特性・フォーム・レギュレーション等の共通マスタは恒久正本を1つにする。
5. 正本の具体的な格納・生成方式は `feat/claude-p1-engine` 上の ADR-0002 の人間確認後に確定する。
   同 ADR はまだ main 未統合であり、現時点ではそこで提案されたバージョン固定のコミット済み
   スナップショット案を第一候補とする。
6. サービスは必要な範囲をビルド時生成物またはローカル read model として利用できる。
   共通マスタのためだけに新しい実行時サービス間依存を追加しない。
7. TB0 のタイプ相性表は temporary adapter として balance 内に置いてよい。純粋コアに provider
   interface を置き、正式マスタ確定後に adapter のみを差し替える。
8. balance の外部 API 契約はサービス境界内の `services/balance/api/openapi.yaml` を正とし、
   oapi-codegen で型を生成する。ルート `api/openapi.yaml` は既存 damage/gateway 契約の正として維持する。

## 責務

- damage-calc: あるポケモンが、ある条件で技を使った場合のダメージを計算する。
- balance: あるパーティのタイプ相性、弱点、耐性、攻撃範囲を分析する。
- 共通マスタ: 両者が参照する識別子・静的事実の正本。分析・ダメージ計算ロジックは持たない。

## 影響

- balance は pokedex-svc の未実装 API を待たず TB0 を進められる。
- temporary type chart は正式データではないことを型名・文書・テストで明示する。
- 既存の pokedex-svc 計画は、この ADR だけでは削除・実装しない。P2 の確定時に、独立サービスが
  本当に必要か、damage-calc 内の master adapter/read model で十分かを再評価する。
- API 化は利用側と責務が確定した場合だけ行い、API 自体を目的にしない。
- 「API はルート `api/openapi.yaml` が唯一の正」という既存規約は damage/gateway の範囲に限定する。
  独立サービスである balance の契約をルートへ混在させず、各サービス内では同じ仕様先行・生成型の規律を守る。

## 却下した案

- balance から damage-calc を呼ぶ: 兄弟サービス間を密結合にする。
- TB0 のために pokedex-svc を先行実装する: Codex の担当外で、共通マスタ方式も未確定。
- balance の静的表を恒久正本にする: サービスごとにマスタが分岐する。
- coordination branch を作る: 既存の main と docs/ai-shared で同じ目的を満たせる。
