import SwiftUI
import WishlistCore

/// 設定画面。接続(API の URL とトークン)・ジャンル・サイト。
/// 接続が未設定のとき(`isInitialSetup`)は接続だけを出し、保存すると API を呼んでホームへ進む。
struct SettingsView: View {
    let isInitialSetup: Bool

    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var connection: ConnectionSettingsViewModel?
    @State private var list: SettingsListViewModel?
    @State private var editingGenre: GenreEditTarget?
    @State private var editingSite: SiteEditTarget?

    var body: some View {
        NavigationStack {
            Form {
                if let connection {
                    ConnectionForm(viewModel: connection) {
                        model.reconnect()
                        self.connection = ConnectionSettingsViewModel(store: model.settingsStore)
                        list = makeList()
                        if !isInitialSetup {
                            Task {
                                await model.home.loadCached()
                                await model.home.refresh()
                            }
                        }
                    }
                }
                if !isInitialSetup, let list {
                    Section("ジャンル") {
                        ForEach(list.sortedGenres) { genre in
                            Button(genre.name) { editingGenre = .edit(genre) }.foregroundStyle(.primary)
                        }
                        Button("ジャンルを追加", systemImage: "plus") { editingGenre = .add }
                    }
                    Section("サイト") {
                        ForEach(list.sites) { site in
                            Button(site.name) { editingSite = .edit(site) }.foregroundStyle(.primary)
                        }
                        Button("サイトを追加", systemImage: "plus") { editingSite = .add }
                    }
                    if let message = list.errorMessage {
                        Section { Text(message).foregroundStyle(.red) }
                    }
                }
            }
            .navigationTitle("設定")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                if !isInitialSetup {
                    ToolbarItem(placement: .confirmationAction) {
                        Button("閉じる") { dismiss() }
                    }
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
            if connection == nil { connection = ConnectionSettingsViewModel(store: model.settingsStore) }
            if list == nil { list = makeList() }
        }
    }

    private func makeList() -> SettingsListViewModel {
        SettingsListViewModel(service: model.service, genres: model.home.genres, sites: model.home.sites)
    }
}

/// 接続設定のフォーム(本体)。Share Extension にも同じ形のものがある。
struct ConnectionForm: View {
    @Bindable var viewModel: ConnectionSettingsViewModel
    let onSaved: () -> Void

    var body: some View {
        Section {
            TextField("API の URL(例 https://…/wishlist/)", text: $viewModel.apiBaseURLText)
                .keyboardType(.URL)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityIdentifier("connectionURLField")
            SecureField("トークン", text: $viewModel.token)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .accessibilityIdentifier("connectionTokenField")
            Button("保存") {
                if viewModel.save() { onSaved() }
            }
            if let message = viewModel.errorMessage {
                Text(message).font(.footnote).foregroundStyle(.red)
            }
        } header: {
            Text("接続")
        } footer: {
            Text("URL とトークンの両方を入れると一覧を読み込みます。")
        }
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

/// ジャンルの追加・編集。表示するサイトとその順序(「上へ」)を決める。
struct GenreEditSheet: View {
    let target: GenreEditTarget
    let sites: [Site]
    let viewModel: SettingsListViewModel
    let onDone: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var template = ""
    @State private var siteIDs: [Int] = []

    private var original: Genre? {
        if case .edit(let genre) = target { genre } else { nil }
    }

    var body: some View {
        NavigationStack {
            Form {
                TextField("名前", text: $name)
                TextField("検索ワードのテンプレート({name} {option})", text: $template)
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
                if let message = viewModel.errorMessage {
                    Section { Text(message).foregroundStyle(.red) }
                }
            }
            .navigationTitle(original == nil ? "ジャンルを追加" : "ジャンルを編集")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("キャンセル") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("保存") {
                        Task {
                            let saved: Genre?
                            if let original {
                                saved = await viewModel.updateGenre(original, name: name, queryTemplate: template, siteIDs: siteIDs)
                            } else {
                                saved = await viewModel.addGenre(name: name, queryTemplate: template, siteIDs: siteIDs)
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
                }
            }
        }
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
                    Section { Text(message).foregroundStyle(.red) }
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
