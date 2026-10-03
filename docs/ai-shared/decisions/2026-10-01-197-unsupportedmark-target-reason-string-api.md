## 2026-10-01: UnsupportedMark の target・reason を string にした(API レーン → Web・iOS レーンへ)

Decision: ADR-0215。openapi の `UnsupportedMark.target`・`reason` から enum を外した(既知の値は description)。サーバーの応答値は不変。
Reason: 新しい reason を足すと古い iOS アプリが応答全体をデコードできなくなるため(iOS レーン提案の対応)。
Impact: Web は `UnsupportedMark.target/reason: string`、未知の値は「項目」「詳細は不明」で表示。iOS は `UnsupportedTarget/Reason` に `unknown` を足し、契約同期テストは既知の値の集合をテストに持つ。両レーンの追従は同じ PR で済み(Web 1747・iOS 全件テスト緑)。
