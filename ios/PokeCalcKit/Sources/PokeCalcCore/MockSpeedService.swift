// MockSpeedService: `SpeedService` のモック(P6-24。XCUITest・オフライン用。ADR-0503)。
// 架空データ(`nameJa` はすべて「テスト」で始まる)。挙動は起動時の環境変数 `POKECALC_MOCK_SPEED` で切り替える
// (`POKECALC_MOCK_DEVICE_DATA` と同じ流儀)。固定する事実は MockSpeedServiceTests と ADR-0503 §8 にある。
//
// 式は本物を写したものではない(整数の簡易な丸め)。モックは計算の正しさを保証しない。

public enum MockSpeedScenario: Equatable, Sendable {
    /// すべて成功(環境変数なし・未知の値の既定)。
    case normal
    /// 表だけ失敗(`master_unavailable`)。ポケモン一覧と位置は成功。
    case tableError
    /// 位置だけ失敗(`master_unavailable`)。
    case positionError
    /// ポケモン一覧だけ失敗(`master_unavailable`)。
    case pokemonError
    /// 3つとも失敗(`master_unavailable`)。
    case allError

    /// 環境変数の値: `table-error` / `position-error` / `pokemon-error` / `all-error`。nil・未知の値は `.normal`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "table-error": self = .tableError
        case "position-error": self = .positionError
        case "pokemon-error": self = .pokemonError
        case "all-error": self = .allError
        default: self = .normal
        }
    }
}

