// SpeedMatchupViewModel: 素早さ画面(相手を選んで上を取る最低ライン)の状態(I-speed-4c。ADR-0610 §4)。
//
// 入力(自分・相手・場)から matchup / reach の要求を作り、`SpeedService` の応答を `SpeedMatchupDisplayBuilder` で整える。
// 計算ロジックは持たない。状態の作りは `CalcViewModel` と同じ流儀:
//  - `@MainActor @Observable`。入力のたびに `LatestTaskRunner` で先行の要求を cancel し、最新の世代の応答だけ反映する。
//  - ポケモン一覧・matchup・reach は互いに独立(片方の失敗がもう片方の表示を消さない)。
//  - `CancellationError` は画面のエラーにしない。
//
// このクラスは旧「素早さの表」の画面の状態を置き換えた(旧型は削除済み。ADR-0610 追記「iOS の画面」)。
// 公開 API の形(名前・型・引数)は SpeedMatchupViewModel*Tests が固定する。

import Foundation
import Observation

/// 表示する失敗。サーバーが使えない理由は `ServiceFailureKind` に従い(ADR-0534)、入力の誤りなどのコードだけ
/// `SpeedLabels.errorMessage(forCode:)` から引く(サーバーの英語 message・番号は出さない)。
public struct SpeedFailure: Equatable, Sendable {
    public let code: String
    private let kind: ServiceFailureKind

    public var message: String {
        switch kind {
        case .other(let otherCode):
            return SpeedLabels.errorMessage(forCode: otherCode)
        case .serverFault where code == Self.masterUnavailableCode:
            // ポケモンのデータを読み込めない旨(原因がより具体的)。
            return SpeedLabels.errorMessage(forCode: code)
        default:
            return kind.message(feature: SpeedLabels.screenName)
        }
    }

    private static let masterUnavailableCode = "master_unavailable"

    public init(code: String) {
        self.init(PokeCalcError(code: code, message: ""))
    }

    public init(_ error: any Error) {
        code = (error as? PokeCalcError)?.code ?? PokeCalcError.Code.decode
        kind = ServiceFailureKind(error)
    }
}

/// ポケモン一覧の読み込み状態。
public enum SpeedLoadState<Value: Equatable & Sendable>: Equatable, Sendable {
    case loading
    case loaded(Value)
    case failed(SpeedFailure)
}

/// 入力の待ち時間などの定数(直書きしない。coding-rules §2)。
public enum SpeedInput {
    /// 再計算の待ち時間(連続入力をまとめる。`CalcInput.debounceInterval` と同じ値)。
    public static let debounceInterval: Duration = CalcInput.debounceInterval
}

/// matchup / reach の取得状態。`idle` は送れる入力が無い(ポケモン未選択など)。
public enum SpeedRequestState<Value: Equatable & Sendable>: Equatable, Sendable {
    case idle
    case loading
    case loaded(Value)
    case failed(SpeedFailure)

    public var isFailed: Bool {
        if case .failed = self { return true }
        return false
    }
}

/// 計算画面の連続技へ渡す値(ADR-0610 §4。連続技が計算画面にあるときだけ使う)。
/// 攻撃側 = 自分、防御側 = 相手。今の振りがあれば素早さの SP・性格(とランク・スカーフ)も渡す。
public struct SpeedComboHandoff: Equatable, Sendable {
    public let attackerPokemonID: String
    public let defenderPokemonID: String
    public let attackerSpeed: SpeedInvestment?
    public let attackerRank: Int
    public let attackerScarf: Bool

    public init(
        attackerPokemonID: String, defenderPokemonID: String, attackerSpeed: SpeedInvestment?, attackerRank: Int,
        attackerScarf: Bool
    ) {
        self.attackerPokemonID = attackerPokemonID
        self.defenderPokemonID = defenderPokemonID
        self.attackerSpeed = attackerSpeed
        self.attackerRank = attackerRank
        self.attackerScarf = attackerScarf
    }
}

@MainActor
@Observable
public final class SpeedMatchupViewModel {
    // MARK: - 状態

    public private(set) var pokemonState: SpeedLoadState<[SpeedPokemon]> = .loading
    public private(set) var matchupState: SpeedRequestState<SpeedMatchup> = .idle
    public private(set) var reachState: SpeedRequestState<SpeedReach> = .idle

    // MARK: - 入力(自分)

    public private(set) var selfPokemonID: String?
    /// -6〜+6。
    public private(set) var rank = 0
    public private(set) var scarf = false
    public private(set) var tailwind = false
    public private(set) var paralysis = false
    /// 今の振り(任意)。nil なら行動順のバッジも「これ以上」も出さない。
    public private(set) var current: SpeedInvestment?

