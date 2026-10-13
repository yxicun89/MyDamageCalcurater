import XCTest

@testable import PokeCalcCore

/// G-05 第 3 弾(ADR-0539。ADR-0501「G-05 逆算の作り直し」章)。逆算画面に出す文字を、
/// 「名前 / 短いラベル / 数値・結果 / 状態の一言」の 4 種に絞る。文を置いてよいのは、エラー・空の状態・取り消せない操作の確認・
/// 単位が分からない欄の 1 行(docs/design.md「文を減らす決まり」)。語は Web の `web/src/i18n/ja.ts`
/// (`reverseScreenText`・`reverseResultText`)と同じ。
final class G05ReverseTextTests: XCTestCase {

    // MARK: - カードの見出し・観測欄(Web の myRegionLabel・observationLabel・percentUnitLabel・damageUnitLabel)

    func testRegionHeadingsMatchWeb() {
        XCTAssertEqual(ReverseLabels.myRegion, "自分")
        XCTAssertEqual(ReverseLabels.opponentRegion, "相手")
    }

    /// 単位はダメージの側で決まる(与えた = %、受けた = HP。ADR-0501「判断した点」)ので iOS に単位の切り替えは無いが、語は Web と同じ「%」「HP」。
    func testUnitLabelsMatchWebAndNoLongerSayDamage() {
        XCTAssertEqual(ReverseLabels.unitLabel(.percent), "%")
        XCTAssertEqual(ReverseLabels.unitLabel(.damage), "HP", "旧: 「ダメージ」(欄の名前と同じ語で単位に見えなかった)")
    }

    func testObservationLabelMatchesWeb() {
        XCTAssertEqual(ReverseLabels.observationLabel(1), "ダメージ1")
        XCTAssertEqual(ReverseLabels.observationLabel(16), "ダメージ16")
    }

    /// 単位の 1 行(補足 1 行まで。単位が分からない欄なので文を置いてよい 4 番目の例外)。Web の `observationHintLabel` と同じ文。
    func testObservationHintMatchesWeb() {
        XCTAssertEqual(ReverseLabels.observationHint(side: .defender), "相手の HP が減った割合(%)")
        XCTAssertEqual(ReverseLabels.observationHint(side: .attacker), "自分の HP が減った実数値(HP)")
        for side in [ReverseSide.defender, .attacker] {
            XCTAssertFalse(ReverseLabels.observationHint(side: side).contains("。"), "1 行の短い補足(文末の句点なし)")
        }
    }

    // MARK: - 候補のカード(Web の reverseResultText)

    func testCandidateBadgeAndLabelWordsMatchWeb() {
        XCTAssertEqual(ReverseLabels.predictedPercent, "予測")
        XCTAssertEqual(ReverseLabels.closeCandidate, "ほぼ合う候補", "用語集の言い換え(「近い候補」は使わない)")
        XCTAssertEqual(ReverseLabels.referenceRange, "参考")
    }

    func testNoExactNoticeMatchesWeb() {
        XCTAssertEqual(
            ReverseLabels.noExactNotice,
            "入力したダメージにぴったり合う振り方が見つかりません(技・持ち物・入力した値を確かめてください)")
    }

    func testLoadingIsAStaticShortWord() {
        XCTAssertEqual(ReverseLabels.loading, "計算中")
        XCTAssertFalse(ReverseLabels.loading.contains("…"), "回って見える「…」を付けない")
    }

    // MARK: - 前提・一致件数は短く(旧: 「相手の HP の SP を 32(H32)と仮定した結果です」「入力したダメージと一致: 2 件 / 候補 3 件」)

    func testAssumedHPNoteMatchesWeb() {
        XCTAssertEqual(ReverseLabels.assumedHP(32), "H32 を仮定した結果です")
        XCTAssertEqual(ReverseLabels.assumedHP(20), "H20 を仮定した結果です", "32 を直書きしない")
    }

    func testExactCountIsANumberLine() {
        XCTAssertEqual(ReverseLabels.exactCount(exact: 2, total: 3), "一致 2 件 / 候補 3 件")
        XCTAssertEqual(ReverseLabels.exactCount(exact: 0, total: 0), "一致 0 件 / 候補 0 件")
    }

    func testNoExactCandidateIsDecidedFromEngineCountOnly() {
        XCTAssertTrue(ReverseLabels.hasNoExactCandidate(exactCount: 0, candidateCount: 1))
        XCTAssertTrue(ReverseLabels.hasNoExactCandidate(exactCount: 0, candidateCount: 5))
        XCTAssertFalse(ReverseLabels.hasNoExactCandidate(exactCount: 0, candidateCount: 0), "候補が無いときは案内を出さない(空の結果)")
        XCTAssertFalse(ReverseLabels.hasNoExactCandidate(exactCount: 1, candidateCount: 3))
        XCTAssertFalse(ReverseLabels.hasNoExactCandidate(exactCount: 3, candidateCount: 3))
    }

