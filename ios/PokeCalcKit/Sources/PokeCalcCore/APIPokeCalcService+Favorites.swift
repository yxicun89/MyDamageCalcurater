import Foundation
import PokeCalcAPI

// お気に入り(ADR-0227・ADR-0509)。`GET/POST /api/record/favorites`・`DELETE /api/record/favorites/{favoriteId}`。
// `PokeCalcService` とは別のプロトコル(`FrequentOpponentsService` と同じ理由)。
// 1回の呼び出しが1回の HTTP 要求。ヘッダは他の操作と同じ `X-Device-Id` / `X-Session-Id`。

extension APIPokeCalcService: FavoritesService {
    public func favorites() async throws -> [Favorite] {
        let output = try await send {
            try await client.listFavorites(headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID))
        }
        switch output {
        case .ok(let ok):
            return try ok.body.json.map(Self.domainFavorite)
        case .badRequest(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func addFavorite(label: String?, individual: Individual) async throws -> FavoriteSaveResult {
        let body = Components.Schemas.FavoriteInput(
            label: FavoriteLabel.normalize(label), individual: Self.generatedIndividual(individual))
        let output = try await send {
            try await client.createFavorite(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID), body: .json(body))
        }
        switch output {
        case .created(let created):
            return .created(Self.domainFavorite(try created.body.json))
        case .ok(let ok):
            return .alreadyPinned(Self.domainFavorite(try ok.body.json))
        case .badRequest(let error), .internalServerError(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func removeFavorite(id: String) async throws {
        let output = try await send {
            try await client.deleteFavorite(
                path: .init(favoriteId: id),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID))
        }
        switch output {
        case .noContent:
            return
        case .badRequest(let error), .notFound(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    // MARK: - 生成型 → ドメイン

    private static func domainFavorite(_ favorite: Components.Schemas.Favorite) -> Favorite {
        Favorite(
            id: favorite.id, label: favorite.label, individual: domainIndividual(favorite.individual),
            createdAt: favorite.createdAt, updatedAt: favorite.updatedAt)
    }

    /// 契約の `Individual` に `moveId` は無いので、常に nil(読まない)。
    static func domainIndividual(_ individual: Components.Schemas.Individual) -> Individual {
        let sp = individual.sp.value1
        let ranks = individual.ranks
        return Individual(
            speciesKey: individual.speciesKey,
            natureId: individual.natureId,
            sp: StatBlock(hp: sp.hp, atk: sp.atk, def: sp.def, spa: sp.spa, spd: sp.spd, spe: sp.spe),
            level: individual.level ?? fixedLevel,
            abilityId: individual.abilityId,
            itemId: individual.itemId,
            ranks: ranks.map { RankBlock(atk: $0.atk ?? 0, def: $0.def ?? 0, spa: $0.spa ?? 0, spd: $0.spd ?? 0, spe: $0.spe ?? 0) }
                ?? RankBlock(),
            teraType: individual.teraType.map { domainPokeType($0.value1) },
            status: individual.status.flatMap { StatusCondition(rawValue: $0.rawValue) } ?? .none
        )
    }
}
