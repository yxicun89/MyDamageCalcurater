import XCTest

@testable import PokeCalcCore

/// `MockPokeCalcService.calcBulk` は計算条件(issue #274)付きの要求も受け付け、条件なしと同じ形の行を返す
/// (モックはダメージを計算しない。ADR-0500 §4。条件で数値が変わることはモックでは確かめない)。
final class MockPokeCalcServiceConditionsTests: XCTestCase {

    func testCalcBulkAcceptsConditionsAndReturnsTheSameRowShape() async throws {
        let mock = try MockPokeCalcService()
        let species = try await mock.searchSpecies(query: "", limit: MasterSearch.pageLimit)
        XCTAssertGreaterThanOrEqual(species.count, 2)
        let attackerDetail = try await mock.species(key: species[0].key)
        let moves = try await mock.searchMoves(query: "", limit: MasterSearch.pageLimit)
        let move = try XCTUnwrap(moves.first { $0.category == .physical && attackerDetail.learnset.contains($0.id) })
        let natures = try await mock.natures()
        let nature = try XCTUnwrap(natures.first)
        let ability = try XCTUnwrap(attackerDetail.abilities.first)
        let sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)

        let plain = try await mock.calcBulk(BulkCalcRequest(
            format: .single,
            attacker: Individual(speciesKey: attackerDetail.key, natureId: nature.id, sp: sp),
            defenderSpeciesKey: species[1].key, moveId: move.id))
        let withConditions = try await mock.calcBulk(BulkCalcRequest(
            format: .single,
            attacker: Individual(
                speciesKey: attackerDetail.key, natureId: nature.id, sp: sp, abilityId: ability.id,
                ranks: RankBlock(atk: 6, spa: -6), status: .burn),
            defenderSpeciesKey: species[1].key, moveId: move.id,
            field: FieldState(
                weather: .snow, terrain: .misty,
                defenderScreens: Screens(reflect: true, lightScreen: true, auroraVeil: true)),
            critical: true))

        XCTAssertEqual(withConditions.defenderSpeciesKey, plain.defenderSpeciesKey)
        XCTAssertEqual(withConditions.rows.map(\.preset), plain.rows.map(\.preset))
        XCTAssertEqual(withConditions.rows.map(\.itemId), plain.rows.map(\.itemId))
        XCTAssertFalse(withConditions.rows.isEmpty)
    }
}
