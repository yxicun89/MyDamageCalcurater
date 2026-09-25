import Observation

// CalcViewModel: ダメージ計算画面の状態(P6-2a・ADR-0500 §1「画面のロジックは ViewModel で
// XCTest に固定し、View は描くだけ」)。
//
// `@Observable` は Apple の Observation フレームワークのマクロ。このモジュールに同名の型
// (旧 `Observation` enum)があると `Observation.ObservationRegistrar` の解決が衝突するため、
// ドメイン側は `DamageObservation` に改名してある(DomainTypes.swift のコメント参照)。

/// ダメージ計算画面の状態。`PokeCalcService` だけに依存し、View は生成型やサービスを直接知らない
/// (ADR-0500 §3)。
@MainActor
@Observable
public final class CalcViewModel: MasterSpeciesSearchProviding, MasterMoveSearchProviding {
    /// 攻撃側・防御側に選べるためには種族が最低2つ要る。
    private static let minimumSpeciesCount = 2

    private let service: any PokeCalcService
    /// 構築の永続化(P6-2d)。省略可(既定 nil。ADR-0501「P6-2d」5章「判断」): 既存のテストを
    /// 変えずに通すため、また nil のときは `UserDefaults` に触れずに済むため。
    private let teamStore: (any TeamStore)?

    // MARK: - 入力 Task の管理(issue #113。ADR-0501「issue #113 の受け入れ条件(iOS 側)」3章・8章)

    /// 画面からの入力操作の Task を1つだけ保持する(計算画面はすべて確定操作なので debounce は
    /// 持たせない。3章「判断」)。
    private let inputTaskRunner = LatestTaskRunner()

    // MARK: - 検索(issue #68。ADR-0501「issue #68」3〜6章・10章)

    /// 種族・技の検索欄1つ分の状態機械(デバウンス・世代保護は `MasterSearchField` に任せる)。
    private let speciesSearch: MasterSearchField<SpeciesSummary>
    private let moveSearch: MasterSearchField<Move>
    /// 一度でも見た種族(検索結果・`species(key:)` の応答のどちらからも合流する。5章)。
    /// 検索語を変えても、選択中の種族の名前をここから引ける。
    private var speciesDictionary: [String: SpeciesSummary] = [:]
    /// 一度でも見た技(検索結果から合流する。5章)。`selectedMove` はここから引く。
    private var moveDictionary: [String: Move] = [:]
    /// 直近の技検索の結果(`moveOptions` は「これと攻撃側の learnset の ID 集合との交差」。6章)。
    private var latestMoveSearchResults: [Move] = []
    /// いまの攻撃側の learnset(ID のみ。`moveOptions` を作るのに使う。6章)。
    private var attackerLearnsetIds: [String] = []

    // MARK: - マスタ(load() で読み込む)

    public private(set) var speciesQuery: String = ""
    public private(set) var moveQuery: String = ""
    public private(set) var isSearchingSpecies = false
    public private(set) var isSearchingMoves = false
    public private(set) var speciesSearchReachedLimit = false
    public private(set) var moveSearchReachedLimit = false

    /// 直近の種族検索の結果(空クエリなら起動時の先頭ページ。意味は「マスタ全件」ではなく
    /// 「検索結果」に変わったが、名前は既存のテストを壊さないため変えない。2章)。
    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var itemOptions: [Item] = []

    /// 持ち物の一覧(`searchItems(query: "", limit: MasterSearch.pageLimit)` の1回の取得)が上限に達した
    /// (ADR-0501「issue #68 の残り」6章)。
    public private(set) var itemOptionsReachedLimit = false
    /// 「直近の技検索の結果 ∩ 攻撃側の learnset の ID 集合」を learnset の順で並べたもの(6章)。
    public private(set) var moveOptions: [Move] = []
    private var natureOptions: [Nature] = []
    /// `load()` の二重実行を防ぐ(同じ画面から複数回 `load()` を呼んでも読み込みは1回だけ)。
    private var didLoad = false

    // MARK: - 画面の入力

