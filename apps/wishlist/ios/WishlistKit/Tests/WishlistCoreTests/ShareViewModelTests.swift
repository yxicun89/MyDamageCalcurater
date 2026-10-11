import XCTest

@testable import WishlistCore

final class SharedURLTests: XCTestCase {
    func testExtractsAPlainURL() {
        XCTAssertEqual(SharedURL.extract(from: "https://example.com/p/1")?.absoluteString, "https://example.com/p/1")
    }

    func testExtractsTheFirstURLFromSurroundingText() {
        XCTAssertEqual(
            SharedURL.extract(from: "これ欲しい https://example.com/p/1 と https://example.com/p/2")?.absoluteString,
            "https://example.com/p/1")
    }

    func testKeepsQueryAndFragment() {
        XCTAssertEqual(
            SharedURL.extract(from: "https://example.com/a?b=1&c=2#x")?.absoluteString, "https://example.com/a?b=1&c=2#x")
    }

    func testStopsAtWhitespaceAndNewlines() {
        XCTAssertEqual(SharedURL.extract(from: "  https://example.com/a\nメモ")?.absoluteString, "https://example.com/a")
    }

    func testExcludesTrailingJapanesePunctuationAndBrackets() {
        XCTAssertEqual(SharedURL.extract(from: "（https://example.com/a）")?.absoluteString, "https://example.com/a")
        XCTAssertEqual(SharedURL.extract(from: "https://example.com/a。")?.absoluteString, "https://example.com/a")
        XCTAssertEqual(SharedURL.extract(from: "「https://example.com/a」です")?.absoluteString, "https://example.com/a")
    }

    func testReturnsNilWhenThereIsNoHTTPURL() {
        XCTAssertNil(SharedURL.extract(from: ""))
        XCTAssertNil(SharedURL.extract(from: "URL なし"))
        XCTAssertNil(SharedURL.extract(from: "ftp://example.com/a"))
        XCTAssertNil(SharedURL.extract(from: "javascript:alert(1)"))
    }
}

// AC-IOS-SHARE-01〜09: Share Extension の ShareViewModel(共有された URL から from-url で下書き → ジャンルを選んで保存)。
@MainActor
final class ShareViewModelTests: XCTestCase {
    private let url = URL(string: "https://shop.example/p/1")!
    private let draft = ItemDraft(
        name: "S.H.Figuarts 仮面ライダーグリス", imageURL: "https://img.example/og.png", sourceURL: "https://shop.example/p/1", genreID: nil)

    private func makeViewModel(
        settings: WishlistSettings = T.configured, service: FakeWishlistService? = nil, draft: ItemDraft? = nil
    ) -> (ShareViewModel, FakeWishlistService) {
        let service = service ?? T.fake()
        service.draft = draft ?? self.draft
        return (ShareViewModel(sharedURL: url, settings: settings, service: service), service)
    }

    // MARK: - 設定・入力が無いとき

    // 【5-3 で置換】旧 testNeedsSettingsWhenTokenIsMissingAndCallsNoAPI(トークン未設定)。トークンを廃止したので URL の有無だけを見る。
    func testNeedsSettingsWhenBaseURLIsMissingAndCallsNoAPI() async {
        let (viewModel, service) = makeViewModel(settings: T.localOnly)
        await viewModel.load()
        XCTAssertEqual(viewModel.phase, .needsSettings)
        XCTAssertEqual(service.calls, [])
    }

    func testFailsWithoutAnAPICallWhenNothingSharedOrNotHTTP() async throws {
        for shared in [nil, URL(string: "ftp://shop.example/p/1")] as [URL?] {
            let service = T.fake()
            let viewModel = ShareViewModel(sharedURL: shared, settings: T.configured, service: service)
            await viewModel.load()
            guard case .failed = viewModel.phase else {
                XCTFail("failed のはず: \(viewModel.phase)")
                continue
            }
            XCTAssertEqual(service.calls, [])
        }
    }

    // MARK: - 下書き

    func testLoadFetchesGenresAndDraftThenIsReady() async {
        let (viewModel, service) = makeViewModel()
        XCTAssertEqual(viewModel.phase, .loading)
        await viewModel.load()
        XCTAssertEqual(viewModel.phase, .ready)
        XCTAssertTrue(service.calls.contains(.listGenres))
        XCTAssertTrue(service.calls.contains(.draftFromURL("https://shop.example/p/1", genreID: nil)))
        XCTAssertEqual(viewModel.draft, draft)
        XCTAssertEqual(viewModel.genres, T.genres)
        XCTAssertEqual(viewModel.name, "S.H.Figuarts 仮面ライダーグリス")
        XCTAssertEqual(viewModel.previewImageURL?.absoluteString, "https://img.example/og.png")
        // 【5-3 で置換】旧: 下書きにジャンルが無ければ sortOrder 昇順の先頭。ユーザー決定 2026-10-11: 未選択にして、選ぶまで「追加」を押せない
        XCTAssertNil(viewModel.genreID)
        XCTAssertFalse(viewModel.canSave)
    }

