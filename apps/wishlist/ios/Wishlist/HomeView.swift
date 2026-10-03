import SwiftUI
import WishlistCore

private enum HomeSheet: Identifiable {
    case detail(Item)
    case actions(Item)
    case edit(Item)
    case register
    case settings

    var id: String {
        switch self {
        case .detail(let item): "detail-\(item.id)"
        case .actions(let item): "actions-\(item.id)"
        case .edit(let item): "edit-\(item.id)"
        case .register: "register"
        case .settings: "settings"
        }
    }
}

/// ホーム: 画像だけの 3 列グリッド + ジャンルのチップ(スクロールで隠れる)+ 右下の「+」。
struct HomeView: View {
    @Environment(AppModel.self) private var model
    @State private var sheet: HomeSheet?

    private var columns: [GridItem] {
        Array(repeating: GridItem(.flexible(), spacing: 4), count: HomeLayout.gridColumnCount)
    }

    var body: some View {
        let home = model.home
        ScrollView {
            VStack(spacing: 12) {
                chips(home)
                if let message = home.loadError, home.items.isEmpty {
                    VStack(spacing: 12) {
                        Text(message).foregroundStyle(.secondary).multilineTextAlignment(.center)
                        Button("再読み込み") { Task { await home.refresh() } }
                    }
                    .padding(32)
                }
                LazyVGrid(columns: columns, spacing: 4) {
                    ForEach(home.visibleItems) { item in
                        cell(item)
                    }
                }
                .padding(.horizontal, 4)
                .accessibilityIdentifier("itemGrid")
            }
            .padding(.bottom, 96)
        }
        .refreshable { await home.refresh() }
        .overlay(alignment: .topTrailing) {
            GlassCircleButton(systemImage: "gearshape", label: "設定") { sheet = .settings }
                .accessibilityIdentifier("settingsButton")
                .padding(.trailing, 16)
        }
        .overlay(alignment: .bottomTrailing) {
            GlassCircleButton(systemImage: "plus", label: "登録") { sheet = .register }
                .accessibilityIdentifier("addButton")
                .padding(20)
        }
        .overlay(alignment: .bottomLeading) {
            if home.isStale {
                Image(systemName: "icloud.slash")
                    .padding(12)
                    .glassEffect(.regular, in: Circle())
                    .padding(20)
                    .accessibilityLabel("保存済みの一覧を表示中")
            }
        }
        .task {
            await model.prepare()
            await model.home.loadCached()
            await model.home.refresh()
        }
        .sheet(item: $sheet) { sheet in
            sheetContent(sheet)
        }
        .alert("エラー", isPresented: Binding(get: { home.errorMessage != nil }, set: { _ in })) {
            Button("OK") {}
        } message: {
            Text(home.errorMessage ?? "")
        }
    }

    // MARK: - 部品

    private func chips(_ home: HomeViewModel) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                chip("すべて", selected: home.selectedGenreID == nil, id: "genreChip-all") { home.selectedGenreID = nil }
                ForEach(home.chipGenres) { genre in
                    chip(genre.name, selected: home.selectedGenreID == genre.id, id: "genreChip-\(genre.id)") {
                        home.selectedGenreID = genre.id
                    }
                }
            }
            .padding(.horizontal, 16)
        }
        .padding(.top, 8)
        .padding(.trailing, 72)
    }

    @ViewBuilder
    private func chip(_ title: String, selected: Bool, id: String, action: @escaping () -> Void) -> some View {
        let label = Text(title)
            .font(.subheadline.weight(selected ? .semibold : .regular))
            .padding(.horizontal, 6)
        if selected {
            Button(action: action) { label }
                .buttonStyle(.glassProminent)
                .accessibilityIdentifier(id)
                .accessibilityAddTraits(.isSelected)
        } else {
            Button(action: action) { label }
                .buttonStyle(.glass)
                .accessibilityIdentifier(id)
        }
    }

    /// 画像だけのセル。タップで詳細シート、長押し(0.5 秒)で編集・削除。
    private func cell(_ item: Item) -> some View {
        ItemImageView(item: item)
            .contentShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
            .onTapGesture { sheet = .detail(item) }
            .onLongPressGesture(minimumDuration: HomeLayout.longPressSeconds) { sheet = .actions(item) }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(item.name)
            .accessibilityAddTraits(.isButton)
            .accessibilityIdentifier("item-\(item.id)")
    }

    @ViewBuilder
    private func sheetContent(_ sheet: HomeSheet) -> some View {
        let home = model.home
        switch sheet {
        case .detail(let item):
            ItemDetailSheet(item: item, genre: home.genres.first { $0.id == item.genreID }, sites: home.sites, service: model.service) { saved in
                Task { await model.home.upsert(saved) }
            }
        case .actions(let item):
            ItemActionsSheet(
                item: item,
                onEdit: { self.sheet = .edit(item) },
                onDelete: {
                    _ = await model.home.delete(item)
                    self.sheet = nil
                })
        case .edit(let item):
            EditItemSheet(item: item, genres: home.genres, service: model.service) { saved in
                Task { await model.home.upsert(saved) }
            }
        case .register:
            RegisterSheet(genres: home.genres, initialGenreID: home.selectedGenreID, service: model.service) { created in
                Task { await model.home.upsert(created) }
            }
        case .settings:
            SettingsView(isInitialSetup: false)
        }
    }
}
