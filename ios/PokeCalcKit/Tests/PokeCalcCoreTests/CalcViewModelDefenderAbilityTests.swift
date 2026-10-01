import XCTest

@testable import PokeCalcCore

/// 計算画面の「防御側の特性」(issue #272。ADR-0501「P6-19」3章)。
///
/// - 既定は「指定なし」(`defenderOverride` を送らない = サーバーが種族の特性をすべて試す)。
/// - 選択肢は防御側の `species(key:)` の `abilities`。起動・防御側の変更では読まず、`loadDefenderAbilityOptions()`
///   (View が「詳細」を開いている間に呼ぶ)と、結果の行が特性で分かれたとき(名前が要る)にだけ読む。
///   既存テストが数える `species(key:)` の回数・順番を変えないため。
/// - 値が変わる選択ごとに計算1回。防御側の種族の変更・入れ替えで「指定なし」に戻る(その操作の計算回数は変えない)。
/// 既存の `CalcViewModel*Tests` は変えない。stub はエコー応答(特性で分けるテストだけ独自の応答)。
@MainActor
final class CalcViewModelDefenderAbilityTests: XCTestCase {

    // MARK: - 補助

    /// 既定の攻撃側 = alpha、防御側 = beta(`load()` の規則)。特性を2つ持つ `abilityXAndY` を防御側に選べるようにする。
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

    /// 防御側を `abilityXAndY` にして、その特性の選択肢(X, Y)を読んだ状態。
    private func viewModelWithXYDefender(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)
        await viewModel.loadDefenderAbilityOptions()
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

