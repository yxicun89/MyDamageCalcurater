import XCTest

@testable import PokeCalcCore

/// 既定で非表示の画面(判定。F-07)の可視性。
final class FeatureVisibilityTests: XCTestCase {
    func testJudgeIsHiddenByDefault() {
        XCTAssertTrue(FeatureVisibility.hiddenByDefaultIDs.contains("judge"))
        XCTAssertFalse(FeatureVisibility.isVisible(featureID: "judge", environment: [:]))
    }

    func testJudgeIsShownOnlyWhenTheEnvironmentIsOne() {
        XCTAssertEqual(FeatureVisibility.showEnvironmentKey(forFeatureID: "judge"), "POKECALC_SHOW_JUDGE")
        XCTAssertTrue(FeatureVisibility.isVisible(featureID: "judge", environment: ["POKECALC_SHOW_JUDGE": "1"]))
        for other in ["0", "", "true", "yes"] {
            XCTAssertFalse(
                FeatureVisibility.isVisible(featureID: "judge", environment: ["POKECALC_SHOW_JUDGE": other]),
                "1 以外では出さない: \(other)")
        }
    }

    func testOtherFeaturesAreAlwaysVisible() {
        for id in ["calc", "reverse", "team", "adjust", "balance", "speed", "about", "favorites"] {
            XCTAssertTrue(FeatureVisibility.isVisible(featureID: id, environment: [:]), id)
        }
    }

    func testTheShowKeyOfAnotherFeatureDoesNotShowJudge() {
        XCTAssertFalse(FeatureVisibility.isVisible(featureID: "judge", environment: ["POKECALC_SHOW_SPEED": "1"]))
    }

    func testCustomHiddenSetIsRespected() {
        XCTAssertFalse(FeatureVisibility.isVisible(featureID: "speed", hiddenByDefaultIDs: ["speed"], environment: [:]))
        XCTAssertTrue(
            FeatureVisibility.isVisible(
                featureID: "speed", hiddenByDefaultIDs: ["speed"], environment: ["POKECALC_SHOW_SPEED": "1"]))
    }
}
