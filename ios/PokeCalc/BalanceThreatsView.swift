import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceThreatsView: タイプバランスの仮想敵(threats。ADR-0415 第3段・§8)。
//
// 仮想敵の入力(最大6体。メンバーと同じカードを再利用)と、仮想敵ごとの結果。結果は表にせず、メンバーごとの縦並びにする
// (Dynamic Type 最大でも崩れない)。倍率は応答のまま、「安全」「抜群」は応答の真偽値を文字で出す(色だけに頼らない)。

struct BalanceThreatsView: View {
    let viewModel: BalanceViewModel
    @State private var isSpeciesSearchPresented = false

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            BalanceSectionHeader(
                title: BalanceScreenText.threatsRegionLabel, isLoading: viewModel.isLoadingThreats,
                identifier: "balanceThreatsLoading", loadingText: BalanceScreenText.threatsLoadingNotice)
            ForEach(Array(viewModel.threatMembers.enumerated()), id: \.element.id) { index, threat in
                BalanceMemberCard(viewModel: viewModel, member: threat, number: index + 1, kind: .threat)
            }
            addThreatButton
            if viewModel.threatError != nil {
                Text(BalanceScreenText.tooManyThreats)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("balanceThreatError")
            }
            if viewModel.threatMembers.isEmpty {
                Text(BalanceScreenText.emptyThreatsNotice)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("balanceEmptyThreatsNotice")
            }
            if let error = viewModel.threatsError {
                ErrorBannerView(message: error.message, identifier: "balanceThreatsError")
            }
            if let threats = viewModel.threats {
                ForEach(Array(threats.threats.enumerated()), id: \.offset) { index, threat in
                    threatCard(threat, number: index + 1)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("balanceThreatsSection")
    }

    private var addThreatButton: some View {
        Button {
            isSpeciesSearchPresented = true
        } label: {
            Label(BalanceScreenText.addThreatLabel, systemImage: "plus.circle")
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
        }
        .accessibilityIdentifier("balanceAddThreatButton")
        .sheet(isPresented: $isSpeciesSearchPresented) {
            SpeciesSearchSheet(viewModel: viewModel) { option in
                Task { await viewModel.addThreat(speciesKey: option.key) }
            }
        }
    }

    /// 応答の順は要求順。名前は画面の入力(同じ並び)から引き、引けなければ ID を出す。
    private func name(of list: [BalanceMember], at index: Int, fallback: String) -> String {
        list.indices.contains(index) ? list[index].nameJa : fallback
    }

    private func threatCard(_ threat: BalanceThreatResult, number: Int) -> some View {
        let title = BalanceScreenText.threatRegionLabel(
            number, name: name(of: viewModel.threatMembers, at: number - 1, fallback: threat.pokemonId))
        return BalanceCard(title: title, identifier: "balanceThreat-\(number)") {
            HStack(spacing: SpacingToken.x1) {
                Text(BalanceScreenText.attackTypesLabel)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                if threat.attackTypes.isEmpty {
                    Text(BalanceLabels.matchupMultiplierLabel(nil))
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                } else {
                    ForEach(threat.attackTypes, id: \.self) { TypeBadgeView(type: $0) }
                }
            }
            ForEach(Array(threat.matchups.enumerated()), id: \.offset) { index, matchup in
                matchupRow(
                    matchup,
                    memberName: name(of: viewModel.members, at: index, fallback: matchup.pokemonId))
            }
            Text(BalanceScreenText.safeMembersLabel(threat.safeMembers))
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fontWeight(threat.safeMembers > 0 ? .semibold : .regular)
            Text(BalanceScreenText.superEffectiveMembersLabel(threat.superEffectiveMembers))
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fontWeight(threat.superEffectiveMembers > 0 ? .semibold : .regular)
        }
    }

    /// メンバー1体分: 名前、受ける倍率・与える倍率、安全・抜群(語で出す)。折り返せる縦並び。
    private func matchupRow(_ matchup: BalanceThreatMatchup, memberName: String) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(memberName)
                .font(TextStyleToken.body.font)
                .fontWeight(.semibold)
                .foregroundStyle(ColorToken.textPrimary.color)
            Text(
                "\(BalanceScreenText.incomingColumnLabel) \(BalanceLabels.matchupMultiplierLabel(matchup.incoming))"
                    + " / \(BalanceScreenText.outgoingColumnLabel) \(BalanceLabels.matchupMultiplierLabel(matchup.outgoing))"
            )
            Text(
                "\(BalanceScreenText.safeColumnLabel) \(BalanceLabels.safeLabel(matchup.safe))"
                    + " / \(BalanceScreenText.superEffectiveColumnLabel) \(BalanceLabels.superEffectiveLabel(matchup.superEffective))"
            )
        }
        .font(TextStyleToken.body.font)
        .foregroundStyle(ColorToken.textPrimary.color)
        .multilineTextAlignment(.leading)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
    }
}
