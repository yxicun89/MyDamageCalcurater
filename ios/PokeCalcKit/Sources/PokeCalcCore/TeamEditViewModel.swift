import Observation

// TeamEditViewModel: 構築編集画面の状態(P6-2c・ADR-0500 §1)。
//
// マスタ参照(species/items/natures/moves)は `CalcViewModel` と同じ `PokeCalcService` を再利用する
// (構築専用のマスタ取得は増やさない。ADR-0501「P6-2c」3章)。特性の選択肢は `PokeCalcService` に
// 特性検索 API が無いため `SpeciesDetail.abilities`(種族ごと)から出す。

/// 構築編集画面の状態。`TeamStore`(保存)と `PokeCalcService`(マスタ)の両方に依存する。
/// ADR-0501「P6-2c」3章の指定どおり `@MainActor`(`CalcViewModel`/`ReverseViewModel` と同じ)。
@MainActor
@Observable
public final class TeamEditViewModel: MasterSpeciesSearchProviding, MasterMoveSearchProviding {
    private let store: any TeamStore
    private let service: any PokeCalcService

    // MARK: - 検索(issue #68。ADR-0501「issue #68」3〜6章・10章。`CalcViewModel` と同じ規則)

    private let speciesSearch: MasterSearchField<SpeciesSummary>
    private let moveSearch: MasterSearchField<Move>
    /// 一度でも見た種族(検索結果・`species(key:)` の応答のどちらからも合流する。5章)。
    private var speciesDictionary: [String: SpeciesSummary] = [:]
    /// 直近の技検索の結果(`moveOptionsByMember` は「これとメンバーごとの learnset の ID 集合との交差」。6章)。
    private var latestMoveSearchResults: [Move] = []
    /// メンバー id → いまの種族の learnset(ID のみ。6章)。
    private var learnsetIdsByMember: [String: [String]] = [:]
    /// 一度でも見た技(先頭ページ・技検索の結果・`move(id:)` の応答が合流する。`move(forID:)` はここから
    /// 引く。ADR-0501「issue #68 の残り」5章)。
    private var moveDictionary: [String: Move] = [:]

    public private(set) var speciesQuery: String = ""
    public private(set) var moveQuery: String = ""
    public private(set) var isSearchingSpecies = false
    public private(set) var isSearchingMoves = false
    public private(set) var speciesSearchReachedLimit = false
    public private(set) var moveSearchReachedLimit = false

    // MARK: - マスタ(load() で読み込む)

    /// 直近の種族検索の結果(空クエリなら起動時の先頭ページ。`CalcViewModel.speciesOptions` と同じ
    /// 理由で名前は変えない。2章)。
    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var itemOptions: [Item] = []

    /// 持ち物の一覧(`searchItems(query: "", limit: MasterSearch.pageLimit)` の1回の取得)が上限に達した
    /// (ADR-0501「issue #68 の残り」6章)。
    public private(set) var itemOptionsReachedLimit = false
    public private(set) var natureOptions: [Nature] = []
    /// メンバー id → 「直近の技検索の結果 ∩ そのメンバーの learnset の ID 集合」を learnset の順で
    /// 並べたもの(issue #68・6章。`CalcViewModel.moveOptions` と同じ規則)。
    public private(set) var moveOptionsByMember: [String: [Move]] = [:]
    /// メンバー id → 現在の種族の特性一覧(`SpeciesDetail.abilities` をそのまま写す)。
    public private(set) var abilityOptionsByMember: [String: [Ability]] = [:]

    // MARK: - 画面の状態

    public private(set) var team: Team
    public private(set) var isLoading = false
    public private(set) var error: TeamScreenError?
    public private(set) var nameError: TeamFieldError?
    public private(set) var teamError: TeamFieldError?
    /// メンバー id → 直近の入力エラー(技の重複・上限・SP の超過)。
    public private(set) var memberErrors: [String: TeamMemberFieldError] = [:]

    /// メンバー id ごとの「最新の種族選択だけを反映する」通し番号(`CalcViewModel` の species
    /// 世代保護[M1]と同じ理由)。メンバーごとに独立させ、他のメンバーの選択をブロックしない。
    private var memberSpeciesGeneration: [String: Int] = [:]

    public init(store: any TeamStore, service: any PokeCalcService, team: Team, searchDebounce: Duration = MasterSearch.debounceInterval) {
        self.store = store
        self.service = service
        self.team = team
        speciesSearch = MasterSearchField(debounce: searchDebounce) { query, limit in
            try await service.searchSpecies(query: query, limit: limit)
        }
        moveSearch = MasterSearchField(debounce: searchDebounce) { query, limit in
            try await service.searchMoves(query: query, limit: limit)
        }
    }

