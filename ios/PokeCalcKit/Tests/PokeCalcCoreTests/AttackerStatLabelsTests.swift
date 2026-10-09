import XCTest

@testable import PokeCalcCore

/// 攻撃側 2 ブロックと技の絞り込みの文言(ADR-0518。Web の `attackerStatText`・`calcScreenText` と同じ語)。
/// 語を変えるときは Web(`web/src/i18n/ja.ts`)と同時に変える(クライアント間で語を揃える)。
final class AttackerStatLabelsTests: XCTestCase {

    func testHeadingsAndUsedSuffix() {
        XCTAssertEqual(AttackerStatLabels.statName(.atk), "攻撃")
        XCTAssertEqual(AttackerStatLabels.statName(.spa), "特攻")
        XCTAssertEqual(AttackerStatLabels.usedSuffix, "(この技で使用)")
        XCTAssertEqual(AttackerStatLabels.heading(for: .atk, isUsed: false), "攻撃")
        XCTAssertEqual(AttackerStatLabels.heading(for: .atk, isUsed: true), "攻撃(この技で使用)")
        XCTAssertEqual(AttackerStatLabels.heading(for: .spa, isUsed: true), "特攻(この技で使用)")
    }

    func testFieldLabels() {
        XCTAssertEqual(AttackerStatLabels.presetGroupLabel(.atk), "攻撃の調整")
        XCTAssertEqual(AttackerStatLabels.presetGroupLabel(.spa), "特攻の調整")
        XCTAssertEqual(AttackerStatLabels.spLabel(.atk), "攻撃のSP")
        XCTAssertEqual(AttackerStatLabels.spLabel(.spa), "特攻のSP")
        XCTAssertEqual(AttackerStatLabels.natureGroupLabel(.atk), "攻撃の性格補正")
        XCTAssertEqual(AttackerStatLabels.natureGroupLabel(.spa), "特攻の性格補正")
    }

    func testModifierNamesAreTheThreeWords() {
        XCTAssertEqual(NatureChoice.allCases.map(AttackerStatLabels.modifierName), ["上昇", "補正なし", "下降"])
        XCTAssertEqual(AttackerStatLabels.custom, "カスタム")
    }

    func testMessages() {
        XCTAssertEqual(AttackerStatLabels.spInvalid(.atk), "攻撃のSPは0〜32の整数で入力してください")
        XCTAssertEqual(AttackerStatLabels.spInvalid(.spa), "特攻のSPは0〜32の整数で入力してください")
        XCTAssertEqual(AttackerStatLabels.natureUnresolved, "この性格補正の組み合わせに当たる性格が、データにありません")
        XCTAssertEqual(AttackerStatLabels.sameDirectionReason, "攻撃と特攻の両方を上昇、または両方を下降にすることはできません")
        XCTAssertEqual(AttackerStatLabels.statusMoveNotice, "変化技はダメージを計算しません")
        XCTAssertEqual(AttackerStatLabels.noDamagingMovesNotice, "このポケモンはダメージを与える技を覚えないため、計算できません")
    }

    func testKeyboardDoneLabel() {
        XCTAssertEqual(AttackerStatLabels.keyboardDone, "完了")
    }

    /// プリセット名は既存の `AttackerPreset.label(for:)`(Web の `attackerPresetText` と同じ)のまま。
    func testPresetNamesPerStatAreUnchanged() {
        XCTAssertEqual(AttackerPreset.allCases.map { $0.label(for: .physical) }, ["無振り", "A特化", "A振り(無補正)"])
        XCTAssertEqual(AttackerPreset.allCases.map { $0.label(for: .special) }, ["無振り", "C特化", "C振り(無補正)"])
    }
}
