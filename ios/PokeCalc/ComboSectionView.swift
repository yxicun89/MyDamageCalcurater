import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ComboSectionView: 計算画面の「連続技」の節(CB-8。ADR-0535・ADR-0501「連続技」)。
// 結果の下に置く。ロジックは `ComboViewModel`(PokeCalcCore)にあり、ここは描いて操作をつなぐだけ(ADR-0500 §1)。
// 計算画面の値(攻撃側・技・場・防御側の振り)は `CalcViewModel.comboContext` で受け取り、別の入力欄は持たない(UC-5)。
// 失敗はこの節の中だけに出す(単発の結果・計算画面のエラー表示は変えない。絶対ルール5)。

struct ComboSectionView: View {
    let calc: CalcViewModel
    @State private var viewModel: ComboViewModel
    @State private var isAddSheetPresented = false
    @State private var isOpponentSheetPresented = false
    @State private var isSpeedFieldExpanded = false
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    init(calc: CalcViewModel, combo: any ComboCalcService, master: any PokeCalcService, speed: (any SpeedService)?) {
        self.calc = calc
        _viewModel = State(initialValue: ComboViewModel(comboService: combo, master: master, speedService: speed))
    }

    var body: some View {
        // 節が無いときも `.task` を持つ入れ物(結果の行が出たら節を出す)。
        VStack(alignment: .leading, spacing: 0) {
            if viewModel.isVisible {
                section
            }
        }
        .task(id: calc.comboContext) { await viewModel.updateContext(calc.comboContext) }
    }

