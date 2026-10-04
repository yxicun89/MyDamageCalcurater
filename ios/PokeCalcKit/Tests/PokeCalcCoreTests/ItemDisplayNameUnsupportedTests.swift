import XCTest

@testable import PokeCalcCore

/// 未対応の印の注記にも、メガストーンの `nameJa`(英語のことがある)を出さない(ADR-0509 §6)。
final class ItemDisplayNameUnsupportedTests: XCTestCase {
    private static let stone = Item(id: "test-unsup-stone", nameJa: "Test Mega Stone", roles: [], isMegaStone: true)
    private static let berry = Item(id: "test-unsup-berry", nameJa: "テストきのみ", roles: [.defender], isMegaStone: false)

    private func noteName(megaStoneNames: [String: String]) -> String {
        let items = ItemDisplayName.displayItems([Self.berry, Self.stone], megaStoneNames: megaStoneNames)
        let names = UnsupportedMarkNames(moves: [], items: items, abilities: [])
        return names.name(for: UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: Self.stone.id))
    }

    func testKnownMegaStoneUsesBaseSpeciesName() {
        XCTAssertEqual(noteName(megaStoneNames: [Self.stone.id: "テストルカのメガストーン"]), "テストルカのメガストーン")
    }

    func testUnknownMegaStoneIsGeneric() {
        XCTAssertEqual(noteName(megaStoneNames: [:]), "メガストーン")
    }

    func testOrdinaryItemKeepsName() {
        let items = ItemDisplayName.displayItems([Self.berry, Self.stone])
        XCTAssertEqual(items.first(where: { $0.id == Self.berry.id })?.nameJa, "テストきのみ")
    }
}
