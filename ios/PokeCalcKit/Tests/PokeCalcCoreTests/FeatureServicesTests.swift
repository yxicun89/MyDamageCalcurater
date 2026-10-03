import XCTest

@testable import PokeCalcCore

/// `FeatureServices`(ADR-0507 §2): 画面ごとのサービスを型で引く。
/// - 登録した型で引ける(プロトコルの存在型をキーにできる)
/// - 登録していない型は `nil`
/// - 同じ型の2回目の登録は上書きせずエラー
final class FeatureServicesTests: XCTestCase {

    func testResolveReturnsRegisteredServiceByProtocolType() throws {
        var services = FeatureServices()
        let deviceData = MockDeviceDataService()
        try services.register((any DeviceDataService).self, deviceData)

        let resolved = try XCTUnwrap(services.resolve((any DeviceDataService).self))
        XCTAssertTrue(resolved as AnyObject === deviceData)
    }

    func testResolveReturnsNilForUnregisteredType() throws {
        var services = FeatureServices()
        try services.register((any SpeedService).self, MockSpeedService())

        XCTAssertNil(services.resolve((any BalanceService).self))
        // 具体型と存在型は別のキー(登録した型でだけ引ける)
        XCTAssertNil(services.resolve(MockSpeedService.self))
    }

    func testDuplicateRegistrationThrowsAndKeepsFirst() throws {
        var services = FeatureServices()
        let first = MockDeviceDataService()
        try services.register((any DeviceDataService).self, first)

        XCTAssertThrowsError(try services.register((any DeviceDataService).self, MockDeviceDataService())) { error in
            guard case .duplicateRegistration(let typeName) = error as? FeatureServicesError else {
                return XCTFail("想定外のエラー: \(error)")
            }
            XCTAssertTrue(typeName.contains("DeviceDataService"), typeName)
        }
        let resolved = try XCTUnwrap(services.resolve((any DeviceDataService).self))
        XCTAssertTrue(resolved as AnyObject === first)
    }
}
