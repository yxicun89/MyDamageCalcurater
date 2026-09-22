import Observation

// ReverseViewModel: 逆算画面の状態(P6-2b・ADR-0500 §1「画面のロジックは ViewModel で XCTest に
// 固定し、View は描くだけ」)。`CalcViewModel` と同じ「最新の要求だけを反映する」世代の保護を使う。
//
// 用語: 「自分」= 既知側(`known`)、「相手」= 逆算する側(`unknownSpeciesKey`)。
// 与えたダメージ = `side: .defender`(自分が攻撃側。技は自分の learnset)、
// 受けたダメージ = `side: .attacker`(自分が防御側。技は相手の learnset)。

/// 観測欄1行の入力状態。
public struct ObservationRow: Identifiable, Equatable, Sendable {
    /// 行を一意に識別する値(`ReverseViewModel` が発行する通し番号。表示順とは独立)。
    public let id: Int
    public var text: String
    /// `text` の検証エラー(空文字は `.empty`。計算は止めないが行には残す)。
    public var error: ObservationFieldError?
    /// 検証に成功したときの観測値(`error` が nil のときだけ非 nil)。
    public var observation: DamageObservation?
}

/// 逆算画面の状態。`PokeCalcService` だけに依存する(ADR-0500 §3)。
@MainActor
@Observable
public final class ReverseViewModel {
    /// openapi の `limit` の上限(`CalcViewModel` と同じ)。
    private static let masterListLimit = 200
    /// 与えたダメージ・受けたダメージのどちらでも自分・相手として選べるためには種族が最低2つ要る。
    private static let minimumSpeciesCount = 2

    private let service: any PokeCalcService
    /// 構築の永続化(P6-2d)。`CalcViewModel.teamStore` と同じ理由で省略可(既定 nil)。
    private let teamStore: (any TeamStore)?

    // MARK: - マスタ(load() で読み込む)

    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var itemOptions: [Item] = []
    /// 攻撃側(与えたダメージ = 自分、受けたダメージ = 相手)の learnset とマスタの技を突き合わせた、
    /// **ダメージ技だけ**の選択肢(learnset の順。変化技は逆算できないので出さない)。
    public private(set) var moveOptions: [Move] = []
    private var natureOptions: [Nature] = []
    private var masterMoves: [Move] = []
    private var didLoad = false

    // MARK: - 画面の入力

    public private(set) var side: ReverseSide = .defender
    public private(set) var mySpeciesKey: String = ""
    public private(set) var opponentSpeciesKey: String = ""
    public private(set) var moveId: String = ""
    /// 与えたダメージ(自分が攻撃側)のときの自分の出どころ(P6-2d。ADR-0501「P6-2d」4章:
    /// 両側に入れる)。既定は `AttackerPreset.allCases` の最初(A特化)。
    public private(set) var attackerBuildSource: AttackerBuildSource = .preset(.aFull)
    /// `attackerBuildSource.preset` のショートカット(`CalcViewModel.attackerPreset` と同じ理由で
    /// 計算プロパティにする)。
    public var attackerPreset: AttackerPreset? { attackerBuildSource.preset }
    /// 受けたダメージ(自分が防御側)のときの自分の出どころ。既定は `KnownDefenderPreset.allCases` の
    /// 最初(無振り)。
    public private(set) var knownDefenderBuildSource: KnownDefenderBuildSource = .preset(.none)
    public var knownDefenderPreset: KnownDefenderPreset? { knownDefenderBuildSource.preset }
    public private(set) var myItemId: String?
    /// 持ち物マスタの順(トグルした順ではない)。
    public private(set) var opponentItemCandidateIds: [String] = []
    private var toggledOpponentItemIds: Set<String> = []

    // MARK: - 構築から個体を呼び出す(P6-2d)

    /// 構築の一覧から作った選択肢(`CalcViewModel.teamOptions` と同じ規則)。
    public private(set) var teamOptions: [TeamPickerGroup] = []
    /// `teamOptions` の元になった構築本体(`CalcViewModel.loadedTeams` と同じ理由)。
    private var loadedTeams: [Team] = []
    /// `loadTeams()` 専用の世代の通し番号(`CalcViewModel.latestTeamListToken` と同じ理由)。
    private var latestTeamListToken = 0

