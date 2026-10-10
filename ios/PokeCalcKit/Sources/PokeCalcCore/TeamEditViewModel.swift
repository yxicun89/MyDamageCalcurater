import Foundation
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
    /// 一度でも読んだ種族のメガ情報(`species(key:)` の応答ごとに覚える。ADR-0509 §4)。
    private var megaInfo: [String: MegaSpeciesInfo] = [:]
    /// 保存データを直したときの通知(メンバー ID → 文言。種族を変えたら消す。ADR-0509 §4)。
    private var itemNotices: [String: String] = [:]

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
    public private(set) var teamError: TeamFieldError?

    // MARK: - 6 つの枠と未保存の判定(F-08。ADR-0522)

    /// 6 つの枠。メンバー id か nil(空の枠)。`team.members` はここに並ぶ id を枠の順に詰めた並びを保つ。
    public private(set) var slotIDs: [String?]
    /// 保存済み(または開いた直後)の内容。`hasUnsavedChanges` の比較元。
    private var savedMembers: [TeamMember]
    private var savedSlotIDs: [String?]
    private var savedOnce = false
    /// 保存に成功していて、以降に編集していない(「保存しました」の表示用)。
    public var didSave: Bool { savedOnce && !hasUnsavedChanges }
    private let now: @Sendable () -> Date
    /// メンバー id → 直近の入力エラー(技の重複・上限・SP の超過)。
    public private(set) var memberErrors: [String: TeamMemberFieldError] = [:]

    /// メンバー id ごとの「最新の種族選択だけを反映する」通し番号(`CalcViewModel` の species
    /// 世代保護[M1]と同じ理由)。メンバーごとに独立させ、他のメンバーの選択をブロックしない。
    private var memberSpeciesGeneration: [String: Int] = [:]

    public init(
        store: any TeamStore, service: any PokeCalcService, team: Team,
        searchDebounce: Duration = MasterSearch.debounceInterval, now: @escaping @Sendable () -> Date = { Date() }
    ) {
        self.store = store
        self.service = service
        self.team = team
        self.now = now
        let slots = Self.initialSlotIDs(for: team.members)
        slotIDs = slots
        savedMembers = team.members
        savedSlotIDs = slots
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
                megaInfo[detail.key] = MegaSpeciesInfo(detail: detail)
                learnsetIdsByMember[member.id] = detail.learnset
                recomputeMoveOptions(forMember: member.id)
                abilityOptionsByMember[member.id] = detail.abilities
            }
            // 保存データのメガ種族の持ち物を直す(下書きの変更。自動では保存しない。ADR-0509 §4)。
            for member in team.members { correctSavedItem(forMember: member.id) }
            // 全メンバーの `species(key:)` が終わった後、保存済みの技のうちまだ見ていないものを
            // 全メンバー分まとめて `moves(ids:)` で1回だけ解決する(A3: 失敗しても `error` を立てず、
            // `moveIds` も変えない。ADR-0501「getMovesByIds による構築編集の技の一括解決」A2)。
            await resolveUnknownMoves(team.members.flatMap(\.moveIds))
            error = nil
        } catch {
            self.error = TeamScreenError(error)
        }
        isLoading = false
    }

    // MARK: - メンバーの追加・削除

    /// 最初の空の枠に追加する。空きが無ければ追加せず false を返し `teamError = .tooManyMembers`。
    /// 新しいメンバー(性格は `natureOptions.first`、他は既定値)を作り、`species(key:)` を呼んで選択肢を用意する。
    @discardableResult
    public func addMember(speciesKey: String) async -> Bool {
        guard let slot = slotIDs.firstIndex(where: { $0 == nil }) else {
            teamError = .tooManyMembers
            return false
        }
        await selectSpecies(slot: slot, speciesKey: speciesKey)
        return true
    }

    /// 枠の種族を選ぶ。空の枠なら新しいメンバーでその枠を埋め、埋まっている枠なら種族を差し替える
    /// (`setMemberSpecies` と同じ。技・特性は新しい種族に合わせて直す)。範囲外の枠は何もしない。
    public func selectSpecies(slot: Int, speciesKey: String) async {
        guard slotIDs.indices.contains(slot) else { return }
        if let id = slotIDs[slot] {
            await setMemberSpecies(id: id, speciesKey: speciesKey)
            return
        }
        let member = TeamMember(speciesKey: speciesKey, natureId: natureOptions.first?.id ?? "")
        slotIDs[slot] = member.id
        team.members.append(member)
        rebuildMembersFromSlots()
        teamError = nil
        let token = nextMemberSpeciesToken(for: member.id)
        do {
            let detail = try await service.species(key: speciesKey)
            guard token == memberSpeciesGeneration[member.id] else { return }
            speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
            megaInfo[detail.key] = MegaSpeciesInfo(detail: detail)
            learnsetIdsByMember[member.id] = detail.learnset
            recomputeMoveOptions(forMember: member.id)
            abilityOptionsByMember[member.id] = detail.abilities
            // 追加したメンバーがメガなら、持ち物はストーンに固定する(通知は出さない。ADR-0509 §4)。
            updateMember(id: member.id) {
                $0.itemId = MegaItemLock.itemIdAfterSpeciesChange(
                    previous: .none, next: MegaItemLock.make(for: megaInfo[detail.key], allItems: itemOptions), currentItemId: $0.itemId)
            }
        } catch {
            guard token == memberSpeciesGeneration[member.id] else { return }
            self.error = TeamScreenError(error)
        }
    }

    public func member(atSlot slot: Int) -> TeamMember? {
        guard slotIDs.indices.contains(slot), let id = slotIDs[slot] else { return nil }
        return team.members.first(where: { $0.id == id })
    }

    /// 枠を空に戻す(他の枠は動かさない)。空の枠・範囲外は何もしない。
    public func removeSlot(_ slot: Int) {
        guard slotIDs.indices.contains(slot), let id = slotIDs[slot] else { return }
        removeMember(id: id)
    }

    /// 隣の枠と入れ替えられるか(埋まっている枠で、1 体目の上・6 体目の下ではない)。
    public func canMoveSlot(_ slot: Int, by offset: Int) -> Bool {
        guard slotIDs.indices.contains(slot), slotIDs[slot] != nil else { return false }
        return slotIDs.indices.contains(slot + offset)
    }

    /// 隣の枠(空の枠を含む)と入れ替える。入れ替えたら true。
    @discardableResult
    public func moveSlot(_ slot: Int, by offset: Int) -> Bool {
        guard canMoveSlot(slot, by: offset) else { return false }
        slotIDs.swapAt(slot, slot + offset)
        rebuildMembersFromSlots()
        return true
    }

    /// 開いた直後の枠: 保存済みのメンバーを先頭から詰める(空き枠の位置は保存しない)。
    private static func initialSlotIDs(for members: [TeamMember]) -> [String?] {
        (0..<TeamLimits.maxMembers).map { members.indices.contains($0) ? members[$0].id : nil }
    }

    /// `team.members` の並びを枠の順に直す。枠に無いメンバーは先頭の空き枠に入れ、
    /// `team.members` に無い id は枠から消す(追加・取り込みの経路が `team.members` を直接触っても枠と食い違わない)。
    private func rebuildMembersFromSlots() {
        let known = Set(team.members.map(\.id))
        for index in slotIDs.indices {
            if let id = slotIDs[index], !known.contains(id) { slotIDs[index] = nil }
        }
        for member in team.members where !slotIDs.contains(where: { $0 == member.id }) {
            if let empty = slotIDs.firstIndex(where: { $0 == nil }) { slotIDs[empty] = member.id }
        }
        let byID = Dictionary(team.members.map { ($0.id, $0) }, uniquingKeysWith: { first, _ in first })
        team.members = slotIDs.compactMap { id in id.flatMap { byID[$0] } }
    }

    /// 保存済みの内容と下書きが違う(枠の並びも含む)。開いた直後・保存の成功後は false。
    public var hasUnsavedChanges: Bool {
        team.members != savedMembers || slotIDs != savedSlotIDs
    }

    /// 取り込んだメンバーを末尾に追加する(P6-20。ADR-0501「P6-20」)。**保存はしない**(`addMember` と同じ。
    /// 画面の「保存」で保存する)。
    /// - 追加できるのは `TeamLimits.maxMembers` までの空き枠ぶん。超えたら先頭から入れられるだけ入れ、
    ///   `teamError = .tooManyMembers`
    /// - 追加したメンバーごとに `species(key:)` を引いて `moveOptionsByMember`/`abilityOptionsByMember` を用意し、
    ///   取り込んだ技の ID(learnset に無いもの)は `resolveUnknownMoves` と同じ経路で名前を解決する
    /// - 通信に失敗しても追加したメンバーは消さない(`error` を立てる。既存のメンバー・保存済みの構築は変えない)
    /// - 戻り値は追加した体数
    @discardableResult
    public func importMembers(_ members: [TeamMember]) async -> Int {
        guard !members.isEmpty else { return 0 }
        let freeSlots = max(0, TeamLimits.maxMembers - team.members.count)
        let accepted = Array(members.prefix(freeSlots))
        teamError = members.count > freeSlots ? .tooManyMembers : nil
        guard !accepted.isEmpty else { return 0 }

        team.members.append(contentsOf: accepted)
        rebuildMembersFromSlots()
        let tokens = Dictionary(uniqueKeysWithValues: accepted.map { ($0.id, nextMemberSpeciesToken(for: $0.id)) })
        var detailsByKey: [String: SpeciesDetail] = [:]
        for member in accepted {
            if detailsByKey[member.speciesKey] == nil {
                do {
                    detailsByKey[member.speciesKey] = try await service.species(key: member.speciesKey)
                } catch {
                    self.error = TeamScreenError(error)
                    continue
                }
            }
            guard let detail = detailsByKey[member.speciesKey], tokens[member.id] == memberSpeciesGeneration[member.id] else {
                continue
            }
            speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
            megaInfo[detail.key] = MegaSpeciesInfo(detail: detail)
            learnsetIdsByMember[member.id] = detail.learnset
            recomputeMoveOptions(forMember: member.id)
            abilityOptionsByMember[member.id] = detail.abilities
            correctSavedItem(forMember: member.id)
        }
        await resolveUnknownMoves(accepted.flatMap(\.moveIds))
        return accepted.count
    }

    /// `team.members` から取り除き、対応する選択肢・エラー・世代も消す。
    public func removeMember(id: String) {
        team.members.removeAll(where: { $0.id == id })
        for index in slotIDs.indices where slotIDs[index] == id { slotIDs[index] = nil }
        moveOptionsByMember[id] = nil
        abilityOptionsByMember[id] = nil
        memberErrors[id] = nil
        memberSpeciesGeneration[id] = nil
        itemNotices[id] = nil
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
        let previousLock = itemLock(forMember: id)
        megaInfo[detail.key] = MegaSpeciesInfo(detail: detail)
        team.members[index].speciesKey = speciesKey
        // メガ種族に変えたらストーンに固定、メガ以外に変えたら未選択に戻す。直した通知は消す(ADR-0509 §4)。
        team.members[index].itemId = MegaItemLock.itemIdAfterSpeciesChange(
            previous: previousLock, next: itemLock(forMember: id), currentItemId: team.members[index].itemId)
        itemNotices[id] = nil
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

    /// 渡された `moveIds`(全メンバー分)のうち技の辞書にまだ無いものを集めて重複除去し、
    /// `moves(ids:)` を1回(64件以下なら)呼んで辞書に入れる(ADR-0501「getMovesByIds による
    /// 構築編集の技の一括解決」A2)。未知の ID が無ければ呼ばない。失敗(通信失敗・キャンセルを
    /// 含む)は無視する(A3: `error` を立てない・`moveIds` を変えない・その ID を解決しないままにする)。
    private func resolveUnknownMoves(_ moveIds: [String]) async {
        var seen = Set<String>()
        let missingIds = moveIds.filter { moveDictionary[$0] == nil && seen.insert($0).inserted }
        guard !missingIds.isEmpty else { return }
        guard let resolved = try? await service.moves(ids: missingIds) else { return }
        for move in resolved { moveDictionary[move.id] = move }
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
        guard !itemLock(forMember: id).disablesItemField else { return }
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

    /// 最終更新を刻んで `store.save(team)` を呼ぶ。成功で true(保存済みの内容を更新し `didSave`)、
    /// 失敗で `error` を立てて false(下書きは残す)。名前は変えない(旧データの名前も保つ)。
    @discardableResult
    public func save() async -> Bool {
        var toSave = team
        toSave.updatedAt = now()
        do {
            try await store.save(toSave)
            team = toSave
            savedMembers = team.members
            savedSlotIDs = slotIDs
            savedOnce = true
            error = nil
            return true
        } catch {
            self.error = TeamScreenError(error)
            return false
        }
    }
}

// MARK: - 持ち物の役割・メガ固定(ADR-0509)

extension TeamEditViewModel {
    /// メンバーの持ち物の選択肢(`.any`。役割で絞らずメガストーンだけ外す。そのメンバーのいまの持ち物は `keeping` で残す)。
    public func itemOptions(forMember id: String) -> [Item] {
        let current = team.members.first(where: { $0.id == id })?.itemId
        return ItemRoleFilter.options(itemOptions, for: .any, keeping: current)
    }

    public func itemLock(forMember id: String) -> MegaItemLock {
        guard let member = team.members.first(where: { $0.id == id }) else { return .none }
        return MegaItemLock.make(for: megaInfo[member.speciesKey], allItems: itemOptions)
    }

    /// 保存データを読み込み時に直したときの通知(`MegaItemText.correctedNotice` / `clearedNotice`)。無ければ nil。
    public func itemNotice(forMember id: String) -> String? { itemNotices[id] }

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

    /// 保存データのメガ種族の持ち物を直し、通知を残す(ADR-0509 §4)。
    fileprivate func correctSavedItem(forMember id: String) {
        guard let index = team.members.firstIndex(where: { $0.id == id }) else { return }
        switch MegaItemLock.correction(currentItemId: team.members[index].itemId, lock: itemLock(forMember: id)) {
        case .unchanged: break
        case .fixed(let itemId, let displayName):
            team.members[index].itemId = itemId
            itemNotices[id] = MegaItemText.correctedNotice(displayName)
        case .cleared:
            team.members[index].itemId = nil
            itemNotices[id] = MegaItemText.clearedNotice
        }
    }
}
