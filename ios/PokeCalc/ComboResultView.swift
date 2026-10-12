import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ComboResultView / ComboSelfSpeedView: 連続技の節の「結果」と「自分の素早さ」(CB-8。ADR-0535)。
// 結果の文言は `ComboResultDisplay` が作る。ここは描くだけ。

/// 連続技の結果(計算中・失敗・結果)。
struct ComboResultView: View {
    let viewModel: ComboViewModel

    var body: some View {
        switch viewModel.state {
        case .idle:
            EmptyView()
        case .loading:
            HStack(spacing: SpacingToken.x2) {
                ProgressView()
                Text(ComboText.loading)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .accessibilityElement(children: .combine)
            .accessibilityIdentifier("comboLoading")
        case .failed(let message):
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                PopNoticeView(kind: .error, message: message, identifier: "comboError")
                // 性格が引けない案内は、入力を変えるまで同じ結果になるので「もう一度」を出さない。
                if message != ComboText.selfSpeedNatureUnavailable {
                    Button {
                        Task { await viewModel.retry() }
                    } label: {
                        PopLabel(title: ComboText.retryButton, systemImage: PopSymbol.reload)
                    }
                    .buttonStyle(PillButtonStyle(kind: .secondary))
                    .accessibilityLabel(ComboText.retryButton)
                    .accessibilityIdentifier("comboRetry")
                }
            }
        case .loaded(let display):
            loaded(display)
        }
    }

    private func loaded(_ display: ComboResultDisplay) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                ForEach(display.steps) { step in
                    ComboResultStepRow(step: step)
                }
            }
            .accessibilityElement(children: .contain)
            .accessibilityLabel(ComboText.resultStepsListLabel)
            text(display.totalText, id: "comboTotal", style: TextStyleToken.heading)
            ComboTotalBar(fraction: display.totalBarFraction, valueText: display.totalText)
            text(display.koText, id: "comboKO", style: TextStyleToken.body)
            text(display.speedSummaryText, id: "comboSpeedSummary", style: TextStyleToken.body)
            if let line = display.speedLineText {
                text(line, id: "comboSpeedLine", style: TextStyleToken.body)
            }
            if let opponentSpeed = display.opponentSpeedText {
                text(opponentSpeed, id: "comboOpponentSpeed", style: TextStyleToken.body)
            }
            if display.showsSpeedExample, !viewModel.speedExampleLines.isEmpty {
                speedExample
            }
            if let survive = display.surviveText {
                text(survive, id: "comboSurvive", style: TextStyleToken.body)
            }
            if let actions = display.actionsText {
                text(actions, id: "comboActions", style: TextStyleToken.body)
            }
            if let perHit = display.perHitText {
                text(perHit, id: "comboPerHit", style: TextStyleToken.caption)
            }
            if let notice = display.unsupportedNotice {
                text(notice, id: "comboUnsupportedNotice", style: TextStyleToken.caption, secondary: true)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("comboResult")
    }

    private var speedExample: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(ComboText.speedExampleListLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
            ForEach(viewModel.speedExampleLines, id: \.self) { line in
                Text(line)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("comboSpeedExample")
    }

    private func text(_ value: String, id: String, style: TextStyleToken, secondary: Bool = false) -> some View {
        Text(value)
            .font(style.font)
            .foregroundStyle(secondary ? ColorToken.textSecondary.color : ColorToken.textPrimary.color)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityIdentifier(id)
    }
}

/// 結果の1手。技名・チップ・%・行動順を1つの要素にまとめる(読み上げと UI テストが1つの label で読む)。
struct ComboResultStepRow: View {
    let step: ComboStepDisplay

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text("\(step.ordinalText) \(step.moveName)")
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
            ComboRankChipsView(chips: step.chips)
            Text(step.damageText)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
            if let order = step.orderText {
                Text(order)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(step.summaryText)
        .accessibilityIdentifier("comboResultStep-\(step.id)")
    }
}

