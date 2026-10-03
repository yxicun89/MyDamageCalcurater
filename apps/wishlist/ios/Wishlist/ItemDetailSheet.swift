import SwiftUI
import WishlistCore

/// 詳細シート(画像タップで下から出る)。画像・検索ワード(その場編集)・サマリ・サイト行。
struct ItemDetailSheet: View {
    @State private var viewModel: ItemDetailViewModel
    let onSaved: (Item) -> Void

    @Environment(\.dismiss) private var dismiss
    @FocusState private var queryFocused: Bool
    @State private var suspiciousExpanded = false

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

                if let notice = viewModel.noticeText {
                    Text(notice).font(.footnote).foregroundStyle(.secondary)
                }

                Button("更新") { Task { await viewModel.refresh() } }
                    .buttonStyle(.bordered)
                    .disabled(!viewModel.canRefresh)
                    .accessibilityIdentifier("refreshButton")

                siteRows

                if let title = viewModel.suspiciousTitle {
                    suspiciousSection(title: title)
                }
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
        .onDisappear { viewModel.stopPolling() }
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
            ForEach(viewModel.siteRows) { row in
                Button {
                    // http(s) 以外は開かない(SiteLinks.resolve でも除いているが、開く直前にも確認する)
                    if Deeplink.isHTTPURL(row.link.url.absoluteString) { UIApplication.shared.open(row.link.url) }
                } label: {
                    HStack(alignment: .top) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(row.link.site.name).font(.body.weight(.medium))
                            // 更新中の表示は文字だけ(スピナーは使わない)
                            if let estimate = row.estimateText {
                                Text([estimate, row.countText, row.stockText].compactMap { $0 }.joined(separator: "  "))
                                    .font(.footnote).foregroundStyle(.secondary)
                            }
                            if let note = row.noteText {
                                Text(note).font(.footnote).foregroundStyle(.secondary)
                            }
                        }
                        Spacer()
                        Image(systemName: "arrow.up.right.square").foregroundStyle(.secondary)
                    }
                    .padding(.horizontal, 16)
                    .padding(.vertical, 14)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
                .accessibilityLabel(row.accessibilityLabel)
                .accessibilityIdentifier("siteRow-\(row.id)")
            }
        }
    }

    /// 参考外の折りたたみ(ボタンで開閉する自前の折りたたみ。DisclosureGroup は識別子が子要素と重なるため使わない)。開いたときに出品を取る。
    private func suspiciousSection(title: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Button {
                suspiciousExpanded.toggle()
                if suspiciousExpanded, viewModel.suspicious == .notLoaded {
                    Task { await viewModel.loadSuspiciousListings() }
                }
            } label: {
                HStack {
                    Text(title)
                    Spacer()
                    Image(systemName: suspiciousExpanded ? "chevron.up" : "chevron.down")
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel(title)
            .accessibilityValue(suspiciousExpanded ? "開いています" : "閉じています")
            .accessibilityIdentifier("suspiciousDisclosure")

            if suspiciousExpanded {
                switch viewModel.suspicious {
                case .notLoaded, .loading:
                    Text("読み込み中").font(.footnote).foregroundStyle(.secondary)
                case .failed(let message):
                    Text(message).font(.footnote).foregroundStyle(.secondary)
                case .loaded(let rows):
                    VStack(alignment: .leading, spacing: 10) {
                        ForEach(rows) { row in suspiciousRow(row) }
                    }
                }
            }
        }
    }

    private func suspiciousRow(_ row: SuspiciousListingRow) -> some View {
        HStack(alignment: .top, spacing: 10) {
            if let imageURL = row.imageURL {
                AsyncImage(url: imageURL) { $0.resizable().scaledToFill() } placeholder: { Color.gray.opacity(0.2) }
                    .frame(width: 56, height: 56)
                    .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(row.title).font(.subheadline)
                Text(([row.priceText] + row.reasonTexts).joined(separator: "  ")).font(.footnote).foregroundStyle(.secondary)
                if let url = row.linkURL {
                    Button("開く") { UIApplication.shared.open(url) }.font(.footnote)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(([row.title, row.priceText] + row.reasonTexts).joined(separator: " "))
        .accessibilityIdentifier("suspiciousListing-\(row.id)")
    }
}
