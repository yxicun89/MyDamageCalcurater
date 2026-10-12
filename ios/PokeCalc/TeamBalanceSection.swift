import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamBalanceSection: 構築編集の中の「タイプバランス」の節(ADR-0421・ADR-0422・ADR-0532。別画面は廃止)。
//
// 入力は構築編集の下書き(種族を選んだ枠だけ・枠の順。`TeamEditViewModel.balanceParty`)。下書きが変わるたびに
// `BalanceViewModel.setParty` へ渡す(呼び出し・待ち・古い応答の無視は BalanceViewModel の中)。
// balance が使えない・失敗しても、エラーはこの節の中だけに出す(構築の編集・保存は止めない)。再計算のボタンつき。
// おすすめのタイルを押すと、構築の最初の空き枠に入れる(`TeamEditViewModel.addMember`)。満杯なら押せない。

struct TeamBalanceSection: View {
    let balance: BalanceViewModel
    let team: TeamEditViewModel

    private var hasError: Bool {
        balance.masterError != nil || balance.analysisError != nil || balance.coverageError != nil
            || balance.threatsError != nil
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x4) {
            PopHeading(title: BalanceScreenText.screenTitle, systemImage: PopSymbol.balance)
            if team.balanceParty.isEmpty {
                PopNoticeView(kind: .empty, message: BalanceScreenText.teamEmptyNotice, identifier: "teamBalanceEmpty")
            }
            if let error = balance.masterError {
                ErrorBannerView(message: error.message, identifier: "balanceMasterError")
            }
            if hasError {
                retryButton
            }
            if !balance.members.isEmpty {
                BalanceResultsView(viewModel: balance)
            }
            BalanceThreatsView(viewModel: balance)
            if !balance.members.isEmpty {
                BalanceRecommendationsView(
                    viewModel: balance,
                    pick: BalancePick(canPick: team.hasEmptySlot) { speciesKey in
                        Task { await team.addMember(speciesKey: speciesKey) }
                    })
            }
            BalanceMoveRangeView(viewModel: balance)
        }
        .padding(.top, SpacingToken.x4)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("teamBalanceSection")
        .task { await balance.load() }
        // 下書きの種族・特性・技が変わったら送り直す(同じ内容なら BalanceViewModel が何もしない)。
        .task(id: team.balanceParty) { await balance.setParty(team.balanceParty) }
    }

    private var retryButton: some View {
        Button {
            Task { await balance.retry(party: team.balanceParty) }
        } label: {
            PopLabel(title: BalanceScreenText.retryLabel, systemImage: PopSymbol.reload)
        }
        .buttonStyle(PillButtonStyle(kind: .secondary))
        .accessibilityIdentifier("teamBalanceRetry")
    }
}

/// おすすめのタイルを押したときの入れ先(構築側が持つ。バランス側は枠の構造を知らない。ADR-0422 §2)。
struct BalancePick {
    /// 空き枠があるか(無ければタイルは押せない)。
    let canPick: Bool
    let onPick: (String) -> Void
}

// MARK: - 相性の色(ADR-0422 §3。弱点・抜群=赤、耐性・いまひとつ=水色系、無効=灰。緑は使わない)

extension BalanceMatchupTone {
    var color: Color {
        switch self {
        case .weak: return ColorToken.danger.color
        case .resist: return ColorToken.matchupResist.color
        case .immune: return ColorToken.matchupImmune.color
        case .neutral: return ColorToken.textPrimary.color
        }
    }

    /// 色だけに頼らない印(Web の G-05 と同じ形: 弱点=上向き・耐性=下向き・無効=×・等倍=横線)。
    var symbol: String {
        switch self {
        case .weak: return "arrow.up"
        case .resist: return "arrow.down"
        case .immune: return "xmark"
        case .neutral: return "minus"
        }
    }
}

/// 印 + 文字(「×2 弱点」)。印は装飾で読み上げない(文字が意味を持つ)。
struct BalanceToneLabel: View {
    let text: String
    let tone: BalanceMatchupTone

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x1) {
            Image(systemName: tone.symbol)
                .accessibilityHidden(true)
            Text(text)
                .fontWeight(tone == .neutral ? .regular : .semibold)
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
        }
        .foregroundStyle(tone.color)
    }
}
