# 開発計画と進行状況

凡例: `[ ]` 未着手 / `[~]` 作業中 / `[x]` 完了 / `[!]` ブロック中
作業する AI は、タスク開始時に `[~]`、完了時に `[x]` へ更新してからコミットする。
完了したタスクの経過(critic の往復・テスト件数・実装の詳細)と解決済みのブロッカーは [plan-archive.md](plan-archive.md) に移す。plan.md と区画のファイル(`docs/plan/`)には、各タスクの1行の状態と未完了の受け入れ条件だけを置く。issue にした軽微指摘は issue を正とし、ここには書かない(二重管理しない)。

## 区画(docs/plan/)

タスクの行は区画ごとのファイルにある(ADR-0170。レーンの PR が同じファイルで衝突しないため)。自分のレーンの区画のファイルだけを編集する。
見出しは移す前の plan.md のまま。「docs/plan.md「SP: 素早さ比較」」のような既存の参照は、この表から区画のファイルを引く。

| 区画(見出し) | ファイル |
|---|---|
| M1: ブラウザで計算できる | [docs/plan/m1.md](plan/m1.md) |
| M2: 保存・構築 | [docs/plan/m2.md](plan/m2.md) |
| M3: iOS | [docs/plan/m3.md](plan/m3.md) |
| TB: タイプバランスチェッカー(タイプバランスレーン。設計は docs/type-balance-design.md) | [docs/plan/tb.md](plan/tb.md) |
| SP: 素早さ比較(素早さレーン。設計は docs/speed-design.md。2026-09-22 ユーザー要望) | [docs/plan/speed.md](plan/speed.md) |
| JD: 判定(判定レーン。設計は docs/judge-design.md。2026-09-22 ユーザー要望) | [docs/plan/judge.md](plan/judge.md) |
| AJ: 調整(ダメージ計算レーン。設計は ADR-0150。2026-10-01 ユーザー要望) | [docs/plan/adjust.md](plan/adjust.md) |
| DOC: 文書(全レーン。docs/coding-rules.md §8。2026-09-22 ユーザー要望) | [docs/plan/doc.md](plan/doc.md) |
| M4: 運用 | [docs/plan/m4.md](plan/m4.md) |
| 後続: 要件との対応(issue #286。M1〜M4 の後。担当レーン付き) | [docs/plan/followups.md](plan/followups.md) |
| ブロッカー | [docs/plan/blockers.md](plan/blockers.md) |
| 改善要望(/improve で追加) | [docs/plan/improvements.md](plan/improvements.md) |

## マイルストーン

| ID | ゴール | 人間の確認方法 |
|---|---|---|
| **M1** | **ブラウザで計算できる**(Mac上のk3d) | `make up` → http://localhost:8080 で計算・一括表示・逆算を触る |
| M2 | 計算の自動保存・よく使う・構築 | 計算後に履歴と「よく計算する相手」が出る |
| M3 | iPhone で使える | Xcode から実機に入れて Tailscale 経由で計算 |
| M4 | 運用(監視・SLO・GitOps) | Grafana でSLOダッシュボードを見る |
| +α | クラウド移行 / ダブル / 推薦ML | |

**キックオフでは M1 完了まで自動で進める。** M2 以降は人間が `/phase M2` などで開始する。
