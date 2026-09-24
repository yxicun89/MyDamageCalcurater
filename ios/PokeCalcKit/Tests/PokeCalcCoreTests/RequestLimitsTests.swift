import XCTest

@testable import PokeCalcCore

/// 要求の件数上限の定数(issue #110。ADR-0501「issue #110 の受け入れ条件(iOS 側)」1章 A1・2章)。
///
/// 値の正は `api/openapi.yaml`(`ReverseRequest.observations` / `ReverseRequest.itemCandidates` /
/// `BulkCalcRequest.itemVariants` の `maxItems`)。iOS のテストはシミュレータでも走るため
/// リポジトリの YAML を読みに行かず、契約値を**リテラルで**固定する(ADR 2章)。
/// 契約と写しのずれは `ios/scripts/check-request-limits.sh`(`make ios-test` の一部)が検出する。
final class RequestLimitsTests: XCTestCase {

    /// A1: 契約の `maxItems` と同じ値を持つ。
    func testLimitsMatchTheOpenAPIContract() {
        XCTAssertEqual(RequestLimits.maxObservations, 16, "ReverseRequest.observations.maxItems")
        XCTAssertEqual(RequestLimits.maxItemCandidates, 64, "ReverseRequest.itemCandidates.maxItems")
        XCTAssertEqual(RequestLimits.maxItemVariants, 64, "BulkCalcRequest.itemVariants.maxItems")
    }

    /// ADR 3章: 送る配列の先頭に入る null(持ち物なし)も `uniqueItems` の1件なので、
    /// トグルで選べる ID の数は上限より1つ少ない。
    func testSelectableCountsLeaveRoomForTheNoItemEntry() {
        XCTAssertEqual(RequestLimits.maxSelectableItemCandidates, RequestLimits.maxItemCandidates - 1)
        XCTAssertEqual(RequestLimits.maxSelectableItemVariants, RequestLimits.maxItemVariants - 1)
    }

    /// 文言は「上限に達した理由」を伝えるためのものなので、件数を含む(言い回しは実装者が決めてよい)。
    func testLabelsExplainTheLimitWithItsNumber() {
        let cases: [(label: String, number: Int, name: String)] = [
            (RequestLimitLabels.observationsReachedLimit, RequestLimits.maxObservations, "観測"),
            (RequestLimitLabels.itemCandidatesReachedLimit, RequestLimits.maxItemCandidates, "持ち物候補"),
            (RequestLimitLabels.itemVariantsReachedLimit, RequestLimits.maxItemVariants, "持ち物の比較"),
        ]
        for testCase in cases {
            XCTAssertFalse(testCase.label.isEmpty, "\(testCase.name): 文言が空")
            XCTAssertTrue(
                testCase.label.contains("\(testCase.number)"),
                "\(testCase.name): 件数 \(testCase.number) が文言に無い(\(testCase.label))"
            )
        }
    }
}
