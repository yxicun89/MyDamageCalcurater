import XCTest

@testable import PokeCalcCore

// BalanceViewModel 第3段(仮想敵・おすすめタイプ・技範囲チェッカー。ADR-0415 §8)。
// マスタは構築画面と同じ `PokeCalcService`(`StubMaster`)、balance は `StubBalanceService`。画面の表示は応答のまま。
//
// 決めた形(第1・2段の `BalanceViewModelTests` の流儀を踏襲。各機能は独立した世代カウンタを持つ):
// - 仮想敵: `threatMembers: [BalanceMember]`(最大 `TeamLimits.maxMembers`)・`threatError: TeamFieldError?`・
//   `addThreat(speciesKey:) async -> Bool`・`removeThreat(id:)`。特性・技の操作(`setAbility`/`addMove`/`removeMove`)と
//   候補(`abilityOptionsByMember`/`moveOptionsByMember`)はメンバーと同じ id 引きで仮想敵にも効く(技は1体4つ・重複不可)。
//   結果: `threats: BalanceThreatsAnalysis?`・`isLoadingThreats`・`threatsError`。`refreshThreats() async` /
//   `scheduleThreatsRefresh()`(仮想敵の編集は threats だけ再計算)。自分のメンバーの編集は `refresh()` が threats も呼ぶ。
//   メンバー0体または仮想敵0体のときは threats を呼ばず結果を消す(Web の ADR-0303 §7 と同じ)。
// - おすすめ: `recommendations: BalanceRecommendations?`・`isLoadingRecommendations`・`recommendationsError`。
//   `refreshRecommendations() async`(メンバー0体なら呼ばず消す。`limit` は送らない=契約の既定 10)。
//   重い計算(サーバーは同時数を絞り overloaded を返す。ADR-0409)なので、メンバー編集からは
//   `recommendationsDebounce`(init の引数)だけ待つ別の debounce で呼ぶ。再試行は `refreshRecommendations()`。
// - 技範囲: `rangeMoveIds`・`rangeInputError: TeamMemberFieldError?`・`addRangeMove(moveId:) -> Bool`・`removeRangeMove(at:)`・
//   `moveRange: BalanceMoveRange?`・`isLoadingMoveRange`・`moveRangeError`・`moveRangeOptions: [Move]`
//   (直近の技検索結果から選択済みを除く)。技は1〜4・重複不可。0個なら呼ばず消す。メンバーとは無関係に呼べる。
// - 特性名: `abilityName(forID:) -> String?`。おすすめ・技範囲の応答に出たポケモンの `species(key:)` から引く
//   (先頭 `BalanceDisplayLimits.abilityNameResolveLimit` 匹まで、同じポケモンは1回だけ。引けなければ nil で画面は ID を出す)。
// - `cancel()` は第3段の保留中の要求もすべて cancel し、計算中を解除し、以後の応答を反映しない。
@MainActor
final class BalanceViewModelStage3Tests: XCTestCase {

    private let ability = StubMaster.ability
    private static let neverFires: Duration = .seconds(60)

    private func makeViewModel(
        balance: StubBalanceService = StubBalanceService(),
        master: StubPokeCalcService = StubMaster.makeService(),
        debounce: Duration = .zero, recommendationsDebounce: Duration = .zero
    ) -> BalanceViewModel {
        BalanceViewModel(
            balance: balance, master: master, refreshDebounce: debounce, recommendationsDebounce: recommendationsDebounce)
    }

    private func loadedViewModel(
        balance: StubBalanceService = StubBalanceService(),
        master: StubPokeCalcService = StubMaster.makeService(),
        debounce: Duration = .zero, recommendationsDebounce: Duration = .zero
    ) async -> BalanceViewModel {
        let viewModel = makeViewModel(
            balance: balance, master: master, debounce: debounce, recommendationsDebounce: recommendationsDebounce)
        await viewModel.load()
        return viewModel
    }

    /// 条件が満たされるまで待つ(上限あり)。
    private func waitUntil(
        _ description: String, file: StaticString = #filePath, line: UInt = #line, _ condition: () -> Bool
    ) async {
        for _ in 0..<10000 where !condition() {
            try? await Task.sleep(for: .milliseconds(1))
        }
        XCTAssertTrue(condition(), description, file: file, line: line)
    }

