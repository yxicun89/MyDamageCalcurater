import Foundation

/// 接続先(ADR-0500 §5)。
public enum Backend: Equatable, Sendable {
    /// `MockPokeCalcService`(架空データ)。
    case mock
    /// `APIPokeCalcService`(このベース URL)。
    case api(URL)
}

/// 設定が壊れている(URL として不正)ときのエラー。黙ってモックに落とさず、
/// 起動時に気付けるようにする(coding-rules §2)。
public struct AppConfigurationError: Error, Equatable, Sendable {
    public let reason: String

    public init(reason: String) {
        self.reason = reason
    }
}

/// 起動時に1か所で読む設定(ADR-0500 §5)。
///
/// | 設定 | 結果 |
/// |---|---|
/// | `PokeCalcAPIBaseURL` が空・無し | モック |
/// | 有効な `http(s)://` の URL | その API |
/// | 起動時の環境変数 `POKECALC_USE_MOCK=1` | URL があってもモック(XCUITest 用) |
/// | URL が不正(スキーム無し・http(s) 以外・ホスト無し) | 起動時にエラー |
public struct AppConfiguration: Sendable {
    /// Info.plist のキー(xcconfig の `POKECALC_API_BASE_URL` から入る)。
    public static let apiBaseURLInfoKey = "PokeCalcAPIBaseURL"
    /// モックを強制する起動時環境変数(XCUITest 用)。
    public static let useMockEnvironmentKey = "POKECALC_USE_MOCK"

    private static let acceptedSchemes: Set<String> = ["http", "https"]
    private static let forceMockValue = "1"

    public let backend: Backend

    public init(infoDictionary: [String: Any], environment: [String: String]) throws {
        // モック強制は不正な URL より優先する。XCUITest はいつでも起動できる必要があるため
        // (ADR-0500 §5・AppConfigurationTests「モック強制は不正な URL より優先」)。
        if environment[Self.useMockEnvironmentKey] == Self.forceMockValue {
            self.backend = .mock
            return
        }

        guard let rawValue = infoDictionary[Self.apiBaseURLInfoKey] else {
            self.backend = .mock
            return
        }
        guard let rawString = rawValue as? String else {
            throw AppConfigurationError(reason: "\(Self.apiBaseURLInfoKey) が文字列でない: \(rawValue)")
        }
        let trimmed = rawString.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            self.backend = .mock
            return
        }
        guard let url = URL(string: trimmed),
              let scheme = url.scheme?.lowercased(), Self.acceptedSchemes.contains(scheme),
              let host = url.host, !host.isEmpty
        else {
            throw AppConfigurationError(reason: "\(Self.apiBaseURLInfoKey) が不正な URL: \(trimmed)")
        }
        self.backend = .api(url)
    }
}
