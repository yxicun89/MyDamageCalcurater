## 2026-09-24: iOS レーンの統合(PR #170)。issue #99(ライトテーマの danger コントラスト不足)の iOS 側が完了、issue クローズ
Decision: Web レーンの依頼(2026-09-24「issue #99 の Web レーン担当分が完了。iOS レーンへ依頼」)どおり、
`ColorToken.danger` のライト値を `#E5484D` → `#CD1D23` に更新した(Web レーンが PR #164 で確定した値と同じ)。
`web/src/test/colorContrast.ts` と同じ算出式(WCAG相対輝度・コントラスト比)を独立実装した
`ios/PokeCalcKit/Tests/PokeCalcDesignTests/ColorContrast.swift`、ライト・ダーク×bg.base・bg.glass合成の4組を
検査する `DangerContrastTests.swift` を新規追加した。旧値に戻すと3.59:1/3.82:1で実際に red になることを
確認済み。`fix/ios-issue-99-danger-contrast` ブランチで PR #170 として main にマージした(`swift test`
PokeCalcDesignTests 13/13・PokeCalcCoreTests 323/323・`make ios-test` unit 336/336 + UI 16/16・
`check-publishable` 0件を確認済み)。軽微な作業のため spec-writer/critic のサブエージェントは使わずメインで実施。
Reason: Web・iOS 両方の対応が揃ったため issue #99 の受け入れ条件を満たした。
Impact: issue #99 はコメント(Web PR #164・iOS PR #170 の要約、両プラットフォームの新値でのコントラスト比)を
残してクローズした。残る iOS 関連: P6-6(issue #110 の iOS 側追従)、P6-7(issue #103。record-svc/team-svc 実装待ち)。
