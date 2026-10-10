import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamEditView: 構築編集画面(P6-2c・ADR-0501「P6-2c」5章。F-08 で作り直し: ADR-0522)。
//
// 6 つの枠(1体目〜6体目)が最初から並ぶ。空の枠は「ポケモン」の欄と案内だけ、種族を選ぶと技・持ち物などの
// カードになる。保存は明示([保存])。ロジックは持たない。`TeamEditViewModel`(PokeCalcCore)の状態を描き、
// 操作を async/同期メソッドへつなぐだけ(ADR-0500 §1)。メンバーカードは `TeamEditMemberCard.swift`、
// 枠の部品は `TeamSlotViews.swift` に分ける。

/// 構築編集画面。
struct TeamEditView: View {
    /// `TeamListView.loadingIndicatorHeight` と同じ値(マスタ読み込み中にレイアウトが動かないよう高さを固定する)。
    private static let loadingIndicatorHeight: CGFloat = 24

    @State private var viewModel: TeamEditViewModel
    @Environment(\.dismiss) private var dismiss
    /// 見出しに出す表示名(「構築 N」または旧データの名前)。
    private let title: String
    /// 「保存していない変更があります。保存せずに一覧に戻りますか」の確認を出している。
    @State private var isConfirmingLeave = false

    init(store: any TeamStore, service: any PokeCalcService, team: Team, title: String) {
        self.title = title
        _viewModel = State(initialValue: TeamEditViewModel(store: store, service: service, team: team))
    }

    var body: some View {
        // 保存の帯はスクロールの外(下)に置く。`safeAreaInset` だとスクロールの中身が帯の下に潜り込み、
        // 帯に隠れた操作に届かない(XCUITest のスクロールも帯の下を「見えている」と数える)ため。
        VStack(spacing: 0) {
            // 識別子は ScrollView だけに付ける(外側の VStack に付けると保存の帯の子の識別子を上書きする)。
            ScrollView { content }
                .accessibilityIdentifier("teamEditScreen")
            saveBar
        }
        .popScreenBackground()
        // 戻る操作は[一覧に戻る]に一本化する(未保存の確認を必ず通すため、システムの戻るは隠す)。
        .navigationBarBackButtonHidden(true)
        .toolbar {
            ToolbarItem(placement: .principal) {
                Text(title)
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .accessibilityIdentifier("teamEditTitle")
            }
        }
        .task { await viewModel.load() }
    }

    private var content: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x4) {
            backRow
            if isConfirmingLeave {
                leaveConfirmation
            }
            if let error = viewModel.error {
                ErrorBannerView(message: error.message, identifier: "teamEditErrorMessage")
            }
            loadingSlot
            slotsSection
            if let teamError = viewModel.teamError {
                PopNoticeView(kind: .error, message: teamError.uiMessage, identifier: "teamLimitMessage")
            }
        }
        .padding(SpacingToken.x4)
    }

    // MARK: - 一覧に戻る

    private var backRow: some View {
        Button {
            if viewModel.hasUnsavedChanges {
                isConfirmingLeave = true
            } else {
                dismiss()
            }
        } label: {
            PopLabel(title: TeamLabels.backToList, systemImage: "chevron.left")
        }
        .buttonStyle(PillButtonStyle(kind: .secondary))
        .accessibilityIdentifier("backToListButton")
    }

    /// 未保存で[一覧に戻る]を押したときの 2 段階目(alert ではなくカードで描く。P6-7)。
    private var leaveConfirmation: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopNoticeView(kind: .error, message: TeamLabels.leaveConfirmNotice, identifier: "teamLeaveConfirmNotice")
            Button {
                dismiss()
            } label: {
                TeamTextControls.buttonLabel(TeamLabels.leaveDiscard)
            }
            .buttonStyle(PillButtonStyle(kind: .danger))
            .accessibilityIdentifier("teamLeaveDiscardButton")
            Button {
                isConfirmingLeave = false
            } label: {
                TeamTextControls.buttonLabel(TeamLabels.leaveCancel)
            }
            .buttonStyle(PillButtonStyle(kind: .secondary))
            .accessibilityIdentifier("teamLeaveCancelButton")
        }
    }

    // MARK: - 6 つの枠

    private var slotsSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x4) {
            ForEach(0..<TeamLimits.maxMembers, id: \.self) { index in
                TeamSlotView(viewModel: viewModel, index: index)
            }
        }
    }

    // MARK: - 保存

    /// 画面の下に常に出す保存の帯(枠が長くても[保存]に届く。スクロールの外)。未保存・保存済みは文字で伝える。
    private var saveBar: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            if viewModel.hasUnsavedChanges {
                Text(TeamLabels.unsavedNotice)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("teamUnsavedNotice")
            } else if viewModel.didSave {
                Text(TeamLabels.savedNotice)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("teamSavedNotice")
            }
            Button {
                Task { await viewModel.save() }
            } label: {
                PopLabel(title: TeamLabels.save, systemImage: PopSymbol.save)
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(PillButtonStyle(kind: .primary))
            .accessibilityIdentifier("saveTeamButton")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(ColorToken.surfaceCard.color)
    }

    /// マスタ読み込み中はピッカーが空のまま無反応に見えるため、`TeamListView.loadingSlot` と同じ
    /// (高さ固定・読み込み中だけ表示の)インジケータを出す。
    private var loadingSlot: some View {
        Group {
            if viewModel.isLoading {
                ProgressView()
                    .accessibilityIdentifier("teamEditLoadingIndicator")
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: Self.loadingIndicatorHeight)
    }
}

#Preview {
    if let mock = try? MockPokeCalcService() {
        NavigationStack {
            TeamEditView(store: LocalTeamStore(), service: mock, team: Team(name: TeamNaming.defaultName), title: TeamNaming.untitled(1))
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
