import XCTest

@testable import PokeCalcCore

/// I-ios-21(ADR-0542・ADR-0501「素早さ画面から連続技へ」AC-h1〜AC-h3): 素早さ画面の値 → 連続技の「自分の素早さ」の写し方と、
/// ルート(`AppNavigationModel`)の「連続技を開く」要求の token。どちらも純粋なロジック。
///
/// 形(implementer はこの名前で作る):
/// - `ComboOpenRequest(token:handoff:)`(Equatable)
/// - `ComboHandoffPlan.selfSpeedInput(for:) -> ComboSelfSpeedInput`・`nature(_:) -> ComboSelfSpeedNature`・`scarfIgnored(for:) -> Bool`
/// - `AppNavigationModel.comboOpenRequest: ComboOpenRequest?`・`mutating openCombo(_:featureID:)`
final class ComboHandoffPlanTests: XCTestCase {

    private func handoff(
        attacker: String = "9003-000", defender: String = "9004-000", speed: SpeedInvestment? = nil, rank: Int = 0,
        scarf: Bool = false
    ) -> SpeedComboHandoff {
        SpeedComboHandoff(
            attackerPokemonID: attacker, defenderPokemonID: defender, attackerSpeed: speed, attackerRank: rank, attackerScarf: scarf)
    }

    // MARK: - AC-h3: 自分の素早さへの写し方

    /// 素早さ画面の性格補正は 3 択。連続技の「計算画面の性格のまま」には写さない(Web の plus/neutral/minus → 上がる/補正なし/下がる)。
    func testNatureMapsOneToOne() {
        XCTAssertEqual(ComboHandoffPlan.nature(.plus), .plus)
        XCTAssertEqual(ComboHandoffPlan.nature(.neutral), .neutral)
        XCTAssertEqual(ComboHandoffPlan.nature(.minus), .minus)
    }

    /// 今の振りがあるとき: SP・性格補正・ランクをすべて入れる。
    func testCurrentInvestmentFillsSPNatureAndRank() {
        let input = ComboHandoffPlan.selfSpeedInput(for: handoff(speed: SpeedInvestment(sp: 20, nature: .plus), rank: 1))
        XCTAssertEqual(input, ComboSelfSpeedInput(sp: 20, nature: .plus, rank: 1))
    }

    /// 今の振りが無いとき: SP・性格補正は未入力(個体の値・計算画面の性格のまま)。ランクは素早さ画面に常にある値なので入れる。
    func testWithoutCurrentInvestmentOnlyTheRankIsFilled() {
        XCTAssertEqual(
            ComboHandoffPlan.selfSpeedInput(for: handoff(speed: nil, rank: -1)), ComboSelfSpeedInput(sp: nil, nature: .keep, rank: -1))
        XCTAssertEqual(
            ComboHandoffPlan.selfSpeedInput(for: handoff(speed: nil, rank: 0)), ComboSelfSpeedInput(sp: nil, nature: .keep, rank: 0),
            "±0 も素早さ画面で見えている値として入れる(前の引き継ぎ・手で入れたランクを残さない)")
    }

    func testZeroSPAndNeutralAreFilledNotTreatedAsEmpty() {
        let input = ComboHandoffPlan.selfSpeedInput(for: handoff(speed: SpeedInvestment(sp: 0, nature: .neutral), rank: 0))
        XCTAssertEqual(input, ComboSelfSpeedInput(sp: 0, nature: .neutral, rank: 0), "無振り・補正なしも入力として渡す(未入力と区別する)")
    }

    /// スカーフは連続技に指定する項目が無い(API の ComboRequest に無い)。反映せず、指定していたことだけを知らせる。
    func testScarfIsNeverAppliedButIsReported() {
        let withScarf = handoff(speed: SpeedInvestment(sp: 32, nature: .plus), rank: 0, scarf: true)
        XCTAssertTrue(ComboHandoffPlan.scarfIgnored(for: withScarf))
        XCTAssertEqual(
            ComboHandoffPlan.selfSpeedInput(for: withScarf), ComboSelfSpeedInput(sp: 32, nature: .plus, rank: 0),
            "スカーフは SP・ランクに混ぜない(×1.5 をランクや SP に置き換えない)")
        XCTAssertTrue(ComboHandoffPlan.scarfIgnored(for: handoff(speed: nil, scarf: true)), "今の振りが無くてもスカーフは反映できない")
        XCTAssertFalse(ComboHandoffPlan.scarfIgnored(for: handoff(speed: SpeedInvestment(sp: 32, nature: .plus), scarf: false)))
    }