    private func settleAll(_ viewModel: BalanceViewModel) async {
        await waitUntil("計算中が終わらない") {
            !(viewModel.isLoadingAnalysis || viewModel.isLoadingCoverage || viewModel.isLoadingThreats
                || viewModel.isLoadingRecommendations || viewModel.isLoadingMoveRange)
        }
    }

    private func input(_ species: SpeciesDetail, ability: String? = nil, moves: [String] = []) -> BalanceMemberInput {
        BalanceMemberInput(pokemonId: species.key, abilityId: ability, moveIds: moves)
    }

    // MARK: - 仮想敵: 呼ぶ条件(Web の ADR-0303 §7 と同じ)

    func testThreatsNotRequestedWithoutPartyMember() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)

        let added = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        XCTAssertTrue(added)
        await settleAll(viewModel)

        XCTAssertEqual(viewModel.threatMembers.map(\.speciesKey), [StubMaster.beta.key])
        let requests = await balance.threatsRequests
        XCTAssertTrue(requests.isEmpty, "メンバー0体では threats を呼ばない")
        XCTAssertNil(viewModel.threats)
        XCTAssertFalse(viewModel.isLoadingThreats)
    }

    func testThreatsNotRequestedWithoutThreat() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settleAll(viewModel)

        let requests = await balance.threatsRequests
        XCTAssertTrue(requests.isEmpty, "仮想敵0体では threats を呼ばない")
        XCTAssertNil(viewModel.threats)
    }

    func testThreatsRequestedWithPartyAndThreat() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForThreatsRequests(1)
        await settleAll(viewModel)

        let requests = await balance.threatsRequests
        XCTAssertEqual(
            requests.last, StubBalanceService.ThreatsRequest(members: [input(StubMaster.alpha)], threats: [input(StubMaster.beta)]))
        XCTAssertEqual(viewModel.threats?.threats.map(\.pokemonId), [StubMaster.beta.key])
        XCTAssertNil(viewModel.threatsError)
    }

    func testThreatEditsRefreshOnlyThreats() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForThreatsRequests(1)
        await settleAll(viewModel)
        let analyzeBefore = await balance.analyzeRequests.count
        let threatsBefore = await balance.threatsRequests.count
        let threatId = viewModel.threatMembers[0].id

        XCTAssertTrue(viewModel.addMove(id: threatId, moveId: StubMaster.specialMove.id))
        viewModel.setAbility(id: threatId, abilityId: ability.id)
        await balance.waitForThreatsRequests(threatsBefore + 1)
        await settleAll(viewModel)

        let requests = await balance.threatsRequests
        XCTAssertEqual(
            requests.last?.threats, [input(StubMaster.beta, ability: ability.id, moves: [StubMaster.specialMove.id])])
        let analyzeAfter = await balance.analyzeRequests.count
        XCTAssertEqual(analyzeAfter, analyzeBefore, "仮想敵の編集では analyze を呼び直さない")
    }

    func testPartyEditRefreshesThreatsToo() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForThreatsRequests(1)
        await settleAll(viewModel)
        let before = await balance.threatsRequests.count

        viewModel.setAbility(id: viewModel.members[0].id, abilityId: ability.id)
        await balance.waitForThreatsRequests(before + 1)
        await settleAll(viewModel)

        let requests = await balance.threatsRequests
        XCTAssertEqual(requests.last?.members, [input(StubMaster.alpha, ability: ability.id)])
    }

    // MARK: - 仮想敵: 上限・0体

    func testAddThreatBeyondMaxIsRejectedWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        for _ in 0..<TeamLimits.maxMembers {
            let added = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
            XCTAssertTrue(added)
        }
        await settleAll(viewModel)
        let before = await balance.threatsRequests.count

        let added = await viewModel.addThreat(speciesKey: StubMaster.gamma.key)

        XCTAssertFalse(added)
        XCTAssertEqual(viewModel.threatMembers.count, TeamLimits.maxMembers)
        XCTAssertEqual(viewModel.threatError, .tooManyMembers)
        try? await Task.sleep(for: .milliseconds(50))
        let after = await balance.threatsRequests.count
        XCTAssertEqual(after, before, "上限超過は要求を出さない")
    }

    func testAddThreatFailureDoesNotAddAndSetsMasterError() async {
        let master = StubMaster.makeService()
        let viewModel = await loadedViewModel(master: master)
        await master.setMasterError(PokeCalcError(code: "master_unavailable", message: "english"))

        let added = await viewModel.addThreat(speciesKey: StubMaster.beta.key)

        XCTAssertFalse(added)
        XCTAssertTrue(viewModel.threatMembers.isEmpty)
        XCTAssertEqual(viewModel.masterError?.code, "master_unavailable")
    }

    func testRemovingLastThreatClearsResultWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForThreatsRequests(1)
        await settleAll(viewModel)
        XCTAssertNotNil(viewModel.threats)
        try? await Task.sleep(for: .milliseconds(50))  // メンバー追加側の refresh が threats を呼ぶ競合を待ち切る
        await settleAll(viewModel)
        let before = await balance.threatsRequests.count

        viewModel.removeThreat(id: viewModel.threatMembers[0].id)
        await settleAll(viewModel)

        XCTAssertTrue(viewModel.threatMembers.isEmpty)
        XCTAssertNil(viewModel.threats)
        XCTAssertNil(viewModel.threatsError)
        let count = await balance.threatsRequests.count
        XCTAssertEqual(count, before, "0体にしても要求は増えない")
    }

    func testRemovingAllMembersClearsThreatsAndRecommendationsWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForThreatsRequests(1)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)
        XCTAssertNotNil(viewModel.threats)
        XCTAssertNotNil(viewModel.recommendations)
        try? await Task.sleep(for: .milliseconds(50))
        await settleAll(viewModel)
        let threatsBefore = await balance.threatsRequests.count
        let recsBefore = await balance.recommendationsRequests.count

        viewModel.removeMember(id: viewModel.members[0].id)
        await settleAll(viewModel)

        XCTAssertNil(viewModel.threats)
        XCTAssertNil(viewModel.recommendations)
        XCTAssertEqual(viewModel.threatMembers.count, 1, "仮想敵の入力そのものは残す")
        let threatsCount = await balance.threatsRequests.count
        let recsCount = await balance.recommendationsRequests.count
        XCTAssertEqual(threatsCount, threatsBefore)
        XCTAssertEqual(recsCount, recsBefore)
    }

    func testThreatMoveRulesMatchMemberRules() async {
        let viewModel = await loadedViewModel()
        _ = await viewModel.addThreat(speciesKey: StubMaster.alpha.key)
        let id = viewModel.threatMembers[0].id

        XCTAssertTrue(viewModel.addMove(id: id, moveId: StubMaster.specialMove.id))
        XCTAssertFalse(viewModel.addMove(id: id, moveId: StubMaster.specialMove.id))
        XCTAssertEqual(viewModel.memberErrors[id], .duplicateMove)
        XCTAssertTrue(viewModel.addMove(id: id, moveId: StubMaster.physicalMove.id))
        XCTAssertTrue(viewModel.addMove(id: id, moveId: StubMaster.statusMove.id))
        XCTAssertTrue(viewModel.addMove(id: id, moveId: StubMaster.alphaOnlyMove.id))
        XCTAssertFalse(viewModel.addMove(id: id, moveId: "stub-fifth"))
        XCTAssertEqual(viewModel.memberErrors[id], .tooManyMoves)
        XCTAssertEqual(viewModel.threatMembers[0].moveIds.count, TeamLimits.maxMovesPerMember)
        viewModel.removeMove(id: id, at: 0)
        XCTAssertNil(viewModel.memberErrors[id])
        XCTAssertEqual(viewModel.threatMembers[0].moveIds.count, 3)
    }

    func testThreatCandidatesComeFromMasterLikeMembers() async {
        let viewModel = await loadedViewModel()
        _ = await viewModel.addThreat(speciesKey: StubMaster.alpha.key)
        let id = viewModel.threatMembers[0].id

        XCTAssertEqual(viewModel.abilityOptionsByMember[id], StubMaster.alpha.abilities)
        XCTAssertEqual(
            viewModel.moveOptionsByMember[id]?.map(\.id),
            [StubMaster.statusMove.id, StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id])
        viewModel.removeThreat(id: id)
        XCTAssertNil(viewModel.abilityOptionsByMember[id])
        XCTAssertNil(viewModel.moveOptionsByMember[id])
    }

    // MARK: - 仮想敵: エラー・独立・stale

    func testThreatsErrorIsMappedAndDoesNotStopAnalyze() async {
        let balance = StubBalanceService()
        await balance.setThreatsError(PokeCalcError(code: "unknown_move", message: "english"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForThreatsRequests(1)
        await settleAll(viewModel)

        XCTAssertEqual(viewModel.threatsError?.code, "unknown_move")
        XCTAssertEqual(viewModel.threatsError?.message, "選んだ技がサーバーのマスタにありません。選び直してください")
        XCTAssertNil(viewModel.threats)
        XCTAssertNotNil(viewModel.analysis, "threats の失敗は analyze を止めない")
        XCTAssertNil(viewModel.analysisError)
    }

    func testThreatsTransportFailureMapsToUnavailable() async {
        let balance = StubBalanceService()
        await balance.setThreatsError(PokeCalcError(code: PokeCalcError.Code.transport, message: "x"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await settleAll(viewModel)

        XCTAssertEqual(viewModel.threatsError?.message, "タイプバランスの API に接続できません")
    }

    func testStaleThreatsResponsesAreIgnored() async {
        let balance = StubBalanceService()
        await balance.setThreatsMode(.manual)
        let viewModel = await loadedViewModel(balance: balance, debounce: Self.neverFires)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        let first = Task { await viewModel.refreshThreats() }
        await balance.waitForThreatsRequests(1)
        _ = await viewModel.addThreat(speciesKey: StubMaster.gamma.key)
        let second = Task { await viewModel.refreshThreats() }
        await balance.waitForThreatsRequests(2)

        let both = [StubMaster.beta.key, StubMaster.gamma.key]
        let member = [StubMaster.alpha.key]
        await balance.resolveThreats(at: 1, with: .success(BalanceFixtures.threatsAnalysis(threatIds: both, memberIds: member)))
        await balance.resolveThreats(at: 0, with: .failure(PokeCalcError(code: "internal_error", message: "x")))
        await first.value
        await second.value
        viewModel.cancel()

        XCTAssertEqual(viewModel.threats?.threats.count, 2, "古い失敗で上書きしない")
        XCTAssertNil(viewModel.threatsError)
    }

    func testThreatsLoadingStaysUntilLatestResponse() async {
        let balance = StubBalanceService()
        await balance.setThreatsMode(.manual)
        let viewModel = await loadedViewModel(balance: balance, debounce: Self.neverFires)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        let first = Task { await viewModel.refreshThreats() }
        await balance.waitForThreatsRequests(1)
        let second = Task { await viewModel.refreshThreats() }
        await balance.waitForThreatsRequests(2)

        await balance.resolveThreats(
            at: 0, with: .success(BalanceFixtures.threatsAnalysis(threatIds: ["old"], memberIds: [])))
        await first.value
        XCTAssertTrue(viewModel.isLoadingThreats, "古い応答では計算中を下ろさない")
        XCTAssertNil(viewModel.threats)

        await balance.resolveThreats(
            at: 1, with: .success(BalanceFixtures.threatsAnalysis(threatIds: [StubMaster.beta.key], memberIds: [])))
        await second.value
        viewModel.cancel()
        XCTAssertFalse(viewModel.isLoadingThreats)
        XCTAssertEqual(viewModel.threats?.threats.map(\.pokemonId), [StubMaster.beta.key])
    }

    // MARK: - おすすめタイプ

    func testRecommendationsRequestedWithMembersAndNoLimit() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)

        let requests = await balance.recommendationsRequests
        XCTAssertEqual(requests, [StubBalanceService.RecommendationsRequest(members: [input(StubMaster.alpha)], limit: nil)])
        XCTAssertEqual(viewModel.recommendations?.defenseHoles.count, 1)
        XCTAssertNil(viewModel.recommendationsError)
    }

    func testRecommendationsDoNotIncludeThreats() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)

        let requests = await balance.recommendationsRequests
        XCTAssertEqual(requests.last?.members, [input(StubMaster.alpha)], "仮想敵は入力に含めない(Web の ADR-0303 §7)")
    }

    func testRecommendationsNotRequestedWithoutMembers() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)

        await viewModel.refreshRecommendations()

        let requests = await balance.recommendationsRequests
        XCTAssertTrue(requests.isEmpty)
        XCTAssertNil(viewModel.recommendations)
        XCTAssertFalse(viewModel.isLoadingRecommendations)
    }

    func testRecommendationsErrorIsMappedAndIndependent() async {
        let balance = StubBalanceService()
        await balance.setRecommendationsError(PokeCalcError(code: "overloaded", message: "english"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)

        XCTAssertEqual(viewModel.recommendationsError?.code, "overloaded")
        XCTAssertEqual(viewModel.recommendationsError?.message, "サーバーが混み合っています。しばらくしてからもう一度お試しください")
        XCTAssertNil(viewModel.recommendations)
        XCTAssertNotNil(viewModel.analysis, "おすすめの失敗は analyze を止めない")
        XCTAssertNotNil(viewModel.coverage)
    }

    func testRetryAfterFailureClearsErrorAndShowsResult() async {
        let balance = StubBalanceService()
        await balance.setRecommendationsError(PokeCalcError(code: "overloaded", message: "english"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)
        XCTAssertNotNil(viewModel.recommendationsError)

        await balance.setRecommendationsError(nil)
        await viewModel.refreshRecommendations()

        XCTAssertNil(viewModel.recommendationsError)
        XCTAssertNotNil(viewModel.recommendations)
    }

    func testStaleRecommendationsResponsesAreIgnored() async {
        let balance = StubBalanceService()
        await balance.setRecommendationsMode(.manual)
        let viewModel = await loadedViewModel(
            balance: balance, debounce: Self.neverFires, recommendationsDebounce: Self.neverFires)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        let first = Task { await viewModel.refreshRecommendations() }
        await balance.waitForRecommendationsRequests(1)
        _ = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        let second = Task { await viewModel.refreshRecommendations() }
        await balance.waitForRecommendationsRequests(2)

        await balance.resolveRecommendations(at: 1, with: .success(BalanceFixtures.recommendations(tag: 2)))
        await balance.resolveRecommendations(at: 0, with: .success(BalanceFixtures.recommendations(tag: 1)))
        await first.value
        await second.value
        viewModel.cancel()

        XCTAssertEqual(viewModel.recommendations?.defenseHoles.count, 2, "古い応答で上書きしない")
    }

    /// 連続操作は、おすすめ専用の debounce で最新の1要求にまとまる(重い計算を連発しない)。
    func testRapidEditsCollapseRecommendationsIntoOneRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance, debounce: .zero, recommendationsDebounce: .milliseconds(120))
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)
        let base = await balance.recommendationsRequests.count
        let id = viewModel.members[0].id

        _ = viewModel.addMove(id: id, moveId: StubMaster.specialMove.id)
        _ = viewModel.addMove(id: id, moveId: StubMaster.alphaOnlyMove.id)
        viewModel.setAbility(id: id, abilityId: ability.id)
        await balance.waitForRecommendationsRequests(base + 1)
        try? await Task.sleep(for: .milliseconds(300))
        await settleAll(viewModel)

        let requests = await balance.recommendationsRequests
        XCTAssertEqual(requests.count, base + 1)
        XCTAssertEqual(
            requests.last?.members,
            [input(StubMaster.alpha, ability: ability.id, moves: [StubMaster.specialMove.id, StubMaster.alphaOnlyMove.id])])
    }

    // MARK: - 技範囲チェッカー

    func testAddRangeMoveRequestsMoveIdsOnlyWithoutMembers() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)

        XCTAssertTrue(viewModel.addRangeMove(moveId: StubMaster.specialMove.id))
        await balance.waitForMoveRangeRequests(1)
        await settleAll(viewModel)

        let requests = await balance.moveRangeRequests
        XCTAssertEqual(requests, [[StubMaster.specialMove.id]])
        XCTAssertNotNil(viewModel.moveRange)
        XCTAssertNil(viewModel.moveRangeError)
        XCTAssertEqual(viewModel.rangeMoveIds, [StubMaster.specialMove.id])
        let analyze = await balance.analyzeRequests
        XCTAssertTrue(analyze.isEmpty, "技範囲はメンバーと無関係(analyze を呼ばない)")
    }

    func testRemovingLastRangeMoveClearsResultWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        _ = viewModel.addRangeMove(moveId: StubMaster.physicalMove.id)
        await balance.waitForMoveRangeRequests(1)
        await settleAll(viewModel)

        viewModel.removeRangeMove(at: 0)
        await balance.waitForMoveRangeRequests(2)
        await settleAll(viewModel)
        let requests = await balance.moveRangeRequests
        XCTAssertEqual(requests.last, [StubMaster.physicalMove.id])

        viewModel.removeRangeMove(at: 0)
        await settleAll(viewModel)
        XCTAssertTrue(viewModel.rangeMoveIds.isEmpty)
        XCTAssertNil(viewModel.moveRange, "0個では結果を消す")
        XCTAssertNil(viewModel.moveRangeError)
        let after = await balance.moveRangeRequests.count
        XCTAssertEqual(after, requests.count, "0個では呼ばない")
    }

    func testRangeMoveLimitsAndDuplicatesAreRejectedWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        let ids = [
            StubMaster.physicalMove.id, StubMaster.specialMove.id, StubMaster.statusMove.id, StubMaster.alphaOnlyMove.id,
        ]
        for id in ids { XCTAssertTrue(viewModel.addRangeMove(moveId: id)) }
        await settleAll(viewModel)

        XCTAssertFalse(viewModel.addRangeMove(moveId: "stub-fifth"))
        XCTAssertEqual(viewModel.rangeInputError, .tooManyMoves)
        viewModel.removeRangeMove(at: 3)
        XCTAssertNil(viewModel.rangeInputError)
        XCTAssertFalse(viewModel.addRangeMove(moveId: StubMaster.physicalMove.id))
        XCTAssertEqual(viewModel.rangeInputError, .duplicateMove)
        XCTAssertEqual(viewModel.rangeMoveIds.count, 3)
        await settleAll(viewModel)
    }

    func testMoveRangeErrorIsMappedAndPreviousResultCleared() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        await balance.waitForMoveRangeRequests(1)
        await settleAll(viewModel)
        XCTAssertNotNil(viewModel.moveRange)

        await balance.setMoveRangeError(PokeCalcError(code: "invalid_request", message: "english"))
        _ = viewModel.addRangeMove(moveId: StubMaster.statusMove.id)
        await balance.waitForMoveRangeRequests(2)
        await settleAll(viewModel)

        XCTAssertEqual(viewModel.moveRangeError?.message, "リクエストが正しくありません。入力を見直してください")
        XCTAssertNil(viewModel.moveRange)
    }

    func testStaleMoveRangeResponsesAreIgnored() async {
        let balance = StubBalanceService()
        await balance.setMoveRangeMode(.manual)
        let viewModel = await loadedViewModel(balance: balance, debounce: Self.neverFires)
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        let first = Task { await viewModel.refreshMoveRange() }
        await balance.waitForMoveRangeRequests(1)
        _ = viewModel.addRangeMove(moveId: StubMaster.physicalMove.id)
        let second = Task { await viewModel.refreshMoveRange() }
        await balance.waitForMoveRangeRequests(2)

        await balance.resolveMoveRange(at: 1, with: .success(BalanceFixtures.moveRange(attackTypes: [.fire, .normal])))
        await balance.resolveMoveRange(at: 0, with: .failure(PokeCalcError(code: "internal_error", message: "x")))
        await first.value
        await second.value
        viewModel.cancel()

        XCTAssertEqual(viewModel.moveRange?.attackTypes, [.fire, .normal])
        XCTAssertNil(viewModel.moveRangeError)
    }

    func testRapidRangeEditsCollapseIntoOneRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance, debounce: .milliseconds(80))
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        await balance.waitForMoveRangeRequests(1)
        await settleAll(viewModel)
        let base = await balance.moveRangeRequests.count

        _ = viewModel.addRangeMove(moveId: StubMaster.physicalMove.id)
        _ = viewModel.addRangeMove(moveId: StubMaster.statusMove.id)
        await balance.waitForMoveRangeRequests(base + 1)
        try? await Task.sleep(for: .milliseconds(250))
        await settleAll(viewModel)

        let requests = await balance.moveRangeRequests
        XCTAssertEqual(requests.count, base + 1)
        XCTAssertEqual(requests.last, [StubMaster.specialMove.id, StubMaster.physicalMove.id, StubMaster.statusMove.id])
    }

    func testMoveRangeOptionsExcludeSelectedMoves() async {
        let viewModel = await loadedViewModel()
        XCTAssertEqual(
            Set(viewModel.moveRangeOptions.map(\.id)),
            Set([StubMaster.physicalMove, StubMaster.specialMove, StubMaster.statusMove, StubMaster.alphaOnlyMove].map(\.id)))

        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)

        XCTAssertFalse(viewModel.moveRangeOptions.contains { $0.id == StubMaster.specialMove.id })
        XCTAssertEqual(viewModel.moveRangeOptions.count, 3)
        XCTAssertEqual(viewModel.move(forID: StubMaster.specialMove.id)?.nameJa, StubMaster.specialMove.nameJa)
    }

    // MARK: - 独立: 技範囲の失敗はメンバーの計算を止めない

    func testMoveRangeFailureDoesNotAffectMemberResults() async {
        let balance = StubBalanceService()
        await balance.setMoveRangeError(PokeCalcError(code: "master_unavailable", message: "english"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForMoveRangeRequests(1)
        await settleAll(viewModel)

        XCTAssertNotNil(viewModel.moveRangeError)
        XCTAssertNotNil(viewModel.analysis)
        XCTAssertNotNil(viewModel.coverage)
        XCTAssertNil(viewModel.analysisError)
        let rangeCount = await balance.moveRangeRequests.count
        XCTAssertEqual(rangeCount, 1, "メンバーの編集で技範囲を呼び直さない")
    }

    // MARK: - 特性名(おすすめ・技範囲の応答に出たポケモンから引く)

    private func recommendationsWithAbilityOption(pokemonIds: [String], abilityId: String) -> BalanceRecommendations {
        BalanceRecommendations(
            defenseHoles: [.fire], offenseHoles: [], candidates: [],
            abilityOptions: [
                BalanceAbilityOption(
                    attackType: .fire,
                    pokemon: pokemonIds.map {
                        BalanceAbilityOptionPokemon(pokemonId: $0, nameJa: nil, abilityId: abilityId, multiplier: "1/2")
                    })
            ])
    }

    func testAbilityNamesAreResolvedFromRecommendationPokemon() async {
        let balance = StubBalanceService()
        await balance.setRecommendationsResult(
            recommendationsWithAbilityOption(pokemonIds: [StubMaster.abilityXOnly.key], abilityId: StubMaster.abilityX.id))
        let master = StubMaster.makeService(species: [StubMaster.alpha, StubMaster.abilityXOnly])
        let viewModel = await loadedViewModel(balance: balance, master: master)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await waitUntil("特性名が引けない") { viewModel.abilityName(forID: StubMaster.abilityX.id) != nil }

        XCTAssertEqual(viewModel.abilityName(forID: StubMaster.abilityX.id), StubMaster.abilityX.nameJa)
        XCTAssertEqual(viewModel.abilityName(forID: ability.id), ability.nameJa, "メンバーの特性候補からも引ける")
        XCTAssertNil(viewModel.abilityName(forID: "unknown-ability"))
    }

    func testAbilityNamesAreResolvedFromMoveRangePokemon() async {
        let balance = StubBalanceService()
        await balance.setMoveRangeResult(
            BalanceFixtures.moveRange(
                walledByAbility: [
                    BalanceWalledByAbilityPokemon(
                        pokemonId: StubMaster.abilityYOnly.key, nameJa: nil, abilityId: StubMaster.abilityY.id, bestMultiplier: "0")
                ]))
        let master = StubMaster.makeService(species: [StubMaster.alpha, StubMaster.abilityYOnly])
        let viewModel = await loadedViewModel(balance: balance, master: master)
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        await balance.waitForMoveRangeRequests(1)
        await waitUntil("特性名が引けない") { viewModel.abilityName(forID: StubMaster.abilityY.id) != nil }

        XCTAssertEqual(viewModel.abilityName(forID: StubMaster.abilityY.id), StubMaster.abilityY.nameJa)
    }

    func testUnresolvableAbilityNameFallsBackToNilWithoutMasterError() async {
        let balance = StubBalanceService()
        await balance.setRecommendationsResult(
            recommendationsWithAbilityOption(pokemonIds: ["9999-999"], abilityId: "ability-unknown"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)
        try? await Task.sleep(for: .milliseconds(50))

        XCTAssertNil(viewModel.abilityName(forID: "ability-unknown"), "引けなければ nil(画面は ID を出す)")
        XCTAssertNil(viewModel.masterError, "特性名の解決失敗は画面のエラーにしない")
        XCTAssertNotNil(viewModel.recommendations)
    }

    func testAbilityNameResolutionIsCappedAndNotRepeated() async {
        let balance = StubBalanceService()
        let cap = BalanceDisplayLimits.abilityNameResolveLimit
        let ids = (0..<(cap + 5)).map { "9\(String(format: "%03d", $0))-000" }
        await balance.setRecommendationsResult(recommendationsWithAbilityOption(pokemonIds: ids, abilityId: "ability-unknown"))
        let master = StubMaster.makeService()
        let viewModel = await loadedViewModel(balance: balance, master: master)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForRecommendationsRequests(1)
        await settleAll(viewModel)
        try? await Task.sleep(for: .milliseconds(100))
        let requested = await master.speciesRequests.filter { ids.contains($0) }
        XCTAssertLessThanOrEqual(requested.count, cap, "解決する件数に上限がある")

        await viewModel.refreshRecommendations()
        try? await Task.sleep(for: .milliseconds(100))
        let after = await master.speciesRequests.filter { ids.contains($0) }
        XCTAssertEqual(after.count, requested.count, "同じポケモンは引き直さない")
    }

    // MARK: - cancel(画面を離れるとき)

    func testCancelAbortsStage3RequestsAndDropsResults() async {
        let balance = StubBalanceService()
        await balance.setThreatsMode(.manual)
        await balance.setRecommendationsMode(.manual)
        await balance.setMoveRangeMode(.manual)
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addThreat(speciesKey: StubMaster.beta.key)
        _ = viewModel.addRangeMove(moveId: StubMaster.specialMove.id)
        await balance.waitForThreatsRequests(1)
        await balance.waitForRecommendationsRequests(1)
        await balance.waitForMoveRangeRequests(1)

        viewModel.cancel()
        XCTAssertFalse(viewModel.isLoadingThreats)
        XCTAssertFalse(viewModel.isLoadingRecommendations)
        XCTAssertFalse(viewModel.isLoadingMoveRange)

        for _ in 0..<10000 {
            let t = await balance.cancelledThreatsRequests
            let r = await balance.cancelledRecommendationsRequests
            let m = await balance.cancelledMoveRangeRequests
            if t.contains(0) && r.contains(0) && m.contains(0) { break }
            try? await Task.sleep(for: .milliseconds(1))
        }
        let threatsCancelled = await balance.cancelledThreatsRequests
        let recommendationsCancelled = await balance.cancelledRecommendationsRequests
        let moveRangeCancelled = await balance.cancelledMoveRangeRequests
        XCTAssertTrue(threatsCancelled.contains(0))
        XCTAssertTrue(recommendationsCancelled.contains(0))
        XCTAssertTrue(moveRangeCancelled.contains(0))
        XCTAssertNil(viewModel.threats)
        XCTAssertNil(viewModel.recommendations)
        XCTAssertNil(viewModel.moveRange)
        XCTAssertNil(viewModel.threatsError, "cancel はエラーではない")
        XCTAssertNil(viewModel.recommendationsError)
        XCTAssertNil(viewModel.moveRangeError)
    }
}
