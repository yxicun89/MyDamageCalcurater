import XCTest

@testable import WishlistCore

// AC-IOS-VM-SHEET-01〜09: 詳細シート(PWA の AC-SHEET と同じ挙動)。
@MainActor
final class ItemDetailViewModelTests: XCTestCase {
    private func makeViewModel(
        item: Item = T.gris, service: FakeWishlistService? = nil
    ) -> (ItemDetailViewModel, FakeWishlistService) {
        let service = service ?? T.fake()
        let genre = T.genres.first { $0.id == item.genreID }
        return (ItemDetailViewModel(item: item, genre: genre, sites: T.sites, service: service), service)
    }

    // MARK: - サイト行と検索ワード

    func testLinksFollowGenreSiteOrder() {
        let (viewModel, _) = makeViewModel()
        // S.H.Figuarts の siteIDs は [2(Amazon), 1(メルカリ), 3(その他)]
        XCTAssertEqual(viewModel.links.map(\.site.name), ["Amazon", "メルカリ", "その他"])
        XCTAssertEqual(viewModel.displayQuery, "S.H.Figuarts グリス")
        XCTAssertEqual(
            viewModel.links.dropFirst().first?.url.absoluteString,
            "https://jp.mercari.com/search?keyword=S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9&status=on_sale&sort=price&order=asc")
    }

    func testOptionlessItemHasNoTrailingSpaceInQuery() {
        let (viewModel, _) = makeViewModel(item: T.item(11, genre: 2, name: "ボルシャック"))
        XCTAssertEqual(viewModel.displayQuery, "ボルシャック")
    }

    func testLinksAreAvailableWithoutAnyServiceCall() {
        let (viewModel, service) = makeViewModel(service: T.fake(offline: true))
        XCTAssertEqual(viewModel.links.count, 3)
        XCTAssertEqual(service.calls, [])
    }

    // MARK: - その場編集は一時的

    func testBeginEditingInitializesDraftFromDisplayQueryAndCallsNoAPI() {
        let (viewModel, service) = makeViewModel()
        XCTAssertFalse(viewModel.isEditingQuery)
        viewModel.beginEditingQuery()
        XCTAssertTrue(viewModel.isEditingQuery)
        XCTAssertEqual(viewModel.queryDraft, "S.H.Figuarts グリス")
        XCTAssertEqual(service.calls, [])
    }

    /// 編集中でも、サイト行のリンクは入力値で変わる(一時的な値。PWA の AC-SHEET-07)。
    func testLinksReflectTheDraftWhileEditing() {
        let (viewModel, service) = makeViewModel()
        viewModel.beginEditingQuery()
        viewModel.queryDraft = "グリス 変身ベルト"
        XCTAssertEqual(viewModel.links.first?.url.absoluteString.contains("%E3%82%B0%E3%83%AA%E3%82%B9%20%E5%A4%89%E8%BA%AB%E3%83%99%E3%83%AB%E3%83%88"), true)
        XCTAssertEqual(viewModel.links.first?.query, "グリス 変身ベルト")
        XCTAssertEqual(viewModel.displayQuery, "S.H.Figuarts グリス", "保存するまで表示値は変わらない")
        XCTAssertEqual(service.calls, [])
    }

    /// 編集中に空にすると、リンクはテンプレートの検索ワードに戻る(override 無し)。
    func testEmptyDraftFallsBackToTemplateInLinks() {
        let (viewModel, _) = makeViewModel()
        viewModel.beginEditingQuery()
        viewModel.queryDraft = "  "
        XCTAssertEqual(viewModel.links.first?.query, "S.H.Figuarts グリス")
    }

    func testCancelDiscardsTheDraft() {
        let (viewModel, service) = makeViewModel()
        viewModel.beginEditingQuery()
        viewModel.queryDraft = "捨てる値"
        viewModel.cancelEditingQuery()
        XCTAssertFalse(viewModel.isEditingQuery)
        XCTAssertEqual(viewModel.links.first?.query, "S.H.Figuarts グリス")
        XCTAssertEqual(viewModel.displayQuery, "S.H.Figuarts グリス")
        XCTAssertEqual(service.calls, [])
    }

    // MARK: - 保存

