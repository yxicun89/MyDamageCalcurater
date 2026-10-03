import Observation

// AdjustViewModel: 調整画面の状態(AJ7。ADR-0502 §2・§5)。Web の AJ6(ADR-0319)と同じ機能を持つ。
//
// - マスタ(種族・性格・持ち物・覚える技)は `PokeCalcService`、調整 API と技の逆引きは `AdjustService`。
// - API を呼ぶのは「調整する」(`submit` / `scheduleSubmit`)と「覚えるポケモン」「続きを読み込む」だけ。
//   `load()`・入力の変更では調整 API を呼ばない(ADR-0319 §4)。
// - 1回の送信で indices とモードの操作を並行に呼び、両方そろってから結果を出す。どちらかが失敗したら結果を出さず
//   日本語のエラーだけを出す(サーバーの message は出さない)。入力は消さない。
// - 送り直したら前の送信を取り消し(`LatestTaskRunner`)、遅れて届いた古い応答で上書きしない。

/// 調整の内容(Web の `AdjustModeKey` と同じ5つ。`CaseIterable` の順が画面の並び)。
public enum AdjustMode: String, CaseIterable, Sendable, Hashable {
    case indices
    case bulk
    case offense
    case minKo
    case minSurvive
}

/// 「覚えるポケモン」を開く技の側。
public enum AdjustMoveSide: String, CaseIterable, Sendable, Hashable {
    /// 自分の技。
    case own
    /// 相手の技。
    case opponent
}

/// モードの操作の結果(indices 以外)。表示に要る送信時の値(発数・回す側・目標の有無)を一緒に持つ。
public enum AdjustModeResult: Equatable, Sendable {
    case ko(AdjustKOResult, hits: Int)
    case survive(AdjustSurviveResult, hits: Int)
    case allocation(AdjustAllocationResult, mode: AllocMode, goalRequested: Bool, speedTargetRequested: Bool)
}

/// 1回の送信の結果(indices とモードの結果の両方がそろったときだけ作る)。
public struct AdjustOutcome: Equatable, Sendable {
    public var indices: AdjustIndicesResult
    /// モードが `.indices` のときは nil。
    public var modeResult: AdjustModeResult?
    /// モードの結果に付いた未対応の印の注記(`UnsupportedNoticeText.summary`。無ければ nil)。
    public var unsupportedNotice: String?

    public init(indices: AdjustIndicesResult, modeResult: AdjustModeResult?, unsupportedNotice: String?) {
        self.indices = indices
        self.modeResult = modeResult
        self.unsupportedNotice = unsupportedNotice
    }
}

/// 「この技を覚えるポケモン」の一覧の状態。
public struct AdjustLearnersState: Equatable, Sendable {
    public var side: AdjustMoveSide
    public var moveId: String
    /// 見出しに使う技名(`AdjustText.learnersHeading`)。
    public var moveName: String
    /// 読み込んだ種族(ページを読んだ順に連結)。
    public var species: [SpeciesSummary]
    /// 直前のページがちょうど `RequestLimits.moveLearnersPageSize` 件だった(「続きを読み込む」を出す)。
    public var canLoadMore: Bool
    public var isLoading: Bool
    /// 一覧の中に出すエラー(日本語。`AdjustText.errorMessage(for:)`)。
    public var errorMessage: String?

    public init(
        side: AdjustMoveSide, moveId: String, moveName: String, species: [SpeciesSummary] = [],
        canLoadMore: Bool = false, isLoading: Bool = false, errorMessage: String? = nil
    ) {
        self.side = side
        self.moveId = moveId
        self.moveName = moveName
        self.species = species
        self.canLoadMore = canLoadMore
        self.isLoading = isLoading
        self.errorMessage = errorMessage
    }
}

