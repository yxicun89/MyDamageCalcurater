import XCTest

@testable import PokeCalcCore

// P6-20: 書き出し・取り込みシートの状態(`TeamTextTransferViewModel`。ADR-0501「P6-20」5章)。
// 取り込めなかった行があっても、取り込める分だけ追加するかを選べる(全か無かにしない)。
@MainActor
final class TeamTextTransferViewModelTests: XCTestCase {
    private func makeViewModel(_ stub: StubPokeCalcService = StubMaster.makeService()) -> TeamTextTransferViewModel {
        TeamTextTransferViewModel(service: stub)
    }

    private let member = TeamMember(id: "m1", speciesKey: "9101-000", natureId: "stub-nature-atk-up")

    // MARK: 書き出し

    func testInitialState() {
        let vm = makeViewModel()
        XCTAssertEqual(vm.phase, .idle)
        XCTAssertEqual(vm.pastedText, "")
        XCTAssertNil(vm.exportText)
        XCTAssertNil(vm.exportError)
        XCTAssertFalse(vm.isExporting)
        XCTAssertEqual(vm.rejected, [])
        XCTAssertEqual(vm.importableCount, 0)
        XCTAssertFalse(vm.canConfirm)
        XCTAssertFalse(vm.needsDecision)
    }

    func testPrepareExportSetsTextAndFailureThenRetryRecovers() async {
        let stub = StubMaster.makeService()
        let vm = makeViewModel(stub)
        let failure = PokeCalcError(code: PokeCalcError.Code.transport, message: "down")
        await stub.setMasterError(failure)
        await vm.prepareExport(members: [member])
        XCTAssertNil(vm.exportText)
        XCTAssertEqual(vm.exportError, failure)
        XCTAssertFalse(vm.isExporting)

        await stub.setMasterError(nil)
        await vm.prepareExport(members: [member])
        XCTAssertEqual(vm.exportText, "テストアルファ\nNature: テストせいかく攻撃上昇")
        XCTAssertNil(vm.exportError, "成功したら前回のエラーを消す")
    }

    func testExportOmissionCountsAreZeroWhenNothingWasOmitted() async {
        let vm = makeViewModel()
        await vm.prepareExport(members: [member])
        XCTAssertEqual(vm.exportUnresolvedCount, 0)
        XCTAssertEqual(vm.exportSkippedMemberCount, 0)
    }

    func testExportOmissionCountsReportSkippedMembersAndUnresolvedIds() async {
        let vm = makeViewModel()
        let missing = TeamMember(id: "missing", speciesKey: "no-such", natureId: "stub-nature-atk-up")
        let withUnknownMove = TeamMember(
            id: "m2", speciesKey: "9101-000", moveIds: ["no-such-move"], natureId: "stub-nature-atk-up")
        await vm.prepareExport(members: [missing, withUnknownMove])
        XCTAssertEqual(vm.exportSkippedMemberCount, 1)
        XCTAssertEqual(vm.exportUnresolvedCount, 1)
        XCTAssertEqual(ShowdownTextLabels.exportUnresolvedNotice(count: 1), "1 項目は名前を引けず省きました。")
        XCTAssertEqual(ShowdownTextLabels.exportSkippedNotice(count: 2), "2 体は種族を引けず書き出していません。")
    }

    // MARK: 取り込み

    func testAllValidNeedsNoDecision() async {
        let vm = makeViewModel()
        vm.setPastedText("テストアルファ\n\nテストベータ")
        await vm.analyze(existingMemberCount: 0)
        XCTAssertEqual(vm.phase, .reviewing)
        XCTAssertEqual(vm.importableCount, 2)
        XCTAssertEqual(vm.rejected, [])
        XCTAssertTrue(vm.canConfirm)
        XCTAssertFalse(vm.needsDecision)
    }

    func testRejectedLinesWithValidMembersLetTheUserChooseAndConfirmReturnsOnlyValidOnes() async {
        let vm = makeViewModel()
        vm.setPastedText("テストアルファ\nEVs: 252 Atk\n\nそんなポケ")
        await vm.analyze(existingMemberCount: 0)
        XCTAssertEqual(vm.phase, .reviewing)
        XCTAssertEqual(vm.importableCount, 1)
        XCTAssertEqual(vm.rejected.map(\.reason), [.unsupportedStatLine, .speciesNotFound])
        XCTAssertEqual(vm.rejected.map(\.lineNumber), [2, 4])
        XCTAssertTrue(vm.canConfirm)
        XCTAssertTrue(vm.needsDecision)

        let confirmed = vm.confirm()
        XCTAssertEqual(confirmed.map(\.speciesKey), ["9101-000"])
        XCTAssertEqual(vm.phase, .idle)
        XCTAssertEqual(vm.pastedText, "", "追加したら貼り付けを空に戻す")
        XCTAssertEqual(vm.rejected, [])
        XCTAssertEqual(vm.confirm(), [], "二重に確定しても追加は増えない")
    }

