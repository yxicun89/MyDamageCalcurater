import PokeCalcCore
import PokeCalcDesign
import SwiftUI

/// タブの上部の区切り(ダメージの 計算/逆算/調整。ADR-0531)。常に見えるボタンの並びで、メニューやシートに隠さない。
/// 押すと画面を切り替えるだけ(切り替えの演出は持たない)。横 1 列に収まるときは横に並べ、大きい文字で収まらない
/// ときだけ縦に積む(文字を省略・縮小しない)。
struct TabSegmentsView: View {
    let tab: AppTab
    let segments: [TabSegment]
    let selectedID: String?
    let onSelect: (String) -> Void

    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: SpacingToken.x2) { buttons(keepsSingleLine: true) }
            VStack(spacing: SpacingToken.x2) { buttons(keepsSingleLine: false) }
        }
        .padding(.horizontal, SpacingToken.x4)
        .padding(.vertical, SpacingToken.x2)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AppNavigationText.segmentsLabel)
        .accessibilityIdentifier(tab.segmentsAccessibilityIdentifier)
    }

    /// `keepsSingleLine` は横並びの候補で、文字を折り返さない幅を求めるため(収まらなければ縦並びの候補になる)。
    private func buttons(keepsSingleLine: Bool) -> some View {
        ForEach(segments) { segment in
            let isSelected = segment.id == selectedID
            Button {
                onSelect(segment.id)
            } label: {
                Text(segment.title)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(PopChipStyle.foreground(isSelected: isSelected))
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: keepsSingleLine, vertical: false)
                    .padding(.horizontal, SpacingToken.x2)
                    .padding(.vertical, SpacingToken.x1)
                    .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide)
                    .background(
                        RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                            .fill(PopChipStyle.fill(isSelected: isSelected))
                    )
                    .overlay(
                        RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                            .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth)
                    )
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier(segment.accessibilityIdentifier)
            .accessibilityAddTraits(isSelected ? .isSelected : [])
        }
    }
}

/// 区切りの 1 つ(`FeatureEntry.tab` の title・識別子)。
struct TabSegment: Identifiable {
    let id: String
    let title: String
    let accessibilityIdentifier: String
}
