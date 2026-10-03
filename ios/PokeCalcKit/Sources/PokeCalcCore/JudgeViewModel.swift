// JudgeViewModel: 判定画面の状態(P6-25。ADR-0504)。
//
// 自分のポケモン1体と相手候補(1〜`RequestLimits.maxJudgeDefenders` 件)の入力から要求を作り、`JudgeService` の応答を表示用に整える。
// 計算ロジックは持たない(素早さ・行動順・確定数は judge-svc の応答のまま。勝敗の真偽値に丸めない)。
//
// 状態の作り(`SpeedViewModel`・`CalcViewModel` と同じ流儀):
//  - `@MainActor @Observable`。送信は **ボタンを押したときだけ**(入力のたびに送らない。debounce もしない。ADR-0705 §7)。
//  - 送信のたびに世代を進め、先行の要求は cancel し、**最新の世代の応答だけ**反映する(cancel を無視するサービスの古い応答も捨てる)。
//  - `CancellationError` は画面の失敗にしない。判定の失敗・マスタの失敗は計算・構築に影響しない(絶対ルール5)。`load()` は throw しない。
//
// 公開 API の形(名前・型・引数)は JudgeViewModel*Tests 群が固定する。

import Foundation
import Observation

// MARK: - 入力の対象・検査・失敗

/// 入力の対象。`.candidate(index)` は相手候補の位置(0 始まり)。
public enum JudgeTarget: Equatable, Hashable, Sendable {
    case attacker
    case candidate(Int)
}

/// 送信前の検査の違反(契約の範囲と同じ。違反していれば judge を呼ばずに理由を出す)。
/// 能力ポイントの1ステータスごとの範囲・ランクの範囲は、setter が収めるのでここでは扱わない(`setSP`/`setRank` のテストが固定する)。
public enum JudgeValidationError: Error, Equatable, Sendable {
    /// ポケモン・性格・技のどれかが未選択。
    case requiredMissing(JudgeTarget)
    /// 技の ID が契約の最大長(`RequestLimits.maxJudgeMoveIdLength`)を超える。
    case moveIdInvalid(JudgeTarget)
    /// 能力ポイントの合計が `SPLimits.maxTotal` を超える。
    case spTotalExceeded(JudgeTarget)

    public var message: String { JudgeLabels.validationMessage(self) }
}

/// 表示する失敗。`message` は `JudgeLabels.errorMessage(forCode:)`(サーバーの英語 message は出さない)。
/// サーバーの message に `defenders[<index>]` があれば、どの候補で失敗したかを `candidateNumber`(1 始まり)で示す(ADR-0703 §3)。
public struct JudgeFailure: Equatable, Sendable {
    public let code: String
    /// 失敗した候補の番号(1 始まり)。message から読めたときだけ(要求の候補数の範囲内のとき)。
    public let candidateNumber: Int?

    public var message: String { JudgeLabels.errorMessage(forCode: code) }
    /// 候補が分かるときの補助の行。
    public var candidateHint: String? { candidateNumber.map(JudgeLabels.failedCandidate) }

    public init(code: String, candidateNumber: Int? = nil) {
        self.code = code
        self.candidateNumber = candidateNumber
    }

    /// サーバーの message から作る。`defenders[2]` → 候補番号 3。範囲外(`candidateCount` 以上)・読めないときは nil。
    public init(code: String, serverMessage: String, candidateCount: Int) {
        self.init(code: code, candidateNumber: Self.candidateNumber(in: serverMessage, candidateCount: candidateCount))
    }

