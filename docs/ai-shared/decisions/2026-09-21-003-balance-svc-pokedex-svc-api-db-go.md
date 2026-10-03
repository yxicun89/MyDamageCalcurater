## 2026-09-21: balance-svc は pokedex-svc の API を呼ぶ(DB直結・Goモジュール共有はしない)
Decision: マスタデータ取得は pokedex-svc の REST API 経由。型定義の共有モジュール化は当面しない。
Reason: DBスキーマ変更やモジュール変更で相手のビルド・デプロイが壊れることを防ぐ。
Impact: balance-svc は独自DBを持たない(TB1時点)。将来必要なら DECISIONS.md に追記して再検討。
