import Foundation

// ジャンルの絵柄の線画(docs/phase5-spec.md 5-2「ジャンルの絵柄」。Web の svg パスと同じ 24x24)。
// View は `GenreGlyph.subpaths(for:)` の折れ線を `stroke` 2・`fill` なしで描く。パスの文字列はここにだけ置く。

public struct GlyphPoint: Equatable, Sendable {
    public var x: Double
    public var y: Double
    public init(x: Double, y: Double) {
        self.x = x
        self.y = y
    }
}

/// 1 つの連続した線(`M` から次の `M` まで)。円弧は折れ線に開いてある。
public struct GlyphSubpath: Equatable, Sendable {
    public var points: [GlyphPoint]
    public var isClosed: Bool
    public init(points: [GlyphPoint], isClosed: Bool) {
        self.points = points
        self.isClosed = isClosed
    }
}

public enum GenreGlyph {
    /// 描画領域の一辺(svg の viewBox)
    public static let size = 24.0

    /// 「すべて」専用の絵柄(API の値ではない)
    public static let allPath = "M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z"

    public static func path(for icon: GenreIcon) -> String {
        switch icon {
        case .card: "M5 2h14v20H5zM12 7l1.5 3 3.3.5-2.4 2.3.6 3.2L12 14.5 9 16l.6-3.2-2.4-2.3 3.3-.5z"
        case .figure: "M12 2a3 3 0 1 1 0 6 3 3 0 0 1 0-6zM12 8v7M7 11l5-1 5 1M9 22l3-7 3 7"
        case .robot: "M6 4h12v9H6zM9 8h.01M15 8h.01M12 1v3M8 13v4M16 13v4M5 21h5M14 21h5"
        case .goods: "M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18zM3 12h6M15 12h6"
        case .star: "M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z"
        case .box: "M3 7l9-4 9 4v10l-9 4-9-4zM3 7l9 4 9-4M12 11v10"
        }
    }

    public static func subpaths(for icon: GenreIcon) -> [GlyphSubpath] { subpaths(of: path(for: icon)) }

    /// svg のパス(`M m L l H h V v A a Z z` だけ)を折れ線にする。円弧は `rx == ry` の円弧だけを扱い、5 度刻み以下の線分に開く。
    public static func subpaths(of d: String) -> [GlyphSubpath] {
        var parser = Parser(Array(d))
        return parser.run()
    }

    private struct Parser {
        let chars: [Character]
        var index = 0
        var result: [GlyphSubpath] = []
        var points: [GlyphPoint] = []
        var current = GlyphPoint(x: 0, y: 0)
        var start = GlyphPoint(x: 0, y: 0)

        init(_ chars: [Character]) { self.chars = chars }

        mutating func run() -> [GlyphSubpath] {
            var command: Character = "M"
            while true {
                skipSeparators()
                guard index < chars.count else { break }
                if chars[index].isLetter {
                    command = chars[index]
                    index += 1
                    if command == "z" || command == "Z" {
                        close()
                        continue
                    }
                }
                guard apply(command) else { break }
            }
            flush(closed: false)
            return result
        }

        mutating func apply(_ command: Character) -> Bool {
            let relative = command.isLowercase
            switch command.uppercased() {
            case "M":
                guard let x = number(), let y = number() else { return false }
                flush(closed: false)
                move(to: relative ? GlyphPoint(x: current.x + x, y: current.y + y) : GlyphPoint(x: x, y: y))
                start = current
            case "L":
                guard let x = number(), let y = number() else { return false }
                line(to: relative ? GlyphPoint(x: current.x + x, y: current.y + y) : GlyphPoint(x: x, y: y))
            case "H":
                guard let x = number() else { return false }
                line(to: GlyphPoint(x: relative ? current.x + x : x, y: current.y))
            case "V":
                guard let y = number() else { return false }
                line(to: GlyphPoint(x: current.x, y: relative ? current.y + y : y))
            case "A":
                guard let rx = number(), number() != nil, number() != nil, let large = number(), let sweep = number(),
                    let x = number(), let y = number()
                else { return false }
                arc(radius: rx, large: large != 0, sweep: sweep != 0, to: relative ? GlyphPoint(x: current.x + x, y: current.y + y) : GlyphPoint(x: x, y: y))
            default:
                return false
            }
            return true
        }

        mutating func move(to p: GlyphPoint) {
            current = p
            points = [p]
        }

        mutating func line(to p: GlyphPoint) {
            if points.isEmpty { points = [current] }
            points.append(p)
            current = p
        }

        mutating func close() {
            flush(closed: true)
            current = start
        }

        mutating func flush(closed: Bool) {
            if points.count > 1 { result.append(GlyphSubpath(points: points, isClosed: closed)) }
            points = []
        }

        mutating func arc(radius: Double, large: Bool, sweep: Bool, to end: GlyphPoint) {
            let p1 = current
            let dx = (p1.x - end.x) / 2
            let dy = (p1.y - end.y) / 2
            let d2 = dx * dx + dy * dy
            guard d2 > 0, radius > 0 else { return }
            let r = max(radius, d2.squareRoot())
            let sign: Double = large != sweep ? 1 : -1
            let coef = sign * max(0, (r * r - d2) / d2).squareRoot()
            let center = GlyphPoint(x: coef * dy + (p1.x + end.x) / 2, y: -coef * dx + (p1.y + end.y) / 2)
            let a1 = atan2(p1.y - center.y, p1.x - center.x)
            var delta = atan2(end.y - center.y, end.x - center.x) - a1
            if sweep, delta < 0 { delta += 2 * .pi }
            if !sweep, delta > 0 { delta -= 2 * .pi }
            let steps = max(2, Int((abs(delta) / (5 * .pi / 180)).rounded(.up)))
            if points.isEmpty { points = [current] }
            for step in 1...steps {
                let a = a1 + delta * Double(step) / Double(steps)
                points.append(GlyphPoint(x: center.x + r * cos(a), y: center.y + r * sin(a)))
            }
            points[points.count - 1] = end
            current = end
        }

        mutating func skipSeparators() {
            while index < chars.count, chars[index] == " " || chars[index] == "," { index += 1 }
        }

        mutating func number() -> Double? {
            skipSeparators()
            var text = ""
            var seenDot = false
            while index < chars.count {
                let c = chars[index]
                if c.isNumber {
                    text.append(c)
                } else if c == "." {
                    if seenDot { break }
                    seenDot = true
                    text.append(c)
                } else if (c == "-" || c == "+") && text.isEmpty {
                    text.append(c)
                } else {
                    break
                }
                index += 1
            }
            return Double(text)
        }
    }
}
