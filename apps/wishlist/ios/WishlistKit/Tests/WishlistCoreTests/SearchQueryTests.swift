import XCTest

@testable import WishlistCore

// AC-IOS-LIB-01/02/03: 検索ワードとディープリンク。Go・TS と共通のテストベクタ(testdata/query-cases.json)を全件検証する。
// ベクタはパッケージのテストリソース(Resources/query-cases.json。scripts/sync-testdata.sh でコピー)から読む。

struct QueryVectors: Decodable {
    struct BuildCase: Decodable {
        let note: String?
        let template: String
        let name: String
        let option: String
        let queryOverride: String?
        let siteQuery: String?
        let want: String

        enum CodingKeys: String, CodingKey {
            case note, template, name, option, want
            case queryOverride = "query_override"
            case siteQuery = "site_query"
        }
    }

    struct DeeplinkCase: Decodable {
        let note: String?
        let template: String
        let query: String
        let want: String
    }

    let build: [BuildCase]
    let deeplink: [DeeplinkCase]

    static func load() throws -> QueryVectors {
        let url = try XCTUnwrap(Bundle.module.url(forResource: "query-cases", withExtension: "json"), "テストリソース query-cases.json が無い")
        return try JSONDecoder().decode(QueryVectors.self, from: Data(contentsOf: url))
    }
}

final class SearchQueryTests: XCTestCase {
    func testVectorsAreLoadedAndNotEmpty() throws {
        let vectors = try QueryVectors.load()
        XCTAssertGreaterThanOrEqual(vectors.build.count, 15)
        XCTAssertGreaterThanOrEqual(vectors.deeplink.count, 11)
    }

    /// AC-IOS-LIB-01
    func testBuildMatchesAllSharedVectors() throws {
        for c in try QueryVectors.load().build {
            let input = QueryInput(
                template: c.template, name: c.name, option: c.option,
                queryOverride: c.queryOverride, siteQuery: c.siteQuery)
            XCTAssertEqual(SearchQuery.build(input), c.want, "build: \(c.note ?? c.template)")
        }
    }

    /// option が nil(API の option_text 無し)でも、空文字と同じ結果になる。
    func testNilOptionBehavesLikeEmpty() {
        let input = QueryInput(template: "{name} {option}", name: "ボルシャック", option: nil)
        XCTAssertEqual(SearchQuery.build(input), "ボルシャック")
    }

    func testSpecTableExamples() {
        XCTAssertEqual(SearchQuery.build(QueryInput(template: "S.H.Figuarts {name}", name: "グリス")), "S.H.Figuarts グリス")
        XCTAssertEqual(
            SearchQuery.build(QueryInput(template: "{name} {option}", name: "ボルシャック", option: "銀トレジャー")),
            "ボルシャック 銀トレジャー")
        XCTAssertEqual(SearchQuery.build(QueryInput(template: "{name}", name: "HG ガンダムエアリアル")), "HG ガンダムエアリアル")
    }
}

final class DeeplinkTests: XCTestCase {
    /// AC-IOS-LIB-02: encodeURIComponent と同じエスケープ集合(`A-Za-z0-9 - _ . ! ~ * ' ( )` 以外を UTF-8 の %XX)
    func testBuildMatchesAllSharedVectors() throws {
        for c in try QueryVectors.load().deeplink {
            XCTAssertEqual(Deeplink.build(template: c.template, query: c.query), c.want, "deeplink: \(c.note ?? c.query)")
        }
    }

    /// 値の中の `$&` や `\` を特別扱いしない(正規表現の置換文字列として解釈しない)。
    func testQueryWithReplacementPatternCharactersIsLiteral() {
        XCTAssertEqual(Deeplink.build(template: "https://e.example/s?q={q}", query: "$&\\1"), "https://e.example/s?q=%24%26%5C1")
    }

    /// AC-IOS-LIB-03
    func testIsValidSearchTemplate() {
        let cases: [(String, Bool)] = [
            ("https://jp.mercari.com/search?keyword={q}&status=on_sale", true),
            ("http://example.com/s?q={q}", true),
            ("https://example.com/s?q=", false),
            ("ftp://example.com/s?q={q}", false),
            ("javascript:alert({q})", false),
            ("example.com/s?q={q}", false),
            ("", false),
        ]
        for (template, want) in cases {
            XCTAssertEqual(Deeplink.isValidSearchTemplate(template), want, template)
        }
    }

    func testIsHTTPURL() {
        let cases: [(String, Bool)] = [
            ("https://example.com/a", true),
            ("http://example.com", true),
            ("javascript:alert(1)", false),
            ("ftp://example.com", false),
            ("example.com", false),
            ("", false),
            ("mercari://search?keyword=a", false),
        ]
        for (s, want) in cases {
            XCTAssertEqual(Deeplink.isHTTPURL(s), want, s)
        }
    }
}