/// 合計の最大%のバー(0〜100%。100% 超はいっぱい)。装飾で、値は隣の文字が持つ。
struct ComboTotalBar: View {
    let fraction: Double
    let valueText: String

    var body: some View {
        GeometryReader { proxy in
            ZStack(alignment: .leading) {
                Capsule()
                    .fill(ColorToken.tableZebra.color)
                    .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
                Capsule()
                    .fill(ColorToken.brandPrimary.color)
                    .frame(width: max(ComboMetrics.minimumVisibleBarWidth, CGFloat(fraction) * proxy.size.width))
            }
        }
        .frame(height: ComboMetrics.barHeight)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(ComboText.totalBarLabel)
        .accessibilityValue(valueText)
        .accessibilityIdentifier("comboTotalBar")
    }
}

/// 自分の素早さ(SP・性格補正・ランク)。未入力・「計算画面の性格のまま」なら攻撃側の個体の値のまま。
struct ComboSelfSpeedView: View {
    let viewModel: ComboViewModel
    @State private var spText = ""
    @FocusState private var isSPFocused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(ComboText.selfSpeedLegend)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
            ComboField(label: ComboText.selfSpeedSPLabel) { spField }
            ComboField(label: ComboText.selfSpeedNatureLabel) { natureMenu }
            Text(ComboText.selfSpeedNatureNote)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
            ComboField(label: ComboText.selfSpeedRankLabel) {
                RankMeterView(
                    field: "comboSelfSpeed", fieldName: ComboText.selfSpeedRankLabel,
                    statName: ComboText.selfSpeedRankStatName, valueLabel: nil,
                    value: viewModel.selfSpeed.rank ?? viewModel.context?.attacker.ranks.spe ?? 0
                ) { next in
                    Task { await viewModel.setSelfSpeedRank(next) }
                }
            }
        }
        .accessibilityElement(children: .contain)
    }

    private var spField: some View {
        TextField(placeholder, text: $spText)
            .keyboardType(.numbersAndPunctuation)
            .submitLabel(.done)
            .focused($isSPFocused)
            .onSubmit { isSPFocused = false }
            .font(TextStyleToken.body.font)
            .padding(.horizontal, SpacingToken.x3)
            .frame(maxWidth: .infinity, minHeight: ComboMetrics.minTapHeight, alignment: .leading)
            .background(ColorToken.surfaceCard.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                    .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            .onChange(of: spText) { _, newValue in
                let digits = newValue.filter(\.isNumber)
                guard digits == newValue else {
                    spText = digits
                    return
                }
                let value = Int(digits)
                Task {
                    await viewModel.setSelfSpeedSP(value)
                    // 上限へ丸めた値を欄にも映す(要求の値と欄の表示を揃える)。
                    if let applied = viewModel.selfSpeed.sp, value != nil, String(applied) != spText { spText = String(applied) }
                }
            }
            .accessibilityLabel(ComboText.selfSpeedSPLabel)
            .accessibilityIdentifier("comboSelfSpeedSP")
    }

    /// 未入力のとき見える値(攻撃側の個体の素早さの SP)。
    private var placeholder: String {
        viewModel.context.map { String($0.attacker.sp.spe) } ?? ""
    }

    private var natureMenu: some View {
        Menu {
            ForEach(ComboSelfSpeedNature.allCases, id: \.self) { nature in
                Button(ComboText.selfSpeedNatureOption(nature)) {
                    Task { await viewModel.setSelfSpeedNature(nature) }
                }
            }
        } label: {
            ComboPickerLabel(text: ComboText.selfSpeedNatureOption(viewModel.selfSpeed.nature), systemImage: "chevron.up.chevron.down")
        }
        .accessibilityLabel(ComboText.selfSpeedNatureLabel)
        .accessibilityValue(ComboText.selfSpeedNatureOption(viewModel.selfSpeed.nature))
        .accessibilityIdentifier("comboSelfSpeedNature")
    }
}