    public private(set) var attackerSpeciesKey: String = ""
    public private(set) var defenderSpeciesKey: String = ""
    public private(set) var moveId: String = ""
    /// 自分側(攻撃側)の SP・性格などの出どころ(P6-2d。ADR-0501「P6-2d」1章)。既定は
    /// `AttackerPreset.defaultPreset`(無振り。`AttackerPresetTests` が順序を固定する)。
    public private(set) var attackerBuildSource: AttackerBuildSource = .preset(AttackerPreset.defaultPreset)
    /// `attackerBuildSource.preset` のショートカット。構築の個体を呼んでいる間は nil になる
    /// (ADR-0501「P6-2d」1章「判断」: 既存のテスト・View のピル選択表示を無変更で保つため、
    /// 計算プロパティとして残す。Swift の optional 昇格で `== .aFull` の比較がそのまま成り立つ)。
    public var attackerPreset: AttackerPreset? { attackerBuildSource.preset }
    public private(set) var attackerItemId: String?
    /// 持ち物マスタの順(トグルした順ではない)。
    public private(set) var comparedDefenderItemIds: [String] = []
    /// `comparedDefenderItemIds` を作るための、トグルされた持ち物 ID の集合(順序は持たない)。
    private var toggledDefenderItemIds: Set<String> = []

    /// 比較する持ち物の選択が上限(`RequestLimits.maxSelectableItemVariants`)に達しているか
    /// (issue #110 A8)。
    public var comparedDefenderItemsReachedLimit: Bool {
        toggledDefenderItemIds.count >= RequestLimits.maxSelectableItemVariants
    }

    // MARK: - 計算条件(issue #274。ADR-0501「issue #274」)

    /// 急所(既定 false)。
    public private(set) var isCritical = false
    /// 攻撃側のやけど(既定 false。true のとき `attacker.status = .burn`)。
    public private(set) var isAttackerBurned = false
    /// 天候(既定 `.none`)。
    public private(set) var weather: Weather = .none
    /// フィールド(既定 `.none`)。
    public private(set) var terrain: Terrain = .none
    /// 防御側の壁(既定はすべて off)。
    public private(set) var defenderScreens = Screens()
    /// 攻撃側のランク。画面で変えられるのは atk と spa だけ(技の分類の関連ステータス。他は常に 0)。
    public private(set) var attackerRanks = RankBlock()
    /// 攻撃側の特性の選択肢(いまの攻撃側の `species(key:)` の `abilities` の順)。
    public private(set) var attackerAbilityOptions: [Ability] = []
    /// 要求に載せる攻撃側の特性(nil は送らない)。
    public private(set) var attackerAbilityId: String?

    /// ランクのステッパーが編集する能力(選択中の技の分類の関連ステータス。技が無いときは atk)。
    public var attackerRankStat: StatKey {
        AttackerPreset.relevantStat(for: selectedMove?.category ?? .physical)
    }

    /// `attackerRankStat` のいまのランク。
    public var attackerRank: Int {
        switch attackerRankStat {
        case .atk: return attackerRanks.atk
        case .spa: return attackerRanks.spa
        // `attackerRankStat`(= `AttackerPreset.relevantStat(for:)`)は atk か spa しか返さないので
        // ここには来ない。`StatKey` の他ケース(hp/def/spd/spe)を網羅するためだけの分岐。
        default: return attackerRanks.atk
        }
    }

    /// ステッパーの表示(「A +1」など。`RankLabel.text`)。
    public var attackerRankText: String {
        RankLabel.text(stat: attackerRankStat, value: attackerRank)
    }

    public func setCritical(_ isOn: Bool) async {
        guard isOn != isCritical else { return }
        let token = beginInput()
        isCritical = isOn
        await recalculate(token: token)
    }

    public func setAttackerBurned(_ isOn: Bool) async {
        guard isOn != isAttackerBurned else { return }
        let token = beginInput()
        isAttackerBurned = isOn
        await recalculate(token: token)
    }

    public func selectWeather(_ weather: Weather) async {
        guard weather != self.weather else { return }
        let token = beginInput()
        self.weather = weather
        await recalculate(token: token)
    }

