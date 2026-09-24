// issue #99(アクセシビリティ): WCAG 2.2 の相対輝度・コントラスト比の計算(SC 1.4.3)。
// `web/src/test/colorContrast.ts` と同じ算出式(独立した実装。coding-rules §2)。
// design.md の RGBA から色を読み、通常文字の基準 4.5:1 を検査するのに使う(テストのみで使う補助。製品コードには含めない)。

import Foundation
@testable import PokeCalcDesign

/// 半透明の前景色を不透明な背景色の上に重ねた、見た目上の不透明色(アルファブレンド)。
func compositeOver(_ foreground: RGBA, over background: RGBA) -> RGBA {
    func blend(_ fgChannel: Int, _ bgChannel: Int) -> Int {
        Int((Double(fgChannel) * foreground.alpha + Double(bgChannel) * (1 - foreground.alpha)).rounded())
    }
    return RGBA(
        red: blend(foreground.red, background.red),
        green: blend(foreground.green, background.green),
        blue: blend(foreground.blue, background.blue),
        alpha: 1.0
    )
}

/// sRGB の1チャンネル(0〜255)を WCAG の相対輝度計算用の線形値にする。
private func linearizeChannel(_ channel255: Int) -> Double {
    let channel = Double(channel255) / 255
    return channel <= 0.03928 ? channel / 12.92 : pow((channel + 0.055) / 1.055, 2.4)
}

/// WCAG 2.2 の相対輝度(0〜1)。半透明色は先に `compositeOver` で不透明にしてから渡すこと
/// (半透明のまま渡すと、実際より明るい/暗い誤った値になりうるため、ここで弾く)。
func relativeLuminance(_ color: RGBA) -> Double {
    precondition(color.alpha == 1.0, "半透明の色はそのまま輝度計算できない(compositeOver で不透明にしてから渡す): \(color)")
    return 0.2126 * linearizeChannel(color.red)
        + 0.7152 * linearizeChannel(color.green)
        + 0.0722 * linearizeChannel(color.blue)
}

/// WCAG 2.2 のコントラスト比(1〜21)。順序に依存しない(常に明るい方 ÷ 暗い方)。
func contrastRatio(_ colorA: RGBA, _ colorB: RGBA) -> Double {
    let luminanceA = relativeLuminance(colorA)
    let luminanceB = relativeLuminance(colorB)
    let lighter = max(luminanceA, luminanceB)
    let darker = min(luminanceA, luminanceB)
    return (lighter + 0.05) / (darker + 0.05)
}

/// WCAG 2.2 SC 1.4.3(通常文字)の最低基準。
let wcagNormalTextMinContrast = 4.5
