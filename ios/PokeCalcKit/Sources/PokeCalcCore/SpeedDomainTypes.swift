// SpeedDomainTypes: 素早さ比較(P6-24。ADR-0503)のドメインの型と、画面が依存する境界 `SpeedService`。
//
// 契約は services/speed/api/openapi.yaml(生成物は PokeCalcSpeedAPI)。ここの型は生成型に依存しない
// (`PokeCalcService` のドメイン型と同じ方針。ADR-0500 §3)。生成型 ↔ ドメインの写像は `APISpeedService` に閉じる。
// 計算(素早さの実数値・並び・位置)はすべて speed-svc が決める。この層は応答を表示用に整えるだけ。
//
// spec-writer の足場: 型の形(名前・case 名・ID の文字列)は SpeedDomainTypesTests が固定する。
// 振る舞いのある部分(`SpeedLabels`)は文言そのものをテストが固定している。

import Foundation

/// 表の行の調整(openapi `PresetId`)。`allCases` は契約の順(ADR-0601 §2)。`rawValue` は契約の ID。
public enum SpeedPresetID: String, CaseIterable, Sendable, Hashable {
    case uninvested
    case neutralMax = "neutral-max"
    case max
    case maxScarf = "max-scarf"
    case maxPlus1 = "max-plus1"
    case maxPlus2 = "max-plus2"
}

/// 自分のポケモンで選べる調整(openapi `MinimalPresetId`。スカーフ無しの3つ。ADR-0602 §5)。
public enum SpeedMinimalPreset: String, CaseIterable, Sendable, Hashable {
    case uninvested
    case neutralMax = "neutral-max"
    case max
}

/// 性格の素早さへの効果(openapi `NatureId`)。
public enum SpeedNature: String, CaseIterable, Sendable, Hashable {
    case minus
    case neutral
    case plus
}

/// 自分のポケモンの入力方法(openapi `PositionRequest.mode`)。
public enum SpeedInputMode: String, CaseIterable, Sendable, Hashable {
    case preset
    case custom
    case raw
}

/// 素早さ比較に出るポケモン(openapi `SpeedPokemon`)。pokedex の `SpeciesSummary` とは別物
/// (speed サービスのマスタの写し。ピッカーはこの一覧だけを使う。ADR-0503)。
public struct SpeedPokemon: Equatable, Hashable, Sendable, Identifiable {
    public let pokemonId: String
    public let nameJa: String
    /// 1〜2個の小文字のタイプ ID(表示用)。
    public let types: [String]
    public let baseSpeed: Int

    public var id: String { pokemonId }

    public init(pokemonId: String, nameJa: String, types: [String], baseSpeed: Int) {
        self.pokemonId = pokemonId
        self.nameJa = nameJa
        self.types = types
        self.baseSpeed = baseSpeed
    }
}

/// `GET /api/speed/v1/pokemon` の応答。
public struct SpeedPokemonList: Equatable, Sendable {
    public let regulationId: String
    public let pokemon: [SpeedPokemon]

    public init(regulationId: String, pokemon: [SpeedPokemon]) {
        self.regulationId = regulationId
        self.pokemon = pokemon
    }
}

/// 表の1行(openapi `SpeedTableEntry`)。ポケモン × 調整。
public struct SpeedTableEntry: Equatable, Hashable, Sendable, Identifiable {
    public let pokemonId: String
    public let nameJa: String
    public let types: [String]
    public let baseSpeed: Int
    public let preset: SpeedPresetID

    public var id: String { "\(pokemonId)-\(preset.rawValue)" }

    public init(pokemonId: String, nameJa: String, types: [String], baseSpeed: Int, preset: SpeedPresetID) {
        self.pokemonId = pokemonId
        self.nameJa = nameJa
        self.types = types
        self.baseSpeed = baseSpeed
        self.preset = preset
    }
}

/// 同じ実数値の行のまとまり(openapi `SpeedTier`)。`entries` が2行以上なら同速。
public struct SpeedTier: Equatable, Sendable {
    public let speed: Int
    public let entries: [SpeedTableEntry]