    /// 指定なしのとき、エコーの各行を特性 X の行と Y の行に分けて返す(サーバーの「結果が違うときだけ分ける」の再現。
    /// 並びは プリセット → 特性 → 持ち物。ADR-0126)。指定ありならその特性だけの行。
    private nonisolated static func splittingResponder(_ request: BulkCalcRequest) -> Result<BulkCalcResult, PokeCalcError> {
        let echo = StubPokeCalcService.echoResult(for: request)
        let groups: [[String]] = request.defenderAbilityId.map { [[$0]] }
            ?? [[StubMaster.abilityX.id], [StubMaster.abilityY.id]]
        var rows: [BulkCalcRow] = []
        let presets = request.presets.isEmpty ? [DefenderPreset.none] : request.presets
        for preset in presets {
            for group in groups {
                for row in echo.rows where row.preset == preset {
                    var split = row
                    split.abilityId = group.first
                    split.abilityIds = group
                    rows.append(split)
                }
            }
        }
        return .success(BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: rows))
    }

    // MARK: - 既定

    func testDefaultIsUnspecifiedAndLoadDoesNotFetchDefenderSpecies() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)

        XCTAssertNil(viewModel.defenderAbilityId, "既定は「指定なし」")
        XCTAssertEqual(viewModel.defenderAbilityOptions, [], "起動では防御側の特性を読まない")
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests, [StubMaster.alpha.key],
                       "起動時の species(key:) は攻撃側の1回のまま(既存テストの前提を変えない)")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.defenderAbilityId, "指定なしは送らない")
        let bulkCount = await stub.bulkRequests.count
        XCTAssertEqual(bulkCount, 1, "起動時の計算は1回のまま")
    }

    // MARK: - 選択肢の読み込み

    func testLoadDefenderAbilityOptionsFetchesDefenderDetailOnceWithoutCalculating() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)
        let rowsBefore = viewModel.rows

        let calcs = await calcCount(stub) { await viewModel.loadDefenderAbilityOptions() }

        XCTAssertEqual(calcs, 0, "選択肢の読み込みでは計算しない")
        XCTAssertEqual(viewModel.defenderAbilityOptions, [StubMaster.abilityX, StubMaster.abilityY],
                       "防御側の species(key:) の abilities の順")
        XCTAssertNil(viewModel.defenderAbilityId, "読んでも選択は「指定なし」のまま")
        XCTAssertEqual(viewModel.rows, rowsBefore)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
        let lastSpecies = await stub.speciesRequests.last
        XCTAssertEqual(lastSpecies, StubMaster.abilityXAndY.key)

        let countBefore = await stub.speciesRequests.count
        await viewModel.loadDefenderAbilityOptions()
        let countAfter = await stub.speciesRequests.count
        XCTAssertEqual(countAfter, countBefore, "同じ防御側の選択肢は読み直さない")
    }

    func testLoadDefenderAbilityOptionsFailureIsSilent() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let rowsBefore = viewModel.rows
        await stub.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト species 失敗"))

        let calcs = await calcCount(stub) { await viewModel.loadDefenderAbilityOptions() }

        XCTAssertEqual(calcs, 0)
        XCTAssertEqual(viewModel.defenderAbilityOptions, [], "失敗したら選択肢は空(「指定なし」だけ)")
        XCTAssertNil(viewModel.error, "選択肢の読み込み失敗は画面のエラーにしない(計算は指定なしで成り立つ)")
        XCTAssertEqual(viewModel.rows, rowsBefore, "結果を消さない")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testStaleDefenderAbilityOptionsAreNotApplied() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)
        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        let loading = Task { await viewModel.loadDefenderAbilityOptions() }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        // 読み込み中に防御側を変える(防御側の変更は species(key:) を呼ばない)。
        await viewModel.selectDefender(speciesKey: StubMaster.beta.key)
        let xyDetail = try await stub.lookupSpecies(key: StubMaster.abilityXAndY.key)
        await stub.resolveSpecies(at: baseline, with: .success(xyDetail))
        await loading.value

        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.defenderAbilityOptions, [], "古い防御側の特性を選択肢に入れない")
    }

    // MARK: - 選択 → 要求(値が変わるたびに計算1回)

    func testSelectDefenderAbilityMapsToRequestWithOneCalcPerChange() async throws {
        let stub = makeStub()
        let viewModel = await viewModelWithXYDefender(stub)

        var calcs = await calcCount(stub) { await viewModel.selectDefenderAbility(id: StubMaster.abilityY.id) }
        XCTAssertEqual(calcs, 1)
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityY.id)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderAbilityId, StubMaster.abilityY.id)
        XCTAssertNil(request.attacker.abilityId, "防御側の特性は攻撃側の abilityId に混ぜない")
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.abilityXAndY.key)

        calcs = await calcCount(stub) { await viewModel.selectDefenderAbility(id: StubMaster.abilityY.id) }
        XCTAssertEqual(calcs, 0, "同じ特性を選び直しても計算しない")

        calcs = await calcCount(stub) { await viewModel.selectDefenderAbility(id: StubMaster.ability.id) }
        XCTAssertEqual(calcs, 0, "選択肢に無い特性は無視する")
        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityY.id)

        calcs = await calcCount(stub) { await viewModel.selectDefenderAbility(id: nil) }
        XCTAssertEqual(calcs, 1)
        XCTAssertNil(viewModel.defenderAbilityId)
        request = try await lastRequest(stub)
        XCTAssertNil(request.defenderAbilityId, "「指定なし」に戻すと送らない")
    }

    func testSelectDefenderAbilityBeforeOptionsAreLoadedIsIgnored() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)

        let calcs = await calcCount(stub) { await viewModel.selectDefenderAbility(id: StubMaster.abilityX.id) }
        XCTAssertEqual(calcs, 0, "選択肢を読む前の ID は選択肢に無いので無視する")
        XCTAssertNil(viewModel.defenderAbilityId)
    }

    // MARK: - リセットと引き継ぎ

    func testChangingDefenderSpeciesResetsAbilityWithoutExtraCalc() async throws {
        let stub = makeStub()
        let viewModel = await viewModelWithXYDefender(stub)
        await viewModel.selectDefenderAbility(id: StubMaster.abilityX.id)

        let calcs = await calcCount(stub) { await viewModel.selectDefender(speciesKey: StubMaster.abilityXOnly.key) }

        XCTAssertEqual(calcs, 1, "防御側の変更の計算回数は変えない(1回)")
        XCTAssertNil(viewModel.defenderAbilityId, "防御側の種族が変わったら「指定なし」に戻す(旧種族の特性を送らない)")
        XCTAssertEqual(viewModel.defenderAbilityOptions, [], "旧種族の選択肢を残さない")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.defenderAbilityId, "その計算から指定なし")
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.abilityXOnly.key)
    }

    func testSwapResetsDefenderAbilityWithoutExtraCalc() async throws {
        let stub = makeStub()
        let viewModel = await viewModelWithXYDefender(stub)
        await viewModel.selectDefenderAbility(id: StubMaster.abilityX.id)

        let calcs = await calcCount(stub) { await viewModel.swapSides() }

        XCTAssertEqual(calcs, 1, "入れ替えの計算回数は変えない(1回)")
        XCTAssertNil(viewModel.defenderAbilityId, "入れ替えで防御側が変わるので「指定なし」に戻す")
        XCTAssertEqual(viewModel.defenderAbilityOptions, [])
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertNil(request.defenderAbilityId)
    }

    func testOtherInputsKeepDefenderAbility() async throws {
        let stub = makeStub()
        let viewModel = await viewModelWithXYDefender(stub)
        await viewModel.selectDefenderAbility(id: StubMaster.abilityX.id)

        await viewModel.selectAttacker(speciesKey: StubMaster.gamma.key)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        await viewModel.selectAttackerPreset(.aMax)
        await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemA.id)
        await viewModel.setCritical(true)
        await viewModel.selectWeather(.rain)

        XCTAssertEqual(viewModel.defenderAbilityId, StubMaster.abilityX.id,
                       "攻撃側・技・プリセット・持ち物の比較・条件の変更では防御側の特性を消さない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.defenderAbilityId, StubMaster.abilityX.id)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.abilityXAndY.key)
    }

    // MARK: - 世代・キャンセル

    func testDefenderAbilityChangeCancelsThePreviousInFlightCalcAndAccumulates() async throws {
        let stub = makeStub()
        let viewModel = await viewModelWithXYDefender(stub)
        let baseline = await stub.bulkRequests.count
        await stub.setBulkMode(.manual)

        let older = viewModel.scheduleLatest { await $0.setCritical(true) }
        try await stub.waitForBulkRequests(count: baseline + 1)
        let newer = viewModel.scheduleLatest { await $0.selectDefenderAbility(id: StubMaster.abilityY.id) }
        try await stub.waitForBulkCancellation(at: baseline)
        try await stub.waitForBulkRequests(count: baseline + 2)
        await stub.resolveBulkWithEcho(at: baseline + 1)
        await newer.value
        await older.value

        XCTAssertTrue(older.isCancelled)
        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, baseline + 2, "特性の変更1回につき計算1回")
        let last = try XCTUnwrap(requests.last)
        XCTAssertTrue(last.critical, "先の変更は状態として残り、次の要求に載る")
        XCTAssertEqual(last.defenderAbilityId, StubMaster.abilityY.id)
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - 特性で分かれた行(名前・ID)

    func testSplitRowsGetUniqueIDsAndAbilityNamesResolvedFromDefenderSpecies() async throws {
        let stub = makeStub()
        await stub.setBulkResponder(Self.splittingResponder)
        let viewModel = await loadedViewModel(stub)

        // 防御側の選択肢はまだ読んでいない。行が分かれたので、名前のために防御側の species(key:) を読む。
        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)

        let ids = viewModel.rows.map(\.id)
        XCTAssertEqual(ids, ["none@-@\(StubMaster.abilityX.id)", "none@-@\(StubMaster.abilityY.id)"])
        XCTAssertEqual(Set(ids).count, ids.count, "ForEach の ID が衝突しない")
        XCTAssertEqual(viewModel.rows.map(\.abilityText), ["特性: テストとくせいX", "特性: テストとくせいY"],
                       "名前は防御側の species(key:) の abilities から引く")
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests.last, StubMaster.abilityXAndY.key)
        let bulkCount = await stub.bulkRequests.count
        XCTAssertEqual(bulkCount, 2, "名前の読み込みで計算し直さない(起動 + 防御側の変更の2回)")
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)

        // 特性を1つに決めると行は分かれず、既存の ID に戻り副題も消える。
        await viewModel.selectDefenderAbility(id: StubMaster.abilityY.id)
        XCTAssertEqual(viewModel.rows.map(\.id), ["none@-"])
        XCTAssertEqual(viewModel.rows.map(\.abilityText), [nil])
    }

    func testUnsplitRowsDoNotFetchDefenderSpecies() async throws {
        let stub = makeStub()
        let viewModel = await loadedViewModel(stub)
        let baseline = await stub.speciesRequests.count

        await viewModel.selectDefender(speciesKey: StubMaster.abilityXAndY.key)

        let after = await stub.speciesRequests.count
        XCTAssertEqual(after, baseline, "行が分かれていなければ名前は要らないので読まない")
        XCTAssertEqual(viewModel.rows.map(\.abilityText), [nil])
    }
}
