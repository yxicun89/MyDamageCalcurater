import XCTest

@testable import PokeCalcCore

// P6-20: 名前 ⇄ ID の解決(`ShowdownTransferService`。ADR-0501「P6-20」4章)。
// マスタは `StubMaster` の架空データ。通信失敗でも投げず、結果で伝える。
final class ShowdownTransferServiceTests: XCTestCase {
    private func makeService(
        species: [SpeciesDetail] = [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly, StubMaster.abilityXOnly]
    ) -> StubPokeCalcService {
        StubMaster.makeService(species: species)
    }

    private func resolve(
        _ text: String, existing: Int = 0, stub: StubPokeCalcService? = nil,
        naming: any ShowdownNaming = JapaneseShowdownNaming(), limit: Int = ShowdownTransferLimits.maxConcurrentRequests
    ) async -> ShowdownImportPlan {
        let service = ShowdownTransferService(service: stub ?? makeService(), naming: naming, maxConcurrentRequests: limit)
        return await service.resolve(ShowdownTextParser.parse(text), existingMemberCount: existing)
    }

    // MARK: 取り込み

    func testResolvesEveryNameToItsID() async throws {
        let plan = await resolve("""
            ニック (テストアルファ) @ テストどうぐA
            Ability: テストとくせい
            Tera Type: ほのお
            SP: 32 Atk / 20 Spe
            Nature: テストせいかく特攻上昇
            - テストわざぶつり
            - テストわざへんか
            """)
        XCTAssertEqual(plan.rejected, [])
        XCTAssertNil(plan.error)
        let member = try XCTUnwrap(plan.members.first)
        XCTAssertEqual(plan.members.count, 1)
        XCTAssertFalse(member.id.isEmpty)
        XCTAssertEqual(member.speciesKey, "9101-000")
        XCTAssertEqual(member.nickname, "ニック")
        XCTAssertEqual(member.itemId, "stub-item-a")
        XCTAssertEqual(member.abilityId, "stub-ability")
        XCTAssertEqual(member.natureId, "stub-nature-spa-up")
        XCTAssertEqual(member.teraType, .fire)
        XCTAssertEqual(member.sp, StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 20))
        XCTAssertEqual(member.moveIds, ["stub-move-physical", "stub-move-status"])
    }

    func testMembersGetDistinctNewIDs() async {
        let plan = await resolve("テストアルファ\n\nテストアルファ")
        XCTAssertEqual(Set(plan.members.map(\.id)).count, 2)
    }

    func testMissingNatureLineUsesTheFirstNatureLikeAddMember() async {
        let plan = await resolve("テストアルファ")
        XCTAssertEqual(plan.members.first?.natureId, "stub-nature-atk-up")
        XCTAssertEqual(plan.rejected, [])
    }

    func testUnresolvedFieldsAreReportedPerLineAndTheMemberIsStillImported() async throws {
        let plan = await resolve("""
            テストアルファ @ そんなどうぐ
            Ability: そんなとくせい
            Tera Type: そんなタイプ
            Nature: そんなせいかく
            - そんなわざ
            - テストわざぶつり
            """)
        XCTAssertEqual(plan.rejected, [
            .init(lineNumber: 1, text: "テストアルファ @ そんなどうぐ", reason: .itemNotFound),
            .init(lineNumber: 2, text: "Ability: そんなとくせい", reason: .abilityNotFound),
            .init(lineNumber: 3, text: "Tera Type: そんなタイプ", reason: .teraTypeNotFound),
            .init(lineNumber: 4, text: "Nature: そんなせいかく", reason: .natureNotFound),
            .init(lineNumber: 5, text: "- そんなわざ", reason: .moveNotFound),
        ])
        let member = try XCTUnwrap(plan.members.first)
        XCTAssertNil(member.itemId)
        XCTAssertNil(member.abilityId)
        XCTAssertNil(member.teraType)
        XCTAssertEqual(member.natureId, "stub-nature-atk-up", "解決できない性格は既定の性格にして報告する")
        XCTAssertEqual(member.moveIds, ["stub-move-physical"])
    }

    func testUnknownSpeciesDropsTheMemberAndReportsItsHeaderOnly() async {
        let plan = await resolve("""
            そんなポケ @ テストどうぐA
            Ability: テストとくせい
            - テストわざぶつり

            テストアルファ
            """)
        XCTAssertEqual(plan.members.map(\.speciesKey), ["9101-000"])
        XCTAssertEqual(plan.rejected, [.init(lineNumber: 1, text: "そんなポケ @ テストどうぐA", reason: .speciesNotFound)])
    }

    func testAbilityMustBelongToTheSpecies() async {
        let ok = await resolve("テストとくせいXのみ\nAbility: テストとくせいX")
        XCTAssertEqual(ok.members.first?.abilityId, StubMaster.abilityX.id)
        XCTAssertEqual(ok.rejected, [])
        let wrong = await resolve("テストとくせいXのみ\nAbility: テストとくせいY")
        XCTAssertNil(wrong.members.first?.abilityId, "マスタにあっても、そのポケモンの特性でなければ採らない")
        XCTAssertEqual(wrong.rejected.map(\.reason), [.abilityNotFound])
    }

    func testParserRejectionsAreMergedInLineOrder() async {
        let plan = await resolve("テストアルファ\nEVs: 252 Atk\nAbility: そんなとくせい\nLevel: 50")
        XCTAssertEqual(plan.rejected.map(\.lineNumber), [2, 3, 4])
        XCTAssertEqual(plan.rejected.map(\.reason), [.unsupportedStatLine, .abilityNotFound, .unrecognizedLine])
        XCTAssertEqual(plan.members.count, 1)
    }

    func testMemberCapacityUsesExistingMembers() async {
        let text = "テストアルファ\n\nテストベータ\n\nテストガンマ"
        let withTwoFree = await resolve(text, existing: TeamLimits.maxMembers - 2)
        XCTAssertEqual(withTwoFree.members.map(\.speciesKey), ["9101-000", "9102-000"])
        XCTAssertEqual(withTwoFree.rejected, [.init(lineNumber: 5, text: "テストガンマ", reason: .memberLimitExceeded)])
        let full = await resolve(text, existing: TeamLimits.maxMembers)
        XCTAssertEqual(full.members, [])
        XCTAssertEqual(full.rejected.map(\.reason), [.memberLimitExceeded, .memberLimitExceeded, .memberLimitExceeded])
    }

    func testEveryResolvedMemberPassesTeamValidator() async {
        let plan = await resolve("""
            テストアルファ
            SP: 32 HP / 32 Atk / 3 Def
            - テストわざぶつり
            - テストわざぶつり
            - テストわざとくしゅ
            - テストわざへんか
            - テストわざアルファ専用
            - テストわざぶつり

            テストベータ
            SP: 33 HP
            """)
        XCTAssertEqual(plan.members.count, 2)
        XCTAssertNil(TeamValidator.firstViolation(in: Team(name: "検証", members: plan.members)))
        XCTAssertTrue(plan.rejected.map(\.reason).contains(.spTotalExceeded))
        XCTAssertTrue(plan.rejected.map(\.reason).contains(.spOutOfRange))
    }

    // MARK: 通信量

    func testSameNameIsLookedUpOnlyOnce() async {
        let stub = makeService()
        _ = await resolve("""
            テストアルファ @ テストどうぐA
            - テストわざぶつり

            テストアルファ @ テストどうぐA
            - テストわざぶつり
            """, stub: stub)
        let speciesCalls = await stub.speciesSearchCalls
        let speciesRequests = await stub.speciesRequests
        let moveCalls = await stub.moveSearchCalls
        XCTAssertEqual(speciesCalls.filter { $0.query == "テストアルファ" }.count, 1)
        XCTAssertEqual(speciesRequests.filter { $0 == "9101-000" }.count, 1)
        XCTAssertEqual(moveCalls.filter { $0.query == "テストわざぶつり" }.count, 1)
        XCTAssertTrue(speciesCalls.allSatisfy { $0.limit == ShowdownTransferLimits.searchLimit })
        XCTAssertTrue(moveCalls.allSatisfy { $0.limit == ShowdownTransferLimits.searchLimit })
    }

    func testConcurrentRequestsNeverExceedTheLimit() async {
        let text = """
            テストアルファ @ テストどうぐA
            - テストわざぶつり
            - テストわざとくしゅ

            テストベータ @ テストどうぐB
            - テストわざへんか
            - テストわざアルファ専用

            テストガンマ
            Nature: テストせいかく特攻上昇

            テストへんかのみ
            """
        for limit in [1, 2, ShowdownTransferLimits.maxConcurrentRequests] {
            let probe = ConcurrencyProbeService(inner: makeService())
            let service = ShowdownTransferService(service: probe, maxConcurrentRequests: limit)
            let plan = await service.resolve(ShowdownTextParser.parse(text), existingMemberCount: 0)
            XCTAssertEqual(plan.members.count, 4, "limit \(limit)")
            let maxInFlight = await probe.maxInFlight
            XCTAssertLessThanOrEqual(maxInFlight, limit, "limit \(limit)")
        }
    }

    // MARK: 通信失敗

    func testMasterFailureDoesNotThrowAndReportsLookupFailed() async {
        let stub = makeService()
        let failure = PokeCalcError(code: PokeCalcError.Code.transport, message: "down")
        await stub.setMasterError(failure)
        let plan = await resolve("テストアルファ\n\nテストベータ", stub: stub)
        XCTAssertEqual(plan.error, failure)
        XCTAssertEqual(plan.members, [])
        XCTAssertEqual(plan.rejected.map(\.reason), [.lookupFailed, .lookupFailed])
        XCTAssertEqual(plan.rejected.map(\.lineNumber), [1, 3], "「見つからない」ではなく通信失敗として先頭行に付く")
    }

    // MARK: 名前の戦略(拡張点)

    /// 名前の末尾に「★」を付ける戦略。検索語は「★」を除いたもの(英語名に広げるときも同じ形で差し替える)。
    struct StarNaming: ShowdownNaming {
        func name(of species: SpeciesDetail) -> String { species.nameJa + "★" }
        func name(of move: Move) -> String { move.nameJa + "★" }
        func name(of item: Item) -> String { item.nameJa + "★" }
        func name(of ability: Ability) -> String { ability.nameJa + "★" }
        func name(of nature: Nature) -> String { nature.nameJa + "★" }
        func name(of type: PokeType) -> String { PokeTypeLabel.japaneseName(for: type) + "★" }
        func searchQuery(forImportedName name: String) -> String { name.replacingOccurrences(of: "★", with: "") }
        func type(forImportedName imported: String) -> PokeType? {
            PokeType.allCases.first { name(of: $0) == imported }
        }
    }

    func testCustomNamingIsUsedForImportAndExport() async throws {
        let naming = StarNaming()
        let stub = makeService()
        let transfer = ShowdownTransferService(service: stub, naming: naming)
        let member = TeamMember(
            id: "m1", speciesKey: "9101-000", moveIds: ["stub-move-physical"], itemId: "stub-item-a",
            abilityId: "stub-ability", natureId: "stub-nature-atk-up", teraType: .fire
        )
        let exported = await transfer.export(members: [member])
        let text = try XCTUnwrap(exported.text)
        XCTAssertEqual(text, """
            テストアルファ★ @ テストどうぐA★
            Ability: テストとくせい★
            Tera Type: ほのお★
            Nature: テストせいかく攻撃上昇★
            - テストわざぶつり★
            """)
        let plan = await transfer.resolve(ShowdownTextParser.parse(text), existingMemberCount: 0)
        XCTAssertEqual(plan.rejected, [])
        let back = try XCTUnwrap(plan.members.first)
        XCTAssertEqual(back.speciesKey, member.speciesKey)
        XCTAssertEqual(back.itemId, member.itemId)
        XCTAssertEqual(back.abilityId, member.abilityId)
        XCTAssertEqual(back.natureId, member.natureId)
        XCTAssertEqual(back.moveIds, member.moveIds)
        XCTAssertEqual(back.teraType, member.teraType)
    }

    // MARK: 書き出し

    func testExportWritesNamesFromTheMaster() async throws {
        let stub = makeService()
        let transfer = ShowdownTransferService(service: stub)
        let a = TeamMember(
            id: "a", speciesKey: "9101-000", moveIds: ["stub-move-physical", "stub-move-status"], itemId: "stub-item-a",
            abilityId: "stub-ability", natureId: "stub-nature-atk-up", sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 20),
            teraType: .fire
        )
        let b = TeamMember(id: "b", speciesKey: "9101-000", natureId: "stub-nature-neutral")
        let result = await transfer.export(members: [a, b])
        XCTAssertNil(result.error)
        XCTAssertEqual(result.text, """
            テストアルファ @ テストどうぐA
            Ability: テストとくせい
            Tera Type: ほのお
            SP: 32 Atk / 20 Spe
            Nature: テストせいかく攻撃上昇
            - テストわざぶつり
            - テストわざへんか

            テストアルファ
            Nature: テストせいかく無補正
            """)
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests, ["9101-000"], "同じ種族は1回しか引かない")
        let batches = await stub.moveBatchRequests
        XCTAssertEqual(batches.count, 1, "技は moves(ids:) を1回")
    }

    func testExportFailureReportsErrorWithoutThrowing() async {
        let stub = makeService()
        let failure = PokeCalcError(code: PokeCalcError.Code.transport, message: "down")
        await stub.setMasterError(failure)
        let member = TeamMember(id: "a", speciesKey: "9101-000", natureId: "stub-nature-atk-up")
        let result = await ShowdownTransferService(service: stub).export(members: [member])
        XCTAssertNil(result.text)
        XCTAssertEqual(result.error, failure)
        XCTAssertEqual(result.skippedMemberIds, ["a"])
    }

    func testExportSkipsAMemberWhoseSpeciesIsMissingFromTheMaster() async {
        let stub = makeService()
        let ok = TeamMember(id: "ok", speciesKey: "9101-000", natureId: "stub-nature-atk-up")
        let missing = TeamMember(id: "missing", speciesKey: "no-such", natureId: "stub-nature-atk-up")
        let result = await ShowdownTransferService(service: stub).export(members: [missing, ok])
        XCTAssertEqual(result.text, "テストアルファ\nNature: テストせいかく攻撃上昇")
        XCTAssertEqual(result.skippedMemberIds, ["missing"])
    }
}