    public init(speed: Int, entries: [SpeedTableEntry]) {
        self.speed = speed
        self.entries = entries
    }
}

/// `GET /api/speed/v1/table` の応答。`tiers` は速い順(トリックルームのときは遅い順)。応答のまま持つ。
public struct SpeedTable: Equatable, Sendable {
    public let regulationId: String
    /// 実際に使われた調整(契約の順)。
    public let presets: [SpeedPresetID]
    public let tiers: [SpeedTier]

    public init(regulationId: String, presets: [SpeedPresetID], tiers: [SpeedTier]) {
        self.regulationId = regulationId
        self.presets = presets
        self.tiers = tiers
    }
}

/// 表の場の状態(`GET /api/speed/v1/table` の `tailwind`・`trickRoom`。ADR-0607)。
/// 既定の false はクエリに載せない(省略 = false と同じ応答。Web と同じ)。
public struct SpeedTableField: Equatable, Sendable {
    /// 相手側(表の全行)の追い風。
    public var tailwind: Bool
    public var trickRoom: Bool

    public init(tailwind: Bool = false, trickRoom: Bool = false) {
        self.tailwind = tailwind
        self.trickRoom = trickRoom
    }
}

/// 自分のポケモンの位置の要求(openapi `PositionRequest`)。mode ごとに送る項目が違うので enum にする
/// (mode に要らない項目は型の上で持てない。契約上 400 になるため)。
public struct SpeedPositionRequest: Equatable, Sendable {
    public enum Input: Equatable, Sendable {
        case preset(pokemonId: String, preset: SpeedMinimalPreset, scarf: Bool, tailwind: Bool, paralysis: Bool)
        case custom(
            pokemonId: String, sp: Int, nature: SpeedNature, rank: Int,
            scarf: Bool, tailwind: Bool, paralysis: Bool
        )
        /// 実数値そのもの。`pokemonId` は表示用(種族値は使われない)。追い風・まひは持てない。
        case raw(value: Int, pokemonId: String?)
    }

    public var input: Input
    /// 表の全行の側の追い風(どの mode でも送れる)。
    public var tableTailwind: Bool

    public init(input: Input, tableTailwind: Bool = false) {
        self.input = input
        self.tableTailwind = tableTailwind
    }
}

/// `POST /api/speed/v1/position` の応答(openapi `PositionResponse`)。
public struct SpeedPosition: Equatable, Sendable {
    /// 自分の実数値。
    public let speed: Int
    /// 要求に pokemonId があったときだけ。
    public let pokemon: SpeedPokemon?
    /// 自分より速い行の数(トリックルームの影響を受けない。常に全6行の表が基準)。
    public let faster: Int
    public let slower: Int
    /// 同じ実数値の行(空なら同速なし)。
    public let tie: [SpeedTableEntry]

    public init(speed: Int, pokemon: SpeedPokemon?, faster: Int, slower: Int, tie: [SpeedTableEntry]) {
        self.speed = speed
        self.pokemon = pokemon
        self.faster = faster
        self.slower = slower
        self.tie = tie
    }
}

/// 画面が依存する素早さの境界(`PokeCalcService` とは別のプロトコル。`DeviceDataService` と同じ流儀。
/// 絶対ルール5: speed の失敗が計算・構築に影響しないよう、混ぜない)。
/// 実装は `APISpeedService`(gateway 経由)と `MockSpeedService`。失敗は `PokeCalcError`
/// (`code` はサーバーの語彙をそのまま運ぶ。通信失敗は `PokeCalcError.Code.transport`)。
public protocol SpeedService: Sendable {
    /// 使用可能なポケモンの一覧(速度の種族値つき)。
    func pokemon() async throws -> SpeedPokemonList
    /// 速い順の表。`presets` が nil なら全6行(クエリを省く)。指定は1つ以上。
    func table(presets: [SpeedPresetID]?, field: SpeedTableField) async throws -> SpeedTable
    /// 自分のポケモンの実数値と、表の中の位置。
    func position(_ request: SpeedPositionRequest) async throws -> SpeedPosition
}