    // MARK: - 起動

    /// マスタ(species/items/natures/moves)を読み、既存の各メンバーについて `species(key:)` を呼んで
    /// `moveOptionsByMember` / `abilityOptionsByMember` を作る。
    public func load() async {
        isLoading = true
        do {
            let natures = try await service.natures()
            let species = try await service.searchSpecies(query: "", limit: MasterSearch.pageLimit)
            let moves = try await service.searchMoves(query: "", limit: MasterSearch.pageLimit)
            let items = try await service.searchItems(query: "", limit: MasterSearch.pageLimit)
            natureOptions = natures
            speciesSearch.setFirstPage(species)
            speciesOptions = speciesSearch.options
            speciesSearchReachedLimit = speciesSearch.reachedLimit
            mergeSpeciesIntoDictionary(species)

            moveSearch.setFirstPage(moves)
            latestMoveSearchResults = moveSearch.options
            moveSearchReachedLimit = moveSearch.reachedLimit
            mergeMovesIntoDictionary(moves)

            itemOptions = items
            itemOptionsReachedLimit = items.count >= MasterSearch.pageLimit

            for member in team.members {
                let detail = try await service.species(key: member.speciesKey)
                speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
                learnsetIdsByMember[member.id] = detail.learnset
                recomputeMoveOptions(forMember: member.id)
                abilityOptionsByMember[member.id] = detail.abilities
                // 保存済みの技のうち、まだ見ていないものだけ `move(id:)` で解決する(A4: 失敗しても
                // `error` を立てず、`moveIds` も変えない。5章)。
                await resolveUnknownMoves(member.moveIds)
            }
            error = nil
        } catch {
            self.error = TeamScreenError(error)
        }
        isLoading = false
    }

    // MARK: - チーム名

    /// 前後空白を落として `team.name` に入れる。空になったら `nameError = .emptyName`、
    /// それ以外では nil にする(保存を止めはしない。保存時に `TeamValidator` で再検証する)。
    public func setName(_ name: String) {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        team.name = trimmedName
        nameError = trimmedName.isEmpty ? .emptyName : nil
    }

    // MARK: - メンバーの追加・削除

    /// 既に `TeamLimits.maxMembers` 体あれば追加せず false を返し `teamError = .tooManyMembers`。
    /// そうでなければ新しいメンバー(性格は `natureOptions.first`、他は既定値)を追加し、
    /// `species(key:)` を呼んで選択肢を用意する。
    @discardableResult
    public func addMember(speciesKey: String) async -> Bool {
        guard team.members.count < TeamLimits.maxMembers else {
            teamError = .tooManyMembers
            return false
        }
        let member = TeamMember(speciesKey: speciesKey, natureId: natureOptions.first?.id ?? "")
        team.members.append(member)
        teamError = nil
        let token = nextMemberSpeciesToken(for: member.id)
        do {
            let detail = try await service.species(key: speciesKey)
            guard token == memberSpeciesGeneration[member.id] else { return true }
            speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
            learnsetIdsByMember[member.id] = detail.learnset
            recomputeMoveOptions(forMember: member.id)
            abilityOptionsByMember[member.id] = detail.abilities
        } catch {
            guard token == memberSpeciesGeneration[member.id] else { return true }
            self.error = TeamScreenError(error)
        }
        return true
    }

    /// `team.members` から取り除き、対応する選択肢・エラー・世代も消す。
    public func removeMember(id: String) {
        team.members.removeAll(where: { $0.id == id })
        moveOptionsByMember[id] = nil
        abilityOptionsByMember[id] = nil
        memberErrors[id] = nil
        memberSpeciesGeneration[id] = nil
        learnsetIdsByMember[id] = nil
    }

    // MARK: - 種族

    /// そのメンバーの `speciesKey` を差し替え、`species(key:)` を呼び直して選択肢を更新する。
    /// 新しい learnset に無い `moveIds` は黙って落とす。応答はメンバーごとの世代で守り、
    /// 連続で種族を変えたときに古い応答が後から上書きしないようにする(`CalcViewModel` と同じ理由)。
    public func setMemberSpecies(id: String, speciesKey: String) async {
        guard team.members.contains(where: { $0.id == id }) else { return }
        let token = nextMemberSpeciesToken(for: id)
        do {
            let detail = try await service.species(key: speciesKey)
            guard token == memberSpeciesGeneration[id] else { return }
            applySpeciesChange(detail, speciesKey: speciesKey, toMemberID: id)
        } catch {
            guard token == memberSpeciesGeneration[id] else { return }
            self.error = TeamScreenError(error)
        }
    }

