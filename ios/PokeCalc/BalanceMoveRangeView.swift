import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceMoveRangeView: タイプバランスの技範囲チェッカー(move-range。ADR-0415 第3段・§8)。
//
// 技を1〜4つ選ぶだけ(ポケモンは選ばない。メンバーとは無関係)。技構成の攻撃範囲(18タイプ別の最大倍率)と、
// それを半減以下で受けられるポケモン(タイプだけ・特性ありは別枠)を縦並びで出す。倍率は応答のまま、有効/抜群は文字で出す。

struct BalanceMoveRangeView: View {
    let viewModel: BalanceViewModel
    /// 非 nil のとき、その index の技スロットの検索シートが開いている(シートは1つ)。
    @State private var moveSearchSlot: RangeSlotTarget?

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            BalanceSectionHeader(
                title: BalanceScreenText.moveRangeRegionLabel, isLoading: viewModel.isLoadingMoveRange,
                identifier: "balanceMoveRangeLoading", loadingText: BalanceScreenText.moveRangeLoadingNotice)
            slots
            if let inputError = viewModel.rangeInputError {
                Text(inputError.uiMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("balanceMoveRangeInputError")
            }
            if viewModel.rangeMoveIds.isEmpty {
                Text(BalanceScreenText.moveRangeEmptyNotice)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("balanceMoveRangeEmptyNotice")
            }
            if let error = viewModel.moveRangeError {
                ErrorBannerView(message: error.message, identifier: "balanceMoveRangeError")
            }
            if let result = viewModel.moveRange {
                resultViews(result)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("balanceMoveRangeSection")
        .sheet(item: $moveSearchSlot) { target in
            let hasMove = viewModel.rangeMoveIds.indices.contains(target.index)
            MoveSearchSheet(
                viewModel: viewModel, options: viewModel.moveRangeOptions,
                onSelect: { move in viewModel.addRangeMove(moveId: move.id) },
                removeAction: hasMove ? { viewModel.removeRangeMove(at: target.index) } : nil
            )
        }
    }

    /// 選んだ技のスロット(選んだ数 + 追加の1つ。最大 `TeamLimits.maxMovesPerMember`)。名前は一度でも見た技から引き、無ければ ID。
    private var slots: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            ForEach(0..<min(viewModel.rangeMoveIds.count + 1, TeamLimits.maxMovesPerMember), id: \.self) { index in
                slotButton(index: index)
            }
        }
    }

    private func slotButton(index: Int) -> some View {
        let selectedID = viewModel.rangeMoveIds.indices.contains(index) ? viewModel.rangeMoveIds[index] : nil
        let label = selectedID.map { id in viewModel.move(forID: id)?.nameJa ?? id } ?? BalanceScreenText.moveRangeAddMoveLabel
        return Button {
            moveSearchSlot = RangeSlotTarget(index: index)
        } label: {
            MenuLabelChip(text: "\(BalanceScreenText.moveRangeMoveLabel(index + 1)): \(label)")
        }
        .accessibilityIdentifier("balanceMoveRangeSlot-\(index)")
    }

    @ViewBuilder
    private func resultViews(_ result: BalanceMoveRange) -> some View {
        BalanceCard(title: BalanceScreenText.moveRangeTypeChartLabel, identifier: "balanceMoveRangeTypeChart") {
            HStack(spacing: SpacingToken.x1) {
                Text(BalanceScreenText.attackTypesLabel)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                ForEach(result.attackTypes, id: \.self) { TypeBadgeView(type: $0) }
            }
            ForEach(result.typeChart, id: \.defenseType) { entry in
                BalanceTypeRow(type: entry.defenseType) {
                    Text(BalanceLabels.coverageLabel(entry.bestMultiplier))
                        .fontWeight(entry.superEffective ? .semibold : .regular)
                }
            }
        }
        BalanceCard(title: BalanceScreenText.walledByLabel, identifier: "balanceWalledBy") {
            if result.walledBy.isEmpty {
                emptyWallNotice
            }
            ForEach(Array(result.walledBy.prefix(BalanceDisplayLimits.pokemonPerList).enumerated()), id: \.offset) { _, pokemon in
                walledRow(
                    name: pokemon.nameJa ?? pokemon.pokemonId, types: pokemon.types, detail: nil,
                    multiplier: pokemon.bestMultiplier)
            }
            moreNotice(total: result.walledBy.count)
        }
        BalanceCard(title: BalanceScreenText.walledByAbilityLabel, identifier: "balanceWalledByAbility") {
            if result.walledByAbility.isEmpty {
                emptyWallNotice
            }
            ForEach(Array(result.walledByAbility.prefix(BalanceDisplayLimits.pokemonPerList).enumerated()), id: \.offset) { _, pokemon in
                walledRow(
                    name: pokemon.nameJa ?? pokemon.pokemonId, types: [],
                    detail: viewModel.abilityName(forID: pokemon.abilityId) ?? pokemon.abilityId,
                    multiplier: pokemon.bestMultiplier)
            }
            moreNotice(total: result.walledByAbility.count)
        }
    }

    private var emptyWallNotice: some View {
        Text(BalanceScreenText.noWalledNotice)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textSecondary.color)
    }

    @ViewBuilder
    private func moreNotice(total: Int) -> some View {
        let rest = total - BalanceDisplayLimits.pokemonPerList
        if rest > 0 {
            Text(BalanceScreenText.moreCountLabel(rest))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
    }

    /// ポケモン1匹: 名前(+ 特性名)・タイプ(あれば)・倍率。折り返せる縦並び。
    private func walledRow(name: String, types: [PokeType], detail: String?, multiplier: String) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(detail.map { "\(name)(\($0))" } ?? name)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
            HStack(spacing: SpacingToken.x1) {
                ForEach(types, id: \.self) { TypeBadgeView(type: $0) }
                Text(BalanceLabels.multiplierLabel(multiplier))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }
}

/// `.sheet(item:)` に渡す、開いている技スロットの index。
private struct RangeSlotTarget: Identifiable, Equatable {
    let index: Int
    var id: Int { index }
}
