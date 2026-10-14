import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ComboSectionParts: 連続技の節で使う小さな部品(CB-8。ADR-0535)。ロジックは持たない。
// 色はトークンだけ。タップ領域は 36pt 以上。文字は折り返す(行数の上限を付けない)。

enum ComboMetrics {
    /// タップ領域の高さ(受け入れ条件は 36pt 以上。丸めの余裕を含めて 44)。
    static let minTapHeight: CGFloat = 44
    static let barHeight: CGFloat = 10
    /// % が小さいと帯が消えて見えなくなるため、常に見える最小幅。
    static let minimumVisibleBarWidth: CGFloat = 6
}

/// 小見出し + 内容(見出しは内容の上に積む。横幅に依らず折り返し先が決まる)。
struct ComboField<Content: View>: View {
    let label: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(label)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
            content
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// 選ぶ欄の見た目(メニュー・シートの入口)。
struct ComboPickerLabel: View {
    let text: String
    let systemImage: String

    var body: some View {
        HStack(spacing: SpacingToken.x2) {
            Text(text)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: SpacingToken.x2)
            PopIcon(systemImage)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
        .padding(.horizontal, SpacingToken.x3)
        .padding(.vertical, SpacingToken.x2)
        .frame(maxWidth: .infinity, minHeight: ComboMetrics.minTapHeight, alignment: .leading)
        .background(ColorToken.surfaceCard.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        .contentShape(Rectangle())
    }
}

/// オン・オフの欄(チェック + 文字)。選択中は塗りと `isSelected`。長い文言は折り返す。
struct ComboToggleButton: View {
    let title: String
    let isOn: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: SpacingToken.x2) {
                // 選択中だけチェックを出す(オフは丸を出さない。状態は選択の特性と塗りで伝わる)。
                if isOn { PopIcon(PopSymbol.done) }
                Text(title)
                    .font(TextStyleToken.body.font)
                    .multilineTextAlignment(.leading)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
            }
            .foregroundStyle(PopChipStyle.foreground(isSelected: isOn))
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x2)
            .frame(maxWidth: .infinity, minHeight: ComboMetrics.minTapHeight, alignment: .leading)
            .background(
                RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous).fill(PopChipStyle.fill(isSelected: isOn))
            )
            .overlay(
                RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                    .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(title)
        .accessibilityAddTraits(isOn ? .isSelected : [])
        .accessibilityIdentifier(identifier)
    }
}

/// ランク変化のチップ(「攻撃 +1」「相手の素早さ -1」)。上昇は brandPrimary・下降は warning(ランクのメーターと同じ意味)。
/// 色だけに頼らず、文字の符号と矢印でも向きを出す。
struct ComboRankChipsView: View {
    let chips: [ComboRankChip]

    var body: some View {
        if !chips.isEmpty {
            ComboWrapLayout(spacing: SpacingToken.x2) {
                ForEach(Array(chips.enumerated()), id: \.offset) { _, chip in
                    let color = chip.direction == .up ? ColorToken.brandPrimary.color : ColorToken.warning.color
                    HStack(spacing: SpacingToken.x1) {
                        PopIcon(chip.direction == .up ? PopSymbol.up : PopSymbol.down)
                        Text(chip.text)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(color)
                    .padding(.horizontal, SpacingToken.x2)
                    .padding(.vertical, SpacingToken.x1)
                    .background(ColorToken.surfaceCard.color, in: Capsule())
                    .overlay(Capsule().stroke(color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
                }
            }
        }
    }
}

/// 子を左から並べ、幅に収まらなければ次の行へ折り返す配置(チップ用)。
struct ComboWrapLayout: Layout {
    var spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = arrange(width: proposal.width ?? .infinity, subviews: subviews)
        return CGSize(width: proposal.width ?? rows.maxWidth, height: rows.height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let rows = arrange(width: bounds.width, subviews: subviews)
        for (index, frame) in rows.frames.enumerated() {
            subviews[index].place(
                at: CGPoint(x: bounds.minX + frame.minX, y: bounds.minY + frame.minY), proposal: ProposedViewSize(frame.size))
        }
    }

    private func arrange(width: CGFloat, subviews: Subviews) -> (frames: [CGRect], height: CGFloat, maxWidth: CGFloat) {
        var frames: [CGRect] = []
        var x: CGFloat = 0
        var y: CGFloat = 0
        var rowHeight: CGFloat = 0
        var maxWidth: CGFloat = 0
        for subview in subviews {
            var size = subview.sizeThatFits(ProposedViewSize(width: width, height: nil))
            size.width = min(size.width, width)
            if x > 0, x + size.width > width {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            frames.append(CGRect(origin: CGPoint(x: x, y: y), size: size))
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
            maxWidth = max(maxWidth, x - spacing)
        }
        return (frames, y + rowHeight, maxWidth)
    }
}

/// 技を1つ選ぶシート(追加の技・相手の技)。タイプ順の群で出す。`noneRowTitle` があれば先頭に「選ばない」の行を出す。
struct ComboMovePickerSheet: View {
    let title: String
    let moves: [Move]
    /// 行の識別子の頭(`<idPrefix><技 ID>`)。
    let idPrefix: String
    let noneRowTitle: String?
    /// 選んだ技の ID(「選ばない」は nil)。
    let onSelect: (String?) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            // `List` は画面の外の行を作らない(文字が大きいと下の行が見つからない)ので、全行を作る縦並びにする。
            ScrollView {
                VStack(alignment: .leading, spacing: SpacingToken.x2) {
                    if let noneRowTitle {
                        Button {
                            onSelect(nil)
                            dismiss()
                        } label: {
                            Text(noneRowTitle)
                                .font(TextStyleToken.body.font)
                                .foregroundStyle(ColorToken.textPrimary.color)
                                .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide, alignment: .leading)
                                .contentShape(Rectangle())
                        }
                        .buttonStyle(.plain)
                        .accessibilityIdentifier("comboOpponentMoveNone")
                        Divider()
                    }
                    ForEach(MoveSort.typeGroups(moves), id: \.type) { group in
                        Text(PokeTypeLabel.japaneseName(for: group.type))
                            .font(TextStyleToken.caption.font)
                            .foregroundStyle(ColorToken.textSecondary.color)
                            .accessibilityAddTraits(.isHeader)
                        ForEach(group.moves, id: \.id) { move in
                            Button {
                                onSelect(move.id)
                                dismiss()
                            } label: {
                                VStack(alignment: .leading, spacing: SpacingToken.x1) {
                                    MasterSearchRow.move(move)
                                    ComboRankChipsView(chips: ComboRankChips.chips(for: move))
                                }
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .contentShape(Rectangle())
                            }
                            .buttonStyle(.plain)
                            .accessibilityIdentifier("\(idPrefix)\(move.id)")
                            .accessibilityLabel(move.nameJa)
                            Divider()
                        }
                    }
                }
                .padding(SpacingToken.x4)
            }
            .navigationTitle(title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(ComboText.closeButton) { dismiss() }
                        .accessibilityIdentifier("comboPickerCloseButton")
                }
            }
        }
    }
}
