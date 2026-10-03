import XCTest

@testable import PokeCalcCore

/// `SpeedViewModel` の表(絞り込み・場の状態)と、結果の表示用の整形(P6-24。ADR-0503 §4・§6)。
/// 表の並び・段・位置は speed-svc の応答のまま(Web の `renderTierRows`・`PositionResult` と同じ整形)。
@MainActor
final class SpeedViewModelTableTests: XCTestCase {
    private let allPresets = SpeedPresetID.allCases

    private func loaded(_ stub: StubSpeedService = StubSpeedService()) async -> (SpeedViewModel, StubSpeedService) {
        let viewModel = SpeedViewModel(service: stub, debounce: .zero)
        await viewModel.load()
        return (viewModel, stub)
    }

    // MARK: - 絞り込み

    func testTogglingAPresetOffReloadsTheTableWithTheRemainingPresetsInContractOrder() async {
        let (viewModel, stub) = await loaded()
        viewModel.toggleFilterPreset(.maxScarf)
        await viewModel.settle()
        XCTAssertFalse(viewModel.selectedFilterPresets.contains(.maxScarf))
        let calls = await stub.tableCalls
        XCTAssertEqual(calls.count, 2)
        XCTAssertEqual(calls.last?.presets, allPresets.filter { $0 != .maxScarf }, "選んだ行だけを契約の順で渡す")
    }

    func testSelectingAllSixAgainOmitsPresets() async {
        let (viewModel, stub) = await loaded()
        viewModel.toggleFilterPreset(.max)
        viewModel.toggleFilterPreset(.max)
        await viewModel.settle()
        let last = await stub.tableCalls.last
        XCTAssertEqual(last?.presets, nil, "全6行に戻したら presets を省く")
    }

    /// 契約上 presets は1つ以上。最後の1つは外せない(Web の toggleFilterPreset と同じ)。
    func testTheLastRemainingPresetCannotBeRemoved() async {
        let (viewModel, stub) = await loaded()
        for id in allPresets.dropLast() { viewModel.toggleFilterPreset(id) }
        await viewModel.settle()
        let last = allPresets[allPresets.count - 1]
        XCTAssertEqual(viewModel.selectedFilterPresets, [last])
        XCTAssertTrue(viewModel.filterMinimumNoticeVisible, "1つだけ残ったら「少なくとも1つは選ぶ必要があります」を出す")
        let before = await stub.tableCalls.count

        viewModel.toggleFilterPreset(last)
        await viewModel.settle()
        XCTAssertEqual(viewModel.selectedFilterPresets, [last], "最後の1つは外れない")
        let after = await stub.tableCalls.count
        XCTAssertEqual(after, before, "外せなかった操作では表を取り直さない")
        let presets = await stub.tableCalls.last?.presets
        XCTAssertEqual(presets, [last])
    }

    func testFilterNoticeIsHiddenWhileTwoOrMoreAreSelected() async {
        let (viewModel, _) = await loaded()
        XCTAssertFalse(viewModel.filterMinimumNoticeVisible)
        for id in allPresets.dropLast(2) { viewModel.toggleFilterPreset(id) }
        XCTAssertEqual(viewModel.selectedFilterPresets.count, 2)
        XCTAssertFalse(viewModel.filterMinimumNoticeVisible)
    }

    // MARK: - 場の状態

    func testTrickRoomReloadsOnlyTheTable() async {
        let (viewModel, stub) = await loaded()
        viewModel.selectPokemon(id: StubSpeed.pokemonA.pokemonId)
        await viewModel.settle()
        let positionBefore = await stub.positionCalls.count

        viewModel.setTrickRoom(true)
        await viewModel.settle()
        let tableCalls = await stub.tableCalls
        XCTAssertEqual(tableCalls.last, StubSpeedService.TableCall(presets: nil, field: SpeedTableField(tailwind: false, trickRoom: true)))
        let positionAfter = await stub.positionCalls.count
        XCTAssertEqual(positionAfter, positionBefore, "トリックルームは位置の入力ではない(faster/slower はそれに依存しない。ADR-0607 §4)")
    }

    func testTableTailwindReloadsTheTableAndThePosition() async {
        let (viewModel, stub) = await loaded()
        viewModel.selectPokemon(id: StubSpeed.pokemonA.pokemonId)
        await viewModel.settle()

        viewModel.setTableTailwind(true)
        await viewModel.settle()
        let tableCalls = await stub.tableCalls
        XCTAssertEqual(tableCalls.last?.field, SpeedTableField(tailwind: true, trickRoom: false))
        let last = await stub.positionCalls.last
        XCTAssertEqual(
            last,
            SpeedPositionRequest(
                input: .preset(pokemonId: StubSpeed.pokemonA.pokemonId, preset: .max, scarf: false, tailwind: false, paralysis: false),
                tableTailwind: true),
            "相手側の追い風は、位置を数える表にも掛かる")
    }

