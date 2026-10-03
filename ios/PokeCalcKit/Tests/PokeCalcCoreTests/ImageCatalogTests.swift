import XCTest

@testable import PokeCalcCore

/// P8-1c: ImageCatalog(取得・キャッシュ・失敗の吸収)・モック・表示判定。ADR-0508 §AC-3〜AC-6。
final class ImageCatalogTests: XCTestCase {
    private let base = URL(string: "https://gw.example")!
    private static let goodManifest = #"{"version":1,"images":{"9001-000":{"thumb":"thumb/9001-000.ab12cd34.webp","detail":"detail/9001-000.ef567890.webp"}}}"#

    /// 取得を数え、決めた結果を返す偽 fetcher。
    private final class FakeFetcher: ImageManifestFetching, @unchecked Sendable {
        enum Behavior { case respond(Int, String), fail }
        private let lock = NSLock()
        private var _urls: [URL] = []
        let behavior: Behavior
        init(_ behavior: Behavior) { self.behavior = behavior }
        var urls: [URL] { lock.withLock { _urls } }
        func fetchManifest(from url: URL) async throws -> ImageManifestResponse {
            lock.withLock { _urls.append(url) }
            try await Task.sleep(nanoseconds: 20_000_000)  // 同時呼び出しを重ねる
            switch behavior {
            case .respond(let code, let body): return ImageManifestResponse(statusCode: code, data: Data(body.utf8))
            case .fail: throw URLError(.notConnectedToInternet)
            }
        }
    }

