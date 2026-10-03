import XCTest

@testable import PokeCalcCore

/// 防御側のランクの文言と表示(issue #274。ADR-0315 の Web と同じ語。ADR-0501「防御側のランクの受け入れ条件」)。
/// 既存の `CalcConditionsDomainTests`(攻撃側の語・A/C の表示)は変えない。
final class DefenderRankLabelsTests: XCTestCase {

    func testDefenderRankLabelsMatchWeb() {
        XCTAssertEqual(CalcConditionLabels.defenderRankTitle, "防御側のランク")
        XCTAssertEqual(CalcConditionLabels.defenderRankIncrement, "防御側のランクを上げる")
        XCTAssertEqual(CalcConditionLabels.defenderRankDecrement, "防御側のランクを下げる")
    }

    func testExistingFixedLabelsAreUnchanged() {
        XCTAssertEqual(CalcConditionLabels.rankTitle, "攻撃側のランク")
        XCTAssertEqual(CalcConditionLabels.rankIncrement, "ランクを上げる")
        XCTAssertEqual(CalcConditionLabels.rankDecrement, "ランクを下げる")
        XCTAssertEqual(AbilityPickerLabels.defenderTitle, "防御側の特性")
    }

    /// def → B、spd → D。0 は「±0」。攻撃側の A/C の表示も変わらない。
    func testRankLabelForDefenseStats() {
        let cases: [(StatKey, Int, String)] = [
            (.def, 0, "B ±0"),
            (.def, 1, "B +1"),
            (.def, 6, "B +6"),
            (.def, -1, "B -1"),
            (.def, -6, "B -6"),
            (.spd, 0, "D ±0"),
            (.spd, 2, "D +2"),
            (.spd, -2, "D -2"),
            (.spd, -6, "D -6"),
            (.atk, 1, "A +1"),
            (.spa, -1, "C -1"),
        ]
        for (stat, value, expected) in cases {
            XCTAssertEqual(RankLabel.text(stat: stat, value: value), expected, "\(stat) \(value)")
        }
    }
}
