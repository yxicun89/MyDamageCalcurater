import XCTest

@testable import WishlistCore

// AC-IOS-5-3g: Share Extension。共有拡張の JavaScript が読んだ og:title・og:image を from-url に渡す(サーバーはページを取りに行かない)。
// docs/phase5-spec.md 5-3「決めたこと」。ユーザー決定 2026-10-11。
final class SharedPageTests: XCTestCase {
    /// AC-IOS-5-3g-1: JavaScript の結果(url・title・imageURL)を読む
    func testParseReadsTheJavaScriptResults() {
        let page = SharedPage.parse(["url": "https://shop.example/p/1", "title": " グリス | ショップ ", "imageURL": "https://img.example/og.png"])
        XCTAssertEqual(page.url?.absoluteString, "https://shop.example/p/1")
        XCTAssertEqual(page.title, "グリス | ショップ")
        XCTAssertEqual(page.imageURL, "https://img.example/og.png")
    }

    /// AC-IOS-5-3g-2: 空・文字でない値は nil。url が http(s) でなければ nil。辞書が無くても作れる
    func testParseIgnoresEmptyAndInvalidValues() {
        let page = SharedPage.parse(["url": "javascript:alert(1)", "title": "  ", "imageURL": 5])
        XCTAssertNil(page.url)
        XCTAssertNil(page.title)
        XCTAssertNil(page.imageURL)
        XCTAssertEqual(SharedPage.parse(nil), SharedPage(url: nil))
        XCTAssertEqual(SharedPage.parse([:]), SharedPage(url: nil))
    }
}

@MainActor
final class ShareFlowTests: XCTestCase {
    private let url = URL(string: "https://shop.example/p/1")!
    private let image = "https://img.example/og.png"

    private func draft(genreID: Int?, guessed: Bool = true, image: String? = "https://img.example/og.png") -> ItemDraft {
        ItemDraft(name: "グリス", imageURL: image, sourceURL: "https://shop.example/p/1", genreID: genreID, genreGuessed: guessed)
    }

    private func makeViewModel(
        page: SharedPage? = nil, settings: WishlistSettings = T.configured, draft: ItemDraft?
    ) -> (ShareViewModel, FakeWishlistService) {
        let service = T.fake()
        service.draft = draft
        let page = page ?? SharedPage(url: url, title: "グリス | ショップ", imageURL: image)
        return (ShareViewModel(sharedPage: page, settings: settings, service: service), service)
    }

    /// AC-IOS-5-3g-3: JavaScript が読んだ title・image_url をそのまま from-url に渡す。ページを取りに行く経路(genre_id 付きの旧呼び出し)は使わない
    func testLoadPassesTitleAndImageURLToFromURL() async {
        let (viewModel, service) = makeViewModel(draft: draft(genreID: 2))
        await viewModel.load()
        XCTAssertEqual(viewModel.phase, .ready)
        XCTAssertTrue(service.calls.contains(.draftFromPage("https://shop.example/p/1", title: "グリス | ショップ", imageURL: image)))
        XCTAssertFalse(service.calls.contains { if case .draftFromURL = $0 { true } else { false } })
    }

    /// AC-IOS-5-3g-4: 推定したジャンルが選ばれ、「追加」で /api/items に image_url で登録する
    func testGuessedGenreIsPreselectedAndSaveCreatesTheItem() async {
        let (viewModel, service) = makeViewModel(draft: draft(genreID: 2))
        await viewModel.load()
        XCTAssertEqual(viewModel.genreID, 2)
        XCTAssertTrue(viewModel.canSave)
        await viewModel.save()
        XCTAssertEqual(
            service.calls.last,
            .createItemFromURL(ItemFields(genreID: 2, name: "グリス", sourceURL: "https://shop.example/p/1"), imageURL: image))
        guard case .saved = viewModel.phase else { return XCTFail("saved のはず: \(viewModel.phase)") }
    }

    /// AC-IOS-5-3g-5: ジャンルが推定できなかったら未選択。選ぶまで「追加」は押せず、API も呼ばない
    func testUnguessedGenreStaysUnselectedAndBlocksSave() async {
        let (viewModel, service) = makeViewModel(draft: draft(genreID: nil, guessed: false))
        await viewModel.load()
        XCTAssertEqual(viewModel.phase, .ready)
        XCTAssertNil(viewModel.genreID)
        XCTAssertFalse(viewModel.canSave)
        let before = service.calls.count
        await viewModel.save()
        XCTAssertEqual(service.calls.count, before)
        viewModel.genreID = 1
        XCTAssertTrue(viewModel.canSave)
    }

    /// AC-IOS-5-3g-6: 画像が取れなかった下書きは追加できない
    func testNoImageCannotBeSaved() async {
        let (viewModel, _) = makeViewModel(draft: draft(genreID: 2, image: nil))
        await viewModel.load()
        XCTAssertFalse(viewModel.canSave)
    }

    /// AC-IOS-5-3g-7: JavaScript が何も読めなかったとき(title・image なし)も url だけで from-url を呼ぶ(サーバーが取りに行く)
    func testWithoutJavaScriptResultsFallsBackToURLOnly() async {
        let (viewModel, service) = makeViewModel(page: SharedPage(url: url), draft: draft(genreID: 1))
        await viewModel.load()
        XCTAssertTrue(service.calls.contains(.draftFromPage("https://shop.example/p/1", title: nil, imageURL: nil)))
        XCTAssertEqual(viewModel.phase, .ready)
    }

    /// AC-IOS-5-3g-8: 接続先(ビルド設定)が無ければ API を呼ばない。トークンの有無は見ない
    func testWithoutBaseURLCallsNoAPI() async {
        let (viewModel, service) = makeViewModel(settings: T.localOnly, draft: draft(genreID: 2))
        await viewModel.load()
        XCTAssertEqual(viewModel.phase, .needsSettings)
        XCTAssertEqual(service.calls, [])
    }

    /// AC-IOS-5-3g-9: URL が無い・http(s) でない共有は API を呼ばず failed
    func testNoURLFails() async {
        let (viewModel, service) = makeViewModel(page: SharedPage(url: nil, title: "t", imageURL: image), draft: draft(genreID: 2))
        await viewModel.load()
        guard case .failed = viewModel.phase else { return XCTFail("failed のはず: \(viewModel.phase)") }
        XCTAssertEqual(service.calls, [])
    }
}
