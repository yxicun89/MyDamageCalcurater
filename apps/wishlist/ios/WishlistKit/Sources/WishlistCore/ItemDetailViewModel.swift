import Foundation
import Observation

/// 詳細シート(画像タップで下から出る)の状態。
@MainActor
@Observable
public final class ItemDetailViewModel {
    /// 時計の差し替え口(テストは待たずに進める)。既定は `Task.sleep`。キャンセルされたら `CancellationError` を投げること。
    public typealias Sleep = @Sendable (Duration) async throws -> Void

    public enum Summary: Equatable, Sendable {
        case loading
        /// 目安がある → `だいたい ¥A〜¥B で買えそう(M/D 時点)`(mid が nil なら `¥A〜`、時点が nil なら括弧なし)
        case priced(low: Int, mid: Int?, at: Date?)
        /// estimates があり、目安を出せるサイトが無く、すべて no_result → `出品ないかも`
        case noListings
        /// 目安価格が無い(フェーズ2は常にこれ)→ 文言 `WishlistText.noPriceInfo`
        case noPriceInfo
        /// 通信できない → 文言 `WishlistText.offline`
        case offline
    }

    /// 保存済みの商品(「保存」に成功したら更新される)
    public private(set) var item: Item
    public private(set) var summary: Summary = .loading
    public private(set) var isEditingQuery = false
    /// 検索ワードの編集中の値。シート内だけの一時的な値で、閉じれば破棄される(`saveQuery()` のときだけ送る)
    public var queryDraft = ""
    /// 保存の失敗(編集中の値とリンクは残す)
    public private(set) var errorMessage: String?

    private let genre: Genre?
    private let sites: [Site]
    private let service: any WishlistService
    private let now: @Sendable () -> Date
    private let sleep: Sleep

    public init(
        item: Item, genre: Genre?, sites: [Site], service: any WishlistService,
        now: @escaping @Sendable () -> Date = { Date() },
        sleep: @escaping Sleep = { try await Task.sleep(for: $0) }
    ) {
        self.item = item
        self.genre = genre
        self.sites = sites
        self.service = service
        self.now = now
        self.sleep = sleep
    }

    // MARK: - フェーズ3: 目安価格(docs/phase3-ios-spec.md)

    /// 直近に反映した estimates(取得できていなければ nil)
    public private(set) var estimates: ItemEstimates?
    /// `refreshing: true` で、再取得を打ち切っていない間 true(サマリに `更新中…` を添える)
    public private(set) var isRefreshing = false
    /// 更新・再取得に失敗したが前回の値は出している、という短い文。無ければ nil
    public private(set) var noticeText: String?
    public private(set) var suspicious: SuspiciousState = .notLoaded

    @ObservationIgnored private var generation = 0
    private var isPosting = false
    @ObservationIgnored private var pollTask: Task<Void, Never>?

    /// 「更新」ボタンを押せるか。更新中(POST の応答待ち・`isRefreshing`)とオフライン(`summary == .offline`)の間は false
    public var canRefresh: Bool { !isPosting && !isRefreshing && summary != .offline }

    /// サイト行(`links` と同じ並び。目安・件数・在庫・注記つき)
    public var siteRows: [SiteRow] {
        links.map { link in
            guard let site = estimates?.sites.first(where: { $0.siteID == link.site.id }) else { return SiteRow(link: link) }
            return Self.siteRow(link: link, site: site, now: now())
        }
    }

    private static func siteRow(link: SiteLink, site: SiteEstimate, now: Date) -> SiteRow {
        switch site.status {
        case .noResult:
            return SiteRow(link: link, noteText: WishlistText.noListings)
        case .ok, .failed:
            guard let low = site.low else {
                return SiteRow(link: link, noteText: site.status == .failed ? WishlistText.couldNotFetch : nil)
            }
            return SiteRow(
                link: link, estimateText: PriceFormat.range(low: low, mid: site.mid), countText: "\(site.count)件",
                stockText: site.inStockCount > 0 ? "在庫あり" : "在庫なし",
                noteText: site.status == .failed ? "最終取得: \(PriceFormat.age(of: site.fetchedAt, now: now))" : nil)
        }
    }

