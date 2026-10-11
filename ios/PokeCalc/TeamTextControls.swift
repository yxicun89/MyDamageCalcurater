import PokeCalcDesign
import SwiftUI

// TeamTextControls: 構築画面のボタン文言の部品。
//
// Showdown 形式の書き出し・取り込みは G-03(ADR-0528)で廃止した。ここには構築の一覧・編集で共有するボタンの
// 見た目だけを残す。

enum TeamTextControls {
    static func buttonLabel(_ title: String) -> some View {
        Text(title)
            .font(TextStyleToken.body.font)
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity)
    }
}
