import XCTest

@testable import PokeCalcCore

/// 逆算画面の状態(P6-2b。ADR-0500 §1: 画面のロジックは ViewModel で XCTest に固定し、View は描くだけ)。
///
/// サービスは `StubPokeCalcService`(`setReverseMode(.immediate / .manual)`)。モックの数値には依存しない。
/// 用語: 「自分」= 既知側(`known`)、「相手」= 逆算する側(`unknownSpeciesKey`)。
/// - 与えたダメージ = `side: .defender`(自分が攻撃側。技は自分の learnset)
/// - 受けたダメージ = `side: .attacker`(自分が防御側。技は相手の learnset)
@MainActor
final class ReverseViewModelTests: XCTestCase {

    /// CLAUDE.md ドメイン規約: SP は 1 ステータス最大 32。
    private let maxStatSP = 32

    // MARK: - 補助

    private func makeStub(
        species: [SpeciesDetail] = [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly]
    ) async -> StubPokeCalcService {
        let stub = StubMaster.makeService(species: species, natures: StubMaster.reverseNatures)
        await stub.setReverseMode(.immediate)
        return stub
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> ReverseViewModel {
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func firstObservationID(_ viewModel: ReverseViewModel) throws -> Int {
        try XCTUnwrap(viewModel.observations.first?.id, "観測の行が無い")
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> ReverseRequest {
        let requests = await stub.reverseRequests
        return try XCTUnwrap(requests.last, "reverse が呼ばれていない")
    }

    private func reverseCount(_ stub: StubPokeCalcService) async -> Int {
        await stub.reverseRequests.count
    }

    private func errorCode(_ error: CalcScreenError?) -> String? {
        guard case .service(let code, _) = error else { return nil }
        return code
    }

    private func sp(hp: Int = 0, atk: Int = 0, def: Int = 0, spa: Int = 0, spd: Int = 0) -> StatBlock {
        StatBlock(hp: hp, atk: atk, def: def, spa: spa, spd: spd, spe: 0)
    }

    /// 結果を見分けるための1候補だけの応答(値は識別用で意味は無い)。
    private nonisolated static func singleCandidateResult(rangeMin: Int, side: ReverseSide = .defender) -> ReverseResult {
        let candidate = ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: nil,
            ranges: [SPRange(min: rangeMin, max: rangeMin)], spCount: 1,
            exact: true, mismatch: 0, support: 1, minPercent: 10.0, maxPercent: 11.0
        )
        return ReverseResult(side: side, stat: .def, assumedHPSP: 32, candidates: [candidate], exactCount: 1)
    }

    // MARK: - 起動時の既定(計算はしない)

    func testLoadSelectsDefaultsWithoutCalculating() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)

        XCTAssertEqual(viewModel.speciesOptions.map(\.key),
                       [StubMaster.alpha.key, StubMaster.beta.key, StubMaster.gamma.key, StubMaster.statusOnly.key])
        XCTAssertEqual(viewModel.itemOptions.map(\.id), [StubMaster.itemA.id, StubMaster.itemB.id])
        XCTAssertEqual(viewModel.side, .defender, "既定は与えたダメージ")
        XCTAssertEqual(viewModel.observationKind, .percent)
        XCTAssertEqual(viewModel.mySpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.opponentSpeciesKey, StubMaster.beta.key)
        // 技 = 攻撃側(与えたダメージでは自分)の learnset の順・マスタにある・ダメージ技だけ(変化技は逆算できないので出さない)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id])
        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id)
        // 既定は engine/presets/attacker.json の default(無振り。P6-12 で A特化から変更。ADR-0501「P6-12」5章)
        XCTAssertEqual(viewModel.attackerPreset, AttackerPreset.defaultPreset)
        // P6-2d で `knownDefenderPreset` が `KnownDefenderPreset?`(計算プロパティ)になったため、
        // 素の `.none` は `Optional<KnownDefenderPreset>.none`(nil)に解決されてしまう
        // (Swift の既知の挙動。ビルド時に警告も出る)。列挙子を明示して曖昧さを消す
        // (比較する意味は変えていない。P6-2d の implementer が気づいた型だけのバグ)。
        XCTAssertEqual(viewModel.knownDefenderPreset, KnownDefenderPreset.none)
        XCTAssertNil(viewModel.myItemId)
        XCTAssertEqual(viewModel.opponentItemCandidateIds, [])

