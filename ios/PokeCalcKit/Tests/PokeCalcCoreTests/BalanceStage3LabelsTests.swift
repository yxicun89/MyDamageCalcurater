import XCTest

@testable import PokeCalcCore

// タイプバランス第3段(仮想敵・おすすめタイプ・技範囲チェッカー。ADR-0415 §8)の日本語ラベル。
// 仮想敵・おすすめの文言は Web の web/src/i18n/ja.ts(balanceLabelText・balanceScreenText)と同じ語。
// 技範囲チェッカーは Web に画面が無いので、この段で iOS 側に決めた文言(設計 §10 の倍率書式は共通)。
final class BalanceStage3LabelsTests: XCTestCase {

    // MARK: - 倍率・真偽値(語は応答の値のまま。倍率から判定し直さない)

    func testMatchupMultiplierLabelUsesResponseStringAndNoAttackMoveForNil() {
        XCTAssertEqual(BalanceLabels.matchupMultiplierLabel("2"), "×2")
        XCTAssertEqual(BalanceLabels.matchupMultiplierLabel("3/4"), "×3/4")
        XCTAssertEqual(BalanceLabels.matchupMultiplierLabel("0"), "×0")
        XCTAssertEqual(BalanceLabels.matchupMultiplierLabel(nil), "攻撃技なし")
    }

    func testSafeAndSuperEffectiveLabelsFollowBooleans() {
        XCTAssertEqual(BalanceLabels.safeLabel(true), "安全")
        XCTAssertEqual(BalanceLabels.safeLabel(false), "注意")
        XCTAssertEqual(BalanceLabels.superEffectiveLabel(true), "抜群")
        XCTAssertEqual(BalanceLabels.superEffectiveLabel(false), "ふつう")
    }

    func testTypeListTextJoinsWithMiddleDotAndShowsNoneWhenEmpty() {
        XCTAssertEqual(BalanceLabels.typeListText([.fire, .water]), "ほのお・みず")
        XCTAssertEqual(BalanceLabels.typeListText([.normal]), "ノーマル")
        XCTAssertEqual(BalanceLabels.typeListText([]), "なし")
    }

    // MARK: - 仮想敵・おすすめタイプ(Web の balanceScreenText と同じ語)

    func testThreatTexts() {
        XCTAssertEqual(BalanceScreenText.threatGroupLabel(2), "仮想敵2")
        XCTAssertEqual(BalanceScreenText.threatsRegionLabel, "仮想敵")
        XCTAssertEqual(BalanceScreenText.addThreatLabel, "仮想敵を追加")
        XCTAssertEqual(BalanceScreenText.removeThreatLabel(2), "仮想敵2を削除")
        XCTAssertEqual(BalanceScreenText.threatRegionLabel(1, name: "テストアルファ"), "仮想敵1(テストアルファ)")
        XCTAssertEqual(BalanceScreenText.threatMatchupTableLabel, "相性")
        XCTAssertEqual(BalanceScreenText.incomingColumnLabel, "受ける倍率")
        XCTAssertEqual(BalanceScreenText.outgoingColumnLabel, "与える倍率")
        XCTAssertEqual(BalanceScreenText.safeColumnLabel, "安全")
        XCTAssertEqual(BalanceScreenText.safeMembersLabel(3), "安全に受けられる 3人")
        XCTAssertEqual(BalanceScreenText.superEffectiveMembersLabel(1), "抜群を取れる 1人")
        XCTAssertEqual(BalanceScreenText.threatsLoadingNotice, "仮想敵を計算中")
        XCTAssertEqual(BalanceScreenText.tooManyThreats, "仮想敵は\(TeamLimits.maxMembers)体までです")
        XCTAssertEqual(BalanceScreenText.emptyThreatsNotice, "仮想敵を追加すると、自分のメンバーとの相性が表示されます")
    }

