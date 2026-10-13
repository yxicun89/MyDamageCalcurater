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

/// 結論の1行: 成功 = `PopSymbol.done` / 失敗 = `PopSymbol.close` のアイコン(装飾。名前は文が持つ)+ 従来の文。
private struct AdjustVerdictLine: View {
    let text: String
    let isMet: Bool
    let identifier: String

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            PopIcon(isMet ? PopSymbol.done : PopSymbol.close)
                .foregroundStyle(isMet ? ColorToken.success.color : ColorToken.danger.color)
                .accessibilityHidden(true)
            AdjustResultLine(text: text, identifier: identifier)
        }
    }
}

/// 振り方(能力ポイント)の能力ごとの横棒(装飾。読み上げない。幅 = 能力ポイント / 32)。
private struct AdjustSPBars: View {
    let sp: StatBlock
    private static let barHeight: CGFloat = 8

    private func value(_ stat: StatKey) -> Int {
        switch stat {
        case .hp: return sp.hp
        case .atk: return sp.atk
        case .def: return sp.def
        case .spa: return sp.spa
        case .spd: return sp.spd
        case .spe: return sp.spe
        }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            ForEach(StatKey.allCases, id: \.self) { stat in
                HStack(spacing: SpacingToken.x2) {
                    Text(AdjustText.statLetter(stat))
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .frame(minWidth: SpacingToken.x6, alignment: .leading)
                    GeometryReader { proxy in
                        ZStack(alignment: .leading) {
                            Capsule().fill(ColorToken.tableZebra.color)
                            Capsule().fill(ColorToken.brandPrimary.color)
                                .frame(width: proxy.size.width * AdjustSPBar.fraction(sp: value(stat)))
                        }
                    }
                    .frame(height: Self.barHeight)
                }
            }
        }
        .accessibilityHidden(true)
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
            AdjustSPBars(sp: plan.sp)
            line(AdjustText.planTotal(plan.totalSp))
            line(AdjustText.statsLine(plan.stats))
            line(AdjustText.indexLine(AdjustText.physicalBulkLabel, plan.physicalBulk))
            line(AdjustText.indexLine(AdjustText.specialBulkLabel, plan.specialBulk))
            if showsSpeed { verdict(plan.speedMet ? AdjustText.speedMet : AdjustText.speedNotMet, isMet: plan.speedMet) }
            if goalRequested {
                verdict(AdjustText.goalLine(met: plan.goalMet, chancePercent: plan.chancePercent), isMet: plan.goalMet)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityIdentifier(identifier)
    }

