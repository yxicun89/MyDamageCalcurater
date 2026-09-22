// PokeCalcDesign: docs/design.md のデザイントークン(ADR-0500 §1)。
// iOS(ここ)と Web(CSS 変数)で同じ名前・同じ値を使う。値を変えるときは design.md と
// DesignTokenTests の両方を合わせて直す(coding-rules §2 の「独立した検証」)。
import SwiftUI

#if canImport(UIKit)
import UIKit
#elseif canImport(AppKit)
import AppKit
#endif

/// 0〜255 の RGB と 0〜1 の alpha を持つ色。デザイントークンの正の値をこの型で持ち、
/// SwiftUI の `Color` へは表示直前に変換する(値そのものはプラットフォームに依存しない)。
public struct RGBA: Equatable, Sendable {
    public let red: Int
    public let green: Int
    public let blue: Int
    public let alpha: Double

    public init(red: Int, green: Int, blue: Int, alpha: Double) {
        self.red = red
        self.green = green
        self.blue = blue
        self.alpha = alpha
    }
}

extension RGBA {
    /// SwiftUI の `Color`(ライト/ダークで切り替わらない固定色)。
    public var color: Color {
        Color(red: Double(red) / 255, green: Double(green) / 255, blue: Double(blue) / 255, opacity: alpha)
    }
}

/// ライト/ダークの組。design.md の表の1行に対応する。
public struct ColorPair: Sendable {
    public let light: RGBA
    public let dark: RGBA

    public init(light: RGBA, dark: RGBA) {
        self.light = light
        self.dark = dark
    }
}

extension ColorPair {
    /// 端末の外観(ライト/ダーク)に応じて自動で切り替わる `Color`。
    ///
    /// なぜ動的な色を使うか: SwiftUI の `colorScheme` 環境値に依存すると、値を読む場所が
    /// View の中に限られる。デザイントークンは View 以外(プレビュー生成・スナップショット等)
    /// からも参照できるよう、色そのものに切り替えロジックを持たせる。
    public var color: Color {
        #if canImport(UIKit)
        return Color(uiColor: UIColor { traits in
            traits.userInterfaceStyle == .dark ? UIColor(self.dark.color) : UIColor(self.light.color)
        })
        #elseif canImport(AppKit)
        return Color(nsColor: NSColor(name: nil, dynamicProvider: { appearance in
            let isDark = appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            return isDark ? NSColor(self.dark.color) : NSColor(self.light.color)
        }))
        #else
        return light.color
        #endif
    }
}

/// design.md「ベース」の6色。
public enum ColorToken {
    public static let bgBase = ColorPair(
        light: RGBA(red: 0xF4, green: 0xF5, blue: 0xF7, alpha: 1.0),
        dark: RGBA(red: 0x0E, green: 0x10, blue: 0x15, alpha: 1.0)
    )
    public static let bgGlass = ColorPair(
        light: RGBA(red: 0xFF, green: 0xFF, blue: 0xFF, alpha: 0.70),
        dark: RGBA(red: 0x1A, green: 0x1D, blue: 0x24, alpha: 0.60)
    )
    public static let textPrimary = ColorPair(
        light: RGBA(red: 0x14, green: 0x16, blue: 0x1A, alpha: 1.0),
        dark: RGBA(red: 0xF2, green: 0xF3, blue: 0xF5, alpha: 1.0)
    )
    public static let textSecondary = ColorPair(
        light: RGBA(red: 0x5C, green: 0x62, blue: 0x70, alpha: 1.0),
        dark: RGBA(red: 0xA3, green: 0xA9, blue: 0xB6, alpha: 1.0)
    )
    public static let borderHairline = ColorPair(
        light: RGBA(red: 0x00, green: 0x00, blue: 0x00, alpha: 0.08),
        dark: RGBA(red: 0xFF, green: 0xFF, blue: 0xFF, alpha: 0.10)
    )
    public static let danger = ColorPair(
        light: RGBA(red: 0xE5, green: 0x48, blue: 0x4D, alpha: 1.0),
        dark: RGBA(red: 0xFF, green: 0x63, blue: 0x69, alpha: 1.0)
    )
}

