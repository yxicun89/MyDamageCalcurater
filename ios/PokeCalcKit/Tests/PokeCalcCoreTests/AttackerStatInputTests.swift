import XCTest

@testable import PokeCalcCore

/// 攻撃側の「攻撃」「特攻」2ブロック入力の純粋関数(ADR-0518。Web の `attackerStatInputs.test.ts` と同じ規則)。
///
/// 規則の正は ADR-0518 §3〜§5。性格は**マスタの一覧から**解決する(実在しない性格は要求に載せない)。
/// プリセットの値は `AttackerPreset.build`(`AttackerPresetCatalogContractTests` が JSON と突き合わせている)と
/// 一致させる(別の正を作らない)。
final class AttackerStatInputTests: XCTestCase {

    private func sp(atk: Int = 0, spa: Int = 0) -> StatBlock {
        StatBlock(hp: 0, atk: atk, def: 0, spa: spa, spd: 0, spe: 0)
    }

    private func input(_ text: String, _ modifier: NatureChoice) -> AttackStatInput {
        AttackStatInput(spText: text, modifier: modifier)
    }

    private func inputs(atk: AttackStatInput, spa: AttackStatInput) -> AttackerStatInputs {
        AttackerStatInputs(atk: atk, spa: spa)
    }

    /// 計算画面の既定のスタブ性格(無補正・+atk/-spa・+spa/-atk)。
    private let stubNatures = [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature]

    // MARK: - 既定・列挙の順

    func testDefaultInputsAreZeroAndNeutralOnBothBlocks() {
        XCTAssertEqual(AttackerStatInputs.default.atk, input("0", .neutral))
        XCTAssertEqual(AttackerStatInputs.default.spa, input("0", .neutral))
    }

    func testAttackStatAndChoiceOrderMatchTheScreenOrder() {
        XCTAssertEqual(AttackStat.allCases, [.atk, .spa], "攻撃 → 特攻")
        XCTAssertEqual(NatureChoice.allCases, [.up, .neutral, .down], "上昇 → 補正なし → 下降")
    }

    func testSubscriptReadsAndWritesEachBlock() {
        var value = AttackerStatInputs.default
        value[.spa] = input("5", .down)
        XCTAssertEqual(value.spa, input("5", .down))
        XCTAssertEqual(value.atk, input("0", .neutral), "もう一方は変わらない")
        XCTAssertEqual(value[.spa], input("5", .down))
    }

    // MARK: - 技の分類が使うブロック

    func testAttackStatForCategory() {
        let cases: [(MoveCategory?, AttackStat?)] = [
            (.physical, .atk), (.status, .atk), (.special, .spa), (nil, nil),
        ]
        for (category, expected) in cases {
            XCTAssertEqual(AttackerStatRules.attackStat(for: category), expected, "\(String(describing: category))")
        }
    }

    // MARK: - SP の1欄の読み方(丸めず、不正は nil)

    func testParseSPAcceptsOnlyDecimalIntegersZeroToThirtyTwo() {
        let valid: [(String, Int)] = [
            ("0", 0), ("1", 1), ("32", 32), ("007", 7), ("032", 32),
            ("", 0), ("   ", 0), (" 12 ", 12), ("\t5\n", 5),
        ]
        for (text, expected) in valid {
            XCTAssertEqual(AttackerStatRules.parseSP(text), expected, "「\(text)」")
        }
        let invalid = [
            "33", "99", "-1", "+5", "3.0", "1e1", "0x10", "12a", "a", "1,0", "1 0",
            "３", "１２",  // 全角
            "999999999999999999999",  // 桁あふれ
        ]
        for text in invalid {
            XCTAssertNil(AttackerStatRules.parseSP(text), "「\(text)」は不正(丸めない)")
        }
    }

    // MARK: - プリセット ⇔ ブロックの値

    func testPresetInputValues() {
        XCTAssertEqual(AttackerStatRules.presetInput(.none), input("0", .neutral))
        XCTAssertEqual(AttackerStatRules.presetInput(.aFull), input("32", .up))
        XCTAssertEqual(AttackerStatRules.presetInput(.aMax), input("32", .neutral))
    }

    func testMatchingPresetIsDerivedFromTheBlockValue() {
        let cases: [(AttackStatInput, AttackerPreset?)] = [
            (input("0", .neutral), AttackerPreset.none),
            (input("", .neutral), AttackerPreset.none),  // 空欄は 0
            (input("32", .up), .aFull),
            (input("032", .up), .aFull),  // 数値で比較する
            (input("32", .neutral), .aMax),
            (input("31", .up), nil),  // 数値だけ違う → カスタム
            (input("0", .up), nil),  // 補正だけ違う → カスタム
            (input("32", .down), nil),
            (input("abc", .neutral), nil),  // 不正はどれとも一致しない
            (input("33", .up), nil),
        ]
        for (value, expected) in cases {
            XCTAssertEqual(AttackerStatRules.matchingPreset(value), expected, "\(value)")
        }
    }

