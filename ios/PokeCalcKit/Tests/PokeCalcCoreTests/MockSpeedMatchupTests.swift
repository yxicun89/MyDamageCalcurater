import XCTest

@testable import PokeCalcCore

/// `MockSpeedService.matchup` / `reach`(I-speed-4c。ADR-0610 追記「モック」): XCUITest・オフライン用の簡易な式。
/// モックは計算の正しさを保証しない。ここで固定するのは、既存のモックの式(整数の簡易な丸め。`MockSpeedService`)から
/// 手計算できる事実と、シナリオ(失敗の独立)だけ。数値は `MockSpeedServiceTests` と同じ架空の 4 体
/// (A = 種族値 100 / B = 40 / C = 80 / D = 130)。UI テスト(SpeedMatchupUITests)の期待値の元でもある。
///
/// 式(既存のまま): stat = (種族値 + 20 + SP) × 補正(9/10/11)÷ 10。ランクは +n が (2+n)/2、-n が 2/(2+n)。
/// スカーフは ×3/2、追い風は ×2、まひは ÷2(いずれも整数の切り捨て)。表の行は 最速 = 32・上昇、スカーフと +1 は最速の ×3/2、+2 は ×2。
final class MockSpeedMatchupTests: XCTestCase {
    private let pokemonA = "9001-000"
    private let pokemonD = "9004-000"

    private func matchup(
        assumptions: [SpeedAssumption]? = nil, trickRoom: Bool = false, current: SpeedInvestment? = nil,
        opponentTailwind: Bool = false, service: MockSpeedService = MockSpeedService()
    ) async throws -> SpeedMatchup {
        try await service.matchup(
            SpeedMatchupRequest(
                own: .init(pokemonId: pokemonA, current: current),
                opponent: .init(pokemonId: pokemonD, assumptions: assumptions, tailwind: opponentTailwind),
                trickRoom: trickRoom))
    }

    // MARK: - matchup

    func testDefaultAssumptionsAreTheFourPresetsWithTheirSpeeds() async throws {
        let result = try await matchup()
        XCTAssertEqual(result.selfPokemon.pokemonId, pokemonA)
        XCTAssertEqual(result.opponent.pokemonId, pokemonD)
        XCTAssertEqual(
            result.lines.map(\.assumption),
            [.preset(.uninvested), .preset(.neutralMax), .preset(.max), .preset(.maxScarf)], "省略時は 無振り・準速・最速・最速スカーフ")
        XCTAssertEqual(result.lines.map(\.opponentSpeed), [150, 182, 200, 300], "D(種族値 130)の 無振り / 準速 / 最速 / 最速スカーフ")
        XCTAssertNil(result.selfSpeed)
        XCTAssertTrue(result.lines.allSatisfy { $0.current == nil })
        XCTAssertTrue(result.lines.allSatisfy { $0.natures.map(\.nature) == [.minus, .neutral, .plus] })
    }

    func testRangesAgainstTheUninvestedOpponent() async throws {
        let line = try await matchup(assumptions: [.preset(.uninvested)]).lines[0]
        // 自分 A(種族値 100)の素早さ: 下降 (120+SP)×9/10(最大 136)/ 補正なし 120+SP / 上昇 (120+SP)×11/10。相手 150。
        let minus = line.natures[0]
        XCTAssertNil(minus.movesFirst, "下降は SP 32 でも 136 で届かない")
        XCTAssertNil(minus.tie)
        let neutral = line.natures[1]
        XCTAssertEqual(neutral.movesFirst, SpeedSPRange(min: 31, max: 32))
        XCTAssertEqual(neutral.tie, SpeedSPRange(min: 30, max: 30), "同速は上を取るに含めない")
        let plus = line.natures[2]
        XCTAssertEqual(plus.movesFirst, SpeedSPRange(min: 18, max: 32))
        XCTAssertEqual(plus.tie, SpeedSPRange(min: 17, max: 17))
    }

    func testNothingReachesTheFasterAssumptions() async throws {
        let result = try await matchup(assumptions: [.preset(.neutralMax), .preset(.max), .preset(.maxScarf)])
        for line in result.lines {
            XCTAssertTrue(line.natures.allSatisfy { $0.movesFirst == nil && $0.tie == nil }, "最速 A(167)は 182 以上に届かない")
        }
    }