    func testSuccessReturnsURLsForBothSizes() async {
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: FakeFetcher(.respond(200, Self.goodManifest)))
        let thumb = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
        let detail = await catalog.imageURL(speciesKey: "9001-000", size: .detail)
        XCTAssertEqual(thumb?.absoluteString, "https://gw.example/images/thumb/9001-000.ab12cd34.webp")
        XCTAssertEqual(detail?.absoluteString, "https://gw.example/images/detail/9001-000.ef567890.webp")
    }

    func testKeyNotInManifestIsNil() async {
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: FakeFetcher(.respond(200, Self.goodManifest)))
        let url = await catalog.imageURL(speciesKey: "9002-000", size: .thumb)
        XCTAssertNil(url)
    }

    func testRequestsManifestUnderImagesWithoutThrowing() async {
        let fetcher = FakeFetcher(.respond(200, Self.goodManifest))
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: fetcher)
        _ = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
        XCTAssertEqual(fetcher.urls.map(\.absoluteString), ["https://gw.example/images/manifest.json"])
    }

    func testNon200StatusesAreNoImage() async {
        for code in [404, 500, 503, 301, 204] {
            let catalog = RemoteImageCatalog(baseURL: base, fetcher: FakeFetcher(.respond(code, Self.goodManifest)))
            let url = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
            XCTAssertNil(url, "status \(code) は本文が正しくても画像なし")
        }
    }

    func testInvalidBodyAndVersionMismatchAreNoImage() async {
        for body in ["<html>", "", #"{"version":2,"images":{"9001-000":{"thumb":"thumb/a.webp"}}}"#] {
            let catalog = RemoteImageCatalog(baseURL: base, fetcher: FakeFetcher(.respond(200, body)))
            let url = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
            XCTAssertNil(url, "body: \(body)")
        }
    }

    func testTransportFailureIsNoImageAndDoesNotThrow() async {
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: FakeFetcher(.fail))
        let url = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
        XCTAssertNil(url)
    }

    func testUnsafeManifestPathIsNoImage() async {
        let body = #"{"version":1,"images":{"9001-000":{"thumb":"../secret.webp","detail":"https://evil.example/a.webp"}}}"#
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: FakeFetcher(.respond(200, body)))
        let thumb = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
        let detail = await catalog.imageURL(speciesKey: "9001-000", size: .detail)
        XCTAssertNil(thumb)
        XCTAssertNil(detail)
    }

    func testManifestIsFetchedOnceEvenForConcurrentAndRepeatedLookups() async {
        let fetcher = FakeFetcher(.respond(200, Self.goodManifest))
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: fetcher)
        await withTaskGroup(of: URL?.self) { group in
            for _ in 0..<10 { group.addTask { await catalog.imageURL(speciesKey: "9001-000", size: .thumb) } }
            for await _ in group {}
        }
        _ = await catalog.imageURL(speciesKey: "9001-000", size: .detail)
        XCTAssertEqual(fetcher.urls.count, 1)
    }

    func testFailureIsAlsoCachedAndNotRetried() async {
        let fetcher = FakeFetcher(.fail)
        let catalog = RemoteImageCatalog(baseURL: base, fetcher: fetcher)
        _ = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
        _ = await catalog.imageURL(speciesKey: "9001-000", size: .thumb)
        XCTAssertEqual(fetcher.urls.count, 1, "起動時1回・手動更新なし。失敗でも再取得しない(画面を遅くしない)")
    }

    func testNoImageCatalogAlwaysNil() async {
        let url = await NoImageCatalog().imageURL(speciesKey: "9001-000", size: .thumb)
        XCTAssertNil(url)
    }

    // MARK: - モック

    func testMockScenarioMapping() {
        XCTAssertEqual(MockImageCatalog.scenarioEnvironmentKey, "POKECALC_MOCK_IMAGES")
        XCTAssertEqual(MockImageScenario(environmentValue: nil), .none)
        XCTAssertEqual(MockImageScenario(environmentValue: "unknown"), .none)
        XCTAssertEqual(MockImageScenario(environmentValue: "0"), .none)
        XCTAssertEqual(MockImageScenario(environmentValue: "1"), .available)
    }

    func testMockDefaultHasNoImagesForAnyKey() async {
        for catalog in [MockImageCatalog(), MockImageCatalog(environment: [:]), MockImageCatalog(environment: ["POKECALC_USE_MOCK": "1"])] {
            for key in ["9001-000", "9002-000", "9003-000", "9999-000"] {
                for size in PokeImageSize.allCases {
                    let url = await catalog.imageURL(speciesKey: key, size: size)
                    XCTAssertNil(url, "既定は画像なし: \(key) \(size)")
                }
            }
        }
    }

    func testMockAvailableGivesTinyFictionalPNGOnlyForListedKeys() async throws {
        let catalog = MockImageCatalog(environment: ["POKECALC_MOCK_IMAGES": "1"])
        XCTAssertEqual(MockImageCatalog.availableKeys, ["9001-000", "9003-000"])
        for key in MockImageCatalog.availableKeys {
            for size in PokeImageSize.allCases {
                let found = await catalog.imageURL(speciesKey: key, size: size)
                let url = try XCTUnwrap(found)
                XCTAssertEqual(url.scheme, "data", "外部通信しない data URL")
                let data = try Data(contentsOf: url)
                XCTAssertLessThan(data.count, 1024, "数バイトの架空画像")
                XCTAssertEqual(Array(data.prefix(8)), [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A], "PNG シグネチャ")
            }
        }
        let missing = await catalog.imageURL(speciesKey: "9002-000", size: .thumb)
        XCTAssertNil(missing, "9002-000 はエンブレムのままにする")
    }

    // MARK: - 表示判定

    func testShowsImageOnlyOnSuccess() {
        XCTAssertTrue(SpeciesImageDisplay.showsImage(phase: .success))
        XCTAssertFalse(SpeciesImageDisplay.showsImage(phase: .loading), "読み込み中はエンブレム")
        XCTAssertFalse(SpeciesImageDisplay.showsImage(phase: .failure), "失敗時はエンブレム")
    }

    func testLookupKeyRejectsNilEmptyAndBlank() {
        XCTAssertEqual(SpeciesImageDisplay.lookupKey(speciesKey: "9001-000"), "9001-000")
        XCTAssertNil(SpeciesImageDisplay.lookupKey(speciesKey: nil))
        XCTAssertNil(SpeciesImageDisplay.lookupKey(speciesKey: ""))
        XCTAssertNil(SpeciesImageDisplay.lookupKey(speciesKey: "  "))
    }
}
