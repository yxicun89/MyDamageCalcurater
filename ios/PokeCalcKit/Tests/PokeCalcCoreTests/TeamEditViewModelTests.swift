import XCTest

@testable import PokeCalcCore

// TeamEditViewModel(構築編集画面の状態。P6-2c・ADR-0501「P6-2c」)。
//
// マスタ参照(species/moves/items/natures)は `CalcViewModel` と同じ `PokeCalcService` を再利用する
// (`StubPokeCalcService` / `StubMaster` は `CalcViewModelTests` と共用。P6-2c 用に新しいサービス実装は作らない)。
// 特性の選択肢は種族ごとの `SpeciesDetail.abilities` から出す(`PokeCalcService` に検索 API が無いため)。
//
// 決定した形:
// - `@MainActor @Observable final class TeamEditViewModel`。`init(store: any TeamStore, service: any PokeCalcService, team: Team)`。
// - `team: Team`(private(set))・`speciesOptions: [SpeciesSummary]`・`itemOptions: [Item]`・`natureOptions: [Nature]`
//   ・`moveOptionsByMember: [String: [Move]]`(メンバー id → 現在の種族の learnset の順・マスタにある技だけ。
//   `CalcViewModel.moveOptions` と同じ規則)・`abilityOptionsByMember: [String: [Ability]]`
//   ・`isLoading: Bool`・`error: TeamScreenError?`・`nameError: TeamFieldError?`(名前が空)
//   ・`teamError: TeamFieldError?`(6体を超える追加の試み)・`memberErrors: [String: TeamMemberFieldError]`
//   (メンバー id → 直近の入力エラー。技の重複・上限・SP の超過)。
// - `load()`: マスタ(species/items/natures)を読み、**既存の各メンバー**について `species(key:)` を呼んで
//   `moveOptionsByMember` / `abilityOptionsByMember` を作る。
// - `setName(_:)`: 前後空白を落として `team.name` に入れる。空になったら `nameError = .emptyName`、
//   それ以外では nil にする(保存を止めはしない。保存時に再検証する)。
// - `addMember(speciesKey:) async -> Bool`: 既に `TeamLimits.maxMembers` 体あれば追加せず false を返し
//   `teamError = .tooManyMembers`。そうでなければ新しい `TeamMember`(性格は `natureOptions.first`、
//   他は既定値)を作り、`species(key:)` を呼んで `moveOptionsByMember` / `abilityOptionsByMember` を用意してから
//   `team.members` へ追加する。
// - `removeMember(id:)`: `team.members` から取り除き、対応する `moveOptionsByMember` / `abilityOptionsByMember` も消す。
// - `setMemberSpecies(id:speciesKey:) async`: そのメンバーの `speciesKey` を差し替え、`species(key:)` を
//   呼び直して選択肢を更新する。新しい learnset に無い `moveIds` は黙って落とす(`CalcViewModel` の
//   「選択肢に無い技は無視」と同じ扱い)。応答は世代(メンバーごとのトークン)で守り、追い越された古い応答は
//   無視する(`CalcViewModel` の species 世代保護と同じ理由: 連続で種族を変えたときに古い応答が後から
//   上書きしないように)。
// - `addMove(id:moveId:) -> Bool`: 既に `TeamLimits.maxMovesPerMember` 個あれば false + `.tooManyMoves`。
//   既に同じ `moveId` を持っていれば false + `.duplicateMove`。それ以外は追加して true(成功時は
//   `memberErrors[id]` を nil にする)。
// - `removeMove(id:at:)`: 範囲内の index を取り除く(範囲外は無視)。
// - `setMemberItem` / `setMemberAbility` / `setMemberNature` / `setMemberTeraType`: そのまま代入するだけ
//   (ID の存在チェックはピッカーの選択肢がマスタ由来なのでここではしない。`CalcViewModel.selectAttackerItem` と同じ扱い)。
// - `setMemberSP(id:stat:value:) -> Bool`: `0...SPLimits.maxPerStat` の外、または変更後の合計が
//   `SPLimits.maxTotal` を超えるなら変更せず false(`memberErrors[id]` に `.spPerStatExceeded` /
//   `.spTotalExceeded`)。それ以外は代入して true(`memberErrors[id]` を nil にする)。
// - `save() async -> Bool`: `store.save(team)` を呼ぶ。成功で true、失敗で `error` を立てて false。
@MainActor
final class TeamEditViewModelTests: XCTestCase {

