import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import PokeCalcSpeedAPI

// APISpeedService: `SpeedService` を生成クライアント(`PokeCalcSpeedAPI.Client`)で実装する(P6-24。ADR-0503)。
//
// このファイルは PokeCalcSpeedAPI だけを import する(root の PokeCalcAPI も `Client`・`Components` を持つので、
// 同じファイルで両方を import すると名前が衝突する)。生成型 ↔ ドメインの写像はここに閉じる。
// 全操作に `X-Device-Id` / `X-Session-Id` を付ける。クエリ・ボディの形は APISpeedServiceTests が固定する。
//

public struct APISpeedService: SpeedService {
    private let client: Client
    private let identity: ClientIdentity

    public init(client: Client, identity: ClientIdentity) {
        self.client = client
        self.identity = identity
    }

    /// `URLSession` の既定 transport で組み立てる(アプリは `PokeCalcSpeedAPI` を直接 import しない)。
    /// `baseURL` は gateway(`/api/speed/*` を speed-svc へ中継する。root の API と同じ接続先)。
    public init(baseURL: URL, identity: ClientIdentity) {
        self.init(client: Client(serverURL: baseURL, transport: ClientFailureMapping.makeTransport()), identity: identity)
    }

    // MARK: - SpeedService

    public func pokemon() async throws -> SpeedPokemonList {
        let output = try await send {
            try await client.listPokemon(.init(headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainPokemonList(ok.body.json)
        case .badRequest(let response):
            throw try Self.domainError(response.body.json, status: 400)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json, status: 503)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json, status: 500)
        case .undocumented(let status, let payload):
            throw await ClientFailureMapping.undocumentedError(status: status, payload: payload)
        }
    }

    public func table(presets: [SpeedPresetID]?, field: SpeedTableField) async throws -> SpeedTable {
        // 既定の false は載せない(省略 = false と同じ応答)。presets は explode: false の 1 パラメータ(生成クライアントが結合する)。
        let query = Operations.GetSpeedTable.Input.Query(
            presets: presets?.compactMap { Components.Schemas.PresetId(rawValue: $0.rawValue) },
            tailwind: field.tailwind ? true : nil,
            trickRoom: field.trickRoom ? true : nil)
        let output = try await send {
            try await client.getSpeedTable(.init(
                query: query, headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainTable(ok.body.json)
        case .badRequest(let response):
            throw try Self.domainError(response.body.json, status: 400)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json, status: 503)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json, status: 500)
        case .undocumented(let status, let payload):
            throw await ClientFailureMapping.undocumentedError(status: status, payload: payload)
        }
    }

