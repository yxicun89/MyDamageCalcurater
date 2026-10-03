import HTTPTypes
import OpenAPIRuntime
import XCTest

@testable import WishlistCore

// AC-IOS-EST-09: APIWishlistService のフェーズ3(estimates の写像・refresh・listings)。
final class APIWishlistEstimatesTests: XCTestCase {
    private let base = URL(string: "https://wishlist.example/wishlist/")!

    private func service(_ transport: RecordingTransport) -> APIWishlistService {
        APIWishlistService(baseURL: base, token: "secret-token", transport: transport)
    }

    private let estimatesJSON = """
        {"item_id":12,"summary_low":3000,"summary_mid":4500,"summary_fetched_at":"2026-10-03T00:30:00Z",
         "sites":[
           {"site_id":1,"low":3000,"mid":4500,"count":5,"suspicious_count":1,"in_stock_count":4,"status":"ok","fetched_at":"2026-10-03T00:30:00Z"},
           {"site_id":3,"low":null,"mid":null,"count":0,"suspicious_count":0,"in_stock_count":0,"status":"no_result","fetched_at":"2026-10-03T00:30:00Z"},
           {"site_id":4,"low":2800,"mid":null,"count":4,"suspicious_count":0,"in_stock_count":0,"status":"failed","fetched_at":"2026-10-01T00:00:00Z"}],
         "refreshing":true}
        """

    private func expectedEstimates() -> ItemEstimates {
        let at = Date(timeIntervalSince1970: 1_790_987_400)
        return ItemEstimates(
            itemID: 12, summaryLow: 3000, summaryMid: 4500, summaryFetchedAt: at,
            sites: [
                SiteEstimate(siteID: 1, low: 3000, mid: 4500, count: 5, suspiciousCount: 1, inStockCount: 4, status: .ok, fetchedAt: at),
                SiteEstimate(siteID: 3, count: 0, suspiciousCount: 0, inStockCount: 0, status: .noResult, fetchedAt: at),
                SiteEstimate(
                    siteID: 4, low: 2800, count: 4, suspiciousCount: 0, inStockCount: 0, status: .failed,
                    fetchedAt: Date(timeIntervalSince1970: 1_790_812_800)),
            ],
            refreshing: true)
    }

    /// 既存の `testEstimatesMapsEmptySites` は空の sites だけ。ここで in_stock_count・status 3 種・null を固定する。
    func testEstimatesMapsStockStatusesAndNulls() async throws {
        let transport = RecordingTransport(reply: .json(estimatesJSON))
        let result = try await service(transport).estimates(itemID: 12)
        XCTAssertEqual(transport.last?.request.method, .get)
        XCTAssertEqual(transport.last?.request.path, "/api/items/12/estimates")
        XCTAssertEqual(result, expectedEstimates())
    }

    /// POST は 202 で本文を返す(GET と同じ形)。Bearer を付ける。
    func testRefreshEstimatesPostsAndReadsThe202Body() async throws {
        let transport = RecordingTransport(reply: .json(202, estimatesJSON))
        let result = try await service(transport).refreshEstimates(itemID: 12)
        let request = try XCTUnwrap(transport.last).request
        XCTAssertEqual(request.method, .post)
        XCTAssertEqual(request.path, "/api/items/12/estimates/refresh")
        XCTAssertEqual(request.headerFields[.authorization], "Bearer secret-token")
        XCTAssertEqual(result, expectedEstimates())
    }

    func testRefreshEstimatesMapsFailures() async {
        let table: [(RecordingTransport.Reply, WishlistError.Code)] = [
            (.json(404, #"{"error":{"code":"not_found","message":"item not found"}}"#), .notFound),
            (.json(401, #"{"error":{"code":"unauthorized","message":"bad token"}}"#), .unauthorized),
        ]
        for (reply, code) in table {
            do {
                _ = try await service(RecordingTransport(reply: reply)).refreshEstimates(itemID: 12)
                XCTFail("失敗するはず")
            } catch let error as WishlistError {
                XCTAssertEqual(error.code, code)
            } catch {
                XCTFail("WishlistError のはず: \(error)")
            }
        }
    }

    private let listingsJSON = """
        {"listings":[
          {"id":102,"site_id":1,"title":"グリス ベルトだけ","price":300,"url":"https://item.example.com/102","image_url":"https://img.example.com/102.jpg",
           "in_stock":true,"suspicious_reasons":["title_mismatch","too_cheap"],"fetched_at":"2026-10-03T00:30:00Z"},
          {"id":101,"site_id":3,"title":"S.H.Figuarts グリス","price":3000,"url":"https://item.example.com/101","image_url":null,
           "in_stock":false,"suspicious_reasons":[],"fetched_at":"2026-10-03T00:30:00Z"}]}
        """

    func testListingsMapsEveryFieldAndReasons() async throws {
        let transport = RecordingTransport(reply: .json(listingsJSON))
        let result = try await service(transport).listings(itemID: 12, siteID: nil)
        let request = try XCTUnwrap(transport.last).request
        XCTAssertEqual(request.method, .get)
        XCTAssertEqual(request.path, "/api/items/12/listings", "site_id なしならクエリを付けない")
        XCTAssertEqual(request.headerFields[.authorization], "Bearer secret-token")
        let at = Date(timeIntervalSince1970: 1_790_987_400)
        XCTAssertEqual(
            result,
            [
                Listing(
                    id: 102, siteID: 1, title: "グリス ベルトだけ", price: 300, url: "https://item.example.com/102",
                    imageURL: "https://img.example.com/102.jpg", inStock: true, suspiciousReasons: [.titleMismatch, .tooCheap], fetchedAt: at),
                Listing(
                    id: 101, siteID: 3, title: "S.H.Figuarts グリス", price: 3000, url: "https://item.example.com/101", imageURL: nil,
                    inStock: false, suspiciousReasons: [], fetchedAt: at),
            ])
    }

    func testListingsSendsSiteIDQuery() async throws {
        let transport = RecordingTransport(reply: .json(#"{"listings":[]}"#))
        let result = try await service(transport).listings(itemID: 12, siteID: 3)
        XCTAssertEqual(result, [])
        let path = try XCTUnwrap(transport.last).request.path ?? ""
        XCTAssertTrue(path.hasPrefix("/api/items/12/listings"), path)
        XCTAssertTrue(path.contains("site_id=3"), path)
    }

    func testListingsTransportFailureIsNetworkError() async {
        let transport = RecordingTransport { _ in throw URLError(.notConnectedToInternet) }
        do {
            _ = try await service(transport).listings(itemID: 12, siteID: nil)
            XCTFail("失敗するはず")
        } catch let error as WishlistError {
            XCTAssertTrue(error.isNetwork)
        } catch {
            XCTFail("WishlistError のはず: \(error)")
        }
    }
}
