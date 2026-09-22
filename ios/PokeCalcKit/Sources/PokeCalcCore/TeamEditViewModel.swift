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
public final class TeamEditViewModel {
    /// openapi の `limit` の上限(`CalcViewModel` と同じ値。coding-rules §2「同じ値を複数箇所に
    /// 書かない」の例外にはならない別モジュール内定数だが、意味は揃えている)。
    private static let masterListLimit = 200

    private let store: any TeamStore
    private let service: any PokeCalcService

    // MARK: - マスタ(load() で読み込む)

    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var itemOptions: [Item] = []
    public private(set) var natureOptions: [Nature] = []
    /// メンバー id → 現在の種族の learnset の順・マスタにある技だけ(`CalcViewModel.moveOptions` と同じ規則)。
    public private(set) var moveOptionsByMember: [String: [Move]] = [:]
    /// メンバー id → 現在の種族の特性一覧(`SpeciesDetail.abilities` をそのまま写す)。
    public private(set) var abilityOptionsByMember: [String: [Ability]] = [:]
    /// `moveOptionsByMember` を作るための技マスタ全件(learnset の ID と突き合わせる)。
    private var masterMoves: [Move] = []

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

    public init(store: any TeamStore, service: any PokeCalcService, team: Team) {
        self.store = store
        self.service = service
        self.team = team
    }

    // MARK: - 起動

    /// マスタ(species/items/natures/moves)を読み、既存の各メンバーについて `species(key:)` を呼んで
    /// `moveOptionsByMember` / `abilityOptionsByMember` を作る。
    public func load() async {
        isLoading = true
        do {
            let natures = try await service.natures()
            let species = try await service.searchSpecies(query: "", limit: Self.masterListLimit)
            let moves = try await service.searchMoves(query: "", limit: Self.masterListLimit)
            let items = try await service.searchItems(query: "", limit: Self.masterListLimit)
            natureOptions = natures
            speciesOptions = species
            masterMoves = moves
            itemOptions = items

            for member in team.members {
                let detail = try await service.species(key: member.speciesKey)
                moveOptionsByMember[member.id] = moveOptions(from: detail)
                abilityOptionsByMember[member.id] = detail.abilities
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
            moveOptionsByMember[member.id] = moveOptions(from: detail)
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

    private func applySpeciesChange(_ detail: SpeciesDetail, speciesKey: String, toMemberID id: String) {
        guard let index = team.members.firstIndex(where: { $0.id == id }) else { return }
        let options = moveOptions(from: detail)
        team.members[index].speciesKey = speciesKey
        team.members[index].moveIds = team.members[index].moveIds.filter { moveId in
            options.contains(where: { $0.id == moveId })
        }
        moveOptionsByMember[id] = options
        abilityOptionsByMember[id] = detail.abilities
    }

    private func moveOptions(from detail: SpeciesDetail) -> [Move] {
        detail.learnset.compactMap { learnedId in masterMoves.first(where: { $0.id == learnedId }) }
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
