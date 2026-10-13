import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamSlotViews: 構築編集画面の 1 つの枠(F-08。ADR-0522)。
//
// 枠の見出し(N体目)と操作(アイコンだけの上へ・下へ・外す。読み上げ名は「N体目を上へ」など)、空の枠(「ポケモン」の欄だけ)、
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
                PopIconButton(
                    systemImage: PopSymbol.up, label: TeamLabels.moveUp(number), identifier: "slotMoveUp-\(number)",
                    isEnabled: viewModel.canMoveSlot(index, by: -1)
                ) { viewModel.moveSlot(index, by: -1) }
                PopIconButton(
                    systemImage: PopSymbol.down, label: TeamLabels.moveDown(number), identifier: "slotMoveDown-\(number)",
                    isEnabled: viewModel.canMoveSlot(index, by: 1)
                ) { viewModel.moveSlot(index, by: 1) }
                PopIconButton(
                    systemImage: PopSymbol.delete, label: TeamLabels.remove(number), identifier: "slotRemove-\(number)",
                    isDanger: true
                ) { viewModel.removeSlot(index) }
            }
        }
    }
}

/// 空の枠: 「ポケモン」の欄だけ(案内の文は置かない。格子の「+」が示す)。種族を選ぶと枠が埋まる(技・持ち物などはカードに出る)。
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
                    PopIcon(PopSymbol.picker)
                        .foregroundStyle(ColorToken.textSecondary.color)
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
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard()
    }
}
