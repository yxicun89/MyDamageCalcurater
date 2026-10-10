import Foundation
import PokeCalcAPI

// 計算履歴(ADR-0230・ADR-0519)。`GET /api/record/calc-history`(`listCalcHistory`)。
// `PokeCalcService` とは別のプロトコル(`FavoritesService` と同じ理由)。1回の呼び出しが1回の HTTP 要求。
// ヘッダは他の操作と同じ `X-Device-Id` / `X-Session-Id`。`cursor` は前のページの `nextCursor` をそのまま載せる。

extension APIPokeCalcService: CalcHistoryService {
    public func calcHistory(limit: Int, cursor: String?) async throws -> CalcHistoryPage {
        let output = try await send {
            try await client.listCalcHistory(
                query: .init(limit: limit, cursor: cursor),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID))
        }
        switch output {
        case .ok(let ok):
            let page = try ok.body.json
            return CalcHistoryPage(items: page.items.map(Self.domainHistoryEntry), nextCursor: page.nextCursor)
        case .badRequest(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    // MARK: - 生成型 → ドメイン

    private static func domainHistoryEntry(_ entry: Components.Schemas.CalcHistoryEntry) -> CalcHistoryEntry {
        CalcHistoryEntry(
            occurredAt: entry.occurredAt, calc: domainCalc(entry.calc),
            minPercent: entry.result.minPercent, maxPercent: entry.result.maxPercent)
    }

    /// `CalcRequest` → ドメイン(履歴の行とお気に入りの `calc` で共通。ADR-0524)。
    static func domainCalc(_ calc: Components.Schemas.CalcRequest) -> CalcHistoryCalc {
        let field = calc.field
        return CalcHistoryCalc(
            format: Format(rawValue: calc.format.rawValue) ?? .single,
            attacker: domainIndividual(calc.attacker),
            defender: domainIndividual(calc.defender),
            moveId: calc.moveId,
            field: FieldState(
                weather: field?.weather.flatMap { Weather(rawValue: $0.rawValue) } ?? .none,
                terrain: field?.terrain.flatMap { Terrain(rawValue: $0.rawValue) } ?? .none,
                attackerScreens: domainScreens(field?.attackerScreens),
                defenderScreens: domainScreens(field?.defenderScreens)),
            critical: calc.options?.critical ?? false)
    }

    static func domainScreens(_ screens: Components.Schemas.Screens?) -> Screens {
        Screens(
            reflect: screens?.reflect ?? false, lightScreen: screens?.lightScreen ?? false,
            auroraVeil: screens?.auroraVeil ?? false)
    }
}