@MainActor
@Observable
public final class AdjustViewModel: MasterSpeciesSearchProviding {
    /// 確率のしきい値の選択肢(%)。先頭の 100(確定)が既定で、契約の既定と同じなので送らない。
    public static let thresholdOptions: [Double] = [100, 90, 75, 50]
    /// 発数の選択肢(1...`RequestLimits.maxAdjustHits`)。
    public static var hitsOptions: [Int] { Array(1...RequestLimits.maxAdjustHits) }
    /// 4096 基準の等倍(契約の `AdjustModifier` の既定)。
    public static let neutralModifier = 4096
    /// タイプ一致の補正(×1.5。ADR-0319 §5)。
    public static let stabModifier = 6144

    /// 回す能力の上限の既定(契約の既定と同じ。`SPLimits.maxPerStat`)。
    private static let defaultCeiling = SPLimits.maxPerStat
    private static let ceilingRange = 0...SPLimits.maxPerStat

    private let service: any PokeCalcService
    private let adjust: any AdjustService
    private let speciesSearch: MasterSpeciesSearchField
    /// 送信と「覚えるポケモン」の Task を1つずつだけ保持する(issue #113。画面を閉じたら `cancelPendingWork()`)。
    private let submitRunner = LatestTaskRunner()
    private let learnersRunner = LatestTaskRunner()
    /// 「最新の送信だけを反映する」世代番号(取り消しに失敗した古い応答で上書きしない)。
    private var submitGeneration = 0
    private var learnersGeneration = 0
    private var ownSpeciesGeneration = 0
    private var opponentSpeciesGeneration = 0
    /// 一度でも見た種族(検索結果・`species(key:)` の応答から合流する。`ownSpecies` / `opponentSpecies` を引く)。
    private var speciesDictionary: [String: SpeciesSummary] = [:]
    private var didLoad = false

    public init(service: any PokeCalcService, adjust: any AdjustService, searchDebounce: Duration = MasterSearch.debounceInterval) {
        self.service = service
        self.adjust = adjust
        speciesSearch = MasterSpeciesSearchField(debounce: searchDebounce) { query, limit in
            try await service.searchSpecies(query: query, limit: limit)
        }
    }

    // MARK: - マスタ

    public private(set) var natureOptions: [Nature] = []
    public private(set) var itemOptions: [Item] = []

    /// 性格・種族の先頭ページ・持ち物を読む。既定は「未選択」(自分・相手とも)。調整 API は呼ばない。
    public func load() async {
        guard !didLoad else { return }
        do {
            let natures = try await service.natures()
            let species = try await service.searchSpecies(query: "", limit: MasterSearch.pageLimit)
            let items = try await service.searchItems(query: "", limit: MasterSearch.pageLimit)
            natureOptions = natures
            itemOptions = items
            speciesSearch.setFirstPage(species)
            syncSpeciesSearch()
            mergeSpecies(species)
            didLoad = true
        } catch is CancellationError {
            return
        } catch {
            showAlert(AdjustText.errorMessage(for: error))
        }
    }

    // MARK: - 種族の検索(MasterSpeciesSearchProviding。自分・相手の検索シートで共用)

    public private(set) var speciesQuery: String = ""
    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var isSearchingSpecies = false
    public private(set) var speciesSearchReachedLimit = false

    @discardableResult
    public func setSpeciesQuery(_ text: String) -> Bool {
        let needsSearch = speciesSearch.setQuery(text)
        speciesQuery = speciesSearch.query
        return needsSearch
    }

    public func runSpeciesSearch() async {
        let results = await speciesSearch.run()
        syncSpeciesSearch()
        if let results { mergeSpecies(results) }
    }

    private func syncSpeciesSearch() {
        speciesOptions = speciesSearch.options
        isSearchingSpecies = speciesSearch.isSearching
        speciesSearchReachedLimit = speciesSearch.reachedLimit
    }

    private func mergeSpecies(_ items: [SpeciesSummary]) {
        for item in items { speciesDictionary[item.key] = item }
    }

    // MARK: - 自分

