import XCTest

@testable import PokeCalcCore

// BalanceLabels / BalanceErrorText / BalanceScreenError(ADR-0415。docs/type-balance-design.md §10、
// Web の web/src/domain/balanceLabels.ts・web/src/i18n/ja.ts の balanceErrorText と同じ文言)。
//
// 決めた形:
// - `enum BalanceLabels`(Foundation のみ):
//   - `defenseLabel(multiplier: String, category: BalanceDefenseCategory) -> String` = 「×<倍率> <語>」。
//     倍率の文字列は balance-svc の応答(DefenseMultiplier)をそのまま使い、iOS で計算し直さない。語は category から引く
//     (弱点: quad_weak/weak、等倍: neutral、耐性: resist/quad_resist、無効: immune)。値の範囲から語を判定し直さない。
//   - `coverageLabel(_ multiplier: BalanceCoverageMultiplier?) -> String` = 「×2 抜群」「×1 等倍」「×1/2 いまひとつ」「×0 無効」、
//     nil は「攻撃技なし」。
//   - `multiplierLabel(_ multiplier: String) -> String` = 「×<倍率>」(語なし)。
// - `enum BalanceScreenText`: 表の見出しなどの固定文言(Web の balanceScreenText と同じ語)。
// - `enum BalanceErrorText { static func message(forCode: String) -> String }`: balance の ErrorCode → 日本語。
//   サーバーの英語の `message` は画面に出さない(ADR-0411 §3 と同じ)。未知のコードは fallback。
// - `struct BalanceScreenError: Equatable, Sendable { init(_ error: any Error); var code: String; var message: String }`:
//   `PokeCalcError`(transport/decode)→ `balance_unavailable`、サービスのコード → そのコード、PokeCalcError 以外 → fallback。
//   `message` は常に `BalanceErrorText` から引く(サーバーの message を使わない)。
final class BalanceLabelsTests: XCTestCase {

    // MARK: - 防御倍率(設計 §10 の書式)

