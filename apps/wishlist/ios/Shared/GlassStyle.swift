import SwiftUI
import WishlistCore

// 配色・角丸・書体(docs/phase5-spec.md 5-2「デザインの決まり」。Web の styles.css のトークンと同じ値)。
// 本文のカード・タイルは「白 + 1px の線」。Liquid Glass はシートとツールバー(閉じるなどの丸いボタン)だけに使う。

extension Color {
    /// `#rrggbb`(先頭の `#` は省略可)。読めなければ強調色。
    init(hex: String) {
        var value: UInt64 = 0
        let text = hex.trimmingCharacters(in: CharacterSet(charactersIn: "#"))
        guard text.count == 6, Scanner(string: text).scanHexInt64(&value) else {
            self = Palette.accent
            return
        }
        self.init(
            .sRGB, red: Double((value >> 16) & 0xff) / 255, green: Double((value >> 8) & 0xff) / 255,
            blue: Double(value & 0xff) / 255, opacity: 1)
    }
}

enum Palette {
    static let bg = Color(hex: "#edf2f8")
    static let surface = Color.white
    static let line = Color(hex: "#d9e2ee")
    static let lineSoft = Color(hex: "#e3e9f1")
    static let fg = Color(hex: "#0e1a2b")
    static let muted = Color(hex: "#56677d")
    static let accent = Color(hex: GenreIcon.defaultColor)
    static let onAccent = Color.white
    static let accentSoft = accent.opacity(0.2)
    static let danger = Color(hex: "#d92d20")
    static let warn = Color(hex: "#c2410c")
    static let disabled = Color(hex: "#c9d5e4")
    /// 価格の札・長押しの背景の暗い地
    static let scrim = fg.opacity(0.28)
    static let tagBackground = fg.opacity(0.6)
}

enum Metrics {
    static let tileHeight: CGFloat = 220
    static let tileRadius: CGFloat = 22
    static let cardRadius: CGFloat = 18
    static let buttonRadius: CGFloat = 14
    static let ringOpacity = 0.35
    static let gridSpacing: CGFloat = 16
}

extension Font {
    /// 丸ゴシック(`.rounded`)。見出し・価格・ボタンは 800(`.heavy`)、本文は 500(`.medium`)。
    static func wl(_ size: CGFloat, _ weight: Font.Weight = .medium) -> Font {
        .system(size: size, weight: weight, design: .rounded)
    }
}

extension View {
    /// ジャンル色の細い枠(不透明度 約 35%)と薄い影。
    func genreRing(_ hex: String, radius: CGFloat = Metrics.tileRadius) -> some View {
        overlay(
            RoundedRectangle(cornerRadius: radius, style: .continuous)
                .strokeBorder(Color(hex: hex).opacity(Metrics.ringOpacity), lineWidth: 1.5)
        )
        .shadow(color: .black.opacity(0.07), radius: 8, y: 6)
    }

    /// 白いカード + 1px の線。
    func card(radius: CGFloat = Metrics.cardRadius) -> some View {
        background(Palette.surface, in: RoundedRectangle(cornerRadius: radius, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: radius, style: .continuous).strokeBorder(Palette.line, lineWidth: 1))
    }
}

/// ジャンルの絵柄の線画(24x24 の svg を `stroke` 2・`fill` なしで描く)。パスは WishlistCore の `GenreGlyph`。
private struct GlyphShape: Shape {
    let subpaths: [GlyphSubpath]

    func path(in rect: CGRect) -> Path {
        let scale = rect.width / GenreGlyph.size
        var path = Path()
        for subpath in subpaths {
            guard let first = subpath.points.first else { continue }
            path.move(to: CGPoint(x: rect.minX + first.x * scale, y: rect.minY + first.y * scale))
            for point in subpath.points.dropFirst() {
                path.addLine(to: CGPoint(x: rect.minX + point.x * scale, y: rect.minY + point.y * scale))
            }
            if subpath.isClosed { path.closeSubpath() }
        }
        return path
    }
}

struct GlyphIcon: View {
    let subpaths: [GlyphSubpath]
    var color: Color
    var size: CGFloat = 24

    init(icon: GenreIcon, color: Color, size: CGFloat = 24) {
        subpaths = GenreGlyph.subpaths(for: icon)
        self.color = color
        self.size = size
    }

    /// 「すべて」専用の絵柄
    init(all color: Color, size: CGFloat = 24) {
        subpaths = GenreGlyph.subpaths(of: GenreGlyph.allPath)
        self.color = color
        self.size = size
    }

    var body: some View {
        GlyphShape(subpaths: subpaths)
            .stroke(color, style: StrokeStyle(lineWidth: 2 * size / GenreGlyph.size, lineCap: .round, lineJoin: .round))
            .frame(width: size, height: size)
            .accessibilityHidden(true)
    }
}

/// サイトのマーク(サイト名の頭文字・サイトの色の地)。
struct SiteMarkView: View {
    let name: String
    var size: CGFloat = 30
    var dim = false

    var body: some View {
        Text(SiteMark.mark(for: name))
            .font(.wl(size * 0.5, .heavy))
            .foregroundStyle(.white)
            .frame(width: size, height: size)
            .background(Color(hex: SiteMark.colorHex(for: name)).opacity(dim ? 0.45 : 1), in: RoundedRectangle(cornerRadius: size / 3, style: .continuous))
            .accessibilityHidden(true)
    }
}

/// 「オフライン」の札(高さ 32・暗い地・白い文字・雲の斜線)。
struct OfflineBadge: View {
    var body: some View {
        HStack(spacing: 6) {
            Image(systemName: "icloud.slash").font(.system(size: 13, weight: .bold))
            Text(WishlistText.offline).font(.wl(13, .heavy))
        }
        .foregroundStyle(.white)
        .padding(.horizontal, 12)
        .frame(height: 32)
        .background(Palette.fg, in: Capsule())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(WishlistText.offline)
        .accessibilityIdentifier("offlineBadge")
    }
}