    public private(set) var ownSpeciesKey: String?
    public var ownSpecies: SpeciesSummary? { ownSpeciesKey.flatMap { speciesDictionary[$0] } }
    public private(set) var ownNatureId: String?
    public private(set) var ownAbilityOptions: [Ability] = []
    public private(set) var ownAbilityId: String?
    public private(set) var ownItemId: String?
    /// 自分の learnset のダメージ技(変化技を除く。learnset の順)。
    public private(set) var ownMoveOptions: [Move] = []
    public private(set) var ownMoveId: String?

    /// `species(key:)` で特性と learnset を読み、`moves(ids:)` で技を引く。新しい種族に無い技・特性の選択は外す。
    public func selectOwnSpecies(key: String) async {
        ownSpeciesGeneration += 1
        let token = ownSpeciesGeneration
        guard let loaded = await loadSpecies(key: key) else { return }
        guard token == ownSpeciesGeneration else { return }
        ownSpeciesKey = key
        ownAbilityOptions = loaded.detail.abilities
        ownMoveOptions = loaded.moves
        if let id = ownAbilityId, !ownAbilityOptions.contains(where: { $0.id == id }) { ownAbilityId = nil }
        if let id = ownMoveId, !ownMoveOptions.contains(where: { $0.id == id }) { ownMoveId = nil }
    }

    public func selectOwnNature(id: String?) {
        guard id == nil || natureOptions.contains(where: { $0.id == id }) else { return }
        ownNatureId = id
    }

    public func selectOwnAbility(id: String?) {
        guard id == nil || ownAbilityOptions.contains(where: { $0.id == id }) else { return }
        ownAbilityId = id
    }

    public func selectOwnItem(id: String?) {
        guard id == nil || itemOptions.contains(where: { $0.id == id }) else { return }
        ownItemId = id
    }

    /// `ownMoveOptions` にある技だけを受け付け、攻撃の分類(`offenseCategory`)を技の分類に合わせる。
    public func selectOwnMove(id: String?) {
        guard let id else {
            ownMoveId = nil
            return
        }
        guard let move = ownMoveOptions.first(where: { $0.id == id }) else { return }
        ownMoveId = id
        offenseCategory = move.category
    }

    // MARK: - 固定する能力ポイント(下限。文字で持ち、送信時に検査する。空は 0)

    private var fixedSPTexts: [StatKey: String] = [:]

    public func fixedSPText(for stat: StatKey) -> String { fixedSPTexts[stat] ?? "" }

    public func setFixedSPText(_ text: String, for stat: StatKey) { fixedSPTexts[stat] = text }

    /// 0...32 の整数として読める欄だけを足した合計(表示用)。
    public var fixedSPTotal: Int {
        StatKey.allCases.reduce(0) { $0 + (Self.parseSP(fixedSPText(for: $1)) ?? 0) }
    }

    /// 空は 0、それ以外は 0...32 の ASCII 数字だけの整数(前後の空白・符号・小数は不可)。範囲外・読めなければ nil。
    private static func parseSP(_ text: String) -> Int? {
        guard let value = parseNonNegativeInteger(text), ceilingRange.contains(value) else { return nil }
        return value
    }

    /// 空は 0、それ以外は ASCII 数字だけの 0 以上の整数。読めなければ nil。
    private static func parseNonNegativeInteger(_ text: String) -> Int? {
        if text.isEmpty { return 0 }
        guard text.allSatisfy({ $0.isASCII && $0.isNumber }) else { return nil }
        return Int(text)
    }

    // MARK: - 調整の内容

    public private(set) var mode: AdjustMode = .indices
    public func selectMode(_ mode: AdjustMode) { self.mode = mode }
    public private(set) var bulkFocus: BulkFocus = .both
    public func selectBulkFocus(_ focus: BulkFocus) { bulkFocus = focus }
    public private(set) var offenseCategory: MoveCategory = .physical
    /// physical / special だけを受け付ける。
    public func selectOffenseCategory(_ category: MoveCategory) {
        guard category != .status else { return }
        offenseCategory = category
    }

