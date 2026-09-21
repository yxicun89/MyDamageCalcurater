import Foundation
import XCTest

@testable import PokeCalcCore

/// `ClientIdentity`(ADR-0017 §5, CLAUDE.md 技術規約「端末ID(UUID)とセッションIDを全リクエストに付与」)。
/// 端末 ID は初回に作って UserDefaults に保存し、以後は同じ値。セッション ID は起動(インスタンス)ごとに新しい。
/// UserDefaults はテスト専用の suite に閉じ、標準の設定を汚さない。
final class ClientIdentityTests: XCTestCase {

    private var suiteName = ""
    private var defaults = UserDefaults()

    override func setUpWithError() throws {
        suiteName = "pokecalc-tests-identity-\(UUID().uuidString)"
        defaults = try XCTUnwrap(UserDefaults(suiteName: suiteName))
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suiteName)
    }

    func testDeviceIDIsCreatedOnceAndReused() {
        let first = ClientIdentity(defaults: defaults)
        let second = ClientIdentity(defaults: defaults)
        XCTAssertNotNil(UUID(uuidString: first.deviceID), "UUID 形式")
        XCTAssertEqual(first.deviceID, second.deviceID, "2回目は保存した値を使う")
        XCTAssertEqual(defaults.string(forKey: ClientIdentity.deviceIDDefaultsKey), first.deviceID,
                       "注入した UserDefaults に保存する")
    }

    func testSessionIDIsNewForEachInstance() {
        let first = ClientIdentity(defaults: defaults)
        let second = ClientIdentity(defaults: defaults)
        XCTAssertNotNil(UUID(uuidString: first.sessionID), "UUID 形式")
        XCTAssertNotNil(UUID(uuidString: second.sessionID), "UUID 形式")
        XCTAssertNotEqual(first.sessionID, second.sessionID)
        XCTAssertNotEqual(first.sessionID, first.deviceID)
    }

    /// 保存値が壊れている(UUID でない)ときは作り直して保存し直す。壊れた値をヘッダーに載せない。
    func testCorruptedStoredDeviceIDIsReplaced() {
        defaults.set("not-a-uuid", forKey: ClientIdentity.deviceIDDefaultsKey)
        let identity = ClientIdentity(defaults: defaults)
        XCTAssertNotNil(UUID(uuidString: identity.deviceID))
        XCTAssertEqual(defaults.string(forKey: ClientIdentity.deviceIDDefaultsKey), identity.deviceID)
    }

    /// 別の端末(別の保存先)では別の ID。
    func testDifferentStoresGetDifferentDeviceIDs() throws {
        let otherSuite = "pokecalc-tests-identity-other-\(UUID().uuidString)"
        let other = try XCTUnwrap(UserDefaults(suiteName: otherSuite))
        defer { other.removePersistentDomain(forName: otherSuite) }
        XCTAssertNotEqual(ClientIdentity(defaults: defaults).deviceID, ClientIdentity(defaults: other).deviceID)
    }

    /// テストや API 実装に固定値を渡すための初期化子。
    func testExplicitValuesAreKept() {
        let identity = ClientIdentity(deviceID: "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11",
                                      sessionID: "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33")
        XCTAssertEqual(identity.deviceID, "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11")
        XCTAssertEqual(identity.sessionID, "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33")
    }
}