    func testTrickRoomFlipsTheRangesToTheUpperLimit() async throws {
        let line = try await matchup(assumptions: [.preset(.uninvested)], trickRoom: true).lines[0]
        XCTAssertEqual(line.natures[0].movesFirst, SpeedSPRange(min: 0, max: 32), "下降は最大でも 136 < 150 なので全部下を取れる")
        XCTAssertEqual(line.natures[1].movesFirst, SpeedSPRange(min: 0, max: 29))
        XCTAssertEqual(line.natures[1].tie, SpeedSPRange(min: 30, max: 30))
        XCTAssertEqual(line.natures[2].movesFirst, SpeedSPRange(min: 0, max: 16))
        XCTAssertEqual(line.natures[2].tie, SpeedSPRange(min: 17, max: 17))
    }

    func testCurrentInvestmentAddsSelfSpeedAndTurnOrder() async throws {
        let result = try await matchup(
            assumptions: [.preset(.uninvested), .preset(.max)], current: SpeedInvestment(sp: 32, nature: .plus))
        XCTAssertEqual(result.selfSpeed, 167, "A の SP 32・上昇 = 152 × 11/10")
        XCTAssertEqual(result.lines.map(\.current), [.selfFirst, .opponentFirst], "167 は 150 より速く、200 より遅い")
        let underTrickRoom = try await matchup(
            assumptions: [.preset(.uninvested), .preset(.max)], trickRoom: true, current: SpeedInvestment(sp: 32, nature: .plus))
        XCTAssertEqual(underTrickRoom.lines.map(\.current), [.opponentFirst, .selfFirst], "トリックルーム中は遅い方が先")
    }

    func testOpponentTailwindDoublesAndCustomAssumptionIsApplied() async throws {
        let tailwind = try await matchup(assumptions: [.preset(.uninvested)], opponentTailwind: true)
        XCTAssertEqual(tailwind.lines[0].opponentSpeed, 300)
        let custom = try await matchup(assumptions: [.custom(SpeedCustomAssumption(sp: 0, nature: .neutral, rank: 1))])
        XCTAssertEqual(custom.lines[0].opponentSpeed, 225, "150 のランク +1 = ×3/2")
        XCTAssertEqual(custom.lines[0].assumption, .custom(SpeedCustomAssumption(sp: 0, nature: .neutral, rank: 1)))
    }

    func testUnknownPokemonFailsWithTheContractCode() async {
        let service = MockSpeedService()
        let error = await assertThrowsPokeCalcError("自分が未知") {
            try await service.matchup(
                SpeedMatchupRequest(own: .init(pokemonId: "0000-000"), opponent: .init(pokemonId: self.pokemonD)))
        }
        XCTAssertEqual(error?.code, "unknown_pokemon")
        let opponent = await assertThrowsPokeCalcError("相手が未知") {
            try await service.matchup(
                SpeedMatchupRequest(own: .init(pokemonId: self.pokemonA), opponent: .init(pokemonId: "0000-000")))
        }
        XCTAssertEqual(opponent?.code, "unknown_pokemon")
    }

    // MARK: - reach(自分の素早さ 170 = 実数値)

    private func reach(
        _ value: Int = 170, tableTailwind: Bool = false, service: MockSpeedService = MockSpeedService()
    ) async throws -> SpeedReach {
        try await service.reach(SpeedPositionRequest(input: .raw(value: value, pokemonId: nil), tableTailwind: tableTailwind))
    }

    /// ADR-0610 §1 の手計算の例(自分 v = 170)と同じ区切りになる式にしてある。
    func testReachBoundariesMatchTheHandCalculatedExample() async throws {
        let result = try await reach()
        XCTAssertEqual(result.speed, 170)
        XCTAssertNil(result.pokemon)
        XCTAssertEqual(result.lines.map(\.preset), SpeedPresetID.allCases)
        func bounds(_ line: SpeedReachLine) -> [Int?] {
            [line.slower?.max, line.tie?.min, line.tie?.max, line.faster?.min]
        }
        XCTAssertEqual(bounds(result.lines[0]), [149, 150, 150, 151], "無振り: 149 族まで抜ける・150 族と同速・151 族以上に抜かれる")
        XCTAssertEqual(bounds(result.lines[1]), [117, 118, 118, 119], "準速")
        XCTAssertEqual(bounds(result.lines[2]), [102, 103, 103, 104], "最速")
        XCTAssertEqual(bounds(result.lines[3]), [51, nil, nil, 52], "最速スカーフ: 同速なし")
        XCTAssertEqual(bounds(result.lines[5]), [25, 26, 26, 27], "最速+2")
        XCTAssertEqual(result.lines[0].slower?.min, 1)
        XCTAssertEqual(result.lines[0].faster?.max, 255)
    }

