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
        self.init(client: Client(serverURL: baseURL, transport: URLSessionTransport()), identity: identity)
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
            throw try Self.domainError(response.body.json)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json)
        case .undocumented(let status, let payload):
            throw await Self.undocumentedError(status: status, payload: payload)
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
            throw try Self.domainError(response.body.json)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json)
        case .undocumented(let status, let payload):
            throw await Self.undocumentedError(status: status, payload: payload)
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
            throw try Self.domainError(response.body.json)
        case .contentTooLarge(let response):
            throw try Self.domainError(response.body.json)
        case .unprocessableContent(let response):
            throw try Self.domainError(response.body.json)
        case .serviceUnavailable(let response):
            throw try Self.domainError(response.body.json)
        case .internalServerError(let response):
            throw try Self.domainError(response.body.json)
        case .undocumented(let status, let payload):
            throw await Self.undocumentedError(status: status, payload: payload)
        }
    }

    // MARK: - 通信の失敗(APIPokeCalcService と同じ分類)

    private func send<T>(_ operation: () async throws -> T) async throws -> T {
        do {
            return try await operation()
        } catch is CancellationError {
            throw CancellationError()
        } catch {
            if Self.isCancellation(error) { throw CancellationError() }
            if Self.isDecodingFailure(error) {
                throw PokeCalcError(code: PokeCalcError.Code.decode, message: "\(error)")
            }
            throw PokeCalcError(code: PokeCalcError.Code.transport, message: "\(error)")
        }
    }

    private static func isCancellation(_ error: any Error) -> Bool {
        guard let clientError = error as? ClientError else { return false }
        if clientError.underlyingError is CancellationError { return true }
        if let urlError = clientError.underlyingError as? URLError, urlError.code == .cancelled { return true }
        return false
    }

    private static func isDecodingFailure(_ error: any Error) -> Bool {
        guard let clientError = error as? ClientError else { return false }
        return clientError.underlyingError is DecodingError
    }

    private static func domainError(_ error: Components.Schemas._Error) -> PokeCalcError {
        PokeCalcError(code: error.code.rawValue, message: error.message)
    }

    /// 本文を読む上限(エラー本文は小さい。ゲートウェイの HTML などを無制限に読まない)。
    private static let undocumentedBodyLimit = 64 * 1024

    /// 契約外のステータス。本文が `{code,message}` ならその code(gateway の 404/405。ADR-0802)、読めなければ専用のコード。
    private static func undocumentedError(status: Int, payload: UndocumentedPayload) async -> PokeCalcError {
        struct Body: Decodable {
            let code: String
            let message: String
        }
        if let httpBody = payload.body,
            let data = try? await Data(collecting: httpBody, upTo: undocumentedBodyLimit),
            let body = try? JSONDecoder().decode(Body.self, from: data)
        {
            return PokeCalcError(code: body.code, message: body.message)
        }
        return PokeCalcError(code: PokeCalcError.Code.unexpectedStatus, message: "HTTP \(status)")
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
}
