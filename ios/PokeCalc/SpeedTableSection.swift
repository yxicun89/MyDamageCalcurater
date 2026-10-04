import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedTableSection: 素早さの表(絞り込み・場の状態・段。P6-24。ADR-0503 §6・§7)。
// 段は `LazyVStack` で遅延描画する(I-speed-2 / F-06。ADR-0517。画面付近の段だけを作る)。段は `.contain` で子の識別子を飲み込ませない。
// 全段は木に出ないので、総数と自分の位置は表の見出しの読み上げ(`speedTablePosition`)で補う。

struct SpeedTableSection: View {
    let viewModel: SpeedViewModel
    /// 行の id へスクロールする(外側の ScrollView の `ScrollViewProxy`。アニメーションしない)。
    let scrollToRow: (String) -> Void
    /// いま描かれている行の添字(`LazyVStack` の行の onAppear / onDisappear で更新)。
    @State private var renderedRows: Set<Int> = []

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text(SpeedLabels.tableRegion)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
            filterGroup
            fieldGroup
            positionSummary
            tableBody
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("speedTable")
    }

    private var filterGroup: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedPillGroup(title: SpeedLabels.filterGroup) {
                ForEach(SpeedPresetID.allCases, id: \.self) { id in
                    SpeedPill(
                        title: SpeedLabels.preset(id), isSelected: viewModel.selectedFilterPresets.contains(id),
                        identifier: "speedFilter-\(id.rawValue)"
                    ) { viewModel.toggleFilterPreset(id) }
                }
            }
            if viewModel.filterMinimumNoticeVisible {
                Text(SpeedLabels.filterMinimumNotice)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.leading)
                    .accessibilityIdentifier("speedFilterMinimumNotice")
            }
        }
    }

    private var fieldGroup: some View {
        SpeedPillGroup(title: SpeedLabels.fieldGroup) {
            SpeedPill(title: SpeedLabels.tableTailwind, isSelected: viewModel.tableTailwind, identifier: "speedTableTailwind") {
                viewModel.setTableTailwind(!viewModel.tableTailwind)
            }
            SpeedPill(title: SpeedLabels.trickRoom, isSelected: viewModel.trickRoom, identifier: "speedTrickRoom") {
                viewModel.setTrickRoom(!viewModel.trickRoom)
            }
        }
    }

    /// 総段数と自分の位置の読み上げ。自分の行が画面外のときは、そこへ移るボタンを添える。
    @ViewBuilder
    private var positionSummary: some View {
        let rows = viewModel.tableRows
        if let summary = SpeedTableNavigation.summary(rows: rows) {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(summary)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.leading)
                    .accessibilityIdentifier("speedTablePosition")
                if let selfIndex = SpeedTableNavigation.selfRowIndex(in: rows),
                    let direction = SpeedTableNavigation.direction(selfIndex: selfIndex, visible: renderedRows),
                    direction != .visible
                {
                    SpeedPill(
                        title: SpeedTableNavigation.jumpLabel(direction), isSelected: false, identifier: "speedJumpToSelf"
                    ) { scrollToRow(rows[selfIndex].id) }
                }
            }
        }
    }

    @ViewBuilder
    private var tableBody: some View {
        switch viewModel.tableState {
        case .loading:
            Text(SpeedLabels.loading)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityIdentifier("speedTableLoading")
        case .failed(let failure):
            ErrorBannerView(message: failure.message, identifier: "speedTableError")
        case .loaded:
            LazyVStack(alignment: .leading, spacing: SpacingToken.x2) {
                ForEach(Array(viewModel.tableRows.enumerated()), id: \.element.id) { index, row in
                    Group {
                        switch row {
                        case .tier(let tier): SpeedTierView(tier: tier)
                        case .selfBoundary: SpeedBoundaryView()
                        }
                    }
                    .id(row.id)
                    .onAppear { renderedRows.insert(index) }
                    .onDisappear { renderedRows.remove(index) }
                }
            }
        }
    }
}

/// 表の 1 段(同じ実数値)。2 行以上なら同速のバッジ、自分と同じ段なら強調する。
private struct SpeedTierView: View {
    let tier: SpeedTierDisplay
    private static let selfBorderWidth: CGFloat = 2

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedFlowLayout {
                Text(tier.speedLabel)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                if tier.isTie { badge(SpeedLabels.tie, id: "speedTierTie-\(tier.speed)") }
                if tier.isSelf { badge(SpeedLabels.selfTier, id: "speedTierSelf-\(tier.speed)") }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            ForEach(tier.entries) { entry in
                HStack(spacing: SpacingToken.x2) {
                    SpeedEmblem(name: entry.nameJa, primaryType: entry.primaryType)
                    Text(entry.label)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            ColorToken.bgGlass.color,
            in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
        )
        .overlay(
            RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                .stroke(
                    tier.isSelf ? ColorToken.textPrimary.color : ColorToken.borderHairline.color,
                    lineWidth: tier.isSelf ? Self.selfBorderWidth : CalcScreenMetrics.hairlineBorderWidth)
        )
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("speedTier-\(tier.speed)")
    }

    private func badge(_ text: String, id: String) -> some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .multilineTextAlignment(.leading)
            .padding(.horizontal, SpacingToken.x2)
            .padding(.vertical, SpacingToken.x1)
            .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            .accessibilityIdentifier(id)
    }
}

/// 自分が入る位置の境界線(自分と同じ実数値の段が無いときだけ)。
private struct SpeedBoundaryView: View {
    private static let lineHeight: CGFloat = 2

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Rectangle()
                .fill(ColorToken.textPrimary.color)
                .frame(height: Self.lineHeight)
            Text(SpeedLabels.selfBoundary)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("speedBoundary")
    }
}
