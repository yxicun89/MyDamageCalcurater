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
        light: RGBA(red: 0xCD, green: 0x1D, blue: 0x23, alpha: 1.0),
        dark: RGBA(red: 0xFF, green: 0x63, blue: 0x69, alpha: 1.0)
    )
}

/// design.md「ポップ配色」(ADR-0334 / F-12)。名前は Web の CSS 変数(`--brand-primary` 等)の camelCase。
/// `-ink` で終わる名前と `type` で始まる名前は使わない(タイプバッジ・タイプ色の命名)。
extension ColorToken {
    private static func solid(_ light: Int, _ dark: Int) -> ColorPair {
        func rgba(_ hex: Int) -> RGBA {
            RGBA(red: (hex >> 16) & 0xFF, green: (hex >> 8) & 0xFF, blue: hex & 0xFF, alpha: 1.0)
        }
        return ColorPair(light: rgba(light), dark: rgba(dark))
    }

    /// 主ボタン・選択中のタブとチップの塗り・リンク。
    public static let brandPrimary = solid(0x1F5FD6, 0x7FA8FF)
    /// `brandPrimary` の塗りの上の文字。
    public static let onPrimary = solid(0xFFFFFF, 0x0E1015)
    /// 塗りの装飾だけ(星・ハイライト)。文字色・境界線には使わない。
    public static let brandAccent = solid(0xFFCB05, 0xFFD84D)
    /// `brandAccent` の塗りの上の文字。
    public static let onAccent = solid(0x14161A, 0x14161A)
    /// 成功の文字・アイコン。
    public static let success = solid(0x17743A, 0x5FD38A)
    /// 成功の案内の塗り。
    public static let successSoft = solid(0xE2F5E8, 0x12301F)
    /// 注意の文字・アイコン。
    public static let warning = solid(0x9A5B00, 0xFFB547)
    /// 注意の案内の塗り。
    public static let warningSoft = solid(0xFFF1D6, 0x33240B)
    /// 情報・読み込み中の文字・アイコン。
    public static let info = solid(0x0B6BA8, 0x5EC2F2)
    /// 情報・読み込み中の案内の塗り。
    public static let infoSoft = solid(0xDCEFFB, 0x0C2A3A)
    /// エラーの案内の塗り(文字は `danger`)。
    public static let dangerSoft = solid(0xFDE3E4, 0x3A1416)
    /// `danger` の塗り(危険ボタン)の上の文字。
    public static let onDanger = solid(0xFFFFFF, 0x0E1015)
    /// カード・ボタン(副)の不透明な面。
    public static let surfaceCard = solid(0xFFFFFF, 0x1A1D24)
    /// 背景のやさしいグラデーション(上)。
    public static let bgGradientStart = solid(0xFFF6E0, 0x14131C)
    /// 背景のやさしいグラデーション(下)。
    public static let bgGradientEnd = solid(0xE8F1FF, 0x0E1622)
    /// 表の見出し行。
    public static let tableHeader = solid(0xDCE7FB, 0x1C2638)
    /// 表・一覧の偶数行。
    public static let tableZebra = solid(0xEEF2F8, 0x151922)
    /// 表・一覧の押下中の行(Web ではホバー)。
    public static let tableHover = solid(0xE3ECFB, 0x1D2535)
    /// フォーカスの輪。
    public static let focusRing = solid(0x1F5FD6, 0x7FA8FF)
    /// 影の色(黒 10% / 40%)。
    public static let shadowColor = ColorPair(
        light: RGBA(red: 0, green: 0, blue: 0, alpha: 0.10),
        dark: RGBA(red: 0, green: 0, blue: 0, alpha: 0.40)
    )

    /// ポップ配色の全トークン(名前つき。テストと一覧用)。
    public static let popPalette: [(name: String, pair: ColorPair)] = [
        ("brandPrimary", brandPrimary), ("onPrimary", onPrimary), ("brandAccent", brandAccent),
        ("onAccent", onAccent), ("success", success), ("successSoft", successSoft),
        ("warning", warning), ("warningSoft", warningSoft), ("info", info), ("infoSoft", infoSoft),
        ("dangerSoft", dangerSoft), ("onDanger", onDanger), ("surfaceCard", surfaceCard),
        ("bgGradientStart", bgGradientStart), ("bgGradientEnd", bgGradientEnd),
        ("tableHeader", tableHeader), ("tableZebra", tableZebra), ("tableHover", tableHover),
        ("focusRing", focusRing), ("shadowColor", shadowColor),
    ]
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

    // MARK: - 文字色(design.md「タイプバッジ」。P6-21)

    /// 背景色 `background`(不透明)の上に載せる文字色。黒 #000000 か白 #FFFFFF のうち、WCAG 2.2 の
    /// コントラスト比が高い方(同値なら黒)。
    public static func preferredInk(over background: RGBA) -> RGBA {
        let black = RGBA(red: 0, green: 0, blue: 0, alpha: 1.0)
        let white = RGBA(red: 255, green: 255, blue: 255, alpha: 1.0)
        let backgroundLuminance = relativeLuminance(background)
        let blackContrast = (backgroundLuminance + 0.05) / (relativeLuminance(black) + 0.05)
        let whiteContrast = (relativeLuminance(white) + 0.05) / (backgroundLuminance + 0.05)
        return whiteContrast > blackContrast ? white : black
    }

