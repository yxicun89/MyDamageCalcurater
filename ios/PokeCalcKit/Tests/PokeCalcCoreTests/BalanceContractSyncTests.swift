import PokeCalcBalanceAPI
import XCTest

@testable import PokeCalcCore

/// 生成の同期(P6-26。ADR-0505 §2): 契約(services/balance/api/openapi.yaml)から生成された enum と、ドメインの enum(値・順序)・
/// 文言の対応表が一致すること。契約が変わって再生成(ios-gen)したあと、ここが落ちたらドメイン・文言・写像を追従する。
/// 範囲の数値(メンバー数・技の件数・技 ID の長さ)は XCTest から契約を読めないので `ios-check-request-limits` が見る。
final class BalanceContractSyncTests: XCTestCase {
    /// 契約の `TypeId`(18 タイプ・正準順)と `PokeType` が値も順序も同じ。iOS はタイプの一覧・相性を持たず、応答のタイプを `PokeType` に写すだけ。
    func testTypeIdMatchesPokeTypeInCanonicalOrder() {
        XCTAssertEqual(Components.Schemas.TypeId.allCases.map(\.rawValue), PokeType.allCases.map(\.rawValue))
    }

    func testDefenseCategoryMatchesContract() {
        XCTAssertEqual(
            Components.Schemas.DefenseCategory.allCases.map(\.rawValue), BalanceDefenseCategory.allCases.map(\.rawValue))
    }

    func testEffectSourceMatchesContract() {
        XCTAssertEqual(Components.Schemas.EffectSource.allCases.map(\.rawValue), BalanceEffectSource.allCases.map(\.rawValue))
    }

    func testDefenseEffectMatchesContract() {
        XCTAssertEqual(Components.Schemas.DefenseEffect.allCases.map(\.rawValue), BalanceDefenseEffect.allCases.map(\.rawValue))
    }

    func testCoverageMultiplierMatchesContract() {
        XCTAssertEqual(
            Components.Schemas.CoverageMultiplier.allCases.map(\.rawValue), BalanceCoverageMultiplier.allCases.map(\.rawValue))
    }

    /// 契約の `ErrorCode` はすべて、文言の対応表(`BalanceLabelsTests.expectedErrorMessages`)に載せて確かめる。
    /// 新しい code が契約に増えたらここで気づく(Web の `balanceErrorText` と同じ運用)。
    func testEveryContractErrorCodeIsCoveredByTheLabelTable() {
        let covered = Set(BalanceLabelsTests.expectedErrorMessages.keys)
        for code in Components.Schemas.ErrorCode.allCases {
            XCTAssertTrue(covered.contains(code.rawValue), "契約の ErrorCode \(code.rawValue) が文言の対応表に無い")
        }
    }

    /// 契約の ErrorCode の値の集合(10 値)そのものも固定する。値が増減したら、この期待値・文言・DECISIONS.md の連絡を一緒に直す。
    func testErrorCodeSetIsTheTenKnownValues() {
        XCTAssertEqual(
            Set(Components.Schemas.ErrorCode.allCases.map(\.rawValue)),
            ["missing_request_context", "invalid_request", "request_too_large", "unknown_pokemon", "unknown_move", "unknown_ability",
             "master_unavailable", "overloaded", "internal_error", "not_found"])
    }

    /// 範囲の定数は契約の値のリテラルで固定する(契約との照合は `ios-check-request-limits`。XCTest はリポジトリのファイルを読めない)。
    /// メンバー数の上限は構築の上限と同じ(1 つの構築をそのまま送れる)。技の上限も構築の上限と同じ。
    func testRequestLimitsAreTheContractValues() {
        XCTAssertEqual(RequestLimits.maxBalanceMembers, 6, "AnalyzeRequest/CoverageRequest.members.maxItems")
        XCTAssertEqual(RequestLimits.minBalanceMembers, 1, "同 minItems")
        XCTAssertEqual(RequestLimits.maxBalanceMovesPerMember, 4, "CoverageRequestMember.moveIds.maxItems")
        XCTAssertEqual(RequestLimits.maxBalanceMoveIdLength, 40, "MoveId.maxLength(judge の 64 とは別の契約)")
        XCTAssertEqual(RequestLimits.maxBalanceMembers, TeamLimits.maxMembers, "構築の上限と同じ(1 つの構築をそのまま送れる)")
        XCTAssertEqual(RequestLimits.maxBalanceMovesPerMember, TeamLimits.maxMovesPerMember)
    }
}
