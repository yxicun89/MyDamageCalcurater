import SwiftUI
import WishlistCore

/// 設定画面。ジャンル(絵柄・名前・件数)・サイト。接続の欄は無い(接続先はビルド設定)。
struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var list: SettingsListViewModel?
    @State private var editingGenre: GenreEditTarget?
    @State private var editingSite: SiteEditTarget?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    if let list {
                        genreSection(list)
                        siteSection(list)
                        if let message = list.errorMessage {
                            Text(message).font(.wl(13)).foregroundStyle(Palette.danger)
                        }
                    }
                }
                .padding(20)
            }
            .background(Palette.bg)
            .navigationTitle("設定")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("閉じる") { dismiss() }
                }
            }
            .sheet(item: $editingGenre) { target in
                if let list {
                    GenreEditSheet(target: target, sites: list.sites, viewModel: list) {
                        Task { await model.home.setGenres(list.genres) }
                    }
                }
            }
            .sheet(item: $editingSite) { target in
                if let list {
                    SiteEditSheet(target: target, viewModel: list) {
                        Task { await model.home.setSites(list.sites) }
                    }
                }
            }
        }
        .accessibilityIdentifier("settingsScreen")
        .onAppear {
            if list == nil { list = makeList() }
        }
    }

    private func genreSection(_ list: SettingsListViewModel) -> some View {
        let counts = SettingsListViewModel.itemCounts(model.home.items)
        return VStack(alignment: .leading, spacing: 8) {
            Text("ジャンル").font(.wl(14, .heavy)).foregroundStyle(Palette.muted)
            VStack(spacing: 0) {
                ForEach(list.sortedGenres) { genre in
                    Button { editingGenre = .edit(genre) } label: {
                        HStack(spacing: 12) {
                            GlyphIcon(icon: genre.icon, color: .white, size: 20)
                                .frame(width: 36, height: 36)
                                .background(Color(hex: genre.color), in: RoundedRectangle(cornerRadius: 11, style: .continuous))
                                .accessibilityIdentifier("genreIcon-\(genre.id)")
                            Text(genre.name).font(.wl(16)).foregroundStyle(Palette.fg)
                            Spacer()
                            Text("\(counts[genre.id] ?? 0)").font(.wl(13)).foregroundStyle(Palette.muted)
                                .accessibilityIdentifier("genreCount-\(genre.id)")
                            Image(systemName: "chevron.right").font(.system(size: 12, weight: .bold)).foregroundStyle(Palette.muted)
                        }
                        .padding(.horizontal, 14)
                        .frame(height: 58)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(genre.name)を編集")
                    .accessibilityValue("\(counts[genre.id] ?? 0)件")
                    .accessibilityAddTraits(.isButton)
                    .accessibilityIdentifier("editGenre-\(genre.id)")
                    Palette.lineSoft.frame(height: 1)
                }
                Button { editingGenre = .add } label: {
                    Label("ジャンルを追加", systemImage: "plus").font(.wl(15, .heavy)).foregroundStyle(Palette.accent)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, 14).frame(height: 52)
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("addGenreButton")
            }
            .card(radius: 20)
        }
    }

    private func siteSection(_ list: SettingsListViewModel) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("サイト").font(.wl(14, .heavy)).foregroundStyle(Palette.muted)
            VStack(spacing: 0) {
                ForEach(list.sites) { site in
                    Button { editingSite = .edit(site) } label: {
                        HStack(spacing: 12) {
                            SiteMarkView(name: site.name, size: 30)
                            Text(site.name).font(.wl(16)).foregroundStyle(Palette.fg)
                            Spacer()
                            Image(systemName: "chevron.right").font(.system(size: 12, weight: .bold)).foregroundStyle(Palette.muted)
                        }
                        .padding(.horizontal, 14)
                        .frame(height: 52)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(site.name)を編集")
                    .accessibilityAddTraits(.isButton)
                    .accessibilityIdentifier("editSite-\(site.id)")
                    Palette.lineSoft.frame(height: 1)
                }
                Button { editingSite = .add } label: {
                    Label("サイトを追加", systemImage: "plus").font(.wl(15, .heavy)).foregroundStyle(Palette.accent)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, 14).frame(height: 52)
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("addSiteButton")
            }
            .card(radius: 20)
        }
    }

    private func makeList() -> SettingsListViewModel {
        SettingsListViewModel(service: model.service, genres: model.home.genres, sites: model.home.sites)
    }
}

enum GenreEditTarget: Identifiable {
    case add
    case edit(Genre)
    var id: String {
        switch self {
        case .add: "add"
        case .edit(let genre): "edit-\(genre.id)"
        }
    }
}

enum SiteEditTarget: Identifiable {
    case add
    case edit(Site)
    var id: String {
        switch self {
        case .add: "add"
        case .edit(let site): "edit-\(site.id)"
        }
    }
}

