import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamEditView: 構築編集画面(P6-2c・ADR-0501「P6-2c」5章)。
//
// ロジックは持たない。`TeamEditViewModel`(PokeCalcCore)の状態を描き、操作を async/同期メソッドへ
// つなぐだけ(ADR-0500 §1)。メンバーカードは `TeamEditMemberCard.swift` に分ける。

/// 構築編集画面。
struct TeamEditView: View {
    /// `TeamListView.loadingIndicatorHeight` と同じ値(マスタ読み込み中にレイアウトが動かないよう
    /// 高さを固定する。同じ定数を画面ごとに複製しているのは coding-rules §2 が認める独立した View
    /// 定数の重複で、値の意味は「読み込み中の枠の高さ」で揃えている)。
    private static let loadingIndicatorHeight: CGFloat = 24

    @State private var viewModel: TeamEditViewModel
    @Environment(\.dismiss) private var dismiss
    /// チーム名の入力欄はローカルの `@State` を真とし、`viewModel.setName(_:)` へ同期的に反映する
    /// (`ReverseScreenObservations.swift` の観測欄と同じ理由: `viewModel.team.name` は前後空白を
    /// トリムした後の値になるため、入力中の文字列をそのまま `TextField` に戻すとカーソル位置が
    /// 揺れうる)。
    @State private var nameText: String
    @State private var isSpeciesSearchPresented = false

    init(store: any TeamStore, service: any PokeCalcService, team: Team) {
        _viewModel = State(initialValue: TeamEditViewModel(store: store, service: service, team: team))
        _nameText = State(initialValue: team.name)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                if let error = viewModel.error {
                    ErrorBannerView(message: error.message, identifier: "teamEditErrorMessage")
                }
                loadingSlot
                nameField
                membersSection
                addMemberSection
                saveButton
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("teamEditScreen")
        .task { await viewModel.load() }
    }

    private var nameField: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            TextField(
                "チーム名",
                text: Binding(
                    get: { nameText },
                    set: { newValue in
                        nameText = newValue
                        viewModel.setName(newValue)
                    }
                )
            )
            .font(TextStyleToken.heading.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .padding(SpacingToken.x3)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .accessibilityIdentifier("teamNameField")

            if viewModel.nameError != nil {
                Text(TeamFieldError.emptyName.uiMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("teamNameError")
            }
        }
    }

    private var membersSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(viewModel.team.members, id: \.id) { member in
                MemberCardView(viewModel: viewModel, member: member)
            }
        }
    }

    /// issue #68: `Menu` ではなく検索シートで選ぶ(`CalcScreenCards` と同じ理由)。
    private var addMemberSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Button {
                isSpeciesSearchPresented = true
            } label: {
                Label("メンバーを追加", systemImage: "plus.circle")
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
            .accessibilityIdentifier("addMemberButton")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel) { option in
                    Task { await viewModel.addMember(speciesKey: option.key) }
                }
            }

            if let teamError = viewModel.teamError {
                Text(teamError.uiMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
            }
        }
    }

    private var saveButton: some View {
        Button {
            Task {
                if await viewModel.save() {
                    dismiss()
                }
            }
        } label: {
            Text("保存")
                .frame(maxWidth: .infinity)
        }
        .buttonStyle(PillButtonStyle())
        .accessibilityIdentifier("saveTeamButton")
    }

    /// マスタ読み込み中はピッカーが空のまま無反応に見えるため、`TeamListView.loadingSlot` と同じ
    /// (高さ固定・読み込み中だけ表示の)インジケータを出す(critic 指摘。ADR-0501「P6-2c」5章の
    /// identifier 契約に無い項目なので任意名)。
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
            TeamEditView(store: LocalTeamStore(), service: mock, team: Team(name: "テストパーティ"))
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
