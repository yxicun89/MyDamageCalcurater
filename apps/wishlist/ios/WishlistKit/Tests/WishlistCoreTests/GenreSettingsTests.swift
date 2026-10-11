import XCTest

@testable import WishlistCore

// AC-IOS-5-3i: ジャンルの編集（色 12 色・絵柄 6 つ・URL の手がかり）。docs/phase5-spec.md 5-2「ジャンルの編集」。
final class GenrePaletteTests: XCTestCase {
    /// AC-IOS-5-3i-1: 色の見本は 12 色（Web と同じ値・日本語名）。新規ジャンルの初期色は見本に無い強調色
    func testTwelveSwatchesMatchTheWeb() {
        XCTAssertEqual(GenrePalette.swatches.count, 12)
        XCTAssertEqual(
            GenrePalette.swatches.map(\.hex),
            ["#e0393e", "#f0642a", "#f59e0b", "#f2b705", "#7cb518", "#16a34a", "#0d9488", "#0ea5e9", "#1f6fe5", "#4f46e5", "#7c4dff", "#e04f9a"])
        XCTAssertEqual(GenrePalette.swatches.map(\.name), ["赤", "朱", "橙", "黄", "黄緑", "緑", "青緑", "水色", "青", "藍", "紫", "桃"])
        XCTAssertFalse(GenrePalette.swatches.map(\.hex).contains(GenreIcon.defaultColor))
        XCTAssertEqual(GenreIcon.defaultColor, "#1d5fd1")
    }

    /// AC-IOS-5-3i-2: 絵柄は 6 つ（API の列挙と同じ並び）とラベル
    func testSixIcons() {
        XCTAssertEqual(GenreIcon.allCases.map(\.rawValue), ["card", "figure", "robot", "goods", "star", "box"])
        XCTAssertEqual(GenreIcon.allCases.map(\.label), ["カード", "フィギュア", "ロボ", "グッズ", "星", "箱"])
    }
}

@MainActor
final class GenreEditTests: XCTestCase {
    private let genre = Genre(
        id: 1, name: "S.H.Figuarts", queryTemplate: "S.H.Figuarts {name}", sortOrder: 1, siteIDs: [2, 1, 3],
        color: "#e0393e", icon: .figure, matchHosts: ["tamashii.jp"], matchKeywords: ["figuarts"])

    private func makeViewModel() -> (SettingsListViewModel, FakeWishlistService) {
        let service = FakeWishlistService(items: [], genres: [genre], sites: T.sites)
        return (SettingsListViewModel(service: service, genres: [genre], sites: T.sites), service)
    }

    private func edit(
        color: String? = nil, icon: GenreIcon? = nil, hosts: String = "tamashii.jp", keywords: String = "figuarts"
    ) -> GenreEdit {
        GenreEdit(
            name: genre.name, queryTemplate: genre.queryTemplate, siteIDs: genre.siteIDs, aliasLines: [],
            color: color ?? genre.color, icon: icon ?? genre.icon, hostsText: hosts, keywordsText: keywords)
    }

    /// AC-IOS-5-3i-3: 手がかりの欄は `,`・`、`・改行で区切れる。前後の空白と空の項目は除く
    func testParseCluesSplitsOnCommaIdeographicCommaAndNewline() {
        XCTAssertEqual(SettingsListViewModel.parseClues("a.example, b.example、c.example\nd.example"), ["a.example", "b.example", "c.example", "d.example"])
        XCTAssertEqual(SettingsListViewModel.parseClues("  a , ,\n\n b ,"), ["a", "b"])
        XCTAssertEqual(SettingsListViewModel.parseClues("デュエマ、 DM"), ["デュエマ", "DM"])
        XCTAssertEqual(SettingsListViewModel.parseClues(""), [])
        XCTAssertEqual(SettingsListViewModel.parseClues("  \n "), [])
    }

    /// AC-IOS-5-3i-4: 色だけ変えたら color だけ PATCH（変えていない項目は送らない）
    func testChangingOnlyTheColorPatchesOnlyTheColor() async {
        let (viewModel, service) = makeViewModel()
        let updated = await viewModel.updateGenre(genre, edit: edit(color: "#0ea5e9"))
        XCTAssertEqual(service.calls, [.updateGenre(id: 1, patch: GenreUpdate(color: "#0ea5e9"))])
        XCTAssertEqual(updated?.color, "#0ea5e9")
        XCTAssertEqual(viewModel.genres.first?.color, "#0ea5e9")
    }

    /// AC-IOS-5-3i-5: 絵柄・手がかりを変えたらそれだけ PATCH。手がかりは解釈した結果が元と違うときだけ送る
    func testChangingIconAndCluesPatchesThoseFields() async {
        let (viewModel, service) = makeViewModel()
        let updated = await viewModel.updateGenre(
            genre, edit: edit(icon: .robot, hosts: "tamashii.jp、bandai.example", keywords: "figuarts,\nフィギュアーツ"))
        XCTAssertEqual(
            service.calls,
            [
                .updateGenre(
                    id: 1,
                    patch: GenreUpdate(icon: .robot, matchHosts: ["tamashii.jp", "bandai.example"], matchKeywords: ["figuarts", "フィギュアーツ"]))
            ])
        XCTAssertEqual(updated?.icon, .robot)
        XCTAssertEqual(updated?.matchHosts, ["tamashii.jp", "bandai.example"])
    }

