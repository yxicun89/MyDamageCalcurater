import XCTest

@testable import PokeCalcCore

/// 逆算画面「受けたダメージ」(side = attacker)で自分 = 既知の防御側を作るプリセット(P6-2b)。
///
/// ADR-0009 のカタログから、H の有無で迷わない3つだけを選ぶ: 無振り(`none`)/ HB(HD)振り(`hb`/`hd`)/
/// HB(HD)特化(`hb_full`/`hd_full`)。関連ステータスは相手の技の分類で決まる(物理・変化 = def、特殊 = spd)。
/// 既定は `allCases` の最初 = 無振り(Web の P4-4 が既知の防御側を SP 0・無補正に固定しているのと既定をそろえる。ADR-0300 §7)。
final class KnownDefenderPresetTests: XCTestCase {

    /// CLAUDE.md ドメイン規約: SP は 1 ステータス最大 32。
    private let maxStatSP = 32

    private let natures = StubMaster.reverseNatures

    func testCasesAndOrder() {
        XCTAssertEqual(KnownDefenderPreset.allCases, [.none, .max, .full])
        XCTAssertEqual(KnownDefenderPreset.allCases.map(\.rawValue), ["none", "max", "full"])
    }

    func testLabelsDependOnMoveCategory() {
        let cases: [(KnownDefenderPreset, MoveCategory, String)] = [
            (.none, .physical, "無振り"),
            (.none, .special, "無振り"),
            (.max, .physical, "HB振り"),
            (.max, .special, "HD振り"),
            (.max, .status, "HB振り"),
            (.full, .physical, "HB特化"),
            (.full, .special, "HD特化"),
        ]
        for (preset, category, expected) in cases {
            XCTAssertEqual(preset.label(for: category), expected, "\(preset) / \(category)")
        }
    }

    func testRelevantStat() {
        XCTAssertEqual(KnownDefenderPreset.relevantStat(for: .physical), .def)
        XCTAssertEqual(KnownDefenderPreset.relevantStat(for: .status), .def)
        XCTAssertEqual(KnownDefenderPreset.relevantStat(for: .special), .spd)
    }

    func testBuild() throws {
        let zero = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        let hb = StatBlock(hp: maxStatSP, atk: 0, def: maxStatSP, spa: 0, spd: 0, spe: 0)
        let hd = StatBlock(hp: maxStatSP, atk: 0, def: 0, spa: 0, spd: maxStatSP, spe: 0)
        let cases: [(KnownDefenderPreset, MoveCategory, DefenderBuild)] = [
            (.none, .physical, DefenderBuild(natureId: StubMaster.neutralNature.id, sp: zero)),
            (.none, .special, DefenderBuild(natureId: StubMaster.neutralNature.id, sp: zero)),
            (.max, .physical, DefenderBuild(natureId: StubMaster.neutralNature.id, sp: hb)),
            (.max, .special, DefenderBuild(natureId: StubMaster.neutralNature.id, sp: hd)),
            // ADR-0009: HB特化 = +def / -atk、HD特化 = +spd / -atk
            (.full, .physical, DefenderBuild(natureId: StubMaster.defUpNature.id, sp: hb)),
            (.full, .special, DefenderBuild(natureId: StubMaster.spdUpNature.id, sp: hd)),
        ]
        for (preset, category, expected) in cases {
            let build = try KnownDefenderPreset.build(preset, moveCategory: category, natures: natures)
            XCTAssertEqual(build, expected, "\(preset) / \(category)")
        }
    }

    func testBuildUsesFirstMatchingNatureInListOrder() throws {
        // 性格 ID を直書きしない: 一覧の順で最初に条件を満たすものを選ぶ。
        let another = Nature(id: "stub-nature-def-up-2", nameJa: "テストせいかく防御上昇2", plus: .def, minus: .atk)
        let build = try KnownDefenderPreset.build(.full, moveCategory: .physical, natures: [another] + natures)
        XCTAssertEqual(build.natureId, another.id)
    }

    func testBuildFailsWhenNatureMissing() async {
        let onlyNeutral = [StubMaster.neutralNature]
        let error = await assertThrowsPokeCalcError("上昇性格が無い") {
            try KnownDefenderPreset.build(.full, moveCategory: .physical, natures: onlyNeutral)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.natureUnavailable)

        let noNeutral = [StubMaster.defUpNature]
        let neutralError = await assertThrowsPokeCalcError("無補正が無い") {
            try KnownDefenderPreset.build(.none, moveCategory: .physical, natures: noNeutral)
        }
        XCTAssertEqual(neutralError?.code, PokeCalcError.Code.natureUnavailable)
    }
}