/// 別名グループの 1 行(編集中の値。ID で行を区別する)
private struct AliasLine: Identifiable {
    let id = UUID()
    var text: String
}

/// ジャンルの追加・編集。名前・検索ワード・色(12 色)・絵柄(6 つ)・表示するサイトとその順序(「上へ」)・URL の手がかり・別名グループ。
struct GenreEditSheet: View {
    let target: GenreEditTarget
    let sites: [Site]
    let viewModel: SettingsListViewModel
    let onDone: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var template = ""
    @State private var siteIDs: [Int] = []
    @State private var color = GenreIcon.defaultColor
    @State private var icon = GenreIcon.box
    /// URL の手がかり(`,`・`、`・改行区切り)
    @State private var hostsText = ""
    @State private var keywordsText = ""
    /// 表記揺れの辞書(1 グループ = 1 行。カンマ区切り)
    @State private var aliasLines: [AliasLine] = []

    private var original: Genre? {
        if case .edit(let genre) = target { genre } else { nil }
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    HStack(spacing: 14) {
                        GlyphIcon(icon: icon, color: .white, size: 32)
                            .frame(width: 60, height: 60)
                            .background(Color(hex: color), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
                            .accessibilityIdentifier("genrePreviewIcon")
                        TextField("ジャンル名", text: $name)
                            .font(.wl(18, .heavy))
                            .accessibilityLabel("ジャンル名")
                            .accessibilityIdentifier("genreNameField")
                    }
                    TextField("検索ワードのテンプレート({name} {option})", text: $template)
                }
                Section("色") {
                    swatchGrid
                }
                Section("絵柄") {
                    iconRow
                }
                Section {
                    preview
                } header: {
                    Text("ホームでの見え方")
                }
                Section("別名グループ(同じ意味の語をカンマで区切る)") {
                    // 行に ID を持たせる(添字で回すと、削除で範囲外を読んで落ちることがある)
                    ForEach($aliasLines) { $line in
                        let number = (aliasLines.firstIndex { $0.id == line.id } ?? 0) + 1
                        HStack {
                            TextField("例: S.H.Figuarts, SHフィギュアーツ", text: $line.text)
                                .accessibilityIdentifier("aliasLine-\(number)")
                            Button("削除", systemImage: "minus.circle") { aliasLines.removeAll { $0.id == line.id } }
                                .labelStyle(.iconOnly)
                                .buttonStyle(.borderless)
                                .accessibilityIdentifier("removeAliasLine-\(number)")
                        }
                    }
                    Button("別名グループを追加", systemImage: "plus") { aliasLines.append(AliasLine(text: "")) }
                        .accessibilityIdentifier("addAliasLine")
                }
                Section("表示するサイト(上から順)") {
                    ForEach(siteIDs, id: \.self) { id in
                        HStack {
                            Text(sites.first { $0.id == id }?.name ?? "#\(id)")
                            Spacer()
                            Button("上へ", systemImage: "arrow.up") { siteIDs = SettingsListViewModel.movingUp(siteIDs, id: id) }
                                .labelStyle(.iconOnly)
                            Button("外す", systemImage: "minus.circle") { siteIDs.removeAll { $0 == id } }
                                .labelStyle(.iconOnly)
                        }
                        .buttonStyle(.borderless)
                    }
                    ForEach(sites.filter { !siteIDs.contains($0.id) }) { site in
                        Button("\(site.name) を追加", systemImage: "plus") { siteIDs.append(site.id) }
                    }
                }
                Section {
                    TextField("例: dm.takaratomy.co.jp", text: $hostsText, axis: .vertical)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .accessibilityLabel("手がかりのホスト")
                        .accessibilityIdentifier("clueHosts")
                    TextField("例: デュエマ, DM", text: $keywordsText, axis: .vertical)
                        .autocorrectionDisabled()
                        .accessibilityLabel("手がかりの語")
                        .accessibilityIdentifier("clueKeywords")
                } header: {
                    Text("URL の手がかり")
                } footer: {
                    Text("URL を追加するとき、ホストや語が合うジャンルを自動で選びます。カンマ・読点・改行で区切れます。")
                }
                if let message = viewModel.errorMessage {
                    Section { Text(message).foregroundStyle(Palette.danger) }
                }
            }
            .scrollContentBackground(.hidden)
            .background(Palette.bg)
            .navigationTitle(original == nil ? "ジャンルを追加" : "ジャンルを編集")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("キャンセル") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("保存") {
                        Task {
                            let edit = GenreEdit(
                                name: name, queryTemplate: template, siteIDs: siteIDs, aliasLines: aliasLines.map(\.text),
                                color: color, icon: icon, hostsText: hostsText, keywordsText: keywordsText)
                            let saved: Genre?
                            if let original {
                                saved = await viewModel.updateGenre(original, edit: edit)
                            } else {
                                saved = await viewModel.addGenre(edit)
                            }
                            if saved != nil {
                                onDone()
                                dismiss()
                            }
                        }
                    }
                }
            }
            .onAppear {
                if let original {
                    name = original.name
                    template = original.queryTemplate
                    siteIDs = original.siteIDs
                    color = original.color
                    icon = original.icon
                    hostsText = original.matchHosts.joined(separator: ", ")
                    keywordsText = original.matchKeywords.joined(separator: ", ")
                    aliasLines = SettingsListViewModel.aliasLines(of: original).map { AliasLine(text: $0) }
                }
            }
        }
    }

    /// 色の見本(12 色。6 列 2 行の丸)。選択中は白 3px + その色のリングにチェック。
    private var swatchGrid: some View {
        LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 10), count: 6), spacing: 10) {
            ForEach(GenrePalette.swatches, id: \.hex) { swatch in
                let selected = color == swatch.hex
                Button { color = swatch.hex } label: {
                    Circle().fill(Color(hex: swatch.hex))
                        .aspectRatio(1, contentMode: .fit)
                        .overlay {
                            if selected { Image(systemName: "checkmark").font(.system(size: 14, weight: .heavy)).foregroundStyle(.white) }
                        }
                        .overlay { if selected { Circle().strokeBorder(.white, lineWidth: 3) } }
                        .background { if selected { Circle().fill(Color(hex: swatch.hex)).padding(-5) } }
                }
                .buttonStyle(.plain)
                .accessibilityLabel(swatch.name)
                .accessibilityAddTraits(selected ? .isSelected : [])
                .accessibilityIdentifier("swatch-\(swatch.name)")
            }
        }
        .padding(.vertical, 6)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("色")
    }

    /// 絵柄の選択(6 つ)。ラベルは カード・フィギュア・ロボ・グッズ・星・箱。
    private var iconRow: some View {
        HStack(spacing: 10) {
            ForEach(GenreIcon.allCases, id: \.self) { choice in
                let selected = icon == choice
                Button { icon = choice } label: {
                    GlyphIcon(icon: choice, color: selected ? .white : Color(hex: color), size: 22)
                        .frame(maxWidth: .infinity)
                        .aspectRatio(1, contentMode: .fit)
                        .background(selected ? Color(hex: color) : Palette.surface, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
                        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(Palette.line, lineWidth: selected ? 0 : 1))
                }
                .buttonStyle(.plain)
                .accessibilityLabel(choice.label)
                .accessibilityAddTraits(selected ? .isSelected : [])
                .accessibilityIdentifier("iconChoice-\(choice.rawValue)")
            }
        }
        .padding(.vertical, 6)
        .accessibilityElement(children: .contain)
        .accessibilityLabel("絵柄")
    }

    /// 「ホームでの見え方」(64x78・角丸 14・ジャンル色の枠)。選んだ色にすぐ追従する。
    private var preview: some View {
        RoundedRectangle(cornerRadius: 14, style: .continuous)
            .fill(Color(hex: color).opacity(0.14))
            .frame(width: 64, height: 78)
            .overlay { GlyphIcon(icon: icon, color: Color(hex: color), size: 26) }
            .genreRing(color, radius: 14)
            .frame(maxWidth: .infinity)
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("ホームでの見え方")
            .accessibilityIdentifier("genrePreview")
    }
}

