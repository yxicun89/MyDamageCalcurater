import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService` の防御側のランクの写像(issue #274。ADR-0501「防御側のランクの受け入れ条件」、ADR-0216)。
///
/// `BulkCalcRequest.defenderRanks`・`defenderAbilityId` → `defenderOverride { abilityId?, ranks? }`。
/// 「abilityId も ranks も無いなら `defenderOverride` 自体を送らない」(これまでの要求本文と同じ)。
/// ranks が非 0 のときは 5 項目(atk/def/spa/spd/spe。0 も含む)を送り、`status` は送らない(式に効かない)。
/// 期待値は api/openapi.yaml のプロパティ名から書く。フィクスチャは架空(ADR-0002)。
/// 既存の `APIPokeCalcServiceTests` / `APIPokeCalcServiceAbilityTests` は変えない。
final class APIPokeCalcServiceDefenderRanksTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private let attacker = Individual(
        speciesKey: "9001-000", natureId: "test-nature-atk",
        sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0),
        ranks: RankBlock(atk: 2)
    )

    private static let bulkJSON = """
    {"defenderSpeciesKey":"9002-000","rows":[
      {"preset":"none","presetLabel":"テスト無振り","itemId":null,
       "defender":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
         "nature":{"plus":null,"minus":null},"natureId":"test-nature-neutral",
         "stats":{"hp":132,"atk":90,"def":80,"spa":70,"spd":85,"spe":90}},
       "result":{"rolls":[40,40,41,41,42,42,43,43,44,44,45,45,46,46,47,48],
         "minDamage":40,"maxDamage":48,"minPercent":30.3,"maxPercent":36.4,"defenderHP":132,
         "effectiveness":1,"stab":false,"category":"physical",
         "ko":{"hits":3,"guaranteed":false,"chancePercent":12.34,"displayChancePercent":12.3},
         "unsupported":[]},
       "abilityId":"test-ability-a","abilityIds":["test-ability-a"]}
    ]}
    """

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIPokeCalcService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private func body(abilityId: String?, ranks: RankBlock) async throws -> [String: Any] {
        let transport = RecordingTransport(json: Self.bulkJSON)
        let request = BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
            defenderAbilityId: abilityId, defenderRanks: ranks)
        _ = try await makeService(transport: transport).calcBulk(request)
        return try XCTUnwrap(transport.requests.first).jsonBody()
    }

    private func intValues(_ object: [String: Any]?) -> [String: Int] {
        (object ?? [:]).compactMapValues { $0 as? Int }
    }

    // MARK: - defenderOverride の有無の全組み合わせ

    func testNoAbilityAndDefaultRanksOmitsDefenderOverride() async throws {
        let json = try await body(abilityId: nil, ranks: RankBlock())
        XCTAssertNil(json["defenderOverride"], "abilityId も ranks も無いなら defenderOverride 自体を送らない(従来の本文と同じ)")
    }

    func testAbilityOnlySendsAbilityIdWithoutRanks() async throws {
        let json = try await body(abilityId: "test-ability-b", ranks: RankBlock())
        let override = try XCTUnwrap(json["defenderOverride"] as? [String: Any])
        XCTAssertEqual(override["abilityId"] as? String, "test-ability-b")
        XCTAssertNil(override["ranks"], "既定(0・0)の ranks は送らない")
        XCTAssertNil(override["status"])
    }

    func testRanksOnlySendsAllFiveRankFieldsWithoutAbilityId() async throws {
        let json = try await body(abilityId: nil, ranks: RankBlock(def: 1))
        let override = try XCTUnwrap(json["defenderOverride"] as? [String: Any], "ranks が非 0 なら defenderOverride を送る")
        XCTAssertNil(override["abilityId"], "特性を指定していないなら abilityId は送らない")
        XCTAssertNil(override["status"], "防御側の状態異常は送らない(式に効かない。ADR-0216 §3)")
        XCTAssertEqual(
            intValues(override["ranks"] as? [String: Any]),
            ["atk": 0, "def": 1, "spa": 0, "spd": 0, "spe": 0], "ranks は 5 項目すべて(0 も含む)")
    }

    func testAbilityAndRanksCoexistInTheSameDefenderOverride() async throws {
        let json = try await body(abilityId: "test-ability-a", ranks: RankBlock(def: -2, spd: 3))
        let override = try XCTUnwrap(json["defenderOverride"] as? [String: Any])
        XCTAssertEqual(override["abilityId"] as? String, "test-ability-a")
        XCTAssertEqual(
            intValues(override["ranks"] as? [String: Any]),
            ["atk": 0, "def": -2, "spa": 0, "spd": 3, "spe": 0])
        XCTAssertNil(override["status"])
    }

    func testSpdOnlyAndExtremeValuesAreSentAsIs() async throws {
        for ranks in [RankBlock(spd: -6), RankBlock(def: 6), RankBlock(def: -6, spd: 6)] {
            let json = try await body(abilityId: nil, ranks: ranks)
            let override = try XCTUnwrap(json["defenderOverride"] as? [String: Any], "\(ranks)")
            XCTAssertEqual(
                intValues(override["ranks"] as? [String: Any]),
                ["atk": ranks.atk, "def": ranks.def, "spa": ranks.spa, "spd": ranks.spd, "spe": ranks.spe],
                "\(ranks)")
        }
    }

    // MARK: - 攻撃側と混ざらない

    func testDefenderRanksDoNotLeakIntoAttackerRanks() async throws {
        let json = try await body(abilityId: nil, ranks: RankBlock(def: 4, spd: -4))
        let attackerJSON = try XCTUnwrap(json["attacker"] as? [String: Any])
        XCTAssertEqual(
            intValues(attackerJSON["ranks"] as? [String: Any]),
            ["atk": 2, "def": 0, "spa": 0, "spd": 0, "spe": 0],
            "攻撃側の ranks は攻撃側の値のまま。防御側のランクを混ぜない")
    }

    func testOtherBodyFieldsAreUnchangedByDefenderRanks() async throws {
        let plain = try await body(abilityId: nil, ranks: RankBlock())
        let withRanks = try await body(abilityId: nil, ranks: RankBlock(def: 1))
        for key in ["format", "attacker", "defenderSpeciesKey", "moveId", "options"] {
            let lhs = try XCTUnwrap(plain[key] as? NSObject, key)
            let rhs = try XCTUnwrap(withRanks[key] as? NSObject, key)
            XCTAssertEqual(lhs, rhs, key)
        }
        XCTAssertNil(plain["field"])
        XCTAssertNil(withRanks["field"])
        XCTAssertNil(withRanks["presets"])
        XCTAssertNil(withRanks["itemVariants"])
    }
}
