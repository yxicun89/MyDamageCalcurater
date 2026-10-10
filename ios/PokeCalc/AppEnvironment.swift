import Foundation
import PokeCalcCore

/// 起動時に1回だけ作る、画面が使う実行時の状態(ADR-0500 §5・ADR-0507 §2)。
///
/// `AppConfiguration` を1か所(ここ)で読み、モック/API のどちらを使うかを決める。画面ごとのサービスは
/// 各 `AppFeature` が `FeatureServices` に登録する(機能を足しても `.ready` の形は変わらない)。
/// 設定が壊れているときは画面にエラーを出す(クラッシュしない。coding-rules §2)。
enum AppEnvironment {
    case ready(core: CoreServices, features: FeatureServices)
    case configurationError(String)

    /// 設定エラー時に画面へ出す文言の接頭辞。
    private static let configurationErrorPrefix = "設定エラー: "

    static func makeAtLaunch(
        infoDictionary: [String: Any] = Bundle.main.infoDictionary ?? [:],
        environment: [String: String] = ProcessInfo.processInfo.environment,
        features: [any AppFeature] = FeatureRegistry.features
    ) -> AppEnvironment {
        do {
            let configuration = try AppConfiguration(infoDictionary: infoDictionary, environment: environment)
            let core: CoreServices
            let backend: FeatureBackend
            switch configuration.backend {
            case .mock:
                core = CoreServices(
                    pokeCalc: try MockPokeCalcService(),
                    frequentOpponents: MockFrequentOpponentsService(environment: environment),
                    backendDescription: "モックデータで動作中",
                    images: MockImageCatalog(environment: environment))
                backend = .mock(environment: environment)
            case .api(let url):
                let identity = ClientIdentity(defaults: .standard)
                let service = APIPokeCalcService(baseURL: url, identity: identity)
                core = CoreServices(
                    pokeCalc: service, frequentOpponents: service,
                    backendDescription: "サーバーに接続中(\(url.host ?? url.absoluteString))",
                    images: RemoteImageCatalog(baseURL: url, fetcher: URLSessionImageManifestFetcher()))
                backend = .api(baseURL: url, identity: identity, pokeCalc: service)
            }
            // 登録の検証(ID・並び順の重複など)→ 各機能のサービス登録 → 必要なサービスがそろっているかの検証。
            let services = try FeatureCatalog.buildServices(for: features.map(\.spec)) { services in
                for feature in features {
                    try feature.registerServices(for: backend, into: &services)
                }
            }
            return .ready(core: core, features: services)
        } catch {
            return .configurationError("\(configurationErrorPrefix)\(error)")
        }
    }
}

/// 複数の画面が同じインスタンスを共有するサービス(ADR-0507 §2)。1つの画面だけが使うものは
/// `FeatureServices` に置く。
struct CoreServices {
    /// マスタ参照・計算・逆算(計算・逆算・構築・調整・タイプバランスが共有)。
    let pokeCalc: any PokeCalcService
    /// よく使う相手(計算・逆算が同じインスタンスを共有する)。
    let frequentOpponents: any FrequentOpponentsService
    /// 状態バッジに出す接続先の説明。
    let backendDescription: String
    /// ポケモン画像の問い合わせ先(画面ではないので core。ADR-0508 §4)。既定は画像なし。
    var images: any ImageCatalog = NoImageCatalog()
}

/// 各機能がサービスを作るときの材料(ADR-0507 §2)。
enum FeatureBackend {
    /// 架空データ。`environment` はモックのシナリオ切り替え(XCUITest 用)に使う。
    case mock(environment: [String: String])
    /// API。`identity` は全機能で1つ(セッション ID を起動ごとに1つに保つ。ADR-0500 §5)。
    /// `pokeCalc` は同じ gateway・同じ契約の機能(調整・端末データ削除など)がそのまま使う。
    case api(baseURL: URL, identity: ClientIdentity, pokeCalc: APIPokeCalcService)
}
