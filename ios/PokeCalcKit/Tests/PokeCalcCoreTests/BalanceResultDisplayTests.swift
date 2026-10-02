import XCTest

@testable import PokeCalcCore

/// `BalanceResultDisplayBuilder`(P6-26。ADR-0505 §6): 応答を画面の形に整える。
/// 規則: **応答の値(倍率の文字列・分類・集計の数・有効/抜群の真偽)をそのまま運ぶ**。倍率の計算・分類の判定し直し・弱点の偏りの判定・相性表の参照をしない。
/// 並びは応答のまま。名前は送信時点の構築から index で引く(同じ種族の重複を取り違えない)。
final class BalanceResultDisplayTests: XCTestCase {
    private func analyzeRequest(abilityOnFirst: Bool = false) -> BalanceAnalyzeRequest {
        BalanceAnalyzeRequest(
            members: [
                BalanceAnalyzeMember(pokemonId: "9001-000", abilityId: abilityOnFirst ? "stub-ability" : nil),
                BalanceAnalyzeMember(pokemonId: "9001-000"),
                BalanceAnalyzeMember(pokemonId: "9002-000"),
            ])
    }

    private let labels = [
        BalanceMemberLabel(name: "テストなまえ一", abilityName: "テストとくせい"),
        BalanceMemberLabel(name: "テストなまえ二"),
        BalanceMemberLabel(name: "テストなまえ三"),
    ]

    private func defense(abilityOnFirst: Bool = false) -> BalanceDefenseDisplay {
        BalanceResultDisplayBuilder.defense(StubBalance.defense(for: analyzeRequest(abilityOnFirst: abilityOnFirst)), labels: labels)
    }

    // MARK: - 防御相性

    /// 行は応答のメンバーの順(要求の順)。名前は index で引く: 同じ pokemonId(9001-000)の 2 体でも取り違えない。
    func testMemberRowsKeepResponseOrderAndTakeNamesByIndexNotByPokemonId() {
        let display = defense()
        XCTAssertEqual(display.members.map(\.index), [0, 1, 2])
        XCTAssertEqual(display.members.map(\.name), ["テストなまえ一", "テストなまえ二", "テストなまえ三"])
    }

    func testMemberRowCarriesTheTypesFromTheResponse() {
        let display = defense()
        let response = StubBalance.defense(for: analyzeRequest())
        XCTAssertEqual(display.members.map(\.types), response.members.map(\.types), "タイプはサーバーが返した値(iOS は引かない)")
    }

    func testEachMemberHas18CellsInCanonicalTypeOrder() {
        for member in defense().members {
            XCTAssertEqual(member.cells.map(\.attackType), PokeType.allCases)
        }
    }

