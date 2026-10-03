import XCTest

@testable import WishlistCore

// AC-IOS-VM-SET-01〜08: 設定のジャンル・サイト(PWA の AC-SET-03〜07 と同じ挙動)。
@MainActor
final class SettingsListViewModelTests: XCTestCase {
    private func makeViewModel() -> (SettingsListViewModel, FakeWishlistService) {
        let service = T.fake()
        return (SettingsListViewModel(service: service, genres: T.genres, sites: T.sites), service)
    }

    func testMovingUp() {
        XCTAssertEqual(SettingsListViewModel.movingUp([2, 1, 3], id: 1), [1, 2, 3])
        XCTAssertEqual(SettingsListViewModel.movingUp([2, 1, 3], id: 3), [2, 3, 1])
        XCTAssertEqual(SettingsListViewModel.movingUp([2, 1, 3], id: 2), [2, 1, 3], "先頭は変えない")
        XCTAssertEqual(SettingsListViewModel.movingUp([2, 1, 3], id: 99), [2, 1, 3], "無い ID は変えない")
    }

    func testSortedGenresBySortOrderThenID() {
        let genres = [Genre(id: 3, name: "C", sortOrder: 2), Genre(id: 1, name: "A", sortOrder: 2), Genre(id: 2, name: "B", sortOrder: 1)]
        let viewModel = SettingsListViewModel(service: T.fake(), genres: genres, sites: [])
        XCTAssertEqual(viewModel.sortedGenres.map(\.id), [2, 1, 3])
    }

    // MARK: - サイト

    func testAddSiteWithoutQPlaceholderIsRejectedWithoutCallingTheAPI() async {
        let (viewModel, service) = makeViewModel()
        let site = await viewModel.addSite(name: "新サイト", searchURLTemplate: "https://new.example/s?q=", fetchType: .linkOnly, isReference: false)
        XCTAssertNil(site)
        XCTAssertTrue(viewModel.errorMessage?.contains("{q}") == true, viewModel.errorMessage ?? "nil")
        XCTAssertEqual(service.calls, [])
    }

    func testAddSiteRejectsNonHTTPTemplateAndBlankName() async {
        let (viewModel, service) = makeViewModel()
        let ftp = await viewModel.addSite(name: "新サイト", searchURLTemplate: "ftp://new.example/s?q={q}", fetchType: .linkOnly, isReference: false)
        let blank = await viewModel.addSite(name: "  ", searchURLTemplate: "https://new.example/s?q={q}", fetchType: .linkOnly, isReference: false)
        XCTAssertNil(ftp)
        XCTAssertNil(blank)
        XCTAssertEqual(service.calls, [])
    }

    func testAddSiteSendsAllFieldsAndAppendsToTheList() async {
        let (viewModel, service) = makeViewModel()
        let site = await viewModel.addSite(name: " 新サイト ", searchURLTemplate: "https://new.example/s?q={q}", fetchType: .scrape, isReference: true)
        XCTAssertEqual(
            service.calls,
            [.createSite(SiteCreate(name: "新サイト", searchURLTemplate: "https://new.example/s?q={q}", fetchType: .scrape, isReference: true))])
        XCTAssertEqual(site?.name, "新サイト")
        XCTAssertEqual(viewModel.sites.last, site)
        XCTAssertEqual(viewModel.sites.count, 4)
        XCTAssertNil(viewModel.errorMessage)
    }

    func testUpdateSiteSendsOnlyTheChangedFields() async {
        let (viewModel, service) = makeViewModel()
        let updated = await viewModel.updateSite(
            T.amazon, name: "アマゾン", searchURLTemplate: T.amazon.searchURLTemplate, fetchType: T.amazon.fetchType, isReference: T.amazon.isReference)
        XCTAssertEqual(service.calls, [.updateSite(id: 2, patch: SiteUpdate(name: "アマゾン"))])
        XCTAssertEqual(updated?.name, "アマゾン")
        XCTAssertEqual(viewModel.sites.first { $0.id == 2 }?.name, "アマゾン")
    }

    func testUpdateSiteWithNoChangeCallsNothing() async {
        let (viewModel, service) = makeViewModel()
        let updated = await viewModel.updateSite(
            T.amazon, name: T.amazon.name, searchURLTemplate: T.amazon.searchURLTemplate, fetchType: T.amazon.fetchType, isReference: T.amazon.isReference)
        XCTAssertEqual(updated, T.amazon)
        XCTAssertEqual(service.calls, [])
    }

