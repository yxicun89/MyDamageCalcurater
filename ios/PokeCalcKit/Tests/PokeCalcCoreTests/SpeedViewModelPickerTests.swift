import XCTest

@testable import PokeCalcCore

/// ポケモンの簡易ピッカー(P6-24。ADR-0503 §5)。pokedex の検索ではなく、speed の `/pokemon` の一覧を
/// 1回取ってクライアントで名前の絞り込みだけを行う(件数が多いので検索欄つき)。
@MainActor
final class SpeedViewModelPickerTests: XCTestCase {
    private func loaded() async -> SpeedViewModel {
        let viewModel = SpeedViewModel(service: StubSpeedService(), debounce: .zero)
        await viewModel.load()
        return viewModel
    }

    private func names(_ viewModel: SpeedViewModel) -> [String] { viewModel.filteredPokemon.map(\.nameJa) }

    func testEmptyQueryListsEverythingInServiceOrder() async {
        let viewModel = await loaded()
        XCTAssertEqual(viewModel.filteredPokemon, StubSpeed.pokemonList.pokemon)
    }

    func testNothingIsListedBeforeTheListIsLoaded() {
        let viewModel = SpeedViewModel(service: StubSpeedService(), debounce: .zero)
        XCTAssertEqual(viewModel.filteredPokemon, [])
    }

    func testQueryFiltersByNameSubstring() async {
        let viewModel = await loaded()
        viewModel.setPokemonQuery("ミズ")
        XCTAssertEqual(names(viewModel), ["テストミズガメ"])
        viewModel.setPokemonQuery("テスト")
        XCTAssertEqual(viewModel.filteredPokemon.count, 4)
        viewModel.setPokemonQuery("ネズミ")
        XCTAssertEqual(names(viewModel), ["テストデンキネズミ"], "名前の途中でも一致する")
    }

    func testHiraganaQueryMatchesKatakanaNames() async {
        let viewModel = await loaded()
        viewModel.setPokemonQuery("みず")
        XCTAssertEqual(names(viewModel), ["テストミズガメ"])
    }

    func testQueryIsTrimmed() async {
        let viewModel = await loaded()
        viewModel.setPokemonQuery("  くさ ")
        XCTAssertEqual(names(viewModel), ["テストクサネコ"])
    }

    func testNoMatchGivesAnEmptyList() async {
        let viewModel = await loaded()
        viewModel.setPokemonQuery("ない名前")
        XCTAssertEqual(viewModel.filteredPokemon, [])
    }

    /// 絞り込みは表示だけのもの。選択中のポケモンも、位置の要求も変えない。
    func testQueryDoesNotChangeTheSelectionOrRequestAnything() async {
        let stub = StubSpeedService()
        let viewModel = SpeedViewModel(service: stub, debounce: .zero)
        await viewModel.load()
        viewModel.selectPokemon(id: StubSpeed.pokemonA.pokemonId)
        await viewModel.settle()
        let before = await stub.positionCalls.count
        viewModel.setPokemonQuery("ミズ")
        await viewModel.settle()
        XCTAssertEqual(viewModel.selectedPokemonID, StubSpeed.pokemonA.pokemonId)
        XCTAssertEqual(viewModel.selectedPokemon, StubSpeed.pokemonA, "絞り込みの外でも選択中は解決できる")
        let after = await stub.positionCalls.count
        XCTAssertEqual(after, before)
        XCTAssertEqual(viewModel.pokemonQuery, "ミズ")
    }

    func testSelectedPokemonIsResolvedFromTheLoadedList() async {
        let viewModel = await loaded()
        XCTAssertNil(viewModel.selectedPokemon)
        viewModel.selectPokemon(id: "9003-000")
        XCTAssertEqual(viewModel.selectedPokemon, StubSpeed.pokemonC)
        viewModel.selectPokemon(id: "0000-000")
        XCTAssertNil(viewModel.selectedPokemon, "一覧に無い ID は解決できない")
        viewModel.selectPokemon(id: nil)
        XCTAssertNil(viewModel.selectedPokemon)
    }
}
