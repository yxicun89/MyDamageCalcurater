import XCTest

@testable import WishlistCore

// AC-IOS-5-3f: 追加の流れ(URL を貼る → 下書き → 推定ジャンルが選ばれ 1 タップで追加)。docs/phase5-spec.md 5-2「追加」・5-3。
@MainActor
final class RegisterFlowTests: XCTestCase {
    private let url = "https://shop.example/p/1"

    private func draft(genreID: Int?, guessed: Bool, image: String? = "https://img.example/og.png", name: String = "グリス") -> ItemDraft {
        ItemDraft(name: name, imageURL: image, sourceURL: "https://shop.example/p/1", genreID: genreID, genreGuessed: guessed)
    }

    private func makeViewModel(initialGenreID: Int? = nil, draft: ItemDraft?) -> (RegisterViewModel, FakeWishlistService) {
        let service = T.fake()
        service.draft = draft
        return (RegisterViewModel(service: service, genres: T.genres, initialGenreID: initialGenreID), service)
    }

    /// AC-IOS-5-3f-1: URL を取得すると、サーバーが推定したジャンルが選ばれ、そのまま追加できる。from-url に genre_id は渡さない
    func testGuessedGenreIsPreselectedAndRegisterIsOneTap() async {
        let (viewModel, service) = makeViewModel(draft: draft(genreID: 2, guessed: true))
        viewModel.urlText = url
        await viewModel.fetchDraft()
        XCTAssertEqual(service.calls, [.draftFromURL(url, genreID: nil)])
        XCTAssertEqual(viewModel.genreID, 2)
        XCTAssertEqual(viewModel.name, "グリス")
        XCTAssertTrue(viewModel.canRegister)
        let item = await viewModel.register()
        XCTAssertEqual(item?.genreID, 2)
        XCTAssertEqual(
            service.calls.last,
            .createItemFromURL(ItemFields(genreID: 2, name: "グリス", sourceURL: url), imageURL: "https://img.example/og.png"))
    }

    /// AC-IOS-5-3f-2: 推定できなかった(genre_id が null)ときは、ジャンルが未選択なら「追加」を押せない。選ぶと押せる
    func testNullGenreDisablesRegisterUntilOneIsPicked() async {
        let (viewModel, _) = makeViewModel(draft: draft(genreID: nil, guessed: false))
        viewModel.urlText = url
        await viewModel.fetchDraft()
        XCTAssertNil(viewModel.genreID)
        XCTAssertFalse(viewModel.canRegister, "名前も画像もあるが、ジャンルが未選択")
        viewModel.genreID = 1
        XCTAssertTrue(viewModel.canRegister)
    }

    /// AC-IOS-5-3f-3: 下書きのジャンルが null で、ホームでジャンルを絞っていたら、そのジャンルのまま
    func testNullGenreKeepsTheFilteredGenre() async {
        let (viewModel, _) = makeViewModel(initialGenreID: 2, draft: draft(genreID: nil, guessed: false))
        viewModel.urlText = url
        await viewModel.fetchDraft()
        XCTAssertEqual(viewModel.genreID, 2)
        XCTAssertTrue(viewModel.canRegister)
    }

    /// AC-IOS-5-3f-4: 推定したジャンルは、絵柄を押して替えられる(替えた先で登録される)
    func testGuessedGenreCanBeChanged() async {
        let (viewModel, service) = makeViewModel(draft: draft(genreID: 2, guessed: true))
        viewModel.urlText = url
        await viewModel.fetchDraft()
        viewModel.genreID = 1
        _ = await viewModel.register()
        XCTAssertEqual(
            service.calls.last,
            .createItemFromURL(ItemFields(genreID: 1, name: "グリス", sourceURL: url), imageURL: "https://img.example/og.png"))
    }

    /// AC-IOS-5-3f-5: 画像が取れなかった下書きは、写真を選ぶまで追加できない(ジャンルが推定できていても)
    func testNoImageRequiresAPhoto() async {
        let (viewModel, _) = makeViewModel(draft: draft(genreID: 2, guessed: true, image: nil))
        viewModel.urlText = url
        await viewModel.fetchDraft()
        XCTAssertFalse(viewModel.canRegister)
        viewModel.setPhoto(ImageUpload(data: Data("PNG".utf8), filename: "p.png", contentType: "image/png"))
        XCTAssertTrue(viewModel.canRegister)
    }

    /// AC-IOS-5-3f-6: 名前が空(駿河屋のように og:title が無い)なら、入力するまで追加できない
    func testEmptyDraftNameRequiresTyping() async {
        let (viewModel, _) = makeViewModel(draft: draft(genreID: 2, guessed: true, name: ""))
        viewModel.urlText = url
        await viewModel.fetchDraft()
        XCTAssertFalse(viewModel.canRegister)
        viewModel.name = "手で入れた名前"
        XCTAssertTrue(viewModel.canRegister)
    }

    // MARK: - コピーした URL を追加

    /// AC-IOS-5-3f-7: 押したときに渡されたクリップボードの文字が http(s) の URL なら、そのまま下書きを取る
    func testPasteURLFetchesTheDraft() async {
        let (viewModel, service) = makeViewModel(draft: draft(genreID: 2, guessed: true))
        let ok = await viewModel.pasteURL("  \(url)\n")
        XCTAssertTrue(ok)
        XCTAssertEqual(viewModel.urlText, url)
        XCTAssertEqual(service.calls, [.draftFromURL(url, genreID: nil)])
        XCTAssertEqual(viewModel.genreID, 2)
    }

    /// AC-IOS-5-3f-8: 読めない・URL でない・http(s) でないときは false(画面は URL の入力欄を出す)。API は呼ばない
    func testPasteURLFallsBackToManualEntry() async {
        for copied: String? in [nil, "", "   ", "ただの文章", "ftp://shop.example/p/1", "javascript:alert(1)", "shop.example/p/1"] {
            let (viewModel, service) = makeViewModel(draft: draft(genreID: 2, guessed: true))
            let ok = await viewModel.pasteURL(copied)
            XCTAssertFalse(ok, "\(String(describing: copied))")
            XCTAssertEqual(service.calls, [])
            XCTAssertNil(viewModel.draft)
        }
    }

    /// AC-IOS-5-3f-9: 取得に失敗したら理由を出し、入力は残す
    func testPasteURLFailureKeepsTheURLAndShowsAnError() async {
        let (viewModel, _) = makeViewModel(draft: nil)  // 422
        let ok = await viewModel.pasteURL(url)
        XCTAssertTrue(ok, "URL としては読めた(取得の失敗は errorMessage で伝える)")
        XCTAssertEqual(viewModel.urlText, url)
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertFalse(viewModel.canRegister)
    }
}