    /// 参考外の合計(`suspicious_count` の合計)。0 なら折りたたみ自体を出さない
    public var suspiciousTotal: Int { estimates?.sites.reduce(0) { $0 + $1.suspiciousCount } ?? 0 }

    /// `参考外 N件`(合計が 0 なら nil)
    public var suspiciousTitle: String? { suspiciousTotal > 0 ? "参考外 \(suspiciousTotal)件" : nil }

    /// 「更新」: `POST estimates/refresh` を呼び、本文を反映して再取得に入る。`canRefresh` が false なら何もしない。
    public func refresh() async {
        guard canRefresh else { return }
        isPosting = true
        defer { isPosting = false }
        let id = nextGeneration()
        do {
            let result = try await service.refreshEstimates(itemID: item.id)
            guard id == generation else { return }
            apply(result)
        } catch {
            guard id == generation else { return }
            fail(error)
        }
    }

    /// 参考外の折りたたみを開いたとき: `GET listings`(`siteID` なし)→ 理由のある出品だけ `suspicious` に入れる。
    public func loadSuspiciousListings() async {
        suspicious = .loading
        do {
            let all = try await service.listings(itemID: item.id, siteID: nil)
            suspicious = .loaded(
                all.filter { !$0.suspiciousReasons.isEmpty }.map { listing in
                    SuspiciousListingRow(
                        id: listing.id, title: listing.title, priceText: PriceFormat.yen(listing.price),
                        reasonTexts: listing.suspiciousReasons.map(PriceFormat.reasonLabel),
                        imageURL: listing.imageURL.flatMap(Self.httpURL), linkURL: Self.httpURL(listing.url))
                })
        } catch {
            suspicious = .failed(WishlistText.suspiciousFetchFailed)
        }
    }

    private static func httpURL(_ text: String) -> URL? {
        Deeplink.isHTTPURL(text) ? URL(string: text) : nil
    }

    /// シートを閉じたとき: 再取得の予約を取り消す(閉じたあとの GET なし)。
    public func stopPolling() {
        pollTask?.cancel()
    }

    /// 再取得のループが終わるまで待つ(テスト用。無ければすぐ返る)。
    public func waitForPolling() async {
        await pollTask?.value
    }

    /// サマリの文言(`WishlistText`)。loading 中は空文字。更新中は ` 更新中…` を添える。
    public var summaryText: String {
        let base: String
        switch summary {
        case .loading: return ""
        case .priced(let low, let mid, let at):
            base = "だいたい \(PriceFormat.range(low: low, mid: mid)) で買えそう" + (at.map { "(\(PriceFormat.jstDate($0)) 時点)" } ?? "")
        case .noListings: base = WishlistText.noListings
        case .noPriceInfo: base = WishlistText.noPriceInfo
        case .offline: return WishlistText.offline
        }
        return isRefreshing ? base + " " + WishlistText.refreshing : base
    }

    // MARK: 取得の反映

    private func nextGeneration() -> Int {
        generation += 1
        return generation
    }

    private static func summary(of estimates: ItemEstimates) -> Summary {
        if estimates.sites.isEmpty { return .noPriceInfo }
        if let low = estimates.summaryLow { return .priced(low: low, mid: estimates.summaryMid, at: estimates.summaryFetchedAt) }
        if estimates.sites.allSatisfy({ $0.status == .noResult }) { return .noListings }
        return .noPriceInfo
    }

    /// 取得に成功した estimates を反映する。`refreshing` なら再取得を予約し直す。
    private func apply(_ result: ItemEstimates) {
        estimates = result
        summary = Self.summary(of: result)
        noticeText = nil
        isRefreshing = result.refreshing
        pollTask?.cancel()
        pollTask = result.refreshing ? makePollTask() : nil
    }

