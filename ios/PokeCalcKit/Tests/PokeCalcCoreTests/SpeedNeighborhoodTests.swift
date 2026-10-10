import XCTest

@testable import PokeCalcCore

/// 素早さ画面の「自分の周り」の純粋な切り出し(G-04。ADR-0527。Web の speedNeighborhood.test.ts N1〜N7 と同じ表で同じ規則)。
/// 入力は `tableRows`(表の並び)と自分の実数値。素早さを計算し直さず、段の実数値と自分の実数値の直接比較だけで決める。
final class SpeedNeighborhoodTests: XCTestCase {
    // MARK: - 補助

    private func tier(_ speed: Int, _ names: String...) -> SpeedTableRow {
        let entries = names.map {
            SpeedEntryDisplay(id: "\($0)-\(speed)", nameJa: $0, primaryType: "normal", presetLabel: "最速", label: "\($0)・最速")
        }
        return .tier(
            SpeedTierDisplay(
                speed: speed, speedLabel: SpeedLabels.tierSpeed(speed), isTie: names.count >= 2, isSelf: false, entries: entries))
    }

    /// 降順(速い順)の表(Web の DESC と同じ)。
    private var desc: [SpeedTableRow] {
        [
            tier(300, "A1", "A2"), tier(280, "B1"), tier(260, "C1", "C2", "C3"), tier(240, "D1"), tier(220, "E1"),
            tier(200, "F1"), tier(180, "G1"), tier(160, "H1", "H2"), tier(140, "I1"),
        ]
    }

    /// トリックルームの昇順の表(`desc` の逆)。
    private var asc: [SpeedTableRow] { Array(desc.reversed()) }

    private func speeds(_ tiers: [SpeedNeighborTier]) -> [Int] { tiers.map(\.speed) }

    private func build(
        _ rows: [SpeedTableRow], own: Int?, trickRoom: Bool = false,
        steps: Int = SpeedNeighborhoodBuilder.defaultSteps,
        namesPerTier: Int = SpeedNeighborhoodBuilder.defaultNamesPerTier,
        tieNames: Int = SpeedNeighborhoodBuilder.defaultTieNames
    ) -> SpeedNeighborhood? {
        SpeedNeighborhoodBuilder.build(
            rows: rows, ownSpeed: own, trickRoom: trickRoom, steps: steps, namesPerTier: namesPerTier, tieNames: tieNames)
    }

    // MARK: - N1 通常の場(同速なし)

    func testN1DefaultsToThreeStepsEachSideAndOrdersFarToNear() throws {
        XCTAssertEqual(SpeedNeighborhoodBuilder.defaultSteps, 3)
        let n = try XCTUnwrap(build(desc, own: 230))
        XCTAssertEqual(n.ownSpeed, 230)
        XCTAssertNil(n.tie)
        // 230 より速い段: 300,280,260,240 → 直近 3 段 = 280,260,240(上 = 遠い → 下 = 近い)
        XCTAssertEqual(speeds(n.before), [280, 260, 240])
        // 230 より遅い段: 220,200,180,160,140 → 直近 3 段 = 220,200,180(上 = 近い → 下 = 遠い)
        XCTAssertEqual(speeds(n.after), [220, 200, 180])
    }

    func testN1StepsCanBeChanged() throws {
        let n = try XCTUnwrap(build(desc, own: 230, steps: 1))
        XCTAssertEqual(speeds(n.before), [240])
        XCTAssertEqual(speeds(n.after), [220])
    }

    func testN1EachTierKeepsItsHeadCount() throws {
        let n = try XCTUnwrap(build(desc, own: 230))
        XCTAssertEqual(n.before.first { $0.speed == 260 }?.entryCount, 3)
    }

    /// `selfBoundary` の行が混じっていても、前後の決め方は変わらない(境界の印は切り出しに使わない)。
    func testN1BoundaryRowInTheInputIsIgnored() throws {
        var rows = desc
        rows.insert(.selfBoundary, at: 4)  // 240 と 220 の間 = 実数値 230 の位置
        let n = try XCTUnwrap(build(rows, own: 230))
        XCTAssertEqual(speeds(n.before), [280, 260, 240])
        XCTAssertEqual(speeds(n.after), [220, 200, 180])
        XCTAssertEqual(n.beforeTotal, 7)
        XCTAssertEqual(n.afterTotal, 6)
    }

    // MARK: - N2 同速の段

    func testN2TieTierIsSeparatedAndListsEveryoneUpToTheLimit() throws {
        let n = try XCTUnwrap(build(desc, own: 260))
        XCTAssertEqual(n.tie?.speed, 260)
        XCTAssertEqual(n.tie?.names, ["C1", "C2", "C3"])
        XCTAssertEqual(n.tie?.moreCount, 0)
        XCTAssertEqual(speeds(n.before), [300, 280])
        XCTAssertEqual(speeds(n.after), [240, 220, 200])
    }

