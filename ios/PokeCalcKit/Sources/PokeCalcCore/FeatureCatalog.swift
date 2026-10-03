/// 画面(機能)の登録情報のうち、View に依存しない部分(ADR-0507 §1)。アプリの `AppFeature` が
/// これを作り、検証・起動時に開く画面の選択は `FeatureCatalog` の純粋関数で行う(単体テストできるように)。
public struct FeatureSpec: Equatable, Sendable {
    /// 画面の識別子(`IOS_SCREEN` の画面名と同じ値)。
    public let id: String
    /// 入口の並び順(小さい順)。重複は設定エラー。
    public let order: Int
    /// 起動時にこの画面を開く環境変数。無ければ `nil`。
    public let openAtLaunchEnvironmentKey: String?
    /// 画面を開くのに必要なサービス(起動時にすべて登録されていることを検証する)。
    public let requiredServices: [ServiceKey]
    /// 入口が右上のアイコン(設定エラー時も出る)か。
    public let isToolbarEntry: Bool
    /// 設定エラーでサービスが無いときも開ける(`destinationWithoutServices` を実装している)か。
    public let availableWithoutServices: Bool

    public init(
        id: String, order: Int, openAtLaunchEnvironmentKey: String? = nil, requiredServices: [ServiceKey] = [],
        isToolbarEntry: Bool = false, availableWithoutServices: Bool = false
    ) {
        self.id = id
        self.order = order
        self.openAtLaunchEnvironmentKey = openAtLaunchEnvironmentKey
        self.requiredServices = requiredServices
        self.isToolbarEntry = isToolbarEntry
        self.availableWithoutServices = availableWithoutServices
    }
}

/// 登録の誤り。どれも起動時の設定エラーとして画面に出す(クラッシュしない。coding-rules §2)。
public enum FeatureCatalogError: Error, Equatable, Sendable, CustomStringConvertible {
    /// 同じ ID の画面が2つ以上ある(後の画面に到達できなくなる)。
    case duplicateID(String)
    /// 同じ並び順の画面が2つ以上ある(並びが登録の行の順に依存してしまう)。
    case duplicateOrder(order: Int, ids: [String])
    /// 画面が必要とするサービスが登録されていない(開くと空白の画面になる)。
    case missingService(featureID: String, service: String)
    /// 右上のアイコンの画面が、設定エラー時の遷移先を持たない(設定エラー時も入口が出るため)。
    case toolbarEntryWithoutFallback(featureID: String)

    public var description: String {
        switch self {
        case .duplicateID(let id):
            FeatureRegistryText.duplicateID(id)
        case .duplicateOrder(let order, let ids):
            FeatureRegistryText.duplicateOrder(order, ids: ids)
        case .missingService(let featureID, let service):
            FeatureRegistryText.missingService(featureID: featureID, service: service)
        case .toolbarEntryWithoutFallback(let featureID):
            FeatureRegistryText.toolbarEntryWithoutFallback(featureID)
        }
    }
}

/// 画面の登録の検証と、起動時に開く画面の選択(ADR-0507 §1・§2)。
public enum FeatureCatalog {
    /// 起動時に開く環境変数の「開く」値。
    public static let openAtLaunchValue = "1"

    /// 登録そのものの検証(ID・並び順の重複、右上のアイコンの画面の設定エラー時の遷移先)。
    public static func validate(_ specs: [FeatureSpec]) throws {
        var seenIDs = Set<String>()
        for spec in specs where !seenIDs.insert(spec.id).inserted {
            throw FeatureCatalogError.duplicateID(spec.id)
        }
        let byOrder = Dictionary(grouping: specs, by: \.order)
        if let (order, group) = byOrder.filter({ $0.value.count > 1 }).min(by: { $0.key < $1.key }) {
            throw FeatureCatalogError.duplicateOrder(order: order, ids: group.map(\.id))
        }
        if let spec = specs.first(where: { $0.isToolbarEntry && !$0.availableWithoutServices }) {
            throw FeatureCatalogError.toolbarEntryWithoutFallback(featureID: spec.id)
        }
    }

    /// すべての画面が必要とするサービスが `services` に登録されているか。
    public static func validateServices(_ specs: [FeatureSpec], services: FeatureServices) throws {
        for spec in specs {
            if let missing = spec.requiredServices.first(where: { !services.contains($0) }) {
                throw FeatureCatalogError.missingService(featureID: spec.id, service: missing.description)
            }
        }
    }

    /// 登録を検証し、`register` でサービスを登録して、必要なサービスがそろっているかまで検証する。
    /// 起動時の組み立て(`AppEnvironment.makeAtLaunch`)はこれを通す。
    public static func buildServices(
        for specs: [FeatureSpec], register: (inout FeatureServices) throws -> Void
    ) throws -> FeatureServices {
        try validate(specs)
        var services = FeatureServices()
        try register(&services)
        try validateServices(specs, services: services)
        return services
    }

    /// 起動時に開く画面の ID。並び順で最初に `openAtLaunchValue` が指定された1つ(従来の else-if と同じ)。
    /// サービスがそろっていない(設定エラー)ときは開かない。
    public static func featureToOpenAtLaunch(
        _ specs: [FeatureSpec], environment: [String: String], servicesReady: Bool
    ) -> String? {
        guard servicesReady else { return nil }
        return specs.sorted { $0.order < $1.order }.first { spec in
            guard let key = spec.openAtLaunchEnvironmentKey else { return false }
            return environment[key] == openAtLaunchValue
        }?.id
    }
}

/// 画面の登録まわりの文言(1か所に持つ。coding-rules §2)。
public enum FeatureRegistryText {
    /// サービスが取り出せず画面を出せないときの見出し。
    public static let unavailableTitle = "この画面を開けません"
    /// 同・説明。
    public static let unavailableDescription = "画面に必要な設定が見つかりませんでした。アプリを再起動してください。"

    static func duplicateServiceRegistration(_ typeName: String) -> String {
        "サービスが重複して登録された: \(typeName)"
    }

    static func duplicateID(_ id: String) -> String {
        "画面の ID が重複して登録された: \(id)"
    }

    static func duplicateOrder(_ order: Int, ids: [String]) -> String {
        "画面の並び順が重複している(\(order)): \(ids.joined(separator: ", "))"
    }

    static func missingService(featureID: String, service: String) -> String {
        "画面 \(featureID) に必要なサービスが登録されていない: \(service)"
    }

    static func toolbarEntryWithoutFallback(_ featureID: String) -> String {
        "右上の入口の画面 \(featureID) に設定エラー時の遷移先が無い"
    }
}
