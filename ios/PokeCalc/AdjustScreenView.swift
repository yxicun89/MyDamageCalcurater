import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenView: 調整画面(AJ7。ADR-0502 §2・§8)。
//
// ロジックは持たない。`AdjustViewModel`(PokeCalcCore)の状態を描き、操作を ViewModel のメソッドへつなぐだけ
// (ADR-0500 §1)。文言はすべて `AdjustText`(PokeCalcCore)から引く。常時動くアニメーションは入れない。

/// 数字キーボードの「完了」で閉じるためのフォーカス先(固定 SP の6欄と素早さの目標)。
enum AdjustFocus: Hashable {
    case fixedSP(StatKey)
    case minSpeed
}

struct AdjustScreenView: View {
    @State private var viewModel: AdjustViewModel
    @FocusState private var focus: AdjustFocus?
    private let backendDescription: String

    /// 読み込み中インジケータの高さ(`ReverseScreenView` と同じ理由で固定する)。
    private static let loadingIndicatorHeight: CGFloat = 24

    init(service: any PokeCalcService, adjust: any AdjustService, backendDescription: String) {
        _viewModel = State(initialValue: AdjustViewModel(service: service, adjust: adjust))
        self.backendDescription = backendDescription
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                backendBadge
                AdjustOwnCardView(viewModel: viewModel, focus: $focus)
                AdjustModeCardView(viewModel: viewModel, focus: $focus)
                if viewModel.needsOpponent {
                    AdjustOpponentCardView(viewModel: viewModel)
                    AdjustGoalCardView(viewModel: viewModel)
                }
                submitButton
                if let message = viewModel.alertMessage {
                    ErrorBannerView(message: message, identifier: "adjustAlert")
                }
                loadingSlot
                AdjustResultCardView(viewModel: viewModel)
                AdjustLearnersCardView(viewModel: viewModel)
            }
            .padding(SpacingToken.x4)
        }
        .scrollDismissesKeyboard(.interactively)
        .background(ColorToken.bgBase.color.ignoresSafeArea())
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
        // 画面を閉じたら送信と一覧の読み込みを止める(issue #113 A6)。
        .onDisappear { viewModel.cancelPendingWork() }
    }

    private var backendBadge: some View {
        Text(backendDescription)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x1)
            .background(ColorToken.bgGlass.color, in: Capsule())
            .accessibilityIdentifier("adjustBackendModeBadge")
    }

    private var submitButton: some View {
        Button {
            focus = nil
            viewModel.scheduleSubmit()
        } label: {
            Text(AdjustText.submitButton)
        }
        .buttonStyle(PillButtonStyle())
        .frame(maxWidth: .infinity)
        .accessibilityIdentifier("adjustSubmitButton")
    }

    private var loadingSlot: some View {
        Group {
            if viewModel.isLoading {
                ProgressView(AdjustText.loadingNotice)
                    .accessibilityIdentifier("adjustLoading")
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: Self.loadingIndicatorHeight)
    }
}

#Preview {
    if let mock = try? MockPokeCalcService(), let adjust = try? MockAdjustService() {
        NavigationStack {
            AdjustScreenView(service: mock, adjust: adjust, backendDescription: "モックデータで動作中")
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
