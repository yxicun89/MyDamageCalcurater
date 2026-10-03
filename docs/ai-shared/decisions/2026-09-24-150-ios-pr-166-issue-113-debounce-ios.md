## 2026-09-24: iOS レーンの統合(PR #166)。issue #113(入力変更時の古い計算要求のキャンセル・debounce)の iOS 側を解消
Decision: `ReverseViewModel`/`CalcViewModel` が入力操作ごとの計算 Task を最新の1つだけ保持する
(`LatestTaskRunner`)実装にした。新しい入力・画面破棄(`.onDisappear` → `cancelPendingWork()`)で先行 Task を
cancel し、送信済みの `reverse`/`calcBulk` まで実際にキャンセルが伝播することをテストで確認済み。逆算の観測
文字入力には200msの trailing debounce(`CalcInput.debounceInterval`)を適用した(同期的な TextField 表示・
入力検証は即時のまま)。`CancellationError` は画面エラーに変換せず、`result`/`rows` も消さない(各 ViewModel の
private `handleInputFailure(_:)` に全 catch を統一)。`fix/ios-issue-113-debounce-cancel` ブランチで PR #166 として
main にマージした(critic 1周目 FAIL→2周目 PASS。`swift test` 323/323・`make ios-test` unit 332/332 + UI 16/16・
`check-publishable` 0件を確認済み)。同 PR で main の `getMove` 追加分に追従し iOS 生成物も再生成済み。
Reason: 1周目の critic 指摘: `reverse`/`calcBulk` を包む catch だけキャンセル対応していたが、
`species(key:)`(種族変更・攻守入れ替え・構築からの呼び出し等、計6箇所)を包む catch が素通りで
`CancellationError` を画面エラーに変換していた退行があった。`handleInputFailure` への集約で解消。
Impact: issue #113 は iOS 側が完了。Web・API 側の対応(`AbortSignal`・`CalcEngine` の cancel signal 境界)は
別レーンの担当のまま。issue #68 の `MasterSearchField`(素朴なデバウンス+世代トークン)は意味論が違う
(検索は250ms・空クエリ即時・失敗を error にしない)ため統合していない(理由は ADR-0501「issue #113」7章)。
残る iOS 関連: P6-6(issue #110 の iOS 側追従。観測16件上限・持ち物候補64件超の扱い。次に着手)、
P6-7(issue #103・ADR-0209 §8 の削除 UI。record-svc/team-svc 実装待ち)、issue #99 の iOS 側
(`ColorToken.danger` のライト値更新。Web レーンから依頼済み・上のエントリ参照)。