    /// 種族変更で `abilityId` が旧種族の特性のまま残らないようにする(issue #100)。現在の `abilityId` が
    /// 新種族の `detail.abilities` にまだあれば保持し、無ければ新種族の先頭特性へ差し替える(候補が空なら
    /// `nil`)。表示(`abilityOptionsByMember`)・保存値(`team.members[index].abilityId`)・計算入力
    /// (`TeamMemberConverter`)を常に一致させるため(既定案どおり)。
    ///
    /// `moveIds` の絞り込みは**learnset の ID 集合**で行う(解決済みの `moveOptionsByMember` ではない。
    /// issue #68・6章「判断」: 先頭ページの外にある合法な技を、実体化できないだけで黙って消さないため)。
    private func applySpeciesChange(_ detail: SpeciesDetail, speciesKey: String, toMemberID id: String) {
        guard let index = team.members.firstIndex(where: { $0.id == id }) else { return }
        let learnsetIds = Set(detail.learnset)
        team.members[index].speciesKey = speciesKey
        team.members[index].moveIds = team.members[index].moveIds.filter { moveId in
            learnsetIds.contains(moveId)
        }
        let currentAbilityId = team.members[index].abilityId
        let abilityStillValid = currentAbilityId.map { abilityId in
            detail.abilities.contains(where: { $0.id == abilityId })
        } ?? false
        if !abilityStillValid {
            team.members[index].abilityId = detail.abilities.first?.id
        }
        speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
        learnsetIdsByMember[id] = detail.learnset
        recomputeMoveOptions(forMember: id)
        abilityOptionsByMember[id] = detail.abilities
    }

    /// `learnsetIdsByMember[id]` と `latestMoveSearchResults` のどちらかが変わったら呼び直す(issue #68・6章)。
    private func recomputeMoveOptions(forMember id: String) {
        let learnsetIds = learnsetIdsByMember[id] ?? []
        moveOptionsByMember[id] = learnsetIds.compactMap { learnedId in
            latestMoveSearchResults.first(where: { $0.id == learnedId })
        }
    }

    /// `moveIds` のうち技の辞書にまだ無いものだけ `move(id:)` で解決し、辞書に入れる(ADR-0501
    /// 「issue #68 の残り」5章)。並行に投げてよい(1体最大 `TeamLimits.maxMovesPerMember` 件)。
    /// 失敗(404・通信失敗)は無視する(A4: `error` を立てない・`moveIds` を変えない・その ID を
    /// 解決しないままにする)。
    private func resolveUnknownMoves(_ moveIds: [String]) async {
        let missingIds = moveIds.filter { moveDictionary[$0] == nil }
        guard !missingIds.isEmpty else { return }
        let service = self.service
        let resolved = await withTaskGroup(of: (String, Move?).self) { group in
            for id in missingIds {
                group.addTask {
                    let move = try? await service.move(id: id)
                    return (id, move)
                }
            }
            var results: [(String, Move?)] = []
            for await entry in group { results.append(entry) }
            return results
        }
        for (id, move) in resolved {
            if let move { moveDictionary[id] = move }
        }
    }

    private func nextMemberSpeciesToken(for id: String) -> Int {
        let token = (memberSpeciesGeneration[id] ?? 0) + 1
        memberSpeciesGeneration[id] = token
        return token
    }

    // MARK: - 技

    /// 既に `TeamLimits.maxMovesPerMember` 個あれば false + `.tooManyMoves`。既に同じ `moveId` を
    /// 持っていれば false + `.duplicateMove`。それ以外は追加して true(`memberErrors[id]` を nil にする)。
    @discardableResult
    public func addMove(id: String, moveId: String) -> Bool {
        guard let index = team.members.firstIndex(where: { $0.id == id }) else { return false }
        guard team.members[index].moveIds.count < TeamLimits.maxMovesPerMember else {
            memberErrors[id] = .tooManyMoves
            return false
        }
        guard !team.members[index].moveIds.contains(moveId) else {
            memberErrors[id] = .duplicateMove
            return false
        }
        team.members[index].moveIds.append(moveId)
        memberErrors[id] = nil
        return true
    }

    /// 範囲内の index を取り除く(範囲外は無視)。
    public func removeMove(id: String, at index: Int) {
        guard let memberIndex = team.members.firstIndex(where: { $0.id == id }) else { return }
        guard team.members[memberIndex].moveIds.indices.contains(index) else { return }
        team.members[memberIndex].moveIds.remove(at: index)
    }

