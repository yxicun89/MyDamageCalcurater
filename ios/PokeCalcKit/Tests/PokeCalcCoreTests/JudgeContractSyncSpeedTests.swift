import PokeCalcJudgeAPI
import XCTest

@testable import PokeCalcCore

/// 契約 v0.3.0 の素早さの欄・状態異常の同期(ADR-0512)。契約から生成された enum の値と、ドメイン・文言の対応表が一致すること。
/// 契約に値が増えて再生成(ios-gen)したら、ここが落ちる = 文言を足す合図。**ただし古いアプリは未知の値でも decode が落ちない**
/// (`APIJudgeServiceUnknownSpeedValuesTests`)。ここは「新しい値の文言を足し忘れない」ための検査で、落ちないことの検査ではない。
final class JudgeContractSyncSpeedTests: XCTestCase {
    func testSpeedFactorValuesAreTheSixKnownValues() {
        XCTAssertEqual(
            Components.Schemas.SpeedFactor.allCases.map(\.rawValue),
            ["rank", "tailwind", "ability", "choiceScarf", "item", "paralysis"])
    }

    func testSpeedIgnoredInputValuesAreTheThreeKnownValues() {
        XCTAssertEqual(Components.Schemas.SpeedIgnoredInput.allCases.map(\.rawValue), ["abilityId", "itemId", "fieldWeather"])
    }

    func testEverySpeedFactorHasALabel() {
        for value in Components.Schemas.SpeedFactor.allCases {
            XCTAssertNotNil(JudgeLabelsSpeedNotesTests.expectedFactorNames[value.rawValue], "契約の SpeedFactor \(value.rawValue) の文言が無い")
            XCTAssertNotEqual(JudgeLabels.speedFactorName(value.rawValue), JudgeLabels.speedFactorUnknown, value.rawValue)
        }
        XCTAssertEqual(
            Set(JudgeLabelsSpeedNotesTests.expectedFactorNames.keys), Set(Components.Schemas.SpeedFactor.allCases.map(\.rawValue)),
            "対応表に契約に無い値が残っていない")
    }

    func testEverySpeedIgnoredInputHasALabel() {
        for value in Components.Schemas.SpeedIgnoredInput.allCases {
            XCTAssertNotNil(JudgeLabelsSpeedNotesTests.expectedIgnoredNames[value.rawValue], "契約の SpeedIgnoredInput \(value.rawValue) の文言が無い")
            XCTAssertNotEqual(JudgeLabels.speedIgnoredName(value.rawValue), JudgeLabels.speedIgnoredUnknown, value.rawValue)
        }
        XCTAssertEqual(
            Set(JudgeLabelsSpeedNotesTests.expectedIgnoredNames.keys), Set(Components.Schemas.SpeedIgnoredInput.allCases.map(\.rawValue)))
    }

    /// 要求の status: 自分・候補とも契約の enum(none/burn/paralysis/poison/badly_poison/sleep/freeze)と、ドメインの値・順が一致する。
    func testStatusMatchesTheContractForBothAttackerAndDefender() {
        let domain = JudgeStatus.allCases.map(\.rawValue)
        XCTAssertEqual(domain, Components.Schemas.Individual.StatusPayload.allCases.map(\.rawValue))
        XCTAssertEqual(domain, Components.Schemas.DefenderCandidate.StatusPayload.allCases.map(\.rawValue))
    }

    func testEveryStatusHasALabel() {
        for status in JudgeStatus.allCases {
            XCTAssertFalse(JudgeLabels.statusName(status).isEmpty, status.rawValue)
        }
    }
}
