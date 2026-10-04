import Foundation
import Synchronization
import XCTest

@testable import WishlistCore

// AC-IOS-EST-01〜07・10〜12: 詳細シートの目安価格(PWA の AC-EST と同じ規則。docs/phase3-ios-spec.md)。
// 時計(now・sleep)は差し替える。5 秒待たずに ManualSleeper で進める。
@MainActor
final class ItemDetailEstimatesTests: XCTestCase {
    /// 「今」。JST 2026-10-03 12:00
    private let now = iso("2026-10-03T03:00:00Z")
    /// 目安の取得時刻。JST 2026-10-03 9:30(→ 10/3 時点・最終取得 今日)
    private let fetched = iso("2026-10-03T00:30:00Z")

    private func make(
        service: FakeWishlistService? = nil, item: Item = T.gris
    ) -> (vm: ItemDetailViewModel, service: FakeWishlistService, sleeper: ManualSleeper) {
        let service = service ?? T.fake()
        let sleeper = ManualSleeper()
        let now = now
        let genre = T.genres.first { $0.id == item.genreID }
        let vm = ItemDetailViewModel(
            item: item, genre: genre, sites: T.sites, service: service, now: { now }, sleep: sleeper.sleep)
        return (vm, service, sleeper)
    }

    private func site(
        _ id: Int, low: Int? = 3000, mid: Int? = 4500, count: Int = 5, inStock: Int = 4, suspicious: Int = 0,
        status: EstimateStatus = .ok, at: Date? = nil
    ) -> SiteEstimate {
        SiteEstimate(
            siteID: id, low: low, mid: mid, count: count, suspiciousCount: suspicious, inStockCount: inStock, status: status,
            fetchedAt: at ?? fetched)
    }

    private func estimates(
        _ sites: [SiteEstimate] = [], low: Int? = nil, mid: Int? = nil, at: Date? = nil, refreshing: Bool = false
    ) -> ItemEstimates {
        ItemEstimates(itemID: 12, summaryLow: low, summaryMid: mid, summaryFetchedAt: at, sites: sites, refreshing: refreshing)
    }

    private func getCount(_ service: FakeWishlistService) -> Int {
        service.calls.filter { $0 == .estimates(itemID: 12) }.count
    }

    // MARK: - AC-IOS-EST-01 サマリの文言

    func testSummaryTextTable() async {
        let noResult = site(1, low: nil, mid: nil, count: 0, inStock: 0, status: .noResult)
        let failed = site(1, low: nil, mid: nil, count: 0, inStock: 0, status: .failed)
        let table: [(name: String, value: ItemEstimates, want: String)] = [
            ("値あり(JST の日付)", estimates([site(1)], low: 3000, mid: 4500, at: iso("2026-10-02T15:30:00Z")), "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)"),
            ("mid が無い", estimates([site(1)], low: 3000, mid: nil, at: fetched), "だいたい ¥3,000〜 で買えそう(10/3 時点)"),
            ("時点が無い", estimates([site(1)], low: 3000, mid: 4500, at: nil), "だいたい ¥3,000〜¥4,500 で買えそう"),
            ("sites が空", estimates(), "まだ価格情報はありません"),
            ("すべて no_result", estimates([noResult, site(2, low: nil, mid: nil, count: 0, status: .noResult)]), "出品ないかも"),
            ("failed だけ(前回値なし)", estimates([failed]), "まだ価格情報はありません"),
            ("no_result と failed の混在", estimates([noResult, failed]), "まだ価格情報はありません"),
        ]
        for row in table {
            let service = T.fake()
            service.setEstimates(row.value)
            let (vm, _, _) = make(service: service)
            await vm.loadEstimates()
            XCTAssertEqual(vm.summaryText, row.want, row.name)
        }
    }

