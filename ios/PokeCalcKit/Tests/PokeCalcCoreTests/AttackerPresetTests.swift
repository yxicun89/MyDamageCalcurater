import XCTest

@testable import PokeCalcCore

/// 自分側のプリセット(ADR-0500 §6・requirements.md「自分側のプリセット」)。
///
/// - A特化 = 関連ステータス SP 32 + 上昇性格 / A振り = SP 32 + 無補正 / 無振り = SP 0 + 無補正。
/// - 関連ステータスは技の分類で決まる: 物理 = atk、特殊 = spa、変化 = atk(ADR-0010 §2 と同じ扱い。
///   変化技はダメージを出さないのでどちらでも結果は変わらないが、逆算・モックの `reverseStat` と規則を揃える)。
/// - 上昇性格は性格一覧(マスタ)から `plus` が関連・`minus` が atk(関連が atk なら spa)の**最初**のもの。
///   無補正は `plus == nil` の**最初**のもの。性格 ID を直書きしない(一覧の順序と中身だけで決まる)。
/// - 該当する性格が一覧に無ければ `PokeCalcError`(code = `PokeCalcError.Code.natureUnavailable`)。
final class AttackerPresetTests: XCTestCase {

    /// CLAUDE.md ドメイン規約: SP は 1 ステータス最大 32。プリセットは振り切り(32)か無振り(0)。
    private let maxStatSP = 32

    // 架空の性格一覧。順序に意味がある(「最初の該当」を確かめるため、囮を先に置く)。
    private let decoyAtkUpDefDown = Nature(id: "tn-atk-def", nameJa: "テスト攻撃上昇防御下降", plus: .atk, minus: .def)
    private let atkUpSpaDown = Nature(id: "tn-atk-spa", nameJa: "テスト攻撃上昇特攻下降", plus: .atk, minus: .spa)
    private let atkUpSpaDownSecond = Nature(id: "tn-atk-spa-2", nameJa: "テスト攻撃上昇特攻下降その2", plus: .atk, minus: .spa)
    private let decoySpaUpDefDown = Nature(id: "tn-spa-def", nameJa: "テスト特攻上昇防御下降", plus: .spa, minus: .def)
    private let spaUpAtkDown = Nature(id: "tn-spa-atk", nameJa: "テスト特攻上昇攻撃下降", plus: .spa, minus: .atk)
    /// plus == minus の無補正(ゲームによっては「同じ能力の上昇と下降」で表す)。plus が nil ではないので
    /// 「無補正 = plus == nil」の規則では選ばれない。
    private let decoySameStat = Nature(id: "tn-same", nameJa: "テスト同じ能力", plus: .def, minus: .def)
    private let neutralFirst = Nature(id: "tn-neutral-1", nameJa: "テスト無補正その1")
    private let neutralSecond = Nature(id: "tn-neutral-2", nameJa: "テスト無補正その2")

    private var fullList: [Nature] {
        [decoyAtkUpDefDown, decoySameStat, atkUpSpaDown, decoySpaUpDefDown, neutralFirst,
         spaUpAtkDown, atkUpSpaDownSecond, neutralSecond]
    }

    private func sp(atk: Int = 0, spa: Int = 0) -> StatBlock {
        StatBlock(hp: 0, atk: atk, def: 0, spa: spa, spd: 0, spe: 0)
    }

    // MARK: - 表示名と順序

    func testCasesAreOrderedAndLabeledAsRequirements() {
        // 並び順は engine/presets/attacker.json(無振り → 特化 → 振り。ADR-0114)。画面のセグメントはこの順に並ぶ。
        // P6-12 で順序だけを JSON に合わせて変えた(旧: A特化 → A振り → 無振り。ADR-0501「P6-12」5章)。文言は変えていない。
        XCTAssertEqual(AttackerPreset.allCases, [.none, .aFull, .aMax])
        XCTAssertEqual(AttackerPreset.allCases.map(\.label), ["無振り", "A特化", "A振り"])
    }

    /// issue #334: `label(for:)` は技の分類で A/C を切り替える(`KnownDefenderPreset.label(for:)` と同じ形)。
    /// Web(`attackerPresetText`)と同じ語: 「A振り」は「A振り(無補正)」にする(ADR-0501「P6-11」)。
    /// 物理・変化は A、特殊は C。「無振り」は分類によらず共通。
    func testLabelForCategoryMatchesWebWording() {
        let cases: [(AttackerPreset, MoveCategory, String)] = [
            (.aFull, .physical, "A特化"),
            (.aFull, .status, "A特化"),
            (.aFull, .special, "C特化"),
            (.aMax, .physical, "A振り(無補正)"),
            (.aMax, .status, "A振り(無補正)"),
            (.aMax, .special, "C振り(無補正)"),
            (.none, .physical, "無振り"),
            (.none, .special, "無振り"),
            (.none, .status, "無振り"),
        ]
        for (preset, category, expected) in cases {
            XCTAssertEqual(preset.label(for: category), expected, "\(preset) / \(category)")
        }
    }

