import XCTest

@testable import PokeCalcCore

/// AJ7: 調整画面の文言と書式(ADR-0502 §6・AC5・AC6)。語は Web の `web/src/i18n/ja.ts`(adjustScreenText・
/// adjustErrorText)と同じ。期待値は文字列で直接書く(`AdjustText` の定数で期待値を作ると、語の変更を検出できない)。
final class AdjustTextTests: XCTestCase {

    // MARK: - 確率(0.1% 単位の切り捨て。ADR-0319 §6)

    func testChancePercentIsFlooredToTenthsAndDropsTrailingZero() {
        let cases: [(Double, String)] = [
            (100, "100%"), (99.99, "99.9%"), (99.95, "99.9%"), (37.5, "37.5%"), (12.34, "12.3%"),
            (50, "50%"), (0, "0%"), (0.04, "0%"), (87.5, "87.5%"),
        ]
        for (input, expected) in cases {
            XCTAssertEqual(AdjustText.chancePercentText(input), expected, "\(input)")
        }
    }

    // MARK: - エラー(code → 日本語。サーバーの message を出さない)

    func testServerErrorCodesMapToJapaneseWithoutServerMessage() {
        let serverMessage = "internal: engine rejected input"
        for (code, expected) in AdjustText.errorMessages {
            let text = AdjustText.errorMessage(for: PokeCalcError(code: code, message: serverMessage))
            XCTAssertEqual(text, expected, code)
            XCTAssertFalse(text.contains(serverMessage), "\(code): サーバーの message を出さない")
        }
    }

    func testErrorTableCoversWebVocabulary() {
        XCTAssertEqual(AdjustText.errorMessages["invalid_input"], "入力の値が範囲の外です。能力ポイント・上限・発数・確率を確かめてください")
        XCTAssertEqual(AdjustText.errorMessages["master_unavailable"], "ポケモンのデータの準備ができていません。しばらくしてからお試しください")
        XCTAssertEqual(AdjustText.errorMessages["not_found"], "見つかりませんでした。入力を確かめてください")
    }

    func testUnknownCodeMapsToFallback() {
        let text = AdjustText.errorMessage(for: PokeCalcError(code: "something_new", message: "secret"))
        XCTAssertEqual(text, "調整に失敗しました")
    }

    func testTransportAndDecodeMapToUnavailable() {
        for code in [PokeCalcError.Code.transport, PokeCalcError.Code.decode] {
            XCTAssertEqual(AdjustText.errorMessage(for: PokeCalcError(code: code, message: "URLError -1009")), "調整のサーバーに接続できません", code)
        }
    }

    func testNonPokeCalcErrorMapsToFallback() {
        struct Other: Error {}
        XCTAssertEqual(AdjustText.errorMessage(for: Other()), "調整に失敗しました")
    }

    // MARK: - 欄・選択肢

    func testFieldAndOptionTexts() {
        XCTAssertEqual(AdjustText.fixedSPLabel(.hp), "H の固定ポイント")
        XCTAssertEqual(AdjustText.fixedSPLabel(.spa), "C の固定ポイント")
        XCTAssertEqual(AdjustText.fixedSPTotal(36), "合計 36 / 66")
        XCTAssertEqual(AdjustText.ceilingLabel(.def), "B の上限")
        XCTAssertEqual(AdjustText.hitsOption(2), "2発")
        XCTAssertEqual(AdjustText.thresholdOption(100), "確定(100%)")
        XCTAssertEqual(AdjustText.thresholdOption(90), "90% 以上")
        XCTAssertEqual(AdjustText.spRangeInvalid, "能力ポイントは0〜32の整数で入力してください")
        XCTAssertEqual(AdjustText.spTotalExceeded, "能力ポイントの合計は66までです")
    }

    func testModeLabelsFollowWebOrder() {
        XCTAssertEqual(AdjustMode.allCases.map(AdjustText.modeLabel), [
            "今の耐久・火力と HP を見る", "耐久に振る", "攻撃と素早さに振る", "倒せるいちばん少ない振り方", "耐えられるいちばん少ない振り方",
        ])
    }

