import Foundation
import Observation

/// 詳細シート(画像タップで下から出る)の状態。
@MainActor
@Observable
public final class ItemDetailViewModel {
    public enum Summary: Equatable, Sendable {
        case loading
        /// 目安価格が無い(フェーズ2は常にこれ)→ 文言 `WishlistText.noPriceInfo`
        case noPriceInfo
        /// 通信できない → 文言 `WishlistText.offline`
        case offline
    }

    /// 保存済みの商品(「保存」に成功したら更新される)
    public private(set) var item: Item
    public private(set) var summary: Summary = .loading
    public private(set) var isEditingQuery = false
    /// 検索ワードの編集中の値。シート内だけの一時的な値で、閉じれば破棄される(`saveQuery()` のときだけ送る)
    public var queryDraft = ""
    /// 保存の失敗(編集中の値とリンクは残す)
    public private(set) var errorMessage: String?

    private let genre: Genre?
    private let sites: [Site]
    private let service: any WishlistService

    public init(item: Item, genre: Genre?, sites: [Site], service: any WishlistService) {
        self.item = item
        self.genre = genre
        self.sites = sites
        self.service = service
    }

    /// サマリの文言(`WishlistText`)。loading 中は空文字。
    public var summaryText: String {
        switch summary {
        case .loading: ""
        case .noPriceInfo: WishlistText.noPriceInfo
        case .offline: WishlistText.offline
        }
    }

    /// 保存済みの検索ワード(サイト別の上書きを除く。検索ワード欄に出す値)
    public var displayQuery: String { SiteLinks.itemQuery(item: item, genre: genre) }

    /// サイト行。ジャンルの siteIDs 順(`SiteLinks.resolve`)。
    /// 編集中は `queryDraft`(前後の空白を除き、空なら override 無し)をこの商品の queryOverride として組み立てる(入力値で変わる。PWA と同じ)。
    /// 通信しないので、オフラインでも常に出る。
    public var links: [SiteLink] {
        var shown = item
        if isEditingQuery {
            let trimmed = queryDraft.trimmingCharacters(in: .whitespacesAndNewlines)
            shown.queryOverride = trimmed.isEmpty ? nil : trimmed
        }
        return SiteLinks.resolve(item: shown, genre: genre, sites: sites)
    }

    /// 編集を始める。`queryDraft` を `displayQuery` で初期化する。API は呼ばない。
    public func beginEditingQuery() {
        queryDraft = displayQuery
        errorMessage = nil
        isEditingQuery = true
    }

    /// 編集をやめて破棄する。API は呼ばない。
    public func cancelEditingQuery() {
        isEditingQuery = false
        queryDraft = ""
        errorMessage = nil
    }

    /// 「保存」。`queryDraft` の前後の空白を除いた値で `PATCH {query_override}` を送る(空なら `.clear` = null)。
    /// 成功したら `item` を返ってきた商品に更新して編集を終え、その商品を返す。
    /// 失敗したら `errorMessage` を立て、編集中のまま(`queryDraft` はそのまま)nil を返す。
    @discardableResult
    public func saveQuery() async -> Item? {
        let trimmed = queryDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            let updated = try await service.updateItem(
                id: item.id, patch: ItemPatch(queryOverride: trimmed.isEmpty ? .clear : .set(trimmed)))
            item = updated
            isEditingQuery = false
            errorMessage = nil
            return updated
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }

    /// `estimates` を取得してサマリを決める。`sites` が空なら `.noPriceInfo`、通信できない(`WishlistError.isNetwork`)なら `.offline`、
    /// その他の失敗も `.noPriceInfo`(オフラインとは言わない)。
    public func loadEstimates() async {
        do {
            let estimates = try await service.estimates(itemID: item.id)
            _ = estimates  // フェーズ2は価格を出さない(sites が空でも埋まっていても同じ文言)
            summary = .noPriceInfo
        } catch let error as WishlistError where error.isNetwork {
            summary = .offline
        } catch {
            summary = .noPriceInfo
        }
    }
}
