import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamListView: 構築一覧画面(P6-2c・ADR-0501「P6-2c」5章)。
//
// ロジックは持たない。`TeamListViewModel`(PokeCalcCore)の状態を描き、操作を async メソッドへ
// つなぐだけ(ADR-0500 §1)。編集画面への行き先は `RootView` の `NavigationStack` の `path` を
// そのまま共有する(`NavigationStack` を入れ子にすると SwiftUI が警告アイコンだけを表示して
// 中身を描かなくなる実機での不具合に当たったため。`navigationDestination(for:)` は宣言した
// 場所に関わらず最も近い祖先の `NavigationStack` に登録される[Apple のドキュメントどおり]ので、
// スタックそのものを増やさなくても済む)。
//
// F-08(ADR-0522)で作り直し: 構築名の入力を廃止(表示は「構築 N」)、[新しい構築]で空の構築を作ってすぐ編集画面を開く、
// 各構築はカード(6体のアイコン・n/6体・最終更新・[開く]・[削除]〈2段階〉)。
// Showdown 形式の取り込みは G-03(ADR-0528)で廃止した。

/// 構築一覧画面。
struct TeamListView: View {
    @State private var viewModel: TeamListViewModel
    private let store: any TeamStore
    private let service: any PokeCalcService
    /// `RootView` が持つ `NavigationStack` の `path` をそのまま共有する(上記コメント参照)。
    @Binding var path: NavigationPath

    @State private var isCreating = false

    /// 読み込み中インジケータの高さ(Calc/Reverse 画面と同じ理由で固定する)。
    private static let loadingIndicatorHeight: CGFloat = 24

    init(store: any TeamStore, service: any PokeCalcService, path: Binding<NavigationPath>) {
        self.store = store
        self.service = service
        _path = path
        _viewModel = State(initialValue: TeamListViewModel(store: store, service: service))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                if let error = viewModel.error {
                    ErrorBannerView(message: error.message, identifier: "teamListErrorMessage")
                }
                createButton
                loadingSlot
                if viewModel.teams.isEmpty {
                    if !viewModel.isLoading {
                        PopNoticeView(kind: .empty, message: TeamLabels.emptyList, identifier: "teamListEmpty")
                    }
                } else {
                    ForEach(viewModel.teams, id: \.id) { team in
                        TeamCardView(viewModel: viewModel, team: team) { path.append(team.id) }
                    }
                }
            }
            .padding(SpacingToken.x4)
        }
        .popScreenBackground()
        .accessibilityIdentifier("teamListScreen")
        .toolbar {
            ToolbarItem(placement: .principal) {
                Text("構築")
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
        }
        .navigationDestination(for: String.self) { teamID in
            if let team = viewModel.teams.first(where: { $0.id == teamID }) {
                TeamEditView(store: store, service: service, team: team, title: viewModel.displayName(for: team.id))
            }
        }
        // `.task` は初回表示の1回しか走らないため、編集画面から戻るたびの再読み込みは
        // `onAppear` で行う(`TeamListViewModel.load()` は何度呼んでもよい設計。
        // ADR-0501「P6-2c」3章)。
        .onAppear { Task { await viewModel.load() } }
    }

    /// 空の構築を作って、すぐその編集画面を開く(作成中は押せない)。
    private var createButton: some View {
        Button {
            guard !isCreating else { return }
            isCreating = true
            Task {
                if let created = await viewModel.createTeam() {
                    path.append(created.id)
                }
                isCreating = false
            }
        } label: {
            PopLabel(title: TeamLabels.createButton, systemImage: PopSymbol.add)
                .frame(maxWidth: .infinity)
        }
        .buttonStyle(PillButtonStyle(kind: .primary))
        .disabled(isCreating)
        .accessibilityIdentifier("createTeamButton")
    }

    private var loadingSlot: some View {
        Group {
            if viewModel.isLoading {
                ProgressView()
                    .accessibilityIdentifier("teamListLoadingIndicator")
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: Self.loadingIndicatorHeight)
    }
}

/// 一覧の 1 つの構築のカード: 表示名・6 体のアイコン・n/6体・最終更新・[開く]・[削除](2 段階)。
private struct TeamCardView: View {
    let viewModel: TeamListViewModel
    let team: Team
    let onOpen: () -> Void

    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    private var name: String { viewModel.displayName(for: team.id) }
    private var isConfirmingDelete: Bool { viewModel.pendingDeleteID == team.id }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text(name)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("teamName-\(team.id)")
            icons
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(TeamLabels.memberCount(team.members.count))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .accessibilityIdentifier("teamCount-\(team.id)")
                Text(TeamLabels.updatedText(team.updatedAt))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("teamUpdated-\(team.id)")
            }
            if isConfirmingDelete {
                deleteConfirmation
            } else {
                actions
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("teamCard-\(team.id)")
    }

    /// 6 体のアイコン。画像が無ければタイプ色のエンブレム(種族が引けなければ無彩色のエンブレム)。
    private var icons: some View {
        let names = team.members.enumerated().map { offset, member in
            viewModel.speciesSummary(forKey: member.speciesKey)?.nameJa ?? TeamLabels.slotTitle(offset + 1)
        }
        return HStack(spacing: SpacingToken.x1) {
            ForEach(Array(team.members.enumerated()), id: \.element.id) { _, member in
                let species = viewModel.speciesSummary(forKey: member.speciesKey)
                SpeciesImageView(speciesKey: member.speciesKey, name: species?.nameJa ?? "-", types: species?.types ?? [])
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(names.isEmpty ? "ポケモンはまだいません" : names.joined(separator: "、"))
        .accessibilityIdentifier("teamIcons-\(team.id)")
    }

    private var actions: some View {
        let layout = dynamicTypeSize.isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x2))
            : AnyLayout(HStackLayout(alignment: .center, spacing: SpacingToken.x2))
        return layout {
            Button(action: onOpen) {
                PopLabel(title: TeamLabels.openButton, systemImage: PopSymbol.edit)
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(PillButtonStyle(kind: .primary))
            .accessibilityLabel(TeamLabels.openHint(name: name))
            .accessibilityIdentifier("teamOpen-\(team.id)")

            Button {
                viewModel.requestDelete(id: team.id)
            } label: {
                PopLabel(title: TeamLabels.deleteButton, systemImage: PopSymbol.delete)
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(PillButtonStyle(kind: .secondary))
            .accessibilityLabel(TeamLabels.deleteHint(name: name))
            .accessibilityIdentifier("teamDelete-\(team.id)")
        }
    }

    private var deleteConfirmation: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            PopNoticeView(
                kind: .error, message: TeamLabels.deleteConfirmNotice(name: name),
                identifier: "teamDeleteConfirmNotice-\(team.id)")
            Button {
                Task { await viewModel.confirmDelete() }
            } label: {
                TeamTextControls.buttonLabel(TeamLabels.deleteConfirmButton)
            }
            .buttonStyle(PillButtonStyle(kind: .danger))
            .accessibilityIdentifier("teamDeleteConfirm-\(team.id)")
            Button {
                viewModel.cancelDelete()
            } label: {
                TeamTextControls.buttonLabel(TeamLabels.deleteCancelButton)
            }
            .buttonStyle(PillButtonStyle(kind: .secondary))
            .accessibilityIdentifier("teamDeleteCancel-\(team.id)")
        }
    }
}

#Preview {
    if let mock = try? MockPokeCalcService() {
        NavigationStack {
            TeamListView(store: LocalTeamStore(), service: mock, path: .constant(NavigationPath()))
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
