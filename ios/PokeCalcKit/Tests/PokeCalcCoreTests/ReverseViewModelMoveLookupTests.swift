import XCTest

@testable import PokeCalcCore

/// 逆算画面: 検索結果に一度も現れていない「選択中の技」を `move(id:)`(openapi `getMove`)で解決する
/// (issue #68 の残り。ADR-0501「issue #68 の残り: getMove による選択中の技の解決」)。
///
/// `CalcViewModelMoveLookupTests` と同じ規則。違いは1つだけ: 逆算の技の候補は**ダメージ技だけ**なので、
/// 上限まで解決してもダメージ技が見つからなければ `moveUnavailable`(変化技にはフォールバックしない)。
/// 与えたダメージ(`side: .defender`)では攻撃側 = 自分(種族一覧の最初)。
@MainActor
final class ReverseViewModelMoveLookupTests: XCTestCase {

    /// 観測の入力値(値そのものに意味は無い。% として有効な整数)。
    private let validObservationText = "40"

    // MARK: - 補助

    private func makeStub(lead: SpeciesDetail) async -> StubPokeCalcService {
        let stub = StubMoveLookupMaster.makeService(lead: lead, natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(
        _ stub: StubPokeCalcService, teamStore: StubTeamStore? = nil
    ) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub, teamStore: teamStore, searchDebounce: .zero, calcDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func enterObservation(_ viewModel: ReverseViewModel) async throws {
        let id = try XCTUnwrap(viewModel.observations.first?.id, "観測の行が無い")
        await viewModel.editObservation(id: id, text: validObservationText)
    }

    private func errorCode(_ error: CalcScreenError?) -> String? {
        guard case .service(let code, _) = error else { return nil }
        return code
    }

    private let hiddenMoveMember = TeamMember(
        id: "stub-member-hidden-move", speciesKey: StubBulkMaster.mixedSpecies.key, nickname: "テストこたいかくれわざ",
        moveIds: [StubBulkMaster.hiddenMove.id], natureId: StubMaster.atkUpNature.id, sp: StubTeams.customSP
    )
    private var hiddenMoveTeam: Team {
        Team(id: "stub-team-hidden-move", name: "テストこうちくかくれわざ", members: [hiddenMoveMember])
    }

    // MARK: - A2: 起動時(issue #68 の再現手順)

    func testLoadResolvesTheDefaultMoveByIDWhenTheWholeLearnsetIsOutsideTheFirstPage() async throws {
        let stub = await makeStub(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.mySpeciesKey, StubMoveLookupMaster.onlyHiddenMoveLead.key)
        XCTAssertNil(viewModel.error, "getMove で解決できるので moveUnavailable にしない")
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertEqual(viewModel.selectedMove?.nameJa, StubBulkMaster.hiddenMove.nameJa)
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id])

        // 観測を入れたら、その技で逆算する(エラーを残さない)
        try await enterObservation(viewModel)
        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.count, 1)
        XCTAssertEqual(requests.last?.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertNil(viewModel.error)
        XCTAssertNotNil(viewModel.result)
    }

    func testLoadSkipsStatusMovesWhenResolvingByID() async throws {
        let stub = await makeStub(lead: StubMoveLookupMaster.statusThenDamagingLead)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)

