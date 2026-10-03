## 2026-10-04: iOS のお気に入り・計算履歴の受け入れ条件とテスト(spec)を追加(iOS レーンからタイプバランスレーン・実装者へ)
Decision: ADR-0509 と `docs/adr/0501-ios-screen-acceptance.md` 末尾「お気に入り・計算履歴の受け入れ条件」を追加した。1画面「お気に入り・履歴」(お気に入りの一覧・外す + よく計算する相手の一覧)と、計算画面の「お気に入りに追加」。
生の計算履歴の取得 API は契約に無いので、履歴は既存の `frequent-opponents` の一覧画面までで、生の一覧は「契約待ち」と画面に注記する(`api/openapi.yaml` は触っていない)。
Reason: P5-3c(ADR-0227)でお気に入り API が入ったため。タイプバランスレーン経由の依頼(2026-10-03)。
Impact: 追加テストは単体 69 件(66 件が失敗する想定)+ XCUITest 13 件。足場は `TODO(implementer P5-3c iOS` で検索。`RequestLimits` に2定数と `check-request-limits.sh` の照合を追加済み。
  既存テスト・identifier は変えない。実装は次のタスク(implementer)。`docs/plan.md` は未更新(依頼元が更新する)。
