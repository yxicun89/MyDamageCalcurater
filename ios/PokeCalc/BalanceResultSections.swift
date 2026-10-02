import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceResultSections: タイプバランスの結果(防御相性・チームの集計・攻撃範囲。P6-26。ADR-0505 §6)。
// 値は応答のまま出す(`BalanceResultDisplayBuilder` が整えた文字列・数)。相性・倍率・分類の判定をここに書かない。
// 色だけで表さず、タイプ名と語(「×2 弱点」)も出す。18 タイプの欄は遅延させず・横スクロールにせず、非アクセシビリティサイズでは 2 列・
// アクセシビリティサイズでは 1 列に折り返す。欄は `ViewThatFits` でバッジと語が収まらなければ縦に積む。
// 段(領域・メンバー)は `.contain` で子の識別子を飲み込ませない。**子が 1 つだけの `.contain` は中の識別子を畳むので、必ず見出しなどを同じコンテナに入れて 2 つ以上にする**。

// MARK: - 防御相性

struct BalanceDefenseSection: View {
    let state: BalanceSectionState<BalanceDefenseDisplay>

    var body: some View {
        switch state {
        case .idle:
            EmptyView()
        case .loading:
            BalanceNotice(text: BalanceLabels.loading, identifier: "balanceLoading")
        case .skipped:
            BalanceNotice(text: BalanceLabels.noMembers, identifier: "balanceNoMembers")
        case .failed(let failure):
            ErrorBannerView(message: failure.message, identifier: "balanceDefenseError")
        case .loaded(let display):
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                VStack(alignment: .leading, spacing: SpacingToken.x3) {
                    BalanceSectionTitle(text: BalanceLabels.defenseRegion)
                    ForEach(display.members, id: \.index) { member in
                        BalanceDefenseMemberCard(member: member)
                    }
                }
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("balanceDefenseRegion")

                VStack(alignment: .leading, spacing: SpacingToken.x2) {
                    BalanceSectionTitle(text: BalanceLabels.teamSummaryRegion)
                    ForEach(display.summary, id: \.attackType) { row in
                        BalanceTypeLine(type: row.attackType, text: row.text, identifier: "balanceSummary-\(row.attackType.rawValue)")
                    }
                }
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("balanceSummaryRegion")
            }
        }
    }
}

private struct BalanceDefenseMemberCard: View {
    let member: BalanceDefenseMemberRow

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            BalanceMemberHeader(name: member.name, abilityName: member.abilityName, types: member.types)
            BalanceCellGrid(items: member.cells.map { cell in
                BalanceGridCell(
                    id: cell.attackType.rawValue, type: cell.attackType, text: cell.text, note: cell.abilityNote,
                    identifier: "balanceDefenseCell-\(member.index)-\(cell.attackType.rawValue)",
                    noteIdentifier: "balanceAbilityNote-\(member.index)-\(cell.attackType.rawValue)")
            })
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(member.name)
        .accessibilityIdentifier("balanceDefenseMember-\(member.index)")
    }
}

// MARK: - 攻撃範囲

struct BalanceCoverageSection: View {
    let state: BalanceSectionState<BalanceCoverageDisplay>
    /// メンバーが 0 体(防御が `.skipped`)のときは攻撃範囲の案内を出さない(「ポケモンがいません」だけにする)。
    let defenseState: BalanceSectionState<BalanceDefenseDisplay>

