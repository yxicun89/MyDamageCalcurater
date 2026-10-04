import XCTest

@testable import PokeCalcCore

/// `MockPokeCalcService.calcBulk` は防御側のランク(issue #274)付きの要求も受け付け、条件なしと同じ形の行を返す
/// (モックはダメージを計算しない。ADR-0500 §4。ランクで数値が変わることはモックでは確かめない)。
final class MockPokeCalcServiceDefenderRanksTests: XCTestCase {

    func testCalcBulkAcceptsDefenderRanksAndReturnsTheSameRowShape() async throws {
        let mock = try MockPokeCalcService()
        let species = try await mock.searchSpecies(query: "", limit: MasterSearch.pageLimit)
        XCTAssertGreaterThanOrEqual(species.count, 2)
        let attackerDetail = try await mock.species(key: species[0].key)
        let moves = try await mock.searchMoves(query: "", limit: MasterSearch.pageLimit)
        let move = try XCTUnwrap(moves.first { $0.category == .physical && attackerDetail.learnset.contains($0.id) })
        let natures = try await mock.natures()
        let nature = try XCTUnwrap(natures.first)
        let sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        let attacker = Individual(speciesKey: attackerDetail.key, natureId: nature.id, sp: sp)

        let plain = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: species[1].key, moveId: move.id))
        for ranks in [RankBlock(def: 6), RankBlock(def: -6, spd: 6)] {
            let withRanks = try await mock.calcBulk(BulkCalcRequest(
                format: .single, attacker: attacker, defenderSpeciesKey: species[1].key, moveId: move.id,
                defenderRanks: ranks))
            XCTAssertEqual(withRanks.defenderSpeciesKey, plain.defenderSpeciesKey)
            XCTAssertEqual(withRanks.rows.map(\.preset), plain.rows.map(\.preset))
            XCTAssertEqual(withRanks.rows.map(\.itemId), plain.rows.map(\.itemId))
            XCTAssertFalse(withRanks.rows.isEmpty)
        }
    }
}
