import SwiftUI
import WishlistCore

/// 共有シートの画面。ページの画像(プレビュー)・名前(編集できる)・ジャンルの絵柄(推定したジャンルが選ばれている)・「追加」。
/// 接続先はビルド設定(Info.plist の `WishlistAPIBaseURL`)。未設定なら、その旨を出して閉じられる。
struct ShareView: View {
    let sharedPage: SharedPage
    let onFinish: () -> Void
    let onCancel: () -> Void

    @State private var viewModel: ShareViewModel?

    var body: some View {
        NavigationStack {
            ZStack {
                Palette.bg.ignoresSafeArea()
                if let viewModel {
                    content(viewModel)
                } else {
                    Text("読み込み中").font(.wl(14)).foregroundStyle(Palette.muted)
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
        let settings = WishlistSettings.fromInfo(Bundle.main.infoDictionary)
        let service = APIWishlistService(baseURL: settings.baseURL ?? URL(string: "http://localhost/")!)
        let model = ShareViewModel(sharedPage: sharedPage, settings: settings, service: service)
        viewModel = model
        await model.load()
    }

    @ViewBuilder
    private func content(_ model: ShareViewModel) -> some View {
        switch model.phase {
        case .needsSettings:
            message("接続先が未設定です(ビルド設定)", action: "閉じる", onCancel)
        case .loading:
            Text("読み込み中").font(.wl(14)).foregroundStyle(Palette.muted)
        case .ready, .saving:
            readyForm(model)
        case .saved(let item):
            VStack(spacing: 14) {
                Image(systemName: "checkmark.circle.fill").font(.system(size: 44)).foregroundStyle(.green)
                Text("「\(item.name)」を追加しました").font(.wl(16, .heavy)).foregroundStyle(Palette.fg)
                Button("閉じる", action: onFinish).buttonStyle(PrimaryButtonStyle(height: 52, radius: Metrics.buttonRadius))
            }
            .padding(24)
        case .failed(let text):
            VStack(spacing: 14) {
                Text(text).font(.wl(14)).foregroundStyle(Palette.danger).multilineTextAlignment(.center)
                if model.draft != nil {
                    Button("もう一度保存") { Task { await model.save() } }
                        .buttonStyle(PrimaryButtonStyle(height: 52, radius: Metrics.buttonRadius))
                } else {
                    Button("もう一度読み込む") { Task { await model.load() } }
                        .buttonStyle(OutlineButtonStyle(height: 52))
                }
            }
            .padding(24)
        }
    }

    private func message(_ text: String, action: String, _ perform: @escaping () -> Void) -> some View {
        VStack(spacing: 14) {
            Text(text).font(.wl(15, .heavy)).foregroundStyle(Palette.fg).multilineTextAlignment(.center)
                .accessibilityIdentifier("shareMessage")
            Button(action, action: perform).buttonStyle(OutlineButtonStyle(height: 52))
        }
        .padding(24)
    }

    private func readyForm(_ model: ShareViewModel) -> some View {
        @Bindable var model = model
        let genre = model.genres.first { $0.id == model.genreID }
        let ring = genre?.color ?? GenreIcon.defaultColor
        return ScrollView {
            VStack(spacing: 16) {
                if let url = model.previewImageURL {
                    AsyncImage(url: url) { $0.resizable().scaledToFill() } placeholder: { Palette.lineSoft }
                        .frame(width: 190, height: 190)
                        .clipShape(RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
                        .genreRing(ring)
                        .accessibilityIdentifier("sharePreview")
                } else {
                    Text("このページには画像がありません。追加するには画像が必要です。")
                        .font(.wl(13)).foregroundStyle(Palette.muted).multilineTextAlignment(.center)
                }
                TextField("名前", text: $model.name)
                    .font(.wl(18, .heavy))
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 14)
                    .frame(height: 48)
                    .card(radius: Metrics.buttonRadius)
                    .accessibilityLabel("名前")
                    .accessibilityIdentifier("shareNameField")
                GenreChooser(
                    genres: model.genres.sorted { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id },
                    selection: $model.genreID)
                Button("追加") { Task { await model.save() } }
                    .buttonStyle(PrimaryButtonStyle())
                    .disabled(!model.canSave || model.phase == .saving)
                    .accessibilityIdentifier("shareAddButton")
            }
            .padding(20)
        }
    }
}