        XCTAssertNil(viewModel.error)
        XCTAssertEqual(viewModel.moveId, StubMoveLookupMaster.hiddenDamagingMove.id, "逆算の技はダメージ技だけ")
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubMoveLookupMaster.hiddenStatusMove(0).id, StubMoveLookupMaster.hiddenDamagingMove.id])
    }

    /// 上限まで解決してもダメージ技が無ければ、今日と同じ `moveUnavailable`(名前で検索すれば復帰できる)。
    func testLoadStopsLookingUpAtTheLimitAndReportsMoveUnavailable() async throws {
        let stub = await makeStub(lead: StubMoveLookupMaster.manyStatusMovesLead)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)

        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, (0..<MasterSearch.maxMoveLookupsPerSelection).map { StubMoveLookupMaster.hiddenStatusMove($0).id },
                       "上限を超えて呼ばない(learnset 全件を解決しない)")
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - A3: 既知の技では呼ばない

    func testMoveLookupIsNotCalledWhenALearnsetMoveIsAlreadyKnown() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setMoveLookupMode(.immediate)

        let viewModel = await loadedViewModel(stub)
        await viewModel.selectMySpecies(key: StubBulkMaster.pageSpecies(1).key)

        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(1).id)
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [])
    }

    // MARK: - A4: 失敗したら今日の振る舞いに戻る

    func testLookupNotFoundFallsBackToMoveUnavailable() async throws {
        let stub = await makeStub(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        // 既定の `.notFound`

        let viewModel = await loadedViewModel(stub)

        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id])
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - A2: 構築から呼び出した個体の技(与えたダメージ)

    func testTeamIndividualWithAnUnseenMoveKeepsThatMove() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        await stub.setMoveLookupMode(.immediate)
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))
        try await enterObservation(viewModel)
        let before = await stub.reverseRequests.count

        await viewModel.selectTeamIndividual(teamID: hiddenMoveTeam.id, memberID: hiddenMoveMember.id)

        XCTAssertEqual(viewModel.mySpeciesKey, StubBulkMaster.mixedSpecies.key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id, "個体の技を黙って既定の技に置き換えない")
        let lookups = await stub.moveLookups
        XCTAssertEqual(lookups, [StubBulkMaster.hiddenMove.id])
        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.count, before + 1)
        XCTAssertEqual(requests.last?.moveId, StubBulkMaster.hiddenMove.id)
        XCTAssertNil(viewModel.error)
    }

    // MARK: - A5: 世代・キャンセル

    func testStaleLookupResponseDoesNotOverwriteTheNewerSelection() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))
        await stub.setMoveLookupMode(.manual)

        let older = Task {
            await viewModel.selectTeamIndividual(teamID: self.hiddenMoveTeam.id, memberID: self.hiddenMoveMember.id)
        }
        try await stub.waitForMoveLookups(count: 1)

        await viewModel.selectMySpecies(key: StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id)

        await stub.resolveMoveLookup(at: 0, with: .success(StubBulkMaster.hiddenMove))
        await older.value

        XCTAssertEqual(viewModel.mySpeciesKey, StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id, "古い操作の解決結果で技を戻さない")
        XCTAssertNil(viewModel.attackerBuildSource.teamSelection, "古い操作で構築の個体に切り替えない")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
    }

    func testCancellingDuringLookupIsNotAScreenError() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        let viewModel = await loadedViewModel(stub, teamStore: StubTeams.makeStore(teams: [hiddenMoveTeam]))
        try await enterObservation(viewModel)
        let previousText = try XCTUnwrap(viewModel.result?.exactCountText, "前の結果が要る")
        let before = await stub.reverseRequests.count
        await stub.setMoveLookupMode(.manual)

        let pending = viewModel.scheduleLatest { viewModel in
            await viewModel.selectTeamIndividual(teamID: self.hiddenMoveTeam.id, memberID: self.hiddenMoveMember.id)
        }
        try await stub.waitForMoveLookups(count: 1)

        viewModel.cancelPendingWork()
        try await stub.waitForMoveLookupCancellation(at: 0)
        await pending.value

        XCTAssertNil(viewModel.error, "キャンセルは画面のエラーではない(issue #113 A5)")
        XCTAssertEqual(viewModel.result?.exactCountText, previousText, "前の結果を消さない")
        XCTAssertFalse(viewModel.isLoading)
        let after = await stub.reverseRequests.count
        XCTAssertEqual(after, before, "キャンセルされた操作は逆算しない")
    }

    // MARK: - A6: 持ち物の一覧が上限に達したら黙って切り捨てない

    func testItemOptionsReachingThePageLimitAreFlagged() async throws {
        let stub = StubMoveLookupMaster.makeService(
            lead: StubBulkMaster.pageSpecies(0), items: StubMoveLookupMaster.pageItemList, natures: StubMaster.reverseNatures
        )
        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.itemOptions.count, MasterSearch.pageLimit)
        XCTAssertTrue(viewModel.itemOptionsReachedLimit)
    }

    func testItemOptionsBelowThePageLimitAreNotFlagged() async throws {
        let viewModel = await loadedViewModel(StubMaster.makeService(natures: StubMaster.reverseNatures))
        XCTAssertFalse(viewModel.itemOptionsReachedLimit)
    }

    // MARK: - critic 指摘: `recalculateIfPossible` の古いエラーの消し方(`selectedMove != nil` だけでは不足)

    /// 回帰: `moveId` が「もう攻撃側の learnset に無い、直前の種族の技」を指したまま残ることがある
    /// (`reselectMove` が `moveUnavailable` を投げても `moveId` は書き換えない)。このとき、その技は
    /// まだ辞書には残っている(前の種族のときに解決済みだったため)ので、`selectedMove != nil` だけで
    /// 判定すると、無関係な入力(観測とは無関係な `selectMyItem` 等)のたびに `moveUnavailable` を
    /// 誤って消してしまう。判定には「いまの攻撃側の learnset(ID 集合)にある」ことも要る。
    func testStaleResolvedMoveDoesNotClearMoveUnavailableAfterASpeciesChange() async throws {
        let stub = StubBulkMaster.makeService(natures: StubMaster.reverseNatures)
        // 既定の `.notFound`(hiddenSpecies の learnset を解決できない)。
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(0).id, "前提: 種族変更前の既定の技")

        // hiddenSpecies を検索で一度でも見た種族にしてから選ぶ(`selectMySpecies` は辞書にある種族しか選べない)。
        viewModel.setSpeciesQuery(StubBulkMaster.hiddenSpeciesQuery)
        await viewModel.runSpeciesSearch()
        await viewModel.selectMySpecies(key: StubBulkMaster.hiddenSpecies.key)

        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable, "前提: 技を解決できず moveUnavailable")
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(0).id, "moveId は書き換わらず、前の種族の技のまま残る")

        // 技とは無関係の入力(観測は空のまま)。`moveId`(pageMove(0))は辞書にまだ残っているが、
        // いまの攻撃側(hiddenSpecies)の learnset には無い。
        await viewModel.selectMyItem(id: nil)

        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable,
                       "解決できていない moveUnavailable を、無関係な入力で消してはいけない(critic 指摘の回帰)")
    }

    /// 変更の理由: 技が `getMove` で解決できていれば(いまの攻撃側の learnset にありダメージ技なら)、
    /// その技は `moveOptions`(検索結果 ∩ learnset)には入らない(5章 A7)。したがって、この判定を
    /// `moveOptions.contains` に戻すと、ID 解決した技については無関係な失敗(例: 前回の `reverse` の
    /// 通信失敗)の残りをいつまでも消せなくなる。`selectedMove` + 攻撃側の learnset の ID 集合 + 分類の
    /// 3条件なら、ID 解決した技でも正しく消せる。
    func testResolvedMoveStillClearsAStaleReverseFailureWithNoObservations() async throws {
        let stub = await makeStub(lead: StubMoveLookupMaster.onlyHiddenMoveLead)
        await stub.setMoveLookupMode(.immediate)
        await stub.setReverseResponder { _ in
            .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト: 通信できない"))
        }

        let viewModel = await loadedViewModel(stub)
        XCTAssertNil(viewModel.error, "前提: 起動時に getMove で解決できている")
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.hiddenMove.id)

        // 観測を入れると reverse が失敗し、通信エラーが立つ(この技は moveOptions には無い)。
        // `PokeCalcError.Code.transport` は `CalcScreenError.transport`(`.service` ではない)に写る
        // ので、`errorCode`(`.service` 専用)ではなく直接比較する。
        try await enterObservation(viewModel)
        XCTAssertEqual(viewModel.error, .transport, "前提: 無関係な reverse 失敗でエラーが立つ")

        // 観測を消す(計算しない状態に戻る)。技はいまも getMove で解決済み・攻撃側の learnset にあり・
        // ダメージ技なので、無関係な失敗の残りを持ち越さない。
        let id = try XCTUnwrap(viewModel.observations.first?.id)
        await viewModel.removeObservation(id: id)

        XCTAssertNil(viewModel.error,
                      "getMove で解決した技はいまも有効なので、無関係な reverse 失敗の残りを消してよい" +
                      "(moveOptions.contains に戻すと、この技は moveOptions に無いため red になる)")
    }

    // MARK: - critic 指摘(OPTIONAL): stage-2(既定の技)の learnset 解決ループの世代保護

    /// `reselectMove` の stage-2(`moveOptions` から既定が決まらないときの learnset 解決ループ)は、
    /// 各 `move(id:)` の `await` の後にも `token == latestRequestToken` を確かめる(A5)。ここを削ると、
    /// 起動時の解決(stage-2)が保留中に別の種族へ切り替えても、遅れて届いた解決結果が新しい選択を
    /// 上書きしてしまう(critic 指摘。stage-1 の世代保護は `testStaleLookupResponseDoesNotOverwriteTheNewerSelection`
    /// が固定しているが、stage-2 は別の guard なので、そちらは別のテストで固定する必要があった)。
    func testStaleStageTwoLookupResponseDoesNotOverwriteTheNewerSelection() async throws {
        let stub = StubMoveLookupMaster.makeService(lead: StubMoveLookupMaster.onlyHiddenMoveLead, natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        await stub.setMoveLookupMode(.manual)
        let viewModel = ReverseViewModel(service: stub, teamStore: nil, searchDebounce: .zero, calcDebounce: .zero)

        let loadTask = Task { await viewModel.load() }
        try await stub.waitForMoveLookups(count: 1)

        // 起動時の stage-2 解決が保留中に、既知の技を持つ別の種族へ切り替える(新しい操作)。
        await viewModel.selectMySpecies(key: StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id)

        // 保留中だった起動時の解決が、いま遅れて届く。
        await stub.resolveMoveLookup(at: 0, with: .success(StubBulkMaster.hiddenMove))
        await loadTask.value

        XCTAssertEqual(viewModel.mySpeciesKey, StubBulkMaster.pageSpecies(2).key)
        XCTAssertEqual(viewModel.moveId, StubBulkMaster.pageMove(2).id, "古い起動時の解決で技を戻さない(stage-2 の世代保護)")
    }
}