    private var ceilings: [StatKey: Int] = [:]
    /// 回す能力の上限(既定 32)。
    public func ceiling(for stat: StatKey) -> Int { ceilings[stat] ?? Self.defaultCeiling }
    /// 0...32 だけを受け付ける。
    public func setCeiling(_ value: Int, for stat: StatKey) {
        guard Self.ceilingRange.contains(value) else { return }
        ceilings[stat] = value
    }

    public private(set) var minSpeedText: String = ""
    public func setMinSpeedText(_ text: String) { minSpeedText = text }
    public private(set) var useGoal = false
    public func setUseGoal(_ isOn: Bool) { useGoal = isOn }

    // MARK: - 相手・目標

    /// 相手が要るモードか(minKo・minSurvive、bulk / offense で目標を指定したとき)。
    public var needsOpponent: Bool {
        switch mode {
        case .indices: return false
        case .minKo, .minSurvive: return true
        case .bulk, .offense: return useGoal
        }
    }

    /// 相手が攻撃する側か(minSurvive と bulk の目標)。true なら相手の技と攻撃側プリセットを出す。
    public var opponentAttacks: Bool {
        switch mode {
        case .minSurvive: return true
        case .bulk: return useGoal
        case .indices, .offense, .minKo: return false
        }
    }

    public private(set) var opponentSpeciesKey: String?
    public var opponentSpecies: SpeciesSummary? { opponentSpeciesKey.flatMap { speciesDictionary[$0] } }
    public private(set) var opponentMoveOptions: [Move] = []
    public private(set) var opponentMoveId: String?
    public private(set) var opponentAttackerPreset: AttackerPreset = .defaultPreset
    public private(set) var opponentDefenderPreset: KnownDefenderPreset = .none

    public func selectOpponentSpecies(key: String) async {
        opponentSpeciesGeneration += 1
        let token = opponentSpeciesGeneration
        guard let loaded = await loadSpecies(key: key) else { return }
        guard token == opponentSpeciesGeneration else { return }
        opponentSpeciesKey = key
        opponentMoveOptions = loaded.moves
        if let id = opponentMoveId, !opponentMoveOptions.contains(where: { $0.id == id }) { opponentMoveId = nil }
    }

    public func selectOpponentMove(id: String?) {
        guard id == nil || opponentMoveOptions.contains(where: { $0.id == id }) else { return }
        opponentMoveId = id
    }

    public func selectOpponentAttackerPreset(_ preset: AttackerPreset) { opponentAttackerPreset = preset }
    public func selectOpponentDefenderPreset(_ preset: KnownDefenderPreset) { opponentDefenderPreset = preset }

    public private(set) var hits = 1
    /// `hitsOptions` の値だけを受け付ける。
    public func selectHits(_ hits: Int) {
        guard Self.hitsOptions.contains(hits) else { return }
        self.hits = hits
    }

    public private(set) var thresholdPercent: Double = AdjustViewModel.thresholdOptions[0]
    /// `thresholdOptions` の値だけを受け付ける。
    public func selectThreshold(_ percent: Double) {
        guard Self.thresholdOptions.contains(percent) else { return }
        thresholdPercent = percent
    }

    /// 種族の詳細と、その learnset のダメージ技(変化技を除く。learnset の順)。失敗は `alertMessage` に出して nil。
    private func loadSpecies(key: String) async -> (detail: SpeciesDetail, moves: [Move])? {
        do {
            let detail = try await service.species(key: key)
            let moves = try await service.moves(ids: detail.learnset)
            let order = Dictionary(detail.learnset.enumerated().map { ($1, $0) }, uniquingKeysWith: { first, _ in first })
            mergeSpecies([SpeciesSummary(detail: detail)])
            let damageMoves = moves
                .filter { $0.category != .status }
                .sorted { (order[$0.id] ?? 0) < (order[$1.id] ?? 0) }
            return (detail, damageMoves)
        } catch is CancellationError {
            return nil
        } catch {
            showAlert(AdjustText.errorMessage(for: error))
            return nil
        }
    }

