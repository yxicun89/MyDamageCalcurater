import XCTest

@testable import PokeCalcCore

/// 判定画面の素早さの反映/無視・状態異常の文言(ADR-0512)。語は Web の `judgeScreenText`(web/src/i18n/judge.ts)と同じ。
final class JudgeLabelsSpeedNotesTests: XCTestCase {
    /// 契約 `SpeedFactor` の既知の値 → 文言(`JudgeContractSyncSpeedTests` が契約との漏れを検出する)。
    static let expectedFactorNames: [String: String] = [
        "rank": "ランク補正", "tailwind": "追い風", "ability": "特性", "choiceScarf": "こだわりスカーフ", "item": "持ち物", "paralysis": "まひ",
    ]
    /// 契約 `SpeedIgnoredInput` の既知の値 → 文言。
    static let expectedIgnoredNames: [String: String] = ["abilityId": "特性", "itemId": "持ち物", "fieldWeather": "天候"]

    func testKnownFactorNames() {
        for (value, expected) in Self.expectedFactorNames {
            XCTAssertEqual(JudgeLabels.speedFactorName(value), expected, value)
        }
    }

    func testKnownIgnoredNames() {
        for (value, expected) in Self.expectedIgnoredNames {
            XCTAssertEqual(JudgeLabels.speedIgnoredName(value), expected, value)
        }
    }

    /// 未知の値(契約に値が増えた)でも落とさず、契約の英語の値を画面に出さない汎用の文言にする。既知の値の文言とは区別できる。
    func testUnknownValuesFallBackToAGenericJapaneseLabel() {
        XCTAssertEqual(JudgeLabels.speedFactorUnknown, "その他の補正")
        XCTAssertEqual(JudgeLabels.speedIgnoredUnknown, "その他の入力")
        for value in ["futureFactor", "", "Rank", "RANK", "choicescarf"] {
            XCTAssertEqual(JudgeLabels.speedFactorName(value), JudgeLabels.speedFactorUnknown, "大文字小文字も区別する: \(value)")
            XCTAssertEqual(JudgeLabels.speedIgnoredName(value), JudgeLabels.speedIgnoredUnknown, value)
        }
        XCTAssertFalse(Self.expectedFactorNames.values.contains(JudgeLabels.speedFactorUnknown))
        XCTAssertFalse(Self.expectedIgnoredNames.values.contains(JudgeLabels.speedIgnoredUnknown))
    }

    func testSideNames() {
        XCTAssertEqual(JudgeLabels.speedSideSelf, "自分")
        XCTAssertEqual(JudgeLabels.speedSideOpponent, "相手")
    }

    func testAppliedNoteJoinsWithMiddleDot() {
        XCTAssertEqual(
            JudgeLabels.speedAppliedNote(side: JudgeLabels.speedSideSelf, names: ["ランク補正", "追い風", "こだわりスカーフ", "まひ"]),
            "自分の素早さに反映: ランク補正・追い風・こだわりスカーフ・まひ")
        XCTAssertEqual(JudgeLabels.speedAppliedNote(side: JudgeLabels.speedSideOpponent, names: ["特性"]), "相手の素早さに反映: 特性")
    }

    func testIgnoredNoteSaysTheInputWasNotApplied() {
        XCTAssertEqual(
            JudgeLabels.speedIgnoredNote(side: JudgeLabels.speedSideOpponent, names: ["特性", "持ち物", "天候"]),
            "相手の素早さに特性・持ち物・天候は反映していません")
        XCTAssertEqual(JudgeLabels.speedIgnoredNote(side: JudgeLabels.speedSideSelf, names: ["持ち物"]), "自分の素早さに持ち物は反映していません")
    }

    func testStatusLabelsMatchTheWebWording() {
        XCTAssertEqual(JudgeLabels.status, "状態異常")
        let expected: [JudgeStatus: String] = [
            .none: "なし", .burn: "やけど", .paralysis: "まひ", .poison: "どく", .badlyPoison: "もうどく", .sleep: "ねむり", .freeze: "こおり",
        ]
        XCTAssertEqual(Set(expected.keys), Set(JudgeStatus.allCases), "全値の文言がある")
        for (status, name) in expected {
            XCTAssertEqual(JudgeLabels.statusName(status), name, status.rawValue)
        }
    }

    func testExistingLabelsAreUnchanged() {
        XCTAssertEqual(JudgeLabels.nature, "性格")
        XCTAssertEqual(JudgeLabels.ability, "特性")
        XCTAssertEqual(JudgeLabels.item, "持ち物")
        XCTAssertEqual(JudgeLabels.errorMessage(forCode: "some_future_code"), JudgeLabels.errorFallback, "ErrorCode の未知値の流儀(既存)")
    }
}
