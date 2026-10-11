import XCTest

@testable import PokeCalcCore

/// `SpeedMatchupViewModel` の入力と要求の組み立て(I-speed-4c。ADR-0610 §4): 自分・相手・場の入力が、
/// matchup / reach の要求と画面の派生(連続技へ渡す値・ピッカーの絞り込み)になる規則。
/// 通信を伴う状態遷移(読み込み中・失敗・キャンセル)は `SpeedMatchupViewModelStateTests`。
@MainActor
final class SpeedMatchupViewModelInputTests: XCTestCase {
    private let pokemonA = StubSpeed.pokemonA.pokemonId
    private let pokemonB = StubSpeed.pokemonB.pokemonId
    private let pokemonD = StubSpeed.pokemonD.pokemonId

    private func makeViewModel(
        _ stub: StubSpeedService = StubSpeedService(), comboAvailable: Bool = false
    ) -> SpeedMatchupViewModel {
        SpeedMatchupViewModel(service: stub, debounce: .zero, comboAvailable: comboAvailable)
    }

    private func loaded(comboAvailable: Bool = false) async -> (SpeedMatchupViewModel, StubSpeedService) {
        let stub = StubSpeedService()
        let viewModel = makeViewModel(stub, comboAvailable: comboAvailable)
        await viewModel.load()
        return (viewModel, stub)
    }

    // MARK: - 初期状態

    func testInitialStateMatchesTheDocumentedDefaults() {
        let viewModel = makeViewModel()
        XCTAssertEqual(viewModel.pokemonState, .loading)
        XCTAssertEqual(viewModel.matchupState, .idle)
        XCTAssertEqual(viewModel.reachState, .idle)
        XCTAssertNil(viewModel.selfPokemonID)
        XCTAssertNil(viewModel.opponentPokemonID)
        XCTAssertEqual(viewModel.rank, 0)
        XCTAssertFalse(viewModel.scarf)
        XCTAssertFalse(viewModel.tailwind)
        XCTAssertFalse(viewModel.paralysis)
        XCTAssertNil(viewModel.current)
        XCTAssertFalse(viewModel.opponentTailwind)
        XCTAssertFalse(viewModel.opponentParalysis)
        XCTAssertFalse(viewModel.trickRoom)
        XCTAssertNil(viewModel.customAssumption)
        XCTAssertEqual(viewModel.selectedPresets, [.uninvested, .neutralMax, .max, .maxScarf], "既定の想定は 4 つ(ADR-0610 §2)")
        XCTAssertNil(viewModel.matchupRequest)
        XCTAssertNil(viewModel.reachRequest)
        XCTAssertEqual(viewModel.matchupCards, [])
        XCTAssertEqual(viewModel.reachRows, [])
    }

    func testLoadFetchesThePokemonListOnceAndNothingElse() async {
        let (viewModel, stub) = await loaded()
        XCTAssertEqual(viewModel.pokemonState, .loaded(StubSpeed.pokemonList.pokemon))
        let counts = (await stub.pokemonCalls, await stub.matchupCalls.count, await stub.reachCalls.count)
        XCTAssertEqual(counts.0, 1)
        XCTAssertEqual(counts.1, 0, "ポケモンを選ぶまで matchup は呼ばない")
        XCTAssertEqual(counts.2, 0, "今の振りを入れるまで reach は呼ばない")
        let tableCalls = await stub.tableCalls.count
        XCTAssertEqual(tableCalls, 0, "表は画面から外れた。table は呼ばない(ADR-0610 §4)")
    }

    func testLoadFailureShowsTheJapaneseMessageAndKeepsTheInputs() async {
        let stub = StubSpeedService()
        await stub.setPokemonResult(.failure(StubSpeed.failure))
        let viewModel = makeViewModel(stub)
        await viewModel.load()
        XCTAssertEqual(viewModel.pokemonState, .failed(SpeedFailure(code: "master_unavailable")))
        XCTAssertEqual(viewModel.filteredPokemon, [])
        viewModel.setScarf(true)
        XCTAssertTrue(viewModel.scarf, "一覧の失敗は他の入力を壊さない")
    }

    // MARK: - matchup の要求

