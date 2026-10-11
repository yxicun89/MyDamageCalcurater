import PhotosUI
import SwiftUI
import WishlistCore

/// 追加シート(「+」)。入口(コピーした URL を追加 / URL を入力 / 写真)→ 確認(画像・名前・ジャンルの絵柄・「追加」)。
/// クリップボードは「コピーした URL を追加」を押した瞬間にだけ読む。
struct RegisterSheet: View {
    @State private var viewModel: RegisterViewModel
    @State private var pickerItem: PhotosPickerItem?
    @State private var showsURLField = false
    let genres: [Genre]
    let onCreated: (Item) -> Void

    @Environment(\.dismiss) private var dismiss

    init(genres: [Genre], initialGenreID: Int?, service: any WishlistService, onCreated: @escaping (Item) -> Void) {
        _viewModel = State(initialValue: RegisterViewModel(service: service, genres: genres, initialGenreID: initialGenreID))
        self.genres = genres
        self.onCreated = onCreated
    }

    private var sortedGenres: [Genre] {
        genres.sorted { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id }
    }

    private var isConfirming: Bool { viewModel.draft != nil || viewModel.photo != nil }

    var body: some View {
        ZStack {
            Palette.bg.ignoresSafeArea()
            VStack(spacing: 0) {
                if isConfirming {
                    confirmStep
                } else if showsURLField {
                    urlStep
                } else {
                    entryStep
                }
                Button("キャンセル") { dismiss() }
                    .font(.wl(15, .heavy))
                    .foregroundStyle(Palette.muted)
                    .padding(.vertical, 14)
                    .accessibilityIdentifier("registerCancel")
            }
            .padding(.horizontal, 20)
            .padding(.top, 24)
        }
        .presentationDetents(isConfirming ? [.large] : [.height(360), .large])
        .accessibilityIdentifier("registerSheet")
        .onChange(of: pickerItem) { _, newValue in
            Task {
                if let upload = await loadImageUpload(from: newValue) { viewModel.setPhoto(upload) }
            }
        }
    }

    // MARK: - 入口

    private var entryStep: some View {
        VStack(spacing: 12) {
            Button {
                // 押した瞬間にだけクリップボードを読む。URL でなければ入力欄を出す。
                Task {
                    if !(await viewModel.pasteURL(UIPasteboard.general.string)) { showsURLField = true }
                }
            } label: {
                Label("コピーした URL を追加", systemImage: "doc.on.clipboard")
            }
            .buttonStyle(PrimaryButtonStyle(height: 64, radius: 20))
            .disabled(viewModel.isBusy)
            .accessibilityIdentifier("pasteURLButton")
            HStack(spacing: 12) {
                Button("URL を入力") { showsURLField = true }
                    .buttonStyle(OutlineButtonStyle())
                    .accessibilityIdentifier("enterURLButton")
                PhotosPicker(selection: $pickerItem, matching: .images) {
                    Label("写真", systemImage: "photo")
                }
                .buttonStyle(OutlineButtonStyle())
                .accessibilityIdentifier("photoButton")
            }
            if let message = viewModel.errorMessage { errorText(message) }
            Text("Safari の共有からも追加できます").font(.wl(13)).foregroundStyle(Palette.muted).padding(.top, 4)
            Spacer(minLength: 0)
        }
    }

    // MARK: - URL の入力

    private var urlStep: some View {
        @Bindable var viewModel = viewModel
        return VStack(spacing: 12) {
            TextField("https://…", text: $viewModel.urlText)
                .keyboardType(.URL)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .font(.wl(16))
                .padding(.horizontal, 14)
                .frame(height: 52)
                .card(radius: Metrics.buttonRadius)
                .accessibilityIdentifier("urlField")
            Button(viewModel.isBusy ? "取得中…" : "取得") { Task { await viewModel.fetchDraft() } }
                .buttonStyle(PrimaryButtonStyle(height: 52, radius: Metrics.buttonRadius))
                .disabled(viewModel.isBusy || viewModel.urlText.trimmingCharacters(in: .whitespaces).isEmpty)
                .accessibilityIdentifier("fetchButton")
            if let message = viewModel.errorMessage { errorText(message) }
            Spacer(minLength: 0)
        }
    }

    // MARK: - 確認

