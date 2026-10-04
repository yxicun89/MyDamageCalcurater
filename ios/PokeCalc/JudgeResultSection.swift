import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// JudgeResultSection: 判定の結果(P6-25。ADR-0504 §6)。
// 値は judge の応答のまま出し、「勝ち」「負け」に丸めない。未対応の印は方向ごとに分けて出す
// (全候補に共通する印は結果の上に 1 回・一部の候補だけの印はその行に。その方向の確定数を確定として見せない添え書きは行ごと)。
// 行は `LazyVStack` ではなく `VStack`(全行をアクセシビリティの木に出す)。行は `.contain` で子の識別子を飲み込ませない。
// 結果の `.contain`(judgeResult)は見出しも含める: 子が行 1 つだけだと外側に畳まれて行の識別子(judgeRow-N)が消えるため。

struct JudgeResultSection: View {
    let viewModel: JudgeViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            switch viewModel.resultState {
            case .idle:
                JudgeSectionTitle(text: JudgeLabels.resultRegion)
                JudgeNotice(text: JudgeLabels.emptyResult, identifier: "judgeEmptyResult")
            case .loading:
                JudgeSectionTitle(text: JudgeLabels.resultRegion)
                JudgeNotice(text: JudgeLabels.loading, identifier: "judgeLoading")
            case .failed(let failure):
                JudgeSectionTitle(text: JudgeLabels.resultRegion)
                ErrorBannerView(message: failure.message, identifier: "judgeError")
                if let hint = failure.candidateHint {
                    JudgeNotice(text: hint, identifier: "judgeErrorCandidate")
                }
            case .loaded(let display):
                loaded(display)
            }
        }
    }

    private func loaded(_ display: JudgeResultDisplay) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            JudgeSectionTitle(text: JudgeLabels.resultRegion)
            if let summary = display.attackerUnsupportedSummary {
                JudgeNotice(text: summary, identifier: "judgeAttackerUnsupportedSummary")
            }
            if let summary = display.defenderUnsupportedSummary {
                JudgeNotice(text: summary, identifier: "judgeDefenderUnsupportedSummary")
            }
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                ForEach(display.rows) { row in
                    JudgeRowView(row: row)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("judgeResult")
    }
}

/// 結果の 1 行(相手候補 1 件)。
private struct JudgeRowView: View {
    let row: JudgeMatchupDisplay

    var body: some View {
        let index = row.defenderIndex
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(row.candidateLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .multilineTextAlignment(.leading)
                .accessibilityIdentifier("judgeRowLabel-\(index)")
            HStack(spacing: SpacingToken.x2) {
                SpeedEmblem(name: row.nameJa, primaryType: row.primaryType ?? "")
                line(row.nameJa, style: .heading, id: "judgeRowName-\(index)")
            }
            line(row.speedText, id: "judgeRowSpeed-\(index)")
            line(row.priorityText, id: "judgeRowPriority-\(index)")
            line(row.speedComparisonText, id: "judgeRowSpeedComparison-\(index)")
            speedNote(row.attackerSpeedAppliedText, id: "judgeRowAttackerSpeedApplied-\(index)")
            speedNote(row.defenderSpeedAppliedText, id: "judgeRowDefenderSpeedApplied-\(index)")
            speedNote(row.attackerSpeedIgnoredText, id: "judgeRowAttackerSpeedIgnored-\(index)")
            speedNote(row.defenderSpeedIgnoredText, id: "judgeRowDefenderSpeedIgnored-\(index)")
            line(row.turnOrderText, id: "judgeRowTurnOrder-\(index)")
            direction(
                ko: row.attackerKoText, unreliable: row.attackerKoUnreliableText, note: row.attackerUnsupportedNote,
                ids: (ko: "judgeRowAttackerKo-\(index)", unreliable: "judgeRowAttackerKoUnreliable-\(index)", note: "judgeRowAttackerNote-\(index)"))
            direction(
                ko: row.defenderKoText, unreliable: row.defenderKoUnreliableText, note: row.defenderUnsupportedNote,
                ids: (ko: "judgeRowDefenderKo-\(index)", unreliable: "judgeRowDefenderKoUnreliable-\(index)", note: "judgeRowDefenderNote-\(index)"))
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("judgeRow-\(index)")
    }

    /// 素早さに反映した補正・反映していない入力の文(空なら出さない。識別子も付けない。ADR-0512)。
    @ViewBuilder
    private func speedNote(_ text: String?, id: String) -> some View {
        if let text {
            line(text, style: .caption, id: id)
        }
    }

    /// 1 方向の確定数と、その方向だけの添え書き・注記(もう片方の方向には影響しない)。
    @ViewBuilder
    private func direction(
        ko: String, unreliable: String?, note: String?, ids: (ko: String, unreliable: String, note: String)
    ) -> some View {
        line(ko, id: ids.ko)
        if let unreliable {
            line(unreliable, style: .caption, id: ids.unreliable)
        }
        if let note {
            line(note, style: .caption, id: ids.note)
        }
    }

    private enum LineStyle { case heading, body, caption }

    private func line(_ text: String, style: LineStyle = .body, id: String) -> some View {
        Text(text)
            .font(font(style))
            .foregroundStyle(style == .caption ? ColorToken.textSecondary.color : ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(id)
    }

    private func font(_ style: LineStyle) -> Font {
        switch style {
        case .heading: return TextStyleToken.heading.font
        case .body: return TextStyleToken.body.font
        case .caption: return TextStyleToken.caption.font
        }
    }
}