    /// no_result のサマリには金額を出さない(ユーザー指示 2026-10-04)。
    func testNoListingsSummaryHasNoAmount() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1, low: nil, mid: nil, count: 0, inStock: 0, status: .noResult)]))
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        XCTAssertEqual(vm.summary, .noListings)
        XCTAssertFalse(vm.summaryText.contains("¥"))
    }

    func testRefreshingAppendsUpdatingToEveryForm() async {
        let table: [(value: ItemEstimates, want: String)] = [
            (estimates([site(1)], low: 3000, mid: 4500, at: fetched, refreshing: true), "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点) 更新中…"),
            (estimates(refreshing: true), "まだ価格情報はありません 更新中…"),
            (estimates([site(1, low: nil, mid: nil, count: 0, status: .noResult)], refreshing: true), "出品ないかも 更新中…"),
        ]
        for row in table {
            let service = T.fake()
            service.setEstimates(row.value)
            let (vm, _, _) = make(service: service)
            await vm.loadEstimates()
            XCTAssertTrue(vm.isRefreshing)
            XCTAssertEqual(vm.summaryText, row.want)
            vm.stopPolling()
            await vm.waitForPolling()
        }
    }

    /// 最初の取得が通信失敗: 「オフライン」。リンクは出し、注記は付けず、更新できない。
    func testFirstFetchNetworkFailureIsOfflineAndDisablesRefresh() async {
        let (vm, service, sleeper) = make(service: T.fake(offline: true))
        await vm.loadEstimates()
        XCTAssertEqual(vm.summaryText, "オフライン")
        XCTAssertNil(vm.noticeText)
        XCTAssertFalse(vm.canRefresh)
        XCTAssertEqual(vm.siteRows.count, 3, "オフラインでもサイト行は出る")
        XCTAssertEqual(sleeper.requested, [], "通信失敗では再取得を予約しない")
        await vm.refresh()
        XCTAssertEqual(service.calls, [.estimates(itemID: 12)], "オフラインの間は POST しない")
    }

    // MARK: - AC-IOS-EST-02 サイト行

    func testSiteRowsTable() async {
        let failedWithValue = site(3, low: 2800, mid: 3900, count: 4, inStock: 0, status: .failed, at: iso("2026-10-01T03:00:00Z"))
        let failedToday = site(3, low: 2800, mid: nil, count: 4, inStock: 2, status: .failed, at: iso("2026-10-02T16:00:00Z"))
        let failedNoValue = site(3, low: nil, mid: nil, count: 0, inStock: 0, status: .failed)
        let noResult = site(3, low: nil, mid: nil, count: 0, inStock: 0, status: .noResult)
        let table: [(name: String, row: SiteEstimate, want: SiteRow.Fields)] = [
            ("ok", site(3), .init(estimate: "¥3,000〜¥4,500", count: "5件", stock: "在庫あり", note: nil)),
            ("ok で mid が無い", site(3, mid: nil), .init(estimate: "¥3,000〜", count: "5件", stock: "在庫あり", note: nil)),
            ("ok で在庫 0", site(3, inStock: 0), .init(estimate: "¥3,000〜¥4,500", count: "5件", stock: "在庫なし", note: nil)),
            ("failed(前回値あり・2 日前)", failedWithValue, .init(estimate: "¥2,800〜¥3,900", count: "4件", stock: "在庫なし", note: "最終取得: 2日前")),
            ("failed(前回値あり・同じ日)", failedToday, .init(estimate: "¥2,800〜", count: "4件", stock: "在庫あり", note: "最終取得: 今日")),
            ("failed(前回値なし)", failedNoValue, .init(estimate: nil, count: nil, stock: nil, note: "取得できませんでした")),
            ("no_result(金額・件数・在庫は出さない)", noResult, .init(estimate: nil, count: nil, stock: nil, note: "出品ないかも")),
        ]
        for entry in table {
            let service = T.fake()
            service.setEstimates(estimates([entry.row]))
            let (vm, _, _) = make(service: service)
            await vm.loadEstimates()
            let row = vm.siteRows.first { $0.id == 3 }
            XCTAssertEqual(row?.fields, entry.want, entry.name)
            XCTAssertEqual(row?.link.site.id, 3, entry.name)
        }
    }

    /// サイト行はジャンルの siteIDs 順([2 Amazon, 1 メルカリ, 3 その他])。estimates に無いサイトはリンクだけ。ジャンルに無いサイトは出さない。
    func testSiteRowsFollowGenreOrderAndKeepLinkOnlyRows() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1), site(99)], low: 3000, mid: 4500, at: fetched))
        let (vm, _, _) = make(service: service)
        XCTAssertEqual(vm.siteRows.map(\.id), [2, 1, 3], "取得前でも links と同じ並びで出る")
        XCTAssertEqual(vm.siteRows.map(\.link), vm.links)
        await vm.loadEstimates()
        XCTAssertEqual(vm.siteRows.map(\.id), [2, 1, 3])
        XCTAssertEqual(
            vm.siteRows.map(\.fields),
            [
                .init(estimate: nil, count: nil, stock: nil, note: nil),  // Amazon は目安なし(リンクだけ)
                .init(estimate: "¥3,000〜¥4,500", count: "5件", stock: "在庫あり", note: nil),
                .init(estimate: nil, count: nil, stock: nil, note: nil),
            ])
    }

    func testSiteRowAccessibilityLabelJoinsNameAndParts() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1), site(3, low: nil, mid: nil, count: 0, inStock: 0, status: .noResult)]))
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        XCTAssertEqual(
            vm.siteRows.map(\.accessibilityLabel), ["Amazon", "メルカリ ¥3,000〜¥4,500 5件 在庫あり", "その他 出品ないかも"])
    }

    // MARK: - AC-IOS-EST-03 再取得(ポーリング)

    func testNoPollingWhenNotRefreshing() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1)], low: 3000, mid: 4500, at: fetched))
        let (vm, _, sleeper) = make(service: service)
        await vm.loadEstimates()
        await vm.waitForPolling()
        XCTAssertEqual(sleeper.requested, [])
        XCTAssertEqual(getCount(service), 1)
        XCTAssertFalse(vm.isRefreshing)
    }

    func testPollsEveryFiveSecondsAndStopsWhenRefreshingIsFalse() async {
        let service = T.fake()
        let first = estimates(refreshing: true)
        let second = estimates([site(1)], low: 3000, mid: 4500, at: fetched, refreshing: false)
        service.setEstimatesSequence([first, second])
        let (vm, _, sleeper) = make(service: service)
        await vm.loadEstimates()
        XCTAssertEqual(getCount(service), 1)
        XCTAssertTrue(vm.isRefreshing)
        await sleeper.waitUntilPending()
        XCTAssertEqual(sleeper.requested, [.seconds(5)])
        XCTAssertEqual(getCount(service), 1, "5 秒たつ前は GET しない")
        sleeper.advance()
        await vm.waitForPolling()
        XCTAssertEqual(getCount(service), 2)
        XCTAssertFalse(vm.isRefreshing)
        XCTAssertEqual(vm.summaryText, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)")
        XCTAssertEqual(sleeper.requested, [.seconds(5)], "false になったら予約しない")
    }

    /// 再取得は最大 6 回(最初と合わせて GET は 7 回)。打ち切ったら「更新中…」を消し、取得済みの値は残す。
    func testGivesUpAfterSixRefetchesAndKeepsTheValue() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1)], low: 3000, mid: 4500, at: fetched, refreshing: true))
        let (vm, _, sleeper) = make(service: service)
        await vm.loadEstimates()
        for round in 1...EstimatePolling.maxRefetches {
            await sleeper.waitUntilPending()
            XCTAssertEqual(sleeper.requested.count, round)
            sleeper.advance()
            if round < EstimatePolling.maxRefetches {
                await sleeper.waitUntilPending()  // 次の予約が入るのを待ってから進める
                XCTAssertEqual(getCount(service), round + 1)
            }
        }
        await vm.waitForPolling()
        XCTAssertEqual(getCount(service), 7)
        XCTAssertEqual(EstimatePolling.maxRefetches, 6)
        XCTAssertEqual(sleeper.requested, Array(repeating: Duration.seconds(5), count: 6))
        XCTAssertEqual(sleeper.pendingCount, 0)
        XCTAssertFalse(vm.isRefreshing)
        XCTAssertEqual(vm.summaryText, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)", "更新中…は消え、値は残る")
        XCTAssertTrue(vm.canRefresh, "打ち切ったら更新できる")
    }

    /// シートを閉じたら予約を取り消す(閉じたあとの GET なし)。
    func testStopPollingCancelsThePendingWait() async {
        let service = T.fake()
        service.setEstimates(estimates(refreshing: true))
        let (vm, _, sleeper) = make(service: service)
        await vm.loadEstimates()
        await sleeper.waitUntilPending()
        vm.stopPolling()
        await vm.waitForPolling()
        XCTAssertEqual(sleeper.pendingCount, 0)
        sleeper.advance()
        try? await Task.sleep(for: .milliseconds(20))
        XCTAssertEqual(getCount(service), 1, "閉じたあとは GET しない")
    }

    /// 再取得が失敗したら前回の値を出し続け、短い文を添え、再取得をやめる。
    func testPollFailureKeepsThePreviousValueAndStops() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1)], low: 3000, mid: 4500, at: fetched, refreshing: true))
        let (vm, _, sleeper) = make(service: service)
        await vm.loadEstimates()
        await sleeper.waitUntilPending()
        service.failOnce(WishlistError(code: .network, message: "offline"))
        sleeper.advance()
        await vm.waitForPolling()
        XCTAssertEqual(vm.summaryText, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)")
        XCTAssertEqual(vm.noticeText, "オフライン(前回の値)")
        XCTAssertEqual(vm.siteRows.first { $0.id == 1 }?.estimateText, "¥3,000〜¥4,500")
        XCTAssertEqual(sleeper.requested, [.seconds(5)], "失敗したら次を予約しない")
        XCTAssertFalse(vm.isRefreshing)
    }

    // MARK: - AC-IOS-EST-04 「更新」ボタン

    func testRefreshPostsShowsUpdatingThenPollsOnce() async {
        let service = T.fake()
        let before = estimates([site(1)], low: 3000, mid: 4500, at: fetched)
        let after = estimates([site(1, low: 2800, mid: 4000)], low: 2800, mid: 4000, at: fetched)
        service.setEstimatesSequence([before, after])
        service.setRefreshResponse(ItemEstimates(
            itemID: 12, summaryLow: 3000, summaryMid: 4500, summaryFetchedAt: fetched, sites: before.sites, refreshing: true))
        let (vm, _, sleeper) = make(service: service)
        await vm.loadEstimates()
        XCTAssertTrue(vm.canRefresh)
        await vm.refresh()
        XCTAssertEqual(service.calls, [.estimates(itemID: 12), .refreshEstimates(itemID: 12)])
        XCTAssertTrue(vm.isRefreshing)
        XCTAssertEqual(vm.summaryText, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点) 更新中…")
        XCTAssertFalse(vm.canRefresh, "更新中は押せない")
        await sleeper.waitUntilPending()
        sleeper.advance()
        await vm.waitForPolling()
        XCTAssertEqual(getCount(service), 2)
        XCTAssertFalse(vm.isRefreshing)
        XCTAssertTrue(vm.canRefresh)
        XCTAssertEqual(vm.summaryText, "だいたい ¥2,800〜¥4,000 で買えそう(10/3 時点)")
    }

    /// POST の応答待ちの間も押せず、連打しても POST は重ならない。
    func testRefreshDoesNotOverlap() async {
        let service = T.fake()
        let gate = Gate()
        service.gate = { call in
            if case .refreshEstimates = call { await gate.pass() }
        }
        service.setRefreshResponse(estimates(refreshing: false))
        let (vm, _, _) = make(service: service)
        let first = Task { await vm.refresh() }
        await gate.waitUntilArrived()
        XCTAssertFalse(vm.canRefresh, "POST の応答待ちは押せない")
        await vm.refresh()
        await vm.refresh()
        gate.open()
        await first.value
        XCTAssertEqual(service.calls.filter { $0 == .refreshEstimates(itemID: 12) }.count, 1)
        XCTAssertTrue(vm.canRefresh)
    }

    func testRefreshIsDisabledWhileRefreshingEvenWithoutPressing() async {
        let service = T.fake()
        service.setEstimates(estimates(refreshing: true))
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        XCTAssertFalse(vm.canRefresh)
        await vm.refresh()
        XCTAssertEqual(service.calls, [.estimates(itemID: 12)])
        vm.stopPolling()
        await vm.waitForPolling()
    }

    // MARK: - AC-IOS-EST-05 失敗しても前回値を残す

    func testRefreshFailureKeepsThePreviousValue() async {
        let table: [(error: WishlistError, notice: String)] = [
            (WishlistError(code: .network, message: "offline"), "オフライン(前回の値)"),
            (T.unauthorized(), "更新できませんでした"),
        ]
        for row in table {
            let service = T.fake()
            service.setEstimates(estimates([site(1)], low: 3000, mid: 4500, at: fetched))
            let (vm, _, sleeper) = make(service: service)
            await vm.loadEstimates()
            service.failOnce(row.error)
            await vm.refresh()
            XCTAssertEqual(vm.summaryText, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)", "\(row.notice): 値は残る")
            XCTAssertEqual(vm.siteRows.first { $0.id == 1 }?.estimateText, "¥3,000〜¥4,500")
            XCTAssertEqual(vm.noticeText, row.notice)
            XCTAssertEqual(sleeper.requested, [], "失敗したら再取得を予約しない")
            XCTAssertFalse(vm.isRefreshing)
            XCTAssertNotEqual(vm.summaryText, "オフライン", "値があるうちは「オフライン」だけにしない")
        }
    }

    func testNoticeClearsOnTheNextSuccess() async {
        let service = T.fake()
        service.setEstimates(estimates([site(1)], low: 3000, mid: 4500, at: fetched))
        service.setRefreshResponse(estimates([site(1)], low: 3000, mid: 4500, at: fetched, refreshing: false))
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        service.failOnce(T.unauthorized())
        await vm.refresh()
        XCTAssertNotNil(vm.noticeText)
        await vm.refresh()
        XCTAssertNil(vm.noticeText)
    }

    // MARK: - AC-IOS-EST-06 古い応答で上書きしない

    /// 保留した古い GET が、あとから返っても、新しい POST の結果を上書きしない。
    func testStaleGetDoesNotOverwriteTheRefreshResult() async {
        let service = T.fake()
        let gate = Gate()
        service.gate = { call in
            if case .estimates = call { await gate.pass() }
        }
        service.setEstimates(estimates([site(1)], low: 3000, mid: 4500, at: fetched, refreshing: false))
        service.setRefreshResponse(estimates([site(1, low: 1000, mid: 1500)], low: 1000, mid: 1500, at: fetched, refreshing: false))
        let (vm, _, _) = make(service: service)
        let load = Task { await vm.loadEstimates() }
        await gate.waitUntilArrived()
        await vm.refresh()
        XCTAssertEqual(vm.summaryText, "だいたい ¥1,000〜¥1,500 で買えそう(10/3 時点)")
        gate.open()
        await load.value
        XCTAssertEqual(vm.summaryText, "だいたい ¥1,000〜¥1,500 で買えそう(10/3 時点)", "古い GET の結果は捨てる")
        XCTAssertEqual(vm.estimates?.summaryLow, 1000)
    }

    // MARK: - AC-IOS-EST-07 参考外

    private func suspiciousSetup(_ counts: [Int]) -> FakeWishlistService {
        let service = T.fake()
        service.setEstimates(estimates(
            counts.enumerated().map { site($0.offset + 1, suspicious: $0.element) }, low: 3000, mid: 4500, at: fetched))
        return service
    }

    func testSuspiciousIsHiddenWhenTheTotalIsZero() async {
        let service = suspiciousSetup([0, 0])
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        XCTAssertEqual(vm.suspiciousTotal, 0)
        XCTAssertNil(vm.suspiciousTitle)
        XCTAssertFalse(service.calls.contains { if case .listings = $0 { true } else { false } }, "合計 0 なら listings を取らない")
    }

    func testSuspiciousTitleSumsAndDoesNotFetchUntilOpened() async {
        let service = suspiciousSetup([1, 2])
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        XCTAssertEqual(vm.suspiciousTotal, 3)
        XCTAssertEqual(vm.suspiciousTitle, "参考外 3件")
        XCTAssertEqual(vm.suspicious, .notLoaded)
        XCTAssertEqual(service.calls, [.estimates(itemID: 12)], "開くまで listings を取らない")
    }

    func testLoadSuspiciousFiltersMapsAndSanitizes() async {
        let service = suspiciousSetup([3])
        service.setListings([
            Listing(id: 1, siteID: 1, title: "グリス", price: 3000, url: "https://item.example.com/1"),  // 参考にしている出品(理由なし)
            Listing(
                id: 2, siteID: 1, title: "ベルトだけ", price: 300, url: "https://item.example.com/2",
                imageURL: "https://img.example.com/2.jpg", suspiciousReasons: [.titleMismatch, .tooCheap]),
            Listing(
                id: 3, siteID: 1, title: "危ない出品", price: 1_200, url: "javascript:alert(1)",
                imageURL: "data:image/png;base64,AAAA", suspiciousReasons: [.belowMin]),
            Listing(id: 4, siteID: 1, title: "画像なし", price: 500, url: "http://item.example.com/4", suspiciousReasons: [.tooCheap]),
        ])
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        await vm.loadSuspiciousListings()
        XCTAssertEqual(service.calls.last, .listings(itemID: 12, siteID: nil))
        XCTAssertEqual(
            vm.suspicious,
            .loaded([
                SuspiciousListingRow(
                    id: 2, title: "ベルトだけ", priceText: "¥300", reasonTexts: ["商品名が一致しない", "安すぎる"],
                    imageURL: URL(string: "https://img.example.com/2.jpg"), linkURL: URL(string: "https://item.example.com/2")),
                SuspiciousListingRow(
                    id: 3, title: "危ない出品", priceText: "¥1,200", reasonTexts: ["下限価格未満"], imageURL: nil, linkURL: nil),
                SuspiciousListingRow(
                    id: 4, title: "画像なし", priceText: "¥500", reasonTexts: ["安すぎる"], imageURL: nil,
                    linkURL: URL(string: "http://item.example.com/4")),
            ]), "理由のある出品だけ・API の並びのまま・http(s) 以外は画像もリンクも nil")
    }

    /// listings の取得に失敗しても、折りたたみの中の短い文で済ませ、シートの目安は壊さない。
    func testLoadSuspiciousFailureKeepsEstimates() async {
        let service = suspiciousSetup([1])
        let (vm, _, _) = make(service: service)
        await vm.loadEstimates()
        service.failOnce(WishlistError(code: .badGateway, status: 502, message: "bad gateway"))
        await vm.loadSuspiciousListings()
        XCTAssertEqual(vm.suspicious, .failed("参考外の出品を取得できませんでした"))
        XCTAssertEqual(vm.summaryText, "だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)")
        XCTAssertEqual(vm.suspiciousTitle, "参考外 1件")
    }
}

extension SiteRow {
    /// テストの比較用(link を除いた 4 項目)
    struct Fields: Equatable {
        var estimate: String?
        var count: String?
        var stock: String?
        var note: String?
    }
    var fields: Fields { Fields(estimate: estimateText, count: countText, stock: stockText, note: noteText) }
}
