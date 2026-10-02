import XCTest

@testable import PokeCalcCore

/// `BalanceLabels`(P6-26。ADR-0505 §9): Web の `balanceScreenText`・`balanceLabelText`・`balanceErrorText` と同じ語(iOS の違いは ADR-0505 §9 の 4 点だけ)。
final class BalanceLabelsTests: XCTestCase {
    /// 契約の ErrorCode 10 値 + クライアント側の code(`BalanceContractSyncTests` が契約との漏れを検出する)。
    static let expectedErrorMessages: [String: String] = [
        "missing_request_context": "端末の情報を送れませんでした。アプリを開き直してください",
        "invalid_request": "リクエストが正しくありません。構築を見直してください",
        "request_too_large": "入力が大きすぎます。構築のメンバーや技を減らしてください",
        "unknown_pokemon": "構築のポケモンがサーバーのマスタにありません。構築を見直してください",
        "unknown_move": "構築の技がサーバーのマスタにありません。構築を見直してください",
        "unknown_ability": "構築の特性がサーバーのマスタにありません。構築を見直してください",
        "master_unavailable": "サーバーのマスタを読み込めません。しばらくしてからもう一度お試しください",
        "overloaded": "サーバーが混み合っています。しばらくしてからもう一度お試しください",
        "internal_error": "サーバーでエラーが起きました。しばらくしてからもう一度お試しください",
        "not_found": "タイプバランスを計算できませんでした。しばらくしてからもう一度お試しください",
        PokeCalcError.Code.transport: "タイプバランスの API に接続できません",
        PokeCalcError.Code.decode: "タイプバランスの API に接続できません",
        PokeCalcError.Code.unexpectedStatus: "タイプバランスの API に接続できません",
    ]

    func testErrorMessagesByCode() {
        for (code, expected) in Self.expectedErrorMessages {
            XCTAssertEqual(BalanceLabels.errorMessage(forCode: code), expected, code)
        }
    }

    func testUnknownCodeFallsBackToTheGenericMessage() {
        XCTAssertEqual(
            BalanceLabels.errorMessage(forCode: "some_future_code"), "タイプバランスを計算できませんでした。しばらくしてからもう一度お試しください")
    }

    func testFailureMessageComesFromTheCodeNotFromTheServer() {
        XCTAssertEqual(BalanceFailure(code: "overloaded").message, Self.expectedErrorMessages["overloaded"])
    }

    func testFixedTexts() {
        XCTAssertEqual(BalanceLabels.openButton, "タイプバランス")
        XCTAssertEqual(BalanceLabels.screenTitle, "タイプバランス")
        XCTAssertEqual(BalanceLabels.defenseRegion, "防御相性")
        XCTAssertEqual(BalanceLabels.teamSummaryRegion, "チームの集計")
        XCTAssertEqual(BalanceLabels.coverageRegion, "攻撃範囲")
        XCTAssertEqual(BalanceLabels.attackTypeLabel, "攻撃タイプ")
        XCTAssertEqual(BalanceLabels.defenseTypeLabel, "防御タイプ")
        XCTAssertEqual(BalanceLabels.weakLabel, "弱点")
        XCTAssertEqual(BalanceLabels.quadWeakLabel, "うち×4")
        XCTAssertEqual(BalanceLabels.resistLabel, "耐性")
        XCTAssertEqual(BalanceLabels.immuneLabel, "無効")
        XCTAssertEqual(BalanceLabels.neutralLabel, "等倍")
        XCTAssertEqual(BalanceLabels.coverageNoAttackMove, "攻撃技なし")
        XCTAssertEqual(BalanceLabels.loading, "解析中")
        XCTAssertEqual(BalanceLabels.memberCount(3), "3体")
    }

