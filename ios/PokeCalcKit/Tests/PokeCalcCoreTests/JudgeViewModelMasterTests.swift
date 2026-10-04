import XCTest

@testable import PokeCalcCore

/// `JudgeViewModel` のマスタ(入力補助。P6-25。ADR-0504 §4): 性格・持ち物・種族・技は ID の自由入力ではなくマスタから選ぶ。
/// 検索は `MasterSpeciesSearchProviding` / `MasterMoveSearchProviding` と同じ規則(同期の文字反映・空クエリは先頭ページへ戻す)。
@MainActor
final class JudgeViewModelMasterTests: XCTestCase {
    private typealias H = JudgeHarness

    func testLoadFillsTheOptionsFromTheMaster() async {
        let master = H.makeMaster()
        let viewModel = H.makeViewModel(master: master)
        await viewModel.load()
        XCTAssertEqual(viewModel.natureOptions, [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature])
        XCTAssertEqual(viewModel.itemOptions, [StubMaster.itemA, StubMaster.itemB])
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.abilityXOnly, StubMaster.abilityYOnly, StubMaster.abilityXAndY].map(\.key))
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.physicalMove, StubMaster.specialMove, StubMaster.statusMove, StubMaster.alphaOnlyMove].map(\.id))
        XCTAssertNil(viewModel.masterFailure)
        let speciesCalls = await master.speciesSearchCalls
        let moveCalls = await master.moveSearchCalls
        XCTAssertEqual(speciesCalls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)], "先頭ページを1回だけ")
        XCTAssertEqual(moveCalls, [StubPokeCalcService.SearchCall(query: "", limit: MasterSearch.pageLimit)])
    }

    // MARK: - 既定の性格(補正なしの最初)

    func testLoadSetsTheFirstNeutralNatureWhereNoNatureIsChosen() async {
        let viewModel = H.makeViewModel()
        viewModel.addCandidate()
        await viewModel.load()
        XCTAssertEqual(viewModel.attacker.natureId, StubMaster.neutralNature.id, "補正なし = plus も minus も無い性格。一覧の先頭が攻撃上昇でも補正なしを選ぶ")
        XCTAssertEqual(viewModel.candidates.map(\.natureId), [StubMaster.neutralNature.id, StubMaster.neutralNature.id])
    }

    func testLoadDoesNotOverwriteANatureTheUserChose() async {
        let viewModel = H.makeViewModel()
        await viewModel.load()
        viewModel.setNature(StubMaster.spaUpNature.id, for: .attacker)
        await viewModel.load()
        XCTAssertEqual(viewModel.attacker.natureId, StubMaster.spaUpNature.id)
    }

    func testANewCandidateStartsWithTheDefaultNature() async {
        let viewModel = H.makeViewModel()
        await viewModel.load()
        viewModel.addCandidate()
        XCTAssertEqual(viewModel.candidates[safe: 1]?.natureId, StubMaster.neutralNature.id)
    }

    func testWithoutANeutralNatureNothingIsPreselected() async {
        let viewModel = H.makeViewModel(master: H.makeMaster(natures: [StubMaster.atkUpNature, StubMaster.spaUpNature]))
        await viewModel.load()
        XCTAssertNil(viewModel.attacker.natureId)
        XCTAssertEqual(H.violation(viewModel), .requiredMissing(.attacker))
    }

    // MARK: - 失敗(判定・計算には影響しない)

    func testMasterFailureIsReportedWithoutThrowing() async {
        let master = H.makeMaster()
        await master.setMasterError(PokeCalcError(code: "upstream_unavailable", message: "english"))
        let viewModel = H.makeViewModel(master: master)
        await viewModel.load()
        XCTAssertEqual(viewModel.masterFailure?.code, "upstream_unavailable")
        XCTAssertEqual(viewModel.natureOptions, [])
        XCTAssertEqual(viewModel.itemOptions, [])
        XCTAssertEqual(viewModel.speciesOptions, [])
    }

    // MARK: - 検索

    func testSpeciesSearchFiltersByPrefixAndRestoresTheFirstPageWithoutCallingTheAPI() async {
        let master = H.makeMaster()
        let viewModel = H.makeViewModel(master: master)
        await viewModel.load()
        XCTAssertTrue(viewModel.setSpeciesQuery("  テストベ  "), "語が変わったので検索が要る")
        XCTAssertEqual(viewModel.speciesQuery, "テストベ", "前後の空白は落とす")
        await viewModel.runSpeciesSearch()
        XCTAssertEqual(viewModel.speciesOptions.map(\.key), [StubMaster.beta.key])
        XCTAssertFalse(viewModel.setSpeciesQuery("テストベ"), "同じ語なら検索は要らない")

        let callsBefore = await master.speciesSearchCalls.count
        XCTAssertTrue(viewModel.setSpeciesQuery(""))
        await viewModel.runSpeciesSearch()
        XCTAssertEqual(viewModel.speciesOptions.count, 6, "空に戻せば先頭ページ")
        let callsAfter = await master.speciesSearchCalls.count
        XCTAssertEqual(callsAfter, callsBefore, "空クエリは API を呼ばない")
    }

    func testMoveSearchFiltersByPrefix() async {
        let viewModel = H.makeViewModel()
        await viewModel.load()
        XCTAssertTrue(viewModel.setMoveQuery("テストわざとくしゅ"))
        await viewModel.runMoveSearch()
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.specialMove.id])
        XCTAssertEqual(viewModel.moveQuery, "テストわざとくしゅ")
    }

    /// 検索で見えなくなった技でも、選んだ技の名前は引ける(`move(forID:)`。構築編集と同じ「一度でも見た技」)。
    func testAChosenMoveStaysResolvableAfterTheSearchResultsChange() async {
        let viewModel = H.makeViewModel()
        await viewModel.load()
        viewModel.setMove(StubMaster.alphaOnlyMove, for: .attacker)
        viewModel.setMoveQuery("テストわざとくしゅ")
        await viewModel.runMoveSearch()
        XCTAssertFalse(viewModel.moveOptions.contains { $0.id == StubMaster.alphaOnlyMove.id })
        XCTAssertEqual(viewModel.move(forID: StubMaster.alphaOnlyMove.id), StubMaster.alphaOnlyMove)
    }

    /// 検索の状態は画面に1つ。種族の検索と技の検索は互いに独立(片方の語が他方を変えない)。
    func testSpeciesAndMoveQueriesAreIndependent() async {
        let viewModel = H.makeViewModel()
        await viewModel.load()
        viewModel.setSpeciesQuery("テストベ")
        viewModel.setMoveQuery("テストわざ")
        XCTAssertEqual(viewModel.speciesQuery, "テストベ")
        XCTAssertEqual(viewModel.moveQuery, "テストわざ")
    }
}
