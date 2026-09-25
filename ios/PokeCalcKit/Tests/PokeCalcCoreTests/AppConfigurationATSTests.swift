import Foundation
import XCTest

@testable import PokeCalcCore

/// issue #250 / ADR-0501「issue #250」: `AppConfiguration` が受理する URL は、
/// ATS(App Transport Security)が実行時に実際に通す URL と一致していなければならない
/// (受理したのに実行時に通信できない・拒否すべきなのに受理する、をどちらも無くす)。
///
/// 採用した判断(A/B の間。2026-09-25 の見直し後): `https` は任意のホストで受理する。`http` は
/// `NSAllowsLocalNetworking`(`ios/PokeCalc-Info.plist` の `NSAppTransportSecurity`)が
/// ATS の例外として実際に通す範囲だけを受理する。Apple のドキュメント
/// (`nsapptransportsecurity/nsallowslocalnetworking` の Discussion)を保守的に読んだ範囲は次の2つ:
///   - 非修飾ドメイン(unqualified domain。ホスト名にドットが無い。例 `localhost`・ルーター名)
///   - `.local` ドメイン(Bonjour)
/// **IP アドレス(IPv4/IPv6)はこの範囲に含めない**。Apple のドキュメントは「iOS 17+/iPadOS 17+/macOS 14+
/// では ATS は既定で IP アドレスへの接続を許可しない。個々の IP アドレスや CIDR 範囲は
/// `NSExceptionDomains` に追加する」と明記しており、`NSAllowsLocalNetworking` が IP アドレスを
/// 無条件に通すという記述ではないため(メインセッションのシミュレータ実験は ATS 自体が有効化されているか
/// 確認できず、IP アドレスを受理してよい根拠にならなかった。ADR-0501「issue #250」§1・§2)。
/// 上記以外(ドットを含み `.local` でも無いホスト名 = 通常の公開ドメイン。IP アドレスを含む)は `http` では
/// 受理せず `AppConfigurationError` にする(このホストは実行時に ATS が拒否する〈はず〉のため)。
///
/// このファイルはこのタスク(issue #250)の受け入れ条件そのもの(ADR-0501「issue #250」§4・§5 で
/// 変更してよいと明記されたテストファイル)。`AppConfigurationTests.swift`(既存・別ファイル)は変更しない。
final class AppConfigurationATSTests: XCTestCase {

    private let key = AppConfiguration.apiBaseURLInfoKey

    private func makeConfig(_ urlString: String) throws -> AppConfiguration {
        try AppConfiguration(infoDictionary: [key: urlString], environment: [:])
    }

    /// `http` で拒否され、`reason` が「http」であることと ATS(NSAllowsLocalNetworking)が理由であることを
    /// 検査する共通アサーション(ADR-0501「issue #250」§3)。
    private func assertRejectedForATS(_ urlString: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertThrowsError(try makeConfig(urlString), file: file, line: line) { error in
            guard let configError = error as? AppConfigurationError else {
                return XCTFail("AppConfigurationError であるべき: \(type(of: error))", file: file, line: line)
            }
            XCTAssertTrue(
                configError.reason.contains("http"), "reason に http が含まれる: \(configError.reason)",
                file: file, line: line
            )
            XCTAssertTrue(
                configError.reason.contains("ATS") || configError.reason.contains("NSAllowsLocalNetworking"),
                "reason に ATS/NSAllowsLocalNetworking への言及が含まれる: \(configError.reason)",
                file: file, line: line
            )
        }
    }

    // MARK: - http: NSAllowsLocalNetworking が通す範囲は受理する

    func testHTTPLocalhostIsAccepted() throws {
        let config = try makeConfig("http://localhost:8080")
        guard case .api(let url) = config.backend else {
            return XCTFail("localhost の http はモックではなく API になるべき: \(config.backend)")
        }
        XCTAssertEqual(url.absoluteString, "http://localhost:8080")
    }

    func testHTTPDotLocalHostIsAccepted() throws {
        // Apple のドキュメントの範囲(.local ドメイン)。大文字小文字は区別しない。
        let config = try makeConfig("http://pokecalc-dev.local:8080")
        guard case .api = config.backend else {
            return XCTFail(".local ホストの http は API になるべき: \(config.backend)")
        }
    }

    func testHTTPDotLocalHostIsAcceptedCaseInsensitive() throws {
        let config = try makeConfig("http://PokeCalc-Dev.LOCAL:8080")
        guard case .api = config.backend else {
            return XCTFail(".local ホスト(大文字)の http は API になるべき: \(config.backend)")
        }
    }

    func testHTTPUnqualifiedHostnameIsAccepted() throws {
        // ドットを含まないホスト名(非修飾ドメイン)。例: 開発機のホスト名やルーター名。
        let config = try makeConfig("http://pokecalc-router:8080")
        guard case .api = config.backend else {
            return XCTFail("非修飾ホスト名の http は API になるべき: \(config.backend)")
        }
    }

    // MARK: - http: IP アドレスは拒否する(2026-09-25 の見直し。ADR-0501「issue #250」§1・§2)

    func testHTTPLoopbackIPv4IsRejected() {
        // ループバックであっても、Apple のドキュメントの「iOS 17+ は IP アドレスへの接続を既定で許可しない」
        // という記述どおり、NSAllowsLocalNetworking だけでは通らない(NSExceptionDomains が必要)。
        assertRejectedForATS("http://127.0.0.1:8080")
    }

    func testHTTPLoopbackIPv6IsRejected() {
        assertRejectedForATS("http://[::1]:8080")
    }

    func testHTTPArbitraryIPAddressIsRejected() {
        assertRejectedForATS("http://0.0.0.0:8080")
    }

    // MARK: - http: それ以外の(通常の公開・末尾ドット等の)ホストは拒否する

    func testHTTPPublicHostIsRejected() {
        assertRejectedForATS("http://example.com")
    }

    func testHTTPPublicHostWithPathIsRejected() {
        XCTAssertThrowsError(try makeConfig("http://pokecalc.example.invalid/base"))
    }

    func testHTTPSubdomainOfDotLocalLikeButNotLocalIsRejected() {
        // ".local" で終わるのではなく、途中に local を含むだけの通常ドメインは対象外。
        XCTAssertThrowsError(try makeConfig("http://local.example.com"))
    }

    func testHTTPTrailingDotHostIsRejected() {
        // `localhost.`(FQDN のルートドット記法)はドットを含むため「非修飾ホスト名」としては扱わない
        // (Apple の「ホスト名にドットが無い」という記述をそのまま読んだ結果。ルートドットを特別扱いする
        // 根拠が文書に無いため、保守的に拒否する。AppConfiguration.swift のコメント参照)。
        assertRejectedForATS("http://localhost.:8080")
    }

    // MARK: - https: ホストを問わず受理する(既存の回帰確認)

    func testHTTPSAnyHostIsAccepted() throws {
        for urlString in [
            "https://example.com",
            "https://pokecalc.example.invalid/base",
            "https://127.0.0.1:8443",
            "https://localhost:8443",
        ] {
            let config = try makeConfig(urlString)
            guard case .api = config.backend else {
                XCTFail("\(urlString): https は常に API になるべき: \(config.backend)")
                continue
            }
        }
    }
}
