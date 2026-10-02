import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import PokeCalcBalanceAPI

// APIBalanceService: `BalanceService` を生成クライアント(`PokeCalcBalanceAPI.Client`)で実装する(P6-26。ADR-0505)。
//
// このファイルは PokeCalcBalanceAPI だけを import する(root の PokeCalcAPI・PokeCalcSpeedAPI・PokeCalcJudgeAPI と `Client`・`Components` が衝突する。ADR-0503 §1)。
// 生成型 ↔ ドメインの写像はここに閉じる。全操作に `X-Device-Id` / `X-Session-Id` を付ける。パス・ボディの形は APIBalanceServiceTests が固定する。
// balance は gateway の `/api/balance/*` から届く(接続先のホストは他の API と同じ `baseURL`。ADR-0505 §1)。
//

public struct APIBalanceService: BalanceService {
    private let client: Client
    private let identity: ClientIdentity

    public init(client: Client, identity: ClientIdentity) {
        self.client = client
        self.identity = identity
    }

    /// `URLSession` の既定 transport で組み立てる(アプリは `PokeCalcBalanceAPI` を直接 import しない)。
    public init(baseURL: URL, identity: ClientIdentity) {
        self.init(client: Client(serverURL: baseURL, transport: URLSessionTransport()), identity: identity)
    }

    public func analyzeTeamBalance(_ request: BalanceAnalyzeRequest) async throws -> BalanceAnalyzeResponse {
        let output = try await send {
            try await client.analyzeTeamBalance(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedAnalyzeRequest(request))))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainAnalyzeResponse(try ok.body.json)
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

    public func analyzeTeamCoverage(_ request: BalanceCoverageRequest) async throws -> BalanceCoverageResponse {
        let output = try await send {
            try await client.analyzeTeamCoverage(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedCoverageRequest(request))))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainCoverageResponse(try ok.body.json)
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

    // MARK: - 通信の失敗(APIJudgeService と同じ分類)

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

    /// 契約外のステータス。本文が `{code,message}` ならその code、読めなければ専用のコード。
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

    private static func generatedAnalyzeRequest(_ request: BalanceAnalyzeRequest) -> Components.Schemas.AnalyzeRequest {
        .init(
            members: request.members.map { .init(pokemonId: $0.pokemonId, abilityId: $0.abilityId) })
    }

    private static func generatedCoverageRequest(_ request: BalanceCoverageRequest) -> Components.Schemas.CoverageRequest {
        .init(members: request.members.map { .init(pokemonId: $0.pokemonId, moveIds: $0.moveIds) })
    }

    // MARK: - 生成型 → ドメイン(値は運ぶだけ。分類・倍率・集計を作り直さない)

    /// 契約の enum → ドメインの enum(rawValue が一致する前提。`BalanceContractSyncTests` が固定。合わなければ decode の失敗にする)。
    private static func mapped<Generated: RawRepresentable, Domain: RawRepresentable>(_ value: Generated) throws -> Domain
    where Generated.RawValue == String, Domain.RawValue == String {
        guard let domain = Domain(rawValue: value.rawValue) else {
            throw PokeCalcError(code: PokeCalcError.Code.decode, message: "unknown value \(value.rawValue)")
        }
        return domain
    }

    private static func domainAnalyzeResponse(_ response: Components.Schemas.AnalyzeResponse) throws -> BalanceAnalyzeResponse {
        let members = try response.members.map { member in
            BalanceMemberDefense(
                pokemonId: member.pokemonId, abilityId: member.abilityId?.value1,
                types: try member.types.map { try mapped($0) as PokeType },
                defense: try member.defense.map { entry in
                    BalanceDefenseEntry(
                        attackType: try mapped(entry.attackType), multiplier: entry.multiplier, category: try mapped(entry.category),
                        source: try mapped(entry.source), effect: try mapped(entry.effect))
                })
        }
        let summary = try response.teamSummary.map { entry in
            BalanceTeamSummaryEntry(
                attackType: try mapped(entry.attackType), weak: entry.weak, quadWeak: entry.quadWeak, resist: entry.resist,
                immune: entry.immune, neutral: entry.neutral)
        }
        return BalanceAnalyzeResponse(members: members, teamSummary: summary)
    }

    private static func domainCoverageResponse(_ response: Components.Schemas.CoverageResponse) throws -> BalanceCoverageResponse {
        let members = try response.members.map { member in
            BalanceMemberCoverage(
                pokemonId: member.pokemonId, moveIds: member.moveIds,
                attackTypes: try member.attackTypes.map { try mapped($0) as PokeType },
                coverage: try member.coverage.map { entry in
                    BalanceDefenseCoverageEntry(
                        defenseType: try mapped(entry.defenseType),
                        bestMultiplier: try entry.bestMultiplier.map { try mapped($0) as BalanceCoverageMultiplier },
                        effective: entry.effective, superEffective: entry.superEffective)
                })
        }
        let team = try response.teamCoverage.map { entry in
            BalanceTeamCoverageEntry(
                defenseType: try mapped(entry.defenseType),
                bestMultiplier: try entry.bestMultiplier.map { try mapped($0) as BalanceCoverageMultiplier },
                effectiveMembers: entry.effectiveMembers, superEffectiveMembers: entry.superEffectiveMembers)
        }
        return BalanceCoverageResponse(members: members, teamCoverage: team)
    }
}
