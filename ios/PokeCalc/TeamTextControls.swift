import PokeCalcDesign
import SwiftUI

// TeamTextControls: 構築画面のボタン文言の部品。
//
// Showdown 形式の書き出し・取り込みは G-03(ADR-0528)で廃止した。ここには構築の一覧・編集で共有するボタンの
// 見た目だけを残す。

enum TeamTextControls {
    /// `systemImage` を渡すと文字の前に装飾アイコンを付ける(既定はなし)。
    @ViewBuilder
    static func buttonLabel(_ title: String, systemImage: String? = nil) -> some View {
        HStack(spacing: SpacingToken.x2) {
            if let systemImage { PopIcon(systemImage) }
            Text(title)
                .font(TextStyleToken.body.font)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity)
    }
}
