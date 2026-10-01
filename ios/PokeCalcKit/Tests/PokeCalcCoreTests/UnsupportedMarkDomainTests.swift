import Foundation
import OpenAPIRuntime
import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// P6-17(ADR-0501「P6-17」1章・ADR-0123): 未対応の印(`UnsupportedMark`)のドメイン型と、
/// `APIPokeCalcService` の写像(`CalcResult`・`BulkCalcRow.result`・`ReverseCandidate` の `unsupported`)。
///
/// 契約との同期は `DomainTypesTests` と同じく生成型の `allCases` と値集合を突き合わせる(生成型は
/// api/openapi.yaml から作られるので、契約を直接読むのと同じ意味になる)。フィクスチャは架空(ADR-0002)。
final class UnsupportedMarkDomainTests: XCTestCase {

    // MARK: - 契約の enum との同期

    /// `UnsupportedTarget` の rawValue の集合 = openapi `UnsupportedMark.target` の enum。
    func testUnsupportedTargetMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(UnsupportedTarget.allCases.map(\.rawValue)), Self.contractTargets)
        XCTAssertEqual(UnsupportedTarget.allCases.count, 5)
    }

    /// `UnsupportedReason` の rawValue の集合 = openapi `UnsupportedMark.reason` の enum
    /// (ADR-0121 の機構13種 + `zero_power` + `unsupported_effect`)。
    func testUnsupportedReasonMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(UnsupportedReason.allCases.map(\.rawValue)), Self.contractReasons)
        XCTAssertEqual(UnsupportedReason.allCases.count, 15)
    }

    /// 契約(api/openapi.yaml の UnsupportedMark)が説明している既知の値。契約は enum にしない(ADR-0215)ので
    /// 生成型からは取れず、ここに写す(値が増えたらここと各ラベル表を足す)。
    private static let contractTargets: Set<String> =
        ["move", "attacker_item", "attacker_ability", "defender_item", "defender_ability"]
    private static let contractReasons: Set<String> = [
        "alt_defense_stat", "alt_offense_stat", "always_crit", "effectiveness_change", "field_specific",
        "fixed_damage", "ignore_defense_ranks", "move_specific", "multi_hit", "ohko", "priority_change",
        "type_change", "variable_power", "zero_power", "unsupported_effect",
    ]

    /// 知らない target・reason は `.unknown` に写り、デコードは失敗しない(ADR-0215)。
    func testUnknownContractValuesMapToUnknown() async throws {
        let json = Self.calcResultJSON(unsupported: Self.marksJSON([("future_target", "future_reason", "x-id")]))
        let result = try await makeService(json: json).calcDamage(calcRequest)
        XCTAssertEqual(result.unsupported.map(\.target), [.unknown])
        XCTAssertEqual(result.unsupported.map(\.reason), [.unknown])
        XCTAssertEqual(result.unsupported.map(\.id), ["x-id"])
        XCTAssertEqual(UnsupportedMarkLabel.text(for: result.unsupported[0], name: "x-id"), "項目「x-id」(詳細は不明)")
    }

    // MARK: - 既定値(既存の呼び出し側を壊さない)

    /// `unsupported` を渡さずに作った結果・候補は印なし(`[]`)。P6-17 より前のテスト・モックの呼び出しの形のまま。
    func testDomainInitializersDefaultToNoMarks() {
        let result = CalcResult(
            rolls: Array(repeating: 1, count: 16), minDamage: 1, maxDamage: 1, minPercent: 1, maxPercent: 1,
            defenderHP: 100, effectiveness: 1, stab: false, category: .physical,
            ko: KOChance(hits: 0, guaranteed: false, chancePercent: 0, displayChancePercent: 0)
        )
        XCTAssertEqual(result.unsupported, [])
        let candidate = ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: nil,
            ranges: [SPRange(min: 0, max: 0)], spCount: 1, exact: true, mismatch: 0, support: 1,
            minPercent: 1, maxPercent: 1
        )
        XCTAssertEqual(candidate.unsupported, [])
    }

    // MARK: - 応答の写像

    private func makeService(json: String) throws -> APIPokeCalcService {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: RecordingTransport(json: json))
        return APIPokeCalcService(
            client: client,
            identity: ClientIdentity(deviceID: "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11",
                                     sessionID: "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33")
        )
    }

    private let attacker = Individual(speciesKey: "9001-000", natureId: "test-nature-atk", sp: zeroSP)
    private let defender = Individual(speciesKey: "9002-000", natureId: "test-nature-neutral", sp: zeroSP)

    /// `unsupported` の JSON 断片。
    private static func marksJSON(_ marks: [(target: String, reason: String, id: String)]) -> String {
        "[" + marks.map { #"{"target":"\#($0.target)","reason":"\#($0.reason)","id":"\#($0.id)"}"# }
            .joined(separator: ",") + "]"
    }

    private static func calcResultJSON(unsupported: String) -> String {
        """
        {"rolls":[40,40,41,41,42,42,43,43,44,44,45,45,46,46,47,48],
         "minDamage":40,"maxDamage":48,"minPercent":30.3,"maxPercent":36.4,"defenderHP":132,
         "effectiveness":1,"stab":false,"category":"physical",
         "ko":{"hits":3,"guaranteed":false,"chancePercent":12.34,"displayChancePercent":12.3},
         "unsupported":\(unsupported)}
        """
    }

    private static func bulkJSON(firstUnsupported: String, secondUnsupported: String) -> String {
        """
        {"defenderSpeciesKey":"9002-000","rows":[
          {"preset":"none","presetLabel":"テスト表示名1","abilityId":"test-ability","abilityIds":["test-ability"],"itemId":null,
           "defender":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
                       "nature":{"plus":null,"minus":null},"natureId":null,
                       "stats":{"hp":119,"atk":90,"def":80,"spa":70,"spd":85,"spe":90}},
           "result":\(calcResultJSON(unsupported: firstUnsupported))},
          {"preset":"none","presetLabel":"テスト表示名1","abilityId":"test-ability","abilityIds":["test-ability"],"itemId":"test-item-a",
           "defender":{"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},
                       "nature":{"plus":null,"minus":null},"natureId":null,
                       "stats":{"hp":119,"atk":90,"def":80,"spa":70,"spd":85,"spe":90}},
           "result":\(calcResultJSON(unsupported: secondUnsupported))}
        ]}
        """
    }

    private static func reverseJSON(firstUnsupported: String, secondUnsupported: String) -> String {
        """
        {"side":"defender","stat":"def","assumedHpSp":32,"exactCount":2,"candidates":[
          {"natureClass":"neutral","abilityId":"test-ability","abilityIds":["test-ability"],"nature":{"plus":null,"minus":null},"natureId":null,
           "itemId":null,"ranges":[{"min":0,"max":3}],"spCount":4,
           "exact":true,"mismatch":0,"support":4,"minPercent":38.2,"maxPercent":47.9,
           "unsupported":\(firstUnsupported)},
          {"natureClass":"neutral","abilityId":"test-ability","abilityIds":["test-ability"],"nature":{"plus":null,"minus":null},"natureId":null,
           "itemId":"test-item-a","ranges":[{"min":0,"max":3}],"spCount":4,
           "exact":true,"mismatch":0,"support":4,"minPercent":38.2,"maxPercent":47.9,
           "unsupported":\(secondUnsupported)}
        ]}
        """
    }

    private var calcRequest: CalcRequest {
        CalcRequest(format: .single, attacker: attacker, defender: defender, moveId: "test-move-a")
    }

    /// `CalcResult.unsupported` の target・reason・id を落とさず、順序も変えない(並びはサーバーが決める。ADR-0123 §2)。
    func testCalcDamageMapsUnsupportedMarksInOrder() async throws {
        let json = Self.calcResultJSON(unsupported: Self.marksJSON([
            ("move", "multi_hit", "test-move-a"),
            ("move", "zero_power", "test-move-a"),
            ("attacker_item", "unsupported_effect", "test-item-a"),
            ("defender_ability", "unsupported_effect", "test-ability-b"),
        ]))
        let result = try await makeService(json: json).calcDamage(calcRequest)
        XCTAssertEqual(result.unsupported, [
            UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-a"),
            UnsupportedMark(target: .move, reason: .zeroPower, id: "test-move-a"),
            UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: "test-item-a"),
            UnsupportedMark(target: .defenderAbility, reason: .unsupportedEffect, id: "test-ability-b"),
        ])
    }

    /// 印なし(`[]`)は空配列のまま。
    func testCalcDamageMapsEmptyUnsupportedToEmpty() async throws {
        let result = try await makeService(json: Self.calcResultJSON(unsupported: "[]")).calcDamage(calcRequest)
        XCTAssertEqual(result.unsupported, [])
    }

    /// 契約の target 5種・reason 15種のどの値も、同じ rawValue のドメインの値に写る(取り違えが無い)。
    func testEveryContractTargetAndReasonMapsToSameRawValue() async throws {
        let targets = Self.contractTargets.sorted()
        let reasons = Self.contractReasons.sorted()
        var marks: [(target: String, reason: String, id: String)] = []
        for target in targets {
            marks.append((target, "unsupported_effect", "test-id-\(target)"))
        }
        for reason in reasons {
            marks.append(("move", reason, "test-id-\(reason)"))
        }
        let json = Self.calcResultJSON(unsupported: Self.marksJSON(marks))
        let result = try await makeService(json: json).calcDamage(calcRequest)
        XCTAssertEqual(result.unsupported.map(\.target.rawValue), marks.map(\.target))
        XCTAssertEqual(result.unsupported.map(\.reason.rawValue), marks.map(\.reason))
        XCTAssertEqual(result.unsupported.map(\.id), marks.map(\.id))
    }

    /// 一括計算は行ごとの `result.unsupported` を写す(行によって違う印を混ぜない)。
    func testCalcBulkMapsUnsupportedPerRow() async throws {
        let json = Self.bulkJSON(
            firstUnsupported: Self.marksJSON([("move", "multi_hit", "test-move-a")]),
            secondUnsupported: Self.marksJSON([
                ("move", "multi_hit", "test-move-a"),
                ("defender_item", "unsupported_effect", "test-item-a"),
            ])
        )
        let bulk = try await makeService(json: json).calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: "9002-000", moveId: "test-move-a"))
        XCTAssertEqual(bulk.rows.map(\.result.unsupported), [
            [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-a")],
            [UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-a"),
             UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: "test-item-a")],
        ])
    }

    /// 逆算は候補ごとの `unsupported` を写す。
    func testReverseMapsUnsupportedPerCandidate() async throws {
        let json = Self.reverseJSON(
            firstUnsupported: "[]",
            secondUnsupported: Self.marksJSON([("defender_item", "unsupported_effect", "test-item-a")])
        )
        let result = try await makeService(json: json).reverse(ReverseRequest(
            format: .single, side: .defender, known: attacker, unknownSpeciesKey: "9002-000",
            moveId: "test-move-a", itemCandidates: [nil, "test-item-a"], observations: [.percent(40)]))
        XCTAssertEqual(result.candidates.map(\.unsupported), [
            [],
            [UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: "test-item-a")],
        ])
    }

    // MARK: - 契約に無い値(将来の追加)

    /// 契約に無い reason(古いアプリが知らない値)は応答全体のデコード失敗にせず、`.unknown` に写す(ADR-0215)。
    func testUnknownReasonMapsToUnknownNotDecodeError() async throws {
        let json = Self.calcResultJSON(unsupported: Self.marksJSON([("move", "test_future_reason", "test-move-a")]))
        let result = try await makeService(json: json).calcDamage(calcRequest)
        XCTAssertEqual(result.unsupported.map(\.reason), [.unknown])
        XCTAssertEqual(result.unsupported.map(\.target), [.move])
    }

    /// 契約に無い target も同じ(`.unknown`。名前は引けないので ID を出す)。
    func testUnknownTargetMapsToUnknownNotDecodeError() async throws {
        let json = Self.calcResultJSON(unsupported: Self.marksJSON([("test_future_target", "multi_hit", "test-move-a")]))
        let result = try await makeService(json: json).calcDamage(calcRequest)
        XCTAssertEqual(result.unsupported.map(\.target), [.unknown])
        XCTAssertEqual(result.unsupported.map(\.id), ["test-move-a"])
    }
}
