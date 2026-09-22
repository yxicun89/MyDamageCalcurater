import Foundation
import PokeCalcCore

/// 起動時に1回だけ作る、画面が使う実行時の状態(ADR-0500 §5)。
///
/// `AppConfiguration` を1か所(ここ)で読み、モック/API のどちらの `PokeCalcService` を使うかを
/// 決める。設定が壊れているときは画面にエラーを出す(クラッシュしない。coding-rules §2)。
enum AppEnvironment {
    case ready(service: any PokeCalcService, backendDescription: String)
    case configurationError(String)

    /// 設定エラー時に画面へ出す文言の接頭辞。
    private static let configurationErrorPrefix = "設定エラー: "

    static func makeAtLaunch(
        infoDictionary: [String: Any] = Bundle.main.infoDictionary ?? [:],
        environment: [String: String] = ProcessInfo.processInfo.environment
    ) -> AppEnvironment {
        do {
            let configuration = try AppConfiguration(infoDictionary: infoDictionary, environment: environment)
            switch configuration.backend {
            case .mock:
                let service = try MockPokeCalcService()
                return .ready(service: service, backendDescription: "モックデータで動作中")
            case .api(let url):
                let identity = ClientIdentity(defaults: .standard)
                let service = APIPokeCalcService(baseURL: url, identity: identity)
                return .ready(service: service, backendDescription: "APIに接続中(\(url.host ?? url.absoluteString))")
            }
        } catch {
            return .configurationError("\(configurationErrorPrefix)\(error)")
        }
    }
}
