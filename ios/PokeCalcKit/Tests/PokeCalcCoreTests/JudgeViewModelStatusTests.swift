import XCTest

@testable import PokeCalcCore

/// 判定画面の状態異常の入力(ADR-0512)。選択は自分・候補ごと。要求への載せ方は ranks と同じ省略の流儀:
/// `none`(既定)は載せない・選んだ値だけ契約の値のまま送る。既存の要求の本文は変えない。
@MainActor
final class JudgeViewModelStatusTests: XCTestCase {
    private typealias H = JudgeHarness

    // MARK: - 既定・型

    func testStatusDefaultsToNoneEverywhere() {
        XCTAssertEqual(JudgeDraft().status, .none)
        let viewModel = H.makeViewModel()
        XCTAssertEqual(viewModel.attacker.status, .none)
        XCTAssertEqual(viewModel.candidates.map(\.status), [.none])
        XCTAssertNil(JudgeIndividual(speciesKey: "k", natureId: "n", sp: zeroSP).status, "個体の status の既定は nil(送らない)")
    }

    func testJudgeStatusHasTheSevenContractValuesInOrder() {
        XCTAssertEqual(
            JudgeStatus.allCases.map(\.rawValue),
            ["none", "burn", "paralysis", "poison", "badly_poison", "sleep", "freeze"], "表示順は Web の JUDGE_STATUS_KEYS と同じ(none が先頭)")
    }

    // MARK: - 選択

    func testSetStatusChangesOnlyTheTargetAndLeavesOtherFieldsAlone() {
        let viewModel = H.makeViewModel()
        viewModel.addCandidate()
        viewModel.setRank(.spe, 2, for: .attacker)
        viewModel.setStatus(.paralysis, for: .candidate(1))
        XCTAssertEqual(viewModel.candidates[1].status, .paralysis)
        XCTAssertEqual(viewModel.candidates[0].status, .none, "他の候補に波及しない")
        XCTAssertEqual(viewModel.attacker.status, .none, "自分に波及しない")
        XCTAssertEqual(viewModel.attacker.ranks.spe, 2)
        viewModel.setStatus(.burn, for: .attacker)
        XCTAssertEqual(viewModel.attacker.status, .burn)
        XCTAssertEqual(viewModel.candidates[1].status, .paralysis)
        viewModel.setStatus(.none, for: .attacker)
        XCTAssertEqual(viewModel.attacker.status, .none, "なし に戻せる")
    }

    func testSetStatusOnAnOutOfRangeCandidateIsIgnored() {
        let viewModel = H.makeViewModel()
        viewModel.setStatus(.sleep, for: .candidate(5))
        XCTAssertEqual(viewModel.candidates.map(\.status), [.none])
    }

    func testEditingTheStatusNeverCallsTheJudgeService() async {
        let service = StubJudgeService()
        let viewModel = await H.makeFilled(service: service)
        viewModel.setStatus(.freeze, for: .attacker)
        await viewModel.settle()
        let requests = await service.requests
        XCTAssertTrue(requests.isEmpty, "送信ボタンを押すまで judge を呼ばない(ADR-0705 §7)")
    }

    // MARK: - リセットの規則

    func testChangingSpeciesKeepsTheStatus() async {
        let viewModel = H.makeViewModel()
        viewModel.setStatus(.poison, for: .attacker)
        await viewModel.setSpecies(H.alpha, for: .attacker)
        await viewModel.setSpecies(H.beta, for: .attacker)
        XCTAssertEqual(viewModel.attacker.status, .poison, "状態異常は種族を変えても残す(ランクと同じ)")
    }

    func testRemovingACandidateMovesItsStatusWithIt() {
        let viewModel = H.makeViewModel()
        viewModel.addCandidate()
        viewModel.addCandidate()
        viewModel.setStatus(.burn, for: .candidate(0))
        viewModel.setStatus(.sleep, for: .candidate(2))
        XCTAssertTrue(viewModel.removeCandidate(at: 1))
        XCTAssertEqual(viewModel.candidates.map(\.status), [.burn, .sleep], "状態異常は候補について行く(位置に貼り付かない)")
    }

    func testApplyingATeamMemberResetsTheStatusToNone() async {
        let viewModel = H.makeViewModel(store: StubTeams.makeStore())
        await viewModel.load()
        viewModel.setStatus(.paralysis, for: .attacker)
        await viewModel.applyTeamMember(teamID: StubTeams.teamAlpha.id, memberID: StubTeams.namedMember.id, to: .attacker)
        XCTAssertEqual(viewModel.attacker.status, .none, "構築に状態異常は無いので、ランクと同じく既定へ戻す(前の値を引きずらない)")
    }

    // MARK: - 要求(省略の規則)

    func testNoneIsNeverSentAndTheRequestIsUnchanged() async throws {
        let viewModel = await H.makeFilled()
        var request = try H.request(viewModel)
        XCTAssertNil(request.attacker.status)
        XCTAssertNil(request.defenders[0].individual.status)
        viewModel.setStatus(.paralysis, for: .attacker)
        viewModel.setStatus(.none, for: .attacker)
        request = try H.request(viewModel)
        XCTAssertNil(request.attacker.status, "選び直して none に戻したら送らない")
        XCTAssertEqual(
            request,
            JudgeRequest(
                attacker: JudgeIndividual(speciesKey: StubMaster.alpha.key, natureId: StubMaster.neutralNature.id, sp: zeroSP),
                moveId: StubMaster.physicalMove.id,
                defenders: [
                    JudgeDefender(
                        individual: JudgeIndividual(speciesKey: StubMaster.beta.key, natureId: StubMaster.neutralNature.id, sp: zeroSP),
                        moveId: StubMaster.specialMove.id)
                ]), "状態異常を選ばなければ従来と同じ要求")
    }

    func testEverySelectedStatusIsSentAsTheContractValueOnlyForItsOwnSide() async throws {
        let viewModel = await H.makeFilled()
        for status in JudgeStatus.allCases where status != .none {
            viewModel.setStatus(status, for: .attacker)
            var request = try H.request(viewModel)
            XCTAssertEqual(request.attacker.status, status)
            XCTAssertNil(request.defenders[0].individual.status, "自分の状態異常を候補に載せない")
            viewModel.setStatus(.none, for: .attacker)
            viewModel.setStatus(status, for: .candidate(0))
            request = try H.request(viewModel)
            XCTAssertEqual(request.defenders[0].individual.status, status)
            XCTAssertNil(request.attacker.status, "候補の状態異常を自分に載せない")
            viewModel.setStatus(.none, for: .candidate(0))
        }
    }

    func testStatusDoesNotChangeTheValidationAndOtherOmissions() async throws {
        let viewModel = await H.makeFilled()
        viewModel.setStatus(.paralysis, for: .attacker)
        let request = try H.request(viewModel)
        XCTAssertNil(request.attacker.ranks)
        XCTAssertNil(request.speedField)
        let empty = H.makeViewModel()
        empty.setStatus(.paralysis, for: .attacker)
        XCTAssertEqual(H.violation(empty), .requiredMissing(.attacker), "状態異常だけでは送れない(検査は変わらない)")
    }
}
