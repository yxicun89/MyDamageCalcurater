import Charts
import SwiftUI
import WishlistCore

/// 詳細シート(画像タップで下から出る)。画像・ジャンルの点と名前と取得日・検索ワード(その場編集)・価格の帯・サイトのタイル・あやしい出品。
struct ItemDetailSheet: View {
    @State private var viewModel: ItemDetailViewModel
    let genre: Genre?
    let onSaved: (Item) -> Void

    @Environment(\.dismiss) private var dismiss
    @FocusState private var queryFocused: Bool
    @State private var showsSuspicious = false
    @State private var historyExpanded = false

    private var genreColor: String { genre?.color ?? GenreIcon.defaultColor }

    init(item: Item, genre: Genre?, sites: [Site], service: any WishlistService, onSaved: @escaping (Item) -> Void) {
        _viewModel = State(initialValue: ItemDetailViewModel(item: item, genre: genre, sites: sites, service: service))
        self.genre = genre
        self.onSaved = onSaved
    }

    var body: some View {
        ZStack {
            Palette.bg.ignoresSafeArea()
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    if showsSuspicious {
                        suspiciousList
                    } else {
                        main
                    }
                }
                .padding(20)
                .padding(.top, 8)
            }
        }
        .accessibilityIdentifier("detailSheet")
        .overlay(alignment: .topTrailing) {
            GlassCircleButton(systemImage: "xmark", label: "閉じる") { dismiss() }
                .accessibilityIdentifier("closeSheet")
                .padding(16)
        }
        .presentationDetents([.large])
        .task { await viewModel.loadEstimates() }
        .onDisappear { viewModel.stopPolling() }
    }

    private var main: some View {
        VStack(alignment: .leading, spacing: 16) {
            ItemImageView(item: viewModel.item, genreColor: genreColor, fit: true)
                .padding(10)
                .frame(width: 300, height: 300)
                .background(Palette.surface, in: RoundedRectangle(cornerRadius: 24, style: .continuous))
                .genreRing(genreColor, radius: 24)
                .frame(maxWidth: .infinity)

            titleRow
            queryRow(viewModel: viewModel)
            priceCard
            statusLine

            siteTiles

            if let official = viewModel.official { officialSection(official) }

            Button("更新") { Task { await viewModel.refresh() } }
                .buttonStyle(OutlineButtonStyle(height: 44))
                .disabled(!viewModel.canRefresh)
                .accessibilityIdentifier("refreshButton")

            if viewModel.suspiciousTotal > 0 { suspiciousButton }

            historySection
        }
    }

    /// ジャンル色の点・名前・取得日(`summaryFetchedAt` の JST `M/D`。無ければ出さない)
    private var titleRow: some View {
        HStack(spacing: 8) {
            Circle().fill(Color(hex: genreColor)).frame(width: 10, height: 10).accessibilityHidden(true)
                .accessibilityIdentifier("genreDot")
            Text(viewModel.item.name).font(.wl(15, .heavy)).foregroundStyle(Palette.fg).lineLimit(1)
            Spacer(minLength: 0)
            if let date = viewModel.estimates?.summaryFetchedAt {
                HStack(spacing: 4) {
                    Image(systemName: "clock").font(.system(size: 11, weight: .semibold))
                    Text(PriceFormat.jstDate(date)).font(.wl(12))
                }
                .foregroundStyle(Palette.muted)
                .accessibilityElement(children: .ignore)
                .accessibilityLabel("取得日 \(PriceFormat.jstDate(date))")
                .accessibilityIdentifier("fetchedAt")
            }
        }
    }

    /// 価格の帯 / 「出品ないかも」/ 空の枠(文は出さない。状態は `summaryText` のラベルで伝える)
    @ViewBuilder
    private var priceCard: some View {
        switch viewModel.summary {
        case .priced(let low, let mid, _):
            let band = PriceBand.make(sites: viewModel.links.map(\.site), estimates: viewModel.estimates?.sites ?? [])
            VStack(alignment: .leading, spacing: 10) {
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Text(PriceFormat.yen(low)).font(.wl(30, .heavy)).foregroundStyle(Palette.fg)
                    Text(mid.map { "〜 \(PriceFormat.yen($0))" } ?? "〜").font(.wl(16, .heavy)).foregroundStyle(Palette.muted)
                }
                PriceBandView(band: band, sites: viewModel.links.map(\.site))
            }
            .padding(18)
            .frame(maxWidth: .infinity, alignment: .leading)
            .card(radius: 20)
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(viewModel.summaryText)
            .accessibilityIdentifier("summaryText")
        case .noListings:
            HStack(spacing: 10) {
                Image(systemName: "magnifyingglass").font(.system(size: 18, weight: .semibold)).foregroundStyle(Palette.muted)
                Text(WishlistText.noListings).font(.wl(17, .heavy)).foregroundStyle(Palette.fg)
            }
            .frame(maxWidth: .infinity, minHeight: 84)
            .card(radius: 20)
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(viewModel.summaryText)
            .accessibilityIdentifier("summaryText")
        case .loading, .noPriceInfo, .offline:
            // 薄い灰色の空の枠(動かない・小さな時計のアイコンだけ)
            Image(systemName: "clock").font(.system(size: 14)).foregroundStyle(Palette.muted.opacity(0.6))
                .frame(maxWidth: .infinity, minHeight: 84)
                .background(Palette.lineSoft.opacity(0.6), in: RoundedRectangle(cornerRadius: 20, style: .continuous))
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(viewModel.summaryText)
                .accessibilityIdentifier("summaryText")
        }
    }

    /// 更新中・失敗の短い文(成功して更新中でもないときは出さない)
    @ViewBuilder
    private var statusLine: some View {
        if let notice = viewModel.noticeText {
            Text(notice).font(.wl(12)).foregroundStyle(Palette.muted)
        } else if viewModel.isRefreshing {
            Text(WishlistText.refreshing).font(.wl(12)).foregroundStyle(Palette.muted)
        }
    }

    // MARK: - サイトのタイル

    private var siteTiles: some View {
        let tiles = SiteTile.make(rows: viewModel.siteRows, estimates: viewModel.estimates, now: Date())
        // 3 列。サイトは数個なので、遅延させず全部を組む(画面の外のタイルも支援技術・UI テストから見える)
        let rows = stride(from: 0, to: tiles.count, by: 3).map { Array(tiles[$0..<min($0 + 3, tiles.count)]) }
        return VStack(spacing: 10) {
            ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                HStack(spacing: 10) {
                    ForEach(row) { tile in siteTile(tile) }
                    ForEach(0..<(3 - row.count), id: \.self) { _ in Color.clear.frame(maxWidth: .infinity, minHeight: 76) }
                }
            }
        }
    }

    private func siteTile(_ tile: SiteTile) -> some View {
        Button {
            // http(s) 以外は開かない(SiteLinks.resolve でも除いているが、開く直前にも確認する)
            if Deeplink.isHTTPURL(tile.url.absoluteString) { UIApplication.shared.open(tile.url) }
        } label: {
            VStack(spacing: 6) {
                SiteMarkView(name: tile.name, dim: tile.isDim)
                Text(tile.priceText ?? " ")
                    .font(.wl(13, .heavy))
                    .foregroundStyle(tile.isBest ? Palette.accent : (tile.isDim ? Palette.muted.opacity(0.7) : Palette.fg))
                    .lineLimit(1)
                    .minimumScaleFactor(0.7)
            }
            .frame(maxWidth: .infinity, minHeight: 76)
            .card(radius: 16)
            .overlay {
                if tile.isBest {
                    RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(Palette.accent, lineWidth: 2)
                }
            }
            .contentShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
        }
        .buttonStyle(.plain)
        .accessibilityLabel(tile.accessibilityLabel)
        .accessibilityAddTraits(tile.isBest ? .isSelected : [])
        .accessibilityIdentifier("siteRow-\(tile.id)")
    }

    // MARK: - あやしい出品

    private var suspiciousButton: some View {
        let title = "あやしい出品 \(viewModel.suspiciousTotal)"
        return Button {
            showsSuspicious = true
            if viewModel.suspicious == .notLoaded { Task { await viewModel.loadSuspiciousListings() } }
        } label: {
            HStack(spacing: 8) {
                Image(systemName: "exclamationmark.triangle").font(.system(size: 14, weight: .semibold))
                Text(title).font(.wl(14, .heavy))
                Spacer()
                Image(systemName: "chevron.right").font(.system(size: 12, weight: .bold))
            }
            .foregroundStyle(Palette.warn)
            .padding(.horizontal, 14)
            .frame(height: 44)
            .card(radius: Metrics.buttonRadius)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(title)
        .accessibilityValue("閉じています")
        .accessibilityIdentifier("suspiciousDisclosure")
    }

    /// 「戻る」と見出し「あやしい出品 N」+ 出品の一覧(同じシートの中身が替わる)
    private var suspiciousList: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Button {
                    showsSuspicious = false
                } label: {
                    Label("戻る", systemImage: "chevron.left").font(.wl(15, .heavy))
                }
                .accessibilityIdentifier("suspiciousBack")
                Spacer()
            }
            .frame(minHeight: 44)
            Text("あやしい出品 \(viewModel.suspiciousTotal)").font(.wl(20, .heavy)).foregroundStyle(Palette.fg)
                .accessibilityAddTraits(.isHeader)
            switch viewModel.suspicious {
            case .notLoaded, .loading:
                Text("読み込み中").font(.wl(13)).foregroundStyle(Palette.muted)
            case .failed(let message):
                Text(message).font(.wl(13)).foregroundStyle(Palette.muted)
            case .loaded(let rows):
                VStack(spacing: 10) {
                    ForEach(rows) { row in suspiciousRow(row) }
                }
            }
        }
    }

    private func suspiciousRow(_ row: SuspiciousListingRow) -> some View {
        let content = HStack(alignment: .center, spacing: 12) {
            Group {
                if let imageURL = row.imageURL {
                    AsyncImage(url: imageURL) { $0.resizable().scaledToFill() } placeholder: { Palette.lineSoft }
                } else {
                    Palette.lineSoft
                }
            }
            .frame(width: 64, height: 64)
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
            VStack(alignment: .leading, spacing: 6) {
                Text(row.priceText).font(.wl(18, .heavy)).foregroundStyle(Palette.fg)
                WrappingHStack(spacing: 6) {
                    ForEach(row.reasonTexts, id: \.self) { reason in
                        Text(reason).font(.wl(12, .heavy)).foregroundStyle(Palette.warn)
                            .padding(.horizontal, 8).frame(height: 24)
                            .background(Color(hex: "#fff1e6"), in: Capsule())
                    }
                }
            }
            Spacer(minLength: 0)
            if let name = viewModel.links.first(where: { $0.site.id == row.siteID })?.site.name {
                SiteMarkView(name: name, size: 32)
            }
        }
        .padding(10)
        .card(radius: Metrics.cardRadius)
        return Group {
            if let url = row.linkURL {
                Button { UIApplication.shared.open(url) } label: { content }.buttonStyle(.plain)
            } else {
                content
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(([row.title, row.priceText] + row.reasonTexts).joined(separator: " "))
        .accessibilityIdentifier("suspiciousListing-\(row.id)")
    }

    /// 公式ページの販売状況(監視中の商品だけ。文字だけで、アニメーションは使わない)
    private func officialSection(_ official: OfficialLines) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            officialLine(official.summary, id: "officialStatus", font: .subheadline)
            if let evidence = official.evidence { officialLine(evidence, id: "officialEvidence", font: .footnote) }
            if let change = official.change { officialLine(change, id: "officialChange", font: .footnote) }
            if let lastAttempt = official.lastAttempt { officialLine(lastAttempt, id: "officialLastAttempt", font: .footnote) }
        }
    }

    private func officialLine(_ text: String, id: String, font: Font) -> some View {
        Text(text)
            .font(font)
            .foregroundStyle(.secondary)
            .accessibilityLabel(text)
            .accessibilityIdentifier(id)
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
                        .font(.wl(13))
                        .foregroundStyle(Palette.muted)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(viewModel.displayQuery)
                .accessibilityIdentifier("queryEditButton")
            }
            if let message = viewModel.errorMessage {
                Text(message).font(.footnote).foregroundStyle(Palette.danger)
            }
        }
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