    /// WCAG 2.2 の相対輝度(0〜1)。sRGB の1チャンネル(0〜255)を線形値にして重み付けする。
    private static func relativeLuminance(_ color: RGBA) -> Double {
        func linear(_ channel255: Int) -> Double {
            let channel = Double(channel255) / 255
            return channel <= 0.03928 ? channel / 12.92 : pow((channel + 0.055) / 1.055, 2.4)
        }
        return 0.2126 * linear(color.red) + 0.7152 * linear(color.green) + 0.0722 * linear(color.blue)
    }

    /// タイプ ID のバッジ・エンブレム文字色(黒/白)。未知の ID は nil。
    /// `rgba(forTypeID:)` から `preferredInk(over:)` で導く(文字色の表を二重に持たない)。
    public static func ink(forTypeID id: String) -> RGBA? {
        rgba(forTypeID: id).map(preferredInk(over:))
    }

    public static func inkColor(forTypeID id: String) -> Color? {
        ink(forTypeID: id)?.color
    }
}

/// design.md「文字」。iOS は SF Pro Rounded(`.rounded` デザインの system font で得られる)。
public enum TextStyleToken: CaseIterable {
    /// タイトル(22pt・太さ 800。画面の最上位の見出し)。
    case title
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
        case .title: return 22
        case .resultPercent: return 28
        case .heading: return 17
        case .body: return 15
        case .caption: return 12
        }
    }

    /// 太さ(design.md「文字」。タイトル 800 / 見出し 700 / 本文・補足 400。結果の%は従来どおり 400)。
    public var weight: FontWeightToken {
        switch self {
        case .title: return .title
        case .heading: return .heading
        case .resultPercent, .body, .caption: return .body
        }
    }

    public var font: Font {
        let base = Self.dynamicRoundedFont(size: size, weight: weight.weight)
        // design.md「数字は等幅(.monospacedDigit())」: 結果の%表示は数字が入れ替わっても
        // レイアウトが揺れないよう等幅数字にする。他のスタイルは通常の(可変幅の)数字でよい。
        switch self {
        case .resultPercent: return base.monospacedDigit()
        case .title, .heading, .body, .caption: return base
        }
    }

    /// SF Pro Rounded で、`size` を「既定の文字サイズでの基準値」として端末の Dynamic Type
    /// (アクセシビリティの文字サイズ設定)に応じて拡大される固定デザインのフォント。
    /// `size` 自体(design.md の 28/17/15/12)は変えない(`DesignTokenTests` が固定する値)。
    private static func dynamicRoundedFont(size: CGFloat, weight: Font.Weight) -> Font {
        #if canImport(UIKit)
        let base = UIFont.systemFont(ofSize: size, weight: weight.uiFontWeight)
        let rounded = base.fontDescriptor.withDesign(.rounded).map { UIFont(descriptor: $0, size: size) } ?? base
        return Font(UIFontMetrics(forTextStyle: .body).scaledFont(for: rounded))
        #else
        // macOS はパッケージテストのためだけの対象(Dynamic Type の主戦場は iOS)。固定サイズで返す。
        return Font.system(size: size, weight: weight, design: .rounded)
        #endif
    }
}

/// design.md「文字」の太さ(タイトル 800 / 見出し 700 / 強調 700 / 本文 400。ADR-0334)。
public enum FontWeightToken: CaseIterable, Sendable {
    case title
    case heading
    case strong
    case body

    /// CSS の font-weight の数値(Web と同じ)。
    public var cssValue: Int {
        switch self {
        case .title: return 800
        case .heading, .strong: return 700
        case .body: return 400
        }
    }

    public var weight: Font.Weight {
        switch self {
        case .title: return .heavy
        case .heading, .strong: return .bold
        case .body: return .regular
        }
    }
}

#if canImport(UIKit)
extension Font.Weight {
    fileprivate var uiFontWeight: UIFont.Weight {
        switch self {
        case .heavy: return .heavy
        case .bold: return .bold
        default: return .regular
        }
    }
}
#endif

/// design.md「形・余白」の影(色は `ColorToken.shadowColor`)。Web の `box-shadow: 0 Ypx BLURpx` に対応。
public struct ShadowToken: Equatable, Sendable {
    public let offsetY: CGFloat
    public let blur: CGFloat
    /// SwiftUI の `.shadow(radius:)` は CSS の blur の半分。
    public var radius: CGFloat { blur / 2 }

    /// カード 0 2px 8px。
    public static let card = ShadowToken(offsetY: 2, blur: 8)
    /// 浮き上がり 0 6px 16px。
    public static let raised = ShadowToken(offsetY: 6, blur: 16)
}

/// design.md「動き」の押下(ADR-0334)。常時動くアニメーションは持たない。
public enum MotionToken {
    /// ボタン・チップを押したとき少し縮む時間(秒)。
    public static let pressDuration: Double = 0.15
    /// 押下で縮む倍率。
    public static let pressScale: Double = 0.96

    /// 「視差効果を減らす」のときは 0 秒。
    public static func pressDuration(reduceMotion: Bool) -> Double {
        reduceMotion ? 0 : pressDuration
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
