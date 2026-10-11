import Foundation

// フェーズ5-3 ジャンル編集の色の見本 12 色(docs/phase5-spec.md 5-2「色の見本」。値は Web と同じ)。完成品(データ)。

public struct GenreSwatch: Equatable, Sendable {
    public var name: String
    public var hex: String
    public init(name: String, hex: String) {
        self.name = name
        self.hex = hex
    }
}

public enum GenrePalette {
    public static let swatches: [GenreSwatch] = [
        .init(name: "赤", hex: "#e0393e"), .init(name: "朱", hex: "#f0642a"), .init(name: "橙", hex: "#f59e0b"),
        .init(name: "黄", hex: "#f2b705"), .init(name: "黄緑", hex: "#7cb518"), .init(name: "緑", hex: "#16a34a"),
        .init(name: "青緑", hex: "#0d9488"), .init(name: "水色", hex: "#0ea5e9"), .init(name: "青", hex: "#1f6fe5"),
        .init(name: "藍", hex: "#4f46e5"), .init(name: "紫", hex: "#7c4dff"), .init(name: "桃", hex: "#e04f9a"),
    ]
}

extension GenreIcon {
    /// 絵柄を選ぶ場面のラベル(カード・フィギュア・ロボ・グッズ・星・箱)
    public var label: String {
        switch self {
        case .card: "カード"
        case .figure: "フィギュア"
        case .robot: "ロボ"
        case .goods: "グッズ"
        case .star: "星"
        case .box: "箱"
        }
    }
}
