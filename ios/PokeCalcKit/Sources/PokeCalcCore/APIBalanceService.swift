import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import PokeCalcBalanceAPI

/// `BalanceService` を生成クライアント(`PokeCalcBalanceAPI.Client`)で実装する(ADR-0415 §2・§3)。
///
/// 生成型 ↔ ドメイン(`Balance*`)の写像はこの1ファイルに閉じる。契約(`services/balance/api/openapi.yaml`)が
/// 変わったら直すのはここだけ。呼び先は gateway(`/api/balance/*`。ADR-0414)。全操作に `X-Device-Id` / `X-Session-Id` を付ける。
public struct APIBalanceService: BalanceService {
    private let client: Client
    private let identity: ClientIdentity

    public init(client: Client, identity: ClientIdentity) {
        self.client = client
        self.identity = identity
    }

    /// `URLSession` を使う既定の transport で組み立てる。`baseURL` は gateway の基点 URL(パスは足さない)。
    public init(baseURL: URL, identity: ClientIdentity) {
        self.init(client: Client(serverURL: baseURL, transport: URLSessionTransport()), identity: identity)
    }

    // MARK: - BalanceService

    public func analyze(members: [BalanceMemberInput]) async throws -> BalanceDefenseAnalysis {
        let body = Components.Schemas.AnalyzeRequest(
            members: members.map { .init(pokemonId: $0.pokemonId, abilityId: $0.abilityId) }
        )
        let output = try await send {
            try await client.analyzeTeamBalance(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(body)
            ))
        }
        switch output {
        case .ok(let ok): return try Self.domainDefense(ok.body.json)
        case .badRequest(let r): throw try Self.domainError(r.body.json)
        case .contentTooLarge(let r): throw try Self.domainError(r.body.json)
        case .unprocessableContent(let r): throw try Self.domainError(r.body.json)
        case .serviceUnavailable(let r): throw try Self.domainError(r.body.json)
        case .internalServerError(let r): throw try Self.domainError(r.body.json)
        case .undocumented(let status, _): throw Self.undocumentedError(status)
        }
    }

    public func coverage(members: [BalanceMemberInput]) async throws -> BalanceCoverageAnalysis {
        let body = Components.Schemas.CoverageRequest(
            members: members.map { .init(pokemonId: $0.pokemonId, moveIds: $0.moveIds) }
        )
        let output = try await send {
            try await client.analyzeTeamCoverage(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(body)
            ))
        }
        switch output {
        case .ok(let ok): return try Self.domainCoverage(ok.body.json)
        case .badRequest(let r): throw try Self.domainError(r.body.json)
        case .contentTooLarge(let r): throw try Self.domainError(r.body.json)
        case .unprocessableContent(let r): throw try Self.domainError(r.body.json)
        case .serviceUnavailable(let r): throw try Self.domainError(r.body.json)
        case .internalServerError(let r): throw try Self.domainError(r.body.json)
        case .undocumented(let status, _): throw Self.undocumentedError(status)
        }
    }

    // MARK: - 通信失敗 → PokeCalcError(`APIPokeCalcService` と同じ規則)

    /// 接続できない(`transport`)と応答をデコードできない(`decode`)を区別する。キャンセルは包まずに投げ直す。
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
        // URLSession 経由のキャンセルは URLError(.cancelled) として届く。
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

    /// 契約に無い HTTP ステータス。サーバーのコードが無いので、接続できないのと同じ扱い(`transport`)にする。
    private static func undocumentedError(_ status: Int) -> PokeCalcError {
        PokeCalcError(code: PokeCalcError.Code.transport, message: "unexpected HTTP status \(status)")
    }

    // MARK: - 応答 → ドメイン

    private static func domainDefense(_ response: Components.Schemas.AnalyzeResponse) throws -> BalanceDefenseAnalysis {
        BalanceDefenseAnalysis(
            members: try response.members.map { member in
                BalanceMemberDefense(
                    pokemonId: member.pokemonId, abilityId: member.abilityId?.value1,
                    types: try member.types.map(domainType),
                    defense: try member.defense.map { entry in
                        BalanceDefenseEntry(
                            attackType: try domainType(entry.attackType), multiplier: entry.multiplier,
                            category: try domainCategory(entry.category), source: try domainSource(entry.source),
                            effect: try domainEffect(entry.effect)
                        )
                    }
                )
            },
            teamSummary: try response.teamSummary.map { entry in
                BalanceTeamSummaryEntry(
                    attackType: try domainType(entry.attackType), weak: entry.weak, quadWeak: entry.quadWeak,
                    resist: entry.resist, immune: entry.immune, neutral: entry.neutral
                )
            }
        )
    }

    private static func domainCoverage(_ response: Components.Schemas.CoverageResponse) throws -> BalanceCoverageAnalysis {
        BalanceCoverageAnalysis(
            members: try response.members.map { member in
                BalanceMemberCoverage(
                    pokemonId: member.pokemonId, moveIds: member.moveIds,
                    attackTypes: try member.attackTypes.map(domainType),
                    coverage: try member.coverage.map { entry in
                        BalanceCoverageEntry(
                            defenseType: try domainType(entry.defenseType),
                            bestMultiplier: try entry.bestMultiplier.map(domainCoverageMultiplier),
                            effective: entry.effective, superEffective: entry.superEffective
                        )
                    }
                )
            },
            teamCoverage: try response.teamCoverage.map { entry in
                BalanceTeamCoverageEntry(
                    defenseType: try domainType(entry.defenseType),
                    bestMultiplier: try entry.bestMultiplier.map(domainCoverageMultiplier),
                    effectiveMembers: entry.effectiveMembers, superEffectiveMembers: entry.superEffectiveMembers
                )
            }
        )
    }

    // 生成の enum とドメインの enum は rawValue(契約の文字列)で写す。知らない値は decode エラー。

    private static func domainType(_ value: Components.Schemas.TypeId) throws -> PokeType {
        try mapped(PokeType(rawValue: value.rawValue), value.rawValue)
    }

    private static func domainCategory(_ value: Components.Schemas.DefenseCategory) throws -> BalanceDefenseCategory {
        try mapped(BalanceDefenseCategory(rawValue: value.rawValue), value.rawValue)
    }

    private static func domainSource(_ value: Components.Schemas.EffectSource) throws -> BalanceEffectSource {
        try mapped(BalanceEffectSource(rawValue: value.rawValue), value.rawValue)
    }

    private static func domainEffect(_ value: Components.Schemas.DefenseEffect) throws -> BalanceEffectKind {
        try mapped(BalanceEffectKind(rawValue: value.rawValue), value.rawValue)
    }

    private static func domainCoverageMultiplier(
        _ value: Components.Schemas.CoverageMultiplier
    ) throws -> BalanceCoverageMultiplier {
        try mapped(BalanceCoverageMultiplier(rawValue: value.rawValue), value.rawValue)
    }

    private static func mapped<T>(_ value: T?, _ raw: String) throws -> T {
        guard let value else {
            throw PokeCalcError(code: PokeCalcError.Code.decode, message: "unknown balance value: \(raw)")
        }
        return value
    }
}
