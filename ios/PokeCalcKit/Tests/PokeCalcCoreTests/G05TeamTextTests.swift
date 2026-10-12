import XCTest

@testable import PokeCalcCore

/// G-05 第 2 弾(ADR-0537。ADR-0501「G-05 構築の作り直し」章)。構築の一覧・編集に出す文字を、
/// 「名前 / 短いラベル / 数値・結果 / 状態の一言」の 4 種に絞る。文を置いてよいのは、エラー・空の状態・取り消せない操作の確認
/// (削除・未保存で戻る)だけ(docs/design.md「文を減らす決まり」)。語は Web の `web/src/i18n/team.ts` と同じ。
final class G05TeamTextTests: XCTestCase {

    // MARK: - 空の状態・読み込み中は一言(Web の emptyNotice・loadingNotice)

    func testEmptyListIsOneShortLine() {
        XCTAssertEqual(TeamLabels.emptyList, "まだ構築がありません")
        XCTAssertFalse(TeamLabels.emptyList.contains("。"))
        XCTAssertEqual(TeamLoadLabels.empty, TeamLabels.emptyList, "計算・逆算の「構築から選ぶ」の空と同じ語")
    }

    func testLoadingIsAStaticShortWord() {
        XCTAssertEqual(TeamLabels.loading, "読み込み中")
        XCTAssertFalse(TeamLabels.loading.contains("…"), "回って見える「…」を付けない")
    }

    // MARK: - タイルの格子(Web の slotsLabel・slotAddLabel)

    func testSlotGridAndAddTileWords() {
        XCTAssertEqual(TeamLabels.slotsGroup, "メンバー")
        XCTAssertEqual(TeamLabels.slotAdd(2), "2体目を追加")
        XCTAssertEqual(TeamLabels.slotAdd(6), "6体目を追加")
        XCTAssertTrue(TeamLabels.slotAdd(3).contains("追加"), "用語集の「追加」(空き枠のタイル)")
    }

    // MARK: - 空の枠の案内の文は無くなった(タイルの「+」と「ポケモン」欄が示す)

    func testEmptySlotKeepsOnlyThePokemonField() {
        XCTAssertEqual(TeamLabels.pokemonField, "ポケモン")
        XCTAssertEqual(TeamLabels.slotTitle(1), "1体目", "枠の見出しは変えない")
    }

    // MARK: - 技の欄(Web の moveNone)

    func testMoveWords() {
        XCTAssertEqual(TeamLabels.movesHeading, "技")
        XCTAssertEqual(TeamLabels.moveNone, "(なし)")
        XCTAssertEqual(TeamLabels.nicknameField, "ニックネーム")
    }

    // MARK: - SP: 合計と残りは数値、ボタンの名前は用語集の語

    func testSPSummaryShowsTotalMaxAndRemaining() {
        XCTAssertEqual(TeamLabels.spSummary(total: 66), "合計 66/66(残り 0)")
        XCTAssertEqual(TeamLabels.spSummary(total: 0), "合計 0/66(残り 66)")
        XCTAssertEqual(TeamLabels.spSummary(total: 40), "合計 40/66(残り 26)")
        XCTAssertFalse(TeamLabels.spSummary(total: 40).contains("。"))
    }

    func testSPButtonNamesFollowGlossary() {
        XCTAssertEqual(TeamLabels.spIncrease(statName: "こうげき"), "こうげきの能力ポイントを増やす")
        XCTAssertEqual(TeamLabels.spDecrease(statName: "こうげき"), "こうげきの能力ポイントを減らす")
        XCTAssertEqual(TeamLabels.spField(statName: "こうげき"), "こうげきの能力ポイント")
    }

    func testSPControlStopsAtLimits() {
        XCTAssertEqual(SPLimits.maxPerStat, 32)
        XCTAssertEqual(SPLimits.maxTotal, 66)
        XCTAssertTrue(TeamSPControl.canIncrease(value: 0, total: 0))
        XCTAssertTrue(TeamSPControl.canIncrease(value: 31, total: 65))
        XCTAssertFalse(TeamSPControl.canIncrease(value: 32, total: 40), "1 ステータスは 32 で止まる")
        XCTAssertFalse(TeamSPControl.canIncrease(value: 10, total: 66), "合計は 66 で止まる")
        XCTAssertTrue(TeamSPControl.canDecrease(value: 1))
        XCTAssertFalse(TeamSPControl.canDecrease(value: 0), "0 で止まる")
    }

