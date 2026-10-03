import Foundation
import Observation

// BalanceViewModel: タイプバランス画面の状態(ADR-0415。第1段=防御相性・チーム集計、第2段=攻撃範囲、
// 第3段=仮想敵・おすすめタイプ・技範囲チェッカー〈§8〉)。
//
// マスタ(種族・技・特性)は構築画面と同じ `PokeCalcService` を再利用し、架空データへはフォールバックしない。
// balance の応答は表示するだけで、iOS では相性を計算しない。analyze と coverage は互いに独立
// (片方の失敗・遅延がもう片方を止めない)で、それぞれ「最新の呼び出しの応答だけ」を反映する(世代カウンタ)。
// 第3段の3機能(threats・recommendations・moveRange)も同じく独立した世代カウンタを持つ。

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
    private let recommendationsDebounce: Duration
    private let refreshRunner = LatestTaskRunner()
    private let threatsRunner = LatestTaskRunner()
    private let recommendationsRunner = LatestTaskRunner()
    private let moveRangeRunner = LatestTaskRunner()
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

    // MARK: - 第3段: 仮想敵(threats。最大 `TeamLimits.maxMembers` 体。メンバーと同じ入力)

    public private(set) var threatMembers: [BalanceMember] = []
    public private(set) var threatError: TeamFieldError?
    public private(set) var threats: BalanceThreatsAnalysis?
    public private(set) var isLoadingThreats = false
    public private(set) var threatsError: BalanceScreenError?
    private var threatsGeneration = 0

    // MARK: - 第3段: おすすめタイプ(recommendations。メンバーだけで決まる。仮想敵は含めない)

    public private(set) var recommendations: BalanceRecommendations?
    public private(set) var isLoadingRecommendations = false
    public private(set) var recommendationsError: BalanceScreenError?
    private var recommendationsGeneration = 0

    // MARK: - 第3段: 技範囲チェッカー(move-range。技 ID 1〜4 件だけ。メンバーとは無関係)

    public private(set) var rangeMoveIds: [String] = []
    public private(set) var rangeInputError: TeamMemberFieldError?
    public private(set) var moveRange: BalanceMoveRange?
    public private(set) var isLoadingMoveRange = false
    public private(set) var moveRangeError: BalanceScreenError?
    private var moveRangeGeneration = 0

    // MARK: - 第3段: 特性名(応答に出たポケモンの `species(key:)` から引く。引けなければ nil)

    private var abilityNames: [String: String] = [:]
    private var abilityNameResolvedPokemon: Set<String> = []

    /// おすすめタイプ専用の debounce の既定。サーバーは重い計算の同時数を絞り、超えると overloaded を返す(ADR-0409)ため、
    /// メンバーの編集ごとには呼ばず、操作が落ち着いてから1回だけ呼ぶ(ADR-0415 §8)。
    public static let defaultRecommendationsDebounce: Duration = .seconds(1)

    public init(
        balance: any BalanceService, master: any PokeCalcService,
        refreshDebounce: Duration = MasterSearch.debounceInterval,
        recommendationsDebounce: Duration = BalanceViewModel.defaultRecommendationsDebounce
    ) {
        self.balance = balance
        self.master = master
        self.refreshDebounce = refreshDebounce
        self.recommendationsDebounce = recommendationsDebounce
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

    /// メンバーでも仮想敵でも、id で引いて代入する(仮想敵は threats だけ再計算する)。
    public func setAbility(id: String, abilityId: String?) {
        guard let slot = slot(forID: id) else { return }
        update(slot) { $0.abilityId = abilityId }
        scheduleRefresh(for: slot)
    }

    /// 技は1体4つまで・重複不可(構築編集と同じ規則)。違反は false + `memberErrors[id]`(要求は出さない)。
    @discardableResult
    public func addMove(id: String, moveId: String) -> Bool {
        guard let slot = slot(forID: id) else { return false }
        let current = entry(at: slot).moveIds
        guard current.count < TeamLimits.maxMovesPerMember else {
            memberErrors[id] = .tooManyMoves
            return false
        }
        guard !current.contains(moveId) else {
            memberErrors[id] = .duplicateMove
            return false
        }
        update(slot) { $0.moveIds.append(moveId) }
        memberErrors[id] = nil
        scheduleRefresh(for: slot)
        return true
    }

    /// 範囲外の index は無視する。
    public func removeMove(id: String, at index: Int) {
        guard let slot = slot(forID: id), entry(at: slot).moveIds.indices.contains(index) else { return }
        update(slot) { $0.moveIds.remove(at: index) }
        memberErrors[id] = nil
        scheduleRefresh(for: slot)
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
    /// メンバーの編集で呼ぶ。analyze・coverage・(仮想敵があれば)threats は `refreshDebounce`、おすすめは専用の debounce。
    public func scheduleRefresh() {
        guard !members.isEmpty else {
            refreshRunner.cancel()
            threatsRunner.cancel()
            recommendationsRunner.cancel()
            clearResults()
            return
        }
        isLoadingAnalysis = true
        isLoadingCoverage = true
        isLoadingThreats = !threatMembers.isEmpty
        refreshRunner.schedule(debounce: refreshDebounce) { [weak self] in
            await self?.refresh()
        }
        scheduleRecommendationsRefresh()
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
        async let threatsDone: Void = refreshThreats()
        _ = await (analyzeDone, coverageDone, threatsDone)
    }

    /// 画面を離れるとき。保持中の Task を cancel し、計算中を解除し、以後に届く応答は反映しない。
    public func cancel() {
        refreshRunner.cancel()
        threatsRunner.cancel()
        recommendationsRunner.cancel()
        moveRangeRunner.cancel()
        analysisGeneration += 1
        coverageGeneration += 1
        threatsGeneration += 1
        recommendationsGeneration += 1
        moveRangeGeneration += 1
        isLoadingAnalysis = false
        isLoadingCoverage = false
        isLoadingThreats = false
        isLoadingRecommendations = false
        isLoadingMoveRange = false
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

    /// メンバーが0体のとき。メンバーに依存する全結果(analyze・coverage・threats・recommendations)を消す。
    private func clearResults() {
        analysisGeneration += 1
        coverageGeneration += 1
        analysis = nil
        coverage = nil
        analysisError = nil
        coverageError = nil
        isLoadingAnalysis = false
        isLoadingCoverage = false
        clearThreats()
        clearRecommendations()
    }

    // MARK: - マスタの候補

    private func register(_ detail: SpeciesDetail, for memberId: String) {
        abilityOptionsByMember[memberId] = detail.abilities
        for ability in detail.abilities { abilityNames[ability.id] = ability.nameJa }
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

    // MARK: - 第3段: 仮想敵

    /// 上限なら追加せず false + `threatError = .tooManyMembers`(要求は出さない)。マスタを引けなければ追加せず false + `masterError`。
    @discardableResult
    public func addThreat(speciesKey: String) async -> Bool {
        guard threatMembers.count < TeamLimits.maxMembers else {
            threatError = .tooManyMembers
            return false
        }
        let detail: SpeciesDetail
        do {
            detail = try await master.species(key: speciesKey)
        } catch {
            if !(error is CancellationError) { masterError = BalanceScreenError(error) }
            return false
        }
        guard threatMembers.count < TeamLimits.maxMembers else {
            threatError = .tooManyMembers
            return false
        }
        let threat = BalanceMember(speciesKey: detail.key, nameJa: detail.nameJa, types: detail.types)
        threatMembers.append(threat)
        register(detail, for: threat.id)
        threatError = nil
        scheduleThreatsRefresh()
        return true
    }

    public func removeThreat(id: String) {
        threatMembers.removeAll { $0.id == id }
        abilityOptionsByMember[id] = nil
        moveOptionsByMember[id] = nil
        learnsetIdsByMember[id] = nil
        memberErrors[id] = nil
        threatError = nil
        scheduleThreatsRefresh()
    }

    /// 仮想敵の編集で呼ぶ。メンバー0体または仮想敵0体なら要求せず結果を消す。
    public func scheduleThreatsRefresh() {
        guard !members.isEmpty, !threatMembers.isEmpty else {
            threatsRunner.cancel()
            clearThreats()
            return
        }
        isLoadingThreats = true
        threatsRunner.schedule(debounce: refreshDebounce) { [weak self] in
            await self?.refreshThreats()
        }
    }

    /// メンバー・仮想敵とも1体以上のときだけ threats を呼ぶ(Web の ADR-0303 §7 と同じ)。最新の呼び出しの応答だけを反映する。
    public func refreshThreats() async {
        guard !members.isEmpty, !threatMembers.isEmpty else {
            clearThreats()
            return
        }
        threatsGeneration += 1
        let token = threatsGeneration
        isLoadingThreats = true
        do {
            let result = try await balance.threats(members: balanceInputs(members), threats: balanceInputs(threatMembers))
            guard token == threatsGeneration else { return }
            threats = result
            threatsError = nil
        } catch is CancellationError {
            return
        } catch {
            guard token == threatsGeneration else { return }
            threats = nil
            threatsError = BalanceScreenError(error)
        }
        isLoadingThreats = false
    }

    private func clearThreats() {
        threatsGeneration += 1
        threats = nil
        threatsError = nil
        isLoadingThreats = false
    }

    // MARK: - 第3段: おすすめタイプ

    /// メンバーの編集で呼ぶ(`recommendationsDebounce` だけ待つ)。メンバー0体なら要求せず結果を消す。
    private func scheduleRecommendationsRefresh() {
        guard !members.isEmpty else {
            recommendationsRunner.cancel()
            clearRecommendations()
            return
        }
        isLoadingRecommendations = true
        recommendationsRunner.schedule(debounce: recommendationsDebounce) { [weak self] in
            await self?.refreshRecommendations()
        }
    }

    /// メンバーが1体以上のときだけ呼ぶ(仮想敵は含めない。`limit` は送らない)。失敗後の再試行にも使う。
    public func refreshRecommendations() async {
        guard !members.isEmpty else {
            clearRecommendations()
            return
        }
        recommendationsGeneration += 1
        let token = recommendationsGeneration
        isLoadingRecommendations = true
        let result: BalanceRecommendations
        do {
            result = try await balance.recommendations(members: balanceInputs(members), limit: nil)
            guard token == recommendationsGeneration else { return }
            recommendations = result
            recommendationsError = nil
        } catch is CancellationError {
            return
        } catch {
            guard token == recommendationsGeneration else { return }
            recommendations = nil
            recommendationsError = BalanceScreenError(error)
            isLoadingRecommendations = false
            return
        }
        isLoadingRecommendations = false
        await resolveAbilityNames(pokemonIds: result.abilityOptions.flatMap { $0.pokemon.map(\.pokemonId) })
    }

    private func clearRecommendations() {
        recommendationsGeneration += 1
        recommendations = nil
        recommendationsError = nil
        isLoadingRecommendations = false
    }

    // MARK: - 第3段: 技範囲チェッカー

    /// 直近の技検索の結果から、選択済みを除いたもの(技範囲は learnset を使わない=ポケモンを選ばない)。
    public var moveRangeOptions: [Move] {
        latestMoveSearchResults.filter { !rangeMoveIds.contains($0.id) }
    }

    /// 技は1〜4つ・重複不可。違反は false + `rangeInputError`(要求は出さない)。
    @discardableResult
    public func addRangeMove(moveId: String) -> Bool {
        guard rangeMoveIds.count < TeamLimits.maxMovesPerMember else {
            rangeInputError = .tooManyMoves
            return false
        }
        guard !rangeMoveIds.contains(moveId) else {
            rangeInputError = .duplicateMove
            return false
        }
        rangeMoveIds.append(moveId)
        rangeInputError = nil
        scheduleMoveRangeRefresh()
        return true
    }

    /// 範囲外の index は無視する。
    public func removeRangeMove(at index: Int) {
        guard rangeMoveIds.indices.contains(index) else { return }
        rangeMoveIds.remove(at: index)
        rangeInputError = nil
        scheduleMoveRangeRefresh()
    }

    public func scheduleMoveRangeRefresh() {
        guard !rangeMoveIds.isEmpty else {
            moveRangeRunner.cancel()
            clearMoveRange()
            return
        }
        isLoadingMoveRange = true
        moveRangeRunner.schedule(debounce: refreshDebounce) { [weak self] in
            await self?.refreshMoveRange()
        }
    }

    /// 技が1つ以上のときだけ呼ぶ。メンバーとは無関係。最新の呼び出しの応答だけを反映する。
    public func refreshMoveRange() async {
        guard !rangeMoveIds.isEmpty else {
            clearMoveRange()
            return
        }
        moveRangeGeneration += 1
        let token = moveRangeGeneration
        isLoadingMoveRange = true
        let result: BalanceMoveRange
        do {
            result = try await balance.moveRange(moveIds: rangeMoveIds)
            guard token == moveRangeGeneration else { return }
            moveRange = result
            moveRangeError = nil
        } catch is CancellationError {
            return
        } catch {
            guard token == moveRangeGeneration else { return }
            moveRange = nil
            moveRangeError = BalanceScreenError(error)
            isLoadingMoveRange = false
            return
        }
        isLoadingMoveRange = false
        await resolveAbilityNames(pokemonIds: result.walledByAbility.map(\.pokemonId))
    }

    private func clearMoveRange() {
        moveRangeGeneration += 1
        moveRange = nil
        moveRangeError = nil
        isLoadingMoveRange = false
    }

    // MARK: - 第3段: 特性名・共通

    /// 特性名(メンバー・仮想敵の候補、または応答に出たポケモンの `species(key:)` から引けたもの)。無ければ nil(画面は ID を出す)。
    public func abilityName(forID id: String) -> String? {
        abilityNames[id]
    }

    /// 応答の先頭 `abilityNameResolveLimit` 匹のうち、まだ引いていないポケモンの特性名をまとめて引く。
    /// 失敗は無視する(特性名が ID のまま出るだけ。画面のエラーにしない)。同じポケモンは2度引かない。
    private func resolveAbilityNames(pokemonIds: [String]) async {
        var seen = Set<String>()
        let ids = pokemonIds.filter { seen.insert($0).inserted }.prefix(BalanceDisplayLimits.abilityNameResolveLimit)
            .filter { !abilityNameResolvedPokemon.contains($0) }
        guard !ids.isEmpty else { return }
        abilityNameResolvedPokemon.formUnion(ids)
        let master = self.master
        let details = await withTaskGroup(of: SpeciesDetail?.self) { group in
            for id in ids {
                group.addTask { try? await master.species(key: id) }
            }
            var collected: [SpeciesDetail] = []
            for await detail in group {
                if let detail { collected.append(detail) }
            }
            return collected
        }
        for detail in details {
            for ability in detail.abilities { abilityNames[ability.id] = ability.nameJa }
        }
    }

    private func balanceInputs(_ list: [BalanceMember]) -> [BalanceMemberInput] {
        list.map { BalanceMemberInput(pokemonId: $0.speciesKey, abilityId: $0.abilityId, moveIds: $0.moveIds) }
    }

    /// 自分のメンバーか仮想敵か(同じ id 引きの操作で両方を扱う)。
    private enum Slot {
        case member(Int)
        case threat(Int)
    }

    private func slot(forID id: String) -> Slot? {
        if let index = members.firstIndex(where: { $0.id == id }) { return .member(index) }
        if let index = threatMembers.firstIndex(where: { $0.id == id }) { return .threat(index) }
        return nil
    }

    private func entry(at slot: Slot) -> BalanceMember {
        switch slot {
        case .member(let index): return members[index]
        case .threat(let index): return threatMembers[index]
        }
    }

    private func update(_ slot: Slot, _ body: (inout BalanceMember) -> Void) {
        switch slot {
        case .member(let index): body(&members[index])
        case .threat(let index): body(&threatMembers[index])
        }
    }

    private func scheduleRefresh(for slot: Slot) {
        switch slot {
        case .member: scheduleRefresh()
        case .threat: scheduleThreatsRefresh()
        }
    }
}