    func testDefenseLabelFollowsDesignFormat() {
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "4", category: .quadWeak), "×4 弱点")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "2", category: .weak), "×2 弱点")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "1", category: .neutral), "×1 等倍")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "1/2", category: .resist), "×1/2 耐性")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "1/4", category: .quadResist), "×1/4 耐性")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "0", category: .immune), "×0 無効")
    }

    /// 特性が入ると 3/4・3/2・5/4 のような中間値が来る(ADR-0017 §3)。語は応答の category に従い、倍率から判定し直さない。
    func testDefenseLabelUsesCategoryNotMultiplierValue() {
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "3/4", category: .resist), "×3/4 耐性")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "3/2", category: .weak), "×3/2 弱点")
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "5/4", category: .weak), "×5/4 弱点")
        // category が応答どおりなら、倍率の値が語と矛盾して見えても語を変えない(再判定しない)。
        XCTAssertEqual(BalanceLabels.defenseLabel(multiplier: "1", category: .weak), "×1 弱点")
    }

    func testEveryDefenseCategoryHasAWord() {
        for category in BalanceDefenseCategory.allCases {
            XCTAssertTrue(
                BalanceLabels.defenseLabel(multiplier: "1", category: category).hasPrefix("×1 "),
                "category \(category) に語が無い"
            )
        }
    }

    func testDefenseCategoryRawValuesMatchContract() {
        // openapi `DefenseCategory` の enum 文字列(デコードの写像が依存する)。
        XCTAssertEqual(
            Set(BalanceDefenseCategory.allCases.map(\.rawValue)),
            ["quad_weak", "weak", "neutral", "resist", "quad_resist", "immune"]
        )
    }

    // MARK: - 攻撃範囲の倍率

    func testCoverageLabel() {
        XCTAssertEqual(BalanceLabels.coverageLabel(.double), "×2 抜群")
        XCTAssertEqual(BalanceLabels.coverageLabel(.neutral), "×1 等倍")
        XCTAssertEqual(BalanceLabels.coverageLabel(.half), "×1/2 いまひとつ")
        XCTAssertEqual(BalanceLabels.coverageLabel(.zero), "×0 無効")
        XCTAssertEqual(BalanceLabels.coverageLabel(nil), "攻撃技なし")
    }

    func testCoverageMultiplierRawValuesMatchContract() {
        XCTAssertEqual(
            Set(BalanceCoverageMultiplier.allCases.map(\.rawValue)),
            ["0", "1/2", "1", "2"]
        )
    }

    func testMultiplierLabelHasNoWord() {
        XCTAssertEqual(BalanceLabels.multiplierLabel("3/2"), "×3/2")
    }

    // MARK: - 表の見出し(Web と同じ語)

    func testScreenTextMatchesWeb() {
        XCTAssertEqual(BalanceScreenText.defenseTableLabel, "防御相性")
        XCTAssertEqual(BalanceScreenText.teamSummaryTableLabel, "チームの集計")
        XCTAssertEqual(BalanceScreenText.coverageTableLabel, "攻撃範囲")
        XCTAssertEqual(BalanceScreenText.attackTypeColumnLabel, "攻撃タイプ")
        XCTAssertEqual(BalanceScreenText.weakColumnLabel, "弱点")
        XCTAssertEqual(BalanceScreenText.quadWeakColumnLabel, "うち×4")
        XCTAssertEqual(BalanceScreenText.resistColumnLabel, "耐性")
        XCTAssertEqual(BalanceScreenText.immuneColumnLabel, "無効")
        XCTAssertEqual(BalanceScreenText.neutralColumnLabel, "等倍")
        XCTAssertEqual(BalanceScreenText.defenseTypeColumnLabel, "防御タイプ")
        XCTAssertEqual(BalanceScreenText.bestMultiplierColumnLabel, "最大倍率")
        XCTAssertEqual(BalanceScreenText.effectiveColumnLabel, "有効")
        XCTAssertEqual(BalanceScreenText.superEffectiveColumnLabel, "抜群")
        XCTAssertEqual(BalanceScreenText.loadingNotice, "計算中")
        XCTAssertEqual(BalanceScreenText.noneLabel, "なし")
    }

    // MARK: - エラー文言(Web の balanceErrorText と同じ。サーバーの英語 message は出さない)

    func testErrorTextByCode() {
        XCTAssertEqual(BalanceErrorText.message(forCode: "invalid_request"), "入力の内容が正しくありません。見直してください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "request_too_large"), "入力が大きすぎます。メンバーや技を減らしてください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "unknown_pokemon"), "選んだポケモンがサーバーのデータにありません。選び直してください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "unknown_move"), "選んだ技がサーバーのデータにありません。選び直してください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "unknown_ability"), "選んだ特性がサーバーのデータにありません。選び直してください")
        XCTAssertEqual(
            BalanceErrorText.message(forCode: "master_unavailable"),
            "サーバーのデータを読み込めません。しばらくしてからもう一度お試しください"
        )
        XCTAssertEqual(BalanceErrorText.message(forCode: "overloaded"), "サーバーが混み合っています。しばらくしてからもう一度お試しください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "internal_error"), "サーバーでエラーが起きました。しばらくしてからもう一度お試しください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "balance_unavailable"), "タイプバランスのサーバーに接続できません")
    }

    /// 端末 ID・セッション ID の不備(balance 0.8.0 の `missing_header`/`invalid_header`、0.7.0 の `missing_request_context`)。
    /// Web の文言の「ページ」はアプリでは「アプリ」に替える(これだけが Web との差)。3つとも空でない日本語で、英語のコードを出さない。
    func testHeaderErrorCodesMapToJapaneseText() {
        for code in ["missing_header", "invalid_header", "missing_request_context"] {
            let text = BalanceErrorText.message(forCode: code)
            XCTAssertTrue(text.contains("端末の情報"), "\(code): \(text)")
            XCTAssertTrue(text.contains("アプリを開き直してください"), "\(code): \(text)")
            XCTAssertFalse(text.contains(code), "英語のコードをそのまま出さない: \(text)")
        }
    }

    func testUnknownCodeFallsBackToGenericText() {
        let fallback = "タイプバランスを計算できませんでした。しばらくしてからもう一度お試しください"
        XCTAssertEqual(BalanceErrorText.message(forCode: "some_future_code"), fallback)
        XCTAssertEqual(BalanceErrorText.message(forCode: ""), fallback)
    }

    // MARK: - BalanceScreenError

    func testScreenErrorMapsTransportAndDecodeToUnavailable() {
        let transport = BalanceScreenError(PokeCalcError(code: PokeCalcError.Code.transport, message: "URLError"))
        XCTAssertEqual(transport.code, "balance_unavailable")
        XCTAssertEqual(transport.message, "タイプバランスのサーバーに接続できません")

        let decode = BalanceScreenError(PokeCalcError(code: PokeCalcError.Code.decode, message: "typeMismatch"))
        XCTAssertEqual(decode.code, "balance_unavailable")
    }

    func testScreenErrorKeepsServerCodeAndIgnoresServerMessage() {
        let error = BalanceScreenError(PokeCalcError(code: "unknown_move", message: "unknown moveId: move-9001"))
        XCTAssertEqual(error.code, "unknown_move")
        XCTAssertEqual(error.message, BalanceErrorText.message(forCode: "unknown_move"))
        XCTAssertFalse(error.message.contains("moveId"), "サーバーの内部メッセージを画面に出さない")
    }

    func testScreenErrorForNonPokeCalcErrorUsesFallback() {
        struct Other: Error {}
        let error = BalanceScreenError(Other())
        XCTAssertEqual(error.message, BalanceErrorText.message(forCode: "no_such_code"))
    }
}
