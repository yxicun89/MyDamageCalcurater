import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenCards: 調整画面の入力カード(自分・調整の内容・相手・目標。AJ7。ADR-0502 §2)。

/// 自分: ポケモン・性格・特性・持ち物・技(+覚えるポケモン)・固定する能力ポイント。
struct AdjustOwnCardView: View {
    let viewModel: AdjustViewModel
    let focus: FocusState<AdjustFocus?>.Binding

    private var natureName: String {
        viewModel.natureOptions.first { $0.id == viewModel.ownNatureId }?.nameJa ?? AdjustText.naturePlaceholder
    }

    private var abilityName: String {
        viewModel.ownAbilityOptions.first { $0.id == viewModel.ownAbilityId }?.nameJa ?? AdjustText.unselectedOption
    }

    private var itemName: String {
        viewModel.ownItemId.map { viewModel.itemLabel(for: $0) } ?? AdjustText.unselectedOption
    }

    private var moveName: String {
        viewModel.ownMoveOptions.first { $0.id == viewModel.ownMoveId }?.nameJa ?? AdjustText.movePlaceholder
    }

    var body: some View {
        AdjustCard(title: AdjustText.ownRegion, identifier: "adjustOwnCard") {
            AdjustSpeciesRow(
                viewModel: viewModel, species: viewModel.ownSpecies, title: AdjustText.speciesField,
                accessibilityLabel: AdjustText.ownSpeciesLabel, identifier: "adjustOwnSpeciesButton"
            ) { option in
                Task { await viewModel.selectOwnSpecies(key: option.key) }
            }
            AdjustMenuRow(
                title: AdjustText.natureField, accessibilityLabel: AdjustText.ownNatureLabel, valueText: natureName,
                identifier: "adjustOwnNaturePicker"
            ) {
                Button(AdjustText.unselectedOption) { viewModel.selectOwnNature(id: nil) }
                ForEach(viewModel.natureOptions, id: \.id) { nature in
                    Button(nature.nameJa) { viewModel.selectOwnNature(id: nature.id) }
                }
            }
            AdjustMenuRow(
                title: AdjustText.abilityField, accessibilityLabel: AdjustText.ownAbilityLabel, valueText: abilityName,
                identifier: "adjustOwnAbilityPicker"
            ) {
                Button(AdjustText.unselectedOption) { viewModel.selectOwnAbility(id: nil) }
                ForEach(viewModel.ownAbilityOptions, id: \.id) { ability in
                    Button(ability.nameJa) { viewModel.selectOwnAbility(id: ability.id) }
                }
            }
            AdjustMenuRow(
                title: AdjustText.itemField, accessibilityLabel: AdjustText.ownItemLabel, valueText: itemName,
                identifier: "adjustOwnItemPicker", lockReason: lockReason(for: viewModel.ownItemLock)
            ) {
                Button(AdjustText.unselectedOption) { viewModel.selectOwnItem(id: nil) }
                ForEach(viewModel.ownItemOptions, id: \.id) { item in
                    Button(item.nameJa) { viewModel.selectOwnItem(id: item.id) }
                }
            }
            AdjustMenuRow(
                title: AdjustText.moveField, accessibilityLabel: AdjustText.ownMoveLabel, valueText: moveName,
                identifier: "adjustOwnMovePicker"
            ) {
                Button(AdjustText.unselectedOption) { viewModel.selectOwnMove(id: nil) }
                ForEach(viewModel.ownMoveOptions, id: \.id) { move in
                    Button(move.nameJa) { viewModel.selectOwnMove(id: move.id) }
                }
            }
            AdjustLearnersButton(
                accessibilityLabel: AdjustText.ownLearnersButtonLabel, identifier: "adjustOwnLearnersButton",
                isEnabled: viewModel.ownMoveId != nil
            ) {
                viewModel.scheduleOpenLearners(.own)
            }
            fixedSPFields
        }
    }

    private var fixedSPFields: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            AdjustFieldCaption(text: AdjustText.fixedSPGroup)
            ForEach(StatKey.allCases, id: \.self) { stat in
                AdjustNumberFieldRow(
                    title: AdjustText.fixedSPLabel(stat), hint: AdjustText.fixedSPHint,
                    identifier: "adjustFixedSP-\(stat.rawValue)",
                    text: Binding(get: { viewModel.fixedSPText(for: stat) }, set: { viewModel.setFixedSPText($0, for: stat) }),
                    focus: focus, focusValue: .fixedSP(stat)
                )
            }
            Text(AdjustText.fixedSPTotal(viewModel.fixedSPTotal))
                .font(TextStyleToken.body.font.monospacedDigit())
                .foregroundStyle(ColorToken.textPrimary.color)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityIdentifier("adjustFixedSPTotal")
        }
    }
}

