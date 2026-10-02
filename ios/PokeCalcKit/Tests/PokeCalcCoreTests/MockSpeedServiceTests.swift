import XCTest

@testable import PokeCalcCore

/// `MockSpeedService`(P6-24。ADR-0503 §8): XCUITest・オフラインが頼る固定の事実と、環境変数でのシナリオ切り替え。
/// モックは計算の正しさを保証しない(正は speed-svc)。ここで固定するのは「画面を確かめるのに足りる決定的な形」だけ。
///
/// 固定する事実(ADR-0503 §8。UI テストが依存する):
/// - ポケモンは4体(9001-000〜9004-000。名前はすべて「テスト」で始まる)。
/// - 表(全6行)は 4 体 × 6 行 = 24 行。同じ段に2行以上ある段(同速)が少なくとも1つある。
/// - 表のどの段の実数値も、位置の `faster`/`slower`/`tie` と同じ基準(全6行)で数える。
final class MockSpeedServiceTests: XCTestCase {
    private let service = MockSpeedService()

    private func fullTable() async throws -> SpeedTable {
        try await service.table(presets: nil, field: SpeedTableField())
    }

    private func rows(_ table: SpeedTable) -> [SpeedTableEntry] { table.tiers.flatMap(\.entries) }

    // MARK: - 一覧・表

    func testPokemonAreFourFictionalEntriesSortedById() async throws {
        let list = try await service.pokemon()
        XCTAssertEqual(list.pokemon.map(\.pokemonId), ["9001-000", "9002-000", "9003-000", "9004-000"])
        for pokemon in list.pokemon {
            XCTAssertTrue(pokemon.nameJa.hasPrefix("テスト"), "架空データの名前は「テスト」で始める: \(pokemon.nameJa)")
            XCTAssertFalse(pokemon.types.isEmpty)
            XCTAssertTrue((1...255).contains(pokemon.baseSpeed))
        }
    }

    func testFullTableHasTwentyFourRowsInDescendingTiersWithATie() async throws {
        let table = try await fullTable()
        XCTAssertEqual(table.presets, SpeedPresetID.allCases)
        XCTAssertEqual(rows(table).count, 24, "4 体 × 6 行")
        let speeds = table.tiers.map(\.speed)
        XCTAssertEqual(speeds, speeds.sorted(by: >), "速い順")
        XCTAssertEqual(Set(speeds).count, speeds.count, "同じ値は1つの段にまとめる")
        XCTAssertTrue(table.tiers.contains { $0.entries.count >= 2 }, "同速の段が少なくとも1つある")
    }

    func testEntriesInsideATierAreSortedByPokemonIdThenPresetOrder() async throws {
        let table = try await fullTable()
        for tier in table.tiers {
            let keys = tier.entries.map { "\($0.pokemonId)|\(SpeedPresetID.allCases.firstIndex(of: $0.preset) ?? -1)" }
            let sorted = tier.entries.sorted {
                ($0.pokemonId, SpeedPresetID.allCases.firstIndex(of: $0.preset) ?? -1)
                    < ($1.pokemonId, SpeedPresetID.allCases.firstIndex(of: $1.preset) ?? -1)
            }
            XCTAssertEqual(tier.entries, sorted, "段の中の並び: \(keys)")
        }
    }

    func testPresetsFilterKeepsOnlyThoseRowsInContractOrder() async throws {
        let table = try await service.table(presets: [.maxScarf, .max], field: SpeedTableField())
        XCTAssertEqual(table.presets, [.max, .maxScarf], "実際に使った調整は契約の順(クエリの順ではない)")
        XCTAssertEqual(rows(table).count, 8, "4 体 × 2 行")
        XCTAssertEqual(Set(rows(table).map(\.preset)), [.max, .maxScarf])
    }

    func testTrickRoomReversesTierOrderAndKeepsTheSpeeds() async throws {
        let normal = try await fullTable()
        let trick = try await service.table(presets: nil, field: SpeedTableField(trickRoom: true))
        XCTAssertEqual(trick.tiers.map(\.speed), normal.tiers.map(\.speed).reversed())
        XCTAssertEqual(trick.tiers.map(\.entries), normal.tiers.reversed().map(\.entries), "段の中の並びは反転しない")
    }

    func testTailwindDoublesEverySpeed() async throws {
        let normal = try await fullTable()
        let windy = try await service.table(presets: nil, field: SpeedTableField(tailwind: true))
        XCTAssertEqual(windy.tiers.map(\.speed), normal.tiers.map { $0.speed * 2 })
    }

    // MARK: - 位置

    func testRawValuePositionCountsAgainstTheFullTable() async throws {
        let table = try await fullTable()
        let lowest = try XCTUnwrap(table.tiers.last).speed
        let slowest = try await service.position(SpeedPositionRequest(input: .raw(value: 1, pokemonId: nil)))
        XCTAssertEqual(slowest.speed, 1)
        XCTAssertEqual(slowest.faster, 24)
        XCTAssertEqual(slowest.slower, 0)
        XCTAssertEqual(slowest.tie, [])
        XCTAssertNil(slowest.pokemon)
        XCTAssertLessThan(1, lowest)

        for tier in table.tiers {
            let position = try await service.position(SpeedPositionRequest(input: .raw(value: tier.speed, pokemonId: nil)))
            XCTAssertEqual(position.tie, tier.entries, "同じ実数値の段が同速の一覧")
            XCTAssertEqual(position.faster + position.tie.count + position.slower, 24)
        }
    }

    func testRawPositionWithPokemonEchoesItForDisplay() async throws {
        let position = try await service.position(SpeedPositionRequest(input: .raw(value: 100, pokemonId: "9002-000")))
        XCTAssertEqual(position.pokemon?.pokemonId, "9002-000")
        XCTAssertTrue(position.pokemon?.nameJa.hasPrefix("テスト") == true)
    }

