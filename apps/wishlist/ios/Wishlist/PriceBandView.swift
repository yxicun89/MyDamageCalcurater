import SwiftUI
import WishlistCore

/// 価格の帯(高さ 34): 4px の線と、安い〜中央の太線(8px・`accentSoft`)、各サイトの点(28px の丸・サイトの色・頭文字・2px の白縁)。
/// 点の位置は `BandDot.fraction`(0...1)を、両端に 6% の余白を取って置く。全部同額のときは中央に置き、太線は描かない(`rangeBar == nil`)。
struct PriceBandView: View {
    let band: PriceBand
    let sites: [Site]

    private let dotSize: CGFloat = 28
    private let margin = 0.06

    var body: some View {
        GeometryReader { proxy in
            let width = proxy.size.width
            ZStack(alignment: .leading) {
                Capsule().fill(Palette.line).frame(height: 4)
                if let bar = band.rangeBar {
                    Capsule().fill(Palette.accentSoft)
                        .frame(width: width * (bar.upperBound - bar.lowerBound), height: 8)
                        .offset(x: width * bar.lowerBound)
                }
                ForEach(band.dots, id: \.siteID) { dot in
                    let name = sites.first { $0.id == dot.siteID }?.name ?? ""
                    SiteMarkDot(name: name, size: dotSize)
                        .offset(x: width * (margin + dot.fraction * (1 - 2 * margin)) - dotSize / 2)
                        .accessibilityElement(children: .ignore)
                        .accessibilityLabel(name)
                        .accessibilityIdentifier("bandDot-\(dot.siteID)")
                }
            }
            .frame(height: proxy.size.height)
        }
        .frame(height: 34)
    }
}

/// 帯の上の点(サイトの色の丸・頭文字・白い縁)
private struct SiteMarkDot: View {
    let name: String
    let size: CGFloat

    var body: some View {
        Text(SiteMark.mark(for: name))
            .font(.wl(size * 0.42, .heavy))
            .foregroundStyle(.white)
            .frame(width: size, height: size)
            .background(Color(hex: SiteMark.colorHex(for: name)), in: Circle())
            .overlay(Circle().strokeBorder(.white, lineWidth: 2))
    }
}