    var body: some View {
        switch state {
        case .idle:
            EmptyView()
        case .loading:
            BalanceNotice(text: BalanceLabels.loading, identifier: "balanceCoverageLoading")
        case .skipped:
            if defenseState != .skipped {
                BalanceNotice(text: BalanceLabels.coverageNoMoves, identifier: "balanceCoverageNoMoves")
            }
        case .failed(let failure):
            ErrorBannerView(message: failure.message, identifier: "balanceCoverageError")
        case .loaded(let display):
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                VStack(alignment: .leading, spacing: SpacingToken.x3) {
                    BalanceSectionTitle(text: BalanceLabels.coverageRegion)
                    ForEach(display.members, id: \.index) { member in
                        BalanceCoverageMemberCard(member: member)
                    }
                }
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("balanceCoverageRegion")

                VStack(alignment: .leading, spacing: SpacingToken.x2) {
                    BalanceSectionTitle(text: BalanceLabels.teamCoverageRegion)
                    ForEach(display.team, id: \.defenseType) { row in
                        BalanceTypeLine(
                            type: row.defenseType, text: "\(row.text)・\(row.membersText)",
                            identifier: "balanceCoverageTeam-\(row.defenseType.rawValue)")
                    }
                }
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("balanceCoverageTeamRegion")
            }
        }
    }
}

private struct BalanceCoverageMemberCard: View {
    let member: BalanceCoverageMemberRow

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            BalanceMemberHeader(name: member.name, abilityName: nil, types: member.attackTypes)
            BalanceCellGrid(items: member.cells.map { cell in
                BalanceGridCell(
                    id: cell.defenseType.rawValue, type: cell.defenseType, text: cell.text, note: nil,
                    identifier: "balanceCoverageCell-\(member.index)-\(cell.defenseType.rawValue)", noteIdentifier: "")
            })
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(member.name)
        .accessibilityIdentifier("balanceCoverageMember-\(member.index)")
    }
}

// MARK: - 部品

/// メンバーの名前(・特性名)とタイプのエンブレム。攻撃範囲では「攻撃タイプ」(技のタイプ)を並べる。
private struct BalanceMemberHeader: View {
    let name: String
    let abilityName: String?
    let types: [PokeType]

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(name)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
            if let abilityName {
                Text(abilityName)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            if !types.isEmpty {
                BalanceBadgeFlow(types: types)
            }
        }
    }
}

/// タイプのエンブレムを左から詰めて並べ、収まらなければ次の行へ(横スクロールにしない)。
private struct BalanceBadgeFlow: View {
    let types: [PokeType]

    var body: some View {
        SpeedFlowLayout {
            ForEach(types, id: \.self) { type in
                TypeBadgeView(type: type)
            }
        }
    }
}

/// 1 つの欄(防御タイプ/攻撃タイプと語。特性の添え書きは別の要素)。
private struct BalanceGridCell: Identifiable {
    let id: String
    let type: PokeType
    let text: String
    let note: String?
    let identifier: String
    let noteIdentifier: String
}

/// 18 タイプの欄。通常は 2 列・アクセシビリティサイズは 1 列(`Grid` は遅延しない)。
private struct BalanceCellGrid: View {
    let items: [BalanceGridCell]
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    private var columns: Int { dynamicTypeSize.isAccessibilitySize ? 1 : 2 }

    var body: some View {
        let rows = stride(from: 0, to: items.count, by: columns).map { Array(items[$0..<min($0 + columns, items.count)]) }
        Grid(alignment: .topLeading, horizontalSpacing: SpacingToken.x2, verticalSpacing: SpacingToken.x2) {
            ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                GridRow {
                    ForEach(row) { item in
                        BalanceCellView(item: item)
                    }
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

private struct BalanceCellView: View {
    let item: BalanceGridCell

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            BalanceTypeLine(type: item.type, text: item.text, identifier: item.identifier)
            if let note = item.note {
                BalanceNotice(text: note, identifier: item.noteIdentifier)
            }
        }
        .frame(maxWidth: .infinity, alignment: .topLeading)
    }
}

/// タイプのエンブレムと語を 1 行(収まらなければ縦)に並べる。1 要素(label にタイプ名と語)。
private struct BalanceTypeLine: View {
    let type: PokeType
    let text: String
    let identifier: String

    var body: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: SpacingToken.x2) {
                TypeBadgeView(type: type)
                label
            }
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                TypeBadgeView(type: type)
                label
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(PokeTypeLabel.japaneseName(for: type)) \(text)")
        .accessibilityIdentifier(identifier)
    }

    private var label: some View {
        Text(text)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
    }
}