    /// プリセットの値は `AttackerPreset.build` と一致する(別の正を作らない)。物理・特殊のどちらでも、
    /// 使う側のブロックにプリセットを入れた入力から作る要求が、従来のプリセットの要求と同じになる。
    func testPresetInputsResolveToTheSameBuildAsAttackerPreset() throws {
        for category in [MoveCategory.physical, .special] {
            let stat = try XCTUnwrap(AttackerStatRules.attackStat(for: category))
            for preset in AttackerPreset.allCases {
                var value = AttackerStatInputs.default
                value[stat] = AttackerStatRules.presetInput(preset)
                let expected = try AttackerPreset.build(preset, moveCategory: category, natures: stubNatures)
                XCTAssertEqual(
                    AttackerStatRules.resolve(inputs: value, natures: stubNatures, category: category),
                    .resolved(expected), "\(preset) / \(category)")
            }
        }
    }

    // MARK: - 同じ向きにできない組み合わせ

    func testSameDirectionModifiersAreNotSelectable() {
        // もう一方が上昇 → こちらの「上昇」は選べない。下降・補正なしは選べる。
        let otherUp = inputs(atk: input("0", .neutral), spa: input("0", .up))
        XCTAssertFalse(AttackerStatRules.isModifierSelectable(inputs: otherUp, stat: .atk, modifier: .up))
        XCTAssertTrue(AttackerStatRules.isModifierSelectable(inputs: otherUp, stat: .atk, modifier: .down))
        XCTAssertTrue(AttackerStatRules.isModifierSelectable(inputs: otherUp, stat: .atk, modifier: .neutral))
        // もう一方が下降 → こちらの「下降」は選べない。
        let otherDown = inputs(atk: input("0", .down), spa: input("0", .neutral))
        XCTAssertFalse(AttackerStatRules.isModifierSelectable(inputs: otherDown, stat: .spa, modifier: .down))
        XCTAssertTrue(AttackerStatRules.isModifierSelectable(inputs: otherDown, stat: .spa, modifier: .up))
        XCTAssertTrue(AttackerStatRules.isModifierSelectable(inputs: otherDown, stat: .spa, modifier: .neutral))
        // 両方補正なしなら全部選べる。
        for stat in AttackStat.allCases {
            for modifier in NatureChoice.allCases {
                XCTAssertTrue(
                    AttackerStatRules.isModifierSelectable(inputs: .default, stat: stat, modifier: modifier),
                    "\(stat) \(modifier)")
            }
        }
    }

    func testFullPresetIsNotSelectableWhileTheOtherBlockIsUp() {
        let otherUp = inputs(atk: input("0", .neutral), spa: input("0", .up))
        XCTAssertFalse(AttackerStatRules.isPresetSelectable(inputs: otherUp, stat: .atk, preset: .aFull),
                       "特化は上昇補正を伴うので選べない")
        XCTAssertTrue(AttackerStatRules.isPresetSelectable(inputs: otherUp, stat: .atk, preset: .aMax))
        XCTAssertTrue(AttackerStatRules.isPresetSelectable(inputs: otherUp, stat: .atk, preset: .none))
        // もう一方が下降なら特化(上昇)は選べる。
        let otherDown = inputs(atk: input("0", .down), spa: input("0", .neutral))
        XCTAssertTrue(AttackerStatRules.isPresetSelectable(inputs: otherDown, stat: .spa, preset: .aFull))
    }

    // MARK: - 性格の解決(マスタの一覧から)

    private let atkUpDefDownEarlyID = Nature(
        id: "stub-nature-a-atk-up-def-down", nameJa: "テストせいかく攻撃上昇防御下降(ID が早い)", plus: .atk, minus: .def)
    private let atkUpDefDown = Nature(
        id: "stub-nature-atk-up-def-down", nameJa: "テストせいかく攻撃上昇防御下降", plus: .atk, minus: .def)
    private let atkUpSpdDown = Nature(
        id: "stub-nature-atk-up-spd-down", nameJa: "テストせいかく攻撃上昇特防下降", plus: .atk, minus: .spd)

    private func resolve(
        _ natures: [Nature], atk: NatureChoice, spa: NatureChoice, _ category: MoveCategory
    ) -> String? {
        AttackerStatRules.resolveNature(natures: natures, atk: atk, spa: spa, category: category)?.id
    }