    /// 同じポケモンの「最速 + スカーフ」と表の「最速+1」は同じ実数値になる(本物と同じ性質。UI テストが同速を出すのに使う)。
    func testPresetMaxWithScarfTiesWithOwnRowsOfTheTable() async throws {
        let position = try await service.position(
            SpeedPositionRequest(
                input: .preset(pokemonId: "9001-000", preset: .max, scarf: true, tailwind: false, paralysis: false)))
        XCTAssertEqual(position.pokemon?.pokemonId, "9001-000")
        XCTAssertGreaterThanOrEqual(position.tie.count, 2, "9001-000 の最速スカーフ・最速+1 の2行と同速")
        XCTAssertTrue(position.tie.contains { $0.pokemonId == "9001-000" && $0.preset == .maxScarf })
        XCTAssertTrue(position.tie.contains { $0.pokemonId == "9001-000" && $0.preset == .maxPlus1 })
    }

    func testTableTailwindMovesTheTableAwayFromOwnSpeed() async throws {
        let table = try await fullTable()
        let top = try XCTUnwrap(table.tiers.first).speed
        let plain = try await service.position(SpeedPositionRequest(input: .raw(value: top + 1, pokemonId: nil)))
        XCTAssertEqual(plain.faster, 0, "表の最速より速い")
        let windy = try await service.position(
            SpeedPositionRequest(input: .raw(value: top + 1, pokemonId: nil), tableTailwind: true))
        XCTAssertGreaterThan(windy.faster, 0, "相手側の追い風で表が 2 倍になり、追い越される行が出る")
    }

    func testCustomAndPresetPositionsAreDeterministic() async throws {
        let request = SpeedPositionRequest(
            input: .custom(pokemonId: "9003-000", sp: 20, nature: .plus, rank: 1, scarf: false, tailwind: false, paralysis: false))
        let first = try await service.position(request)
        let second = try await service.position(request)
        XCTAssertEqual(first, second)
        XCTAssertGreaterThan(first.speed, 0)
    }

    func testUnknownPokemonIsUnknownPokemonError() async throws {
        let request = SpeedPositionRequest(
            input: .preset(pokemonId: "0000-000", preset: .max, scarf: false, tailwind: false, paralysis: false))
        let error = await assertThrowsPokeCalcError("unknown") { try await service.position(request) }
        XCTAssertEqual(error?.code, "unknown_pokemon")
    }

    func testRawValueBelowOneIsInvalidRequest() async throws {
        let error = await assertThrowsPokeCalcError("zero") {
            try await service.position(SpeedPositionRequest(input: .raw(value: 0, pokemonId: nil)))
        }
        XCTAssertEqual(error?.code, "invalid_request")
    }

    // MARK: - シナリオ(環境変数 POKECALC_MOCK_SPEED)

    func testScenarioParsingFollowsTheEnvironmentValue() {
        XCTAssertEqual(MockSpeedService.scenarioEnvironmentKey, "POKECALC_MOCK_SPEED")
        XCTAssertEqual(MockSpeedScenario(environmentValue: nil), .normal)
        XCTAssertEqual(MockSpeedScenario(environmentValue: "unknown"), .normal)
        XCTAssertEqual(MockSpeedScenario(environmentValue: "table-error"), .tableError)
        XCTAssertEqual(MockSpeedScenario(environmentValue: "position-error"), .positionError)
        XCTAssertEqual(MockSpeedScenario(environmentValue: "pokemon-error"), .pokemonError)
        XCTAssertEqual(MockSpeedScenario(environmentValue: "all-error"), .allError)
    }

    private func outcomes(_ scenario: MockSpeedScenario) async -> (pokemon: String?, table: String?, position: String?) {
        let service = MockSpeedService(scenario: scenario)
        func code(_ body: () async throws -> Void) async -> String? {
            do {
                try await body()
                return nil
            } catch let error as PokeCalcError {
                return error.code
            } catch {
                return "unexpected: \(error)"
            }
        }
        let pokemon = await code { _ = try await service.pokemon() }
        let table = await code { _ = try await service.table(presets: nil, field: SpeedTableField()) }
        let position = await code {
            _ = try await service.position(SpeedPositionRequest(input: .raw(value: 100, pokemonId: nil)))
        }
        return (pokemon, table, position)
    }

    func testScenariosFailOnlyTheirOwnOperation() async {
        let normal = await outcomes(.normal)
        XCTAssertEqual([normal.pokemon, normal.table, normal.position].compactMap { $0 }, [])
        let tableError = await outcomes(.tableError)
        XCTAssertEqual(tableError.table, "master_unavailable")
        XCTAssertNil(tableError.pokemon)
        XCTAssertNil(tableError.position)
        let positionError = await outcomes(.positionError)
        XCTAssertEqual(positionError.position, "master_unavailable")
        XCTAssertNil(positionError.pokemon)
        XCTAssertNil(positionError.table)
        let pokemonError = await outcomes(.pokemonError)
        XCTAssertEqual(pokemonError.pokemon, "master_unavailable")
        XCTAssertNil(pokemonError.table)
        XCTAssertNil(pokemonError.position)
        let all = await outcomes(.allError)
        XCTAssertEqual([all.pokemon, all.table, all.position].compactMap { $0 }, Array(repeating: "master_unavailable", count: 3))
    }

    func testEnvironmentInitializerReadsTheScenario() async {
        let service = MockSpeedService(environment: [MockSpeedService.scenarioEnvironmentKey: "table-error"])
        let error = await assertThrowsPokeCalcError("table") { try await service.table(presets: nil, field: SpeedTableField()) }
        XCTAssertEqual(error?.code, "master_unavailable")
    }
}