    // MARK: - 入力(相手)

    public private(set) var opponentPokemonID: String?
    /// 想定のチップ(既定は 無振り・準速・最速・最速スカーフ の 4 つ)。
    public private(set) var selectedPresets: Set<SpeedPresetID> = SpeedMatchupViewModel.defaultPresets
    /// カスタムの想定(最大 1 つ)。
    public private(set) var customAssumption: SpeedCustomAssumption?
    public private(set) var opponentTailwind = false
    public private(set) var opponentParalysis = false

    // MARK: - 入力(場)

    public private(set) var trickRoom = false

    // MARK: - ポケモンの選択

    public private(set) var pokemonQuery = ""

    /// 想定の既定(ADR-0610 §2 と同じ 4 つ)。
    public static let defaultPresets: Set<SpeedPresetID> = [.uninvested, .neutralMax, .max, .maxScarf]

    /// 計算画面に連続技があるか(連続技 CB-8 が入るまで false。false の間は `comboHandoff` が nil)。
    public let comboAvailable: Bool

    private let service: any SpeedService
    private let debounce: Duration

    /// matchup・reach の要求を 1 つずつだけ保持する(先行の要求は新しい入力で cancel する)。
    private let matchupRunner = LatestTaskRunner()
    private let reachRunner = LatestTaskRunner()
    private var matchupTask: Task<Void, Never>?
    private var reachTask: Task<Void, Never>?
    /// 最新の要求かを確かめる世代(サービスが cancel を無視して古い応答を返しても反映しない)。
    private var matchupGeneration = 0
    private var reachGeneration = 0

    public init(
        service: any SpeedService, debounce: Duration = SpeedInput.debounceInterval, comboAvailable: Bool = false
    ) {
        self.service = service
        self.debounce = debounce
        self.comboAvailable = comboAvailable
    }

    // MARK: - 読み込み

    /// ポケモン一覧を 1 回取る(matchup・reach は入力が揃ったときに取る)。
    public func load() async {
        pokemonState = .loading
        do {
            pokemonState = .loaded(try await service.pokemon().pokemon)
        } catch is CancellationError {
            return
        } catch {
            pokemonState = .failed(Self.failure(error))
        }
    }

    private static func failure(_ error: any Error) -> SpeedFailure {
        SpeedFailure(error)
    }

    /// ポケモン一覧の読み込みに失敗していて、やり直せる(View は「もう一度」を出す。ADR-0534)。
    public var canRetryLoad: Bool {
        if case .failed = pokemonState { return true }
        return false
    }

    /// 失敗したポケモン一覧の読み込みをやり直す。読み込めているとき・読み込み中は何もしない。
    public func retryLoad() async {
        guard canRetryLoad else { return }
        await load()
    }

    // MARK: - 入力の操作(要求が要るものは、先行の要求を cancel して予約する。`settle()` で待てる)

    /// nil は未選択。
    public func selectSelfPokemon(id: String?) {
        changeInput { selfPokemonID = id }
    }

    public func selectOpponentPokemon(id: String?) {
        changeInput { opponentPokemonID = id }
    }

    /// `RankLimits.min...RankLimits.max` に収める。
    public func setRank(_ value: Int) {
        changeInput { rank = min(max(value, RankLimits.min), RankLimits.max) }
    }

    public func setScarf(_ value: Bool) {
        changeInput { scarf = value }
    }

    public func setTailwind(_ value: Bool) {
        changeInput { tailwind = value }
    }

    public func setParalysis(_ value: Bool) {
        changeInput { paralysis = value }
    }

    /// 「今の振りを入れる」。on にすると (SP 0・補正なし) から始め、off にすると nil に戻す。
    public func setCurrentEnabled(_ enabled: Bool) {
        changeInput {
            if !enabled {
                current = nil
            } else if current == nil {
                current = SpeedInvestment(sp: 0, nature: .neutral)
            }
        }
    }

    /// `0...SPLimits.maxPerStat` に収める。今の振りが無いときは何もしない。
    public func setCurrentSP(_ value: Int) {
        changeInput { current?.sp = min(max(value, 0), SPLimits.maxPerStat) }
    }

    /// 今の振りが無いときは何もしない。
    public func setCurrentNature(_ nature: SpeedNature) {
        changeInput { current?.nature = nature }
    }

