import XCTest

@testable import PokeCalcCore

/// `FeatureCatalog`(ADR-0507): 画面の登録の検証と起動時に開く画面の選択。
/// - 起動時に開く画面は並び順で最初に `=1` だった1つ(登録の行の順ではない)。設定エラー時は開かない
/// - ID・並び順の重複、必要なサービスの欠落、右上の入口で設定エラー時の遷移先が無いことは、起動時の設定エラー
final class FeatureCatalogTests: XCTestCase {
    private let deviceDataKey = ServiceKey((any DeviceDataService).self)
    private let speedKey = ServiceKey((any SpeedService).self)

    // MARK: 起動時に開く画面

    func testFeatureToOpenAtLaunch() {
        // 登録の行の順と並び順をわざとずらす
        let specs = [
            FeatureSpec(id: "speed", order: 600, openAtLaunchEnvironmentKey: "OPEN_SPEED"),
            FeatureSpec(id: "calc", order: 100, openAtLaunchEnvironmentKey: "OPEN_CALC"),
            FeatureSpec(id: "balance", order: 500),
            FeatureSpec(id: "team", order: 300, openAtLaunchEnvironmentKey: "OPEN_TEAM"),
        ]
        let cases: [(name: String, env: [String: String], ready: Bool, expected: String?)] = [
            ("指定なし", [:], true, nil),
            ("1つだけ", ["OPEN_TEAM": "1"], true, "team"),
            ("複数指定は並び順で最初", ["OPEN_SPEED": "1", "OPEN_TEAM": "1", "OPEN_CALC": "1"], true, "calc"),
            ("複数指定(後ろ2つ)", ["OPEN_SPEED": "1", "OPEN_TEAM": "1"], true, "team"),
            ("値が1以外は開かない", ["OPEN_CALC": "true", "OPEN_SPEED": "1"], true, "speed"),
            ("キーの無い画面は開かない", ["": "1"], true, nil),
            ("設定エラー時は開かない", ["OPEN_CALC": "1"], false, nil),
        ]
        for testCase in cases {
            XCTAssertEqual(
                FeatureCatalog.featureToOpenAtLaunch(specs, environment: testCase.env, servicesReady: testCase.ready),
                testCase.expected, testCase.name)
        }
    }

    // MARK: 登録の検証

    func testValidateAcceptsValidSpecs() throws {
        try FeatureCatalog.validate([
            FeatureSpec(id: "calc", order: 100),
            FeatureSpec(id: "about", order: 10_000, isToolbarEntry: true, availableWithoutServices: true),
        ])
    }

    func testValidateRejectsDuplicateID() {
        XCTAssertThrowsError(
            try FeatureCatalog.validate([FeatureSpec(id: "calc", order: 100), FeatureSpec(id: "calc", order: 200)])
        ) { error in
            XCTAssertEqual(error as? FeatureCatalogError, .duplicateID("calc"))
        }
    }

    func testValidateRejectsDuplicateOrder() {
        XCTAssertThrowsError(
            try FeatureCatalog.validate([
                FeatureSpec(id: "calc", order: 100), FeatureSpec(id: "judge", order: 700),
                FeatureSpec(id: "speed", order: 700),
            ])
        ) { error in
            XCTAssertEqual(error as? FeatureCatalogError, .duplicateOrder(order: 700, ids: ["judge", "speed"]))
        }
    }

    func testValidateRejectsToolbarEntryWithoutFallback() {
        XCTAssertThrowsError(
            try FeatureCatalog.validate([FeatureSpec(id: "about", order: 10_000, isToolbarEntry: true)])
        ) { error in
            XCTAssertEqual(error as? FeatureCatalogError, .toolbarEntryWithoutFallback(featureID: "about"))
        }
    }

    // MARK: 必要なサービス

    func testBuildServicesSucceedsWhenAllRequiredServicesRegistered() throws {
        let specs = [
            FeatureSpec(id: "speed", order: 600, requiredServices: [speedKey]),
            FeatureSpec(id: "calc", order: 100),
        ]
        let services = try FeatureCatalog.buildServices(for: specs) { services in
            try services.register((any SpeedService).self, MockSpeedService())
        }
        XCTAssertNotNil(services.resolve((any SpeedService).self))
    }

    func testBuildServicesRejectsMissingService() {
        let specs = [FeatureSpec(id: "about", order: 10_000, requiredServices: [deviceDataKey])]
        XCTAssertThrowsError(
            try FeatureCatalog.buildServices(for: specs) { services in
                // 登録と取り出しの型違い(具体型で登録し、存在型で要求している)も欠落として検出する
                try services.register(MockDeviceDataService.self, MockDeviceDataService())
            }
        ) { error in
            guard case .missingService(let featureID, let service) = error as? FeatureCatalogError else {
                return XCTFail("想定外のエラー: \(error)")
            }
            XCTAssertEqual(featureID, "about")
            XCTAssertTrue(service.contains("DeviceDataService"), service)
        }
    }

    func testBuildServicesValidatesSpecsBeforeRegistering() {
        var registered = false
        XCTAssertThrowsError(
            try FeatureCatalog.buildServices(
                for: [FeatureSpec(id: "calc", order: 100), FeatureSpec(id: "calc", order: 200)]
            ) { _ in registered = true }
        ) { error in
            XCTAssertEqual(error as? FeatureCatalogError, .duplicateID("calc"))
        }
        XCTAssertFalse(registered)
    }

    func testBuildServicesPropagatesRegistrationError() {
        XCTAssertThrowsError(
            try FeatureCatalog.buildServices(for: [FeatureSpec(id: "speed", order: 600)]) { services in
                try services.register((any SpeedService).self, MockSpeedService())
                try services.register((any SpeedService).self, MockSpeedService())
            }
        ) { error in
            XCTAssertTrue(error is FeatureServicesError, "\(error)")
        }
    }

    /// 設定エラーとして画面に出す文言は日本語で、原因(ID・型名)を含む。
    func testErrorDescriptionsNameTheCause() {
        XCTAssertTrue(FeatureCatalogError.duplicateID("calc").description.contains("calc"))
        XCTAssertTrue(
            FeatureCatalogError.duplicateOrder(order: 700, ids: ["judge", "speed"]).description.contains("judge, speed"))
        XCTAssertTrue(
            FeatureCatalogError.missingService(featureID: "adjust", service: "AdjustService").description
                .contains("AdjustService"))
        XCTAssertTrue(FeatureCatalogError.toolbarEntryWithoutFallback(featureID: "about").description.contains("about"))
    }
}
