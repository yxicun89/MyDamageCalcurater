// AdjustViewModel+Goals: 調整の「目標」方式(F-11。ADR-0331 §5〜§7・ADR-0525)。
//
// 「相手を選んで目標の種類(素早さを上回る・この技を耐える・この技で倒す)を選ぶ」。目標は 1〜`RequestLimits.maxAdjustGoals` 件。
// 「調整する」で indices(今の振り方)と adjustGoals を並行して呼び、両方そろってから出す(従来のモードと同じ)。
// サーバーが目標の操作を提供していない(404/501 等)ときは `goalsUnavailable` を立て、従来の調整に戻して案内を出す
// (絶対ルール5: 画面は壊れない)。計算はしない(相手の素早さも engine が求める)。

/// 目標1件の入力(画面の1枚のカード)。振り方・技は種類ごとの既定に戻る(`setGoalKind`)。
public struct AdjustGoalDraft: Equatable, Identifiable, Sendable {
    public let id: Int
    public var kind: AdjustGoalKind = .outspeed
    public var opponentSpeciesKey: String?
    /// 相手の learnset のダメージ技(survive の相手の技の選択肢)。
    public var opponentMoves: [Move] = []
    public var opponentMoveId: String?
    /// ko の自分の技。
    public var ownMoveId: String?
    /// outspeed の「先に使う技」(任意)。
    public var boostMoveId: String?
    public var speedPreset: AdjustSpeedPreset = .defaultPreset
    public var attackerPreset: AttackerPreset = .aFull
    public var defenderPreset: KnownDefenderPreset = .none
    public var hits = 1
    public var thresholdPercent: Double = 100

    public init(id: Int) { self.id = id }
}

/// 送信した時点の名前の写し(結果の文に使う。送信後に入力を変えても結果の文は変わらない)。
public struct AdjustGoalSnapshot: Equatable, Sendable {
    public var kind: AdjustGoalKind
    /// 「相手(振り方)」。
    public var opponentName: String
    /// survive = 相手の技、ko = 自分の技。outspeed は nil。
    public var moveName: String?
    /// outspeed の「先に使う技」。
    public var boostMoveName: String?
    public var hits: Int

    public init(kind: AdjustGoalKind, opponentName: String, moveName: String? = nil, boostMoveName: String? = nil, hits: Int = 1) {
        self.kind = kind
        self.opponentName = opponentName
        self.moveName = moveName
        self.boostMoveName = boostMoveName
        self.hits = hits
    }
}

/// 目標方式の結果(応答 + 送信時の名前)。
public struct AdjustGoalsPresentation: Equatable, Sendable {
    public var result: AdjustGoalsResult
    public var snapshots: [AdjustGoalSnapshot]

    public init(result: AdjustGoalsResult, snapshots: [AdjustGoalSnapshot]) {
        self.result = result
        self.snapshots = snapshots
    }
}

extension AdjustViewModel {
    // MARK: - モード

    /// 目標方式を選べるか(サービスがあり、サーバーが提供していると分かっていない)。
    public var goalsAvailable: Bool { goalsService != nil && !goalsUnavailable }

    public var canAddGoal: Bool { goalDrafts.count < RequestLimits.maxAdjustGoals }

    /// 「目標から振り方を決める」を選ぶ。使えないときは何もしない。
    public func selectGoalsMode() {
        guard goalsAvailable else { return }
        isGoalsMode = true
    }

    // MARK: - 目標の追加・削除

    /// 目標を末尾に足す(上限なら何もしない)。足した目標の id を返す。
    @discardableResult
    public func addGoal() -> Int? {
        guard canAddGoal else { return nil }
        nextGoalSerial += 1
        goalDrafts.append(AdjustGoalDraft(id: nextGoalSerial))
        return nextGoalSerial
    }

    public func removeGoal(id: Int) {
        goalDrafts.removeAll { $0.id == id }
        goalOpponentGenerations[id] = nil
    }

    // MARK: - 目標の編集

    /// 種類を変える。相手のポケモンは保ち、振り方・技・発数・確率は新しい種類の既定に戻す。
    public func setGoalKind(id: Int, _ kind: AdjustGoalKind) {
        updateGoal(id) { draft in
            guard draft.kind != kind else { return }
            var fresh = AdjustGoalDraft(id: draft.id)
            fresh.kind = kind
            fresh.opponentSpeciesKey = draft.opponentSpeciesKey
            fresh.opponentMoves = draft.opponentMoves
            draft = fresh
        }
    }

    /// 相手のポケモンを選ぶ(learnset と特性は読むが、技は選び直し。新しい相手に無い技の選択は外す)。
    public func selectGoalOpponent(id: Int, key: String) async {
        goalOpponentGenerations[id, default: 0] += 1
        let token = goalOpponentGenerations[id, default: 0]
        guard let loaded = await loadSpecies(key: key) else { return }
        guard token == goalOpponentGenerations[id], goalDrafts.contains(where: { $0.id == id }) else { return }
        updateGoal(id) { draft in
            draft.opponentSpeciesKey = key
            draft.opponentMoves = loaded.moves
            if let moveId = draft.opponentMoveId, !loaded.moves.contains(where: { $0.id == moveId }) { draft.opponentMoveId = nil }
        }
    }