    public private(set) var observations: [ObservationRow] = []
    /// 次に発行する観測行の id(単調増加。行の入れ替えをまたいでも重複しない)。
    private var nextObservationRowID = 0

    // MARK: - 結果

    public private(set) var result: ReverseResultDisplay?
    public private(set) var isLoading = false
    public private(set) var error: CalcScreenError?

    /// 「最新の要求だけを反映する」ための通し番号(`CalcViewModel.beginInput()` と同じ規則)。
    private var latestRequestToken = 0

    public init(service: any PokeCalcService, teamStore: (any TeamStore)? = nil) {
        self.service = service
        self.teamStore = teamStore
    }

    /// 側から観測の精度が決まる(与えたダメージ = %、受けたダメージ = 実点数)。
    public var observationKind: ObservationKind { ObservationKind(side: side) }

    // MARK: - 起動

    /// マスタを読み、既定の入力(与えたダメージ・自分=種族一覧の最初・相手=2番目・技=攻撃側 learnset の
    /// 順で最初のダメージ技・A特化・自分の防御側=無振り・持ち物なし・相手の持ち物候補なし・観測は空の1行)を
    /// 選ぶ。**逆算は呼ばない**(規則3)。2回目以降の呼び出しは何もしない。
    public func load() async {
        guard !didLoad else { return }
        didLoad = true
        let token = beginInput()
        isLoading = true
        do {
            let natures = try await service.natures()
            let species = try await service.searchSpecies(query: "", limit: Self.masterListLimit)
            let moves = try await service.searchMoves(query: "", limit: Self.masterListLimit)
            let items = try await service.searchItems(query: "", limit: Self.masterListLimit)
            guard token == latestRequestToken else { return }

            natureOptions = natures
            speciesOptions = species
            masterMoves = moves
            itemOptions = items

            guard species.count >= Self.minimumSpeciesCount else {
                throw PokeCalcError(
                    code: PokeCalcError.Code.insufficientSpecies,
                    message: "計算に必要な種族が足りません(\(species.count) 件)"
                )
            }
            side = .defender
            mySpeciesKey = species[0].key
            opponentSpeciesKey = species[1].key
            attackerBuildSource = .preset(.aFull)
            knownDefenderBuildSource = .preset(.none)
            myItemId = nil
            toggledOpponentItemIds = []
            opponentItemCandidateIds = []
            observations = [freshObservationRow()]

            try await reloadMoveOptions(token: token)
            guard token == latestRequestToken else { return }
            try reselectMove(preferringCurrent: nil)
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            result = nil
            isLoading = false
            return
        }
        // 観測は空の1行から始まるので計算しない状態(規則3。isLoading だけ解く)。
        guard token == latestRequestToken else { return }
        isLoading = false
        // 構築の読み込みは起動時の入力確定の後(`CalcViewModel.load()` と同じ理由)。
        await loadTeams()
    }

    // MARK: - 構築から個体を呼び出す(P6-2d。ADR-0501「P6-2d」4章: 両側に効かせる)

    /// `CalcViewModel.loadTeams()` と同じ規則。
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

    /// 構築の個体を呼び出す。**いま表示している側**(`side`)の出どころだけを変える(4章)。
    /// `teamOptions` に無い teamID/memberID は無視する(計算もしない)。
    public func selectTeamIndividual(teamID: String, memberID: String) async {
        guard let selection = TeamIndividualSelectionBuilder.make(
            teamID: teamID, memberID: memberID, teamOptions: teamOptions, teams: loadedTeams, moves: masterMoves
        ) else { return }

        mySpeciesKey = selection.individual.speciesKey
        myItemId = selection.individual.itemId
        let token = beginInput()

        switch side {
        case .defender:
            // 与えたダメージ: 自分が攻撃側なので、自分の learnset を読み直して技を選び直す
            // (`CalcViewModel.selectTeamIndividual` と同じ。3章)。
            isLoading = true
            do {
                try await reloadMoveOptions(token: token)
                guard token == latestRequestToken else { return }
                try reselectMove(preferringCurrent: selection.individual.moveId)
            } catch {
                guard token == latestRequestToken else { return }
                self.error = CalcScreenError(error)
                result = nil
                isLoading = false
                return
            }
            guard token == latestRequestToken else { return }
            error = nil
            attackerBuildSource = .team(selection)
            await recalculateIfPossible(token: token)
        case .attacker:
            // 受けたダメージ: 技は相手の learnset なので触らない(3章)。
            knownDefenderBuildSource = .team(selection)
            await recalculateIfPossible(token: token)
        }
    }

