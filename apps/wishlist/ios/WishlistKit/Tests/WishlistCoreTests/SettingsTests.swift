import Foundation
import XCTest

@testable import WishlistCore

// AC-IOS-SET-01/02/03: 接続設定(API のベース URL と トークン)。保存先は UserDefaults(Keychain は将来)。
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
            XCTAssertEqual(WishlistSettings(apiBaseURL: text, token: "t").baseURL?.absoluteString, want, text ?? "nil")
        }
    }

    func testIsConfiguredNeedsUsableURLAndNonBlankToken() {
        XCTAssertTrue(WishlistSettings(apiBaseURL: "https://h.example/", token: "t").isConfigured)
        XCTAssertFalse(WishlistSettings(apiBaseURL: "https://h.example/", token: "").isConfigured)
        XCTAssertFalse(WishlistSettings(apiBaseURL: "https://h.example/", token: "  ").isConfigured)
        XCTAssertFalse(WishlistSettings(apiBaseURL: nil, token: "t").isConfigured)
        XCTAssertFalse(WishlistSettings(apiBaseURL: "ftp://h.example/", token: "t").isConfigured)
    }
}

final class UserDefaultsSettingsStoreTests: XCTestCase {
    private var suiteName = ""
    private var defaults: UserDefaults!

    override func setUp() {
        suiteName = "wishlist-tests-\(UUID().uuidString)"
        defaults = UserDefaults(suiteName: suiteName)
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suiteName)
    }

    func testLoadWithNothingStoredGivesDefaults() {
        XCTAssertEqual(UserDefaultsSettingsStore(defaults: defaults).load(), WishlistSettings(apiBaseURL: nil, token: ""))
    }

    func testSaveThenLoadRoundTrips() {
        let store = UserDefaultsSettingsStore(defaults: defaults)
        let settings = WishlistSettings(apiBaseURL: "https://host.example/wishlist/", token: "secret")
        XCTAssertTrue(store.save(settings))
        XCTAssertEqual(UserDefaultsSettingsStore(defaults: defaults).load(), settings, "別のインスタンスでも読める")
    }

    /// PWA の localStorage と同じ形の JSON(`{"apiBaseUrl": string|null, "token": string}`)を、`wishlist.settings` に Data で置く。
    func testStoredShapeMatchesPWAContract() throws {
        let store = UserDefaultsSettingsStore(defaults: defaults)
        XCTAssertEqual(UserDefaultsSettingsStore.key, "wishlist.settings")
        XCTAssertTrue(store.save(WishlistSettings(apiBaseURL: "https://h.example/wishlist/", token: "tok")))
        let data = try XCTUnwrap(defaults.data(forKey: "wishlist.settings"))
        XCTAssertEqual(jsonObject(data), ["apiBaseUrl": "https://h.example/wishlist/", "token": "tok"] as NSDictionary)

        XCTAssertTrue(store.save(WishlistSettings(apiBaseURL: nil, token: "tok")))
        let data2 = try XCTUnwrap(defaults.data(forKey: "wishlist.settings"))
        XCTAssertEqual(jsonObject(data2), ["apiBaseUrl": NSNull(), "token": "tok"] as NSDictionary)
    }

    func testBrokenOrWrongShapeFallsBackToDefaults() {
        let store = UserDefaultsSettingsStore(defaults: defaults)
        for raw in ["not json", "[]", #"{"apiBaseUrl":1,"token":"t"}"#, #"{"apiBaseUrl":null,"token":5}"#, #"{"apiBaseUrl":null}"#] {
            defaults.set(Data(raw.utf8), forKey: "wishlist.settings")
            XCTAssertEqual(store.load(), WishlistSettings(), raw)
        }
    }

    /// トークンを送る先なので、http(s) 以外の URL は使わない(トークンは残す)。
    func testNonHTTPURLIsDroppedButTokenIsKept() {
        defaults.set(Data(#"{"apiBaseUrl":"ftp://evil.example/","token":"tok"}"#.utf8), forKey: "wishlist.settings")
        XCTAssertEqual(UserDefaultsSettingsStore(defaults: defaults).load(), WishlistSettings(apiBaseURL: nil, token: "tok"))
    }
}

@MainActor
final class ConnectionSettingsViewModelTests: XCTestCase {
    func testInitializesFromStore() {
        let store = InMemorySettingsStore(WishlistSettings(apiBaseURL: "https://h.example/wishlist/", token: "tok"))
        let viewModel = ConnectionSettingsViewModel(store: store)
        XCTAssertEqual(viewModel.apiBaseURLText, "https://h.example/wishlist/")
        XCTAssertEqual(viewModel.token, "tok")
        XCTAssertNil(viewModel.errorMessage)
    }

    func testInitializesEmptyWhenNothingStored() {
        let viewModel = ConnectionSettingsViewModel(store: InMemorySettingsStore())
        XCTAssertEqual(viewModel.apiBaseURLText, "")
        XCTAssertEqual(viewModel.token, "")
    }

    func testSaveTrimsAndPersists() {
        let store = InMemorySettingsStore()
        let viewModel = ConnectionSettingsViewModel(store: store)
        viewModel.apiBaseURLText = "  https://h.example/wishlist/ \n"
        viewModel.token = "  tok  "
        XCTAssertTrue(viewModel.save())
        XCTAssertEqual(store.stored, WishlistSettings(apiBaseURL: "https://h.example/wishlist/", token: "tok"))
        XCTAssertNil(viewModel.errorMessage)
    }

    func testSaveRejectsNonHTTPURLWithoutSaving() {
        let store = InMemorySettingsStore()
        let viewModel = ConnectionSettingsViewModel(store: store)
        viewModel.apiBaseURLText = "ftp://h.example/"
        viewModel.token = "tok"
        XCTAssertFalse(viewModel.save())
        XCTAssertNotNil(viewModel.errorMessage)
        XCTAssertEqual(store.saves, 0)
    }

    func testEmptyURLIsSavedAsUnset() {
        let store = InMemorySettingsStore(WishlistSettings(apiBaseURL: "https://old.example/", token: "old"))
        let viewModel = ConnectionSettingsViewModel(store: store)
        viewModel.apiBaseURLText = "   "
        viewModel.token = "tok"
        XCTAssertTrue(viewModel.save())
        XCTAssertEqual(store.stored, WishlistSettings(apiBaseURL: nil, token: "tok"))
    }

    func testSaveFailureOfStoreIsReported() {
        let store = InMemorySettingsStore(canSave: false)
        let viewModel = ConnectionSettingsViewModel(store: store)
        viewModel.apiBaseURLText = "https://h.example/"
        viewModel.token = "tok"
        XCTAssertFalse(viewModel.save())
        XCTAssertNotNil(viewModel.errorMessage)
    }
}
