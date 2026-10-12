import XCTest

@testable import PokeCalcCore

/// `MockBalanceService`(ADR-0532): 構築の中の「タイプバランス」を XCUITest で確かめるための架空の応答。
///
/// モック構成の既定は従来どおり `UnavailableBalanceService`(架空の相性表を利用者に見せない。ADR-0415 §4)。
/// 環境変数 `POKECALC_MOCK_BALANCE=fixture` を渡したとき**だけ**この架空の応答に替える(UI テスト専用)。
/// 固定する事実(UI テストが依存する):
/// - analyze はメンバーの数・順のまま 1 体ずつ返し、ほのお=×2 弱点・みず=×1/2 耐性・じめん=×0 無効・ほかは等倍。
/// - recommendations の 1 つ目の候補は 9 件以上のポケモン(「もっと見る」が出る)。先頭はモックのマスタ(`species.json`)にいる種族。
/// - 名前はすべて「テスト」で始める(ADR-0002)。
final class MockBalanceServiceTests: XCTestCase {
    private let party = [
        BalanceMemberInput(pokemonId: "9001-000", abilityId: nil, moveIds: []),
        BalanceMemberInput(pokemonId: "9002-000", abilityId: nil, moveIds: ["test-move-special-b"]),
    ]

    func testDefaultMockIsUnavailable() async {
        let service = MockBalanceService.make(environment: [:])
        do {
            _ = try await service.analyze(members: party)
            XCTFail("既定のモックは架空データを返さない")
        } catch let error as PokeCalcError {
            XCTAssertEqual(error.code, UnavailableBalanceService.unavailableCode)
        } catch {
            XCTFail("想定外のエラー: \(error)")
        }
    }

    func testUnknownValueIsAlsoUnavailable() async {
        let service = MockBalanceService.make(environment: [MockBalanceService.environmentKey: "something"])
        let result = try? await service.analyze(members: party)
        XCTAssertNil(result)
    }

    func testFixtureAnalyzeFollowsMembersWithFixedCategories() async throws {
        let service = MockBalanceService.make(environment: [MockBalanceService.environmentKey: MockBalanceService.fixtureValue])
        let result = try await service.analyze(members: party)

        XCTAssertEqual(result.members.map(\.pokemonId), ["9001-000", "9002-000"])
        let defense = try XCTUnwrap(result.members.first?.defense)
        XCTAssertEqual(defense.map(\.attackType), PokeType.allCases, "18 タイプを正準順")
        XCTAssertEqual(defense.first { $0.attackType == .fire }?.category, .weak)
        XCTAssertEqual(defense.first { $0.attackType == .water }?.category, .resist)
        XCTAssertEqual(defense.first { $0.attackType == .ground }?.category, .immune)
        XCTAssertEqual(defense.first { $0.attackType == .normal }?.category, .neutral)
        let fireSummary = result.teamSummary.first { $0.attackType == .fire }
        XCTAssertEqual(fireSummary?.weak, 2)
    }

    func testFixtureCoverageHasNoAttackWithoutMoves() async throws {
        let service = MockBalanceService()
        let result = try await service.coverage(members: party)
        XCTAssertEqual(result.members.count, 2)
        XCTAssertTrue(result.members[0].attackTypes.isEmpty)
        XCTAssertNil(result.members[0].coverage.first?.bestMultiplier, "技 0 個は「攻撃技なし」")
        XCTAssertFalse(result.members[1].attackTypes.isEmpty)
    }

    func testFixtureRecommendationsHaveManyPickablePokemon() async throws {
        let service = MockBalanceService()
        let result = try await service.recommendations(members: party, limit: 5)

        XCTAssertFalse(result.candidates.isEmpty)
        XCTAssertLessThanOrEqual(result.candidates.count, 5, "limit を超えない")
        let first = result.candidates[0]
        XCTAssertGreaterThan(first.pokemon.count, BalancePickTiles.visibleCount, "「もっと見る」が出る件数")
        let master = try MockPokeCalcService()
        for pokemon in first.pokemon.prefix(4) {
            let detail = try await master.species(key: pokemon.pokemonId)
            XCTAssertEqual(detail.nameJa, pokemon.nameJa, "先頭はモックのマスタにいる種族(タップで枠に入れられる)")
        }
        for pokemon in result.candidates.flatMap(\.pokemon) {
            XCTAssertTrue(pokemon.nameJa?.hasPrefix("テスト") ?? false, "架空の名前: \(pokemon.nameJa ?? "-")")
        }
        XCTAssertFalse(result.skewedAttackTypes.isEmpty)
    }

    func testFixtureThreatsFollowRequestOrder() async throws {
        let service = MockBalanceService()
        let threats = [BalanceMemberInput(pokemonId: "9003-000", abilityId: nil, moveIds: [])]
        let result = try await service.threats(members: party, threats: threats)
        XCTAssertEqual(result.threats.map(\.pokemonId), ["9003-000"])
        XCTAssertEqual(result.threats[0].matchups.map(\.pokemonId), ["9001-000", "9002-000"])
    }
}
