// SpeedViewModel: 素早さ比較画面の状態(P6-24。ADR-0503)。
//
// 入力(自分のポケモン・表の絞り込み・表の場の状態)から要求を作り、`SpeedService` の応答を表示用に整える。
// 計算ロジックは持たない(素早さ・並び・位置は speed-svc の応答のまま)。例外は Web と同じ1点だけ:
// 表の境界線は「表示中の tiers の speed と自分の実数値を直接比べて」決める(絞り込みで表示行が減ると、
// 応答の faster/slower〈常に全6行基準〉では引けないため。ADR-0503 §6)。
//
// 状態の作り(CalcViewModel・ReverseViewModel と同じ流儀):
//  - `@MainActor @Observable`。入力のたびに `LatestTaskRunner` で先行の要求を cancel し、最新の世代の応答だけ反映する。
//  - 表・位置・ポケモン一覧は互いに独立(片方の失敗がもう片方の表示を消さない)。
//  - `CancellationError` は画面のエラーにしない。
//
// 公開 API の形(名前・型・引数)は SpeedViewModel*Tests 群が固定する。

import Foundation
import Observation

/// 表示する失敗。`message` は `SpeedLabels.errorMessage(forCode:)`(サーバーの英語 message は出さない)。
public struct SpeedFailure: Equatable, Sendable {
    public let code: String
    public var message: String { SpeedLabels.errorMessage(forCode: code) }

    public init(code: String) {
        self.code = code
    }
}

/// ポケモン一覧・表の読み込み状態。
public enum SpeedLoadState<Value: Equatable & Sendable>: Equatable, Sendable {
    case loading
    case loaded(Value)
    case failed(SpeedFailure)
}

/// 自分の位置の状態。`idle` は送れる入力が無い(未選択・未入力・範囲外)。
public enum SpeedPositionState: Equatable, Sendable {
    case idle
    case loading
    case loaded(SpeedPosition)
    case failed(SpeedFailure)
}

/// 画面に出す1行(ポケモン × 調整)。
public struct SpeedEntryDisplay: Equatable, Identifiable, Sendable {
    public let id: String
    public let nameJa: String
    /// エンブレム色用の先頭タイプ ID。
    public let primaryType: String
    /// 調整名(例「最速スカーフ」)。
    public let presetLabel: String
    /// 「名前・調整」(例「テストカソウドリ・最速」)。
    public let label: String

    public init(id: String, nameJa: String, primaryType: String, presetLabel: String, label: String) {
        self.id = id
        self.nameJa = nameJa
        self.primaryType = primaryType
        self.presetLabel = presetLabel
        self.label = label
    }
}

/// 表の1段(同じ実数値)。
public struct SpeedTierDisplay: Equatable, Identifiable, Sendable {
    public let speed: Int
    /// 「素早さ 250」
    public let speedLabel: String
    /// 2行以上(同速)。
    public let isTie: Bool
    /// 自分の実数値と同じ段(強調する。境界線は引かない)。
    public let isSelf: Bool
    public let entries: [SpeedEntryDisplay]

    public var id: Int { speed }

    public init(speed: Int, speedLabel: String, isTie: Bool, isSelf: Bool, entries: [SpeedEntryDisplay]) {
        self.speed = speed
        self.speedLabel = speedLabel
        self.isTie = isTie
        self.isSelf = isSelf
        self.entries = entries
    }
}

/// 表の並び: 段と、自分が挟まる境界の印(自分の実数値と同じ段が無いときだけ1つ)。
public enum SpeedTableRow: Equatable, Identifiable, Sendable {
    case tier(SpeedTierDisplay)
    case selfBoundary

    public var id: String {
        switch self {
        case .tier(let tier): return "tier-\(tier.speed)"
        case .selfBoundary: return "boundary"
        }
    }
}

/// 右の結果の表示用の整形(応答のまま。計算しない)。
public struct SpeedPositionDisplay: Equatable, Sendable {
    /// 要求にポケモンがあったときの名前。
    public let pokemonName: String?
    /// 「実数値 301」
    public let speedLabel: String
    public let fasterLabel: String
    public let slowerLabel: String
    /// 表の「トリックルーム」が on のときだけ非 nil(slower → 先に動く、faster → 後に動く)。
    public let movesBeforeLabel: String?
    public let movesAfterLabel: String?
    public let tieEntries: [SpeedEntryDisplay]
    public var hasTie: Bool { !tieEntries.isEmpty }

    public init(
        pokemonName: String?, speedLabel: String, fasterLabel: String, slowerLabel: String,
        movesBeforeLabel: String?, movesAfterLabel: String?, tieEntries: [SpeedEntryDisplay]
    ) {
        self.pokemonName = pokemonName
        self.speedLabel = speedLabel
        self.fasterLabel = fasterLabel
        self.slowerLabel = slowerLabel
        self.movesBeforeLabel = movesBeforeLabel
        self.movesAfterLabel = movesAfterLabel
        self.tieEntries = tieEntries
    }
}

