import XCTest

@testable import PokeCalcCore

/// ダメージ計算画面の状態(P6-2a。ADR-0017 §1: 画面のロジックは ViewModel で XCTest に固定し、View は描くだけ)。
///
/// サービスは `StubPokeCalcService`(架空マスタ・呼び出しの記録・応答の保留と順不同の返却)。
/// 計算結果の数値には依存しない(ViewModel は結果を整形して並べるだけ)。
@MainActor
final class CalcViewModelTests: XCTestCase {

    /// CLAUDE.md ドメイン規約: SP は 1 ステータス最大 32(A特化・A振りは関連ステータスに 32)。
    private let maxStatSP = 32

    private func sp(atk: Int = 0, spa: Int = 0) -> StatBlock {
        StatBlock(hp: 0, atk: atk, def: 0, spa: spa, spd: 0, spe: 0)
    }

    private func loadedViewModel(_ stub: StubPokeCalcService) async -> CalcViewModel {
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        return viewModel
    }

    private func lastRequest(_ stub: StubPokeCalcService) async throws -> BulkCalcRequest {
        let requests = await stub.bulkRequests
        return try XCTUnwrap(requests.last, "calcBulk が呼ばれていない")
    }

    // MARK: - 起動時の既定

    func testLoadSelectsDeterministicDefaultsAndCalculatesOnce() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        // 種族は検索結果(空クエリ)の最初を攻撃側、2番目を防御側にする
        XCTAssertEqual(viewModel.speciesOptions.map(\.key),
                       [StubMaster.alpha.key, StubMaster.beta.key, StubMaster.gamma.key, StubMaster.statusOnly.key])
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        // 技は攻撃側の learnset の順で最初の「ダメージ技」(先頭の変化技は飛ばす)
        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id)
        // プリセットは AttackerPreset.allCases の最初(A特化)、持ち物なし、比較なし
        XCTAssertEqual(viewModel.attackerPreset, .aFull)
        XCTAssertNil(viewModel.attackerItemId)
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [])
        XCTAssertEqual(viewModel.itemOptions.map(\.id), [StubMaster.itemA.id, StubMaster.itemB.id])

        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, 1, "起動時の一括計算はちょうど1回")
        let request = try XCTUnwrap(requests.first)
        XCTAssertEqual(request.format, .single)
        XCTAssertFalse(request.critical)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.attacker.sp, sp(atk: maxStatSP))
        XCTAssertEqual(request.attacker.natureId, StubMaster.atkUpNature.id)
        XCTAssertNil(request.attacker.itemId)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.moveId, StubMaster.alphaOnlyMove.id)
        // presets は省略(技の分類で engine の既定セットが選ばれる。ADR-0009)
        XCTAssertEqual(request.presets, [])
        // 比較する持ち物が無いときは省略(素の1通り)
        XCTAssertTrue(request.itemVariants.isEmpty)

        XCTAssertEqual(viewModel.rows.map(\.id), ["none@-"])
        XCTAssertEqual(viewModel.rows.first?.itemLabel, "持ち物なし")
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
    }

    func testMoveOptionsAreAttackerLearnsetOnlyInLearnsetOrder() async {
        let viewModel = await loadedViewModel(StubMaster.makeService())
        // alpha の learnset: [変化, アルファ専用, 特殊, マスタに無い ID]。マスタに無いものは出さない。
        // 物理技 stub-move-physical は alpha の learnset に無いので出ない。
        XCTAssertEqual(viewModel.moveOptions.map(\.id),
                       [StubMaster.statusMove.id, StubMaster.alphaOnlyMove.id, StubMaster.specialMove.id])
    }

    func testAttackerWithOnlyStatusMovesFallsBackToFirstLearnsetMove() async throws {
        // ダメージ技が無ければ learnset の最初の技(計算自体はサーバーに任せる)
        let stub = StubMaster.makeService(species: [StubMaster.statusOnly, StubMaster.beta])
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.moveId, StubMaster.statusMove.id)
        // 変化技は物理と同じく atk に振る(AttackerPreset.relevantStat)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp(atk: maxStatSP))
    }

    // MARK: - 入力が変わるたびに1回だけ計算する

    func testEachInputChangeCallsCalcBulkExactlyOnceWithTheRightShape() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        // 特殊技にすると関連ステータスが spa になり、A特化の性格は (+spa, -atk)
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        var count = await stub.bulkRequests.count
        XCTAssertEqual(count, 2)
        var request = try await lastRequest(stub)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.attacker.sp, sp(spa: maxStatSP))
        XCTAssertEqual(request.attacker.natureId, StubMaster.spaUpNature.id)

        await viewModel.selectAttackerPreset(.aMax)
        count = await stub.bulkRequests.count
        XCTAssertEqual(count, 3)
        request = try await lastRequest(stub)
        XCTAssertEqual(viewModel.attackerPreset, .aMax)
        XCTAssertEqual(request.attacker.sp, sp(spa: maxStatSP))
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id)

        await viewModel.selectAttackerPreset(.none)
        count = await stub.bulkRequests.count
        XCTAssertEqual(count, 4)
        request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.sp, sp())
        XCTAssertEqual(request.attacker.natureId, StubMaster.neutralNature.id)

        await viewModel.selectAttackerItem(id: StubMaster.itemA.id)
        count = await stub.bulkRequests.count
        XCTAssertEqual(count, 5)
        request = try await lastRequest(stub)
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemA.id)
        XCTAssertEqual(request.attacker.itemId, StubMaster.itemA.id)

        await viewModel.selectAttackerItem(id: nil)
        count = await stub.bulkRequests.count
        XCTAssertEqual(count, 6)
        request = try await lastRequest(stub)
        XCTAssertNil(request.attacker.itemId)

        await viewModel.selectDefender(speciesKey: StubMaster.gamma.key)
        count = await stub.bulkRequests.count
        XCTAssertEqual(count, 7)
        request = try await lastRequest(stub)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.presets, [], "presets は常に省略")
    }

    func testSelectingMoveOutsideOptionsIsIgnored() async {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        // physicalMove は alpha の learnset に無い
        await viewModel.selectMove(id: StubMaster.physicalMove.id)
        XCTAssertEqual(viewModel.moveId, StubMaster.alphaOnlyMove.id)
        let count = await stub.bulkRequests.count
        XCTAssertEqual(count, 1, "選べない技では計算しない")
    }

    // MARK: - 持ち物の比較トグル

    func testDefenderItemToggleBuildsItemVariantsInMasterOrderWithNoItemFirst() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        let a = StubMaster.itemA.id
        let b = StubMaster.itemB.id

        // (トグル操作, 比較中の ID(マスタ順), 要求の itemVariants)
        // 比較が1つでもあれば「持ち物なし(nil)」を先頭に含める。比較が無ければ省略(素の1通り)。
        let steps: [(toggle: String, compared: [String], variants: [String?])] = [
            (b, [b], [nil, b]),
            (a, [a, b], [nil, a, b]),
            (b, [a], [nil, a]),
            (a, [], []),
        ]
        for (index, step) in steps.enumerated() {
            await viewModel.toggleDefenderItemComparison(itemId: step.toggle)
            XCTAssertEqual(viewModel.comparedDefenderItemIds, step.compared, "step \(index)")
            let count = await stub.bulkRequests.count
            XCTAssertEqual(count, index + 2, "トグル1回で計算1回(step \(index))")
            let request = try await lastRequest(stub)
            XCTAssertEqual(request.itemVariants, step.variants, "step \(index)")
        }
    }

    func testRowsFollowResponseOrderAndCarryItemNames() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemA.id)
        // スタブは presets 省略時 `none` × itemVariants を返す
        XCTAssertEqual(viewModel.rows.map(\.id), ["none@-", "none@\(StubMaster.itemA.id)"])
        XCTAssertEqual(viewModel.rows.map(\.itemLabel), ["持ち物なし", StubMaster.itemA.nameJa])
    }

    // MARK: - 攻撃側の変更と攻守入れ替え

    func testSelectingAttackerReselectsMoveByRule() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        // いまの技(アルファ専用)を gamma は覚えない → gamma の learnset の最初のダメージ技
        await viewModel.selectAttacker(speciesKey: StubMaster.gamma.key)
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.gamma.key)
        XCTAssertEqual(viewModel.moveOptions.map(\.id), [StubMaster.physicalMove.id, StubMaster.specialMove.id])
        XCTAssertEqual(viewModel.moveId, StubMaster.physicalMove.id)
        var count = await stub.bulkRequests.count
        XCTAssertEqual(count, 2, "攻撃側の変更で計算1回")

        // いまの技を新しい攻撃側も覚えるなら、そのまま残す
        await viewModel.selectMove(id: StubMaster.specialMove.id)
        await viewModel.selectAttacker(speciesKey: StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
        count = await stub.bulkRequests.count
        XCTAssertEqual(count, 4)
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
    }

    func testSwapSidesSwapsSpeciesAndReselectsMoveWithOneCalc() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        await viewModel.selectAttackerItem(id: StubMaster.itemA.id)
        await viewModel.toggleDefenderItemComparison(itemId: StubMaster.itemB.id)
        let before = await stub.bulkRequests.count

        await viewModel.swapSides()
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.alpha.key)
        // アルファ専用技を beta は覚えない → beta の learnset の最初のダメージ技(先頭の変化技を飛ばして特殊技)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(viewModel.moveOptions.map(\.id),
                       [StubMaster.statusMove.id, StubMaster.specialMove.id, StubMaster.physicalMove.id])
        // 入れ替えるのは種族だけ。プリセット・攻撃側の持ち物・比較トグルは画面の設定として残す
        XCTAssertEqual(viewModel.attackerPreset, .aFull)
        XCTAssertEqual(viewModel.attackerItemId, StubMaster.itemA.id)
        XCTAssertEqual(viewModel.comparedDefenderItemIds, [StubMaster.itemB.id])

        let after = await stub.bulkRequests.count
        XCTAssertEqual(after, before + 1, "入れ替えは計算1回(種族2つを別々に変えたことにしない)")
        let request = try await lastRequest(stub)
        XCTAssertEqual(request.attacker.speciesKey, StubMaster.beta.key)
        XCTAssertEqual(request.defenderSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(request.moveId, StubMaster.specialMove.id)
        XCTAssertEqual(request.attacker.sp, sp(spa: maxStatSP))
        XCTAssertEqual(request.attacker.natureId, StubMaster.spaUpNature.id)
        XCTAssertEqual(request.attacker.itemId, StubMaster.itemA.id)
        XCTAssertEqual(request.itemVariants, [nil, StubMaster.itemB.id])

        // もう一度入れ替えると元の種族。特殊技は alpha も覚えるので残る
        await viewModel.swapSides()
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
        XCTAssertEqual(viewModel.defenderSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveId, StubMaster.specialMove.id)
    }

    // MARK: - 最新の要求だけを反映する

    /// 応答で見分けられる行を返す(presetLabel に目印を入れる)。
    private func markedResult(_ mark: String, for request: BulkCalcRequest) -> BulkCalcResult {
        BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: [
            BulkCalcRow(preset: .hp, presetLabel: mark, itemId: nil, result: StubPokeCalcService.echoCalcResult),
        ])
    }

    /// load を保留モードで済ませ、要求 0 に応答して待つ。
    private func loadWithManualStub(_ stub: StubPokeCalcService) async throws -> CalcViewModel {
        await stub.setBulkMode(.manual)
        let viewModel = CalcViewModel(service: stub)
        let loadTask = Task { await viewModel.load() }
        try await stub.waitForBulkRequests(count: 1)
        XCTAssertTrue(viewModel.isLoading, "計算の応答待ちの間は読み込み中")
        await stub.resolveBulkWithEcho(at: 0)
        await loadTask.value
        XCTAssertFalse(viewModel.isLoading)
        return viewModel
    }

    func testOlderResponseArrivingLaterDoesNotOverwriteNewer() async throws {
        let stub = StubMaster.makeService()
        let viewModel = try await loadWithManualStub(stub)

        let older = Task { await viewModel.selectAttackerPreset(.none) }
        try await stub.waitForBulkRequests(count: 2)
        let newer = Task { await viewModel.selectAttackerPreset(.aMax) }
        try await stub.waitForBulkRequests(count: 3)
        let requests = await stub.bulkRequests

        // 新しい要求が先に返る
        await stub.resolveBulk(at: 2, with: .success(markedResult("テスト新", for: requests[2])))
        await newer.value
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), ["テスト新"])
        XCTAssertFalse(viewModel.isLoading, "最新の要求が返ったら読み込み中を解く")

        // 古い要求が後から返っても上書きしない
        await stub.resolveBulk(at: 1, with: .success(markedResult("テスト旧", for: requests[1])))
        await older.value
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), ["テスト新"])
        XCTAssertEqual(viewModel.attackerPreset, .aMax)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertNil(viewModel.error)
    }

    func testOlderFailureArrivingLaterDoesNotSetError() async throws {
        let stub = StubMaster.makeService()
        let viewModel = try await loadWithManualStub(stub)

        let older = Task { await viewModel.selectAttackerPreset(.none) }
        try await stub.waitForBulkRequests(count: 2)
        let newer = Task { await viewModel.selectAttackerPreset(.aMax) }
        try await stub.waitForBulkRequests(count: 3)
        let requests = await stub.bulkRequests

        await stub.resolveBulk(at: 2, with: .success(markedResult("テスト新", for: requests[2])))
        await newer.value
        await stub.resolveBulk(at: 1, with: .failure(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト古い失敗")))
        await older.value
        XCTAssertNil(viewModel.error, "古い要求の失敗で最新の結果を消さない")
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), ["テスト新"])
    }

    func testOlderSuccessDoesNotClearNewerPendingLoadingState() async throws {
        let stub = StubMaster.makeService()
        let viewModel = try await loadWithManualStub(stub)

        let older = Task { await viewModel.selectAttackerPreset(.none) }
        try await stub.waitForBulkRequests(count: 2)
        let newer = Task { await viewModel.selectAttackerPreset(.aMax) }
        try await stub.waitForBulkRequests(count: 3)
        let requests = await stub.bulkRequests

        // 古い要求が先に返る: 反映しない。最新がまだなので読み込み中のまま
        await stub.resolveBulk(at: 1, with: .success(markedResult("テスト旧", for: requests[1])))
        await older.value
        XCTAssertNotEqual(viewModel.rows.map(\.presetLabel), ["テスト旧"])
        XCTAssertTrue(viewModel.isLoading)

        await stub.resolveBulk(at: 2, with: .success(markedResult("テスト新", for: requests[2])))
        await newer.value
        XCTAssertEqual(viewModel.rows.map(\.presetLabel), ["テスト新"])
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - エラー

    func testCalcFailureBecomesScreenErrorAndClearsRows() async throws {
        let cases: [(name: String, error: PokeCalcError, expected: CalcScreenError)] = [
            ("API 未対応", PokeCalcError(code: PokeCalcError.Code.apiUnsupported, message: "テスト"), .apiUnsupported),
            ("通信失敗", PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト"), .transport),
            ("応答の形が不正", PokeCalcError(code: PokeCalcError.Code.decode, message: "テスト"), .unexpectedResponse),
            ("サーバーの code", PokeCalcError(code: "invalid_input", message: "テスト入力が不正"),
             .service(code: "invalid_input", message: "テスト入力が不正")),
        ]
        for c in cases {
            let stub = StubMaster.makeService()
            let viewModel = await loadedViewModel(stub)
            XCTAssertFalse(viewModel.rows.isEmpty, c.name)

            let failure = c.error
            await stub.setBulkResponder { _ in .failure(failure) }
            await viewModel.selectAttackerPreset(.none)
            XCTAssertEqual(viewModel.error, c.expected, c.name)
            XCTAssertTrue(viewModel.rows.isEmpty, "失敗したら古い結果を出したままにしない(\(c.name))")
            XCTAssertFalse(viewModel.isLoading, c.name)

            // 次の成功でエラーは消える
            await stub.setBulkResponder { request in .success(StubPokeCalcService.echoResult(for: request)) }
            await viewModel.selectAttackerPreset(.aMax)
            XCTAssertNil(viewModel.error, c.name)
            XCTAssertFalse(viewModel.rows.isEmpty, c.name)
        }
    }

    func testMasterLoadFailureSetsErrorWithoutCalculating() async {
        let stub = StubMaster.makeService()
        await stub.setMasterError(PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト"))
        let viewModel = await loadedViewModel(stub)
        XCTAssertEqual(viewModel.error, .transport)
        XCTAssertFalse(viewModel.isLoading)
        XCTAssertTrue(viewModel.rows.isEmpty)
        let count = await stub.bulkRequests.count
        XCTAssertEqual(count, 0)
    }

    func testMissingPresetNatureSetsErrorWithoutCalculating() async {
        // 物理技の A特化に要る (+atk, -spa) が一覧に無い
        let stub = StubMaster.makeService(natures: [StubMaster.neutralNature, StubMaster.spaUpNature])
        let viewModel = await loadedViewModel(stub)
        guard case .service(let code, _) = viewModel.error else {
            return XCTFail("性格が無いエラーにならない: \(String(describing: viewModel.error))")
        }
        XCTAssertEqual(code, PokeCalcError.Code.natureUnavailable)
        let count = await stub.bulkRequests.count
        XCTAssertEqual(count, 0, "要求を組み立てられないときは計算しない")
    }

    func testTooFewSpeciesSetsError() async {
        // 攻撃側と防御側に2種族が要る
        let stub = StubMaster.makeService(species: [StubMaster.alpha])
        let viewModel = await loadedViewModel(stub)
        XCTAssertNotNil(viewModel.error)
        let count = await stub.bulkRequests.count
        XCTAssertEqual(count, 0)
    }

    func testAttackerLearnsetEmptyAfterMasterFilterSetsErrorWithoutCalculating() async {
        // learnset の技がマスタに1つも無い(moveOptions が空になる)。
        let stub = StubMaster.makeService(species: [StubMaster.unknownMovesOnly, StubMaster.beta])
        let viewModel = await loadedViewModel(stub)
        guard case .service(let code, _) = viewModel.error else {
            return XCTFail("技が無いエラーにならない: \(String(describing: viewModel.error))")
        }
        XCTAssertEqual(code, PokeCalcError.Code.moveUnavailable)
        let count = await stub.bulkRequests.count
        XCTAssertEqual(count, 0, "技を選べないときは計算しない")
    }

    // MARK: - 世代の保護(M1・M2 批評対応: species(key:) の応答も世代で守る)

    func testStaleSpeciesDetailDoesNotOverwriteNewerAttackerSelection() async throws {
        // 攻撃側を alpha → gamma → beta と連続で変え、gamma(古い方)の species 応答が
        // beta(新しい方)より後に届いても、moveOptions・最後の calcBulk は beta のものであること。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        // `load()` 自体も攻撃側(alpha)の species(key:) を1回呼んでいるので、そのぶんを基準にする
        // (以後の添字は `baseline + 1` = gamma、`baseline + 2` = beta)。
        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)

        let selectGamma = Task { await viewModel.selectAttacker(speciesKey: StubMaster.gamma.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        let selectBeta = Task { await viewModel.selectAttacker(speciesKey: StubMaster.beta.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 2)

        // 新しい方(beta)を先に解決する。
        let betaDetail = try await stub.lookupSpecies(key: StubMaster.beta.key)
        await stub.resolveSpecies(at: baseline + 1, with: .success(betaDetail))
        await selectBeta.value
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key)
        XCTAssertEqual(viewModel.moveOptions.map(\.id),
                       [StubMaster.statusMove.id, StubMaster.specialMove.id, StubMaster.physicalMove.id])
        var lastRequestSoFar = try await lastRequest(stub)
        XCTAssertEqual(lastRequestSoFar.attacker.speciesKey, StubMaster.beta.key)

        // 古い方(gamma)が後から解決しても、何も上書きしない。
        let gammaDetail = try await stub.lookupSpecies(key: StubMaster.gamma.key)
        await stub.resolveSpecies(at: baseline, with: .success(gammaDetail))
        await selectGamma.value
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.beta.key, "古い species 応答で攻撃側を上書きしない")
        XCTAssertEqual(viewModel.moveOptions.map(\.id),
                       [StubMaster.statusMove.id, StubMaster.specialMove.id, StubMaster.physicalMove.id],
                       "古い species 応答で moveOptions を上書きしない")
        lastRequestSoFar = try await lastRequest(stub)
        XCTAssertEqual(lastRequestSoFar.attacker.speciesKey, StubMaster.beta.key, "最後の要求は beta のまま")
        XCTAssertEqual(lastRequestSoFar.moveId, viewModel.moveId)

        let requests = await stub.bulkRequests
        XCTAssertEqual(requests.count, 2, "gamma の選択は古い species 応答が来ても計算を呼ばない(load分と beta 分だけ)")
    }

    func testIsLoadingWhileWaitingOnAttackerSpeciesDetail() async throws {
        // `species(key:)`(learnset の読み直し)の応答待ちの間も `isLoading` が true であること
        // (`calcBulk` の応答待ちだけでなく、その手前も同じ1回の操作として読み込み中にする)。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        XCTAssertFalse(viewModel.isLoading)

        let baseline = await stub.speciesRequests.count
        await stub.setSpeciesMode(.manual)
        let selectGamma = Task { await viewModel.selectAttacker(speciesKey: StubMaster.gamma.key) }
        try await stub.waitForSpeciesRequests(count: baseline + 1)
        XCTAssertTrue(viewModel.isLoading, "species(key:) の応答待ちの間も読み込み中のはず")

        let gammaDetail = try await stub.lookupSpecies(key: StubMaster.gamma.key)
        await stub.resolveSpecies(at: baseline, with: .success(gammaDetail))
        await selectGamma.value
        XCTAssertFalse(viewModel.isLoading, "計算まで終われば読み込み中は解ける")
    }

    func testStaleCalcSuccessDoesNotClearNewerSpeciesFailureError() async throws {
        // 保留中の古い計算があるとき、新しい攻撃側選択が species() で失敗する → error が立つ。
        // その後で古い計算の成功応答が届いても、error を消さず rows も空のままであること。
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        await stub.setBulkMode(.manual)
        let olderCalc = Task { await viewModel.selectDefender(speciesKey: StubMaster.gamma.key) }
        try await stub.waitForBulkRequests(count: 2)

        let failure = PokeCalcError(code: PokeCalcError.Code.transport, message: "テスト species 失敗")
        await stub.setMasterError(failure)
        await viewModel.selectAttacker(speciesKey: StubMaster.beta.key)

        XCTAssertEqual(viewModel.error, .transport)
        XCTAssertTrue(viewModel.rows.isEmpty)

        let requests = await stub.bulkRequests
        await stub.resolveBulk(at: 1, with: .success(StubPokeCalcService.echoResult(for: requests[1])))
        await olderCalc.value

        XCTAssertEqual(viewModel.error, .transport, "古い計算の成功でエラーを消さない")
        XCTAssertTrue(viewModel.rows.isEmpty, "古い計算の成功で行を出さない")
        XCTAssertFalse(viewModel.isLoading)
    }

    // MARK: - load() の二重呼び出し

    func testLoadCalledTwiceOnlyLoadsOnce() async throws {
        let stub = StubMaster.makeService()
        let viewModel = CalcViewModel(service: stub)
        await viewModel.load()
        await viewModel.load()

        let count = await stub.bulkRequests.count
        XCTAssertEqual(count, 1, "2回目の load() は何もしない")
        XCTAssertEqual(viewModel.attackerSpeciesKey, StubMaster.alpha.key)
    }

    // MARK: - 技の要約・相性(M4)

    private nonisolated static func rowWithEffectiveness(_ value: Double, preset: DefenderPreset, label: String) -> BulkCalcRow {
        let result = CalcResult(
            rolls: Array(repeating: 10, count: 16), minDamage: 10, maxDamage: 10,
            minPercent: 10.0, maxPercent: 10.0, defenderHP: 100, effectiveness: value, stab: false,
            ko: KOChance(hits: 10, guaranteed: true, chancePercent: 0, displayChancePercent: 100)
        )
        return BulkCalcRow(preset: preset, presetLabel: label, itemId: nil, result: result)
    }

    func testMoveEffectivenessIsTheUniformRowValueOrNilWhenMixed() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)

        for value in [0.0, 0.25, 1.0, 4.0] {
            await stub.setBulkResponder { request in
                .success(BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: [
                    Self.rowWithEffectiveness(value, preset: .none, label: "x"),
                    Self.rowWithEffectiveness(value, preset: .hp, label: "y"),
                ]))
            }
            await viewModel.selectAttackerPreset(.aMax)
            XCTAssertEqual(viewModel.moveEffectiveness, value, "\(value)")
        }

        await stub.setBulkResponder { request in
            .success(BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: [
                Self.rowWithEffectiveness(1, preset: .none, label: "x"),
                Self.rowWithEffectiveness(2, preset: .hp, label: "y"),
            ]))
        }
        await viewModel.selectAttackerPreset(.none)
        XCTAssertNil(viewModel.moveEffectiveness, "行ごとに値が割れているときは言い切らない")
    }

    func testMoveSummaryTextIncludesPowerCategoryAndEffectiveness() async throws {
        let stub = StubMaster.makeService()
        let viewModel = await loadedViewModel(stub)
        // 既定の技は alphaOnlyMove(威力40・物理)。
        await stub.setBulkResponder { request in
            .success(BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: [
                Self.rowWithEffectiveness(2, preset: .none, label: "x"),
            ]))
        }
        await viewModel.selectAttackerPreset(.aMax)
        XCTAssertEqual(viewModel.selectedMove?.id, StubMaster.alphaOnlyMove.id)
        XCTAssertEqual(viewModel.moveSummaryText, "威力40 / 物理 / ばつぐん(×2)")
    }
}

