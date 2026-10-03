import Foundation
import Observation

// BalanceViewModel: タイプバランス画面の状態(ADR-0415。第1段=防御相性・チーム集計、第2段=攻撃範囲)。
//
// マスタ(種族・技・特性)は構築画面と同じ `PokeCalcService` を再利用し、架空データへはフォールバックしない。
// balance の応答は表示するだけで、iOS では相性を計算しない。analyze と coverage は互いに独立
// (片方の失敗・遅延がもう片方を止めない)で、それぞれ「最新の呼び出しの応答だけ」を反映する(世代カウンタ)。

/// 画面のメンバー1体(構築 `TeamMember` から SP・性格などを省いた、balance に要るものだけ)。
public struct BalanceMember: Equatable, Sendable, Identifiable {
    public var id: String
    public var speciesKey: String
    public var nameJa: String
    public var types: [PokeType]
    public var abilityId: String?
    public var moveIds: [String]

    public init(
        id: String = UUID().uuidString, speciesKey: String, nameJa: String, types: [PokeType],
        abilityId: String? = nil, moveIds: [String] = []
    ) {
        self.id = id
        self.speciesKey = speciesKey
        self.nameJa = nameJa
        self.types = types
        self.abilityId = abilityId
        self.moveIds = moveIds
    }
}

@MainActor
@Observable
public final class BalanceViewModel: MasterSpeciesSearchProviding, MasterMoveSearchProviding {
    private let balance: any BalanceService
    private let master: any PokeCalcService
    private let refreshDebounce: Duration
    private let refreshRunner = LatestTaskRunner()
    private let speciesSearch: MasterSearchField<SpeciesSummary>
    private let moveSearch: MasterSearchField<Move>

    // MARK: - 検索(構築編集と同じ規則: 先頭ページ・前方一致の検索・上限の案内。ADR-0501「issue #68」)

    public private(set) var speciesQuery = ""
    public private(set) var moveQuery = ""
    public private(set) var isSearchingSpecies = false
    public private(set) var isSearchingMoves = false
    public private(set) var speciesSearchReachedLimit = false
    public private(set) var moveSearchReachedLimit = false

    // MARK: - マスタ

    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var isLoadingMaster = false
    public private(set) var masterError: BalanceScreenError?
    /// 直近の技検索の結果(空クエリなら `load()` の先頭ページ)。メンバーの技の候補はこれと learnset の交差(構築編集と同じ規則)。
    private var latestMoveSearchResults: [Move] = []
    /// 一度でも見た技(先頭ページ・検索結果・構築の技の解決)。技名の表示に使う。
    private var moveDictionary: [String: Move] = [:]
    private var learnsetIdsByMember: [String: [String]] = [:]

    // MARK: - メンバー

    public private(set) var members: [BalanceMember] = []
    public private(set) var abilityOptionsByMember: [String: [Ability]] = [:]
    public private(set) var moveOptionsByMember: [String: [Move]] = [:]
    public private(set) var teamError: TeamFieldError?
    public private(set) var memberErrors: [String: TeamMemberFieldError] = [:]

    // MARK: - 結果(analyze と coverage は独立)

    public private(set) var analysis: BalanceDefenseAnalysis?
    public private(set) var coverage: BalanceCoverageAnalysis?
    public private(set) var isLoadingAnalysis = false
    public private(set) var isLoadingCoverage = false
    public private(set) var analysisError: BalanceScreenError?
    public private(set) var coverageError: BalanceScreenError?

    /// 呼び出しごとの世代。最新の呼び出しの応答だけを反映する(古い応答は成功でも失敗でも捨てる)。
    private var analysisGeneration = 0
    private var coverageGeneration = 0

    public init(
        balance: any BalanceService, master: any PokeCalcService,
        refreshDebounce: Duration = MasterSearch.debounceInterval
    ) {
        self.balance = balance
        self.master = master
        self.refreshDebounce = refreshDebounce
        speciesSearch = MasterSearchField(debounce: MasterSearch.debounceInterval) { query, limit in
            try await master.searchSpecies(query: query, limit: limit)
        }
        moveSearch = MasterSearchField(debounce: MasterSearch.debounceInterval) { query, limit in
            try await master.searchMoves(query: query, limit: limit)
        }
    }

    // MARK: - 起動

