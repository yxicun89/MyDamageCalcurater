import SwiftUI
import WishlistCore

/// 詳細シート(画像タップで下から出る)。画像・検索ワード(その場編集)・サマリ・サイト行。
struct ItemDetailSheet: View {
    @State private var viewModel: ItemDetailViewModel
    let onSaved: (Item) -> Void

    @Environment(\.dismiss) private var dismiss
    @FocusState private var queryFocused: Bool

    init(item: Item, genre: Genre?, sites: [Site], service: any WishlistService, onSaved: @escaping (Item) -> Void) {
        _viewModel = State(initialValue: ItemDetailViewModel(item: item, genre: genre, sites: sites, service: service))
        self.onSaved = onSaved
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                ItemImageView(item: viewModel.item, cornerRadius: 20)
                    .frame(maxHeight: 320)
                    .frame(maxWidth: .infinity)

                queryRow(viewModel: viewModel)

                Text(viewModel.summaryText)
                    .font(.headline)
                    .accessibilityIdentifier("summaryText")
                    .accessibilityLabel(viewModel.summaryText)

                siteRows
            }
            .padding(20)
        }
        .accessibilityIdentifier("detailSheet")
        .overlay(alignment: .topTrailing) {
            GlassCircleButton(systemImage: "xmark", label: "閉じる") { dismiss() }
                .accessibilityIdentifier("closeSheet")
                .padding(16)
        }
        .presentationDetents([.medium, .large])
        .task { await viewModel.loadEstimates() }
    }

    @ViewBuilder
    private func queryRow(viewModel: ItemDetailViewModel) -> some View {
        @Bindable var viewModel = viewModel
        VStack(alignment: .leading, spacing: 8) {
            if viewModel.isEditingQuery {
                TextField("検索ワード", text: $viewModel.queryDraft)
                    .textFieldStyle(.roundedBorder)
                    .focused($queryFocused)
                    .onAppear { queryFocused = true }
                    .accessibilityIdentifier("queryField")
                HStack {
                    Button("保存") {
                        Task {
                            if let saved = await viewModel.saveQuery() { onSaved(saved) }
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .accessibilityIdentifier("querySaveButton")
                    Button("やめる") { viewModel.cancelEditingQuery() }
                        .buttonStyle(.bordered)
                }
            } else {
                Button {
                    viewModel.beginEditingQuery()
                } label: {
                    Label(viewModel.displayQuery, systemImage: "magnifyingglass")
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(viewModel.displayQuery)
                .accessibilityIdentifier("queryEditButton")
            }
            if let message = viewModel.errorMessage {
                Text(message).font(.footnote).foregroundStyle(.red)
            }
        }
    }

    private var siteRows: some View {
        VStack(spacing: 8) {
            ForEach(viewModel.links, id: \.site.id) { link in
                Button {
                    // http(s) 以外は開かない(SiteLinks.resolve でも除いているが、開く直前にも確認する)
                    if Deeplink.isHTTPURL(link.url.absoluteString) { UIApplication.shared.open(link.url) }
                } label: {
                    HStack {
                        Text(link.site.name).font(.body.weight(.medium))
                        Spacer()
                        Image(systemName: "arrow.up.right.square").foregroundStyle(.secondary)
                    }
                    .padding(.horizontal, 16)
                    .padding(.vertical, 14)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
                .accessibilityIdentifier("siteRow-\(link.site.id)")
            }
        }
    }
}