    func testN2CrowdedTieShowsTieNamesThenMoreCount() throws {
        let crowd = [tier(200, "P1", "P2", "P3", "P4", "P5", "P6")]
        XCTAssertEqual(SpeedNeighborhoodBuilder.defaultTieNames, 4)
        let n = try XCTUnwrap(build(crowd, own: 200))
        XCTAssertEqual(n.tie?.names, ["P1", "P2", "P3", "P4"])
        XCTAssertEqual(n.tie?.moreCount, 2)
        XCTAssertEqual(n.tie?.entryCount, 6)
    }

    // MARK: - N3 端

    func testN3FasterThanEveryoneLeavesTheBeforeSideEmpty() throws {
        let n = try XCTUnwrap(build(desc, own: 999))
        XCTAssertEqual(n.before, [])
        XCTAssertEqual(n.beforeTotal, 0)
        XCTAssertFalse(n.beforeHasMore)
        XCTAssertEqual(speeds(n.after), [300, 280, 260])
        XCTAssertTrue(n.afterHasMore)
    }

    func testN3SlowerThanEveryoneLeavesTheAfterSideEmpty() throws {
        let n = try XCTUnwrap(build(desc, own: 1))
        XCTAssertEqual(n.after, [])
        XCTAssertEqual(n.afterTotal, 0)
        XCTAssertFalse(n.afterHasMore)
        XCTAssertEqual(speeds(n.before), [180, 160, 140])
    }

    func testN3TiedWithTheFirstTierLeavesTheBeforeSideEmpty() throws {
        let n = try XCTUnwrap(build(desc, own: 300))
        XCTAssertEqual(n.tie?.speed, 300)
        XCTAssertEqual(n.before, [])
        XCTAssertFalse(n.beforeHasMore)
    }

    func testN3HasMoreIsFalseWhenTheShownTiersCoverTheWholeSide() throws {
        let n = try XCTUnwrap(build(desc, own: 150))  // 速い側 7 段、遅い側 1 段(140)
        XCTAssertFalse(n.afterHasMore)
        XCTAssertTrue(n.beforeHasMore)
    }

    // MARK: - N4 合計体数

    func testN4TotalsCoverTheWholeSideIncludingHiddenTiers() throws {
        let n = try XCTUnwrap(build(desc, own: 230))
        XCTAssertEqual(n.beforeTotal, 2 + 1 + 3 + 1)  // 300,280,260,240
        XCTAssertEqual(n.afterTotal, 1 + 1 + 1 + 2 + 1)  // 220,200,180,160,140
        XCTAssertTrue(n.beforeHasMore)
        XCTAssertTrue(n.afterHasMore)
    }

    func testN4TieTierIsInNeitherTotal() throws {
        let n = try XCTUnwrap(build(desc, own: 260))
        XCTAssertEqual(n.beforeTotal, 3)
        XCTAssertEqual(n.afterTotal, 1 + 1 + 1 + 1 + 2 + 1)
    }

    // MARK: - N5 トリックルーム(昇順の表)

    func testN5TrickRoomUsesTableOrderSoSlowTiersAreOnTheBeforeSide() throws {
        let n = try XCTUnwrap(build(asc, own: 230, trickRoom: true))
        // 表は 140,160,…,300。230 より表の前 = 220,200,180,160,140 → 直近 3 段(遠い → 近い)= 180,200,220
        XCTAssertEqual(speeds(n.before), [180, 200, 220])
        // 表の後 = 240,260,… → 直近 3 段 = 240,260,280
        XCTAssertEqual(speeds(n.after), [240, 260, 280])
        XCTAssertEqual(n.beforeTotal, 6)
        XCTAssertEqual(n.afterTotal, 7)
    }

    func testN5TrickRoomTieTierExcludedFromBothSides() throws {
        let n = try XCTUnwrap(build(asc, own: 260, trickRoom: true))
        XCTAssertEqual(n.tie?.speed, 260)
        XCTAssertEqual(speeds(n.before), [200, 220, 240])
        XCTAssertEqual(speeds(n.after), [280, 300])
    }

    func testN5SlowerThanEveryoneMovesFirstSoBeforeSideIsEmptyInTrickRoom() throws {
        let n = try XCTUnwrap(build(asc, own: 1, trickRoom: true))
        XCTAssertEqual(n.before, [])
        XCTAssertEqual(n.beforeTotal, 0)
        XCTAssertEqual(speeds(n.after), [140, 160, 180])
    }

    // MARK: - N6 空・自分未決定

    func testN6NilOwnSpeedGivesNil() {
        XCTAssertNil(build(desc, own: nil))
    }

    func testN6EmptyTableWithOwnSpeedGivesEmptySides() throws {
        let n = try XCTUnwrap(build([], own: 200))
        XCTAssertNil(n.tie)
        XCTAssertEqual(n.before, [])
        XCTAssertEqual(n.after, [])
        XCTAssertEqual(n.beforeTotal, 0)
        XCTAssertEqual(n.afterTotal, 0)
        XCTAssertFalse(n.beforeHasMore)
        XCTAssertFalse(n.afterHasMore)
    }