/// サイトの追加・編集。検索 URL は `{q}` を含む http(s) だけ。
struct SiteEditSheet: View {
    let target: SiteEditTarget
    let viewModel: SettingsListViewModel
    let onDone: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var template = ""
    @State private var fetchType = FetchType.linkOnly
    @State private var isReference = false

    private var original: Site? {
        if case .edit(let site) = target { site } else { nil }
    }

    var body: some View {
        NavigationStack {
            Form {
                TextField("名前", text: $name)
                TextField("検索 URL({q} を含む)", text: $template)
                    .keyboardType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                Picker("取得方式", selection: $fetchType) {
                    ForEach(FetchType.allCases, id: \.self) { Text($0.rawValue).tag($0) }
                }
                Toggle("基準価格に使う", isOn: $isReference)
                if let message = viewModel.errorMessage {
                    Section { Text(message).foregroundStyle(Palette.danger) }
                }
            }
            .navigationTitle(original == nil ? "サイトを追加" : "サイトを編集")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("キャンセル") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("保存") {
                        Task {
                            let saved: Site?
                            if let original {
                                saved = await viewModel.updateSite(
                                    original, name: name, searchURLTemplate: template, fetchType: fetchType, isReference: isReference)
                            } else {
                                saved = await viewModel.addSite(
                                    name: name, searchURLTemplate: template, fetchType: fetchType, isReference: isReference)
                            }
                            if saved != nil {
                                onDone()
                                dismiss()
                            }
                        }
                    }
                }
            }
            .onAppear {
                if let original {
                    name = original.name
                    template = original.searchURLTemplate
                    fetchType = original.fetchType
                    isReference = original.isReference
                }
            }
        }
    }
}
