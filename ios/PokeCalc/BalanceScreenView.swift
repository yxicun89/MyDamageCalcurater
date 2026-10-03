import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// BalanceScreenView: タイプバランス画面(P6-21・P6-22。ADR-0415 第1段=防御相性・チーム集計、第2段=攻撃範囲、
// 第3段=仮想敵・おすすめタイプ・技範囲チェッカー)。
//
// ロジックは持たない。`BalanceViewModel`(PokeCalcCore)の状態を描き、操作をメソッドへつなぐだけ(ADR-0500 §1)。
// 倍率・集計は balance の応答をそのまま出す(iOS で相性を計算しない)。弱点/耐性/無効は色だけでなく文字でも出す。
// 結果の表は `BalanceResultViews.swift`、メンバーカードは `BalanceMemberCard.swift`。

struct BalanceScreenView: View {
    /// 読み込み中インジケータの高さ(他の画面と同じ理由で固定し、レイアウトを動かさない)。
    private static let loadingIndicatorHeight: CGFloat = 24

    @State private var viewModel: BalanceViewModel
    private let teamStore: any TeamStore
    @State private var savedTeams: [Team] = []
    @State private var isSpeciesSearchPresented = false

    init(balance: any BalanceService, service: any PokeCalcService, teamStore: any TeamStore) {
        _viewModel = State(initialValue: BalanceViewModel(balance: balance, master: service))
        self.teamStore = teamStore
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                if let error = viewModel.masterError {
                    ErrorBannerView(message: error.message, identifier: "balanceMasterError")
                }
                loadingSlot
                membersSection
                addMemberSection
                BalanceResultsView(viewModel: viewModel)
                // 第3段(ADR-0415 §8)。仮想敵・おすすめはメンバーが要る。技範囲はメンバーと無関係。
                BalanceThreatsView(viewModel: viewModel)
                if !viewModel.members.isEmpty {
                    BalanceRecommendationsView(viewModel: viewModel)
                }
                BalanceMoveRangeView(viewModel: viewModel)
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("balanceScreen")
        .toolbar {
            ToolbarItem(placement: .principal) {
                Text(BalanceScreenText.screenTitle)
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
        }
        .task {
            await viewModel.load()
            savedTeams = (try? await teamStore.list()) ?? []
        }
        // 画面を離れたら保留中の要求を cancel し、以後の応答を反映しない。
        .onDisappear { viewModel.cancel() }
    }

    private var membersSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(Array(viewModel.members.enumerated()), id: \.element.id) { index, member in
                BalanceMemberCard(viewModel: viewModel, member: member, number: index + 1)
            }
        }
    }

    private var addMemberSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Button {
                isSpeciesSearchPresented = true
            } label: {
                Label(BalanceScreenText.addMemberLabel, systemImage: "plus.circle")
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
            .accessibilityIdentifier("balanceAddMemberButton")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel) { option in
                    Task { await viewModel.addMember(speciesKey: option.key) }
                }
            }

            loadTeamMenu

            if let teamError = viewModel.teamError {
                Text(teamError.uiMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("balanceTeamError")
            }
        }
    }

    /// 保存済みの構築からメンバーを読み込む(入れ替え)。構築が無ければ案内だけ出す。
    @ViewBuilder
    private var loadTeamMenu: some View {
        if savedTeams.isEmpty {
            Text(BalanceScreenText.emptyTeamLoadNotice)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityIdentifier("balanceNoSavedTeams")
        } else {
            Menu {
                ForEach(savedTeams, id: \.id) { team in
                    Button(team.name) { Task { await viewModel.loadTeam(team) } }
                }
            } label: {
                MenuLabelChip(text: BalanceScreenText.loadTeamLabel)
            }
            .accessibilityIdentifier("balanceLoadTeamMenu")
        }
    }

    private var loadingSlot: some View {
        Group {
            if viewModel.isLoadingMaster {
                ProgressView()
                    .accessibilityIdentifier("balanceLoadingIndicator")
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: Self.loadingIndicatorHeight)
    }
}

#Preview {
    if let mock = try? MockPokeCalcService() {
        NavigationStack {
            BalanceScreenView(balance: UnavailableBalanceService(), service: mock, teamStore: LocalTeamStore())
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
