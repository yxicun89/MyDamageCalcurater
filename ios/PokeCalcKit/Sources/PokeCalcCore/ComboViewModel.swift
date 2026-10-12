import Observation

// ComboViewModel: 計算画面の「連続技」の節の状態(CB-8。ADR-0535・ADR-0501「連続技」AC-i1〜AC-i10)。
//
// ロジックはここに置き、View は描くだけ(ADR-0500 §1)。連続技の入力は計算画面の値(`ComboContext`)を使い、
// 別の入力欄を持たない(UC-5)。計算は `ComboCalcService`(API)だけを使い、失敗はこの節の中に閉じる(絶対ルール5)。

/// 連続技の1手(表示する手)。メインは計算画面の技で外せない。
public enum ComboEntry: Equatable, Identifiable, Sendable {
    case main
    case extra(key: String, moveId: String, critical: Bool)

    public var id: String {
        switch self {
        case .main: return "main"
        case .extra(let key, _, _): return key
        }
    }
}

/// 連続技の節の計算状態。
public enum ComboViewState: Equatable, Sendable {
    case idle
    case loading
    case loaded(ComboResultDisplay)
    case failed(message: String)
}

@MainActor
@Observable
public final class ComboViewModel {
    private let comboService: any ComboCalcService
    private let master: any PokeCalcService
    private let speedService: (any SpeedService)?

    /// 手の並び(メインと追加の手)。覚えなくなった追加の手も消さず残し、表示と要求からだけ外す(Web と同じ)。
    private var slots: [Slot] = [.main]
    private var extras: [String: ExtraStep] = [:]
    private var nextExtraNumber = 0
    /// 一度でも見た技(結果の技名・チップに使う)。
    private var knownMoves: [String: Move] = [:]
    /// 「最新の要求だけを反映する」通し番号。
    private var generation = 0
    private var candidatesGeneration = 0

    public private(set) var context: ComboContext?
    public private(set) var isExpanded = false
    public private(set) var extraCandidates: [Move] = []
    public private(set) var opponentCandidates: [Move] = []
    /// 選んだ相手の振りの行キー。nil は先頭の振り。
    private var selectedDefenderKey: String?
    public private(set) var defenderWasReset = false
    public private(set) var opponentMoveId: String?
    public private(set) var opponentSpeedFromDefender = false
    public private(set) var speedField = ComboSpeedField()
    public private(set) var selfSpeed = ComboSelfSpeedInput.default
    public private(set) var state: ComboViewState = .idle
    public private(set) var speedExampleLines: [String] = []

    public init(comboService: any ComboCalcService, master: any PokeCalcService, speedService: (any SpeedService)? = nil) {
        self.comboService = comboService
        self.master = master
        self.speedService = speedService
    }

    // MARK: - 見え方

    /// 計算画面に結果の行があるときだけ節を出す。
    public var isVisible: Bool { context != nil }

    /// 表示する手(攻撃側が覚えなくなった追加の手は除く。上限を超えるぶんも出さない)。
    public var entries: [ComboEntry] {
        var result: [ComboEntry] = []
        for index in visibleSlotIndices { result.append(entry(for: slots[index])) }
        return result
    }

    public var canAddStep: Bool { entries.count < RequestLimits.maxComboSteps }

    /// 手がメインだけで相手の技も無いときだけ閉じられる。
    public var canClose: Bool { entries == [.main] && opponentMoveId == nil }

    public var selectedDefender: ComboDefenderOption? {
        guard let context else { return nil }
        if let key = selectedDefenderKey, let found = context.defenders.first(where: { $0.key == key }) { return found }
        return context.defenders.first
    }

    /// 手の技(メインは計算画面の技。引けなければ nil)。
    public func move(for entry: ComboEntry) -> Move? {
        switch entry {
        case .main: return context?.mainMove
        case .extra(_, let moveId, _): return knownMoves[moveId]
        }
    }

    public var opponentMove: Move? { opponentMoveId.flatMap { knownMoves[$0] } }

    /// 素早さの SP の入力の上限(素早さ以外の SP から)。
    public var selfSpeedMaxSP: Int { context.map { ComboSelfSpeed.maxSP(for: $0.attacker) } ?? SPLimits.maxPerStat }

    // MARK: - 操作