    /// `load()` 中に `species(key:)` を呼ぶ既存メンバー1体(alpha)を持つチームで組み立てた ViewModel。
    private func loadedViewModel(
        _ service: StubPokeCalcService = StubMaster.makeService(),
        member: TeamMember? = nil,
        store: StubTeamStore = StubTeamStore()
    ) async -> (TeamEditViewModel, StubTeamStore) {
        let initialMember = member ?? TeamMember(id: "member-alpha", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")
        let team = Team(id: "team-1", name: "テストチーム", members: [initialMember])
        let viewModel = TeamEditViewModel(store: store, service: service, team: team)
        await viewModel.load()
        return (viewModel, store)
    }

    // MARK: - load

    func testLoadPopulatesMasterOptions() async {
        let (viewModel, _) = await loadedViewModel()
        XCTAssertEqual(Set(viewModel.speciesOptions.map(\.key)),
                       Set([StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly].map(\.key)))
        XCTAssertEqual(viewModel.itemOptions.map(\.id), [StubMaster.itemA.id, StubMaster.itemB.id])
        XCTAssertEqual(viewModel.natureOptions.map(\.id), ["stub-nature-atk-up", "stub-nature-neutral", "stub-nature-spa-up"])
    }

    /// alpha の learnset([status, alphaOnly, special, 未知])のうちマスタにある技だけ、learnset の順。
    func testLoadPopulatesPerMemberMoveAndAbilityOptions() async {
        let (viewModel, _) = await loadedViewModel()
        XCTAssertEqual(
            viewModel.moveOptionsByMember["member-alpha"]?.map(\.id),
            [StubMaster.statusMove.id, StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id]
        )
        XCTAssertEqual(viewModel.abilityOptionsByMember["member-alpha"], StubMaster.alpha.abilities)
    }

    // MARK: - addMember

    func testAddMemberAppendsWithDefaultsAndOptions() async {
        let (viewModel, _) = await loadedViewModel()
        let ok = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        XCTAssertTrue(ok)
        XCTAssertEqual(viewModel.team.members.count, 2)
        let added = try? XCTUnwrap(viewModel.team.members.last)
        XCTAssertEqual(added?.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(added?.natureId, viewModel.natureOptions.first?.id, "既定の性格は一覧の先頭")
        XCTAssertEqual(added?.moveIds, [])
        XCTAssertEqual(added?.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
        guard let addedID = added?.id else { return XCTFail("追加したメンバーが無い") }
        XCTAssertEqual(
            viewModel.moveOptionsByMember[addedID]?.map(\.id),
            [StubMaster.statusMove.id, StubMaster.specialMove.id, StubMaster.physicalMove.id]
        )
        XCTAssertEqual(viewModel.abilityOptionsByMember[addedID], StubMaster.beta.abilities)
        XCTAssertNil(viewModel.teamError)
    }

    func testAddMemberFailsAtMaxMembers() async {
        let members = (0..<TeamLimits.maxMembers).map { index in
            TeamMember(id: "member-\(index)", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral")
        }
        let store = StubTeamStore()
        let service = StubMaster.makeService()
        let team = Team(id: "team-1", name: "テストチーム", members: members)
        let viewModel = TeamEditViewModel(store: store, service: service, team: team)
        await viewModel.load()

        let ok = await viewModel.addMember(speciesKey: StubMaster.beta.key)
        XCTAssertFalse(ok)
        XCTAssertEqual(viewModel.team.members.count, TeamLimits.maxMembers, "追加しない")
        XCTAssertEqual(viewModel.teamError, .tooManyMembers)
    }

    // MARK: - removeMember

    func testRemoveMemberRemovesFromTeamAndOptions() async {
        let (viewModel, _) = await loadedViewModel()
        viewModel.removeMember(id: "member-alpha")
        XCTAssertEqual(viewModel.team.members, [])
        XCTAssertNil(viewModel.moveOptionsByMember["member-alpha"])
        XCTAssertNil(viewModel.abilityOptionsByMember["member-alpha"])
    }

    // MARK: - setMemberSpecies

    func testSetMemberSpeciesUpdatesOptionsAndDropsInvalidMoves() async {
        let member = TeamMember(
            id: "member-alpha", speciesKey: StubMaster.alpha.key,
            moveIds: [StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id], natureId: "stub-nature-neutral"
        )
        let (viewModel, _) = await loadedViewModel(member: member)

        await viewModel.setMemberSpecies(id: "member-alpha", speciesKey: StubMaster.gamma.key)

        let updated = viewModel.team.members.first { $0.id == "member-alpha" }
        XCTAssertEqual(updated?.speciesKey, StubMaster.gamma.key)
        // gamma の learnset は [physical, special]。alphaOnly はもう覚えないので落ちる。
        XCTAssertEqual(updated?.moveIds, [StubMaster.specialMove.id])
        XCTAssertEqual(
            viewModel.moveOptionsByMember["member-alpha"]?.map(\.id),
            [StubMaster.physicalMove.id, StubMaster.specialMove.id]
        )
        XCTAssertEqual(viewModel.abilityOptionsByMember["member-alpha"], StubMaster.gamma.abilities)
    }

    /// 種族を連続で変えたとき、古い方の `species(key:)` 応答が後から届いても上書きしない
    /// (`CalcViewModel` の `testStaleSpeciesDetailDoesNotOverwriteNewerAttackerSelection` と同じ理由)。
    func testStaleSpeciesResponseForMemberIsIgnored() async throws {
        let service = StubMaster.makeService()
        let (viewModel, _) = await loadedViewModel(service)
        let baseline = await service.speciesRequests.count
        await service.setSpeciesMode(.manual)

        let selectGamma = Task { await viewModel.setMemberSpecies(id: "member-alpha", speciesKey: StubMaster.gamma.key) }
        try await service.waitForSpeciesRequests(count: baseline + 1)
        let selectBeta = Task { await viewModel.setMemberSpecies(id: "member-alpha", speciesKey: StubMaster.beta.key) }
        try await service.waitForSpeciesRequests(count: baseline + 2)

        let betaDetail = try await service.lookupSpecies(key: StubMaster.beta.key)
        await service.resolveSpecies(at: baseline + 1, with: .success(betaDetail))
        await selectBeta.value
        XCTAssertEqual(viewModel.team.members.first?.speciesKey, StubMaster.beta.key)

        let gammaDetail = try await service.lookupSpecies(key: StubMaster.gamma.key)
        await service.resolveSpecies(at: baseline, with: .success(gammaDetail))
        await selectGamma.value
        XCTAssertEqual(viewModel.team.members.first?.speciesKey, StubMaster.beta.key, "古い応答で上書きしない")
        XCTAssertEqual(
            viewModel.moveOptionsByMember["member-alpha"]?.map(\.id),
            [StubMaster.statusMove.id, StubMaster.specialMove.id, StubMaster.physicalMove.id],
            "古い応答で選択肢を上書きしない"
        )
    }

    // MARK: - addMove / removeMove

    func testAddMoveAppendsAndRemoveMoveRemoves() async {
        let (viewModel, _) = await loadedViewModel()
        XCTAssertTrue(viewModel.addMove(id: "member-alpha", moveId: StubMaster.specialMove.id))
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [StubMaster.specialMove.id])
        XCTAssertNil(viewModel.memberErrors["member-alpha"])

        viewModel.removeMove(id: "member-alpha", at: 0)
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [])
    }

    func testAddMoveRejectsDuplicate() async {
        let (viewModel, _) = await loadedViewModel()
        XCTAssertTrue(viewModel.addMove(id: "member-alpha", moveId: StubMaster.specialMove.id))
        let ok = viewModel.addMove(id: "member-alpha", moveId: StubMaster.specialMove.id)
        XCTAssertFalse(ok)
        XCTAssertEqual(viewModel.team.members.first?.moveIds, [StubMaster.specialMove.id], "増えない")
        XCTAssertEqual(viewModel.memberErrors["member-alpha"], .duplicateMove)
    }

    func testAddMoveRejectsOverMax() async {
        let member = TeamMember(
            id: "member-alpha", speciesKey: StubMaster.alpha.key,
            moveIds: ["m1", "m2", "m3", "m4"], natureId: "stub-nature-neutral"
        )
        let (viewModel, _) = await loadedViewModel(member: member)
        XCTAssertEqual(TeamLimits.maxMovesPerMember, 4)
        let ok = viewModel.addMove(id: "member-alpha", moveId: "m5")
        XCTAssertFalse(ok)
        XCTAssertEqual(viewModel.team.members.first?.moveIds, ["m1", "m2", "m3", "m4"])
        XCTAssertEqual(viewModel.memberErrors["member-alpha"], .tooManyMoves)
    }

    // MARK: - item / ability / nature / teraType

    func testSimpleFieldSetters() async {
        let (viewModel, _) = await loadedViewModel()
        viewModel.setMemberItem(id: "member-alpha", itemId: StubMaster.itemA.id)
        viewModel.setMemberAbility(id: "member-alpha", abilityId: "stub-ability")
        viewModel.setMemberNature(id: "member-alpha", natureId: "stub-nature-spa-up")
        viewModel.setMemberTeraType(id: "member-alpha", teraType: .fairy)

        let updated = viewModel.team.members.first
        XCTAssertEqual(updated?.itemId, StubMaster.itemA.id)
        XCTAssertEqual(updated?.abilityId, "stub-ability")
        XCTAssertEqual(updated?.natureId, "stub-nature-spa-up")
        XCTAssertEqual(updated?.teraType, .fairy)

        // nil に戻せる。
        viewModel.setMemberItem(id: "member-alpha", itemId: nil)
        viewModel.setMemberTeraType(id: "member-alpha", teraType: nil)
        XCTAssertNil(viewModel.team.members.first?.itemId)
        XCTAssertNil(viewModel.team.members.first?.teraType)
    }

    /// 前後空白を落とし、空になったら nil にする(`setName` と同じ規則)。
    func testSetMemberNicknameTrimsAndTreatsBlankAsNil() async {
        let (viewModel, _) = await loadedViewModel()
        viewModel.setMemberNickname(id: "member-alpha", nickname: "  テストのニックネーム  ")
        XCTAssertEqual(viewModel.team.members.first?.nickname, "テストのニックネーム")

        viewModel.setMemberNickname(id: "member-alpha", nickname: "   ")
        XCTAssertNil(viewModel.team.members.first?.nickname)
    }

    // MARK: - setMemberSP

    func testSetMemberSPWithinLimitsSucceeds() async {
        let (viewModel, _) = await loadedViewModel()
        let ok = viewModel.setMemberSP(id: "member-alpha", stat: .atk, value: 32)
        XCTAssertTrue(ok)
        XCTAssertEqual(viewModel.team.members.first?.sp.atk, 32)
        XCTAssertNil(viewModel.memberErrors["member-alpha"])
    }

    func testSetMemberSPPerStatOverLimitIsRejected() async {
        let (viewModel, _) = await loadedViewModel()
        let ok = viewModel.setMemberSP(id: "member-alpha", stat: .atk, value: SPLimits.maxPerStat + 1)
        XCTAssertFalse(ok)
        XCTAssertEqual(viewModel.team.members.first?.sp.atk, 0, "変更しない")
        XCTAssertEqual(viewModel.memberErrors["member-alpha"], .spPerStatExceeded)
    }

    func testSetMemberSPTotalOverLimitIsRejected() async {
        let member = TeamMember(
            id: "member-alpha", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral",
            sp: StatBlock(hp: 0, atk: 32, def: 32, spa: 0, spd: 0, spe: 0)
        )
        let (viewModel, _) = await loadedViewModel(member: member)
        // 現在合計64。+4 すると68 > 66 なので拒否。
        let ok = viewModel.setMemberSP(id: "member-alpha", stat: .spa, value: 4)
        XCTAssertFalse(ok)
        XCTAssertEqual(viewModel.team.members.first?.sp.spa, 0)
        XCTAssertEqual(viewModel.memberErrors["member-alpha"], .spTotalExceeded)
    }

    func testSetMemberSPNegativeIsRejected() async {
        let (viewModel, _) = await loadedViewModel()
        let ok = viewModel.setMemberSP(id: "member-alpha", stat: .atk, value: -1)
        XCTAssertFalse(ok)
        XCTAssertEqual(viewModel.memberErrors["member-alpha"], .spPerStatExceeded)
    }

    // MARK: - setName

    func testSetNameTrimsAndClearsError() async {
        let (viewModel, _) = await loadedViewModel()
        viewModel.setName("  テストあたらしい名前  ")
        XCTAssertEqual(viewModel.team.name, "テストあたらしい名前")
        XCTAssertNil(viewModel.nameError)
    }

    func testSetBlankNameSetsError() async {
        let (viewModel, _) = await loadedViewModel()
        viewModel.setName("   ")
        XCTAssertEqual(viewModel.nameError, .emptyName)
    }

    // MARK: - save

    func testSaveSucceedsAndPersistsThroughStore() async {
        let (viewModel, store) = await loadedViewModel()
        let ok = await viewModel.save()
        XCTAssertTrue(ok)
        XCTAssertNil(viewModel.error)
        let saved = await store.saveCalls
        XCTAssertEqual(saved.last?.id, "team-1")
    }

    func testSaveFailureSetsError() async {
        let store = StubTeamStore()
        await store.setSaveError(PokeCalcError(code: "test_save_failure", message: "テスト: 保存に失敗"))
        let service = StubMaster.makeService()
        let team = Team(id: "team-1", name: "テストチーム", members: [
            TeamMember(id: "member-alpha", speciesKey: StubMaster.alpha.key, natureId: "stub-nature-neutral"),
        ])
        let viewModel = TeamEditViewModel(store: store, service: service, team: team)
        await viewModel.load()

        let ok = await viewModel.save()
        XCTAssertFalse(ok)
        XCTAssertNotNil(viewModel.error)
    }
}