    /// 失敗の反映。値を取得済みなら値を残して注記だけ。まだなら通信失敗は「オフライン」、それ以外は価格情報なし。
    private func fail(_ error: any Error) {
        let isNetwork = (error as? WishlistError)?.isNetwork == true
        isRefreshing = false
        pollTask?.cancel()
        pollTask = nil
        if estimates == nil {
            summary = isNetwork ? .offline : .noPriceInfo
        } else {
            noticeText = isNetwork ? WishlistText.offlineKeepingPrevious : WishlistText.updateFailed
        }
    }

    /// `interval` ごとに最大 `maxRefetches` 回 GET し直す。待ちは必ず注入された `sleep` を通す。
    private func makePollTask() -> Task<Void, Never> {
        let sleep = self.sleep
        return Task { [weak self] in
            for _ in 0..<EstimatePolling.maxRefetches {
                do { try await sleep(EstimatePolling.interval) } catch { return }
                guard !Task.isCancelled, let self else { return }
                guard await self.pollOnce() else { return }
            }
            self?.giveUpPolling()
        }
    }

    /// 1 回再取得する。続けるなら true。
    private func pollOnce() async -> Bool {
        let id = nextGeneration()
        do {
            let result = try await service.estimates(itemID: item.id)
            guard id == generation, !Task.isCancelled else { return false }
            estimates = result
            summary = Self.summary(of: result)
            noticeText = nil
            isRefreshing = result.refreshing
            return result.refreshing
        } catch {
            if id == generation, !Task.isCancelled { fail(error) }
            return false
        }
    }

    private func giveUpPolling() {
        isRefreshing = false
    }

    // MARK: - フェーズ4-2: 価格の推移(docs/phase4-spec.md 4-2)

    /// 「価格の推移」の折りたたみの状態。開いたときに `loadPriceHistory()` で取る
    public private(set) var priceHistory: PriceHistoryState = .notLoaded
    /// 凡例で表示を選んだサイト(既定は空 = 全体の最安だけ)
    public private(set) var selectedHistorySiteIDs: Set<Int> = []

    /// `GET price-history`(days は付けない)。取得中・取得済み(`.loaded`・`.empty`)なら何もしない。失敗(`.failed`)のあとは取り直す。
    /// 全体の最安の点が 2 未満なら `.empty`。通信できなければ `.failed(WishlistText.offline)`、それ以外は `.failed(WishlistText.priceHistoryFailed)`。
    /// 目安価格(`summary`・`estimates`)は変えない。
    public func loadPriceHistory() async {
        // TODO(implementer): docs/phase4-spec.md AC-IOS-HIS-01〜03
    }

    /// 凡例のボタン: そのサイトの線の表示を切り替える(推移に無いサイトは無視)
    public func toggleHistorySite(_ siteID: Int) {
        _ = siteID  // TODO(implementer): AC-IOS-HIS-04
    }

    /// 凡例(推移の `sites` の順。`.loaded` 以外は空)
    public var historyLegend: [HistoryLegendItem] {
        []  // TODO(implementer): AC-IOS-HIS-04
    }

    /// 描く線: 全体の最安 + 選んだサイト(凡例の順)。`.loaded` 以外は空
    public var visibleHistorySeries: [HistorySeries] {
        []  // TODO(implementer): AC-IOS-HIS-04
    }

    /// 保存済みの検索ワード(サイト別の上書きを除く。検索ワード欄に出す値)
    public var displayQuery: String { SiteLinks.itemQuery(item: item, genre: genre) }

    /// サイト行。ジャンルの siteIDs 順(`SiteLinks.resolve`)。
    /// 編集中は `queryDraft`(前後の空白を除き、空なら override 無し)をこの商品の queryOverride として組み立てる(入力値で変わる。PWA と同じ)。
    /// 通信しないので、オフラインでも常に出る。
    public var links: [SiteLink] {
        var shown = item
        if isEditingQuery {
            let trimmed = queryDraft.trimmingCharacters(in: .whitespacesAndNewlines)
            shown.queryOverride = trimmed.isEmpty ? nil : trimmed
        }
        return SiteLinks.resolve(item: shown, genre: genre, sites: sites)
    }

