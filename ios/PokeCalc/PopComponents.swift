import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// PopComponents: ポップ・カラフルの共通の部品(ADR-0521 / F-12。Web の `.ui-*` と対応)。
// 画面は配置だけを持ち、色・角丸・影・押下の動きはここのトークンだけを参照する(色の直書き禁止)。
// 常時動くアニメーションは持たない。押下の縮みは操作時のみで、「視差効果を減らす」では 0 秒。

// MARK: - 背景・カード

/// 部品の見た目の定数。
enum PopMetrics {
    /// カード上端の帯(装飾)の高さ。
    static let cardBandHeight: CGFloat = 4
    /// 装飾アイコンの大きさの基準(Dynamic Type に追従する)。
    static let iconSize: CGFloat = 15
}

extension View {
    /// 画面の背景: やさしいグラデーション(bgGradientStart → bgGradientEnd)。下地は bgBase。
    func popScreenBackground() -> some View {
        background {
            ZStack {
                ColorToken.bgBase.color
                LinearGradient(
                    colors: [ColorToken.bgGradientStart.color, ColorToken.bgGradientEnd.color],
                    startPoint: .top, endPoint: .bottom)
            }
            .ignoresSafeArea()
        }
    }

    /// カード(`.ui-card`): surfaceCard の面・カードの影・ヘアラインの枠。上端の帯を `typeID` のタイプ色で
    /// 染める(未選択・未知のタイプはブランド色)。帯は装飾で、支援技術には出さない。
    func popCard(typeID: String? = nil, cornerRadius: CGFloat = RadiusToken.card) -> some View {
        modifier(PopCardModifier(typeID: typeID, cornerRadius: cornerRadius, inset: false))
    }

    /// カードの中の入れ子の面(`popCard` の内側。影と帯なし。tableZebra の面)。
    func popInset(cornerRadius: CGFloat = RadiusToken.input) -> some View {
        modifier(PopCardModifier(typeID: nil, cornerRadius: cornerRadius, inset: true))
    }