/// 調整の内容: モード5つ(縦に並ぶ選択肢)とモードごとの欄。
struct AdjustModeCardView: View {
    let viewModel: AdjustViewModel
    let focus: FocusState<AdjustFocus?>.Binding

    /// 回す能力(上限の欄を出す能力)。耐久は H・B・D、攻撃は A か C と S。
    private var rotatedStats: [StatKey] {
        if viewModel.isGoalsMode { return [] }
        switch viewModel.mode {
        case .bulk: return [.hp, .def, .spd]
        case .offense: return [AttackerPreset.relevantStat(for: viewModel.offenseCategory), .spe]
        case .indices, .minKo, .minSurvive: return []
        }
    }

    var body: some View {
        AdjustCard(title: AdjustText.modeRegion, identifier: "adjustModeCard") {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                // 目標方式(F-11)は先頭。サーバーが提供していないと分かったら出さない(従来の5つだけに戻る)。
                if viewModel.goalsAvailable {
                    AdjustChoiceButton(
                        title: AdjustText.goalsModeLabel, isSelected: viewModel.isGoalsMode, identifier: "adjustMode-goals"
                    ) {
                        viewModel.selectGoalsMode()
                    }
                }
                ForEach(AdjustMode.allCases, id: \.self) { mode in
                    AdjustChoiceButton(
                        title: AdjustText.modeLabel(mode), isSelected: !viewModel.isGoalsMode && viewModel.mode == mode,
                        identifier: "adjustMode-\(mode.rawValue)"
                    ) {
                        viewModel.selectMode(mode)
                    }
                }
            }
            if !viewModel.isGoalsMode {
                if viewModel.mode == .bulk { focusChoices }
                if viewModel.mode == .offense { categoryChoices }
                if !rotatedStats.isEmpty { ceilingPickers }
                if viewModel.mode == .offense { minSpeedField }
                if viewModel.mode == .bulk || viewModel.mode == .offense { goalToggle }
            }
        }
    }

    private var focusChoices: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: AdjustText.focusField)
            ForEach(BulkFocus.allCases, id: \.self) { focusValue in
                AdjustChoiceButton(
                    title: AdjustText.focusLabel(focusValue), isSelected: viewModel.bulkFocus == focusValue,
                    identifier: "adjustBulkFocus-\(focusValue.rawValue)"
                ) {
                    viewModel.selectBulkFocus(focusValue)
                }
            }
        }
    }

    private var categoryChoices: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: AdjustText.offenseCategoryField)
            ForEach([MoveCategory.physical, .special], id: \.self) { category in
                AdjustChoiceButton(
                    title: AdjustText.offenseCategoryLabel(category), isSelected: viewModel.offenseCategory == category,
                    identifier: "adjustOffenseCategory-\(category.rawValue)"
                ) {
                    viewModel.selectOffenseCategory(category)
                }
            }
        }
    }

    private var ceilingPickers: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            AdjustFieldCaption(text: AdjustText.ceilingGroup)
            ForEach(rotatedStats, id: \.self) { stat in
                AdjustPickerRow(
                    title: AdjustText.ceilingLabel(stat), identifier: "adjustCeiling-\(stat.rawValue)",
                    selection: Binding(get: { viewModel.ceiling(for: stat) }, set: { viewModel.setCeiling($0, for: stat) })
                ) {
                    ForEach(0...SPLimits.maxPerStat, id: \.self) { value in
                        Text("\(value)").tag(value)
                    }
                }
            }
        }
    }

    private var minSpeedField: some View {
        AdjustNumberFieldRow(
            title: AdjustText.minSpeedField, hint: AdjustText.minSpeedHint, identifier: "adjustMinSpeed",
            text: Binding(get: { viewModel.minSpeedText }, set: { viewModel.setMinSpeedText($0) }),
            focus: focus, focusValue: .minSpeed
        )
    }

    private var goalToggle: some View {
        Toggle(AdjustText.useGoalField, isOn: Binding(get: { viewModel.useGoal }, set: { viewModel.setUseGoal($0) }))
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .tint(ColorToken.textPrimary.color)
            .accessibilityIdentifier("adjustUseGoalToggle")
    }
}