    /// {q} の検証は編集でも効く。
    func testUpdateSiteValidatesTheTemplate() async {
        let (viewModel, service) = makeViewModel()
        let bad = await viewModel.updateSite(
            T.amazon, name: T.amazon.name, searchURLTemplate: "https://www.amazon.co.jp/s?k=", fetchType: .linkOnly, isReference: false)
        XCTAssertNil(bad)
        XCTAssertTrue(viewModel.errorMessage?.contains("{q}") == true)
        XCTAssertEqual(service.calls, [])
    }

    // MARK: - ジャンル

    func testAddGenreSendsSiteIDsInOrderAndOmitsBlankTemplate() async {
        let (viewModel, service) = makeViewModel()
        let genre = await viewModel.addGenre(name: " 新ジャンル ", queryTemplate: "{name}", siteIDs: [2, 1])
        XCTAssertEqual(service.calls, [.createGenre(GenreCreate(name: "新ジャンル", queryTemplate: "{name}", siteIDs: [2, 1]))])
        XCTAssertEqual(genre?.siteIDs, [2, 1])
        XCTAssertEqual(viewModel.genres.count, 3)

        let defaulted = await viewModel.addGenre(name: "既定", queryTemplate: "  ", siteIDs: [])
        XCTAssertEqual(service.calls.last, .createGenre(GenreCreate(name: "既定", queryTemplate: nil, siteIDs: [])))
        XCTAssertEqual(defaulted?.queryTemplate, "{name} {option}")
    }

    func testAddGenreWithBlankNameCallsNothing() async {
        let (viewModel, service) = makeViewModel()
        let genre = await viewModel.addGenre(name: "  ", queryTemplate: "{name}", siteIDs: [])
        XCTAssertNil(genre)
        XCTAssertEqual(service.calls, [])
    }

    /// site_ids は元と(順序込みで)違うときだけ、全件置き換えとして送る(PWA の AC-SET-04)。
    func testUpdateGenreSendsSiteIDsOnlyWhenTheOrderOrSetChanged() async {
        let (viewModel, service) = makeViewModel()
        let updated = await viewModel.updateGenre(T.figuarts, name: T.figuarts.name, queryTemplate: T.figuarts.queryTemplate, siteIDs: [1, 2])
        XCTAssertEqual(service.calls, [.updateGenre(id: 1, patch: GenreUpdate(siteIDs: [1, 2]))])
        XCTAssertEqual(updated?.siteIDs, [1, 2])
        XCTAssertEqual(viewModel.genres.first { $0.id == 1 }?.siteIDs, [1, 2])
    }

    func testUpdateGenreSendsOnlyTheName() async {
        let (viewModel, service) = makeViewModel()
        _ = await viewModel.updateGenre(T.figuarts, name: "フィギュアーツ", queryTemplate: T.figuarts.queryTemplate, siteIDs: T.figuarts.siteIDs)
        XCTAssertEqual(service.calls, [.updateGenre(id: 1, patch: GenreUpdate(name: "フィギュアーツ"))])
    }

    func testUpdateGenreWithNoChangeCallsNothing() async {
        let (viewModel, service) = makeViewModel()
        let updated = await viewModel.updateGenre(T.figuarts, name: T.figuarts.name, queryTemplate: T.figuarts.queryTemplate, siteIDs: T.figuarts.siteIDs)
        XCTAssertEqual(updated, T.figuarts)
        XCTAssertEqual(service.calls, [])
    }

    // MARK: - 失敗

    func testAPIFailureSetsErrorAndLeavesTheListsAlone() async {
        let (viewModel, service) = makeViewModel()
        service.offline = true
        let site = await viewModel.addSite(name: "新サイト", searchURLTemplate: "https://new.example/s?q={q}", fetchType: .linkOnly, isReference: false)
        let genre = await viewModel.addGenre(name: "新ジャンル", queryTemplate: "{name}", siteIDs: [])
        XCTAssertNil(site)
        XCTAssertNil(genre)
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertEqual(viewModel.sites, T.sites)
        XCTAssertEqual(viewModel.genres, T.genres)
    }
}