/// 入力の待ち時間などの定数(直書きしない。coding-rules §2)。
public enum SpeedInput {
    /// 位置の再計算の待ち時間(連続入力をまとめる。`CalcInput.debounceInterval` と同じ値)。
    public static let debounceInterval: Duration = CalcInput.debounceInterval
}

@MainActor
@Observable
public final class SpeedViewModel {
    // MARK: - 状態

    public private(set) var pokemonState: SpeedLoadState<[SpeedPokemon]> = .loading
    public private(set) var tableState: SpeedLoadState<SpeedTable> = .loading
    public private(set) var positionState: SpeedPositionState = .idle

    // MARK: - 入力(自分のポケモン。初期値は Web と同じ: preset・最速・スカーフ無し・ポケモン未選択)

    public private(set) var mode: SpeedInputMode = .preset
    public private(set) var selectedPokemonID: String?
    public private(set) var preset: SpeedMinimalPreset = .max
    public private(set) var scarf = false
    public private(set) var tailwind = false
    public private(set) var paralysis = false
    public private(set) var sp = 0
    public private(set) var nature: SpeedNature = .neutral
    public private(set) var rank = 0
    public private(set) var rawValueText = ""

    // MARK: - 入力(表)

    /// 表に出す調整(既定は全6行 = 絞り込みなし)。
    public private(set) var selectedFilterPresets: Set<SpeedPresetID> = Set(SpeedPresetID.allCases)
    public private(set) var tableTailwind = false
    public private(set) var trickRoom = false

    // MARK: - ポケモンの選択(speed の一覧だけを使う簡易ピッカー)

    public private(set) var pokemonQuery = ""

    private let service: any SpeedService
    private let debounce: Duration

    /// 位置・表の要求を1つずつだけ保持する(先行の要求は新しい入力で cancel する)。
    private let positionRunner = LatestTaskRunner()
    private let tableRunner = LatestTaskRunner()
    private var positionTask: Task<Void, Never>?
    private var tableTask: Task<Void, Never>?
    /// 最新の要求かを確かめる世代(サービスが cancel を無視して古い応答を返しても反映しない)。
    private var positionGeneration = 0
    private var tableGeneration = 0

    public init(service: any SpeedService, debounce: Duration = SpeedInput.debounceInterval) {
        self.service = service
        self.debounce = debounce
    }

    // MARK: - 読み込み

    /// ポケモン一覧と表(今の絞り込み・場の状態。初期は全6行・off)を1回ずつ取る。2つは独立(片方の失敗がもう片方を消さない)。
    public func load() async {
        pokemonState = .loading
        tableState = .loading
        tableRunner.cancel()
        tableGeneration += 1
        let token = tableGeneration
        let presets = requestedPresets
        let field = tableField
        let service = self.service
        async let pokemonResult = Self.fetchPokemon(service)
        async let tableResult = Self.fetchTable(service, presets: presets, field: field)
        let (pokemon, table) = await (pokemonResult, tableResult)
        applyPokemon(pokemon)
        if token == tableGeneration { applyTable(table) }
    }

    private nonisolated static func fetchPokemon(_ service: any SpeedService) async -> Result<SpeedPokemonList, any Error> {
        do { return .success(try await service.pokemon()) } catch { return .failure(error) }
    }

    private nonisolated static func fetchTable(
        _ service: any SpeedService, presets: [SpeedPresetID]?, field: SpeedTableField
    ) async -> Result<SpeedTable, any Error> {
        do { return .success(try await service.table(presets: presets, field: field)) } catch { return .failure(error) }
    }

    private func applyPokemon(_ result: Result<SpeedPokemonList, any Error>) {
        switch result {
        case .success(let list): pokemonState = .loaded(list.pokemon)
        case .failure(let error):
            if !(error is CancellationError) { pokemonState = .failed(Self.failure(error)) }
        }
    }

    private func applyTable(_ result: Result<SpeedTable, any Error>) {
        switch result {
        case .success(let table): tableState = .loaded(table)
        case .failure(let error):
            if !(error is CancellationError) { tableState = .failed(Self.failure(error)) }
        }
    }

    private static func failure(_ error: any Error) -> SpeedFailure {
        SpeedFailure(code: (error as? PokeCalcError)?.code ?? PokeCalcError.Code.transport)
    }

    // MARK: - 入力の操作(要求が要るものは、先行の要求を cancel して予約する。`settle()` で待てる)

    /// nil は未選択。
    public func selectPokemon(id: String?) {
        changePositionInput { selectedPokemonID = id }
    }

    public func setMode(_ mode: SpeedInputMode) {
        changePositionInput { self.mode = mode }
    }

