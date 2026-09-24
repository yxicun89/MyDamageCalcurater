import Foundation
import XCTest

@testable import PokeCalcCore

/// 画面の状態(`CalcViewModel`)のテスト用の `PokeCalcService`(P6-2a)。
///
/// - マスタは架空データをテストごとに差し込む(モックの JSON の数値に依存しない)。
/// - `calcBulk` の要求をすべて記録する。
/// - `bulkMode = .manual` のとき、`calcBulk` は応答を保留し、テストが `resolveBulk(at:with:)` で
///   **任意の順序**で応答を返す(古い要求の応答が後から届く状況を再現する)。
/// - マスタの読み込みを失敗させられる(`masterError`)。
/// - `reverse` は既定で呼ばれたら失敗(`.disallowed`。計算画面が逆算を呼ばないことの確認を保つ)。
///   逆算画面のテスト(P6-2b)は `setReverseMode(.immediate)` / `.manual` で有効にし、要求を記録する。
actor StubPokeCalcService: PokeCalcService {
    enum BulkMode {
        /// 要求を受けたらすぐ `bulkResponder` の結果を返す。
        case immediate
        /// 応答を保留する。テストが `resolveBulk(at:with:)` で返す。
        case manual
    }

    enum SpeciesMode {
        /// 要求を受けたらすぐ見つかった詳細(または not_found)を返す。
        case immediate
        /// 応答を保留する。テストが `resolveSpecies(at:with:)` で返す
        /// (`species(key:)` の応答が世代を追い越して届く状況を再現する。M1)。
        case manual
    }

    /// `searchSpecies` / `searchMoves` の1回の呼び出しの記録(issue #68: 検索語と上限が契約どおり
    /// 渡っていることと、余計な再取得をしていないことを固定するため)。
    struct SearchCall: Equatable, Sendable {
        let query: String
        let limit: Int
    }

    /// 検索(`searchSpecies` / `searchMoves`)の応答の返し方。`species(key:)` の `SpeciesMode` と同じ規則。
    enum SearchMode {
        /// 要求を受けたらすぐ架空マスタを前方一致で絞って返す。
        case immediate
        /// 応答を保留する。テストが `resolveSpeciesSearch(at:with:)` / `resolveMoveSearch(at:with:)` で
        /// **任意の順序**で返す(新しい検索の応答が古い検索より先に届く状況を再現する)。
        case manual
    }

    enum ReverseMode {
        /// 呼ばれたらテストを失敗させる(P6-2a の計算画面は reverse を使わない)。
        case disallowed
        /// 要求を受けたらすぐ `reverseResponder` の結果を返す。
        case immediate
        /// 応答を保留する。テストが `resolveReverse(at:with:)` で返す。
        case manual
    }

    /// 呼び出しの記録と保留の待ち合わせで、条件がそろうまで待つ上限(1ms × 回数)。
    /// ここを超えたら実装が要求を出していないとみなしてテストを失敗させる(無限に待たない)。
    /// CI 環境の遅延でのフレークを避けるため余裕を持たせてある。
    private static let waitPollLimit = 10000

    private let speciesDetails: [SpeciesDetail]
    private let moves: [Move]
    private let items: [Item]
    private let natureList: [Nature]

    private var bulkMode: BulkMode = .immediate
    /// `.immediate` のときの応答。既定は要求の形をそのまま写した行(`StubPokeCalcService.echoRows`)。
    private var bulkResponder: @Sendable (BulkCalcRequest) -> Result<BulkCalcResult, PokeCalcError> = { request in
        .success(StubPokeCalcService.echoResult(for: request))
    }
    /// マスタ参照(searchSpecies / species / searchMoves / searchItems / natures)をすべて失敗させる。
    private var masterError: PokeCalcError?
    private var speciesMode: SpeciesMode = .immediate

    private var reverseMode: ReverseMode = .disallowed
    /// `.immediate` のときの応答。既定は要求の形を写した候補(`StubPokeCalcService.echoReverseResult`)。
    private var reverseResponder: @Sendable (ReverseRequest) -> Result<ReverseResult, PokeCalcError> = { request in
        .success(StubPokeCalcService.echoReverseResult(for: request))
    }
    private(set) var reverseRequests: [ReverseRequest] = []
    private var pendingReverse: [Int: CheckedContinuation<ReverseResult, any Error>] = [:]

    private(set) var bulkRequests: [BulkCalcRequest] = []
    private var pendingBulk: [Int: CheckedContinuation<BulkCalcResult, any Error>] = [:]
    /// `species(key:)` に渡された `key` の記録(呼ばれた順)。
    private(set) var speciesRequests: [String] = []
    private var pendingSpecies: [Int: CheckedContinuation<SpeciesDetail, any Error>] = [:]

    /// `searchSpecies` / `searchMoves` の記録(呼ばれた順)と保留(issue #68)。
    private var speciesSearchMode: SearchMode = .immediate
    private(set) var speciesSearchCalls: [SearchCall] = []
    private var pendingSpeciesSearch: [Int: CheckedContinuation<[SpeciesSummary], any Error>] = [:]
    private var moveSearchMode: SearchMode = .immediate
    private(set) var moveSearchCalls: [SearchCall] = []
    private var pendingMoveSearch: [Int: CheckedContinuation<[Move], any Error>] = [:]

    init(species: [SpeciesDetail], moves: [Move], items: [Item], natures: [Nature]) {
        speciesDetails = species
        self.moves = moves
        self.items = items
        natureList = natures
    }

    // MARK: - テストからの操作

    func setBulkMode(_ mode: BulkMode) {
        bulkMode = mode
    }

    func setBulkResponder(_ responder: @escaping @Sendable (BulkCalcRequest) -> Result<BulkCalcResult, PokeCalcError>) {
        bulkResponder = responder
    }

    func setReverseMode(_ mode: ReverseMode) {
        reverseMode = mode
    }

    func setReverseResponder(_ responder: @escaping @Sendable (ReverseRequest) -> Result<ReverseResult, PokeCalcError>) {
        reverseResponder = responder
    }

    /// 保留中の `index` 番目(0 始まり、`reverseRequests` の添字)の `reverse` に応答する。
    func resolveReverse(at index: Int, with result: Result<ReverseResult, PokeCalcError>) {
        guard let continuation = pendingReverse.removeValue(forKey: index) else {
            XCTFail("保留中の reverse が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// 保留中の `index` 番目の `reverse` に、要求の形を写した既定の応答を返す。
    func resolveReverseWithEcho(at index: Int) {
        guard index < reverseRequests.count else {
            XCTFail("reverse の要求が無い: index \(index)")
            return
        }
        resolveReverse(at: index, with: .success(Self.echoReverseResult(for: reverseRequests[index])))
    }

    // MARK: - テストからの操作(キャンセルの観測。issue #113)

    /// Task cancel を受け取った `reverse` の要求番号(`reverseRequests` の添字)。
    private(set) var cancelledReverseRequests: Set<Int> = []
    /// Task cancel を受け取った `calcBulk` の要求番号(`bulkRequests` の添字)。
    private(set) var cancelledBulkRequests: Set<Int> = []
    /// Task cancel を受け取った `species(key:)` の要求番号(`speciesRequests` の添字。critic 指摘:
    /// master 取得中のキャンセルも `reverse`/`calcBulk` と同じ形で観測できる必要がある)。
    private(set) var cancelledSpeciesRequests: Set<Int> = []

    /// 保留中(`.manual`)の `reverse` が Task cancel を受けたときの処理。
    /// `APIPokeCalcService` が URLSession のキャンセルを `CancellationError` として投げ直すのと
    /// 同じ形を作る(ViewModel から見た「送信済みの要求が cancel された」状況)。
    func cancelPendingReverse(at index: Int) {
        cancelledReverseRequests.insert(index)
        if let continuation = pendingReverse.removeValue(forKey: index) {
            continuation.resume(throwing: CancellationError())
        }
    }

    /// `cancelPendingReverse` の `calcBulk` 版。
    func cancelPendingBulk(at index: Int) {
        cancelledBulkRequests.insert(index)
        if let continuation = pendingBulk.removeValue(forKey: index) {
            continuation.resume(throwing: CancellationError())
        }
    }

    /// `cancelPendingReverse` の `species(key:)` 版。
    func cancelPendingSpecies(at index: Int) {
        cancelledSpeciesRequests.insert(index)
        if let continuation = pendingSpecies.removeValue(forKey: index) {
            continuation.resume(throwing: CancellationError())
        }
    }

    /// `index` 番目の `reverse` が cancel されるまで待つ(壁時計の固定待ちをしない。1ms ポーリング)。
    func waitForReverseCancellation(at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if cancelledReverseRequests.contains(index) { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("reverse の要求 \(index) が cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "reverse のキャンセル待ちがタイムアウト")
    }

    /// `waitForReverseCancellation` の `calcBulk` 版。
    func waitForBulkCancellation(at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if cancelledBulkRequests.contains(index) { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("calcBulk の要求 \(index) が cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "calcBulk のキャンセル待ちがタイムアウト")
    }

    /// `waitForReverseCancellation` の `species(key:)` 版。
    func waitForSpeciesCancellation(at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if cancelledSpeciesRequests.contains(index) { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("species の要求 \(index) が cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "species のキャンセル待ちがタイムアウト")
    }

    /// `reverse` がちょうど `count` 回以上呼ばれるまで待つ。
    func waitForReverseRequests(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if reverseRequests.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("reverse が \(count) 回呼ばれなかった(\(reverseRequests.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "reverse の待ち合わせがタイムアウト")
    }

    func setMasterError(_ error: PokeCalcError?) {
        masterError = error
    }

    // MARK: - テストからの操作(検索。issue #68)

    func setSpeciesSearchMode(_ mode: SearchMode) {
        speciesSearchMode = mode
    }

    func setMoveSearchMode(_ mode: SearchMode) {
        moveSearchMode = mode
    }

    /// 保留中の `index` 番目(0 始まり、`speciesSearchCalls` の添字)の `searchSpecies` に応答する。
    func resolveSpeciesSearch(at index: Int, with result: Result<[SpeciesSummary], PokeCalcError>) {
        guard let continuation = pendingSpeciesSearch.removeValue(forKey: index) else {
            XCTFail("保留中の searchSpecies が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// 保留中の `index` 番目の `searchMoves` に応答する。
    func resolveMoveSearch(at index: Int, with result: Result<[Move], PokeCalcError>) {
        guard let continuation = pendingMoveSearch.removeValue(forKey: index) else {
            XCTFail("保留中の searchMoves が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// `searchSpecies` が `count` 回以上呼ばれるまで待つ。
    func waitForSpeciesSearchCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if speciesSearchCalls.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("searchSpecies が \(count) 回呼ばれなかった(\(speciesSearchCalls.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "searchSpecies の待ち合わせがタイムアウト")
    }

    /// `searchMoves` が `count` 回以上呼ばれるまで待つ。
    func waitForMoveSearchCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if moveSearchCalls.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("searchMoves が \(count) 回呼ばれなかった(\(moveSearchCalls.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "searchMoves の待ち合わせがタイムアウト")
    }

    /// `.immediate` の既定応答、および手動モードでテストが `resolveSpeciesSearch` に渡す値を組み立てるのに使う。
    func matchedSpecies(query: String, limit: Int) -> [SpeciesSummary] {
        speciesDetails
            .filter { query.isEmpty || $0.nameJa.hasPrefix(query) }
            .prefix(limit)
            .map { SpeciesSummary(key: $0.key, dexNo: $0.dexNo, form: $0.form, nameJa: $0.nameJa, types: $0.types) }
    }

    /// `matchedSpecies` の技版。
    func matchedMoves(query: String, limit: Int) -> [Move] {
        Array(moves.filter { query.isEmpty || $0.nameJa.hasPrefix(query) }.prefix(limit))
    }

    func setSpeciesMode(_ mode: SpeciesMode) {
        speciesMode = mode
    }

    /// 保留中の `index` 番目(0 始まり、`speciesRequests` の添字)の `species(key:)` に応答する。
    func resolveSpecies(at index: Int, with result: Result<SpeciesDetail, PokeCalcError>) {
        guard let continuation = pendingSpecies.removeValue(forKey: index) else {
            XCTFail("保留中の species が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// `species(key:)` がちょうど `count` 回以上呼ばれるまで待つ。
    func waitForSpeciesRequests(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if speciesRequests.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("species が \(count) 回呼ばれなかった(\(speciesRequests.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "species の待ち合わせがタイムアウト")
    }

    /// 保留中の `index` 番目(0 始まり、`bulkRequests` の添字)の要求に応答する。
    func resolveBulk(at index: Int, with result: Result<BulkCalcResult, PokeCalcError>) {
        guard let continuation = pendingBulk.removeValue(forKey: index) else {
            XCTFail("保留中の calcBulk が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// 保留中の `index` 番目の要求に、要求の形を写した既定の応答を返す。
    func resolveBulkWithEcho(at index: Int) {
        guard index < bulkRequests.count else {
            XCTFail("calcBulk の要求が無い: index \(index)")
            return
        }
        resolveBulk(at: index, with: .success(Self.echoResult(for: bulkRequests[index])))
    }

    /// `calcBulk` がちょうど `count` 回以上呼ばれるまで待つ。
    func waitForBulkRequests(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if bulkRequests.count >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("calcBulk が \(count) 回呼ばれなかった(\(bulkRequests.count) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "calcBulk の待ち合わせがタイムアウト")
    }

    // MARK: - 既定の応答

    /// テストで使う決め打ちの計算結果。数値に意味は無い(ViewModel はそのまま整形するだけ)。
    static let echoCalcResult = CalcResult(
        rolls: Array(repeating: 10, count: 16), minDamage: 10, maxDamage: 10,
        minPercent: 10.0, maxPercent: 10.0, defenderHP: 100, effectiveness: 1, stab: false, category: .physical,
        ko: KOChance(hits: 10, guaranteed: true, chancePercent: 0, displayChancePercent: 100)
    )

    /// 要求の形(presets 省略時は `none` の1つ × itemVariants。空は素の1通り)を写した結果。
    static func echoResult(for request: BulkCalcRequest) -> BulkCalcResult {
        let presets = request.presets.isEmpty ? [DefenderPreset.none] : request.presets
        let variants = request.itemVariants.isEmpty ? [String?.none] : request.itemVariants
        var rows: [BulkCalcRow] = []
        for preset in presets {
            for itemId in variants {
                rows.append(BulkCalcRow(
                    preset: preset, presetLabel: "テスト調整-\(preset.rawValue)", itemId: itemId, defender: testBulkDefender, result: echoCalcResult
                ))
            }
        }
        return BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: rows)
    }

    /// 要求の形(性格クラス × 持ち物候補。空は「持ち物なし」の1通り)を写した逆算の結果。
    /// 数値に意味は無い(ViewModel は結果を並べ替えずに整形するだけ)。`stat` は側だけで決め打ちする。
    /// ADR-0010 §R1: 逆算は相手の H の SP を defender=32・attacker=0 と仮定する(サーバーの値を写すだけ)。
    static let echoAssumedDefenderHPSP = 32
    static let echoAssumedAttackerHPSP = 0

    static func echoReverseResult(for request: ReverseRequest) -> ReverseResult {
        let items = request.itemCandidates.isEmpty ? [String?.none] : request.itemCandidates
        var candidates: [ReverseCandidate] = []
        for natureClass in NatureClass.allCases {
            for itemId in items {
                candidates.append(ReverseCandidate(
                    natureClass: natureClass, nature: NatureModifier(), natureId: nil, itemId: itemId,
                    ranges: [SPRange(min: 10, max: 12)], spCount: 3,
                    exact: true, mismatch: 0, support: 1, minPercent: 10.0, maxPercent: 12.0
                ))
            }
        }
        return ReverseResult(
            side: request.side, stat: request.side == .defender ? .def : .atk,
            assumedHPSP: request.side == .defender ? echoAssumedDefenderHPSP : echoAssumedAttackerHPSP,
            candidates: candidates, exactCount: candidates.count
        )
    }

    // MARK: - PokeCalcService

    func searchSpecies(query: String, limit: Int) async throws -> [SpeciesSummary] {
        let index = speciesSearchCalls.count
        speciesSearchCalls.append(SearchCall(query: query, limit: limit))
        if let masterError { throw masterError }
        switch speciesSearchMode {
        case .immediate:
            return matchedSpecies(query: query, limit: limit)
        case .manual:
            return try await withCheckedThrowingContinuation { continuation in
                pendingSpeciesSearch[index] = continuation
            }
        }
    }

    func species(key: String) async throws -> SpeciesDetail {
        if let masterError { throw masterError }
        let index = speciesRequests.count
        speciesRequests.append(key)
        switch speciesMode {
        case .immediate:
            return try lookupSpecies(key: key)
        case .manual:
            // 保留中に Task cancel を受けたら `CancellationError` を投げて終える(issue #113。
            // `reverse`/`calcBulk` と同じ形。critic 指摘: master 取得中のキャンセルも観測できる必要がある)。
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<SpeciesDetail, any Error>) in
                    if cancelledSpeciesRequests.contains(index) {
                        continuation.resume(throwing: CancellationError())
                    } else {
                        pendingSpecies[index] = continuation
                    }
                }
            } onCancel: {
                Task { await self.cancelPendingSpecies(at: index) }
            }
        }
    }

    /// `.immediate` の既定応答、および手動モードでテストが `resolveSpecies` に渡す値を組み立てるのに使う。
    func lookupSpecies(key: String) throws -> SpeciesDetail {
        guard let detail = speciesDetails.first(where: { $0.key == key }) else {
            throw PokeCalcError(code: "not_found", message: "テスト: 種族が無い \(key)")
        }
        return detail
    }

    func searchMoves(query: String, limit: Int) async throws -> [Move] {
        let index = moveSearchCalls.count
        moveSearchCalls.append(SearchCall(query: query, limit: limit))
        if let masterError { throw masterError }
        switch moveSearchMode {
        case .immediate:
            return matchedMoves(query: query, limit: limit)
        case .manual:
            return try await withCheckedThrowingContinuation { continuation in
                pendingMoveSearch[index] = continuation
            }
        }
    }

    func searchItems(query: String, limit: Int) async throws -> [Item] {
        if let masterError { throw masterError }
        return Array(items.filter { query.isEmpty || $0.nameJa.hasPrefix(query) }.prefix(limit))
    }

    func natures() async throws -> [Nature] {
        if let masterError { throw masterError }
        return natureList
    }

    func calcDamage(_ request: CalcRequest) async throws -> CalcResult {
        XCTFail("P6-2a の画面は calcDamage を使わない(一括計算だけ)")
        throw PokeCalcError(code: "test_unexpected", message: "calcDamage")
    }

    func calcBulk(_ request: BulkCalcRequest) async throws -> BulkCalcResult {
        let index = bulkRequests.count
        bulkRequests.append(request)
        switch bulkMode {
        case .immediate:
            return try bulkResponder(request).get()
        case .manual:
            // 保留中に Task cancel を受けたら `CancellationError` を投げて終える(issue #113。
            // 実装の `APIPokeCalcService` が URLSession のキャンセルでそうするのと同じ)。
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<BulkCalcResult, any Error>) in
                    if cancelledBulkRequests.contains(index) {
                        continuation.resume(throwing: CancellationError())
                    } else {
                        pendingBulk[index] = continuation
                    }
                }
            } onCancel: {
                Task { await self.cancelPendingBulk(at: index) }
            }
        }
    }

    func reverse(_ request: ReverseRequest) async throws -> ReverseResult {
        switch reverseMode {
        case .disallowed:
            XCTFail("P6-2a の画面は reverse を使わない")
            throw PokeCalcError(code: "test_unexpected", message: "reverse")
        case .immediate:
            reverseRequests.append(request)
            return try reverseResponder(request).get()
        case .manual:
            let index = reverseRequests.count
            reverseRequests.append(request)
            // `calcBulk` と同じ理由でキャンセルを観測する(issue #113)。
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<ReverseResult, any Error>) in
                    if cancelledReverseRequests.contains(index) {
                        continuation.resume(throwing: CancellationError())
                    } else {
                        pendingReverse[index] = continuation
                    }
                }
            } onCancel: {
                Task { await self.cancelPendingReverse(at: index) }
            }
        }
    }
}

// MARK: - 架空のマスタ(「テスト」で始まる名前。実在の名前・数値を使わない)

/// `CalcViewModelTests` の架空マスタ。ID は意味の分かる文字列にし、順序に意味を持たせる
/// (既定の選択は「一覧の最初」「learnset の最初のダメージ技」で決まるため)。
enum StubMaster {
    static let physicalMove = Move(id: "stub-move-physical", nameJa: "テストわざぶつり", type: .normal, category: .physical, power: 40)
    static let specialMove = Move(id: "stub-move-special", nameJa: "テストわざとくしゅ", type: .fire, category: .special, power: 40)
    static let statusMove = Move(id: "stub-move-status", nameJa: "テストわざへんか", type: .normal, category: .status, power: 0)
    /// 攻撃側 A の learnset にだけある物理技(入れ替え後に選び直されることの確認用)。
    static let alphaOnlyMove = Move(id: "stub-move-alpha-only", nameJa: "テストわざアルファ専用", type: .water, category: .physical, power: 40)

    /// 架空の特性(どの種族も同じものを持つ。`Individual.abilityId` を写していることの確認に使う。P6-2d)。
    static let ability = Ability(id: "stub-ability", nameJa: "テストとくせい")

    /// 種族ごとに異なる特性集合を持たせるための架空の特性(issue #100: 種族変更で `abilityId` が
    /// 新種族の候補と食い違わないことの確認用。`alpha`/`beta`/`gamma`/`statusOnly` はすべて同じ
    /// `ability` を共有するため、この確認には使えない)。
    static let abilityX = Ability(id: "stub-ability-x", nameJa: "テストとくせいX")
    static let abilityY = Ability(id: "stub-ability-y", nameJa: "テストとくせいY")

    /// learnset の先頭が変化技。既定の技は「最初のダメージ技」なので先頭は選ばれない。
    static let alpha = SpeciesDetail(
        key: "9101-000", dexNo: 9101, form: 0, nameJa: "テストアルファ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [ability],
        learnset: [statusMove.id, alphaOnlyMove.id, specialMove.id, "stub-move-unknown"]
    )
    static let beta = SpeciesDetail(
        key: "9102-000", dexNo: 9102, form: 0, nameJa: "テストベータ", types: [.fire],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [ability],
        learnset: [statusMove.id, specialMove.id, physicalMove.id]
    )
    static let gamma = SpeciesDetail(
        key: "9103-000", dexNo: 9103, form: 0, nameJa: "テストガンマ", types: [.water],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [ability],
        learnset: [physicalMove.id, specialMove.id]
    )
    /// 変化技しか覚えない(ダメージ技が無いときの既定の規則の確認用)。
    static let statusOnly = SpeciesDetail(
        key: "9104-000", dexNo: 9104, form: 0, nameJa: "テストへんかのみ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [ability],
        learnset: [statusMove.id]
    )
    /// learnset がマスタのどの技とも一致しない(`moveOptions` が空になる。`moveUnavailable` の確認用)。
    static let unknownMovesOnly = SpeciesDetail(
        key: "9105-000", dexNo: 9105, form: 0, nameJa: "テストみしらぬわざ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [ability],
        learnset: ["stub-move-does-not-exist"]
    )

    /// 特性Xだけを持つ(issue #100 用。`abilityX`/`abilityY` は種族間で重ならない)。
    static let abilityXOnly = SpeciesDetail(
        key: "9106-000", dexNo: 9106, form: 0, nameJa: "テストとくせいXのみ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [abilityX],
        learnset: [statusMove.id, physicalMove.id]
    )
    /// 特性Yだけを持つ(issue #100 用。`abilityXOnly` と重ならない)。
    static let abilityYOnly = SpeciesDetail(
        key: "9107-000", dexNo: 9107, form: 0, nameJa: "テストとくせいYのみ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [abilityY],
        learnset: [statusMove.id, physicalMove.id]
    )
    /// 特性XとYの両方を持つ(issue #100 用。「旧特性が新種族にもまだある」ケースの確認)。
    static let abilityXAndY = SpeciesDetail(
        key: "9108-000", dexNo: 9108, form: 0, nameJa: "テストとくせいXY", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [abilityX, abilityY],
        learnset: [statusMove.id, physicalMove.id]
    )
    /// 特性が無い(issue #100 用。フォールバック先が `nil` になるケースの確認)。
    static let noAbilities = SpeciesDetail(
        key: "9109-000", dexNo: 9109, form: 0, nameJa: "テストとくせいなし", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [],
        learnset: [statusMove.id, physicalMove.id]
    )

    static let itemA = Item(id: "stub-item-a", nameJa: "テストどうぐA")
    static let itemB = Item(id: "stub-item-b", nameJa: "テストどうぐB")

    static let neutralNature = Nature(id: "stub-nature-neutral", nameJa: "テストせいかく無補正")
    static let atkUpNature = Nature(id: "stub-nature-atk-up", nameJa: "テストせいかく攻撃上昇", plus: .atk, minus: .spa)
    static let spaUpNature = Nature(id: "stub-nature-spa-up", nameJa: "テストせいかく特攻上昇", plus: .spa, minus: .atk)
    /// 逆算画面の「受けたダメージ」で自分(防御側)の HB/HD 特化に使う上昇性格(ADR-0009: +def/-atk・+spd/-atk)。
    static let defUpNature = Nature(id: "stub-nature-def-up", nameJa: "テストせいかく防御上昇", plus: .def, minus: .atk)
    static let spdUpNature = Nature(id: "stub-nature-spd-up", nameJa: "テストせいかく特防上昇", plus: .spd, minus: .atk)

    /// 逆算画面のテスト用の性格一覧(攻撃・防御の両方の上昇性格を含む)。`makeService` の既定は変えない
    /// (計算画面のテストの前提を崩さないため)。
    static let reverseNatures = [atkUpNature, neutralNature, spaUpNature, defUpNature, spdUpNature]

    static func makeService(
        species: [SpeciesDetail] = [alpha, beta, gamma, statusOnly],
        moves: [Move] = [physicalMove, specialMove, statusMove, alphaOnlyMove],
        items: [Item] = [itemA, itemB],
        natures: [Nature] = [atkUpNature, neutralNature, spaUpNature]
    ) -> StubPokeCalcService {
        StubPokeCalcService(species: species, moves: moves, items: items, natures: natures)
    }
}

// MARK: - 検索の上限(issue #68)を実際に踏む架空マスタ

/// `MasterSearch.pageLimit` 件の「先頭ページに入る」架空マスタと、その**外側**に1件だけ置いた架空マスタ。
///
/// 空クエリの検索は `prefix(limit)` で切られるので、`hiddenSpecies` / `hiddenMove` は
/// **名前で検索したときだけ**返る。これが issue #68 の再現手順(先頭200件だけを返す fake service +
/// 201件目の技だけを持つ learnset)に相当する。アプリの架空データ(`Sources/PokeCalcCore/Resources/*.json`)は
/// 増やさない(実データの件数をモックに持ち込まないため。ADR-0501「issue #68」9章)。
enum StubBulkMaster {
    /// 先頭ページに入る詰め物の技(すべて物理。既定の技選択が成立するように変化技は混ぜない)。
    static func pageMove(_ index: Int) -> Move {
        Move(id: "stub-page-move-\(index)", nameJa: "テストページわざ\(index)", type: .normal, category: .physical, power: 40)
    }

    /// 先頭ページに入る詰め物の種族。learnset は同じ index の詰め物の技1つだけ(先頭ページの内側)。
    static func pageSpecies(_ index: Int) -> SpeciesDetail {
        SpeciesDetail(
            key: "8\(index)-000", dexNo: 8000 + index, form: 0, nameJa: "テストページしゅぞく\(index)", types: [.normal],
            baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
            abilities: [StubMaster.ability],
            learnset: [pageMove(index).id]
        )
    }

    /// 先頭ページの**外**にある技(名前で検索したときだけ返る)。
    static let hiddenMove = Move(id: "stub-hidden-move", nameJa: "テストかくれわざ", type: .water, category: .physical, power: 40)

    /// 先頭ページの**外**にある種族。learnset も先頭ページの外の技だけ(issue #68 の再現手順4)。
    static let hiddenSpecies = SpeciesDetail(
        key: "8999-000", dexNo: 8999, form: 0, nameJa: "テストかくれしゅぞく", types: [.water],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [StubMaster.ability],
        learnset: [hiddenMove.id]
    )

    /// learnset に先頭ページの内と外の技を1つずつ持つ種族(構築に保存済みの技の扱いの確認用)。
    /// この種族自体も先頭ページの外に置く(`species(key:)` は key 指定なので検索しなくても引ける)。
    static let mixedSpecies = SpeciesDetail(
        key: "8998-000", dexNo: 8998, form: 0, nameJa: "テストまざりしゅぞく", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [StubMaster.ability],
        learnset: [pageMove(0).id, hiddenMove.id]
    )

    /// `hiddenSpecies` だけに当たる検索語(前方一致。詰め物の名前とは接頭辞が重ならない)。
    static let hiddenSpeciesQuery = "テストかくれしゅぞく"
    /// `hiddenMove` だけに当たる検索語。
    static let hiddenMoveQuery = "テストかくれわざ"
    /// 詰め物すべてに当たる検索語(先頭ページと同じ集合が返る = 上限に達する)。
    static let pageSpeciesQuery = "テストページしゅぞく"

    static var pageSpeciesList: [SpeciesDetail] { (0..<MasterSearch.pageLimit).map(pageSpecies) }
    static var pageMoveList: [Move] { (0..<MasterSearch.pageLimit).map(pageMove) }

    /// 種族・技ともに「先頭ページ + その外の1件」。持ち物・性格は `StubMaster` のものをそのまま使う
    /// (どちらも上限内なので issue #68 の対象外)。
    static func makeService(natures: [Nature] = [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature]) -> StubPokeCalcService {
        StubPokeCalcService(
            species: pageSpeciesList + [mixedSpecies, hiddenSpecies],
            moves: pageMoveList + [hiddenMove],
            items: [StubMaster.itemA, StubMaster.itemB],
            natures: natures
        )
    }
}
