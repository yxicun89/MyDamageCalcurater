import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// JudgeIndividualCard: 判定画面の 1 体ぶんの入力(自分・相手候補で同じ形。P6-25。ADR-0504 §10)。
// 種族・技は検索シート、性格・特性・持ち物は選択シート。能力ポイントとランクは −/値/+ のボタン(`Stepper` ではない)。
// 文言が長くても行数を制限せず折り返す(AX5 でも横にはみ出さない)。識別子は `<identifierPrefix>…`。

struct JudgeIndividualCard: View {
    let viewModel: JudgeViewModel
    let target: JudgeTarget
    let identifierPrefix: String
    let title: String
    let removeAction: JudgeRemoveAction?
    let onPick: (JudgePickerRequest) -> Void

    private var draft: JudgeDraft { viewModel.draft(for: target) ?? JudgeDraft() }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            header
            selectButton(
                caption: JudgeLabels.species, value: viewModel.speciesName(for: target), id: "SpeciesButton", request: .species)
            selectButton(caption: JudgeLabels.nature, value: natureName, id: "NatureButton", request: .option(.nature))
            selectButton(caption: JudgeLabels.ability, value: abilityName, id: "AbilityButton", request: .option(.ability))
            selectButton(
                caption: JudgeLabels.item, value: itemName, id: "ItemButton", request: .option(.item),
                lockReason: lockReason(for: viewModel.itemLock(for: target)))
            selectButton(caption: JudgeLabels.move, value: moveName, id: "MoveButton", request: .move)
            spSection
            rankSection
            TeamSourceMenuRow(
                teamOptions: viewModel.teamOptions, selection: nil, identifierPrefix: "\(identifierPrefix)Team"
            ) { teamID, memberID in
                Task { await viewModel.applyTeamMember(teamID: teamID, memberID: memberID, to: target) }
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier(identifierPrefix)
    }

    // MARK: - 見出し

