import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceScreenView: タイプバランス画面(P6-26。ADR-0505)。
//
// ロジックは持たない。`BalanceViewModel`(PokeCalcCore)の状態を描き、操作をメソッドへつなぐだけ(ADR-0500 §1)。
// 縦の 1 本のスクロール: 構築を選ぶ → 防御相性(メンバーごと・チームの集計)→ 攻撃範囲。
// 構築は行のボタンで選ぶ(`Menu` は使わない)。選ぶと解析する。相性・倍率・弱点の判定は持たず、応答を並べるだけ。
// 色を持つのはタイプのエンブレム(`TypeBadgeView`)だけ。常時動くアニメーションは入れない。

struct BalanceScreenView: View {
    @State private var viewModel: BalanceViewModel
    /// 構築の一覧を読み終えたか(読む前に「構築がありません」を一瞬出さない)。
    @State private var hasLoaded = false

    init(service: any BalanceService, master: any PokeCalcService, teamStore: any TeamStore) {
        _viewModel = State(initialValue: BalanceViewModel(service: service, master: master, teamStore: teamStore))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                Text(BalanceLabels.screenTitle)
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                teamSection
                if viewModel.selectedTeamID == nil, hasLoaded, !viewModel.teamOptions.isEmpty {
                    BalanceNotice(text: BalanceLabels.selectPrompt, identifier: "balanceSelectPrompt")
                }
                if viewModel.selectedTeamID != nil {
                    BalanceDefenseSection(state: viewModel.defenseState)
                    BalanceCoverageSection(state: viewModel.coverageState, defenseState: viewModel.defenseState)
                    if hasFailure {
                        retryButton
                    }
                }
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("balanceScreen")
        .task {
            await viewModel.load()
            hasLoaded = true
        }
        .onDisappear { viewModel.cancelPendingWork() }
    }

    // MARK: - 構築

    @ViewBuilder
    private var teamSection: some View {
        if viewModel.teamLoadFailed {
            ErrorBannerView(message: BalanceLabels.teamLoadFailed, identifier: "balanceTeamLoadError")
        } else if viewModel.teamOptions.isEmpty {
            if hasLoaded {
                BalanceNotice(text: BalanceLabels.noTeams, identifier: "balanceTeamEmptyMessage")
            }
        } else {
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                BalanceSectionTitle(text: BalanceLabels.teamRegion)
                ForEach(Array(viewModel.teamOptions.enumerated()), id: \.element.id) { index, option in
                    BalanceTeamRow(option: option, index: index, isSelected: option.id == viewModel.selectedTeamID) {
                        viewModel.selectTeam(id: option.id)
                    }
                }
            }
            .accessibilityElement(children: .contain)
            .accessibilityIdentifier("balanceTeamList")
        }
    }

    // MARK: - 再試行

    private var hasFailure: Bool {
        if case .failed = viewModel.defenseState { return true }
        if case .failed = viewModel.coverageState { return true }
        return false
    }

    private var retryButton: some View {
        Button {
            Task { await viewModel.reanalyze() }
        } label: {
            Text(BalanceLabels.retry)
                .multilineTextAlignment(.leading)
        }
        .buttonStyle(PillButtonStyle())
        .accessibilityIdentifier("balanceRetry")
    }
}

/// 構築の 1 行(構築名・メンバー数)。選択中は反転する(`isSelected` も付ける)。タップ範囲は行全体。
private struct BalanceTeamRow: View {
    let option: BalanceTeamOption
    let index: Int
    let isSelected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(option.name)
                    .font(TextStyleToken.body.font)
                    .multilineTextAlignment(.leading)
                Text(BalanceLabels.memberCount(option.memberCount))
                    .font(TextStyleToken.caption.font)
                    .multilineTextAlignment(.leading)
            }
            .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(SpacingToken.x3)
            .background(
                RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                    .fill(isSelected ? ColorToken.textPrimary.color : ColorToken.bgGlass.color)
            )
            .overlay(
                RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                    .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(option.name) \(BalanceLabels.memberCount(option.memberCount))")
        .accessibilityIdentifier("balanceTeam-\(index)")
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

/// 見出し。
struct BalanceSectionTitle: View {
    let text: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.heading.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// 補足の案内(折り返す)。
struct BalanceNotice: View {
    let text: String
    let identifier: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(identifier)
    }
}
