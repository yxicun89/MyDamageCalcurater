import XCTest

@testable import PokeCalcCore

/// 計算画面の「詳細」の「防御側のランク」(issue #274。ADR-0315 の iOS 版。ADR-0501「防御側のランクの受け入れ条件」)。
///
/// - 編集対象は選択中の技の分類で決まる(物理・変化・技なし = def〈B〉、特殊 = spd〈D〉)。def / spd は別々に保持する。
/// - 既定(0・0)なら要求は従来と同じ(`defenderRanks` が既定値)。どちらかが非 0 なら 5 項目を持つ `RankBlock` を載せる。
/// - 防御側のランクは、種族変更・攻守入れ替え・技の変更・構築の呼び出しで消さない(Web の ADR-0315 §6 と同じ。
///   攻撃側のランクの既存規則とも同じ)。特性(P6-19)だけが種族変更・入れ替えで「指定なし」に戻る。
/// - 攻撃側のランク・防御側の特性と混ざらない。値が変わる操作ごとに計算1回、変わらない操作は0回。
/// 既存の `CalcViewModel*Tests` は変えない。stub はエコー応答。
@MainActor
final class CalcViewModelDefenderRanksTests: XCTestCase {

    private let maxRank = 6
    private let minRank = -6

    // MARK: - 補助