    public func selectTerrain(_ terrain: Terrain) async {
        guard terrain != self.terrain else { return }
        let token = beginInput()
        self.terrain = terrain
        await recalculate(token: token)
    }

    public func setDefenderScreen(_ kind: ScreenKind, isOn: Bool) async {
        guard defenderScreens.isOn(kind) != isOn else { return }
        let token = beginInput()
        defenderScreens = defenderScreens.setting(kind, to: isOn)
        await recalculate(token: token)
    }

    /// `attackerRankStat` のランクを `value`(-6..+6 に丸める)にする。
    public func setAttackerRank(_ value: Int) async {
        let clamped = min(RankLimits.max, max(RankLimits.min, value))
        guard clamped != attackerRank else { return }
        let token = beginInput()
        switch attackerRankStat {
        case .atk: attackerRanks.atk = clamped
        case .spa: attackerRanks.spa = clamped
        // `attackerRank` のコメントと同じ理由(atk/spa 以外には来ない)。
        default: attackerRanks.atk = clamped
        }
        await recalculate(token: token)
    }

    /// nil(指定なし)か `attackerAbilityOptions` にある ID だけを受け付ける。
    public func selectAttackerAbility(id: String?) async {
        guard id != attackerAbilityId else { return }
        if let id, !attackerAbilityOptions.contains(where: { $0.id == id }) { return }
        let token = beginInput()
        attackerAbilityId = id
        await recalculate(token: token)
    }

    // MARK: - 構築から個体を呼び出す(P6-2d)

    /// 構築の一覧から作った選択肢(メンバーが0体の構築は含まない)。`teamStore` が nil、
    /// または読み込みに失敗したときは空のまま(ADR-0501「P6-2d」5章)。
    public private(set) var teamOptions: [TeamPickerGroup] = []
    /// `teamOptions` の元になった構築本体(`selectTeamIndividual` が個体を引くのに使う。
    /// `TeamMemberOption` は表示用の射影で `TeamMember` の全フィールドを持たないため別に持つ)。
    private var loadedTeams: [Team] = []
    /// `loadTeams()` 専用の世代の通し番号(`beginInput()` とは別。6章「判断」: 構築の読み込みは
    /// 要求の内容に影響しないので、進行中の計算を追い越したことにしない)。
    private var latestTeamListToken = 0

    // MARK: - 計算結果

    public private(set) var rows: [BulkRowDisplay] = []
    public private(set) var isLoading = false
    public private(set) var error: CalcScreenError?

    /// 「最新の要求だけを反映する」ための通し番号(規則7)。`beginInput()` が入力操作のたびに進め、
    /// その操作から生まれる `species(key:)` / `calcBulk` の応答・失敗はすべて同じ番号で判定する
    /// (species の応答も含めて世代を守る。M1)。値そのものに意味は無い。
    private var latestRequestToken = 0

    public init(service: any PokeCalcService, teamStore: (any TeamStore)? = nil, searchDebounce: Duration = MasterSearch.debounceInterval) {
        self.service = service
        self.teamStore = teamStore
        speciesSearch = MasterSearchField(debounce: searchDebounce) { query, limit in
            try await service.searchSpecies(query: query, limit: limit)
        }
        moveSearch = MasterSearchField(debounce: searchDebounce) { query, limit in
            try await service.searchMoves(query: query, limit: limit)
        }
    }

    // MARK: - 起動

