import XCTest

@testable import PokeCalcCore

/// 目標方式(F-11)の ViewModel テストの組み立て。
/// 架空マスタは `AdjustTestSupport` と同じ(自分 = beta〈learnset: 変化・特殊・物理〉、相手 = gamma〈物理・特殊〉)に、
/// 素早さ上昇の性格とメガ種族を足す。
@MainActor
enum AdjustGoalsTestSupport {
    static let speUpNature = Nature(id: "stub-nature-spe-up", nameJa: "テストせいかく素早さ上昇", plus: .spe, minus: .atk)

    struct Fixture {
        let master: StubPokeCalcService
        let adjust: StubAdjustService
        let goals: StubAdjustGoalsService
        let viewModel: AdjustViewModel
    }

    static func makeFixture(
        natures: [Nature] = StubMaster.reverseNatures + [speUpNature], withGoalsService: Bool = true
    ) async -> Fixture {
        let master = StubMegaMaster.makeService(natures: natures)
        await master.setMoveBatchMode(.immediate)
        let adjust = StubAdjustService()
        let goals = StubAdjustGoalsService()
        let viewModel = AdjustViewModel(
            service: master, adjust: adjust, goals: withGoalsService ? goals : nil, searchDebounce: .zero)
        await viewModel.load()
        return Fixture(master: master, adjust: adjust, goals: goals, viewModel: viewModel)
    }

    /// 自分 = beta・無補正。目標方式を選ぶ。
    static func selectOwnAndGoalsMode(_ viewModel: AdjustViewModel) async {
        await AdjustTestSupport.selectOwnBeta(viewModel)
        viewModel.selectGoalsMode()
    }

    /// 目標を1件足して返す(id)。
    @discardableResult
    static func addGoal(_ viewModel: AdjustViewModel, kind: AdjustGoalKind = .outspeed, opponent: String? = nil) async -> Int {
        let id = viewModel.addGoal()!
        viewModel.setGoalKind(id: id, kind)
        if let opponent { await viewModel.selectGoalOpponent(id: id, key: opponent) }
        return id
    }
}