    // MARK: - 送信

    public private(set) var isLoading = false
    /// 送信前の検査の違反・API の失敗の文(日本語)。無ければ nil。
    public private(set) var alertMessage: String?
    /// 文を出すたびに増える(同じ文が続いても View が VoiceOver に読み直させるため)。
    public private(set) var alertSerial = 0
    public private(set) var outcome: AdjustOutcome?

    private func showAlert(_ message: String) {
        alertMessage = message
        alertSerial += 1
    }

    /// 検査 → indices とモードの操作を並行に呼ぶ → 両方そろったら `outcome`。
    public func submit() async {
        submitGeneration += 1
        let token = submitGeneration
        alertMessage = nil
        outcome = nil
        let plan: SubmitPlan
        do {
            plan = try makePlan()
        } catch let rejection as Rejection {
            outcome = nil
            isLoading = false
            showAlert(rejection.message)
            return
        } catch {
            outcome = nil
            isLoading = false
            showAlert(AdjustText.errorMessage(for: error))
            return
        }
        isLoading = true
        do {
            let adjust = adjust
            async let indices = adjust.adjustIndices(plan.indices)
            async let modeResult = Self.perform(plan.modeCall, on: adjust)
            let (indicesResult, mode) = try await (indices, modeResult)
            guard token == submitGeneration, !Task.isCancelled else { return }
            outcome = AdjustOutcome(indices: indicesResult, modeResult: mode, unsupportedNotice: unsupportedNotice(for: mode))
            isLoading = false
        } catch is CancellationError {
            guard token == submitGeneration else { return }
            isLoading = false
        } catch {
            guard token == submitGeneration else { return }
            outcome = nil
            showAlert(AdjustText.errorMessage(for: error))
            isLoading = false
        }
    }

    /// 前の送信を取り消してから `submit()` を呼ぶ(View の「調整する」はこれを呼ぶ)。
    @discardableResult
    public func scheduleSubmit() -> Task<Void, Never> {
        submitRunner.schedule(debounce: .zero) { [weak self] in
            await self?.submit()
        }
    }

    /// 送信中・一覧の読み込み中の Task を取り消す(View の `.onDisappear`)。
    public func cancelPendingWork() {
        submitRunner.cancel()
        learnersRunner.cancel()
        submitGeneration += 1
        learnersGeneration += 1
        isLoading = false
        learners?.isLoading = false
    }

    // MARK: - 送信前の検査と要求の組み立て

    /// 検査の違反(画面に出す文)。
    private struct Rejection: Error {
        let message: String
    }

    /// 1回の送信で呼ぶ内容。
    private struct SubmitPlan {
        let indices: AdjustIndicesRequest
        let modeCall: ModeCall
    }

    private enum ModeCall: Sendable {
        case none
        case ko(AdjustSearchRequest, hits: Int)
        case survive(AdjustSearchRequest, hits: Int)
        case allocation(AdjustAllocationRequest, mode: AllocMode, goalRequested: Bool, speedTargetRequested: Bool)
    }

    private nonisolated static func perform(_ call: ModeCall, on adjust: any AdjustService) async throws -> AdjustModeResult? {
        switch call {
        case .none:
            return nil
        case .ko(let request, let hits):
            return .ko(try await adjust.adjustMinSpToKo(request), hits: hits)
        case .survive(let request, let hits):
            return .survive(try await adjust.adjustMinSpToSurvive(request), hits: hits)
        case .allocation(let request, let mode, let goalRequested, let speedTargetRequested):
            return .allocation(
                try await adjust.adjustAllocation(request), mode: mode, goalRequested: goalRequested,
                speedTargetRequested: speedTargetRequested)
        }
    }