/// design.md「タイプ色(自作パレット)」。キーは `api/openapi.yaml` の `PokeType` enum の
/// 英小文字 ID(normal, fire, ... fairy)。タイプ相性表と同じくレギュレーションに依存しないので
/// マスタではなくコードの定数として持つ(CLAUDE.md ドメイン規約の「タイプ相性表」とは別物。
/// こちらは表示色であって計算には使わない)。
public enum TypeColorToken {
    /// 単一の正(宣言順を保った配列)。辞書はここから作る(重複定義を避ける。coding-rules §2)。
    /// 宣言順は openapi `PokeType` enum の宣言順と同じにする。
    private static let orderedEntries: [(id: String, rgba: RGBA)] = [
        ("normal", RGBA(red: 0x9F, green: 0xA1, blue: 0x9F, alpha: 1.0)),
        ("fire", RGBA(red: 0xE8, green: 0x62, blue: 0x2B, alpha: 1.0)),
        ("water", RGBA(red: 0x3B, green: 0x8F, blue: 0xE0, alpha: 1.0)),
        ("electric", RGBA(red: 0xF2, green: 0xC2, blue: 0x1B, alpha: 1.0)),
        ("grass", RGBA(red: 0x4C, green: 0xAF, blue: 0x50, alpha: 1.0)),
        ("ice", RGBA(red: 0x5C, green: 0xC8, blue: 0xD8, alpha: 1.0)),
        ("fighting", RGBA(red: 0xD0, green: 0x45, blue: 0x3A, alpha: 1.0)),
        ("poison", RGBA(red: 0x9B, green: 0x51, blue: 0xC6, alpha: 1.0)),
        ("ground", RGBA(red: 0xB8, green: 0x86, blue: 0x3B, alpha: 1.0)),
        ("flying", RGBA(red: 0x7F, green: 0xA7, blue: 0xE8, alpha: 1.0)),
        ("psychic", RGBA(red: 0xE8, green: 0x51, blue: 0x7F, alpha: 1.0)),
        ("bug", RGBA(red: 0x93, green: 0xB2, blue: 0x1E, alpha: 1.0)),
        ("rock", RGBA(red: 0xA9, green: 0x9A, blue: 0x62, alpha: 1.0)),
        ("ghost", RGBA(red: 0x6B, green: 0x55, blue: 0xA8, alpha: 1.0)),
        ("dragon", RGBA(red: 0x51, green: 0x60, blue: 0xD8, alpha: 1.0)),
        ("dark", RGBA(red: 0x54, green: 0x47, blue: 0x4A, alpha: 1.0)),
        ("steel", RGBA(red: 0x6E, green: 0x93, blue: 0xA8, alpha: 1.0)),
        ("fairy", RGBA(red: 0xE6, green: 0x8A, blue: 0xD6, alpha: 1.0)),
    ]
    private static let colorsByTypeID: [String: RGBA] = Dictionary(
        uniqueKeysWithValues: orderedEntries.map { ($0.id, $0.rgba) }
    )

    /// 欠け・余りの無い 18 タイプの ID(`orderedEntries` の宣言順。`Dictionary.keys` は列挙順が
    /// 不定なので直接使わない。タイプ一覧 UI が呼ぶたびに順序が変わるおそれがあるため)。
    public static let allTypeIDs: [String] = orderedEntries.map(\.id)

    /// `id` は openapi の `PokeType` の英小文字そのもの(完全一致)。未知の ID は nil を返す
    /// (既定色に黙って落とさない。エンブレムの代替表示は画面側の責務)。
    public static func rgba(forTypeID id: String) -> RGBA? {
        colorsByTypeID[id]
    }

    public static func color(forTypeID id: String) -> Color? {
        colorsByTypeID[id]?.color
    }
}

/// design.md「文字」。iOS は SF Pro Rounded(`.rounded` デザインの system font で得られる)。
public enum TextStyleToken: CaseIterable {
    /// 結果の%表示(28pt)。
    case resultPercent
    /// 見出し(17pt)。
    case heading
    /// 本文(15pt)。
    case body
    /// 補足(12pt)。
    case caption

    public var size: CGFloat {
        switch self {
        case .resultPercent: return 28
        case .heading: return 17
        case .body: return 15
        case .caption: return 12
        }
    }

    public var font: Font {
        let base = Self.dynamicRoundedFont(size: size)
        // design.md「数字は等幅(.monospacedDigit())」: 結果の%表示は数字が入れ替わっても
        // レイアウトが揺れないよう等幅数字にする。他のスタイルは通常の(可変幅の)数字でよい。
        switch self {
        case .resultPercent: return base.monospacedDigit()
        case .heading, .body, .caption: return base
        }
    }

    /// SF Pro Rounded で、`size` を「既定の文字サイズでの基準値」として端末の Dynamic Type
    /// (アクセシビリティの文字サイズ設定)に応じて拡大される固定デザインのフォント。
    /// `size` 自体(design.md の 28/17/15/12)は変えない(`DesignTokenTests` が固定する値)。
    private static func dynamicRoundedFont(size: CGFloat) -> Font {
        #if canImport(UIKit)
        let base = UIFont.systemFont(ofSize: size, weight: .regular)
        let rounded = base.fontDescriptor.withDesign(.rounded).map { UIFont(descriptor: $0, size: size) } ?? base
        return Font(UIFontMetrics(forTextStyle: .body).scaledFont(for: rounded))
        #else
        // macOS はパッケージテストのためだけの対象(Dynamic Type の主戦場は iOS)。固定サイズで返す。
        return Font.system(size: size, design: .rounded)
        #endif
    }
}

/// design.md「形・余白」の角丸。
public enum RadiusToken {
    /// カード。
    public static let card: CGFloat = 20
    /// ボタン・チップ(ピル)。
    public static let pill: CGFloat = 999
    /// 入力。
    public static let input: CGFloat = 12
}

/// design.md「形・余白」の余白(4 の倍数)。名前は 4 の何倍かを表す。
public enum SpacingToken {
    public static let x1: CGFloat = 4
    public static let x2: CGFloat = 8
    public static let x3: CGFloat = 12
    public static let x4: CGFloat = 16
    public static let x6: CGFloat = 24
    /// 小さい順の一覧(design.md の「4, 8, 12, 16, 24」)。
    public static let all: [CGFloat] = [x1, x2, x3, x4, x6]
}