/// 丸い Liquid Glass のボタン(シートの「閉じる」など。ツールバー相当の場所だけ)。
struct GlassCircleButton: View {
    let systemImage: String
    let label: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemImage)
                .font(.system(size: 16, weight: .bold))
                .foregroundStyle(Palette.fg)
                .frame(width: 44, height: 44)
        }
        .buttonStyle(.plain)
        .glassEffect(.regular, in: Circle())
        .accessibilityLabel(label)
    }
}

/// 主ボタン(強調色の地・白い文字)。押せないときは `--disabled`。
struct PrimaryButtonStyle: ButtonStyle {
    var height: CGFloat = 58
    var radius: CGFloat = 18

    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.wl(17, .heavy))
            .foregroundStyle(Palette.onAccent)
            .frame(maxWidth: .infinity)
            .frame(height: height)
            .background(isEnabled ? Palette.accent : Palette.disabled, in: RoundedRectangle(cornerRadius: radius, style: .continuous))
            .opacity(configuration.isPressed ? 0.85 : 1)
    }
}

/// 白い副ボタン(1px の線)。
struct OutlineButtonStyle: ButtonStyle {
    var height: CGFloat = 56

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.wl(16, .heavy))
            .foregroundStyle(Palette.fg)
            .frame(maxWidth: .infinity)
            .frame(height: height)
            .card(radius: 16)
            .opacity(configuration.isPressed ? 0.8 : 1)
    }
}

/// ジャンルのボタンの共通の見た目(phase6-spec 6-2)。選ばれているときだけ絵柄の隣に名前を出す丸薬になる。
/// 名前は `GenreLabel.visibleName` が決め、1 行・`GenreLabel.maxNameWidth` で末尾を省略する。ボタンの高さは変えない。
struct GenrePill<Glyph: View>: View {
    let name: String
    let selected: Bool
    let background: Color
    let foreground: Color
    let size: CGFloat
    let radius: CGFloat
    let nameIdentifier: String
    let buttonIdentifier: String
    @ViewBuilder let glyph: (Color) -> Glyph
    let action: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        let shown = GenreLabel.visibleName(name, selected: selected)
        Button(action: action) {
            HStack(spacing: 8) {
                // 名前が出ているときは絵柄の枠を詰め、余白を Web の丸薬(左 14・絵柄・8・名前・右 16)にそろえる
                glyph(selected ? foreground : background)
                    .frame(width: shown == nil ? size : nil, height: size)
                    .fixedSize()
                if let shown {
                    Text(shown)
                        .font(.subheadline.weight(.bold))
                        // 丸薬の高さは固定なので、文字の拡大はここで止める(読み上げの名前は全文のまま)
                        .dynamicTypeSize(...DynamicTypeSize.xxxLarge)
                        .lineLimit(1)
                        .truncationMode(.tail)
                        .frame(maxWidth: GenreLabel.maxNameWidth, alignment: .leading)
                        .foregroundStyle(foreground)
                        .accessibilityIdentifier(nameIdentifier)
                }
            }
            .padding(.leading, shown == nil ? 0 : 14)
            .padding(.trailing, shown == nil ? 0 : 16)
            .frame(minWidth: size, minHeight: size, maxHeight: size)
            .background(selected ? background : Palette.surface, in: RoundedRectangle(cornerRadius: radius, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: radius, style: .continuous).strokeBorder(Palette.line, lineWidth: selected ? 0 : 1))
        }
        .buttonStyle(.plain)
        .animation(reduceMotion ? nil : .easeInOut(duration: 0.12), value: selected)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(name)
        .accessibilityAddTraits(selected ? .isSelected : [])
        .accessibilityIdentifier(buttonIdentifier)
    }
}

/// ジャンルのボタン(選ぶ場面)。選択は `genreColor` 地+(地の明るさで白か濃い色の)線画と名前、非選択は白地+ジャンル色の線画。
struct GenreIconButton: View {
    let genre: Genre
    let selected: Bool
    var size: CGFloat = 60
    var radius: CGFloat = 18
    let action: () -> Void

    var body: some View {
        GenrePill(
            name: genre.name, selected: selected, background: Color(hex: genre.color),
            foreground: GenreContrast.prefersDarkText(onHex: genre.color) ? Palette.fg : .white,
            size: size, radius: radius, nameIdentifier: "genreChoiceName-\(genre.id)",
            buttonIdentifier: "genreChoice-\(genre.id)",
            glyph: { GlyphIcon(icon: genre.icon, color: $0, size: size * 0.45) }, action: action)
    }
}

/// ジャンルを選ぶ絵柄の並び(`role="group" aria-label="ジャンル"`)。
struct GenreChooser: View {
    let genres: [Genre]
    @Binding var selection: Int?
    var size: CGFloat = 60
    var radius: CGFloat = 18
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 12) {
                    ForEach(genres) { genre in
                        GenreIconButton(genre: genre, selected: selection == genre.id, size: size, radius: radius) {
                            selection = genre.id
                        }
                        .id(genre.id)
                    }
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 10)
            }
            // 選んだ丸薬が列の端で見切れないよう列の中に入れる(視差効果を減らすときは動かさず切り替える)
            .onChange(of: selection) { _, new in
                guard let new else { return }
                if reduceMotion {
                    proxy.scrollTo(new, anchor: nil)
                } else {
                    withAnimation(.easeInOut(duration: 0.12)) { proxy.scrollTo(new, anchor: nil) }
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("ジャンル")
    }
}
