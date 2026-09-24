import XCTest

@testable import PokeCalcCore

/// 計算画面: 検索結果に一度も現れていない「選択中の技」を `move(id:)`(openapi `getMove`)で解決する
/// (issue #68 の残り。ADR-0501「issue #68 の残り: getMove による選択中の技の解決」)。
///
/// 壊れていたこと: learnset の技がすべて先頭ページ(`MasterSearch.pageLimit` 件)の外にある種族を攻撃側にすると、
/// 起動直後から `moveUnavailable` で計算できない。構築に保存された個体の技が先頭ページの外だと、
/// 呼び出したときに黙って別の技(既定の技)に置き換わる。
///
/// 既定のスタブは `move(id:)` が `not_found`(解決できない)なので、ここでは `.immediate` / `.manual` を明示する。
@MainActor
final class CalcViewModelMoveLookupTests: XCTestCase {

    // MARK: - 補助

    private func loadedViewModel(
        _ stub: StubPokeCalcService, teamStore: StubTeamStore? = nil
    ) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub, teamStore: teamStore, searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func errorCode(_ error: CalcScreenError?) -> String? {
        guard case .service(let code, _) = error else { return nil }
        return code
    }

    /// `StubBulkMaster.mixedSpecies`(learnset = 先頭ページの技 + 先頭ページの外の技)で、
    /// 先頭ページの外の技だけを持つ個体。呼び出したら、その技がそのまま選ばれてほしい。
    private let hiddenMoveMember = TeamMember(
        id: "stub-member-hidden-move", speciesKey: StubBulkMaster.mixedSpecies.key, nickname: "テストこたいかくれわざ",
        moveIds: [StubBulkMaster.hiddenMove.id], natureId: StubMaster.atkUpNature.id, sp: StubTeams.customSP
    )
    private var hiddenMoveTeam: Team {
        Team(id: "stub-team-hidden-move", name: "テストこうちくかくれわざ", members: [hiddenMoveMember])
    }

    // MARK: - A2: 起動時(issue #68 の再現手順)