    /// 計算画面の値が変わったときに呼ぶ(View は `.task(id: comboContext)`)。同じ値なら何もしない。
    public func updateContext(_ newContext: ComboContext?) async {
        let old = context
        guard newContext != old else { return }
        context = newContext
        guard let new = newContext else {
            generation += 1
            state = .idle
            speedExampleLines = []
            return
        }
        if let key = selectedDefenderKey, !new.defenders.contains(where: { $0.key == key }) {
            selectedDefenderKey = new.defenders.first?.key
            defenderWasReset = true
        }
        knownMoves[new.mainMove.id] = new.mainMove
        if old?.defenderSpeciesKey != new.defenderSpeciesKey { opponentMoveId = nil }
        if isExpanded, old?.attackerLearnset != new.attackerLearnset || old?.defenderSpeciesKey != new.defenderSpeciesKey {
            await loadCandidates()
        }
        await recalculate()
    }

    /// 広げる。候補を読むだけで、計算はしない。
    public func open() async {
        guard isVisible else { return }
        isExpanded = true
        await loadCandidates()
    }

    /// 閉じる(手がメインだけで相手の技も無いときだけ)。
    public func close() {
        guard canClose else { return }
        isExpanded = false
    }

    public func addStep(moveId: String) async {
        guard canAddStep, extraCandidates.contains(where: { $0.id == moveId }) else { return }
        nextExtraNumber += 1
        let key = "extra-\(nextExtraNumber)"
        extras[key] = ExtraStep(moveId: moveId, critical: false)
        slots.append(.extra(key))
        await recalculate()
    }

    /// `index`(表示する手の位置)の手を `delta` だけ動かす。端より外へは動かない。メインも動かせる。
    public func moveStep(at index: Int, by delta: Int) async {
        let indices = visibleSlotIndices
        let target = index + delta
        guard indices.indices.contains(index), indices.indices.contains(target), delta != 0 else { return }
        slots.swapAt(indices[index], indices[target])
        await recalculate()
    }

    /// メインは外せない。
    public func removeStep(at index: Int) async {
        let indices = visibleSlotIndices
        guard indices.indices.contains(index), case .extra(let key) = slots[indices[index]] else { return }
        slots.remove(at: indices[index])
        extras[key] = nil
        await recalculate()
    }

    public func setExtraCritical(key: String, isOn: Bool) async {
        guard var step = extras[key], step.critical != isOn else { return }
        step.critical = isOn
        extras[key] = step
        await recalculate()
    }

    /// 無い振りは選ばない。選び直したら「戻した」案内を消す。
    public func selectDefender(key: String) async {
        guard let context, context.defenders.contains(where: { $0.key == key }) else { return }
        defenderWasReset = false
        guard selectedDefender?.key != key else { return }
        selectedDefenderKey = key
        await recalculate()
    }

    /// 防御側の覚える攻撃技だけ。nil は「選ばない」。
    public func selectOpponentMove(id: String?) async {
        if let id, !opponentCandidates.contains(where: { $0.id == id }) { return }
        guard id != opponentMoveId else { return }
        opponentMoveId = id
        await recalculate()
    }

    public func setOpponentSpeedFromDefender(_ isOn: Bool) async {
        guard isOn != opponentSpeedFromDefender else { return }
        opponentSpeedFromDefender = isOn
        await recalculate()
    }

    public func setSpeedField(_ field: ComboSpeedField) async {
        guard field != speedField else { return }
        speedField = field
        await recalculate()
    }

    /// nil は個体の値のまま。入力時に 0〜上限へ丸める。
    public func setSelfSpeedSP(_ sp: Int?) async {
        let clamped = sp.map { value in context.map { ComboSelfSpeed.clampSP(value, for: $0.attacker) } ?? value }
        guard clamped != selfSpeed.sp else { return }
        selfSpeed.sp = clamped
        await recalculate()
    }

    public func setSelfSpeedNature(_ nature: ComboSelfSpeedNature) async {
        guard nature != selfSpeed.nature else { return }
        selfSpeed.nature = nature
        await recalculate()
    }

    /// nil は個体の値のまま。-6〜+6 に丸める。
    public func setSelfSpeedRank(_ rank: Int?) async {
        let clamped = rank.map { min(RankLimits.max, max(RankLimits.min, $0)) }
        guard clamped != selfSpeed.rank else { return }
        selfSpeed.rank = clamped
        await recalculate()
    }

    /// 失敗のあとに同じ入力でもう一度計算する。
    public func retry() async {
        await recalculate()
    }

    // MARK: - 手

    private enum Slot: Equatable {
        case main
        case extra(String)
    }

    private struct ExtraStep {
        var moveId: String
        var critical: Bool
    }

    private func entry(for slot: Slot) -> ComboEntry {
        switch slot {
        case .main: return .main
        case .extra(let key):
            let step = extras[key]
            return .extra(key: key, moveId: step?.moveId ?? "", critical: step?.critical ?? false)
        }
    }

