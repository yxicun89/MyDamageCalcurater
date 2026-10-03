import XCTest

@testable import WishlistCore

// AC-IOS-VM-REG-01〜08: 登録(写真 multipart / URL → 下書き → 確認 → JSON)。PWA の AC-REG と同じ挙動。
@MainActor
final class RegisterViewModelTests: XCTestCase {
    private let photo = ImageUpload(data: Data("PNG".utf8), filename: "photo.png", contentType: "image/png")
    private let draft = ItemDraft(
        name: "OGP のタイトル", imageURL: "https://img.example/og.png", sourceURL: "https://shop.example/p/1", genreID: 1)

    private func makeViewModel(initialGenreID: Int? = nil, service: FakeWishlistService? = nil) -> (RegisterViewModel, FakeWishlistService) {
        let service = service ?? T.fake()
        return (RegisterViewModel(service: service, genres: T.genres, initialGenreID: initialGenreID), service)
    }

    // MARK: - 初期値と登録できる条件

    func testInitialGenreIsTheChipGenreOtherwiseTheFirstBySortOrder() {
        XCTAssertEqual(makeViewModel(initialGenreID: 2).0.genreID, 2)
        XCTAssertEqual(makeViewModel(initialGenreID: nil).0.genreID, 1)
        XCTAssertEqual(makeViewModel(initialGenreID: 99).0.genreID, 1, "ジャンルに無い ID は使わない")
        let reversed = [Genre(id: 5, name: "後", sortOrder: 9), Genre(id: 6, name: "先", sortOrder: 1)]
        XCTAssertEqual(RegisterViewModel(service: T.fake(), genres: reversed, initialGenreID: nil).genreID, 6)
    }

    func testCanRegisterNeedsNameGenreAndImage() {
        let (viewModel, _) = makeViewModel()
        XCTAssertFalse(viewModel.canRegister)
        viewModel.name = "名前だけ"
        XCTAssertFalse(viewModel.canRegister)
        viewModel.setPhoto(photo)
        XCTAssertTrue(viewModel.canRegister)
        viewModel.name = "  \u{3000}"
        XCTAssertFalse(viewModel.canRegister, "空白だけの名前は不可")
        viewModel.name = "名前"
        viewModel.genreID = nil
        XCTAssertFalse(viewModel.canRegister)
        viewModel.genreID = 1
        viewModel.clearPhoto()
        XCTAssertFalse(viewModel.canRegister)
    }

    func testRegisterDoesNothingWhenNotReady() async {
        let (viewModel, service) = makeViewModel()
        viewModel.name = "名前だけ"
        let item = await viewModel.register()
        XCTAssertNil(item)
        XCTAssertEqual(service.calls, [])
    }

    // MARK: - 写真で登録(multipart)

    func testRegisterWithPhotoUsesMultipart() async {
        let (viewModel, service) = makeViewModel(initialGenreID: 2)
        viewModel.name = "  新商品 "
        viewModel.setPhoto(photo)
        let item = await viewModel.register()
        XCTAssertEqual(service.calls, [.createItemMultipart(ItemFields(genreID: 2, name: "新商品"), filename: "photo.png")])
        XCTAssertEqual(item?.name, "新商品")
        XCTAssertEqual(item?.genreID, 2)
        XCTAssertNil(viewModel.errorMessage)
    }

    // MARK: - URL → 下書き → 確認 → JSON

    func testFetchDraftFillsNameAndGenreFromDraft() async {
        let (viewModel, service) = makeViewModel(initialGenreID: 2)
        service.draft = draft
        viewModel.urlText = "  https://shop.example/p/1 "
        await viewModel.fetchDraft()
        XCTAssertEqual(service.calls, [.draftFromURL("https://shop.example/p/1", genreID: 2)])
        XCTAssertEqual(viewModel.draft, draft)
        XCTAssertEqual(viewModel.name, "OGP のタイトル")
        XCTAssertEqual(viewModel.genreID, 1, "下書きの genre_id が選択に反映される")
        XCTAssertNil(viewModel.errorMessage)
    }

