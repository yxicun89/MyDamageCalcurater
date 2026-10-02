import XCTest

@testable import PokeCalcCore

/// 構築 → 要求の純粋関数(P6-26。ADR-0505 §5)。送るのは `pokemonId`(= speciesKey)・特性 ID・技 ID だけ。
/// タイプ・相性は balance が read model から引くので、iOS は種族のタイプも相性表も持ち込まない(ADR-0014 §1)。
final class BalanceRequestBuilderTests: XCTestCase {
    private typealias B = StubBalance

    func testAnalyzeUsesSpeciesKeyAsPokemonIdInTeamOrder() {
        let requests = BalanceRequestBuilder.make(from: B.teamMixed)
        XCTAssertEqual(
            requests.analyze?.members.map(\.pokemonId), [StubMaster.alpha.key, StubMaster.beta.key], "メンバーは構築の順・pokemonId は speciesKey のまま")
    }

    /// 特性は nil・空文字なら欄ごと送らない(`null` も空文字も送らない)。
    func testAnalyzeAbilityIsOmittedWhenAbsentOrEmpty() {
        let requests = BalanceRequestBuilder.make(from: B.teamFour)
        XCTAssertEqual(
            requests.analyze?.members.map(\.abilityId), [StubMaster.ability.id, nil, nil, nil],
            "A は特性あり・B は nil・C は空文字 → nil・Unknown は nil")
    }

    /// 同じ種族の重複も 1 体ずつ送る(取りまとめない。応答の index と対応づけるため)。
    func testDuplicatedSpeciesAreKeptAsSeparateMembers() {
        let requests = BalanceRequestBuilder.make(from: B.teamFour)
        XCTAssertEqual(requests.analyze?.members.count, 4)
        XCTAssertEqual(requests.analyze?.members[0].pokemonId, requests.analyze?.members[2].pokemonId)
    }

    func testCoverageCarriesMoveIdsInOrderIncludingEmptyMembers() {
        let requests = BalanceRequestBuilder.make(from: B.teamMixed)
        XCTAssertEqual(
            requests.coverage?.members,
            [BalanceCoverageMember(pokemonId: StubMaster.alpha.key, moveIds: [StubMaster.physicalMove.id, StubMaster.specialMove.id]),
             BalanceCoverageMember(pokemonId: StubMaster.beta.key, moveIds: [])],
            "技の順を保つ・技が無いメンバーも空配列で載せる(coverage の moveIds は必須)")
    }

    func testCoverageMoveIdsAreCappedAtTheContractLimitAndDeduplicated() {
        let member = TeamMember(
            id: "dup", speciesKey: StubMaster.alpha.key,
            moveIds: ["stub-move-a", "stub-move-a", "stub-move-b", "stub-move-c", "stub-move-d", "stub-move-e"],
            natureId: StubMaster.neutralNature.id)
        let requests = BalanceRequestBuilder.make(from: Team(id: "t", name: "テスト", members: [member]))
        XCTAssertEqual(
            requests.coverage?.members[0].moveIds, ["stub-move-a", "stub-move-b", "stub-move-c", "stub-move-d"],
            "重複を除き(契約の uniqueItems)・先頭から maxBalanceMovesPerMember 件まで")
        XCTAssertLessThanOrEqual(requests.coverage?.members[0].moveIds.count ?? 99, RequestLimits.maxBalanceMovesPerMember)
    }

    /// 技を持つメンバーが 1 体もいなければ coverage は送らない(analyze は送る)。
    func testNoMovesAnywhereMeansNoCoverageRequest() {
        let requests = BalanceRequestBuilder.make(from: B.teamNoMoves)
        XCTAssertNotNil(requests.analyze)
        XCTAssertNil(requests.coverage)
    }

    func testEmptyTeamMeansNoRequestsAtAll() {
        let requests = BalanceRequestBuilder.make(from: B.teamEmpty)
        XCTAssertNil(requests.analyze)
        XCTAssertNil(requests.coverage)
    }

    func testSixMembersAreAllSent() {
        let requests = BalanceRequestBuilder.make(from: B.teamSix)
        XCTAssertEqual(requests.analyze?.members.count, 6)
        XCTAssertEqual(requests.coverage?.members.count, 6)
        XCTAssertEqual(requests.coverage?.members.map { $0.moveIds.count }, [1, 0, 1, 0, 1, 0])
    }

    /// 壊れた保存データ(7 体)でも契約の上限を超えて送らない(先頭 6 体。analyze と coverage で同じメンバー)。
    func testMoreThanTheLimitSendsOnlyTheFirstSix() {
        let requests = BalanceRequestBuilder.make(from: B.teamSeven)
        XCTAssertEqual(requests.analyze?.members.count, RequestLimits.maxBalanceMembers)
        XCTAssertEqual(requests.coverage?.members.count, RequestLimits.maxBalanceMembers)
    }

    /// 要求に相性・タイプを持ち込まない(型の上でも、送る欄は pokemonId・abilityId・moveIds だけ)。
    func testRequestsDoNotCarryTypesOrMultipliers() {
        let requests = BalanceRequestBuilder.make(from: B.teamMixed)
        let text = String(describing: requests)
        for word in ["types", "multiplier", "category"] {
            XCTAssertFalse(text.contains(word), "要求に \(word) が入っている: \(text)")
        }
    }
}
