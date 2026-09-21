import Foundation
import XCTest

@testable import PokeCalcCore

/// `MockPokeCalcService`(ADR-0017 §4): サーバーができるまで画面を動かすための架空データ。
/// - 架空であること(名前は「テスト」で始める。ADR-0002)をデータの全列挙で検査する。
/// - **モックはダメージを計算しない**(絶対ルール2・coding-rules §2)。数値の正しさは検査せず、
///   契約(api/openapi.yaml)と ADR-0009 / ADR-0010 §R が決める「形と順序」だけを検査する。
final class MockPokeCalcServiceTests: XCTestCase {

    /// 契約上の検索件数の上限(openapi `limit` の maximum)。モックのデータはこれに収まる小ささにする。
    private let contractMaxLimit = 200

    private var mock: MockPokeCalcService!

    override func setUpWithError() throws {
        mock = try MockPokeCalcService()
    }

    // MARK: - 架空データの検査

    /// リソースの JSON を生のまま全走査し、`nameJa` キーの値がすべて「テスト」で始まることを確かめる。
    /// サービスの API を通さないので、画面に出ない項目(特性など)に実在名が紛れても落ちる。
    func testEveryNameJaInFixtureResourcesStartsWithTest() throws {
        let urls = try MockPokeCalcService.fixtureURLs()
        XCTAssertFalse(urls.isEmpty, "モックのリソースが見つからない")
        var checked = 0
        for url in urls {
            let object = try JSONSerialization.jsonObject(with: Data(contentsOf: url))
            visitNameJa(in: object) { name in
                checked += 1
                XCTAssertTrue(name.hasPrefix("テスト"), "\(url.lastPathComponent): \(name)")
            }
        }
        XCTAssertGreaterThan(checked, 0, "nameJa が1つも無い")
    }