    // MARK: - 画面の整形が新しい語を使う

    func testDisplayUsesTheNewWords() {
        let candidate = ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: nil,
            ranges: [SPRange(min: 0, max: 32)], spCount: 33, exact: true, mismatch: 0, support: 99,
            minPercent: 10, maxPercent: 20)
        let defender = ReverseResultDisplay(
            result: ReverseResult(side: .defender, stat: .def, assumedHPSP: 32, candidates: [candidate], exactCount: 1), items: [])
        XCTAssertEqual(defender.premiseText, ReverseLabels.assumedHP(32))
        XCTAssertEqual(defender.exactCountText, ReverseLabels.exactCount(exact: 1, total: 1))
        let attacker = ReverseResultDisplay(
            result: ReverseResult(side: .attacker, stat: .atk, assumedHPSP: 0, candidates: [], exactCount: 0), items: [])
        XCTAssertNil(attacker.premiseText, "攻撃側の逆算は相手の H を使わないので前提を出さない(不変)")
    }

    // MARK: - 変えない語(読み上げ名・Web と同じ語。既存テストと同じ値をここでも固定する)

    func testExistingWordsStay() {
        XCTAssertEqual(ReverseCandidateDisplay.natureClassLabel(.neutral, stat: .def), "補正なし")
        XCTAssertEqual(ReverseCandidateDisplay.natureClassLabel(.plus, stat: .def), "B上昇")
        XCTAssertEqual(ReverseCandidateDisplay.matchLabel(exact: true), "入力したダメージと一致", "読み上げの値として残す")
        XCTAssertEqual(ReverseCandidateDisplay.matchLabel(exact: false), "一致なし(最も近い SP)")
        XCTAssertEqual(AbilityPickerLabels.opponentTitle, "相手の特性")
        XCTAssertEqual(CommonPartsText.help, "説明")
        XCTAssertEqual(CommonPartsText.close, "閉じる")
    }

    // MARK: - 常に画面に出る文字に「。」を置かない(文を置いてよいのは確認・エラー・状態の案内・単位の 1 行だけ)

    func testAlwaysVisibleStringsAreNotSentences() {
        let visible: [(String, String)] = [
            ("myRegion", ReverseLabels.myRegion), ("opponentRegion", ReverseLabels.opponentRegion),
            ("unit%", ReverseLabels.unitLabel(.percent)), ("unitHP", ReverseLabels.unitLabel(.damage)),
            ("observationLabel", ReverseLabels.observationLabel(1)),
            ("predicted", ReverseLabels.predictedPercent), ("close", ReverseLabels.closeCandidate),
            ("reference", ReverseLabels.referenceRange), ("loading", ReverseLabels.loading),
            ("assumedHP", ReverseLabels.assumedHP(32)), ("exactCount", ReverseLabels.exactCount(exact: 2, total: 3)),
        ]
        for (name, text) in visible {
            XCTAssertFalse(text.isEmpty, "\(name) が空")
            XCTAssertFalse(text.contains("。"), "\(name) は文ではなく短い語・数値: \(text)")
        }
    }

    /// 短いラベルの目安(8 文字まで。数値・単位・状態の案内は除く)。
    func testShortLabelsStayShort() {
        for text in [ReverseLabels.myRegion, ReverseLabels.opponentRegion, ReverseLabels.predictedPercent, ReverseLabels.closeCandidate, ReverseLabels.referenceRange, ReverseLabels.loading] {
            XCTAssertFalse(text.isEmpty)
            XCTAssertLessThanOrEqual(text.count, 8, text)
        }
    }

    // MARK: - 予測%のバー(装飾。既存の `BulkRowDisplay.barFraction` をそのまま使う)

    func testBarFractionIsClampedToUnitRange() {
        XCTAssertEqual(BulkRowDisplay.barFraction(percent: 0), 0, accuracy: 0.0001)
        XCTAssertEqual(BulkRowDisplay.barFraction(percent: 45), 0.45, accuracy: 0.0001)
        XCTAssertEqual(BulkRowDisplay.barFraction(percent: 100), 1, accuracy: 0.0001)
        XCTAssertEqual(BulkRowDisplay.barFraction(percent: 250), 1, accuracy: 0.0001)
        XCTAssertEqual(BulkRowDisplay.barFraction(percent: -5), 0, accuracy: 0.0001)
    }
}