    /// マスタを読み、既定の入力(攻撃側=先頭・防御側=2番目・技=先頭のダメージ技・A特化・持ち物なし・比較なし)
    /// を選んで一括計算を1回呼ぶ(規則3)。2回目以降の呼び出しは何もしない(読み込み済みのため)。
    public func load() async {
        guard !didLoad else { return }
        didLoad = true
        let token = beginInput()
        isLoading = true
        do {
            let natures = try await service.natures()
            let species = try await service.searchSpecies(query: "", limit: MasterSearch.pageLimit)
            let moves = try await service.searchMoves(query: "", limit: MasterSearch.pageLimit)
            let items = try await service.searchItems(query: "", limit: MasterSearch.pageLimit)
            guard token == latestRequestToken else { return }

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

            guard species.count >= Self.minimumSpeciesCount else {
                throw PokeCalcError(
                    code: PokeCalcError.Code.insufficientSpecies,
                    message: "計算に必要な種族が足りません(\(species.count) 件)"
                )
            }
            attackerSpeciesKey = species[0].key
            defenderSpeciesKey = species[1].key
            attackerBuildSource = .preset(AttackerPreset.defaultPreset)
            attackerItemId = nil
            toggledDefenderItemIds = []
            comparedDefenderItemIds = []

            try await reloadAttackerMoveOptions(token: token)
            guard token == latestRequestToken else { return }
            try await reselectMove(preferringCurrent: nil, token: token)
            guard token == latestRequestToken else { return }
        } catch {
            guard token == latestRequestToken else { return }
            handleInputFailure(error)
            return
        }
        await recalculate(token: token)
        // 構築の読み込みは計算の後(6章「起動時の構築の読み込みで起動時の計算を遅らせない」)。
        await loadTeams()
    }

    // MARK: - 構築から個体を呼び出す(P6-2d。ADR-0501「P6-2d」)

    /// 保存済みの構築を読み直し、`teamOptions` を作り直す。`teamStore` が無ければ常に空にする。
    /// 何度でも呼べる(構築ビルダーで編集して戻ってきたときに View から呼び直すため。6章)。
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

    /// 構築の個体を呼び出す(1〜4章)。`teamOptions` に無い teamID/memberID は無視する(計算もしない)。
    public func selectTeamIndividual(teamID: String, memberID: String) async {
        // 構築に保存された技の分類判定には、一度でも見た技の辞書を使う(issue #68: 先頭ページの
        // 外にある技しか持たないメンバーを呼び出しても分類判定を諦めないため。5章)。
        guard let selection = TeamIndividualSelectionBuilder.make(
            teamID: teamID, memberID: memberID, teamOptions: teamOptions, teams: loadedTeams,
            moves: Array(moveDictionary.values)
        ) else { return }

        let token = beginInput()
        isLoading = true
        attackerSpeciesKey = selection.individual.speciesKey
        attackerItemId = selection.individual.itemId
        do {
            try await reloadAttackerMoveOptions(token: token)
            guard token == latestRequestToken else { return }
            // 個体の技を優先する(無ければ・いまの learnset に無ければ規則3・4の既定に落ちる)。
            try await reselectMove(preferringCurrent: selection.individual.moveId, token: token)
            guard token == latestRequestToken else { return }
        } catch {
            guard token == latestRequestToken else { return }
            handleInputFailure(error)
            return
        }
        guard token == latestRequestToken else { return }
        attackerBuildSource = .team(selection)
        // 保存された特性を選択状態にする(`reloadAttackerMoveOptions` が「新種族に無ければ nil」に
        // 戻した後を上書きする。issue #274。ADR-0501「issue #274」2章・8章)。
        attackerAbilityId = selection.individual.abilityId
        await recalculate(token: token)
    }

    // MARK: - 入力の変更(規則4〜6。どれも calcBulk をちょうど1回呼ぶ)

    /// 選べるかどうかは「いま見えている検索結果」ではなく「一度でも見た種族」の辞書で判定する
    /// (issue #68。ADR-0501「issue #68」5章: 検索で見つけた種族を選べるようにするため)。
    public func selectAttacker(speciesKey: String) async {
        guard speciesDictionary[speciesKey] != nil else { return }
        let token = beginInput()
        attackerSpeciesKey = speciesKey
        await applyAttackerChangeAndRecalculate(token: token)
    }

    public func selectDefender(speciesKey: String) async {
        guard speciesDictionary[speciesKey] != nil else { return }
        let token = beginInput()
        defenderSpeciesKey = speciesKey
        await recalculate(token: token)
    }

    /// `moveOptions` に無い技は無視する(計算しない。規則4)。
    public func selectMove(id: String) async {
        guard moveOptions.contains(where: { $0.id == id }) else { return }
        let token = beginInput()
        moveId = id
        await recalculate(token: token)
    }

