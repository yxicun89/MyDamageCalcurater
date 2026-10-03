import XCTest

@testable import PokeCalcCore

// BalanceViewModel(タイプバランス画面の状態。ADR-0415 第1段=防御相性・チーム集計、第2段=攻撃範囲)。
//
// マスタ(種族・技・特性)は構築画面と同じ `PokeCalcService` を再利用する(`StubPokeCalcService` / `StubMaster` を共用。
// 新しいマスタ取得口は作らない。架空データへのフォールバックもしない。Web の ADR-0411 と同じ)。
// balance の呼び出しは `BalanceService`(`StubBalanceService`)。画面の表示はすべて balance の応答のまま(iOS で相性を計算しない)。
//
// 決めた形:
// - `@MainActor @Observable public final class BalanceViewModel`
//   `init(balance: any BalanceService, master: any PokeCalcService, refreshDebounce: Duration = MasterSearch.debounceInterval)`。
// - マスタ: `speciesOptions: [SpeciesSummary]`・`isLoadingMaster: Bool`・`masterError: BalanceScreenError?`。
//   `load() async` は `searchSpecies("", limit: MasterSearch.pageLimit)` と `searchMoves("", limit: MasterSearch.pageLimit)` を1回ずつ呼ぶ。
//   失敗したら `masterError` を立て、選択肢は空のまま(架空データを入れない)。
// - メンバー(最大 `TeamLimits.maxMembers`): `members: [BalanceMember]`
//   (`BalanceMember { id: String; speciesKey: String; nameJa: String; types: [PokeType]; abilityId: String?; moveIds: [String] }`、
//   id は追加時に作る UUID 文字列)・`abilityOptionsByMember: [String: [Ability]]`(`species(key:)` の特性)・
//   `moveOptionsByMember: [String: [Move]]`(learnset の順で、`load()` で取った技の先頭ページにあるものだけ。構築編集と同じ規則)。
//   - `addMember(speciesKey:) async -> Bool`: 上限なら追加せず false + `teamError = .tooManyMembers`(要求は出さない)。
//     `species(key:)` が失敗したら追加せず false + `masterError`。成功したら追加して true、`scheduleRefresh()`。
//   - `removeMember(id:)`: 取り除き、`scheduleRefresh()`。
//   - `setAbility(id:abilityId:)`: 代入して `scheduleRefresh()`。
//   - `addMove(id:moveId:) -> Bool` / `removeMove(id:at:)`: 構築編集と同じ規則(最大 `TeamLimits.maxMovesPerMember`・重複不可。
//     違反は false + `memberErrors[id] = .tooManyMoves/.duplicateMove` で要求を出さない)。成功したら `scheduleRefresh()`。
//   - `loadTeam(_ team: Team) async`: 保存済みの構築(`Team`)のメンバーで入れ替える(種族・特性・技 ID を写す。
//     各メンバーの `species(key:)` で名前・タイプ・選択肢を作る)。`scheduleRefresh()`。
// - 結果: `analysis: BalanceDefenseAnalysis?`・`coverage: BalanceCoverageAnalysis?`(それぞれ最後の成功応答)・
//   `isLoadingAnalysis` / `isLoadingCoverage`・`analysisError: BalanceScreenError?` / `coverageError: BalanceScreenError?`。
//   analyze と coverage は**独立**(片方の失敗・遅延がもう片方を止めない)。
// - `refresh() async`: メンバーが0なら要求を出さず `analysis`・`coverage`・エラー・計算中をすべて消す。
//   1体以上なら analyze と coverage を**両方**、全メンバー分(`BalanceMemberInput(pokemonId: speciesKey, abilityId:, moveIds:)`)で呼ぶ。
//   呼ぶたびに計算中にし、**最新の呼び出しの応答だけ**を反映する(古い呼び出しの応答は成功でも失敗でも捨てる。
//   Web の ADR-0303 §9 と同じ)。通信失敗などで失敗したときは `xxxError` を立て、直前の `analysis`/`coverage` は残さず nil にする。
// - `scheduleRefresh()`: `LatestTaskRunner` で `refreshDebounce` だけ待ってから `refresh()` を呼ぶ。待機中に次の呼び出しが来たら
//   先行は要求を出さずに終わる(連続操作で要求を連発しない)。
// - `cancel()`: 保持中の Task を cancel し、計算中を解除する。以後に届く応答は反映しない(画面の `.onDisappear` 用)。
@MainActor
final class BalanceViewModelTests: XCTestCase {

