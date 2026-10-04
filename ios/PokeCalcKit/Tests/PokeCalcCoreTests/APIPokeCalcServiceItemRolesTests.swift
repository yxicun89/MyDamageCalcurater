import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService` の写像: `Item.roles`・`Item.isMegaStone`(ADR-0175 §3)と
/// `SpeciesDetail.isMega`・`requiredItemId`・`baseSpeciesKey`・`baseSpeciesNameJa`(ADR-0509 §1)。
/// 応答をそのまま写す(省略は nil / false。クライアントで効果から再導出しない)。フィクスチャは架空。
final class APIPokeCalcServiceItemRolesTests: XCTestCase {
    private func makeService(json: String) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: RecordingTransport(json: json))
        return APIPokeCalcService(
            client: client,
            identity: ClientIdentity(deviceID: "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11", sessionID: "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"))
    }

    func testItemRolesAndMegaStoneAreMapped() async throws {
        let json = """
        [{"id":"test-item-both","nameJa":"テストりょうほう","roles":["attacker","defender"],"isMegaStone":false},
         {"id":"test-item-def","nameJa":"テストぼうぎょ","roles":["defender"],"isMegaStone":false},
         {"id":"test-item-stone","nameJa":"Test Stone","roles":[],"isMegaStone":true}]
        """
        let items = try await makeService(json: json).searchItems(query: "", limit: 5)
        XCTAssertEqual(items.map(\.roles), [[.attacker, .defender], [.defender], []])
        XCTAssertEqual(items.map(\.isMegaStone), [false, false, true])
    }

    /// 古いサーバー(項目なし)は nil のまま(「不明」= 役割で絞らない)。
    func testItemWithoutRolesMapsToNil() async throws {
        let json = #"[{"id":"test-item-old","nameJa":"テストむかし"}]"#
        let items = try await makeService(json: json).searchItems(query: "", limit: 5)
        XCTAssertEqual(items, [Item(id: "test-item-old", nameJa: "テストむかし", roles: nil, isMegaStone: nil)])
    }

    func testMegaSpeciesDetailIsMapped() async throws {
        let json = """
        {"key":"9601-001","dexNo":9601,"form":1,"nameJa":"テストメガモン","types":["fighting"],
         "baseStats":{"hp":70,"atk":145,"def":88,"spa":140,"spd":70,"spe":112},
         "abilities":[{"id":"test-ability","nameJa":"テストとくせい"}],"learnset":[],
         "isMega":true,"requiredItemId":"test-item-stone","baseSpeciesKey":"9601-000","baseSpeciesNameJa":"テストモン"}
        """
        let detail = try await makeService(json: json).species(key: "9601-001")
        XCTAssertTrue(detail.isMega)
        XCTAssertEqual(detail.requiredItemId, "test-item-stone")
        XCTAssertEqual(detail.baseSpeciesKey, "9601-000")
        XCTAssertEqual(detail.baseSpeciesNameJa, "テストモン")
    }

    /// 非メガ(null を返す)と古いサーバー(キーなし)は、どちらも非メガ・nil。
    func testNonMegaAndLegacySpeciesDetail() async throws {
        let base = """
        "key":"9601-000","dexNo":9601,"form":0,"nameJa":"テストモン","types":["fighting"],
        "baseStats":{"hp":70,"atk":110,"def":70,"spa":115,"spd":70,"spe":90},"abilities":[],"learnset":[]
        """
        let withNulls = "{\(base),\"isMega\":false,\"requiredItemId\":null,\"baseSpeciesKey\":null,\"baseSpeciesNameJa\":null}"
        for json in [withNulls, "{\(base)}"] {
            let detail = try await makeService(json: json).species(key: "9601-000")
            XCTAssertFalse(detail.isMega)
            XCTAssertNil(detail.requiredItemId)
            XCTAssertNil(detail.baseSpeciesKey)
            XCTAssertNil(detail.baseSpeciesNameJa)
        }
    }
}