    /// プリセットを押すと構築の選択は外れる(排他。ADR-0501「P6-2d」1章「判断」)。
    public func selectAttackerPreset(_ preset: AttackerPreset) async {
        let token = beginInput()
        // 構築から来た特性は外す(直前がプリセット同士の切り替えなら、利用者が選んだ特性を残す。
        // issue #274。ADR-0501「issue #274」2章・8章)。
        if attackerBuildSource.teamSelection != nil {
            attackerAbilityId = nil
        }
        attackerBuildSource = .preset(preset)
        await recalculate(token: token)
    }

    public func selectAttackerItem(id: String?) async {
        let token = beginInput()
        attackerItemId = id
        await recalculate(token: token)
    }

    /// 比較する持ち物のトグル。`comparedDefenderItemIds` はトグルした順ではなく持ち物マスタの順(規則5)。
    /// 上限到達中の ON 操作は拒否する(OFF は常に通す。issue #110 A8)。
    public func toggleDefenderItemComparison(itemId: String) async {
        if toggledDefenderItemIds.contains(itemId) {
            toggledDefenderItemIds.remove(itemId)
        } else {
            guard !comparedDefenderItemsReachedLimit else { return }
            toggledDefenderItemIds.insert(itemId)
        }
        let token = beginInput()
        comparedDefenderItemIds = itemOptions.map(\.id).filter { toggledDefenderItemIds.contains($0) }
        await recalculate(token: token)
    }

    /// 攻守入れ替え。種族だけを入れ替え、技は規則4で選び直す。プリセット・攻撃側の持ち物・
    /// 比較トグルは画面の設定として残す(規則6)。
    public func swapSides() async {
        let token = beginInput()
        swapTick += 1
        swap(&attackerSpeciesKey, &defenderSpeciesKey)
        await applyAttackerChangeAndRecalculate(token: token)
    }

    /// 攻守入れ替えが起きた回数(値そのものに意味は無い)。View はこれの変化だけを見て
    /// カード入れ替えのアニメーションを掛ける(design.md「攻守入れ替え」だけに動きを絞り、
    /// 通常の種族セレクタでの変更や起動時の読み込みでは動かさない)。`swapSides()` の中で
    /// `attackerSpeciesKey`/`defenderSpeciesKey` と同じ同期区間で増やすので、View 側の
    /// `.animation(value:)` はこの2つの変更を同じひとまとまりとして扱える。
    public private(set) var swapTick = 0

    // MARK: - 検索(issue #68。ADR-0501「issue #68」10章)

    /// 一度でも見た種族から引く(検索結果を変えても、選択中の種族の名前が消えないようにするため。5章)。
    public func speciesSummary(forKey key: String) -> SpeciesSummary? {
        speciesDictionary[key]
    }

    public var attackerSpecies: SpeciesSummary? { speciesDictionary[attackerSpeciesKey] }
    public var defenderSpecies: SpeciesSummary? { speciesDictionary[defenderSpeciesKey] }

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

    public func runMoveSearch() async {
        let results = await moveSearch.run()
        latestMoveSearchResults = moveSearch.options
        isSearchingMoves = moveSearch.isSearching
        moveSearchReachedLimit = moveSearch.reachedLimit
        if let results { mergeMovesIntoDictionary(results) }
        recomputeMoveOptions()
    }

    // MARK: - 表示用の派生値(M4: 技の要約・相性)

    /// 選択中の技。「見えている候補」(`moveOptions`)ではなく一度でも見た技の辞書から引く
    /// (issue #68。ADR-0501「issue #68」5章: 技の検索語を learnset と重ならない語に変えても、
    /// 選択中の技の名前が消えないようにするため)。
    public var selectedMove: Move? {
        moveDictionary[moveId]
    }

    /// 表示中の全行が同じタイプ相性ならその値、行が無い・値が割れているときは nil
    /// (防御側の調整によって相性が変わることは無いはずだが、行が空のときや混在時は言い切らない)。
    public var moveEffectiveness: Double? {
        guard let first = rows.first?.effectiveness else { return nil }
        return rows.allSatisfy { $0.effectiveness == first } ? first : nil
    }

