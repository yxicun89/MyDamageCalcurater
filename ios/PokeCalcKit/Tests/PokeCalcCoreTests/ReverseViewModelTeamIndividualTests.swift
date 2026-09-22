import XCTest

@testable import PokeCalcCore

/// 逆算画面で「構築から個体を呼び出す」(P6-2d。ADR-0501「P6-2d」4章: **両側**に効かせる)。
///
/// 用語は `ReverseViewModelTests` と同じ。与えたダメージ = `side: .defender`(自分が攻撃側)、
/// 受けたダメージ = `side: .attacker`(自分が防御側)。どちらでも `known` は自分なので、
/// どちらでも構築の個体を呼べる。既存の `ReverseViewModelTests` は一切変えない。
@MainActor
final class ReverseViewModelTeamIndividualTests: XCTestCase {

    /// 観測の入力値(値そのものに意味は無い。% としても実点数としても有効な整数)。
    private let validObservationText = "12"

    // MARK: - 補助

    private func makeStub() async -> StubPokeCalcService {
        let stub = StubMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(
        _ stub: StubPokeCalcService, store: StubTeamStore
    ) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub, teamStore: store)
        await viewModel.load()
        return viewModel
    }

    /// 逆算が呼ばれる状態にする(有効な観測を1つ入れる)。
    private func enterObservation(_ viewModel: ReverseViewModel) async throws {
        let id = try XCTUnwrap(viewModel.observations.first?.id, "観測の行が無い")
        await viewModel.editObservation(id: id, text: validObservationText)
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> ReverseRequest {
        let requests = await stub.reverseRequests
        return try XCTUnwrap(requests.last, "reverse が呼ばれていない")
    }

    private func reverseCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.reverseRequests.count
    }

    private func selectNamedMember(_ viewModel: ReverseViewModel) async {
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id
        )
    }

    // MARK: - 構築の一覧

    func testLoadPopulatesTeamOptions() async {
        let store = StubTeams.makeStore()
        let viewModel = await loadedViewModel(await makeStub(), store: store)
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamAlpha.id, StubTeams.teamBeta.id])
        // `CalcViewModelTeamIndividualTests` と同じ型推論の回避(比較する値は変えていない)。
        XCTAssertEqual(
            viewModel.teamOptions.first?.members.map { $0.displayName as String? },
            [StubTeams.namedMember.nickname, StubMaster.alpha.nameJa] as [String?]
        )
    }

    func testWithoutTeamStoreTeamOptionsStayEmpty() async {
        let viewModel = ReverseViewModel(service: await makeStub())
        await viewModel.load()
        XCTAssertTrue(viewModel.teamOptions.isEmpty)
        await viewModel.loadTeams()
        XCTAssertTrue(viewModel.teamOptions.isEmpty)
        XCTAssertNil(viewModel.error)
    }

    func testTeamStoreFailureDoesNotSetScreenError() async {
        let store = StubTeams.makeStore()
        await store.setListError(PokeCalcError(code: PokeCalcError.Code.teamStoreCorrupted, message: "テスト: 壊れている"))
        let viewModel = await loadedViewModel(await makeStub(), store: store)
        XCTAssertTrue(viewModel.teamOptions.isEmpty)
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 与えたダメージ(自分が攻撃側)

    func testTeamIndividualOnDefenderSideUsesExactBuild() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        try await enterObservation(viewModel)
        let before = await reverseCount(stub)

        await selectNamedMember(viewModel)

        XCTAssertEqual(viewModel.side, .defender)
        XCTAssertNil(viewModel.attackerPreset, "構築を呼んでいる間はプリセットの選択表示を消す")
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id)
        XCTAssertEqual(viewModel.mySpeciesKey, StubTeams.namedMember.speciesKey)
        XCTAssertEqual(viewModel.myItemId, StubTeams.namedMember.itemId)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id, "個体の技がダメージ技の選択肢にあるので残る")

        let after = await reverseCount(stub)
        XCTAssertEqual(after - before, 1, "呼び出しで逆算はちょうど1回")

        let request = try await lastRequest(stub)
        XCTAssertEqual(request.side, .defender)
        XCTAssertEqual(request.known.sp, StubTeams.customSP)
        XCTAssertEqual(request.known.natureId, StubTeams.namedMember.natureId)
        XCTAssertNotEqual(request.known.natureId, StubMaster.atkUpNature.id, "A特化に作り直していない")
        XCTAssertEqual(request.known.abilityId, StubTeams.namedMember.abilityId)
        XCTAssertEqual(request.known.teraType, StubTeams.namedMember.teraType)
        XCTAssertEqual(request.known.itemId, StubTeams.namedMember.itemId)
        XCTAssertEqual(request.known.speciesKey, StubTeams.namedMember.speciesKey)
        XCTAssertNil(request.known.moveId, "技は要求本体の moveId が正")
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertNil(viewModel.error)
    }

    func testStatusMoveMemberFallsBackToFirstDamagingMoveOnDefenderSide() async throws {
        // 逆算の技の選択肢はダメージ技だけ(P6-2b 規則4)なので、変化技しか持たない個体は技が落ちる。
        // 計算画面(`CalcViewModelTeamIndividualTests`)では変化技のまま残るのと対になる。
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamBeta.id, memberID: StubTeams.statusMoveMember.id
        )

        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id)
        XCTAssertNil(viewModel.error, "技が落ちてもエラーにはしない")
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.statusMoveMember.id)
    }

    func testMemberWithoutMovesFallsBackOnDefenderSide() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        try await enterObservation(viewModel)
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamAlpha.id, memberID: StubTeams.noMoveMember.id
        )

        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(request.known.sp, StubTeams.customSP)
        XCTAssertNil(request.known.itemId)
    }

    func testCallingTeamWithoutObservationsDoesNotCalculate() async {
        // P6-2b 規則5: 計算できる状態でなければ逆算を呼ばない(入力の変更は覚えるだけ)。
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await selectNamedMember(viewModel)

        let count = await reverseCount(stub)
        XCTAssertEqual(count, 0, "観測が無いうちは逆算を呼ばない")
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id,
                       "計算しなくても出どころは覚える")
    }

    // MARK: - 受けたダメージ(自分が防御側)

    func testTeamIndividualOnAttackerSideUsesExactBuildForKnownDefender() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        await viewModel.selectSide(.attacker)
        try await enterObservation(viewModel)

        let movesBefore = viewModel.moveOptions.map(\.id)
        let moveBefore = viewModel.moveId
        let speciesCallsBefore = await stub.speciesRequests.count

        await selectNamedMember(viewModel)

        XCTAssertNil(viewModel.knownDefenderPreset, "受けたダメージ側のプリセットの選択表示が消える")
        XCTAssertEqual(viewModel.knownDefenderBuildSource.teamSelection?.memberID, StubTeams.namedMember.id)
        XCTAssertEqual(viewModel.mySpeciesKey, StubTeams.namedMember.speciesKey)
        XCTAssertEqual(viewModel.myItemId, StubTeams.namedMember.itemId)

        // 技は相手の learnset なので触らない(自分の種族の learnset を読み直しもしない)。
        XCTAssertEqual(viewModel.moveOptions.map(\.id), movesBefore, "相手の技の選択肢は変わらない")
        XCTAssertEqual(viewModel.moveId, moveBefore)
        let speciesCallsAfter = await stub.speciesRequests.count
        XCTAssertEqual(speciesCallsAfter, speciesCallsBefore,
                       "受けたダメージ側では自分の learnset を読み直さない")

        let request = try await lastRequest(stub)
        XCTAssertEqual(request.side, .attacker)
        XCTAssertEqual(request.known.sp, StubTeams.customSP, "HB(HD)プリセットの 0/32 に丸めない")
        XCTAssertEqual(request.known.natureId, StubTeams.namedMember.natureId)
        XCTAssertNotEqual(request.known.natureId, StubMaster.neutralNature.id, "無振りの無補正に作り直していない")
        XCTAssertEqual(request.known.abilityId, StubTeams.namedMember.abilityId)
        XCTAssertEqual(request.known.teraType, StubTeams.namedMember.teraType)
        XCTAssertEqual(request.known.itemId, StubTeams.namedMember.itemId)
        XCTAssertEqual(request.moveId, moveBefore)
    }

    // MARK: - 側ごとに独立した出どころ

    func testPresetClearsOnlyItsOwnSide() async {
        let viewModel = await loadedViewModel(await makeStub(), store: StubTeams.makeStore())
        await selectNamedMember(viewModel)
        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection)

        // もう一方の側のプリセットを変えても、与えたダメージ側の構築の選択は外れない。
        await viewModel.selectKnownDefenderPreset(.full)
        XCTAssertEqual(viewModel.knownDefenderPreset, .full)
        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection)

        // 自分の側のプリセットを押したときだけ外れる。
        await viewModel.selectAttackerPreset(.none)
        // `attackerPreset` は `AttackerPreset?`。素の `.none` は `Optional.none`(nil)に解決されてしまう
        // (Swift の既知の挙動。`CalcViewModelTeamIndividualTests` と同じ回避。値の意味は変えていない)。
        XCTAssertEqual(viewModel.attackerPreset, AttackerPreset.none)
        XCTAssertNil(viewModel.attackerBuildSource.teamSelection)
    }

    func testEachSideKeepsItsOwnBuildSourceAcrossSideSwitch() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        // 与えたダメージ側で構築の個体を呼ぶ。
        await selectNamedMember(viewModel)

        // 受けたダメージ側に切り替えると、そちらは既定のプリセットのまま。
        await viewModel.selectSide(.attacker)
        // `knownDefenderPreset` は `KnownDefenderPreset?`。同じ回避(上記コメント参照)。
        XCTAssertEqual(viewModel.knownDefenderPreset, KnownDefenderPreset.none)
        XCTAssertNil(viewModel.knownDefenderBuildSource.teamSelection)
        XCTAssertNotNil(viewModel.attackerBuildSource.teamSelection,
                        "側を切り替えても、もう一方の側の出どころは残る(P6-2b 規則7)")

        // 受けたダメージ側でも呼ぶ。
        await viewModel.selectTeamIndividual(
            teamID: StubTeams.teamBeta.id, memberID: StubTeams.unlearnedMoveMember.id
        )
        XCTAssertEqual(viewModel.knownDefenderBuildSource.teamSelection?.memberID,
                       StubTeams.unlearnedMoveMember.id)

        // 与えたダメージ側に戻しても、それぞれの側の出どころは保たれる。
        await viewModel.selectSide(.defender)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, StubTeams.namedMember.id)
        XCTAssertEqual(viewModel.knownDefenderBuildSource.teamSelection?.memberID,
                       StubTeams.unlearnedMoveMember.id)
        // 自分の持ち物は側の切り替えで外れる(P6-2b 規則7。構築の選択とは別の値)。
        XCTAssertNil(viewModel.myItemId)

        try await enterObservation(viewModel)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.known.sp, StubTeams.customSP)
        XCTAssertNil(request.known.itemId)
    }

    // MARK: - 知らない ID は無視する

    func testUnknownTeamOrMemberIsIgnored() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub, store: StubTeams.makeStore())
        try await enterObservation(viewModel)
        let before = await reverseCount(stub)

        await viewModel.selectTeamIndividual(teamID: "stub-team-does-not-exist", memberID: StubTeams.namedMember.id)
        await viewModel.selectTeamIndividual(teamID: StubTeams.teamAlpha.id, memberID: "stub-member-does-not-exist")

        let after = await reverseCount(stub)
        XCTAssertEqual(after, before, "知らない ID では計算しない")
        XCTAssertEqual(viewModel.attackerPreset, .aFull, "出どころは起動時のまま")
        XCTAssertNil(viewModel.error)
    }

    // MARK: - 世代の保護

    func testStaleTeamListDoesNotOverwriteNewerOne() async throws {
        let store = StubTeams.makeStore(teams: [StubTeams.teamAlpha, StubTeams.teamBeta])
        let viewModel = await loadedViewModel(await makeStub(), store: store)
        let baseline = await store.listCallCount
        await store.setListMode(.manual)

        let first = Task { await viewModel.loadTeams() }
        try await store.waitForListCalls(count: baseline + 1)
        let second = Task { await viewModel.loadTeams() }
        try await store.waitForListCalls(count: baseline + 2)

        await store.resolveList(at: baseline + 1, with: .success([StubTeams.teamBeta]))
        await second.value
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamBeta.id])

        await store.resolveList(at: baseline, with: .success([StubTeams.teamAlpha]))
        await first.value
        XCTAssertEqual(viewModel.teamOptions.map(\.id), [StubTeams.teamBeta.id],
                       "古い一覧の応答で選択肢を上書きしない")
    }
}