    func testFixtureHasEnoughFictionalMasterData() async throws {
        let species = try await mock.searchSpecies(query: "", limit: contractMaxLimit)
        let moves = try await mock.searchMoves(query: "", limit: contractMaxLimit)
        let items = try await mock.searchItems(query: "", limit: contractMaxLimit)
        let natures = try await mock.natures()

        XCTAssertGreaterThanOrEqual(species.count, 2, "攻撃側と防御側に別の種族を置けること")
        XCTAssertGreaterThanOrEqual(items.count, 1)
        // 一括計算の既定セットは技の分類ごとに違う(ADR-0009)ので、3分類すべての技が要る
        XCTAssertEqual(Set(moves.map(\.category)), [.physical, .special, .status])

        let names = species.map(\.nameJa) + moves.map(\.nameJa) + items.map(\.nameJa) + natures.map(\.nameJa)
        for name in names {
            XCTAssertTrue(name.hasPrefix("テスト"), name)
        }
        for summary in species {
            // openapi `SpeciesKey` の形。図鑑番号は実在の種族を指さない 9000 番台にする
            XCTAssertNotNil(summary.key.range(of: #"^[0-9]{4}-[0-9]{3}$"#, options: .regularExpression), summary.key)
            XCTAssertGreaterThanOrEqual(summary.dexNo, 9000, summary.key)
            XCTAssertTrue((1...2).contains(summary.types.count), summary.key)
        }
    }

    /// ADR-0017 §6(自分側のプリセット)で性格 ID を直書きせずに選べるだけの性格があること。
    /// 無補正(plus == nil)と、ADR-0009 の防御プリセット(+B/-A・+D/-A)・攻撃側の上昇性格(+A/-C・+C/-A)。
    func testFixtureNaturesCoverPresetSelection() async throws {
        let natures = try await mock.natures()
        XCTAssertTrue(natures.contains { $0.plus == nil && $0.minus == nil }, "無補正")
        let required: [(plus: StatKey, minus: StatKey)] = [
            (.atk, .spa), (.spa, .atk), (.def, .atk), (.spd, .atk),
        ]
        for pair in required {
            XCTAssertTrue(natures.contains { $0.plus == pair.plus && $0.minus == pair.minus },
                          "+\(pair.plus.rawValue)/-\(pair.minus.rawValue) の性格が無い")
        }
    }

    /// 種族の詳細が全種族で引け、覚える技はモックの技一覧にあるものだけ。
    func testSpeciesDetailIsConsistentWithSummariesAndMoves() async throws {
        let species = try await mock.searchSpecies(query: "", limit: contractMaxLimit)
        let allMoves = try await mock.searchMoves(query: "", limit: contractMaxLimit)
        let moveIDs = Set(allMoves.map(\.id))
        for summary in species {
            let detail = try await mock.species(key: summary.key)
            XCTAssertEqual(detail.key, summary.key)
            XCTAssertEqual(detail.nameJa, summary.nameJa)
            XCTAssertEqual(detail.types, summary.types)
            for ability in detail.abilities {
                XCTAssertTrue(ability.nameJa.hasPrefix("テスト"), ability.nameJa)
            }
            XCTAssertTrue(Set(detail.learnset).isSubset(of: moveIDs), summary.key)
        }
        let error = await assertThrowsPokeCalcError("unknown species") {
            try await self.mock.species(key: "0000-999")
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.notFound)
    }

    // MARK: - 検索(openapi: 日本語名の前方一致・limit まで)

    func testSearchIsPrefixMatchWithLimit() async throws {
        let all = try await mock.searchSpecies(query: "", limit: contractMaxLimit)
        let target = try XCTUnwrap(all.last)

        let exact = try await mock.searchSpecies(query: target.nameJa, limit: contractMaxLimit)
        XCTAssertTrue(exact.contains { $0.key == target.key })
        XCTAssertTrue(exact.allSatisfy { $0.nameJa.hasPrefix(target.nameJa) })

        let limited = try await mock.searchSpecies(query: "テスト", limit: 1)
        XCTAssertEqual(limited.count, 1, "limit で切る")
        let none = try await mock.searchSpecies(query: "該当しない接頭辞", limit: contractMaxLimit)
        XCTAssertTrue(none.isEmpty)
        // 中間一致ではない(先頭の「テスト」を除いた部分では当たらない)
        let middle = String(target.nameJa.dropFirst("テスト".count))
        if !middle.isEmpty, !middle.hasPrefix("テスト") {
            let middleHits = try await mock.searchSpecies(query: middle, limit: contractMaxLimit)
            XCTAssertFalse(middleHits.contains { $0.key == target.key })
        }

        let moves = try await mock.searchMoves(query: "テスト", limit: 1)
        XCTAssertEqual(moves.count, 1)
        let items = try await mock.searchItems(query: "テスト", limit: 1)
        XCTAssertEqual(items.count, 1)
        let noMoves = try await mock.searchMoves(query: "該当しない接頭辞", limit: contractMaxLimit)
        XCTAssertTrue(noMoves.isEmpty)
        let noItems = try await mock.searchItems(query: "該当しない接頭辞", limit: contractMaxLimit)
        XCTAssertTrue(noItems.isEmpty)
    }

    // MARK: - 一括計算(形と順序)

    /// openapi `BulkCalcRequest.presets` の description / ADR-0009: 省略(空を含む)は技の分類で既定セット。
    func testCalcBulkDefaultPresetsDependOnMoveCategory() async throws {
        let expected: [MoveCategory: [String]] = [
            .physical: ["none", "hp", "hb_boost", "hb", "hb_full"],
            .special: ["none", "hp", "hd_boost", "hd", "hd_full"],
            .status: ["none", "hp"],
        ]
        let context = try await fixtureContext()
        for (category, presetOrder) in expected {
            let move = try XCTUnwrap(context.moves.first { $0.category == category }, "\(category)")
            let result = try await mock.calcBulk(BulkCalcRequest(
                format: .single, attacker: context.attacker,
                defenderSpeciesKey: context.defenderKey, moveId: move.id))
            XCTAssertEqual(result.defenderSpeciesKey, context.defenderKey)
            XCTAssertEqual(result.rows.map(\.preset.rawValue), presetOrder, "\(category)")
            XCTAssertTrue(result.rows.allSatisfy { $0.itemId == nil }, "itemVariants 省略は素の1通り")
            assertRowsWellFormed(result.rows, "\(category)")
        }
    }

    /// 指定したプリセットはその順に返す(openapi)。
    func testCalcBulkKeepsRequestedPresetOrder() async throws {
        let context = try await fixtureContext()
        let result = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: context.attacker, defenderSpeciesKey: context.defenderKey,
            moveId: context.physicalMove.id, presets: try presets(["hb_full", "none", "hp"])))
        XCTAssertEqual(result.rows.map(\.preset.rawValue), ["hb_full", "none", "hp"])
        assertRowsWellFormed(result.rows, "指定順")
    }

