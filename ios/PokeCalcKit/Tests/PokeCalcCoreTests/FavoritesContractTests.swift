import Foundation
import PokeCalcAPI
import XCTest

/// P5-3c(ADR-0227): お気に入り API の生成型(`make ios-gen`)の形を固定する契約の網。
/// API レーンが契約を先に出し、iOS レーンが画面(お気に入りの一覧・ピン留め・外す)を作るときの前提を確かめる。
/// ここが落ちたら(またはコンパイルできなくなったら)、契約の変更に画面側の写像を追従させる。
/// 架空の key だけを使う(実データは使わない。ADR-0002)。
final class FavoritesContractTests: XCTestCase {
    private func decoder() -> JSONDecoder {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }

    /// 一覧の応答(`Favorite` の配列)を読める。`individual` は計算 API と同じ `Individual` 型、`id` は文字列、
    /// `label` は null を取りうる。
    func testFavoriteListResponseDecodes() throws {
        let json = #"""
        [{"id":"42","label":"HB特化","individual":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
          "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0},
          "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
          "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-02T00:00:00Z"},
         {"id":"7","label":null,"individual":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
          "sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
          "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
          "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T00:00:00Z"}]
        """#
        let list = try decoder().decode([Components.Schemas.Favorite].self, from: Data(json.utf8))
        XCTAssertEqual(list.map(\.id), ["42", "7"])
        XCTAssertEqual(list[0].label, "HB特化")
        XCTAssertNil(list[1].label)
        let individual: Components.Schemas.Individual = list[0].individual
        XCTAssertEqual(individual.speciesKey, "9002-000")
        XCTAssertLessThan(list[1].updatedAt, list[0].updatedAt)
    }

    /// 作成の本文は `label`(任意)と `individual`(`Individual` そのもの。包む型を挟まない)。
    func testFavoriteInputUsesIndividualDirectly() throws {
        let json = #"""
        {"label":"物理受け","individual":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
          "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0}}}
        """#
        let input = try decoder().decode(Components.Schemas.FavoriteInput.self, from: Data(json.utf8))
        let individual: Components.Schemas.Individual = input.individual
        XCTAssertEqual(individual.natureId, "fake-nature")
        XCTAssertEqual(input.label, "物理受け")
    }

    /// ADR-0228(usability-round2 F-09): `calc` を持つお気に入りは、計算の入力全体を `CalcRequest` そのものとして読める
    /// (そのまま `calcDamage` の本文にできる)。`calc` の無い(旧い)お気に入りは `calc` が nil で、従来どおり読める。
    func testFavoriteWithCalcDecodesAsCalcRequest() throws {
        let json = #"""
        [{"id":"42","label":"雨ダブル","individual":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
          "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32},
          "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
          "calc":{"format":"double",
            "attacker":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
              "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32},
              "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
            "defender":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
              "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0},
              "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
            "moveId":"fake-move",
            "field":{"weather":"rain","terrain":"none",
              "attackerScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false},
              "defenderScreens":{"reflect":true,"lightScreen":false,"auroraVeil":false}},
            "options":{"critical":false}},
          "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-02T00:00:00Z"},
         {"id":"7","label":null,"individual":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
          "sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
          "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
          "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T00:00:00Z"}]
        """#
        let list = try decoder().decode([Components.Schemas.Favorite].self, from: Data(json.utf8))
        let calc: Components.Schemas.CalcRequest? = list[0].calc
        XCTAssertEqual(calc?.moveId, "fake-move")
        XCTAssertEqual(calc?.format, .double)
        XCTAssertEqual(calc?.defender.speciesKey, "9003-000")
        XCTAssertEqual(calc?.field?.weather, .rain)
        XCTAssertNil(list[1].calc, "calc の無いお気に入りは nil(旧い行・旧クライアントの作成)")
    }

    /// ADR-0228: 作成の本文に `calc`(`CalcRequest` そのもの)を付けられる。`individual` は従来どおり必須。
    func testFavoriteInputAcceptsCalcRequest() throws {
        let json = #"""
        {"individual":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
          "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32}},
         "calc":{"format":"single",
          "attacker":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
            "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32}},
          "defender":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
            "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0}},
          "moveId":"fake-move"}}
        """#
        let input = try decoder().decode(Components.Schemas.FavoriteInput.self, from: Data(json.utf8))
        let calc: Components.Schemas.CalcRequest? = input.calc
        XCTAssertEqual(calc?.moveId, "fake-move")
        XCTAssertEqual(input.individual.speciesKey, "9002-000")
        // 復元した calc はそのまま計算 API の本文(Operations.CalcDamage の JSON 本文)に入る。
        if let calc {
            let body: Operations.CalcDamage.Input.Body = .json(calc)
            XCTAssertNotNil(body)
        }
        // calc を付けない従来の本文も作れる(memberwise init の calc は省略可)。
        let legacy = Components.Schemas.FavoriteInput(label: nil, individual: input.individual)
        XCTAssertNil(legacy.calc)
    }

    /// 操作は一覧・作成・削除の3つ(更新は持たない。ADR-0227 §1)。生成クライアントに3つがあることを型で確かめる。
    func testClientHasFavoriteOperations() {
        let list: (Client) -> (Operations.ListFavorites.Input) async throws -> Operations.ListFavorites.Output =
            Client.listFavorites
        let create: (Client) -> (Operations.CreateFavorite.Input) async throws -> Operations.CreateFavorite.Output =
            Client.createFavorite
        let delete: (Client) -> (Operations.DeleteFavorite.Input) async throws -> Operations.DeleteFavorite.Output =
            Client.deleteFavorite
        XCTAssertNotNil(list)
        XCTAssertNotNil(create)
        XCTAssertNotNil(delete)
    }
}