    public func setPreset(_ preset: SpeedMinimalPreset) {
        changePositionInput { self.preset = preset }
    }

    public func setScarf(_ value: Bool) {
        changePositionInput { scarf = value }
    }

    public func setTailwind(_ value: Bool) {
        changePositionInput { tailwind = value }
    }

    public func setParalysis(_ value: Bool) {
        changePositionInput { paralysis = value }
    }

    /// `0...SPLimits.maxPerStat` に収める。
    public func setSP(_ value: Int) {
        changePositionInput { sp = min(max(value, 0), SPLimits.maxPerStat) }
    }

    public func setNature(_ nature: SpeedNature) {
        changePositionInput { self.nature = nature }
    }

    /// `RankLimits.min...RankLimits.max` に収める。
    public func setRank(_ value: Int) {
        changePositionInput { rank = min(max(value, RankLimits.min), RankLimits.max) }
    }

    public func setRawValueText(_ text: String) {
        changePositionInput { rawValueText = text }
    }

    /// 表の絞り込みの切り替え。最後の1つは外せない(契約上 presets は1つ以上)。
    public func toggleFilterPreset(_ id: SpeedPresetID) {
        if selectedFilterPresets.contains(id) {
            guard selectedFilterPresets.count > 1 else { return }
            selectedFilterPresets.remove(id)
        } else {
            selectedFilterPresets.insert(id)
        }
        reloadTable()
    }

    public func setTableTailwind(_ value: Bool) {
        guard value != tableTailwind else { return }
        changePositionInput { tableTailwind = value }
        reloadTable()
    }

    public func setTrickRoom(_ value: Bool) {
        guard value != trickRoom else { return }
        trickRoom = value
        reloadTable()
    }

    public func setPokemonQuery(_ text: String) {
        pokemonQuery = text
    }

    /// 予約済みの表・位置の要求がすべて終わる(または cancel される)まで待つ。テストと View の `.task` 用。
    public func settle() async {
        await tableTask?.value
        await positionTask?.value
    }

    /// 画面破棄(`.onDisappear`)。待機中・送信済みの要求を cancel する。
    public func cancelPendingWork() {
        positionRunner.cancel()
        tableRunner.cancel()
    }

    // MARK: - 要求の予約

    /// 入力を変え、位置の要求が変わったときだけ取り直す(preset のまま実数値欄を触る、などでは送らない)。
    private func changePositionInput(_ change: () -> Void) {
        let before = positionRequest
        change()
        let after = positionRequest
        guard after != before else { return }
        guard let request = after else {
            positionRunner.cancel()
            positionGeneration += 1
            positionTask = nil
            positionState = .idle
            return
        }
        positionGeneration += 1
        let token = positionGeneration
        positionState = .loading
        let service = self.service
        positionTask = positionRunner.schedule(debounce: debounce) { [weak self] in
            let result: Result<SpeedPosition, any Error>
            do { result = .success(try await service.position(request)) } catch { result = .failure(error) }
            self?.applyPosition(result, token: token)
        }
    }

    private func applyPosition(_ result: Result<SpeedPosition, any Error>, token: Int) {
        guard token == positionGeneration else { return }
        switch result {
        case .success(let position): positionState = .loaded(position)
        case .failure(let error):
            if !(error is CancellationError) { positionState = .failed(Self.failure(error)) }
        }
    }

    /// 表の取り直し(離散の操作なので debounce しない)。
    private func reloadTable() {
        tableGeneration += 1
        let token = tableGeneration
        tableState = .loading
        let service = self.service
        let presets = requestedPresets
        let field = tableField
        tableTask = tableRunner.schedule(debounce: .zero) { [weak self] in
            let result = await Self.fetchTable(service, presets: presets, field: field)
            guard let self, token == self.tableGeneration else { return }
            self.applyTable(result)
        }
    }

    /// 全6行なら nil(クエリを省く)、そうでなければ選んだ行を契約の順で。
    private var requestedPresets: [SpeedPresetID]? {
        guard selectedFilterPresets.count < SpeedPresetID.allCases.count else { return nil }
        return SpeedPresetID.allCases.filter { selectedFilterPresets.contains($0) }
    }

    private var tableField: SpeedTableField {
        SpeedTableField(tailwind: tableTailwind, trickRoom: trickRoom)
    }

    // MARK: - 派生(表示用)

    /// 今の入力から作る位置の要求。送れない(未選択・未入力・範囲外)なら nil。
    public var positionRequest: SpeedPositionRequest? {
        let input: SpeedPositionRequest.Input
        switch mode {
        case .preset:
            guard let id = selectedPokemonID else { return nil }
            input = .preset(pokemonId: id, preset: preset, scarf: scarf, tailwind: tailwind, paralysis: paralysis)
        case .custom:
            guard let id = selectedPokemonID else { return nil }
            input = .custom(
                pokemonId: id, sp: sp, nature: nature, rank: rank, scarf: scarf, tailwind: tailwind, paralysis: paralysis)
        case .raw:
            guard let value = rawValue else { return nil }
            input = .raw(value: value, pokemonId: selectedPokemonID)
        }
        return SpeedPositionRequest(input: input, tableTailwind: tableTailwind)
    }