    func testRelevantStatByMoveCategory() {
        let cases: [(MoveCategory, StatKey)] = [
            (.physical, .atk),
            (.special, .spa),
            // 変化技は物理と同じ扱い(ADR-0010 §2。MockPokeCalcService.reverseStat と同じ規則)
            (.status, .atk),
        ]
        for (category, expected) in cases {
            XCTAssertEqual(AttackerPreset.relevantStat(for: category), expected, "\(category)")
        }
    }

    // MARK: - SP と性格の組み立て(テーブル駆動)

    func testBuildTable() throws {
        struct Case {
            let name: String
            let preset: AttackerPreset
            let category: MoveCategory
            let natureId: String
            let sp: StatBlock
        }
        let cases: [Case] = [
            Case(name: "物理 A特化: atk32 + 最初の(+atk,-spa)", preset: .aFull, category: .physical,
                 natureId: atkUpSpaDown.id, sp: sp(atk: maxStatSP)),
            Case(name: "物理 A振り: atk32 + 最初の無補正", preset: .aMax, category: .physical,
                 natureId: neutralFirst.id, sp: sp(atk: maxStatSP)),
            Case(name: "物理 無振り: SP0 + 最初の無補正", preset: .none, category: .physical,
                 natureId: neutralFirst.id, sp: sp()),
            Case(name: "特殊 A特化: spa32 + 最初の(+spa,-atk)", preset: .aFull, category: .special,
                 natureId: spaUpAtkDown.id, sp: sp(spa: maxStatSP)),
            Case(name: "特殊 A振り: spa32 + 最初の無補正", preset: .aMax, category: .special,
                 natureId: neutralFirst.id, sp: sp(spa: maxStatSP)),
            Case(name: "特殊 無振り: SP0 + 最初の無補正", preset: .none, category: .special,
                 natureId: neutralFirst.id, sp: sp()),
            Case(name: "変化 A特化: 物理と同じ(atk32 + (+atk,-spa))", preset: .aFull, category: .status,
                 natureId: atkUpSpaDown.id, sp: sp(atk: maxStatSP)),
            Case(name: "変化 無振り: SP0 + 最初の無補正", preset: .none, category: .status,
                 natureId: neutralFirst.id, sp: sp()),
        ]
        for c in cases {
            let build = try AttackerPreset.build(c.preset, moveCategory: c.category, natures: fullList)
            XCTAssertEqual(build.natureId, c.natureId, c.name)
            XCTAssertEqual(build.sp, c.sp, c.name)
        }
    }

    func testSelectionFollowsListOrderNotIds() throws {
        // 同じ規則でも、一覧の順序を変えると選ばれる性格が変わる(ID を直書きしていないこと)。
        let reordered = [neutralSecond, atkUpSpaDownSecond, atkUpSpaDown, neutralFirst]
        let full = try AttackerPreset.build(.aFull, moveCategory: .physical, natures: reordered)
        XCTAssertEqual(full.natureId, atkUpSpaDownSecond.id)
        let flat = try AttackerPreset.build(.aMax, moveCategory: .physical, natures: reordered)
        XCTAssertEqual(flat.natureId, neutralSecond.id)
    }

    // MARK: - 該当する性格が無い

    func testMissingNatureThrows() async {
        struct Case {
            let name: String
            let preset: AttackerPreset
            let category: MoveCategory
            let natures: [Nature]
        }
        let cases: [Case] = [
            Case(name: "空の一覧", preset: .none, category: .physical, natures: []),
            // plus==minus は無補正と見なさない(plus == nil の規則)
            Case(name: "無補正が無い(A振り)", preset: .aMax, category: .physical,
                 natures: [atkUpSpaDown, decoySameStat]),
            Case(name: "物理の上昇性格が無い(囮の -def だけ)", preset: .aFull, category: .physical,
                 natures: [neutralFirst, decoyAtkUpDefDown, spaUpAtkDown]),
            Case(name: "特殊の上昇性格が無い(囮の -def だけ)", preset: .aFull, category: .special,
                 natures: [neutralFirst, decoySpaUpDefDown, atkUpSpaDown]),
        ]
        for c in cases {
            let error = await assertThrowsPokeCalcError(c.name) {
                try AttackerPreset.build(c.preset, moveCategory: c.category, natures: c.natures)
            }
            XCTAssertEqual(error?.code, PokeCalcError.Code.natureUnavailable, c.name)
            XCTAssertFalse(error?.message.isEmpty ?? true, c.name)
        }
    }

    func testFullPresetDoesNotNeedNeutralNature() throws {
        // A特化は上昇性格だけを使う。無補正が無くても作れる。
        let build = try AttackerPreset.build(.aFull, moveCategory: .special, natures: [spaUpAtkDown])
        XCTAssertEqual(build.natureId, spaUpAtkDown.id)
    }
}
