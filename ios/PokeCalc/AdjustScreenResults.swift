import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenResults: 調整の結果と「この技を覚えるポケモン」の一覧(AJ7。ADR-0502 §2・§7・§8)。
// 結果は差し替えるだけでアニメーションしない。長い行は折り返す(AX5 で横にはみ出さない)。

/// 結果の1行(折り返す)。
private struct AdjustResultLine: View {
    let text: String
    let identifier: String
    var isCaption = false

    var body: some View {
        Text(text)
            .font(isCaption ? TextStyleToken.caption.font : TextStyleToken.body.font)
            .foregroundStyle(isCaption ? ColorToken.textSecondary.color : ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(identifier)
    }
}

/// 結果の小見出し。
private struct AdjustResultHeading: View {
    let text: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.body.font.weight(.semibold))
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityAddTraits(.isHeader)
    }
}

/// 配分の1案(1つの要素にまとめて読ませる)。
private struct AdjustPlanView: View {
    let plan: AdjustAllocPlan
    let goalRequested: Bool
    let showsSpeed: Bool
    let identifier: String

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            line(AdjustText.planSPLine(plan.sp))
            line(AdjustText.planTotal(plan.totalSp))
            line(AdjustText.statsLine(plan.stats))
            line(AdjustText.indexLine(AdjustText.physicalBulkLabel, plan.physicalBulk))
            line(AdjustText.indexLine(AdjustText.specialBulkLabel, plan.specialBulk))
            if showsSpeed { line(plan.speedMet ? AdjustText.speedMet : AdjustText.speedNotMet) }
            if goalRequested { line(AdjustText.goalLine(met: plan.goalMet, chancePercent: plan.chancePercent)) }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier(identifier)
    }

    private func line(_ text: String) -> some View {
        Text(text)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// 調整の結果: 指数と 16n + モードの結果 + 未対応の印。
struct AdjustResultCardView: View {
    let viewModel: AdjustViewModel

    var body: some View {
        AdjustCard(title: AdjustText.resultRegion, identifier: "adjustResultCard") {
            if let outcome = viewModel.outcome {
                if let notice = outcome.unsupportedNotice {
                    AdjustResultLine(text: notice, identifier: "adjustUnsupportedNotice", isCaption: true)
                }
                indices(outcome.indices)
                if let modeResult = outcome.modeResult { modeResultView(modeResult) }
            } else if viewModel.alertMessage == nil && !viewModel.isLoading {
                AdjustResultLine(text: AdjustText.emptyResultNotice, identifier: "adjustEmptyResult", isCaption: true)
            }
        }
    }

    @ViewBuilder
    private func indices(_ result: AdjustIndicesResult) -> some View {
        AdjustResultHeading(text: AdjustText.indicesHeading)
        AdjustResultLine(text: AdjustText.statsLine(result.stats), identifier: "adjustStatsLine")
        AdjustResultLine(
            text: result.firepowerIndex.map { AdjustText.indexLine(AdjustText.firepowerIndexLabel, $0) }
                ?? AdjustText.firepowerIndexNone,
            identifier: "adjustFirepowerIndex")
        AdjustResultLine(
            text: AdjustText.indexLine(AdjustText.physicalBulkLabel, result.physicalBulkIndex), identifier: "adjustPhysicalBulk")
        AdjustResultLine(
            text: AdjustText.indexLine(AdjustText.specialBulkLabel, result.specialBulkIndex), identifier: "adjustSpecialBulk")
        AdjustResultLine(text: AdjustText.indexNote, identifier: "adjustIndexNote", isCaption: true)
        AdjustResultHeading(text: AdjustText.hpLineHeading)
        AdjustResultLine(text: AdjustText.hpCurrent(result.hpLines), identifier: "adjustHPCurrent")
        ForEach(Array(AdjustText.hpLinePoints(result.hpLines).enumerated()), id: \.offset) { index, text in
            AdjustResultLine(text: text, identifier: "adjustHPLine-\(index)")
        }
    }

    @ViewBuilder
    private func modeResultView(_ result: AdjustModeResult) -> some View {
        switch result {
        case .ko(let ko, let hits):
            AdjustResultLine(text: AdjustText.koLine(ko, hits: hits), identifier: "adjustModeResult")
        case .survive(let survive, let hits):
            AdjustResultLine(text: AdjustText.surviveLine(survive, hits: hits), identifier: "adjustModeResult")
        case .allocation(let allocation, let mode, let goalRequested, let speedTargetRequested):
            let showsSpeed = mode == .offense && speedTargetRequested
            AdjustResultLine(text: AdjustText.remainingLine(allocation.remaining), identifier: "adjustRemaining")
            AdjustResultHeading(text: AdjustText.maxIndexHeading)
            AdjustPlanView(
                plan: allocation.maxIndex, goalRequested: goalRequested, showsSpeed: showsSpeed, identifier: "adjustMaxIndexPlan")
            if let minSp = allocation.minSp {
                AdjustResultHeading(text: AdjustText.minSpHeading)
                AdjustPlanView(plan: minSp, goalRequested: goalRequested, showsSpeed: showsSpeed, identifier: "adjustMinSpPlan")
            } else if !goalRequested {
                AdjustResultLine(text: AdjustText.minSpNotRequested, identifier: "adjustMinSpNotRequested", isCaption: true)
            }
        }
    }
}

/// この技を覚えるポケモン(「覚えるポケモン」を押したときだけ出す。行は種族名だけ)。
struct AdjustLearnersCardView: View {
    let viewModel: AdjustViewModel

    var body: some View {
        if let learners = viewModel.learners {
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                Text(AdjustText.learnersHeading(learners.moveName))
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityAddTraits(.isHeader)
                    .accessibilityIdentifier("adjustLearnersHeading")
                ForEach(learners.species, id: \.key) { species in
                    AdjustResultLine(text: species.nameJa, identifier: "adjustLearnerRow-\(species.key)")
                }
                if learners.species.isEmpty && !learners.isLoading && learners.errorMessage == nil {
                    AdjustResultLine(text: AdjustText.learnersEmpty, identifier: "adjustLearnersEmpty", isCaption: true)
                }
                if learners.isLoading {
                    ProgressView(AdjustText.learnersLoading)
                }
                if let message = learners.errorMessage {
                    Text(message)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.danger.color)
                        .multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .accessibilityIdentifier("adjustLearnersError")
                }
                if learners.canLoadMore && !learners.isLoading {
                    textButton(AdjustText.learnersMore, identifier: "adjustLearnersMore") {
                        viewModel.scheduleLoadMoreLearners()
                    }
                }
                textButton(AdjustText.learnersClose, identifier: "adjustLearnersClose") {
                    viewModel.closeLearners()
                }
            }
            .padding(SpacingToken.x3)
            .frame(maxWidth: .infinity, alignment: .leading)
            .popCard()
            .accessibilityElement(children: .contain)
            .accessibilityLabel(AdjustText.learnersRegion)
            .accessibilityIdentifier("adjustLearnersCard")
        }
    }

    private func textButton(_ title: String, identifier: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.tableZebra.color, in: Capsule())
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
    }
}