    /// message の `defenders[<index>]` の index だけを読む(message の他の部分は使わない。ADR-0706 §4)。
    private static func candidateNumber(in message: String, candidateCount: Int) -> Int? {
        guard let regex = try? NSRegularExpression(pattern: #"defenders\[([0-9]+)\]"#),
            let match = regex.firstMatch(in: message, range: NSRange(message.startIndex..., in: message)),
            let range = Range(match.range(at: 1), in: message),
            let index = Int(message[range]), index >= 0, index < candidateCount
        else { return nil }
        return index + 1
    }
}

/// 結果の状態。`idle` は未送信(「判定する」を押すと結果が出る旨を出す)。
public enum JudgeResultState: Equatable, Sendable {
    case idle
    case loading
    case loaded(JudgeResultDisplay)
    case failed(JudgeFailure)
}

/// 1体ぶんの入力(自分・候補で同じ形。`moveId` は自分なら要求直下の技、候補ならその候補が撃ち返す技)。
public struct JudgeDraft: Equatable, Sendable {
    public var speciesKey: String?
    public var natureId: String?
    public var sp: StatBlock
    public var ranks: RankBlock
    public var abilityId: String?
    public var itemId: String?
    public var moveId: String?

    public init(
        speciesKey: String? = nil, natureId: String? = nil,
        sp: StatBlock = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0), ranks: RankBlock = RankBlock(),
        abilityId: String? = nil, itemId: String? = nil, moveId: String? = nil
    ) {
        self.speciesKey = speciesKey
        self.natureId = natureId
        self.sp = sp
        self.ranks = ranks
        self.abilityId = abilityId
        self.itemId = itemId
        self.moveId = moveId
    }
}

/// 1体ぶんの入力に付く、画面表示用の補助(種族名・特性の候補)。候補を消しても名前・特性の候補が候補について行くよう、
/// 位置ではなく `id` で結ぶ(非同期の応答は `id` で自分の候補を探す)。
private struct JudgeSlotMeta: Equatable {
    var id: Int
    var speciesName: String?
    var abilityOptions: [Ability] = []
    /// 種族の選択のたびに進める世代(古い `species(key:)` の応答で最新の選択を壊さない)。
    var speciesToken = 0
}

@MainActor
@Observable
public final class JudgeViewModel: MasterSpeciesSearchProviding, MasterMoveSearchProviding {
    // MARK: - 入力

    public private(set) var attacker = JudgeDraft()
    /// 常に 1〜`RequestLimits.maxJudgeDefenders` 件(初期は空の1件)。
    public private(set) var candidates: [JudgeDraft] = [JudgeDraft()]
    public private(set) var speedField = JudgeSpeedField()

    // MARK: - 結果・検査

    public private(set) var resultState: JudgeResultState = .idle
    /// 直近の送信前検査の違反(送信に成功したら nil に戻る)。違反しているとき judge は呼ばれない。
    public private(set) var validationError: JudgeValidationError?

    // MARK: - マスタ(入力補助。ID の自由入力にはしない)

    public private(set) var natureOptions: [Nature] = []
    public private(set) var itemOptions: [Item] = []
    /// マスタの読み込み(`load()`)の失敗。判定・計算・構築には影響しない。
    public private(set) var masterFailure: JudgeFailure?
    public private(set) var speciesQuery = ""
    public private(set) var moveQuery = ""
    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var moveOptions: [Move] = []
    public private(set) var isSearchingSpecies = false
    public private(set) var isSearchingMoves = false
    public private(set) var speciesSearchReachedLimit = false
    public private(set) var moveSearchReachedLimit = false

    // MARK: - 構築から呼び出す(自分側・各候補側を構築のメンバーから埋める)

    public private(set) var teamOptions: [TeamPickerGroup] = []

    private let service: any JudgeService
    private let master: any PokeCalcService
    private let teamStore: (any TeamStore)?

    private let speciesSearch: MasterSearchField<SpeciesSummary>
    private let moveSearch: MasterSearchField<Move>
    /// 一度でも見た種族・技(検索結果・選んだもの)。名前の引き当てに使う。
    private var speciesDictionary: [String: SpeciesSummary] = [:]
    private var moveDictionary: [String: Move] = [:]
    /// 一度でも読んだ種族のメガ情報(`species(key:)` の応答ごとに覚える。ADR-0509 §4)。
    private var megaInfo: [String: MegaSpeciesInfo] = [:]
    private var loadedTeams: [Team] = []
    private var latestTeamListToken = 0

    private var attackerMeta: JudgeSlotMeta
    private var candidateMetas: [JudgeSlotMeta]
    private var nextSlotID = 2
    /// 「補正なし(plus も minus も無い)の最初の性格」。性格が読めるまでは nil。
    private var defaultNatureId: String?

