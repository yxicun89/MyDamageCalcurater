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
// スタックそのものを増やさなくても済む)。design.md には構築ビルダーの節が無いため、
// 見た目(カード/ホロ風の行・ドットでメンバー数を示す)は implementer の判断(ADR-0501「P6-2c」
// 「### 7 確認事項」に追記した)。

/// 構築一覧画面。
struct TeamListView: View {
    @State private var viewModel: TeamListViewModel
    private let store: any TeamStore
    private let service: any PokeCalcService
    /// `RootView` が持つ `NavigationStack` の `path` をそのまま共有する(上記コメント参照)。
    @Binding var path: NavigationPath

    @State private var isPresentingCreateAlert = false
    @State private var newTeamName = ""

    /// 読み込み中インジケータの高さ(Calc/Reverse 画面と同じ理由で固定する)。
    private static let loadingIndicatorHeight: CGFloat = 24

    init(store: any TeamStore, service: any PokeCalcService, path: Binding<NavigationPath>) {
        self.store = store
        self.service = service
        _path = path
        _viewModel = State(initialValue: TeamListViewModel(store: store))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                if let error = viewModel.error {
                    ErrorBannerView(message: error.message, identifier: "teamListErrorMessage")
                }
                loadingSlot
                if viewModel.teams.isEmpty {
                    if !viewModel.isLoading {
                        Text("まだ構築がありません。「新規作成」から始めましょう。")
                            .font(TextStyleToken.body.font)
                            .foregroundStyle(ColorToken.textSecondary.color)
                            .accessibilityIdentifier("teamListEmpty")
                    }
                } else {
                    ForEach(viewModel.teams, id: \.id) { team in
                        TeamRowView(
                            team: team,
                            onTap: { path.append(team.id) },
                            onDelete: { Task { await viewModel.deleteTeam(id: team.id) } }
                        )
                    }
                }
                createButton
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
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
                TeamEditView(store: store, service: service, team: team)
            }
        }
        .alert("新しい構築", isPresented: $isPresentingCreateAlert) {
            // `.alert` の中身は UIKit の `UIAlertController` が生成する `UITextField` に写像され、
            // SwiftUI 側の `.accessibilityIdentifier` はそこへ橋渡しされない(実装時に XCUITest で
            // 確認した実機の制約。ADR-0501「P6-2c」「### 7 確認事項」に追記)。XCUITest 側は
            // `app.alerts.textFields.firstMatch` で辿る(アラートに入力欄は1つしか無い)。
            TextField("構築名", text: $newTeamName)
            Button("キャンセル", role: .cancel) {
                newTeamName = ""
            }
            Button("作成") {
                let name = newTeamName
                newTeamName = ""
                Task {
                    if let created = await viewModel.createTeam(name: name) {
                        path.append(created.id)
                    }
                }
            }
        } message: {
            Text("構築の名前を入力してください")
        }
        // `.task` は初回表示の1回しか走らないため、編集画面から戻るたびの再読み込みは
        // `onAppear` で行う(`TeamListViewModel.load()` は何度呼んでもよい設計。
        // ADR-0501「P6-2c」3章)。
        .onAppear { Task { await viewModel.load() } }
    }

    private var createButton: some View {
        Button {
            newTeamName = ""
            isPresentingCreateAlert = true
        } label: {
            Label("新規作成", systemImage: "plus.circle")
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
        }
        .buttonStyle(.plain)
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

/// 一覧の1行: チーム名・メンバー数(ドット表示)・開く/削除。design.md「カード/ホロのコレクション風」
/// の角丸カードを踏襲する(実データは無いのでタイプ色は使わず無彩色のドットで数を示す)。
private struct TeamRowView: View {
    let team: Team
    let onTap: () -> Void
    let onDelete: () -> Void

    var body: some View {
        HStack(spacing: SpacingToken.x3) {
            Button(action: onTap) {
                HStack(spacing: SpacingToken.x3) {
                    VStack(alignment: .leading, spacing: SpacingToken.x1) {
                        Text(team.name)
                            .font(TextStyleToken.heading.font)
                            .foregroundStyle(ColorToken.textPrimary.color)
                            .lineLimit(1)
                        Text("メンバー \(team.members.count)/\(TeamLimits.maxMembers)")
                            .font(TextStyleToken.caption.font)
                            .foregroundStyle(ColorToken.textSecondary.color)
                        memberDots
                    }
                    Spacer(minLength: 0)
                    Image(systemName: "chevron.right")
                        .foregroundStyle(ColorToken.textSecondary.color)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("teamRow-\(team.id)")
            .accessibilityLabel(team.name)
            .accessibilityHint("開く")

            Button(action: onDelete) {
                Image(systemName: "trash")
                    .foregroundStyle(ColorToken.danger.color)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("teamDelete-\(team.id)")
            .accessibilityLabel("この構築を削除")
        }
        .padding(SpacingToken.x3)
        .glassCard()
    }

    private var memberDots: some View {
        HStack(spacing: SpacingToken.x1) {
            ForEach(0..<TeamLimits.maxMembers, id: \.self) { index in
                Circle()
                    .fill(index < team.members.count ? ColorToken.textPrimary.color : ColorToken.borderHairline.color)
                    .frame(width: 6, height: 6)
            }
        }
        .accessibilityHidden(true)
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
