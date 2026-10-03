import XCTest

@testable import PokeCalcCore

/// `SpeedViewModel` の入力 → 要求(P6-24。ADR-0503 §4・§5)。Web の `SpeedScreen`(buildPositionRequest)と同じ規則。
/// サービスはテスト内のスタブ(モックの数値に依存しない)。待ち時間に依存しないよう debounce は `.zero` を注入する。
@MainActor
final class SpeedViewModelInputTests: XCTestCase {
    private func makeViewModel(_ stub: StubSpeedService = StubSpeedService()) -> (SpeedViewModel, StubSpeedService) {
        (SpeedViewModel(service: stub, debounce: .zero), stub)
    }

    private func loaded() async -> (SpeedViewModel, StubSpeedService) {
        let (viewModel, stub) = makeViewModel()
        await viewModel.load()
        return (viewModel, stub)
    }

    private let pokemonA = StubSpeed.pokemonA.pokemonId

    // MARK: - 初期値と読み込み

    /// Web と同じ既定: preset・最速・スカーフ無し・ポケモン未選択・表は全6行・場の状態は off。
    func testInitialState() {
        let (viewModel, _) = makeViewModel()
        XCTAssertEqual(viewModel.mode, .preset)
        XCTAssertEqual(viewModel.preset, .max)
        XCTAssertFalse(viewModel.scarf)
        XCTAssertFalse(viewModel.tailwind)
        XCTAssertFalse(viewModel.paralysis)
        XCTAssertNil(viewModel.selectedPokemonID)
        XCTAssertEqual(viewModel.sp, 0)
        XCTAssertEqual(viewModel.nature, .neutral)
        XCTAssertEqual(viewModel.rank, 0)
        XCTAssertEqual(viewModel.rawValueText, "")
        XCTAssertEqual(viewModel.selectedFilterPresets, Set(SpeedPresetID.allCases))
        XCTAssertFalse(viewModel.tableTailwind)
        XCTAssertFalse(viewModel.trickRoom)
        XCTAssertEqual(viewModel.pokemonState, .loading)
        XCTAssertEqual(viewModel.tableState, .loading)
        XCTAssertEqual(viewModel.positionState, .idle)
        XCTAssertNil(viewModel.positionRequest)
    }

    func testLoadFetchesPokemonAndTheFullTableOnce() async {
        let (viewModel, stub) = await loaded()
        let pokemonCalls = await stub.pokemonCalls
        let tableCalls = await stub.tableCalls
        let positionCalls = await stub.positionCalls
        XCTAssertEqual(pokemonCalls, 1)
        XCTAssertEqual(tableCalls, [StubSpeedService.TableCall(presets: nil, field: SpeedTableField())], "全6行は presets を省く")
        XCTAssertEqual(positionCalls, [], "ポケモンを選ぶまで位置は呼ばない")
        XCTAssertEqual(viewModel.pokemonState, .loaded(StubSpeed.pokemonList.pokemon))
        XCTAssertEqual(viewModel.tableState, .loaded(StubSpeed.table()))
        XCTAssertEqual(viewModel.positionState, .idle)
    }

    // MARK: - preset

