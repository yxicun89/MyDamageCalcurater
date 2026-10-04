import SwiftUI
import WishlistCore

/// 共有シートの画面。OGP のプレビュー(名前・画像)→ ジャンルを選ぶ → 保存。接続設定が無ければ設定から。
struct ShareView: View {
    let sharedURL: URL?
    let onFinish: () -> Void
    let onCancel: () -> Void

    private let store = UserDefaultsSettingsStore()
    @State private var viewModel: ShareViewModel?
    @State private var connection: ConnectionSettingsViewModel?

    var body: some View {
        NavigationStack {
            Group {
                if let viewModel {
                    content(viewModel)
                } else {
                    ProgressView()
                }
            }
            .navigationTitle("欲しいもの")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("キャンセル", action: onCancel) }
            }
        }
        .task { await start() }
    }

    private func start() async {
        let settings = store.load()
        let service = APIWishlistService(baseURL: settings.baseURL ?? URL(string: "http://localhost/")!, token: settings.token)
        let model = ShareViewModel(sharedURL: sharedURL, settings: settings, service: service)
        viewModel = model
        connection = ConnectionSettingsViewModel(store: store)
        await model.load()
    }

    @ViewBuilder
    private func content(_ model: ShareViewModel) -> some View {
        switch model.phase {
        case .needsSettings:
            settingsForm
        case .loading:
            ProgressView("読み込み中")
        case .ready, .saving:
            readyForm(model)
        case .saved(let item):
            VStack(spacing: 12) {
                Image(systemName: "checkmark.circle.fill").font(.largeTitle).foregroundStyle(.green)
                Text("「\(item.name)」を登録しました")
                Button("閉じる", action: onFinish).buttonStyle(.borderedProminent)
            }
            .padding()
        case .failed(let message):
            VStack(spacing: 12) {
                Text(message).multilineTextAlignment(.center).foregroundStyle(.red)
                if model.draft != nil {
                    Button("もう一度保存") { Task { await model.save() } }.buttonStyle(.borderedProminent)
                } else {
                    Button("もう一度読み込む") { Task { await model.load() } }.buttonStyle(.bordered)
                }
            }
            .padding()
        }
    }

    @ViewBuilder
    private var settingsForm: some View {
        if let connection {
            Form {
                Section {
                    TextField("API の URL(例 https://…/wishlist/)", text: Bindable(connection).apiBaseURLText)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    SecureField("トークン", text: Bindable(connection).token)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("保存して読み込む") {
                        if connection.save() { Task { await start() } }
                    }
                    if let message = connection.errorMessage {
                        Text(message).font(.footnote).foregroundStyle(.red)
                    }
                } header: {
                    Text("接続の設定が必要です")
                } footer: {
                    Text("共有拡張は本体アプリと設定を共有しません。ここでも URL とトークンを入れてください。")
                }
            }
        }
    }

    private func readyForm(_ model: ShareViewModel) -> some View {
        @Bindable var model = model
        return Form {
            if let url = model.previewImageURL {
                AsyncImage(url: url) { image in
                    image.resizable().scaledToFit()
                } placeholder: {
                    ProgressView()
                }
                .frame(maxWidth: .infinity, maxHeight: 220)
            } else {
                Text("このページには画像がありません。登録するには画像が必要です。")
                    .font(.footnote).foregroundStyle(.secondary)
            }
            TextField("名前", text: $model.name)
            Picker("ジャンル", selection: $model.genreID) {
                ForEach(model.genres) { genre in Text(genre.name).tag(Optional(genre.id)) }
            }
            Button("保存") { Task { await model.save() } }
                .disabled(!model.canSave || model.phase == .saving)
        }
    }
}