    // MARK: - 観測の操作(規則5: 送る観測の列が変わったときだけ reverse を1回呼ぶ)

    /// 観測欄を1行追加する(空の行。送る観測は変わらないので計算しない)。
    public func addObservation() {
        observations.append(freshObservationRow())
    }

    /// `id` の行を削除する。最後の1行を消すと空の1行に戻す(入力欄を無くさない)。
    public func removeObservation(id: Int) async {
        guard let index = observations.firstIndex(where: { $0.id == id }) else { return }
        let previousSendKey = currentSendKey
        observations.remove(at: index)
        if observations.isEmpty {
            observations = [freshObservationRow()]
        }
        await recalculateIfSendKeyChanged(from: previousSendKey)
    }

    /// `id` の行のテキストを差し替え、検証する(同期)。SwiftUI の `TextField` の `Binding` から直接
    /// 呼べるようにするための分割(批評対応): async な `editObservation(id:text:)` を `Task { await ... }`
    /// 経由でしか呼べないと、`Task` の起動がイベントループを1回分後回しにするため、キー入力から
    /// `TextField` の表示反映までに1フレームの遅れが出る。テキストの反映・検証だけはここで同期的に終わらせ、
    /// 逆算の呼び出しが要るときだけ呼び出し側が `recalculateAfterObservationEdit()` を(`Task` の中で)呼ぶ。
    /// 送る観測の列が操作の前後で変わっていれば true を返す(規則5)。
    @discardableResult
    public func setObservationText(id: Int, text: String) -> Bool {
        guard let index = observations.firstIndex(where: { $0.id == id }) else { return false }
        let previousSendKey = currentSendKey
        observations[index].text = text
        switch ObservationParser.parse(text, kind: observationKind) {
        case .success(let observation):
            observations[index].observation = observation
            observations[index].error = nil
        case .failure(let fieldError):
            observations[index].observation = nil
            observations[index].error = fieldError
        }
        return currentSendKey != previousSendKey
    }