    func testBothNeutralResolvesToTheFirstNeutralNatureInListOrder() {
        // 従来(`AttackerPreset.build`)と同じ: 一覧の順で最初の無補正。既定の要求を変えない。
        let neutralEarly = Nature(id: "stub-nature-neutral-z", nameJa: "テストせいかく無補正Z")
        let natures = [StubMaster.atkUpNature, neutralEarly, StubMaster.neutralNature]
        XCTAssertEqual(resolve(natures, atk: .neutral, spa: .neutral, .physical), neutralEarly.id)
        XCTAssertEqual(resolve(natures, atk: .neutral, spa: .neutral, .special), neutralEarly.id)
    }

    func testBothNeutralWithoutANeutralNatureIsUnresolved() {
        XCTAssertNil(resolve([StubMaster.atkUpNature, StubMaster.spaUpNature], atk: .neutral, spa: .neutral, .physical))
        XCTAssertNil(resolve([], atk: .neutral, spa: .neutral, .physical))
    }

    func testUsedSideUpWithOtherNeutralPrefersTheLegacyRepresentativeNature() {
        // 物理 = +A/−C、特殊 = +C/−A(従来の A特化・C特化の性格。素早さに効くお気に入りの個体が変わらない)。
        XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .up, spa: .neutral, .physical), StubMaster.atkUpNature.id)
        XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .neutral, spa: .up, .special), StubMaster.spaUpNature.id)
        // ID が早い別の候補(+A/−防御)があっても代表性格が優先される。
        let natures = [atkUpDefDownEarlyID, StubMaster.atkUpNature, StubMaster.neutralNature]
        XCTAssertEqual(resolve(natures, atk: .up, spa: .neutral, .physical), StubMaster.atkUpNature.id)
    }

    func testBothSidesMatchingNatureIsPickedByAscendingID() {
        // 使う側(特攻)が補正なし・攻撃が上昇 → 代表性格の優先は働かず、A↑ と C(上昇でも下降でもない)の両方に合う
        // 性格のうち ID が昇順で最初。+atk/-spa(代表性格)は C が下降なので合わない。
        let natures = [atkUpSpdDown, StubMaster.atkUpNature, atkUpDefDown, StubMaster.neutralNature]
        XCTAssertEqual(resolve(natures, atk: .up, spa: .neutral, .special), atkUpDefDown.id,
                       "ID の昇順(def < spd)で最初")
    }

    func testBothDirectionsSetResolvesTheMatchingNature() {
        for category in [MoveCategory.physical, .special] {
            XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .up, spa: .down, category),
                           StubMaster.atkUpNature.id, "A↑ C↓ / \(category)")
            XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .down, spa: .up, category),
                           StubMaster.spaUpNature.id, "A↓ C↑ / \(category)")
        }
    }

    func testDownOnOneSideWithOtherNeutralPicksByAscendingID() {
        // A↓・C補正なし: minus=atk で plus が特攻でない性格。defUp("…def-up") < spdUp("…spd-up") の昇順で defUp。
        // spaUp(+spa/-atk)は C が補正なしの条件(plus でも minus でもない)に合わないので除外される。
        XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .down, spa: .neutral, .physical), StubMaster.defUpNature.id)
    }

    func testFallsBackToTheUsedSideOnlyWhenNoNatureMatchesBothSides() {
        // 物理技で C↑・A補正なし: C を上げる性格(+spa/-atk)は A が下降になり「A補正なし」に合わない。
        // 使う側(A)は補正なしなので、無補正でよい(C は物理技のダメージに効かない)。
        XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .neutral, spa: .up, .physical),
                       StubMaster.neutralNature.id)
        // 特殊技で A↑・C補正なし(両方に合う性格が無い一覧): 使う側(C)は補正なし → 無補正。
        XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .up, spa: .neutral, .special),
                       StubMaster.neutralNature.id)
        // 特殊技で C↓・A補正なし(両方に合う性格が無い一覧): 使う側(C)だけ合わせる → minus=spa の性格。
        XCTAssertEqual(resolve(StubMaster.reverseNatures, atk: .neutral, spa: .down, .special),
                       StubMaster.atkUpNature.id)
    }

    func testSameDirectionOrMissingNatureIsUnresolved() {
        XCTAssertNil(resolve(StubMaster.reverseNatures, atk: .up, spa: .up, .physical), "両方上昇は実在しない")
        XCTAssertNil(resolve(StubMaster.reverseNatures, atk: .down, spa: .down, .special), "両方下降は実在しない")
        // 使う側を上げる性格がマスタに無い(無補正と +spa/-atk だけ)。
        let natures = [StubMaster.neutralNature, StubMaster.spaUpNature]
        XCTAssertNil(resolve(natures, atk: .up, spa: .neutral, .physical))
    }

    // MARK: - 要求にする(SP は攻撃と特攻の両方を載せる)

    func testResolveCarriesBothSPValuesRegardlessOfTheUsedSide() {
        let value = inputs(atk: input("20", .neutral), spa: input("7", .neutral))
        for category in [MoveCategory.physical, .special] {
            XCTAssertEqual(
                AttackerStatRules.resolve(inputs: value, natures: stubNatures, category: category),
                .resolved(AttackerBuild(natureId: StubMaster.neutralNature.id, sp: sp(atk: 20, spa: 7))),
                "\(category): 使わない側の SP もそのまま載る")
        }
    }

    func testResolveTotalNeverExceedsTheLimit() {
        let value = inputs(atk: input("32", .neutral), spa: input("32", .neutral))
        guard case .resolved(let build) = AttackerStatRules.resolve(
            inputs: value, natures: stubNatures, category: .physical)
        else { return XCTFail("32 + 32 は要求にできる") }
        let total = build.sp.hp + build.sp.atk + build.sp.def + build.sp.spa + build.sp.spd + build.sp.spe
        XCTAssertEqual(total, 64)
        XCTAssertLessThanOrEqual(total, SPLimits.maxTotal, "構造上 66 を超えない")
        XCTAssertEqual([build.sp.hp, build.sp.def, build.sp.spd, build.sp.spe], [0, 0, 0, 0], "H・B・D・S は 0")
    }

    func testResolveTrimsWhitespaceAndTreatsBlankAsZero() {
        let value = inputs(atk: input(" 12 ", .neutral), spa: input("", .neutral))
        XCTAssertEqual(
            AttackerStatRules.resolve(inputs: value, natures: stubNatures, category: .physical),
            .resolved(AttackerBuild(natureId: StubMaster.neutralNature.id, sp: sp(atk: 12))))
    }

    func testResolveReportsEveryProblemWithoutRounding() {
        // どちらかの SP が不正なら、使わない側でも要求にしない。
        XCTAssertEqual(
            AttackerStatRules.resolve(
                inputs: inputs(atk: input("0", .neutral), spa: input("33", .neutral)),
                natures: stubNatures, category: .physical),
            .invalid([.sp(.spa)]), "技が使わない側の SP が不正でも計算しない")
        XCTAssertEqual(
            AttackerStatRules.resolve(
                inputs: inputs(atk: input("abc", .neutral), spa: input("-1", .neutral)),
                natures: stubNatures, category: .special),
            .invalid([.sp(.atk), .sp(.spa)]), "SP の不正は 攻撃 → 特攻 の順にすべて")
        // SP の不正と性格の解決失敗は両方報告する(性格は最後)。
        XCTAssertEqual(
            AttackerStatRules.resolve(
                inputs: inputs(atk: input("99", .up), spa: input("0", .neutral)),
                natures: [StubMaster.neutralNature], category: .physical),
            .invalid([.sp(.atk), .nature]))
        XCTAssertEqual(
            AttackerStatRules.resolve(
                inputs: inputs(atk: input("0", .up), spa: input("0", .up)),
                natures: stubNatures, category: .physical),
            .invalid([.nature]), "同じ向きは防御的に性格の解決失敗")
    }

    // MARK: - 技の絞り込み

    func testDamagingMovesKeepOrderAndDropStatusMoves() {
        let moves = [StubMaster.statusMove, StubMaster.alphaOnlyMove, StubMaster.specialMove, StubMaster.physicalMove]
        XCTAssertEqual(CalcMoveRules.damagingMoves(moves).map(\.id),
                       [StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id, StubMaster.physicalMove.id])
        XCTAssertEqual(CalcMoveRules.damagingMoves([StubMaster.statusMove]), [])
        XCTAssertEqual(CalcMoveRules.damagingMoves([]), [])
    }

    func testIsStatusMoveJudgesByCategoryOnly() {
        XCTAssertTrue(CalcMoveRules.isStatusMove(StubMaster.statusMove))
        XCTAssertFalse(CalcMoveRules.isStatusMove(StubMaster.physicalMove))
        XCTAssertFalse(CalcMoveRules.isStatusMove(StubMaster.specialMove))
        // 威力 0 でも分類が変化でなければ変化技ではない(固定ダメージ等)。技名・ID も見ない。
        let fixedDamage = Move(id: "stub-move-status-looking-id", nameJa: "テストわざ", type: .normal, category: .physical, power: 0)
        XCTAssertFalse(CalcMoveRules.isStatusMove(fixedDamage))
        let statusWithPower = Move(id: "stub-move-x", nameJa: "テストわざ", type: .normal, category: .status, power: 40)
        XCTAssertTrue(CalcMoveRules.isStatusMove(statusWithPower))
    }
}
