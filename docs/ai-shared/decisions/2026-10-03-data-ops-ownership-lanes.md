## 2026-10-03: M2・M4・運用の持ち主のレーン(issue #285。データレーン → 全レーン)
Decision: ユーザー決定 2026-10-03「既定案で OK」。新しいレーンは作らない。
- **M2(record/team・保存・構築・履歴)は API レーン**(既に P5-1 を進めている)。Web 側の画面は Web レーン。
- **M4(監視・SLO・GitOps・バックアップ)と、Tailscale 等の到達経路・運用(deploy・scripts・runbook・up.sh・k8s)はデータレーン**。
Reason: 持ち主のいない「運用レーン」の選定待ちで P4-20 などが止まっていた。レーン数(7本)を増やさず、既存のレーンに割り当てる。
Impact: COORDINATION.md のレーン表と `docs/plan.md` の P4-20・P5-1・P7-3・P7-4 に担当を書いた。CLAUDE.md・COORDINATION.md のレーン数の記述を一致させた。
issue #285 は閉じてよい。