    private let submitRunner = LatestTaskRunner()
    private var submitTask: Task<Void, Never>?
    private var submitGeneration = 0

    public init(
        service: any JudgeService, master: any PokeCalcService, teamStore: (any TeamStore)? = nil,
        searchDebounce: Duration = MasterSearch.debounceInterval
    ) {
        self.service = service
        self.master = master
        self.teamStore = teamStore
        attackerMeta = JudgeSlotMeta(id: 0)
        candidateMetas = [JudgeSlotMeta(id: 1)]
        speciesSearch = MasterSearchField(debounce: searchDebounce) { query, limit in
            try await master.searchSpecies(query: query, limit: limit)
        }
        moveSearch = MasterSearchField(debounce: searchDebounce) { query, limit in
            try await master.searchMoves(query: query, limit: limit)
        }
    }

    // MARK: - 読み込み

    /// 性格・持ち物・技・種族の先頭ページと、構築の一覧を取る。互いに独立(1つの失敗が他を消さない)。throw しない。
    /// 性格が読めたら、性格が未選択の自分・候補には「補正なし(plus も minus も無い)の最初の性格」を既定として入れる。
    public func load() async {
        var failure: JudgeFailure?
        func remember(_ error: any Error) {
            if failure == nil { failure = JudgeFailure(code: Self.code(of: error)) }
        }
        do {
            let natures = try await master.natures()
            natureOptions = natures
            defaultNatureId = natures.first { $0.plus == nil && $0.minus == nil }?.id
            applyDefaultNature()
        } catch {
            remember(error)
        }
        do {
            let items = try await master.searchItems(query: "", limit: MasterSearch.pageLimit)
            itemOptions = items
        } catch {
            remember(error)
        }
        do {
            let moves = try await master.searchMoves(query: "", limit: MasterSearch.pageLimit)
            moveSearch.setFirstPage(moves)
            moveOptions = moveSearch.options
            moveSearchReachedLimit = moveSearch.reachedLimit
            mergeMoves(moves)
        } catch {
            remember(error)
        }
        do {
            let species = try await master.searchSpecies(query: "", limit: MasterSearch.pageLimit)
            speciesSearch.setFirstPage(species)
            speciesOptions = speciesSearch.options
            speciesSearchReachedLimit = speciesSearch.reachedLimit
            mergeSpecies(species)
        } catch {
            remember(error)
        }
        masterFailure = failure
        await loadTeams()
    }

    public func loadTeams() async {
        guard let teamStore else {
            loadedTeams = []
            teamOptions = []
            return
        }
        latestTeamListToken += 1
        let token = latestTeamListToken
        let (teams, groups) = await TeamListFetcher.fetchGroups(from: teamStore, species: speciesOptions)
        guard token == latestTeamListToken else { return }
        loadedTeams = teams
        teamOptions = groups
    }

    // MARK: - 入力の操作(同期。対象の index が範囲外なら何もしない)

    public func draft(for target: JudgeTarget) -> JudgeDraft? {
        switch target {
        case .attacker: return attacker
        case .candidate(let index): return candidates.indices.contains(index) ? candidates[index] : nil
        }
    }

