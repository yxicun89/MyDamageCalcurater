import XCTest

@testable import PokeCalcCore

/// 計算画面で「構築から個体を呼び出す」(P6-2d。ADR-0501「P6-2d」)。
///
/// 既存の `CalcViewModelTests`(プリセットだけの経路)は一切変えない。ここではプリセット経路を壊さずに
/// 構築の個体を呼べること、呼んだ個体は**プリセットに丸め直されず**その個体の SP・性格・特性・テラスタイプで
/// 要求が組み立てられることを固定する。
@MainActor
final class CalcViewModelTeamIndividualTests: XCTestCase {

    // MARK: - 補助

    private func loadedViewModel(
        _ stub: StubPokeCalcService, store: StubTeamStore
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

    /// `StubTeams.teamAlpha` の `namedMember` を呼び出す(いちばんよく使う組み合わせ)。
    private func selectNamedMember(_ viewModel: CalcViewModel) async {
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id
        )
    }

    // MARK: - 構築の一覧を読む

    func testLoadPopulatesTeamOptionsSkippingEmptyTeams() async {
        let store = StubTeams.makeStore()
        let viewModel = await loadedViewModel(StubMaster.makeService(), store: store)

        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamAlpha.id, StubTeams.teamBeta.id])
        // `nickname`(String?)と `nameJa`(String)を1つの配列リテラルに混ぜると、型推論が `[Any]` に
        // 落ちてコンパイルできない(Swift の既知の制約)。両辺を `[String?]` にそろえて比較する
        // (比較する値そのものは変えていない。implementer が気づいた型だけのバグ)。
        XCTAssertEqual(
            viewModel.teamOptions.first?.members.map { $0.displayName as String? },
            [StubTeams.namedMember.nickname, StubMaster.alpha.nameJa] as [String?]
        )
        let listCalls = await store.listCallCount
        XCTAssertEqual(listCalls, 1, "起動時の構築の読み込みは1回")
    }

    func testLoadTeamsCanBeCalledAgainToPickUpEdits() async {
        // 構築ビルダーで編集して戻ってきたときに View から呼び直せること(`load()` の一度きりガードとは別)。
        let store = StubTeams.makeStore(teams: [])
        let viewModel = await loadedViewModel(StubMaster.makeService(), store: store)
        XCTAssertTrue(viewModel.teamOptions.isEmpty)

        await store.seed([StubTeams.teamAlpha])
        await viewModel.loadTeams()
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamAlpha.id])
    }

    func testWithoutTeamStoreTeamOptionsStayEmptyAndCalculationIsUnchanged() async throws {
        // `teamStore` を渡さない既存の使い方(テスト・プレビュー)がそのまま動くこと。
        let stub = StubMaster.makeService()
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()

        XCTAssertTrue(viewModel.teamOptions.isEmpty, "構築の保存先が無ければ選べる個体も無い")
        await viewModel.loadTeams()
        XCTAssertTrue(viewModel.teamOptions.isEmpty)
        let count = await bulkCount(stub)
        XCTAssertEqual(count, 1, "起動時の一括計算はこれまでどおり1回")
        XCTAssertNil(viewModel.error)
    }

    func testTeamStoreFailureLeavesOptionsEmptyWithoutBreakingCalculation() async {
        // 保存の都合で計算を止めない(CLAUDE.md 絶対ルール5と同じ発想。ADR-0501「P6-2d」5章)。
        let store = StubTeams.makeStore()
        await store.setListError(PokeCalcError(code: PokeCalcError.Code.teamStoreCorrupted, message: "テスト: 壊れている"))
        let viewModel = await loadedViewModel(StubMaster.makeService(), store: store)

        XCTAssertTrue(viewModel.teamOptions.isEmpty)
        XCTAssertNil(viewModel.error, "構築が読めなくても計算画面のエラーにはしない")
        XCTAssertFalse(viewModel.rows.isEmpty, "計算は成功したまま")
    }

    // MARK: - 呼び出した個体をそのまま使う

    func testSelectTeamIndividualUsesExactBuildInsteadOfPreset() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await selectNamedMember(viewModel)

        // 出どころが構築になり、どのプリセットも選ばれていない状態になる。
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.teamID, StubTeams.teamAlpha.id)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.displayName, StubTeams.namedMember.nickname)
        XCTAssertNil(viewModel.attackerPreset, "構築を呼んでいる間はプリセットの選択表示を消す")

        // 画面の状態も個体の値に合わせる。
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubTeams.namedMember.speciesKey)
        XCTAssertEqual(viewModel.attackerItemId, StubTeams.namedMember.itemId)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id, "個体の技が learnset にあるのでそのまま選ぶ")

        // 要求は個体の値そのまま。プリセットから作り直していない。
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)
        XCTAssertEqual(request.attacker.natureId, StubTeams.namedMember.natureId)
        XCTAssertNotEqual(request.attacker.natureId, StubMaster.atkUpNature.id,
                          "A特化の上昇性格に作り直していない")
        XCTAssertNotEqual(request.attacker.natureId, StubMaster.neutralNature.id,
                          "無補正に作り直していない")
        XCTAssertEqual(request.attacker.abilityId, StubTeams.namedMember.abilityId,
                       "プリセット経路では常に nil の特性も、構築からは写す")
        XCTAssertEqual(request.attacker.teraType, StubTeams.namedMember.teraType)
        XCTAssertEqual(request.attacker.itemId, StubTeams.namedMember.itemId)
        XCTAssertEqual(request.attacker.speciesKey, StubTeams.namedMember.speciesKey)
        XCTAssertNil(request.attacker.moveId, "技は要求本体の moveId が正(プリセット経路と同じく nil にそろえる)")
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertNil(viewModel.error)
    }

    /// スナップショット方式の中核不変条件(ADR-0501「P6-2d」1章「呼んだ個体はその個体の値をそのまま
    /// 使う」の裏側): 呼び出した**後**に構築ビルダー側でその個体の値が変わっても、次に呼び直すまでは
    /// 追従しない。`loadTeams()`(一覧の再読み込み)だけでは古いスナップショットのまま計算し続け、
    /// 同じ teamID/memberID を選び直したときだけ新しい値に切り替わる。
    func testSelectedSnapshotDoesNotFollowLaterTeamEditsUntilReselected() async throws {
        let stub = StubMaster.makeService()
        let store = StubTeams.makeStore()
        let viewModel = await loadedViewModel(stub, store: store)
        await selectNamedMember(viewModel)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)

        // 構築ビルダー側で同じメンバーの SP を変えて保存する(id は変えない編集)。
        let editedSP = StatBlock(hp: 30, atk: 2, def: 2, spa: 2, spd: 2, spe: 2)
        var editedMember = StubTeams.namedMember
        editedMember.sp = editedSP
        var editedTeam = StubTeams.teamAlpha
        editedTeam.members = [editedMember, StubTeams.noMoveMember]
        try await store.save(editedTeam)

        // 一覧を読み直すだけでは(呼び直していないので)スナップショットは古いまま。
        await viewModel.loadTeams()
        await viewModel.selectAttackerItem(id: StubMaster.itemA.id) // 画面状態の変更で再計算だけさせる
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP, "呼び直すまでは編集前の値のまま")
        XCTAssertNotEqual(request.attacker.sp, editedSP)

        // 同じ teamID/memberID をもう一度選ぶと、新しい値に切り替わる。
        await selectNamedMember(viewModel)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, editedSP, "呼び直すと最新の値に切り替わる")
    }

    func testSelectTeamIndividualCalculatesExactlyOnce() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        let before = await bulkCount(stub)
        await selectNamedMember(viewModel)
        let after = await bulkCount(stub)
        XCTAssertEqual(after - before, 1, "呼び出しで一括計算はちょうど1回")
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - プリセットと構築は排他

    func testSelectingPresetAfterTeamClearsTeamSelection() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await selectNamedMember(viewModel)
        await viewModel.selectAttackerPreset(.aMax)

        XCTAssertEqual(viewModel.attackerPreset, .aMax)
        XCTAssertNil(viewModel.attackerBuildSource.teamSelection, "プリセットを押したら構築の選択は外れる")

        let request = try await lastRequest(stub)
        XCTAssertNotEqual(request.attacker.sp, StubTeams.customSP, "SP はプリセットのものに戻る")
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id, "A振りは無補正")
        XCTAssertNil(request.attacker.abilityId, "プリセット経路は特性を持たない")
        XCTAssertNil(request.attacker.teraType)
        // 種族・持ち物・技は画面の状態なので、プリセットに戻しても呼び出したときのまま残る。
        XCTAssertEqual(request.attacker.speciesKey, StubTeams.namedMember.speciesKey)
        XCTAssertEqual(request.attacker.itemId, StubTeams.namedMember.itemId)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
    }

    func testSelectingTeamAfterPresetClearsPreset() async {
        let viewModel = await loadedViewModel(StubMaster.makeService(), store: StubTeams.makeStore())
        await viewModel.selectAttackerPreset(.none)
        // `attackerPreset` が `AttackerPreset?` になったため、素の `.none` は `Optional<AttackerPreset>.none`
        // (nil)に解決されてしまう(Swift の既知の挙動。ビルド時に警告も出る)。列挙子を明示して
        // 曖昧さを消す(比較する意味は変えていない。implementer が気づいた型だけのバグ)。
        XCTAssertEqual(viewModel.attackerPreset, AttackerPreset.none)

        await selectNamedMember(viewModel)
        XCTAssertNil(viewModel.attackerPreset)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id)
    }

    // MARK: - 技の落としどころ(ADR-0501「P6-2d」3章)

    func testMemberWithoutMovesFallsBackToFirstDamagingLearnsetMove() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamAlpha.id, memberID: StubTeams.noMoveMember.id
        )

        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id,
                       "技が空なら learnset の順で最初のダメージ技に落とす(規則3と同じ)")
        XCTAssertNil(viewModel.error, "技が無い個体はエラーにしない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.alphaOnlyMove.id)
        XCTAssertNil(request.attacker.moveId)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP, "技が落ちても SP・性格は個体のまま")
        XCTAssertNil(request.attacker.itemId, "持ち物を持たない個体は持ち物なしになる")
    }

    func testMemberMoveOutsideLearnsetFallsBackToFirstDamagingLearnsetMove() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamBeta.id, memberID: StubTeams.unlearnedMoveMember.id
        )

        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id,
                       "いまの種族が覚えない技は選べないので既定の技に落とす")
        XCTAssertNil(viewModel.error)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)
    }

    func testMemberWithStatusMoveKeepsItOnCalcScreen() async {
        // 計算画面の技の選択肢は変化技も含む(P6-2a 規則4)ので、変化技の個体はそのまま選ばれる。
        let viewModel = await loadedViewModel(StubMaster.makeService(), store: StubTeams.makeStore())
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamBeta.id, memberID: StubTeams.statusMoveMember.id
        )
        XCTAssertEqual(viewModel.moveId, StubMaster.statusMove.id)
    }

    // MARK: - 呼び出したあとの操作

    func testChangingMoveKeepsTeamBuildAndUsesScreenMove() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await selectNamedMember(viewModel)
        await viewModel.selectMove(id: StubMaster.alphaOnlyMove.id)

        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection, "技を変えても構築の選択は外れない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.alphaOnlyMove.id, "要求の技は画面の技が正")
        XCTAssertNil(request.attacker.moveId)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)
    }

    func testChangingItemKeepsTeamBuildAndUsesScreenItem() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await selectNamedMember(viewModel)
        await viewModel.selectAttackerItem(id: StubMaster.itemA.id)

        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.itemId, StubMaster.itemA.id, "要求の持ち物は画面の持ち物が正")
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)
    }

    func testSwapSidesKeepsTeamBuild() async throws {
        // 攻守入れ替えは `attackerSpeciesKey` を書き換えるので、ここで構築の選択が外れると
        // 自分の調整が黙って消える(P6-2a 規則6「プリセットは入れ替えない」と同じ扱い)。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await selectNamedMember(viewModel)
        await viewModel.swapSides()

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection, "入れ替えで構築の選択は外れない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.attacker.sp, StubTeams.customSP)
        XCTAssertEqual(request.attacker.natureId, StubTeams.namedMember.natureId)
    }

    // MARK: - 知らない ID は無視する

    func testUnknownTeamOrMemberIsIgnored() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        let before = await bulkCount(stub)

        await viewModel.selectTeamIndividual(teamID: "stub-team-does-not-exist", memberID: StubTeams.namedMember.id)
        await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: "stub-member-does-not-exist")
        // メンバーが0体の構築は選択肢に無いので、その id も無視される。
        await viewModel.selectTeamIndividual(teamID: StubTeams.emptyTeam.id, memberID: StubTeams.namedMember.id)

        let after = await bulkCount(stub)
        XCTAssertEqual(after, before, "知らない ID では計算しない")
        XCTAssertEqual(viewModel.attackerPreset, .aFull, "出どころは起動時のまま")
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 世代の保護

    func testStaleTeamListDoesNotOverwriteNewerOne() async throws {
        let store = StubTeams.makeStore(teams: [StubTeams.teamAlpha, StubTeams.teamBeta])
        let viewModel = await loadedViewModel(StubMaster.makeService(), store: store)
        let baseline = await store.listCallCount
        await store.setListMode(.manual)

        let first = Task { await viewModel.loadTeams() }
        try await store.waitForListCalls(count: baseline + 1)
        let second = Task { await viewModel.loadTeams() }
        try await store.waitForListCalls(count: baseline + 2)

        // 新しい方を先に解決する。
        await store.resolveList(at: baseline + 1, with: .success([StubTeams.teamBeta]))
        await second.value
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamBeta.id])

        // 古い方が後から届いても上書きしない。
        await store.resolveList(at: baseline, with: .success([StubTeams.teamAlpha]))
        await first.value
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamBeta.id],
                       "古い一覧の応答で選択肢を上書きしない")
    }

    func testStaleSpeciesDetailDoesNotOverwriteNewerTeamSelection() async throws {
        // 構築の個体を続けて呼んだとき、古い方の species 応答が後から届いても
        // 出どころ・技・要求を上書きしないこと(P6-2a 規則7と同じ世代の保護)。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        let selectUnlearned = Task {
            await viewModel.selectTeamIndividual(
                teamID: StubTeams.teamBeta.id, memberID: StubTeams.unlearnedMoveMember.id
            )
        }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let selectNamed = Task { await self.selectNamedMember(viewModel) }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        let alphaDetail = try await stub.lookupSpecies(key: StubMaster.alpha.key)
        // 新しい方(namedMember)を先に解決する。
        await stub.resolveSpecies(at: baseline + 1, with: .success(alphaDetail))
        await selectNamed.value
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)

        // 古い方(unlearnedMoveMember)が後から解決しても何も変わらない。
        await stub.resolveSpecies(at: baseline, with: .success(alphaDetail))
        await selectUnlearned.value
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id,
                       "古い species 応答で出どころを上書きしない")
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id, "古い species 応答で技を上書きしない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.itemId, StubTeams.namedMember.itemId)
    }
}
