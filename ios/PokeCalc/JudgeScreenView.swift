import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// JudgeScreenView: 判定画面(P6-25。ADR-0504)。
//
// ロジックは持たない。`JudgeViewModel`(PokeCalcCore)の状態を描き、操作をメソッドへつなぐだけ(ADR-0500 §1)。
// 縦の 1 本のスクロール: 自分のポケモン → 相手の候補 → 場の効果 → 判定ボタン → 結果。
// 送信は「判定する」を押したときだけ。選択はシート(`Menu` は使わない)。
// 色を持つのはタイプのエンブレムだけ(design.md)。常時動くアニメーションは入れない。

/// 画面から開くシート。`id` は同じ種類・同じ対象で一意(`.sheet(item:)` 用)。
private enum JudgePicker: Identifiable {
    case species(JudgeTarget)
    case move(JudgeTarget)
    case option(JudgeTarget, JudgeOptionKind)

    var id: String {
        switch self {
        case .species(let target): return "species-\(target)"
        case .move(let target): return "move-\(target)"
        case .option(let target, let kind): return "option-\(kind.rawValue)-\(target)"
        }
    }
}

struct JudgeScreenView: View {
    @State private var viewModel: JudgeViewModel
    @State private var picker: JudgePicker?

    init(service: any JudgeService, master: any PokeCalcService, teamStore: any TeamStore) {
        _viewModel = State(initialValue: JudgeViewModel(service: service, master: master, teamStore: teamStore))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                PopHeading(title: JudgeLabels.screenTitle, systemImage: PopSymbol.judge)
                if viewModel.masterFailure != nil {
                    ErrorBannerView(message: JudgeLabels.masterLoadFailed, identifier: "judgeMasterError")
                }
                attackerSection
                candidatesSection
                fieldSection
                submitSection
                JudgeResultSection(viewModel: viewModel)
            }
            .padding(SpacingToken.x4)
        }
        .popScreenBackground()
        .accessibilityIdentifier("judgeScreen")
        .task { await viewModel.load() }
        .onDisappear { viewModel.cancelPendingWork() }
        .sheet(item: $picker) { picker in
            sheetContent(for: picker)
        }
    }

    // MARK: - 自分・候補

    private var attackerSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            JudgeSectionTitle(text: JudgeLabels.attackerRegion)
            JudgeIndividualCard(
                viewModel: viewModel, target: .attacker, identifierPrefix: "judgeAttacker", title: JudgeLabels.attackerRegion,
                removeAction: nil, onPick: { picker = Self.picker(for: $0, target: .attacker) })
        }
    }

    private var candidatesSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            JudgeSectionTitle(text: JudgeLabels.defendersRegion)
            ForEach(Array(viewModel.candidates.indices), id: \.self) { index in
                let number = index + 1
                JudgeIndividualCard(
                    viewModel: viewModel, target: .candidate(index), identifierPrefix: "judgeCandidate\(number)",
                    title: JudgeLabels.candidate(number),
                    removeAction: JudgeRemoveAction(number: number, isEnabled: viewModel.canRemoveCandidate) {
                        viewModel.removeCandidate(at: index)
                    },
                    onPick: { picker = Self.picker(for: $0, target: .candidate(index)) })
            }
            Button {
                viewModel.addCandidate()
            } label: {
                PopLabel(title: JudgeLabels.addCandidate, systemImage: PopSymbol.add)
            }
            .buttonStyle(PillButtonStyle())
            .disabled(!viewModel.canAddCandidate)
            .opacity(viewModel.canAddCandidate ? 1 : CalcScreenMetrics.disabledChipOpacity)
            .accessibilityIdentifier("judgeAddCandidate")
            if !viewModel.canAddCandidate {
                JudgeNotice(text: JudgeLabels.maxCandidatesNotice(RequestLimits.maxJudgeDefenders), identifier: "judgeCandidateLimitNotice")
            }
            if !viewModel.canRemoveCandidate {
                JudgeNotice(text: JudgeLabels.minimumCandidateNotice, identifier: "judgeCandidateMinimumNotice")
            }
        }
    }

    private static func picker(for request: JudgePickerRequest, target: JudgeTarget) -> JudgePicker {
        switch request {
        case .species: return .species(target)
        case .move: return .move(target)
        case .option(let kind): return .option(target, kind)
        }
    }

    // MARK: - 場の効果・送信

    private var fieldSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            SpeedPillGroup(title: JudgeLabels.speedFieldGroup) {
                SpeedPill(
                    title: JudgeLabels.trickRoom, isSelected: viewModel.speedField.trickRoom, identifier: "judgeTrickRoom"
                ) { viewModel.setTrickRoom(!viewModel.speedField.trickRoom) }
                SpeedPill(
                    title: JudgeLabels.attackerTailwind, isSelected: viewModel.speedField.attackerTailwind,
                    identifier: "judgeAttackerTailwind"
                ) { viewModel.setAttackerTailwind(!viewModel.speedField.attackerTailwind) }
                SpeedPill(
                    title: JudgeLabels.defenderTailwind, isSelected: viewModel.speedField.defenderTailwind,
                    identifier: "judgeDefenderTailwind"
                ) { viewModel.setDefenderTailwind(!viewModel.speedField.defenderTailwind) }
            }
            JudgeNotice(text: JudgeLabels.defenderTailwindNotice, identifier: "judgeDefenderTailwindNotice")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.card)
    }

    private var submitSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Button {
                viewModel.submit()
            } label: {
                PopLabel(title: JudgeLabels.submit, systemImage: PopSymbol.judge)
            }
            .buttonStyle(PillButtonStyle(kind: .primary))
            .accessibilityIdentifier("judgeSubmit")
            if let error = viewModel.validationError {
                ErrorBannerView(message: error.message, identifier: "judgeValidationError")
            }
        }
    }

    // MARK: - シート

    @ViewBuilder
    private func sheetContent(for picker: JudgePicker) -> some View {
        switch picker {
        case .species(let target):
            SpeciesSearchSheet(viewModel: viewModel) { species in
                Task { await viewModel.setSpecies(species, for: target) }
            }
        case .move(let target):
            MoveSearchSheet(viewModel: viewModel, options: viewModel.moveOptions) { move in
                viewModel.setMove(move, for: target)
            }
        case .option(let target, let kind):
            JudgeOptionSheet(viewModel: viewModel, target: target, kind: kind)
        }
    }
}

// MARK: - 小さな部品

/// 候補カードの「削除」(最低 1 件は残すので、`isEnabled` が false の間は押せない)。
struct JudgeRemoveAction {
    let number: Int
    let isEnabled: Bool
    let perform: () -> Void

    init(number: Int, isEnabled: Bool, perform: @escaping () -> Void) {
        self.number = number
        self.isEnabled = isEnabled
        self.perform = perform
    }
}

struct JudgeSectionTitle: View {
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
struct JudgeNotice: View {
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