    // MARK: - 持ち物・特性・性格・テラスタイプ(そのまま代入。ID の存在チェックはしない)

    public func setMemberItem(id: String, itemId: String?) {
        updateMember(id: id) { $0.itemId = itemId }
    }

    public func setMemberAbility(id: String, abilityId: String?) {
        updateMember(id: id) { $0.abilityId = abilityId }
    }

    public func setMemberNature(id: String, natureId: String) {
        updateMember(id: id) { $0.natureId = natureId }
    }

    public func setMemberTeraType(id: String, teraType: PokeType?) {
        updateMember(id: id) { $0.teraType = teraType }
    }

    /// 前後空白を落とし、空になったら nil にする(`TeamListViewModel.createTeam` のチーム名と同じ
    /// 規則)。ニックネームは任意項目なので空を「未設定」として扱う。
    public func setMemberNickname(id: String, nickname: String) {
        let trimmed = nickname.trimmingCharacters(in: .whitespacesAndNewlines)
        updateMember(id: id) { $0.nickname = trimmed.isEmpty ? nil : trimmed }
    }

    private func updateMember(id: String, _ mutate: (inout TeamMember) -> Void) {
        guard let index = team.members.firstIndex(where: { $0.id == id }) else { return }
        mutate(&team.members[index])
    }

    // MARK: - SP

    /// `0...SPLimits.maxPerStat` の外、または変更後の合計が `SPLimits.maxTotal` を超えるなら
    /// 変更せず false(`memberErrors[id]` に理由を立てる)。それ以外は代入して true。
    /// 無効な入力をいったん状態に入れてから検証するのではなく、変更そのものを拒否する
    /// (SP はステッパー/スライダー操作が主で、範囲外の値を一瞬でも保持する UI 上の理由が無いため。
    /// ADR-0501「P6-2c」3章の判断)。
    @discardableResult
    public func setMemberSP(id: String, stat: StatKey, value: Int) -> Bool {
        guard let index = team.members.firstIndex(where: { $0.id == id }) else { return false }
        guard (0...SPLimits.maxPerStat).contains(value) else {
            memberErrors[id] = .spPerStatExceeded
            return false
        }
        var sp = team.members[index].sp
        let newTotal = sp.total - sp.value(for: stat) + value
        guard newTotal <= SPLimits.maxTotal else {
            memberErrors[id] = .spTotalExceeded
            return false
        }
        sp.setValue(value, for: stat)
        team.members[index].sp = sp
        memberErrors[id] = nil
        return true
    }

    // MARK: - 検索(issue #68。ADR-0501「issue #68」10章。`CalcViewModel` と同じ規則)

    public func speciesSummary(forKey key: String) -> SpeciesSummary? {
        speciesDictionary[key]
    }

    /// 一度でも見た技(先頭ページ・検索結果・`move(id:)` の応答)から引く。技スロットの表示名に使う
    /// (ADR-0501「issue #68 の残り」5章)。無ければ nil(View は ID をそのまま出す)。
    public func move(forID id: String) -> Move? {
        moveDictionary[id]
    }

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
        guard let results else { return }
        mergeSpeciesIntoDictionary(results)
    }

    /// メンバーごとの候補(`moveOptionsByMember`)はどれも同じ検索結果から作るので、全メンバー分
    /// 作り直す(issue #68・6章)。検索結果は技の辞書にも合流させる(ADR-0501「issue #68 の残り」5章:
    /// 検索語を変えても、既に見た技スロットの名前が消えないようにするため)。
    public func runMoveSearch() async {
        let results = await moveSearch.run()
        latestMoveSearchResults = moveSearch.options
        isSearchingMoves = moveSearch.isSearching
        moveSearchReachedLimit = moveSearch.reachedLimit
        if let results { mergeMovesIntoDictionary(results) }
        for memberID in learnsetIdsByMember.keys {
            recomputeMoveOptions(forMember: memberID)
        }
    }

    private func mergeSpeciesIntoDictionary(_ items: [SpeciesSummary]) {
        for item in items { speciesDictionary[item.key] = item }
    }

    private func mergeMovesIntoDictionary(_ items: [Move]) {
        for item in items { moveDictionary[item.id] = item }
    }

    // MARK: - 保存

    /// `store.save(team)` を呼ぶ。成功で true、失敗で `error` を立てて false。
    @discardableResult
    public func save() async -> Bool {
        do {
            try await store.save(team)
            error = nil
            return true
        } catch {
            self.error = TeamScreenError(error)
            return false
        }
    }
}
