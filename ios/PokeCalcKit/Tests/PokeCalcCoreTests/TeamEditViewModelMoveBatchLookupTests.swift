import XCTest

@testable import PokeCalcCore

/// 構築編集画面: 保存済みメンバーの未知の技を `moves(ids:)`(openapi `getMovesByIds`)で
/// **全メンバー分まとめて1回で**解決する(ADR-0501「getMovesByIds による構築編集の技の一括解決」)。
///
/// `TeamEditViewModelMoveLookupTests`(issue #68 の残り。`move(id:)` を1件ずつ呼ぶ実装を固定している)は
/// このタスクでは変更しない。`load()` の実装を `moves(ids:)` の一括呼び出しに切り替えても
/// `StubPokeCalcService.moveLookups`(`move(id:)`/`moves(ids:)` のどちらで引いた ID かを問わず、
/// 引いた ID をフラットに記録する)経由でそのテストのアサーションは満たされ続ける設計にしてある
/// (`StubPokeCalcService.moves(ids:)` の実装コメント参照)。このファイルは `moveBatchRequests`
/// (1回の `moves(ids:)` 呼び出し = 1エントリ)を見て、**呼び出しの回数と中身**そのものを固定する。
@MainActor
final class TeamEditViewModelMoveBatchLookupTests: XCTestCase {

    /// `StubBulkMaster` の先頭ページ + 独自の2種族(それぞれ別の「先頭ページの外」の技を1つ覚える)。
    /// 複数メンバーにまたがる未知の技を1回の `moves(ids:)` にまとめられることを確かめるための架空マスタ
    /// (`StubBulkMaster` 自体は変更しない。追加のテスト専用データはこのファイルに閉じる)。
    private enum BatchFixture {
        static let hiddenMoveA = Move(id: "stub-batch-hidden-a", nameJa: "テストバッチかくれA", type: .water, category: .physical, power: 40)
        static let hiddenMoveB = Move(id: "stub-batch-hidden-b", nameJa: "テストバッチかくれB", type: .fire, category: .special, power: 60)

        static let speciesA = SpeciesDetail(
            key: "8997-000", dexNo: 8997, form: 0, nameJa: "テストバッチしゅぞくA", types: [.water],
            baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
            abilities: [StubMaster.ability],
            learnset: [StubBulkMaster.pageMove(0).id, hiddenMoveA.id]
        )
        static let speciesB = SpeciesDetail(
            key: "8996-000", dexNo: 8996, form: 0, nameJa: "テストバッチしゅぞくB", types: [.fire],
            baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
            abilities: [StubMaster.ability],
            learnset: [hiddenMoveA.id, hiddenMoveB.id]
        )

        static func makeService() -> StubPokeCalcService {
            StubPokeCalcService(
                species: StubBulkMaster.pageSpeciesList + [speciesA, speciesB],
                moves: StubBulkMaster.pageMoveList + [hiddenMoveA, hiddenMoveB],
                items: [StubMaster.itemA, StubMaster.itemB],
                natures: [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature]
            )
        }
    }

    private func loadedViewModel(_ stub: StubPokeCalcService, members: [TeamMember]) async -> TeamEditViewModel {
        let team = Team(id: "team-batch-lookup-1", name: "テストチーム技一括解決", members: members)
        let viewModel = TeamEditViewModel(store: StubTeamStore(), service: stub, team: team, searchDebounce: .zero)
        await viewModel.load()
        return viewModel
    }

    // MARK: - 複数メンバーの未知の技を1回にまとめる

    func testLoadResolvesUnknownMovesOfMultipleMembersInASingleBatchCall() async throws {
        let stub = BatchFixture.makeService()
        await stub.setMoveBatchMode(.immediate)
        let memberA = TeamMember(
            id: "member-a", speciesKey: BatchFixture.speciesA.key,
            moveIds: [StubBulkMaster.pageMove(0).id, BatchFixture.hiddenMoveA.id], natureId: StubMaster.neutralNature.id
        )
        let memberB = TeamMember(
            id: "member-b", speciesKey: BatchFixture.speciesB.key,
            moveIds: [BatchFixture.hiddenMoveB.id], natureId: StubMaster.neutralNature.id
        )

        let viewModel = await loadedViewModel(stub, members: [memberA, memberB])

        let batches = await stub.moveBatchRequests
        XCTAssertEqual(batches.count, 1, "全メンバー分の未知の技を1回の moves(ids:) にまとめる")
        if let firstBatch = batches.first {
            XCTAssertEqual(Set(firstBatch), [BatchFixture.hiddenMoveA.id, BatchFixture.hiddenMoveB.id],
                           "送るのは未知の技だけ(先頭ページで既に見た pageMove(0) は含めない)")
        }
        XCTAssertEqual(viewModel.move(forID: BatchFixture.hiddenMoveA.id)?.nameJa, BatchFixture.hiddenMoveA.nameJa)
        XCTAssertEqual(viewModel.move(forID: BatchFixture.hiddenMoveB.id)?.nameJa, BatchFixture.hiddenMoveB.nameJa)
        XCTAssertEqual(viewModel.move(forID: StubBulkMaster.pageMove(0).id)?.nameJa, StubBulkMaster.pageMove(0).nameJa)
    }

