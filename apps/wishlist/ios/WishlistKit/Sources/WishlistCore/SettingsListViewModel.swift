import Foundation
import Observation

/// 設定画面のジャンル・サイト(接続先の設定は ConnectionSettingsViewModel)。
@MainActor
@Observable
public final class SettingsListViewModel {
    public private(set) var genres: [Genre]
    public private(set) var sites: [Site]
    public private(set) var errorMessage: String?

    private let service: any WishlistService

    public init(service: any WishlistService, genres: [Genre], sites: [Site]) {
        self.service = service
        self.genres = genres
        self.sites = sites
    }

    /// 並べ替えの「上へ」。`id` が先頭または無いときは変えない。
    public static func movingUp(_ ids: [Int], id: Int) -> [Int] {
        guard let index = ids.firstIndex(of: id), index > 0 else { return ids }
        var result = ids
        result.swapAt(index, index - 1)
        return result
    }

    /// sortOrder 昇順、同順は id 昇順
    public var sortedGenres: [Genre] {
        genres.sorted { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id }
    }

    private static let templateMessage = "検索 URL は http(s) で始まり、{q} を含めてください"

    private static func trimmed(_ s: String) -> String { s.trimmingCharacters(in: .whitespacesAndNewlines) }

    /// サイトを追加する。`Deeplink.isValidSearchTemplate` でなければ API を呼ばず `errorMessage`(`{q}` を含む文言)を立てて nil。
    /// 名前が空白のみでも API を呼ばず nil。成功したら `sites` に追加して返す(送る値は name(trim)・テンプレート・fetchType・isReference をすべて明示)。
    /// API の失敗は `errorMessage` を立てて nil。
    public func addSite(name: String, searchURLTemplate: String, fetchType: FetchType, isReference: Bool) async -> Site? {
        let name = Self.trimmed(name)
        guard !name.isEmpty else { return nil }
        guard Deeplink.isValidSearchTemplate(searchURLTemplate) else {
            errorMessage = Self.templateMessage
            return nil
        }
        do {
            let site = try await service.createSite(
                SiteCreate(name: name, searchURLTemplate: searchURLTemplate, fetchType: fetchType, isReference: isReference))
            sites.append(site)
            errorMessage = nil
            return site
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }

    /// サイトを編集する。変えた項目だけ PATCH(何も変えていなければ API を呼ばず元のサイトを返す)。
    /// 検索 URL テンプレートを変えるときも `{q}` 検証が効く。成功したら `sites` を更新して返す。
    public func updateSite(_ site: Site, name: String, searchURLTemplate: String, fetchType: FetchType, isReference: Bool) async -> Site? {
        var patch = SiteUpdate()
        let newName = Self.trimmed(name)
        if newName != site.name { patch.name = newName }
        if searchURLTemplate != site.searchURLTemplate {
            guard Deeplink.isValidSearchTemplate(searchURLTemplate) else {
                errorMessage = Self.templateMessage
                return nil
            }
            patch.searchURLTemplate = searchURLTemplate
        }
        if fetchType != site.fetchType { patch.fetchType = fetchType }
        if isReference != site.isReference { patch.isReference = isReference }
        if patch == SiteUpdate() { return site }
        do {
            let updated = try await service.updateSite(id: site.id, patch: patch)
            if let index = sites.firstIndex(where: { $0.id == updated.id }) { sites[index] = updated }
            errorMessage = nil
            return updated
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }

    /// ジャンルを追加する。名前が空白のみなら API を呼ばず nil。テンプレートが空白のみなら送らない(サーバーの既定 `{name} {option}`)。
    /// `siteIDs` は表示順のまま送る。成功したら `genres` に追加して返す。
    public func addGenre(name: String, queryTemplate: String, siteIDs: [Int]) async -> Genre? {
        await addGenre(name: name, queryTemplate: queryTemplate, siteIDs: siteIDs, aliases: nil)
    }

    private func addGenre(name: String, queryTemplate: String, siteIDs: [Int], aliases: [[String]]?) async -> Genre? {
        let name = Self.trimmed(name)
        guard !name.isEmpty else { return nil }
        let template = Self.trimmed(queryTemplate)
        do {
            let genre = try await service.createGenre(
                GenreCreate(name: name, queryTemplate: template.isEmpty ? nil : template, siteIDs: siteIDs, aliases: aliases))
            genres.append(genre)
            errorMessage = nil
            return genre
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }

    /// ジャンルを編集する。変えた項目だけ PATCH。`siteIDs` は元と(順序込みで)違うときだけ、全件置き換えとして送る。
    /// 何も変えていなければ API を呼ばず元のジャンルを返す。成功したら `genres` を更新して返す。
    public func updateGenre(_ genre: Genre, name: String, queryTemplate: String, siteIDs: [Int]) async -> Genre? {
        await updateGenre(genre, name: name, queryTemplate: queryTemplate, siteIDs: siteIDs, aliases: nil)
    }

    private func updateGenre(_ genre: Genre, name: String, queryTemplate: String, siteIDs: [Int], aliases: [[String]]?) async -> Genre? {
        var patch = GenreUpdate()
        let newName = Self.trimmed(name)
        if newName != genre.name { patch.name = newName }
        let template = Self.trimmed(queryTemplate)
        if template != genre.queryTemplate, !template.isEmpty { patch.queryTemplate = template }
        if siteIDs != genre.siteIDs { patch.siteIDs = siteIDs }
        if let aliases, aliases != genre.aliases { patch.aliases = aliases }
        if patch == GenreUpdate() { return genre }
        do {
            let updated = try await service.updateGenre(id: genre.id, patch: patch)
            if let index = genres.firstIndex(where: { $0.id == updated.id }) { genres[index] = updated }
            errorMessage = nil
            return updated
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }

    // MARK: - フェーズ4-1 表記揺れの辞書(docs/phase4-spec.md AC-IOS-ALI-02〜05)

    /// 編集画面に出す行(1 グループ = 1 行。`AliasLines.format`)。辞書が無ければ空
    public static func aliasLines(of genre: Genre) -> [String] {
        genre.aliases.map(AliasLines.format)
    }

    /// ジャンルを追加する(`addGenre(name:queryTemplate:siteIDs:)` と同じ規則に加えて辞書)。
    /// `aliasLines` を `AliasLines.parseGroups` で読み、1 語だけの行があれば API を呼ばず
    /// `errorMessage = AliasLines.invalidRowMessage(行)` にして nil。それ以外は常に `aliases`(空の行を除く。無ければ [])を送る。
    public func addGenre(name: String, queryTemplate: String, siteIDs: [Int], aliasLines: [String]) async -> Genre? {
        let parsed = AliasLines.parseGroups(aliasLines)
        if let row = parsed.invalidRow {
            errorMessage = AliasLines.invalidRowMessage(row)
            return nil
        }
        return await addGenre(name: name, queryTemplate: queryTemplate, siteIDs: siteIDs, aliases: parsed.groups)
    }

    /// ジャンルを編集する(`updateGenre(_:name:queryTemplate:siteIDs:)` と同じ規則に加えて辞書)。
    /// 1 語だけの行の扱いは addGenre と同じ。読み直したグループが元(`genre.aliases`)と違うときだけ `aliases` を送る(全部消したら [])。
    /// 何も変えていなければ API を呼ばず元のジャンルを返す。
    public func updateGenre(_ genre: Genre, name: String, queryTemplate: String, siteIDs: [Int], aliasLines: [String]) async -> Genre? {
        let parsed = AliasLines.parseGroups(aliasLines)
        if let row = parsed.invalidRow {
            errorMessage = AliasLines.invalidRowMessage(row)
            return nil
        }
        return await updateGenre(genre, name: name, queryTemplate: queryTemplate, siteIDs: siteIDs, aliases: parsed.groups)
    }
}
