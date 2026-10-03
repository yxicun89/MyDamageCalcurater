import Foundation
import Observation

/// 登録ダイアログ(写真 or URL)の状態。
@MainActor
@Observable
public final class RegisterViewModel {
    public var name = ""
    public var urlText = ""
    /// 選択中のジャンル ID
    public var genreID: Int?
    public private(set) var draft: ItemDraft?
    public private(set) var photo: ImageUpload?
    public private(set) var isBusy = false
    public private(set) var errorMessage: String?

    /// - Parameter initialGenreID: ホームのチップで選んでいるジャンル(あれば初期値)。無い・`genres` に無いときは、sortOrder 昇順(同順は id 昇順)の先頭のジャンル。
    private let service: any WishlistService
    private let genres: [Genre]

    public init(service: any WishlistService, genres: [Genre], initialGenreID: Int?) {
        self.service = service
        self.genres = genres
        if let initialGenreID, genres.contains(where: { $0.id == initialGenreID }) {
            genreID = initialGenreID
        } else {
            genreID = genres.min { $0.sortOrder != $1.sortOrder ? $0.sortOrder < $1.sortOrder : $0.id < $1.id }?.id
        }
    }

    private var trimmedName: String { name.trimmingCharacters(in: .whitespacesAndNewlines) }

    /// 名前(空白のみは不可)・ジャンル・画像(写真か、下書きの画像 URL)が揃っているとき true
    public var canRegister: Bool {
        !trimmedName.isEmpty && genreID != nil && (photo != nil || draft?.imageURL != nil)
    }

    public func setPhoto(_ photo: ImageUpload) { self.photo = photo }
    public func clearPhoto() { photo = nil }

    /// 「取得」。`urlText`(前後の空白を除く)が空なら何もしない。`draftFromURL(url, genreID: 選択中)` を呼び、
    /// 成功したら `draft` を持ち、`name` を下書きの名前にし、下書きの genreID が `genres` にあれば `genreID` にする。
    /// 失敗したら `errorMessage` を立てる(`urlText`・`name` はそのまま)。
    public func fetchDraft() async {
        let url = urlText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !url.isEmpty else { return }
        isBusy = true
        defer { isBusy = false }
        do {
            let fetched = try await service.draftFromURL(url, genreID: genreID)
            draft = fetched
            name = fetched.name
            if let id = fetched.genreID, genres.contains(where: { $0.id == id }) { genreID = id }
            errorMessage = nil
        } catch {
            errorMessage = errorText(error)
        }
    }

    /// 「登録」。`canRegister` でなければ何もせず nil。
    /// 写真があれば multipart(`createItem(_:image:)`。下書きがあれば `sourceURL` も付ける)、無ければ下書きの `imageURL` で JSON(`createItem(_:imageURL:)`。
    /// fields は `genreID`・trim した `name`・下書きの `sourceURL`)。写真と下書きの両方があれば写真を使う。
    /// 成功したら登録した商品を返す。失敗したら `errorMessage` を立て、入力は残して nil。
    public func register() async -> Item? {
        guard canRegister, let genreID else { return nil }
        isBusy = true
        defer { isBusy = false }
        let fields = ItemFields(genreID: genreID, name: trimmedName, sourceURL: draft?.sourceURL)
        do {
            let item: Item
            if let photo {
                item = try await service.createItem(fields, image: photo)
            } else if let imageURL = draft?.imageURL {
                item = try await service.createItem(fields, imageURL: imageURL)
            } else {
                return nil
            }
            errorMessage = nil
            return item
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }
}
