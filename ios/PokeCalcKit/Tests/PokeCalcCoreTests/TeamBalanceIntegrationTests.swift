import XCTest

@testable import PokeCalcCore

// 構築編集の中の「タイプバランス」(ADR-0421・ADR-0422・ADR-0532)。
//
// 決めた形:
// - `TeamEditViewModel.balanceParty: [TeamMember]`: 種族を選んだ枠のメンバーだけを枠の順に(空き枠は入れない)。
//   画面はこれを `BalanceViewModel.setParty` に渡す(構築側はバランスの内部を知らない。ADR-0421 §3)。
// - `TeamEditViewModel.hasEmptySlot: Bool`: おすすめのタイルを押せるか(空き枠が 1 つでもある)。
//   タイルを押したら `addMember(speciesKey:)`(最初の空き枠に入る。満杯なら入れず false)。
// - `BalanceViewModel.retry(party:) async`: 節の「再計算」。下書きが今のメンバーと違えば引き直し、同じなら計算し直す。
// - `BalanceViewModel(recommendationsLimit:)`: 構築の節は `BalanceViewModel.teamRecommendationsLimit`(5)で呼ぶ(ADR-0422 §1)。
// - balance が失敗しても、構築の編集・保存は止まらない(エラーは BalanceViewModel の側にだけ立つ。ADR-0421 §2)。
@MainActor
final class TeamBalanceIntegrationTests: XCTestCase {
    private func makeTeamViewModel(store: StubTeamStore = StubTeamStore()) async -> TeamEditViewModel {
        let viewModel = TeamEditViewModel(
            store: store, service: StubMaster.makeService(),
            team: Team(id: "team-1", name: TeamNaming.defaultName, members: []), searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func makeBalanceViewModel(
        _ balance: StubBalanceService, recommendationsLimit: Int? = nil
    ) async -> BalanceViewModel {
        let viewModel = BalanceViewModel(
            balance: balance, master: StubMaster.makeService(), refreshDebounce: .zero, recommendationsDebounce: .zero,
            recommendationsLimit: recommendationsLimit)
        await viewModel.load()
        return viewModel
    }

    private func settle(_ viewModel: BalanceViewModel, file: StaticString = #filePath, line: UInt = #line) async {
        for _ in 0..<10000
        where viewModel.isLoadingAnalysis || viewModel.isLoadingCoverage || viewModel.isLoadingRecommendations {
            try? await Task.sleep(for: .milliseconds(1))
        }
        XCTAssertFalse(viewModel.isLoadingAnalysis || viewModel.isLoadingCoverage, "計算中が終わらない", file: file, line: line)
    }

    // MARK: - 下書き → setParty

    func testBalancePartyIgnoresEmptySlotsAndKeepsSlotOrder() async {
        let team = await makeTeamViewModel()
        await team.selectSpecies(slot: 4, speciesKey: StubMaster.beta.key)
        await team.selectSpecies(slot: 1, speciesKey: StubMaster.alpha.key)

        XCTAssertEqual(team.balanceParty.map(\.speciesKey), [StubMaster.alpha.key, StubMaster.beta.key], "空き枠は無視し、枠の順")

        let balance = StubBalanceService()
        let viewModel = await makeBalanceViewModel(balance)
        await viewModel.setParty(team.balanceParty)
        await settle(viewModel)

        XCTAssertEqual(viewModel.members.map(\.speciesKey), [StubMaster.alpha.key, StubMaster.beta.key])
        let sent = await balance.analyzeRequests.last
        XCTAssertEqual(sent?.map(\.pokemonId), [StubMaster.alpha.key, StubMaster.beta.key])
        XCTAssertNotNil(viewModel.analysis)
    }

    func testEmptyDraftHasNoPartyAndRequestsNothing() async {
        let team = await makeTeamViewModel()
        XCTAssertTrue(team.balanceParty.isEmpty)

        let balance = StubBalanceService()
        let viewModel = await makeBalanceViewModel(balance)
        await viewModel.setParty(team.balanceParty)

        XCTAssertNil(viewModel.analysis)
        let count = await balance.analyzeRequests.count
        XCTAssertEqual(count, 0, "0 体なら呼ばない")
    }

    func testOneMemberWithoutMovesStillGetsAResult() async {
        let team = await makeTeamViewModel()
        await team.selectSpecies(slot: 0, speciesKey: StubMaster.gamma.key)
        XCTAssertEqual(team.balanceParty.first?.moveIds, [], "技 0 個のまま")

        let balance = StubBalanceService()
        let viewModel = await makeBalanceViewModel(balance)
        await viewModel.setParty(team.balanceParty)
        await settle(viewModel)

        XCTAssertNotNil(viewModel.analysis, "1 体・技 0 個でも結果が出る")
        XCTAssertNotNil(viewModel.coverage)
        let coverageSent = await balance.coverageRequests.last
        XCTAssertEqual(coverageSent?.first?.moveIds, [])
    }

    func testDraftEditsFollowedByRemovalUpdateTheParty() async {
        let team = await makeTeamViewModel()
        await team.selectSpecies(slot: 0, speciesKey: StubMaster.alpha.key)
        await team.selectSpecies(slot: 1, speciesKey: StubMaster.beta.key)
        let balance = StubBalanceService()
        let viewModel = await makeBalanceViewModel(balance)
        await viewModel.setParty(team.balanceParty)
        await settle(viewModel)

        team.removeSlot(0)
        await viewModel.setParty(team.balanceParty)
        await settle(viewModel)

        XCTAssertEqual(viewModel.members.map(\.speciesKey), [StubMaster.beta.key], "枠を外すと結果が追随する")
        let sent = await balance.analyzeRequests.last
        XCTAssertEqual(sent?.map(\.pokemonId), [StubMaster.beta.key])
    }

    // MARK: - balance が使えなくても構築は止まらない

    func testBalanceFailureDoesNotStopEditingOrSaving() async {
        let store = StubTeamStore()
        let team = await makeTeamViewModel(store: store)
        await team.selectSpecies(slot: 0, speciesKey: StubMaster.alpha.key)
        let balance = StubBalanceService()
        let unavailable = PokeCalcError(code: PokeCalcError.Code.transport, message: "offline")
        await balance.setAnalyzeError(unavailable)
        await balance.setCoverageError(unavailable)
        await balance.setRecommendationsError(unavailable)
        let viewModel = await makeBalanceViewModel(balance)

        await viewModel.setParty(team.balanceParty)
        await settle(viewModel)

        XCTAssertEqual(viewModel.analysisError?.code, BalanceErrorText.unavailableCode)
        XCTAssertEqual(viewModel.coverageError?.code, BalanceErrorText.unavailableCode)
        // 構築の編集・保存は続けられる。
        await team.selectSpecies(slot: 1, speciesKey: StubMaster.beta.key)
        guard let id = team.member(atSlot: 0)?.id else { return XCTFail("1 体目が無い") }
        XCTAssertTrue(team.addMove(id: id, moveId: StubMaster.physicalMove.id))
        let saved = await team.save()
        XCTAssertTrue(saved)
        XCTAssertNil(team.error, "構築の側にエラーは立たない")
        let stored = try? await store.get(id: "team-1")
        XCTAssertEqual(stored?.members.count, 2)
    }

    func testRetryAfterFailureRequestsAgainAndShowsResult() async {
        let team = await makeTeamViewModel()
        await team.selectSpecies(slot: 0, speciesKey: StubMaster.alpha.key)
        let balance = StubBalanceService()
        await balance.setAnalyzeError(PokeCalcError(code: "overloaded", message: "busy"))
        let viewModel = await makeBalanceViewModel(balance)
        await viewModel.setParty(team.balanceParty)
        await settle(viewModel)
        XCTAssertNotNil(viewModel.analysisError)
        let before = await balance.analyzeRequests.count

        await balance.setAnalyzeError(nil)
        await viewModel.retry(party: team.balanceParty)
        await settle(viewModel)

        let after = await balance.analyzeRequests.count
        XCTAssertEqual(after, before + 1, "同じ下書きでも再計算は送り直す")
        XCTAssertNil(viewModel.analysisError)
        XCTAssertNotNil(viewModel.analysis)
    }

    func testRetryAfterMasterFailureResolvesTheDraftAgain() async {
        let balance = StubBalanceService()
        let master = StubMaster.makeService()
        let viewModel = BalanceViewModel(balance: balance, master: master, refreshDebounce: .zero)
        await viewModel.load()
        let party = [TeamMember(speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")]
        await master.setMasterError(PokeCalcError(code: "master_unavailable", message: "down"))
        await viewModel.setParty(party)
        XCTAssertNotNil(viewModel.masterError)
        XCTAssertTrue(viewModel.members.isEmpty)

        await master.setMasterError(nil)
        await viewModel.retry(party: party)
        await settle(viewModel)

        XCTAssertNil(viewModel.masterError, "引き直せたらエラーを消す")
        XCTAssertEqual(viewModel.members.map(\.speciesKey), [StubMaster.alpha.key])
        XCTAssertNotNil(viewModel.analysis)
    }

    // MARK: - おすすめ(limit 5)

    func testTeamSectionRequestsFiveRecommendations() async {
        XCTAssertEqual(BalanceViewModel.teamRecommendationsLimit, 5)
        let balance = StubBalanceService()
        let viewModel = await makeBalanceViewModel(balance, recommendationsLimit: BalanceViewModel.teamRecommendationsLimit)
        await viewModel.setParty([TeamMember(speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")])
        await balance.waitForRecommendationsRequests(1)

        let sent = await balance.recommendationsRequests.last
        XCTAssertEqual(sent?.limit, 5)
    }

    // MARK: - タイルを押して枠に入れる

    func testPickedPokemonGoesToTheFirstEmptySlot() async {
        let team = await makeTeamViewModel()
        await team.selectSpecies(slot: 0, speciesKey: StubMaster.alpha.key)
        await team.selectSpecies(slot: 1, speciesKey: StubMaster.beta.key)
        await team.selectSpecies(slot: 2, speciesKey: StubMaster.alpha.key)
        team.removeSlot(1)
        XCTAssertTrue(team.hasEmptySlot)

        let added = await team.addMember(speciesKey: StubMaster.gamma.key)

        XCTAssertTrue(added)
        XCTAssertEqual(team.member(atSlot: 1)?.speciesKey, StubMaster.gamma.key, "最初の空き枠(2 体目)に入る")
        XCTAssertNil(team.member(atSlot: 3))
        XCTAssertEqual(
            team.balanceParty.map(\.speciesKey), [StubMaster.alpha.key, StubMaster.gamma.key, StubMaster.alpha.key],
            "入れた後の下書きがそのままバランスの入力になる")
    }

    func testPickWhenAllSlotsAreFilledAddsNothing() async {
        let team = await makeTeamViewModel()
        for slot in 0..<TeamLimits.maxMembers {
            await team.selectSpecies(slot: slot, speciesKey: StubMaster.alpha.key)
        }
        XCTAssertFalse(team.hasEmptySlot, "満杯ならタイルは押せない")
        let before = team.team.members

        let added = await team.addMember(speciesKey: StubMaster.gamma.key)

        XCTAssertFalse(added)
        XCTAssertEqual(team.team.members, before, "満杯なら何も変えない")
        XCTAssertEqual(team.teamError, .tooManyMembers)
    }
}
