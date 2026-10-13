import XCTest

@testable import PokeCalcCore

/// G-05 第 4 弾(ADR-0540。ADR-0501「G-05 調整の作り直し」章)。調整画面に出す文字を、
/// 「名前 / 短いラベル / 数値・結果 / 状態の一言」の 4 種に絞る。文を置いてよいのは、エラー・空の状態・取り消せない操作の確認・
/// 単位が分からない欄の説明だけ(docs/design.md「文を減らす決まり」)。語は Web の `web/src/i18n/adjust.ts` と同じ。
final class G05AdjustTextTests: XCTestCase {

    // MARK: - 足す語(Web と同じ)

    func testNewWordsMatchWebAndGlossary() {
        XCTAssertEqual(AdjustText.goalRecentOpponents, "選んだ相手から")
        // 用語集「増やす/減らす」: <欄の名前>を増やす / 減らす(Web は欄の名前 + uiText.stepperIncrease)。
        XCTAssertEqual(AdjustText.stepperIncrease("H の固定ポイント"), "H の固定ポイントを増やす")
        XCTAssertEqual(AdjustText.stepperDecrease("H の固定ポイント"), "H の固定ポイントを減らす")
        XCTAssertEqual(AdjustText.stepperIncrease("B の上限"), "B の上限を増やす")
        XCTAssertEqual(AdjustText.stepperDecrease(AdjustText.ceilingLabel(.def)), "B の上限を減らす")
    }

    /// 目標の検査の文が Web と同じ語(「データ」。用語集の禁止語「マスタ」を使わない)。
    func testGoalNatureNotFoundUsesDataNotMaster() {
        XCTAssertEqual(AdjustText.goalNatureNotFoundMessage(2), "目標 2: 相手の振り方に合う性格がデータにありません")
        XCTAssertFalse(AdjustText.goalNatureNotFoundMessage(1).contains("マスタ"))
    }

    // MARK: - 常に画面に出る文字は文ではない(句点を含まない)

    func testAlwaysVisibleStringsAreNotSentences() {
        let visible: [String] = [
            AdjustText.screenTitle, AdjustText.ownRegion, AdjustText.modeRegion, AdjustText.opponentRegion,
            AdjustText.goalRegion, AdjustText.resultRegion, AdjustText.learnersRegion,
            AdjustText.speciesField, AdjustText.natureField, AdjustText.abilityField, AdjustText.itemField,
            AdjustText.moveField, AdjustText.presetField, AdjustText.speciesPlaceholder, AdjustText.naturePlaceholder,
            AdjustText.movePlaceholder, AdjustText.unselectedOption, AdjustText.fixedSPGroup, AdjustText.focusField,
            AdjustText.ceilingGroup, AdjustText.offenseCategoryField, AdjustText.minSpeedField, AdjustText.useGoalField,
            AdjustText.hitsField, AdjustText.thresholdField, AdjustText.submitButton, AdjustText.loadingNotice,
            AdjustText.emptyResultNotice, AdjustText.learnersButton, AdjustText.learnersEmpty, AdjustText.learnersMore,
            AdjustText.learnersLoading, AdjustText.learnersClose, AdjustText.keyboardDone, AdjustText.addGoalButton,
            AdjustText.removeGoalVisible, AdjustText.noGoalsNotice, AdjustText.goalKindField, AdjustText.goalPresetField,
            AdjustText.goalBoostMoveField, AdjustText.goalBoostMoveNone, AdjustText.goalHitsAndChanceField,
            AdjustText.goalOpponentSpeciesField, AdjustText.goalOpponentMoveField, AdjustText.goalOwnMoveField,
            AdjustText.goalRecentOpponents, AdjustText.indicesHeading, AdjustText.hpLineHeading, AdjustText.maxIndexHeading,
            AdjustText.minSpHeading, AdjustText.goalsPlanHeading, AdjustText.goalsNearestHeading,
            AdjustText.goalLimitHint(RequestLimits.maxAdjustGoals), AdjustText.fixedSPTotal(36),
            AdjustText.remainingLine(30), AdjustText.planTotal(66),
        ] + AdjustMode.allCases.map(AdjustText.modeLabel) + AdjustGoalKind.allCases.map(AdjustText.goalKindOption)
        for text in visible {
            XCTAssertFalse(text.isEmpty, "空の文字がある")
            XCTAssertFalse(text.contains("。"), "常に出る文字は文にしない: \(text)")
        }
    }

    /// 区切りボタン(目標の種類)のラベルは短い(8 文字まで。アイコンと並べても折り返しが 2 行以内)。
    func testGoalKindLabelsAreShort() {
        for kind in AdjustGoalKind.allCases {
            XCTAssertLessThanOrEqual(AdjustText.goalKindOption(kind).count, 8, "\(kind)")
        }
        XCTAssertEqual(AdjustGoalKind.allCases.map(AdjustText.goalKindOption), ["素早さを上回る", "この技を耐える", "この技で倒す"])
    }

    // MARK: - 補足の文は文面を変えず「説明」ボタンの奥へ移る(語は不変)

    func testHelpTextsKeepTheirWording() {
        XCTAssertEqual(AdjustText.fixedSPHint, "ここで決めた値より下には振りません。残りを調整に回します")
        XCTAssertEqual(AdjustText.minSpeedHint, "この実数値以上になるように S に振ります。空なら目標なし")
        XCTAssertEqual(AdjustText.goalBoostMoveHint, "ニトロチャージのように自分の素早さが上がる技を選ぶと、上がったあとの素早さで比べます")
        XCTAssertEqual(AdjustText.indexNote, "火力の目安には、タイプ一致だけを含めます(持ち物・特性・テラスタルは含めません)")
        XCTAssertEqual(AdjustText.minSpNotRequested, "目標を指定すると、目標に届くいちばん少ない振り方も出します")
        // 説明の中は 3 行まで: 1 文か 2 文(句点は文の数)。
        for text in [AdjustText.fixedSPHint, AdjustText.minSpeedHint, AdjustText.goalBoostMoveHint, AdjustText.indexNote, AdjustText.minSpNotRequested] {
            XCTAssertLessThanOrEqual(text.filter { $0 == "。" }.count, 1, text)
        }
        XCTAssertEqual(CommonPartsText.help, "説明")
        XCTAssertEqual(CommonPartsText.close, "閉じる")
    }

    /// 閉じる(アイコンだけ)の読み上げ名は共通の語。調整の「覚えるポケモン」を閉じる・技のシートを閉じるも同じ。
    func testCloseWordsAreTheCommonWord() {
        XCTAssertEqual(AdjustText.learnersClose, CommonPartsText.close)
        XCTAssertEqual(AdjustText.goalMoveSheetClose, CommonPartsText.close)
    }

    // MARK: - 変えない語(読み上げ名・識別子の元)

    func testAccessibilityNamesAreUnchanged() {
        XCTAssertEqual(AdjustText.goalFieldName(2, AdjustText.hitsDecrease), "目標 2 の発数を減らす")
        XCTAssertEqual(AdjustText.goalFieldName(2, AdjustText.hitsIncrease), "目標 2 の発数を増やす")
        XCTAssertEqual(AdjustText.removeGoalName(1), "目標 1 を外す")
        XCTAssertEqual(AdjustText.ownLearnersButtonLabel, "自分の技を覚えるポケモン")
        XCTAssertEqual(AdjustText.opponentLearnersButtonLabel, "相手の技を覚えるポケモン")
        XCTAssertEqual(AdjustText.fixedSPLabel(.hp), "H の固定ポイント")
        XCTAssertEqual(AdjustText.ceilingLabel(.spe), "S の上限")
    }
}
