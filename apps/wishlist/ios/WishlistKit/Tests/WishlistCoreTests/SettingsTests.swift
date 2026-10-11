import Foundation
import XCTest

@testable import WishlistCore

// AC-IOS-5-3b: 接続先(ユーザー決定 2026-10-11 / docs/phase5-spec.md 5-3「決めたこと」)。
// 旧 AC-IOS-SET-01/02/03(UserDefaults の接続設定・トークン・ConnectionSettingsViewModel)は、設定画面の接続の欄ごと無くしたので置き換えた。
// 接続先はビルド設定 WISHLIST_API_BASE_URL(git に入れない xcconfig)→ Info.plist の WishlistAPIBaseURL → WishlistSettings.fromInfo。
final class WishlistSettingsTests: XCTestCase {
    func testBaseURLAcceptsOnlyHTTPAndHTTPS() {
        let cases: [(String?, String?)] = [
            ("https://host.example/wishlist/", "https://host.example/wishlist/"),
            ("http://127.0.0.1:8080/wishlist", "http://127.0.0.1:8080/wishlist"),
            ("ftp://host.example/", nil),
            ("javascript:alert(1)", nil),
            ("host.example/wishlist", nil),
            ("", nil),
            (nil, nil),
        ]
        for (text, want) in cases {
            XCTAssertEqual(WishlistSettings(apiBaseURL: text).baseURL?.absoluteString, want, text ?? "nil")
        }
    }

    /// AC-IOS-5-3b-1: 使える URL があれば設定済み。トークンは見ない
    func testIsConfiguredNeedsOnlyAUsableURL() {
        XCTAssertTrue(WishlistSettings(apiBaseURL: "https://h.example/").isConfigured)
        XCTAssertFalse(WishlistSettings(apiBaseURL: nil).isConfigured)
        XCTAssertFalse(WishlistSettings(apiBaseURL: "ftp://h.example/").isConfigured)
    }

    /// AC-IOS-5-3b-2: Info.plist(ビルド設定を通した値)から読む。キーは WishlistAPIBaseURL
    func testFromInfoReadsTheBundleValue() {
        XCTAssertEqual(WishlistSettings.infoKey, "WishlistAPIBaseURL")
        let settings = WishlistSettings.fromInfo([WishlistSettings.infoKey: " https://wishlist.example/wishlist/ "])
        XCTAssertEqual(settings.apiBaseURL, "https://wishlist.example/wishlist/")
        XCTAssertTrue(settings.isConfigured)
    }

    /// AC-IOS-5-3b-3: 空・無い・展開されていない `$(...)`・http(s) でない → 未設定(キャッシュだけで動く)
    func testFromInfoTreatsEmptyOrUnexpandedValuesAsNotConfigured() {
        let values: [Any?] = [nil, "", "   ", "$(WISHLIST_API_BASE_URL)", "ftp://h.example/", "host.example", 5]
        for value in values {
            var info: [String: Any] = [:]
            if let value { info[WishlistSettings.infoKey] = value }
            let settings = WishlistSettings.fromInfo(info)
            XCTAssertNil(settings.apiBaseURL, "\(String(describing: value))")
            XCTAssertFalse(settings.isConfigured)
        }
        XCTAssertFalse(WishlistSettings.fromInfo(nil).isConfigured)
    }
}