    private var confirmStep: some View {
        @Bindable var viewModel = viewModel
        let genre = genres.first { $0.id == viewModel.genreID }
        let ring = genre?.color ?? GenreIcon.defaultColor
        return ScrollView {
            VStack(spacing: 16) {
                preview(ringColor: ring)
                if viewModel.draft != nil, viewModel.draft?.imageURL == nil, viewModel.photo == nil {
                    Text("このページの画像は取れませんでした").font(.wl(13)).foregroundStyle(Palette.muted)
                }
                TextField("名前", text: $viewModel.name)
                    .font(.wl(18, .heavy))
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 14)
                    .frame(height: 48)
                    .card(radius: Metrics.buttonRadius)
                    .accessibilityLabel("名前")
                    .accessibilityIdentifier("nameField")
                GenreChooser(genres: sortedGenres, selection: $viewModel.genreID)
                if let message = viewModel.errorMessage { errorText(message) }
                Button("追加") {
                    Task {
                        if let created = await viewModel.register() {
                            onCreated(created)
                            dismiss()
                        }
                    }
                }
                .buttonStyle(PrimaryButtonStyle())
                .disabled(!viewModel.canRegister || viewModel.isBusy)
                .accessibilityIdentifier("registerButton")
            }
        }
        .scrollBounceBehavior(.basedOnSize)
    }

    @ViewBuilder
    private func preview(ringColor: String) -> some View {
        if let photo = viewModel.photo, let image = UIImage(data: photo.data) {
            Image(uiImage: image).resizable().scaledToFill()
                .frame(width: 190, height: 190)
                .clipShape(RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
                .genreRing(ringColor)
                .accessibilityIdentifier("registerPreview")
        } else if let text = viewModel.draft?.imageURL, Deeplink.isHTTPURL(text), let url = URL(string: text) {
            AsyncImage(url: url) { $0.resizable().scaledToFill() } placeholder: { Palette.lineSoft }
                .frame(width: 190, height: 190)
                .clipShape(RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
                .genreRing(ringColor)
                .accessibilityIdentifier("registerPreview")
        } else {
            PhotosPicker(selection: $pickerItem, matching: .images) {
                Text("写真を選ぶ")
                    .font(.wl(16, .heavy))
                    .foregroundStyle(Palette.accent)
                    .frame(width: 200, height: 200)
                    .background(Palette.bg, in: RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous)
                            .strokeBorder(Color(hex: "#b9c8db"), style: StrokeStyle(lineWidth: 2, dash: [6, 5])))
            }
            .accessibilityIdentifier("choosePhotoFrame")
        }
    }

    private func errorText(_ message: String) -> some View {
        Text(message).font(.wl(13)).foregroundStyle(Palette.danger).multilineTextAlignment(.center)
            .accessibilityAddTraits(.updatesFrequently)
            .accessibilityIdentifier("registerError")
    }
}

/// 編集シート(長押し → 編集)。変えた項目だけ PATCH する。
struct EditItemSheet: View {
    @State private var viewModel: EditItemViewModel
    @State private var pickerItem: PhotosPickerItem?
    let item: Item
    let genres: [Genre]
    let onSaved: (Item) -> Void

    @Environment(\.dismiss) private var dismiss

    init(item: Item, genres: [Genre], service: any WishlistService, onSaved: @escaping (Item) -> Void) {
        _viewModel = State(initialValue: EditItemViewModel(item: item, genres: genres, service: service))
        self.item = item
        self.genres = genres
        self.onSaved = onSaved
    }

    var body: some View {
        @Bindable var viewModel = viewModel
        let genre = genres.first { $0.id == viewModel.genreID }
        let replaced = viewModel.replacementImage != nil
        NavigationStack {
            Form {
                Section {
                    HStack(spacing: 14) {
                        ItemImageView(item: item, genreColor: genre?.color ?? GenreIcon.defaultColor)
                            .frame(width: 88, height: 88)
                            .clipShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
                            .genreRing(genre?.color ?? GenreIcon.defaultColor, radius: 18)
                        PhotosPicker(selection: $pickerItem, matching: .images) {
                            Label(replaced ? "選び直す" : "画像を差し替え", systemImage: "photo")
                                .font(.wl(15, .heavy))
                        }
                    }
                    GenreChooser(genres: genres.sorted { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id }, selection: genreBinding, size: 48, radius: 16)
                }
                Section("内容") {
                    TextField("名前", text: $viewModel.name)
                    TextField("オプション(空でもよい)", text: $viewModel.optionText)
                    TextField("検索ワードの上書き(空ならテンプレート)", text: $viewModel.queryOverride)
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
                if let message = viewModel.errorMessage {
                    Section { Text(message).foregroundStyle(Palette.danger) }
                }
            }
            .scrollContentBackground(.hidden)
            .background(Palette.bg)
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

    /// `EditItemViewModel.genreID` は必須の Int。絵柄を押して替える。
    private var genreBinding: Binding<Int?> {
        Binding(get: { viewModel.genreID }, set: { if let id = $0 { viewModel.genreID = id } })
    }
}
