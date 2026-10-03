import Foundation
import Observation

/// 編集ダイアログの状態。変えた項目だけ PATCH する。
@MainActor
@Observable
public final class EditItemViewModel {
    public var name: String
    public var optionText: String
    public var queryOverride: String
    public var genreID: Int
    /// 最低価格(円)。空なら未設定
    public var minPriceText: String
    public private(set) var replacementImage: ImageUpload?
    public private(set) var isBusy = false
    public private(set) var errorMessage: String?

    private let original: Item
    private let service: any WishlistService

    public init(item: Item, genres: [Genre], service: any WishlistService) {
        original = item
        self.service = service
        name = item.name
        optionText = item.optionText ?? ""
        queryOverride = item.queryOverride ?? ""
        genreID = item.genreID
        minPriceText = item.minPrice.map(String.init) ?? ""
    }

    private static func trimmed(_ s: String) -> String { s.trimmingCharacters(in: .whitespacesAndNewlines) }

    /// 空にしたら、元が値ありのときだけ `.clear`。変わっていなければ `.keep`。
    private static func textUpdate(_ new: String, original: String?) -> FieldUpdate<String> {
        let value = trimmed(new)
        let before = original.map(trimmed) ?? ""
        if value == before { return .keep }
        return value.isEmpty ? .clear : .set(value)
    }

    /// 空は nil(未設定)。0 以上の ASCII 数字だけの整数は値。それ以外は無効(nil と区別して `.invalid`)。
    private enum MinPrice { case unset, value(Int), invalid }

    private var parsedMinPrice: MinPrice {
        let text = Self.trimmed(minPriceText)
        if text.isEmpty { return .unset }
        guard text.utf8.allSatisfy({ $0 >= 0x30 && $0 <= 0x39 }), let value = Int(text) else { return .invalid }
        return .value(value)
    }

    public func setReplacementImage(_ image: ImageUpload?) { replacementImage = image }

    /// 元の商品との差分(前後の空白を除いて比べる)。
    /// 変えた項目だけ入れる。option・検索ワード上書き・最低価格を空にしたら、元が値ありのときだけ `.clear`(null)。元も空なら `.keep`。
    public var patch: ItemPatch {
        var result = ItemPatch()
        let newName = Self.trimmed(name)
        if newName != Self.trimmed(original.name) { result.name = newName }
        if genreID != original.genreID { result.genreID = genreID }
        result.optionText = Self.textUpdate(optionText, original: original.optionText)
        result.queryOverride = Self.textUpdate(queryOverride, original: original.queryOverride)
        switch parsedMinPrice {
        case .unset: if original.minPrice != nil { result.minPrice = .clear }
        case .value(let v): if v != original.minPrice { result.minPrice = .set(v) }
        case .invalid: break
        }
        return result
    }

    /// 名前が空でなく、最低価格が空か 0 以上の整数のとき true
    public var isValid: Bool {
        if Self.trimmed(name).isEmpty { return false }
        if case .invalid = parsedMinPrice { return false }
        return true
    }

    /// 「保存」。`isValid` でなければ何も送らず `errorMessage` を立てて nil。
    /// `patch` が空でなければ PATCH、`replacementImage` があれば PUT(画像の差し替え)を送る。両方あれば PATCH → PUT の順。
    /// 何も変えていなければ API を呼ばず元の商品を返す。成功したら最後に返ってきた商品を返す。
    /// 失敗したら `errorMessage` を立て、入力は残して nil。
    public func save() async -> Item? {
        guard isValid else {
            errorMessage = Self.trimmed(name).isEmpty ? "名前を入力してください" : "最低価格は 0 以上の整数で入力してください"
            return nil
        }
        isBusy = true
        defer { isBusy = false }
        do {
            var result = original
            let patch = patch
            if !patch.isEmpty { result = try await service.updateItem(id: original.id, patch: patch) }
            if let replacementImage { result = try await service.replaceItemImage(id: original.id, image: replacementImage) }
            errorMessage = nil
            return result
        } catch {
            errorMessage = errorText(error)
            return nil
        }
    }
}
