import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// `APIPokeCalcService`(ADR-0500 §3): 生成された `Client` と偽の transport で、
/// ドメイン ↔ HTTP(api/openapi.yaml)の写像を検証する。期待値は openapi.yaml のパス・パラメータ名・スキーマから書く。
/// フィクスチャは架空(名前は「テスト」で始める。ADR-0002)。
///
/// P6-2 契約追従(ADR-0200 / ADR-0202 の P3-1・P3-2)で期待値を更新した:
/// - `Error.code` が `ErrorCode` enum になり、全操作に 503、計算系に 500 が増えた。
/// - `CalcResult.category`・`BulkCalcRow.defender` が必須になった。
/// - 逆算の契約が ADR-0010 §R の形になったので、「HTTP を送らず API 未対応を返す」テストを
///   要求・応答の写像のテストに置き換えた(ADR-0500 §3 の「P3-1 までは API 未対応」の条件が満たされた)。
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
     "effectiveness":2,"stab":true,"category":"physical",
     "ko":{"hits":3,"guaranteed":false,"chancePercent":12.34,"displayChancePercent":12.3},
     "unsupported":[]}
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

    /// 行1: 性格補正あり(+def/-atk)で、マスタに該当する性格が無い(natureId null)。
    /// 行2: 無補正(plus/minus とも null)で、natureId あり。
    private static var bulkJSON: String {
        """
        {"defenderSpeciesKey":"9002-000","rows":[
          {"preset":"hb_full","presetLabel":"テスト表示名1","abilityId":"test-ability","abilityIds":["test-ability"],"itemId":null,
           "defender":{"sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":0,"spe":0},
                       "nature":{"plus":"def","minus":"atk"},"natureId":null,
                       "stats":{"hp":151,"atk":81,"def":122,"spa":70,"spd":85,"spe":90}},
           "result":\(calcResultJSON)},
          {"preset":"none","presetLabel":"テスト表示名2","abilityId":"test-ability","abilityIds":["test-ability"],"itemId":"test-item-a",
           "defender":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
                       "nature":{"plus":null,"minus":null},"natureId":"test-nature-neutral",
                       "stats":{"hp":119,"atk":90,"def":80,"spa":70,"spd":85,"spe":90}},
           "result":\(calcResultJSON)}
        ]}
        """
    }

    /// openapi `ReverseResult`(ADR-0010 §R3)。候補1: neutral・持ち物なし・exact・区間2つ・natureId あり。
    /// 候補2: plus(+spa/-atk)・持ち物あり・exact でない・natureId null。
    private static let reverseJSON = """
    {"side":"attacker","stat":"spa","assumedHpSp":0,"exactCount":1,"candidates":[
      {"natureClass":"neutral","abilityId":"test-ability","abilityIds":["test-ability"],"nature":{"plus":null,"minus":null},"natureId":"test-nature-neutral",
       "itemId":null,"ranges":[{"min":0,"max":3},{"min":6,"max":32}],"spCount":31,
       "exact":true,"mismatch":0,"support":44,"minPercent":38.2,"maxPercent":47.9,"unsupported":[]},
      {"natureClass":"plus","abilityId":"test-ability","abilityIds":["test-ability"],"nature":{"plus":"spa","minus":"atk"},"natureId":null,
       "itemId":"test-item-a","ranges":[{"min":0,"max":0}],"spCount":1,
       "exact":false,"mismatch":7,"support":0,"minPercent":51.5,"maxPercent":61.1,"unsupported":[]}
    ]}
    """

    private var reverseRequest: ReverseRequest {
        ReverseRequest(format: .single, side: .defender, known: attacker,
                       unknownSpeciesKey: "9002-000", moveId: "test-move-physical",
                       itemCandidates: [nil], observations: [.percent(40)])
    }

    // MARK: - (1) 全操作で端末 ID とセッション ID が付く

    /// openapi: 全操作が `X-Device-Id` / `X-Session-Id`(required ヘッダー)を持つ。
    /// 値は `ClientIdentity` のもの。あわせてメソッドとパスも openapi の `paths` と照合する。
    func testEveryOperationSendsIdentityHeadersMethodAndPath() async throws {
        let bulkRequest = BulkCalcRequest(format: .single, attacker: attacker,
                                          defenderSpeciesKey: "9002-000", moveId: "test-move-physical")
        let calcRequest = CalcRequest(format: .single, attacker: attacker, defender: defender,
                                      moveId: "test-move-physical")
        let reverseRequest = reverseRequest
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
            // P3-1 で逆算も HTTP を送るようになった(以前は送らなかった。ADR-0500 §3)
            ("reverse", Self.reverseJSON, "POST", "/api/calc/reverse",
             { _ = try await $0.reverse(reverseRequest) }),
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
        XCTAssertEqual(result.category, .physical)
        XCTAssertEqual(result.ko.hits, 3)
        XCTAssertFalse(result.ko.guaranteed)
        XCTAssertEqual(result.ko.chancePercent, 12.34, accuracy: 1e-9, "engine の生値(画面には出さないが落とさない)")
        XCTAssertEqual(result.ko.displayChancePercent, 12.3, accuracy: 1e-9)
    }

    /// openapi `CalcResult.category`(必須)→ ドメインの `MoveCategory`。3分類すべてを写す。
    /// 一括計算の各行の result も同じ写像を通る。
    func testCalcResultCategoryMapsAllMoveCategories() async throws {
        for category in ["physical", "special", "status"] {
            let json = Self.calcResultJSON.replacingOccurrences(of: #""category":"physical""#,
                                                                with: #""category":"\#(category)""#)
            let service = try makeService(transport: RecordingTransport(json: json))
            let result = try await service.calcDamage(CalcRequest(format: .single, attacker: attacker,
                                                                  defender: defender, moveId: "test-move-physical"))
            XCTAssertEqual(result.category.rawValue, category)

            let bulkJSON = Self.bulkJSON.replacingOccurrences(of: #""category":"physical""#,
                                                              with: #""category":"\#(category)""#)
            let bulkService = try makeService(transport: RecordingTransport(json: bulkJSON))
            let bulk = try await bulkService.calcBulk(BulkCalcRequest(
                format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical"))
            XCTAssertEqual(bulk.rows.map(\.result.category.rawValue), [category, category])
        }
    }

    /// openapi `CalcResult.category` は必須。欠けた応答は契約違反なのでデコード失敗(`decode`)にする。
    func testCalcResultWithoutCategoryIsDecodeError() async throws {
        let json = Self.calcResultJSON.replacingOccurrences(of: #""category":"physical","#, with: "")
        XCTAssertFalse(json.contains("category"), "フィクスチャの置換が効いていない")
        let service = try makeService(transport: RecordingTransport(json: json))
        let error = await assertThrowsPokeCalcError("category 欠落") {
            try await service.calcDamage(CalcRequest(format: .single, attacker: attacker,
                                                     defender: defender, moveId: "test-move-physical"))
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
    }

    /// openapi `BulkCalcRow.defender`(`BulkDefender{sp, nature{plus,minus}, natureId, stats}`)→ ドメイン。
    /// natureId の null・無補正(plus/minus とも null)を落とさない。実数値はサーバーの値をそのまま運ぶ。
    func testCalcBulkResponseMapsDefender() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.bulkJSON))
        let result = try await service.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical"))
        XCTAssertEqual(result.rows.count, 2)
        let boosted = result.rows[0].defender
        XCTAssertEqual(boosted.sp, StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0))
        XCTAssertEqual(boosted.nature, NatureModifier(plus: .def, minus: .atk))
        XCTAssertNil(boosted.natureId, "マスタに該当する性格が無いときは null のまま")
        XCTAssertEqual(boosted.stats, StatBlock(hp: 151, atk: 81, def: 122, spa: 70, spd: 85, spe: 90))

        let neutral = result.rows[1].defender
        XCTAssertEqual(neutral.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
        XCTAssertNil(neutral.nature.plus, "無補正は plus も minus も null")
        XCTAssertNil(neutral.nature.minus)
        XCTAssertEqual(neutral.nature, NatureModifier())
        XCTAssertEqual(neutral.natureId, "test-nature-neutral")
        XCTAssertEqual(neutral.stats, StatBlock(hp: 119, atk: 90, def: 80, spa: 70, spd: 85, spe: 90))
    }

    /// openapi `BulkCalcRow.defender` は必須。欠けた応答はデコード失敗(`decode`)にする。
    func testCalcBulkRowWithoutDefenderIsDecodeError() async throws {
        let json = """
        {"defenderSpeciesKey":"9002-000","rows":[
          {"preset":"none","presetLabel":"テスト表示名","abilityId":"test-ability","abilityIds":["test-ability"],"itemId":null,"result":\(Self.calcResultJSON)}
        ]}
        """
        let service = try makeService(transport: RecordingTransport(json: json))
        let error = await assertThrowsPokeCalcError("defender 欠落") {
            try await service.calcBulk(BulkCalcRequest(
                format: .single, attacker: self.attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-physical"))
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
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

    /// openapi `Error{code: ErrorCode, message}` → `PokeCalcError`。`code` は ErrorCode の rawValue の文字列
    /// (ADR-0500 §3。ドメインは enum に写さない。DomainTypesTests の決定を参照)。
    /// 期待値の変更理由: 契約の `Error.code` が `ErrorCode` enum になった(P3-1)。以前の例
    /// `searchSpecies 503 unavailable` は語彙に無くなったので `upstream_unavailable` に置き換え、
    /// 全操作の 503 と計算系の 500・503 `master_unavailable` を追加した。
    func testErrorResponsesMapToPokeCalcErrorKeepingCode() async throws {
        let calcRequest = CalcRequest(format: .single, attacker: attacker, defender: defender,
                                      moveId: "test-move-physical")
        let bulkRequest = BulkCalcRequest(format: .single, attacker: attacker,
                                          defenderSpeciesKey: "9002-000", moveId: "test-move-physical")
        let reverseRequest = reverseRequest
        let operations: [(name: String, call: @Sendable (APIPokeCalcService) async throws -> Void)] = [
            ("searchSpecies", { _ = try await $0.searchSpecies(query: "", limit: 5) }),
            ("species", { _ = try await $0.species(key: "9999-000") }),
            ("searchMoves", { _ = try await $0.searchMoves(query: "", limit: 5) }),
            ("searchItems", { _ = try await $0.searchItems(query: "", limit: 5) }),
            ("natures", { _ = try await $0.natures() }),
            ("calcDamage", { _ = try await $0.calcDamage(calcRequest) }),
            ("calcBulk", { _ = try await $0.calcBulk(bulkRequest) }),
            ("reverse", { _ = try await $0.reverse(reverseRequest) }),
        ]
        let calcOperations: Set<String> = ["calcDamage", "calcBulk", "reverse"]

        var cases: [(name: String, status: Int, code: String,
                     call: @Sendable (APIPokeCalcService) async throws -> Void)] = []
        // openapi: 全操作に 503(gateway から下流に届かない。ADR-0202)
        for operation in operations {
            cases.append(("\(operation.name) 503", 503, "upstream_unavailable", operation.call))
        }
        // openapi: 計算系(calc / bulk / reverse)に 500 と 503 `master_unavailable`
        for operation in operations where calcOperations.contains(operation.name) {
            cases.append(("\(operation.name) 500", 500, "internal", operation.call))
            cases.append(("\(operation.name) 503 master", 503, "master_unavailable", operation.call))
        }
        let specificCases: [(name: String, status: Int, code: String,
                             call: @Sendable (APIPokeCalcService) async throws -> Void)] = [
            ("calcDamage 400", 400, "invalid_input", { _ = try await $0.calcDamage(calcRequest) }),
            ("calcBulk 400", 400, "unknown_preset", { _ = try await $0.calcBulk(bulkRequest) }),
            ("calcBulk 400 duplicate", 400, "duplicate_preset", { _ = try await $0.calcBulk(bulkRequest) }),
            ("reverse 400 observation", 400, "invalid_observation", { _ = try await $0.reverse(reverseRequest) }),
            ("reverse 400 no observation", 400, "no_observation", { _ = try await $0.reverse(reverseRequest) }),
            ("species 404", 404, "not_found", { _ = try await $0.species(key: "9999-000") }),
            // 個別の定義が無いステータスは default(openapi `responses.Error`)で受ける
            ("natures 500 (default)", 500, "internal", { _ = try await $0.natures() }),
            ("searchSpecies 400 (default)", 400, "invalid_header", { _ = try await $0.searchSpecies(query: "", limit: 5) }),
        ]
        cases += specificCases
        for testCase in cases {
            let json = #"{"code":"\#(testCase.code)","message":"テスト用のエラー"}"#
            let service = try makeService(transport: RecordingTransport(status: testCase.status, json: json))
            let error = await assertThrowsPokeCalcError(testCase.name) { try await testCase.call(service) }
            XCTAssertEqual(error?.code, testCase.code, testCase.name)
            XCTAssertEqual(error?.message, "テスト用のエラー", testCase.name)
        }
    }

    /// 契約の `ErrorCode` に無い code を返すエラー応答は、契約違反の応答としてデコード失敗(`decode`)にする
    /// (生成型の enum でデコードできないため。サーバーの語彙を黙って別のコードに読み替えない)。
    func testErrorResponseWithUnknownCodeIsDecodeError() async throws {
        let json = #"{"code":"test_unknown_code","message":"テスト用のエラー"}"#
        let service = try makeService(transport: RecordingTransport(status: 503, json: json))
        let error = await assertThrowsPokeCalcError("unknown code") {
            try await service.searchSpecies(query: "", limit: 5)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.decode)
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

    // MARK: - (5) 逆算(ADR-0010 §R の契約。P3-1)

    /// openapi `ReverseRequest`(side=defender): 既知側 = 自分の攻撃側。known は `Individual` の形、
    /// unknownSpeciesKey・moveId・options.critical・itemCandidates(null を含めてその順)・
    /// observations(3種類それぞれちょうど1つのキー)・maxCandidates を送る。
    func testReverseRequestBodyMapsDefenderSide() async throws {
        let transport = RecordingTransport(json: Self.reverseJSON)
        let service = try makeService(transport: transport)
        _ = try await service.reverse(ReverseRequest(
            format: .single, side: .defender, known: attacker,
            unknownSpeciesKey: "9002-000", moveId: "test-move-physical",
            itemCandidates: [nil, "test-item-a"],
            observations: [.percent(40), .percentTenths(453), .damage(57)],
            critical: true, maxCandidates: 5))
        let sent = try XCTUnwrap(transport.requests.first)
        XCTAssertEqual(sent.operationID, "calcReverse")
        let body = try sent.jsonBody()

        XCTAssertEqual(body["format"] as? String, "single")
        XCTAssertEqual(body["side"] as? String, "defender")
        XCTAssertEqual(body["unknownSpeciesKey"] as? String, "9002-000")
        XCTAssertEqual(body["moveId"] as? String, "test-move-physical")
        XCTAssertEqual(body["maxCandidates"] as? Int, 5)
        let options = try XCTUnwrap(body["options"] as? [String: Any])
        XCTAssertEqual(options["critical"] as? Bool, true)

        let known = try XCTUnwrap(body["known"] as? [String: Any])
        XCTAssertEqual(known["speciesKey"] as? String, "9001-000")
        XCTAssertEqual(known["natureId"] as? String, "test-nature-atk")
        XCTAssertEqual(known["itemId"] as? String, "test-item-a")
        XCTAssertEqual(known["level"] as? Int, 50)
        XCTAssertEqual(known["sp"] as? [String: Int],
                       ["hp": 2, "atk": 32, "def": 0, "spa": 0, "spd": 0, "spe": 32])

        let items = try XCTUnwrap(body["itemCandidates"] as? [Any])
        XCTAssertEqual(items.count, 2)
        XCTAssertTrue(items[0] is NSNull, "持ち物なしは null")
        XCTAssertEqual(items[1] as? String, "test-item-a")

        let observations = try XCTUnwrap(body["observations"] as? [[String: Any]])
        XCTAssertEqual(observations.count, 3)
        XCTAssertEqual(Set(observations[0].keys), ["percent"], "ちょうど1つのキー")
        XCTAssertEqual(observations[0]["percent"] as? Int, 40)
        XCTAssertEqual(Set(observations[1].keys), ["percentTenths"])
        XCTAssertEqual(observations[1]["percentTenths"] as? Int, 453)
        XCTAssertEqual(Set(observations[2].keys), ["damage"])
        XCTAssertEqual(observations[2]["damage"] as? Int, 57)
    }

    /// openapi `ReverseRequest`(side=attacker): 既知側 = 自分の防御側。known に防御側の個体を送る。
    /// itemCandidates の空・maxCandidates の 0 は既定値と同じ意味なので、省略するか既定値を送る。
    func testReverseRequestBodyMapsAttackerSideAndDefaults() async throws {
        let transport = RecordingTransport(json: Self.reverseJSON)
        let service = try makeService(transport: transport)
        _ = try await service.reverse(ReverseRequest(
            format: .double, side: .attacker, known: defender,
            unknownSpeciesKey: "9001-000", moveId: "test-move-special",
            observations: [.damage(12)]))
        let body = try XCTUnwrap(transport.requests.first).jsonBody()

        XCTAssertEqual(body["format"] as? String, "double")
        XCTAssertEqual(body["side"] as? String, "attacker")
        XCTAssertEqual(body["unknownSpeciesKey"] as? String, "9001-000")
        XCTAssertEqual(body["moveId"] as? String, "test-move-special")
        let known = try XCTUnwrap(body["known"] as? [String: Any])
        XCTAssertEqual(known["speciesKey"] as? String, "9002-000")
        XCTAssertEqual(known["natureId"] as? String, "test-nature-neutral")
        XCTAssertEqual(known["sp"] as? [String: Int],
                       ["hp": 32, "atk": 0, "def": 32, "spa": 0, "spd": 2, "spe": 0])
        XCTAssertNil(known["itemId"] as? String, "持ち物なしは itemId を送らない(または null)")
        let observations = try XCTUnwrap(body["observations"] as? [[String: Any]])
        XCTAssertEqual(observations.count, 1)
        XCTAssertEqual(Set(observations[0].keys), ["damage"])
        XCTAssertEqual(observations[0]["damage"] as? Int, 12)

        // 空の itemCandidates と maxCandidates の 0 は openapi の既定値と同じ意味なので送らない(省略する。ADR-0501 の受け入れ条件 4)
        XCTAssertNil(body["itemCandidates"], "空の itemCandidates は省略する")
        XCTAssertNil(body["maxCandidates"], "maxCandidates の 0 は省略する")
        // 急所の既定は false を明示して送る
        let sentOptions = try XCTUnwrap(body["options"] as? [String: Any])
        XCTAssertEqual(sentOptions["critical"] as? Bool, false)
    }

    /// openapi `ReverseResult` → ドメイン。side・stat・assumedHpSp・exactCount と、候補の順序・
    /// natureClass・nature(無補正の null/null を含む)・natureId(null を含む)・itemId(null を含む)・
    /// ranges・spCount・exact・mismatch・support・表示%を保つ(並べ替えない。順序は §R4 でサーバーが決める)。
    func testReverseResponseMapsToDomain() async throws {
        let service = try makeService(transport: RecordingTransport(json: Self.reverseJSON))
        let result = try await service.reverse(reverseRequest)

        XCTAssertEqual(result.side, .attacker)
        XCTAssertEqual(result.stat, .spa)
        XCTAssertEqual(result.assumedHPSP, 0)
        XCTAssertEqual(result.exactCount, 1)
        XCTAssertEqual(result.candidates.count, 2)

        let first = result.candidates[0]
        XCTAssertEqual(first.natureClass, .neutral)
        XCTAssertEqual(first.nature, NatureModifier())
        XCTAssertEqual(first.natureId, "test-nature-neutral")
        XCTAssertNil(first.itemId)
        XCTAssertEqual(first.ranges, [SPRange(min: 0, max: 3), SPRange(min: 6, max: 32)])
        XCTAssertEqual(first.spCount, 31)
        XCTAssertTrue(first.exact)
        XCTAssertEqual(first.mismatch, 0)
        XCTAssertEqual(first.support, 44)
        XCTAssertEqual(first.minPercent, 38.2, accuracy: 1e-9)
        XCTAssertEqual(first.maxPercent, 47.9, accuracy: 1e-9)

        let second = result.candidates[1]
        XCTAssertEqual(second.natureClass, .plus)
        XCTAssertEqual(second.nature, NatureModifier(plus: .spa, minus: .atk))
        XCTAssertNil(second.natureId, "マスタに該当する性格が無いときは null のまま")
        XCTAssertEqual(second.itemId, "test-item-a")
        XCTAssertEqual(second.ranges, [SPRange(min: 0, max: 0)])
        XCTAssertEqual(second.spCount, 1)
        XCTAssertFalse(second.exact)
        XCTAssertEqual(second.mismatch, 7)
        XCTAssertEqual(second.support, 0)
        XCTAssertEqual(second.minPercent, 51.5, accuracy: 1e-9)
        XCTAssertEqual(second.maxPercent, 61.1, accuracy: 1e-9)
    }

    /// `side=defender` の応答(assumedHpSp=32)も写す。
    func testReverseResponseMapsDefenderSide() async throws {
        let json = Self.reverseJSON
            .replacingOccurrences(of: #""side":"attacker","stat":"spa","assumedHpSp":0"#,
                                  with: #""side":"defender","stat":"def","assumedHpSp":32"#)
        XCTAssertTrue(json.contains(#""assumedHpSp":32"#), "フィクスチャの置換が効いていない")
        let service = try makeService(transport: RecordingTransport(json: json))
        let result = try await service.reverse(reverseRequest)
        XCTAssertEqual(result.side, .defender)
        XCTAssertEqual(result.stat, .def)
        XCTAssertEqual(result.assumedHPSP, 32)
    }
}
