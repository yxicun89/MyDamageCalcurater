## 2026-09-24: issue #99(ライトテーマの danger コントラスト不足)の Web レーン担当分が完了。iOS レーンへ依頼
Decision: danger のライト値を `#E5484D` → `#CD1D23` に変更した(色相・彩度は変えず明度だけ下げる。WCAG 2.2
SC 1.4.3 の通常文字基準4.5:1を、bg.base単体(5.07:1)・bg.glassをbg.baseに重ねた合成色(5.40:1)の両方で満たす。
ダーク値 `#FF6369` は元から基準を満たしており〈bg.base 6.56:1・glass合成 6.13:1〉変更していない)。
`docs/design.md`「デザイントークン」に理由・数値を記録。`web/src/test/colorContrast.ts`(WCAG相対輝度・
コントラスト比の計算。既知の参照値で検算済み)・`web/src/styles/contrast.test.ts`(design.md から値を読み、
ライト・ダーク×bg.base・bg.glass合成の4組を検査)を新規追加。critic PASS(独立実装での検算・変異テストで
実効性を確認済み)。
Reason: issue #99(Codexレビュー。タイプバランスレーンから2026-09-23連絡)。ライトテーマの danger 文字色が
WCAG基準を満たさず、弱視・低コントラスト環境の利用者がエラー文言を読み取りにくい状態だった。
Impact: **iOSレーンへ依頼**: `ios/PokeCalcKit/Sources/PokeCalcDesign/PokeCalcDesign.swift`(`ColorToken.danger`
のライト値。現在 `RGBA(red: 0xE5, green: 0x48, blue: 0x4D, alpha: 1.0)`)と
`ios/PokeCalcKit/Tests/PokeCalcDesignTests/DesignTokenTests.swift`(同じ旧値を手書きで期待値にしている36行目
付近)を `0xCD, 0x1D, 0x23` に更新し、Web と同様にコントラスト比を検査するテストを追加してほしい(値は
design.md「デザイントークン」が正)。issue #99 は iOS 側が完了するまでクローズしない。
