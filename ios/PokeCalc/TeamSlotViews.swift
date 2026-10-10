import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamSlotViews: 構築編集画面の 1 つの枠(F-08。ADR-0522)。
//
// 枠の見出し(N体目)と操作([N体目を上へ]・[N体目を下へ]・[N体目を外す])、空の枠(「ポケモン」の欄と案内だけ)、
// 種族の決まった枠(`MemberCardView`)を出す。ロジックは `TeamEditViewModel` が持つ。

struct TeamSlotView: View {
    let viewModel: TeamEditViewModel
    /// 0 始まりの枠の位置。表示・識別子は 1 始まり(`number`)。
    let index: Int

    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    private var number: Int { index + 1 }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            header
            if let member = viewModel.member(atSlot: index) {
                // 並べ替えで枠の位置にいるメンバーが変わっても、入力途中の状態(ニックネーム欄など)が
                // 別のメンバーに移らないよう、メンバー id を同一性にする。
                MemberCardView(viewModel: viewModel, member: member)
                    .id(member.id)
            } else {
                EmptySlotCard(viewModel: viewModel, index: index)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("teamSlot-\(number)")
    }

    private var header: some View {
        let isFilled = viewModel.member(atSlot: index) != nil
        let layout = dynamicTypeSize.isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x2))
            : AnyLayout(HStackLayout(alignment: .center, spacing: SpacingToken.x2))
        return layout {
            Text(TeamLabels.slotTitle(number))
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: dynamicTypeSize.isAccessibilitySize ? .infinity : nil, alignment: .leading)
                .accessibilityAddTraits(.isHeader)
            if !dynamicTypeSize.isAccessibilitySize { Spacer(minLength: 0) }
            if isFilled {
                SlotControlButton(
                    title: "上へ", symbol: "arrow.up", label: TeamLabels.moveUp(number),
                    identifier: "slotMoveUp-\(number)", isDanger: false, isEnabled: viewModel.canMoveSlot(index, by: -1)
                ) { viewModel.moveSlot(index, by: -1) }
                SlotControlButton(
                    title: "下へ", symbol: "arrow.down", label: TeamLabels.moveDown(number),
                    identifier: "slotMoveDown-\(number)", isDanger: false, isEnabled: viewModel.canMoveSlot(index, by: 1)
                ) { viewModel.moveSlot(index, by: 1) }
                SlotControlButton(
                    title: "外す", symbol: PopSymbol.delete, label: TeamLabels.remove(number),
                    identifier: "slotRemove-\(number)", isDanger: true, isEnabled: true
                ) { viewModel.removeSlot(index) }
            }
        }
    }
}

/// 枠の小さな操作ボタン。見える文字は短く(「上へ」)、読み上げ名は枠の番号を含む文(「2体目を上へ」)。
private struct SlotControlButton: View {
    let title: String
    let symbol: String
    let label: String
    let identifier: String
    let isDanger: Bool
    let isEnabled: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: SpacingToken.x1) {
                PopIcon(symbol)
                Text(title)
                    .font(TextStyleToken.body.font)
            }
            .foregroundStyle(isDanger ? ColorToken.danger.color : ColorToken.textPrimary.color)
            .padding(.horizontal, SpacingToken.x3)
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            .background(PopChipStyle.fill(isSelected: false), in: Capsule())
            .opacity(isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .disabled(!isEnabled)
        .accessibilityLabel(label)
        .accessibilityIdentifier(identifier)
    }
}

/// 空の枠: 「ポケモン」の欄と案内だけ。種族を選ぶと枠が埋まる(技・持ち物などはカードに出る)。
private struct EmptySlotCard: View {
    let viewModel: TeamEditViewModel
    let index: Int
    @State private var isSpeciesSearchPresented = false

    private var number: Int { index + 1 }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Button {
                isSpeciesSearchPresented = true
            } label: {
                HStack(spacing: SpacingToken.x2) {
                    Text(TeamLabels.pokemonField)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Image(systemName: "chevron.up.chevron.down")
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .accessibilityHidden(true)
                }
                .padding(SpacingToken.x3)
                .frame(minHeight: CalcScreenMetrics.minimumTapSide)
                .popInset()
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(TeamLabels.slotTitle(number))の\(TeamLabels.pokemonField)")
            .accessibilityIdentifier("slotSpeciesPicker-\(number)")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel) { option in
                    Task { await viewModel.selectSpecies(slot: index, speciesKey: option.key) }
                }
            }
            Text(TeamLabels.emptySlotHint)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("slotEmptyHint-\(number)")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard()
    }
}
