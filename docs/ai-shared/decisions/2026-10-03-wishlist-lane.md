## 2026-10-03: Wishlist レーンを追加(state ファイル・CURRENT_STATE の 1 行・ルート Makefile の include)
Decision: 別アプリ「欲しいものリスト」(`apps/wishlist/`)のレーンを追加した。状態は `state/wishlist.md`、`CURRENT_STATE.md` に 1 行足した。
ルートの `Makefile` に `include apps/wishlist/Makefile` を 1 行足し、`make test` / `lint` / `build` に `wishlist-*` を前提条件として合成した(balance と同じ方式)。
Go モジュールはルートの `go.work` に入れず `GOWORK=off` で扱う。
Reason: ユーザー決定(2026-10-03。state ファイルと CURRENT_STATE の追加、ルート Makefile の 1 行追加を許可)。CI で wishlist のテスト・lint を回すため。
Impact: 他レーンへの影響は `make test` / `lint` / `build` の時間が少し延びることだけ。wishlist は `apps/wishlist/` の外を変えない(仕様)。
ADR 番号の帯は使わず、設計判断は `apps/wishlist/docs/design.md` の W-番号で持つ(ダメ計の ADR と混ぜないため)。
