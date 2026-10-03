import XCTest

@testable import WishlistCore

// AC-IOS-OFF-06: 編集の「公式ページを監視する」(PWA の AC-OFF-09 と同じ。docs/phase4-spec.md 4-3)。
@MainActor
final class EditItemOfficialTests: XCTestCase {
    private func make(_ item: Item) -> (EditItemViewModel, FakeWishlistService) {
        let service = FakeWishlistService(items: [item], genres: T.genres, sites: T.sites)
        return (EditItemViewModel(item: item, genres: T.genres, service: service), service)
    }

    private func item(watch: Bool, source: String?) -> Item {
        var item = T.item(12, genre: 1, name: "グリス", sourceURL: source)
        item.watchOfficial = watch
        return item
    }

    func testInitialValueAndAvailability() {
        let (on, _) = make(item(watch: true, source: "https://tamashii.example/item/1/"))
        XCTAssertTrue(on.watchOfficial)
        XCTAssertTrue(on.canWatchOfficial)
        XCTAssertNil(on.watchOfficialHint)

        for source in [nil, "", "  "] as [String?] {
            let (off, _) = make(item(watch: false, source: source))
            XCTAssertFalse(off.watchOfficial)
            XCTAssertFalse(off.canWatchOfficial, "sourceURL = \(String(describing: source))")
            XCTAssertEqual(off.watchOfficialHint, "公式ページの URL が無いので監視できません")
        }
    }

    func testTurningOffSendsOnlyWatchOfficial() async {
        let (viewModel, service) = make(item(watch: true, source: "https://tamashii.example/item/1/"))
        viewModel.watchOfficial = false
        XCTAssertEqual(viewModel.patch, ItemPatch(watchOfficial: false))
        let saved = await viewModel.save()
        XCTAssertEqual(service.calls, [.updateItem(id: 12, patch: ItemPatch(watchOfficial: false))])
        XCTAssertEqual(saved?.watchOfficial, false)
    }

    func testTurningOnSendsTrue() async {
        let (viewModel, service) = make(item(watch: false, source: "https://tamashii.example/item/1/"))
        viewModel.watchOfficial = true
        _ = await viewModel.save()
        XCTAssertEqual(service.calls, [.updateItem(id: 12, patch: ItemPatch(watchOfficial: true))])
    }

    func testUnchangedOrUnavailableIsNotSent() {
        let (same, _) = make(item(watch: true, source: "https://tamashii.example/item/1/"))
        same.name = "グリス改"
        XCTAssertEqual(same.patch, ItemPatch(name: "グリス改"), "変えなければ送らない")

        let (noURL, _) = make(item(watch: false, source: nil))
        noURL.watchOfficial = true
        XCTAssertTrue(noURL.patch.isEmpty, "sourceURL が無いときは ON にしても送らない")
    }
}