    public func selectGoalSpeedPreset(id: Int, _ preset: AdjustSpeedPreset) { updateGoal(id) { $0.speedPreset = preset } }
    public func selectGoalAttackerPreset(id: Int, _ preset: AttackerPreset) { updateGoal(id) { $0.attackerPreset = preset } }
    public func selectGoalDefenderPreset(id: Int, _ preset: KnownDefenderPreset) { updateGoal(id) { $0.defenderPreset = preset } }

    /// 相手の learnset にある技だけを受け付ける(nil は未選択)。
    public func selectGoalOpponentMove(id: Int, moveId: String?) {
        updateGoal(id) { draft in
            guard moveId == nil || draft.opponentMoves.contains(where: { $0.id == moveId }) else { return }
            draft.opponentMoveId = moveId
        }
    }

    /// 自分の learnset にある技だけを受け付ける(nil は未選択)。ko の自分の技。
    public func selectGoalOwnMove(id: Int, moveId: String?) {
        guard moveId == nil || ownMoveOptions.contains(where: { $0.id == moveId }) else { return }
        updateGoal(id) { $0.ownMoveId = moveId }
    }

    /// 自分の learnset にある技だけを受け付ける(nil は「使わない」)。outspeed の先に使う技。
    public func selectGoalBoostMove(id: Int, moveId: String?) {
        guard moveId == nil || ownMoveOptions.contains(where: { $0.id == moveId }) else { return }
        updateGoal(id) { $0.boostMoveId = moveId }
    }

    /// `hitsOptions` の値だけを受け付ける。
    public func selectGoalHits(id: Int, _ hits: Int) {
        guard Self.hitsOptions.contains(hits) else { return }
        updateGoal(id) { $0.hits = hits }
    }

    /// `thresholdOptions` の値だけを受け付ける。
    public func selectGoalThreshold(id: Int, _ percent: Double) {
        guard Self.thresholdOptions.contains(percent) else { return }
        updateGoal(id) { $0.thresholdPercent = percent }
    }

    private func updateGoal(_ id: Int, _ change: (inout AdjustGoalDraft) -> Void) {
        guard let index = goalDrafts.firstIndex(where: { $0.id == id }) else { return }
        change(&goalDrafts[index])
    }

    // MARK: - 表示のための引き当て

    public func goalOpponentSpecies(_ draft: AdjustGoalDraft) -> SpeciesSummary? {
        draft.opponentSpeciesKey.flatMap { speciesDictionary[$0] }
    }

    /// 自分の技(選択肢の中にあるものだけ。自分のポケモンを変えて無くなっていれば nil)。
    public func goalOwnMove(_ draft: AdjustGoalDraft) -> Move? {
        draft.ownMoveId.flatMap { id in ownMoveOptions.first { $0.id == id } }
    }

    public func goalBoostMove(_ draft: AdjustGoalDraft) -> Move? {
        draft.boostMoveId.flatMap { id in ownMoveOptions.first { $0.id == id } }
    }

    public func goalOpponentMove(_ draft: AdjustGoalDraft) -> Move? {
        draft.opponentMoveId.flatMap { id in draft.opponentMoves.first { $0.id == id } }
    }

    // MARK: - 送信

    /// 検査 → indices と adjustGoals を並行に呼ぶ → 両方そろったら `outcome`(`submit()` から呼ばれる)。
    func submitGoals(token: Int) async {
        guard let goalsService else { return }
        let built: (indices: AdjustIndicesRequest, request: AdjustGoalsRequest, snapshots: [AdjustGoalSnapshot])
        do {
            built = try makeGoalsPlan()
        } catch let rejection as AdjustRejection {
            isLoading = false
            showAlert(rejection.message)
            return
        } catch {
            isLoading = false
            showAlert(AdjustText.errorMessage(for: error))
            return
        }
        isLoading = true
        do {
            let adjust = adjust
            async let indices = adjust.adjustIndices(built.indices)
            async let goals = goalsService.adjustGoals(built.request)
            let (indicesResult, goalsResult) = try await (indices, goals)
            guard token == submitGeneration, !Task.isCancelled else { return }
            let mode = AdjustModeResult.goals(AdjustGoalsPresentation(result: goalsResult, snapshots: built.snapshots))
            outcome = AdjustOutcome(indices: indicesResult, modeResult: mode, unsupportedNotice: unsupportedNotice(for: mode))
            isLoading = false
        } catch is CancellationError {
            guard token == submitGeneration else { return }
            isLoading = false
        } catch {
            guard token == submitGeneration else { return }
            outcome = nil
            isLoading = false
            if AdjustGoalsAvailability.isUnavailable(error) {
                // 機能が無い: 目標方式を引っ込めて従来の調整に戻す(入力は消さない)。
                goalsUnavailable = true
                isGoalsMode = false
                showAlert(AdjustText.goalsUnavailable)
            } else {
                showAlert(AdjustText.errorMessage(for: error))
            }
        }
    }