    /// 種族を選ぶ。キー・名前を同期で反映し、特性は新しい種族の候補に無ければ外す。特性の候補は `species(key:)` で取る
    /// (取れなくても入力は止めない。特性は任意)。最新の種族選択だけを反映する。
    public func setSpecies(_ species: SpeciesSummary, for target: JudgeTarget) async {
        guard let slot = slotID(for: target) else { return }
        speciesDictionary[species.key] = species
        let changed = draft(for: target)?.speciesKey != species.key
        let previousLock = itemLock(for: target)
        update(target) { $0.speciesKey = species.key }
        let token = updateMeta(slot) { meta in
            meta.speciesName = species.nameJa
            if changed { meta.abilityOptions = [] }
            meta.speciesToken += 1
            return meta.speciesToken
        }
        guard let token else { return }
        do {
            let detail = try await master.species(key: species.key)
            guard let index = currentTarget(ofSlot: slot), metaToken(slot) == token else { return }
            megaInfo[detail.key] = MegaSpeciesInfo(detail: detail)
            updateMeta(slot) { $0.abilityOptions = detail.abilities }
            let nextLock = MegaItemLock.make(for: megaInfo[detail.key], allItems: itemOptions)
            update(index) { draft in
                // メガ種族ならストーンに固定、メガ以外に変えたら未選択に戻す(ADR-0509 §4)。
                draft.itemId = MegaItemLock.itemIdAfterSpeciesChange(
                    previous: previousLock, next: nextLock, currentItemId: draft.itemId)
                if let ability = draft.abilityId, !detail.abilities.contains(where: { $0.id == ability }) {
                    draft.abilityId = nil
                }
            }
        } catch {
            // 特性の候補が取れなくても入力は止めない。確かめられない特性は外す(別の種族の特性を送らない)。
            guard let index = currentTarget(ofSlot: slot), metaToken(slot) == token, changed else { return }
            update(index) { $0.abilityId = nil }
        }
    }

    public func setNature(_ natureId: String?, for target: JudgeTarget) {
        update(target) { $0.natureId = natureId }
    }

    /// `0...SPLimits.maxPerStat` に収める。
    public func setSP(_ stat: StatKey, _ value: Int, for target: JudgeTarget) {
        let clamped = min(max(value, 0), SPLimits.maxPerStat)
        update(target) { draft in
            switch stat {
            case .hp: draft.sp.hp = clamped
            case .atk: draft.sp.atk = clamped
            case .def: draft.sp.def = clamped
            case .spa: draft.sp.spa = clamped
            case .spd: draft.sp.spd = clamped
            case .spe: draft.sp.spe = clamped
            }
        }
    }

    /// HP のランクは無い(`stat == .hp` は無視)。`RankLimits.min...RankLimits.max` に収める。
    public func setRank(_ stat: StatKey, _ value: Int, for target: JudgeTarget) {
        let clamped = min(max(value, RankLimits.min), RankLimits.max)
        update(target) { draft in
            switch stat {
            case .hp: break
            case .atk: draft.ranks.atk = clamped
            case .def: draft.ranks.def = clamped
            case .spa: draft.ranks.spa = clamped
            case .spd: draft.ranks.spd = clamped
            case .spe: draft.ranks.spe = clamped
            }
        }
    }

    public func setAbility(_ abilityId: String?, for target: JudgeTarget) {
        update(target) { $0.abilityId = abilityId }
    }

    public func setItem(_ itemId: String?, for target: JudgeTarget) {
        guard !itemLock(for: target).disablesItemField else { return }
        update(target) { $0.itemId = itemId }
    }

    /// 技を選ぶ(自分なら要求直下の技、候補ならその候補の技)。nil で未選択に戻す。
    public func setMove(_ move: Move?, for target: JudgeTarget) {
        guard draft(for: target) != nil else { return }
        if let move { moveDictionary[move.id] = move }
        update(target) { $0.moveId = move?.id }
    }

    public func setTrickRoom(_ value: Bool) {
        speedField.trickRoom = value
    }

    public func setAttackerTailwind(_ value: Bool) {
        speedField.attackerTailwind = value
    }

    public func setDefenderTailwind(_ value: Bool) {
        speedField.defenderTailwind = value
    }

    // MARK: - 相手候補の増減

    public var canAddCandidate: Bool { candidates.count < RequestLimits.maxJudgeDefenders }
    public var canRemoveCandidate: Bool { candidates.count > RequestLimits.minJudgeDefenders }

    /// 空の候補(性格は既定)を末尾に足す。上限なら足さず false。
    @discardableResult
    public func addCandidate() -> Bool {
        guard canAddCandidate else { return false }
        candidates.append(JudgeDraft(natureId: defaultNatureId))
        candidateMetas.append(JudgeSlotMeta(id: nextSlotID))
        nextSlotID += 1
        return true
    }

    /// 最低1件は残す。範囲外・最後の1件なら何もせず false。
    @discardableResult
    public func removeCandidate(at index: Int) -> Bool {
        guard canRemoveCandidate, candidates.indices.contains(index) else { return false }
        candidates.remove(at: index)
        candidateMetas.remove(at: index)
        return true
    }