    /// かんたん設定(無振り = SP 0・補正なし / 準速 = SP 32・補正なし / 最速 = SP 32・上昇)。今の振りが無ければ入れる。
    public func applyCurrentPreset(_ preset: SpeedMinimalPreset) {
        changeInput {
            switch preset {
            case .uninvested: current = SpeedInvestment(sp: 0, nature: .neutral)
            case .neutralMax: current = SpeedInvestment(sp: SPLimits.maxPerStat, nature: .neutral)
            case .max: current = SpeedInvestment(sp: SPLimits.maxPerStat, nature: .plus)
            }
        }
    }

    /// 想定のチップの切り替え。最後の 1 つ(カスタムを含めて)は外せない。
    public func togglePreset(_ id: SpeedPresetID) {
        if selectedPresets.contains(id) {
            guard assumptionCount > 1 else { return }
            changeInput { selectedPresets.remove(id) }
        } else {
            changeInput { selectedPresets.insert(id) }
        }
    }

    /// カスタムの想定を入れる / 外す(nil)。SP は 0...32、ランクは -6...+6 に収める。外すとき、チップが 0 個になるなら何もしない。
    public func setCustomAssumption(_ custom: SpeedCustomAssumption?) {
        if let custom {
            changeInput {
                customAssumption = SpeedCustomAssumption(
                    sp: min(max(custom.sp, 0), SPLimits.maxPerStat), nature: custom.nature,
                    rank: min(max(custom.rank, RankLimits.min), RankLimits.max), scarf: custom.scarf)
            }
        } else {
            guard !selectedPresets.isEmpty else { return }
            changeInput { customAssumption = nil }
        }
    }

    public func setOpponentTailwind(_ value: Bool) {
        changeInput { opponentTailwind = value }
    }

    public func setOpponentParalysis(_ value: Bool) {
        changeInput { opponentParalysis = value }
    }

    public func setTrickRoom(_ value: Bool) {
        changeInput { trickRoom = value }
    }

    public func setPokemonQuery(_ text: String) {
        pokemonQuery = text
    }

    /// 予約済みの要求がすべて終わる(または cancel される)まで待つ。テストと View の `.task` 用。
    public func settle() async {
        await matchupTask?.value
        await reachTask?.value
    }

    /// 画面破棄(`.onDisappear`)。待機中・送信済みの要求を cancel する。
    public func cancelPendingWork() {
        matchupRunner.cancel()
        reachRunner.cancel()
    }

    // MARK: - 要求の予約

    /// 入力を変え、matchup / reach の要求が変わったほうだけ取り直す(要求が変わらない入力では送らない)。
    private func changeInput(_ change: () -> Void) {
        let matchupBefore = matchupRequest
        let reachBefore = reachRequest
        change()
        let matchupAfter = matchupRequest
        let reachAfter = reachRequest
        if matchupAfter != matchupBefore { rescheduleMatchup(matchupAfter) }
        if reachAfter != reachBefore { rescheduleReach(reachAfter) }
    }

    private func rescheduleMatchup(_ request: SpeedMatchupRequest?) {
        matchupGeneration += 1
        let token = matchupGeneration
        guard let request else {
            matchupRunner.cancel()
            matchupTask = nil
            matchupState = .idle
            return
        }
        matchupState = .loading
        let service = self.service
        matchupTask = matchupRunner.schedule(debounce: debounce) { [weak self] in
            let result: Result<SpeedMatchup, any Error>
            do { result = .success(try await service.matchup(request)) } catch { result = .failure(error) }
            self?.applyMatchup(result, token: token)
        }
    }

    private func rescheduleReach(_ request: SpeedPositionRequest?) {
        reachGeneration += 1
        let token = reachGeneration
        guard let request else {
            reachRunner.cancel()
            reachTask = nil
            reachState = .idle
            return
        }
        reachState = .loading
        let service = self.service
        reachTask = reachRunner.schedule(debounce: debounce) { [weak self] in
            let result: Result<SpeedReach, any Error>
            do { result = .success(try await service.reach(request)) } catch { result = .failure(error) }
            self?.applyReach(result, token: token)
        }
    }

    private func applyMatchup(_ result: Result<SpeedMatchup, any Error>, token: Int) {
        guard token == matchupGeneration else { return }
        switch result {
        case .success(let matchup): matchupState = .loaded(matchup)
        case .failure(let error):
            if !(error is CancellationError) { matchupState = .failed(Self.failure(error)) }
        }
    }

