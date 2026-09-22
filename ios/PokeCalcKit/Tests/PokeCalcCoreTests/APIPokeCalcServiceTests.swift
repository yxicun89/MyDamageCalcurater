import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService`(ADR-0500 §3): 生成された `Client` と偽の transport で、
/// ドメイン ↔ HTTP(api/openapi.yaml)の写像を検証する。期待値は openapi.yaml のパス・パラメータ名・スキーマから書く。
/// フィクスチャは架空(名前は「テスト」で始める。ADR-0002)。
final class APIPokeCalcServiceTests: XCTestCase {

    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: transport)
        return APIPokeCalcService(client: client, identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    // MARK: - 架空のフィクスチャ

    private let attacker = Individual(
        speciesKey: "9001-000",
        natureId: "test-nature-atk",
        sp: StatBlock(hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32),
        itemId: "test-item-a"
    )
    private let defender = Individual(
        speciesKey: "9002-000",
        natureId: "test-nature-neutral",
        sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0)
    )

    /// 16 ロールは非減少。min/max は両端(openapi `CalcResult.rolls`)。
    private static let calcResultJSON = """
    {"rolls":[40,40,41,41,42,42,43,43,44,44,45,45,46,46,47,48],
     "minDamage":40,"maxDamage":48,"minPercent":30.3,"maxPercent":36.4,"defenderHP":132,
     "effectiveness":2,"stab":true,
     "ko":{"hits":3,"guaranteed":false,"chancePercent":12.34,"displayChancePercent":12.3}}
    """

    private static let speciesSummaryJSON = """
    [{"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストモンA","types":["fire","flying"]}]
    """

    private static let speciesDetailJSON = """
    {"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストモンA","types":["fire"],
     "baseStats":{"hp":81,"atk":92,"def":73,"spa":64,"spd":55,"spe":106},
     "abilities":[{"id":"test-ability","nameJa":"テストとくせい"}],
     "learnset":["test-move-physical"]}
    """

    private static let movesJSON = """
    [{"id":"test-move-physical","nameJa":"テストわざ","type":"normal","category":"physical","power":80}]
    """

    private static let itemsJSON = """
    [{"id":"test-item-a","nameJa":"テストどうぐ"}]
    """

    private static let naturesJSON = """
    [{"id":"test-nature-neutral","nameJa":"テストせいかくN","plus":null,"minus":null},
     {"id":"test-nature-atk","nameJa":"テストせいかくA","plus":"atk","minus":"spa"}]
    """

    private static var bulkJSON: String {
        """
        {"defenderSpeciesKey":"9002-000","rows":[
          {"preset":"hb_full","presetLabel":"テスト表示名1","itemId":null,"result":\(calcResultJSON)},
          {"preset":"none","presetLabel":"テスト表示名2","itemId":"test-item-a","result":\(calcResultJSON)}
        ]}
        """
    }

    // MARK: - (1) 全操作で端末 ID とセッション ID が付く

    /// openapi: 全操作が `X-Device-Id` / `X-Session-Id`(required ヘッダー)を持つ。
    /// 値は `ClientIdentity` のもの。あわせてメソッドとパスも openapi の `paths` と照合する。
    func testEveryOperationSendsIdentityHeadersMethodAndPath() async throws {
        let bulkRequest = BulkCalcRequest(format: .single, attacker: attacker,
                                          defenderSpeciesKey: "9002-000", moveId: "test-move-physical")
        let calcRequest = CalcRequest(format: .single, attacker: attacker, defender: defender,
                                      moveId: "test-move-physical")
        let cases: [(name: String, json: String, method: String, path: String,
                     call: @Sendable (APIPokeCalcService) async throws -> Void)] = [
            ("searchSpecies", Self.speciesSummaryJSON, "GET", "/api/pokedex/species",
             { _ = try await $0.searchSpecies(query: "テスト", limit: 5) }),
            ("species", Self.speciesDetailJSON, "GET", "/api/pokedex/species/9001-000",
             { _ = try await $0.species(key: "9001-000") }),
            ("searchMoves", Self.movesJSON, "GET", "/api/pokedex/moves",
             { _ = try await $0.searchMoves(query: "テスト", limit: 5) }),
            ("searchItems", Self.itemsJSON, "GET", "/api/pokedex/items",
             { _ = try await $0.searchItems(query: "テスト", limit: 5) }),
            ("natures", Self.naturesJSON, "GET", "/api/pokedex/natures",
             { _ = try await $0.natures() }),
            ("calcDamage", Self.calcResultJSON, "POST", "/api/calc",
             { _ = try await $0.calcDamage(calcRequest) }),
            ("calcBulk", Self.bulkJSON, "POST", "/api/calc/bulk",
             { _ = try await $0.calcBulk(bulkRequest) }),
        ]
        for testCase in cases {
            let transport = RecordingTransport(json: testCase.json)
            let service = try makeService(transport: transport)
            try await testCase.call(service)
            let sent = try XCTUnwrap(transport.requests.first, testCase.name)
            XCTAssertEqual(transport.requests.count, 1, testCase.name)
            XCTAssertEqual(sent.header("X-Device-Id"), deviceID, testCase.name)
            XCTAssertEqual(sent.header("X-Session-Id"), sessionID, testCase.name)
            XCTAssertEqual(sent.request.method.rawValue, testCase.method, testCase.name)
            XCTAssertEqual(sent.pathWithoutQuery, testCase.path, testCase.name)
        }
    }

    // MARK: - (2) 要求の写像

    /// openapi: 検索は `q`(前方一致)と `limit` をクエリで渡す。
    func testSearchQueriesMapToQAndLimit() async throws {
        let cases: [(name: String, json: String, call: @Sendable (APIPokeCalcService) async throws -> Void)] = [
            ("species", Self.speciesSummaryJSON, { _ = try await $0.searchSpecies(query: "テスト", limit: 7) }),
            ("moves", Self.movesJSON, { _ = try await $0.searchMoves(query: "テスト", limit: 7) }),
            ("items", Self.itemsJSON, { _ = try await $0.searchItems(query: "テスト", limit: 7) }),
        ]
        for testCase in cases {
            let transport = RecordingTransport(json: testCase.json)
            try await testCase.call(try makeService(transport: transport))
            let sent = try XCTUnwrap(transport.requests.first, testCase.name)
            XCTAssertEqual(sent.queryItems["q"], "テスト", testCase.name)
            XCTAssertEqual(sent.queryItems["limit"], "7", testCase.name)
        }
    }

    /// openapi `CalcRequest`: format / attacker / defender / moveId / options.critical。
    /// `Individual` は speciesKey・natureId・sp(StatBlock の6キー)・itemId を運ぶ。
    func testCalcDamageRequestBodyMapsFromDomain() async throws {
        let transport = RecordingTransport(json: Self.calcResultJSON)
        let service = try makeService(transport: transport)
        _ = try await service.calcDamage(CalcRequest(format: .single, attacker: attacker, defender: defender,
                                                     moveId: "test-move-physical", critical: true))
        let body = try XCTUnwrap(transport.requests.first).jsonBody()

        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertEqual(body["moveId"] as? String, "test-move-physical")
        let attackerJSON = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(attackerJSON["speciesKey"] as? String, "9001-000")
        XCTAssertEqual(attackerJSON["natureId"] as? String, "test-nature-atk")
        XCTAssertEqual(attackerJSON["itemId"] as? String, "test-item-a")
        XCTAssertEqual(attackerJSON["sp"] as? [String: Int],
                       ["hp": 2, "atk": 32, "def": 0, "spa": 0, "spd": 0, "spe": 32])
        let defenderJSON = try XCTUnwrap(body["defender"] as? [String: Any])
        XCTAssertEqual(defenderJSON["speciesKey"] as? String, "9002-000")
        XCTAssertEqual(defenderJSON["sp"] as? [String: Int],
                       ["hp": 32, "atk": 0, "def": 32, "spa": 0, "spd": 2, "spe": 0])
        XCTAssertNil(defenderJSON["itemId"] as? String, "持ち物なしは itemId を送らない(または null)")
        let options = try XCTUnwrap(body["options"] as? [String: Any])
        XCTAssertEqual(options["critical"] as? Bool, true)
    }

    /// openapi `Individual` の残りのフィールド(level・abilityId・moveId・ranks・teraType・status)も
    /// 正しく送る。攻撃側に値ありを、防御側(クラスの `defender` フィクスチャ)に既定値を置いて
    /// 「値がある」「省略・既定のまま」の両方を1テストで確かめる。
    func testCalcDamageRequestBodyMapsRanksTeraTypeStatusAbilityMoveLevel() async throws {
        let transport = RecordingTransport(json: Self.calcResultJSON)
        let service = try makeService(transport: transport)
        let attackerWithOptions = Individual(
            speciesKey: "9001-000", natureId: "test-nature-atk",
            sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0),
            level: 50, abilityId: "test-ability-alpha", itemId: "test-item-a", moveId: "test-move-physical",
            ranks: RankBlock(atk: 6, def: -6, spa: 2, spd: -2, spe: 1),
            teraType: .fairy, status: .badlyPoison
        )
        _ = try await service.calcDamage(CalcRequest(
            format: .single, attacker: attackerWithOptions, defender: defender, moveId: "test-move-physical"))
        let body = try XCTUnwrap(transport.requests.first).jsonBody()

        let attackerJSON = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(attackerJSON["level"] as? Int, 50)
        XCTAssertEqual(attackerJSON["abilityId"] as? String, "test-ability-alpha")
        XCTAssertEqual(attackerJSON["moveId"] as? String, "test-move-physical")
        XCTAssertEqual(attackerJSON["teraType"] as? String, "fairy")
        XCTAssertEqual(attackerJSON["status"] as? String, "badly_poison")
        let ranks = try XCTUnwrap(attackerJSON["ranks"] as? [String: Int])
        XCTAssertEqual(ranks, ["atk": 6, "def": -6, "spa": 2, "spd": -2, "spe": 1])

        // `defender` フィクスチャは teraType・abilityId・moveId を渡していない(既定値のまま)。
        let defenderJSON = try XCTUnwrap(body["defender"] as? [String: Any])
        XCTAssertNil(defenderJSON["teraType"], "テラスタルなしは送らない(または null)")
        XCTAssertNil(defenderJSON["abilityId"], "特性未指定は送らない(または null)")
        XCTAssertNil(defenderJSON["moveId"], "moveId 未指定は送らない(または null)")
        XCTAssertEqual(defenderJSON["status"] as? String, "none", "状態異常の既定は none")
        let defenderRanks = try XCTUnwrap(defenderJSON["ranks"] as? [String: Int])
        XCTAssertEqual(defenderRanks, ["atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0], "ランク既定は全 0")
    }

    /// openapi `BulkCalcRequest`: presets はその順に、itemVariants は null(持ち物なし)を含めてその順に送る。
    func testCalcBulkRequestBodyMapsPresetsAndItemVariants() async throws {
        let transport = RecordingTransport(json: Self.bulkJSON)
        let service = try makeService(transport: transport)
        _ = try await service.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical",
            presets: try presets(["hb_full", "none"]), itemVariants: [nil, "test-item-a"]))
        let body = try XCTUnwrap(transport.requests.first).jsonBody()

        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertEqual(body["defenderSpeciesKey"] as? String, "9002-000")
        XCTAssertEqual(body["moveId"] as? String, "test-move-physical")
        XCTAssertEqual(body["presets"] as? [String], ["hb_full", "none"])
        let variants = try XCTUnwrap(body["itemVariants"] as? [Any])
        XCTAssertEqual(variants.count, 2)
        XCTAssertTrue(variants[0] is NSNull, "持ち物なしは null")
        XCTAssertEqual(variants[1] as? String, "test-item-a")
        let attackerJSON = try XCTUnwrap(body["attacker"] as? [String: Any])
        XCTAssertEqual(attackerJSON["sp"] as? [String: Int],
                       ["hp": 2, "atk": 32, "def": 0, "spa": 0, "spd": 0, "spe": 32])
    }

    /// openapi: presets の省略と空配列は同じ(既定セット)。itemVariants の省略は「素の1通り」。
    /// 空のときは送らないか空配列を送る(どちらも契約上同じ意味)。
    func testCalcBulkWithoutPresetsOrVariantsSendsNothingMeaningful() async throws {
        let transport = RecordingTransport(json: Self.bulkJSON)
        let service = try makeService(transport: transport)
        _ = try await service.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical"))
        let body = try XCTUnwrap(transport.requests.first).jsonBody()
        if let sentPresets = body["presets"] {
            XCTAssertEqual((sentPresets as? [Any])?.count, 0)
        }
        if let sentVariants = body["itemVariants"] {
            XCTAssertEqual((sentVariants as? [Any])?.count, 0)
        }
    }

    // MARK: - (3) 応答の写像

    /// openapi `CalcResult` → ドメイン。表示に使う minPercent/maxPercent/ko.displayChancePercent を落とさない。
    func testCalcDamageResponseMapsToDomain() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.calcResultJSON))
        let result = try await service.calcDamage(CalcRequest(format: .single, attacker: attacker,
                                                              defender: defender, moveId: "test-move-physical"))
        XCTAssertEqual(result.rolls, [40, 40, 41, 41, 42, 42, 43, 43, 44, 44, 45, 45, 46, 46, 47, 48])
        XCTAssertEqual(result.minDamage, 40)
        XCTAssertEqual(result.maxDamage, 48)
        XCTAssertEqual(result.minPercent, 30.3, accuracy: 1e-9)
        XCTAssertEqual(result.maxPercent, 36.4, accuracy: 1e-9)
        XCTAssertEqual(result.defenderHP, 132)
        XCTAssertEqual(result.effectiveness, 2, accuracy: 1e-9)
        XCTAssertTrue(result.stab)
        XCTAssertEqual(result.ko.hits, 3)
        XCTAssertFalse(result.ko.guaranteed)
        XCTAssertEqual(result.ko.chancePercent, 12.34, accuracy: 1e-9, "engine の生値(画面には出さないが落とさない)")
        XCTAssertEqual(result.ko.displayChancePercent, 12.3, accuracy: 1e-9)
    }

    /// openapi `BulkCalcResult` → ドメイン。行の順序・preset・presetLabel・itemId(null 含む)を保つ。
    func testCalcBulkResponseMapsRowsInOrder() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.bulkJSON))
        let result = try await service.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical"))
        XCTAssertEqual(result.defenderSpeciesKey, "9002-000")
        XCTAssertEqual(result.rows.map(\.preset.rawValue), ["hb_full", "none"])
        XCTAssertEqual(result.rows.map(\.presetLabel), ["テスト表示名1", "テスト表示名2"])
        XCTAssertEqual(result.rows.map(\.itemId), [nil, "test-item-a"])
        XCTAssertEqual(result.rows.first?.result.maxDamage, 48)
        XCTAssertEqual(result.rows.first?.result.ko.displayChancePercent ?? -1, 12.3, accuracy: 1e-9)
    }

    func testSearchSpeciesResponseMapsToDomain() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.speciesSummaryJSON))
        let species = try await service.searchSpecies(query: "テスト", limit: 5)
        XCTAssertEqual(species.count, 1)
        let first = try XCTUnwrap(species.first)
        XCTAssertEqual(first.key, "9001-000")
        XCTAssertEqual(first.dexNo, 9001)
        XCTAssertEqual(first.form, 0)
        XCTAssertEqual(first.nameJa, "テストモンA")
        XCTAssertEqual(first.types, [.fire, .flying])
    }

    func testSpeciesDetailResponseMapsToDomain() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.speciesDetailJSON))
        let detail = try await service.species(key: "9001-000")
        XCTAssertEqual(detail.key, "9001-000")
        XCTAssertEqual(detail.nameJa, "テストモンA")
        XCTAssertEqual(detail.types, [.fire])
        XCTAssertEqual(detail.baseStats, StatBlock(hp: 81, atk: 92, def: 73, spa: 64, spd: 55, spe: 106))
        XCTAssertEqual(detail.abilities.map(\.id), ["test-ability"])
        XCTAssertEqual(detail.abilities.map(\.nameJa), ["テストとくせい"])
        XCTAssertEqual(detail.learnset, ["test-move-physical"])
    }

    /// openapi `Move.priority` は既定 0(省略時)。
    func testMovesResponseMapsToDomainWithDefaultPriority() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.movesJSON))
        let moves = try await service.searchMoves(query: "テスト", limit: 5)
        let move = try XCTUnwrap(moves.first)
        XCTAssertEqual(move.id, "test-move-physical")
        XCTAssertEqual(move.nameJa, "テストわざ")
        XCTAssertEqual(move.type, .normal)
        XCTAssertEqual(move.category, .physical)
        XCTAssertEqual(move.power, 80)
        XCTAssertEqual(move.priority, 0)
    }

    func testItemsResponseMapsToDomain() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.itemsJSON))
        let items = try await service.searchItems(query: "テスト", limit: 5)
        XCTAssertEqual(items.map(\.id), ["test-item-a"])
        XCTAssertEqual(items.map(\.nameJa), ["テストどうぐ"])
    }

    /// openapi `Nature.plus/minus` は nullable。null は無補正(ADR-0500 §6 の「plus == nil の最初のもの」に使う)。
    func testNaturesResponseMapsNullableStats() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.naturesJSON))
        let natures = try await service.natures()
        XCTAssertEqual(natures.map(\.id), ["test-nature-neutral", "test-nature-atk"])
        XCTAssertNil(natures[0].plus)
        XCTAssertNil(natures[0].minus)
        XCTAssertEqual(natures[1].plus, .atk)
        XCTAssertEqual(natures[1].minus, .spa)
    }

    // MARK: - (4) エラーの写像

    /// openapi `responses.Error`(400 / 404 / default)の body `{code, message}` → `PokeCalcError`。
    /// code はサーバーの値をそのまま運ぶ(ADR-0500 §3)。
    func testErrorResponsesMapToPokeCalcErrorKeepingCode() async throws {
        let calcRequest = CalcRequest(format: .single, attacker: attacker, defender: defender,
                                      moveId: "test-move-physical")
        let bulkRequest = BulkCalcRequest(format: .single, attacker: attacker,
                                          defenderSpeciesKey: "9002-000", moveId: "test-move-physical")
        let cases: [(name: String, status: Int, code: String,
                     call: @Sendable (APIPokeCalcService) async throws -> Void)] = [
            ("calcDamage 400", 400, "invalid_input", { _ = try await $0.calcDamage(calcRequest) }),
            ("calcDamage 500", 500, "internal", { _ = try await $0.calcDamage(calcRequest) }),
            ("calcBulk 400", 400, "unknown_preset", { _ = try await $0.calcBulk(bulkRequest) }),
            ("species 404", 404, "not_found", { _ = try await $0.species(key: "9999-000") }),
            ("searchSpecies 503", 503, "unavailable", { _ = try await $0.searchSpecies(query: "", limit: 5) }),
            ("natures 500", 500, "internal", { _ = try await $0.natures() }),
        ]
        for testCase in cases {
            let json = #"{"code":"\#(testCase.code)","message":"テスト用のエラー"}"#
            let service = try makeService(transport: RecordingTransport(status: testCase.status, json: json))
            let error = await assertThrowsPokeCalcError(testCase.name) { try await testCase.call(service) }
            XCTAssertEqual(error?.code, testCase.code, testCase.name)
            XCTAssertEqual(error?.message, "テスト用のエラー", testCase.name)
        }
    }

    /// 接続できないなど HTTP 応答が無い失敗も `PokeCalcError` にする(画面は1種類のエラーだけを扱う)。
    func testTransportFailureMapsToTransportError() async throws {
        let service = try makeService(transport: FailingTransport())
        let error = await assertThrowsPokeCalcError("transport") {
            try await service.searchSpecies(query: "テスト", limit: 5)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    /// タスクのキャンセルはサービスの失敗ではないので `PokeCalcError` に包まない
    /// (画面が「1種類のエラー」として誤って表示しないように、そのまま `CancellationError` を伝える)。
    func testCancellationIsNotWrappedAsPokeCalcError() async throws {
        struct CancellingTransport: ClientTransport {
            func send(
                _ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String
            ) async throws -> (HTTPResponse, HTTPBody?) {
                throw CancellationError()
            }
        }
        let service = try makeService(transport: CancellingTransport())
        do {
            _ = try await service.searchSpecies(query: "テスト", limit: 5)
            XCTFail("キャンセルなのに成功した")
        } catch is CancellationError {
            // 期待どおり: PokeCalcError ではなく CancellationError がそのまま伝わる。
        } catch {
            XCTFail("CancellationError ではない: \(type(of: error))")
        }
    }

    /// URLSession 経由のキャンセルは URLError(.cancelled) として届く。これも transport の失敗ではなく
    /// キャンセルとして CancellationError で伝える。
    func testURLSessionCancellationIsNotWrappedAsPokeCalcError() async throws {
        struct URLCancellingTransport: ClientTransport {
            func send(
                _ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String
            ) async throws -> (HTTPResponse, HTTPBody?) {
                throw URLError(.cancelled)
            }
        }
        let service = try makeService(transport: URLCancellingTransport())
        do {
            _ = try await service.searchSpecies(query: "テスト", limit: 5)
            XCTFail("キャンセルなのに成功した")
        } catch is CancellationError {
            // 期待どおり
        } catch {
            XCTFail("CancellationError ではない: \(type(of: error))")
        }
    }

    /// 応答は受け取れたが期待した JSON 形式にデコードできない失敗は、接続できない(`transport`)とは
    /// 別のコードにする(原因が違うため)。
    func testDecodingFailureMapsToDecodeError() async throws {
        let service = try makeService(transport: RecordingTransport(json: "not json"))
        let error = await assertThrowsPokeCalcError("decode") {
            try await service.searchSpecies(query: "テスト", limit: 5)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    // MARK: - (5) 逆算は API 未対応

    /// ADR-0500 §3: いまの契約の `ReverseCandidate` は P3-1 で廃止が決まっているので写像しない。
    /// 契約が更新されるまで、HTTP を送らずに「API 未対応」のエラーを返す。
    func testReverseThrowsAPIUnsupportedWithoutSendingHTTP() async throws {
        let transport = RecordingTransport(json: "{}")
        let service = try makeService(transport: transport)
        let request = ReverseRequest(format: .single, side: .defender, known: attacker,
                                     unknownSpeciesKey: "9002-000", moveId: "test-move-physical",
                                     itemCandidates: [nil], observations: [.percent(40)])
        let error = await assertThrowsPokeCalcError("reverse") { try await service.reverse(request) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.apiUnsupported)
        XCTAssertEqual(transport.requests.count, 0, "HTTP を送らない")
    }
}
