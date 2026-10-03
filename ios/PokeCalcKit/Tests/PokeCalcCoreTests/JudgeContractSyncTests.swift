import PokeCalcJudgeAPI
import XCTest

@testable import PokeCalcCore

/// 生成の同期(P6-25。ADR-0504 §2): 契約(services/judge/api/openapi.yaml)から生成された enum と、ドメインの enum(値・順序)・
/// 文言の対応表が一致すること。契約が変わって再生成(ios-gen)したあと、ここが落ちたらドメイン・文言・写像を追従する。
/// 範囲の数値(候補数・技 ID の長さ・能力ポイント・ランク)は XCTest から契約を読めないので `make ios-check-request-limits` が見る。
final class JudgeContractSyncTests: XCTestCase {
    func testFormatMatchesContract() {
        XCTAssertEqual(Format.allCases.map(\.rawValue), Components.Schemas.Format.allCases.map(\.rawValue))
    }

    /// 契約の `ErrorCode` はすべて、文言の対応表(`JudgeLabelsTests.expectedErrorMessages`)に載せて確かめる。
    /// 新しい code が契約に増えたらここで気づく(Web の `judgeErrorText` と同じ運用)。
    func testEveryContractErrorCodeIsCoveredByTheLabelTable() {
        let covered = Set(JudgeLabelsTests.expectedErrorMessages.keys)
        for code in Components.Schemas.ErrorCode.allCases {
            XCTAssertTrue(covered.contains(code.rawValue), "契約の ErrorCode \(code.rawValue) が文言の対応表に無い")
        }
    }

    /// 契約の ErrorCode の値の集合(10 値。v0.2.0 で missing_header・invalid_header が増えた)そのものも固定する。値が増減したら、この期待値・文言・DECISIONS.md の連絡を一緒に直す。
    func testErrorCodeSetIsTheTenKnownValues() {
        XCTAssertEqual(
            Set(Components.Schemas.ErrorCode.allCases.map(\.rawValue)),
            ["invalid_request", "unknown_species", "unknown_move", "unknown_nature", "request_too_large",
             "upstream_unavailable", "internal_error", "not_found", "missing_header", "invalid_header"])
    }

    /// 未対応の印の `target`・`reason` は契約で enum にしない(ADR-0215・ADR-0708 §4)ので、生成型は文字列のまま。
    /// ドメインは知らない値を `.unknown` に写す(古いアプリが新しい値で壊れない)。
    func testUnsupportedMarkFieldsAreFreeStringsInTheContract() {
        let mark = Components.Schemas.UnsupportedMark(target: "future_target", reason: "future_reason", id: "x")
        XCTAssertEqual(UnsupportedTarget(contractValue: mark.target), .unknown)
        XCTAssertEqual(UnsupportedReason(contractValue: mark.reason), .unknown)
    }
}
