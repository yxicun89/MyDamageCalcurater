import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// F-09(ADR-0524): API 写像(`FavoriteInput.calc` / `Favorite.calc`)・モック・一覧の行・ピン留め・文言。
final class FavoriteCalcRestoreTests: XCTestCase {
    private let deviceID = "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11"
    private let sessionID = "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33"

    private func makeService(transport: any ClientTransport) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        return APIPokeCalcService(
            client: Client(serverURL: url, transport: transport),
            identity: ClientIdentity(deviceID: deviceID, sessionID: sessionID))
    }

    private let zero = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)

    private func calc(
        field: FieldState = FieldState(), critical: Bool = false, attackerAbility: String? = nil,
        attackerItem: String? = nil, attackerRanks: RankBlock = RankBlock(), attackerStatus: StatusCondition = .none,
        defenderAbility: String? = nil, defenderRanks: RankBlock = RankBlock()
    ) -> CalcHistoryCalc {
        CalcHistoryCalc(
            format: .single,
            attacker: Individual(
                speciesKey: "9002-000", natureId: "test-nature-atk-up",
                sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32), abilityId: attackerAbility,
                itemId: attackerItem, ranks: attackerRanks, status: attackerStatus),
            defender: Individual(
                speciesKey: "9003-000", natureId: "test-nature-neutral", sp: zero, abilityId: defenderAbility,
                ranks: defenderRanks),
            moveId: "test-move-physical-a", field: field, critical: critical)
    }

    private func body(of calc: CalcHistoryCalc?) async throws -> [String: Any] {
        let transport = RecordingTransport(status: 201, json: Self.favoriteJSON(withCalc: false))
        let attacker = calc?.attacker ?? calc0().attacker
        _ = try await makeService(transport: transport).addFavorite(label: "A→B(C)", individual: attacker, calc: calc)
        return try XCTUnwrap(transport.requests.first).jsonBody()
    }

    private func calc0() -> CalcHistoryCalc { calc() }

    private static func favoriteJSON(withCalc: Bool) -> String {
        let individual = #"""
        {"speciesKey":"9002-000","level":50,"natureId":"test-nature-atk-up",
         "sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":2,"spe":32},
         "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"}
        """#
        let defender = #"""
        {"speciesKey":"9003-000","level":50,"natureId":"test-nature-neutral",
         "sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
         "ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"}
        """#
        let calc = withCalc
            ? #"""
            ,"calc":{"format":"single","attacker":\#(individual),"defender":\#(defender),"moveId":"test-move-physical-a",
              "field":{"weather":"rain","terrain":"none",
                "attackerScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false},
                "defenderScreens":{"reflect":true,"lightScreen":false,"auroraVeil":false}},
              "options":{"critical":true}}
            """#
            : ""
        return #"""
        {"id":"42","label":"A→B(C)","individual":\#(individual)\#(calc),
         "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-02T00:00:00Z"}
        """#
    }

    // MARK: - API: 保存の本文

    func testDefaultConditionsOmitKeys() async throws {
        let json = try await body(of: calc())
        XCTAssertEqual(Set(json.keys), ["label", "individual", "calc"])
        let calc = try XCTUnwrap(json["calc"] as? [String: Any])
        XCTAssertEqual(calc["format"] as? String, "single")
        XCTAssertEqual(calc["moveId"] as? String, "test-move-physical-a")
        XCTAssertNil(calc["field"], "既定の場はキーごと省く")
        XCTAssertNil(calc["options"], "急所でなければ options を省く")
        let attacker = try XCTUnwrap(calc["attacker"] as? [String: Any])
        XCTAssertNil(attacker["abilityId"])
        XCTAssertNil(attacker["itemId"])
        XCTAssertNil(attacker["ranks"], "ランクがすべて 0 なら省く")
        XCTAssertNil(attacker["status"], "状態異常なしは省く")
        XCTAssertNil(attacker["teraType"])
        XCTAssertEqual(attacker["natureId"] as? String, "test-nature-atk-up")
        XCTAssertEqual(attacker["sp"] as? [String: Int], ["hp": 0, "atk": 32, "def": 0, "spa": 0, "spd": 2, "spe": 32])
        let defender = try XCTUnwrap(calc["defender"] as? [String: Any])
        XCTAssertNil(defender["abilityId"], "おまかせは省く")
        XCTAssertNil(defender["ranks"])
        let individual = try XCTUnwrap(json["individual"] as? [String: Any])
        XCTAssertEqual(individual["speciesKey"] as? String, attacker["speciesKey"] as? String, "individual = calc.attacker")
    }

    func testNonDefaultConditionsAreSent() async throws {
        let withConditions = calc(
            field: FieldState(weather: .rain, terrain: .grassy, defenderScreens: Screens(reflect: true)),
            critical: true, attackerAbility: "test-ability-1", attackerItem: "test-item-1",
            attackerRanks: RankBlock(atk: 2), attackerStatus: .burn, defenderAbility: "test-ability-2",
            defenderRanks: RankBlock(def: -1))
        let json = try await body(of: withConditions)
        let calc = try XCTUnwrap(json["calc"] as? [String: Any])
        let field = try XCTUnwrap(calc["field"] as? [String: Any])
        XCTAssertEqual(field["weather"] as? String, "rain")
        XCTAssertEqual(field["terrain"] as? String, "grassy")
        let screens = try XCTUnwrap(field["defenderScreens"] as? [String: Bool])
        XCTAssertEqual(screens["reflect"], true)
        XCTAssertEqual((calc["options"] as? [String: Bool])?["critical"], true)
        let attacker = try XCTUnwrap(calc["attacker"] as? [String: Any])
        XCTAssertEqual(attacker["abilityId"] as? String, "test-ability-1")
        XCTAssertEqual(attacker["itemId"] as? String, "test-item-1")
        XCTAssertEqual(attacker["status"] as? String, "burn")
        XCTAssertEqual((attacker["ranks"] as? [String: Int])?["atk"], 2)
        let defender = try XCTUnwrap(calc["defender"] as? [String: Any])
        XCTAssertEqual(defender["abilityId"] as? String, "test-ability-2")
        XCTAssertEqual((defender["ranks"] as? [String: Int])?["def"], -1)
    }

    func testWithoutCalcBodyIsUnchanged() async throws {
        let transport = RecordingTransport(status: 201, json: Self.favoriteJSON(withCalc: false))
        _ = try await makeService(transport: transport).addFavorite(
            label: nil, individual: calc().attacker)
        let json = try XCTUnwrap(transport.requests.first).jsonBody()
        XCTAssertNil(json["calc"], "calc を付けないときは従来の本文")
    }

    // MARK: - API: 一覧の写像

    func testListMapsCalcWhenPresentAndNilWhenAbsent() async throws {
        let json = "[\(Self.favoriteJSON(withCalc: true)), \(Self.favoriteJSON(withCalc: false))]"
        let list = try await makeService(transport: RecordingTransport(json: json)).favorites()
        let calc = try XCTUnwrap(list[0].calc)
        XCTAssertEqual(calc.moveId, "test-move-physical-a")
        XCTAssertEqual(calc.attacker.speciesKey, "9002-000")
        XCTAssertEqual(calc.defender.speciesKey, "9003-000")
        XCTAssertEqual(calc.field.weather, .rain)
        XCTAssertEqual(calc.field.defenderScreens, Screens(reflect: true))
        XCTAssertTrue(calc.critical)
        XCTAssertNil(list[1].calc, "calc の無い旧お気に入りは nil")
    }

    // MARK: - モック

    func testMockStoresCalcAndDistinguishesByCalc() async throws {
        let service = MockFavoritesService()
        let first = calc()
        let created = try await service.addFavorite(label: "L", individual: first.attacker, calc: first)
        guard case .created(let favorite) = created else { return XCTFail("新規のはず") }
        XCTAssertEqual(favorite.calc, first)
        let again = try await service.addFavorite(label: "L", individual: first.attacker, calc: first)
        guard case .alreadyPinned = again else { return XCTFail("同じ内容は alreadyPinned: \(again)") }
        let other = calc(critical: true)
        let different = try await service.addFavorite(label: "L", individual: other.attacker, calc: other)
        guard case .created = different else { return XCTFail("calc が違えば別のお気に入り(ADR-0228): \(different)") }
        let noCalc = try await service.addFavorite(label: "L", individual: first.attacker)
        guard case .created = noCalc else { return XCTFail("calc の有無でも別: \(noCalc)") }
    }

    func testCalcScenarioHasRestorableRowsAndLegacyRow() async throws {
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "calc"), .calc)
        let list = try await MockFavoritesService(scenario: .calc).favorites()
        XCTAssertEqual(list.map(\.id), ["402", "401", "400"])
        XCTAssertNotNil(list[0].calc)
        XCTAssertEqual(list[1].calc?.moveId, "test-move-gone", "マスタに無い技 = 復元は計算の失敗")
        XCTAssertNil(list[2].calc, "calc の無い旧お気に入り")
        XCTAssertTrue(list.allSatisfy { $0.individual.speciesKey.hasPrefix("900") })
    }

    // MARK: - ピン留め・一覧の行・文言

    @MainActor
    func testPinPassesLabelAndCalcToService() async throws {
        let stub = StubFavoritesService()
        let viewModel = FavoritePinViewModel(service: stub)
        let target = FavoritePinTarget(label: "A→B(C)", individual: calc().attacker, calc: calc())
        await viewModel.pin(target)
        let requests = await stub.addRequests
        XCTAssertEqual(requests.first?.label, "A→B(C)")
        let calcs = await stub.addCalcs
        XCTAssertEqual(calcs, [calc()])
        XCTAssertEqual(viewModel.status, .pinned)
    }

    @MainActor
    func testPinIndividualOnlyKeepsOldBehaviorWithoutCalc() async throws {
        let stub = StubFavoritesService()
        let viewModel = FavoritePinViewModel(service: stub)
        await viewModel.pin(calc().attacker)
        let calcs = await stub.addCalcs
        XCTAssertEqual(calcs.count, 1)
        XCTAssertNil(calcs[0])
        let requests = await stub.addRequests
        XCTAssertNil(requests.first?.label)
    }

    func testRowReportsWhetherItHasCalc() {
        let withCalc = Favorite(
            id: "1", label: "L", individual: calc().attacker, createdAt: Date(), updatedAt: Date(), calc: calc())
        let legacy = StubFavoritesService.favorite("2")
        XCTAssertTrue(FavoriteRow(favorite: withCalc, speciesName: nil).hasCalc)
        XCTAssertFalse(FavoriteRow(favorite: legacy, speciesName: nil).hasCalc)
    }

    func testLabels() {
        XCTAssertEqual(FavoritesLabels.useButton, "計算に使う")
        XCTAssertEqual(FavoritesLabels.useAccessibilityLabel(title: "見出し"), "「見出し」を計算に使う")
        XCTAssertEqual(FavoritesLabels.restoredNotice(title: "見出し"), "「見出し」の計算を開きました")
        XCTAssertEqual(FavoritesLabels.legacyFavoriteHint.isEmpty, false)
        XCTAssertTrue(FavoritesLabels.restoreLimitNote.contains("防御側の性格"), "復元しない範囲を画面に明記する(ADR-0519 と同じ限界)")
        XCTAssertFalse(FavoritesLabels.restoreLimitNote.contains("\n"))
    }
}
