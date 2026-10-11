import Foundation
import XCTest

@testable import WishlistCore

// AC-IOS-5-3a: トークンの廃止(ユーザー決定 2026-10-11 / docs/phase5-spec.md 5-3)。
// 旧 AuthPolicyTests(ベース URL の配下にだけ Bearer を付ける境界)は、Bearer 自体を無くしたので置き換えた。
// API クライアントの「Authorization を付けない」は APIWishlist*Tests の各テスト(`XCTAssertNil(...headerFields[.authorization])`)で見ている。
final class ImageLoaderNoAuthTests: XCTestCase {
    private final class Seen: @unchecked Sendable {
        private let lock = NSLock()
        private var items: [(String, String?)] = []
        func add(_ url: String, _ auth: String?) { lock.withLock { items.append((url, auth)) } }
        var all: [(String, String?)] { lock.withLock { items } }
    }

    /// AC-IOS-5-3a-1: 画像の取得にはどの URL にも Authorization を付けない(自分のサーバーの画像にも)
    func testImageLoaderSendsNoAuthorizationToAnyHost() async throws {
        let seen = Seen()
        StubURLProtocol.install { request in
            seen.add(request.url?.absoluteString ?? "", request.value(forHTTPHeaderField: "Authorization"))
            return (200, Data("IMG".utf8))
        }
        let load = ImageLoader.make(session: StubURLProtocol.makeSession())

        _ = try await load(try XCTUnwrap(URL(string: "https://wishlist.example/wishlist/images/a.png")))
        _ = try await load(try XCTUnwrap(URL(string: "https://cdn.other.example/og.png")))

        let all = seen.all
        XCTAssertEqual(all.count, 2)
        for (url, auth) in all { XCTAssertNil(auth, "Authorization を送らない: \(url)") }
    }

    /// AC-IOS-5-3a-2: 設定には「トークン」の項目が無い(保存も送信もしない)
    func testSettingsHasNoTokenField() {
        let labels = Mirror(reflecting: WishlistSettings(apiBaseURL: "https://h.example/")).children.compactMap(\.label)
        XCTAssertEqual(labels, ["apiBaseURL"])
    }
}
