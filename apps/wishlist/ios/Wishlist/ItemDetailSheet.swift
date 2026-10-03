import Charts
import SwiftUI
import WishlistCore

/// 詳細シート(画像タップで下から出る)。画像・検索ワード(その場編集)・サマリ・サイト行。
struct ItemDetailSheet: View {
    @State private var viewModel: ItemDetailViewModel
    let onSaved: (Item) -> Void

    @Environment(\.dismiss) private var dismiss
    @FocusState private var queryFocused: Bool
    @State private var suspiciousExpanded = false
    @State private var historyExpanded = false

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

                historySection
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

    /// 価格の推移の折りたたみ(参考外と同じ自前の作り)。開いたときに推移を取る。常時動くアニメーション・スピナーは使わない。
    private var historySection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Button {
                historyExpanded.toggle()
                if historyExpanded { Task { await viewModel.loadPriceHistory() } }
            } label: {
                HStack {
                    Text(WishlistText.priceHistoryTitle)
                    Spacer()
                    Image(systemName: historyExpanded ? "chevron.up" : "chevron.down")
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel(WishlistText.priceHistoryTitle)
            .accessibilityValue(historyExpanded ? "開いています" : "閉じています")
            .accessibilityIdentifier("historyDisclosure")

            if historyExpanded {
                switch viewModel.priceHistory {
                case .notLoaded, .loading:
                    Text("読み込み中…").font(.footnote).foregroundStyle(.secondary)
                        .accessibilityIdentifier("historyMessage")
                case .failed(let message):
                    Text(message).font(.footnote).foregroundStyle(.secondary)
                        .accessibilityIdentifier("historyMessage")
                case .empty:
                    Text(WishlistText.priceHistoryEmpty).font(.footnote).foregroundStyle(.secondary)
                        .accessibilityLabel(WishlistText.priceHistoryEmpty)
                        .accessibilityIdentifier("historyEmpty")
                case .loaded(let chart):
                    historyChart(chart)
                    historyLegend
                }
            }
        }
    }

    private func historyChart(_ chart: PriceHistoryChart) -> some View {
        Chart {
            ForEach(viewModel.visibleHistorySeries) { series in
                ForEach(series.points) { point in
                    LineMark(
                        x: .value("日", point.date, unit: .day), y: .value("最安", point.value),
                        series: .value("線", "\(series.id)-\(point.segment)")
                    )
                    .foregroundStyle(by: .value("サイト", series.name))
                    PointMark(x: .value("日", point.date, unit: .day), y: .value("最安", point.value))
                        .foregroundStyle(by: .value("サイト", series.name))
                }
            }
        }
        .chartLegend(.hidden)
        .chartYScale(domain: .automatic(includesZero: false))
        .frame(height: 180)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(chart.accessibilityLabel)
        .accessibilityIdentifier("priceHistoryChart")
    }

    private var historyLegend: some View {
        WrappingHStack(spacing: 8) {
            ForEach(viewModel.historyLegend) { item in
                Button(item.name) { viewModel.toggleHistorySite(item.siteID) }
                    .buttonStyle(.bordered)
                    .tint(item.isSelected ? .accentColor : .secondary)
                    .accessibilityLabel(item.name)
                    .accessibilityValue(item.isSelected ? "表示中" : "非表示")
                    .accessibilityIdentifier("historySite-\(item.siteID)")
            }
        }
    }
}

/// 子を横に並べ、幅に収まらなければ次の行へ折り返す(凡例用)。
private struct WrappingHStack: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        arrange(width: proposal.width ?? .infinity, subviews: subviews).size
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let result = arrange(width: bounds.width, subviews: subviews)
        for (index, origin) in result.origins.enumerated() {
            subviews[index].place(at: CGPoint(x: bounds.minX + origin.x, y: bounds.minY + origin.y), proposal: .unspecified)
        }
    }

    private func arrange(width: CGFloat, subviews: Subviews) -> (origins: [CGPoint], size: CGSize) {
        var origins: [CGPoint] = []
        var x: CGFloat = 0
        var y: CGFloat = 0
        var rowHeight: CGFloat = 0
        var maxX: CGFloat = 0
        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if x > 0, x + size.width > width {
                x = 0
                y += rowHeight + spacing
                rowHeight = 0
            }
            origins.append(CGPoint(x: x, y: y))
            x += size.width + spacing
            rowHeight = max(rowHeight, size.height)
            maxX = max(maxX, x - spacing)
        }
        return (origins, CGSize(width: maxX, height: y + rowHeight))
    }
}
