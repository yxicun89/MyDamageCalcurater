import XCTest

@testable import PokeCalcCore

/// メガストーンの表示名は、マスタの正式名称(日本語の文字を含む nameJa)をそのまま出す。
/// 日本語の文字が無い(英語名のフォールバック)ときだけ、従来の「{基本種名}のメガストーン」(ADR-0509 追記 §6')。
/// 既存の英語名の期待値(MegaItemLockTests 等)は変えず、ここに正式名称の期待値を足す。保存済みの ID 参照は表示名の影響を受けない。
@MainActor
final class MegaStoneOfficialNameTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private static let official = Mega.officialStone
    private static let english = Mega.stone
    private static let officialName = "テストナイトX"
    private static let baseName = StubMaster.alpha.nameJa

    // MARK: - 日本語の文字の判定(純粋関数)

    func testContainsJapanese() {
        let cases: [(name: String, text: String, expected: Bool)] = [
            ("ひらがな", "ひらがな", true),
            ("カタカナ", "リザードナイト", true),
            ("漢字", "石", true),
            ("長音だけ(ー)", "ー", true),
            ("カタカナ + 英字 + 記号", "リザードナイトX", true),
            ("英字の中に1文字だけ漢字", "Mega石", true),
            ("英数字のみ", "Charizardite X", false),
            ("数字のみ", "12345", false),
            ("全角英数字", "ＡＢＣ１２３", false),
            ("全角英字 + 全角空白", "Ｍｅｇａ　Ｓｔｏｎｅ", false),
            ("空文字", "", false),
            ("空白のみ", "  ", false),
            ("記号のみ", "-_.!?()", false),
            ("半角カナも日本語", "ﾘｻﾞｰﾄﾞ", true),
        ]
        for testCase in cases {
            XCTAssertEqual(ItemDisplayName.containsJapanese(testCase.text), testCase.expected, testCase.name)
        }
    }

    // MARK: - 表示名の関数

    func testMegaStoneNameTable() {
        let cases: [(name: String, item: Item, base: String?, expected: String)] = [
            ("正式名称 + 基本種名 → 正式名称", Self.official, Self.baseName, Self.officialName),
            ("正式名称 + 基本種名なし → 正式名称(推測せず、ある名前を出す)", Self.official, nil, Self.officialName),
            ("英語名 + 基本種名 → 従来の組み立て", Self.english, Self.baseName, "\(Self.baseName)専用のメガストーン"),
            ("英語名 + 基本種名なし → メガストーン", Self.english, nil, "メガストーン"),
            ("空の nameJa + 基本種名 → 従来の組み立て",
             Item(id: "x", nameJa: "", roles: [], isMegaStone: true), Self.baseName, "\(Self.baseName)専用のメガストーン"),
            ("全角英字の nameJa → 従来の組み立て",
             Item(id: "x", nameJa: "Ｍｅｇａ", roles: [], isMegaStone: true), Self.baseName, "\(Self.baseName)専用のメガストーン"),
        ]
        for testCase in cases {
            XCTAssertEqual(ItemDisplayName.megaStoneName(for: testCase.item, baseSpeciesNameJa: testCase.base), testCase.expected, testCase.name)
        }
    }

    func testTextForStoneWithoutKnownSpecies() {
        let items = [Self.official, Self.english]
        XCTAssertEqual(ItemDisplayName.text(itemId: Self.official.id, items: items), Self.officialName, "知らないメガでも正式名称を出す")
        XCTAssertEqual(ItemDisplayName.text(itemId: Self.english.id, items: items), "メガストーン", "英語名は従来どおり(既存の期待値)")
    }

    func testTextWithMegaStoneNamesStillWinsForEnglish() {
        let items = [Self.official, Self.english]
        let names = [Self.english.id: "\(Self.baseName)専用のメガストーン"]
        XCTAssertEqual(ItemDisplayName.text(itemId: Self.english.id, items: items, megaStoneNames: names), "\(Self.baseName)専用のメガストーン")
        XCTAssertEqual(ItemDisplayName.text(itemId: Self.official.id, items: items, megaStoneNames: names), Self.officialName)
    }

    func testDisplayItemsKeepOfficialNameAndFlag() {
        let items = ItemDisplayName.displayItems([Self.official, Self.english], megaStoneNames: [Self.english.id: "\(Self.baseName)専用のメガストーン"])
        XCTAssertEqual(items.first(where: { $0.id == Self.official.id })?.nameJa, Self.officialName)
        XCTAssertEqual(items.first(where: { $0.id == Self.english.id })?.nameJa, "\(Self.baseName)専用のメガストーン")
        XCTAssertEqual(items.map(\.id), [Self.official.id, Self.english.id], "順序・ID は変わらない")
    }

    func testBulkRowItemLabel() {
        XCTAssertEqual(BulkRowDisplay.itemLabel(itemId: Self.official.id, items: [Self.official]), Self.officialName)
        XCTAssertEqual(BulkRowDisplay.itemLabel(itemId: Self.english.id, items: [Self.english]), "メガストーン", "既存の期待値")
    }

    // MARK: - 固定・補正

    func testLockDisplayName() {
        let info = MegaSpeciesInfo(isMega: true, requiredItemId: Self.official.id, baseSpeciesNameJa: Self.baseName)
        XCTAssertEqual(
            MegaItemLock.make(for: info, allItems: [Self.official]),
            .locked(itemId: Self.official.id, displayName: Self.officialName))
        let noBase = MegaSpeciesInfo(isMega: true, requiredItemId: Self.official.id, baseSpeciesNameJa: nil)
        XCTAssertEqual(
            MegaItemLock.make(for: noBase, allItems: [Self.official]),
            .locked(itemId: Self.official.id, displayName: Self.officialName))
        let englishInfo = MegaSpeciesInfo(isMega: true, requiredItemId: Self.english.id, baseSpeciesNameJa: Self.baseName)
        XCTAssertEqual(
            MegaItemLock.make(for: englishInfo, allItems: [Self.english]),
            .locked(itemId: Self.english.id, displayName: "\(Self.baseName)専用のメガストーン"), "英語名は従来どおり")
    }

    func testCorrectionNoticeCarriesOfficialName() {
        let lock = MegaItemLock.make(
            for: MegaSpeciesInfo(isMega: true, requiredItemId: Self.official.id, baseSpeciesNameJa: Self.baseName), allItems: [Self.official])
        XCTAssertEqual(
            MegaItemLock.correction(currentItemId: nil, lock: lock), .fixed(itemId: Self.official.id, displayName: Self.officialName))
    }

    // MARK: - 未対応の印の注記

    func testUnsupportedNoteUsesOfficialName() {
        let items = ItemDisplayName.displayItems([Self.official, Self.english], megaStoneNames: [:])
        let names = UnsupportedMarkNames(moves: [], items: items, abilities: [])
        XCTAssertEqual(
            names.name(for: UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: Self.official.id)), Self.officialName)
        XCTAssertEqual(
            names.name(for: UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: Self.english.id)), "メガストーン",
            "英語名は従来どおり(既存の期待値)")
    }

    // MARK: - 計算

    private func loadedCalc(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    func testCalcAttackerShowsOfficialNameAndSendsStoneId() async throws {
        let stub = Mega.makeOfficialService()
        let viewModel = await loadedCalc(stub)

        await viewModel.selectAttacker(speciesKey: Mega.megaOfficial.key)

        XCTAssertEqual(viewModel.attackerItemLock, .locked(itemId: Self.official.id, displayName: Self.officialName))
        XCTAssertEqual(viewModel.itemLabel(for: viewModel.attackerItemId), Self.officialName)
        let requests = await stub.bulkRequests
        let request = try XCTUnwrap(requests.last)
        XCTAssertEqual(request.attacker.itemId, Self.official.id, "表示名を変えても要求の ID は変わらない")
    }

    func testCalcAttackerWithoutBaseNameShowsOfficialName() async {
        let viewModel = await loadedCalc(Mega.makeOfficialService())

        await viewModel.selectAttacker(speciesKey: Mega.megaOfficialNoBaseName.key)

        XCTAssertEqual(viewModel.itemLabel(for: Self.official.id), Self.officialName)
    }

    func testCalcDefenderRowsShowOfficialName() async {
        let viewModel = await loadedCalc(Mega.makeOfficialService())
        await viewModel.selectDefender(speciesKey: Mega.megaOfficial.key)

        await viewModel.loadDefenderAbilityOptions()

        XCTAssertEqual(viewModel.rows.map(\.itemLabel), [Self.officialName])
    }

    func testCalcEnglishStoneKeepsComposedName() async {
        let viewModel = await loadedCalc(Mega.makeOfficialService())

        await viewModel.selectAttacker(speciesKey: Mega.megaAlpha.key)

        XCTAssertEqual(viewModel.itemLabel(for: Mega.stone.id), Mega.megaAlphaStoneName, "英語名のときは従来どおり")
    }

    // MARK: - 逆算

    func testReverseCandidatesShowOfficialName() async throws {
        let stub = Mega.makeOfficialService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        let id = try XCTUnwrap(viewModel.observations.first?.id)
        await viewModel.editObservation(id: id, text: "12")
        await viewModel.selectOpponentSpecies(key: Mega.megaOfficial.key)

        await viewModel.loadOpponentAbilityOptions()

        XCTAssertEqual(viewModel.opponentItemLock, .locked(itemId: Self.official.id, displayName: Self.officialName))
        let labels = try XCTUnwrap(viewModel.result?.candidates.map(\.itemLabel))
        XCTAssertFalse(labels.isEmpty)
        XCTAssertTrue(labels.allSatisfy { $0 == Self.officialName }, "\(labels)")
        let requests = await stub.reverseRequests
        let request = try XCTUnwrap(requests.last)
        XCTAssertEqual(request.itemCandidates, [Self.official.id])
    }

    // MARK: - 判定

    func testJudgeCandidateShowsOfficialName() async throws {
        let viewModel = JudgeHarness.makeViewModel(master: Mega.makeOfficialService())
        await viewModel.load()
        await viewModel.setSpecies(JudgeHarness.alpha, for: .attacker)

        await viewModel.setSpecies(SpeciesSummary(detail: Mega.megaOfficial), for: .candidate(0))

        XCTAssertEqual(viewModel.itemLock(for: .candidate(0)), .locked(itemId: Self.official.id, displayName: Self.officialName))
        XCTAssertEqual(viewModel.itemLabel(for: Self.official.id), Self.officialName)
        XCTAssertEqual(viewModel.draft(for: .candidate(0))?.itemId, Self.official.id)
    }

    // MARK: - 調整

    func testAdjustOwnItemShowsOfficialName() async {
        let master = Mega.makeOfficialService(natures: StubMaster.reverseNatures)
        await master.setMoveBatchMode(.immediate)
        let viewModel = AdjustViewModel(service: master, adjust: StubAdjustService(), searchDebounce: .zero)
        await viewModel.load()

        await viewModel.selectOwnSpecies(key: Mega.megaOfficial.key)

        XCTAssertEqual(viewModel.ownItemLock, .locked(itemId: Self.official.id, displayName: Self.officialName))
        XCTAssertEqual(viewModel.itemLabel(for: Self.official.id), Self.officialName)
    }

    // MARK: - 構築

    private func loadedTeam(member: TeamMember) async -> TeamEditViewModel {
        let team = Team(id: "team-official", name: "テスト正式名称チーム", members: [member])
        let viewModel = TeamEditViewModel(store: StubTeamStore(), service: Mega.makeOfficialService(), team: team)
        await viewModel.load()
        return viewModel
    }

    func testTeamMemberShowsOfficialNameAndKeepsId() async throws {
        let plain = TeamMember(id: "m-1", speciesKey: StubMaster.alpha.key, itemId: Mega.both.id, natureId: "stub-nature-neutral")
        let viewModel = await loadedTeam(member: plain)

        await viewModel.setMemberSpecies(id: plain.id, speciesKey: Mega.megaOfficial.key)

        XCTAssertEqual(viewModel.itemLock(forMember: plain.id), .locked(itemId: Self.official.id, displayName: Self.officialName))
        XCTAssertEqual(viewModel.itemLabel(for: Self.official.id), Self.officialName)
        XCTAssertEqual(viewModel.team.members.first?.itemId, Self.official.id, "保存される値は ID のまま")
    }

    func testTeamCorrectionNoticeUsesOfficialName() async {
        let saved = TeamMember(id: "m-saved", speciesKey: Mega.megaOfficial.key, itemId: Mega.both.id, natureId: "stub-nature-neutral")
        let viewModel = await loadedTeam(member: saved)

        XCTAssertEqual(
            viewModel.itemNotice(forMember: saved.id),
            "メガシンカのため持ち物を\(Self.officialName)に直しました。保存すると反映されます")
    }

    func testTeamNonMegaWithOfficialStoneShowsOfficialName() async {
        let nonMega = TeamMember(id: "m-non", speciesKey: StubMaster.alpha.key, itemId: Self.official.id, natureId: "stub-nature-neutral")
        let viewModel = await loadedTeam(member: nonMega)

        XCTAssertEqual(viewModel.itemLabel(for: Self.official.id), Self.officialName)
        XCTAssertEqual(viewModel.team.members.first?.itemId, Self.official.id)
    }
}
