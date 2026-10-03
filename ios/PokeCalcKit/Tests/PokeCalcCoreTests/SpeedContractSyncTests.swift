import PokeCalcSpeedAPI
import XCTest

@testable import PokeCalcCore

/// 生成の同期(P6-24。ADR-0503 §2): 契約(services/speed/api/openapi.yaml)から生成された enum と、
/// ドメインの enum(値・順序)が一致すること。契約が変わって `make ios-gen` したあと、ここが落ちたら
/// ドメインの型・文言・写像を追従する(落ちるべきところで落ちる同期の網)。
/// 範囲の数値(SP の最大・ランクの範囲)は XCTest から契約を読めないので `make ios-check-request-limits` が見る。
final class SpeedContractSyncTests: XCTestCase {
    func testPresetIDsMatchContractInTableOrder() {
        XCTAssertEqual(
            SpeedPresetID.allCases.map(\.rawValue),
            Components.Schemas.PresetId.allCases.map(\.rawValue),
            "表の行の調整の ID と順序(ADR-0601 §2)が契約と一致する")
    }

    func testMinimalPresetsMatchContract() {
        XCTAssertEqual(
            SpeedMinimalPreset.allCases.map(\.rawValue),
            Components.Schemas.MinimalPresetId.allCases.map(\.rawValue))
    }

    func testMinimalPresetsAreTheScarfFreeRowsOfThePresetTable() {
        let presetIDs = Set(SpeedPresetID.allCases.map(\.rawValue))
        for minimal in SpeedMinimalPreset.allCases {
            XCTAssertTrue(presetIDs.contains(minimal.rawValue), "\(minimal) は表の行にもある調整(同じ語で表示する)")
        }
    }

    func testNaturesMatchContract() {
        XCTAssertEqual(SpeedNature.allCases.map(\.rawValue), Components.Schemas.NatureId.allCases.map(\.rawValue))
    }

    func testModesMatchContract() {
        XCTAssertEqual(
            SpeedInputMode.allCases.map(\.rawValue),
            Components.Schemas.PositionRequest.ModePayload.allCases.map(\.rawValue))
    }

    /// 契約の `ErrorCode` はすべて、文言の対応表(`SpeedLabelsTests.expectedErrorMessages`)に載せて確かめる。
    /// 新しい code が契約に増えたらここで気づく(Web の `speedScreenText.errorByCode` と同じ運用)。
    func testEveryContractErrorCodeIsCoveredByTheLabelTable() {
        let covered = Set(SpeedLabelsTests.expectedErrorMessages.keys)
        for code in Components.Schemas.ErrorCode.allCases {
            XCTAssertTrue(covered.contains(code.rawValue), "契約の ErrorCode \(code.rawValue) が文言の対応表に無い")
        }
    }
}
