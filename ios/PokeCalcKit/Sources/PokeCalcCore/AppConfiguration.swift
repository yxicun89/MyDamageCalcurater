import Foundation
#if canImport(Network)
import Network
#endif

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

/// 起動時に1か所で読む設定(ADR-0500 §5・ADR-0501「issue #250」)。
///
/// | 設定 | 結果 |
/// |---|---|
/// | `PokeCalcAPIBaseURL` が空・無し | モック |
/// | 有効な `https://` の URL(ホストは任意) | その API |
/// | 有効な `http://` の URL で、ホストが非修飾ドメイン(ドット無し。例 `localhost`)または `.local`
///   ドメイン(ATS の `NSAllowsLocalNetworking`。`ios/PokeCalc-Info.plist` 参照) | その API |
/// | 有効な `http://` の URL で、上記に当てはまらないホスト(IP アドレス・通常の公開ドメインなど) | 起動時にエラー |
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
        // https はホストを問わず受理する。http は ATS の NSAllowsLocalNetworking
        // (ios/PokeCalc-Info.plist)が実際に通す範囲だけ受理する(ADR-0501「issue #250」§1・§3、
        // 2026-09-25 の見直しで IP アドレスを対象から外した)。それ以外の http は実行時に ATS が
        // 拒否する(はず)ため、ここでもエラーにする(受理条件 == 実行時の挙動)。
        if scheme == "http", !Self.isAllowedByNSAllowsLocalNetworking(host: host) {
            throw AppConfigurationError(
                reason: "\(Self.apiBaseURLInfoKey) は http でホストが ATS(NSAllowsLocalNetworking)の対象外: \(trimmed)"
            )
        }
        self.backend = .api(url)
    }

    /// ATS の `NSAllowsLocalNetworking` が実際に通す範囲かどうか(ADR-0501「issue #250」§2、
    /// 2026-09-25 の見直し後)。
    /// - 非修飾ホスト名(ドットを含まない。トップレベルの末尾ドットを含む場合はドット有りとして扱う
    ///   〈例 `localhost.` は非修飾扱いにしない〉。「ドットが無い」という Apple の記述をそのまま読んだ結果で、
    ///   FQDN のルートドット記法を特別扱いする根拠が文書に無いため)
    /// - `.local` ドメイン(大文字小文字を区別しない)
    ///
    /// IP アドレス(IPv4/IPv6)は対象に**含めない**。Apple のドキュメントは「iOS 17+/iPadOS 17+/macOS 14+
    /// では ATS は既定で IP アドレスへの接続を許可しない。個々の IP アドレスや CIDR 範囲は
    /// `NSExceptionDomains` に追加する」と明記しており、`NSAllowsLocalNetworking` が IP アドレスを
    /// 無条件に通すという記述ではない(旧版の本 ADR にあった「(または NSExceptionDomains)」という読みは
    /// 裏付けが無かったため撤回。詳しくは ADR-0501「issue #250」§1・§2)。
    private static func isAllowedByNSAllowsLocalNetworking(host: String) -> Bool {
        // IPv6 アドレス(例 `::1`)はドットを含まないため、先に IP アドレス判定を行い、
        // 非修飾ホスト名として誤って受理しないようにする(IP は受理しない)。
        if isIPAddress(host) {
            return false
        }
        if host.lowercased().hasSuffix(".local") {
            return true
        }
        if !host.contains(".") {
            return true
        }
        return false
    }

    /// `host` が IPv4 または IPv6 アドレスとして解釈できるかどうか。文字列の形(ドット・コロンの数)ではなく
    /// `inet_pton` 相当(Network フレームワークの `IPv4Address`/`IPv6Address`)で判定する
    /// (ADR-0501「issue #250」§5: 簡易正規表現は不完全な IP や短縮 IPv6 を取りこぼす)。
    private static func isIPAddress(_ host: String) -> Bool {
        IPv4Address(host) != nil || IPv6Address(host) != nil
    }
}