    // MARK: - N7 代表名・入力不変

    func testN7NearbyTierShowsOneNameAndTheRestAsMoreCount() throws {
        XCTAssertEqual(SpeedNeighborhoodBuilder.defaultNamesPerTier, 1)
        let n = try XCTUnwrap(build(desc, own: 230))
        let c = try XCTUnwrap(n.before.first { $0.speed == 260 })
        XCTAssertEqual(c.names, ["C1"])
        XCTAssertEqual(c.moreCount, 2)
        let b = try XCTUnwrap(n.before.first { $0.speed == 280 })
        XCTAssertEqual(b.names, ["B1"])
        XCTAssertEqual(b.moreCount, 0)
    }

    func testN7NamesPerTierCanBeRaised() throws {
        let n = try XCTUnwrap(build(desc, own: 230, namesPerTier: 2))
        let c = try XCTUnwrap(n.before.first { $0.speed == 260 })
        XCTAssertEqual(c.names, ["C1", "C2"])
        XCTAssertEqual(c.moreCount, 1)
    }

    func testN7InputIsNotMutated() {
        let before = desc
        _ = build(before, own: 230)
        XCTAssertEqual(before, desc)
    }

    // MARK: - 文言(Web の speedScreenText.neighborhood* と同じ。ADR-0609/0527)

    func testLabelsMatchTheWebWording() {
        XCTAssertEqual(SpeedLabels.neighborhoodHeading, "自分の周り")
        XCTAssertEqual(SpeedLabels.neighborhoodFasterList, "自分より速い側")
        XCTAssertEqual(SpeedLabels.neighborhoodSlowerList, "自分より遅い側")
        XCTAssertEqual(SpeedLabels.neighborhoodBeforeList, "先に動く側")
        XCTAssertEqual(SpeedLabels.neighborhoodAfterList, "後に動く側")
        XCTAssertEqual(SpeedLabels.neighborhoodSelfBadge, "自分")
        XCTAssertEqual(SpeedLabels.neighborhoodMore(2), "ほか 2 体")
        XCTAssertEqual(SpeedLabels.neighborhoodFasterTotal(7), "これより速いポケモンは計 7 体")
        XCTAssertEqual(SpeedLabels.neighborhoodSlowerTotal(6), "これより遅いポケモンは計 6 体")
        XCTAssertEqual(SpeedLabels.neighborhoodBeforeTotal(7), "これより先に動くポケモンは計 7 体")
        XCTAssertEqual(SpeedLabels.neighborhoodAfterTotal(6), "これより後に動くポケモンは計 6 体")
        XCTAssertEqual(SpeedLabels.neighborhoodUpArrow, "↑")
        XCTAssertEqual(SpeedLabels.neighborhoodDownArrow, "↓")
    }
}

/// `SpeedViewModel.neighborhood`(`tableRows` と位置の実数値からの結線)。素早さは再計算せず、応答の実数値のまま使う。
@MainActor
final class SpeedNeighborhoodViewModelTests: XCTestCase {
    private func loadedWithRawValue(_ text: String, trickRoom: Bool = false) async -> SpeedViewModel {
        let viewModel = SpeedViewModel(service: MockSpeedService(), debounce: .zero)
        await viewModel.load()
        if trickRoom { viewModel.setTrickRoom(true) }
        viewModel.setMode(.raw)
        viewModel.setRawValueText(text)
        await viewModel.settle()
        return viewModel
    }

    /// N6: 自分が未決定(位置が idle)の間は周りも無い。
    func testNeighborhoodIsNilWhileOwnSpeedIsUndecided() async {
        let viewModel = SpeedViewModel(service: MockSpeedService(), debounce: .zero)
        await viewModel.load()
        XCTAssertNil(viewModel.neighborhood)
    }

    /// 実数値 1 は全行より遅い(モックの固定の事実: 4 体 × 6 行 = 24 行)。通常の場は全員が先に動く側(速い側)。
    func testNeighborhoodUsesTheLoadedTableAndOwnRawValue() async throws {
        let viewModel = await loadedWithRawValue("1")
        let n = try XCTUnwrap(viewModel.neighborhood)
        XCTAssertEqual(n.ownSpeed, 1)
        XCTAssertNil(n.tie)
        XCTAssertEqual(n.beforeTotal, 24)
        XCTAssertEqual(n.afterTotal, 0)
        XCTAssertEqual(n.after, [])
        XCTAssertLessThanOrEqual(n.before.count, SpeedNeighborhoodBuilder.defaultSteps)
    }

    /// トリックルームでは表が昇順なので、同じ実数値 1 でも全員が後に動く側になる。
    func testNeighborhoodFollowsTheTrickRoomOrder() async throws {
        let viewModel = await loadedWithRawValue("1", trickRoom: true)
        let n = try XCTUnwrap(viewModel.neighborhood)
        XCTAssertEqual(n.beforeTotal, 0)
        XCTAssertEqual(n.afterTotal, 24)
        XCTAssertEqual(n.before, [])
    }
}
