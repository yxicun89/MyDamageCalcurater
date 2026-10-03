import XCTest

@testable import WishlistCore

// AC-IOS-VM-EDIT-01〜07: 編集(変えた項目だけ PATCH。空にした任意項目は null)。PWA の AC-EDIT と同じ挙動。
@MainActor
final class EditItemViewModelTests: XCTestCase {
    private func makeViewModel(item: Item = T.borushack, service: FakeWishlistService? = nil) -> (EditItemViewModel, FakeWishlistService) {
        let service = service ?? FakeWishlistService(items: [item], genres: T.genres, sites: T.sites)
        return (EditItemViewModel(item: item, genres: T.genres, service: service), service)
    }

    private var filled: Item {
        T.item(11, genre: 2, name: "ボルシャック", option: "銀トレジャー", override: "ボルシャック 銀", minPrice: 1500)
    }

    func testFormIsInitializedFromTheItem() {
        let (viewModel, _) = makeViewModel(item: filled)
        XCTAssertEqual(viewModel.name, "ボルシャック")
        XCTAssertEqual(viewModel.optionText, "銀トレジャー")
        XCTAssertEqual(viewModel.queryOverride, "ボルシャック 銀")
        XCTAssertEqual(viewModel.genreID, 2)
        XCTAssertEqual(viewModel.minPriceText, "1500")
        XCTAssertTrue(viewModel.isValid)
    }

    func testNilFieldsStartEmpty() {
        let (viewModel, _) = makeViewModel()
        XCTAssertEqual(viewModel.queryOverride, "")
        XCTAssertEqual(viewModel.minPriceText, "")
    }

    // MARK: - 差分

    func testNoChangeMeansEmptyPatchAndNoAPICall() async {
        let (viewModel, service) = makeViewModel()
        XCTAssertTrue(viewModel.patch.isEmpty)
        let saved = await viewModel.save()
        XCTAssertEqual(saved, T.borushack)
        XCTAssertEqual(service.calls, [])
    }

    func testWhitespaceOnlyChangesAreIgnored() {
        let (viewModel, _) = makeViewModel()
        viewModel.name = "ボルシャック  "
        viewModel.optionText = " 銀トレジャー"
        XCTAssertTrue(viewModel.patch.isEmpty)
    }

    func testChangedFieldsOnlyAreSent() async {
        let (viewModel, service) = makeViewModel()
        viewModel.name = "ボルシャック・ドラゴン"
        viewModel.queryOverride = "ボルシャック 銀"
        viewModel.genreID = 1
        viewModel.minPriceText = "1500"
        let expected = ItemPatch(genreID: 1, name: "ボルシャック・ドラゴン", queryOverride: .set("ボルシャック 銀"), minPrice: .set(1500))
        XCTAssertEqual(viewModel.patch, expected)
        let saved = await viewModel.save()
        XCTAssertEqual(service.calls, [.updateItem(id: 11, patch: expected)])
        XCTAssertEqual(saved?.name, "ボルシャック・ドラゴン")
        XCTAssertEqual(saved?.minPrice, 1500)
    }

    /// 空にした任意項目は null(消す)。
    func testClearingOptionalFieldsSendsClear() async {
        let (viewModel, service) = makeViewModel(item: filled)
        viewModel.optionText = ""
        XCTAssertEqual(viewModel.patch, ItemPatch(optionText: .clear))
        viewModel.queryOverride = "  "
        viewModel.minPriceText = ""
        let expected = ItemPatch(optionText: .clear, queryOverride: .clear, minPrice: .clear)
        XCTAssertEqual(viewModel.patch, expected)
        let saved = await viewModel.save()
        XCTAssertEqual(service.calls, [.updateItem(id: 11, patch: expected)])
        XCTAssertNil(saved?.optionText)
        XCTAssertNil(saved?.queryOverride)
        XCTAssertNil(saved?.minPrice)
    }

    /// 元も空のまま空なら、何も送らない(null を送らない)。
    func testStayingEmptyDoesNotSendClear() {
        let (viewModel, _) = makeViewModel()
        viewModel.queryOverride = ""
        viewModel.minPriceText = ""
        XCTAssertEqual(viewModel.patch.queryOverride, .keep)
        XCTAssertEqual(viewModel.patch.minPrice, .keep)
    }

    // MARK: - 検証

    func testInvalidMinPriceOrEmptyNameBlocksSaving() async {
        for bad in ["abc", "-5", "1.5", "１２"] {
            let (viewModel, service) = makeViewModel()
            viewModel.minPriceText = bad
            XCTAssertFalse(viewModel.isValid, bad)
            let saved = await viewModel.save()
            XCTAssertNil(saved, bad)
            XCTAssertNotNil(viewModel.errorMessage, bad)
            XCTAssertEqual(service.calls, [], bad)
        }
        let (viewModel, service) = makeViewModel()
        viewModel.name = "  "
        XCTAssertFalse(viewModel.isValid)
        let saved = await viewModel.save()
        XCTAssertNil(saved)
        XCTAssertEqual(service.calls, [])
    }

    func testZeroMinPriceIsValid() {
        let (viewModel, _) = makeViewModel()
        viewModel.minPriceText = "0"
        XCTAssertTrue(viewModel.isValid)
        XCTAssertEqual(viewModel.patch.minPrice, .set(0))
    }

    // MARK: - 画像の差し替え

    func testReplacingTheImageUsesPutOnly() async {
        let (viewModel, service) = makeViewModel()
        viewModel.setReplacementImage(ImageUpload(data: Data("NEW".utf8), filename: "new.png", contentType: "image/png"))
        let saved = await viewModel.save()
        XCTAssertEqual(service.calls, [.replaceItemImage(id: 11, filename: "new.png")])
        XCTAssertNotEqual(saved?.imageURLPath, T.borushack.imageURLPath)
    }

    func testPatchThenPutWhenBothChange() async {
        let (viewModel, service) = makeViewModel()
        viewModel.name = "改名"
        viewModel.setReplacementImage(ImageUpload(data: Data("NEW".utf8), filename: "new.png", contentType: "image/png"))
        let saved = await viewModel.save()
        XCTAssertEqual(
            service.calls, [.updateItem(id: 11, patch: ItemPatch(name: "改名")), .replaceItemImage(id: 11, filename: "new.png")])
        XCTAssertEqual(saved?.name, "改名", "最後に返ってきた商品に、先の PATCH の結果も入っている")
        XCTAssertNotEqual(saved?.imageURLPath, T.borushack.imageURLPath)
    }

    // MARK: - 失敗

    func testFailureKeepsInputAndReportsError() async {
        let (viewModel, service) = makeViewModel()
        viewModel.name = "改名"
        service.offline = true
        let saved = await viewModel.save()
        XCTAssertNil(saved)
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertEqual(viewModel.name, "改名")
        XCTAssertFalse(viewModel.isBusy)
    }
}