    func testNothingImportableOffersNoConfirmAndKeepsTheList() async {
        let vm = makeViewModel()
        vm.setPastedText("そんなポケ")
        await vm.analyze(existingMemberCount: 0)
        XCTAssertEqual(vm.phase, .reviewing)
        XCTAssertEqual(vm.importableCount, 0)
        XCTAssertEqual(vm.rejected.map(\.reason), [.speciesNotFound])
        XCTAssertFalse(vm.canConfirm)
        XCTAssertFalse(vm.needsDecision)
        XCTAssertEqual(vm.confirm(), [])
        XCTAssertEqual(vm.phase, .reviewing, "確定できないときは状態を変えない")
    }

    func testEmptyInputDoesNotCommunicate() async {
        let stub = StubMaster.makeService()
        let vm = makeViewModel(stub)
        vm.setPastedText("  \n\u{3000}\n")
        await vm.analyze(existingMemberCount: 0)
        XCTAssertEqual(vm.phase, .idle)
        XCTAssertFalse(vm.canConfirm)
        let calls = await stub.speciesSearchCalls
        XCTAssertEqual(calls, [])
    }

    func testChangingTheTextDiscardsTheReviewButTheSameTextKeepsIt() async {
        let vm = makeViewModel()
        vm.setPastedText("テストアルファ")
        await vm.analyze(existingMemberCount: 0)
        vm.setPastedText("テストアルファ")
        XCTAssertEqual(vm.phase, .reviewing, "同じ文字列なら解釈の結果を残す")
        vm.setPastedText("テストベータ")
        XCTAssertEqual(vm.phase, .idle, "見せている一覧と文字列が食い違わないようにする")
        XCTAssertFalse(vm.canConfirm)
        XCTAssertEqual(vm.confirm(), [])
        XCTAssertEqual(vm.rejected, [])
        XCTAssertEqual(vm.importableCount, 0)
    }

    func testCancelResetsImportButKeepsExport() async {
        let vm = makeViewModel()
        await vm.prepareExport(members: [member])
        vm.setPastedText("テストアルファ\nEVs: 1 HP")
        await vm.analyze(existingMemberCount: 0)
        vm.cancel()
        XCTAssertEqual(vm.phase, .idle)
        XCTAssertEqual(vm.pastedText, "")
        XCTAssertEqual(vm.rejected, [])
        XCTAssertEqual(vm.importableCount, 0)
        XCTAssertNotNil(vm.exportText)
    }

    func testExistingMemberCountLimitsWhatCanBeImported() async {
        let vm = makeViewModel()
        vm.setPastedText("テストアルファ\n\nテストベータ")
        await vm.analyze(existingMemberCount: TeamLimits.maxMembers - 1)
        XCTAssertEqual(vm.importableCount, 1)
        XCTAssertEqual(vm.rejected.map(\.reason), [.memberLimitExceeded])
        XCTAssertTrue(vm.needsDecision)
    }

    func testCommunicationFailureIsReportedWithoutBreakingTheScreen() async {
        let stub = StubMaster.makeService()
        let failure = PokeCalcError(code: PokeCalcError.Code.transport, message: "down")
        await stub.setMasterError(failure)
        let vm = makeViewModel(stub)
        vm.setPastedText("テストアルファ")
        await vm.analyze(existingMemberCount: 0)
        XCTAssertEqual(vm.phase, .reviewing)
        XCTAssertEqual(vm.importError, failure)
        XCTAssertEqual(vm.rejected.map(\.reason), [.lookupFailed])
        XCTAssertFalse(vm.canConfirm)
        XCTAssertEqual(vm.pastedText, "テストアルファ", "貼り付けは消さない(通信を直して再実行できる)")

        await stub.setMasterError(nil)
        await vm.analyze(existingMemberCount: 0)
        XCTAssertNil(vm.importError, "再実行で成功したらエラーを消す")
        XCTAssertTrue(vm.canConfirm)
    }
}
