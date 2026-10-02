import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import PokeCalcJudgeAPI

// APIJudgeService: `JudgeService` を生成クライアント(`PokeCalcJudgeAPI.Client`)で実装する(P6-25。ADR-0504)。
//
// このファイルは PokeCalcJudgeAPI だけを import する(root の PokeCalcAPI・PokeCalcSpeedAPI も `Client`・`Components` を持つので、
// 同じファイルで複数を import すると名前が衝突する。ADR-0503 §1)。生成型 ↔ ドメインの写像はここに閉じる。
// 全操作に `X-Device-Id` / `X-Session-Id` を付ける。パス・ボディの形は APIJudgeServiceTests が固定する。
// judge は gateway ではなく自分の Ingress(`/api/judge` prefix。ADR-0700 §6-3)を持つが、接続先のホストは他の API と同じ `baseURL`。

public struct APIJudgeService: JudgeService {
    private let client: Client
    private let identity: ClientIdentity

    public init(client: Client, identity: ClientIdentity) {
        self.client = client
        self.identity = identity
    }

    /// `URLSession` の既定 transport で組み立てる(アプリは `PokeCalcJudgeAPI` を直接 import しない)。
    public init(baseURL: URL, identity: ClientIdentity) {
        self.init(client: Client(serverURL: baseURL, transport: URLSessionTransport()), identity: identity)
    }

    public func outspeedAndKo(_ request: JudgeRequest) async throws -> JudgeResponse {
        let output = try await send {
            try await client.outspeedAndKo(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedRequest(request))))
        }
        switch output {
        case .ok(let ok):
            return Self.domainResponse(try ok.body.json)
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

    // MARK: - 通信の失敗(APISpeedService と同じ分類)

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

    private static func generatedStats(_ sp: StatBlock) -> Components.Schemas.StatBlock {
        .init(hp: sp.hp, atk: sp.atk, def: sp.def, spa: sp.spa, spd: sp.spd, spe: sp.spe)
    }

    private static func generatedRanks(_ ranks: RankBlock) -> Components.Schemas.RankBlock {
        .init(atk: ranks.atk, def: ranks.def, spa: ranks.spa, spd: ranks.spd, spe: ranks.spe)
    }

    private static func generatedIndividual(_ value: JudgeIndividual) -> Components.Schemas.Individual {
        .init(
            speciesKey: value.speciesKey, natureId: value.natureId, sp: generatedStats(value.sp),
            ranks: value.ranks.map(generatedRanks), abilityId: value.abilityId, itemId: value.itemId)
    }

    private static func generatedDefender(_ value: JudgeDefender) -> Components.Schemas.DefenderCandidate {
        let individual = value.individual
        return .init(
            speciesKey: individual.speciesKey, natureId: individual.natureId, sp: generatedStats(individual.sp),
            ranks: individual.ranks.map(generatedRanks), abilityId: individual.abilityId, itemId: individual.itemId,
            moveId: .init(value1: value.moveId))
    }

    private static func generatedRequest(_ request: JudgeRequest) -> Components.Schemas.OutspeedAndKoRequest {
        .init(
            format: request.format == .single ? .single : .double,
            attacker: .init(value1: generatedIndividual(request.attacker)),
            defenders: request.defenders.map(generatedDefender), moveId: .init(value1: request.moveId),
            speedField: request.speedField.map {
                .init(trickRoom: $0.trickRoom, attackerTailwind: $0.attackerTailwind, defenderTailwind: $0.defenderTailwind)
            })
    }

    // MARK: - 生成型 → ドメイン

    private static func domainKO(_ ko: Components.Schemas.KOChance) -> JudgeKOChance {
        JudgeKOChance(hits: ko.hits, guaranteed: ko.guaranteed, displayChancePercent: ko.displayChancePercent)
    }

    private static func domainMark(_ mark: Components.Schemas.UnsupportedMark) -> UnsupportedMark {
        UnsupportedMark(
            target: UnsupportedTarget(contractValue: mark.target), reason: UnsupportedReason(contractValue: mark.reason),
            id: mark.id)
    }

    private static func domainMatchup(_ value: Components.Schemas.Matchup) -> JudgeMatchup {
        JudgeMatchup(
            defenderIndex: value.defenderIndex, outspeeds: value.outspeeds, speedTie: value.speedTie,
            attackerSpeed: value.attackerSpeed, defenderSpeed: value.defenderSpeed,
            attackerMovePriority: value.attackerMovePriority, defenderMovePriority: value.defenderMovePriority,
            attackerMovesFirst: value.attackerMovesFirst, turnOrderTie: value.turnOrderTie,
            attackerKo: domainKO(value.attackerKo.value1), defenderKo: domainKO(value.defenderKo.value1),
            attackerKoUnsupported: value.attackerKoUnsupported.map(domainMark),
            defenderKoUnsupported: value.defenderKoUnsupported.map(domainMark))
    }

    private static func domainResponse(_ response: Components.Schemas.OutspeedAndKoResponse) -> JudgeResponse {
        JudgeResponse(matchups: response.matchups.map(domainMatchup))
    }
}
