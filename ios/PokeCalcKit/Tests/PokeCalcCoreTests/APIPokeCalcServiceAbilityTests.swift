import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService` の特性の写像(issue #272。ADR-0501「P6-19」1章、ADR-0214)。
///
/// 要求: `BulkCalcRequest.defenderAbilityId` → `defenderOverride.abilityId`(nil なら `defenderOverride` を送らない)、
/// `ReverseRequest.unknownAbilityId` → `unknownAbilityId`(nil なら送らない)。
/// 応答: `BulkCalcRow`・`ReverseCandidate` の `abilityId`・`abilityIds` を並べ替えずにドメインへ写す。
/// 期待値は api/openapi.yaml のプロパティ名から書く。フィクスチャは架空(ADR-0002)。
/// 既存の `APIPokeCalcServiceTests` は変えない(その private な補助は使えないので、ここに最小限を持つ)。
final class APIPokeCalcServiceAbilityTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private let attacker = Individual(
        speciesKey: "9001-000", natureId: "test-nature-atk",
        sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0)
    )

    private static let calcResultJSON = """
    {"rolls":[40,40,41,41,42,42,43,43,44,44,45,45,46,46,47,48],
     "minDamage":40,"maxDamage":48,"minPercent":30.3,"maxPercent":36.4,"defenderHP":132,
     "effectiveness":1,"stab":false,"category":"physical",
     "ko":{"hits":3,"guaranteed":false,"chancePercent":12.34,"displayChancePercent":12.3},
     "unsupported":[]}
    """

    private static let zeroResultJSON = """
    {"rolls":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],
     "minDamage":0,"maxDamage":0,"minPercent":0,"maxPercent":0,"defenderHP":132,
     "effectiveness":1,"stab":false,"category":"physical",
     "ko":{"hits":0,"guaranteed":false,"chancePercent":0,"displayChancePercent":0},
     "unsupported":[]}
    """

    private static let defenderJSON = """
    {"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
     "nature":{"plus":null,"minus":null},"natureId":"test-nature-neutral",
     "stats":{"hp":132,"atk":90,"def":80,"spa":70,"spd":85,"spe":90}}
    """

    /// 同じ (none, 持ち物なし) が特性で2行に分かれた応答(特性 A と C は同じ結果、B だけ無効)。
    private static var splitBulkJSON: String {
        """
        {"defenderSpeciesKey":"9002-000","rows":[
          {"preset":"none","presetLabel":"テスト無振り","itemId":null,"defender":\(defenderJSON),
           "result":\(calcResultJSON),"abilityId":"test-ability-a","abilityIds":["test-ability-a","test-ability-c"]},
          {"preset":"none","presetLabel":"テスト無振り","itemId":null,"defender":\(defenderJSON),
           "result":\(zeroResultJSON),"abilityId":"test-ability-b","abilityIds":["test-ability-b"]}
        ]}
        """
    }

    private static let splitReverseJSON = """
    {"side":"defender","stat":"def","assumedHpSp":32,"exactCount":1,"candidates":[
      {"natureClass":"neutral","nature":{"plus":null,"minus":null},"natureId":"test-nature-neutral",
       "itemId":null,"ranges":[{"min":0,"max":32}],"spCount":33,
       "exact":true,"mismatch":0,"support":16,"minPercent":30.3,"maxPercent":36.4,"unsupported":[],
       "abilityId":"test-ability-c","abilityIds":["test-ability-c","test-ability-a"]},
      {"natureClass":"neutral","nature":{"plus":null,"minus":null},"natureId":"test-nature-neutral",
       "itemId":null,"ranges":[{"min":0,"max":32}],"spCount":33,
       "exact":false,"mismatch":1,"support":0,"minPercent":0,"maxPercent":0,"unsupported":[],
       "abilityId":"test-ability-b","abilityIds":["test-ability-b"]}
    ]}
    """

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIPokeCalcService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private func bulkRequest(defenderAbilityId: String?) -> BulkCalcRequest {
        BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
            defenderAbilityId: defenderAbilityId
        )
    }

    private func reverseRequest(unknownAbilityId: String?) -> ReverseRequest {
        ReverseRequest(
            format: .single, side: .defender, known: attacker, unknownSpeciesKey: "9002-000",
            moveId: "test-move-physical", observations: [.percent(30)], unknownAbilityId: unknownAbilityId
        )
    }

    // MARK: - 要求

    func testCalcBulkSendsDefenderOverrideAbilityIdOnlyWhenSpecified() async throws {
        let pinned = RecordingTransport(json: Self.splitBulkJSON)
        _ = try await makeService(transport: pinned).calcBulk(bulkRequest(defenderAbilityId: "test-ability-b"))
        let pinnedBody = try XCTUnwrap(pinned.requests.first).jsonBody()
        let override = try XCTUnwrap(pinnedBody["defenderOverride"] as? [String: Any], "指定したら defenderOverride を送る")
        XCTAssertEqual(override["abilityId"] as? String, "test-ability-b")
        XCTAssertNil(
            (pinnedBody["attacker"] as? [String: Any])?["abilityId"] as? String,
            "防御側の特性を攻撃側の abilityId に混ぜない"
        )

        let unspecified = RecordingTransport(json: Self.splitBulkJSON)
        _ = try await makeService(transport: unspecified).calcBulk(bulkRequest(defenderAbilityId: nil))
        let unspecifiedBody = try XCTUnwrap(unspecified.requests.first).jsonBody()
        XCTAssertNil(unspecifiedBody["defenderOverride"],
                     "指定なしは defenderOverride 自体を送らない(サーバーが種族の特性をすべて試す。これまでの本文と同じ)")
    }

    func testReverseSendsUnknownAbilityIdOnlyWhenSpecified() async throws {
        let pinned = RecordingTransport(json: Self.splitReverseJSON)
        _ = try await makeService(transport: pinned).reverse(reverseRequest(unknownAbilityId: "test-ability-a"))
        let pinnedBody = try XCTUnwrap(pinned.requests.first).jsonBody()
        XCTAssertEqual(pinnedBody["unknownAbilityId"] as? String, "test-ability-a")
        XCTAssertNil((pinnedBody["known"] as? [String: Any])?["abilityId"] as? String,
                     "相手の特性を既知側の abilityId に混ぜない")

        let unspecified = RecordingTransport(json: Self.splitReverseJSON)
        _ = try await makeService(transport: unspecified).reverse(reverseRequest(unknownAbilityId: nil))
        let unspecifiedBody = try XCTUnwrap(unspecified.requests.first).jsonBody()
        XCTAssertNil(unspecifiedBody["unknownAbilityId"], "指定なしは送らない")
    }

    // MARK: - 応答

    func testCalcBulkRowsMapAbilityIdAndAbilityIdsInOrder() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.splitBulkJSON))
        let result = try await service.calcBulk(bulkRequest(defenderAbilityId: nil))
        XCTAssertEqual(result.rows.count, 2, "特性で分かれた行を落とさない")
        XCTAssertEqual(result.rows.map(\.abilityId), ["test-ability-a", "test-ability-b"])
        XCTAssertEqual(result.rows.map(\.abilityIds), [["test-ability-a", "test-ability-c"], ["test-ability-b"]],
                       "abilityIds はサーバーの順のまま(並べ替えない)")
        XCTAssertEqual(result.rows.map(\.result.maxDamage), [48, 0])
    }

    func testReverseCandidatesMapAbilityIdAndAbilityIdsInOrder() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.splitReverseJSON))
        let result = try await service.reverse(reverseRequest(unknownAbilityId: nil))
        XCTAssertEqual(result.candidates.map(\.abilityId), ["test-ability-c", "test-ability-b"])
        XCTAssertEqual(result.candidates.map(\.abilityIds), [["test-ability-c", "test-ability-a"], ["test-ability-b"]])
    }
}