    /// 「威力37 / 物理 / ばつぐん(×2)」のような技セレクタの要約文言(design.md「技セレクタ」)。
    public var moveSummaryText: String {
        guard let move = selectedMove else { return "" }
        var parts = ["威力\(move.power)", MoveCategoryLabel.japaneseName(for: move.category)]
        if let effectiveness = moveEffectiveness {
            parts.append(EffectivenessLabel.text(effectiveness))
        }
        return parts.joined(separator: " / ")
    }

    // MARK: - 入力 Task の管理(issue #113。ADR-0501「issue #113 の受け入れ条件(iOS 側)」8章)

    /// 確定操作(select・toggle・入れ替え・構築からの呼び出し)用。先行 Task を cancel し、
    /// ただちに `operation` を呼ぶ(A1・A4)。View は
    /// `viewModel.scheduleLatest { await $0.selectAttackerItem(id: id) }` の形で呼ぶ(`[weak viewModel]`
    /// を書かせないため、`self` を引数で渡す。8章「判断」)。
    @discardableResult
    public func scheduleLatest(_ operation: @escaping @MainActor @Sendable (CalcViewModel) async -> Void) -> Task<Void, Never> {
        inputTaskRunner.schedule(debounce: .zero) { [weak self] in
            guard let self else { return }
            await operation(self)
        }
    }

    /// 保持中の入力 Task を cancel する(A6。View は `.onDisappear` で呼ぶ)。
    public func cancelPendingWork() {
        inputTaskRunner.cancel()
    }

    /// 入力操作から生まれる**すべての** `catch`(`species(key:)`・`calcBulk` のどちらが投げた
    /// エラーでも)が使う共通処理(issue #113 A5。ADR-0501「issue #113」5章「ViewModel の各 catch は、
    /// キャンセルとそれ以外を分ける」)。`CancellationError` は画面のエラーにしない(`rows` も消さず、
    /// 表示中の最後の結果を残す。`isLoading` だけ解く)。それ以外は `error` を立てて `rows` を空にする。
    /// 呼び出し元は `guard token == latestRequestToken else { return }` の後にこれを呼ぶこと。
    private func handleInputFailure(_ error: Error) {
        guard !(error is CancellationError) else {
            isLoading = false
            return
        }
        self.error = CalcScreenError(error)
        rows = []
        isLoading = false
    }

    // MARK: - 内部

    /// 入力操作の入口。世代を1つ進めて返す。以降その操作から生まれる `await` はすべてこの番号で
    /// 「まだ最新か」を確かめ、途中で失敗しても・古い応答が後から届いても、最新の操作だけが
    /// 画面の状態(`rows` / `error` / `isLoading` / `moveOptions`)を書き換えるようにする(M1・M2)。
    private func beginInput() -> Int {
        latestRequestToken += 1
        return latestRequestToken
    }

    /// 攻撃側の種族が変わった(選択・入れ替え)ときの共通処理: learnset を読み直し、技を選び直し、計算する。
    private func applyAttackerChangeAndRecalculate(token: Int) async {
        // `species(key:)` の応答待ちの間も読み込み中にする(`performCalc` が `calcBulk` の間だけ
        // 立てていたが、その手前の learnset 読み直しも同じ1回の操作の一部なので合わせる)。
        isLoading = true
        do {
            let previousMoveId = moveId
            try await reloadAttackerMoveOptions(token: token)
            guard token == latestRequestToken else { return }
            try await reselectMove(preferringCurrent: previousMoveId, token: token)
            guard token == latestRequestToken else { return }
        } catch {
            guard token == latestRequestToken else { return }
            handleInputFailure(error)
            return
        }
        await recalculate(token: token)
    }

