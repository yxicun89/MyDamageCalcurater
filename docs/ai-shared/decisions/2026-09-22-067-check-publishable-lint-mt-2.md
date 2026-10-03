## 2026-09-22: check-publishable の自己テストを lint に含める(整備レーン MT-2 の既定案)
Decision: `scripts/check-publishable.sh --self-test` の既知の失敗2件を修正し、`make lint` から `make check-publishable-selftest` を実行する。A は任意のホーム相対パスを検出し、利用者名を含まない共有の worktree と権限定義のプレースホルダだけを許可する。E は自己テストへ実際に禁止される GitHub module path を投入する。
Reason: 検査規則そのものの退行を通常の lint で検出し、自己テストが安全な値を投入して偽陰性になっていた状態を解消するため。plan.md の既定案どおり進めた。
Impact: `make lint` の所要時間が自己テスト分だけ約6秒増える。A〜F の違反検出・値の非表示・正常系を毎回確認する。
