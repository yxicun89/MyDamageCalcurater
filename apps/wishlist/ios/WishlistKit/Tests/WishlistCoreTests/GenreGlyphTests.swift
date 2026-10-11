import XCTest

@testable import WishlistCore

// ジャンルの絵柄の線画(phase5-spec 5-2「ジャンルの絵柄」。Web の svg パスを折れ線にする)。
final class GenreGlyphTests: XCTestCase {
    func testAllPathIsFourClosedSquares() {
        let subpaths = GenreGlyph.subpaths(of: GenreGlyph.allPath)
        XCTAssertEqual(subpaths.count, 4)
        XCTAssertTrue(subpaths.allSatisfy(\.isClosed))
        XCTAssertEqual(subpaths[0].points, [.init(x: 3, y: 3), .init(x: 10, y: 3), .init(x: 10, y: 10), .init(x: 3, y: 10)])
        XCTAssertEqual(subpaths[3].points.first, GlyphPoint(x: 14, y: 14))
    }

    func testRelativeAndImplicitRepeatedCommands() {
        // card の星: `l1.5 3 3.3.5-2.4 2.3.6 3.2` は l の暗黙の繰り返し・`3.3.5` は 3.3 と .5
        let subpaths = GenreGlyph.subpaths(of: "M12 7l1.5 3 3.3.5z")
        XCTAssertEqual(subpaths.count, 1)
        let points = subpaths[0].points
        XCTAssertEqual(points.count, 3)
        XCTAssertEqual(points[1].x, 13.5, accuracy: 1e-9)
        XCTAssertEqual(points[1].y, 10, accuracy: 1e-9)
        XCTAssertEqual(points[2].x, 16.8, accuracy: 1e-9)
        XCTAssertEqual(points[2].y, 10.5, accuracy: 1e-9)
    }

    func testCircleArcsStayOnTheCircleAndEndAtTheEndpoint() {
        // goods: 中心 (12,12) 半径 9 の円(2 つの半円)
        let subpaths = GenreGlyph.subpaths(for: .goods)
        let circle = subpaths[0]
        XCTAssertTrue(circle.isClosed)
        XCTAssertGreaterThan(circle.points.count, 30)
        for p in circle.points {
            XCTAssertEqual(((p.x - 12) * (p.x - 12) + (p.y - 12) * (p.y - 12)).squareRoot(), 9, accuracy: 1e-6)
        }
        XCTAssertEqual(circle.points.first, GlyphPoint(x: 12, y: 3))
    }

    func testEveryIconStaysInsideTheViewBox() {
        for icon in GenreIcon.allCases {
            let subpaths = GenreGlyph.subpaths(for: icon)
            XCTAssertFalse(subpaths.isEmpty, icon.rawValue)
            for p in subpaths.flatMap(\.points) {
                XCTAssertTrue((0...GenreGlyph.size).contains(p.x) && (0...GenreGlyph.size).contains(p.y), "\(icon) \(p)")
            }
        }
    }

    func testTinyDotsKeepTheirSegment() {
        // robot の目: `M9 8h.01`
        let eyes = GenreGlyph.subpaths(for: .robot).filter { $0.points.count == 2 && abs($0.points[1].x - $0.points[0].x) < 0.1 && abs($0.points[1].y - $0.points[0].y) < 0.1 }
        XCTAssertEqual(eyes.count, 2)
    }
}
