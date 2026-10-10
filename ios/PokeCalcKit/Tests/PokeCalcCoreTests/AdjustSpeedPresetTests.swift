import XCTest

@testable import PokeCalcCore

/// F-11: 素早さの目標の相手の振り方(ADR-0331 §5 の表)。性格はマスタの一覧から引く。
final class AdjustSpeedPresetTests: XCTestCase {
    private let natures = [
        Nature(id: "n-neutral", nameJa: "無補正"),
        Nature(id: "n-spe-spa", nameJa: "素早さ上昇・特攻下降", plus: .spe, minus: .spa),
        Nature(id: "n-spe-atk", nameJa: "素早さ上昇・攻撃下降", plus: .spe, minus: .atk),
    ]

    func testOrderAndLabelsFollowWeb() {
        XCTAssertEqual(AdjustSpeedPreset.allCases, [.fastest, .neutralMax, .none])
        XCTAssertEqual(AdjustSpeedPreset.allCases.map(\.label), ["最速", "準速", "無振り"])
        XCTAssertEqual(AdjustSpeedPreset.defaultPreset, .fastest)
    }

    func testBuildsSPAndNature() throws {
        let fastest = try AdjustSpeedPreset.fastest.build(natures: natures)
        XCTAssertEqual(fastest.natureId, "n-spe-atk", "素早さ上昇・攻撃下降を優先")
        XCTAssertEqual(fastest.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 32))
        let neutralMax = try AdjustSpeedPreset.neutralMax.build(natures: natures)
        XCTAssertEqual(neutralMax.natureId, "n-neutral")
        XCTAssertEqual(neutralMax.sp.spe, 32)
        let none = try AdjustSpeedPreset.none.build(natures: natures)
        XCTAssertEqual(none.natureId, "n-neutral")
        XCTAssertEqual(none.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
    }

    func testFastestFallsBackToAnySpeedUpNatureAndThrowsWhenMissing() throws {
        let only = [Nature(id: "n-neutral", nameJa: "無補正"), Nature(id: "n-spe-spa", nameJa: "x", plus: .spe, minus: .spa)]
        XCTAssertEqual(try AdjustSpeedPreset.fastest.build(natures: only).natureId, "n-spe-spa")
        XCTAssertThrowsError(try AdjustSpeedPreset.fastest.build(natures: [natures[0]])) { error in
            XCTAssertEqual((error as? PokeCalcError)?.code, PokeCalcError.Code.natureUnavailable)
        }
        XCTAssertThrowsError(try AdjustSpeedPreset.none.build(natures: [natures[1]]), "無補正が無ければ別の性格で代えない")
    }
}
