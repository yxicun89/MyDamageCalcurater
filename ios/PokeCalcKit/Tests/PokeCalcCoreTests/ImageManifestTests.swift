import XCTest

@testable import PokeCalcCore

/// P8-1c: manifest の decode(全分岐)と URL 組み立て。ADR-0508 §AC-1・AC-2。架空キー(9xxx)だけを使う。
final class ImageManifestTests: XCTestCase {
    private func manifest(_ json: String) -> ImageManifest {
        ImageManifest(decoding: Data(json.utf8))
    }

    func testDecodesNormalManifest() {
        let m = manifest(#"{"version":1,"images":{"9001-000":{"thumb":"thumb/9001-000.ab12cd34.webp","detail":"detail/9001-000.ef567890.webp"}}}"#)
        XCTAssertEqual(m.relativePath(forKey: "9001-000", size: .thumb), "thumb/9001-000.ab12cd34.webp")
        XCTAssertEqual(m.relativePath(forKey: "9001-000", size: .detail), "detail/9001-000.ef567890.webp")
    }

    func testMissingKeyMeansNoImage() {
        let m = manifest(#"{"version":1,"images":{"9001-000":{"thumb":"thumb/a.webp","detail":"detail/a.webp"}}}"#)
        XCTAssertNil(m.relativePath(forKey: "9002-000", size: .thumb))
    }

    func testEmptyImagesMeansNoImage() {
        XCTAssertEqual(manifest(#"{"version":1,"images":{}}"#), .empty)
    }

    func testOnlyOneSizePresentGivesNilForTheOther() {
        let m = manifest(#"{"version":1,"images":{"9001-000":{"thumb":"thumb/a.webp"}}}"#)
        XCTAssertEqual(m.relativePath(forKey: "9001-000", size: .thumb), "thumb/a.webp")
        XCTAssertNil(m.relativePath(forKey: "9001-000", size: .detail))
    }

    func testVersionMismatchIsNoImageNotError() {
        for v in ["0", "2", "99", "\"1\"", "null", "1.5"] {
            let m = manifest(#"{"version":\#(v),"images":{"9001-000":{"thumb":"thumb/a.webp","detail":"detail/a.webp"}}}"#)
            XCTAssertEqual(m, .empty, "version=\(v)")
        }
    }

    func testMissingVersionIsNoImage() {
        XCTAssertEqual(manifest(#"{"images":{"9001-000":{"thumb":"thumb/a.webp"}}}"#), .empty)
    }

    func testInvalidJSONIsNoImage() {
        for bad in ["", "not json", "{", "[]", "null", "\"x\"", "<html>404</html>", #"{"version":1,"images":[]}"#, #"{"version":1,"images":"x"}"#] {
            XCTAssertEqual(manifest(bad), .empty, "入力: \(bad)")
        }
    }

    func testMissingImagesFieldIsNoImage() {
        XCTAssertEqual(manifest(#"{"version":1}"#), .empty)
    }

    func testUnknownFieldsAreIgnored() {
        let m = manifest(#"{"version":1,"generatedAt":"x","images":{"9001-000":{"thumb":"thumb/a.webp","detail":"detail/a.webp","blurhash":"abc"}},"extra":{"a":1}}"#)
        XCTAssertEqual(m.relativePath(forKey: "9001-000", size: .thumb), "thumb/a.webp")
    }

    func testMalformedEntryDoesNotDropTheOthers() {
        let m = manifest(#"{"version":1,"images":{"9001-000":{"thumb":123},"9003-000":{"thumb":"thumb/c.webp"}}}"#)
        XCTAssertNil(m.relativePath(forKey: "9001-000", size: .thumb), "型違いの1件は画像なし")
        XCTAssertEqual(m.relativePath(forKey: "9003-000", size: .thumb), "thumb/c.webp")
    }

    func testSupportedVersionIsOne() {
        XCTAssertEqual(ImageManifest.supportedVersion, 1)
    }
}

final class PokeImageURLTests: XCTestCase {
    private let base = URL(string: "https://gw.example")!

    func testJoinsBaseImagesAndRelativePath() {
        XCTAssertEqual(PokeImageURL.url(baseURL: base, relativePath: "thumb/9001-000.ab12cd34.webp")?.absoluteString,
                       "https://gw.example/images/thumb/9001-000.ab12cd34.webp")
    }

    func testBaseWithTrailingSlashAndPathPrefix() {
        XCTAssertEqual(PokeImageURL.url(baseURL: URL(string: "https://gw.example/")!, relativePath: "thumb/a.webp")?.absoluteString,
                       "https://gw.example/images/thumb/a.webp")
        XCTAssertEqual(PokeImageURL.url(baseURL: URL(string: "http://localhost:8080/prefix/")!, relativePath: "thumb/a.webp")?.absoluteString,
                       "http://localhost:8080/prefix/images/thumb/a.webp")
        XCTAssertEqual(PokeImageURL.url(baseURL: URL(string: "http://localhost:8080")!, relativePath: "detail/a.webp")?.absoluteString,
                       "http://localhost:8080/images/detail/a.webp")
    }

    func testRejectsUnsafeOrAbsoluteRelativePaths() {
        let bad = [
            "", " ", "/thumb/a.webp", "../a.webp", "thumb/../../a.webp", "thumb/..", "..",
            "https://evil.example/a.webp", "http://evil.example/a.webp", "//evil.example/a.webp",
            "file:///etc/passwd", "data:image/png;base64,AAAA", "javascript:alert(1)",
            "thumb/a.webp?x=1", "thumb/a.webp#frag", "thumb\\a.webp", "thumb/a\u{0}.webp", "thumb/a\n.webp",
        ]
        for path in bad {
            XCTAssertNil(PokeImageURL.url(baseURL: base, relativePath: path), "拒否すべき: \(path.debugDescription)")
        }
    }

    func testSameHostAndSchemeAsBase() {
        let url = PokeImageURL.url(baseURL: URL(string: "http://localhost:8080")!, relativePath: "thumb/a.webp")
        XCTAssertEqual(url?.scheme, "http")
        XCTAssertEqual(url?.host, "localhost")
        XCTAssertEqual(url?.port, 8080)
    }
}
