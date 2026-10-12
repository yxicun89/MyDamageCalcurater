// SpeedMatchupTypes: 「相手を選んで上を取る最低ライン」(matchup)と「これ以上」(reach)のドメインの型(I-speed-4c。ADR-0610)。
//
// 契約は services/speed/api/openapi.yaml 0.7.0(`POST /api/speed/v1/matchup`・`/reach`)。型は生成型に依存しない
// (`SpeedDomainTypes` と同じ方針。生成型 ↔ ドメインの写像は `APISpeedService` に閉じる)。
// 計算(相手の素早さ・SP の区間・族の区切り)はすべて speed-svc が決める。この層は応答を表示用に整えるだけ。
//
// spec-writer の足場: 型の形と本文の組み立て(省略規則)は APISpeedMatchupTests が固定する。

import Foundation

/// 素早さの SP と性格補正(openapi `SpeedInvestment`)。
public struct SpeedInvestment: Equatable, Hashable, Sendable {
    public var sp: Int
    public var nature: SpeedNature

    public init(sp: Int, nature: SpeedNature) {
        self.sp = sp
        self.nature = nature
    }
}

/// 相手の「カスタム」の想定(openapi `OpponentCustom`)。
public struct SpeedCustomAssumption: Equatable, Hashable, Sendable {
    public var sp: Int
    public var nature: SpeedNature
    public var rank: Int
    public var scarf: Bool

    public init(sp: Int, nature: SpeedNature, rank: Int = 0, scarf: Bool = false) {
        self.sp = sp
        self.nature = nature
        self.rank = rank
        self.scarf = scarf
    }
}

/// 相手の振りの想定(openapi `OpponentAssumption`。`preset` か `custom` のどちらか1つ)。
public enum SpeedAssumption: Equatable, Hashable, Sendable {
    case preset(SpeedPresetID)
    case custom(SpeedCustomAssumption)
}

/// `POST /api/speed/v1/matchup` の要求(openapi `MatchupRequest`)。
public struct SpeedMatchupRequest: Equatable, Sendable {
    /// 自分(openapi `MatchupSelf`)。ランク 0・場の効果 false・`current` なしは本文に載せない(省略 = 同じ意味)。
    public struct Own: Equatable, Sendable {
        public var pokemonId: String
        public var rank: Int
        public var scarf: Bool
        public var tailwind: Bool
        public var paralysis: Bool
        /// 今の振り。あるときだけ応答に `selfSpeed` と各想定の行動順(`current`)が付く。
        public var current: SpeedInvestment?

        public init(
            pokemonId: String, rank: Int = 0, scarf: Bool = false, tailwind: Bool = false, paralysis: Bool = false,
            current: SpeedInvestment? = nil
        ) {
            self.pokemonId = pokemonId
            self.rank = rank
            self.scarf = scarf
            self.tailwind = tailwind
            self.paralysis = paralysis
            self.current = current
        }
    }

    /// 相手(openapi `MatchupOpponent`)。
    public struct Opponent: Equatable, Sendable {
        public var pokemonId: String
        /// nil なら本文から省く(サーバーの既定 = 無振り・準速・最速・最速スカーフの4つ)。
        public var assumptions: [SpeedAssumption]?
        public var tailwind: Bool
        public var paralysis: Bool

        public init(pokemonId: String, assumptions: [SpeedAssumption]? = nil, tailwind: Bool = false, paralysis: Bool = false) {
            self.pokemonId = pokemonId
            self.assumptions = assumptions
            self.tailwind = tailwind
            self.paralysis = paralysis
        }
    }

    public var own: Own
    public var opponent: Opponent
    public var trickRoom: Bool

    public init(own: Own, opponent: Opponent, trickRoom: Bool = false) {
        self.own = own
        self.opponent = opponent
        self.trickRoom = trickRoom
    }
}

/// 素早さだけで比べた行動順(openapi `TurnOrder`。engine.CompareSpeed と同じ語彙)。
public enum SpeedTurnOrder: String, CaseIterable, Sendable, Hashable {
    case selfFirst = "self_first"
    case opponentFirst = "opponent_first"
    case speedTie = "speed_tie"
}

/// SP の閉区間(openapi `SpRange`)。
public struct SpeedSPRange: Equatable, Hashable, Sendable {
    public let min: Int
    public let max: Int

    public init(min: Int, max: Int) {
        self.min = min
        self.max = max
    }
}

