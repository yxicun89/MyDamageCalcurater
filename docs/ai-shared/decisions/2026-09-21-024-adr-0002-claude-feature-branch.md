## 2026-09-21: 共通マスタ候補 ADR-0002 は Claude feature branch 上の提案として参照する
Decision: ADR-0012 と CURRENT_STATE が参照する ADR-0002 は `feat/claude-p1-engine` 上にあり、main へは未統合であることを明記する。
Reason: main の共有状態を正本にした時点で、ブランチ指定のない ADR-0002 参照が main 上では辿れなかったため。
Impact: 共通マスタ方式は確定扱いにしない。Claude ブランチが通常手順で main に統合された後は main の ADR-0002 を参照する。
