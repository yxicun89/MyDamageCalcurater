import Foundation
import Observation

/// 共有されたテキストから http(s) の URL を取り出す(Safari は URL、他のアプリはテキストで渡してくる)。
public enum SharedURL {
    /// 最初に見つかった http(s) の URL。無ければ nil。前後の空白・改行・日本語の句読点や括弧は URL に含めない。
    public static func extract(from text: String) -> URL? {
        let pattern = #"(?i)https?://[^\s、。，．！？（）()「」『』【】〈〉<>"]+"#
        guard let range = text.range(of: pattern, options: .regularExpression) else { return nil }
        var candidate = String(text[range])
        while let last = candidate.last, ".,;:!?".contains(last) { candidate.removeLast() }
        guard let url = URL(string: candidate), Deeplink.isHTTPURL(url.absoluteString) else { return nil }
        return url
    }
}

/// Share Extension の状態(共有シートから登録)。本体とデータは共有しない(仕様 §9.5)ので、拡張自身の設定(WishlistSettings)で API を呼ぶ。
@MainActor
@Observable
public final class ShareViewModel {
    public enum Phase: Equatable, Sendable {
        /// URL かトークンが未設定。設定画面へ誘導する(API は呼ばない)
        case needsSettings
        case loading
        /// 下書きのプレビューを出して、ジャンルを選んで保存できる
        case ready
        case saving
        case saved(Item)
        case failed(String)
    }

    public private(set) var phase: Phase = .loading
    public private(set) var draft: ItemDraft?
    public private(set) var genres: [Genre] = []
    /// 名前(下書きの名前で初期化。編集できる)
    public var name = ""
    public var genreID: Int?

    /// - Parameter sharedURL: 共有された URL(`SharedURL.extract` の結果。nil は共有に URL が無かった)
    private let sharedURL: URL?
    private let settings: WishlistSettings
    private let service: any WishlistService

    public init(sharedURL: URL?, settings: WishlistSettings, service: any WishlistService) {
        self.sharedURL = sharedURL
        self.settings = settings
        self.service = service
    }

    /// プレビューに出す画像の URL(下書きの imageURL が http(s) のときだけ)
    public var previewImageURL: URL? {
        guard let text = draft?.imageURL, Deeplink.isHTTPURL(text) else { return nil }
        return URL(string: text)
    }

    /// 名前が空白のみでなく、ジャンルが選ばれ、下書きに画像があるとき true
    public var canSave: Bool {
        !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && genreID != nil && draft?.imageURL != nil
    }

    /// 1. 設定が `isConfigured` でなければ `.needsSettings`(API を呼ばない)。
    /// 2. 共有に URL が無い・http(s) でなければ `.failed`(API を呼ばない)。
    /// 3. `listGenres()` と `draftFromURL(url, genreID: nil)` を呼ぶ。成功したら `draft`・`genres` を持ち、`name` を下書きの名前に、
    ///    `genreID` を下書きの genreID(`genres` にあれば)か sortOrder 昇順(同順は id 昇順)の先頭ジャンルにして `.ready`。どちらかが失敗したら `.failed(メッセージ)`。
    public func load() async {
        guard settings.isConfigured else {
            phase = .needsSettings
            return
        }
        guard let sharedURL, Deeplink.isHTTPURL(sharedURL.absoluteString) else {
            phase = .failed("共有された内容に http(s) の URL がありません")
            return
        }
        phase = .loading
        do {
            let loadedGenres = try await service.listGenres()
            let loadedDraft = try await service.draftFromURL(sharedURL.absoluteString, genreID: nil)
            genres = loadedGenres
            draft = loadedDraft
            name = loadedDraft.name
            if let id = loadedDraft.genreID, loadedGenres.contains(where: { $0.id == id }) {
                genreID = id
            } else {
                genreID = loadedGenres.min { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id }?.id
            }
            phase = .ready
        } catch {
            phase = .failed(errorText(error))
        }
    }

    /// `canSave` のときだけ、JSON で登録(`createItem(_:imageURL:)`。fields は genreID・trim した name・下書きの sourceURL)。
    /// 成功したら `.saved(商品)`。失敗したら `.failed(メッセージ)`(入力は残す。再度 `save()` できる)。
    public func save() async {
        guard canSave, let genreID, let draft, let imageURL = draft.imageURL else { return }
        phase = .saving
        let fields = ItemFields(
            genreID: genreID, name: name.trimmingCharacters(in: .whitespacesAndNewlines), sourceURL: draft.sourceURL)
        do {
            phase = .saved(try await service.createItem(fields, imageURL: imageURL))
        } catch {
            phase = .failed(errorText(error))
        }
    }
}