    /// ADR-0502 §5 の順(Web の AdjustScreen.tsx と同じ)に検査し、先に当たった1つで止める。通れば送る内容を返す。
    private func makePlan() throws -> SubmitPlan {
        // 1. 自分のポケモンと性格
        guard let speciesKey = ownSpeciesKey, let natureId = ownNatureId else {
            throw Rejection(message: AdjustText.ownRequired)
        }
        // 2・3. 固定 SP
        var sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        for stat in StatKey.allCases {
            guard let value = Self.parseSP(fixedSPText(for: stat)) else { throw Rejection(message: AdjustText.spRangeInvalid) }
            Self.set(&sp, stat, value)
        }
        guard fixedSPTotal <= SPLimits.maxTotal else { throw Rejection(message: AdjustText.spTotalExceeded) }
        let own = Individual(
            speciesKey: speciesKey, natureId: natureId, sp: sp, abilityId: ownAbilityId, itemId: ownItemId)
        let ownMove = ownMoveId.flatMap { id in ownMoveOptions.first { $0.id == id } }
        // 4〜8. モードごとの必須・上限・分類・素早さ・プリセット
        let modeCall = try makeModeCall(own: own, ownMove: ownMove, fixed: sp)
        let indices = AdjustIndicesRequest(
            individual: own, moveId: ownMove?.id, modifier: isSameTypeAttackBonus(ownMove) ? Self.stabModifier : nil)
        return SubmitPlan(indices: indices, modeCall: modeCall)
    }

    private func makeModeCall(own: Individual, ownMove: Move?, fixed: StatBlock) throws -> ModeCall {
        let opponentMove = opponentMoveId.flatMap { id in opponentMoveOptions.first { $0.id == id } }
        let threshold: Double? = thresholdPercent == Self.thresholdOptions[0] ? nil : thresholdPercent
        switch mode {
        case .indices:
            return .none
        case .minKo:
            let (move, opponentKey) = try requireOwnMoveAndOpponent(ownMove)
            let defender = try defenderIndividual(opponentKey, moveCategory: move.category)
            return .ko(
                AdjustSearchRequest(format: .single, attacker: own, defender: defender, moveId: move.id, hits: hits, thresholdPercent: threshold),
                hits: hits)
        case .minSurvive:
            let (opponentKey, move) = try requireOpponentAndMove(opponentMove)
            let attacker = try attackerIndividual(opponentKey, moveCategory: move.category)
            return .survive(
                AdjustSearchRequest(format: .single, attacker: attacker, defender: own, moveId: move.id, hits: hits, thresholdPercent: threshold),
                hits: hits)
        case .bulk:
            try checkCeiling(rotated: [.hp, .def, .spd], fixed: fixed)
            var goal: AdjustAllocGoal?
            if useGoal {
                let (opponentKey, move) = try requireOpponentAndMove(opponentMove)
                let attacker = try attackerIndividual(opponentKey, moveCategory: move.category)
                goal = AdjustAllocGoal(format: .single, opponent: attacker, moveId: move.id, hits: hits, thresholdPercent: threshold)
            }
            let ceiling = AdjustCeiling(hp: ceiling(for: .hp), def: ceiling(for: .def), spd: ceiling(for: .spd))
            return .allocation(
                AdjustAllocationRequest(selfIndividual: own, ceiling: ceiling, mode: .bulk, focus: bulkFocus, goal: goal),
                mode: .bulk, goalRequested: useGoal, speedTargetRequested: false)
        case .offense:
            let attackStat = AttackerPreset.relevantStat(for: offenseCategory)
            try checkCeiling(rotated: [attackStat, .spe], fixed: fixed)
            guard let minSpeed = Self.parseNonNegativeInteger(minSpeedText) else {
                throw Rejection(message: AdjustText.minSpeedInvalid)
            }
            var goal: AdjustAllocGoal?
            if useGoal {
                guard let ownMove else { throw Rejection(message: AdjustText.ownMoveRequired) }
                try checkCategoryMatches(ownMove)
                guard let opponentSpeciesKey else { throw Rejection(message: AdjustText.opponentRequired) }
                goal = AdjustAllocGoal(
                    format: .single, opponent: try defenderIndividual(opponentSpeciesKey, moveCategory: ownMove.category),
                    moveId: ownMove.id, hits: hits, thresholdPercent: threshold)
            }
            let ceiling = AdjustCeiling(
                atk: attackStat == .atk ? ceiling(for: .atk) : nil, spa: attackStat == .spa ? ceiling(for: .spa) : nil,
                spe: ceiling(for: .spe))
            return .allocation(
                AdjustAllocationRequest(
                    selfIndividual: own, ceiling: ceiling, mode: .offense, offenseCategory: offenseCategory,
                    minSpeed: minSpeed, goal: goal),
                mode: .offense, goalRequested: useGoal, speedTargetRequested: minSpeed > 0)
        }
    }

