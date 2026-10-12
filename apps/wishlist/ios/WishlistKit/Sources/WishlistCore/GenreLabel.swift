import Foundation

/// ジャンルのボタンに出す名前の規則(phase6-spec 6-2)。選ばれているものだけ絵柄の隣に名前を出す。
public enum GenreLabel {
    /// ホームの「すべて」
    public static let allName = "すべて"
    /// 名前の最大幅(pt)。View はこの値で `frame(maxWidth:)` し、超えたら末尾を省略する
    public static let maxNameWidth = 140.0

    /// 選ばれているときだけ前後の空白を除いた名前。選ばれていない・空白だけなら nil。文字列は切らない。
    public static func visibleName(_ name: String, selected: Bool) -> String? {
        guard selected else { return nil }
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }
}

/// 地の色の上に置く文字・線画の色(WCAG の相対輝度で、白と濃い色のうち対比の高いほう)。Web の lib/contrast.ts と同じ。
public enum GenreContrast {
    /// 濃い側の色(Palette.fg と同じ)
    public static let darkHex = "#0e1a2b"

    /// `#rrggbb` の相対輝度(0〜1)。読めなければ nil。
    public static func luminance(hex: String) -> Double? {
        let text = hex.trimmingCharacters(in: CharacterSet(charactersIn: "#"))
        guard text.count == 6, let value = UInt64(text, radix: 16) else { return nil }
        func channel(_ v: UInt64) -> Double {
            let s = Double(v) / 255
            return s <= 0.03928 ? s / 12.92 : pow((s + 0.055) / 1.055, 2.4)
        }
        return 0.2126 * channel((value >> 16) & 255) + 0.7152 * channel((value >> 8) & 255) + 0.0722 * channel(value & 255)
    }

    /// 地が `background` のとき、濃い文字のほうが読みやすければ true(読めない色は白 = false)。
    public static func prefersDarkText(onHex background: String) -> Bool {
        guard let l = luminance(hex: background), let dark = luminance(hex: darkHex) else { return false }
        return (l + 0.05) / (dark + 0.05) > 1.05 / (l + 0.05)
    }
}
