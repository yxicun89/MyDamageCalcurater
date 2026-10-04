import XCTest

@testable import PokeCalcCore

/// 計算画面の攻撃側「攻撃」「特攻」の SP 数値入力・性格補正と、技の選択肢のダメージ技への絞り込み
/// (ADR-0518。F-01 / I-ios-1・I-ios-5。Web の ADR-0329・ADR-0328 と同じ規則)。
///
/// 既存の `CalcViewModelTests` 等のうち、仕様変更で期待値が変わるもの(ADR-0518「既存テストへの影響」)は
/// このファイルでは触らず、実装者が理由つきで更新する。ここでは新しい規則だけを固定する。
/// 計算結果の数値には依存しない(stub はエコー応答)。
@MainActor
final class CalcViewModelAttackerStatsTests: XCTestCase {

    // MARK: - 補助

    private func sp(atk: Int = 0, spa: Int = 0) -> StatBlock {
        StatBlock(hp: 0, atk: atk, def: 0, spa: spa, spd: 0, spe: 0)
    }

    private func loadedViewModel(
        _ stub: StubPokeCalcService, store: StubTeamStore? = nil
    ) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub, teamStore: store)
        await viewModel.load()
        return viewModel
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    private func bulkCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.bulkRequests.count
    }

    /// `operation` の前後で calcBulk が何回呼ばれたか。
    private func calcCount(_ stub: StubPokeCalcService, during operation: () async -> Void) async -> Int {
        let before = await bulkCount(stub)
        await operation()
        let after = await bulkCount(stub)
        return after - before
    }

    private func input(_ text: String, _ modifier: NatureChoice) -> AttackStatInput {
        AttackStatInput(spText: text, modifier: modifier)
    }

    // MARK: - 1. 既定は今の要求と同じ

    func testDefaultsAreZeroNeutralOnBothBlocksAndMatchTheNoneNatureRequest() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.attackerStatInputs, AttackerStatInputs.default)
        XCTAssertEqual(viewModel.usedAttackStat, .atk, "既定の技は物理")
        XCTAssertEqual(viewModel.attackerPreset, AttackerPreset.none)
        XCTAssertEqual(viewModel.attackerPreset(for: .atk), AttackerPreset.none)
        XCTAssertEqual(viewModel.attackerPreset(for: .spa), AttackerPreset.none)
        XCTAssertFalse(viewModel.isCustom(.atk))
        XCTAssertFalse(viewModel.isCustom(.spa))
        XCTAssertEqual(viewModel.attackerStatIssues, [])
        let count = await bulkCount(stub)
        XCTAssertEqual(count, 1, "起動時の計算は1回のまま")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp())
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id, "既定の要求は従来と同一")
    }

    func testUsedAttackStatFollowsTheSelectedMoveCategory() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        XCTAssertEqual(viewModel.usedAttackStat, .atk)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.usedAttackStat, .spa)
        await viewModel.selectMove(id: StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(viewModel.usedAttackStat, .atk)
    }

    // MARK: - 2. SP の数値入力

    func testSettingSPTextCalculatesOnceAndMakesTheBlockCustom() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        let calls = await calcCount(stub) { await viewModel.setAttackerSPText("20", for: .atk) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.attackerStatInputs.atk, input("20", .neutral), "補正は変わらない")
        XCTAssertNil(viewModel.attackerPreset, "どのプリセットとも一致しない = カスタム")
        XCTAssertNil(viewModel.attackerPreset(for: .atk))
        XCTAssertTrue(viewModel.isCustom(.atk))
        XCTAssertFalse(viewModel.isCustom(.spa), "もう一方は無振りのまま")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 20))
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }

    func testBothBlocksSPAreSentRegardlessOfTheUsedSide() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)  // 物理技
        await viewModel.setAttackerSPText("20", for: .atk)
        await viewModel.setAttackerSPText("7", for: .spa)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 20, spa: 7), "使わない側(特攻)の SP もそのまま載る")

        await viewModel.selectMove(id: StubMaster.specialMove.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 20, spa: 7), "技を替えても値は両方残る")
    }

    func testSettingTheSameTextDoesNotCalculate() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let calls = await calcCount(stub) { await viewModel.setAttackerSPText("0", for: .atk) }
        XCTAssertEqual(calls, 0, "文字列が変わらない操作は計算しない")
    }

    func testBlankAndPaddedTextAreValidAndKeptAsTyped() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        let blankCalls = await calcCount(stub) { await viewModel.setAttackerSPText("", for: .atk) }
        XCTAssertEqual(blankCalls, 1)
        XCTAssertEqual(viewModel.attackerStatInputs.atk.spText, "", "入力途中の文字列をそのまま保つ")
        XCTAssertEqual(viewModel.attackerStatIssues, [], "空欄は 0")
        XCTAssertEqual(viewModel.attackerPreset(for: .atk), AttackerPreset.none, "空欄は 0 と同じ = 無振り")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp())

        await viewModel.setAttackerSPText(" 12 ", for: .spa)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(spa: 12), "前後の空白は無視する")
    }

    // MARK: - 3. 性格補正

    func testNatureModifierResolvesANatureFromTheMasterAndKeepsTheNumber() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        let calls = await calcCount(stub) { await viewModel.setAttackerNatureModifier(.up, for: .atk) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.attackerStatInputs.atk, input("0", .up), "数値は変わらない")
        XCTAssertNil(viewModel.attackerPreset, "0・上昇はどのプリセットとも一致しない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.natureId, StubMaster.atkUpNature.id, "物理 = +A/−C の代表性格")
        XCTAssertEqual(request.attacker.sp, sp())
    }

    func testSettingTheSameModifierDoesNotCalculate() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let calls = await calcCount(stub) { await viewModel.setAttackerNatureModifier(.neutral, for: .atk) }
        XCTAssertEqual(calls, 0)
    }

    func testOtherBlockUpWithPhysicalMoveSendsNeutralNatureBecauseTheUsedSideIsNeutral() async throws {
        // 特攻↑でも物理技のダメージには効かない: 使う側(攻撃)が補正なし → 両方に合う性格が無ければ無補正。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerNatureModifier(.up, for: .spa)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id)
        XCTAssertEqual(viewModel.attackerStatInputs.spa.modifier, .up, "入力は残る")
    }

    func testNatureModifierWithFullMasterUsesAscendingIDForDownOnOneSide() async throws {
        let stub = StubMaster.makeService(natures: StubMaster.reverseNatures)
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerNatureModifier(.down, for: .atk)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.natureId, StubMaster.defUpNature.id,
                       "A↓・C補正なしは ID 昇順で最初(def-up < spd-up)")
    }

    // MARK: - 4. プリセットとの連動

    func testPresetFillsTheNumberAndModifierOfTheChosenBlockOnly() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        let calls = await calcCount(stub) { await viewModel.selectAttackerPreset(.aFull, for: .atk) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.attackerStatInputs.atk, input("32", .up))
        XCTAssertEqual(viewModel.attackerStatInputs.spa, input("0", .neutral), "もう一方は変えない")
        XCTAssertEqual(viewModel.attackerPreset, .aFull)
        XCTAssertEqual(viewModel.attackerPreset(for: .atk), .aFull)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 32))
        XCTAssertEqual(request.attacker.natureId, StubMaster.atkUpNature.id)

        await viewModel.selectAttackerPreset(.aMax, for: .spa)
        XCTAssertEqual(viewModel.attackerStatInputs.spa, input("32", .neutral))
        XCTAssertEqual(viewModel.attackerStatInputs.atk, input("32", .up), "攻撃のブロックは残る")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 32, spa: 32), "合計 64 でも要求にできる")

        await viewModel.selectAttackerPreset(.none, for: .atk)
        XCTAssertEqual(viewModel.attackerStatInputs.atk, input("0", .neutral))
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(spa: 32))
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id)
    }

    func testPresetWithoutAStatAppliesToTheBlockTheMoveUses() async throws {
        // 既存の `selectAttackerPreset(_:)`(ピルの行)は、選んだ技が使う側のブロックに入れる。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        await viewModel.selectAttackerPreset(.aFull)
        XCTAssertEqual(viewModel.attackerStatInputs.spa, input("32", .up))
        XCTAssertEqual(viewModel.attackerStatInputs.atk, input("0", .neutral))
        XCTAssertEqual(viewModel.attackerPreset, .aFull, "使う側(特攻)のブロックで判定する")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(spa: 32))
        XCTAssertEqual(request.attacker.natureId, StubMaster.spaUpNature.id)
    }

    func testPresetSelectionIsDerivedFromTheValueNotRemembered() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        await viewModel.selectAttackerPreset(.aFull, for: .atk)
        XCTAssertEqual(viewModel.attackerPreset(for: .atk), .aFull)

        await viewModel.setAttackerSPText("31", for: .atk)
        XCTAssertNil(viewModel.attackerPreset(for: .atk), "数値を変えるとカスタム")
        XCTAssertTrue(viewModel.isCustom(.atk))
        XCTAssertEqual(viewModel.attackerStatInputs.atk.modifier, .up, "数値を変えても補正は変わらない")

        await viewModel.setAttackerSPText("32", for: .atk)
        XCTAssertEqual(viewModel.attackerPreset(for: .atk), .aFull, "数値を戻すとまたそのプリセット")
        XCTAssertFalse(viewModel.isCustom(.atk))

        await viewModel.setAttackerNatureModifier(.neutral, for: .atk)
        XCTAssertEqual(viewModel.attackerPreset(for: .atk), .aMax, "補正を変えても数値は変わらない")
        XCTAssertEqual(viewModel.attackerStatInputs.atk.spText, "32")
    }

    // MARK: - 5. 同じ向きにできない

    func testSameDirectionChoicesAreRefusedWithoutRewritingTheOtherBlock() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerNatureModifier(.up, for: .spa)

        XCTAssertFalse(viewModel.isModifierSelectable(.up, for: .atk))
        XCTAssertTrue(viewModel.isModifierSelectable(.down, for: .atk))
        XCTAssertTrue(viewModel.isModifierSelectable(.neutral, for: .atk))
        XCTAssertFalse(viewModel.isPresetSelectable(.aFull, for: .atk), "特化は上昇を伴うので選べない")
        XCTAssertTrue(viewModel.isPresetSelectable(.aMax, for: .atk))
        XCTAssertTrue(viewModel.isPresetSelectable(.none, for: .atk))

        let before = viewModel.attackerStatInputs
        let calls = await calcCount(stub) {
            await viewModel.setAttackerNatureModifier(.up, for: .atk)
            await viewModel.selectAttackerPreset(.aFull, for: .atk)
        }
        XCTAssertEqual(calls, 0, "選べない操作は計算しない")
        XCTAssertEqual(viewModel.attackerStatInputs, before, "もう一方の値も黙って書き換えない")
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 6. 不正入力は丸めず、計算しない

    func testInvalidSPShowsAnIssueKeepsTheTextAndDoesNotCalculate() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        XCTAssertFalse(viewModel.rows.isEmpty)

        let calls = await calcCount(stub) { await viewModel.setAttackerSPText("33", for: .atk) }
        XCTAssertEqual(calls, 0, "不正な入力は計算しない")
        XCTAssertEqual(viewModel.attackerStatInputs.atk.spText, "33", "丸めずそのまま保つ")
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.atk)])
        XCTAssertTrue(viewModel.isSPInvalid(.atk))
        XCTAssertFalse(viewModel.isSPInvalid(.spa))
        XCTAssertTrue(viewModel.rows.isEmpty, "古い結果も出さない")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error, "SP の不正は画面のエラーバナーにしない(入力の近くに出す)")

        // 直せば計算し直す。
        let fixed = await calcCount(stub) { await viewModel.setAttackerSPText("32", for: .atk) }
        XCTAssertEqual(fixed, 1)
        XCTAssertEqual(viewModel.attackerStatIssues, [])
        XCTAssertFalse(viewModel.isSPInvalid(.atk))
        XCTAssertFalse(viewModel.rows.isEmpty)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 32))
    }

    func testInvalidSPOnTheUnusedSideAlsoBlocksCalculation() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)  // 物理技(使うのは攻撃)
        let calls = await calcCount(stub) { await viewModel.setAttackerSPText("abc", for: .spa) }
        XCTAssertEqual(calls, 0, "技が使わない側が不正でも計算しない(両方の SP を載せるため)")
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.spa)])
        XCTAssertTrue(viewModel.rows.isEmpty)
        // 技を替えても(使う側が変わっても)不正は残り、計算しない。
        let moveCalls = await calcCount(stub) { await viewModel.selectMove(id: StubMaster.specialMove.id) }
        XCTAssertEqual(moveCalls, 0)
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.spa)])
    }

    func testEveryKindOfInvalidTextIsRejected() async {
        for text in ["33", "-1", "+5", "3.5", "1e1", "12a", "１２"] {
            let stub = StubMaster.makeService()
            let viewModel = await loadedViewModel(stub)
            let calls = await calcCount(stub) { await viewModel.setAttackerSPText(text, for: .atk) }
            XCTAssertEqual(calls, 0, "「\(text)」は計算しない")
            XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.atk)], "「\(text)」")
        }
    }

    func testBothBlocksInvalidReportsBothInOrder() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        await viewModel.setAttackerSPText("x", for: .spa)
        await viewModel.setAttackerSPText("y", for: .atk)
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.atk), .sp(.spa)], "攻撃 → 特攻の順")
    }

    func testUnresolvableNatureSetsAnExplicitErrorAndDoesNotCalculate() async {
        // 使う側(攻撃)を上げる性格がマスタに無い。
        let stub = StubMaster.makeService(natures: [StubMaster.neutralNature, StubMaster.spaUpNature])
        let viewModel = await loadedViewModel(stub)
        let calls = await calcCount(stub) { await viewModel.setAttackerNatureModifier(.up, for: .atk) }
        XCTAssertEqual(calls, 0)
        XCTAssertEqual(viewModel.attackerStatIssues, [.nature])
        XCTAssertTrue(viewModel.rows.isEmpty, "古い結果も出さない")
        guard case .service(let code, let message) = viewModel.error else {
            return XCTFail("性格が無いエラーにならない: \(String(describing: viewModel.error))")
        }
        XCTAssertEqual(code, PokeCalcError.Code.natureUnavailable)
        XCTAssertEqual(message, AttackerStatLabels.natureUnresolved)

        // 直せば(補正なしに戻す)エラーが消えて計算し直す。
        let fixed = await calcCount(stub) { await viewModel.setAttackerNatureModifier(.neutral, for: .atk) }
        XCTAssertEqual(fixed, 1)
        XCTAssertNil(viewModel.error)
        XCTAssertEqual(viewModel.attackerStatIssues, [])
    }

    // MARK: - 7. 古い応答の破棄

    /// 応答で見分けられる行を返す(presetLabel に目印を入れる)。
    private func markedResult(_ mark: String, for request: BulkCalcRequest) -> BulkCalcResult {
        BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: [
            BulkCalcRow(preset: .hp, presetLabel: mark, itemId: nil, defender: testBulkDefender,
                        result: StubPokeCalcService.echoCalcResult),
        ])
    }

    private func loadWithManualStub(_ stub: StubPokeCalcService) async throws -> CalcViewModel {
        await stub.setBulkMode(.manual)
        let viewModel = CalcViewModel(service: stub)
        let loadTask = Task { await viewModel.load() }
        try await stub.waitForBulkRequests(count: 1)
        await stub.resolveBulkWithEcho(at: 0)
        await loadTask.value
        return viewModel
    }

    func testOlderResponseIsDiscardedWhenALaterSPEditArrivesFirst() async throws {
        let stub = StubMaster.makeService()
        let viewModel = try await loadWithManualStub(stub)

        let older = Task { await viewModel.setAttackerSPText("10", for: .atk) }
        try await stub.waitForBulkRequests(count: 2)
        let newer = Task { await viewModel.setAttackerSPText("20", for: .atk) }
        try await stub.waitForBulkRequests(count: 3)
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests[1].attacker.sp, sp(atk: 10))
        XCTAssertEqual(requests[2].attacker.sp, sp(atk: 20))

        await stub.resolveBulk(at: 2, with: .success(markedResult("テスト新", for: requests[2])))
        await newer.value
        await stub.resolveBulk(at: 1, with: .success(markedResult("テスト旧", for: requests[1])))
        await older.value
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), ["テスト新"], "古い要求の応答で上書きしない")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testResponseForAValidValueIsDiscardedAfterTheInputBecomesInvalid() async throws {
        let stub = StubMaster.makeService()
        let viewModel = try await loadWithManualStub(stub)

        let pending = Task { await viewModel.setAttackerSPText("10", for: .atk) }
        try await stub.waitForBulkRequests(count: 2)
        await viewModel.setAttackerSPText("33", for: .atk)  // 不正にした: 待っている要求は無効
        XCTAssertTrue(viewModel.rows.isEmpty)
        XCTAssertFalse(viewModel.isLoading, "不正になったら読み込み中も解く")

        let requests = await stub.bulkRequests
        await stub.resolveBulk(at: 1, with: .success(markedResult("テスト旧", for: requests[1])))
        await pending.value
        XCTAssertTrue(viewModel.rows.isEmpty, "不正な入力の下に古い結果を出さない")
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.atk)])
    }

    // MARK: - 8. 値の寿命(技・種族・攻守入れ替えで消さない)

    func testInputsSurviveMoveSpeciesAndSwapChanges() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerSPText("20", for: .atk)
        await viewModel.setAttackerNatureModifier(.up, for: .atk)
        await viewModel.setAttackerSPText("7", for: .spa)
        let expected = AttackerStatInputs(atk: input("20", .up), spa: input("7", .neutral))
        XCTAssertEqual(viewModel.attackerStatInputs, expected)

        await viewModel.selectMove(id: StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.attackerStatInputs, expected, "物理 → 特殊でも両方残る")
        XCTAssertEqual(viewModel.usedAttackStat, .spa, "強調だけが移る")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 20, spa: 7))

        await viewModel.swapSides()
        XCTAssertEqual(viewModel.attackerStatInputs, expected, "攻守入れ替えで消さない")
        await viewModel.selectAttacker(speciesKey: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.attackerStatInputs, expected, "攻撃側の種族を替えても消さない")
        await viewModel.selectDefender(speciesKey: StubMaster.alpha.key)
        XCTAssertEqual(viewModel.attackerStatInputs, expected, "防御側の種族を替えても消さない")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 20, spa: 7))
    }

    func testInvalidTextAlsoSurvivesAMoveChange() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        await viewModel.setAttackerSPText("abc", for: .atk)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.attackerStatInputs.atk.spText, "abc")
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.atk)])
    }

    // MARK: - 9. 構築の個体との関係

    func testCalledTeamIndividualKeepsItsOwnValuesAndIgnoresInputFields() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.setAttackerSPText("33", for: .atk)  // 不正な入力が残っていても
        XCTAssertEqual(viewModel.attackerStatIssues, [.sp(.atk)])

        await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP, "構築の個体の値のまま(入力欄の値を使わない)")
        XCTAssertEqual(request.attacker.natureId, StubTeams.namedMember.natureId)
        XCTAssertEqual(viewModel.attackerStatIssues, [], "構築の個体を呼んでいる間は入力欄の不正で止めない")
        XCTAssertFalse(viewModel.isSPInvalid(.atk))
        XCTAssertEqual(viewModel.attackerStatInputs.atk.spText, "33", "入力欄の値は書き換えない(個体の値を写さない)")
        XCTAssertNil(viewModel.attackerPreset, "構築を呼んでいる間はプリセットの選択表示を消す")
    }

    func testEditingAnyInputReleasesTheTeamIndividualAndUsesTheInputs() async throws {
        let operations: [(String, @MainActor (CalcViewModel) async -> Void)] = [
            ("SP", { await $0.setAttackerSPText("20", for: .atk) }),
            ("補正", { await $0.setAttackerNatureModifier(.up, for: .atk) }),
            ("プリセット(ブロック指定)", { await $0.selectAttackerPreset(.aMax, for: .atk) }),
        ]
        for (name, operation) in operations {
            let stub = StubMaster.makeService()
            let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
            await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id)
            XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection, name)

            await operation(viewModel)
            XCTAssertNil(viewModel.attackerBuildSource.teamSelection, "\(name): 構築の選択は外れる")
            let request = try await lastRequest(stub)
            XCTAssertNil(request.attacker.abilityId, "\(name): 構築から来た特性も外す(プリセットと同じ)")
            XCTAssertNotEqual(request.attacker.sp, StubTeams.customSP, name)
            XCTAssertEqual(request.attacker.sp.hp, 0, "\(name): H・B・D・S は 0")
            XCTAssertEqual(request.attacker.sp.spe, 0, name)
        }
    }

    func testTeamIndividualWithOnlyStatusMovesGetsTheDefaultDamagingMove() async throws {
        // 構築の個体の技が変化技だけでも、計算画面は変化技を選ばない(既定のダメージ技にする)。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.selectTeamIndividual(teamID: StubTeams.teamBeta.id, memberID: StubTeams.statusMoveMember.id)
        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection, "個体の SP・性格は使う")
        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id, "learnset の最初のダメージ技")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)
    }

    // MARK: - 10. お気に入りに入れる個体

    func testFavoriteIndividualUsesTheResolvedInputs() async throws {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        await viewModel.setAttackerSPText("20", for: .atk)
        await viewModel.setAttackerNatureModifier(.up, for: .atk)
        await viewModel.setAttackerSPText("7", for: .spa)

        let individual = try viewModel.attackerIndividualForFavorite()
        XCTAssertEqual(individual.sp, sp(atk: 20, spa: 7), "使わない側の SP も載る")
        XCTAssertEqual(individual.natureId, StubMaster.atkUpNature.id)
        XCTAssertEqual(individual.speciesKey, StubMaster.alpha.key)
    }

    /// お気に入りに入れる個体を作ろうとして投げられた `PokeCalcError`(投げなければ nil)。
    private func favoriteError(_ viewModel: CalcViewModel) -> PokeCalcError? {
        do {
            _ = try viewModel.attackerIndividualForFavorite()
            return nil
        } catch {
            return error as? PokeCalcError
        }
    }

    func testFavoriteIndividualThrowsWhenTheInputsCannotBeResolved() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        await viewModel.setAttackerSPText("33", for: .atk)
        XCTAssertEqual(favoriteError(viewModel)?.code, PokeCalcError.Code.attackerSPInvalid, "SP 不正")

        let noNature = await loadedViewModel(
            StubMaster.makeService(natures: [StubMaster.neutralNature, StubMaster.spaUpNature]))
        await noNature.setAttackerNatureModifier(.up, for: .atk)
        XCTAssertEqual(favoriteError(noNature)?.code, PokeCalcError.Code.natureUnavailable, "性格なし")
    }

    // MARK: - 11. 技の選択肢はダメージ技だけ

    func testMoveOptionsContainOnlyDamagingMovesInLearnsetOrder() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        // alpha の learnset: [変化, アルファ専用(物理), 特殊, マスタに無い ID]。変化技は出さない。
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id])
        XCTAssertFalse(viewModel.moveOptions.contains { $0.category == .status })
    }

    func testMoveSearchResultsNeverBringStatusMovesBack() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        // 変化技の名前で検索しても、選択肢には出ない(検索結果 ∩ learnset ∩ ダメージ技)。
        viewModel.setMoveQuery(StubMaster.statusMove.nameJa)
        await viewModel.runMoveSearch()
        XCTAssertFalse(viewModel.moveOptions.contains { $0.id == StubMaster.statusMove.id })
        XCTAssertEqual(viewModel.moveOptions, [], "その語に一致するダメージ技は無い")
        // 検索語を空に戻せば絞り込み後の一覧に戻る。
        viewModel.setMoveQuery("")
        await viewModel.runMoveSearch()
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id])
    }

    func testSelectingAStatusMoveIsIgnoredWithoutCalculating() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let before = viewModel.moveId
        let calls = await calcCount(stub) { await viewModel.selectMove(id: StubMaster.statusMove.id) }
        XCTAssertEqual(calls, 0, "選択肢に無い技は無視して計算しない")
        XCTAssertEqual(viewModel.moveId, before)
        XCTAssertFalse(viewModel.isStatusMoveSelected)
    }

    func testChangingSpeciesNeverSelectsAStatusMove() async {
        // beta の learnset は [変化, 特殊, 物理]。先頭の変化技ではなく最初のダメージ技(特殊)になる。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.swapSides()
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
        XCTAssertFalse(viewModel.isStatusMoveSelected)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.specialMove.id, StubMaster.physicalMove.id])
    }

    func testSpeciesWithOnlyStatusMovesHasNoMoveAndDoesNotCalculate() async throws {
        // ダメージ技を1つも覚えない種族: 技欄は空、案内を出し、計算しない(エラーにもしない)。
        let stub = StubMaster.makeService(species: [StubMaster.statusOnly, StubMaster.beta])
        let viewModel = await loadedViewModel(stub)
        XCTAssertTrue(viewModel.hasNoDamagingMoves)
        XCTAssertEqual(viewModel.moveOptions, [])
        XCTAssertNil(viewModel.selectedMove)
        XCTAssertEqual(viewModel.moveSummaryText, "")
        XCTAssertNil(viewModel.usedAttackStat, "技が無いときは強調しない")
        XCTAssertNil(viewModel.error, "マスタの不具合ではないのでエラーバナーにしない")
        XCTAssertTrue(viewModel.rows.isEmpty)
        XCTAssertFalse(viewModel.isLoading)
        let count = await bulkCount(stub)
        XCTAssertEqual(count, 0, "計算要求を送らない")

        // 入力を触っても計算しない。
        let editCalls = await calcCount(stub) { await viewModel.setAttackerSPText("5", for: .atk) }
        XCTAssertEqual(editCalls, 0)
        XCTAssertEqual(viewModel.attackerStatInputs.atk.spText, "5", "入力は受け付ける")

        // ダメージ技を覚える種族に替えれば計算が走り、案内は消える。
        let calls = await calcCount(stub) { await viewModel.selectAttacker(speciesKey: StubMaster.beta.key) }
        XCTAssertEqual(calls, 1)
        XCTAssertFalse(viewModel.hasNoDamagingMoves)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: 5), "攻撃側を替えても入力は残る")
    }

    func testSwappingToASpeciesWithOnlyStatusMovesStopsCalculating() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.statusOnly.key)
        XCTAssertFalse(viewModel.rows.isEmpty)

        let calls = await calcCount(stub) { await viewModel.swapSides() }
        XCTAssertEqual(calls, 0, "入れ替え後の攻撃側がダメージ技を持たないので計算しない")
        XCTAssertTrue(viewModel.hasNoDamagingMoves)
        XCTAssertTrue(viewModel.rows.isEmpty, "古い結果も出さない")
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - 12. モック(アプリのモック実装でも同じ規則)

    func testMockServiceMoveOptionsHaveNoStatusMovesAndTheDefaultIsTheFirstDamagingMove() async throws {
        let mock = try MockPokeCalcService()
        let viewModel = CalcViewModel(service: mock)
        await viewModel.load()
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.moveOptions.isEmpty)
        XCTAssertFalse(viewModel.moveOptions.contains { $0.category == .status },
                       "モックの種族は変化技も覚えるが、選択肢には出ない")
        XCTAssertEqual(viewModel.moveId, "test-move-physical-a")
        XCTAssertFalse(viewModel.rows.isEmpty)

        // モックでも SP・性格の入力で計算が成功し続ける(モックは数値を計算しない)。
        await viewModel.setAttackerSPText("20", for: .atk)
        await viewModel.setAttackerNatureModifier(.up, for: .atk)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }
}
