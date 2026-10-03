import XCTest

@testable import PokeCalcCore

/// `AdjustViewModel*Tests` の共通の組み立て(AJ7)。
///
/// 架空マスタは `StubMaster`(自分 = beta〈ほのお。learnset: 変化・特殊ほのお・物理ノーマル〉、
/// 相手 = gamma〈みず。learnset: 物理・特殊〉)。性格は `StubMaster.reverseNatures`(攻撃・特攻・防御・特防の
/// 上昇性格と無補正)。技の実体は `moves(ids:)` で引くので `setMoveBatchMode(.immediate)` にする。
@MainActor
enum AdjustTestSupport {
    struct Fixture {
        let master: StubPokeCalcService
        let adjust: StubAdjustService
        let viewModel: AdjustViewModel
    }

    static func makeFixture(natures: [Nature] = StubMaster.reverseNatures) async -> Fixture {
        let master = StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly],
            natures: natures
        )
        await master.setMoveBatchMode(.immediate)
        let adjust = StubAdjustService()
        let viewModel = AdjustViewModel(service: master, adjust: adjust, searchDebounce: .zero)
        await viewModel.load()
        return Fixture(master: master, adjust: adjust, viewModel: viewModel)
    }

    /// 自分 = beta・無補正(送信前の検査の最低条件を満たす)。
    static func selectOwnBeta(_ viewModel: AdjustViewModel) async {
        await viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        viewModel.selectOwnNature(id: StubMaster.neutralNature.id)
    }

    /// 自分 = beta・無補正 + 相手 = gamma。
    static func selectOwnAndOpponent(_ viewModel: AdjustViewModel) async {
        await selectOwnBeta(viewModel)
        await viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
    }

    /// 送信前の検査で止まり、調整 API を1回も呼んでいないことを確かめる。
    static func assertRejected(
        _ fixture: Fixture, message expected: String, file: StaticString = #filePath, line: UInt = #line
    ) async {
        XCTAssertEqual(fixture.viewModel.alertMessage, expected, file: file, line: line)
        XCTAssertFalse(expected.isEmpty, "期待する文が空(AdjustText のスタブのまま)", file: file, line: line)
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls, [], "検査に違反したら API を呼ばない", file: file, line: line)
        XCTAssertNil(fixture.viewModel.outcome, file: file, line: line)
    }

    /// 架空の種族の要約(技を覚えるポケモンの一覧のページ用)。
    nonisolated static func learner(_ index: Int) -> SpeciesSummary {
        SpeciesSummary(key: "7\(index)-000", dexNo: 7000 + index, form: 0, nameJa: "テストまなぶ\(index)", types: [.normal])
    }

    nonisolated static func learners(_ range: Range<Int>) -> [SpeciesSummary] { range.map(learner) }

    nonisolated static let zero = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
}
