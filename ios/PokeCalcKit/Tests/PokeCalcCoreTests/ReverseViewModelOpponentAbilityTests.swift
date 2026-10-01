import XCTest

@testable import PokeCalcCore

/// 逆算画面の「相手の特性」(issue #272。ADR-0501「P6-19」4章)。
///
/// - 既定は「指定なし」(`unknownAbilityId` を送らない = サーバーが相手の種族の特性をすべて候補にする)。
/// - 選択肢は相手の `species(key:)` の `abilities`。起動・相手の変更では読まず、`loadOpponentAbilityOptions()`
///   (View が呼ぶ)と、候補が特性で分かれたとき(名前が要る)にだけ読む。
/// - 値が変わる選択ごとに `recalculateIfPossible` を1回(観測が無ければ reverse は呼ばない)。
///   相手の種族の変更・側の切り替えで「指定なし」に戻る。
/// 既存の `ReverseViewModel*Tests` は変えない。
@MainActor
final class ReverseViewModelOpponentAbilityTests: XCTestCase {

    // MARK: - 補助

    /// 既定の自分 = alpha、相手 = beta、与えたダメージ(`load()` の規則)。
    private func makeStub() async -> StubPokeCalcService {
        let stub = StubMaster.makeService(
            species: [
                StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly,
                StubMaster.abilityXAndY, StubMaster.abilityXOnly,
            ],
            natures: StubMaster.reverseNatures
        )
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    /// 相手を `abilityXAndY` にして、その特性の選択肢(X, Y)を読んだ状態(観測はまだ無い)。
    private func viewModelWithXYOpponent(_ stub: StubPokeCalcService) async -> ReverseViewModel {
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectOpponentSpecies(key: StubMaster.abilityXAndY.key)
        await viewModel.loadOpponentAbilityOptions()
        return viewModel
    }

    private func enterObservation(_ viewModel: ReverseViewModel, _ text: String = "12") async throws {
        let id = try XCTUnwrap(viewModel.observations.first?.id, "観測の行が無い")
        await viewModel.editObservation(id: id, text: text)
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> ReverseRequest {
        let requests = await stub.reverseRequests
        return try XCTUnwrap(requests.last, "reverse が呼ばれていない")
    }

    private func reverseCount(_ stub: StubPokeCalcService, during operation: () async -> Void) async -> Int {
        let before = await stub.reverseRequests.count
        await operation()
        return await stub.reverseRequests.count - before
    }

    /// 指定なしのとき、エコーの各候補を特性 X と Y に分けて返す(定義順は 性格クラス → 特性 → 持ち物。ADR-0126)。
    private nonisolated static func splittingResponder(_ request: ReverseRequest) -> Result<ReverseResult, PokeCalcError> {
        let echo = StubPokeCalcService.echoReverseResult(for: request)
        let groups: [[String]] = request.unknownAbilityId.map { [[$0]] }
            ?? [[StubMaster.abilityX.id], [StubMaster.abilityY.id]]
        var candidates: [ReverseCandidate] = []
        for natureClass in NatureClass.allCases {
            for group in groups {
                for candidate in echo.candidates where candidate.natureClass == natureClass {
                    var split = candidate
                    split.abilityId = group.first
                    split.abilityIds = group
                    candidates.append(split)
                }
            }
        }
        return .success(ReverseResult(
            side: echo.side, stat: echo.stat, assumedHPSP: echo.assumedHPSP,
            candidates: candidates, exactCount: candidates.filter(\.exact).count
        ))
    }

    // MARK: - 既定

    func testDefaultIsUnspecifiedAndLoadDoesNotFetchOpponentSpecies() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)

        XCTAssertNil(viewModel.opponentAbilityId)
        XCTAssertEqual(viewModel.opponentAbilityOptions, [])
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests, [StubMaster.alpha.key], "起動時の species(key:) は自分(攻撃側)の1回のまま")

