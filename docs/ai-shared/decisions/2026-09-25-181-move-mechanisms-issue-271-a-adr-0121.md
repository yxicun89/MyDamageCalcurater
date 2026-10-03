## 2026-09-25: 技の機構(多段・固定ダメージ・威力変動 等)を move_mechanisms 表としてマスタに持つ(データレーン。issue #271-a・ADR-0121)
Decision: Showdown の技データ(multihit・damage・ohko・willCrit・override*・ignoreDefensive・ハンドラ名)と、天候・フィールドのハンドラが技を名指ししている箇所から、importer が攻撃技の機構を 13 種に機械的に分類し、`move_mechanisms(move_id, mechanism)` に入れる。技名は持たない。分類表に無いハンドラは安全側(move_specific / field_specific)+警告。取得物の `mechanism` は必須(古い取得物は拒否)。
Reason: 多段・威力変動・固定ダメージ等の技が黙って誤ったダメージになる(#271・#233)。engine の「未対応の印」(D16)の前提になるデータが無かった。
Impact: 実データで攻撃技 335 のうち 93 が機構を持つ。マージ後に Showdown の取得をやり直す必要がある(同じ commit・キャッシュ使用)。API レーンへ: `MasterMove.mechanisms: string[]` の追加を依頼(`api/openapi.yaml`)。engine・calc-svc への受け渡しは D16。
