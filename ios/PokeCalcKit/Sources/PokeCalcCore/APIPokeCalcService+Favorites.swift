import Foundation
import PokeCalcAPI

// お気に入り(ADR-0227・ADR-0511)。`GET/POST /api/record/favorites`・`DELETE /api/record/favorites/{favoriteId}`。
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

    public func addFavorite(label: String?, individual: Individual, calc: CalcHistoryCalc?) async throws -> FavoriteSaveResult {
        let body = Components.Schemas.FavoriteInput(
            label: FavoriteLabel.normalize(label), individual: Self.generatedIndividual(individual),
            calc: calc.map(Self.generatedFavoriteCalc))
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
            createdAt: favorite.createdAt, updatedAt: favorite.updatedAt, calc: favorite.calc.map(domainCalc))
    }

    /// 保存する `calc`(ADR-0524。Web の ADR-0333 §1 と同じく、既定のままの条件はキーごと省く)。
    /// 重複判定(ADR-0227)がサーバーの既定値補完後の JSON で行われるので、省いても同じ内容は同じお気に入りになる。
    private static func generatedFavoriteCalc(_ calc: CalcHistoryCalc) -> Components.Schemas.CalcRequest {
        .init(
            format: generatedFormat(calc.format),
            attacker: compactIndividual(calc.attacker),
            defender: compactIndividual(calc.defender),
            moveId: calc.moveId,
            field: compactField(calc.field),
            options: calc.critical ? .init(critical: true) : nil)
    }

    /// 既定の項目(天候・フィールドなし、壁なし)は省いた場。すべて既定なら nil。
    private static func compactField(_ field: FieldState) -> Components.Schemas.FieldState? {
        guard field != FieldState() else { return nil }
        return .init(
            weather: field.weather == .none ? nil : generatedWeather(field.weather),
            terrain: field.terrain == .none ? nil : generatedTerrain(field.terrain),
            attackerScreens: field.attackerScreens == Screens() ? nil : generatedScreens(field.attackerScreens),
            defenderScreens: field.defenderScreens == Screens() ? nil : generatedScreens(field.defenderScreens))
    }

    /// ランクがすべて 0 なら `ranks`、状態異常なしなら `status` を省いた個体。
    private static func compactIndividual(_ individual: Individual) -> Components.Schemas.Individual {
        var generated = generatedIndividual(individual)
        if individual.ranks == RankBlock() { generated.ranks = nil }
        if individual.status == .none { generated.status = nil }
        return generated
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