    private var section: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text(ComboText.sectionLabel)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
            if viewModel.isExpanded {
                expandedContent
            } else {
                openButton
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.input)
        // `.contain`: 節を1つの要素として見つけられるようにしつつ、中の各入力は個別の要素のままにする
        // (`CalcConditionsSection.conditionsPanel` と同じ理由)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("comboSection")
    }

    private var openButton: some View {
        Button {
            Task { await viewModel.open() }
        } label: {
            PopLabel(title: ComboText.addStepButton, systemImage: "plus")
        }
        .buttonStyle(PillButtonStyle(kind: .secondary))
        .accessibilityLabel(ComboText.addStepButton)
        .accessibilityIdentifier("comboOpenButton")
    }

    @ViewBuilder
    private var expandedContent: some View {
        stepsList
        addStepControl
        defenderControl
        opponentMoveControl
        ComboToggleButton(
            title: ComboText.opponentSpeedFromDefender, isOn: viewModel.opponentSpeedFromDefender,
            identifier: "comboOpponentSpeedToggle"
        ) {
            Task { await viewModel.setOpponentSpeedFromDefender(!viewModel.opponentSpeedFromDefender) }
        }
        ComboSelfSpeedView(viewModel: viewModel)
        speedFieldControl
        ComboResultView(viewModel: viewModel)
        Text(ComboText.note)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityIdentifier("comboNote")
        if viewModel.canClose {
            Button {
                viewModel.close()
            } label: {
                PopLabel(title: ComboText.closeButton, systemImage: "xmark")
            }
            .buttonStyle(PillButtonStyle(kind: .secondary))
            .accessibilityLabel(ComboText.closeButton)
            .accessibilityIdentifier("comboCloseButton")
        }
    }

    // MARK: 手

    private var stepsList: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(Array(viewModel.entries.enumerated()), id: \.element.id) { index, entry in
                stepRow(index: index, entry: entry)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(ComboText.stepsListLabel)
    }

    private func stepRow(index: Int, entry: ComboEntry) -> some View {
        let move = viewModel.move(for: entry)
        let name = move?.nameJa ?? ""
        let isMain = entry == .main
        let count = viewModel.entries.count
        return VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(ComboText.stepOrdinal(index))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            Text(name)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
            if isMain {
                Text(ComboText.mainStepBadge)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if let move {
                ComboRankChipsView(chips: ComboRankChips.chips(for: move))
            }
            if move?.category != .status {
                criticalToggle(index: index, entry: entry)
            }
            stepControls(index: index, name: name, isMain: isMain, isFirst: index == 0, isLast: index == count - 1)
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("comboStep-\(index)")
    }

    /// メインの急所は計算画面の「急所」、追加の手は手ごと。
    private func criticalToggle(index: Int, entry: ComboEntry) -> some View {
        let isOn: Bool
        let action: () -> Void
        switch entry {
        case .main:
            isOn = calc.isCritical
            action = { calc.scheduleLatest { await $0.setCritical(!$0.isCritical) } }
        case .extra(let key, _, let critical):
            isOn = critical
            action = { Task { await viewModel.setExtraCritical(key: key, isOn: !critical) } }
        }
        return ComboToggleButton(
            title: ComboText.critical(index), isOn: isOn, identifier: "comboStepCritical-\(index)", action: action)
    }

    @ViewBuilder
    private func stepControls(index: Int, name: String, isMain: Bool, isFirst: Bool, isLast: Bool) -> some View {
        let layout = dynamicTypeSize.isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x2))
            : AnyLayout(HStackLayout(spacing: SpacingToken.x2))
        layout {
            ComboIconButton(
                systemName: "arrow.up", label: ComboText.moveUp(name), identifier: "comboStepUp-\(index)", isEnabled: !isFirst
            ) {
                Task { await viewModel.moveStep(at: index, by: -1) }
            }
            ComboIconButton(
                systemName: "arrow.down", label: ComboText.moveDown(name), identifier: "comboStepDown-\(index)",
                isEnabled: !isLast
            ) {
                Task { await viewModel.moveStep(at: index, by: 1) }
            }
            if !isMain {
                ComboIconButton(
                    systemName: "xmark", label: ComboText.removeStep(name), identifier: "comboStepRemove-\(index)", isEnabled: true
                ) {
                    Task { await viewModel.removeStep(at: index) }
                }
            }
        }
    }

    @ViewBuilder
    private var addStepControl: some View {
        if viewModel.canAddStep {
            ComboField(label: ComboText.addPickerLabel) {
                Button {
                    isAddSheetPresented = true
                } label: {
                    ComboPickerLabel(text: ComboText.addPickerLabel, systemImage: "plus")
                }
                .accessibilityLabel(ComboText.addPickerLabel)
                .accessibilityIdentifier("comboAddPicker")
            }
            .sheet(isPresented: $isAddSheetPresented) {
                ComboMovePickerSheet(
                    title: ComboText.addPickerLabel, moves: viewModel.extraCandidates, idPrefix: "comboAddMove-",
                    noneRowTitle: nil
                ) { id in
                    if let id { Task { await viewModel.addStep(moveId: id) } }
                }
            }
        } else {
            Text(ComboText.maxStepsNotice)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("comboMaxStepsNotice")
        }
    }

    // MARK: 相手

    @ViewBuilder
    private var defenderControl: some View {
        if let context = viewModel.context {
            ComboField(label: ComboText.defenderLabel) {
                Menu {
                    ForEach(context.defenders, id: \.key) { option in
                        Button(option.label) { Task { await viewModel.selectDefender(key: option.key) } }
                    }
                } label: {
                    ComboPickerLabel(text: viewModel.selectedDefender?.label ?? "", systemImage: "chevron.up.chevron.down")
                }
                .accessibilityLabel(ComboText.defenderLabel)
                .accessibilityValue(viewModel.selectedDefender?.label ?? "")
                .accessibilityIdentifier("comboDefenderPicker")
            }
            if viewModel.defenderWasReset {
                Text(ComboText.defenderReset)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("comboDefenderReset")
            }
        }
    }

    private var opponentMoveControl: some View {
        ComboField(label: ComboText.opponentMoveLabel) {
            Button {
                isOpponentSheetPresented = true
            } label: {
                ComboPickerLabel(
                    text: viewModel.opponentMove?.nameJa ?? ComboText.opponentMoveNone, systemImage: "chevron.up.chevron.down")
            }
            .accessibilityLabel(ComboText.opponentMoveLabel)
            .accessibilityValue(viewModel.opponentMove?.nameJa ?? ComboText.opponentMoveNone)
            .accessibilityIdentifier("comboOpponentMovePicker")
        }
        .sheet(isPresented: $isOpponentSheetPresented) {
            ComboMovePickerSheet(
                title: ComboText.opponentMoveLabel, moves: viewModel.opponentCandidates, idPrefix: "comboOpponentMove-",
                noneRowTitle: ComboText.opponentMoveNone
            ) { id in
                Task { await viewModel.selectOpponentMove(id: id) }
            }
        }
    }

    // MARK: 素早さの場

    private var speedFieldControl: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Button {
                isSpeedFieldExpanded.toggle()
            } label: {
                HStack(spacing: SpacingToken.x2) {
                    Text(ComboText.speedFieldToggle)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .fixedSize(horizontal: false, vertical: true)
                        .multilineTextAlignment(.leading)
                    Spacer(minLength: SpacingToken.x2)
                    Image(systemName: isSpeedFieldExpanded ? "chevron.up" : "chevron.down")
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .accessibilityHidden(true)
                }
                .frame(minHeight: ComboMetrics.minTapHeight)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityAddTraits(isSpeedFieldExpanded ? .isSelected : [])
            .accessibilityIdentifier("comboSpeedFieldToggle")
            if isSpeedFieldExpanded {
                speedFieldToggle(ComboText.trickRoom, id: "comboTrickRoom", keyPath: \.trickRoom)
                speedFieldToggle(ComboText.selfTailwind, id: "comboSelfTailwind", keyPath: \.selfTailwind)
                speedFieldToggle(ComboText.opponentTailwind, id: "comboOpponentTailwind", keyPath: \.opponentTailwind)
            }
        }
    }

    private func speedFieldToggle(_ title: String, id: String, keyPath: WritableKeyPath<ComboSpeedField, Bool>) -> some View {
        ComboToggleButton(title: title, isOn: viewModel.speedField[keyPath: keyPath], identifier: id) {
            var next = viewModel.speedField
            next[keyPath: keyPath].toggle()
            Task { await viewModel.setSpeedField(next) }
        }
    }
}
