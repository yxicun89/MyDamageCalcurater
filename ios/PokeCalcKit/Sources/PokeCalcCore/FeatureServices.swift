/// 画面(機能)ごとのサービスを型で引くコンテナ(ADR-0507 §2)。
///
/// 各機能が起動時に自分のサービスを `register` し、画面を作るときに `resolve` で引く。機能を足しても
/// `AppEnvironment.ready` の形は変わらない。キーは型そのもの(プロトコルなら `(any AdjustService).self`)。
public struct FeatureServices: Sendable {
    private var storage: [ObjectIdentifier: any Sendable] = [:]

    public init() {}

    /// `type` のサービスを登録する。同じ型を2回登録したら上書きせずにエラーにする(2つの機能が
    /// 同じサービスを別々に作る取り違えを黙らせない)。
    public mutating func register<Service: Sendable>(_ type: Service.Type, _ service: Service) throws {
        let key = ObjectIdentifier(type)
        guard storage[key] == nil else {
            throw FeatureServicesError.duplicateRegistration(typeName: String(describing: type))
        }
        storage[key] = service
    }

    /// `type` で登録されたサービス。登録されていなければ `nil`。
    public func resolve<Service>(_ type: Service.Type) -> Service? {
        storage[ObjectIdentifier(type)] as? Service
    }
}

/// `FeatureServices` の登録の誤り。
public enum FeatureServicesError: Error, Equatable, Sendable, CustomStringConvertible {
    /// 同じ型のサービスが2回登録された。
    case duplicateRegistration(typeName: String)

    public var description: String {
        switch self {
        case .duplicateRegistration(let typeName):
            "サービスが重複して登録された: \(typeName)"
        }
    }
}
