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
    /// 3つとも失敗(`master_unavailable`)。matchup・reach も失敗する。
    case allError
    /// matchup と reach だけ失敗(gateway が上流に届かないときの `upstream_unavailable`。ポケモン一覧は成功。I-speed-4c)。
    case matchupError

    /// 環境変数の値: `table-error` / `position-error` / `pokemon-error` / `all-error` / `matchup-error`。nil・未知の値は `.normal`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "matchup-error": self = .matchupError
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
        let (speed, pokemon) = try Self.resolveSpeed(request.input)
        // 位置は常に全6行の表が基準(契約)。
        let entries = Self.tiers(presets: SpeedPresetID.allCases, tailwind: request.tableTailwind)
        return SpeedPosition(
            speed: speed, pokemon: pokemon,
            faster: entries.filter { $0.speed > speed }.reduce(0) { $0 + $1.entries.count },
            slower: entries.filter { $0.speed < speed }.reduce(0) { $0 + $1.entries.count },
            tie: entries.first { $0.speed == speed }?.entries ?? [])
    }

    /// 要求の入力から自分の実数値(と、あればポケモン)を決める(position・reach で共用)。
    private static func resolveSpeed(_ input: SpeedPositionRequest.Input) throws -> (Int, SpeedPokemon?) {
        switch input {
        case .preset(let pokemonId, let preset, let scarf, let tailwind, let paralysis):
            let found = try find(pokemonId)
            let (sp, nature) = invest(preset)
            return (
                applyField(stat(base: found.baseSpeed, sp: sp, nature: nature), scarf: scarf, tailwind: tailwind, paralysis: paralysis),
                found
            )
        case .custom(let pokemonId, let sp, let nature, let rank, let scarf, let tailwind, let paralysis):
            let found = try find(pokemonId)
            return (
                applyField(
                    applyRank(stat(base: found.baseSpeed, sp: sp, nature: nature), rank: rank),
                    scarf: scarf, tailwind: tailwind, paralysis: paralysis),
                found
            )
        case .raw(let value, let pokemonId):
            guard value >= 1 else {
                throw PokeCalcError(code: "invalid_request", message: "モック: 実数値は1以上")
            }
            return (value, try pokemonId.map(find))
        }
    }

    // MARK: - matchup・reach(I-speed-4c。ADR-0610。簡易な式。固定する事実は MockSpeedMatchupTests)

    private static let defaultAssumptions: [SpeedAssumption] = [
        .preset(.uninvested), .preset(.neutralMax), .preset(.max), .preset(.maxScarf),
    ]

    public func matchup(_ request: SpeedMatchupRequest) async throws -> SpeedMatchup {
        if let failure = matchupFailure { throw failure }
        let own = try Self.find(request.own.pokemonId)
        let opponent = try Self.find(request.opponent.pokemonId)
        let ownSpeed = { (sp: Int, nature: SpeedNature) -> Int in
            Self.applyField(
                Self.applyRank(Self.stat(base: own.baseSpeed, sp: sp, nature: nature), rank: request.own.rank),
                scarf: request.own.scarf, tailwind: request.own.tailwind, paralysis: request.own.paralysis)
        }
        let selfSpeed = request.own.current.map { ownSpeed($0.sp, $0.nature) }
        let lines = (request.opponent.assumptions ?? Self.defaultAssumptions).map { assumption -> SpeedMatchupLine in
            let opponentSpeed = Self.assumedSpeed(assumption, of: opponent, request.opponent)
            let natures = SpeedNature.allCases.map { nature -> SpeedNatureLine in
                let sps = Array(0...SPLimits.maxPerStat)
                let first = sps.filter {
                    let own = ownSpeed($0, nature)
                    return request.trickRoom ? own < opponentSpeed : own > opponentSpeed
                }
                let tie = sps.filter { ownSpeed($0, nature) == opponentSpeed }
                return SpeedNatureLine(nature: nature, movesFirst: Self.spRange(first), tie: Self.spRange(tie))
            }
            return SpeedMatchupLine(
                assumption: assumption, opponentSpeed: opponentSpeed,
                current: selfSpeed.map { Self.turnOrder(own: $0, opponent: opponentSpeed, trickRoom: request.trickRoom) },
                natures: natures)
        }
        return SpeedMatchup(
            selfPokemon: own, opponent: opponent, trickRoom: request.trickRoom, selfSpeed: selfSpeed, lines: lines)
    }

    public func reach(_ request: SpeedPositionRequest) async throws -> SpeedReach {
        if let failure = matchupFailure { throw failure }
        let (speed, pokemon) = try Self.resolveSpeed(request.input)
        let lines = SpeedPresetID.allCases.map { preset -> SpeedReachLine in
            func presetSpeed(_ base: Int) -> Int { Self.rowSpeed(base: base, preset, tailwind: request.tableTailwind) }
            let bases = Array(1...Self.maxBaseSpeed)
            let slower = bases.filter { presetSpeed($0) < speed }
            let tie = bases.filter { presetSpeed($0) == speed }
            let faster = bases.filter { presetSpeed($0) > speed }
            let slowerPokemon = Self.pokemonList.filter { presetSpeed($0.baseSpeed) < speed }
            let tiePokemon = Self.pokemonList.filter { presetSpeed($0.baseSpeed) == speed }
            let fasterPokemon = Self.pokemonList.filter { presetSpeed($0.baseSpeed) > speed }
            return SpeedReachLine(
                preset: preset, slower: Self.baseRange(slower), tie: Self.baseRange(tie), faster: Self.baseRange(faster),
                slowerCount: slowerPokemon.count, tieCount: tiePokemon.count, fasterCount: fasterPokemon.count,
                fastestSlower: slowerPokemon.max { presetSpeed($0.baseSpeed) < presetSpeed($1.baseSpeed) },
                slowestFaster: fasterPokemon.min { presetSpeed($0.baseSpeed) < presetSpeed($1.baseSpeed) })
        }
        return SpeedReach(speed: speed, pokemon: pokemon, lines: lines)
    }

    /// matchup・reach だけの失敗(シナリオ)。
    private var matchupFailure: PokeCalcError? {
        switch scenario {
        case .allError: return Self.unavailable
        case .matchupError: return PokeCalcError(code: "upstream_unavailable", message: "モック: gateway が上流に届かない")
        default: return nil
        }
    }

    /// 契約の族(種族値)の上限。
    private static let maxBaseSpeed = 255

    private static func assumedSpeed(
        _ assumption: SpeedAssumption, of pokemon: SpeedPokemon, _ opponent: SpeedMatchupRequest.Opponent
    ) -> Int {
        switch assumption {
        case .preset(let preset):
            return applyField(
                rowSpeed(base: pokemon.baseSpeed, preset, tailwind: false),
                scarf: false, tailwind: opponent.tailwind, paralysis: opponent.paralysis)
        case .custom(let custom):
            return applyField(
                applyRank(stat(base: pokemon.baseSpeed, sp: custom.sp, nature: custom.nature), rank: custom.rank),
                scarf: custom.scarf, tailwind: opponent.tailwind, paralysis: opponent.paralysis)
        }
    }

    private static func turnOrder(own: Int, opponent: Int, trickRoom: Bool) -> SpeedTurnOrder {
        if own == opponent { return .speedTie }
        return (own > opponent) != trickRoom ? .selfFirst : .opponentFirst
    }

    /// 昇順の SP の列 → 閉区間(空なら nil)。
    private static func spRange(_ values: [Int]) -> SpeedSPRange? {
        guard let first = values.first, let last = values.last else { return nil }
        return SpeedSPRange(min: first, max: last)
    }

    private static func baseRange(_ values: [Int]) -> SpeedBaseSpeedRange? {
        guard let first = values.first, let last = values.last else { return nil }
        return SpeedBaseSpeedRange(min: first, max: last)
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
    private static func rowSpeed(base: Int, _ preset: SpeedPresetID, tailwind: Bool) -> Int {
        let maxStat = stat(base: base, sp: SPLimits.maxPerStat, nature: .plus)
        let value: Int
        switch preset {
        case .uninvested: value = stat(base: base, sp: 0, nature: .neutral)
        case .neutralMax: value = stat(base: base, sp: SPLimits.maxPerStat, nature: .neutral)
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
                bySpeed[rowSpeed(base: pokemon.baseSpeed, preset, tailwind: tailwind), default: []].append(entry)
            }
        }
        return bySpeed.keys.sorted(by: >).map { SpeedTier(speed: $0, entries: bySpeed[$0] ?? []) }
    }
}