    func testMatchupRequestNeedsBothPokemon() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        XCTAssertNil(viewModel.matchupRequest, "相手が未選択")
        viewModel.selectOpponentPokemon(id: pokemonD)
        XCTAssertNotNil(viewModel.matchupRequest)
        viewModel.selectSelfPokemon(id: nil)
        XCTAssertNil(viewModel.matchupRequest, "自分を外すと送れない")
    }

    func testMatchupRequestCarriesEveryInput() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.selectOpponentPokemon(id: pokemonD)
        viewModel.setRank(2)
        viewModel.setScarf(true)
        viewModel.setTailwind(true)
        viewModel.setParalysis(true)
        viewModel.setCurrentEnabled(true)
        viewModel.setCurrentSP(20)
        viewModel.setCurrentNature(.plus)
        viewModel.setOpponentTailwind(true)
        viewModel.setOpponentParalysis(true)
        viewModel.setTrickRoom(true)
        await viewModel.settle()

        XCTAssertEqual(
            viewModel.matchupRequest,
            SpeedMatchupRequest(
                own: .init(
                    pokemonId: pokemonA, rank: 2, scarf: true, tailwind: true, paralysis: true,
                    current: SpeedInvestment(sp: 20, nature: .plus)),
                opponent: .init(
                    pokemonId: pokemonD,
                    assumptions: [.preset(.uninvested), .preset(.neutralMax), .preset(.max), .preset(.maxScarf)],
                    tailwind: true, paralysis: true),
                trickRoom: true))
    }

    /// 想定は常に明示して送る(応答の行とチップを対応づけるため)。順は契約のプリセットの順で、カスタムは最後。
    func testAssumptionsAreSentInContractOrderWithCustomLast() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.selectOpponentPokemon(id: pokemonD)
        viewModel.togglePreset(.maxPlus2)
        viewModel.togglePreset(.maxPlus1)
        viewModel.togglePreset(.uninvested)
        let custom = SpeedCustomAssumption(sp: 12, nature: .minus, rank: -1, scarf: false)
        viewModel.setCustomAssumption(custom)
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.matchupRequest?.opponent.assumptions,
            [.preset(.neutralMax), .preset(.max), .preset(.maxScarf), .preset(.maxPlus1), .preset(.maxPlus2), .custom(custom)])
    }

    // MARK: - 想定のチップ

    func testTogglingAPresetOffAndOn() async {
        let (viewModel, _) = await loaded()
        viewModel.togglePreset(.max)
        XCTAssertEqual(viewModel.selectedPresets, [.uninvested, .neutralMax, .maxScarf])
        viewModel.togglePreset(.max)
        XCTAssertEqual(viewModel.selectedPresets, [.uninvested, .neutralMax, .max, .maxScarf])
    }

    func testTheLastAssumptionCannotBeTurnedOff() async {
        let (viewModel, _) = await loaded()
        for id in [SpeedPresetID.uninvested, .neutralMax, .max] { viewModel.togglePreset(id) }
        XCTAssertEqual(viewModel.selectedPresets, [.maxScarf])
        XCTAssertTrue(viewModel.assumptionMinimumNoticeVisible, "1 つだけ残ったら説明を出す")
        viewModel.togglePreset(.maxScarf)
        XCTAssertEqual(viewModel.selectedPresets, [.maxScarf], "最後の 1 つは外せない(契約上 assumptions は 1 つ以上)")
        viewModel.togglePreset(.max)
        XCTAssertFalse(viewModel.assumptionMinimumNoticeVisible)
    }

    func testCustomAssumptionCountsAsAnAssumptionWhenTurningThePresetsOff() async {
        let (viewModel, _) = await loaded()
        for id in [SpeedPresetID.uninvested, .neutralMax, .max] { viewModel.togglePreset(id) }
        viewModel.setCustomAssumption(SpeedCustomAssumption(sp: 4, nature: .neutral))
        viewModel.togglePreset(.maxScarf)
        XCTAssertEqual(viewModel.selectedPresets, [], "カスタムがあれば、プリセットを全部外せる")
        XCTAssertFalse(viewModel.assumptionMinimumNoticeVisible, "カスタムが残っているので 1 つ以上ある")
        XCTAssertEqual(viewModel.customAssumption, SpeedCustomAssumption(sp: 4, nature: .neutral))

        viewModel.setCustomAssumption(nil)
        XCTAssertNotNil(viewModel.customAssumption, "チップが 0 個になるなら、カスタムは外せない")
        viewModel.togglePreset(.max)
        viewModel.setCustomAssumption(nil)
        XCTAssertNil(viewModel.customAssumption, "他のチップがあれば外せる")
    }

    func testCustomAssumptionValuesAreClamped() async {
        let (viewModel, _) = await loaded()
        viewModel.setCustomAssumption(SpeedCustomAssumption(sp: 99, nature: .plus, rank: 9, scarf: true))
        XCTAssertEqual(viewModel.customAssumption, SpeedCustomAssumption(sp: SPLimits.maxPerStat, nature: .plus, rank: RankLimits.max, scarf: true))
        viewModel.setCustomAssumption(SpeedCustomAssumption(sp: -3, nature: .minus, rank: -9))
        XCTAssertEqual(viewModel.customAssumption, SpeedCustomAssumption(sp: 0, nature: .minus, rank: RankLimits.min))
    }

    // MARK: - 範囲

    func testRankIsClampedToTheContractRange() async {
        let (viewModel, _) = await loaded()
        viewModel.setRank(9)
        XCTAssertEqual(viewModel.rank, RankLimits.max)
        viewModel.setRank(-9)
        XCTAssertEqual(viewModel.rank, RankLimits.min)
        viewModel.setRank(3)
        XCTAssertEqual(viewModel.rank, 3)
    }

    // MARK: - 今の振り

    func testCurrentInvestmentStartsFromUninvestedAndIsClamped() async {
        let (viewModel, _) = await loaded()
        XCTAssertNil(viewModel.current)
        viewModel.setCurrentEnabled(true)
        XCTAssertEqual(viewModel.current, SpeedInvestment(sp: 0, nature: .neutral))
        viewModel.setCurrentSP(40)
        XCTAssertEqual(viewModel.current?.sp, SPLimits.maxPerStat)
        viewModel.setCurrentSP(-1)
        XCTAssertEqual(viewModel.current?.sp, 0)
        viewModel.setCurrentNature(.plus)
        XCTAssertEqual(viewModel.current, SpeedInvestment(sp: 0, nature: .plus))
        viewModel.setCurrentEnabled(false)
        XCTAssertNil(viewModel.current, "外すと今の振りは無くなる(行動順のバッジも「これ以上」も出なくなる)")
    }

    func testCurrentSetterWithoutCurrentDoNothing() async {
        let (viewModel, _) = await loaded()
        viewModel.setCurrentSP(10)
        viewModel.setCurrentNature(.plus)
        XCTAssertNil(viewModel.current, "今の振りを入れていないときは、SP・性格の操作で勝手に入れない")
    }

    func testCurrentPresetsMapToTheTableRows() async {
        let (viewModel, _) = await loaded()
        viewModel.applyCurrentPreset(.max)
        XCTAssertEqual(viewModel.current, SpeedInvestment(sp: SPLimits.maxPerStat, nature: .plus), "入れていなければ入れる")
        viewModel.applyCurrentPreset(.neutralMax)
        XCTAssertEqual(viewModel.current, SpeedInvestment(sp: SPLimits.maxPerStat, nature: .neutral))
        viewModel.applyCurrentPreset(.uninvested)
        XCTAssertEqual(viewModel.current, SpeedInvestment(sp: 0, nature: .neutral))
    }

    // MARK: - reach の要求

    func testReachRequestNeedsSelfAndCurrentButNotTheOpponent() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        XCTAssertNil(viewModel.reachRequest, "今の振りが無ければ自分の素早さが決まらない")
        viewModel.applyCurrentPreset(.max)
        XCTAssertNotNil(viewModel.reachRequest, "「これ以上」は相手を選ばなくても出る(ADR-0610 §4)")
        viewModel.selectSelfPokemon(id: nil)
        XCTAssertNil(viewModel.reachRequest)
    }

    func testReachRequestIsACustomPositionWithTheOpponentTailwindAsTheTableSide() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.applyCurrentPreset(.max)
        viewModel.setRank(1)
        viewModel.setScarf(true)
        viewModel.setTailwind(true)
        viewModel.setParalysis(true)
        viewModel.setOpponentTailwind(true)
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.reachRequest,
            SpeedPositionRequest(
                input: .custom(
                    pokemonId: pokemonA, sp: SPLimits.maxPerStat, nature: .plus, rank: 1, scarf: true, tailwind: true,
                    paralysis: true),
                tableTailwind: true),
            "契約: tableTailwind は表のプリセット側(= 相手側)の追い風")
    }

    func testReachRequestIgnoresTheOpponentPokemonTrickRoomAndOpponentParalysis() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.applyCurrentPreset(.neutralMax)
        let before = viewModel.reachRequest
        viewModel.selectOpponentPokemon(id: pokemonD)
        viewModel.setTrickRoom(true)
        viewModel.setOpponentParalysis(true)
        await viewModel.settle()
        XCTAssertEqual(viewModel.reachRequest, before)
    }

    // MARK: - 連続技へのリンク

    func testComboHandoffIsNilWhileTheComboIsNotAvailable() async {
        let (viewModel, _) = await loaded()
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.selectOpponentPokemon(id: pokemonD)
        XCTAssertFalse(viewModel.comboAvailable, "連続技が計算画面に入るまで既定は false")
        XCTAssertNil(viewModel.comboHandoff, "連続技が未実装の間はリンクを出さない")
    }

    func testComboHandoffNeedsBothPokemonAndCarriesTheAttackerInputs() async {
        let (viewModel, _) = await loaded(comboAvailable: true)
        XCTAssertNil(viewModel.comboHandoff)
        viewModel.selectSelfPokemon(id: pokemonA)
        XCTAssertNil(viewModel.comboHandoff, "相手が未選択")
        viewModel.selectOpponentPokemon(id: pokemonD)
        XCTAssertEqual(
            viewModel.comboHandoff,
            SpeedComboHandoff(
                attackerPokemonID: pokemonA, defenderPokemonID: pokemonD, attackerSpeed: nil, attackerRank: 0, attackerScarf: false),
            "ダメージ・確定数は素早さ画面に出さない。ポケモンを渡すだけ")
        viewModel.applyCurrentPreset(.max)
        viewModel.setRank(-1)
        viewModel.setScarf(true)
        XCTAssertEqual(
            viewModel.comboHandoff,
            SpeedComboHandoff(
                attackerPokemonID: pokemonA, defenderPokemonID: pokemonD,
                attackerSpeed: SpeedInvestment(sp: SPLimits.maxPerStat, nature: .plus), attackerRank: -1, attackerScarf: true),
            "今の振りがあれば素早さの SP・性格・ランク・スカーフも渡す(ADR-0610 §6 Q7 の推奨)")
        await viewModel.settle()
    }

    // MARK: - ポケモンの選択

    func testSelectedPokemonAreLookedUpFromTheLoadedList() async {
        let (viewModel, _) = await loaded()
        XCTAssertNil(viewModel.selfPokemon)
        viewModel.selectSelfPokemon(id: pokemonA)
        viewModel.selectOpponentPokemon(id: pokemonD)
        XCTAssertEqual(viewModel.selfPokemon, StubSpeed.pokemonA)
        XCTAssertEqual(viewModel.opponentPokemon, StubSpeed.pokemonD)
        viewModel.selectOpponentPokemon(id: nil)
        XCTAssertNil(viewModel.opponentPokemon)
        await viewModel.settle()
    }

    func testPickerFiltersByNameAndTreatsHiraganaAsKatakana() async {
        let (viewModel, _) = await loaded()
        XCTAssertEqual(viewModel.filteredPokemon, StubSpeed.pokemonList.pokemon, "空の検索は全件")
        viewModel.setPokemonQuery("ミズ")
        XCTAssertEqual(viewModel.filteredPokemon, [StubSpeed.pokemonB])
        viewModel.setPokemonQuery("  みず ")
        XCTAssertEqual(viewModel.filteredPokemon, [StubSpeed.pokemonB], "ひらがなはカタカナとして照合し、前後の空白は無視する")
        viewModel.setPokemonQuery("該当なし")
        XCTAssertEqual(viewModel.filteredPokemon, [])
    }
}
