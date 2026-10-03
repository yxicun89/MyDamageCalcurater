/// サービスの型を表すキー(ADR-0507 §2)。`FeatureServices` の登録・取り出しと、各機能が必要とする
/// サービスの宣言(`FeatureSpec.requiredServices`)で同じキーを使う。
public struct ServiceKey: Hashable, Sendable, CustomStringConvertible {
    private let identifier: ObjectIdentifier
    /// エラー文言に出す型名。
    public let description: String

    /// `type` のキー(プロトコルなら `ServiceKey((any AdjustService).self)`)。
    public init<Service>(_ type: Service.Type) {
        self.identifier = ObjectIdentifier(type)
        self.description = String(describing: type)
    }

    public static func == (lhs: ServiceKey, rhs: ServiceKey) -> Bool {
        lhs.identifier == rhs.identifier
    }

    public func hash(into hasher: inout Hasher) {
        hasher.combine(identifier)
    }
}

/// 画面(機能)ごとのサービスを型で引くコンテナ(ADR-0507 §2)。
///
/// 各機能が起動時に自分のサービスを `register` し、画面を作るときに `resolve` で引く。機能を足しても
/// `AppEnvironment.ready` の形は変わらない。キーは型そのもの(プロトコルなら `(any AdjustService).self`)。
public struct FeatureServices: Sendable {
    private var storage: [ServiceKey: any Sendable] = [:]

    public init() {}

    /// `type` のサービスを登録する。同じ型を2回登録したら上書きせずにエラーにする(2つの機能が
    /// 同じサービスを別々に作る取り違えを黙らせない)。
    public mutating func register<Service: Sendable>(_ type: Service.Type, _ service: Service) throws {
        let key = ServiceKey(type)
        guard storage[key] == nil else {
            throw FeatureServicesError.duplicateRegistration(typeName: key.description)
        }
        storage[key] = service
    }

    /// `type` で登録されたサービス。登録されていなければ `nil`。
    public func resolve<Service>(_ type: Service.Type) -> Service? {
        storage[ServiceKey(type)] as? Service
    }

    /// `key` の型のサービスが登録されているか。
    public func contains(_ key: ServiceKey) -> Bool {
        storage[key] != nil
    }
}

/// `FeatureServices` の登録の誤り。
public enum FeatureServicesError: Error, Equatable, Sendable, CustomStringConvertible {
    /// 同じ型のサービスが2回登録された。
    case duplicateRegistration(typeName: String)

    public var description: String {
        switch self {
        case .duplicateRegistration(let typeName):
            FeatureRegistryText.duplicateServiceRegistration(typeName)
        }
    }
}
