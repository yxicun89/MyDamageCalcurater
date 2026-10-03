import Foundation
import XCTest

@testable import WishlistCore

/// Bearer トークンはベース URL の配下にだけ付ける(外部の画像 URL へ漏らさない)。
final class AuthPolicyTests: XCTestCase {
    private let base = URL(string: "https://wishlist.example/wishlist/")!

    private func allowed(_ url: String, base: URL? = nil) -> Bool {
        AuthPolicy.shouldAttachToken(url: URL(string: url)!, baseURL: base ?? self.base)
    }

    func testSameOriginUnderBasePathIsAllowed() {
        XCTAssertTrue(allowed("https://wishlist.example/wishlist/images/a.png"))
        XCTAssertTrue(allowed("https://WISHLIST.example:443/wishlist/api/items"))
    }

    func testExternalHostIsRejected() {
        XCTAssertFalse(allowed("https://cdn.other.example/wishlist/images/a.png"))
        XCTAssertFalse(allowed("https://wishlist.example.evil.example/wishlist/images/a.png"))
    }

    func testDifferentPortOrSchemeIsRejected() {
        XCTAssertFalse(allowed("https://wishlist.example:8443/wishlist/images/a.png"))
        XCTAssertFalse(allowed("http://wishlist.example/wishlist/images/a.png"))
    }

    func testPathOutsideBaseIsRejected() {
        XCTAssertFalse(allowed("https://wishlist.example/other/images/a.png"))
        XCTAssertFalse(allowed("https://wishlist.example/wishlist-evil/images/a.png"))
        XCTAssertFalse(allowed("https://wishlist.example/"))
    }

    func testBaseWithoutTrailingSlashAndRootBase() {
        XCTAssertTrue(allowed("https://wishlist.example/wishlist/images/a.png", base: URL(string: "https://wishlist.example/wishlist")!))
        XCTAssertTrue(allowed("http://127.0.0.1:8080/images/a.png", base: URL(string: "http://127.0.0.1:8080")!))
    }
}

final class ImageLoaderTests: XCTestCase {
    private final class Seen: @unchecked Sendable {
        private let lock = NSLock()
        private var items: [(String, String?)] = []
        func add(_ url: String, _ auth: String?) { lock.withLock { items.append((url, auth)) } }
        var all: [(String, String?)] { lock.withLock { items } }
    }

    func testBearerOnlyForBaseURLNotForExternalImages() async throws {
        let seen = Seen()
        StubURLProtocol.install { request in
            seen.add(request.url?.absoluteString ?? "", request.value(forHTTPHeaderField: "Authorization"))
            return (200, Data("IMG".utf8))
        }
        let base = try XCTUnwrap(URL(string: "https://wishlist.example/wishlist/"))
        let load = ImageLoader.make(baseURL: base, token: "secret", session: StubURLProtocol.makeSession())

        _ = try await load(try XCTUnwrap(URL(string: "https://wishlist.example/wishlist/images/a.png")))
        _ = try await load(try XCTUnwrap(URL(string: "https://cdn.other.example/og.png")))

        let all = seen.all
        XCTAssertEqual(all.count, 2)
        XCTAssertEqual(all[0].1, "Bearer secret")
        XCTAssertNil(all[1].1, "外部ホストへ Authorization を送らない")
    }
}