    /// 前後の空白を落とした実数値(1以上の整数。そうでなければ nil)。
    private var rawValue: Int? {
        let text = rawValueText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty, text.allSatisfy({ $0.isASCII && $0.isNumber }), let value = Int(text), value >= 1 else {
            return nil
        }
        return value
    }

    /// 実数値の入力欄の検証メッセージ(raw モードで、空でなく 1 以上の整数でないとき)。
    public var rawValueError: String? {
        guard mode == .raw else { return nil }
        let text = rawValueText.trimmingCharacters(in: .whitespacesAndNewlines)
        return text.isEmpty || rawValue != nil ? nil : SpeedLabels.rawValueRange
    }

    /// 絞り込みが1つだけ残っているとき true(「少なくとも1つは選ぶ必要があります」を出す)。
    public var filterMinimumNoticeVisible: Bool {
        selectedFilterPresets.count == 1
    }

    /// ピッカーに出す一覧(`pokemonQuery` で名前を絞る。前後空白を落とし、ひらがなはカタカナとして照合)。
    public var filteredPokemon: [SpeedPokemon] {
        guard case .loaded(let all) = pokemonState else { return [] }
        let query = Self.katakana(pokemonQuery.trimmingCharacters(in: .whitespacesAndNewlines))
        guard !query.isEmpty else { return all }
        return all.filter { Self.katakana($0.nameJa).contains(query) }
    }

    private static func katakana(_ text: String) -> String {
        text.applyingTransform(.hiraganaToKatakana, reverse: false) ?? text
    }

    public var selectedPokemon: SpeedPokemon? {
        guard let id = selectedPokemonID, case .loaded(let all) = pokemonState else { return nil }
        return all.first { $0.pokemonId == id }
    }

    /// 表の並び(段 + 境界の印)。表が読み込めていないときは空。
    /// 境界・強調は、表示中の段の speed と自分の実数値の直接比較で決める(ADR-0503 §6)。
    public var tableRows: [SpeedTableRow] {
        guard case .loaded(let table) = tableState else { return [] }
        var ownSpeed: Int?
        if case .loaded(let position) = positionState { ownSpeed = position.speed }
        var rows: [SpeedTableRow] = []
        var boundaryDrawn = ownSpeed == nil || table.tiers.contains { $0.speed == ownSpeed }
        for tier in table.tiers {
            // 通常は降順(自分より遅い最初の段の前)、トリックルーム中は昇順(自分より速い最初の段の前)。
            if !boundaryDrawn, let own = ownSpeed, trickRoom ? tier.speed > own : tier.speed < own {
                rows.append(.selfBoundary)
                boundaryDrawn = true
            }
            rows.append(.tier(tierDisplay(tier, isSelf: tier.speed == ownSpeed)))
        }
        if !boundaryDrawn { rows.append(.selfBoundary) }
        return rows
    }

    private func tierDisplay(_ tier: SpeedTier, isSelf: Bool) -> SpeedTierDisplay {
        SpeedTierDisplay(
            speed: tier.speed, speedLabel: SpeedLabels.tierSpeed(tier.speed), isTie: tier.entries.count >= 2,
            isSelf: isSelf, entries: tier.entries.map(Self.entryDisplay))
    }

    private static func entryDisplay(_ entry: SpeedTableEntry) -> SpeedEntryDisplay {
        let presetLabel = SpeedLabels.preset(entry.preset)
        return SpeedEntryDisplay(
            id: entry.id, nameJa: entry.nameJa, primaryType: entry.types.first ?? "", presetLabel: presetLabel,
            label: entry.nameJa + SpeedLabels.entrySeparator + presetLabel)
    }

    /// 位置が読み込めているときの結果の整形。
    public var positionDisplay: SpeedPositionDisplay? {
        guard case .loaded(let position) = positionState else { return nil }
        return SpeedPositionDisplay(
            pokemonName: position.pokemon?.nameJa,
            speedLabel: SpeedLabels.selfSpeed(position.speed),
            fasterLabel: SpeedLabels.faster(position.faster),
            slowerLabel: SpeedLabels.slower(position.slower),
            movesBeforeLabel: trickRoom ? SpeedLabels.movesBefore(position.slower) : nil,
            movesAfterLabel: trickRoom ? SpeedLabels.movesAfter(position.faster) : nil,
            tieEntries: position.tie.map(Self.entryDisplay))
    }
}
