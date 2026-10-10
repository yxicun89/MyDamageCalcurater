import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedNeighborhoodSection: 素早さ画面の「自分の周り」パネル(G-04。ADR-0527)。
//
// 自分の結果カードの中(`SpeedResultView` の下)に常時表示する。表(`SpeedTableSection` の LazyVStack)とは独立。
// 切り出しは `SpeedViewModel.neighborhood`(PokeCalcCore)。ここは描くだけ(ロジックを持たない)。
// 色だけに頼らない: 「自分」バッジと ↑ ↓ の文字を持つ。固定高にしない(Dynamic Type・AX5 で折り返す)。
// 識別子: speedNeighborhood / ...Before / ...After / ...Self / ...BeforeTotal / ...AfterTotal / ...Empty。

struct SpeedNeighborhoodSection: View {
    let viewModel: SpeedViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(SpeedLabels.neighborhoodHeading)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .accessibilityAddTraits(.isHeader)
            if let neighborhood = viewModel.neighborhood {
                content(neighborhood)
            } else {
                Text(SpeedLabels.neighborhoodEmpty)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("speedNeighborhoodEmpty")
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(SpeedLabels.neighborhoodRegion)
        .accessibilityIdentifier("speedNeighborhood")
    }

    private func content(_ n: SpeedNeighborhood) -> some View {
        let trick = viewModel.trickRoom
        return VStack(alignment: .leading, spacing: SpacingToken.x2) {
            side(
                tiers: n.before, arrow: SpeedLabels.neighborhoodUpArrow,
                title: trick ? SpeedLabels.neighborhoodBeforeList : SpeedLabels.neighborhoodFasterList,
                total: trick
                    ? SpeedLabels.neighborhoodBeforeTotal(n.beforeTotal) : SpeedLabels.neighborhoodFasterTotal(n.beforeTotal),
                listID: "speedNeighborhoodBefore", totalID: "speedNeighborhoodBeforeTotal")
            selfRow(n)
            side(
                tiers: n.after, arrow: SpeedLabels.neighborhoodDownArrow,
                title: trick ? SpeedLabels.neighborhoodAfterList : SpeedLabels.neighborhoodSlowerList,
                total: trick
                    ? SpeedLabels.neighborhoodAfterTotal(n.afterTotal) : SpeedLabels.neighborhoodSlowerTotal(n.afterTotal),
                listID: "speedNeighborhoodAfter", totalID: "speedNeighborhoodAfterTotal")
        }
    }

    private func side(
        tiers: [SpeedNeighborTier], arrow: String, title: String, total: String, listID: String, totalID: String
    ) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x1) {
                    Text(arrow).accessibilityHidden(true)
                    Text(title)
                }
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                if tiers.isEmpty {
                    Text(SpeedLabels.neighborhoodNone)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                } else {
                    ForEach(tiers) { tierRow($0) }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .contain)
            .accessibilityLabel(title)
            .accessibilityIdentifier(listID)
            Text(total)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityIdentifier(totalID)
        }
    }

    private func tierRow(_ tier: SpeedNeighborTier) -> some View {
        Text("\(tier.speed)  " + tierNames(tier))
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// 自分の行。同速の段があればその名前も並べる。無ければ境界の文言。
    private func selfRow(_ n: SpeedNeighborhood) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
                Text(SpeedLabels.neighborhoodSelfBadge)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .padding(.horizontal, SpacingToken.x2)
                    .padding(.vertical, SpacingToken.x1)
                    .overlay(Capsule().stroke(ColorToken.textPrimary.color, lineWidth: 1))
                Text(String(n.ownSpeed))
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
            if let tie = n.tie {
                Text(SpeedLabels.neighborhoodTie + "  " + tierNames(tie))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .multilineTextAlignment(.leading)
            } else {
                Text(SpeedLabels.neighborhoodBoundary)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.leading)
            }
        }
        .padding(SpacingToken.x2)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.card))
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("speedNeighborhoodSelf")
    }

    private func tierNames(_ tier: SpeedNeighborTier) -> String {
        var text = tier.names.joined(separator: "・")
        if tier.moreCount > 0 { text += "  " + SpeedLabels.neighborhoodMore(tier.moreCount) }
        return text
    }
}