/// 性格 1 つぶんの区間(openapi `NatureLine`)。`nil` の区間は「当てはまる SP が無い」。
public struct SpeedNatureLine: Equatable, Hashable, Sendable {
    public let nature: SpeedNature
    /// 上を取れる SP(トリックルームなし: [最低ライン, 32]、トリックルーム中: [0, 上限])。
    public let movesFirst: SpeedSPRange?
    public let tie: SpeedSPRange?

    public init(nature: SpeedNature, movesFirst: SpeedSPRange?, tie: SpeedSPRange?) {
        self.nature = nature
        self.movesFirst = movesFirst
        self.tie = tie
    }
}

/// 想定 1 つぶんの行(openapi `MatchupLine`)。
public struct SpeedMatchupLine: Equatable, Hashable, Sendable {
    public let assumption: SpeedAssumption
    public let opponentSpeed: Int
    /// `self.current` を送ったときだけ。
    public let current: SpeedTurnOrder?
    /// 常に 3 件(下降・補正なし・上昇の順)。
    public let natures: [SpeedNatureLine]

    public init(assumption: SpeedAssumption, opponentSpeed: Int, current: SpeedTurnOrder?, natures: [SpeedNatureLine]) {
        self.assumption = assumption
        self.opponentSpeed = opponentSpeed
        self.current = current
        self.natures = natures
    }
}

/// `POST /api/speed/v1/matchup` の応答(openapi `MatchupResponse`)。`lines` は要求の想定の順。
public struct SpeedMatchup: Equatable, Sendable {
    public let selfPokemon: SpeedPokemon
    public let opponent: SpeedPokemon
    public let trickRoom: Bool
    /// `self.current` を送ったときの自分の素早さ。
    public let selfSpeed: Int?
    public let lines: [SpeedMatchupLine]

    public init(selfPokemon: SpeedPokemon, opponent: SpeedPokemon, trickRoom: Bool, selfSpeed: Int?, lines: [SpeedMatchupLine]) {
        self.selfPokemon = selfPokemon
        self.opponent = opponent
        self.trickRoom = trickRoom
        self.selfSpeed = selfSpeed
        self.lines = lines
    }
}

/// 種族値(族)の閉区間(openapi `BaseSpeedRange`。1〜255)。
public struct SpeedBaseSpeedRange: Equatable, Hashable, Sendable {
    public let min: Int
    public let max: Int

    public init(min: Int, max: Int) {
        self.min = min
        self.max = max
    }
}

/// 表の 1 プリセットぶんの「これ以上」(openapi `ReachLine`)。`nil` の区間は「その族が無い」。
public struct SpeedReachLine: Equatable, Hashable, Sendable {
    public let preset: SpeedPresetID
    /// 自分より遅い族([1, a] →「a 族まで抜ける」)。
    public let slower: SpeedBaseSpeedRange?
    public let tie: SpeedBaseSpeedRange?
    /// 自分より速い族([c, 255] →「c 族以上に抜かれる」)。
    public let faster: SpeedBaseSpeedRange?
    public let slowerCount: Int
    public let tieCount: Int
    public let fasterCount: Int
    /// 遅い側で最も速い 1 体 / 速い側で最も遅い 1 体(使用可能なポケモンの中)。
    public let fastestSlower: SpeedPokemon?
    public let slowestFaster: SpeedPokemon?

    public init(
        preset: SpeedPresetID, slower: SpeedBaseSpeedRange?, tie: SpeedBaseSpeedRange?, faster: SpeedBaseSpeedRange?,
        slowerCount: Int, tieCount: Int, fasterCount: Int, fastestSlower: SpeedPokemon?, slowestFaster: SpeedPokemon?
    ) {
        self.preset = preset
        self.slower = slower
        self.tie = tie
        self.faster = faster
        self.slowerCount = slowerCount
        self.tieCount = tieCount
        self.fasterCount = fasterCount
        self.fastestSlower = fastestSlower
        self.slowestFaster = slowestFaster
    }
}

/// `POST /api/speed/v1/reach` の応答(openapi `ReachResponse`)。`lines` は常に 6 件(表のプリセットの順)。
public struct SpeedReach: Equatable, Sendable {
    public let speed: Int
    public let pokemon: SpeedPokemon?
    public let lines: [SpeedReachLine]

    public init(speed: Int, pokemon: SpeedPokemon?, lines: [SpeedReachLine]) {
        self.speed = speed
        self.pokemon = pokemon
        self.lines = lines
    }
}