    /// 表示する手の `slots` 上の位置。覚えなくなった追加の手を除き、メインを残したまま上限を超えるぶんを後ろから落とす。
    private var visibleSlotIndices: [Int] {
        guard let context else { return slots.isEmpty ? [] : slots.indices.filter { slots[$0] == .main } }
        let learned = Set(context.attackerLearnset)
        var indices = slots.indices.filter { index in
            switch slots[index] {
            case .main: return true
            case .extra(let key): return extras[key].map { learned.contains($0.moveId) } ?? false
            }
        }
        while indices.count > RequestLimits.maxComboSteps, let drop = indices.lastIndex(where: { slots[$0] != .main }) {
            indices.remove(at: drop)
        }
        return indices
    }

    // MARK: - 候補

    private func loadCandidates() async {
        guard let context else { return }
        candidatesGeneration += 1
        let token = candidatesGeneration
        let learnset = context.attackerLearnset
        let defenderKey = context.defenderSpeciesKey
        let attackMoves = (try? await master.moves(ids: learnset)) ?? []
        let opponentLearnset = (try? await master.species(key: defenderKey).learnset) ?? []
        let opponentMoves = (try? await master.moves(ids: opponentLearnset)) ?? []
        guard token == candidatesGeneration else { return }
        for move in attackMoves + opponentMoves { knownMoves[move.id] = move }
        extraCandidates = MoveSort.byType(attackMoves.filter(ComboMoveRules.canBeStep))
        opponentCandidates = MoveSort.byType(opponentMoves.filter(ComboMoveRules.canBeOpponentMove))
    }

    // MARK: - 計算

    /// 手が 2 つ以上、または相手の技を選んでいるときだけ計算する。
    private var shouldCalculate: Bool { entries.count >= 2 || opponentMoveId != nil }

    private func buildRequest() throws -> ComboRequest? {
        guard let context, let defender = selectedDefender else { return nil }
        let steps = entries.map { entry -> ComboStepInput in
            switch entry {
            case .main: return ComboStepInput(moveId: context.mainMove.id, critical: context.mainCritical)
            case .extra(_, let moveId, let critical): return ComboStepInput(moveId: moveId, critical: critical)
            }
        }
        let attacker = try ComboSelfSpeed.apply(selfSpeed, to: context.attacker, natures: context.natures)
        return ComboRequest(
            format: .single, attacker: attacker, defender: defender.individual, field: context.field, steps: steps,
            opponentMoveId: opponentMoveId, opponentSpeed: opponentSpeedFromDefender ? .fromDefender : .unknown,
            speedField: speedField)
    }

    private func recalculate() async {
        generation += 1
        let token = generation
        guard shouldCalculate else {
            state = .idle
            speedExampleLines = []
            return
        }
        let request: ComboRequest
        do {
            guard let built = try buildRequest() else {
                state = .idle
                speedExampleLines = []
                return
            }
            request = built
        } catch {
            state = .failed(message: (error as? PokeCalcError)?.message ?? ComboText.failureMessage(for: error))
            speedExampleLines = []
            return
        }
        state = .loading
        do {
            let result = try await comboService.calcCombo(request)
            guard token == generation else { return }
            let display = makeDisplay(result, request: request)
            state = .loaded(display)
            await loadSpeedExample(for: display, token: token)
        } catch is CancellationError {
            guard token == generation else { return }
            state = .idle
        } catch {
            guard token == generation else { return }
            state = .failed(message: ComboText.failureMessage(for: error))
            speedExampleLines = []
        }
    }

    private func makeDisplay(_ result: ComboResult, request: ComboRequest) -> ComboResultDisplay {
        let moves = request.steps.map { knownMoves[$0.moveId] }
        var names = context?.markNames ?? UnsupportedMarkNames()
        for move in knownMoves.values where names.moveNames[move.id] == nil { names.moveNames[move.id] = move.nameJa }
        return ComboResultDisplay(result: result, stepMoves: moves, names: names, trickRoom: request.speedField.trickRoom)
    }

    /// 相手の素早さが分からないときだけ、最後の手の自分の素早さで「族」の例を引く。失敗しても連続技の結果は壊さない。
    private func loadSpeedExample(for display: ComboResultDisplay, token: Int) async {
        guard display.showsSpeedExample, let speedService else {
            speedExampleLines = []
            return
        }
        let lines: [String]
        do {
            let reach = try await speedService.reach(SpeedPositionRequest(input: .raw(value: display.lastSelfSpeed, pokemonId: nil)))
            lines = ComboText.speedExampleLines(reach)
        } catch {
            lines = []
        }
        guard token == generation else { return }
        speedExampleLines = lines
    }
}
