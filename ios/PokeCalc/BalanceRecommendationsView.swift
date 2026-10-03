import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceRecommendationsView: タイプバランスのおすすめタイプ(recommendations。ADR-0415 第3段・§8)。
//
// メンバーだけで決まる(仮想敵は含めない)。防御の穴・攻撃範囲の穴、候補、特性で補えるポケモンを縦並びで出す。
// 長い一覧は `BalanceDisplayLimits.pokemonPerList` 件までにして「ほか N件」と出す。特性名は引けたものだけ名前、
// 引けなければ特性 ID。失敗(overloaded を含む)は日本語の文言と「再計算」ボタン(ユーザー操作で再試行)。

struct BalanceRecommendationsView: View {
    let viewModel: BalanceViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            BalanceSectionHeader(
                title: BalanceScreenText.recommendationsRegionLabel, isLoading: viewModel.isLoadingRecommendations,
                identifier: "balanceRecommendationsLoading", loadingText: BalanceScreenText.recommendationsLoadingNotice)
            if let error = viewModel.recommendationsError {
                ErrorBannerView(message: error.message, identifier: "balanceRecommendationsError")
                Button {
                    Task { await viewModel.refreshRecommendations() }
                } label: {
                    Label(BalanceScreenText.retryLabel, systemImage: "arrow.clockwise")
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                }
                .accessibilityIdentifier("balanceRecommendationsRetry")
            }
            if let result = viewModel.recommendations {
                holes(result)
                ForEach(Array(result.candidates.enumerated()), id: \.offset) { index, candidate in
                    candidateCard(candidate, number: index + 1)
                }
                if !result.abilityOptions.isEmpty {
                    abilityOptionsCard(result.abilityOptions)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("balanceRecommendationsSection")
    }

    private func holes(_ result: BalanceRecommendations) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(BalanceScreenText.defenseHolesLabel(BalanceLabels.typeListText(result.defenseHoles)))
            Text(BalanceScreenText.offenseHolesLabel(BalanceLabels.typeListText(result.offenseHoles)))
        }
        .font(TextStyleToken.body.font)
        .foregroundStyle(ColorToken.textPrimary.color)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("balanceHoles")
    }

    private func candidateCard(_ candidate: BalanceTypeCandidate, number: Int) -> some View {
        BalanceCard(title: BalanceLabels.typeListText(candidate.types), identifier: "balanceCandidate-\(number)") {
            HStack(spacing: SpacingToken.x1) {
                ForEach(candidate.types, id: \.self) { TypeBadgeView(type: $0) }
            }
            labeledText(BalanceScreenText.defenseCoveredColumnLabel, BalanceLabels.typeListText(candidate.defenseCovered))
            labeledText(BalanceScreenText.offenseCoveredColumnLabel, BalanceLabels.typeListText(candidate.offenseCovered))
            labeledText(
                BalanceScreenText.pokemonColumnLabel,
                pokemonListText(
                    candidate.pokemon.map { $0.nameJa ?? $0.pokemonId }))
        }
    }

    private func abilityOptionsCard(_ options: [BalanceAbilityOption]) -> some View {
        BalanceCard(title: BalanceScreenText.abilityOptionsTableLabel, identifier: "balanceAbilityOptions") {
            ForEach(options, id: \.attackType) { option in
                BalanceTypeRow(type: option.attackType) {
                    Text(
                        pokemonListText(
                            option.pokemon.map {
                                BalanceScreenText.abilityOptionEntryLabel(
                                    name: $0.nameJa ?? $0.pokemonId,
                                    ability: viewModel.abilityName(forID: $0.abilityId) ?? $0.abilityId,
                                    multiplier: BalanceLabels.multiplierLabel($0.multiplier))
                            })
                    )
                }
            }
        }
    }

    private func labeledText(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(label)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            Text(value)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }

    /// 「・」区切りで `pokemonPerList` 件まで。超えた分は「ほか N件」。空なら「なし」。
    private func pokemonListText(_ entries: [String]) -> String {
        BalanceListText.joined(entries)
    }
}

/// 長い一覧を上限までにして「ほか N件」を足す(推奨・技範囲で共通)。
enum BalanceListText {
    static func joined(_ entries: [String]) -> String {
        guard !entries.isEmpty else { return BalanceScreenText.noneLabel }
        let shown = entries.prefix(BalanceDisplayLimits.pokemonPerList)
            .joined(separator: BalanceScreenText.listSeparator)
        let rest = entries.count - BalanceDisplayLimits.pokemonPerList
        return rest > 0 ? "\(shown) \(BalanceScreenText.moreCountLabel(rest))" : shown
    }
}
