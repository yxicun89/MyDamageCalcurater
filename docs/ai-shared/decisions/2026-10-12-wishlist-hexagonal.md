## 2026-10-12: 欲しいものリストの API もヘキサゴナルにそろえる(ユーザー決定)
Decision: `apps/wishlist/api` を ADR-0811 と同じ形(`internal/domain`・`internal/app`・`internal/adapters`、依存は外 → 内の一方向)に再配置した(#744)。向きはモジュール内の `internal/archtest` で検査する。`tools/archcheck` は `services/` だけを見るので、対象に入れない。
Reason: ユーザーの意向「ヘキサゴナルをこのプロジェクト全てで採用している」(2026-10-11)。計画(docs/plan/architecture-adoption.md)は apps/wishlist を対象外にしていたため、計画の記述も直した(ユーザー回答 2026-10-12「私が PR で直す」)。
Impact: ダメ計の各レーンへの影響なし。wishlist の新しいコードは domain / app / adapters のどこに置くかを apps/wishlist/docs/arch-spec.md で決める。
