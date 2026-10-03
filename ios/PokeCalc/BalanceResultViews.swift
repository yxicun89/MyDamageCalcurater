import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceResultViews: タイプバランス画面の結果(ADR-0415。docs/type-balance-design.md §10)。
//
// 表は横に広げず、タイプごとの縦並びにする(Dynamic Type 最大でも崩れない・ダークモードはトークンに従う)。
// 倍率の語(弱点/耐性/無効・抜群/いまひとつ)は文字で出し、色だけに頼らない。倍率と集計は応答のまま。

struct BalanceResultsView: View {
    let viewModel: BalanceViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x4) {
            if viewModel.members.isEmpty {
                Text(BalanceScreenText.emptyNotice)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("balanceEmptyNotice")
            } else {
                defenseSection
                coverageSection
            }
        }
    }

    // MARK: - 防御相性(analyze)

    private var defenseSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            BalanceSectionHeader(
                title: BalanceScreenText.defenseTableLabel, isLoading: viewModel.isLoadingAnalysis,
                identifier: "balanceDefenseLoading")
            if let error = viewModel.analysisError {
                ErrorBannerView(message: error.message, identifier: "balanceAnalysisError")
            }
            if let analysis = viewModel.analysis {
                teamSummaryCard(analysis)
                ForEach(Array(analysis.members.enumerated()), id: \.offset) { index, member in
                    memberDefenseCard(member, number: index + 1)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("balanceDefenseSection")
    }

    private func teamSummaryCard(_ analysis: BalanceDefenseAnalysis) -> some View {
        BalanceCard(title: BalanceScreenText.teamSummaryTableLabel, identifier: "balanceTeamSummary") {
            ForEach(analysis.teamSummary, id: \.attackType) { entry in
                BalanceTypeRow(type: entry.attackType) {
                    Text(
                        [
                            BalanceScreenText.weakSummary(weak: entry.weak, quadWeak: entry.quadWeak),
                            BalanceScreenText.resistSummary(entry.resist),
                            BalanceScreenText.immuneSummary(entry.immune),
                            BalanceScreenText.neutralSummary(entry.neutral),
                        ].joined(separator: " / ")
                    )
                    // 弱点が1体でもいるタイプは太字にして、色以外でも目に留まるようにする。
                    .fontWeight(entry.weak > 0 ? .semibold : .regular)
                }
            }
        }
    }

    private func memberDefenseCard(_ member: BalanceMemberDefense, number: Int) -> some View {
        BalanceCard(title: memberTitle(number: number, pokemonId: member.pokemonId), identifier: "balanceMemberDefense-\(number)") {
            ForEach(member.defense, id: \.attackType) { entry in
                BalanceTypeRow(type: entry.attackType) {
                    let note = entry.source == .ability ? "(\(BalanceScreenText.abilityEffectNote))" : ""
                    Text(BalanceLabels.defenseLabel(multiplier: entry.multiplier, category: entry.category) + note)
                        .fontWeight(entry.category == .neutral ? .regular : .semibold)
                }
            }
        }
    }

    // MARK: - 攻撃範囲(coverage)

    private var coverageSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            BalanceSectionHeader(
                title: BalanceScreenText.coverageTableLabel, isLoading: viewModel.isLoadingCoverage,
                identifier: "balanceCoverageLoading")
            if let error = viewModel.coverageError {
                ErrorBannerView(message: error.message, identifier: "balanceCoverageError")
            }
            if let coverage = viewModel.coverage {
                BalanceCard(title: BalanceScreenText.defenseTypeColumnLabel, identifier: "balanceTeamCoverage") {
                    ForEach(coverage.teamCoverage, id: \.defenseType) { entry in
                        BalanceTypeRow(type: entry.defenseType) {
                            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                                Text(BalanceLabels.coverageLabel(entry.bestMultiplier))
                                    .fontWeight(entry.superEffectiveMembers > 0 ? .semibold : .regular)
                                Text(
                                    "\(BalanceScreenText.effectiveSummary(entry.effectiveMembers)) / \(BalanceScreenText.superEffectiveSummary(entry.superEffectiveMembers))"
                                )
                                .font(TextStyleToken.caption.font)
                                .foregroundStyle(ColorToken.textSecondary.color)
                            }
                        }
                    }
                }
                ForEach(Array(coverage.members.enumerated()), id: \.offset) { index, member in
                    memberCoverageCard(member, number: index + 1)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("balanceCoverageSection")
    }

    private func memberCoverageCard(_ member: BalanceMemberCoverage, number: Int) -> some View {
        BalanceCard(title: memberTitle(number: number, pokemonId: member.pokemonId), identifier: "balanceMemberCoverage-\(number)") {
            if member.attackTypes.isEmpty {
                Text(BalanceLabels.coverageLabel(nil))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
            } else {
                HStack(spacing: SpacingToken.x1) {
                    Text(BalanceScreenText.attackTypesLabel)
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                    ForEach(member.attackTypes, id: \.self) { TypeBadgeView(type: $0) }
                }
            }
            ForEach(member.coverage.filter { $0.bestMultiplier != nil }, id: \.defenseType) { entry in
                BalanceTypeRow(type: entry.defenseType) {
                    Text(BalanceLabels.coverageLabel(entry.bestMultiplier))
                        .fontWeight(entry.superEffective ? .semibold : .regular)
                }
            }
        }
    }

    /// 応答の members は要求順。名前は画面のメンバー(同じ並び)から引く。
    private func memberTitle(number: Int, pokemonId: String) -> String {
        let name = viewModel.members.indices.contains(number - 1) ? viewModel.members[number - 1].nameJa : pokemonId
        return "\(BalanceScreenText.memberGroupLabel(number)) \(name)"
    }
}

// MARK: - 部品

/// 見出し + 計算中の文言(色や動きに頼らず「計算中」の文字で出す。常時動くアニメーションは入れない)。
private struct BalanceSectionHeader: View {
    let title: String
    let isLoading: Bool
    let identifier: String

    var body: some View {
        HStack(spacing: SpacingToken.x2) {
            Text(title)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .accessibilityAddTraits(.isHeader)
            if isLoading {
                Text(BalanceScreenText.loadingNotice)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier(identifier)
            }
        }
    }
}

private struct BalanceCard<Content: View>: View {
    let title: String
    let identifier: String
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(title)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .accessibilityAddTraits(.isHeader)
            content
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier(identifier)
    }
}

/// 「タイプバッジ + 内容」の1行。文字が増えても(Dynamic Type 最大)折り返せるよう、内容は残りの幅いっぱいに取る。
private struct BalanceTypeRow<Content: View>: View {
    let type: PokeType
    @ViewBuilder let content: Content

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            TypeBadgeView(type: type)
            content
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityElement(children: .combine)
    }
}