    func testPresetRequestFromPokemonPresetAndFlags() async {
        let (viewModel, stub) = await loaded()
        viewModel.selectPokemon(id: pokemonA)
        await viewModel.settle()
        let expected = SpeedPositionRequest(
            input: .preset(pokemonId: pokemonA, preset: .max, scarf: false, tailwind: false, paralysis: false))
        XCTAssertEqual(viewModel.positionRequest, expected)
        let calls = await stub.positionCalls
        XCTAssertEqual(calls, [expected])

        viewModel.setPreset(.neutralMax)
        viewModel.setScarf(true)
        viewModel.setTailwind(true)
        viewModel.setParalysis(true)
        viewModel.setTableTailwind(true)
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.positionRequest,
            SpeedPositionRequest(
                input: .preset(pokemonId: pokemonA, preset: .neutralMax, scarf: true, tailwind: true, paralysis: true),
                tableTailwind: true))
        let last = await stub.positionCalls.last
        XCTAssertEqual(last, viewModel.positionRequest, "最後に送った要求は、今の入力から作った要求と同じ")
    }

    func testNoPositionRequestWhilePokemonIsUnselected() async {
        let (viewModel, stub) = await loaded()
        viewModel.setScarf(true)
        viewModel.setPreset(.uninvested)
        await viewModel.settle()
        XCTAssertNil(viewModel.positionRequest)
        XCTAssertEqual(viewModel.positionState, .idle)
        let calls = await stub.positionCalls
        XCTAssertEqual(calls, [])
    }

    func testPositionResponseIsShownAfterTheRequest() async {
        let (viewModel, stub) = await loaded()
        await stub.setPositionResponder { _, _ in .success(StubSpeed.position(speed: 301, pokemon: StubSpeed.pokemonA, faster: 12, slower: 30)) }
        viewModel.selectPokemon(id: pokemonA)
        XCTAssertEqual(viewModel.positionState, .loading, "要求を予約した時点で計算中にする(古い結果を出し続けない)")
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.positionState,
            .loaded(StubSpeed.position(speed: 301, pokemon: StubSpeed.pokemonA, faster: 12, slower: 30)))
    }

    // MARK: - custom

    func testCustomRequestHasSPNatureRankAndScarf() async {
        let (viewModel, _) = await loaded()
        viewModel.selectPokemon(id: pokemonA)
        viewModel.setMode(.custom)
        viewModel.setSP(20)
        viewModel.setNature(.plus)
        viewModel.setRank(-1)
        viewModel.setScarf(true)
        viewModel.setTailwind(true)
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.positionRequest,
            SpeedPositionRequest(
                input: .custom(pokemonId: pokemonA, sp: 20, nature: .plus, rank: -1, scarf: true, tailwind: true, paralysis: false)))
    }

    func testCustomSPAndRankAreClampedToTheDomainRange() {
        let (viewModel, _) = makeViewModel()
        viewModel.setSP(SPLimits.maxPerStat + 8)
        XCTAssertEqual(viewModel.sp, SPLimits.maxPerStat)
        viewModel.setSP(-3)
        XCTAssertEqual(viewModel.sp, 0)
        viewModel.setRank(RankLimits.max + 3)
        XCTAssertEqual(viewModel.rank, RankLimits.max)
        viewModel.setRank(RankLimits.min - 3)
        XCTAssertEqual(viewModel.rank, RankLimits.min)
    }

    func testCustomWithoutPokemonSendsNothing() async {
        let (viewModel, stub) = await loaded()
        viewModel.setMode(.custom)
        viewModel.setSP(32)
        await viewModel.settle()
        XCTAssertNil(viewModel.positionRequest)
        let calls = await stub.positionCalls
        XCTAssertEqual(calls, [])
    }

    // MARK: - raw

    func testRawRequestUsesTheValueAndOptionalPokemon() async {
        let (viewModel, _) = await loaded()
        viewModel.setMode(.raw)
        viewModel.setRawValueText("301")
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionRequest, SpeedPositionRequest(input: .raw(value: 301, pokemonId: nil)), "raw はポケモン未選択でも送れる")

        viewModel.selectPokemon(id: pokemonA)
        viewModel.setTableTailwind(true)
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.positionRequest,
            SpeedPositionRequest(input: .raw(value: 301, pokemonId: pokemonA), tableTailwind: true))
    }

    /// raw は補正済みの値を入れるので、自分の追い風・まひ・スカーフは要求に載らない(契約上 400)。
    func testRawRequestIgnoresSelfFlags() async {
        let (viewModel, _) = await loaded()
        viewModel.setMode(.raw)
        viewModel.setRawValueText("301")
        viewModel.setTailwind(true)
        viewModel.setParalysis(true)
        viewModel.setScarf(true)
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionRequest, SpeedPositionRequest(input: .raw(value: 301, pokemonId: nil)))
    }

    func testRawValueIsTrimmed() {
        let (viewModel, _) = makeViewModel()
        viewModel.setMode(.raw)
        viewModel.setRawValueText("  301 ")
        XCTAssertEqual(viewModel.positionRequest, SpeedPositionRequest(input: .raw(value: 301, pokemonId: nil)))
        XCTAssertNil(viewModel.rawValueError)
    }

    /// 空欄は「未入力」(送らず、エラーも出さない)。
    func testEmptyRawValueIsIdleWithoutError() async {
        let (viewModel, stub) = await loaded()
        viewModel.setMode(.raw)
        for text in ["", "   "] {
            viewModel.setRawValueText(text)
            await viewModel.settle()
            XCTAssertNil(viewModel.positionRequest, "「\(text)」")
            XCTAssertNil(viewModel.rawValueError, "「\(text)」")
            XCTAssertEqual(viewModel.positionState, .idle)
        }
        let calls = await stub.positionCalls
        XCTAssertEqual(calls, [])
    }

    /// 契約の下限(1)・整数だけを画面で止める。上限は speed サービスが式から導くので画面に複製しない(API の 400 を日本語にする)。
    func testInvalidRawValueShowsRangeMessageAndSendsNothing() async {
        let (viewModel, stub) = await loaded()
        viewModel.setMode(.raw)
        for text in ["0", "-5", "1.5", "abc", "30a", "99999999999999999999"] {
            viewModel.setRawValueText(text)
            await viewModel.settle()
            XCTAssertNil(viewModel.positionRequest, "「\(text)」")
            XCTAssertEqual(viewModel.rawValueError, SpeedLabels.rawValueRange, "「\(text)」")
            XCTAssertEqual(viewModel.positionState, .idle, "「\(text)」")
        }
        let calls = await stub.positionCalls
        XCTAssertEqual(calls, [])
    }

    func testRawValueAboveAnyDisplayedLimitIsStillSentToTheServer() async {
        let (viewModel, stub) = await loaded()
        viewModel.setMode(.raw)
        viewModel.setRawValueText("99999")
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionRequest, SpeedPositionRequest(input: .raw(value: 99999, pokemonId: nil)))
        let calls = await stub.positionCalls
        XCTAssertEqual(calls.count, 1, "上限の判定はサーバー(契約の説明)")
    }

    func testRawValueErrorIsOnlyForRawMode() {
        let (viewModel, _) = makeViewModel()
        viewModel.setRawValueText("abc")
        XCTAssertNil(viewModel.rawValueError, "preset モードでは実数値欄を使わないのでエラーにしない")
    }

    // MARK: - モード切り替え

    func testSwitchingModeRebuildsTheRequestFromTheNewMode() async {
        let (viewModel, _) = await loaded()
        viewModel.selectPokemon(id: pokemonA)
        viewModel.setRawValueText("250")
        await viewModel.settle()
        XCTAssertEqual(
            viewModel.positionRequest,
            SpeedPositionRequest(input: .preset(pokemonId: pokemonA, preset: .max, scarf: false, tailwind: false, paralysis: false)),
            "preset のあいだは実数値欄の文字は送らない")
        viewModel.setMode(.raw)
        await viewModel.settle()
        XCTAssertEqual(viewModel.positionRequest, SpeedPositionRequest(input: .raw(value: 250, pokemonId: pokemonA)))
        viewModel.selectPokemon(id: nil)
        viewModel.setMode(.custom)
        await viewModel.settle()
        XCTAssertNil(viewModel.positionRequest, "custom はポケモン未選択では送れない")
        XCTAssertEqual(viewModel.positionState, .idle)
    }
}
