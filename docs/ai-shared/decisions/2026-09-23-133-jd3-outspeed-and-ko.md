## 2026-09-23: 判定 JD3 で outspeed-and-ko の契約を破壊的に変更する(判定レーン)
Decision: `POST /api/judge/v1/outspeed-and-ko` の request の `defender`(単数)を `defenders`(Individual[]、1〜6件)に、
response の単数の5欄(outspeeds/speedTie/attackerSpeed/defenderSpeed/ko)を `matchups`(defenders と同じ順序・同じ件数の
Matchup 配列。各行が defenderIndex を持つ)に置き換えた。版は上げない(v1 のまま)。設計は ADR-0703。
Reason: JD5(Web/iOS の画面)が未着手で judge-svc を呼ぶクライアントが1つも無く、gateway もルートの api/openapi.yaml も
judge を含まない(ADR-0700 §6-3・ADR-0701 §7)ため、壊れるものが無い。互換のために単数の defender を残すと同じ問いに
入口が2つでき、response も単数・配列の2形態になる。judge-design.md §3 が JD5 を最後に置いたのは、まさにこの変更を
クライアントが付く前に済ませるため。
Impact:
- 他レーンへの影響は無い(ルートの api/openapi.yaml・gateway・Web・iOS のいずれも judge の型を生成していない)。
- JD5 に着手する時点の契約は `defenders` / `matchups` の形になる。JD5 を Web/iOS レーンに依頼する場合は
  services/judge/api/openapi.yaml を参照先として渡す。
- 候補のどれかで失敗したら request 全体を打ち切り、部分成功は返さない。エラーの message は
  どの候補かを `defenders[<index>]` の形で示す(上流の URL・本文は含めないので ADR-0700 §3 は保たれる)。
