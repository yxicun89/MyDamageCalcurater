/// ルート画面から開く画面の一覧(ADR-0507 §1・§3)。
enum FeatureRegistry {
    /// 登録されている画面。**画面を足すときは末尾に1行足すだけ**(1行1要素・末尾カンマ)。
    /// 並びは各機能の `order` で決まるので、ここの行の順は意味を持たない。
    private static let registered: [any AppFeature] = [
        CalcFeature(),
        ReverseFeature(),
        TeamListFeature(),
        AdjustFeature(),
        BalanceFeature(),
        SpeedFeature(),
        AboutFeature(),
    ]

    /// `order` 順の画面(入口の並び・起動時に開く画面の優先順)。
    static let features: [any AppFeature] = registered.sorted { $0.order < $1.order }

    /// 起動時に開く環境変数の「開く」値。
    static let openAtLaunchValue = "1"

    static func feature(id: String) -> (any AppFeature)? {
        features.first { $0.id == id }
    }

    /// 機能 ID の重複を起動時の設定エラーにする(後から登録した画面に到達できなくなるのを黙らせない)。
    static func validateUniqueIDs(_ features: [any AppFeature]) throws {
        var seen = Set<String>()
        for feature in features where !seen.insert(feature.id).inserted {
            throw FeatureRegistryError.duplicateID(feature.id)
        }
    }
}

enum FeatureRegistryError: Error, CustomStringConvertible {
    case duplicateID(String)

    var description: String {
        switch self {
        case .duplicateID(let id):
            "画面の ID が重複して登録された: \(id)"
        }
    }
}
