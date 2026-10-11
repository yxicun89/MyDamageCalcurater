import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceRecommendationsView: タイプバランスのおすすめタイプ(recommendations。ADR-0415 第3段・§8)。
//
// メンバーだけで決まる(仮想敵は含めない)。防御の穴・攻撃範囲の穴、候補、特性で補えるポケモンを縦並びで出す。
// 長い一覧は `BalanceDisplayLimits.pokemonPerList` 件までにして「ほか N件」と出す。特性名は引けたものだけ名前、
// 引けなければ特性 ID。失敗(overloaded を含む)は日本語の文言と「再計算」ボタン(ユーザー操作で再試行)。
// 構築の中では、候補ごとの該当ポケモンをタイル(画像 + 名前)で出し、押すと構築の空き枠に入る(ADR-0422 §2。`pick`)。

struct BalanceRecommendationsView: View {
    let viewModel: BalanceViewModel
    /// タイルを押したときの入れ先(構築側が渡す)。nil なら従来の文字の一覧。
    var pick: BalancePick?

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
            if let pick {
                BalancePickTilesView(candidate: candidate, number: number, pick: pick)
            } else {
                labeledText(
                    BalanceScreenText.pokemonColumnLabel,
                    pokemonListText(
                        candidate.pokemon.map { $0.nameJa ?? $0.pokemonId }))
            }
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

/// 候補の該当ポケモンのタイル(ADR-0422 §2)。最初の 8 件、残りは「もっと見る」。満杯なら押せず、理由を一言。
/// 画像が無ければタイプ色のエンブレム(`SpeciesImageView`)。並びは応答のまま(exactMatch が先 → pokemonId 昇順)。
struct BalancePickTilesView: View {
    let candidate: BalanceTypeCandidate
    /// 候補の 1 始まりの番号(識別子用。同じポケモンが複数の候補に出るため)。
    let number: Int
    let pick: BalancePick
    @State private var expanded = false
    /// タイルの最小幅(文字の大きさに合わせて広げる。AX5 では 1 列になる)。
    @ScaledMetric(relativeTo: .body) private var tileMinWidth: CGFloat = 132

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(BalanceScreenText.pokemonColumnLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            if candidate.pokemon.isEmpty {
                Text(BalanceScreenText.noneLabel)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            } else {
                // 遅延しない格子(タイルは最大でも候補 5 × 全件。XCUITest がスクロールせずに要素を引ける)。
                BalanceTileColumns(minColumnWidth: tileMinWidth, spacing: SpacingToken.x2) {
                    ForEach(BalancePickTiles.visible(candidate.pokemon, expanded: expanded), id: \.pokemonId) { pokemon in
                        tile(pokemon)
                    }
                }
                if candidate.pokemon.count > BalancePickTiles.visibleCount {
                    Button {
                        expanded.toggle()
                    } label: {
                        PopLabel(
                            title: expanded ? BalanceScreenText.pickShowLess : BalanceScreenText.pickShowMore,
                            systemImage: expanded ? "chevron.up" : "chevron.down")
                    }
                    .buttonStyle(PillButtonStyle(kind: .secondary))
                    .accessibilityIdentifier("balancePickMore-\(number)")
                }
                if !pick.canPick {
                    PopNoticeView(kind: .info, message: BalanceScreenText.pickNoEmptySlot, identifier: "balancePickNoEmptySlot-\(number)")
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(BalanceScreenText.pickGroupLabel(BalanceLabels.typeListText(candidate.types)))
        .accessibilityIdentifier("balancePickTiles-\(number)")
    }

    private func tile(_ pokemon: BalanceCandidatePokemon) -> some View {
        let name = pokemon.nameJa ?? pokemon.pokemonId
        return Button {
            pick.onPick(pokemon.pokemonId)
        } label: {
            HStack(spacing: SpacingToken.x2) {
                SpeciesImageView(speciesKey: pokemon.pokemonId, name: name, types: pokemon.types)
                Text(name)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .multilineTextAlignment(.leading)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
            }
            .padding(SpacingToken.x2)
            .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide, alignment: .leading)
            .background(ColorToken.surfaceCard.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                    .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(!pick.canPick)
        .opacity(pick.canPick ? 1 : CalcScreenMetrics.disabledChipOpacity * 1.5)
        .accessibilityLabel(name)
        .accessibilityIdentifier("balancePickTile-\(number)-\(pokemon.pokemonId)")
    }
}

/// 幅に収まるだけの等幅の列にタイルを並べる(最小幅 `minColumnWidth`。収まらなければ 1 列)。行の高さはその行の最大。
struct BalanceTileColumns: Layout {
    let minColumnWidth: CGFloat
    let spacing: CGFloat

    private func columnCount(for width: CGFloat) -> Int {
        guard width.isFinite, width > 0 else { return 1 }
        return max(1, Int((width + spacing) / (minColumnWidth + spacing)))
    }

    private func rows(_ subviews: Subviews, width: CGFloat) -> (columnWidth: CGFloat, heights: [CGFloat], columns: Int) {
        let columns = columnCount(for: width)
        let columnWidth = width.isFinite ? (width - spacing * CGFloat(columns - 1)) / CGFloat(columns) : minColumnWidth
        var heights: [CGFloat] = []
        for start in stride(from: 0, to: subviews.count, by: columns) {
            let row = subviews[start..<min(start + columns, subviews.count)]
            heights.append(row.map { $0.sizeThatFits(ProposedViewSize(width: columnWidth, height: nil)).height }.max() ?? 0)
        }
        return (columnWidth, heights, columns)
    }

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? minColumnWidth
        let layout = rows(subviews, width: width)
        let height = layout.heights.reduce(0, +) + spacing * CGFloat(max(0, layout.heights.count - 1))
        return CGSize(width: width, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let layout = rows(subviews, width: bounds.width)
        var y = bounds.minY
        for (rowIndex, height) in layout.heights.enumerated() {
            for column in 0..<layout.columns {
                let index = rowIndex * layout.columns + column
                guard index < subviews.count else { break }
                let x = bounds.minX + CGFloat(column) * (layout.columnWidth + spacing)
                subviews[index].place(
                    at: CGPoint(x: x, y: y), anchor: .topLeading,
                    proposal: ProposedViewSize(width: layout.columnWidth, height: height))
            }
            y += height + spacing
        }
    }
}
