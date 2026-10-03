import XCTest

@testable import PokeCalcCore

/// 持ち物を役割で絞る唯一の関数(ADR-0509 §2。ADR-0175 §4 の規約)。表駆動。
///
/// - 選択肢 = `roles` にその側の役割を含む持ち物(`.either` はどちらかを含む)。並びはマスタの順のまま。
/// - `roles == nil`(古いサーバー・古いモック)は絞らない。`roles` が空(効果なし・メガストーン)は出さない。
/// - `isMegaStone == true` は `roles` に関係なく出さない。
/// - `keeping`(いまの選択)は規則で外れても `items` にあればその1件だけ残す(§5)。
final class ItemRoleFilterTests: XCTestCase {
    private static let attackOnly = Item(id: "test-role-attack", nameJa: "テストこうげきどうぐ", roles: [.attacker], isMegaStone: false)
    private static let defenseOnly = Item(id: "test-role-defense", nameJa: "テストぼうぎょどうぐ", roles: [.defender], isMegaStone: false)
    private static let both = Item(id: "test-role-both", nameJa: "テストりょうほうどうぐ", roles: [.attacker, .defender], isMegaStone: false)
    private static let noRole = Item(id: "test-role-none", nameJa: "テストこうかなしどうぐ", roles: [], isMegaStone: false)
    private static let stone = Item(id: "test-role-stone", nameJa: "Test Mega Stone", roles: [], isMegaStone: true)
    private static let unknown = Item(id: "test-role-unknown", nameJa: "テストふめいどうぐ")
    /// 不整合(古いデータ): 役割は不明だがメガストーン。
    private static let unknownStone = Item(id: "test-role-unknown-stone", nameJa: "Test Old Stone", roles: nil, isMegaStone: true)

    private static let master = [attackOnly, defenseOnly, both, noRole, stone, unknown, unknownStone]

    func testOptionsByRequirement() {
        let cases: [(name: String, requirement: ItemRoleRequirement, expected: [String])] = [
            ("攻撃側: attacker を含むものと不明", .attacker, [Self.attackOnly.id, Self.both.id, Self.unknown.id]),
            ("防御側: defender を含むものと不明", .defender, [Self.defenseOnly.id, Self.both.id, Self.unknown.id]),
            ("either: どちらかを含むものと不明", .either, [Self.attackOnly.id, Self.defenseOnly.id, Self.both.id, Self.unknown.id]),
            ("any: 役割を見ず、メガストーンだけ外す", .any,
             [Self.attackOnly.id, Self.defenseOnly.id, Self.both.id, Self.noRole.id, Self.unknown.id]),
        ]
        for testCase in cases {
            XCTAssertEqual(
                ItemRoleFilter.options(Self.master, for: testCase.requirement).map(\.id), testCase.expected, testCase.name)
        }
    }

    /// `roles` が1件も無い(古いサーバー・古いモック)ときは役割で絞らない(効果から再導出しない)。
    func testItemsWithoutRolesAreNotFiltered() {
        let legacy = [
            Item(id: "test-legacy-a", nameJa: "テストむかしA"),
            Item(id: "test-legacy-b", nameJa: "テストむかしB"),
        ]
        for requirement in [ItemRoleRequirement.attacker, .defender, .either, .any] {
            XCTAssertEqual(ItemRoleFilter.options(legacy, for: requirement), legacy, "\(requirement)")
        }
    }

    /// メガストーンはどの欄にも出さない(`roles` が nil の不整合データでも)。
    func testMegaStonesAreNeverOptions() {
        for requirement in [ItemRoleRequirement.attacker, .defender, .either, .any] {
            let ids = ItemRoleFilter.options(Self.master, for: requirement).map(\.id)
            XCTAssertFalse(ids.contains(Self.stone.id), "\(requirement)")
            XCTAssertFalse(ids.contains(Self.unknownStone.id), "\(requirement)")
        }
    }

    /// 並びはマスタの順のまま(役割で並べ替えない)。
    func testOrderFollowsMaster() {
        let reversed = Array(Self.master.reversed())
        XCTAssertEqual(
            ItemRoleFilter.options(reversed, for: .either).map(\.id),
            [Self.unknown.id, Self.both.id, Self.defenseOnly.id, Self.attackOnly.id])
    }

    /// いまの選択が規則で外れても、その1件だけ残す(マスタの位置に)。nil・マスタに無い ID は何も足さない。
    func testKeepingSelectedItem() {
        let cases: [(name: String, requirement: ItemRoleRequirement, keeping: String?, expected: [String])] = [
            ("攻撃側に防御側専用を持っている", .attacker, Self.defenseOnly.id,
             [Self.attackOnly.id, Self.defenseOnly.id, Self.both.id, Self.unknown.id]),
            ("防御側に役割なしを持っている", .defender, Self.noRole.id,
             [Self.defenseOnly.id, Self.both.id, Self.noRole.id, Self.unknown.id]),
            ("選択肢にあるものは重ねない", .attacker, Self.both.id,
             [Self.attackOnly.id, Self.both.id, Self.unknown.id]),
            ("nil は何も足さない", .attacker, nil, [Self.attackOnly.id, Self.both.id, Self.unknown.id]),
            ("マスタに無い ID は足さない", .attacker, "test-role-missing",
             [Self.attackOnly.id, Self.both.id, Self.unknown.id]),
        ]
        for testCase in cases {
            XCTAssertEqual(
                ItemRoleFilter.options(Self.master, for: testCase.requirement, keeping: testCase.keeping).map(\.id),
                testCase.expected, testCase.name)
        }
    }
}
