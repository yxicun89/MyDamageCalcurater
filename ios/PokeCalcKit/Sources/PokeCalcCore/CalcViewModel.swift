import Observation

// CalcViewModel: ダメージ計算画面の状態(P6-2a・ADR-0017 §1「画面のロジックは ViewModel で
// XCTest に固定し、View は描くだけ」)。
//
// `@Observable` は Apple の Observation フレームワークのマクロ。このモジュールに同名の型
// (旧 `Observation` enum)があると `Observation.ObservationRegistrar` の解決が衝突するため、
// ドメイン側は `DamageObservation` に改名してある(DomainTypes.swift のコメント参照)。

/// ダメージ計算画面の状態。`PokeCalcService` だけに依存し、View は生成型やサービスを直接知らない
/// (ADR-0017 §3)。
@MainActor
@Observable
public final class CalcViewModel {
    /// openapi の `limit` の上限(`searchSpecies` 等)。画面は検索語を絞らず、まとめて読む。
    private static let masterListLimit = 200
    /// 攻撃側・防御側に選べるためには種族が最低2つ要る。
    private static let minimumSpeciesCount = 2

    private let service: any PokeCalcService

    // MARK: - マスタ(load() で読み込む)

    public private(set) var speciesOptions: [SpeciesSummary] = []
    public private(set) var itemOptions: [Item] = []
    /// 攻撃側の learnset とマスタの技を突き合わせた選択肢(learnset の順)。
    public private(set) var moveOptions: [Move] = []
    private var natureOptions: [Nature] = []
    /// `moveOptions` を作るための技マスタ全件(learnset の ID と突き合わせる)。
    private var masterMoves: [Move] = []
    /// `load()` の二重実行を防ぐ(同じ画面から複数回 `load()` を呼んでも読み込みは1回だけ)。
    private var didLoad = false

    // MARK: - 画面の入力

    public private(set) var attackerSpeciesKey: String = ""
    public private(set) var defenderSpeciesKey: String = ""
    public private(set) var moveId: String = ""
    /// 既定は `AttackerPreset.allCases` の最初(A特化。`AttackerPresetTests` が順序を固定する)。
    public private(set) var attackerPreset: AttackerPreset = .aFull
    public private(set) var attackerItemId: String?
    /// 持ち物マスタの順(トグルした順ではない)。
    public private(set) var comparedDefenderItemIds: [String] = []
    /// `comparedDefenderItemIds` を作るための、トグルされた持ち物 ID の集合(順序は持たない)。
    private var toggledDefenderItemIds: Set<String> = []

    // MARK: - 計算結果

    public private(set) var rows: [BulkRowDisplay] = []
    public private(set) var isLoading = false
    public private(set) var error: CalcScreenError?

    /// 「最新の要求だけを反映する」ための通し番号(規則7)。`beginInput()` が入力操作のたびに進め、
    /// その操作から生まれる `species(key:)` / `calcBulk` の応答・失敗はすべて同じ番号で判定する
    /// (species の応答も含めて世代を守る。M1)。値そのものに意味は無い。
    private var latestRequestToken = 0