    func testLoadDedupesTheSameUnknownMoveSharedByTwoMembers() async throws {
        let stub = BatchFixture.makeService()
        await stub.setMoveBatchMode(.immediate)
        let memberA = TeamMember(
            id: "member-a", speciesKey: BatchFixture.speciesA.key, moveIds: [BatchFixture.hiddenMoveA.id],
            natureId: StubMaster.neutralNature.id
        )
        let memberB = TeamMember(
            id: "member-b", speciesKey: BatchFixture.speciesB.key,
            moveIds: [BatchFixture.hiddenMoveA.id, BatchFixture.hiddenMoveB.id], natureId: StubMaster.neutralNature.id
        )

        let viewModel = await loadedViewModel(stub, members: [memberA, memberB])

        let batches = await stub.moveBatchRequests
        XCTAssertEqual(batches.count, 1)
        if let requestedIds = batches.first {
            XCTAssertEqual(requestedIds.count, Set(requestedIds).count, "2体が同じ技を持っていても、送る ID は重複させない")
            XCTAssertEqual(Set(requestedIds), [BatchFixture.hiddenMoveA.id, BatchFixture.hiddenMoveB.id])
        }
        XCTAssertEqual(viewModel.move(forID: BatchFixture.hiddenMoveA.id)?.nameJa, BatchFixture.hiddenMoveA.nameJa)
    }

    func testLoadMakesNoBatchCallWhenEveryMembersSavedMoveIsAlreadyKnown() async throws {
        let stub = BatchFixture.makeService()
        await stub.setMoveBatchMode(.immediate)
        let member = TeamMember(
            id: "member-known", speciesKey: BatchFixture.speciesA.key,
            moveIds: [StubBulkMaster.pageMove(0).id], natureId: StubMaster.neutralNature.id
        )

        _ = await loadedViewModel(stub, members: [member])

        let batches = await stub.moveBatchRequests
        XCTAssertEqual(batches, [], "先頭ページで既に見た技だけなら moves(ids:) を呼ばない")
    }

    // MARK: - 失敗は今日の振る舞い(ID のまま・error なし・moveIds 不変)

    func testLoadBatchFailureLeavesTheIDsUnresolvedWithoutAScreenError() async throws {
        let stub = BatchFixture.makeService()
        await stub.setMoveBatchMode(.immediate)
        await stub.setMoveBatchError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト: 通信できない"))
        let member = TeamMember(
            id: "member-a", speciesKey: BatchFixture.speciesA.key, moveIds: [BatchFixture.hiddenMoveA.id],
            natureId: StubMaster.neutralNature.id
        )

        let viewModel = await loadedViewModel(stub, members: [member])

        XCTAssertNil(viewModel.move(forID: BatchFixture.hiddenMoveA.id), "解決できなければ ID のまま(nil)")
        XCTAssertNil(viewModel.error, "補助の解決の失敗で編集画面を止めない")
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [BatchFixture.hiddenMoveA.id], "moveIds は変えない")
        XCTAssertFalse(viewModel.isLoading)
    }

    /// `load()` の Task を、`moves(ids:)` が保留中(`.manual`)のまま cancel する。`StubPokeCalcService.moves(ids:)`
    /// の `withTaskCancellationHandler` 経由で継続が `CancellationError` を受け取り、`resolveUnknownMoves` の
    /// `try?` に飲み込まれる(通信失敗と同じ A3 の扱い。ADR-0501「getMovesByIds による構築編集の技の一括解決」)。
    func testLoadCancellationDuringPendingBatchLeavesStateUnchanged() async throws {
        let stub = BatchFixture.makeService()
        await stub.setMoveBatchMode(.manual)
        let member = TeamMember(
            id: "member-a", speciesKey: BatchFixture.speciesA.key, moveIds: [BatchFixture.hiddenMoveA.id],
            natureId: StubMaster.neutralNature.id
        )
        let team = Team(id: "team-batch-cancel-1", name: "テストチーム技一括解決キャンセル", members: [member])
        let viewModel = TeamEditViewModel(store: StubTeamStore(), service: stub, team: team, searchDebounce: .zero)

        let loadTask = Task { await viewModel.load() }
        try await stub.waitForMoveBatchRequests(count: 1)
        loadTask.cancel()
        try await stub.waitForMoveBatchCancellation(at: 0)
        await loadTask.value

        XCTAssertNil(viewModel.move(forID: BatchFixture.hiddenMoveA.id), "キャンセルされたので解決されないまま")
        XCTAssertNil(viewModel.error, "保留中の一括解決を cancel しても編集画面を止めない")
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [BatchFixture.hiddenMoveA.id], "moveIds は変えない")
        XCTAssertFalse(viewModel.isLoading)
    }
}