public struct MockSpeedService: SpeedService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_SPEED"

    private let scenario: MockSpeedScenario

    public init(scenario: MockSpeedScenario = .normal) {
        self.scenario = scenario
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockSpeedScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    // MARK: - 架空のマスタ(名前は「テスト」で始める。ADR-0002)

    private static let pokemonList: [SpeedPokemon] = [
        SpeedPokemon(pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100),
        SpeedPokemon(pokemonId: "9002-000", nameJa: "テストミズガメ", types: ["water"], baseSpeed: 40),
        SpeedPokemon(pokemonId: "9003-000", nameJa: "テストクサネコ", types: ["grass"], baseSpeed: 80),
        SpeedPokemon(pokemonId: "9004-000", nameJa: "テストデンキネズミ", types: ["electric"], baseSpeed: 130),
    ]

    private static let regulationID = "example"
    /// 性格補正の分子(分母は 10)。
    private static let natureNumerator: [SpeedNature: Int] = [.minus: 9, .neutral: 10, .plus: 11]

    // MARK: - SpeedService

    public func pokemon() async throws -> SpeedPokemonList {
        if scenario == .pokemonError || scenario == .allError { throw Self.unavailable }
        return SpeedPokemonList(regulationId: Self.regulationID, pokemon: Self.pokemonList)
    }

    public func table(presets: [SpeedPresetID]?, field: SpeedTableField) async throws -> SpeedTable {
        if scenario == .tableError || scenario == .allError { throw Self.unavailable }
        let used = SpeedPresetID.allCases.filter { presets?.contains($0) ?? true }
        let tiers = Self.tiers(presets: used, tailwind: field.tailwind)
        return SpeedTable(regulationId: Self.regulationID, presets: used, tiers: field.trickRoom ? tiers.reversed() : tiers)
    }

    public func position(_ request: SpeedPositionRequest) async throws -> SpeedPosition {
        if scenario == .positionError || scenario == .allError { throw Self.unavailable }
        let speed: Int
        let pokemon: SpeedPokemon?
        switch request.input {
        case .preset(let pokemonId, let preset, let scarf, let tailwind, let paralysis):
            let found = try Self.find(pokemonId)
            pokemon = found
            let (sp, nature) = Self.invest(preset)
            speed = Self.applyField(
                Self.stat(base: found.baseSpeed, sp: sp, nature: nature), scarf: scarf, tailwind: tailwind, paralysis: paralysis)
        case .custom(let pokemonId, let sp, let nature, let rank, let scarf, let tailwind, let paralysis):
            let found = try Self.find(pokemonId)
            pokemon = found
            speed = Self.applyField(
                Self.applyRank(Self.stat(base: found.baseSpeed, sp: sp, nature: nature), rank: rank),
                scarf: scarf, tailwind: tailwind, paralysis: paralysis)
        case .raw(let value, let pokemonId):
            guard value >= 1 else {
                throw PokeCalcError(code: "invalid_request", message: "モック: 実数値は1以上")
            }
            pokemon = try pokemonId.map(Self.find)
            speed = value
        }
        // 位置は常に全6行の表が基準(契約)。
        let entries = Self.tiers(presets: SpeedPresetID.allCases, tailwind: request.tableTailwind)
        return SpeedPosition(
            speed: speed, pokemon: pokemon,
            faster: entries.filter { $0.speed > speed }.reduce(0) { $0 + $1.entries.count },
            slower: entries.filter { $0.speed < speed }.reduce(0) { $0 + $1.entries.count },
            tie: entries.first { $0.speed == speed }?.entries ?? [])
    }

    // MARK: - 簡易な式

    private static let unavailable = PokeCalcError(code: "master_unavailable", message: "モック: 失敗のシナリオ")

    private static func find(_ pokemonId: String) throws -> SpeedPokemon {
        guard let found = pokemonList.first(where: { $0.pokemonId == pokemonId }) else {
            throw PokeCalcError(code: "unknown_pokemon", message: "モック: マスタに無いポケモン")
        }
        return found
    }

    private static func invest(_ preset: SpeedMinimalPreset) -> (sp: Int, nature: SpeedNature) {
        switch preset {
        case .uninvested: return (0, .neutral)
        case .neutralMax: return (SPLimits.maxPerStat, .neutral)
        case .max: return (SPLimits.maxPerStat, .plus)
        }
    }

    private static func stat(base: Int, sp: Int, nature: SpeedNature) -> Int {
        (base + 20 + sp) * (natureNumerator[nature] ?? 10) / 10
    }

    /// ランクの倍率(+n は (2+n)/2、-n は 2/(2+n))。
    private static func applyRank(_ value: Int, rank: Int) -> Int {
        rank >= 0 ? value * (2 + rank) / 2 : value * 2 / (2 - rank)
    }

    private static func applyField(_ value: Int, scarf: Bool, tailwind: Bool, paralysis: Bool) -> Int {
        var result = value
        if scarf { result = result * 3 / 2 }
        if tailwind { result *= 2 }
        if paralysis { result /= 2 }
        return result
    }

    /// 表の1行の実数値(スカーフ・+1 は同じ 1.5 倍、+2 は 2 倍)。
    private static func rowSpeed(_ pokemon: SpeedPokemon, _ preset: SpeedPresetID, tailwind: Bool) -> Int {
        let maxStat = stat(base: pokemon.baseSpeed, sp: SPLimits.maxPerStat, nature: .plus)
        let value: Int
        switch preset {
        case .uninvested: value = stat(base: pokemon.baseSpeed, sp: 0, nature: .neutral)
        case .neutralMax: value = stat(base: pokemon.baseSpeed, sp: SPLimits.maxPerStat, nature: .neutral)
        case .max: value = maxStat
        case .maxScarf, .maxPlus1: value = maxStat * 3 / 2
        case .maxPlus2: value = maxStat * 2
        }
        return tailwind ? value * 2 : value
    }

    /// 速い順の段。段の中は ポケモン ID → 調整(契約の順)。
    private static func tiers(presets: [SpeedPresetID], tailwind: Bool) -> [SpeedTier] {
        var bySpeed: [Int: [SpeedTableEntry]] = [:]
        for pokemon in pokemonList {
            for preset in presets {
                let entry = SpeedTableEntry(
                    pokemonId: pokemon.pokemonId, nameJa: pokemon.nameJa, types: pokemon.types,
                    baseSpeed: pokemon.baseSpeed, preset: preset)
                bySpeed[rowSpeed(pokemon, preset, tailwind: tailwind), default: []].append(entry)
            }
        }
        return bySpeed.keys.sorted(by: >).map { SpeedTier(speed: $0, entries: bySpeed[$0] ?? []) }
    }
}
