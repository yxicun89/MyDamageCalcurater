import PokeCalcDesign
import SwiftUI

// SpeedScreenControls: 素早さ比較画面(P6-24。ADR-0503 §7)の小さな部品。
// 入力は選択ボタン(ピル。`isSelected` で状態を出す)。メニュー形式の選択部品は使わない。
// 文言が長くても行数を制限せず、ピルの中で折り返す(AX5 でも横にはみ出さない)。

/// 選択状態を持つピル。長い文言は折り返す(`ChipButton` は1行に固定するので別に持つ)。
struct SpeedPill: View {
    let title: String
    let isSelected: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(TextStyleToken.body.font)
                .multilineTextAlignment(.leading)
                .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(Capsule().fill(isSelected ? ColorToken.textPrimary.color : ColorToken.bgGlass.color))
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

/// 子を左から詰めて並べ、収まらなければ次の行へ送る(横スクロールにしない)。1つが幅より広ければその子の中で折り返す。
struct SpeedFlowLayout: Layout {
    var spacing: CGFloat = SpacingToken.x2

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        arrange(maxWidth: proposal.width ?? .infinity, subviews: subviews).size
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let result = arrange(maxWidth: bounds.width, subviews: subviews)
        for (index, frame) in result.frames.enumerated() {
            subviews[index].place(
                at: CGPoint(x: bounds.minX + frame.minX, y: bounds.minY + frame.minY),
                proposal: ProposedViewSize(frame.size))
        }
    }

    private func arrange(maxWidth: CGFloat, subviews: Subviews) -> (frames: [CGRect], size: CGSize) {
        var frames: [CGRect] = []
        var x: CGFloat = 0
        var y: CGFloat = 0
        var rowHeight: CGFloat = 0
        var usedWidth: CGFloat = 0
        for subview in subviews {
            let size = subview.sizeThatFits(ProposedViewSize(width: maxWidth, height: nil))
            if x > 0, x + size.width > maxWidth {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            frames.append(CGRect(origin: CGPoint(x: x, y: y), size: size))
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
            usedWidth = max(usedWidth, x - spacing)
        }
        return (frames, CGSize(width: usedWidth, height: y + rowHeight))
    }
}

/// 見出し(小さい文字)+ ピルの並び。
struct SpeedPillGroup<Content: View>: View {
    let title: String
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: title)
            SpeedFlowLayout { content() }
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

struct SpeedCaption: View {
    let text: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// −/値/+ の組(`Stepper` ではなく2つのボタン。ロケールに依存せず XCUITest で押せる)。
struct SpeedStepper: View {
    let title: String
    let valueText: String
    let decrementLabel: String
    let incrementLabel: String
    let canDecrement: Bool
    let canIncrement: Bool
    let identifierPrefix: String
    let onDecrement: () -> Void
    let onIncrement: () -> Void

    private static let valueMinWidth: CGFloat = 48

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: title)
            HStack(spacing: SpacingToken.x2) {
                stepButton("minus", label: decrementLabel, enabled: canDecrement, id: "\(identifierPrefix)Decrement", action: onDecrement)
                Text(valueText)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .frame(minWidth: Self.valueMinWidth)
                    .accessibilityIdentifier("\(identifierPrefix)Value")
                stepButton("plus", label: incrementLabel, enabled: canIncrement, id: "\(identifierPrefix)Increment", action: onIncrement)
            }
        }
    }

    private func stepButton(_ systemName: String, label: String, enabled: Bool, id: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: systemName)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .padding(SpacingToken.x2)
                .background(ColorToken.bgGlass.color, in: Circle())
        }
        .buttonStyle(.plain)
        .disabled(!enabled)
        .opacity(enabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
        .accessibilityLabel(label)
        .accessibilityIdentifier(id)
    }
}

/// 名前の頭文字を載せたタイプ色の円(画像は使わない)。色は文字列のタイプ ID から引き、未知の ID は無彩色。
struct SpeedEmblem: View {
    let name: String
    let primaryType: String
    private static let diameter: CGFloat = 32

    var body: some View {
        Circle()
            .fill(TypeColorToken.color(forTypeID: primaryType) ?? ColorToken.textSecondary.color)
            .frame(width: Self.diameter, height: Self.diameter)
            .overlay(
                Text(name.prefix(1))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(.white)
            )
            .accessibilityHidden(true)
    }
}
