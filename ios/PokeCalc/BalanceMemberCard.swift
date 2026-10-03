import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceMemberCard: タイプバランス画面のメンバー1体分のカード(ADR-0415)。
// 種族(変更はメンバーを削除して追加し直す)・特性・技(最大4。攻撃範囲の入力)・エラー。
// 構築編集の `MemberCardView` と同じ部品(`MenuLabelChip`・`MoveSearchSheet`)を再利用する。

struct BalanceMemberCard: View {
    let viewModel: BalanceViewModel
    let member: BalanceMember
    /// 1 始まりの並び順(アクセシビリティ用の「メンバー1」)。
    let number: Int
    /// 自分のメンバーか仮想敵か(ラベルと削除の宛先だけが違う。ADR-0415 §8)。
    var kind: BalanceMemberKind = .party
    /// 非 nil のとき、その index の技スロットの検索シートが開いている(シートはカードに1つ)。
    @State private var moveSearchSlot: MoveSlotTarget?

    private var abilityOptions: [Ability] {
        viewModel.abilityOptionsByMember[member.id] ?? []
    }

    private var moveOptions: [Move] {
        viewModel.moveOptionsByMember[member.id] ?? []
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            header
            abilityPicker
            movesSection
            if let memberError = viewModel.memberErrors[member.id] {
                Text(memberError.uiMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("balanceMemberError-\(member.id)")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityLabel(kind.groupLabel(number))
        .accessibilityIdentifier("balanceMemberCard-\(member.id)")
    }

    private var header: some View {
        HStack(spacing: SpacingToken.x2) {
            SpeciesImageView(speciesKey: member.speciesKey, name: member.nameJa, types: member.types)
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(member.nameJa)
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .lineLimit(2)
                    .fixedSize(horizontal: false, vertical: true)
                HStack(spacing: SpacingToken.x1) {
                    ForEach(member.types, id: \.self) { TypeBadgeView(type: $0) }
                }
            }
            Spacer(minLength: 0)
            Button {
                switch kind {
                case .party: viewModel.removeMember(id: member.id)
                case .threat: viewModel.removeThreat(id: member.id)
                }
            } label: {
                Image(systemName: "trash")
                    .foregroundStyle(ColorToken.danger.color)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("balanceMemberDelete-\(member.id)")
            .accessibilityLabel(kind.removeLabel(number))
        }
    }

    private var abilityLabel: String {
        abilityOptions.first(where: { $0.id == member.abilityId })?.nameJa ?? BalanceScreenText.noAbilityOption
    }

    private var abilityPicker: some View {
        Menu {
            Button(BalanceScreenText.noAbilityOption) {
                viewModel.setAbility(id: member.id, abilityId: nil)
            }
            ForEach(abilityOptions, id: \.id) { ability in
                Button(ability.nameJa) {
                    viewModel.setAbility(id: member.id, abilityId: ability.id)
                }
            }
        } label: {
            MenuLabelChip(text: "\(BalanceScreenText.abilityLabel): \(abilityLabel)")
        }
        .accessibilityIdentifier("balanceMemberAbilityPicker-\(member.id)")
    }

    private var movesSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            ForEach(0..<TeamLimits.maxMovesPerMember, id: \.self) { index in
                moveSlot(index: index)
            }
        }
        // シートは1つだけ持ち、いま編集中のスロットで内容を出し分ける(構築編集と同じ)。
        .sheet(item: $moveSearchSlot) { target in
            let selectedID = member.moveIds.indices.contains(target.index) ? member.moveIds[target.index] : nil
            let remainingOptions = moveOptions.filter { !member.moveIds.contains($0.id) }
            MoveSearchSheet(
                viewModel: viewModel, options: remainingOptions,
                onSelect: { move in viewModel.addMove(id: member.id, moveId: move.id) },
                removeAction: selectedID != nil ? { viewModel.removeMove(id: member.id, at: target.index) } : nil
            )
        }
    }

    /// 選んだ技の名前は一度でも見た技から引く。引けないときは ID をそのまま出す(構築編集と同じ規則)。
    private func moveSlot(index: Int) -> some View {
        let selectedID = member.moveIds.indices.contains(index) ? member.moveIds[index] : nil
        let label = selectedID.map { id in viewModel.move(forID: id)?.nameJa ?? id } ?? "技を選択"
        return Button {
            moveSearchSlot = MoveSlotTarget(index: index)
        } label: {
            MenuLabelChip(text: "\(BalanceScreenText.moveLabel(index + 1)): \(label)")
        }
        .accessibilityIdentifier("balanceMemberMoveSlot-\(member.id)-\(index)")
    }
}

/// `.sheet(item:)` に渡す、開いている技スロットの index。
private struct MoveSlotTarget: Identifiable, Equatable {
    let index: Int
    var id: Int { index }
}

/// `BalanceMemberCard` が描く対象。自分のパーティのメンバーか、仮想敵か。
enum BalanceMemberKind {
    case party
    case threat

    func groupLabel(_ number: Int) -> String {
        switch self {
        case .party: return BalanceScreenText.memberGroupLabel(number)
        case .threat: return BalanceScreenText.threatGroupLabel(number)
        }
    }

    func removeLabel(_ number: Int) -> String {
        switch self {
        case .party: return BalanceScreenText.removeMemberLabel(number)
        case .threat: return BalanceScreenText.removeThreatLabel(number)
        }
    }
}
