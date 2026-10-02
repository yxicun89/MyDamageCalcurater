import XCTest

@testable import PokeCalcCore

/// `JudgeLabels`(P6-25。ADR-0504 §9): 文言は Web の `judgeScreenText`・`judgeErrorText`(web/src/i18n/ja.ts)と同じ。
/// 違う点(ラベルのステータス名・確率の桁・iOS 固有の添え書き)は ADR-0504 §9 に書いてある。
final class JudgeLabelsTests: XCTestCase {
    /// 契約の ErrorCode 8 値 + クライアント側の code(`JudgeContractSyncTests` が契約との漏れを検出する)。
    static let expectedErrorMessages: [String: String] = [
        "invalid_request": "入力の形が正しくありません",
        "unknown_species": "このポケモンはマスタにありません",
        "unknown_move": "この技の ID はマスタにありません",
        "unknown_nature": "この性格はマスタにありません",
        "request_too_large": "入力が大きすぎます",
        "upstream_unavailable": "判定に必要なサービスに接続できません",
        "internal_error": "判定に失敗しました",
        "not_found": "判定に失敗しました",
        PokeCalcError.Code.transport: "判定の API に接続できません",
        PokeCalcError.Code.decode: "判定の API に接続できません",
        PokeCalcError.Code.unexpectedStatus: "判定の API に接続できません",
    ]

    func testErrorMessagesByCode() {
        for (code, expected) in Self.expectedErrorMessages {
            XCTAssertEqual(JudgeLabels.errorMessage(forCode: code), expected, code)
        }
    }

    func testUnknownCodeFallsBackToTheGenericMessage() {
        XCTAssertEqual(JudgeLabels.errorMessage(forCode: "some_future_code"), "判定に失敗しました")
    }

    func testFixedTexts() {
        XCTAssertEqual(JudgeLabels.openButton, "抜いて倒せるか判定")
        XCTAssertEqual(JudgeLabels.screenTitle, "判定")
        XCTAssertEqual(JudgeLabels.attackerRegion, "自分のポケモン")
        XCTAssertEqual(JudgeLabels.defendersRegion, "相手の候補")
        XCTAssertEqual(JudgeLabels.resultRegion, "判定結果")
        XCTAssertEqual(JudgeLabels.submit, "判定する")
        XCTAssertEqual(JudgeLabels.loading, "判定中")
        XCTAssertEqual(JudgeLabels.emptyResult, "「判定する」を押すと結果が出ます")
        XCTAssertEqual(JudgeLabels.speedFieldGroup, "場の効果")
        XCTAssertEqual(JudgeLabels.trickRoom, "トリックルーム")
        XCTAssertEqual(JudgeLabels.attackerTailwind, "自分の側の追い風")
        XCTAssertEqual(JudgeLabels.defenderTailwind, "相手の側の追い風")
        XCTAssertEqual(JudgeLabels.defenderTailwindNotice, "相手の側の追い風は、すべての相手候補に同じように適用されます")
        XCTAssertEqual(JudgeLabels.addCandidate, "相手候補を追加")
        XCTAssertEqual(JudgeLabels.candidate(2), "相手候補2")
        XCTAssertEqual(JudgeLabels.removeCandidate(2), "相手候補2を削除")
    }

    func testCandidateLimitNoticeComesFromTheLimit() {
        XCTAssertEqual(JudgeLabels.maxCandidatesNotice(RequestLimits.maxJudgeDefenders), "相手候補は6件までです")
    }

    func testResultTexts() {
        XCTAssertEqual(JudgeLabels.speed(attacker: 150, defender: 120), "素早さ 150 対 120")
        XCTAssertEqual(JudgeLabels.priority(attacker: 0, defender: 1), "優先度 0 対 1")
        XCTAssertEqual(JudgeLabels.outspeedsTrue, "素早さで上回る")
        XCTAssertEqual(JudgeLabels.outspeedsFalse, "素早さで下回る")
        XCTAssertEqual(JudgeLabels.speedTie, "同速")
        XCTAssertEqual(JudgeLabels.attackerMovesFirst, "自分が先に動く")
        XCTAssertEqual(JudgeLabels.defenderMovesFirst, "相手が先に動く")
        XCTAssertEqual(JudgeLabels.turnOrderTie, "どちらが先に動くか決まらない")
        XCTAssertEqual(JudgeLabels.attackerKoPrefix, "自分の技で相手を")
        XCTAssertEqual(JudgeLabels.defenderKoPrefix, "相手の技で自分が")
        XCTAssertEqual(JudgeLabels.koNone, "倒せない")
        XCTAssertEqual(JudgeLabels.koGuaranteed(hits: 2), "確定2発")
        XCTAssertEqual(JudgeLabels.koRandom(hits: 3, percent: "37.5"), "乱数3発(37.5%)")
    }