    /// 検索は先頭200件の技だけを返し、攻撃側(種族一覧の最初)の learnset は201件目の技だけ。
    /// → 起動時にその技を `move(id:)` で解決し、`moveUnavailable` にせず計算する。
    func testLoadResolvesTheDefaultMoveByIDWhenTheWholeLearnsetIsOutsideTheFirstPage() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMoveLookupMaster.onlyHiddenMoveLead.key)
        XCTAssertNil(viewModel.error, "getMove で解決できるので moveUnavailable にしない")
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertEqual(viewModel.selectedMove?.nameJa, StubBulkMaster.hiddenMove.nameJa, "技の名前を出せる")
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id], "解決するのは選ぶ技だけ")
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, 1, "起動時の計算はこれまでどおり1回")
        XCTAssertEqual(requests.last?.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertFalse(viewModel.rows.isEmpty)
    }

    /// 既定の技の規則(learnset の順で最初のダメージ技)は ID で解決するときも変えない。
    func testLoadKeepsTheFirstDamagingMoveRuleWhenResolvingByID() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.statusThenDamagingLead)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)

        XCTAssertNil(viewModel.error)
        XCTAssertEqual(viewModel.moveId, StubMoveLookupMaster.hiddenDamagingMove.id,
                       "learnset の先頭の変化技ではなく、最初のダメージ技を選ぶ")
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubMoveLookupMaster.hiddenStatusMove(0).id, StubMoveLookupMaster.hiddenDamagingMove.id],
                       "learnset の順に1つずつ解決し、ダメージ技が見つかったら止める")
    }

    /// learnset 全件を ID で実体化しない: 1回の操作で `move(id:)` を呼ぶのは
    /// `MasterSearch.maxMoveLookupsPerSelection` 回まで。計算画面はダメージ技が見つからなければ
    /// 規則3のとおり「learnset の最初(に解決できた技)」を選ぶ。
    func testLoadStopsLookingUpAtTheLimitAndFallsBackToTheFirstResolvedMove() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.manyStatusMovesLead)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)

        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups.count, MasterSearch.maxMoveLookupsPerSelection, "上限を超えて呼ばない")
        XCTAssertEqual(lookups, (0..<MasterSearch.maxMoveLookupsPerSelection).map { StubMoveLookupMaster.hiddenStatusMove($0).id })
        XCTAssertFalse(lookups.contains(StubMoveLookupMaster.hiddenDamagingMove.id), "上限の先にあるダメージ技は引かない")
        XCTAssertEqual(viewModel.moveId, StubMoveLookupMaster.hiddenStatusMove(0).id)
        XCTAssertNil(viewModel.error)
    }

    // MARK: - A3: 既知の技では呼ばない

    func testMoveLookupIsNotCalledWhenALearnsetMoveIsAlreadyKnown() async throws {
        // 先頭ページの技を覚える種族(StubBulkMaster)と、learnset にマスタに無い ID を含む種族(StubMaster.alpha)。
        for (name, stub) in [("先頭ページの技", StubBulkMaster.makeService()), ("未知の ID を含む learnset", StubMaster.makeService())] {
            await stub.setMoveLookupMode(.immediate)
            let viewModel = await loadedViewModel(stub)
            XCTAssertNil(viewModel.error, name)
            let lookups = await stub.moveLookups
            XCTAssertEqual(lookups, [], "\(name): 既定の技を既知の技から選べるなら getMove を呼ばない(learnset 全件を解決しない)")
        }
    }

    func testSpeciesChangeWithAKnownLearnsetMoveDoesNotLookUp() async throws {
        let stub = StubBulkMaster.makeService()
        await stub.setMoveLookupMode(.immediate)
        let viewModel = await loadedViewModel(stub)

        await viewModel.selectAttacker(speciesKey: StubBulkMaster.pageSpecies(1).key)

        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(1).id)
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [])
    }

    // MARK: - A4: 失敗したら今日の振る舞い(moveUnavailable)に戻る

    func testLookupNotFoundFallsBackToMoveUnavailable() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        // 既定の `.notFound`(getMove で解決できない)

        let viewModel = await loadedViewModel(stub)

        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id], "解決は試みる")
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable,
                       "解決できなければ今日と同じ moveUnavailable(not_found そのものは出さない)")
        let requests = await stub.bulkRequests
        XCTAssertTrue(requests.isEmpty, "技が無いので計算しない")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testLookupTransportFailureFallsBackToMoveUnavailable() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        await stub.setMoveLookupMode(.immediate)
        await stub.setMoveLookupError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト: 通信できない"))

        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable,
                       "補助の解決の失敗で画面のエラーの種類を変えない(今日と同じ moveUnavailable)")
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - A2: 構築から呼び出した個体の技(先頭ページの外)

    func testTeamIndividualWithAnUnseenMoveKeepsThatMove() async throws {
        let stub = StubBulkMaster.makeService()
        await stub.setMoveLookupMode(.immediate)
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))
        let before = await stub.bulkRequests.count

        await viewModel.selectTeamIndividual(teamID: hiddenMoveTeam.id, memberID: hiddenMoveMember.id)

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubBulkMaster.mixedSpecies.key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id,
                       "個体の技を黙って既定の技(先頭ページの技)に置き換えない")
        XCTAssertEqual(viewModel.selectedMove?.nameJa, StubBulkMaster.hiddenMove.nameJa)
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id])
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, before + 1)
        XCTAssertEqual(requests.last?.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertEqual(viewModel.attackerBuildSource.teamSelection?.memberID, hiddenMoveMember.id)
        XCTAssertNil(viewModel.error)
    }

    func testTeamIndividualLookupNotFoundFallsBackToTheDefaultMove() async throws {
        let stub = StubBulkMaster.makeService()
        // 既定の `.notFound`
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))

        await viewModel.selectTeamIndividual(teamID: hiddenMoveTeam.id, memberID: hiddenMoveMember.id)

        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(0).id,
                       "解決できなければ今日と同じく既定の技(learnset の順で最初の既知のダメージ技)")
        XCTAssertNil(viewModel.error)
    }

    // MARK: - A5: 世代・キャンセル

    /// 解決の応答を待っている間に次の入力が来たら、遅れて届いた応答は画面の状態を書き換えない。
    func testStaleLookupResponseDoesNotOverwriteTheNewerSelection() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))
        await stub.setMoveLookupMode(.manual)

        let older = Task {
            await viewModel.selectTeamIndividual(teamID: self.hiddenMoveTeam.id, memberID: self.hiddenMoveMember.id)
        }
        try await stub.waitForMoveLookups(count: 1)

        // 新しい入力(先頭ページの種族。技は既知なので解決は要らない)
        await viewModel.selectAttacker(speciesKey: StubBulkMaster.pageSpecies(2).key)
        let afterNewer = await stub.bulkRequests.count
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id)

        await stub.resolveMoveLookup(at: 0, with: .success(StubBulkMaster.hiddenMove))
        await older.value

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id, "古い操作の解決結果で技を戻さない")
        XCTAssertNil(viewModel.attackerBuildSource.teamSelection, "古い操作で構築の個体に切り替えない")
        let afterStale = await stub.bulkRequests.count
        XCTAssertEqual(afterStale, afterNewer, "古い操作は計算しない")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
    }

    /// 画面破棄(`cancelPendingWork()`)で解決中の Task を止めても、キャンセルを画面のエラーにしない。
    func testCancellingDuringLookupIsNotAScreenError() async throws {
        let stub = StubBulkMaster.makeService()
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))
        let previousRows = viewModel.rows.map(\.presetLabel)
        XCTAssertFalse(previousRows.isEmpty, "起動時の計算結果が要る")
        let before = await stub.bulkRequests.count
        await stub.setMoveLookupMode(.manual)

        let pending = viewModel.scheduleLatest { viewModel in
            await viewModel.selectTeamIndividual(teamID: self.hiddenMoveTeam.id, memberID: self.hiddenMoveMember.id)
        }
        try await stub.waitForMoveLookups(count: 1)

        viewModel.cancelPendingWork()
        try await stub.waitForMoveLookupCancellation(at: 0)
        await pending.value

        XCTAssertNil(viewModel.error, "キャンセルは画面のエラーではない(issue #113 A5)")
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), previousRows, "前の結果を消さない")
        XCTAssertFalse(viewModel.isLoading)
        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before, "キャンセルされた操作は計算しない")
    }

    // MARK: - A6: 持ち物の一覧が上限に達したら黙って切り捨てない

    func testItemOptionsReachingThePageLimitAreFlagged() async throws {
        let stub = StubMoveLookupMaster.makeService(
            lead: StubBulkMaster.pageSpecies(0), items: StubMoveLookupMaster.pageItemList
        )
        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.itemOptions.count, MasterSearch.pageLimit)
        XCTAssertTrue(viewModel.itemOptionsReachedLimit, "上限ちょうど = まだ他にあるかもしれない、と画面に伝える")
    }

    func testItemOptionsBelowThePageLimitAreNotFlagged() async throws {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        XCTAssertFalse(viewModel.itemOptionsReachedLimit)
    }

    func testItemsTruncatedLabelIsDistinctFromTheSearchLabels() {
        XCTAssertFalse(MasterSearchLabels.itemsTruncated.isEmpty)
        XCTAssertFalse(
            [MasterSearchLabels.prompt, MasterSearchLabels.truncated, MasterSearchLabels.noMatch]
                .contains(MasterSearchLabels.itemsTruncated),
            "持ち物は名前で絞れないので「入力して絞り込んで」とは別の文言"
        )
    }

    func testMoveLookupLimitIsPositiveAndSmall() {
        XCTAssertGreaterThan(MasterSearch.maxMoveLookupsPerSelection, 0)
        XCTAssertLessThanOrEqual(MasterSearch.maxMoveLookupsPerSelection, TeamLimits.maxMovesPerMember,
                                 "1操作あたりの getMove は技スロット数を超えない")
    }

    // MARK: - critic 指摘(OPTIONAL): stage-2(既定の技)の learnset 解決ループの世代保護

    /// `reselectMove` の stage-2(`moveOptions` から既定が決まらないときの learnset 解決ループ)は、
    /// 各 `move(id:)` の `await` の後にも `token == latestRequestToken` を確かめる(A5)。ここを削ると、
    /// 起動時の解決(stage-2)が保留中に別の種族へ切り替えても、遅れて届いた解決結果が新しい選択を
    /// 上書きしてしまう(critic 指摘。stage-1 の世代保護は `testStaleLookupResponseDoesNotOverwriteTheNewerSelection`
    /// が固定しているが、stage-2 は別の guard なので、そちらは別のテストで固定する必要があった)。
    func testStaleStageTwoLookupResponseDoesNotOverwriteTheNewerSelection() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        await stub.setMoveLookupMode(.manual)
        let viewModel = CalcViewModel(service: stub, teamStore: nil, searchDebounce: .zero)

        let loadTask = Task { await viewModel.load() }
        try await stub.waitForMoveLookups(count: 1)

        // 起動時の stage-2 解決が保留中に、既知の技を持つ別の種族へ切り替える(新しい操作)。
        await viewModel.selectAttacker(speciesKey: StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id)

        // 保留中だった起動時の解決が、いま遅れて届く。
        await stub.resolveMoveLookup(at: 0, with: .success(StubBulkMaster.hiddenMove))
        await loadTask.value

        XCTAssertEqual(viewModel.attackerSpeciesKey, StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id, "古い起動時の解決で技を戻さない(stage-2 の世代保護)")
    }
}
