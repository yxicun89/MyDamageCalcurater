import Foundation
import XCTest

@testable import WishlistCore

// AC-IOS-HIS-01〜05: 詳細シートの価格の推移(PWA の AC-HIS-01〜06 と同じ規則。docs/phase4-spec.md)。
@MainActor
final class ItemDetailPriceHistoryTests: XCTestCase {
    private func make(_ service: FakeWishlistService = T.fake()) -> (ItemDetailViewModel, FakeWishlistService) {
        let vm = ItemDetailViewModel(
            item: T.gris, genre: T.figuarts, sites: T.sites, service: service, now: { iso("2026-10-03T03:00:00Z") },
            sleep: { _ in })
        return (vm, service)
    }

    private let history = PriceHistory(
        itemID: 12,
        sites: [
            SitePriceHistory(siteID: 1, points: [PricePoint(day: "2026-10-01", low: 3000, mid: 3400), PricePoint(day: "2026-10-03", low: 3500)]),
            SitePriceHistory(siteID: 2, points: [PricePoint(day: "2026-10-02", low: 3300, mid: 3600)]),
            SitePriceHistory(siteID: 9, points: [PricePoint(day: "2026-10-03", low: 3600)]),
        ],
        overall: [DayLow(day: "2026-10-01", low: 3000), DayLow(day: "2026-10-02", low: 3300), DayLow(day: "2026-10-03", low: 3500)])

    private func historyCalls(_ service: FakeWishlistService) -> [FakeWishlistService.Call] {
        service.calls.filter { if case .priceHistory = $0 { return true } else { return false } }
    }

    // MARK: - AC-IOS-HIS-01 開いたときに 1 回取る

    func testDoesNotFetchUntilLoadIsCalled() async {
        let (vm, service) = make()
        service.setPriceHistory(history)
        await vm.loadEstimates()
        XCTAssertEqual(vm.priceHistory, .notLoaded)
        XCTAssertTrue(historyCalls(service).isEmpty, "折りたたみを開くまで取らない")
    }

    func testLoadFetchesOnceWithoutDaysAndBuildsTheChart() async throws {
        let (vm, service) = make()
        service.setPriceHistory(history)
        await vm.loadPriceHistory()
        XCTAssertEqual(historyCalls(service), [.priceHistory(itemID: 12, days: nil)])
        guard case .loaded(let chart) = vm.priceHistory else { return XCTFail("loaded のはず: \(vm.priceHistory)") }
        XCTAssertEqual(chart.accessibilityLabel, "価格の推移 10/1〜10/3 最安 ¥3,000 最高 ¥3,500")
        XCTAssertEqual(chart.overall.points.map(\.value), [3000, 3300, 3500])

        await vm.loadPriceHistory()
        XCTAssertEqual(historyCalls(service).count, 1, "取得済みなら取り直さない(閉じて開き直しても)")
    }

    // MARK: - AC-IOS-HIS-02 点が 2 未満

    func testFewerThanTwoPointsIsEmpty() async {
        for overall in [[], [DayLow(day: "2026-10-03", low: 3000)]] {
            let (vm, service) = make()
            service.setPriceHistory(PriceHistory(itemID: 12, overall: overall))
            await vm.loadPriceHistory()
            XCTAssertEqual(vm.priceHistory, .empty, "点 \(overall.count) 個")
            XCTAssertTrue(vm.visibleHistorySeries.isEmpty)
            XCTAssertTrue(vm.historyLegend.isEmpty)
        }
    }

    // MARK: - AC-IOS-HIS-03 失敗(目安価格は壊さない。失敗のあとは取り直せる)

    func testFailureTextsAndRetry() async {
        let table: [(String, WishlistError, String)] = [
            ("通信失敗", WishlistError(code: .network, message: "offline"), "オフライン"),
            ("それ以外", WishlistError(code: .internal, status: 500, message: "boom"), "価格の推移を取得できませんでした"),
        ]
        for (name, error, want) in table {
            let (vm, service) = make()
            service.setEstimates(ItemEstimates(
                itemID: 12, summaryLow: 3000, summaryMid: 4500, summaryFetchedAt: iso("2026-10-03T00:30:00Z"),
                sites: [SiteEstimate(siteID: 1, low: 3000, mid: 4500, count: 5, suspiciousCount: 0, inStockCount: 4, status: .ok, fetchedAt: iso("2026-10-03T00:30:00Z"))]))
            service.setPriceHistory(history)
            await vm.loadEstimates()
            let summaryBefore = vm.summaryText
            service.failOnce(error)
            await vm.loadPriceHistory()
            XCTAssertEqual(vm.priceHistory, .failed(want), name)
            XCTAssertEqual(vm.summaryText, summaryBefore, "\(name): サマリは変えない")
            XCTAssertNil(vm.noticeText, "\(name): 目安価格の注記は立てない")

            await vm.loadPriceHistory()
            guard case .loaded = vm.priceHistory else { return XCTFail("\(name): 失敗のあとは取り直す: \(vm.priceHistory)") }
            XCTAssertEqual(historyCalls(service).count, 2, name)
        }
    }

    // MARK: - AC-IOS-HIS-04 凡例(既定は全体の最安だけ)

    func testLegendTogglesSiteSeries() async {
        let (vm, service) = make()
        service.setPriceHistory(history)
        XCTAssertTrue(vm.historyLegend.isEmpty, "取得前は空")
        await vm.loadPriceHistory()
        XCTAssertEqual(
            vm.historyLegend,
            [
                HistoryLegendItem(siteID: 1, name: "メルカリ", isSelected: false),
                HistoryLegendItem(siteID: 2, name: "Amazon", isSelected: false),
                HistoryLegendItem(siteID: 9, name: "サイト9", isSelected: false),
            ])
        XCTAssertEqual(vm.visibleHistorySeries.map(\.id), ["overall"])

        vm.toggleHistorySite(9)
        vm.toggleHistorySite(1)
        XCTAssertEqual(vm.selectedHistorySiteIDs, [1, 9])
        XCTAssertEqual(vm.visibleHistorySeries.map(\.id), ["overall", "site-1", "site-9"], "凡例の順")
        XCTAssertEqual(vm.visibleHistorySeries.map(\.name), ["全体の最安", "メルカリ", "サイト9"])
        XCTAssertEqual(vm.historyLegend.map(\.isSelected), [true, false, true])

        vm.toggleHistorySite(1)
        XCTAssertEqual(vm.visibleHistorySeries.map(\.id), ["overall", "site-9"])
        vm.toggleHistorySite(42)
        XCTAssertEqual(vm.selectedHistorySiteIDs, [9], "推移に無いサイトは無視")
    }

    // MARK: - AC-IOS-HIS-05 読み込み中

    func testLoadingStateWhileWaiting() async {
        let (vm, service) = make()
        service.setPriceHistory(history)
        let gate = Gate()
        service.gate = { call in if case .priceHistory = call { await gate.pass() } }
        let task = Task { await vm.loadPriceHistory() }
        await gate.waitUntilArrived()
        XCTAssertEqual(vm.priceHistory, .loading)
        await vm.loadPriceHistory()
        XCTAssertEqual(historyCalls(service).count, 1, "取得中に呼んでも重ねない")
        gate.open()
        await task.value
        guard case .loaded = vm.priceHistory else { return XCTFail("loaded のはず: \(vm.priceHistory)") }
    }
}