    private let ability = StubMaster.ability

    private func makeViewModel(
        balance: StubBalanceService = StubBalanceService(),
        master: StubPokeCalcService = StubMaster.makeService(),
        debounce: Duration = .zero
    ) -> BalanceViewModel {
        BalanceViewModel(balance: balance, master: master, refreshDebounce: debounce)
    }

    /// `load()` 済みの ViewModel。
    private func loadedViewModel(
        balance: StubBalanceService = StubBalanceService(),
        master: StubPokeCalcService = StubMaster.makeService(),
        debounce: Duration = .zero
    ) async -> BalanceViewModel {
        let viewModel = makeViewModel(balance: balance, master: master, debounce: debounce)
        await viewModel.load()
        return viewModel
    }

    /// 計算中が両方終わるまで待つ(上限あり)。
    private func settle(_ viewModel: BalanceViewModel, file: StaticString = #filePath, line: UInt = #line) async {
        for _ in 0..<10000 where viewModel.isLoadingAnalysis || viewModel.isLoadingCoverage {
            try? await Task.sleep(for: .milliseconds(1))
        }
        XCTAssertFalse(viewModel.isLoadingAnalysis || viewModel.isLoadingCoverage, "計算中が終わらない", file: file, line: line)
    }

    private func input(_ species: SpeciesDetail, ability: String? = nil, moves: [String] = []) -> BalanceMemberInput {
        BalanceMemberInput(pokemonId: species.key, abilityId: ability, moveIds: moves)
    }

    // MARK: - load(マスタ。フォールバックしない)

    func testLoadPopulatesSpeciesOptionsFromMaster() async {
        let master = StubMaster.makeService()
        let viewModel = await loadedViewModel(master: master)
        XCTAssertEqual(
            Set(viewModel.speciesOptions.map(\.key)),
            Set([StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly].map(\.key))
        )
        XCTAssertNil(viewModel.masterError)
        XCTAssertFalse(viewModel.isLoadingMaster)
        let speciesCalls = await master.speciesSearchCalls
        XCTAssertEqual(speciesCalls, [.init(query: "", limit: MasterSearch.pageLimit)])
    }

    func testLoadFailureSetsMasterErrorAndDoesNotInventData() async {
        let master = StubMaster.makeService()
        await master.setMasterError(PokeCalcError(code: "master_unavailable", message: "internal english"))
        let viewModel = await loadedViewModel(master: master)
        XCTAssertTrue(viewModel.speciesOptions.isEmpty, "架空データへフォールバックしない")
        XCTAssertEqual(viewModel.masterError?.code, "master_unavailable")
        XCTAssertEqual(viewModel.masterError?.message, BalanceErrorText.message(forCode: "master_unavailable"))
    }

    // MARK: - addMember / removeMember

