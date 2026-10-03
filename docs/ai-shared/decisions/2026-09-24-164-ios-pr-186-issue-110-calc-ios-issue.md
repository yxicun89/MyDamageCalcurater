## 2026-09-24: iOS レーンの統合(PR #186)。issue #110(calc の候補・観測件数の上限)の iOS 側が完了、issue クローズ
Decision: ADR-0501「issue #110」章のとおり `RequestLimits`/`RequestLimitLabels` を新設し、観測16行で追加ボタンを
無効化+理由表示、逆算の持ち物候補・計算画面の比較トグル(`itemVariants`)は上限到達中の ON を拒否(null 込み64件。
OFF は常時可)。Web の「送信直前の決定的な切り捨て」とは画面の振る舞いをあえて変えた(選択表示と要求を常に一致させる。
ADR 5章)。iOS の定数は契約の写しなので、`ios/scripts/check-request-limits.sh`(`make ios-test` の一部)で
`api/openapi.yaml` の `maxItems` と突き合わせる(XCTest はシミュレータのサンドボックスでリポジトリを読めないため)。
Reason: 前任が実装・critic PASS まで進めて未コミットだった作業を引き継ぎ、自分で検証した。その際、ADR が当初
「契約とのずれは Web の `requestLimits.test.ts` が検出する」としていたが、あれは Web の定数しか見ないため
iOS の写しのずれを検出できないと分かり、上記の検査を追加した(追加分も critic PASS)。`make ios-test`
unit 353/353・XCUITest 17/17、`make test`・`make lint`・`make check-publishable` 成功。
Impact: API(PR #130)・データ(PR #138)・Web(PR #142)・iOS(PR #186)が揃ったので issue #110 をクローズした。
残る iOS 関連: P6-7(issue #103・ADR-0209 §8 の削除 UI。record-svc/team-svc の API が契約に入るまで着手できない)。