    /// 結果の文言に「勝ち」「負け」に丸めた語を持たない(ADR-0700 §6-1・ADR-0704 §3)。
    func testNoWinLoseWordsExist() {
        let texts = [
            JudgeLabels.outspeedsTrue, JudgeLabels.outspeedsFalse, JudgeLabels.speedTie, JudgeLabels.attackerMovesFirst,
            JudgeLabels.defenderMovesFirst, JudgeLabels.turnOrderTie, JudgeLabels.attackerKoPrefix, JudgeLabels.defenderKoPrefix,
            JudgeLabels.koNone, JudgeLabels.emptyResult, JudgeLabels.loading,
        ]
        for text in texts {
            for word in ["勝ち", "負け", "勝てる", "負ける"] {
                XCTAssertFalse(text.contains(word), "「\(word)」を含む文言: \(text)")
            }
        }
    }

    func testStatLabelsUseJapaneseStatNames() {
        XCTAssertEqual(JudgeLabels.spLabel(.spe), "すばやさの能力ポイント")
        XCTAssertEqual(JudgeLabels.spLabel(.hp), "HPの能力ポイント")
        XCTAssertEqual(JudgeLabels.rankLabel(.atk), "こうげきのランク")
    }

    func testValidationMessagesNameWhoseInputIsWrongAndComeFromTheLimits() {
        XCTAssertEqual(JudgeLabels.validationMessage(.requiredMissing(.attacker)), "自分のポケモン: ポケモン・性格・技をすべて選んでください")
        XCTAssertEqual(
            JudgeLabels.validationMessage(.requiredMissing(.candidate(1))), "相手候補2: ポケモン・性格・技をすべて選んでください",
            "候補は 0 始まりの index を 1 始まりの番号で出す")
        XCTAssertEqual(
            JudgeLabels.validationMessage(.spTotalExceeded(.candidate(0))),
            "相手候補1: 能力ポイントの合計は\(SPLimits.maxTotal)までです")
        XCTAssertEqual(
            JudgeLabels.validationMessage(.moveIdInvalid(.attacker)),
            "自分のポケモン: 技の ID が長すぎます(最大\(RequestLimits.maxJudgeMoveIdLength)文字)")
        XCTAssertEqual(JudgeValidationError.spTotalExceeded(.attacker).message, JudgeLabels.validationMessage(.spTotalExceeded(.attacker)))
    }

    func testFailedCandidateHint() {
        XCTAssertEqual(JudgeLabels.failedCandidate(3), "相手候補3の入力で失敗しました")
    }

    /// 方向ごとの印の文言は、どちらの確定数についてかを先頭で言い分ける(混ぜない。ADR-0708 §4)。
    func testUnsupportedNoticesNameTheDirection() {
        XCTAssertEqual(
            JudgeLabels.attackerKoUnsupportedNotice(detail: "未対応: X"), "自分の技の確定数は正確でない可能性があります(未対応: X)")
        XCTAssertEqual(
            JudgeLabels.defenderKoUnsupportedNotice(detail: "未対応: X"), "相手の技の確定数は正確でない可能性があります(未対応: X)")
        XCTAssertNotEqual(JudgeLabels.attackerKoUnreliable, JudgeLabels.defenderKoUnreliable)
        XCTAssertTrue(JudgeLabels.attackerKoUnreliable.hasPrefix("自分の技"))
        XCTAssertTrue(JudgeLabels.defenderKoUnreliable.hasPrefix("相手の技"))
    }
}