    /// ゼブラの一覧の行(`.ui-rows`): 入力の角丸・ヘアラインの枠。偶数行(1 始まり。`index` は 0 始まり)を
    /// tableZebra、奇数行を surfaceCard で染める。押下中の行は `PopRowButtonStyle` が tableHover にする。
    func popRow(index: Int, isPressed: Bool = false) -> some View {
        let shape = RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
        return background(PopRowStyle.fill(index: index, isPressed: isPressed), in: shape)
            .overlay(shape.stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
    }

    /// 案内の淡い塗り(読み込み中・情報など。`PopNoticeView` と同じ見た目を既存の View に当てる)。
    func popNotice(_ kind: PopNoticeKind) -> some View {
        padding(SpacingToken.x3)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(kind.fill, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
    }
}

private struct PopCardModifier: ViewModifier {
    let typeID: String?
    let cornerRadius: CGFloat
    let inset: Bool

    private var accent: Color {
        typeID.flatMap { TypeColorToken.color(forTypeID: $0) } ?? ColorToken.brandPrimary.color
    }

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
        if inset {
            content
                .background(ColorToken.tableZebra.color, in: shape)
                .overlay(shape.stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        } else {
            content
                .background(ColorToken.surfaceCard.color, in: shape)
                .overlay(alignment: .top) {
                    // 帯は大きいカードだけ(入力の角丸の行・セレクタは面と影だけ)。
                    if cornerRadius >= RadiusToken.card {
                        UnevenRoundedRectangle(
                            topLeadingRadius: cornerRadius, bottomLeadingRadius: 0,
                            bottomTrailingRadius: 0, topTrailingRadius: cornerRadius, style: .continuous
                        )
                        .fill(accent)
                        .frame(height: PopMetrics.cardBandHeight)
                        .accessibilityHidden(true)
                    }
                }
                .overlay(shape.stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
                .shadow(
                    color: ColorToken.shadowColor.color, radius: ShadowToken.card.radius, x: 0,
                    y: ShadowToken.card.offsetY)
        }
    }
}

/// ゼブラの行の塗り。
enum PopRowStyle {
    static func fill(index: Int, isPressed: Bool) -> Color {
        if isPressed { return ColorToken.tableHover.color }
        return index % 2 == 1 ? ColorToken.tableZebra.color : ColorToken.surfaceCard.color
    }
}

/// 行全体がボタンのときの見た目(ゼブラ + 押下中は tableHover。縮みはしない)。
struct PopRowButtonStyle: ButtonStyle {
    let index: Int

    func makeBody(configuration: Configuration) -> some View {
        configuration.label.popRow(index: index, isPressed: configuration.isPressed)
    }
}

// MARK: - アイコン

/// 装飾の SF Symbols(`.ui-icon`)。文字色に従い、支援技術からは隠す(意味は必ず隣の文字が持つ)。
struct PopIcon: View {
    let systemName: String
    @ScaledMetric private var size = PopMetrics.iconSize

    init(_ systemName: String) { self.systemName = systemName }

    var body: some View {
        Image(systemName: systemName)
            .font(.system(size: size, weight: .semibold, design: .rounded))
            .accessibilityHidden(true)
    }
}

/// 各画面・操作の SF Symbols の割り当て(意味の合うものを1か所で決める)。
enum PopSymbol {
    static let calc = "function"
    static let reverse = "arrow.uturn.backward.circle"
    static let speed = "hare"
    static let balance = "chart.pie"
    static let judge = "checkmark.seal"
    static let team = "person.3"
    static let favorites = "star"
    static let adjust = "slider.horizontal.3"
    static let about = "info.circle"
    static let swap = "arrow.left.arrow.right"
    static let add = "plus"
    static let delete = "trash"
    static let edit = "pencil"
    static let search = "magnifyingglass"
    static let warning = "exclamationmark.triangle"
    static let info = "info.circle"
    static let done = "checkmark.circle"
    static let history = "clock.arrow.circlepath"
    static let reload = "arrow.clockwise"
    static let save = "square.and.arrow.down"

    /// ルートの入口(`AppFeature.id`)に付ける装飾アイコン。未知の ID は汎用の「情報」。
    static func forFeature(id: String) -> String {
        switch id {
        case "calc": return calc
        case "reverse": return reverse
        case "speed": return speed
        case "balance": return balance
        case "judge": return judge
        case "team": return team
        case "favorites": return favorites
        case "adjust": return adjust
        case "about": return about
        default: return info
        }
    }
}

/// 見出し(アイコン + 文字)。`.ui-heading`。文字は見出しの特性を持つ。
struct PopHeading: View {
    let title: String
    let systemImage: String

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            PopIcon(systemImage)
                .foregroundStyle(ColorToken.brandPrimary.color)
            Text(title)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// 文字の前に装飾アイコンを置いたラベル(ボタン・行用)。アイコンは読み上げない(文字ラベルが名前)。
struct PopLabel: View {
    let title: String
    let systemImage: String

    var body: some View {
        HStack(spacing: SpacingToken.x2) {
            PopIcon(systemImage)
            Text(title).multilineTextAlignment(.center)
        }
    }
}

// MARK: - ボタン・チップ

/// ボタンの種類(`.ui-button--primary / --secondary / --danger`)。
enum PopButtonKind {
    case primary, secondary, danger

    var fill: Color {
        switch self {
        case .primary: return ColorToken.brandPrimary.color
        case .secondary: return ColorToken.surfaceCard.color
        case .danger: return ColorToken.danger.color
        }
    }

    var ink: Color {
        switch self {
        case .primary: return ColorToken.onPrimary.color
        case .secondary: return ColorToken.brandPrimary.color
        case .danger: return ColorToken.onDanger.color
        }
    }
}

/// ピルのボタン(`.ui-button`)。押下で 0.96 に縮む(操作時のみ。視差効果を減らす で 0 秒)。
/// 押している間は影を浮き上がらせず、薄くもしない(縮みだけが反応)。無効のときは減光。
struct PillButtonStyle: ButtonStyle {
    var kind: PopButtonKind = .secondary

    func makeBody(configuration: Configuration) -> some View {
        PillButtonBody(kind: kind, configuration: configuration)
    }
}

private struct PillButtonBody: View {
    let kind: PopButtonKind
    let configuration: ButtonStyleConfiguration
    @Environment(\.isEnabled) private var isEnabled
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        configuration.label
            .font(TextStyleToken.heading.font)
            .foregroundStyle(kind.ink)
            .padding(.horizontal, SpacingToken.x6)
            .padding(.vertical, SpacingToken.x3)
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            .background(kind.fill, in: Capsule())
            .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            .opacity(isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity * 1.5)
            .scaleEffect(configuration.isPressed ? MotionToken.pressScale : 1)
            .animation(
                .easeOut(duration: MotionToken.pressDuration(reduceMotion: reduceMotion)),
                value: configuration.isPressed)
    }
}

/// チップ・タブの選択中の塗り(brandPrimary)/ 未選択(tableZebra。カードの上でも背景の上でも見分けられる)。
enum PopChipStyle {
    static func fill(isSelected: Bool) -> Color {
        isSelected ? ColorToken.brandPrimary.color : ColorToken.tableZebra.color
    }

    static func foreground(isSelected: Bool) -> Color {
        isSelected ? ColorToken.onPrimary.color : ColorToken.textPrimary.color
    }
}

// MARK: - 案内

/// 案内の種類(`.ui-notice--empty / --error / --loading / --info`)。
/// 色だけで意味を伝えない: 種類ごとのアイコンと、呼び出し側の文字を必ず併記する。
enum PopNoticeKind {
    case empty, error, loading, info

    var fill: Color {
        switch self {
        case .empty: return ColorToken.surfaceCard.color
        case .error: return ColorToken.dangerSoft.color
        case .loading, .info: return ColorToken.infoSoft.color
        }
    }

    var ink: Color {
        switch self {
        case .empty: return ColorToken.textSecondary.color
        case .error: return ColorToken.danger.color
        case .loading, .info: return ColorToken.info.color
        }
    }

    var symbol: String {
        switch self {
        case .empty: return PopSymbol.info
        case .error: return PopSymbol.warning
        case .loading: return PopSymbol.reload
        case .info: return PopSymbol.info
        }
    }
}

/// 案内の囲み。文字の識別子は `identifier`(従来の `ErrorBannerView` と同じ。子を畳まない)。
struct PopNoticeView: View {
    let kind: PopNoticeKind
    let message: String
    var identifier: String

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            PopIcon(kind.symbol)
            Text(message)
                .font(TextStyleToken.body.font)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier(identifier)
        }
        .foregroundStyle(kind.ink)
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(kind.fill, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
    }
}