    /// 検査の順(Web の ADR-0331 §6 と同じ): 自分のポケモン・性格 → 固定 SP → 目標が 0 件 → 目標ごとに
    /// 相手 → 技 → 性格。最初の違反だけ出す。
    private func makeGoalsPlan() throws -> (indices: AdjustIndicesRequest, request: AdjustGoalsRequest, snapshots: [AdjustGoalSnapshot]) {
        let (own, _) = try makeOwnIndividual()
        guard !goalDrafts.isEmpty else { throw AdjustRejection(message: AdjustText.goalsRequiredMessage) }
        var inputs: [AdjustGoalInput] = []
        var snapshots: [AdjustGoalSnapshot] = []
        for (offset, draft) in goalDrafts.enumerated() {
            let (input, snapshot) = try makeGoal(draft, number: offset + 1)
            inputs.append(input)
            snapshots.append(snapshot)
        }
        let ownMove = ownMoveId.flatMap { id in ownMoveOptions.first { $0.id == id } }
        let indices = AdjustIndicesRequest(
            individual: own, moveId: ownMove?.id, modifier: isSameTypeAttackBonus(ownMove) ? Self.stabModifier : nil)
        return (indices, AdjustGoalsRequest(format: .single, selfIndividual: own, goals: inputs), snapshots)
    }

    private func makeGoal(_ draft: AdjustGoalDraft, number n: Int) throws -> (AdjustGoalInput, AdjustGoalSnapshot) {
        guard let speciesKey = draft.opponentSpeciesKey, let species = speciesDictionary[speciesKey] else {
            throw AdjustRejection(message: AdjustText.goalOpponentRequiredMessage(n))
        }
        let threshold: Double? = draft.thresholdPercent == Self.thresholdOptions[0] ? nil : draft.thresholdPercent
        switch draft.kind {
        case .outspeed:
            let preset = draft.speedPreset
            let opponent = try goalOpponent(speciesKey, number: n) { try preset.build(natures: $0) }
            let boost = goalBoostMove(draft)
            return (
                AdjustGoalInput(kind: .outspeed, opponent: opponent, moveId: boost?.id),
                AdjustGoalSnapshot(
                    kind: .outspeed, opponentName: AdjustText.goalOpponentName(species.nameJa, preset.label),
                    boostMoveName: boost?.nameJa)
            )
        case .survive:
            guard let move = goalOpponentMove(draft) else {
                throw AdjustRejection(message: AdjustText.goalOpponentMoveRequiredMessage(n))
            }
            let preset = draft.attackerPreset
            let opponent = try goalOpponent(speciesKey, number: n) {
                let build = try AttackerPreset.build(preset, moveCategory: move.category, natures: $0)
                return (build.natureId, build.sp)
            }
            return (
                AdjustGoalInput(kind: .survive, opponent: opponent, moveId: move.id, hits: draft.hits, thresholdPercent: threshold),
                AdjustGoalSnapshot(
                    kind: .survive,
                    opponentName: AdjustText.goalOpponentName(species.nameJa, preset.label(for: move.category)),
                    moveName: move.nameJa, hits: draft.hits)
            )
        case .ko:
            guard let move = goalOwnMove(draft) else {
                throw AdjustRejection(message: AdjustText.goalOwnMoveRequiredMessage(n))
            }
            let preset = draft.defenderPreset
            let opponent = try goalOpponent(speciesKey, number: n) {
                let build = try KnownDefenderPreset.build(preset, moveCategory: move.category, natures: $0)
                return (build.natureId, build.sp)
            }
            return (
                AdjustGoalInput(kind: .ko, opponent: opponent, moveId: move.id, hits: draft.hits, thresholdPercent: threshold),
                AdjustGoalSnapshot(
                    kind: .ko,
                    opponentName: AdjustText.goalOpponentName(species.nameJa, preset.label(for: move.category)),
                    moveName: move.nameJa, hits: draft.hits)
            )
        }
    }

    /// プリセットから相手の個体を作る。性格がマスタに無ければ別の性格で代えずに違反にする。
    /// 相手がメガ種族ならストーンを持たせる(ADR-0331 §1。引けなければ送らない)。
    private func goalOpponent(
        _ speciesKey: String, number n: Int, build: ([Nature]) throws -> (natureId: String, sp: StatBlock)
    ) throws -> Individual {
        let built: (natureId: String, sp: StatBlock)
        do {
            built = try build(natureOptions)
        } catch {
            throw AdjustRejection(message: AdjustText.goalNatureNotFoundMessage(n))
        }
        let stone = MegaItemLock.make(for: megaInfo[speciesKey], allItems: itemOptions).lockedItemId
        return Individual(speciesKey: speciesKey, natureId: built.natureId, sp: built.sp, itemId: stone)
    }
}