    func testSaveSendsQueryOverrideOnlyAndUpdatesItem() async {
        let (viewModel, service) = makeViewModel()
        viewModel.beginEditingQuery()
        viewModel.queryDraft = "  グリス 変身ベルト \n"
        let saved = await viewModel.saveQuery()
        XCTAssertEqual(service.calls, [.updateItem(id: 12, patch: ItemPatch(queryOverride: .set("グリス 変身ベルト")))])
        XCTAssertEqual(saved?.queryOverride, "グリス 変身ベルト")
        XCTAssertEqual(viewModel.item.queryOverride, "グリス 変身ベルト")
        XCTAssertEqual(viewModel.displayQuery, "グリス 変身ベルト")
        XCTAssertFalse(viewModel.isEditingQuery)
        XCTAssertNil(viewModel.errorMessage)
    }

    /// 空にして保存すると null(テンプレートに戻す)。
    func testSavingEmptyDraftClearsTheOverride() async {
        let item = T.item(12, genre: 1, name: "グリス", override: "グリス 上書き")
        let (viewModel, service) = makeViewModel(item: item, service: FakeWishlistService(items: [item], genres: T.genres, sites: T.sites))
        viewModel.beginEditingQuery()
        XCTAssertEqual(viewModel.queryDraft, "グリス 上書き")
        viewModel.queryDraft = ""
        let saved = await viewModel.saveQuery()
        XCTAssertEqual(service.calls, [.updateItem(id: 12, patch: ItemPatch(queryOverride: .clear))])
        XCTAssertNil(saved?.queryOverride)
        XCTAssertEqual(viewModel.displayQuery, "S.H.Figuarts グリス")
    }

    func testSavingWhitespaceOnlyDraftAlsoClears() async {
        let (viewModel, service) = makeViewModel()
        viewModel.beginEditingQuery()
        viewModel.queryDraft = " \u{3000} "
        _ = await viewModel.saveQuery()
        XCTAssertEqual(service.calls, [.updateItem(id: 12, patch: ItemPatch(queryOverride: .clear))])
    }

    /// 保存に失敗したら理由を出し、編集中の値とリンクは残す(PWA の AC-SHEET-09)。
    func testSaveFailureKeepsEditingStateAndSetsErrorMessage() async {
        let (viewModel, service) = makeViewModel()
        viewModel.beginEditingQuery()
        viewModel.queryDraft = "S.H.Figuarts グリスX"
        service.offline = true
        let saved = await viewModel.saveQuery()
        XCTAssertNil(saved)
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertTrue(viewModel.isEditingQuery)
        XCTAssertEqual(viewModel.queryDraft, "S.H.Figuarts グリスX")
        XCTAssertFalse(viewModel.links.isEmpty)
        XCTAssertNil(viewModel.item.queryOverride, "保存済みの商品は変わらない")
    }

    // MARK: - サマリ

    func testSummaryIsEmptyWhileLoading() {
        let (viewModel, _) = makeViewModel()
        XCTAssertEqual(viewModel.summary, .loading)
        XCTAssertEqual(viewModel.summaryText, "")
    }

    func testSummaryWithoutPriceInfo() async {
        let (viewModel, service) = makeViewModel()
        await viewModel.loadEstimates()
        XCTAssertEqual(service.calls, [.estimates(itemID: 12)])
        XCTAssertEqual(viewModel.summary, .noPriceInfo)
        XCTAssertEqual(viewModel.summaryText, "まだ価格情報はありません")
    }

    /// AC-OFF-02: 通信できないときは「オフライン」。
    func testSummaryIsOfflineWhenTheNetworkFails() async {
        let (viewModel, _) = makeViewModel(service: T.fake(offline: true))
        await viewModel.loadEstimates()
        XCTAssertEqual(viewModel.summary, .offline)
        XCTAssertEqual(viewModel.summaryText, "オフライン")
        XCTAssertFalse(viewModel.links.isEmpty, "オフラインでもリンクは出る")
    }

    /// 401 などの失敗は、オフラインとは言わない。
    func testSummaryForNonNetworkFailureIsNotOffline() async {
        let service = T.fake()
        service.failure = T.unauthorized()
        let (viewModel, _) = makeViewModel(service: service)
        await viewModel.loadEstimates()
        XCTAssertEqual(viewModel.summary, .noPriceInfo)
    }
}