    public init(service: any PokeCalcService) {
        self.service = service
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
            attackerSpeciesKey = species[0].key
            defenderSpeciesKey = species[1].key
            attackerPreset = .aFull
            attackerItemId = nil
            toggledDefenderItemIds = []
            comparedDefenderItemIds = []

            try await reloadAttackerMoveOptions(token: token)
            guard token == latestRequestToken else { return }
            try reselectMove(preferringCurrent: nil)
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            rows = []
            isLoading = false
            return
        }
        await recalculate(token: token)
    }

    // MARK: - 入力の変更(規則4〜6。どれも calcBulk をちょうど1回呼ぶ)

    public func selectAttacker(speciesKey: String) async {
        guard speciesOptions.contains(where: { $0.key == speciesKey }) else { return }
        let token = beginInput()
        attackerSpeciesKey = speciesKey
        await applyAttackerChangeAndRecalculate(token: token)
    }

    public func selectDefender(speciesKey: String) async {
        guard speciesOptions.contains(where: { $0.key == speciesKey }) else { return }
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

    public func selectAttackerPreset(_ preset: AttackerPreset) async {
        let token = beginInput()
        attackerPreset = preset
        await recalculate(token: token)
    }

    public func selectAttackerItem(id: String?) async {
        let token = beginInput()
        attackerItemId = id
        await recalculate(token: token)
    }

    /// 比較する持ち物のトグル。`comparedDefenderItemIds` はトグルした順ではなく持ち物マスタの順(規則5)。
    public func toggleDefenderItemComparison(itemId: String) async {
        let token = beginInput()
        if toggledDefenderItemIds.contains(itemId) {
            toggledDefenderItemIds.remove(itemId)
        } else {
            toggledDefenderItemIds.insert(itemId)
        }
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

    // MARK: - 表示用の派生値(M4: 技の要約・相性)

    /// 選択中の技。`moveOptions` に無ければ nil(読み込み中・不整合)。
    public var selectedMove: Move? {
        moveOptions.first(where: { $0.id == moveId })
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
            try reselectMove(preferringCurrent: previousMoveId)
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            rows = []
            isLoading = false
            return
        }
        await recalculate(token: token)
    }

    /// いまの `attackerSpeciesKey` の詳細を読み、`moveOptions`(learnset の順・マスタにある技だけ)を作る。
    /// 応答が届いた時点で `token` が最新でない、または詳細の種族がいまの攻撃側と違う(＝この操作は
    /// 追い越された)ときは、黙って反映しない(M1)。
    private func reloadAttackerMoveOptions(token: Int) async throws {
        let detail = try await service.species(key: attackerSpeciesKey)
        guard token == latestRequestToken, detail.key == attackerSpeciesKey else { return }
        moveOptions = detail.learnset.compactMap { learnedId in
            masterMoves.first(where: { $0.id == learnedId })
        }
    }

    /// `currentMoveId` がいまの `moveOptions` にまだあればそれを残し、無ければ
    /// 「learnset の順で最初のダメージ技(無ければ learnset の最初)」を選ぶ(規則3・4)。
    /// `moveOptions` が空(learnset とマスタの技が1つも一致しない)ときは `moveUnavailable`。
    private func reselectMove(preferringCurrent currentMoveId: String?) throws {
        if let currentMoveId, moveOptions.contains(where: { $0.id == currentMoveId }) {
            moveId = currentMoveId
            return
        }
        guard let defaultMove = moveOptions.first(where: { $0.category != .status }) ?? moveOptions.first else {
            throw PokeCalcError(code: PokeCalcError.Code.moveUnavailable, message: "覚える技がマスタに見つかりません")
        }
        moveId = defaultMove.id
    }

    /// いまの入力から要求を組み立て、計算する。組み立てに失敗した(性格が無い等)ときは
    /// calcBulk を呼ばずにエラーを立てる。
    private func recalculate(token: Int) async {
        let request: BulkCalcRequest
        do {
            request = try buildRequest()
        } catch {
            guard token == latestRequestToken else { return }
            self.error = CalcScreenError(error)
            rows = []
            isLoading = false
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
        let build = try AttackerPreset.build(attackerPreset, moveCategory: move.category, natures: natureOptions)
        let attacker = Individual(speciesKey: attackerSpeciesKey, natureId: build.natureId, sp: build.sp, itemId: attackerItemId)
        // 比較する持ち物が1つ以上あれば「持ち物なし」を先頭に含める。無ければ素の1通り(省略。規則5)。
        let itemVariants: [String?] = comparedDefenderItemIds.isEmpty
            ? []
            : [String?.none] + comparedDefenderItemIds.map { $0 as String? }
        return BulkCalcRequest(
            format: .single, attacker: attacker, defenderSpeciesKey: defenderSpeciesKey, moveId: moveId,
            critical: false, presets: [], itemVariants: itemVariants
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
            self.error = CalcScreenError(error)
            rows = []
            isLoading = false
        }
    }
}
