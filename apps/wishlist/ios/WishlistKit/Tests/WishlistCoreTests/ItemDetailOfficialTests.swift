import XCTest

@testable import WishlistCore

// AC-IOS-OFF-05: 詳細シートの「公式」の行(ItemDetailViewModel.official。PWA の AC-OFF-05〜07 と同じ)。
@MainActor
final class ItemDetailOfficialTests: XCTestCase {
    private func make(_ item: Item, now: Date = iso("2026-10-04T00:00:00Z")) -> ItemDetailViewModel {
        ItemDetailViewModel(
            item: item, genre: T.figuarts, sites: T.sites, service: T.fake(), now: { now }, sleep: { _ in })
    }

    private func watched(_ status: OfficialStatus?) -> Item {
        var item = T.item(12, genre: 1, name: "グリス", sourceURL: "https://tamashii.example/item/1/")
        item.watchOfficial = true
        item.officialStatus = status
        return item
    }

    func testWatchedItemShowsSummaryAndEvidence() {
        let status = OfficialStatus(status: .preorder, evidence: ["予約受付中", "予約する"], checkedAt: iso("2026-10-03T18:00:00Z"))
        XCTAssertEqual(
            make(watched(status)).official,
            OfficialLines(summary: "公式: 予約受付中(10/4 確認)", evidence: "根拠: 予約受付中・予約する"))
    }

    func testChangeIsShownWithinSevenDaysUsingTheInjectedClock() {
        let status = OfficialStatus(
            status: .ended, evidence: ["販売終了"], checkedAt: iso("2026-10-03T18:00:00Z"), changedAt: iso("2026-10-02T18:00:00Z"),
            previousStatus: .available)
        XCTAssertEqual(make(watched(status)).official?.change, "10/3 に 販売中 → 販売終了")
        XCTAssertNil(make(watched(status), now: iso("2026-10-10T18:00:00Z")).official?.change, "8 日目は出さない")
    }

    func testLastAttemptNoteAndNotYetChecked() {
        let failedLater = OfficialStatus(
            status: .preorder, evidence: ["予約受付中"], checkedAt: iso("2026-10-03T18:00:00Z"), lastResult: .failed,
            lastAttemptAt: iso("2026-10-04T18:00:00Z"))
        XCTAssertEqual(make(watched(failedLater)).official?.lastAttempt, "最新の確認(10/5): 取得できませんでした")
        XCTAssertEqual(make(watched(nil)).official, OfficialLines(summary: "公式: まだ確認していません"))
    }

    func testNotWatchedShowsNothingEvenWithAStoredStatus() {
        var item = watched(OfficialStatus(status: .available, evidence: ["販売中"], checkedAt: iso("2026-10-03T18:00:00Z")))
        item.watchOfficial = false
        XCTAssertNil(make(item).official)
        XCTAssertNil(make(T.gris).official)
    }
}