    /// 種族の先頭ページと技の先頭ページを1回ずつ取る。失敗したら `masterError` を立て、選択肢は空のまま。
    public func load() async {
        isLoadingMaster = true
        do {
            let species = try await master.searchSpecies(query: "", limit: MasterSearch.pageLimit)
            let moves = try await master.searchMoves(query: "", limit: MasterSearch.pageLimit)
            speciesSearch.setFirstPage(species)
            speciesOptions = speciesSearch.options
            speciesSearchReachedLimit = speciesSearch.reachedLimit
            moveSearch.setFirstPage(moves)
            latestMoveSearchResults = moveSearch.options
            moveSearchReachedLimit = moveSearch.reachedLimit
            for move in moves { moveDictionary[move.id] = move }
            masterError = nil
        } catch {
            if !(error is CancellationError) { masterError = BalanceScreenError(error) }
        }
        isLoadingMaster = false
    }

    /// 一度でも見た技(技名の表示用)。無ければ nil(View は ID をそのまま出す)。
    public func move(forID id: String) -> Move? {
        moveDictionary[id]
    }

    // MARK: - 検索

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
        _ = await speciesSearch.run()
        speciesOptions = speciesSearch.options
        isSearchingSpecies = speciesSearch.isSearching
        speciesSearchReachedLimit = speciesSearch.reachedLimit
    }

    /// 候補(`moveOptionsByMember`)は全メンバー分作り直す。検索結果は技名の辞書にも合流させる。
    public func runMoveSearch() async {
        let results = await moveSearch.run()
        latestMoveSearchResults = moveSearch.options
        isSearchingMoves = moveSearch.isSearching
        moveSearchReachedLimit = moveSearch.reachedLimit
        if let results { for move in results { moveDictionary[move.id] = move } }
        for memberId in learnsetIdsByMember.keys { recomputeMoveOptions(forMember: memberId) }
    }

    // MARK: - メンバーの操作

    /// 上限なら追加せず false + `teamError = .tooManyMembers`(要求は出さない)。マスタを引けなければ追加せず false + `masterError`。
    @discardableResult
    public func addMember(speciesKey: String) async -> Bool {
        guard members.count < TeamLimits.maxMembers else {
            teamError = .tooManyMembers
            return false
        }
        let detail: SpeciesDetail
        do {
            detail = try await master.species(key: speciesKey)
        } catch {
            if !(error is CancellationError) { masterError = BalanceScreenError(error) }
            return false
        }
        // 待っている間に別の追加で上限に達した場合を取りこぼさない。
        guard members.count < TeamLimits.maxMembers else {
            teamError = .tooManyMembers
            return false
        }
        let member = BalanceMember(speciesKey: detail.key, nameJa: detail.nameJa, types: detail.types)
        members.append(member)
        register(detail, for: member.id)
        teamError = nil
        scheduleRefresh()
        return true
    }

    public func removeMember(id: String) {
        members.removeAll { $0.id == id }
        abilityOptionsByMember[id] = nil
        moveOptionsByMember[id] = nil
        learnsetIdsByMember[id] = nil
        memberErrors[id] = nil
        teamError = nil
        scheduleRefresh()
    }

    public func setAbility(id: String, abilityId: String?) {
        guard let index = members.firstIndex(where: { $0.id == id }) else { return }
        members[index].abilityId = abilityId
        scheduleRefresh()
    }

    /// 技は1体4つまで・重複不可(構築編集と同じ規則)。違反は false + `memberErrors[id]`(要求は出さない)。
    @discardableResult
    public func addMove(id: String, moveId: String) -> Bool {
        guard let index = members.firstIndex(where: { $0.id == id }) else { return false }
        guard members[index].moveIds.count < TeamLimits.maxMovesPerMember else {
            memberErrors[id] = .tooManyMoves
            return false
        }
        guard !members[index].moveIds.contains(moveId) else {
            memberErrors[id] = .duplicateMove
            return false
        }
        members[index].moveIds.append(moveId)
        memberErrors[id] = nil
        scheduleRefresh()
        return true
    }

    /// 範囲外の index は無視する。
    public func removeMove(id: String, at index: Int) {
        guard let memberIndex = members.firstIndex(where: { $0.id == id }),
            members[memberIndex].moveIds.indices.contains(index)
        else { return }
        members[memberIndex].moveIds.remove(at: index)
        memberErrors[id] = nil
        scheduleRefresh()
    }

    /// 保存済みの構築のメンバーで入れ替える。1体でもマスタを引けなければ入れ替えず `masterError` を立てる。
    public func loadTeam(_ team: Team) async {
        var loaded: [(member: BalanceMember, detail: SpeciesDetail)] = []
        for teamMember in team.members.prefix(TeamLimits.maxMembers) {
            do {
                let detail = try await master.species(key: teamMember.speciesKey)
                let member = BalanceMember(
                    speciesKey: detail.key, nameJa: detail.nameJa, types: detail.types,
                    abilityId: teamMember.abilityId, moveIds: teamMember.moveIds
                )
                loaded.append((member, detail))
            } catch {
                if !(error is CancellationError) { masterError = BalanceScreenError(error) }
                return
            }
        }
        await resolveUnknownMoves(loaded.flatMap(\.member.moveIds))
        members = loaded.map(\.member)
        abilityOptionsByMember = [:]
        moveOptionsByMember = [:]
        learnsetIdsByMember = [:]
        memberErrors = [:]
        teamError = nil
        for entry in loaded { register(entry.detail, for: entry.member.id) }
        scheduleRefresh()
    }

    // MARK: - 計算

    /// 連続操作を `refreshDebounce` でまとめて、最新の1回だけ `refresh()` する。待機中も「計算中」にする。
    public func scheduleRefresh() {
        if members.isEmpty {
            refreshRunner.cancel()
            clearResults()
        } else {
            isLoadingAnalysis = true
            isLoadingCoverage = true
        }
        guard !members.isEmpty else { return }
        refreshRunner.schedule(debounce: refreshDebounce) { [weak self] in
            await self?.refresh()
        }
    }

    /// メンバーが0体なら要求を出さず結果を消す。1体以上なら analyze と coverage を両方、全メンバー分で呼ぶ。
    public func refresh() async {
        guard !members.isEmpty else {
            clearResults()
            return
        }
        let inputs = members.map {
            BalanceMemberInput(pokemonId: $0.speciesKey, abilityId: $0.abilityId, moveIds: $0.moveIds)
        }
        analysisGeneration += 1
        coverageGeneration += 1
        let analysisToken = analysisGeneration
        let coverageToken = coverageGeneration
        isLoadingAnalysis = true
        isLoadingCoverage = true
        async let analyzeDone: Void = runAnalyze(inputs, token: analysisToken)
        async let coverageDone: Void = runCoverage(inputs, token: coverageToken)
        _ = await (analyzeDone, coverageDone)
    }

    /// 画面を離れるとき。保持中の Task を cancel し、計算中を解除し、以後に届く応答は反映しない。
    public func cancel() {
        refreshRunner.cancel()
        analysisGeneration += 1
        coverageGeneration += 1
        isLoadingAnalysis = false
        isLoadingCoverage = false
    }

    private func runAnalyze(_ inputs: [BalanceMemberInput], token: Int) async {
        do {
            let result = try await balance.analyze(members: inputs)
            guard token == analysisGeneration else { return }
            analysis = result
            analysisError = nil
        } catch is CancellationError {
            return
        } catch {
            guard token == analysisGeneration else { return }
            analysis = nil
            analysisError = BalanceScreenError(error)
        }
        isLoadingAnalysis = false
    }

    private func runCoverage(_ inputs: [BalanceMemberInput], token: Int) async {
        do {
            let result = try await balance.coverage(members: inputs)
            guard token == coverageGeneration else { return }
            coverage = result
            coverageError = nil
        } catch is CancellationError {
            return
        } catch {
            guard token == coverageGeneration else { return }
            coverage = nil
            coverageError = BalanceScreenError(error)
        }
        isLoadingCoverage = false
    }

    private func clearResults() {
        analysisGeneration += 1
        coverageGeneration += 1
        analysis = nil
        coverage = nil
        analysisError = nil
        coverageError = nil
        isLoadingAnalysis = false
        isLoadingCoverage = false
    }

    // MARK: - マスタの候補

    private func register(_ detail: SpeciesDetail, for memberId: String) {
        abilityOptionsByMember[memberId] = detail.abilities
        learnsetIdsByMember[memberId] = detail.learnset
        // learnset の順で、直近の技検索の結果にあるものだけ(構築編集と同じ規則)。
        recomputeMoveOptions(forMember: memberId)
    }

    private func recomputeMoveOptions(forMember memberId: String) {
        moveOptionsByMember[memberId] = (learnsetIdsByMember[memberId] ?? []).compactMap { learnedId in
            latestMoveSearchResults.first { $0.id == learnedId }
        }
    }

    /// 構築から読んだ技のうち、まだ見ていないものを `moves(ids:)` で1回だけ引く。失敗は無視する(技名が ID のまま出るだけ)。
    private func resolveUnknownMoves(_ moveIds: [String]) async {
        var seen = Set<String>()
        let missing = moveIds.filter { moveDictionary[$0] == nil && seen.insert($0).inserted }
        guard !missing.isEmpty, let resolved = try? await master.moves(ids: missing) else { return }
        for move in resolved { moveDictionary[move.id] = move }
    }
}