    /// `setObservationText(id:text:)` が true を返したとき(送る観測の列が変わったとき)に呼ぶ、逆算の
    /// 呼び出しだけを担う部分(規則5)。
    public func recalculateAfterObservationEdit() async {
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    /// `id` の行のテキストを差し替え、検証し、必要なら計算する(`setObservationText` + 条件付きの
    /// `recalculateAfterObservationEdit` をまとめた一括版。テスト・XCUITest を介さない呼び出し用)。
    public func editObservation(id: Int, text: String) async {
        guard setObservationText(id: id, text: text) else { return }
        await recalculateAfterObservationEdit()
    }

    // MARK: - 側の切り替え(規則7)

    /// 同じ側なら何もしない。観測を空の1行に戻し、自分の持ち物・相手の持ち物候補を外し、結果を消す。
    /// 種族と両方のプリセットは残す。技は新しい攻撃側の learnset で選び直す(覚えていれば残す)。
    public func selectSide(_ newSide: ReverseSide) async {
        guard newSide != side else { return }
        side = newSide
        let token = beginInput()
        observations = [freshObservationRow()]
        result = nil
        myItemId = nil
        toggledOpponentItemIds = []
        opponentItemCandidateIds = []
        await reloadAttackingMovesAndRecalculate(token: token)
    }

    // MARK: - 種族の選択(規則7: 攻撃側の種族が変わったときだけ learnset を読み直す)

    public func selectMySpecies(key: String) async {
        guard speciesOptions.contains(where: { $0.key == key }) else { return }
        mySpeciesKey = key
        let token = beginInput()
        if side == .defender {
            await reloadAttackingMovesAndRecalculate(token: token)
        } else {
            await recalculateIfPossible(token: token)
        }
    }

    public func selectOpponentSpecies(key: String) async {
        guard speciesOptions.contains(where: { $0.key == key }) else { return }
        opponentSpeciesKey = key
        let token = beginInput()
        if side == .attacker {
            await reloadAttackingMovesAndRecalculate(token: token)
        } else {
            await recalculateIfPossible(token: token)
        }
    }

    // MARK: - その他の入力の変更(規則5: 計算できる状態なら1回呼ぶ)

    /// `moveOptions` に無い技は無視する。
    public func selectMove(id: String) async {
        guard moveOptions.contains(where: { $0.id == id }) else { return }
        moveId = id
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    public func selectMyItem(id: String?) async {
        myItemId = id
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    /// 相手の持ち物候補のトグル。`opponentItemCandidateIds` はトグルした順ではなく持ち物マスタの順。
    public func toggleOpponentItemCandidate(itemId: String) async {
        if toggledOpponentItemIds.contains(itemId) {
            toggledOpponentItemIds.remove(itemId)
        } else {
            toggledOpponentItemIds.insert(itemId)
        }
        opponentItemCandidateIds = itemOptions.map(\.id).filter { toggledOpponentItemIds.contains($0) }
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    /// 与えたダメージのときの自分のプリセット。受けたダメージのときに変えても値を覚えるだけ(計算しない)。
    /// 押すと**この側だけ**構築の選択が外れる(排他。ADR-0501「P6-2d」4章)。
    public func selectAttackerPreset(_ preset: AttackerPreset) async {
        attackerBuildSource = .preset(preset)
        guard side == .defender else { return }
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    /// 受けたダメージのときの自分のプリセット。与えたダメージのときに変えても値を覚えるだけ(計算しない)。
    /// 押すと**この側だけ**構築の選択が外れる(排他。ADR-0501「P6-2d」4章)。
    public func selectKnownDefenderPreset(_ preset: KnownDefenderPreset) async {
        knownDefenderBuildSource = .preset(preset)
        guard side == .attacker else { return }
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    // MARK: - 内部: 世代の保護

    private func beginInput() -> Int {
        latestRequestToken += 1
        return latestRequestToken
    }

    private func freshObservationRow() -> ObservationRow {
        let row = ObservationRow(id: nextObservationRowID, text: "", error: .empty, observation: nil)
        nextObservationRowID += 1
        return row
    }

    // MARK: - 内部: 送る観測の列

    /// いまの行から送る観測(有効な行だけ、行の順)。不正な行(`.empty` 以外のエラー)が1つでもあれば nil、
    /// 有効な行が1つも無くても nil(計算しない状態)。`.empty` の行(空の行)は無視するだけで計算を止めない。
    private var currentSendKey: [DamageObservation]? {
        let hasBlockingError = observations.contains { row in
            guard let error = row.error else { return false }
            return error != .empty
        }
        guard !hasBlockingError else { return nil }
        let valid = observations.compactMap(\.observation)
        return valid.isEmpty ? nil : valid
    }

    /// 観測の操作の直後に呼ぶ: 送る観測の列(`currentSendKey`)が操作の前後で変わっていなければ何もしない
    /// (空の行の追加・削除、"12" → "012" のような送る値が変わらない編集は reverse を呼ばない。規則5)。
    private func recalculateIfSendKeyChanged(from previousSendKey: [DamageObservation]?) async {
        guard currentSendKey != previousSendKey else { return }
        let token = beginInput()
        await recalculateIfPossible(token: token)
    }

    /// いまの `attackingSpeciesKey`(与えたダメージ=自分、受けたダメージ=相手)の learnset を読み直し、
    /// 技を選び直してから計算する(失敗したら `error` を立てて計算しない)。
    private func reloadAttackingMovesAndRecalculate(token: Int) async {
        isLoading = true
        do {
            let previousMoveId = moveId
            try await reloadMoveOptions(token: token)
            guard token == latestRequestToken else { return }
            try reselectMove(preferringCurrent: previousMoveId)
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            result = nil
            isLoading = false
            return
        }
        // 技の読み直し・選び直しが成功した = その原因で立っていた古いエラー(例: moveUnavailable)は
        // もう成り立たない。次の `recalculateIfPossible` が新しいエラーを見つければそこで立て直す。
        guard token == latestRequestToken else { return }
        error = nil
        await recalculateIfPossible(token: token)
    }

    /// 攻撃側(与えたダメージ=自分、受けたダメージ=相手)の種族。
    private var attackingSpeciesKey: String {
        side == .defender ? mySpeciesKey : opponentSpeciesKey
    }

    /// いまの `attackingSpeciesKey` の詳細を読み、`moveOptions`(learnset の順・マスタにある・ダメージ技だけ)を作る。
    /// 応答が届いた時点で `token` が最新でない、または詳細の種族がいまの攻撃側と違う(＝追い越された)ときは
    /// 黙って反映しない(`CalcViewModel.reloadAttackerMoveOptions` と同じ規則)。
    private func reloadMoveOptions(token: Int) async throws {
        let attackingKey = attackingSpeciesKey
        let detail = try await service.species(key: attackingKey)
        guard token == latestRequestToken, detail.key == attackingKey else { return }
        moveOptions = detail.learnset
            .compactMap { learnedId in masterMoves.first(where: { $0.id == learnedId }) }
            .filter { $0.category != .status }
    }

    /// `currentMoveId` がいまの `moveOptions` にまだあればそれを残し、無ければ最初のダメージ技を選ぶ。
    /// `moveOptions` が空(ダメージ技が1つも無い)ときは `moveUnavailable`。
    private func reselectMove(preferringCurrent currentMoveId: String?) throws {
        if let currentMoveId, moveOptions.contains(where: { $0.id == currentMoveId }) {
            moveId = currentMoveId
            return
        }
        guard let firstMove = moveOptions.first else {
            throw PokeCalcError(code: PokeCalcError.Code.moveUnavailable, message: "覚えるダメージ技がマスタに見つかりません")
        }
        moveId = firstMove.id
    }

    // MARK: - 内部: 逆算を呼ぶ

    /// 計算できる状態(`currentSendKey` が非 nil)なら `reverse` を1回呼び、そうでなければ結果を消して
    /// 読み込み中を解く(規則5・8。応答は `token` が最新のときだけ反映する)。
    private func recalculateIfPossible(token: Int) async {
        guard let observationsToSend = currentSendKey else {
            guard token == latestRequestToken else { return }
            result = nil
            isLoading = false
            // 技が選べている(= マスタ起因のエラーではない)なら、計算しない状態に戻ったときに
            // 古いエラー(前回の reverse 失敗等)を持ち越さない。`moveOptions` が空(moveUnavailable 等)
            // のときは、その原因がまだ直っていないので消さない。
            if moveOptions.contains(where: { $0.id == moveId }) {
                error = nil
            }
            return
        }
        isLoading = true
        let request: ReverseRequest
        do {
            request = try buildRequest(observations: observationsToSend)
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            result = nil
            isLoading = false
            return
        }
        do {
            let response = try await service.reverse(request)
            guard token == latestRequestToken else { return }
            result = ReverseResultDisplay(result: response, items: itemOptions)
            error = nil
            isLoading = false
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            result = nil
            isLoading = false
        }
    }

    /// いまの入力から `ReverseRequest` を組み立てる(規則6)。既知側(`known`)は常に自分。
    private func buildRequest(observations: [DamageObservation]) throws -> ReverseRequest {
        guard let move = moveOptions.first(where: { $0.id == moveId }) else {
            throw PokeCalcError(code: PokeCalcError.Code.selectedMoveMissing, message: "選択中の技が一覧にありません")
        }
        let known: Individual
        switch side {
        case .defender:
            switch attackerBuildSource {
            case .preset(let preset):
                let build = try AttackerPreset.build(preset, moveCategory: move.category, natures: natureOptions)
                known = Individual(speciesKey: mySpeciesKey, natureId: build.natureId, sp: build.sp, itemId: myItemId)
            case .team(let selection):
                known = selection.individualForRequest(speciesKey: mySpeciesKey, itemId: myItemId)
            }
        case .attacker:
            switch knownDefenderBuildSource {
            case .preset(let preset):
                let build = try KnownDefenderPreset.build(preset, moveCategory: move.category, natures: natureOptions)
                known = Individual(speciesKey: mySpeciesKey, natureId: build.natureId, sp: build.sp, itemId: myItemId)
            case .team(let selection):
                known = selection.individualForRequest(speciesKey: mySpeciesKey, itemId: myItemId)
            }
        }
        // 相手の持ち物候補が1つ以上あれば「持ち物なし」を先頭に含める。無ければ省略(規則6)。
        let itemCandidates: [String?] = opponentItemCandidateIds.isEmpty
            ? []
            : [String?.none] + opponentItemCandidateIds.map { $0 as String? }
        return ReverseRequest(
            format: .single, side: side, known: known, unknownSpeciesKey: opponentSpeciesKey, moveId: moveId,
            itemCandidates: itemCandidates, observations: observations, critical: false, maxCandidates: 0
        )
    }
}
