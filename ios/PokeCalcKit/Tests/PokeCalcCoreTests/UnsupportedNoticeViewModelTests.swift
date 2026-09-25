import XCTest

@testable import PokeCalcCore

/// P6-17(ADR-0501「P6-17」3章): 計算画面・逆算画面の ViewModel が、応答の未対応の印を
/// 「結果の上に1回(共通)/ 行ごと」に分け、印の技・持ち物・特性の名前を**マスタから**引いて出すこと。
/// 失敗・印の無い応答では注記が消えること(前の結果の注記を残さない)。
@MainActor
final class UnsupportedNoticeViewModelTests: XCTestCase {

    private static let summaryPrefix = "この結果は正確でない可能性があります(未対応: "

    /// 要求の形を写した行(`echoResult`)に、全行共通の印と、`flaggedItemID` を持つ行だけの印を付ける。
    private nonisolated static func bulkResponder(
        commonMarks: @escaping @Sendable (BulkCalcRequest) -> [UnsupportedMark],
        flaggedItemID: String?
    ) -> @Sendable (BulkCalcRequest) -> Result<BulkCalcResult, PokeCalcError> {
        { request in
            let echo = StubPokeCalcService.echoResult(for: request)
            let rows = echo.rows.map { row -> BulkCalcRow in
                var marks = commonMarks(request)
                if let flaggedItemID, row.itemId == flaggedItemID {
                    marks.append(UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: flaggedItemID))
                }
                var result = row.result
                result.unsupported = marks
                return BulkCalcRow(preset: row.preset, presetLabel: row.presetLabel, itemId: row.itemId,
                                   defender: row.defender, result: result)
            }
            return .success(BulkCalcResult(defenderSpeciesKey: echo.defenderSpeciesKey, rows: rows))
        }
    }

    // MARK: - 計算画面

    /// 全行に付く技・攻撃側の特性の印は `unsupportedNotice` に1回。名前は技の辞書・攻撃側の特性の一覧から引く。
    func testCalcScreenShowsCommonMarksOnceWithMasterNames() async throws {
        let stub = StubMaster.makeService()
        await stub.setBulkResponder(Self.bulkResponder(commonMarks: { request in
            [UnsupportedMark(target: .move, reason: .multiHit, id: request.moveId),
             UnsupportedMark(target: .attackerAbility, reason: .unsupportedEffect, id: StubMaster.ability.id)]
        }, flaggedItemID: nil))
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()

        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id, "前提: 既定の技")
        XCTAssertEqual(
            viewModel.unsupportedNotice,
            "\(Self.summaryPrefix)技「\(StubMaster.alphaOnlyMove.nameJa)」(多段技)、攻撃側の特性「\(StubMaster.ability.nameJa)」)"
        )
        XCTAssertFalse(viewModel.rows.isEmpty)
        XCTAssertTrue(viewModel.rows.allSatisfy { $0.unsupportedNote == nil }, "共通の印は行に重ねて出さない")
    }

    /// 持ち物の比較で増えた行だけに付く防御側の持ち物の印は、その行の `unsupportedNote`。名前は持ち物の一覧から引く。
    func testCalcScreenShowsItemMarkOnlyOnFlaggedRows() async throws {
        let stub = StubMaster.makeService()
        await stub.setBulkResponder(Self.bulkResponder(commonMarks: { _ in [] }, flaggedItemID: StubMaster.itemA.id))
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemA.id)

        XCTAssertEqual(viewModel.rows.map(\.id), ["none@-", "none@\(StubMaster.itemA.id)"])
        XCTAssertEqual(viewModel.rows.map(\.unsupportedNote),
                       [nil, "未対応: 防御側の持ち物「\(StubMaster.itemA.nameJa)」"])
        XCTAssertNil(viewModel.unsupportedNotice, "一部の行だけの印は結果の上に出さない")
    }

    /// 印の無い応答・失敗した応答の後は、前の注記を残さない。
    func testCalcScreenClearsNoticeOnUnmarkedResponseAndOnFailure() async throws {
        let stub = StubMaster.makeService()
        await stub.setBulkResponder(Self.bulkResponder(commonMarks: { request in
            [UnsupportedMark(target: .move, reason: .fixedDamage, id: request.moveId)]
        }, flaggedItemID: nil))
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        XCTAssertNotNil(viewModel.unsupportedNotice, "前提: 印のある応答で注記が出る")

        await stub.setBulkResponder(Self.bulkResponder(commonMarks: { _ in [] }, flaggedItemID: nil))
        await viewModel.setCritical(true)
        XCTAssertNil(viewModel.unsupportedNotice, "印の無い応答で消える")

        await stub.setBulkResponder(Self.bulkResponder(commonMarks: { request in
            [UnsupportedMark(target: .move, reason: .fixedDamage, id: request.moveId)]
        }, flaggedItemID: nil))
        await viewModel.setCritical(false)
        XCTAssertNotNil(viewModel.unsupportedNotice, "前提: もう一度出す")

        await stub.setBulkResponder { _ in
            .failure(PokeCalcError(code: PokeCalcError.Code.invalidInput, message: "テスト用の失敗"))
        }
        await viewModel.setCritical(true)
        XCTAssertNotNil(viewModel.error, "前提: 失敗した")
        XCTAssertNil(viewModel.unsupportedNotice, "失敗で行を消すときは注記も消す")
    }

    /// マスタに無い ID は ID のまま出す(黙って消さない)。
    func testCalcScreenFallsBackToIDForUnknownNames() async throws {
        let stub = StubMaster.makeService()
        await stub.setBulkResponder(Self.bulkResponder(commonMarks: { _ in
            [UnsupportedMark(target: .defenderAbility, reason: .unsupportedEffect, id: "stub-ability-unknown")]
        }, flaggedItemID: nil))
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        XCTAssertEqual(viewModel.unsupportedNotice, "\(Self.summaryPrefix)防御側の特性「stub-ability-unknown」)")
    }

    // MARK: - 逆算画面

    /// 技の印は全候補共通で `result.unsupportedNotice` に1回、相手の持ち物候補の印はその候補のカードだけ。
    func testReverseScreenPlacesMoveMarkAboveAndItemMarkOnCandidates() async throws {
        let stub = StubMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        let flaggedItemID = StubMaster.itemA.id
        await stub.setReverseResponder { request in
            let echo = StubPokeCalcService.echoReverseResult(for: request)
            let candidates = echo.candidates.map { candidate -> ReverseCandidate in
                var copy = candidate
                copy.unsupported = [UnsupportedMark(target: .move, reason: .variablePower, id: request.moveId)]
                if candidate.itemId == flaggedItemID {
                    copy.unsupported.append(UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: flaggedItemID))
                }
                return copy
            }
            return .success(ReverseResult(side: echo.side, stat: echo.stat, assumedHPSP: echo.assumedHPSP,
                                          candidates: candidates, exactCount: echo.exactCount))
        }
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        let observationID = try XCTUnwrap(viewModel.observations.first?.id)
        await viewModel.editObservation(id: observationID, text: "12")
        await viewModel.toggleOpponentItemCandidate(itemId: flaggedItemID)

        let result = try XCTUnwrap(viewModel.result)
        XCTAssertEqual(result.unsupportedNotice,
                       "\(Self.summaryPrefix)技「\(StubMaster.alphaOnlyMove.nameJa)」(威力が変化))")
        XCTAssertEqual(result.candidates.map(\.id),
                       ["neutral@-", "neutral@\(flaggedItemID)", "plus@-", "plus@\(flaggedItemID)"])
        let itemNote = "未対応: 防御側の持ち物「\(StubMaster.itemA.nameJa)」"
        XCTAssertEqual(result.candidates.map(\.unsupportedNote), [nil, itemNote, nil, itemNote])
    }

    /// 印の無い逆算結果には注記を出さない(既存の画面のまま)。
    func testReverseScreenWithoutMarksHasNoNotice() async throws {
        let stub = StubMaster.makeService(natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        await viewModel.editObservation(id: try XCTUnwrap(viewModel.observations.first?.id), text: "12")

        let result = try XCTUnwrap(viewModel.result)
        XCTAssertNil(result.unsupportedNotice)
        XCTAssertTrue(result.candidates.allSatisfy { $0.unsupportedNote == nil })
    }
}