    private func makeStub() -> StubPokeCalcService {
        StubMaster.makeService(species: [
            StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly,
            StubMaster.abilityXAndY, StubMaster.abilityXOnly,
        ])
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    private func calcCount(_ stub: StubPokeCalcService, during operation: () async -> Void) async -> Int {
        let before = await stub.bulkRequests.count
        await operation()
        return await stub.bulkRequests.count - before
    }

    // MARK: - 既定

    func testDefaultsAreZeroAndRequestCarriesNoDefenderRanks() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.defenderRanks, RankBlock())
        XCTAssertEqual(viewModel.selectedMove?.category, .physical, "起動直後は物理技")
        XCTAssertEqual(viewModel.defenderRankStat, .def)
        XCTAssertEqual(viewModel.defenderRank, 0)
        XCTAssertEqual(viewModel.defenderRankText, "B ±0")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(), "既定(0・0)は defenderOverride.ranks を送らない")
    }

    // MARK: - 変更と要求(混ざらない)

    func testSetDefenderRankMapsToDefOnlyAndDoesNotTouchAttackerRanks() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerRank(2)

        let calls = await calcCount(stub) { await viewModel.setDefenderRank(1) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.defenderRank, 1)
        XCTAssertEqual(viewModel.defenderRankText, "B +1")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 1), "物理技なので def だけ。5 項目の他は 0")
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 2), "防御側のランクを攻撃側の ranks に混ぜない")
        XCTAssertNil(request.defenderAbilityId, "ランクだけでは特性を指定しない")
        XCTAssertEqual(viewModel.attackerRank, 2, "攻撃側のステッパーの値は変わらない")
    }

    func testAttackerRankChangeDoesNotChangeDefenderRanks() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setDefenderRank(-3)

        await viewModel.setAttackerRank(4)
        XCTAssertEqual(viewModel.defenderRanks, RankBlock(def: -3))
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: -3))
        XCTAssertEqual(request.attacker.ranks, RankBlock(atk: 4))
    }

    // MARK: - 上下限 ±6 と変化なし

    func testDefenderRankClampsToContractRangeAndIgnoresNoOpChanges() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        var calls = await calcCount(stub) { await viewModel.setDefenderRank(0) }
        XCTAssertEqual(calls, 0, "同じ値(既定の 0)は計算しない")

        calls = await calcCount(stub) { await viewModel.setDefenderRank(maxRank) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.defenderRankText, "B +6")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks.def, maxRank)

        calls = await calcCount(stub) { await viewModel.setDefenderRank(maxRank + 1) }
        XCTAssertEqual(calls, 0, "+6 を超える指定は +6 に丸まり、値が変わらないので計算しない")
        XCTAssertEqual(viewModel.defenderRank, maxRank)

        calls = await calcCount(stub) { await viewModel.setDefenderRank(100) }
        XCTAssertEqual(calls, 0)

        calls = await calcCount(stub) { await viewModel.setDefenderRank(minRank) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.defenderRankText, "B -6")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks.def, minRank)

        calls = await calcCount(stub) { await viewModel.setDefenderRank(minRank - 1) }
        XCTAssertEqual(calls, 0, "-6 を下回る指定は -6 に丸まる")
        XCTAssertEqual(viewModel.defenderRank, minRank)

        calls = await calcCount(stub) { await viewModel.setDefenderRank(0) }
        XCTAssertEqual(calls, 1, "0 に戻すと計算1回")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(), "0・0 に戻したら既定(defenderOverride を送らない形)")
    }

    func testRangeUsesSharedRankLimits() {
        XCTAssertEqual(RankLimits.min, minRank)
        XCTAssertEqual(RankLimits.max, maxRank)
    }

    // MARK: - 技の分類で編集対象が切り替わる(別々に保持)

    func testStepperFollowsMoveCategoryAndKeepsEachStatsRank() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.selectedMove?.category, .physical)

        await viewModel.setDefenderRank(2)
        XCTAssertEqual(viewModel.defenderRanks, RankBlock(def: 2))

        // 特殊技に変えると、ステッパーは D(spd)を編集する。B のランクは消さずに持ち続ける。
        let calls = await calcCount(stub) { await viewModel.selectMove(id: StubMaster.specialMove.id) }
        XCTAssertEqual(calls, 1, "技の変更は従来どおり計算1回(ランクの付け替えで余計に計算しない)")
        XCTAssertEqual(viewModel.defenderRankStat, .spd)
        XCTAssertEqual(viewModel.defenderRank, 0)
        XCTAssertEqual(viewModel.defenderRankText, "D ±0")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 2, spd: 0),
                       "def/spd は両方そのまま送る(engine は技の分類の関連ステータスだけを使う)")

        await viewModel.setDefenderRank(-2)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 2, spd: -2))
        XCTAssertEqual(viewModel.defenderRankText, "D -2")

        // 物理技に戻すと B の +2 がまた見える。
        await viewModel.selectMove(id: StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(viewModel.defenderRankStat, .def)
        XCTAssertEqual(viewModel.defenderRank, 2)
        XCTAssertEqual(viewModel.defenderRankText, "B +2")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 2, spd: -2))
    }

    func testSpecialMoveRankOnlyRequestCarriesSpdAndOtherFieldsZero() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectMove(id: StubMaster.specialMove.id)

        await viewModel.setDefenderRank(5)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(atk: 0, def: 0, spa: 0, spd: 5, spe: 0))
    }

    func testStatusMoveEditsDef() async throws {
        // 変化技は物理と同じ def〈B〉(攻撃側の「変化技は atk」と同じ規則)。
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.defenderRankStat, .spd)
        await viewModel.selectMove(id: StubMaster.statusMove.id)
        XCTAssertEqual(viewModel.selectedMove?.category, .status)
        XCTAssertEqual(viewModel.defenderRankStat, .def)
        XCTAssertEqual(viewModel.defenderRankText, "B ±0")
        await viewModel.setDefenderRank(1)
        XCTAssertEqual(viewModel.defenderRanks.def, 1)
        XCTAssertEqual(viewModel.defenderRanks.spd, 0)
    }

    // MARK: - リセットしない(Web の ADR-0315 §6 と同じ)

    func testDefenderRanksSurviveDefenderChangeSwapAndMoveChange() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setDefenderRank(3)

        var calls = await calcCount(stub) { await viewModel.selectDefender(speciesKey: StubMaster.gamma.key) }
        XCTAssertEqual(calls, 1, "防御側の変更は従来どおり計算1回")
        XCTAssertEqual(viewModel.defenderRanks, RankBlock(def: 3), "防御側の種族変更で消さない")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 3))

        calls = await calcCount(stub) { await viewModel.swapSides() }
        XCTAssertEqual(calls, 1, "攻守入れ替えは従来どおり計算1回")
        XCTAssertEqual(viewModel.defenderRanks, RankBlock(def: 3), "攻守入れ替えで消さない(攻撃側のランクの既存規則と同じ)")
        request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 3))

        calls = await calcCount(stub) { await viewModel.selectMove(id: StubMaster.specialMove.id) }
        XCTAssertEqual(calls, 1)
        XCTAssertEqual(viewModel.defenderRanks, RankBlock(def: 3))
    }

    func testAttackerRanksAreAlsoKeptAcrossSwapUnchanged() async throws {
        // 既存規則(攻撃側のランクは入れ替えで消えない)を防御側の規則の根拠として固定する。
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.setAttackerRank(2)
        await viewModel.setDefenderRank(-1)

        await viewModel.swapSides()
        XCTAssertEqual(viewModel.attackerRanks, RankBlock(atk: 2))
        XCTAssertEqual(viewModel.defenderRanks, RankBlock(def: -1))
    }

    // MARK: - 特性と併存(P6-19)

    func testRanksCoexistWithDefenderAbilityAndAbilityStillResetsOnDefenderChange() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)
        await viewModel.loadDefenderAbilityOptions()
        await viewModel.selectDefenderAbility(id: StubMaster.abilityX.id)
        await viewModel.setDefenderRank(2)

        var request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderAbilityId, StubMaster.abilityX.id)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 2), "特性とランクを同じ要求に載せる")

        // 特性だけ変えてもランクは残る。
        await viewModel.selectDefenderAbility(id: StubMaster.abilityY.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderAbilityId, StubMaster.abilityY.id)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 2))

        // 防御側の種族を変えると特性は「指定なし」に戻るが、ランクは残る。
        await viewModel.selectDefender(speciesKey: StubMaster.gamma.key)
        request = try await lastRequest(stub)
        XCTAssertNil(request.defenderAbilityId)
        XCTAssertEqual(request.defenderRanks, RankBlock(def: 2))
    }

    // MARK: - 世代(古い計算は追い越される)

    func testDefenderRankChangeCancelsThePreviousInFlightCalcAndAccumulates() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.bulkRequests.count
        await stub.setBulkMode(.manual)

        let older = viewModel.scheduleLatest { await $0.setDefenderRank(1) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.setDefenderRank(2) }
        try await stub.waitForBulkCancellation(at: baseline)
        try await stub.waitForBulkRequests(count: baseline + 2)
        await stub.resolveBulkWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        XCTAssertTrue(older.isCancelled)
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, baseline + 2, "ランク1回の変更につき計算1回")
        XCTAssertEqual(try XCTUnwrap(requests.last).defenderRanks, RankBlock(def: 2))
        XCTAssertEqual(viewModel.defenderRank, 2)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    func testStaleResponseDoesNotOverwriteLatestRows() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.bulkRequests.count
        await stub.setBulkMode(.manual)

        let older = viewModel.scheduleLatest { await $0.setDefenderRank(1) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.setDefenderRank(2) }
        try await stub.waitForBulkRequests(count: baseline + 2)
        // 古い要求は新しい入力で cancel され、スタブの保留から外れる(応答できない)。最新の要求の結果だけが残る(規則7)。
        try await stub.waitForBulkCancellation(at: baseline)
        await stub.resolveBulkWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        // 古い要求は cancel され、最新の要求の結果だけが残ること(既存の CalcViewModelCancellationTests と同じ強さ)。
        XCTAssertTrue(older.isCancelled, "先行の計算 Task を cancel すること")
        let cancelled = await stub.cancelledBulkRequests
        XCTAssertEqual(cancelled, [baseline], "送信済みの古い要求だけが cancel されること")
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, baseline + 2)
        XCTAssertEqual(requests.last?.defenderRanks.def, 2, "最新の要求は最新の防御側ランクを載せる")
        XCTAssertFalse(viewModel.rows.isEmpty)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertEqual(viewModel.defenderRank, 2)
    }
}