/// 相手: ポケモン・調整(プリセット)・技(相手が攻撃する側のときだけ)+「覚えるポケモン」。
struct AdjustOpponentCardView: View {
    let viewModel: AdjustViewModel
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    private var opponentMove: Move? { viewModel.opponentMoveOptions.first { $0.id == viewModel.opponentMoveId } }
    private var ownMove: Move? { viewModel.ownMoveOptions.first { $0.id == viewModel.ownMoveId } }

    var body: some View {
        AdjustCard(title: AdjustText.opponentRegion, identifier: "adjustOpponentCard") {
            AdjustSpeciesRow(
                viewModel: viewModel, species: viewModel.opponentSpecies, title: AdjustText.speciesField,
                accessibilityLabel: AdjustText.opponentSpeciesLabel, identifier: "adjustOpponentSpeciesButton"
            ) { option in
                Task { await viewModel.selectOpponentSpecies(key: option.key) }
            }
            if viewModel.opponentAttacks {
                opponentMovePicker
                AdjustLearnersButton(
                    accessibilityLabel: AdjustText.opponentLearnersButtonLabel, identifier: "adjustOpponentLearnersButton",
                    isEnabled: viewModel.opponentMoveId != nil
                ) {
                    viewModel.scheduleOpenLearners(.opponent)
                }
                attackerPresets
            } else {
                defenderPresets
            }
        }
    }

    private var opponentMovePicker: some View {
        AdjustMenuRow(
            title: AdjustText.moveField, accessibilityLabel: AdjustText.opponentMoveLabel,
            valueText: opponentMove?.nameJa ?? AdjustText.movePlaceholder, identifier: "adjustOpponentMovePicker"
        ) {
            Button(AdjustText.unselectedOption) { viewModel.selectOpponentMove(id: nil) }
            ForEach(viewModel.opponentMoveOptions, id: \.id) { move in
                Button(move.nameJa) { viewModel.selectOpponentMove(id: move.id) }
            }
        }
    }

    /// 相手が攻撃する側: ラベルの A / C は相手の技の分類(読み込み前は物理扱いで暫定表示)。
    private var attackerPresets: some View {
        let category = opponentMove?.category ?? .physical
        return presetRow {
            ForEach(AttackerPreset.allCases, id: \.self) { preset in
                AdjustPillButton(
                    title: preset.label(for: category), isSelected: viewModel.opponentAttackerPreset == preset,
                    identifier: "adjustOpponentAttackerPreset-\(preset.rawValue)"
                ) {
                    viewModel.selectOpponentAttackerPreset(preset)
                }
            }
        }
    }

    /// 相手が受ける側: ラベルの HB / HD は自分の技の分類(読み込み前は物理扱いで暫定表示)。
    private var defenderPresets: some View {
        let category = ownMove?.category ?? .physical
        return presetRow {
            ForEach(KnownDefenderPreset.allCases, id: \.self) { preset in
                AdjustPillButton(
                    title: preset.label(for: category), isSelected: viewModel.opponentDefenderPreset == preset,
                    identifier: "adjustOpponentDefenderPreset-\(preset.rawValue)"
                ) {
                    viewModel.selectOpponentDefenderPreset(preset)
                }
            }
        }
    }

    /// 3等分のピル。大きい文字サイズでは縦に積む(P6-15 と同じ分岐)。
    @ViewBuilder
    private func presetRow<Content: View>(@ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: AdjustText.presetField)
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x2) { content() }
            } else {
                HStack(spacing: SpacingToken.x2) { content() }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AdjustText.opponentPresetLabel)
    }
}

/// 目標: 発数(1〜10)・確率(確定 / 90 / 75 / 50%)。
struct AdjustGoalCardView: View {
    let viewModel: AdjustViewModel

    var body: some View {
        AdjustCard(title: AdjustText.goalRegion, identifier: "adjustGoalCard") {
            AdjustPickerRow(
                title: AdjustText.hitsField, identifier: "adjustHitsPicker",
                selection: Binding(get: { viewModel.hits }, set: { viewModel.selectHits($0) })
            ) {
                ForEach(AdjustViewModel.hitsOptions, id: \.self) { hits in
                    Text(AdjustText.hitsOption(hits)).tag(hits)
                }
            }
            AdjustPickerRow(
                title: AdjustText.thresholdField, identifier: "adjustThresholdPicker",
                selection: Binding(get: { viewModel.thresholdPercent }, set: { viewModel.selectThreshold($0) })
            ) {
                ForEach(AdjustViewModel.thresholdOptions, id: \.self) { percent in
                    Text(AdjustText.thresholdOption(percent)).tag(percent)
                }
            }
        }
    }
}