    private var header: some View {
        HStack(alignment: .top, spacing: SpacingToken.x2) {
            Text(title)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
            if let removeAction {
                Button(action: removeAction.perform) {
                    Image(systemName: "xmark")
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .padding(SpacingToken.x2)
                        .background(ColorToken.bgGlass.color, in: Circle())
                }
                .buttonStyle(.plain)
                .disabled(!removeAction.isEnabled)
                .opacity(removeAction.isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
                .accessibilityLabel(JudgeLabels.removeCandidate(removeAction.number))
                .accessibilityIdentifier("judgeRemoveCandidate-\(removeAction.number)")
            }
        }
    }

    // MARK: - 選択ボタン

    private var natureName: String? {
        guard let id = draft.natureId else { return nil }
        return viewModel.natureOptions.first { $0.id == id }?.nameJa ?? id
    }

    private var abilityName: String? {
        guard let id = draft.abilityId else { return nil }
        return viewModel.abilityOptions(for: target).first { $0.id == id }?.nameJa ?? id
    }

    private var itemName: String? {
        guard let id = draft.itemId else { return nil }
        return viewModel.itemLabel(for: id)
    }

    private var moveName: String? {
        guard let id = draft.moveId else { return nil }
        return viewModel.move(forID: id)?.nameJa ?? id
    }

    /// `lockReason` が非 nil なら操作できない(メガ種族の持ち物。理由の文を下に出す。ADR-0509 §8)。
    private func selectButton(
        caption: String, value: String?, id: String, request: JudgePickerRequest, lockReason: String? = nil
    ) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: caption)
            Button {
                onPick(request)
            } label: {
                HStack(spacing: SpacingToken.x2) {
                    Text(value ?? JudgeLabels.unselected)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(value == nil ? ColorToken.textSecondary.color : ColorToken.textPrimary.color)
                        .multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Image(systemName: "chevron.right")
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                }
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(
                    ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                )
                .overlay(
                    RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                        .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            }
            .buttonStyle(.plain)
            .disabled(lockReason != nil)
            .accessibilityLabel("\(caption) \(value ?? JudgeLabels.unselected)")
            .accessibilityHint(lockReason ?? "")
            .accessibilityIdentifier("\(identifierPrefix)\(id)")
            if let lockReason {
                Text(lockReason)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("\(identifierPrefix)\(id)LockReason")
            }
        }
    }

    // MARK: - 能力ポイント・ランク

    private var spSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            SpeedFlowLayout {
                ForEach(StatKey.allCases, id: \.self) { stat in
                    let value = Self.spValue(draft.sp, stat)
                    JudgeStepper(
                        title: JudgeLabels.spLabel(stat), valueText: String(value),
                        decrementLabel: JudgeLabels.spDecrement(stat), incrementLabel: JudgeLabels.spIncrement(stat),
                        canDecrement: value > 0, canIncrement: value < SPLimits.maxPerStat,
                        identifiers: .init(prefix: identifierPrefix, kind: "SP", stat: stat),
                        onDecrement: { viewModel.setSP(stat, value - 1, for: target) },
                        onIncrement: { viewModel.setSP(stat, value + 1, for: target) })
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            let total = Self.spTotal(draft.sp)
            Text("\(JudgeLabels.spTotalCaption) \(total) / \(SPLimits.maxTotal)")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(total > SPLimits.maxTotal ? ColorToken.danger.color : ColorToken.textSecondary.color)
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityIdentifier("\(identifierPrefix)SPTotal")
        }
    }

    private var rankSection: some View {
        SpeedFlowLayout {
            ForEach(StatKey.allCases.filter { $0 != .hp }, id: \.self) { stat in
                let value = Self.rankValue(draft.ranks, stat)
                JudgeStepper(
                    title: JudgeLabels.rankLabel(stat), valueText: Self.rankText(value),
                    decrementLabel: JudgeLabels.rankDecrement(stat), incrementLabel: JudgeLabels.rankIncrement(stat),
                    canDecrement: value > RankLimits.min, canIncrement: value < RankLimits.max,
                    identifiers: .init(prefix: identifierPrefix, kind: "Rank", stat: stat),
                    onDecrement: { viewModel.setRank(stat, value - 1, for: target) },
                    onIncrement: { viewModel.setRank(stat, value + 1, for: target) })
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private static func spValue(_ sp: StatBlock, _ stat: StatKey) -> Int {
        switch stat {
        case .hp: return sp.hp
        case .atk: return sp.atk
        case .def: return sp.def
        case .spa: return sp.spa
        case .spd: return sp.spd
        case .spe: return sp.spe
        }
    }

    private static func spTotal(_ sp: StatBlock) -> Int {
        StatKey.allCases.reduce(0) { $0 + spValue(sp, $1) }
    }

    private static func rankValue(_ ranks: RankBlock, _ stat: StatKey) -> Int {
        switch stat {
        case .hp: return 0
        case .atk: return ranks.atk
        case .def: return ranks.def
        case .spa: return ranks.spa
        case .spd: return ranks.spd
        case .spe: return ranks.spe
        }
    }

    /// 「+2」「0」「-3」。
    private static func rankText(_ value: Int) -> String { value > 0 ? "+\(value)" : String(value) }
}

/// 選択の種類。`JudgePicker`(画面側)へ対象を足して渡すための、カード側の要求。
enum JudgePickerRequest {
    case species
    case move
    case option(JudgeOptionKind)
}

/// −/値/+ の組(識別子は `<prefix><kind>Decrement-<stat>` / `Value-<stat>` / `Increment-<stat>`)。
private struct JudgeStepper: View {
    struct Identifiers {
        let prefix: String
        let kind: String
        let stat: StatKey

        var decrement: String { "\(prefix)\(kind)Decrement-\(stat.rawValue)" }
        var value: String { "\(prefix)\(kind)Value-\(stat.rawValue)" }
        var increment: String { "\(prefix)\(kind)Increment-\(stat.rawValue)" }
    }

    let title: String
    let valueText: String
    let decrementLabel: String
    let incrementLabel: String
    let canDecrement: Bool
    let canIncrement: Bool
    let identifiers: Identifiers
    let onDecrement: () -> Void
    let onIncrement: () -> Void

    private static let valueMinWidth: CGFloat = 48
    /// − と + で押せる範囲をそろえる(「−」の記号は細く、そのままだと押せる範囲が小さい)。
    private static let stepButtonMinSide: CGFloat = 36

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: title)
            HStack(spacing: SpacingToken.x2) {
                stepButton("minus", label: decrementLabel, enabled: canDecrement, id: identifiers.decrement, action: onDecrement)
                Text(valueText)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .frame(minWidth: Self.valueMinWidth)
                    .accessibilityIdentifier(identifiers.value)
                stepButton("plus", label: incrementLabel, enabled: canIncrement, id: identifiers.increment, action: onIncrement)
            }
        }
    }

    private func stepButton(_ systemName: String, label: String, enabled: Bool, id: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: systemName)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .padding(SpacingToken.x2)
                .frame(minWidth: Self.stepButtonMinSide, minHeight: Self.stepButtonMinSide)
                .background(ColorToken.bgGlass.color, in: Circle())
                .contentShape(Circle())
        }
        .buttonStyle(.plain)
        .disabled(!enabled)
        .opacity(enabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
        .accessibilityLabel(label)
        .accessibilityIdentifier(id)
    }
}
