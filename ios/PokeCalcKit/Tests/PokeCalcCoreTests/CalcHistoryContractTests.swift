import Foundation
import PokeCalcAPI
import XCTest

/// ADR-0230: 計算履歴 API(`GET /api/record/calc-history`)の生成型(`make ios-gen`)の形を固定する契約の網。
/// API レーンが契約を先に出し、iOS レーンが履歴の画面(一覧・続きを読む・行から計算を出し直す)を作るときの前提を確かめる。
/// ここが落ちたら(またはコンパイルできなくなったら)、契約の変更に画面側の写像を追従させる。
/// 架空の key だけを使う(実データは使わない。ADR-0002)。
final class CalcHistoryContractTests: XCTestCase {
    private func decoder() -> JSONDecoder {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }

    /// 1ページの応答(`CalcHistoryPage`)を読める。行は `occurredAt`・`calc`(`CalcRequest` そのもの)・`result` の3つ、
    /// `nextCursor` は続きがあれば文字列。
    func testCalcHistoryPageDecodes() throws {
        let json = #"""
        {"items":[
          {"occurredAt":"2026-10-09T01:00:00Z",
           "calc":{"format":"single",
             "attacker":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
               "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32},
               "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
             "defender":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
               "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0},
               "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},
             "moveId":"fake-move",
             "field":{"weather":"none","terrain":"none",
               "attackerScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false},
               "defenderScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false}},
             "options":{"critical":false}},
           "result":{"minPercent":41.2,"maxPercent":48.9}},
          {"occurredAt":"2026-10-08T01:00:00Z",
           "calc":{"format":"double",
             "attacker":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
               "sp":{"hp":0,"atk":0,"def":0,"spa":32,"spd":2,"spe":32}},
             "defender":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
               "sp":{"hp":32,"atk":0,"def":0,"spa":0,"spd":32,"spe":0}},
             "moveId":"fake-move-2"},
           "result":{"minPercent":87.6,"maxPercent":137.0}}],
         "nextCursor":"MTcyODQzNTYwMDAwMDAwMDAwMHxj"}
        """#
        let page = try decoder().decode(Components.Schemas.CalcHistoryPage.self, from: Data(json.utf8))
        XCTAssertEqual(page.items.count, 2)
        XCTAssertEqual(page.nextCursor, "MTcyODQzNTYwMDAwMDAwMDAwMHxj")
        XCTAssertGreaterThan(page.items[0].occurredAt, page.items[1].occurredAt, "新しい順")
        let calc: Components.Schemas.CalcRequest = page.items[0].calc
        XCTAssertEqual(calc.moveId, "fake-move")
        XCTAssertEqual(calc.defender.speciesKey, "9003-000")
        XCTAssertEqual(page.items[1].calc.format, .double)
        let result: Components.Schemas.CalcHistoryResult = page.items[1].result
        XCTAssertEqual(result.minPercent, 87.6)
        XCTAssertEqual(result.maxPercent, 137.0, "100% を超える値も切らずに読む")
    }

    /// 最後のページ・記録の無い端末: `items` が空配列、`nextCursor` が null。
    func testEmptyLastPageDecodes() throws {
        let json = #"{"items":[],"nextCursor":null}"#
        let page = try decoder().decode(Components.Schemas.CalcHistoryPage.self, from: Data(json.utf8))
        XCTAssertTrue(page.items.isEmpty)
        XCTAssertNil(page.nextCursor)
    }

    /// 行の `calc` はそのまま計算 API の本文(`Operations.CalcDamage` の JSON 本文)に入る(お気に入りの `calc` と同じ型)。
    func testHistoryCalcIsCalcDamageBody() throws {
        let json = #"""
        {"occurredAt":"2026-10-09T01:00:00Z",
         "calc":{"format":"single",
           "attacker":{"speciesKey":"9002-000","level":50,"natureId":"fake-nature",
             "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32}},
           "defender":{"speciesKey":"9003-000","level":50,"natureId":"fake-nature",
             "sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":2,"spe":0}},
           "moveId":"fake-move"},
         "result":{"minPercent":10.0,"maxPercent":12.5}}
        """#
        let entry = try decoder().decode(Components.Schemas.CalcHistoryEntry.self, from: Data(json.utf8))
        let body: Operations.CalcDamage.Input.Body = .json(entry.calc)
        XCTAssertNotNil(body)
        let favoriteCalc: Components.Schemas.CalcRequest? = Components.Schemas.Favorite(
            id: "1", label: nil, individual: entry.calc.attacker, calc: entry.calc,
            createdAt: entry.occurredAt, updatedAt: entry.occurredAt
        ).calc
        XCTAssertEqual(favoriteCalc?.moveId, entry.calc.moveId)
    }

    /// 操作は一覧の1つだけで、クエリは `limit`(任意)と `cursor`(任意。前のページの `nextCursor`)。
    func testClientHasListCalcHistory() {
        let list: (Client) -> (Operations.ListCalcHistory.Input) async throws -> Operations.ListCalcHistory.Output =
            Client.listCalcHistory
        XCTAssertNotNil(list)
        let first = Operations.ListCalcHistory.Input.Query(limit: 20)
        XCTAssertNil(first.cursor, "cursor を省略すると先頭(最新)から")
        let next = Operations.ListCalcHistory.Input.Query(limit: 20, cursor: "MTcyODQzNTYwMDAwMDAwMDAwMHxj")
        XCTAssertEqual(next.cursor, "MTcyODQzNTYwMDAwMDAwMDAwMHxj")
    }
}