    // MARK: - 結果: 指数と 16n

    func testIndicesLines() {
        XCTAssertEqual(
            AdjustText.statsLine(StatBlock(hp: 155, atk: 100, def: 90, spa: 80, spd: 85, spe: 120)),
            "実数値 H 155 / A 100 / B 90 / C 80 / D 85 / S 120")
        XCTAssertEqual(AdjustText.indexLine("火力の目安", 12000), "火力の目安 12000")
        XCTAssertEqual(AdjustText.hpCurrent(HPLineReport(hp: 160, sp: 5, current: .line16n)), "HP 160(16の倍数)")
        XCTAssertEqual(
            AdjustText.hpCurrent(HPLineReport(hp: 155, sp: 0, current: .none)), "HP 155(16の倍数でも、16の倍数-1でもない)")
    }

    func testHPLinePointsInFixedOrderWithSignedDeltaAndNone() {
        let report = HPLineReport(
            hp: 160, sp: 5, current: .line16n,
            next16n: nil, prev16n: HPLinePoint(hp: 144, sp: 0, spDelta: -5),
            next16nMinus1: HPLinePoint(hp: 175, sp: 20, spDelta: 15), prev16nMinus1: HPLinePoint(hp: 159, sp: 4, spDelta: -1)
        )
        XCTAssertEqual(AdjustText.hpLinePoints(report), [
            "次の16の倍数: なし",
            "前の16の倍数: HP 144(H 0、-5)",
            "次の16の倍数-1: HP 175(H 20、+15)",
            "前の16の倍数-1: HP 159(H 4、-1)",
        ])
    }

    // MARK: - 結果: 最小 SP・配分

    func testKOLines() {
        XCTAssertEqual(
            AdjustText.koLine(AdjustKOResult(stat: .atk, searchLimit: 32, feasible: true, sp: 12, chancePercent: 100), hits: 2),
            "A に 12 振れば 2発で倒せます(確率 100%)")
        XCTAssertEqual(
            AdjustText.koLine(AdjustKOResult(stat: .spa, searchLimit: 32, feasible: false, sp: 32, chancePercent: 37.55), hits: 1),
            "C に 32 振っても 1発では倒せません(確率 37.5%)")
    }

    func testSurviveLines() {
        let feasible = AdjustSurviveResult(
            stat: .def, searchLimit: 66, feasible: true, hpSp: 20, statSp: 8, totalSp: 28, bulkIndex: 1, chancePercent: 100)
        XCTAssertEqual(AdjustText.surviveLine(feasible, hits: 1), "H に 20・B に 8 振れば 1発耐えます(確率 100%)")
        let infeasible = AdjustSurviveResult(
            stat: .spd, searchLimit: 66, feasible: false, hpSp: 32, statSp: 32, totalSp: 64, bulkIndex: 1, chancePercent: 99.99)
        XCTAssertEqual(AdjustText.surviveLine(infeasible, hits: 2), "H に 32・D に 32 振っても 2発は耐えられません(確率 99.9%)")
    }

    func testAllocationLines() {
        XCTAssertEqual(AdjustText.remainingLine(30), "残りの能力ポイント 30")
        XCTAssertEqual(AdjustText.planSPLine(StatBlock(hp: 32, atk: 0, def: 17, spa: 0, spd: 17, spe: 0)),
                       "能力ポイント H 32 / A 0 / B 17 / C 0 / D 17 / S 0")
        XCTAssertEqual(AdjustText.planTotal(66), "合計 66")
        XCTAssertEqual(AdjustText.goalLine(met: true, chancePercent: 100), "目標を満たします(確率 100%)")
        XCTAssertEqual(AdjustText.goalLine(met: false, chancePercent: 87.56), "目標に届きません(確率 87.5%)")
    }

    func testLearnersHeading() {
        XCTAssertEqual(AdjustText.learnersHeading("テストわざぶつり"), "テストわざぶつりを覚えるポケモン")
    }
}