    /// 倍率は応答の文字列のまま「×4 弱点」。メンバーごとに違う値(行の取り違えを検出)。
    func testCellTextShowsTheServerMultiplierAndCategoryWord() {
        let display = defense()
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 0]?.text, "×4 弱点")
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 0]?.category, .quadWeak)
        XCTAssertEqual(display.members[safe: 1]?.cells[safe: 0]?.text, "×2 弱点")
        XCTAssertEqual(display.members[safe: 2]?.cells[safe: 0]?.text, "×1/2 耐性")
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 2]?.text, "×1 等倍")
    }

    /// 特性が変えた欄だけ添え書きが付く(色だけにしない)。特性名は要求時点のラベルから。特性の無いメンバーは添え書きなし。
    func testAbilityChangedCellsGetANoteAndOthersDoNot() {
        let display = defense(abilityOnFirst: true)
        guard let first = display.members[safe: 0] else { return XCTFail("行が無い") }
        XCTAssertEqual(first.abilityName, "テストとくせい")
        XCTAssertEqual(first.cells[safe: 1]?.text, "×0 無効")
        XCTAssertEqual(first.cells[safe: 1]?.abilityNote, "特性で無効")
        XCTAssertNil(first.cells[safe: 0]?.abilityNote, "type 由来の欄に添え書きは付かない")
        XCTAssertNil(display.members[safe: 1]?.abilityName)
        XCTAssertEqual(display.members[safe: 1]?.cells.count, 18)
        XCTAssertEqual(display.members[safe: 1]?.cells.allSatisfy { $0.abilityNote == nil }, true)
    }

    /// 分類・倍率は応答のまま運ぶ(iOS で判定し直さない): 倍率 "1" なのに category が weak の応答は「×1 弱点」のまま出す。
    func testCategoryIsNeverRecomputedFromTheMultiplier() {
        var response = StubBalance.defense(for: analyzeRequest())
        response.members[2].defense[5] = BalanceDefenseEntry(attackType: .ice, multiplier: "1", category: .weak)
        let display = BalanceResultDisplayBuilder.defense(response, labels: labels)
        XCTAssertEqual(display.members[safe: 2]?.cells[safe: 5]?.text, "×1 弱点")
        XCTAssertEqual(display.members[safe: 2]?.cells[safe: 5]?.category, .weak)
    }

    func testSummaryRowsKeepServerCountsAndOrder() {
        let display = defense()
        XCTAssertEqual(display.summary.map(\.attackType), PokeType.allCases)
        guard let first = display.summary[safe: 0] else { return XCTFail("行が無い") }
        // メンバー 0〜2 の normal は quad_weak・weak・resist(StubBalance.defense)。
        XCTAssertEqual([first.weak, first.quadWeak, first.resist, first.immune, first.neutral], [2, 1, 1, 0, 0])
        XCTAssertEqual(first.text, "弱点 2(うち×4 1)・耐性 1・無効 0・等倍 0")
    }

    /// 集計は応答の数をそのまま出す(メンバーの分類から数え直さない。弱点の偏りの判定もしない)。
    func testSummaryIsNotRecountedFromTheMembers() {
        var response = StubBalance.defense(for: analyzeRequest())
        response.teamSummary[0] = BalanceTeamSummaryEntry(attackType: .normal, weak: 5, quadWeak: 4, resist: 0, immune: 0, neutral: 0)
        let display = BalanceResultDisplayBuilder.defense(response, labels: labels)
        XCTAssertEqual(display.summary[safe: 0]?.weak, 5)
        XCTAssertEqual(display.summary[safe: 0]?.text, "弱点 5(うち×4 4)・耐性 0・無効 0・等倍 0")
    }

    /// 並び替えない(サーバーの正準順のまま。弱点の多い順などに並べ替えない)。
    func testSummaryIsNotSortedByCounts() {
        var response = StubBalance.defense(for: analyzeRequest())
        response.teamSummary.reverse()
        let display = BalanceResultDisplayBuilder.defense(response, labels: labels)
        XCTAssertEqual(display.summary.map(\.attackType), PokeType.allCases.reversed(), "応答の順のまま")
    }

    /// ラベルが足りない index は名前を捏造せず、応答の pokemonId を使う。
    func testMissingLabelFallsBackToThePokemonId() {
        let display = BalanceResultDisplayBuilder.defense(StubBalance.defense(for: analyzeRequest()), labels: [labels[0]])
        XCTAssertEqual(display.members.map(\.name), ["テストなまえ一", "9001-000", "9002-000"])
    }

    // MARK: - 攻撃範囲

    private func coverageRequest() -> BalanceCoverageRequest {
        BalanceCoverageRequest(
            members: [
                BalanceCoverageMember(pokemonId: "9001-000", moveIds: ["stub-move-a"]),
                BalanceCoverageMember(pokemonId: "9001-000", moveIds: []),
                BalanceCoverageMember(pokemonId: "9002-000", moveIds: ["stub-move-a", "stub-move-b"]),
            ])
    }

    private func coverage() -> BalanceCoverageDisplay {
        BalanceResultDisplayBuilder.coverage(StubBalance.coverage(for: coverageRequest()), labels: labels)
    }

    func testCoverageMemberRowsKeepOrderAndNames() {
        let display = coverage()
        XCTAssertEqual(display.members.map(\.index), [0, 1, 2])
        XCTAssertEqual(display.members.map(\.name), ["テストなまえ一", "テストなまえ二", "テストなまえ三"])
        for member in display.members {
            XCTAssertEqual(member.cells.map(\.defenseType), PokeType.allCases)
        }
    }

    /// 攻撃技を持つメンバー(スタブでは i が偶数)は「×2 抜群」・奇数(技あり)は「×1/2 いまひとつ」・技なしは「攻撃技なし」。応答の値のまま。
    func testCoverageCellTextFollowsTheResponse() {
        let display = coverage()
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 0]?.text, "×2 抜群")
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 0]?.effective, true)
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 0]?.superEffective, true)
        XCTAssertEqual(display.members[safe: 1]?.cells[safe: 0]?.text, "攻撃技なし")
        XCTAssertEqual(display.members[safe: 1]?.cells[safe: 0]?.effective, false)
        XCTAssertEqual(display.members[safe: 1]?.cells[safe: 0]?.superEffective, false)
    }

    func testCoverageMemberWithoutAttackMoveIsMarked() {
        let display = coverage()
        XCTAssertEqual(display.members.map(\.hasAttackMove), [true, false, true])
        XCTAssertEqual(display.members[safe: 0]?.attackTypes, [.fire])
        XCTAssertEqual(display.members[safe: 1]?.attackTypes, [])
    }

    func testCoverageTeamRowsShowTheBestMultiplierAndMemberCounts() {
        let display = coverage()
        XCTAssertEqual(display.team.map(\.defenseType), PokeType.allCases)
        guard let first = display.team[safe: 0] else { return XCTFail("行が無い") }
        XCTAssertEqual(first.text, "×2 抜群")
        XCTAssertEqual([first.effectiveMembers, first.superEffectiveMembers], [2, 2])
        XCTAssertEqual(first.membersText, "有効 2体・抜群 2体")
    }

    func testCoverageTeamRowWithNoAttackMoveAnywhere() {
        let request = BalanceCoverageRequest(members: [BalanceCoverageMember(pokemonId: "9001-000", moveIds: [])])
        let display = BalanceResultDisplayBuilder.coverage(StubBalance.coverage(for: request), labels: [labels[0]])
        XCTAssertEqual(display.team.count, 18)
        XCTAssertTrue(display.team.allSatisfy { $0.text == "攻撃技なし" })
        XCTAssertTrue(display.team.allSatisfy { $0.effectiveMembers == 0 && $0.superEffectiveMembers == 0 })
    }

    /// 有効・抜群の真偽は応答のまま(倍率から導かない)。
    func testEffectiveFlagsAreNotDerivedFromTheMultiplier() {
        var response = StubBalance.coverage(for: coverageRequest())
        response.members[0].coverage[3] = BalanceDefenseCoverageEntry(defenseType: .electric, bestMultiplier: .half, effective: true, superEffective: false)
        let display = BalanceResultDisplayBuilder.coverage(response, labels: labels)
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 3]?.text, "×1/2 いまひとつ")
        XCTAssertEqual(display.members[safe: 0]?.cells[safe: 3]?.effective, true, "応答が有効と言えば有効(iOS で再判定しない)")
    }
}