    /// 編集を始める。`queryDraft` を `displayQuery` で初期化する。API は呼ばない。
    public func beginEditingQuery() {
        queryDraft = displayQuery
        errorMessage = nil
        isEditingQuery = true
    }

    /// 編集をやめて破棄する。API は呼ばない。
    public func cancelEditingQuery() {
        isEditingQuery = false
        queryDraft = ""
        errorMessage = nil
    }

    /// 「保存」。`queryDraft` の前後の空白を除いた値で `PATCH {query_override}` を送る(空なら `.clear` = null)。
    /// 成功したら `item` を返ってきた商品に更新して編集を終え、その商品を返す。
    /// 失敗したら `errorMessage` を立て、編集中のまま(`queryDraft` はそのまま)nil を返す。
    @discardableResult
    public func saveQuery() async -> Item? {
        let trimmed = queryDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            let updated = try await service.updateItem(
                id: item.id, patch: ItemPatch(queryOverride: trimmed.isEmpty ? .clear : .set(trimmed)))
            item = updated
            isEditingQuery = false
            errorMessage = nil
            return updated
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }

    /// `estimates` を取得してサマリを決める。通信できない(`WishlistError.isNetwork`)最初の取得は `.offline`、その他の失敗は `.noPriceInfo`。
    /// 取得済みなら失敗しても値を残して `noticeText` だけ立てる。`refreshing: true` なら再取得を予約する。
    public func loadEstimates() async {
        let id = nextGeneration()
        do {
            let result = try await service.estimates(itemID: item.id)
            guard id == generation else { return }
            apply(result)
        } catch {
            guard id == generation else { return }
            fail(error)
        }
    }
}

/// 詳細シートのサイト行 1 つ分の表示(`SiteLink` + 目安)。文言は PWA と同じ。
public struct SiteRow: Equatable, Sendable, Identifiable {
    public var link: SiteLink
    /// `¥3,000〜¥4,500`(mid が nil なら `¥3,000〜`)。no_result・low が無いときは nil
    public var estimateText: String?
    /// `5件`。no_result・low が無いときは nil
    public var countText: String?
    /// `在庫あり`(in_stock_count > 0)/ `在庫なし`(0)。no_result・low が無いときは nil
    public var stockText: String?
    /// ok は nil。failed で前回値あり → `最終取得: 2日前`(同じ日は `最終取得: 今日`)。failed で前回値なし → `取得できませんでした`。no_result → `出品ないかも`
    public var noteText: String?

    public var id: Int { link.site.id }

    public init(link: SiteLink, estimateText: String? = nil, countText: String? = nil, stockText: String? = nil, noteText: String? = nil) {
        self.link = link
        self.estimateText = estimateText
        self.countText = countText
        self.stockText = stockText
        self.noteText = noteText
    }

    /// VoiceOver・XCUITest 用: サイト名と、あるものを半角スペースでつなぐ(例 `メルカリ ¥3,000〜¥4,500 5件 在庫あり`)
    public var accessibilityLabel: String {
        ([link.site.name] + [estimateText, countText, stockText, noteText].compactMap { $0 }).joined(separator: " ")
    }
}

/// 参考外の出品 1 件の表示。`imageURL`・`linkURL` は http(s) のときだけ入る(`javascript:`・`data:` は nil)。
public struct SuspiciousListingRow: Equatable, Sendable, Identifiable {
    public var id: Int
    public var title: String
    public var priceText: String
    public var reasonTexts: [String]
    public var imageURL: URL?
    public var linkURL: URL?

    public init(id: Int, title: String, priceText: String, reasonTexts: [String], imageURL: URL?, linkURL: URL?) {
        self.id = id
        self.title = title
        self.priceText = priceText
        self.reasonTexts = reasonTexts
        self.imageURL = imageURL
        self.linkURL = linkURL
    }
}

public enum SuspiciousState: Equatable, Sendable {
    case notLoaded
    case loading
    case loaded([SuspiciousListingRow])
    /// 折りたたみの中に短い文を出す(`WishlistText.suspiciousFetchFailed`)。シート全体は壊さない
    case failed(String)
}
