## 2026-10-04: iOS の「防御側のランク」の受け入れ条件とテストを先に置いた(iOS レーン。issue #274 の残り、ADR-0315・ADR-0216)
Decision: 防御側のランクは種族変更・攻守入れ替え・技の変更で消さない(Web の ADR-0315 §6 と攻撃側ランクの既存規則に揃える)。防御側の特性(P6-19)だけが種族変更・入れ替えで「指定なし」に戻る。
`defenderOverride` は「abilityId も ranks も無いなら送らない」。ranks は非 0 のとき 5 項目(0 も含む)、`status` は送らない。
Reason: Web と文言・規則を揃え、既存の要求本文(既定時)を変えないため。新しい ADR は起こさず ADR-0501 末尾「防御側のランクの受け入れ条件」に判断を書いた。
Impact: 追加は単体 24 件(失敗 16・成功 8)と XCUITest 5 件。足場は `TODO(implementer` を検索。api/openapi.yaml・services/・既存テストは未変更。