    // MARK: - 名前・選択肢の参照

    /// 選んだ種族の名前(`setSpecies` で渡された値。構築から呼び出したときは `species(key:)` の応答)。未選択なら nil。
    public func speciesName(for target: JudgeTarget) -> String? {
        meta(for: target)?.speciesName
    }

    /// 種族ごとの特性の候補(`species(key:)` の応答)。未取得・取得失敗なら空。
    public func abilityOptions(for target: JudgeTarget) -> [Ability] {
        meta(for: target)?.abilityOptions ?? []
    }

    /// 一度でも見た技(検索結果・`setMove`)から引く。無ければ nil。
    public func move(forID id: String) -> Move? {
        moveDictionary[id]
    }

    // MARK: - 検索(`MasterSpeciesSearchProviding` / `MasterMoveSearchProviding` と同じ規則。検索の状態は画面に1つ)

    @discardableResult
    public func setSpeciesQuery(_ text: String) -> Bool {
        let needsSearch = speciesSearch.setQuery(text)
        speciesQuery = speciesSearch.query
        return needsSearch
    }

    @discardableResult
    public func setMoveQuery(_ text: String) -> Bool {
        let needsSearch = moveSearch.setQuery(text)
        moveQuery = moveSearch.query
        return needsSearch
    }

    public func runSpeciesSearch() async {
        let results = await speciesSearch.run()
        speciesOptions = speciesSearch.options
        isSearchingSpecies = speciesSearch.isSearching
        speciesSearchReachedLimit = speciesSearch.reachedLimit
        if let results { mergeSpecies(results) }
    }

    public func runMoveSearch() async {
        let results = await moveSearch.run()
        moveOptions = moveSearch.options
        isSearchingMoves = moveSearch.isSearching
        moveSearchReachedLimit = moveSearch.reachedLimit
        if let results { mergeMoves(results) }
    }

    // MARK: - 構築から呼び出す

    /// 構築のメンバー1体の内容を、自分または候補1体の入力に写す(種族・性格・SP・特性・持ち物・技。ランクは構築に無いので 0 に戻す)。
    /// 技は `TeamMemberConverter` の規則(最初のダメージ技、無ければ最初の技、無ければ未選択)。`teamOptions` に無い ID は何もしない。
    /// 写したあとも各欄は直せる(スナップショット。構築側の以後の編集は追従しない)。
    public func applyTeamMember(teamID: String, memberID: String, to target: JudgeTarget) async {
        guard let slot = slotID(for: target),
            let selection = TeamIndividualSelectionBuilder.make(
                teamID: teamID, memberID: memberID, teamOptions: teamOptions, teams: loadedTeams,
                moves: Array(moveDictionary.values))
        else { return }
        let individual = selection.individual
        update(target) { draft in
            draft = JudgeDraft(
                speciesKey: individual.speciesKey, natureId: individual.natureId, sp: individual.sp,
                abilityId: individual.abilityId, itemId: individual.itemId, moveId: individual.moveId)
        }
        let token = updateMeta(slot) { meta in
            meta.speciesName = speciesDictionary[individual.speciesKey]?.nameJa
            meta.abilityOptions = []
            meta.speciesToken += 1
            return meta.speciesToken
        }
        guard let token else { return }
        if let moveId = individual.moveId, moveDictionary[moveId] == nil, let move = try? await master.move(id: moveId) {
            moveDictionary[moveId] = move
        }
        guard let detail = try? await master.species(key: individual.speciesKey) else { return }
        guard metaToken(slot) == token else { return }
        speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
        megaInfo[detail.key] = MegaSpeciesInfo(detail: detail)
        updateMeta(slot) { meta in
            meta.speciesName = detail.nameJa
            meta.abilityOptions = detail.abilities
        }
        // 呼び出した個体がメガなら持ち物をストーンに固定する(ADR-0509 §4)。
        if let index = currentTarget(ofSlot: slot) {
            let nextLock = MegaItemLock.make(for: megaInfo[detail.key], allItems: itemOptions)
            update(index) { draft in
                draft.itemId = MegaItemLock.itemIdAfterSpeciesChange(previous: .none, next: nextLock, currentItemId: draft.itemId)
            }
        }
    }