    private func applyReach(_ result: Result<SpeedReach, any Error>, token: Int) {
        guard token == reachGeneration else { return }
        switch result {
        case .success(let reach): reachState = .loaded(reach)
        case .failure(let error):
            if !(error is CancellationError) { reachState = .failed(Self.failure(error)) }
        }
    }

    // MARK: - 派生

    /// 想定の数(チップ + カスタム)。
    private var assumptionCount: Int { selectedPresets.count + (customAssumption == nil ? 0 : 1) }

    /// 今の入力から作る matchup の要求。自分と相手が決まっていなければ nil。
    /// 想定は常に明示する(選んだプリセットを契約の順に並べ、カスタムを最後に置く)。
    public var matchupRequest: SpeedMatchupRequest? {
        guard let selfID = selfPokemonID, let opponentID = opponentPokemonID else { return nil }
        var assumptions = SpeedPresetID.allCases.filter { selectedPresets.contains($0) }.map { SpeedAssumption.preset($0) }
        if let customAssumption { assumptions.append(.custom(customAssumption)) }
        return SpeedMatchupRequest(
            own: .init(
                pokemonId: selfID, rank: rank, scarf: scarf, tailwind: tailwind, paralysis: paralysis, current: current),
            opponent: .init(
                pokemonId: opponentID, assumptions: assumptions, tailwind: opponentTailwind, paralysis: opponentParalysis),
            trickRoom: trickRoom)
    }

    /// 今の入力から作る reach の要求。自分が決まっていて、今の振りがあるときだけ(それ以外は nil)。
    /// 本文は custom(今の振り + ランク・スカーフ・自分の追い風・まひ)、`tableTailwind` は相手の追い風。
    public var reachRequest: SpeedPositionRequest? {
        guard let selfID = selfPokemonID, let current else { return nil }
        return SpeedPositionRequest(
            input: .custom(
                pokemonId: selfID, sp: current.sp, nature: current.nature, rank: rank, scarf: scarf, tailwind: tailwind,
                paralysis: paralysis),
            tableTailwind: opponentTailwind)
    }

    /// 連続技へ渡す値。`comboAvailable` が true で、自分と相手が決まっているときだけ。
    public var comboHandoff: SpeedComboHandoff? {
        guard comboAvailable, let selfID = selfPokemonID, let opponentID = opponentPokemonID else { return nil }
        return SpeedComboHandoff(
            attackerPokemonID: selfID, defenderPokemonID: opponentID, attackerSpeed: current, attackerRank: rank,
            attackerScarf: scarf)
    }

    public var selfPokemon: SpeedPokemon? { pokemon(id: selfPokemonID) }

    public var opponentPokemon: SpeedPokemon? { pokemon(id: opponentPokemonID) }

    private func pokemon(id: String?) -> SpeedPokemon? {
        guard let id, case .loaded(let all) = pokemonState else { return nil }
        return all.first { $0.pokemonId == id }
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

    /// 想定のチップを 1 つだけ残していて(カスタムなし)、それ以上外せないとき true(「想定は少なくとも1つ必要です」を出す)。
    public var assumptionMinimumNoticeVisible: Bool { customAssumption == nil && selectedPresets.count == 1 }

    /// 出力 1 のカード(matchup が読み込めているとき。そうでなければ空)。
    public var matchupCards: [SpeedMatchupCardDisplay] {
        guard case .loaded(let matchup) = matchupState else { return [] }
        return SpeedMatchupDisplayBuilder.cards(matchup)
    }

    /// 出力 2 の行(reach が読み込めているとき。そうでなければ空)。言い換えは今の `trickRoom` に従う。
    public var reachRows: [SpeedReachRowDisplay] {
        guard case .loaded(let reach) = reachState else { return [] }
        return SpeedMatchupDisplayBuilder.reachRows(reach, trickRoom: trickRoom)
    }

    /// 出力 1 の結論の 1 行(読み込めているときだけ)。
    public var matchupConclusion: String? {
        guard case .loaded(let matchup) = matchupState else { return nil }
        return SpeedMatchupDisplayBuilder.matchupConclusion(matchup)
    }

    /// 出力 2 の結論の 1 行(読み込めているときだけ)。
    public var reachConclusion: String? {
        SpeedMatchupDisplayBuilder.reachConclusion(reachRows)
    }

    /// 入力カード・出力の端の帯に使うタイプ ID(未選択は nil)。
    public var selfAccentTypeID: String? { SpeedMatchupDisplayBuilder.accentTypeID(selfPokemon) }
    public var opponentAccentTypeID: String? { SpeedMatchupDisplayBuilder.accentTypeID(opponentPokemon) }
}
