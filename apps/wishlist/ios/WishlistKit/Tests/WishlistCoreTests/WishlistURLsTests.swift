import XCTest

@testable import WishlistCore

// AC-IOS-SVC-03: 画像 URL の解決(ベース URL の末尾 / を保証して、前置 /wishlist を残す)と UI 定数。
final class WishlistURLsTests: XCTestCase {
    private let path = "images/00000000-0000-4000-8000-000000000012.png"

    func testImageURLResolvesAgainstBaseWithTrailingSlash() throws {
        let base = try XCTUnwrap(URL(string: "https://h.example/wishlist/"))
        XCTAssertEqual(WishlistURLs.imageURL(path: path, baseURL: base)?.absoluteString, "https://h.example/wishlist/\(path)")
    }

    /// 末尾 / が無くても前置 `/wishlist` が抜けない(オリジン基準で解決すると抜ける。docs/design.md W-03)
    func testImageURLKeepsPrefixWhenBaseHasNoTrailingSlash() throws {
        let base = try XCTUnwrap(URL(string: "https://h.example/wishlist"))
        XCTAssertEqual(WishlistURLs.imageURL(path: path, baseURL: base)?.absoluteString, "https://h.example/wishlist/\(path)")
    }

    func testImageURLWithRootBase() throws {
        let base = try XCTUnwrap(URL(string: "http://127.0.0.1:8080"))
        XCTAssertEqual(WishlistURLs.imageURL(path: path, baseURL: base)?.absoluteString, "http://127.0.0.1:8080/\(path)")
    }

    func testAbsoluteImageURLIsKept() throws {
        let base = try XCTUnwrap(URL(string: "https://h.example/wishlist/"))
        XCTAssertEqual(WishlistURLs.imageURL(path: "https://cdn.example/x.png", baseURL: base)?.absoluteString, "https://cdn.example/x.png")
    }

    func testNormalizeBaseURL() throws {
        XCTAssertEqual(WishlistURLs.normalizeBaseURL(try XCTUnwrap(URL(string: "https://h.example/wishlist"))).absoluteString, "https://h.example/wishlist/")
        XCTAssertEqual(WishlistURLs.normalizeBaseURL(try XCTUnwrap(URL(string: "https://h.example/wishlist/"))).absoluteString, "https://h.example/wishlist/")
    }

    // AC-IOS-UI-01/02 の数値: 3 列グリッド・長押し 0.5 秒・サマリ文言(PWA と同じ)
    func testUIConstants() {
        XCTAssertEqual(HomeLayout.gridColumnCount, 3)
        XCTAssertEqual(HomeLayout.longPressSeconds, 0.5)
        XCTAssertEqual(WishlistText.noPriceInfo, "まだ価格情報はありません")
        XCTAssertEqual(WishlistText.offline, "オフライン")
    }
}