    // MARK: - AC-h1: ルートの token

    private let specs = [
        FeatureSpec(id: "calc", order: 100, tab: .damage),
        FeatureSpec(id: "reverse", order: 200, tab: .damage),
        FeatureSpec(id: "team", order: 300, tab: .team),
        FeatureSpec(id: "speed", order: 600, tab: .speed),
        FeatureSpec(id: "favorites", order: 800, tab: .records),
        FeatureSpec(id: "about", order: 10_000, isToolbarEntry: true, availableWithoutServices: true),
    ]

    func testNoRequestAtStart() {
        XCTAssertNil(AppNavigationModel(specs: specs).comboOpenRequest)
    }

    func testOpenComboPlacesARequestAndMovesToTheCalcSegment() {
        var model = AppNavigationModel(specs: specs)
        model.select(featureID: "reverse")
        model.select(tab: .speed)
        let sent = handoff(speed: SpeedInvestment(sp: 32, nature: .neutral), rank: 1)
        model.openCombo(sent, featureID: "calc")
        XCTAssertEqual(model.comboOpenRequest, ComboOpenRequest(token: 1, handoff: sent))
        XCTAssertEqual(model.selectedTab, .damage)
        XCTAssertEqual(model.currentFeatureID, "calc", "ダメージタブで逆算を見ていても計算の区切りへ移る")
        XCTAssertTrue(model.visitedFeatureIDs.contains("calc"))
        XCTAssertTrue(model.visitedFeatureIDs.contains("speed"), "素早さ画面は破棄しない(入力が残る)")
    }

    /// 同じ値でも押すたびに token が進む(計算画面で入力を変えたあと、もう一度押すと入れ直せる)。
    func testTokenAdvancesOnEveryOpenEvenWithTheSameHandoff() {
        var model = AppNavigationModel(specs: specs)
        let sent = handoff()
        model.openCombo(sent, featureID: "calc")
        model.openCombo(sent, featureID: "calc")
        XCTAssertEqual(model.comboOpenRequest?.token, 2)
        let other = handoff(attacker: "9001-000")
        model.openCombo(other, featureID: "calc")
        XCTAssertEqual(model.comboOpenRequest, ComboOpenRequest(token: 3, handoff: other), "最後の要求だけを持つ")
    }

    /// タブ・区切りを移っても要求(token)は変わらない = 計算画面は入れ直さない(タブを戻っても入力が壊れない)。
    func testSwitchingTabsDoesNotChangeTheRequest() {
        var model = AppNavigationModel(specs: specs)
        model.openCombo(handoff(), featureID: "calc")
        let request = model.comboOpenRequest
        model.select(tab: .speed)
        model.select(tab: .damage)
        model.select(featureID: "reverse")
        model.select(featureID: "calc")
        XCTAssertEqual(model.comboOpenRequest, request)
    }

    func testOpenComboIgnoresAFeatureOutsideTheTabs() {
        var model = AppNavigationModel(specs: specs)
        model.select(tab: .speed)
        model.openCombo(handoff(), featureID: "about")
        XCTAssertNil(model.comboOpenRequest, "タブに属さない画面には渡さない")
        XCTAssertEqual(model.selectedTab, .speed, "移らない")
        model.openCombo(handoff(), featureID: "unknown")
        XCTAssertNil(model.comboOpenRequest)
    }

    func testOpenComboKeepsOtherTabsSegments() {
        var model = AppNavigationModel(specs: specs)
        model.select(tab: .team)
        model.openCombo(handoff(), featureID: "calc")
        XCTAssertEqual(model.selectedFeatureID(in: .team), "team")
        XCTAssertEqual(model.selectedFeatureID(in: .speed), "speed")
    }
}
