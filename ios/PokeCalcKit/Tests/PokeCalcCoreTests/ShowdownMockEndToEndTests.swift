import XCTest

@testable import PokeCalcCore

// P6-20: `MockPokeCalcService`(架空データ 9001〜9004)で書き出し → 取り込みが通る。XCUITest はこの経路を使う。
final class ShowdownMockEndToEndTests: XCTestCase {
    func testMockResolvesFixtureNames() async throws {
        let transfer = try ShowdownTransferService(service: MockPokeCalcService())
        let plan = await transfer.resolve(ShowdownTextParser.parse("""
            テストモンいち @ テストどうぐきのみ
            Ability: テストとくせいアルファ
            Tera Type: ほのお
            SP: 32 Atk / 2 Spe
            Nature: テストせいかく攻撃上昇
            - テストわざぶつりA
            - テストわざへんかA

            テストモンよん
            Ability: テストとくせいかくとうむこう
            """), existingMemberCount: 0)
        XCTAssertEqual(plan.rejected, [])
        XCTAssertNil(plan.error)
        XCTAssertEqual(plan.members.map(\.speciesKey), ["9001-000", "9004-000"])
        let first = try XCTUnwrap(plan.members.first)
        XCTAssertEqual(first.itemId, "test-item-berry")
        XCTAssertEqual(first.abilityId, "test-ability-alpha")
        XCTAssertEqual(first.natureId, "test-nature-atk-up")
        XCTAssertEqual(first.moveIds, ["test-move-physical-a", "test-move-status-a"])
        XCTAssertEqual(first.sp, StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 2))
        XCTAssertEqual(plan.members.last?.abilityId, "test-ability-fighting-immune")
    }

    func testMockRoundTrip() async throws {
        let transfer = try ShowdownTransferService(service: MockPokeCalcService())
        let member = TeamMember(
            id: "m", speciesKey: "9002-000", nickname: "ニック", moveIds: ["test-move-special-b", "test-move-status-a"],
            itemId: "test-item-berry", abilityId: "test-ability-beta", natureId: "test-nature-def-up",
            sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0), teraType: .water
        )
        let exported = await transfer.export(members: [member])
        let text = try XCTUnwrap(exported.text)
        let plan = await transfer.resolve(ShowdownTextParser.parse(text), existingMemberCount: 0)
        XCTAssertEqual(plan.rejected, [])
        var back = try XCTUnwrap(plan.members.first)
        back.id = member.id
        XCTAssertEqual(back, member)
    }
}