    private func requireOwnMoveAndOpponent(_ ownMove: Move?) throws -> (Move, String) {
        guard let ownMove else { throw Rejection(message: AdjustText.ownMoveRequired) }
        guard let opponentSpeciesKey else { throw Rejection(message: AdjustText.opponentRequired) }
        return (ownMove, opponentSpeciesKey)
    }

    private func requireOpponentAndMove(_ opponentMove: Move?) throws -> (String, Move) {
        guard let opponentSpeciesKey else { throw Rejection(message: AdjustText.opponentRequired) }
        guard let opponentMove else { throw Rejection(message: AdjustText.opponentMoveRequired) }
        return (opponentSpeciesKey, opponentMove)
    }

    private func checkCeiling(rotated: [StatKey], fixed: StatBlock) throws {
        for stat in rotated where ceiling(for: stat) < Self.value(of: fixed, stat) {
            throw Rejection(message: AdjustText.ceilingBelowFixed)
        }
    }

    private func checkCategoryMatches(_ ownMove: Move) throws {
        guard ownMove.category == offenseCategory else { throw Rejection(message: AdjustText.categoryMismatch) }
    }

    /// 相手が攻撃する側(攻撃側プリセット)。性格がマスタに無ければ `natureNotFound`。
    private func attackerIndividual(_ speciesKey: String, moveCategory: MoveCategory) throws -> Individual {
        do {
            let build = try AttackerPreset.build(opponentAttackerPreset, moveCategory: moveCategory, natures: natureOptions)
            return Individual(speciesKey: speciesKey, natureId: build.natureId, sp: build.sp)
        } catch {
            throw Rejection(message: AdjustText.natureNotFound)
        }
    }

    /// 相手が受ける側(防御側プリセット)。性格がマスタに無ければ `natureNotFound`。
    private func defenderIndividual(_ speciesKey: String, moveCategory: MoveCategory) throws -> Individual {
        do {
            let build = try KnownDefenderPreset.build(opponentDefenderPreset, moveCategory: moveCategory, natures: natureOptions)
            return Individual(speciesKey: speciesKey, natureId: build.natureId, sp: build.sp)
        } catch {
            throw Rejection(message: AdjustText.natureNotFound)
        }
    }

    /// 技のタイプが自分の種族のタイプに含まれるか(タイプ一致。補正 ×1.5)。
    private func isSameTypeAttackBonus(_ move: Move?) -> Bool {
        guard let move, let types = ownSpecies?.types else { return false }
        return types.contains(move.type)
    }

    private static func set(_ block: inout StatBlock, _ stat: StatKey, _ value: Int) {
        switch stat {
        case .hp: block.hp = value
        case .atk: block.atk = value
        case .def: block.def = value
        case .spa: block.spa = value
        case .spd: block.spd = value
        case .spe: block.spe = value
        }
    }

