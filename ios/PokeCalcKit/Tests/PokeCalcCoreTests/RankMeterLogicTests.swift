import XCTest

@testable import PokeCalcCore

/// 能力ランクのメーター(13 段)の純粋ロジック。Web の `RankMeter`(ADR-0357)と段数・符号・読み上げをそろえる
/// (docs/ai-shared/decisions/2026-10-11-web-rank-meter.md)。
final class RankMeterLogicTests: XCTestCase {
    func testSegmentCountIsThirteenAndMatchesRankLimits() {
        XCTAssertEqual(RankMeterLogic.segmentCount, 13)
        XCTAssertEqual(RankMeterLogic.segmentCount, RankLimits.max - RankLimits.min + 1)
        XCTAssertEqual(RankMeterLogic.values, Array(-6...6))
    }

    func testIndexAndValueRoundTrip() {
        for value in -6...6 {
            let index = RankMeterLogic.index(forValue: value)
            XCTAssertEqual(index, value + 6)
            XCTAssertEqual(RankMeterLogic.value(forIndex: index), value)
        }
        XCTAssertEqual(RankMeterLogic.index(forValue: 0), RankMeterLogic.zeroIndex)
        XCTAssertEqual(RankMeterLogic.zeroIndex, 6)
    }

    func testIndexAndValueClampOutOfRange() {
        XCTAssertEqual(RankMeterLogic.index(forValue: 99), 12)
        XCTAssertEqual(RankMeterLogic.index(forValue: -99), 0)
        XCTAssertEqual(RankMeterLogic.value(forIndex: 40), 6)
        XCTAssertEqual(RankMeterLogic.value(forIndex: -3), -6)
    }

    func testStepClampsAtBothEnds() {
        XCTAssertEqual(RankMeterLogic.stepped(0, by: 1), 1)
        XCTAssertEqual(RankMeterLogic.stepped(0, by: -1), -1)
        XCTAssertEqual(RankMeterLogic.stepped(6, by: 1), 6)
        XCTAssertEqual(RankMeterLogic.stepped(-6, by: -1), -6)
        XCTAssertEqual(RankMeterLogic.stepped(5, by: 3), 6)
        XCTAssertEqual(RankMeterLogic.stepped(100, by: 0), 6)
    }

    func testCanStep() {
        XCTAssertTrue(RankMeterLogic.canIncrement(5))
        XCTAssertFalse(RankMeterLogic.canIncrement(6))
        XCTAssertTrue(RankMeterLogic.canDecrement(-5))
        XCTAssertFalse(RankMeterLogic.canDecrement(-6))
    }

    func testValueForDragPositionPicksNearestSegment() {
        let width = 130.0  // 1 段 10pt
        XCTAssertEqual(RankMeterLogic.value(atX: 0, width: width), -6)
        XCTAssertEqual(RankMeterLogic.value(atX: 9.9, width: width), -6)
        XCTAssertEqual(RankMeterLogic.value(atX: 10.1, width: width), -5)
        XCTAssertEqual(RankMeterLogic.value(atX: 65, width: width), 0, "中央は 0")
        XCTAssertEqual(RankMeterLogic.value(atX: 129.9, width: width), 6)
    }

    func testValueForDragPositionClampsOutsideAndIgnoresZeroWidth() {
        XCTAssertEqual(RankMeterLogic.value(atX: -50, width: 130), -6)
        XCTAssertEqual(RankMeterLogic.value(atX: 500, width: 130), 6)
        XCTAssertNil(RankMeterLogic.value(atX: 10, width: 0))
    }

    func testSignedText() {
        XCTAssertEqual(RankMeterLogic.signedText(2), "+2")
        XCTAssertEqual(RankMeterLogic.signedText(-3), "-3")
        XCTAssertEqual(RankMeterLogic.signedText(0), "±0")
        XCTAssertEqual(RankMeterLogic.signedText(9), "+6", "範囲外は丸めて表示")
    }

    func testSegmentStateUpDownZeroOff() {
        typealias S = RankMeterLogic.SegmentState
        XCTAssertEqual(RankMeterLogic.segmentState(step: 0, value: 3), S.zero)
        XCTAssertEqual(RankMeterLogic.segmentState(step: 2, value: 3), S.up)
        XCTAssertEqual(RankMeterLogic.segmentState(step: 3, value: 3), S.up)
        XCTAssertEqual(RankMeterLogic.segmentState(step: 4, value: 3), S.off)
        XCTAssertEqual(RankMeterLogic.segmentState(step: -1, value: 3), S.off, "上昇中は左側を塗らない")
        XCTAssertEqual(RankMeterLogic.segmentState(step: -2, value: -3), S.down)
        XCTAssertEqual(RankMeterLogic.segmentState(step: -3, value: -3), S.down)
        XCTAssertEqual(RankMeterLogic.segmentState(step: -4, value: -3), S.off)
        XCTAssertEqual(RankMeterLogic.segmentState(step: 1, value: -3), S.off)
        XCTAssertEqual(RankMeterLogic.segmentState(step: 1, value: 0), S.off)
    }

    func testStatNamesAreTheFiveRankStatsOfTheWeb() {
        XCTAssertEqual(RankMeterLogic.statName(for: .atk), "攻撃")
        XCTAssertEqual(RankMeterLogic.statName(for: .def), "防御")
        XCTAssertEqual(RankMeterLogic.statName(for: .spa), "特攻")
        XCTAssertEqual(RankMeterLogic.statName(for: .spd), "特防")
        XCTAssertEqual(RankMeterLogic.statName(for: .spe), "素早さ")
    }

    func testSpokenReadoutMatchesWeb() {
        XCTAssertEqual(RankMeterLogic.spokenText(statName: "攻撃", value: 2), "攻撃 +2")
        XCTAssertEqual(RankMeterLogic.spokenText(statName: "攻撃", value: 0), "攻撃 ±0")
        XCTAssertEqual(RankMeterLogic.spokenText(statName: "素早さ", value: -3), "素早さ -3")
    }

    func testButtonLabelsFollowWebWording() {
        XCTAssertEqual(RankMeterLogic.increaseLabel(fieldName: "攻撃側のランク"), "攻撃側のランクを上げる")
        XCTAssertEqual(RankMeterLogic.decreaseLabel(fieldName: "攻撃側のランク"), "攻撃側のランクを下げる")
    }

    func testIdentifiersAreStablePerField() {
        XCTAssertEqual(RankMeterLogic.identifier(field: "calcAttacker"), "rankMeter-calcAttacker")
        XCTAssertEqual(RankMeterLogic.minusIdentifier(field: "speed"), "rankMeterMinus-speed")
        XCTAssertEqual(RankMeterLogic.plusIdentifier(field: "speed"), "rankMeterPlus-speed")
    }
}
