import XCTest

@testable import PokeCalcCore

/// F-11: 目標方式の文言(Web の `adjustScreenText` の goals の節と同じ語。ADR-0331 §5〜§7)。
final class AdjustGoalsTextTests: XCTestCase {
    func testFixedWordsMatchWeb() {
        XCTAssertEqual(AdjustText.goalsModeLabel, "目標から振り方を決める")
        XCTAssertEqual(AdjustGoalKind.allCases.map(AdjustText.goalKindOption), ["素早さを上回る", "この技を耐える", "この技で倒す"])
        XCTAssertEqual(AdjustText.addGoalButton, "目標を追加")
        XCTAssertEqual(AdjustText.noGoalsNotice, "目標を追加してください")
        XCTAssertEqual(AdjustText.goalLimitHint(RequestLimits.maxAdjustGoals), "目標は 6 つまでです")
        XCTAssertEqual(AdjustText.goalCardLegend(2), "目標 2")
        XCTAssertEqual(AdjustText.goalFieldName(2, "種類"), "目標 2 の種類")
        XCTAssertEqual(AdjustText.removeGoalName(3), "目標 3 を外す")
        XCTAssertEqual(AdjustText.goalsPlanHeading, "目標をすべて満たす振り方")
        XCTAssertEqual(AdjustText.goalsNearestHeading, "目標に一番近い振り方")
        XCTAssertEqual(AdjustText.goalsInfeasibleNotice, "すべての目標は満たせませんでした")
        XCTAssertEqual(AdjustText.goalOutcomesLabel, "目標ごとの結果")
        XCTAssertEqual(AdjustText.goalBoostMoveHint, "ニトロチャージのように自分の素早さが上がる技を選ぶと、上がったあとの素早さで比べます")
    }

    func testOutspeedOutcomeLines() {
        let plain = AdjustGoalSnapshot(kind: .outspeed, opponentName: "テストA(最速)")
        XCTAssertEqual(
            AdjustText.goalOutcomeLine(1, snapshot: plain, outcome: AdjustGoalOutcome(kind: .outspeed, met: true, selfSpeed: 130, opponentSpeed: 120, selfSpeedRank: 0)),
            "目標 1: テストA(最速)より先に動けます(自分 130 / 相手 120)")
        XCTAssertEqual(
            AdjustText.goalOutcomeLine(2, snapshot: plain, outcome: AdjustGoalOutcome(kind: .outspeed, met: false, selfSpeed: 100, opponentSpeed: 120, selfSpeedRank: 0)),
            "目標 2: テストA(最速)より先には動けません(自分 100 / 相手 120)")
    }

    func testOutspeedBoostPrefixByRank() {
        func line(rank: Int, move: String?) -> String {
            let snapshot = AdjustGoalSnapshot(kind: .outspeed, opponentName: "テストA(準速)", boostMoveName: move)
            return AdjustText.goalOutcomeLine(
                1, snapshot: snapshot,
                outcome: AdjustGoalOutcome(kind: .outspeed, met: true, selfSpeed: 150, opponentSpeed: 120, selfSpeedRank: rank))
        }
        XCTAssertEqual(line(rank: 1, move: "テストわざ"), "目標 1: テストわざで素早さが1段階上がったあと、テストA(準速)より先に動けます(自分 150 / 相手 120)")
        XCTAssertEqual(line(rank: -2, move: "テストわざ"), "目標 1: テストわざで素早さが2段階下がったあと、テストA(準速)より先に動けます(自分 150 / 相手 120)")
        XCTAssertEqual(line(rank: 0, move: "テストわざ"), "目標 1: テストわざでは素早さは上がりません。テストA(準速)より先に動けます(自分 150 / 相手 120)")
        XCTAssertEqual(line(rank: 0, move: nil), "目標 1: テストA(準速)より先に動けます(自分 150 / 相手 120)")
    }

    func testSurviveAndKoOutcomeLinesTruncateChance() {
        let survive = AdjustGoalSnapshot(kind: .survive, opponentName: "テストB(C特化)", moveName: "テストわざ", hits: 2)
        XCTAssertEqual(
            AdjustText.goalOutcomeLine(1, snapshot: survive, outcome: AdjustGoalOutcome(kind: .survive, met: true, chancePercent: 100)),
            "目標 1: テストB(C特化)のテストわざを2発耐えます(耐える確率 100%)")
        XCTAssertEqual(
            AdjustText.goalOutcomeLine(1, snapshot: survive, outcome: AdjustGoalOutcome(kind: .survive, met: false, chancePercent: 37.55)),
            "目標 1: テストB(C特化)のテストわざを2発は耐えられません(耐える確率 37.5%)")
        let ko = AdjustGoalSnapshot(kind: .ko, opponentName: "テストB(無振り)", moveName: "テストわざ", hits: 1)
        XCTAssertEqual(
            AdjustText.goalOutcomeLine(3, snapshot: ko, outcome: AdjustGoalOutcome(kind: .ko, met: true, chancePercent: 99.99)),
            "目標 3: テストわざでテストB(無振り)を1発で倒せます(倒す確率 99.9%)")
        XCTAssertEqual(
            AdjustText.goalOutcomeLine(3, snapshot: ko, outcome: AdjustGoalOutcome(kind: .ko, met: false, chancePercent: 0)),
            "目標 3: テストわざでテストB(無振り)を1発では倒せません(倒す確率 0%)")
    }
}
