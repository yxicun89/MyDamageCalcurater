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
        if let masterError { throw masterError }
        let matched = speciesDetails.filter { query.isEmpty || $0.nameJa.hasPrefix(query) }
        return matched.prefix(limit).map {
            SpeciesSummary(key: $0.key, dexNo: $0.dexNo, form: $0.form, nameJa: $0.nameJa, types: $0.types)
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
            return try await withCheckedThrowingContinuation { continuation in
                pendingSpecies[index] = continuation
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
        if let masterError { throw masterError }
        return Array(moves.filter { query.isEmpty || $0.nameJa.hasPrefix(query) }.prefix(limit))
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
            return try await withCheckedThrowingContinuation { continuation in
                pendingBulk[index] = continuation
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
            return try await withCheckedThrowingContinuation { continuation in
                pendingReverse[index] = continuation
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

    /// learnset の先頭が変化技。既定の技は「最初のダメージ技」なので先頭は選ばれない。
    static let alpha = SpeciesDetail(
        key: "9101-000", dexNo: 9101, form: 0, nameJa: "テストアルファ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-ability", nameJa: "テストとくせい")],
        learnset: [statusMove.id, alphaOnlyMove.id, specialMove.id, "stub-move-unknown"]
    )
    static let beta = SpeciesDetail(
        key: "9102-000", dexNo: 9102, form: 0, nameJa: "テストベータ", types: [.fire],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-ability", nameJa: "テストとくせい")],
        learnset: [statusMove.id, specialMove.id, physicalMove.id]
    )
    static let gamma = SpeciesDetail(
        key: "9103-000", dexNo: 9103, form: 0, nameJa: "テストガンマ", types: [.water],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-ability", nameJa: "テストとくせい")],
        learnset: [physicalMove.id, specialMove.id]
    )
    /// 変化技しか覚えない(ダメージ技が無いときの既定の規則の確認用)。
    static let statusOnly = SpeciesDetail(
        key: "9104-000", dexNo: 9104, form: 0, nameJa: "テストへんかのみ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-ability", nameJa: "テストとくせい")],
        learnset: [statusMove.id]
    )
    /// learnset がマスタのどの技とも一致しない(`moveOptions` が空になる。`moveUnavailable` の確認用)。
    static let unknownMovesOnly = SpeciesDetail(
        key: "9105-000", dexNo: 9105, form: 0, nameJa: "テストみしらぬわざ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-ability", nameJa: "テストとくせい")],
        learnset: ["stub-move-does-not-exist"]
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
