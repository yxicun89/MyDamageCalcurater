import Foundation
import XCTest

@testable import PokeCalcCore

/// `JudgeViewModel` のテストの共通の組み立て(架空マスタは `StubMaster`・`StubTeams`)。
/// 既定のマスタの性格は [攻撃上昇, 無補正, 特攻上昇](`StubMaster.makeService`)。`load()` が入れる既定の性格は「補正なしの最初」= `StubMaster.neutralNature`。
@MainActor
enum JudgeHarness {
    static let alpha = SpeciesSummary(detail: StubMaster.alpha)
    static let beta = SpeciesSummary(detail: StubMaster.beta)
    static let gamma = SpeciesSummary(detail: StubMaster.gamma)

    /// テスト用の master に載せる種族(`StubMaster.makeService` の既定に加えて特性違いも引けるようにする)。
    static func makeMaster(natures: [Nature]? = nil) -> StubPokeCalcService {
        StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.abilityXOnly, StubMaster.abilityYOnly, StubMaster.abilityXAndY],
            natures: natures ?? [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature])
    }

    static func makeViewModel(
        service: StubJudgeService = StubJudgeService(), master: StubPokeCalcService = makeMaster(),
        store: (any TeamStore)? = nil
    ) -> JudgeViewModel {
        JudgeViewModel(service: service, master: master, teamStore: store, searchDebounce: .zero)
    }

    /// 自分(alpha・物理技)と候補1(beta・特殊技)を、そのまま送れる状態にする(性格は `load()` の既定)。
    static func makeFilled(
        service: StubJudgeService = StubJudgeService(), master: StubPokeCalcService = makeMaster(),
        store: (any TeamStore)? = nil
    ) async -> JudgeViewModel {
        let viewModel = makeViewModel(service: service, master: master, store: store)
        await viewModel.load()
        await viewModel.setSpecies(alpha, for: .attacker)
        viewModel.setMove(StubMaster.physicalMove, for: .attacker)
        await viewModel.setSpecies(beta, for: .candidate(0))
        viewModel.setMove(StubMaster.specialMove, for: .candidate(0))
        return viewModel
    }

    /// `makeRequest()` の成功値(失敗ならテストを失敗させる)。
    static func request(_ viewModel: JudgeViewModel, file: StaticString = #filePath, line: UInt = #line) throws -> JudgeRequest {
        switch viewModel.makeRequest() {
        case .success(let request): return request
        case .failure(let error):
            XCTFail("要求を作れない: \(error)", file: file, line: line)
            throw error
        }
    }

    /// `makeRequest()` の失敗値(成功ならテストを失敗させる)。
    static func violation(_ viewModel: JudgeViewModel, file: StaticString = #filePath, line: UInt = #line) -> JudgeValidationError? {
        switch viewModel.makeRequest() {
        case .success:
            XCTFail("検査に通ってしまった", file: file, line: line)
            return nil
        case .failure(let error):
            return error
        }
    }
}