    func testReachCountsAndBoundaryPokemonComeFromTheMockMaster() async throws {
        let lines = try await reach().lines
        // 無振りは X+20: A 120 / B 60 / C 100 / D 150 はすべて 170 より遅い。
        XCTAssertEqual([lines[0].slowerCount, lines[0].tieCount, lines[0].fasterCount], [4, 0, 0])
        XCTAssertEqual(lines[0].fastestSlower?.pokemonId, pokemonD)
        XCTAssertNil(lines[0].slowestFaster)
        // 最速スカーフは A 250 / B 151 / C 217 / D 300。遅いのは B だけ。速い側で最も遅いのは C。
        XCTAssertEqual([lines[3].slowerCount, lines[3].tieCount, lines[3].fasterCount], [1, 0, 3])
        XCTAssertEqual(lines[3].fastestSlower?.pokemonId, "9002-000")
        XCTAssertEqual(lines[3].slowestFaster?.pokemonId, "9003-000")
        // 最速+2(ランクではなく ×2)は 4 体とも 170 より速い。区間は種族値 1〜255 で決まり、使えるポケモンには依存しない。
        XCTAssertEqual([lines[5].slowerCount, lines[5].tieCount, lines[5].fasterCount], [0, 0, 4])
        XCTAssertNil(lines[5].fastestSlower)
        XCTAssertEqual(lines[5].slowestFaster?.pokemonId, "9002-000")
        XCTAssertEqual(lines[5].slower, SpeedBaseSpeedRange(min: 1, max: 25))
    }

    func testReachWithTableTailwindDoublesThePresetSide() async throws {
        let line = try await reach(tableTailwind: true).lines[0]
        XCTAssertEqual(line.slower?.max, 64, "(X+20)×2 < 170")
        XCTAssertEqual(line.tie, SpeedBaseSpeedRange(min: 65, max: 65))
        XCTAssertEqual(line.faster?.min, 66)
    }

    func testReachFromACustomInputUsesTheSameSpeedAsPosition() async throws {
        let result = try await MockSpeedService().reach(
            SpeedPositionRequest(
                input: .custom(pokemonId: pokemonA, sp: 32, nature: .plus, rank: 0, scarf: false, tailwind: false, paralysis: false)))
        XCTAssertEqual(result.speed, 167)
        XCTAssertEqual(result.pokemon?.pokemonId, pokemonA)
        XCTAssertEqual(result.lines.count, 6)
    }

    func testReachRejectsBadInput() async {
        let service = MockSpeedService()
        let invalid = await assertThrowsPokeCalcError("実数値 0") { try await self.reach(0, service: service) }
        XCTAssertEqual(invalid?.code, "invalid_request")
        let unknown = await assertThrowsPokeCalcError("未知のポケモン") {
            try await service.reach(SpeedPositionRequest(input: .raw(value: 100, pokemonId: "0000-000")))
        }
        XCTAssertEqual(unknown?.code, "unknown_pokemon")
    }

    // MARK: - シナリオ(失敗の独立)

    func testMatchupErrorScenarioFailsMatchupAndReachOnlyWithTheGatewayCode() async {
        XCTAssertEqual(MockSpeedScenario(environmentValue: "matchup-error"), .matchupError)
        let service = MockSpeedService(scenario: .matchupError)
        let matchupError = await assertThrowsPokeCalcError("matchup") { try await self.matchup(service: service) }
        XCTAssertEqual(matchupError?.code, "upstream_unavailable", "gateway が上流に届かない 503 を再現する")
        let reachError = await assertThrowsPokeCalcError("reach") { try await self.reach(service: service) }
        XCTAssertEqual(reachError?.code, "upstream_unavailable")
        let pokemon = try? await service.pokemon()
        XCTAssertNotNil(pokemon, "ポケモン一覧は成功する(選択はできる)")
    }

    func testAllErrorScenarioAlsoFailsMatchupAndReach() async {
        let service = MockSpeedService(scenario: .allError)
        let matchupError = await assertThrowsPokeCalcError("matchup") { try await self.matchup(service: service) }
        XCTAssertEqual(matchupError?.code, "master_unavailable")
        let reachError = await assertThrowsPokeCalcError("reach") { try await self.reach(service: service) }
        XCTAssertEqual(reachError?.code, "master_unavailable")
    }

    func testOtherScenariosDoNotAffectMatchupAndReach() async throws {
        for scenario in [MockSpeedScenario.tableError, .positionError, .pokemonError] {
            let service = MockSpeedService(scenario: scenario)
            _ = try await matchup(service: service)
            _ = try await reach(service: service)
        }
    }
}