        // 観測は空の1行から始まる。空の行はエラー(.empty)として持つが、計算は止めない(送らないだけ)。
        XCTAssertEqual(viewModel.observations.count, 1)
        XCTAssertEqual(viewModel.observations.first?.text, "")
        XCTAssertEqual(viewModel.observations.first?.error, .empty)
        XCTAssertNil(viewModel.observations.first?.observation)

        XCTAssertNil(viewModel.result)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
        let reverseRequests = await stub.reverseRequests
        XCTAssertTrue(reverseRequests.isEmpty, "有効な観測が無いうちは逆算しない")
        let bulkRequests = await stub.bulkRequests
        XCTAssertTrue(bulkRequests.isEmpty, "逆算画面は一括計算を呼ばない")
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests, [StubMaster.alpha.key], "learnset は攻撃側(自分)の分だけ読む")
    }

    func testLoadCalledTwiceOnlyLoadsOnce() async {
        let stub = await makeStub()
        let viewModel = ReverseViewModel(service: stub)
        await viewModel.load()
        await viewModel.load()
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests.count, 1)
    }

    // MARK: - 観測 → 逆算の要求(与えたダメージ)

    func testValidPercentObservationCallsReverseOnceWithDefenderSideRequest() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        // 既定は無振り(P6-12)。A特化の組み立てを見るため、観測を入れる前(逆算しない状態)に選ぶ
        // (ADR-0501「P6-12」5章)。
        await viewModel.selectAttackerPreset(.aFull)

        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        let requests = await stub.reverseRequests
        XCTAssertEqual(requests.count, 1, "入力の変更1回につき reverse は1回")
        let request = try XCTUnwrap(requests.first)
        XCTAssertEqual(request.format, .single)
        XCTAssertEqual(request.side, .defender)
        // 既知 = 自分 = 攻撃側。A特化(物理技 → atk 32 + atk 上昇性格。ADR-0500 §6)
        XCTAssertEqual(request.known.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.known.sp, sp(atk: maxStatSP))
        XCTAssertEqual(request.known.natureId, StubMaster.atkUpNature.id)
        XCTAssertNil(request.known.itemId)
        XCTAssertEqual(request.unknownSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.moveId, StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(request.itemCandidates, [], "相手の持ち物候補を選んでいなければ省略(=持ち物なしの1通り)")
        XCTAssertEqual(request.observations, [.percent(12)])
        XCTAssertFalse(request.critical)
        XCTAssertEqual(request.maxCandidates, 0, "候補は切らない(2 × 持ち物候補数で十分少ない。ADR-0300 §7 と同じ)")

        XCTAssertEqual(viewModel.observations.first?.observation, .percent(12))
        XCTAssertNil(viewModel.observations.first?.error)
        let result = try XCTUnwrap(viewModel.result)
        XCTAssertEqual(result.candidates.map(\.id), ["neutral@-", "plus@-"])
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
    }

    func testInvalidObservationSetsFieldErrorAndDoesNotCalculate() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)

        let cases: [(String, ObservationFieldError)] = [("abc", .notANumber), ("0", .outOfRange), ("101", .outOfRange), ("12.5", .notANumber)]
        for (text, expected) in cases {
            await viewModel.editObservation(id: id, text: text)
            XCTAssertEqual(viewModel.observations.first?.text, text)
            XCTAssertEqual(viewModel.observations.first?.error, expected, "入力 \(text)")
            XCTAssertNil(viewModel.observations.first?.observation)
        }
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 0, "不正な観測では逆算しない")
        XCTAssertNil(viewModel.result)
    }

    func testAddingEmptyRowDoesNotRecalculateAndSecondObservationNarrows() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        viewModel.addObservation()
        XCTAssertEqual(viewModel.observations.count, 2)
        XCTAssertEqual(viewModel.observations.last?.text, "")
        XCTAssertNotEqual(viewModel.observations[0].id, viewModel.observations[1].id, "行の id は一意")
        var count = await reverseCount(stub)
        XCTAssertEqual(count, 1, "空の行を足しただけでは逆算しない(送る観測が変わらない)")
        XCTAssertNotNil(viewModel.result, "空の行を足しても結果は消さない")

        await viewModel.editObservation(id: viewModel.observations[1].id, text: "20")
        count = await reverseCount(stub)
        XCTAssertEqual(count, 2)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.observations, [.percent(12), .percent(20)], "行の順で送る")
    }

    func testAnyInvalidRowBlocksCalculationAndClearsResult() async throws {
        // Web(ADR-0300 §7)と同じ: 空の行は送らないだけ、不正な行が1つでもあれば計算しない。
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        XCTAssertNotNil(viewModel.result)

        viewModel.addObservation()
        await viewModel.editObservation(id: viewModel.observations[1].id, text: "abc")
        XCTAssertEqual(viewModel.observations[1].error, .notANumber)
        var count = await reverseCount(stub)
        XCTAssertEqual(count, 1, "不正な行があるうちは逆算しない")
        XCTAssertNil(viewModel.result, "いまの入力に対応しない古い候補を出したままにしない")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error, "行ごとのエラーは画面全体のエラーにしない")

        await viewModel.editObservation(id: viewModel.observations[1].id, text: "20")
        count = await reverseCount(stub)
        XCTAssertEqual(count, 2)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.observations, [.percent(12), .percent(20)])
        XCTAssertNotNil(viewModel.result)
    }

    func testEditKeepingSameParsedValueDoesNotRecalculate() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await viewModel.editObservation(id: id, text: "12")
        await viewModel.editObservation(id: id, text: "012")
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1, "送る観測が変わらない編集では逆算しない")
        XCTAssertEqual(viewModel.observations.first?.text, "012", "文字列はそのまま保持する")
    }

    func testClearingTextDropsResultAndInFlightResponse() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseMode(.manual)

        let pending = Task { await viewModel.editObservation(id: id, text: "12") }
        try await stub.waitForReverseRequests(count: 1)
        XCTAssertTrue(viewModel.isLoading)

        await viewModel.editObservation(id: id, text: "")
        XCTAssertNil(viewModel.result)
        XCTAssertFalse(viewModel.isLoading, "計算しない状態に戻ったら読み込み中を解く")

        await stub.resolveReverseWithEcho(at: 0)
        await pending.value
        XCTAssertNil(viewModel.result, "有効な観測が無くなった後に古い応答が届いても出さない")
        XCTAssertFalse(viewModel.isLoading)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1)
    }

    func testRemoveObservation() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        viewModel.addObservation()
        let emptyRowID = viewModel.observations[1].id

        // 空の行を消しても送る観測は変わらないので逆算しない
        await viewModel.removeObservation(id: emptyRowID)
        var count = await reverseCount(stub)
        XCTAssertEqual(count, 1)
        XCTAssertEqual(viewModel.observations.count, 1)

        viewModel.addObservation()
        await viewModel.editObservation(id: viewModel.observations[1].id, text: "20")
        count = await reverseCount(stub)
        XCTAssertEqual(count, 2)

        // 有効な行を消すと残りで逆算し直す
        await viewModel.removeObservation(id: viewModel.observations[0].id)
        count = await reverseCount(stub)
        XCTAssertEqual(count, 3)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.observations, [.percent(20)])

        // 最後の1行を消すと空の1行に戻る(入力欄が無くならない)。計算はせず結果を消す。
        await viewModel.removeObservation(id: viewModel.observations[0].id)
        XCTAssertEqual(viewModel.observations.count, 1)
        XCTAssertEqual(viewModel.observations.first?.text, "")
        XCTAssertNil(viewModel.result)
        count = await reverseCount(stub)
        XCTAssertEqual(count, 3)
    }

    func testUnknownObservationIDIsIgnored() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let unknownID = try firstObservationID(viewModel) + 1000
        await viewModel.editObservation(id: unknownID, text: "12")
        await viewModel.removeObservation(id: unknownID)
        XCTAssertEqual(viewModel.observations.count, 1)
        XCTAssertEqual(viewModel.observations.first?.text, "")
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 0)
    }

    // MARK: - 与えたダメージ側の入力変更

    func testDefenderSideInputChangesRecalculateOnce() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        await viewModel.selectAttackerPreset(.aMax)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.known.sp, sp(atk: maxStatSP))
        XCTAssertEqual(request.known.natureId, StubMaster.neutralNature.id)

        await viewModel.selectMyItem(id: StubMaster.itemA.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.known.itemId, StubMaster.itemA.id)

        await viewModel.selectMove(id: StubMaster.specialMove.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.known.sp, sp(spa: maxStatSP), "特殊技なら C に振る(ADR-0500 §6)")

        // 相手の種族を変えても、与えたダメージでは技(自分の learnset)は読み直さない
        let speciesBefore = await stub.speciesRequests.count
        await viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.unknownSpeciesKey, StubMaster.gamma.key)
        let speciesAfter = await stub.speciesRequests.count
        XCTAssertEqual(speciesAfter, speciesBefore)

        let count = await reverseCount(stub)
        XCTAssertEqual(count, 5, "観測1 + 変更4 = 各1回")
    }

    func testInactiveSidePresetChangeDoesNotRecalculate() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        // 与えたダメージでは自分の防御側プリセットは要求に入らない(値は覚えておくが計算しない)
        await viewModel.selectKnownDefenderPreset(.full)
        XCTAssertEqual(viewModel.knownDefenderPreset, .full)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1)
    }

    func testSelectingMoveOutsideOptionsIsIgnored() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        await viewModel.selectMove(id: StubMaster.statusMove.id)   // 変化技は選択肢に無い
        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1)
    }

    func testMySpeciesChangeOnDefenderSideReloadsMovesAndReselects() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        // gamma は alphaOnlyMove を覚えないので、learnset の順で最初のダメージ技(physical)を選び直す
        await viewModel.selectMySpecies(key: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.mySpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.physicalMove.id, StubMaster.specialMove.id])
        XCTAssertEqual(viewModel.moveId, StubMaster.physicalMove.id)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.known.speciesKey, StubMaster.gamma.key)
        XCTAssertEqual(request.moveId, StubMaster.physicalMove.id)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 2)

        // 覚えている技はそのまま残す
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        await viewModel.selectMySpecies(key: StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
    }

    func testUnknownSpeciesKeyIsIgnored() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        await viewModel.selectMySpecies(key: "9999-000")
        await viewModel.selectOpponentSpecies(key: "9999-000")
        XCTAssertEqual(viewModel.mySpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.opponentSpeciesKey, StubMaster.beta.key)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1)
    }

    // MARK: - 相手の持ち物候補

    func testOpponentItemCandidatesToggleInMasterOrderWithNoItemFirst() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        await viewModel.toggleOpponentItemCandidate(itemId: StubMaster.itemB.id)
        await viewModel.toggleOpponentItemCandidate(itemId: StubMaster.itemA.id)
        XCTAssertEqual(viewModel.opponentItemCandidateIds, [StubMaster.itemA.id, StubMaster.itemB.id], "トグル順ではなくマスタの順")
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates, [nil, StubMaster.itemA.id, StubMaster.itemB.id], "「持ち物なし」を先頭に含める")
        XCTAssertEqual(viewModel.result?.candidates.map(\.id),
                       ["neutral@-", "neutral@\(StubMaster.itemA.id)", "neutral@\(StubMaster.itemB.id)",
                        "plus@-", "plus@\(StubMaster.itemA.id)", "plus@\(StubMaster.itemB.id)"])

        await viewModel.toggleOpponentItemCandidate(itemId: StubMaster.itemA.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates, [nil, StubMaster.itemB.id])

        await viewModel.toggleOpponentItemCandidate(itemId: StubMaster.itemB.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.itemCandidates, [], "全部外したら省略に戻る")
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 5)
    }

    // MARK: - 受けたダメージ(side = attacker)

    func testSwitchingToAttackerSideResetsObservationsAndUsesOpponentLearnset() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        await viewModel.selectMyItem(id: StubMaster.itemA.id)
        await viewModel.toggleOpponentItemCandidate(itemId: StubMaster.itemB.id)
        let countBefore = await reverseCount(stub)

        await viewModel.selectSide(.attacker)

        XCTAssertEqual(viewModel.side, .attacker)
        XCTAssertEqual(viewModel.observationKind, .damage)
        // 観測の単位の意味が変わる(%→実点数)ので空の1行に戻す(ADR-0300 §7 と同じ)
        XCTAssertEqual(viewModel.observations.count, 1)
        XCTAssertEqual(viewModel.observations.first?.text, "")
        XCTAssertNil(viewModel.result)
        // 持ち物は側で意味が変わる(攻撃用/防御用)ので外す。種族・両方のプリセットは残す。
        XCTAssertNil(viewModel.myItemId)
        XCTAssertEqual(viewModel.opponentItemCandidateIds, [])
        XCTAssertEqual(viewModel.mySpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.opponentSpeciesKey, StubMaster.beta.key)
        // 側を変えても自分のプリセットは起動時の既定のまま(P6-12 で既定が無振りに変わったため既定を参照する)
        XCTAssertEqual(viewModel.attackerPreset, AttackerPreset.defaultPreset)
        // 技は攻撃側 = 相手(beta)の learnset から、ダメージ技だけ
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.specialMove.id, StubMaster.physicalMove.id])
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
        let lastSpecies = await stub.speciesRequests.last
        XCTAssertEqual(lastSpecies, StubMaster.beta.key)
        var count = await reverseCount(stub)
        XCTAssertEqual(count, countBefore, "観測が空に戻るので計算しない")

        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "35")
        count = await reverseCount(stub)
        XCTAssertEqual(count, countBefore + 1)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.side, .attacker)
        // 既知 = 自分 = 防御側。既定の無振り(SP 0・無補正)
        XCTAssertEqual(request.known.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.known.sp, sp())
        XCTAssertEqual(request.known.natureId, StubMaster.neutralNature.id)
        XCTAssertNil(request.known.itemId)
        XCTAssertEqual(request.unknownSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.observations, [.damage(35)])
    }

    func testSelectingSameSideIsNoOp() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        await viewModel.selectSide(.defender)
        XCTAssertEqual(viewModel.observations.first?.text, "12", "同じ側を選んでも観測を消さない")
        XCTAssertNotNil(viewModel.result)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1)
    }

    func testAttackerSideDamageValidationAndKnownDefenderPreset() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectSide(.attacker)
        let id = try firstObservationID(viewModel)

        await viewModel.editObservation(id: id, text: "0")
        XCTAssertEqual(viewModel.observations.first?.error, .outOfRange)
        await viewModel.editObservation(id: id, text: "250")   // 実点数は 100 を超えてよい
        XCTAssertEqual(viewModel.observations.first?.observation, .damage(250))

        // 相手の技は特殊(beta の既定)→ 自分の防御側は D。HD特化 = H32 + D32 + (+spd/-atk)(ADR-0009)
        await viewModel.selectKnownDefenderPreset(.full)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.known.sp, sp(hp: maxStatSP, spd: maxStatSP))
        XCTAssertEqual(request.known.natureId, StubMaster.spdUpNature.id)

        await viewModel.selectMove(id: StubMaster.physicalMove.id)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.known.sp, sp(hp: maxStatSP, def: maxStatSP), "物理技なら B")
        XCTAssertEqual(request.known.natureId, StubMaster.defUpNature.id)

        // 受けたダメージでは攻撃側プリセットは要求に入らない
        let countBefore = await reverseCount(stub)
        await viewModel.selectAttackerPreset(.none)
        let countAfter = await reverseCount(stub)
        XCTAssertEqual(countAfter, countBefore)
    }

    func testOpponentSpeciesChangeOnAttackerSideReloadsOpponentMoves() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectSide(.attacker)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "35")
        let speciesBefore = await stub.speciesRequests.count

        await viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
        let speciesRequests = await stub.speciesRequests
        XCTAssertEqual(speciesRequests.count, speciesBefore + 1)
        XCTAssertEqual(speciesRequests.last, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.physicalMove.id, StubMaster.specialMove.id])
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id, "覚えている技は残す")

        // 自分の種族を変えても、受けたダメージでは技を読み直さない
        await viewModel.selectMySpecies(key: StubMaster.gamma.key)
        let speciesAfter = await stub.speciesRequests.count
        XCTAssertEqual(speciesAfter, speciesBefore + 1)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.known.speciesKey, StubMaster.gamma.key)
    }

    // MARK: - 最新の要求だけを反映(CalcViewModel と同じ世代の保護)

    func testStaleReverseSuccessDoesNotOverwriteNewer() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseMode(.manual)

        let older = Task { await viewModel.editObservation(id: id, text: "12") }
        try await stub.waitForReverseRequests(count: 1)
        let newer = Task { await viewModel.editObservation(id: id, text: "20") }
        try await stub.waitForReverseRequests(count: 2)

        await stub.resolveReverse(at: 1, with: .success(Self.singleCandidateResult(rangeMin: 20)))
        await newer.value
        XCTAssertEqual(viewModel.result?.candidates.map(\.spRangeText), ["B 20"])
        XCTAssertFalse(viewModel.isLoading)

        await stub.resolveReverse(at: 0, with: .success(Self.singleCandidateResult(rangeMin: 12)))
        await older.value
        XCTAssertEqual(viewModel.result?.candidates.map(\.spRangeText), ["B 20"], "古い応答で上書きしない")
        XCTAssertFalse(viewModel.isLoading)
    }

    func testStaleReverseFailureDoesNotSetError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseMode(.manual)

        let older = Task { await viewModel.editObservation(id: id, text: "12") }
        try await stub.waitForReverseRequests(count: 1)
        let newer = Task { await viewModel.editObservation(id: id, text: "20") }
        try await stub.waitForReverseRequests(count: 2)

        await stub.resolveReverseWithEcho(at: 1)
        await newer.value
        await stub.resolveReverse(at: 0, with: .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト")))
        await older.value

        XCTAssertNil(viewModel.error, "古い失敗でエラーを立てない")
        XCTAssertNotNil(viewModel.result)
    }

    func testStaleSpeciesDetailDoesNotOverwriteNewerMySpecies() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        let selectGamma = Task { await viewModel.selectMySpecies(key: StubMaster.gamma.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let selectBeta = Task { await viewModel.selectMySpecies(key: StubMaster.beta.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        let betaDetail = try await stub.lookupSpecies(key: StubMaster.beta.key)
        await stub.resolveSpecies(at: baseline + 1, with: .success(betaDetail))
        await selectBeta.value
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.specialMove.id, StubMaster.physicalMove.id])

        let gammaDetail = try await stub.lookupSpecies(key: StubMaster.gamma.key)
        await stub.resolveSpecies(at: baseline, with: .success(gammaDetail))
        await selectGamma.value

        XCTAssertEqual(viewModel.mySpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.specialMove.id, StubMaster.physicalMove.id],
                       "古い species 応答で moveOptions を上書きしない")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.known.speciesKey, StubMaster.beta.key)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 2, "観測1回 + beta の選択1回だけ(gamma は追い越されたので計算しない)")
    }

    func testIsLoadingWhileWaitingForReverse() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseMode(.manual)
        let pending = Task { await viewModel.editObservation(id: id, text: "12") }
        try await stub.waitForReverseRequests(count: 1)
        XCTAssertTrue(viewModel.isLoading)
        await stub.resolveReverseWithEcho(at: 0)
        await pending.value
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - エラー

    func testReverseFailureSetsErrorAndNextSuccessClearsIt() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseResponder { _ in
            .failure(PokeCalcError(code: "invalid_observation", message: "テスト観測が不正"))
        }
        await viewModel.editObservation(id: id, text: "12")
        XCTAssertEqual(errorCode(viewModel.error), "invalid_observation")
        XCTAssertNil(viewModel.result)
        XCTAssertFalse(viewModel.isLoading)

        await stub.setReverseResponder { request in .success(StubPokeCalcService.echoReverseResult(for: request)) }
        await viewModel.editObservation(id: id, text: "20")
        XCTAssertNil(viewModel.error)
        XCTAssertNotNil(viewModel.result)
    }

    func testTransportFailureMapsToCalcScreenError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await stub.setReverseResponder { _ in .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト")) }
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        XCTAssertEqual(viewModel.error, .transport)
    }

    func testMasterLoadFailureSetsError() async {
        let stub = await makeStub()
        await stub.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト"))
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.error, .transport)
        XCTAssertFalse(viewModel.isLoading)
    }

    func testInsufficientSpeciesSetsError() async {
        let stub = await makeStub(species: [StubMaster.alpha])
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.insufficientSpecies)
    }

    func testAttackerWithoutDamageMovesSetsMoveUnavailable() async throws {
        // 変化技しか覚えない種族は逆算できる技が無い(変化技は選択肢に出さない)
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        await viewModel.selectMySpecies(key: StubMaster.statusOnly.key)
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable)
        XCTAssertNil(viewModel.result)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 1, "技が無ければ計算しない")
    }

    func testMissingNatureSetsErrorWithoutCalculating() async throws {
        // 受けたダメージの HB特化に要る +def/-atk の性格がマスタに無い
        let stub = StubMaster.makeService(natures: [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature])
        await stub.setReverseMode(.immediate)
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectSide(.attacker)
        await viewModel.selectMove(id: StubMaster.physicalMove.id)
        await viewModel.selectKnownDefenderPreset(.full)
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "35")
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.natureUnavailable)
        let count = await reverseCount(stub)
        XCTAssertEqual(count, 0)
    }

    // MARK: - 古いエラーを持ち越さない(批評対応: 計算しない状態に戻ったときに前回の失敗を消す)

    func testClearingObservationsAfterFailureClearsError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        let id = try firstObservationID(viewModel)
        await stub.setReverseResponder { _ in
            .failure(PokeCalcError(code: "invalid_observation", message: "テスト観測が不正"))
        }
        await viewModel.editObservation(id: id, text: "12")
        XCTAssertNotNil(viewModel.error)

        // 観測を全部消す(計算しない状態に戻る)と、直前の失敗を画面に残さない。
        await viewModel.editObservation(id: id, text: "")
        XCTAssertNil(viewModel.error, "観測を消したら古いエラーを持ち越さない")
        XCTAssertNil(viewModel.result)
    }

    func testSwitchingSideAfterFailureClearsError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        await stub.setReverseResponder { _ in
            .failure(PokeCalcError(code: "invalid_observation", message: "テスト観測が不正"))
        }
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")
        XCTAssertNotNil(viewModel.error)

        // 側を切り替えると観測が空の1行に戻る(計算しない状態)。古い失敗を持ち越さない。
        await viewModel.selectSide(.attacker)
        XCTAssertNil(viewModel.error, "側を切り替えたら古いエラーを持ち越さない")
    }

    func testSelectingSpeciesWithMovesAfterMoveUnavailableClearsError() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        // 変化技しか覚えない種族に変えると moveUnavailable(観測はまだ空のまま)。
        await viewModel.selectMySpecies(key: StubMaster.statusOnly.key)
        XCTAssertEqual(errorCode(viewModel.error), PokeCalcError.Code.moveUnavailable)
        XCTAssertEqual(viewModel.observations.first?.text, "", "観測はまだ空のまま")

        // ダメージ技を覚える種族に変えれば、moveUnavailable の原因は直る。
        await viewModel.selectMySpecies(key: StubMaster.beta.key)
        XCTAssertNil(viewModel.error, "技が選べる種族に変えたら moveUnavailable を持ち越さない")
    }

    // MARK: - 結果の表示

    func testResultKeepsServerOrderExactCountAndPremise() async throws {
        let stub = await makeStub()
        let viewModel = await loadedViewModel(stub)
        // 並べ替えたら順が変わる並び(後ろの方が support が大きい・spCount が大きい)
        let first = ReverseCandidate(
            natureClass: .plus, nature: NatureModifier(plus: .def, minus: .atk), natureId: nil, itemId: nil,
            ranges: [SPRange(min: 4, max: 7)], spCount: 4, exact: true, mismatch: 0, support: 1,
            minPercent: 20.0, maxPercent: 24.0
        )
        let second = ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: nil,
            ranges: [SPRange(min: 20, max: 32)], spCount: 13, exact: false, mismatch: 3, support: 50,
            minPercent: 18.0, maxPercent: 22.0
        )
        await stub.setReverseResponder { request in
            .success(ReverseResult(side: request.side, stat: .def, assumedHPSP: 32, candidates: [first, second], exactCount: 1))
        }
        await viewModel.editObservation(id: try firstObservationID(viewModel), text: "12")

        let result = try XCTUnwrap(viewModel.result)
        XCTAssertEqual(result.candidates.map(\.id), ["plus@-", "neutral@-"], "サーバーの順(ADR-0010 §R4)のまま")
        XCTAssertEqual(result.candidates.map(\.spRangeText), ["B 4\u{301C}7", "B 20\u{301C}32"])
        XCTAssertEqual(result.exactCount, 1)
        XCTAssertEqual(result.premiseText, "相手の HP の SP を 32(H32)と仮定した結果です")
    }
}
