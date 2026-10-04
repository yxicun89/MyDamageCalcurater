import XCTest

@testable import PokeCalcCore

/// 逆算画面の持ち物の役割とメガ固定(ADR-0509 §3・§4)。
///
/// - 与えたダメージ(`side == .defender`): 自分 = `.attacker`、相手の候補 = `.defender`。受けたダメージは逆。
/// - 相手がメガ: `itemCandidates = [ストーン]`(null も混ぜない)。候補のトグルは固定中は使わない・ON にできない。
/// - 自分がメガ: `known.itemId = ストーン`。メガ以外に変えたら未選択に戻す。
/// - 相手・受けたダメージの自分の変更では species(key:) を読まない約束を保つ(L1・L2)。
@MainActor
final class ReverseViewModelItemRolesTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private func makeStub() async -> StubPokeCalcService {
        let stub = Mega.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
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

    // MARK: - 側ごとの役割

    func testOptionsFollowSide() async {
        let viewModel = await loadedViewModel(await makeStub())

        XCTAssertEqual(viewModel.side, .defender, "前提: 既定は与えたダメージ")
        XCTAssertEqual(viewModel.myItemOptions.map(\.id), [Mega.attackOnly.id, Mega.both.id])
        XCTAssertEqual(viewModel.opponentItemCandidateOptions.map(\.id), [Mega.defenseOnly.id, Mega.both.id])

        await viewModel.selectSide(.attacker)

        XCTAssertEqual(viewModel.myItemOptions.map(\.id), [Mega.defenseOnly.id, Mega.both.id], "受けたダメージの自分は防御側")
        XCTAssertEqual(viewModel.opponentItemCandidateOptions.map(\.id), [Mega.attackOnly.id, Mega.both.id])
    }

    func testOutOfRoleSelectionsAreIgnored() async {
        let viewModel = await loadedViewModel(await makeStub())

        await viewModel.selectMyItem(id: Mega.defenseOnly.id)
        await viewModel.toggleOpponentItemCandidate(itemId: Mega.attackOnly.id)

        XCTAssertNil(viewModel.myItemId)
        XCTAssertEqual(viewModel.opponentItemCandidateIds, [])
    }

    // MARK: - 自分のメガ

    func testMyMegaLocksStoneOnDealtSide() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        try await enterObservation(viewModel)

        await viewModel.selectMySpecies(key: Mega.megaAlpha.key)

        XCTAssertEqual(viewModel.myItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(viewModel.myItemId, Mega.stone.id)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.known.itemId, Mega.stone.id)

        await viewModel.selectMySpecies(key: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.myItemLock, .none)
        XCTAssertNil(viewModel.myItemId, "メガ以外に変えたら未選択に戻す")
    }

    /// 受けたダメージでは自分の変更で species(key:) を読まない(既存の約束)。View が呼ぶ `loadMySpeciesDetail()` で固定する。
    func testMyMegaOnReceivedSideIsLockedAfterDetailLoad() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectSide(.attacker)
        try await enterObservation(viewModel, "35")
        await viewModel.selectMySpecies(key: Mega.megaAlpha.key)

        let calls = await reverseCount(stub) { await viewModel.loadMySpeciesDetail() }

        XCTAssertEqual(viewModel.myItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(viewModel.myItemId, Mega.stone.id)
        XCTAssertEqual(calls, 1, "固定が変わったので1回計算し直す")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.known.itemId, Mega.stone.id)
    }

    // MARK: - 相手のメガ

    func testOpponentMegaLockAfterDetailLoad() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        try await enterObservation(viewModel)
        let speciesBefore = await stub.speciesRequests.count
        await viewModel.selectOpponentSpecies(key: Mega.megaAlpha.key)
        let speciesAfterSelect = await stub.speciesRequests.count
        XCTAssertEqual(speciesAfterSelect, speciesBefore, "候補が無ければ相手の変更では読まない(既存の約束)")

        let calls = await reverseCount(stub) { await viewModel.loadOpponentAbilityOptions() }

        XCTAssertEqual(viewModel.opponentItemLock, .locked(itemId: Mega.stone.id, displayName: Mega.megaAlphaStoneName))
        XCTAssertEqual(calls, 1)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates, [Mega.stone.id], "メガの相手はストーン1件(null も混ぜない)")
        let labels = try XCTUnwrap(viewModel.result?.candidates.map(\.itemLabel))
        XCTAssertTrue(labels.allSatisfy { $0 == Mega.megaAlphaStoneName }, "候補にストーンの nameJa を出さない: \(labels)")
    }

    func testOpponentMegaWithCandidatesReadsDetailBeforeRequest() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        try await enterObservation(viewModel)
        await viewModel.toggleOpponentItemCandidate(itemId: Mega.both.id)
        let reverseBefore = await stub.reverseRequests.count

        await viewModel.selectOpponentSpecies(key: Mega.megaAlpha.key)

        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests.last, Mega.megaAlpha.key)
        let sent = await stub.reverseRequests.dropFirst(reverseBefore)
        XCTAssertFalse(sent.isEmpty)
        for request in sent {
            XCTAssertEqual(request.itemCandidates, [Mega.stone.id], "メガの相手にストーン以外の候補を送らない")
        }

        await viewModel.toggleOpponentItemCandidate(itemId: Mega.defenseOnly.id)
        XCTAssertEqual(viewModel.opponentItemCandidateIds, [Mega.both.id], "固定中は ON にできない")
    }
}