    private static func value(of block: StatBlock, _ stat: StatKey) -> Int {
        switch stat {
        case .hp: return block.hp
        case .atk: return block.atk
        case .def: return block.def
        case .spa: return block.spa
        case .spd: return block.spd
        case .spe: return block.spe
        }
    }

    /// モードの結果の未対応の印を1回だけの注記にする(名前は見た技・持ち物・特性から引く。数値は変えない)。
    private func unsupportedNotice(for result: AdjustModeResult?) -> String? {
        let marks: [UnsupportedMark]
        switch result {
        case .ko(let ko, _): marks = ko.unsupported
        case .survive(let survive, _): marks = survive.unsupported
        case .allocation(let allocation, _, _, _): marks = allocation.unsupported
        case nil: marks = []
        }
        let names = UnsupportedMarkNames(moves: ownMoveOptions + opponentMoveOptions, items: itemOptions, abilities: ownAbilityOptions)
        return UnsupportedNoticeText.summary(marks, names: names)
    }

    // MARK: - 技を覚えるポケモン

    public private(set) var learners: AdjustLearnersState?

    /// その側の技が選ばれていなければ何もしない。前の読み込みを取り消し、一覧を置き換えて先頭ページを読む。
    public func openLearners(_ side: AdjustMoveSide) async {
        let moveId: String?
        let move: Move?
        switch side {
        case .own:
            moveId = ownMoveId
            move = ownMoveOptions.first { $0.id == moveId }
        case .opponent:
            moveId = opponentMoveId
            move = opponentMoveOptions.first { $0.id == moveId }
        }
        guard let moveId, let move else { return }
        learnersGeneration += 1
        let token = learnersGeneration
        learners = AdjustLearnersState(side: side, moveId: moveId, moveName: move.nameJa, isLoading: true)
        await loadLearnersPage(token: token, moveId: moveId, offset: 0)
    }

    /// 次のページ(offset = 読み込んだ件数)を読み、連結する。失敗しても読み込んだ一覧は残す。
    public func loadMoreLearners() async {
        guard let state = learners, state.canLoadMore, !state.isLoading else { return }
        learnersGeneration += 1
        let token = learnersGeneration
        learners?.isLoading = true
        learners?.errorMessage = nil
        await loadLearnersPage(token: token, moveId: state.moveId, offset: state.species.count)
    }

    @discardableResult
    public func scheduleOpenLearners(_ side: AdjustMoveSide) -> Task<Void, Never> {
        learnersRunner.schedule(debounce: .zero) { [weak self] in
            await self?.openLearners(side)
        }
    }

    @discardableResult
    public func scheduleLoadMoreLearners() -> Task<Void, Never> {
        learnersRunner.schedule(debounce: .zero) { [weak self] in
            await self?.loadMoreLearners()
        }
    }

    public func closeLearners() {
        learnersRunner.cancel()
        learnersGeneration += 1
        learners = nil
    }

    private func loadLearnersPage(token: Int, moveId: String, offset: Int) async {
        let pageSize = RequestLimits.moveLearnersPageSize
        do {
            let page = try await adjust.moveLearners(moveId: moveId, limit: pageSize, offset: offset)
            guard token == learnersGeneration else { return }
            learners?.species.append(contentsOf: page)
            learners?.canLoadMore = page.count == pageSize
            learners?.errorMessage = nil
            learners?.isLoading = false
        } catch is CancellationError {
            guard token == learnersGeneration else { return }
            learners?.isLoading = false
        } catch {
            guard token == learnersGeneration else { return }
            learners?.errorMessage = AdjustText.errorMessage(for: error)
            learners?.isLoading = false
        }
    }
}

/// 種族の検索欄(`MasterSearchField` の別名。`MasterSearchField<SpeciesSummary>` を1か所の名前で呼ぶ)。
private typealias MasterSpeciesSearchField = MasterSearchField<SpeciesSummary>