    func testSPFillFractionIsClampedToUnitRange() {
        XCTAssertEqual(TeamSPControl.fillFraction(total: 0), 0, accuracy: 0.0001)
        XCTAssertEqual(TeamSPControl.fillFraction(total: 33), 0.5, accuracy: 0.0001)
        XCTAssertEqual(TeamSPControl.fillFraction(total: 66), 1, accuracy: 0.0001)
        XCTAssertEqual(TeamSPControl.fillFraction(total: 99), 1, accuracy: 0.0001)
        XCTAssertEqual(TeamSPControl.fillFraction(total: -3), 0, accuracy: 0.0001)
    }

    // MARK: - 変えない語(読み上げ名・Web と同じ語。既存テストと同じ値をここでも固定する)

    func testExistingWordsStay() {
        XCTAssertEqual(TeamLabels.createButton, "新しい構築")
        XCTAssertEqual(TeamLabels.openButton, "開く")
        XCTAssertEqual(TeamLabels.openHint(name: "構築 1"), "「構築 1」を開く")
        XCTAssertEqual(TeamLabels.deleteHint(name: "構築 1"), "「構築 1」を削除")
        XCTAssertEqual(TeamLabels.deleteConfirmButton, "削除する")
        XCTAssertEqual(TeamLabels.deleteCancelButton, "やめる")
        XCTAssertEqual(TeamLabels.moveUp(1), "1体目を上へ")
        XCTAssertEqual(TeamLabels.moveDown(1), "1体目を下へ")
        XCTAssertEqual(TeamLabels.remove(1), "1体目を外す")
        XCTAssertEqual(TeamLabels.save, "保存")
        XCTAssertEqual(TeamLabels.savedNotice, "保存しました")
        XCTAssertEqual(TeamLabels.unsavedNotice, "保存していない変更があります")
    }

    // MARK: - 常に画面に出る文字に「。」を置かない(文を置いてよいのは確認とエラーだけ)

    func testAlwaysVisibleStringsAreNotSentences() {
        let visible: [(String, String)] = [
            ("createButton", TeamLabels.createButton),
            ("emptyList", TeamLabels.emptyList),
            ("loading", TeamLabels.loading),
            ("openButton", TeamLabels.openButton),
            ("deleteConfirmButton", TeamLabels.deleteConfirmButton),
            ("deleteCancelButton", TeamLabels.deleteCancelButton),
            ("memberCount", TeamLabels.memberCount(3)),
            ("backToList", TeamLabels.backToList),
            ("save", TeamLabels.save),
            ("savedNotice", TeamLabels.savedNotice),
            ("unsavedNotice", TeamLabels.unsavedNotice),
            ("leaveDiscard", TeamLabels.leaveDiscard),
            ("leaveCancel", TeamLabels.leaveCancel),
            ("pokemonField", TeamLabels.pokemonField),
            ("slotTitle", TeamLabels.slotTitle(1)),
            ("slotAdd", TeamLabels.slotAdd(1)),
            ("slotsGroup", TeamLabels.slotsGroup),
            ("movesHeading", TeamLabels.movesHeading),
            ("moveNone", TeamLabels.moveNone),
            ("nicknameField", TeamLabels.nicknameField),
            ("spSummary", TeamLabels.spSummary(total: 10)),
            ("entryTitle", TeamLoadLabels.entryTitle),
            ("TeamLoadLabels.empty", TeamLoadLabels.empty),
        ]
        for (name, text) in visible {
            XCTAssertFalse(text.isEmpty, "\(name) が空")
            XCTAssertFalse(text.contains("。"), "\(name) は文にしない: \(text)")
            XCTAssertFalse(text.contains("\n"), name)
        }
    }

    /// 取り消せない操作の確認は文のまま(削除・未保存で戻る)。文は 2 文まで。
    func testConfirmationSentencesAreKept() {
        XCTAssertEqual(TeamLabels.deleteConfirmNotice(name: "構築 1"), "「構築 1」を削除します。よろしいですか")
        XCTAssertEqual(TeamLabels.leaveConfirmNotice, "保存していない変更があります。保存せずに一覧に戻りますか")
        XCTAssertLessThanOrEqual(TeamLabels.leaveConfirmNotice.filter { $0 == "。" }.count, 1)
    }

    /// 短いラベルは目安 8 文字まで(名前つきの読み上げ名・確認の文は除く)。
    func testShortLabelsFitTheGuide() {
        let shortLabels = [
            TeamLabels.createButton, TeamLabels.openButton, TeamLabels.backToList, TeamLabels.save, TeamLabels.pokemonField,
            TeamLabels.slotsGroup, TeamLabels.movesHeading, TeamLabels.moveNone, TeamLabels.nicknameField,
            TeamLabels.leaveDiscard, TeamLabels.leaveCancel, TeamLabels.loading, TeamLabels.deleteConfirmButton,
        ]
        for label in shortLabels {
            XCTAssertLessThanOrEqual(label.count, 8, label)
        }
    }
}