    func testRecommendationTexts() {
        XCTAssertEqual(BalanceScreenText.recommendationsRegionLabel, "おすすめタイプ")
        XCTAssertEqual(BalanceScreenText.recommendationsLoadingNotice, "おすすめタイプを計算中")
        XCTAssertEqual(BalanceScreenText.defenseHolesLabel("ほのお・みず"), "防御の穴: ほのお・みず")
        XCTAssertEqual(BalanceScreenText.offenseHolesLabel("なし"), "攻撃範囲の穴: なし")
        XCTAssertEqual(BalanceScreenText.candidatesTableLabel, "おすすめタイプの候補")
        XCTAssertEqual(BalanceScreenText.typesColumnLabel, "タイプ")
        XCTAssertEqual(BalanceScreenText.defenseCoveredColumnLabel, "ふさぐ防御の穴")
        XCTAssertEqual(BalanceScreenText.offenseCoveredColumnLabel, "ふさぐ攻撃範囲の穴")
        XCTAssertEqual(BalanceScreenText.pokemonColumnLabel, "ポケモン")
        XCTAssertEqual(BalanceScreenText.abilityOptionsTableLabel, "特性で補えるポケモン")
        XCTAssertEqual(BalanceScreenText.listSeparator, "・")
        XCTAssertEqual(
            BalanceScreenText.abilityOptionEntryLabel(name: "テストアルファ", ability: "テストとくせい", multiplier: "×1/2"),
            "テストアルファ(テストとくせい ×1/2)")
    }

    // MARK: - 技範囲チェッカー(iOS で決めた文言)

    func testMoveRangeTexts() {
        XCTAssertEqual(BalanceScreenText.moveRangeRegionLabel, "技範囲チェッカー")
        XCTAssertEqual(BalanceScreenText.moveRangeAddMoveLabel, "技を追加")
        XCTAssertEqual(BalanceScreenText.moveRangeEmptyNotice, "技を1つ以上選ぶと、攻撃範囲と受けられるポケモンが表示されます")
        XCTAssertEqual(BalanceScreenText.moveRangeLoadingNotice, "技範囲を計算中")
        XCTAssertEqual(BalanceScreenText.moveRangeTypeChartLabel, "技構成の攻撃範囲")
        XCTAssertEqual(BalanceScreenText.walledByLabel, "半減以下で受けられるポケモン")
        XCTAssertEqual(BalanceScreenText.walledByAbilityLabel, "特性で半減以下にできるポケモン")
        XCTAssertEqual(BalanceScreenText.noWalledNotice, "受けられるポケモンはいません")
        XCTAssertEqual(BalanceScreenText.moreCountLabel(5), "ほか 5件")
        XCTAssertEqual(BalanceScreenText.retryLabel, "再計算")
        XCTAssertEqual(BalanceScreenText.moveRangeMoveLabel(3), "技3")
    }

    /// 技範囲の倍率は攻撃範囲と同じ書式(「×2 抜群」)。null は来ないので、非 nil の値で引く。
    func testMoveRangeMultiplierUsesCoverageLabel() {
        XCTAssertEqual(BalanceLabels.coverageLabel(BalanceCoverageMultiplier.double), "×2 抜群")
        XCTAssertEqual(BalanceLabels.coverageLabel(BalanceCoverageMultiplier.half), "×1/2 いまひとつ")
    }

    // MARK: - エラー(第3段で増えるコードも Web と同じ日本語)

    func testOverloadedAndUnavailableMessages() {
        XCTAssertEqual(BalanceErrorText.message(forCode: "overloaded"), "サーバーが混み合っています。しばらくしてからもう一度お試しください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "unknown_move"), "選んだ技がサーバーのデータにありません。選び直してください")
        XCTAssertEqual(
            BalanceScreenError(PokeCalcError(code: PokeCalcError.Code.transport, message: "x")).message,
            "タイプバランスのサーバーに接続できません")
    }

    // MARK: - 表示上限(長い一覧を端末で出し切らない)

    func testDisplayLimitsArePositive() {
        XCTAssertGreaterThan(BalanceDisplayLimits.pokemonPerList, 0)
        XCTAssertGreaterThan(BalanceDisplayLimits.abilityNameResolveLimit, 0)
    }
}