    /// AC-IOS-5-3i-6: 手がかりを全部消すと `[]` を送る
    func testClearingCluesSendsEmptyLists() async {
        let (viewModel, service) = makeViewModel()
        _ = await viewModel.updateGenre(genre, edit: edit(hosts: "  ", keywords: ""))
        XCTAssertEqual(service.calls, [.updateGenre(id: 1, patch: GenreUpdate(matchHosts: [], matchKeywords: []))])
    }

    /// AC-IOS-5-3i-7: 何も変えていなければ API を呼ばず元のジャンルを返す（手がかりの書式の違いだけでは送らない）
    func testNoChangeCallsNothing() async {
        let (viewModel, service) = makeViewModel()
        let same = await viewModel.updateGenre(genre, edit: edit(hosts: " tamashii.jp ,", keywords: "figuarts"))
        XCTAssertEqual(same, genre)
        XCTAssertEqual(service.calls, [])
    }

    /// AC-IOS-5-3i-8: 追加は color・icon・match_hosts・match_keywords を常に送る（初期色は強調色）
    func testAddGenreAlwaysSendsColorIconAndClues() async {
        let (viewModel, service) = makeViewModel()
        let added = await viewModel.addGenre(GenreEdit(name: " ガンプラ ", queryTemplate: "{name}", siteIDs: [1]))
        XCTAssertEqual(
            service.calls,
            [
                .createGenre(
                    GenreCreate(
                        name: "ガンプラ", queryTemplate: "{name}", siteIDs: [1], aliases: [], color: "#1d5fd1", icon: .box, matchHosts: [],
                        matchKeywords: []))
            ])
        XCTAssertEqual(added?.color, "#1d5fd1")
        XCTAssertEqual(viewModel.genres.last?.name, "ガンプラ")
    }

    /// AC-IOS-5-3i-9: 追加で色・絵柄・手がかりを選んだらそのまま送る。名前が空白のみなら API を呼ばない
    func testAddGenreWithChoicesAndBlankName() async {
        let (viewModel, service) = makeViewModel()
        let blank = await viewModel.addGenre(GenreEdit(name: "  "))
        XCTAssertNil(blank)
        XCTAssertEqual(service.calls, [])

        let added = await viewModel.addGenre(
            GenreEdit(name: "カード", color: "#16a34a", icon: .card, hostsText: "a.example, b.example", keywordsText: "TCG"))
        XCTAssertEqual(added?.color, "#16a34a")
        XCTAssertEqual(added?.icon, .card)
        XCTAssertEqual(added?.matchHosts, ["a.example", "b.example"])
        XCTAssertEqual(added?.matchKeywords, ["TCG"])
    }

    /// AC-IOS-5-3i-10: 設定の行の件数（ジャンルごとの商品数。商品が無いジャンルは含めない）
    func testItemCounts() {
        let items = [T.item(1, genre: 1), T.item(2, genre: 1), T.item(3, genre: 2)]
        XCTAssertEqual(SettingsListViewModel.itemCounts(items), [1: 2, 2: 1])
        XCTAssertEqual(SettingsListViewModel.itemCounts([]), [:])
    }
}

/// AC-IOS-5-3i-11: キャッシュに残っている古い JSON（color・icon・summary などが無い）も読める
final class GenreCacheCompatibilityTests: XCTestCase {
    func testOldCachedGenreDecodesWithDefaults() throws {
        let json = #"{"id":1,"name":"A","queryTemplate":"{name}","sortOrder":0,"siteIDs":[1]}"#
        let genre = try JSONDecoder().decode(Genre.self, from: Data(json.utf8))
        XCTAssertEqual(genre.color, "#1d5fd1")
        XCTAssertEqual(genre.icon, .box)
        XCTAssertEqual(genre.matchHosts, [])
        XCTAssertEqual(genre.matchKeywords, [])
    }

    func testGenreRoundTripKeepsColorIconAndClues() throws {
        let genre = Genre(id: 1, name: "A", color: "#e0393e", icon: .star, matchHosts: ["h.example"], matchKeywords: ["k"])
        let decoded = try JSONDecoder().decode(Genre.self, from: JSONEncoder().encode(genre))
        XCTAssertEqual(decoded, genre)
    }

    func testItemSummaryRoundTripsThroughTheCache() throws {
        var item = T.item(1)
        item.summary = ItemSummary(low: 880, status: .ok, fetchedAt: Date(timeIntervalSince1970: 1_790_985_600))
        let decoded = try JSONDecoder().decode(Item.self, from: JSONEncoder().encode(item))
        XCTAssertEqual(decoded.summary, item.summary)
    }
}
