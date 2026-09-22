import XCTest

@testable import PokeCalcCore

// TeamMemberConverter(P6-2c・requirements.md「自分側のプリセット: 構築から個体を呼び出す」の下ごしらえ)。
//
// **範囲外(重要)**: この変換関数を計算画面(`CalcViewModel`)・逆算画面(`ReverseViewModel`)へ実際に
// 配線する(「構築から呼び出す」ボタンを画面に置く)のは、この P6-2c タスクの範囲外。ここでは
// 「後で配線できるように、純粋な変換関数を用意してテストで固定する」ところまで(依頼文の指示どおり)。
// 配線は別タスクの follow-up として `docs/plan.md` に残す。
//
// 決定した形:
// - `TeamMemberConverter.makeIndividual(from: TeamMember, moves: [Move]) -> Individual`(純粋関数)。
// - `speciesKey` / `natureId` / `sp` / `itemId` / `abilityId` / `teraType` は `TeamMember` の値をそのまま写す
//   (`Individual` も ID をそのまま運ぶ値型なので、種族・持ち物・性格のマスタ照合はここでは行わない。
//   マスタに実在するかどうかは画面が表示するとき・API が計算するときに分かればよい)。
// - `moveId`(`Individual` は1つしか持てない)は `member.moveIds` から選ぶ: `moves`(技マスタ)と突き合わせて
//   **最初の変化技でない技**、無ければ `moveIds` の最初、`moveIds` が空なら nil
//   (`CalcViewModel.reselectMove` の「最初のダメージ技、無ければ先頭」と同じ規則に揃える)。
//   `moves` に無い(マスタから外れた)ID は変化技かどうか判定できないので、ダメージ技扱いしない
//   (=飛ばして次を見る)。全部そうなら `moveIds` の最初にフォールバックする。
final class TeamMemberConversionTests: XCTestCase {

    private let physical = Move(id: "test-move-physical", nameJa: "テストわざぶつり", type: .normal, category: .physical, power: 40)
    private let status = Move(id: "test-move-status", nameJa: "テストわざへんか", type: .normal, category: .status, power: 0)
    private let special = Move(id: "test-move-special", nameJa: "テストわざとくしゅ", type: .fire, category: .special, power: 60)

    private var masterMoves: [Move] { [physical, status, special] }

    func testMapsIdentityFieldsDirectly() {
        let member = TeamMember(
            id: "member-1", speciesKey: "test-species-alpha", nickname: "テストのニックネーム",
            moveIds: [physical.id], itemId: "test-item-1", abilityId: "test-ability-1",
            natureId: "test-nature-1", sp: StatBlock(hp: 4, atk: 32, def: 0, spa: 0, spd: 0, spe: 30),
            teraType: .dragon
        )
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertEqual(individual.speciesKey, "test-species-alpha")
        XCTAssertEqual(individual.natureId, "test-nature-1")
        XCTAssertEqual(individual.sp, StatBlock(hp: 4, atk: 32, def: 0, spa: 0, spd: 0, spe: 30))
        XCTAssertEqual(individual.itemId, "test-item-1")
        XCTAssertEqual(individual.abilityId, "test-ability-1")
        XCTAssertEqual(individual.teraType, .dragon)
        XCTAssertEqual(individual.level, fixedLevel)
    }

    func testNoMovesGivesNilMoveId() {
        let member = TeamMember(speciesKey: "test-species-alpha", moveIds: [], natureId: "test-nature-1")
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertNil(individual.moveId)
    }

    func testPrefersFirstNonStatusMove() {
        let member = TeamMember(
            speciesKey: "test-species-alpha", moveIds: [status.id, physical.id, special.id], natureId: "test-nature-1"
        )
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertEqual(individual.moveId, physical.id)
    }

    func testFallsBackToFirstMoveWhenAllStatus() {
        let member = TeamMember(speciesKey: "test-species-alpha", moveIds: [status.id], natureId: "test-nature-1")
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertEqual(individual.moveId, status.id)
    }

    /// マスタに無い技 ID はダメージ技と判定できないので飛ばし、マスタにある変化技でない技を優先する。
    func testUnknownMoveIDsAreSkippedWhenADamagingMoveIsKnown() {
        let member = TeamMember(
            speciesKey: "test-species-alpha", moveIds: ["test-move-unknown", special.id], natureId: "test-nature-1"
        )
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertEqual(individual.moveId, special.id)
    }

    /// 全部マスタに無ければ `moveIds` の最初にフォールバック。
    func testFallsBackToFirstMoveWhenNoneAreKnown() {
        let member = TeamMember(
            speciesKey: "test-species-alpha", moveIds: ["test-move-unknown-1", "test-move-unknown-2"],
            natureId: "test-nature-1"
        )
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertEqual(individual.moveId, "test-move-unknown-1")
    }

    func testOptionalFieldsNilPassThroughAsNil() {
        let member = TeamMember(speciesKey: "test-species-alpha", natureId: "test-nature-1")
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: masterMoves)
        XCTAssertNil(individual.itemId)
        XCTAssertNil(individual.abilityId)
        XCTAssertNil(individual.teraType)
        XCTAssertNil(individual.moveId)
    }
}
