# ADR-0215: UnsupportedMark の target・reason を enum にしない(前方互換)

- 状態: 採用
- 日付: 2026-10-01
- 関連: ADR-0123(未対応の印)、ADR-0214(契約の追加の前例)、ADR-0501「P6-17」(iOS の写像)、
  DECISIONS.md 2026-09-25「未対応の印の表示」(iOS レーンの提案)

## 背景

`UnsupportedMark.target`(5種)・`reason`(15種)は OpenAPI の enum だった。生成された Swift の
`@frozen` な String enum は未知の値を受け取れないため、サーバーが新しい reason を足すと、古い iOS アプリは
**計算・逆算の応答全体**をデコードできなくなる(印は補助情報なのに、数値まで失う)。

## 決定

- `target`・`reason` を `type: string` にし、enum を外す。既知の値は description に書く(契約の正は
  ADR-0123 §2 の表と engine の定数)。値を足しても破壊的変更にならない。
- クライアントは未知の値を「項目」「詳細は不明」の汎用の語で表示し、ID は必ず出す(印を捨てない・
  応答全体を失敗にしない)。Web は `string` 型で受けて表の引き当てを `Object.hasOwn` で守る。
  iOS はドメイン enum に `unknown` を足し(`allCases` は既知のみ)、契約の文字列から `init(contractValue:)` で写す。
- サーバー側(calc-svc・judge-svc)は engine の定数を `string` に変換して返す(値は不変)。

## 影響

- 応答の JSON は不変(既存の値はそのまま)。生成物は Go・TS・Swift を再生成。
- iOS の契約同期テストは生成型の `allCases` を使えなくなったため、既知の値の集合をテストに写した
  (値を足すときはテストとラベル表も足す)。
- 表示専用の語は Web・iOS で揃えた(項目 / 詳細は不明)。
