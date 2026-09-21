import Foundation
import XCTest

@testable import PokeCalcCore

/// `AppConfiguration`(ADR-0017 §5): 接続先は Info.plist の `PokeCalcAPIBaseURL`(xcconfig の
/// `POKECALC_API_BASE_URL` から入る)と、起動時の環境変数 `POKECALC_USE_MOCK` から1か所で決める。
/// - 空・無し → モック
/// - 有効な http(s) の URL → API
/// - `POKECALC_USE_MOCK=1` → URL があってもモック(XCUITest 用)
/// - 不正な URL → エラー(黙ってモックに落とさない。設定ミスを起動時に気付けるように。coding-rules §2)
final class AppConfigurationTests: XCTestCase {

    /// キー名は ADR-0017 §5 の名前そのもの。
    func testKeyNamesMatchADR() {
        XCTAssertEqual(AppConfiguration.apiBaseURLInfoKey, "PokeCalcAPIBaseURL")
        XCTAssertEqual(AppConfiguration.useMockEnvironmentKey, "POKECALC_USE_MOCK")
    }

    func testBackendSelection() throws {
        let key = "PokeCalcAPIBaseURL"
        let cases: [(name: String, info: [String: Any], env: [String: String], expected: String?)] = [
            // expected: "mock" / URL 文字列(api)
            ("キー無し", [:], [:], "mock"),
            ("空文字", [key: ""], [:], "mock"),
            ("空白だけ", [key: "  "], [:], "mock"),
            ("http", [key: "http://localhost:8080"], [:], "http://localhost:8080"),
            ("https とパス", [key: "https://pokecalc.example.invalid/base"], [:],
             "https://pokecalc.example.invalid/base"),
            ("前後の空白は除く", [key: " https://pokecalc.example.invalid "], [:], "https://pokecalc.example.invalid"),
            ("モック強制", [key: "https://pokecalc.example.invalid"], ["POKECALC_USE_MOCK": "1"], "mock"),
            ("モック強制は URL 無しでもモック", [:], ["POKECALC_USE_MOCK": "1"], "mock"),
            ("1 以外は強制しない", [key: "https://pokecalc.example.invalid"], ["POKECALC_USE_MOCK": "0"],
             "https://pokecalc.example.invalid"),
            // XCUITest はモック強制で起動する。接続先の設定が壊れていても UI テストは起動できる
            ("モック強制は不正な URL より優先", [key: "not a url"], ["POKECALC_USE_MOCK": "1"], "mock"),
        ]
        for testCase in cases {
            let config = try AppConfiguration(infoDictionary: testCase.info, environment: testCase.env)
            switch (config.backend, testCase.expected) {
            case (.mock, "mock"):
                break
            case (.api(let url), let expected?) where expected != "mock":
                XCTAssertEqual(url.absoluteString, expected, testCase.name)
            default:
                XCTFail("\(testCase.name): \(config.backend) (期待 \(testCase.expected ?? "nil"))")
            }
        }
    }

    func testInvalidBaseURLIsAnError() {
        let key = "PokeCalcAPIBaseURL"
        let invalid: [(name: String, value: Any)] = [
            ("スキーム無し", "pokecalc.example.invalid/api"),
            ("ホスト:ポートだけ(スキームと誤読される)", "localhost:8080"),
            ("http(s) 以外", "ftp://pokecalc.example.invalid"),
            ("ホスト無し", "http://"),
            ("URL でない", "not a url"),
            ("文字列でない", 8080),
        ]
        for testCase in invalid {
            XCTAssertThrowsError(
                try AppConfiguration(infoDictionary: [key: testCase.value], environment: [:]), testCase.name
            ) { error in
                XCTAssertTrue(error is AppConfigurationError, "\(testCase.name): \(type(of: error))")
            }
        }
    }
}