    func testTableIsLoadingWhileItReloads() async throws {
        let (viewModel, stub) = await loaded()
        await stub.hold([.table])
        viewModel.toggleFilterPreset(.max)
        XCTAssertEqual(viewModel.tableState, .loading, "取り直しの間は読み込み中(Web と同じ)")
        try await stub.waitForCalls(.table, count: 2)
        await stub.release(.table, at: 1)
        await viewModel.settle()
        XCTAssertEqual(viewModel.tableState, .loaded(StubSpeed.table()))
    }

    // MARK: - 表の並び・境界

    private func rowsForOwnSpeed(
        _ speed: Int, tiers: [SpeedTier] = StubSpeed.tiersDescending, trickRoom: Bool = false
    ) async -> [SpeedTableRow] {
        let stub = StubSpeedService()
        await stub.setTableResponder { _, _, _ in .success(StubSpeed.table(tiers: tiers)) }
        await stub.setPositionResponder { _, _ in
            // faster/slower は全6行基準の値。境界の判定には使われない(わざと食い違う値を返す)。
            .success(StubSpeed.position(speed: speed, faster: 99, slower: 99))
        }
        let (viewModel, _) = await loaded(stub)
        viewModel.setTrickRoom(trickRoom)
        viewModel.setMode(.raw)
        viewModel.setRawValueText(String(speed))
        await viewModel.settle()
        return viewModel.tableRows
    }

    private func speeds(_ rows: [SpeedTableRow]) -> [String] {
        rows.map {
            switch $0 {
            case .tier(let tier): return String(tier.speed)
            case .selfBoundary: return "|"
            }
        }
    }

    func testRowsWithoutPositionAreTheTiersInResponseOrder() async {
        let (viewModel, _) = await loaded()
        XCTAssertEqual(speeds(viewModel.tableRows), ["300", "250", "200", "100"])
        for case .tier(let tier) in viewModel.tableRows {
            XCTAssertFalse(tier.isSelf)
        }
    }

    func testRowsAreEmptyWhileTheTableIsNotLoaded() {
        let viewModel = SpeedViewModel(service: StubSpeedService(), debounce: .zero)
        XCTAssertEqual(viewModel.tableRows, [])
    }

    func testTierDisplayLabelsAndTieFlag() async {
        let (viewModel, _) = await loaded()
        guard case .tier(let top) = viewModel.tableRows.first, case .tier(let second) = viewModel.tableRows[1] else {
            return XCTFail("先頭2つは段")
        }
        XCTAssertEqual(top.speedLabel, "素早さ 300")
        XCTAssertTrue(top.isTie, "2行以上の段は同速")
        XCTAssertEqual(
            top.entries.map(\.label), ["テストカソウドリ・最速スカーフ", "テストカソウドリ・最速+1"], "「名前・調整」。段の中の並びは応答のまま")
        XCTAssertEqual(top.entries.first?.presetLabel, "最速スカーフ")
        XCTAssertEqual(top.entries.first?.nameJa, "テストカソウドリ")
        XCTAssertEqual(top.entries.first?.primaryType, "fire", "エンブレムは先頭のタイプ")
        XCTAssertFalse(second.isTie)
    }

    func testEntryIdentitiesAreUniquePerPokemonAndPreset() async {
        let (viewModel, _) = await loaded()
        let ids = viewModel.tableRows.compactMap { row -> [String]? in
            if case .tier(let tier) = row { return tier.entries.map(\.id) }
            return nil
        }.flatMap { $0 }
        XCTAssertEqual(Set(ids).count, ids.count)
    }

    /// 同じ実数値の段が表示されていれば、その段を強調し、境界線は引かない。
    func testSameSpeedTierIsMarkedSelfAndHasNoBoundary() async {
        let rows = await rowsForOwnSpeed(250)
        XCTAssertEqual(speeds(rows), ["300", "250", "200", "100"])
        let flags = rows.compactMap { row -> Bool? in if case .tier(let tier) = row { return tier.isSelf } else { return nil } }
        XCTAssertEqual(flags, [false, true, false, false])
    }

    func testBoundaryBetweenTwoTiers() async {
        let rows = await rowsForOwnSpeed(220)
        XCTAssertEqual(speeds(rows), ["300", "250", "|", "200", "100"])
    }

    func testBoundaryAtTheTopWhenFasterThanEveryTier() async {
        let rows = await rowsForOwnSpeed(400)
        XCTAssertEqual(speeds(rows), ["|", "300", "250", "200", "100"])
    }

    func testBoundaryAtTheBottomWhenSlowerThanEveryTier() async {
        let rows = await rowsForOwnSpeed(50)
        XCTAssertEqual(speeds(rows), ["300", "250", "200", "100", "|"])
    }