    func testAddMemberResolvesSpeciesAndRequestsBothAnalyses() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)

        let added = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        XCTAssertTrue(added)
        await settle(viewModel)

        XCTAssertEqual(viewModel.members.count, 1)
        let member = viewModel.members[0]
        XCTAssertEqual(member.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(member.nameJa, StubMaster.alpha.nameJa)
        XCTAssertEqual(member.types, StubMaster.alpha.types)
        XCTAssertNil(member.abilityId)
        XCTAssertEqual(member.moveIds, [])
        XCTAssertEqual(viewModel.abilityOptionsByMember[member.id], StubMaster.alpha.abilities)
        XCTAssertEqual(
            viewModel.moveOptionsByMember[member.id]?.map(\.id),
            [StubMaster.statusMove.id, StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id],
            "learnset の順・マスタの先頭ページにある技だけ"
        )
        let analyzeRequests = await balance.analyzeRequests
        let coverageRequests = await balance.coverageRequests
        XCTAssertEqual(analyzeRequests, [[input(StubMaster.alpha)]])
        XCTAssertEqual(coverageRequests, [[input(StubMaster.alpha)]])
        XCTAssertEqual(viewModel.analysis?.members.map(\.pokemonId), [StubMaster.alpha.key])
        XCTAssertEqual(viewModel.coverage?.members.map(\.pokemonId), [StubMaster.alpha.key])
        XCTAssertNil(viewModel.analysisError)
        XCTAssertNil(viewModel.coverageError)
    }

    func testAddMemberBeyondMaxIsRejectedWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        for _ in 0..<TeamLimits.maxMembers {
            let added = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
            XCTAssertTrue(added)
        }
        await settle(viewModel)
        let before = await balance.analyzeRequests.count

        let added = await viewModel.addMember(speciesKey: StubMaster.beta.key)

        XCTAssertFalse(added)
        XCTAssertEqual(viewModel.members.count, TeamLimits.maxMembers)
        XCTAssertEqual(viewModel.teamError, .tooManyMembers)
        let after = await balance.analyzeRequests.count
        XCTAssertEqual(after, before, "上限超えでは要求を出さない")
    }

    /// 同じ種族を複数体入れてよい(契約: 重複した pokemonId も要求順のまま保つ)。メンバー id は別々。
    func testSameSpeciesTwiceKeepsDistinctMembers() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settle(viewModel)
        XCTAssertEqual(Set(viewModel.members.map(\.id)).count, 2)
        let last = await balance.analyzeRequests.last
        XCTAssertEqual(last?.map(\.pokemonId), [StubMaster.alpha.key, StubMaster.alpha.key])
    }

    func testAddMemberWithUnknownSpeciesFailsAndKeepsMembers() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        let added = await viewModel.addMember(speciesKey: "0000-000")
        XCTAssertFalse(added)
        XCTAssertTrue(viewModel.members.isEmpty)
        XCTAssertNotNil(viewModel.masterError)
        let requests = await balance.analyzeRequests
        XCTAssertTrue(requests.isEmpty)
    }

    func testRemoveMemberRefreshesWithRemainingMembers() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        _ = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        await settle(viewModel)

        viewModel.removeMember(id: viewModel.members[0].id)
        await balance.waitForAnalyzeRequests(3)
        await settle(viewModel)

        XCTAssertEqual(viewModel.members.map(\.speciesKey), [StubMaster.beta.key])
        let last = await balance.analyzeRequests.last
        XCTAssertEqual(last, [input(StubMaster.beta)])
    }

    func testRemovingLastMemberClearsResultsWithoutRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settle(viewModel)
        XCTAssertNotNil(viewModel.analysis)
        let before = await balance.analyzeRequests.count

        viewModel.removeMember(id: viewModel.members[0].id)
        await viewModel.refresh()

        XCTAssertNil(viewModel.analysis)
        XCTAssertNil(viewModel.coverage)
        XCTAssertNil(viewModel.analysisError)
        XCTAssertFalse(viewModel.isLoadingAnalysis)
        let after = await balance.analyzeRequests.count
        XCTAssertEqual(after, before, "メンバー0体では balance を呼ばない(契約は members 1〜6)")
    }

    // MARK: - 特性・技

    func testSetAbilityAndMovesAreSentToBalance() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settle(viewModel)
        let id = viewModel.members[0].id

        viewModel.setAbility(id: id, abilityId: ability.id)
        XCTAssertTrue(viewModel.addMove(id: id, moveId: StubMaster.specialMove.id))
        await settle(viewModel)

        XCTAssertEqual(viewModel.members[0].abilityId, ability.id)
        XCTAssertEqual(viewModel.members[0].moveIds, [StubMaster.specialMove.id])
        // debounce 0 でも、最後の要求が最終状態を送っていること(途中の要求の有無は問わない)。
        let lastAnalyze = await balance.analyzeRequests.last
        let lastCoverage = await balance.coverageRequests.last
        XCTAssertEqual(lastAnalyze, [input(StubMaster.alpha, ability: ability.id, moves: [StubMaster.specialMove.id])])
        XCTAssertEqual(lastCoverage, lastAnalyze)
    }

    func testAddMoveRulesMatchTeamEdit() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        let id = viewModel.members[0].id

        XCTAssertTrue(viewModel.addMove(id: id, moveId: "m1"))
        XCTAssertFalse(viewModel.addMove(id: id, moveId: "m1"))
        XCTAssertEqual(viewModel.memberErrors[id], .duplicateMove)
        XCTAssertTrue(viewModel.addMove(id: id, moveId: "m2"))
        XCTAssertNil(viewModel.memberErrors[id], "成功したらエラーを消す")
        XCTAssertTrue(viewModel.addMove(id: id, moveId: "m3"))
        XCTAssertTrue(viewModel.addMove(id: id, moveId: "m4"))
        XCTAssertFalse(viewModel.addMove(id: id, moveId: "m5"))
        XCTAssertEqual(viewModel.memberErrors[id], .tooManyMoves)
        XCTAssertEqual(viewModel.members[0].moveIds, ["m1", "m2", "m3", "m4"])

        viewModel.removeMove(id: id, at: 0)
        viewModel.removeMove(id: id, at: 99)  // 範囲外は無視
        XCTAssertEqual(viewModel.members[0].moveIds, ["m2", "m3", "m4"])
    }

    // MARK: - 構築からの読み込み

    func testLoadTeamReplacesMembersWithTeamMembers() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.gamma.key)
        let team = Team(
            id: "team-1", name: "テストチーム",
            members: [
                TeamMember(
                    speciesKey: StubMaster.alpha.key, moveIds: [StubMaster.specialMove.id], abilityId: ability.id,
                    natureId: "stub-nature-neutral"
                ),
                TeamMember(speciesKey: StubMaster.beta.key, natureId: "stub-nature-neutral"),
            ]
        )

        await viewModel.loadTeam(team)
        await settle(viewModel)

        XCTAssertEqual(viewModel.members.map(\.speciesKey), [StubMaster.alpha.key, StubMaster.beta.key], "入れ替える(追加しない)")
        XCTAssertEqual(viewModel.members[0].abilityId, ability.id)
        XCTAssertEqual(viewModel.members[0].moveIds, [StubMaster.specialMove.id])
        XCTAssertEqual(viewModel.members[0].nameJa, StubMaster.alpha.nameJa)
        let last = await balance.analyzeRequests.last
        XCTAssertEqual(
            last,
            [input(StubMaster.alpha, ability: ability.id, moves: [StubMaster.specialMove.id]), input(StubMaster.beta)]
        )
    }

    // MARK: - エラー(analyze と coverage は独立)

    func testAnalyzeFailureDoesNotBlockCoverage() async {
        let balance = StubBalanceService()
        await balance.setAnalyzeError(PokeCalcError(code: "unknown_pokemon", message: "unknown pokemonId: 9101-000"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settle(viewModel)

        XCTAssertNil(viewModel.analysis)
        XCTAssertEqual(viewModel.analysisError?.code, "unknown_pokemon")
        XCTAssertEqual(viewModel.analysisError?.message, BalanceErrorText.message(forCode: "unknown_pokemon"))
        XCTAssertNotNil(viewModel.coverage, "攻撃範囲は analyze の失敗に引きずられない")
        XCTAssertNil(viewModel.coverageError)
    }

    func testCoverageFailureDoesNotBlockAnalysis() async {
        let balance = StubBalanceService()
        await balance.setCoverageError(PokeCalcError(code: PokeCalcError.Code.transport, message: "URLError"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settle(viewModel)

        XCTAssertNotNil(viewModel.analysis)
        XCTAssertNil(viewModel.analysisError)
        XCTAssertNil(viewModel.coverage)
        XCTAssertEqual(viewModel.coverageError?.code, "balance_unavailable")
    }

    func testSuccessAfterFailureClearsError() async {
        let balance = StubBalanceService()
        await balance.setAnalyzeError(PokeCalcError(code: "master_unavailable", message: "x"))
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await settle(viewModel)
        XCTAssertNotNil(viewModel.analysisError)

        await balance.setAnalyzeError(nil)
        await viewModel.refresh()

        XCTAssertNil(viewModel.analysisError)
        XCTAssertNotNil(viewModel.analysis)
    }

    // MARK: - stale 応答の抑止(Web の ADR-0303 §9 と同じ)
    //
    // `scheduleRefresh()` は先行 Task を cancel する(LatestTaskRunner)ので、cancel を受けずに「追い越された応答」が
    // 届く状況は `refresh()` を直接2回(別 Task で)呼んで作る。予約された refresh が走らないよう debounce は十分長くする。

    private static let neverFires: Duration = .seconds(60)

    /// 古い refresh の応答が、新しい refresh の応答より後に届いても、画面は新しい方のまま。
    func testStaleResponsesAreIgnoredWhenNewerCompletedFirst() async {
        let balance = StubBalanceService()
        await balance.setAnalyzeMode(.manual)
        await balance.setCoverageMode(.manual)
        let viewModel = await loadedViewModel(balance: balance, debounce: Self.neverFires)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        let first = Task { await viewModel.refresh() }
        await balance.waitForAnalyzeRequests(1)
        await balance.waitForCoverageRequests(1)
        _ = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        let second = Task { await viewModel.refresh() }
        await balance.waitForAnalyzeRequests(2)
        await balance.waitForCoverageRequests(2)

        // 新しい方(index 1)が先に返り、古い方(index 0)が後から返る。
        let both = [StubMaster.alpha.key, StubMaster.beta.key]
        await balance.resolveAnalyze(at: 1, with: .success(BalanceFixtures.defenseAnalysis(memberIds: both)))
        await balance.resolveCoverage(at: 1, with: .success(BalanceFixtures.coverageAnalysis(memberIds: both)))
        await balance.resolveAnalyze(at: 0, with: .success(BalanceFixtures.defenseAnalysis(memberIds: [StubMaster.alpha.key])))
        await balance.resolveCoverage(at: 0, with: .success(BalanceFixtures.coverageAnalysis(memberIds: [StubMaster.alpha.key])))
        await first.value
        await second.value
        viewModel.cancel()

        XCTAssertEqual(viewModel.analysis?.members.count, 2, "古い応答(1体)で上書きされない")
        XCTAssertEqual(viewModel.coverage?.members.count, 2)
    }

    /// 古い呼び出しの失敗も反映しない(新しい成功をエラーで上書きしない)。
    func testStaleFailureDoesNotOverwriteNewerSuccess() async {
        let balance = StubBalanceService()
        await balance.setAnalyzeMode(.manual)
        let viewModel = await loadedViewModel(balance: balance, debounce: Self.neverFires)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        let first = Task { await viewModel.refresh() }
        await balance.waitForAnalyzeRequests(1)
        _ = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        let second = Task { await viewModel.refresh() }
        await balance.waitForAnalyzeRequests(2)

        let both = [StubMaster.alpha.key, StubMaster.beta.key]
        await balance.resolveAnalyze(at: 1, with: .success(BalanceFixtures.defenseAnalysis(memberIds: both)))
        await balance.resolveAnalyze(at: 0, with: .failure(PokeCalcError(code: "internal_error", message: "x")))
        await first.value
        await second.value
        viewModel.cancel()

        XCTAssertEqual(viewModel.analysis?.members.count, 2)
        XCTAssertNil(viewModel.analysisError)
    }

    /// 古い要求だけが返っても、最新の応答が来るまで計算中のまま(古い応答で計算中を下ろさず、表示もしない)。
    func testLoadingStaysUntilLatestResponse() async {
        let balance = StubBalanceService()
        await balance.setAnalyzeMode(.manual)
        let viewModel = await loadedViewModel(balance: balance, debounce: Self.neverFires)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        let first = Task { await viewModel.refresh() }
        await balance.waitForAnalyzeRequests(1)
        _ = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        let second = Task { await viewModel.refresh() }
        await balance.waitForAnalyzeRequests(2)

        await balance.resolveAnalyze(at: 0, with: .success(BalanceFixtures.defenseAnalysis(memberIds: [StubMaster.alpha.key])))
        await first.value
        XCTAssertTrue(viewModel.isLoadingAnalysis, "古い応答では計算中を下ろさない")
        XCTAssertNil(viewModel.analysis, "古い応答を表示しない")

        let both = [StubMaster.alpha.key, StubMaster.beta.key]
        await balance.resolveAnalyze(at: 1, with: .success(BalanceFixtures.defenseAnalysis(memberIds: both)))
        await second.value
        viewModel.cancel()
        XCTAssertFalse(viewModel.isLoadingAnalysis)
        XCTAssertEqual(viewModel.analysis?.members.count, 2)
    }

    // MARK: - デバウンスとキャンセル(LatestTaskRunner)

    /// 連続操作は最新の1回だけ要求になる(待機中に次の操作が来た先行は要求を出さない)。
    func testRapidEditsCollapseIntoOneRequest() async {
        let balance = StubBalanceService()
        let viewModel = await loadedViewModel(balance: balance, debounce: .milliseconds(80))
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        let id = viewModel.members[0].id
        // ここまでの debounce 待ちを終わらせる。
        await balance.waitForAnalyzeRequests(1)
        await settle(viewModel)
        let base = await balance.analyzeRequests.count

        _ = viewModel.addMove(id: id, moveId: StubMaster.specialMove.id)
        _ = viewModel.addMove(id: id, moveId: StubMaster.alphaOnlyMove.id)
        viewModel.setAbility(id: id, abilityId: ability.id)
        await balance.waitForAnalyzeRequests(base + 1)
        try? await Task.sleep(for: .milliseconds(250))  // 余分な要求が出ないことを確かめる待ち
        await settle(viewModel)

        let requests = await balance.analyzeRequests
        XCTAssertEqual(requests.count, base + 1, "連続3操作は最新の1要求にまとまる")
        XCTAssertEqual(
            requests.last,
            [input(StubMaster.alpha, ability: ability.id, moves: [StubMaster.specialMove.id, StubMaster.alphaOnlyMove.id])]
        )
    }

    /// 画面を離れたら(cancel)、保留中の要求は cancel され、計算中は下り、結果は何も反映されない。
    func testCancelAbortsPendingRequestsAndDropsResults() async {
        let balance = StubBalanceService()
        await balance.setAnalyzeMode(.manual)
        await balance.setCoverageMode(.manual)
        let viewModel = await loadedViewModel(balance: balance)
        _ = await viewModel.addMember(speciesKey: StubMaster.alpha.key)
        await balance.waitForAnalyzeRequests(1)
        await balance.waitForCoverageRequests(1)

        viewModel.cancel()
        XCTAssertFalse(viewModel.isLoadingAnalysis)
        XCTAssertFalse(viewModel.isLoadingCoverage)

        // 送信済みの要求が cancel される(サービスが CancellationError で終える)のを待つ。
        for _ in 0..<10000 {
            let analyzeCancelled = await balance.cancelledAnalyzeRequests
            let coverageCancelled = await balance.cancelledCoverageRequests
            if analyzeCancelled.contains(0) && coverageCancelled.contains(0) { break }
            try? await Task.sleep(for: .milliseconds(1))
        }
        let analyzeCancelled = await balance.cancelledAnalyzeRequests
        let coverageCancelled = await balance.cancelledCoverageRequests
        XCTAssertTrue(analyzeCancelled.contains(0))
        XCTAssertTrue(coverageCancelled.contains(0))
        XCTAssertNil(viewModel.analysis)
        XCTAssertNil(viewModel.coverage)
        XCTAssertNil(viewModel.analysisError, "cancel はエラーではない")
        XCTAssertNil(viewModel.coverageError)
    }
}
