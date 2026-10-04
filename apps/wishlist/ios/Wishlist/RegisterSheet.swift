import PhotosUI
import SwiftUI
import WishlistCore

/// 登録シート(「+」)。写真を選ぶか、URL を貼って OGP から下書きを作る。
struct RegisterSheet: View {
    @State private var viewModel: RegisterViewModel
    @State private var pickerItem: PhotosPickerItem?
    let genres: [Genre]
    let onCreated: (Item) -> Void

    @Environment(\.dismiss) private var dismiss

    init(genres: [Genre], initialGenreID: Int?, service: any WishlistService, onCreated: @escaping (Item) -> Void) {
        _viewModel = State(initialValue: RegisterViewModel(service: service, genres: genres, initialGenreID: initialGenreID))
        self.genres = genres
        self.onCreated = onCreated
    }

    var body: some View {
        @Bindable var viewModel = viewModel
        NavigationStack {
            Form {
                Section("写真") {
                    PhotosPicker(selection: $pickerItem, matching: .images) {
                        Label(viewModel.photo == nil ? "写真を選ぶ" : "写真を選び直す", systemImage: "photo")
                    }
                    if let photo = viewModel.photo, let preview = UIImage(data: photo.data) {
                        Image(uiImage: preview).resizable().scaledToFit().frame(maxHeight: 160)
                    }
                }
                Section("URL から") {
                    TextField("https://…", text: $viewModel.urlText)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("取得") { Task { await viewModel.fetchDraft() } }
                        .disabled(viewModel.isBusy)
                }
                Section("内容") {
                    TextField("名前", text: $viewModel.name)
                    Picker("ジャンル", selection: $viewModel.genreID) {
                        ForEach(genres) { genre in Text(genre.name).tag(Optional(genre.id)) }
                    }
                }
                if let message = viewModel.errorMessage {
                    Section { Text(message).foregroundStyle(.red) }
                }
            }
            .accessibilityIdentifier("registerSheet")
            .navigationTitle("登録")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("キャンセル") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("登録") {
                        Task {
                            if let created = await viewModel.register() {
                                onCreated(created)
                                dismiss()
                            }
                        }
                    }
                    .disabled(!viewModel.canRegister || viewModel.isBusy)
                }
            }
            .onChange(of: pickerItem) { _, newValue in
                Task {
                    if let upload = await loadImageUpload(from: newValue) { viewModel.setPhoto(upload) }
                }
            }
        }
    }
}

/// 編集シート(長押し → 編集)。変えた項目だけ PATCH する。
struct EditItemSheet: View {
    @State private var viewModel: EditItemViewModel
    @State private var pickerItem: PhotosPickerItem?
    let genres: [Genre]
    let onSaved: (Item) -> Void

    @Environment(\.dismiss) private var dismiss

    init(item: Item, genres: [Genre], service: any WishlistService, onSaved: @escaping (Item) -> Void) {
        _viewModel = State(initialValue: EditItemViewModel(item: item, genres: genres, service: service))
        self.genres = genres
        self.onSaved = onSaved
    }

    var body: some View {
        @Bindable var viewModel = viewModel
        NavigationStack {
            Form {
                Section("内容") {
                    TextField("名前", text: $viewModel.name)
                    TextField("オプション(空でもよい)", text: $viewModel.optionText)
                    TextField("検索ワードの上書き(空ならテンプレート)", text: $viewModel.queryOverride)
                    Picker("ジャンル", selection: $viewModel.genreID) {
                        ForEach(genres) { genre in Text(genre.name).tag(genre.id) }
                    }
                    TextField("最低価格(円。空なら未設定)", text: $viewModel.minPriceText)
                        .keyboardType(.numberPad)
                    Toggle("公式ページを監視する", isOn: $viewModel.watchOfficial)
                        .disabled(!viewModel.canWatchOfficial)
                        .accessibilityIdentifier("watchOfficialToggle")
                    if let hint = viewModel.watchOfficialHint {
                        Text(hint).font(.footnote).foregroundStyle(.secondary)
                            .accessibilityIdentifier("watchOfficialHint")
                    }
                }
                Section("画像") {
                    PhotosPicker(selection: $pickerItem, matching: .images) {
                        Label(viewModel.replacementImage == nil ? "画像を差し替える" : "選び直す", systemImage: "photo")
                    }
                }
                if let message = viewModel.errorMessage {
                    Section { Text(message).foregroundStyle(.red) }
                }
            }
            .navigationTitle("編集")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("キャンセル") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("保存") {
                        Task {
                            if let saved = await viewModel.save() {
                                onSaved(saved)
                                dismiss()
                            }
                        }
                    }
                    .disabled(viewModel.isBusy)
                }
            }
            .onChange(of: pickerItem) { _, newValue in
                Task { viewModel.setReplacementImage(await loadImageUpload(from: newValue)) }
            }
        }
        .accessibilityIdentifier("editSheet")
    }
}