    private func verdict(_ text: String, isMet: Bool) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            PopIcon(isMet ? PopSymbol.done : PopSymbol.close)
                .foregroundStyle(isMet ? ColorToken.success.color : ColorToken.danger.color)
                .accessibilityHidden(true)
            line(text)
        }
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
        AdjustCard(title: AdjustText.resultRegion, systemImage: PopSymbol.calc, identifier: "adjustResultCard") {
            if let outcome = viewModel.outcome {
                if let notice = outcome.unsupportedNotice {
                    PopNoticeView(kind: .info, message: notice, identifier: "adjustUnsupportedNotice")
                }
                indices(outcome.indices)
                if let modeResult = outcome.modeResult { modeResultView(modeResult) }
            } else if viewModel.alertMessage == nil && !viewModel.isLoading {
                PopNoticeView(kind: .empty, message: AdjustText.emptyResultNotice, identifier: "adjustEmptyResult")
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
        PopHelpButton(title: CommonPartsText.help, systemImage: PopSymbol.help, identifier: "adjustIndexNoteHelpButton") {
            AdjustHelpText(text: AdjustText.indexNote, identifier: "adjustIndexNote")
        }
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
            AdjustVerdictLine(text: AdjustText.koLine(ko, hits: hits), isMet: ko.feasible, identifier: "adjustModeResult")
        case .survive(let survive, let hits):
            AdjustVerdictLine(
                text: AdjustText.surviveLine(survive, hits: hits), isMet: survive.feasible, identifier: "adjustModeResult")
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
                PopHelpButton(
                    title: CommonPartsText.help, systemImage: PopSymbol.help, identifier: "adjustMinSpNotRequestedHelpButton"
                ) {
                    AdjustHelpText(text: AdjustText.minSpNotRequested, identifier: "adjustMinSpNotRequested")
                }
            }
        case .goals(let presentation):
            goalsResultView(presentation)
        }
    }

    /// 目標方式の結果(F-11。ADR-0331 §7): 振り方(すべて満たす/一番近い)+ 目標ごとの1行。
    @ViewBuilder
    private func goalsResultView(_ presentation: AdjustGoalsPresentation) -> some View {
        let result = presentation.result
        AdjustResultHeading(text: result.feasible ? AdjustText.goalsPlanHeading : AdjustText.goalsNearestHeading)
        if !result.feasible {
            AdjustResultLine(text: AdjustText.goalsInfeasibleNotice, identifier: "adjustGoalsInfeasible")
        }
        AdjustResultLine(text: AdjustText.planSPLine(result.plan.sp), identifier: "adjustGoalsPlanSP")
        AdjustSPBars(sp: result.plan.sp)
        AdjustResultLine(text: AdjustText.planTotal(result.plan.totalSp), identifier: "adjustGoalsPlanTotal")
        AdjustResultLine(text: AdjustText.statsLine(result.plan.stats), identifier: "adjustGoalsPlanStats")
        AdjustResultLine(text: AdjustText.remainingLine(result.remaining), identifier: "adjustRemaining")
        AdjustResultHeading(text: AdjustText.goalOutcomesLabel)
        ForEach(Array(zip(presentation.snapshots, result.goals).enumerated()), id: \.offset) { index, pair in
            AdjustVerdictLine(
                text: AdjustText.goalOutcomeLine(index + 1, snapshot: pair.0, outcome: pair.1), isMet: pair.1.met,
                identifier: "adjustGoalOutcome-\(index + 1)")
        }
    }
}

/// この技を覚えるポケモン(「覚えるポケモン」を押したときだけ出す。行は種族名だけ)。
struct AdjustLearnersCardView: View {
    let viewModel: AdjustViewModel

    var body: some View {
        if let learners = viewModel.learners {
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                HStack(alignment: .top, spacing: SpacingToken.x2) {
                    PopHeading(
                        title: AdjustText.learnersHeading(learners.moveName), systemImage: PopSymbol.search,
                        identifier: "adjustLearnersHeading")
                    PopIconButton(
                        systemImage: PopSymbol.close, label: AdjustText.learnersClose, identifier: "adjustLearnersClose",
                        minSide: AdjustMetrics.iconButtonSide
                    ) {
                        viewModel.closeLearners()
                    }
                }
                ForEach(learners.species, id: \.key) { species in
                    AdjustResultLine(text: species.nameJa, identifier: "adjustLearnerRow-\(species.key)")
                }
                if learners.species.isEmpty && !learners.isLoading && learners.errorMessage == nil {
                    PopNoticeView(kind: .info, message: AdjustText.learnersEmpty, identifier: "adjustLearnersEmpty")
                }
                if learners.isLoading {
                    PopNoticeView(kind: .loading, message: AdjustText.learnersLoading, identifier: "adjustLearnersLoading")
                }
                if let message = learners.errorMessage {
                    PopNoticeView(kind: .error, message: message, identifier: "adjustLearnersError")
                }
                if learners.canLoadMore && !learners.isLoading {
                    textButton(AdjustText.learnersMore, identifier: "adjustLearnersMore") {
                        viewModel.scheduleLoadMoreLearners()
                    }
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
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .frame(minHeight: CalcScreenMetrics.minimumTapSide)
                .background(ColorToken.tableZebra.color, in: Capsule())
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
    }
}
