import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenView: 調整画面(AJ7。ADR-0502 §2・§8)。
//
// ロジックは持たない。`AdjustViewModel`(PokeCalcCore)の状態を描き、操作を ViewModel のメソッドへつなぐだけ
// (ADR-0500 §1)。文言はすべて `AdjustText`(PokeCalcCore)から引く。常時動くアニメーションは入れない。

/// 数字キーボードの「完了」で閉じるためのフォーカス先(固定 SP の6欄・上限・素早さの目標)。
enum AdjustFocus: Hashable {
    case fixedSP(StatKey)
    case minSpeed
    case ceiling(StatKey)
}

struct AdjustScreenView: View {
    @State private var viewModel: AdjustViewModel
    @FocusState private var focus: AdjustFocus?
    private let backendDescription: String

    /// 画面の持ち方(タブ・区切りは `.retained`。ADR-0531 §2)。離れたときに送信を取り消すかを決める。
    private let lifetime: ScreenLifetime

    init(
        service: any PokeCalcService, adjust: any AdjustService, goals: (any AdjustGoalsService)? = nil,
        backendDescription: String, lifetime: ScreenLifetime = .retained
    ) {
        self.lifetime = lifetime
        _viewModel = State(initialValue: AdjustViewModel(service: service, adjust: adjust, goals: goals))
        self.backendDescription = backendDescription
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                backendBadge
                AdjustOwnCardView(viewModel: viewModel, focus: $focus)
                AdjustModeCardView(viewModel: viewModel, focus: $focus)
                if viewModel.isGoalsMode {
                    AdjustGoalsCardView(viewModel: viewModel)
                }
                if viewModel.needsOpponent {
                    AdjustOpponentCardView(viewModel: viewModel)
                    AdjustGoalCardView(viewModel: viewModel)
                }
                submitButton
                if let message = viewModel.alertMessage {
                    ErrorBannerView(message: message, identifier: "adjustAlert")
                    if viewModel.canRetryLoad {
                        LoadRetryButton(identifier: "adjustLoadRetry") { await viewModel.load() }
                    }
                }
                loadingSlot
                AdjustResultCardView(viewModel: viewModel)
                AdjustLearnersCardView(viewModel: viewModel)
            }
            .padding(SpacingToken.x4)
        }
        .scrollDismissesKeyboard(.interactively)
        .popScreenBackground()
        .accessibilityIdentifier("adjustScreen")
        .navigationTitle(AdjustText.screenTitle)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            // 数字キーボードには Return が無いので、閉じる手段を別に用意する。
            ToolbarItemGroup(placement: .keyboard) {
                Spacer()
                Button(AdjustText.keyboardDone) { focus = nil }
            }
        }
        .task { await viewModel.load() }
        // エラーは VoiceOver にも読ませる(送信後のフォーカスは動かさない。ADR-0502 §8)。
        .onChange(of: viewModel.alertSerial) { _, _ in
            if let message = viewModel.alertMessage { AccessibilityNotification.Announcement(message).post() }
        }
        // 戻ると破棄される画面(積んだ画面)だけ、離れたら入力・送信を止める(issue #113 A6・ADR-0531 §2)。
        .cancelsPendingWorkOnDisappear(lifetime) { viewModel.cancelPendingWork() }
    }

    private var backendBadge: some View {
        Text(backendDescription)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x1)
            .background(ColorToken.tableZebra.color, in: Capsule())
            .accessibilityIdentifier("adjustBackendModeBadge")
    }

    private var submitButton: some View {
        Button {
            focus = nil
            viewModel.scheduleSubmit()
        } label: {
            PopLabel(title: AdjustText.submitButton, systemImage: PopSymbol.adjust)
        }
        .buttonStyle(PillButtonStyle(kind: .primary))
        .frame(maxWidth: .infinity)
        .accessibilityIdentifier("adjustSubmitButton")
    }

    private var loadingSlot: some View {
        Group {
            // 静止したアイコン + 一言(回り続ける表示は置かない。ADR-0540 §5)。
            if viewModel.isLoading {
                PopNoticeView(kind: .loading, message: AdjustText.loadingNotice, identifier: "adjustLoading")
            }
        }
    }
}

#Preview {
    if let mock = try? MockPokeCalcService(), let adjust = try? MockAdjustService() {
        NavigationStack {
            AdjustScreenView(service: mock, adjust: adjust, goals: try? MockAdjustGoalsService(), backendDescription: "モックデータで動作中")
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