/// 画面用のエラー(PokeCalcError → 表示の種類と文言)。
final class CalcScreenErrorTests: XCTestCase {

    private struct OtherError: Error {}

    func testMappingFromErrors() {
        let cases: [(name: String, error: any Error, expected: CalcScreenError)] = [
            ("API 未対応", PokeCalcError(code: PokeCalcError.Code.apiUnsupported, message: "m"), .apiUnsupported),
            ("通信失敗", PokeCalcError(code: PokeCalcError.Code.transport, message: "m"), .transport),
            ("デコード失敗", PokeCalcError(code: PokeCalcError.Code.decode, message: "m"), .unexpectedResponse),
            ("サーバーの not_found", PokeCalcError(code: "not_found", message: "テストが無い"),
             .service(code: "not_found", message: "テストが無い")),
            ("クライアントの性格なし", PokeCalcError(code: PokeCalcError.Code.natureUnavailable, message: "テスト"),
             .service(code: PokeCalcError.Code.natureUnavailable, message: "テスト")),
            // サービスは PokeCalcError だけを投げる契約だが、それ以外が来ても画面を止めない
            ("PokeCalcError 以外", OtherError(), .unexpectedResponse),
        ]
        for c in cases {
            XCTAssertEqual(CalcScreenError(c.error), c.expected, c.name)
        }
    }

    func testMessagesAreShowableAndDistinguishable() {
        let apiUnsupported = CalcScreenError.apiUnsupported.message
        let transport = CalcScreenError.transport.message
        let unexpected = CalcScreenError.unexpectedResponse.message
        let service = CalcScreenError.service(code: "test_code", message: "テストの説明").message
        for message in [apiUnsupported, transport, unexpected, service] {
            XCTAssertFalse(message.isEmpty)
        }
        XCTAssertEqual(Set([apiUnsupported, transport, unexpected, service]).count, 4, "種類ごとに別の文言")
        // サーバーのエラーは原因が分かるように code と説明を含める
        XCTAssertTrue(service.contains("test_code"), service)
        XCTAssertTrue(service.contains("テストの説明"), service)
    }
}