        try await enterObservation(viewModel)
        let request = try await lastRequest(stub)
        XCTAssertNil(request.unknownAbilityId, "指定なしは送らない")
    }

    // MARK: - 選択肢の読み込み

    func testLoadOpponentAbilityOptionsFetchesOpponentDetailOnceWithoutCalculating() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectOpponentSpecies(key: StubMaster.abilityXAndY.key)
        try await enterObservation(viewModel)

        let calcs = await reverseCount(stub) { await viewModel.loadOpponentAbilityOptions() }

        XCTAssertEqual(calcs, 0, "選択肢の読み込みでは逆算しない")
        XCTAssertEqual(viewModel.opponentAbilityOptions, [StubMaster.abilityX, StubMaster.abilityY])
        XCTAssertNil(viewModel.opponentAbilityId)
        let lastSpecies = await stub.speciesRequests.last
        XCTAssertEqual(lastSpecies, StubMaster.abilityXAndY.key)

        let before = await stub.speciesRequests.count
        await viewModel.loadOpponentAbilityOptions()
        let after = await stub.speciesRequests.count
        XCTAssertEqual(after, before, "同じ相手の選択肢は読み直さない")
    }

    func testStaleOpponentAbilityOptionsAreNotApplied() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectOpponentSpecies(key: StubMaster.abilityXAndY.key)
        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        let loading = Task { await viewModel.loadOpponentAbilityOptions() }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        // 与えたダメージでは相手の変更は species(key:) を呼ばない。
        await viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
        let xyDetail = try await stub.lookupSpecies(key: StubMaster.abilityXAndY.key)
        await stub.resolveSpecies(at: baseline, with: .success(xyDetail))
        await loading.value

        XCTAssertEqual(viewModel.opponentSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.opponentAbilityOptions, [], "古い相手の特性を選択肢に入れない")
    }

    // MARK: - 選択 → 要求

    func testSelectOpponentAbilityWithoutObservationOnlyRemembersIt() async throws {
        let stub = await makeStub()
        let viewModel = await viewModelWithXYOpponent(stub)

        let calcs = await reverseCount(stub) { await viewModel.selectOpponentAbility(id: StubMaster.abilityX.id) }
        XCTAssertEqual(calcs, 0, "有効な観測が無いうちは逆算しない")
        XCTAssertEqual(viewModel.opponentAbilityId, StubMaster.abilityX.id)

        try await enterObservation(viewModel)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.unknownAbilityId, StubMaster.abilityX.id, "覚えた特性が次の要求に載る")
        XCTAssertEqual(request.unknownSpeciesKey, StubMaster.abilityXAndY.key)
        XCTAssertNil(request.known.abilityId, "相手の特性を既知側の abilityId に混ぜない")
    }

    func testSelectOpponentAbilityWithObservationCalculatesOncePerChange() async throws {
        let stub = await makeStub()
        let viewModel = await viewModelWithXYOpponent(stub)
        try await enterObservation(viewModel)

        var calcs = await reverseCount(stub) { await viewModel.selectOpponentAbility(id: StubMaster.abilityY.id) }
        XCTAssertEqual(calcs, 1)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.unknownAbilityId, StubMaster.abilityY.id)

        calcs = await reverseCount(stub) { await viewModel.selectOpponentAbility(id: StubMaster.abilityY.id) }
        XCTAssertEqual(calcs, 0, "同じ特性を選び直しても逆算しない")

        calcs = await reverseCount(stub) { await viewModel.selectOpponentAbility(id: StubMaster.ability.id) }
        XCTAssertEqual(calcs, 0, "選択肢に無い特性は無視する")
        XCTAssertEqual(viewModel.opponentAbilityId, StubMaster.abilityY.id)

        calcs = await reverseCount(stub) { await viewModel.selectOpponentAbility(id: nil) }
        XCTAssertEqual(calcs, 1)
        request = try await lastRequest(stub)
        XCTAssertNil(request.unknownAbilityId)
    }

    // MARK: - リセットと引き継ぎ

    func testChangingOpponentSpeciesResetsAbilityWithoutExtraCalc() async throws {
        let stub = await makeStub()
        let viewModel = await viewModelWithXYOpponent(stub)
        try await enterObservation(viewModel)
        await viewModel.selectOpponentAbility(id: StubMaster.abilityX.id)

        let calcs = await reverseCount(stub) { await viewModel.selectOpponentSpecies(key: StubMaster.abilityXOnly.key) }

        XCTAssertEqual(calcs, 1, "相手の変更の逆算回数は変えない(1回)")
        XCTAssertNil(viewModel.opponentAbilityId, "相手の種族が変わったら「指定なし」に戻す")
        XCTAssertEqual(viewModel.opponentAbilityOptions, [], "旧種族の選択肢を残さない")
        let request = try await lastRequest(stub)
        XCTAssertNil(request.unknownAbilityId)
        XCTAssertEqual(request.unknownSpeciesKey, StubMaster.abilityXOnly.key)
    }

    func testSwitchingSideResetsOpponentAbility() async throws {
        let stub = await makeStub()
        let viewModel = await viewModelWithXYOpponent(stub)
        await viewModel.selectOpponentAbility(id: StubMaster.abilityX.id)

        await viewModel.selectSide(.attacker)

        XCTAssertNil(viewModel.opponentAbilityId,
                     "側を切り替えると相手の役割(防御側 ↔ 攻撃側)が変わるので「指定なし」に戻す")
        try await enterObservation(viewModel)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.side, .attacker)
        XCTAssertNil(request.unknownAbilityId)
    }

    func testOtherInputsKeepOpponentAbility() async throws {
        let stub = await makeStub()
        let viewModel = await viewModelWithXYOpponent(stub)
        try await enterObservation(viewModel)
        await viewModel.selectOpponentAbility(id: StubMaster.abilityX.id)

        await viewModel.selectMySpecies(key: StubMaster.gamma.key)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        await viewModel.selectAttackerPreset(.aMax)
        await viewModel.selectMyItem(id: StubMaster.itemA.id)
        await viewModel.toggleOpponentItemCandidate(itemId: StubMaster.itemB.id)

        XCTAssertEqual(viewModel.opponentAbilityId, StubMaster.abilityX.id,
                       "自分・技・プリセット・持ち物の変更では相手の特性を消さない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.unknownAbilityId, StubMaster.abilityX.id)
    }

    // MARK: - 特性で分かれた候補

    func testSplitCandidatesGetUniqueIDsAndAbilityNamesResolvedFromOpponentSpecies() async throws {
        let stub = await makeStub()
        await stub.setReverseResponder(Self.splittingResponder)
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectOpponentSpecies(key: StubMaster.abilityXAndY.key)

        try await enterObservation(viewModel)

        let result = try XCTUnwrap(viewModel.result)
        let ids = result.candidates.map(\.id)
        XCTAssertEqual(ids, [
            "neutral@-@\(StubMaster.abilityX.id)", "neutral@-@\(StubMaster.abilityY.id)",
            "plus@-@\(StubMaster.abilityX.id)", "plus@-@\(StubMaster.abilityY.id)",
        ])
        XCTAssertEqual(Set(ids).count, ids.count)
        XCTAssertEqual(result.candidates.map(\.abilityText), [
            "特性: テストとくせいX", "特性: テストとくせいY", "特性: テストとくせいX", "特性: テストとくせいY",
        ], "名前は相手の species(key:) の abilities から引く")
        let reverseCount = await stub.reverseRequests.count
        XCTAssertEqual(reverseCount, 1, "名前の読み込みで逆算し直さない")
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.isLoading)
    }
}
