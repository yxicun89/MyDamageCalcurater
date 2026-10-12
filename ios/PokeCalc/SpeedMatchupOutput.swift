import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedMatchupOutput: 素早さ画面の出力(I-speed-4c。ADR-0610 追記)。
// 出力 1「上を取る最低ライン」(想定ごとのカード)と出力 2「これ以上」(表の 6 プリセットごとの族の区切り)。
// 状態の描き分け(案内 / 読み込み中 / 失敗 / 結果)だけを持ち、語の選び方は `SpeedMatchupDisplayBuilder` が決める。
// 固定高にしない。届かない・同速・行動順は文字で言う(色だけに頼らない)。

private struct SpeedNote: View {
    let text: String
    let identifier: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(identifier)
    }
}

/// 出力 1: 想定ごとのカード。
struct SpeedMatchupSection: View {
    let viewModel: SpeedMatchupViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: SpeedMatchupLabels.matchupRegion, systemImage: PopSymbol.speed)
            content
        }
        .accessibilityElement(children: .contain)
    }

    @ViewBuilder
    private var content: some View {
        switch viewModel.matchupState {
        case .idle:
            SpeedNote(text: SpeedMatchupLabels.matchupHint, identifier: "speedMatchupHint")
        case .loading:
            SpeedNote(text: SpeedMatchupLabels.matchupLoading, identifier: "speedMatchupLoading")
        case .failed(let failure):
            ErrorBannerView(message: failure.message, identifier: "speedMatchupError")
        case .loaded(let matchup):
            if let speed = matchup.selfSpeed {
                SpeedNote(text: SpeedLabels.selfSpeed(speed), identifier: "speedSelfSpeed")
            }
            ForEach(viewModel.matchupCards) { card in
                SpeedMatchupCard(card: card)
            }
        }
    }
}

/// 想定 1 つぶんのカード: 見出し・相手の素早さ・性格ごとの行・同速・行動順のバッジ。
private struct SpeedMatchupCard: View {
    let card: SpeedMatchupCardDisplay

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(card.assumptionLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .multilineTextAlignment(.leading)
            Text(card.headline)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityIdentifier("speedCardHeadline-\(card.id)")
            Text(card.opponentSpeedLabel)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityIdentifier("speedCardOpponentSpeed-\(card.id)")
            if let badge = card.orderBadge {
                orderBadge(badge)
            }
            ForEach(card.rows) { row in
                natureRow(row)
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("speedCard-\(card.id)")
    }

    /// 行動順。記号と文字の両方で伝える(色は使わない)。
    private func orderBadge(_ text: String) -> some View {
        HStack(spacing: SpacingToken.x1) {
            Image(systemName: Self.symbol(card.order))
                .accessibilityHidden(true)
            Text(text)
                .multilineTextAlignment(.leading)
        }
        .font(TextStyleToken.caption.font)
        .foregroundStyle(ColorToken.textPrimary.color)
        .padding(.horizontal, SpacingToken.x3)
        .padding(.vertical, SpacingToken.x1)
        .background(ColorToken.tableZebra.color, in: Capsule())
        .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier("speedCardOrder-\(card.id)")
    }

    private static func symbol(_ order: SpeedTurnOrder?) -> String {
        switch order {
        case .selfFirst: return "arrow.up"
        case .opponentFirst: return "arrow.down"
        case .speedTie, .none: return "equal"
        }
    }

    private func natureRow(_ row: SpeedNatureRowDisplay) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            VStack(alignment: .leading, spacing: 0) {
                Text(row.natureLabel)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                Text(row.text)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .combine)
            .accessibilityIdentifier("speedCardNature-\(card.id)-\(row.id)")
            if let tie = row.tieText {
                Text(tie)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("speedCardTie-\(card.id)-\(row.id)")
            }
        }
    }
}

/// 出力 2:「これ以上」。今の振りを入れたときだけ出る(入れていなければ案内)。
struct SpeedReachSection: View {
    let viewModel: SpeedMatchupViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: SpeedMatchupLabels.reachRegion, systemImage: PopSymbol.speed)
            content
        }
        .accessibilityElement(children: .contain)
    }

    @ViewBuilder
    private var content: some View {
        switch viewModel.reachState {
        case .idle:
            SpeedNote(text: SpeedMatchupLabels.reachHint, identifier: "speedReachHint")
        case .loading:
            SpeedNote(text: SpeedMatchupLabels.reachLoading, identifier: "speedReachLoading")
        case .failed(let failure):
            ErrorBannerView(message: failure.message, identifier: "speedReachError")
        case .loaded:
            VStack(alignment: .leading, spacing: SpacingToken.x3) {
                ForEach(viewModel.reachRows) { row in
                    SpeedReachRow(row: row)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityElement(children: .contain)
            .accessibilityIdentifier("speedReach")
        }
    }
}

/// 表の 1 プリセットぶん: 「a 族まで抜ける・b 族と同速・c 族以上に抜かれる」、各区間の数、境界の例。
private struct SpeedReachRow: View {
    let row: SpeedReachRowDisplay

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            line(row.slowerText, count: row.slowerCountLabel, example: row.slowerExample, id: "speedReachSlower-\(row.id)")
            if let tie = row.tieText {
                line(tie, count: row.tieCountLabel, example: nil, id: "speedReachTie-\(row.id)")
            }
            line(row.fasterText, count: row.fasterCountLabel, example: row.fasterExample, id: "speedReachFaster-\(row.id)")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("speedReachRow-\(row.id)")
    }

    private func line(_ text: String, count: String, example: String?, id: String) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(text)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
            Text([count, example].compactMap { $0 }.joined(separator: SpeedLabels.summarySeparator))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
        .multilineTextAlignment(.leading)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier(id)
    }
}
