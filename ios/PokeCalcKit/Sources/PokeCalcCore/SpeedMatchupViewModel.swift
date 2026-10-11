// SpeedMatchupViewModel: 素早さ画面(相手を選んで上を取る最低ライン)の状態(I-speed-4c。ADR-0610 §4)。
//
// 入力(自分・相手・場)から matchup / reach の要求を作り、`SpeedService` の応答を `SpeedMatchupDisplayBuilder` で整える。
// 計算ロジックは持たない。状態の作りは `SpeedViewModel` と同じ流儀:
//  - `@MainActor @Observable`。入力のたびに `LatestTaskRunner` で先行の要求を cancel し、最新の世代の応答だけ反映する。
//  - ポケモン一覧・matchup・reach は互いに独立(片方の失敗がもう片方の表示を消さない)。
//  - `CancellationError` は画面のエラーにしない。
//
// このクラスは `SpeedViewModel`(表の画面)を置き換える。表の部品が画面から消えたら `SpeedViewModel` と表専用の
// 型・テストは削除する(ADR-0610 追記「iOS の画面」)。`SpeedFailure`・`SpeedLoadState`・`SpeedInput` は引き続き使う。
//
// spec-writer の足場: 公開 API の形(名前・型・引数)は SpeedMatchupViewModel*Tests が固定する。
// 本体は未実装(入力の操作は何もせず、状態は変わらない)。implementer が埋める。

import Foundation
import Observation

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
        // TODO(implementer)
    }

    // MARK: - 入力の操作(要求が要るものは、先行の要求を cancel して予約する。`settle()` で待てる)

    /// nil は未選択。
    public func selectSelfPokemon(id: String?) {}

    public func selectOpponentPokemon(id: String?) {}

    /// `RankLimits.min...RankLimits.max` に収める。
    public func setRank(_ value: Int) {}

    public func setScarf(_ value: Bool) {}

    public func setTailwind(_ value: Bool) {}

    public func setParalysis(_ value: Bool) {}

    /// 「今の振りを入れる」。on にすると (SP 0・補正なし) から始め、off にすると nil に戻す。
    public func setCurrentEnabled(_ enabled: Bool) {}

    /// `0...SPLimits.maxPerStat` に収める。今の振りが無いときは何もしない。
    public func setCurrentSP(_ value: Int) {}

    /// 今の振りが無いときは何もしない。
    public func setCurrentNature(_ nature: SpeedNature) {}

    /// かんたん設定(無振り = SP 0・補正なし / 準速 = SP 32・補正なし / 最速 = SP 32・上昇)。今の振りが無ければ入れる。
    public func applyCurrentPreset(_ preset: SpeedMinimalPreset) {}

    /// 想定のチップの切り替え。最後の 1 つ(カスタムを含めて)は外せない。
    public func togglePreset(_ id: SpeedPresetID) {}

    /// カスタムの想定を入れる / 外す(nil)。SP は 0...32、ランクは -6...+6 に収める。外すとき、チップが 0 個になるなら何もしない。
    public func setCustomAssumption(_ custom: SpeedCustomAssumption?) {}

    public func setOpponentTailwind(_ value: Bool) {}

    public func setOpponentParalysis(_ value: Bool) {}

    public func setTrickRoom(_ value: Bool) {}

    public func setPokemonQuery(_ text: String) {}

    /// 予約済みの要求がすべて終わる(または cancel される)まで待つ。テストと View の `.task` 用。
    public func settle() async {}

    /// 画面破棄(`.onDisappear`)。待機中・送信済みの要求を cancel する。
    public func cancelPendingWork() {}

    // MARK: - 派生

    /// 今の入力から作る matchup の要求。自分と相手が決まっていなければ nil。
    /// 想定は常に明示する(選んだプリセットを契約の順に並べ、カスタムを最後に置く)。
    public var matchupRequest: SpeedMatchupRequest? { nil }

    /// 今の入力から作る reach の要求。自分が決まっていて、今の振りがあるときだけ(それ以外は nil)。
    /// 本文は custom(今の振り + ランク・スカーフ・自分の追い風・まひ)、`tableTailwind` は相手の追い風。
    public var reachRequest: SpeedPositionRequest? { nil }

    /// 連続技へ渡す値。`comboAvailable` が true で、自分と相手が決まっているときだけ。
    public var comboHandoff: SpeedComboHandoff? { nil }

    public var selfPokemon: SpeedPokemon? { nil }

    public var opponentPokemon: SpeedPokemon? { nil }

    /// ピッカーに出す一覧(`pokemonQuery` で名前を絞る。前後空白を落とし、ひらがなはカタカナとして照合)。
    public var filteredPokemon: [SpeedPokemon] { [] }

    /// 想定のチップを 1 つだけ残しているとき true(「想定は少なくとも1つ必要です」を出す)。
    public var assumptionMinimumNoticeVisible: Bool { false }

    /// 出力 1 のカード(matchup が読み込めているとき。そうでなければ空)。
    public var matchupCards: [SpeedMatchupCardDisplay] { [] }

    /// 出力 2 の行(reach が読み込めているとき。そうでなければ空)。言い換えは今の `trickRoom` に従う。
    public var reachRows: [SpeedReachRowDisplay] { [] }
}