    /// いまの `attackerSpeciesKey` の詳細を読み、`moveOptions`(「直近の技検索の結果 ∩ learnset の
    /// ID 集合」を learnset の順で並べたもの。issue #68・6章)を作り直す。応答が届いた時点で `token`
    /// が最新でない、または詳細の種族がいまの攻撃側と違う(＝この操作は追い越された)ときは、
    /// 黙って反映しない(M1)。
    private func reloadAttackerMoveOptions(token: Int) async throws {
        let detail = try await service.species(key: attackerSpeciesKey)
        guard token == latestRequestToken, detail.key == attackerSpeciesKey else { return }
        speciesDictionary[detail.key] = SpeciesSummary(detail: detail)
        attackerLearnsetIds = detail.learnset
        // 特性の選択肢を新しい攻撃側の abilities にする。選択中の特性が新種族に無ければ nil に戻す
        // (旧種族の特性を送らない。issue #274。ADR-0501「issue #274」2章・8章。構築の呼び出しは
        // `selectTeamIndividual` がこの後で保存された特性を上書きする)。
        attackerAbilityOptions = detail.abilities
        if let abilityId = attackerAbilityId, !detail.abilities.contains(where: { $0.id == abilityId }) {
            attackerAbilityId = nil
        }
        recomputeMoveOptions()
    }

    /// `attackerLearnsetIds` と `latestMoveSearchResults` のどちらかが変わったら呼び直す(6章)。
    private func recomputeMoveOptions() {
        moveOptions = attackerLearnsetIds.compactMap { learnedId in
            latestMoveSearchResults.first(where: { $0.id == learnedId })
        }
    }

    private func mergeSpeciesIntoDictionary(_ items: [SpeciesSummary]) {
        for item in items { speciesDictionary[item.key] = item }
    }

    private func mergeMovesIntoDictionary(_ items: [Move]) {
        for item in items { moveDictionary[item.id] = item }
    }

    /// `currentMoveId` を2段で選び直す(ADR-0501「issue #68 の残り」3章)。
    ///
    /// 1. **優先する技**: `currentMoveId` がいまの learnset(ID 集合)にあれば、辞書にあるならそのまま、
    ///    無ければ `move(id:)` で1回だけ解決して選ぶ(`moveOptions` に無くても選ぶ。構築の個体の技を
    ///    黙って既定の技に置き換えないため)。
    /// 2. **既定の技**: `moveOptions`(検索結果 ∩ learnset)から選べればそれ(learnset の順で最初の
    ///    ダメージ技、無ければ最初)。それでも選べないときだけ、learnset を先頭から辞書 or `move(id:)`
    ///    (上限 `MasterSearch.maxMoveLookupsPerSelection` 回まで)で解決し、最初のダメージ技を探す。
    ///    見つからなければ「解決できた最初の技」、それも無ければ `moveUnavailable`。
    ///
    /// `token` は `beginInput()` の世代(issue #113 A5): `move(id:)` の各 `await` の後にこれで最新かを
    /// 確かめ、追い越されていたら `moveId` を書き換えずに戻る(呼び出し側が続く
    /// `guard token == latestRequestToken` で気付く)。`CancellationError` はそのまま投げ直し、
    /// 呼び出し側の `catch`(`handleInputFailure`)に任せる。
    private func reselectMove(preferringCurrent currentMoveId: String?, token: Int) async throws {
        if let currentMoveId, attackerLearnsetIds.contains(currentMoveId) {
            var candidate = moveDictionary[currentMoveId]
            if candidate == nil {
                candidate = try await resolveMove(id: currentMoveId)
                guard token == latestRequestToken else { return }
            }
            if let candidate {
                moveId = candidate.id
                return
            }
            // 解決できなかった(404・通信失敗。A4)ので、下の既定の技に進む。
        }
        if let defaultMove = moveOptions.first(where: { $0.category != .status }) ?? moveOptions.first {
            moveId = defaultMove.id
            return
        }
        // 既知の技だけでは既定が決まらない: learnset を先頭から解決し、最初のダメージ技を探す(4章)。
        var firstResolved: Move?
        var lookups = 0
        for learnedId in attackerLearnsetIds {
            let known: Move?
            if let dictionaryMove = moveDictionary[learnedId] {
                known = dictionaryMove
            } else {
                guard lookups < MasterSearch.maxMoveLookupsPerSelection else { break }
                lookups += 1
                let resolved = try await resolveMove(id: learnedId)
                guard token == latestRequestToken else { return }
                known = resolved
            }
            guard let move = known else { continue }
            if firstResolved == nil { firstResolved = move }
            if move.category != .status {
                moveId = move.id
                return
            }
        }
        guard let fallback = firstResolved else {
            throw PokeCalcError(code: PokeCalcError.Code.moveUnavailable, message: "覚える技がマスタに見つかりません")
        }
        moveId = fallback.id
    }

