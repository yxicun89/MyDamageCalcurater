import HTTPTypes
import OpenAPIRuntime
import XCTest

@testable import WishlistCore

// AC-IOS-HIS-API-01: APIWishlistService の価格の推移(docs/phase4-spec.md)。
final class APIWishlistPriceHistoryTests: XCTestCase {
    private let base = URL(string: "https://wishlist.example/wishlist/")!

    private func service(_ transport: RecordingTransport) -> APIWishlistService {
        APIWishlistService(baseURL: base, token: "secret-token", transport: transport)
    }

    private let json = """
        {"item_id":12,"days":90,
         "sites":[{"site_id":1,"points":[{"day":"2026-10-01","low":3000,"mid":3400},{"day":"2026-10-03","low":3500,"mid":null}]}],
         "overall":[{"day":"2026-10-01","low":3000},{"day":"2026-10-03","low":3500}]}
        """

    func testPriceHistoryGetsWithoutDaysAndMapsEveryField() async throws {
        let transport = RecordingTransport(reply: .json(json))
        let result = try await service(transport).priceHistory(itemID: 12, days: nil)
        let request = try XCTUnwrap(transport.last).request
        XCTAssertEqual(request.method, .get)
        XCTAssertEqual(request.path, "/api/items/12/price-history")
        XCTAssertEqual(request.headerFields[.authorization], "Bearer secret-token")
        XCTAssertEqual(
            result,
            PriceHistory(
                itemID: 12, days: 90,
                sites: [SitePriceHistory(siteID: 1, points: [PricePoint(day: "2026-10-01", low: 3000, mid: 3400), PricePoint(day: "2026-10-03", low: 3500)])],
                overall: [DayLow(day: "2026-10-01", low: 3000), DayLow(day: "2026-10-03", low: 3500)]))
    }

    func testPriceHistoryPassesDays() async throws {
        let transport = RecordingTransport(reply: .json(json))
        _ = try await service(transport).priceHistory(itemID: 12, days: 180)
        let path = try XCTUnwrap(transport.last).request.path ?? ""
        XCTAssertTrue(path.hasPrefix("/api/items/12/price-history"), path)
        XCTAssertTrue(path.contains("days=180"), path)
    }

    func testPriceHistoryFailures() async {
        let table: [(String, RecordingTransport, WishlistError.Code)] = [
            ("404", RecordingTransport(reply: .json(404, #"{"code":"not_found","message":"item not found"}"#)), .notFound),
            ("通信失敗", RecordingTransport { _ in throw URLError(.notConnectedToInternet) }, .network),
        ]
        for (name, transport, code) in table {
            do {
                _ = try await service(transport).priceHistory(itemID: 12, days: nil)
                XCTFail("\(name): 失敗するはず")
            } catch let error as WishlistError {
                XCTAssertEqual(error.code, code, name)
            } catch {
                XCTFail("\(name): WishlistError のはず: \(error)")
            }
        }
    }
}
