import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService.calcBulk` が計算条件(issue #274)を openapi `BulkCalcRequest` に写すこと
/// (ADR-0501「issue #274」4章)。期待値は api/openapi.yaml の `FieldState`・`Screens`・`CalcOptions`・
/// `Individual` のプロパティ名から書く。フィクスチャは架空(ADR-0002)。
///
/// 既存の `APIPokeCalcServiceTests` は変えない(その private な補助は使えないので、ここに最小限を持つ)。
final class APIPokeCalcServiceConditionsTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    /// 応答の中身はこのテストでは見ない(要求の写像だけを見る)。行0件の最小の正しい応答。
    private static let emptyBulkJSON = #"{"defenderSpeciesKey":"9002-000","rows":[]}"#

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIPokeCalcService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private func attacker(
        abilityId: String? = nil, ranks: RankBlock = RankBlock(), status: StatusCondition = .none
    ) -> Individual {
        Individual(
            speciesKey: "9001-000", natureId: "test-nature-atk",
            sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0),
            abilityId: abilityId, ranks: ranks, status: status
        )
    }

    private func sentBody(_ request: BulkCalcRequest) async throws -> [String: Any] {
        let transport = RecordingTransport(json: Self.emptyBulkJSON)
        let service = try makeService(transport: transport)
        _ = try await service.calcBulk(request)
        return try XCTUnwrap(transport.requests.first).jsonBody()
    }

    /// 全条件ありの要求: 場・急所・攻撃側のランク・状態異常・特性がそれぞれ決まった場所に載る。
    func testCalcBulkMapsFieldCriticalRanksStatusAndAbility() async throws {
        let body = try await sentBody(BulkCalcRequest(
            format: .single,
            attacker: attacker(abilityId: "test-ability-alpha", ranks: RankBlock(atk: 6, spa: -6), status: .burn),
            defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
            field: FieldState(
                weather: .sun, terrain: .psychic,
                defenderScreens: Screens(reflect: true, lightScreen: false, auroraVeil: true)
            ),
            critical: true
        ))

        let field = try XCTUnwrap(body["field"] as? [String: Any], "既定でない場は field に載せる")
        XCTAssertEqual(field["weather"] as? String, "sun")
        XCTAssertEqual(field["terrain"] as? String, "psychic")
        let defenderScreens = try XCTUnwrap(field["defenderScreens"] as? [String: Bool])
        XCTAssertEqual(defenderScreens["reflect"], true)
        XCTAssertEqual(defenderScreens["auroraVeil"], true)
        XCTAssertNotEqual(defenderScreens["lightScreen"], true, "張っていない壁は false か省略")
        if let attackerScreens = field["attackerScreens"] as? [String: Bool] {
            XCTAssertFalse(attackerScreens.values.contains(true), "攻撃側の壁は張っていない")
        }

        let options = try XCTUnwrap(body["options"] as? [String: Any])
        XCTAssertEqual(options["critical"] as? Bool, true)

        let attackerJSON = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(attackerJSON["status"] as? String, "burn")
        XCTAssertEqual(attackerJSON["abilityId"] as? String, "test-ability-alpha")
        XCTAssertEqual(attackerJSON["ranks"] as? [String: Int], ["atk": 6, "def": 0, "spa": -6, "spd": 0, "spe": 0])
    }

    /// 壁・天候・フィールドの全値が openapi の値の綴りで送られる(`lightScreen` の綴り等)。
    func testCalcBulkMapsEveryWeatherTerrainAndScreen() async throws {
        for weather in Weather.allCases where weather != .none {
            let body = try await sentBody(BulkCalcRequest(
                format: .single, attacker: attacker(), defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
                field: FieldState(weather: weather)))
            let field = try XCTUnwrap(body["field"] as? [String: Any], "\(weather)")
            XCTAssertEqual(field["weather"] as? String, weather.rawValue)
            XCTAssertNotEqual(field["terrain"] as? String, "electric", "フィールドは既定(none か省略)")
        }
        for terrain in Terrain.allCases where terrain != .none {
            let body = try await sentBody(BulkCalcRequest(
                format: .single, attacker: attacker(), defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
                field: FieldState(terrain: terrain)))
            let field = try XCTUnwrap(body["field"] as? [String: Any], "\(terrain)")
            XCTAssertEqual(field["terrain"] as? String, terrain.rawValue)
        }
        for kind in ScreenKind.allCases {
            let body = try await sentBody(BulkCalcRequest(
                format: .single, attacker: attacker(), defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
                field: FieldState(defenderScreens: Screens().setting(kind, to: true))))
            let field = try XCTUnwrap(body["field"] as? [String: Any], "\(kind)")
            let screens = try XCTUnwrap(field["defenderScreens"] as? [String: Bool], "\(kind)")
            XCTAssertEqual(screens[kind.rawValue], true, "\(kind)")
            XCTAssertEqual(screens.filter { $0.value }.count, 1, "\(kind) だけが true")
        }
    }

    /// 既定の場(`FieldState()`)は `field` を送らない(この機能より前の要求本文と同じにする。4章「判断」)。
    func testCalcBulkOmitsFieldWhenDefault() async throws {
        let body = try await sentBody(BulkCalcRequest(
            format: .single, attacker: attacker(), defenderSpeciesKey: "9002-000", moveId: "test-move-physical"))
        XCTAssertNil(body["field"], "何もない場は送らない")
        let options = try XCTUnwrap(body["options"] as? [String: Any])
        XCTAssertEqual(options["critical"] as? Bool, false, "急所はこれまでどおり明示の false")
    }
}