    public func position(_ request: SpeedPositionRequest) async throws -> SpeedPosition {
        let output = try await send {
            try await client.getSpeedPosition(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedPositionRequest(request))))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainPosition(ok.body.json)
        case .badRequest(let response):
            throw try Self.domainError(response.body.json, status: 400)
        case .contentTooLarge(let response):
            throw try Self.domainError(response.body.json, status: 413)
        case .unprocessableContent(let response):
            throw try Self.domainError(response.body.json, status: 422)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json, status: 503)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json, status: 500)
        case .undocumented(let status, let payload):
            throw await ClientFailureMapping.undocumentedError(status: status, payload: payload)
        }
    }

    // MARK: - matchup・reach(I-speed-4c。ADR-0610 §2)

    public func matchup(_ request: SpeedMatchupRequest) async throws -> SpeedMatchup {
        let output = try await send {
            try await client.getSpeedMatchup(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedMatchupRequest(request))))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainMatchup(ok.body.json)
        case .badRequest(let response):
            throw try Self.domainError(response.body.json, status: 400)
        case .contentTooLarge(let response):
            throw try Self.domainError(response.body.json, status: 413)
        case .unprocessableContent(let response):
            throw try Self.domainError(response.body.json, status: 422)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json, status: 503)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json, status: 500)
        case .undocumented(let status, let payload):
            throw await ClientFailureMapping.undocumentedError(status: status, payload: payload)
        }
    }

    /// 本文は `position` と同じ(`generatedPositionRequest` を共有)。
    public func reach(_ request: SpeedPositionRequest) async throws -> SpeedReach {
        let output = try await send {
            try await client.getSpeedReach(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedPositionRequest(request))))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainReach(ok.body.json)
        case .badRequest(let response):
            throw try Self.domainError(response.body.json, status: 400)
        case .contentTooLarge(let response):
            throw try Self.domainError(response.body.json, status: 413)
        case .unprocessableContent(let response):
            throw try Self.domainError(response.body.json, status: 422)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json, status: 503)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json, status: 500)
        case .undocumented(let status, let payload):
            throw await ClientFailureMapping.undocumentedError(status: status, payload: payload)
        }
    }

    // MARK: - 通信の失敗(APIPokeCalcService と同じ分類)

    private func send<T>(_ operation: () async throws -> T) async throws -> T {
        do {
            return try await operation()
        } catch is CancellationError {
            throw CancellationError()
        } catch {
            if ClientFailureMapping.isCancellation(error) { throw CancellationError() }
            throw ClientFailureMapping.pokeCalcError(from: error)
        }
    }

    private static func domainError(_ error: Components.Schemas._Error, status: Int? = nil) -> PokeCalcError {
        PokeCalcError(code: error.code.rawValue, message: error.message, statusCode: status)
    }

    // MARK: - ドメイン → 生成型

    private static func generatedPositionRequest(_ request: SpeedPositionRequest) -> Components.Schemas.PositionRequest {
        let tableTailwind: Bool? = request.tableTailwind ? true : nil
        switch request.input {
        case .preset(let pokemonId, let preset, let scarf, let tailwind, let paralysis):
            return .init(
                mode: .preset, pokemonId: pokemonId, preset: Components.Schemas.MinimalPresetId(rawValue: preset.rawValue),
                scarf: scarf, tailwind: tailwind ? true : nil, paralysis: paralysis ? true : nil,
                tableTailwind: tableTailwind)
        case .custom(let pokemonId, let sp, let nature, let rank, let scarf, let tailwind, let paralysis):
            return .init(
                mode: .custom, pokemonId: pokemonId, scarf: scarf, tailwind: tailwind ? true : nil,
                paralysis: paralysis ? true : nil, tableTailwind: tableTailwind, sp: sp,
                nature: Components.Schemas.NatureId(rawValue: nature.rawValue), rank: rank)
        case .raw(let value, let pokemonId):
            return .init(mode: .raw, pokemonId: pokemonId, tableTailwind: tableTailwind, value: value)
        }
    }

    /// 既定の値(ランク 0・場の効果 false・今の振りなし・想定なし)は載せない(省略 = 既定と同じ意味)。
    private static func generatedMatchupRequest(_ request: SpeedMatchupRequest) -> Components.Schemas.MatchupRequest {
        let own = request.own
        let opponent = request.opponent
        return .init(
            _self: .init(
                pokemonId: own.pokemonId, rank: own.rank != 0 ? own.rank : nil, scarf: own.scarf ? true : nil,
                tailwind: own.tailwind ? true : nil, paralysis: own.paralysis ? true : nil,
                current: own.current.map {
                    .init(sp: $0.sp, nature: Components.Schemas.NatureId(rawValue: $0.nature.rawValue) ?? .neutral)
                }),
            opponent: .init(
                pokemonId: opponent.pokemonId, assumptions: opponent.assumptions?.map(generatedAssumption),
                tailwind: opponent.tailwind ? true : nil, paralysis: opponent.paralysis ? true : nil),
            trickRoom: request.trickRoom ? true : nil)
    }

    private static func generatedAssumption(_ assumption: SpeedAssumption) -> Components.Schemas.OpponentAssumption {
        switch assumption {
        case .preset(let preset):
            return .init(preset: Components.Schemas.PresetId(rawValue: preset.rawValue))
        case .custom(let custom):
            return .init(
                custom: .init(
                    sp: custom.sp, nature: Components.Schemas.NatureId(rawValue: custom.nature.rawValue) ?? .neutral,
                    rank: custom.rank != 0 ? custom.rank : nil, scarf: custom.scarf ? true : nil))
        }
    }

    // MARK: - 生成型 → ドメイン

    private static func domainPokemon(_ pokemon: Components.Schemas.SpeedPokemon) -> SpeedPokemon {
        SpeedPokemon(pokemonId: pokemon.pokemonId, nameJa: pokemon.nameJa, types: pokemon.types, baseSpeed: pokemon.baseSpeed)
    }

    private static func domainPokemonList(_ list: Components.Schemas.PokemonListResponse) -> SpeedPokemonList {
        SpeedPokemonList(regulationId: list.regulationId, pokemon: list.pokemon.map(domainPokemon))
    }

    /// 契約の `PresetId` はドメインの `SpeedPresetID` と同じ値(SpeedContractSyncTests が固定)。
    private static func domainPreset(_ id: Components.Schemas.PresetId) throws -> SpeedPresetID {
        guard let preset = SpeedPresetID(rawValue: id.rawValue) else {
            throw PokeCalcError(code: PokeCalcError.Code.decode, message: "未知の調整: \(id.rawValue)")
        }
        return preset
    }

    private static func domainEntry(_ entry: Components.Schemas.SpeedTableEntry) throws -> SpeedTableEntry {
        SpeedTableEntry(
            pokemonId: entry.pokemonId, nameJa: entry.nameJa, types: entry.types, baseSpeed: entry.baseSpeed,
            preset: try domainPreset(entry.preset))
    }

    private static func domainTable(_ table: Components.Schemas.TableResponse) throws -> SpeedTable {
        SpeedTable(
            regulationId: table.regulationId, presets: try table.presets.map(domainPreset),
            tiers: try table.tiers.map { SpeedTier(speed: $0.speed, entries: try $0.entries.map(domainEntry)) })
    }

    private static func domainPosition(_ position: Components.Schemas.PositionResponse) throws -> SpeedPosition {
        SpeedPosition(
            speed: position.speed, pokemon: position.pokemon.map(domainPokemon), faster: position.faster,
            slower: position.slower, tie: try position.tie.map(domainEntry))
    }

    private static func domainNature(_ id: Components.Schemas.NatureId) throws -> SpeedNature {
        guard let nature = SpeedNature(rawValue: id.rawValue) else {
            throw PokeCalcError(code: PokeCalcError.Code.decode, message: "未知の性格: \(id.rawValue)")
        }
        return nature
    }

    private static func domainAssumption(_ assumption: Components.Schemas.OpponentAssumption) throws -> SpeedAssumption {
        if let preset = assumption.preset { return .preset(try domainPreset(preset)) }
        if let custom = assumption.custom {
            return .custom(
                SpeedCustomAssumption(
                    sp: custom.sp, nature: try domainNature(custom.nature), rank: custom.rank ?? 0, scarf: custom.scarf ?? false))
        }
        throw PokeCalcError(code: PokeCalcError.Code.decode, message: "想定が preset でも custom でもない")
    }

    private static func domainSPRange(_ range: Components.Schemas.SpRange?) -> SpeedSPRange? {
        range.map { SpeedSPRange(min: $0.min, max: $0.max) }
    }

    private static func domainBaseRange(_ range: Components.Schemas.BaseSpeedRange?) -> SpeedBaseSpeedRange? {
        range.map { SpeedBaseSpeedRange(min: $0.min, max: $0.max) }
    }

    private static func domainMatchup(_ response: Components.Schemas.MatchupResponse) throws -> SpeedMatchup {
        SpeedMatchup(
            selfPokemon: domainPokemon(response._self), opponent: domainPokemon(response.opponent),
            trickRoom: response.trickRoom, selfSpeed: response.selfSpeed,
            lines: try response.lines.map { line in
                SpeedMatchupLine(
                    assumption: try domainAssumption(line.assumption), opponentSpeed: line.opponentSpeed,
                    current: line.current.flatMap { SpeedTurnOrder(rawValue: $0.rawValue) },
                    natures: try line.natures.map {
                        SpeedNatureLine(
                            nature: try domainNature($0.nature), movesFirst: domainSPRange($0.movesFirst),
                            tie: domainSPRange($0.tie))
                    })
            })
    }

    private static func domainReach(_ response: Components.Schemas.ReachResponse) throws -> SpeedReach {
        SpeedReach(
            speed: response.speed, pokemon: response.pokemon.map(domainPokemon),
            lines: try response.lines.map { line in
                SpeedReachLine(
                    preset: try domainPreset(line.preset), slower: domainBaseRange(line.slower),
                    tie: domainBaseRange(line.tie), faster: domainBaseRange(line.faster),
                    slowerCount: line.slowerCount, tieCount: line.tieCount, fasterCount: line.fasterCount,
                    fastestSlower: line.fastestSlower.map(domainPokemon), slowestFaster: line.slowestFaster.map(domainPokemon))
            })
    }
}
