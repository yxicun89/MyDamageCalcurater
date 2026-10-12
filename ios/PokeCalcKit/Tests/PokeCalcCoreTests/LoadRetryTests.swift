import XCTest

@testable import PokeCalcCore

/// 最初の読み込みに失敗した画面の「もう一度」(I-ios-19。監査 3.3 の 8 件目)。
/// View のボタンは `canRetryLoad` が true のときだけ出し、押すと `retryLoad()` を呼ぶ。
/// 読み込み済みの `load()`(2 回目以降は何もしない)とは別に、失敗後だけやり直せる。
@MainActor
final class LoadRetryTests: XCTestCase {

    private let offline = PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト")

    func testCalcRetryRecoversAfterMasterFailure() async {
        let stub = StubMaster.makeService()
        await stub.setMasterError(offline)
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        XCTAssertNotNil(viewModel.error)
        XCTAssertTrue(viewModel.canRetryLoad)
        XCTAssertTrue(viewModel.rows.isEmpty)

        await stub.setMasterError(nil)
        await viewModel.retryLoad()
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.canRetryLoad)
        XCTAssertFalse(viewModel.rows.isEmpty, "やり直しで既定の計算まで進む")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testCalcRetryIsNotOfferedAfterSuccessOrForCalculationFailures() async {
        let stub = StubMaster.makeService()
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        XCTAssertFalse(viewModel.canRetryLoad, "成功後は出さない")
        let before = await stub.bulkRequests.count
        await viewModel.retryLoad()
        let after = await stub.bulkRequests.count
        XCTAssertEqual(before, after, "読み込めているときの retryLoad は何もしない(二重に計算しない)")
    }

    func testReverseRetryRecoversAfterMasterFailure() async {
        let stub = StubMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        await stub.setMasterError(offline)
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        XCTAssertNotNil(viewModel.error)
        XCTAssertTrue(viewModel.canRetryLoad)

        await stub.setMasterError(nil)
        await viewModel.retryLoad()
        XCTAssertNil(viewModel.error)
        XCTAssertFalse(viewModel.canRetryLoad)
        XCTAssertFalse(viewModel.speciesOptions.isEmpty)
    }

    func testBalanceMasterFailureCanRetry() async {
        let master = StubMaster.makeService()
        await master.setMasterError(offline)
        let viewModel = BalanceViewModel(balance: StubBalanceService(), master: master, refreshDebounce: .zero)
        await viewModel.load()
        XCTAssertNotNil(viewModel.masterError)
        XCTAssertTrue(viewModel.canRetryLoad)

        await master.setMasterError(nil)
        await viewModel.retryLoad()
        XCTAssertNil(viewModel.masterError)
        XCTAssertFalse(viewModel.canRetryLoad)
        XCTAssertFalse(viewModel.speciesOptions.isEmpty)
    }

    func testBalanceMasterFailureMessageNamesPokedexNotBalanceServer() async {
        // ポケモン・技の一覧(マスタ側のサーバー)の失敗を、タイプバランスのサーバーの障害と取り違えない(13 件の 6)。
        let master = StubMaster.makeService()
        await master.setMasterError(PokeCalcError(code: "internal_error", message: "x", statusCode: 500))
        let viewModel = BalanceViewModel(balance: StubBalanceService(), master: master, refreshDebounce: .zero)
        await viewModel.load()
        let message = viewModel.masterError?.message ?? ""
        XCTAssertFalse(message.contains("タイプバランス"), "一覧の失敗にタイプバランスの名前を付けない: \(message)")
        XCTAssertEqual(message, ServiceFailureKind.serverFault.message(feature: "ポケモンの一覧"))
    }

    func testSpeedPokemonListFailureCanRetry() async {
        let stub = StubSpeedService()
        await stub.setPokemonResult(.failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "x")))
        let viewModel = SpeedMatchupViewModel(service: stub, debounce: .zero)
        await viewModel.load()
        XCTAssertTrue(viewModel.canRetryLoad)

        await stub.setPokemonResult(.success(StubSpeed.pokemonList))
        await viewModel.retryLoad()
        XCTAssertFalse(viewModel.canRetryLoad)
        XCTAssertEqual(viewModel.pokemonState, .loaded(StubSpeed.pokemonList.pokemon))
    }
}