    /// 8種類のプリセットすべてに表示名がある(モックの `presetLabel` は網羅 switch。§実装の意図を固定する)。
    func testCalcBulkAllPresetsHaveDistinctLabels() async throws {
        let context = try await fixtureContext()
        let allPresets = try presets(DefenderPreset.allCases.map(\.rawValue))
        let result = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: context.attacker, defenderSpeciesKey: context.defenderKey,
            moveId: context.physicalMove.id, presets: allPresets))
        XCTAssertEqual(result.rows.count, DefenderPreset.allCases.count)
        assertRowsWellFormed(result.rows, "全プリセット")
    }

    /// 持ち物の差し替え: 行はプリセット優先(presets × itemVariants)。plan.md P3-1 の「行の順序はプリセット優先」。
    func testCalcBulkRowsArePresetMajorOverItemVariants() async throws {
        let context = try await fixtureContext()
        let itemID = try XCTUnwrap(context.items.first?.id)
        let result = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: context.attacker, defenderSpeciesKey: context.defenderKey,
            moveId: context.physicalMove.id, presets: try presets(["none", "hp"]), itemVariants: [nil, itemID]))
        XCTAssertEqual(result.rows.map(\.preset.rawValue), ["none", "none", "hp", "hp"])
        XCTAssertEqual(result.rows.map(\.itemId), [nil, itemID, nil, itemID])
        assertRowsWellFormed(result.rows, "持ち物の差し替え")
    }

    func testCalcBulkRejectsUnknownIDs() async throws {
        let context = try await fixtureContext()
        let unknownAttacker = Individual(speciesKey: "0000-999", natureId: context.neutralNatureID, sp: zeroSP)
        let cases: [(name: String, request: BulkCalcRequest)] = [
            ("未知の防御側", BulkCalcRequest(format: .single, attacker: context.attacker,
                                         defenderSpeciesKey: "0000-999", moveId: context.physicalMove.id)),
            ("未知の攻撃側", BulkCalcRequest(format: .single, attacker: unknownAttacker,
                                         defenderSpeciesKey: context.defenderKey, moveId: context.physicalMove.id)),
            ("未知の技", BulkCalcRequest(format: .single, attacker: context.attacker,
                                      defenderSpeciesKey: context.defenderKey, moveId: "test-unknown-move")),
        ]
        for testCase in cases {
            let error = await assertThrowsPokeCalcError(testCase.name) { try await self.mock.calcBulk(testCase.request) }
            XCTAssertEqual(error?.code, PokeCalcError.Code.notFound, testCase.name)
        }
    }

    // MARK: - 1対1

    func testCalcDamageReturnsWellFormedResult() async throws {
        let context = try await fixtureContext()
        let defender = Individual(speciesKey: context.defenderKey, natureId: context.neutralNatureID, sp: zeroSP)
        let result = try await mock.calcDamage(CalcRequest(format: .single, attacker: context.attacker,
                                                           defender: defender, moveId: context.physicalMove.id))
        assertResultWellFormed(result, "calcDamage")

        let error = await assertThrowsPokeCalcError("未知の技") {
            try await self.mock.calcDamage(CalcRequest(format: .single, attacker: context.attacker,
                                                       defender: defender, moveId: "test-unknown-move"))
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.notFound)
    }

    // MARK: - 逆算(ADR-0010 §R の形)

    /// §R1: 候補は (性格クラス neutral/plus) × 持ち物候補。持ち物候補が空なら「持ち物なし」の1通り。
    func testReverseCandidatesAreNatureClassTimesItems() async throws {
        let context = try await fixtureContext()
        let itemID = try XCTUnwrap(context.items.first?.id)
        let cases: [(name: String, items: [String?], expected: Set<String>)] = [
            ("持ち物候補なし", [], ["neutral|-", "plus|-"]),
            ("持ち物あり", [nil, itemID], ["neutral|-", "neutral|\(itemID)", "plus|-", "plus|\(itemID)"]),
        ]
        for testCase in cases {
            let result = try await mock.reverse(ReverseRequest(
                format: .single, side: .defender, known: context.attacker,
                unknownSpeciesKey: context.defenderKey, moveId: context.physicalMove.id,
                itemCandidates: testCase.items, observations: [.percent(40)]))
            let pairs = result.candidates.map { "\($0.natureClass.rawValue)|\($0.itemId ?? "-")" }
            XCTAssertEqual(pairs.count, testCase.expected.count, testCase.name)
            XCTAssertEqual(Set(pairs), testCase.expected, testCase.name)
            assertReverseResultWellFormed(result, testCase.name)
        }
    }

    /// §2: 逆算する関連ステータスは側と技の分類で決まる(変化技は物理と同じ扱い)。
    /// §R1: 仮定した H の SP は defender = 32、attacker = 0。
    func testReverseStatAndAssumedHPSPFollowSideAndCategory() async throws {
        let context = try await fixtureContext()
        let cases: [(side: ReverseSide, category: MoveCategory, stat: StatKey, assumedHPSP: Int)] = [
            (.defender, .physical, .def, 32),
            (.defender, .special, .spd, 32),
            (.defender, .status, .def, 32),
            (.attacker, .physical, .atk, 0),
            (.attacker, .special, .spa, 0),
        ]
        for testCase in cases {
            let move = try XCTUnwrap(context.moves.first { $0.category == testCase.category })
            let result = try await mock.reverse(ReverseRequest(
                format: .single, side: testCase.side, known: context.attacker,
                unknownSpeciesKey: context.defenderKey, moveId: move.id,
                itemCandidates: [], observations: [.percent(40), .percentTenths(401), .damage(50)]))
            let label = "\(testCase.side.rawValue)/\(testCase.category.rawValue)"
            XCTAssertEqual(result.side, testCase.side, label)
            XCTAssertEqual(result.stat, testCase.stat, label)
            XCTAssertEqual(result.assumedHPSP, testCase.assumedHPSP, label)
            assertReverseResultWellFormed(result, label)
        }
    }

    func testReverseRejectsInvalidRequests() async throws {
        let context = try await fixtureContext()
        let cases: [(name: String, request: ReverseRequest, code: String)] = [
            ("未知の種族", ReverseRequest(format: .single, side: .defender, known: context.attacker,
                                     unknownSpeciesKey: "0000-999", moveId: context.physicalMove.id,
                                     itemCandidates: [], observations: [.percent(40)]),
             PokeCalcError.Code.notFound),
            ("未知の技", ReverseRequest(format: .single, side: .defender, known: context.attacker,
                                    unknownSpeciesKey: context.defenderKey, moveId: "test-unknown-move",
                                    itemCandidates: [], observations: [.percent(40)]),
             PokeCalcError.Code.notFound),
            // openapi `ReverseRequest.observations` は minItems 1
            ("観測なし", ReverseRequest(format: .single, side: .defender, known: context.attacker,
                                    unknownSpeciesKey: context.defenderKey, moveId: context.physicalMove.id,
                                    itemCandidates: [], observations: []),
             PokeCalcError.Code.invalidInput),
        ]
        for testCase in cases {
            let error = await assertThrowsPokeCalcError(testCase.name) { try await self.mock.reverse(testCase.request) }
            XCTAssertEqual(error?.code, testCase.code, testCase.name)
        }
    }

    // MARK: - 補助

    private struct FixtureContext {
        let attacker: Individual
        let defenderKey: String
        let neutralNatureID: String
        let moves: [Move]
        let items: [Item]
        var physicalMove: Move { moves.first { $0.category == .physical } ?? moves[0] }
    }

    /// モック自身のデータから要求を組み立てる(ID をテストに直書きしない)。
    private func fixtureContext() async throws -> FixtureContext {
        let species = try await mock.searchSpecies(query: "", limit: contractMaxLimit)
        let natures = try await mock.natures()
        let moves = try await mock.searchMoves(query: "", limit: contractMaxLimit)
        let items = try await mock.searchItems(query: "", limit: contractMaxLimit)
        XCTAssertGreaterThanOrEqual(species.count, 2)
        XCTAssertFalse(moves.isEmpty)
        let attackerKey = try XCTUnwrap(species.first?.key)
        let defenderKey = try XCTUnwrap(species.last?.key)
        let neutral = try XCTUnwrap(natures.first { $0.plus == nil })
        let attacker = Individual(speciesKey: attackerKey, natureId: neutral.id,
                                  sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 32, spd: 0, spe: 0))
        return FixtureContext(attacker: attacker, defenderKey: defenderKey, neutralNatureID: neutral.id,
                              moves: moves, items: items)
    }

    /// openapi `CalcResult`: rolls は 16 件・非減少、minDamage/maxDamage は両端。表示%は min ≤ max。
    /// `ko.displayChancePercent` は 0〜100(openapi `KOChance`)。
    private func assertResultWellFormed(_ result: CalcResult, _ label: String,
                                        file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(result.rolls.count, 16, label, file: file, line: line)
        XCTAssertEqual(result.rolls, result.rolls.sorted(), "\(label) 非減少", file: file, line: line)
        XCTAssertEqual(result.minDamage, result.rolls.first, label, file: file, line: line)
        XCTAssertEqual(result.maxDamage, result.rolls.last, label, file: file, line: line)
        XCTAssertLessThanOrEqual(result.minPercent, result.maxPercent, label, file: file, line: line)
        XCTAssertGreaterThan(result.defenderHP, 0, label, file: file, line: line)
        XCTAssertGreaterThanOrEqual(result.ko.hits, 0, label, file: file, line: line)
        XCTAssertTrue((0.0...100.0).contains(result.ko.displayChancePercent), label, file: file, line: line)
    }

    private func assertRowsWellFormed(_ rows: [BulkCalcRow], _ label: String,
                                      file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertFalse(rows.isEmpty, label, file: file, line: line)
        var labelByPreset: [String: String] = [:]
        for row in rows {
            XCTAssertFalse(row.presetLabel.isEmpty, label, file: file, line: line)
            if let known = labelByPreset[row.preset.rawValue] {
                XCTAssertEqual(known, row.presetLabel, "\(label) 同じプリセットは同じ表示名", file: file, line: line)
            }
            labelByPreset[row.preset.rawValue] = row.presetLabel
            assertResultWellFormed(row.result, "\(label) \(row.preset.rawValue)", file: file, line: line)
        }
        XCTAssertEqual(Set(labelByPreset.values).count, labelByPreset.count,
                       "\(label) 別のプリセットは別の表示名", file: file, line: line)
    }

    /// ADR-0010 §R3・§R4 の形: Ranges は 0..32 の中で昇順・互いに素・隣接しない・空でない。
    /// spCount は Ranges の SP 数、exact ⇔ mismatch == 0、exactCount は exact の数、並びは mismatch 昇順。
    private func assertReverseResultWellFormed(_ result: ReverseResult, _ label: String,
                                               file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertFalse(result.candidates.isEmpty, label, file: file, line: line)
        XCTAssertEqual(result.exactCount, result.candidates.filter(\.exact).count, label, file: file, line: line)
        let mismatches = result.candidates.map(\.mismatch)
        XCTAssertEqual(mismatches, mismatches.sorted(), "\(label) mismatch 昇順(§R4)", file: file, line: line)
        for candidate in result.candidates {
            let name = "\(label) \(candidate.natureClass.rawValue)/\(candidate.itemId ?? "-")"
            XCTAssertFalse(candidate.ranges.isEmpty, name, file: file, line: line)
            var previousMax: Int?
            for range in candidate.ranges {
                XCTAssertTrue(0 <= range.min && range.min <= range.max && range.max <= 32,
                              "\(name) \(range)", file: file, line: line)
                if let previousMax {
                    XCTAssertGreaterThan(range.min, previousMax + 1, "\(name) 昇順・隣接しない", file: file, line: line)
                }
                previousMax = range.max
            }
            let count = candidate.ranges.reduce(0) { $0 + $1.max - $1.min + 1 }
            XCTAssertEqual(candidate.spCount, count, name, file: file, line: line)
            XCTAssertEqual(candidate.exact, candidate.mismatch == 0, name, file: file, line: line)
            XCTAssertGreaterThanOrEqual(candidate.mismatch, 0, name, file: file, line: line)
            XCTAssertGreaterThanOrEqual(candidate.support, 0, name, file: file, line: line)
            XCTAssertLessThanOrEqual(candidate.minPercent, candidate.maxPercent, name, file: file, line: line)
        }
    }

    private func visitNameJa(in object: Any, _ visit: (String) -> Void) {
        if let dictionary = object as? [String: Any] {
            for (key, value) in dictionary {
                if key == "nameJa", let name = value as? String {
                    visit(name)
                } else {
                    visitNameJa(in: value, visit)
                }
            }
        } else if let array = object as? [Any] {
            for element in array {
                visitNameJa(in: element, visit)
            }
        }
    }
}