    /// `move(id:)`(openapi `getMove`)を呼び、成功したら技の辞書に入れて返す。失敗(404・通信失敗)は
    /// `nil`(A4: それ自体を画面のエラーにせず、呼び出し元が既定の技へのフォールバックを続ける)。
    /// `CancellationError` はそのまま投げ直す(A5: 呼び出し元の `catch` で `handleInputFailure` に渡す)。
    private func resolveMove(id: String) async throws -> Move? {
        do {
            let move = try await service.move(id: id)
            moveDictionary[id] = move
            return move
        } catch is CancellationError {
            throw CancellationError()
        } catch {
            return nil
        }
    }

    /// いまの入力から要求を組み立て、計算する。組み立てに失敗した(性格が無い等)ときは
    /// calcBulk を呼ばずにエラーを立てる。
    private func recalculate(token: Int) async {
        let request: BulkCalcRequest
        do {
            request = try buildRequest()
        } catch {
            guard token == latestRequestToken else { return }
            handleInputFailure(error)
            return
        }
        await performCalc(request, token: token)
    }

    private func buildRequest() throws -> BulkCalcRequest {
        guard let move = selectedMove else {
            // `moveId` は `selectMove`/`reselectMove` を通じてしか変わらず、どちらも
            // `moveOptions` にある値しか設定しないはずなので、ここに来るのは内部の不整合
            // (`moveUnavailable`: 技が1つも無い、とは原因が違うので別のコードにする)。
            throw PokeCalcError(code: PokeCalcError.Code.selectedMoveMissing, message: "選択中の技が一覧にありません")
        }
        var attacker: Individual
        switch attackerBuildSource {
        case .preset(let preset):
            let build = try AttackerPreset.build(preset, moveCategory: move.category, natures: natureOptions)
            attacker = Individual(speciesKey: attackerSpeciesKey, natureId: build.natureId, sp: build.sp, itemId: attackerItemId)
        case .team(let selection):
            // 呼び出した個体の性格・SP・特性・テラスタイプをそのまま使う(プリセットに丸め直さない。
            // 種族・持ち物は画面の状態が正。ADR-0501「P6-2d」2章)。
            attacker = selection.individualForRequest(speciesKey: attackerSpeciesKey, itemId: attackerItemId)
        }
        // 画面の計算条件(急所以外)をプリセット経路・構築経路の両方に当てる(`individualForRequest` は
        // ranks/status を落とすので、組み立てた後に上書きする。issue #274。ADR-0501「issue #274」4章・8章)。
        attacker.abilityId = attackerAbilityId
        attacker.ranks = attackerRanks
        attacker.status = isAttackerBurned ? .burn : .none
        // 比較する持ち物が1つ以上あれば「持ち物なし」を先頭に含める。無ければ素の1通り(省略。規則5)。
        let itemVariants: [String?] = comparedDefenderItemIds.isEmpty
            ? []
            : [String?.none] + comparedDefenderItemIds.map { $0 as String? }
        let field = FieldState(weather: weather, terrain: terrain, defenderScreens: defenderScreens)
        return BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: defenderSpeciesKey, moveId: moveId,
            field: field, critical: isCritical, presets: [], itemVariants: itemVariants
        )
    }

    /// `request` で `calcBulk` を呼び、応答が最新の要求のものだけを画面へ反映する(規則7)。
    private func performCalc(_ request: BulkCalcRequest, token: Int) async {
        isLoading = true
        do {
            let result = try await service.calcBulk(request)
            guard token == latestRequestToken else { return }
            rows = result.rows.map { BulkRowDisplay(row: $0, items: itemOptions) }
            error = nil
            isLoading = false
        } catch {
            guard token == latestRequestToken else { return }
            handleInputFailure(error)
        }
    }
}