    func testFetchDraftIgnoresUnknownDraftGenre() async {
        let (viewModel, service) = makeViewModel(initialGenreID: 2)
        service.draft = ItemDraft(name: "N", imageURL: nil, sourceURL: "https://shop.example/p/1", genreID: 99)
        viewModel.urlText = "https://shop.example/p/1"
        await viewModel.fetchDraft()
        XCTAssertEqual(viewModel.genreID, 2)
    }

    func testFetchDraftWithBlankURLCallsNothing() async {
        let (viewModel, service) = makeViewModel()
        viewModel.urlText = "   "
        await viewModel.fetchDraft()
        XCTAssertEqual(service.calls, [])
    }

    func testRegisterFromDraftUsesJSONWithEditedName() async {
        let (viewModel, service) = makeViewModel()
        service.draft = draft
        viewModel.urlText = "https://shop.example/p/1"
        await viewModel.fetchDraft()
        viewModel.name = "直した名前"
        XCTAssertTrue(viewModel.canRegister)
        let item = await viewModel.register()
        XCTAssertEqual(
            service.calls.last,
            .createItemFromURL(
                ItemFields(genreID: 1, name: "直した名前", sourceURL: "https://shop.example/p/1"), imageURL: "https://img.example/og.png"))
        XCTAssertEqual(item?.name, "直した名前")
    }

    /// 下書きに画像が無く、写真も無いときは登録できない(API は画像が必須)。
    func testDraftWithoutImageCannotRegisterUntilAPhotoIsAdded() async {
        let (viewModel, service) = makeViewModel()
        service.draft = ItemDraft(name: "画像なし", imageURL: nil, sourceURL: "https://shop.example/p/2", genreID: nil)
        viewModel.urlText = "https://shop.example/p/2"
        await viewModel.fetchDraft()
        XCTAssertEqual(viewModel.name, "画像なし")
        XCTAssertFalse(viewModel.canRegister)
        viewModel.setPhoto(photo)
        XCTAssertTrue(viewModel.canRegister)
    }

    /// 写真と下書きの両方があれば写真を使い、source_url は下書きのものを付ける。
    func testPhotoWinsOverDraftImageButKeepsSourceURL() async {
        let (viewModel, service) = makeViewModel()
        service.draft = draft
        viewModel.urlText = "https://shop.example/p/1"
        await viewModel.fetchDraft()
        viewModel.setPhoto(photo)
        _ = await viewModel.register()
        XCTAssertEqual(
            service.calls.last,
            .createItemMultipart(ItemFields(genreID: 1, name: "OGP のタイトル", sourceURL: "https://shop.example/p/1"), filename: "photo.png"))
    }

    // MARK: - 失敗

    /// 取得に失敗したら理由を出し、入力は残す(PWA の AC-REG-05)。
    func testFetchDraftFailureKeepsInput() async {
        let (viewModel, service) = makeViewModel()
        viewModel.urlText = "https://shop.example/p/1"
        viewModel.name = "入力済み"
        service.offline = true
        await viewModel.fetchDraft()
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertEqual(viewModel.urlText, "https://shop.example/p/1")
        XCTAssertEqual(viewModel.name, "入力済み")
        XCTAssertNil(viewModel.draft)
        XCTAssertFalse(viewModel.isBusy)
    }

    func testRegisterFailureKeepsInputAndReportsError() async {
        let (viewModel, service) = makeViewModel()
        viewModel.name = "新商品"
        viewModel.setPhoto(photo)
        service.offline = true
        let item = await viewModel.register()
        XCTAssertNil(item)
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertEqual(viewModel.name, "新商品")
        XCTAssertEqual(viewModel.photo, photo)
        XCTAssertFalse(viewModel.isBusy)

        service.offline = false
        let retried = await viewModel.register()
        XCTAssertEqual(retried?.name, "新商品", "入力が残っているので、そのまま再試行できる")
    }
}
