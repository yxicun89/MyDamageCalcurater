import XCTest

@testable import PokeCalcCore

/// 既定で非表示にできる画面の可視性の仕組み(F-07。判定の撤去後は既定の非表示の画面は無く、仕組みだけ残す。ADR-0715)。
final class FeatureVisibilityTests: XCTestCase {
    func testNothingIsHiddenByDefault() {
        XCTAssertTrue(FeatureVisibility.hiddenByDefaultIDs.isEmpty)
    }

    func testShowEnvironmentKeyUsesTheUppercasedID() {
        XCTAssertEqual(FeatureVisibility.showEnvironmentKey(forFeatureID: "speed"), "POKECALC_SHOW_SPEED")
    }

    func testAllFeaturesAreVisibleByDefault() {
        for id in ["calc", "reverse", "team", "adjust", "balance", "speed", "about", "favorites"] {
            XCTAssertTrue(FeatureVisibility.isVisible(featureID: id, environment: [:]), id)
        }
    }

    func testCustomHiddenSetIsRespected() {
        XCTAssertFalse(FeatureVisibility.isVisible(featureID: "speed", hiddenByDefaultIDs: ["speed"], environment: [:]))
        XCTAssertTrue(
            FeatureVisibility.isVisible(
                featureID: "speed", hiddenByDefaultIDs: ["speed"], environment: ["POKECALC_SHOW_SPEED": "1"]))
        for other in ["0", "", "true", "yes"] {
            XCTAssertFalse(
                FeatureVisibility.isVisible(
                    featureID: "speed", hiddenByDefaultIDs: ["speed"], environment: ["POKECALC_SHOW_SPEED": other]),
                "1 以外では出さない: \(other)")
        }
        XCTAssertFalse(
            FeatureVisibility.isVisible(
                featureID: "speed", hiddenByDefaultIDs: ["speed"], environment: ["POKECALC_SHOW_CALC": "1"]))
    }
}
