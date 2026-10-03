import XCTest

@testable import WishlistCore

// AC-IOS-ALI-02〜05: 設定画面のジャンル編集での表記揺れの辞書(PWA の AC-SET-08〜12 と同じ。docs/phase4-spec.md 4-1 iOS)。
@MainActor
final class SettingsAliasesTests: XCTestCase {
    private var figuartsWithAliases: Genre {
        var genre = T.figuarts
        genre.aliases = [["S.H.Figuarts", "SHフィギュアーツ"], ["真骨彫", "真骨彫製法"]]
        return genre
    }

    private func make(_ genres: [Genre]? = nil) -> (SettingsListViewModel, FakeWishlistService) {
        let genres = genres ?? [figuartsWithAliases, T.duema]
        let service = FakeWishlistService(items: [], genres: genres, sites: T.sites)
        return (SettingsListViewModel(service: service, genres: genres, sites: T.sites), service)
    }

    /// AC-IOS-ALI-02: 既存のグループを 1 行ずつ `, ` 区切りで出す。辞書が無ければ空
    func testAliasLinesOfGenre() {
        XCTAssertEqual(SettingsListViewModel.aliasLines(of: figuartsWithAliases), ["S.H.Figuarts, SHフィギュアーツ", "真骨彫, 真骨彫製法"])
        XCTAssertEqual(SettingsListViewModel.aliasLines(of: T.duema), [])
    }

    /// AC-IOS-ALI-03: 行の編集・削除で、aliases だけを全件置き換えで送る。全部消すと []。変えなければ送らない
    func testUpdateSendsAliasesOnlyWhenChanged() async {
        let (viewModel, service) = make()
        let genre = figuartsWithAliases
        let same = await viewModel.updateGenre(
            genre, name: genre.name, queryTemplate: genre.queryTemplate, siteIDs: genre.siteIDs,
            aliasLines: ["S.H.Figuarts，SHフィギュアーツ", " 真骨彫 、真骨彫製法 ", ""])
        XCTAssertEqual(same, genre, "区切り・空白・空の行が違っても、読み直したグループが同じなら送らない")
        XCTAssertEqual(service.calls, [])

        let updated = await viewModel.updateGenre(
            genre, name: genre.name, queryTemplate: genre.queryTemplate, siteIDs: genre.siteIDs,
            aliasLines: ["S.H.Figuarts, SHフィギュアーツ, フィギュアーツ"])
        XCTAssertEqual(
            service.calls, [.updateGenre(id: 1, patch: GenreUpdate(aliases: [["S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"]]))])
        XCTAssertEqual(updated?.aliases, [["S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"]])
        XCTAssertEqual(viewModel.genres.first { $0.id == 1 }?.aliases, updated?.aliases)

        let cleared = await viewModel.updateGenre(
            updated ?? genre, name: genre.name, queryTemplate: genre.queryTemplate, siteIDs: genre.siteIDs, aliasLines: ["", "  "])
        XCTAssertEqual(service.calls.last, .updateGenre(id: 1, patch: GenreUpdate(aliases: [])))
        XCTAssertEqual(cleared?.aliases, [])
    }

    /// AC-IOS-ALI-03: 名前と辞書を同時に変えたら両方を 1 回の PATCH で送る
    func testUpdateCombinesNameAndAliases() async {
        let (viewModel, service) = make()
        _ = await viewModel.updateGenre(
            T.duema, name: "デュエル・マスターズ", queryTemplate: T.duema.queryTemplate, siteIDs: T.duema.siteIDs,
            aliasLines: ["デュエマ, デュエル・マスターズ"])
        XCTAssertEqual(
            service.calls, [.updateGenre(id: 2, patch: GenreUpdate(name: "デュエル・マスターズ", aliases: [["デュエマ", "デュエル・マスターズ"]]))])
    }

    /// AC-IOS-ALI-04: 1 語だけの行があると API を呼ばず、行番号つきの文言を出す(追加・編集とも)
    func testSingleWordLineIsRejectedWithoutCallingTheAPI() async {
        let (viewModel, service) = make()
        let updated = await viewModel.updateGenre(
            figuartsWithAliases, name: "S.H.Figuarts", queryTemplate: T.figuarts.queryTemplate, siteIDs: T.figuarts.siteIDs,
            aliasLines: ["S.H.Figuarts, SHフィギュアーツ", "真骨彫"])
        XCTAssertNil(updated)
        XCTAssertEqual(viewModel.errorMessage, "別名グループ2は2語以上をカンマで区切って入力してください")
        let added = await viewModel.addGenre(name: "ガンプラ", queryTemplate: "{name}", siteIDs: [], aliasLines: ["HG"])
        XCTAssertNil(added)
        XCTAssertEqual(viewModel.errorMessage, "別名グループ1は2語以上をカンマで区切って入力してください")
        XCTAssertEqual(service.calls, [])
    }

    /// AC-IOS-ALI-05: 追加は常に aliases を送る(空の行は除く。無ければ [])
    func testAddAlwaysSendsAliases() async {
        let (viewModel, service) = make()
        let genre = await viewModel.addGenre(name: "ガンプラ", queryTemplate: "{name}", siteIDs: [2], aliasLines: ["HG, ハイグレード", ""])
        XCTAssertEqual(
            service.calls, [.createGenre(GenreCreate(name: "ガンプラ", queryTemplate: "{name}", siteIDs: [2], aliases: [["HG", "ハイグレード"]]))])
        XCTAssertEqual(genre?.aliases, [["HG", "ハイグレード"]])
        XCTAssertEqual(viewModel.genres.last, genre)

        _ = await viewModel.addGenre(name: "ポケモン", queryTemplate: "", siteIDs: [], aliasLines: [])
        XCTAssertEqual(service.calls.last, .createGenre(GenreCreate(name: "ポケモン", siteIDs: [], aliases: [])))
    }

    /// AC-IOS-ALI-05: API の 422(正規化後の重複など)はそのまま errorMessage に出す
    func testServerRejectionIsShown() async {
        let (viewModel, service) = make()
        service.failOnce(WishlistError(code: .unprocessable, status: 422, message: "aliases の語が重複しています"))
        let updated = await viewModel.updateGenre(
            T.duema, name: T.duema.name, queryTemplate: T.duema.queryTemplate, siteIDs: T.duema.siteIDs, aliasLines: ["HG, ｈｇ"])
        XCTAssertNil(updated)
        XCTAssertNotNil(viewModel.errorMessage)
    }
}