    /// トリックルーム中の表は遅い順(昇順)。境界は「自分より速い最初の段」の前に引く。
    func testBoundaryUnderTrickRoomFollowsTheAscendingOrder() async {
        let ascending = Array(StubSpeed.tiersDescending.reversed())
        let middle = await rowsForOwnSpeed(220, tiers: ascending, trickRoom: true)
        XCTAssertEqual(speeds(middle), ["100", "200", "|", "250", "300"])
        let top = await rowsForOwnSpeed(50, tiers: ascending, trickRoom: true)
        XCTAssertEqual(speeds(top), ["|", "100", "200", "250", "300"])
        let bottom = await rowsForOwnSpeed(400, tiers: ascending, trickRoom: true)
        XCTAssertEqual(speeds(bottom), ["100", "200", "250", "300", "|"])
    }

    /// 絞り込みで段が減っても、境界は表示中の段の実数値と自分の実数値を直接比べて引く
    /// (応答の faster/slower は全6行基準で、表示と合わない)。
    func testBoundaryUsesTheDisplayedTiersNotTheFasterSlowerCounts() async {
        let filtered = [StubSpeed.tiersDescending[0], StubSpeed.tiersDescending[3]]
        let rows = await rowsForOwnSpeed(220, tiers: filtered)
        XCTAssertEqual(speeds(rows), ["300", "|", "100"])
    }

    func testNoBoundaryOrSelfMarkWhilePositionIsNotLoaded() async {
        let stub = StubSpeedService()
        let (viewModel, _) = await loaded(stub)
        await stub.setPositionResponder { _, _ in .failure(StubSpeed.failure) }
        viewModel.setMode(.raw)
        viewModel.setRawValueText("220")
        await viewModel.settle()
        XCTAssertEqual(speeds(viewModel.tableRows), ["300", "250", "200", "100"], "位置が失敗したら境界は引かない")
    }

    // MARK: - 結果の整形

    func testPositionDisplayShowsTheResponseAsIs() async {
        let stub = StubSpeedService()
        await stub.setPositionResponder { _, _ in
            .success(
                StubSpeed.position(
                    speed: 301, pokemon: StubSpeed.pokemonA, faster: 12, slower: 30,
                    tie: [StubSpeed.entry(StubSpeed.pokemonD, .max)]))
        }
        let (viewModel, _) = await loaded(stub)
        viewModel.selectPokemon(id: StubSpeed.pokemonA.pokemonId)
        await viewModel.settle()
        let display = viewModel.positionDisplay
        XCTAssertEqual(display?.pokemonName, "テストカソウドリ")
        XCTAssertEqual(display?.speedLabel, "実数値 301")
        XCTAssertEqual(display?.fasterLabel, "自分より速い 12行")
        XCTAssertEqual(display?.slowerLabel, "自分より遅い 30行")
        XCTAssertEqual(display?.tieEntries.map(\.label), ["テストデンキネズミ・最速"])
        XCTAssertEqual(display?.hasTie, true)
        XCTAssertNil(display?.movesBeforeLabel, "トリックルームが off のときは行動順の読み替えを出さない")
        XCTAssertNil(display?.movesAfterLabel)
    }

    func testPositionDisplayWithoutTieAndWithoutPokemon() async {
        let stub = StubSpeedService()
        await stub.setPositionResponder { _, _ in .success(StubSpeed.position(speed: 301)) }
        let (viewModel, _) = await loaded(stub)
        viewModel.setMode(.raw)
        viewModel.setRawValueText("301")
        await viewModel.settle()
        XCTAssertNil(viewModel.positionDisplay?.pokemonName)
        XCTAssertEqual(viewModel.positionDisplay?.hasTie, false)
    }

    /// トリックルーム中は faster/slower を行動順に読み替えた行を足す(遅い = 先に動く、速い = 後に動く。ADR-0607 §4)。
    func testTrickRoomAddsMoveOrderLabelsReadFromFasterAndSlower() async {
        let stub = StubSpeedService()
        await stub.setPositionResponder { _, _ in .success(StubSpeed.position(speed: 220, faster: 3, slower: 5)) }
        let (viewModel, _) = await loaded(stub)
        viewModel.setTrickRoom(true)
        viewModel.setMode(.raw)
        viewModel.setRawValueText("220")
        await viewModel.settle()
        let display = viewModel.positionDisplay
        XCTAssertEqual(display?.fasterLabel, "自分より速い 3行", "速い・遅いの行はそのまま残す")
        XCTAssertEqual(display?.movesBeforeLabel, "自分より先に動く 5行", "遅い行(slower)が先に動く")
        XCTAssertEqual(display?.movesAfterLabel, "自分より後に動く 3行", "速い行(faster)が後に動く")
    }

    func testPositionDisplayIsNilUnlessLoaded() async {
        let (viewModel, _) = await loaded()
        XCTAssertNil(viewModel.positionDisplay)
    }
}
