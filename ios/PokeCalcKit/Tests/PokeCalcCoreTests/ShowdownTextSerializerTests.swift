import XCTest

@testable import PokeCalcCore

// P6-20: `ShowdownTextSerializer`(純粋関数・テーブル駆動。ADR-0501「P6-20」2章)。
final class ShowdownTextSerializerTests: XCTestCase {
    static let names = ShowdownNames(
        speciesByKey: ["9101-000": "テストアルファ", "9102-000": "テストベータ"],
        itemsById: ["stub-item-a": "テストどうぐA"],
        abilitiesById: ["stub-ability": "テストとくせい"],
        naturesById: ["stub-nature-atk-up": "テストせいかく攻撃上昇", "stub-nature-neutral": "テストせいかく無補正"],
        movesById: [
            "stub-move-physical": "テストわざぶつり", "stub-move-special": "テストわざとくしゅ",
            "stub-move-status": "テストわざへんか", "stub-move-alpha-only": "テストわざアルファ専用",
        ],
        typeNames: [.fire: "ほのお", .water: "みず"]
    )

    private func member(
        species: String = "9101-000", nickname: String? = nil, moves: [String] = [], item: String? = nil,
        ability: String? = nil, nature: String = "stub-nature-atk-up", sp: StatBlock = zeroSP, tera: PokeType? = nil
    ) -> TeamMember {
        TeamMember(
            id: "m-\(species)", speciesKey: species, nickname: nickname, moveIds: moves, itemId: item,
            abilityId: ability, natureId: nature, sp: sp, teraType: tera
        )
    }

    func testFullMemberExactText() throws {
        let m = member(
            moves: ["stub-move-physical", "stub-move-status"], item: "stub-item-a", ability: "stub-ability",
            sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 20), tera: .fire
        )
        let out = try XCTUnwrap(ShowdownTextSerializer.serialize(member: m, names: Self.names))
        XCTAssertEqual(out.text, """
            テストアルファ @ テストどうぐA
            Ability: テストとくせい
            Tera Type: ほのお
            SP: 32 Atk / 20 Spe
            Nature: テストせいかく攻撃上昇
            - テストわざぶつり
            - テストわざへんか
            """)
        XCTAssertEqual(out.unresolvedIds, [])
    }

    func testOptionalLinesAreOmittedTable() throws {
        struct Row {
            let name: String
            let member: TeamMember
            let text: String
        }
        let rows = [
            Row(name: "最小(持ち物・特性・テラ・SP・技なし)", member: member(), text: "テストアルファ\nNature: テストせいかく攻撃上昇"),
            Row(name: "ニックネーム", member: member(nickname: "ニック", item: "stub-item-a"),
                text: "ニック (テストアルファ) @ テストどうぐA\nNature: テストせいかく攻撃上昇"),
            Row(name: "SP は 0 のステータスを省く", member: member(sp: StatBlock(hp: 32, atk: 0, def: 0, spa: 0, spd: 0, spe: 2)),
                text: "テストアルファ\nSP: 32 HP / 2 Spe\nNature: テストせいかく攻撃上昇"),
            Row(name: "SP 全部 0 なら SP 行ごと省く", member: member(sp: zeroSP), text: "テストアルファ\nNature: テストせいかく攻撃上昇"),
            Row(name: "6 つとも", member: member(sp: StatBlock(hp: 1, atk: 2, def: 3, spa: 4, spd: 5, spe: 6)),
                text: "テストアルファ\nSP: 1 HP / 2 Atk / 3 Def / 4 SpA / 5 SpD / 6 Spe\nNature: テストせいかく攻撃上昇"),
            Row(name: "技 4 つ", member: member(moves: ["stub-move-physical", "stub-move-special", "stub-move-status", "stub-move-alpha-only"]),
                text: "テストアルファ\nNature: テストせいかく攻撃上昇\n- テストわざぶつり\n- テストわざとくしゅ\n- テストわざへんか\n- テストわざアルファ専用"),
        ]
        for row in rows {
            let out = try XCTUnwrap(ShowdownTextSerializer.serialize(member: row.member, names: Self.names), row.name)
            XCTAssertEqual(out.text, row.text, row.name)
            XCTAssertFalse(out.text.hasSuffix("\n"), "末尾に改行を付けない: \(row.name)")
        }
    }

    func testUnresolvedNamesAreOmittedAndReported() throws {
        let m = member(
            moves: ["stub-move-physical", "stub-move-unknown"], item: "stub-item-missing", ability: "stub-ability-missing",
            nature: "stub-nature-missing", tera: .water
        )
        let out = try XCTUnwrap(ShowdownTextSerializer.serialize(member: m, names: Self.names))
        XCTAssertEqual(out.text, "テストアルファ\nTera Type: みず\n- テストわざぶつり")
        XCTAssertEqual(
            Set(out.unresolvedIds),
            ["stub-move-unknown", "stub-item-missing", "stub-ability-missing", "stub-nature-missing"]
        )
    }

    func testUnknownSpeciesYieldsNil() {
        XCTAssertNil(ShowdownTextSerializer.serialize(member: member(species: "no-such"), names: Self.names))
    }

    func testTeamJoinsWithOneBlankLineAndSkipsUnknownSpecies() {
        let a = member(species: "9101-000")
        let bad = member(species: "no-such")
        let b = member(species: "9102-000", nature: "stub-nature-neutral")
        let out = ShowdownTextSerializer.serialize(members: [a, bad, b], names: Self.names)
        XCTAssertEqual(
            out.text,
            "テストアルファ\nNature: テストせいかく攻撃上昇\n\nテストベータ\nNature: テストせいかく無補正"
        )
        XCTAssertEqual(out.skippedMemberIds, [bad.id])
    }

    func testEmptyTeamIsEmptyText() {
        XCTAssertEqual(ShowdownTextSerializer.serialize(members: [], names: Self.names), ShowdownTeamText(text: ""))
    }

    /// 書き出したテキストを取り込み直すと、名前の組が元と同じになる(書式の往復)。
    func testRoundTripThroughParser() throws {
        let m = member(
            nickname: "ニック", moves: ["stub-move-physical", "stub-move-status"], item: "stub-item-a",
            ability: "stub-ability", sp: StatBlock(hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 30), tera: .fire
        )
        let text = try XCTUnwrap(ShowdownTextSerializer.serialize(member: m, names: Self.names)).text
        let result = ShowdownTextParser.parse(text)
        XCTAssertEqual(result.rejected, [])
        let parsed = try XCTUnwrap(result.members.first)
        XCTAssertEqual(parsed.speciesName, "テストアルファ")
        XCTAssertEqual(parsed.nickname, "ニック")
        XCTAssertEqual(parsed.itemName, "テストどうぐA")
        XCTAssertEqual(parsed.abilityName, "テストとくせい")
        XCTAssertEqual(parsed.natureName, "テストせいかく攻撃上昇")
        XCTAssertEqual(parsed.teraTypeName, "ほのお")
        XCTAssertEqual(parsed.sp, [.hp: 2, .atk: 32, .spe: 30])
        XCTAssertEqual(parsed.moveNames, ["テストわざぶつり", "テストわざへんか"])
    }
}