    // MARK: - 送信

    /// 入力から要求を作る。違反があれば最初の1件(検査の順: 自分 → 候補を index 昇順。各体の中は 必須 → 技 ID → 能力ポイント合計)。
    /// 省略の規則(ADR-0504 §5。Web と同じ): ランクは5項目すべて 0 なら nil・特性/持ち物は未選択なら nil・speedField は3つすべて false なら nil。format は常に single。
    public func makeRequest() -> Result<JudgeRequest, JudgeValidationError> {
        if let violation = validate(attacker, as: .attacker) { return .failure(violation) }
        for (index, draft) in candidates.enumerated() {
            if let violation = validate(draft, as: .candidate(index)) { return .failure(violation) }
        }
        // 検査を通っているので、必須の欄は埋まっている。
        guard let attackerIndividual = individual(from: attacker), let attackerMove = attacker.moveId else {
            return .failure(.requiredMissing(.attacker))
        }
        var defenders: [JudgeDefender] = []
        for (index, draft) in candidates.enumerated() {
            guard let individual = individual(from: draft), let moveId = draft.moveId else {
                return .failure(.requiredMissing(.candidate(index)))
            }
            defenders.append(JudgeDefender(individual: individual, moveId: moveId))
        }
        return .success(
            JudgeRequest(
                format: .single, attacker: attackerIndividual, moveId: attackerMove, defenders: defenders,
                speedField: speedField.isDefault ? nil : speedField))
    }

    /// 「判定する」。同期で: 検査 → 違反なら `validationError` を立てて終わる(呼ばない・結果は変えない)/ 通れば `validationError = nil`・
    /// `resultState = .loading`・世代を進め先行を cancel し、要求を予約する。結果は送信時点の入力から整える(送信後の入力変更の影響を受けない)。
    public func submit() {
        let request: JudgeRequest
        switch makeRequest() {
        case .failure(let error):
            validationError = error
            return
        case .success(let built):
            request = built
        }
        validationError = nil
        resultState = .loading
        submitGeneration += 1
        let generation = submitGeneration
        let species = speciesDictionary
        let names = UnsupportedMarkNames(
            moves: Array(moveDictionary.values),
            items: ItemDisplayName.displayItems(itemOptions, megaStoneNames: megaStoneNames),
            abilities: ([attackerMeta] + candidateMetas).flatMap(\.abilityOptions))
        submitTask = submitRunner.schedule(debounce: .zero) { [self] in
            await perform(request, generation: generation, species: species, names: names)
        }
    }

    /// 予約済みの送信が終わる(または cancel される)まで待つ。テストと View の `.task` 用。
    public func settle() async {
        await submitTask?.value
    }

    /// 画面破棄(`.onDisappear`)。送信済みの要求と検索を cancel する。
    public func cancelPendingWork() {
        submitRunner.cancel()
        submitGeneration += 1
        if resultState == .loading { resultState = .idle }
    }

    // MARK: - 内部

    private func perform(
        _ request: JudgeRequest, generation: Int, species: [String: SpeciesSummary], names: UnsupportedMarkNames
    ) async {
        do {
            let response = try await service.outspeedAndKo(request)
            guard generation == submitGeneration else { return }
            resultState = .loaded(
                JudgeResultDisplayBuilder.make(response: response, request: request, species: species, names: names))
        } catch is CancellationError {
            return
        } catch {
            guard generation == submitGeneration else { return }
            let message = (error as? PokeCalcError)?.message ?? ""
            resultState = .failed(
                JudgeFailure(code: Self.code(of: error), serverMessage: message, candidateCount: request.defenders.count))
        }
    }

    private static func code(of error: any Error) -> String {
        (error as? PokeCalcError)?.code ?? PokeCalcError.Code.transport
    }

    private func applyDefaultNature() {
        guard let defaultNatureId else { return }
        if attacker.natureId == nil { attacker.natureId = defaultNatureId }
        for index in candidates.indices where candidates[index].natureId == nil {
            candidates[index].natureId = defaultNatureId
        }
    }

