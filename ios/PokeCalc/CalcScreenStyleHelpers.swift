import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcScreenStyleHelpers: 計算画面だけで使う見た目のヘルパー(P6-2a)。
//
// タイプ・技の分類・相性の日本語ラベルは `PokeCalcCore`(`DisplayLabels.swift`)にある
// (`CalcViewModel.moveSummaryText` が Core 側で組み立てるため。View はそれをそのまま描く)。

/// 計算画面のあちこちで使う見た目の定数を1か所にまとめる(批評「任意」対応: 0.7 の縮小率・
/// 1pt のヘアライン枠線が複数ファイルに同じ値でばらばらに書かれていた。coding-rules §2
/// 「同じ定義を複数箇所に書かない」)。
enum CalcScreenMetrics {
    /// 1行に収めるためのテキストの最小縮小率。design.md には数値指定が無いため実装側で決める。
    static let compactMinimumScaleFactor: CGFloat = 0.7
    /// カード・チップ・ダメージバーのトラックに使うヘアライン枠線の太さ。
    static let hairlineBorderWidth: CGFloat = 1
}

/// design.md「Liquid Glass 系のクリーン」の角丸カード背景。requirements のビジュアル B・ADR-0500 §1
/// に合わせ、iOS 26+ の本物の Liquid Glass(`.glassEffect(_:in:)`)を使う。`ColorToken.bgGlass` を
/// `Glass.tint(_:)` で載せ、design.md の bg.glass のトーンに近づける(不透明度はシステムの
/// ガラス素材が持つため、そこは design.md の「+ ぼかし」の意図どおりシステムに委ねる)。
extension View {
    func glassCard(cornerRadius: CGFloat = RadiusToken.card) -> some View {
        modifier(GlassCardModifier(cornerRadius: cornerRadius))
    }
}

private struct GlassCardModifier: ViewModifier {
    let cornerRadius: CGFloat

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
        content
            .glassEffect(.regular.tint(ColorToken.bgGlass.color), in: shape)
            .overlay(shape.stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
    }
}
