import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// JudgeOptionSheet: 性格・特性・持ち物・状態異常の選択シート(P6-25。ADR-0504 §4)。
// `Menu` は使わない(中のボタンに identifier が付かない)。特性・持ち物は「未選択」に戻せる。性格は必須なので戻せない。状態異常は「なし」が実の選択肢なので「未選択」行を出さない(ADR-0512)。

enum JudgeOptionKind: String {
    case nature, ability, item, status

    var title: String {
        switch self {
        case .nature: return JudgeLabels.nature
        case .ability: return JudgeLabels.ability
        case .item: return JudgeLabels.item
        case .status: return JudgeLabels.status
        }
    }
}

struct JudgeOptionSheet: View {
    let viewModel: JudgeViewModel
    let target: JudgeTarget
    let kind: JudgeOptionKind
    @Environment(\.dismiss) private var dismiss

    /// (id, 名前)の並び。
    private var options: [(id: String, name: String)] {
        switch kind {
        case .nature: return viewModel.natureOptions.map { ($0.id, $0.nameJa) }
        case .ability: return viewModel.abilityOptions(for: target).map { ($0.id, $0.nameJa) }
        case .item: return viewModel.selectableItemOptions(for: target).map { ($0.id, $0.nameJa) }
        case .status: return JudgeStatus.allCases.map { ($0.rawValue, JudgeLabels.statusName($0)) }
        }
    }

    private var selectedID: String? {
        guard let draft = viewModel.draft(for: target) else { return nil }
        switch kind {
        case .nature: return draft.natureId
        case .ability: return draft.abilityId
        case .item: return draft.itemId
        case .status: return draft.status.rawValue
        }
    }

    var body: some View {
        NavigationStack {
            List {
                if kind == .ability || kind == .item {
                    Button {
                        choose(nil)
                    } label: {
                        row(name: JudgeLabels.unselected, isSelected: selectedID == nil, isSecondary: true)
                    }
                    .accessibilityIdentifier("judgeOptionNone")
                }
                if options.isEmpty {
                    Text(JudgeLabels.optionSheetEmpty)
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .multilineTextAlignment(.leading)
                        .accessibilityIdentifier("judgeOptionEmpty")
                }
                ForEach(options, id: \.id) { option in
                    Button {
                        choose(option.id)
                    } label: {
                        row(name: option.name, isSelected: selectedID == option.id, isSecondary: false)
                    }
                    .accessibilityLabel(option.name)
                    .accessibilityIdentifier("judgeOption-\(option.id)")
                }
            }
            .listStyle(.plain)
            .navigationTitle(kind.title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(JudgeLabels.optionSheetClose) { dismiss() }
                        .accessibilityIdentifier("judgeOptionSheetClose")
                }
            }
        }
        .accessibilityIdentifier("judgeOptionSheet")
    }

    private func choose(_ id: String?) {
        switch kind {
        case .nature: viewModel.setNature(id, for: target)
        case .ability: viewModel.setAbility(id, for: target)
        case .item: viewModel.setItem(id, for: target)
        case .status: viewModel.setStatus(id.flatMap(JudgeStatus.init(rawValue:)) ?? .none, for: target)
        }
        dismiss()
    }

    private func row(name: String, isSelected: Bool, isSecondary: Bool) -> some View {
        HStack(spacing: SpacingToken.x2) {
            Text(name)
                .font(TextStyleToken.body.font)
                .foregroundStyle(isSecondary ? ColorToken.textSecondary.color : ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
            if isSelected {
                Image(systemName: "checkmark")
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .accessibilityHidden(true)
            }
        }
    }
}