    private func mergeSpecies(_ items: [SpeciesSummary]) {
        for item in items { speciesDictionary[item.key] = item }
    }

    private func mergeMoves(_ items: [Move]) {
        for item in items { moveDictionary[item.id] = item }
    }

    private func update(_ target: JudgeTarget, _ body: (inout JudgeDraft) -> Void) {
        switch target {
        case .attacker:
            body(&attacker)
        case .candidate(let index):
            guard candidates.indices.contains(index) else { return }
            body(&candidates[index])
        }
    }

    private func meta(for target: JudgeTarget) -> JudgeSlotMeta? {
        switch target {
        case .attacker: return attackerMeta
        case .candidate(let index): return candidateMetas.indices.contains(index) ? candidateMetas[index] : nil
        }
    }

    private func slotID(for target: JudgeTarget) -> Int? { meta(for: target)?.id }

    /// 非同期の応答が戻ったとき、`id` の体がいまどこにいるか(消されていれば nil)。
    private func currentTarget(ofSlot id: Int) -> JudgeTarget? {
        if attackerMeta.id == id { return .attacker }
        return candidateMetas.firstIndex { $0.id == id }.map(JudgeTarget.candidate)
    }

    private func metaToken(_ id: Int) -> Int? {
        currentTarget(ofSlot: id).flatMap { meta(for: $0)?.speciesToken }
    }

    @discardableResult
    private func updateMeta<T>(_ id: Int, _ body: (inout JudgeSlotMeta) -> T) -> T? {
        if attackerMeta.id == id { return body(&attackerMeta) }
        guard let index = candidateMetas.firstIndex(where: { $0.id == id }) else { return nil }
        return body(&candidateMetas[index])
    }

    private func validate(_ draft: JudgeDraft, as target: JudgeTarget) -> JudgeValidationError? {
        guard draft.speciesKey != nil, draft.natureId != nil, let moveId = draft.moveId else {
            return .requiredMissing(target)
        }
        if moveId.count > RequestLimits.maxJudgeMoveIdLength { return .moveIdInvalid(target) }
        let sp = draft.sp
        if sp.hp + sp.atk + sp.def + sp.spa + sp.spd + sp.spe > SPLimits.maxTotal { return .spTotalExceeded(target) }
        return nil
    }

    private func individual(from draft: JudgeDraft) -> JudgeIndividual? {
        guard let speciesKey = draft.speciesKey, let natureId = draft.natureId else { return nil }
        return JudgeIndividual(
            speciesKey: speciesKey, natureId: natureId, sp: draft.sp,
            ranks: draft.ranks == RankBlock() ? nil : draft.ranks, abilityId: draft.abilityId, itemId: draft.itemId)
    }
}

// MARK: - 持ち物の役割・メガ固定(ADR-0509)

extension JudgeViewModel {
    /// 自分・候補の持ち物の選択肢(`.any`。判定は攻守の両方をするため)。
    public var selectableItemOptions: [Item] {
        ItemRoleFilter.options(itemOptions, for: .any)
    }

    /// `target` の選択肢(`selectableItemOptions` + そのいまの持ち物を残す)。
    public func selectableItemOptions(for target: JudgeTarget) -> [Item] {
        ItemRoleFilter.options(itemOptions, for: .any, keeping: draft(for: target)?.itemId)
    }

    public func itemLock(for target: JudgeTarget) -> MegaItemLock {
        guard let key = draft(for: target)?.speciesKey else { return .none }
        return MegaItemLock.make(for: megaInfo[key], allItems: itemOptions)
    }

    public func itemLabel(for itemId: String?) -> String {
        ItemDisplayName.text(itemId: itemId, items: itemOptions, megaStoneNames: megaStoneNames)
    }

    private var megaStoneNames: [String: String] {
        var names: [String: String] = [:]
        for info in megaInfo.values {
            if case .locked(let itemId, let displayName) = MegaItemLock.make(for: info, allItems: itemOptions) {
                names[itemId] = displayName
            }
        }
        return names
    }
}