    /// 語は応答の分類から選ぶ(値の範囲を iOS で判定し直さない)。`quad_weak` も「弱点」(×4 は倍率に出る)。
    func testDefenseCategoryWords() {
        XCTAssertEqual(BalanceLabels.defenseCategoryWord(.quadWeak), "弱点")
        XCTAssertEqual(BalanceLabels.defenseCategoryWord(.weak), "弱点")
        XCTAssertEqual(BalanceLabels.defenseCategoryWord(.neutral), "等倍")
        XCTAssertEqual(BalanceLabels.defenseCategoryWord(.resist), "耐性")
        XCTAssertEqual(BalanceLabels.defenseCategoryWord(.quadResist), "耐性")
        XCTAssertEqual(BalanceLabels.defenseCategoryWord(.immune), "無効")
    }

    /// 倍率は応答の文字列のまま(既約分数をそのまま出す。float にしない)。色だけで表さず語も出す。
    func testDefenseTextKeepsTheMultiplierString() {
        XCTAssertEqual(BalanceLabels.defenseText(multiplier: "4", category: .quadWeak), "×4 弱点")
        XCTAssertEqual(BalanceLabels.defenseText(multiplier: "1/4", category: .quadResist), "×1/4 耐性")
        XCTAssertEqual(BalanceLabels.defenseText(multiplier: "3/4", category: .resist), "×3/4 耐性")
        XCTAssertEqual(BalanceLabels.defenseText(multiplier: "0", category: .immune), "×0 無効")
        XCTAssertEqual(BalanceLabels.defenseText(multiplier: "1", category: .neutral), "×1 等倍")
    }

    func testAbilityNotes() {
        XCTAssertEqual(BalanceLabels.abilityNote(.immune), "特性で無効")
        XCTAssertEqual(BalanceLabels.abilityNote(.absorb), "特性で吸収")
        XCTAssertEqual(BalanceLabels.abilityNote(.multiplier), "特性で倍率が変わる")
        XCTAssertEqual(BalanceLabels.abilityNote(.none), "特性の影響")
    }

    func testSummaryText() {
        XCTAssertEqual(
            BalanceLabels.summaryText(weak: 2, quadWeak: 1, resist: 1, immune: 0, neutral: 3), "弱点 2(うち×4 1)・耐性 1・無効 0・等倍 3")
    }

    func testCoverageWordsAndText() {
        XCTAssertEqual(BalanceLabels.coverageText(.zero), "×0 無効")
        XCTAssertEqual(BalanceLabels.coverageText(.half), "×1/2 いまひとつ")
        XCTAssertEqual(BalanceLabels.coverageText(.neutral), "×1 等倍")
        XCTAssertEqual(BalanceLabels.coverageText(.double), "×2 抜群")
        XCTAssertEqual(BalanceLabels.coverageText(nil), "攻撃技なし")
        XCTAssertEqual(BalanceLabels.teamCoverageText(effectiveMembers: 4, superEffectiveMembers: 2), "有効 4体・抜群 2体")
    }

    /// 構築から選ぶ入口の案内(Web には無い iOS 固有。違いは ADR-0505 §9)。
    func testTeamInputGuidance() {
        XCTAssertEqual(BalanceLabels.selectPrompt, "構築を選ぶと、防御相性と攻撃範囲を解析します")
        XCTAssertEqual(BalanceLabels.noTeams, "まだ構築がありません。構築ビルダーで作ると解析できます")
        XCTAssertEqual(BalanceLabels.noMembers, "この構築にはポケモンがいません。構築ビルダーで追加してください")
        XCTAssertEqual(BalanceLabels.coverageNoMoves, "どのポケモンにも技が登録されていないため、攻撃範囲は出せません")
        XCTAssertEqual(BalanceLabels.teamLoadFailed, "構築を読み込めません")
    }

    /// 相性の語に総合評価の語を持たない(総合点・ランキング・独自スコアは作らない。type-balance-design.md §1)。
    func testNoOverallScoreWords() {
        let texts = [
            BalanceLabels.defenseText(multiplier: "2", category: .weak), BalanceLabels.coverageText(.double),
            BalanceLabels.summaryText(weak: 1, quadWeak: 0, resist: 0, immune: 0, neutral: 0), BalanceLabels.selectPrompt,
        ]
        for text in texts {
            for word in ["総合", "スコア", "点数", "ランキング", "評価"] {
                XCTAssertFalse(text.contains(word), "\(text) に \(word)")
            }
        }
    }
}
