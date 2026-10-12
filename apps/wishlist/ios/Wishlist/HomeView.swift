import SwiftUI
import WishlistCore

private enum HomeSheet: Identifiable {
    case detail(Item)
    case edit(Item)
    case register
    case settings

    var id: String {
        switch self {
        case .detail(let item): "detail-\(item.id)"
        case .edit(let item): "edit-\(item.id)"
        case .register: "register"
        case .settings: "settings"
        }
    }
}

/// ホーム: 2 列の画像グリッド(ジャンル色の枠・価格の札)+ ジャンルの絵柄のボタン + 右下の「+」。
/// 長押し(0.5 秒)で背景をぼかして編集・元のページを開く・削除のメニューを出す。
struct HomeView: View {
    @Environment(AppModel.self) private var model
    @State private var sheet: HomeSheet?
    @State private var menuTile: HomeTile?

    private let columns = Array(repeating: GridItem(.flexible(), spacing: Metrics.gridSpacing), count: HomeLayout.tileColumnCount)

    private var isSettingsOpen: Bool {
        if case .settings = sheet { true } else { false }
    }

    var body: some View {
        let home = model.home
        ZStack {
            Palette.bg.ignoresSafeArea()
            content(home)
                .blur(radius: menuTile == nil ? 0 : 6)
                .accessibilityHidden(menuTile != nil || isSettingsOpen)
            if let tile = menuTile {
                ItemContextMenu(
                    tile: tile,
                    onEdit: {
                        menuTile = nil
                        sheet = .edit(tile.item)
                    },
                    onDelete: {
                        _ = await model.home.delete(tile.item)
                        menuTile = nil
                    },
                    onClose: { menuTile = nil })
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

    private func content(_ home: HomeViewModel) -> some View {
        ScrollView {
            VStack(spacing: 14) {
                header(home)
                genreButtons(home)
                if let message = home.loadError, home.items.isEmpty {
                    VStack(spacing: 12) {
                        Text(message).font(.wl(14)).foregroundStyle(Palette.muted).multilineTextAlignment(.center)
                        Button("再読み込み") { Task { await home.refresh() } }
                            .buttonStyle(OutlineButtonStyle(height: 44))
                            .frame(maxWidth: 200)
                    }
                    .padding(32)
                }
                LazyVGrid(columns: columns, spacing: Metrics.gridSpacing) {
                    ForEach(home.tiles) { tile in
                        tileView(tile)
                    }
                }
                .padding(.horizontal, 16)
                .accessibilityIdentifier("itemGrid")
            }
            .padding(.bottom, 110)
        }
        .refreshable { await home.refresh() }
        .overlay(alignment: .bottomTrailing) {
            Button {
                sheet = .register
            } label: {
                Image(systemName: "plus").font(.system(size: 26, weight: .bold))
                    .foregroundStyle(Palette.onAccent)
                    .frame(width: 64, height: 64)
                    .background(Palette.accent, in: RoundedRectangle(cornerRadius: 22, style: .continuous))
                    .shadow(color: .black.opacity(0.3), radius: 15, y: 10)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("登録")
            .accessibilityIdentifier("addButton")
            .padding(20)
        }
    }

    private func header(_ home: HomeViewModel) -> some View {
        HStack(alignment: .center, spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                Text("欲しいもの").font(.wl(26, .heavy)).foregroundStyle(Palette.fg)
                    .accessibilityAddTraits(.isHeader)
                Text("\(home.tiles.count)件").font(.wl(13)).foregroundStyle(Palette.muted)
                    .accessibilityIdentifier("itemCount")
            }
            Spacer()
            if home.showsOfflineBadge { OfflineBadge() }
            Button {
                sheet = .settings
            } label: {
                Image(systemName: "gearshape").font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(Palette.fg)
                    .frame(width: 44, height: 44)
                    .card(radius: Metrics.buttonRadius)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("設定")
            .accessibilityIdentifier("settingsButton")
        }
        .padding(.horizontal, 16)
        .padding(.top, 8)
    }

    private func genreButtons(_ home: HomeViewModel) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 10) {
                genreButton(
                    name: GenreLabel.allName, selected: home.selectedGenreID == nil, id: "genreChip-all", glyph: { GlyphIcon(all: $0, size: 24) }
                ) { home.selectedGenreID = nil }
                ForEach(home.chipGenres) { genre in
                    genreButton(
                        name: genre.name, selected: home.selectedGenreID == genre.id, id: "genreChip-\(genre.id)",
                        color: Color(hex: genre.color), glyph: { GlyphIcon(icon: genre.icon, color: $0, size: 24) }
                    ) { home.selectedGenreID = genre.id }
                }
            }
            .padding(.horizontal, 16)
        }
    }

    /// ジャンルボタン(高さ 52・角丸 16)。非選択は白地+ジャンル色の線画だけ、選択は強調色の地+白の線画と名前(`GenrePill`)。
    private func genreButton<Glyph: View>(
        name: String, selected: Bool, id: String, color: Color = Palette.muted,
        @ViewBuilder glyph: @escaping (Color) -> Glyph, action: @escaping () -> Void
    ) -> some View {
        GenrePill(
            name: name, selected: selected, background: selected ? Palette.accent : color, foreground: .white,
            size: 52, radius: 16, nameIdentifier: id.replacingOccurrences(of: "genreChip-", with: "genreName-"),
            buttonIdentifier: id, glyph: glyph, action: action)
    }

    /// 画像と価格の札だけのタイル。タップで詳細シート、長押し(0.5 秒)でメニュー。
    private func tileView(_ tile: HomeTile) -> some View {
        ItemImageView(item: tile.item, genreColor: tile.genreColor)
            .frame(height: Metrics.tileHeight)
            .frame(maxWidth: .infinity)
            .overlay(alignment: .bottomTrailing) {
                if let tag = tile.priceTag { PriceTagView(tag: tag).padding(8) }
            }
            .clipShape(RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
            .genreRing(tile.genreColor)
            .contentShape(RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
            .onTapGesture { sheet = .detail(tile.item) }
            .onLongPressGesture(minimumDuration: HomeLayout.longPressSeconds) { menuTile = tile }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(tile.item.name)
            .accessibilityValue(tile.priceTag?.accessibilityText ?? "")
            .accessibilityAddTraits(.isButton)
            .accessibilityIdentifier("item-\(tile.item.id)")
    }

    @ViewBuilder
    private func sheetContent(_ sheet: HomeSheet) -> some View {
        let home = model.home
        switch sheet {
        case .detail(let item):
            ItemDetailSheet(item: item, genre: home.genres.first { $0.id == item.genreID }, sites: home.sites, service: model.service) { saved in
                Task { await model.home.upsert(saved) }
            }
        case .edit(let item):
            EditItemSheet(item: item, genres: home.genres, service: model.service) { saved in
                Task { await model.home.upsert(saved) }
            }
        case .register:
            RegisterSheet(genres: home.genres, initialGenreID: home.selectedGenreID, service: model.service) { created in
                Task { await model.home.upsert(created) }
            }
        case .settings:
            SettingsView()
        }
    }
}

/// タイル右下の価格の札。暗い半透明の地+白い文字(`¥880〜` は 14px/800、「出品なし」は 12px)。
private struct PriceTagView: View {
    let tag: PriceTag

    var body: some View {
        Text(tag.accessibilityText)
            .font(.wl(isPrice ? 14 : 12, isPrice ? .heavy : .medium))
            .foregroundStyle(isPrice ? Color.white : Color(hex: "#e6ebf2"))
            .padding(.horizontal, 10)
            .padding(.vertical, 4)
            .background(Palette.tagBackground, in: Capsule())
            .background(.ultraThinMaterial, in: Capsule())
            .accessibilityHidden(true)
    }

    private var isPrice: Bool {
        if case .price = tag { true } else { false }
    }
}

extension PriceTag {
    /// 札の文字(`¥880〜` / `出品なし`)
    var accessibilityText: String {
        switch self {
        case .price(let text): text
        case .noListing: "出品なし"
        }
    }
}