    func testDraftGenreIsPreselectedWhenItExists() async {
        var withGenre = draft
        withGenre.genreID = 2
        let (viewModel, _) = makeViewModel(draft: withGenre)
        await viewModel.load()
        XCTAssertEqual(viewModel.genreID, 2)

        var unknown = draft
        unknown.genreID = 99
        let (other, _) = makeViewModel(draft: unknown)
        await other.load()
        XCTAssertNil(other.genreID, "【5-3 で置換】旧: 先頭のジャンル。ジャンルに無い ID は未選択")
    }

    func testDraftFetchFailureIsReported() async {
        let service = T.fake()
        let viewModel = ShareViewModel(sharedURL: url, settings: T.configured, service: service)  // draft なし → 422
        await viewModel.load()
        guard case .failed(let message) = viewModel.phase else { return XCTFail("failed のはず: \(viewModel.phase)") }
        XCTAssertFalse(message.isEmpty)
        XCTAssertFalse(viewModel.canSave)
    }

    func testOfflineIsReportedAsFailed() async {
        let (viewModel, _) = makeViewModel(service: T.fake(offline: true))
        await viewModel.load()
        guard case .failed = viewModel.phase else { return XCTFail("failed のはず: \(viewModel.phase)") }
    }

    // MARK: - 保存

    func testSaveCreatesTheItemFromTheDraftImageURL() async {
        let (viewModel, service) = makeViewModel()
        await viewModel.load()
        viewModel.genreID = 1  // 5-3: 既定では未選択
        await viewModel.save()
        XCTAssertEqual(
            service.calls.last,
            .createItemFromURL(
                ItemFields(genreID: 1, name: "S.H.Figuarts 仮面ライダーグリス", sourceURL: "https://shop.example/p/1"),
                imageURL: "https://img.example/og.png"))
        guard case .saved(let item) = viewModel.phase else { return XCTFail("saved のはず: \(viewModel.phase)") }
        XCTAssertEqual(item.name, "S.H.Figuarts 仮面ライダーグリス")
        XCTAssertEqual(item.genreID, 1)
    }

    func testEditedNameAndGenreAreUsed() async {
        let (viewModel, service) = makeViewModel()
        await viewModel.load()
        viewModel.name = "  グリス "
        viewModel.genreID = 2
        await viewModel.save()
        XCTAssertEqual(
            service.calls.last,
            .createItemFromURL(
                ItemFields(genreID: 2, name: "グリス", sourceURL: "https://shop.example/p/1"), imageURL: "https://img.example/og.png"))
    }

    /// OGP に画像が無いときは保存できない(API は画像が必須)。
    func testDraftWithoutImageCannotBeSaved() async {
        let (viewModel, service) = makeViewModel(draft: ItemDraft(name: "画像なし", imageURL: nil, sourceURL: "https://shop.example/p/1"))
        await viewModel.load()
        XCTAssertEqual(viewModel.phase, .ready)
        XCTAssertNil(viewModel.previewImageURL)
        XCTAssertFalse(viewModel.canSave)
        let callsBefore = service.calls.count
        await viewModel.save()
        XCTAssertEqual(service.calls.count, callsBefore, "保存の API を呼ばない")
        XCTAssertEqual(viewModel.phase, .ready)
    }

    func testBlankNameCannotBeSaved() async {
        let (viewModel, _) = makeViewModel()
        await viewModel.load()
        viewModel.genreID = 1  // 5-3: 既定では未選択
        viewModel.name = "  "
        XCTAssertFalse(viewModel.canSave)
    }

    func testNonHTTPPreviewImageIsNotShown() async {
        let (viewModel, _) = makeViewModel(draft: ItemDraft(name: "N", imageURL: "javascript:alert(1)", sourceURL: "https://shop.example/p/1"))
        await viewModel.load()
        XCTAssertNil(viewModel.previewImageURL)
    }

    /// 保存に失敗しても入力は残り、直ったらそのまま再保存できる。
    func testSaveFailureKeepsInputAndCanBeRetried() async {
        let (viewModel, service) = makeViewModel()
        await viewModel.load()
        viewModel.genreID = 1  // 5-3: 既定では未選択
        viewModel.name = "直した名前"
        service.offline = true
        await viewModel.save()
        guard case .failed = viewModel.phase else { return XCTFail("failed のはず: \(viewModel.phase)") }
        XCTAssertEqual(viewModel.name, "直した名前")
        XCTAssertTrue(viewModel.canSave)

        service.offline = false
        await viewModel.save()
        guard case .saved(let item) = viewModel.phase else { return XCTFail("saved のはず: \(viewModel.phase)") }
        XCTAssertEqual(item.name, "直した名前")
    }
}
