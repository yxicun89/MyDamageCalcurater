import SwiftUI
import XCTest

@testable import PokeCalcDesign

/// docs/design.md「デザイントークン」の値を、実装の写しではなく design.md から**独立に**書き写して照合する
/// (coding-rules §2 の「独立した検証」)。design.md を変えたら、このテストの表も同じ変更で直す。
///
/// 名前の対応(design.md → Swift): `bg.base` → `ColorToken.bgBase`、`bg.glass` → `bgGlass`、
/// `text.primary` → `textPrimary`、`text.secondary` → `textSecondary`、`border.hairline` → `borderHairline`、`danger` → `danger`。
final class DesignTokenTests: XCTestCase {

    // MARK: - ベースの色

    /// design.md「ベース」の表。「白 70%」= #FFFFFF の alpha 0.70、「#1A1D24 60%」= #1A1D24 の alpha 0.60、
    /// 「黒 8%」= #000000 の alpha 0.08、「白 10%」= #FFFFFF の alpha 0.10。% の無い色は不透明(alpha 1.0)。
    /// 「+ ぼかし」は色ではなく素材(Liquid Glass / material)の指定なので RGBA には含めない。
    func testBaseColorTokensMatchDesignDoc() {
        let cases: [(name: String, token: ColorPair, light: RGBA, dark: RGBA)] = [
            ("bg.base", ColorToken.bgBase,
             RGBA(red: 0xF4, green: 0xF5, blue: 0xF7, alpha: 1.0),
             RGBA(red: 0x0E, green: 0x10, blue: 0x15, alpha: 1.0)),
            ("bg.glass", ColorToken.bgGlass,
             RGBA(red: 0xFF, green: 0xFF, blue: 0xFF, alpha: 0.70),
             RGBA(red: 0x1A, green: 0x1D, blue: 0x24, alpha: 0.60)),
            ("text.primary", ColorToken.textPrimary,
             RGBA(red: 0x14, green: 0x16, blue: 0x1A, alpha: 1.0),
             RGBA(red: 0xF2, green: 0xF3, blue: 0xF5, alpha: 1.0)),
            ("text.secondary", ColorToken.textSecondary,
             RGBA(red: 0x5C, green: 0x62, blue: 0x70, alpha: 1.0),
             RGBA(red: 0xA3, green: 0xA9, blue: 0xB6, alpha: 1.0)),
            ("border.hairline", ColorToken.borderHairline,
             RGBA(red: 0x00, green: 0x00, blue: 0x00, alpha: 0.08),
             RGBA(red: 0xFF, green: 0xFF, blue: 0xFF, alpha: 0.10)),
            ("danger", ColorToken.danger,
             RGBA(red: 0xCD, green: 0x1D, blue: 0x23, alpha: 1.0),
             RGBA(red: 0xFF, green: 0x63, blue: 0x69, alpha: 1.0)),
        ]
        for testCase in cases {
            assertRGBA(testCase.token.light, testCase.light, "\(testCase.name) light")
            assertRGBA(testCase.token.dark, testCase.dark, "\(testCase.name) dark")
        }
    }

    /// RGBA は 0〜255 の整数チャンネルと 0〜1 の alpha を持つ。範囲外を作れない(または丸めない)ことは実装に任せ、
    /// ここでは全トークンが範囲内であることだけを見る。
    func testBaseColorChannelsAreInRange() {
        let pairs = [ColorToken.bgBase, ColorToken.bgGlass, ColorToken.textPrimary,
                     ColorToken.textSecondary, ColorToken.borderHairline, ColorToken.danger]
        for pair in pairs {
            for rgba in [pair.light, pair.dark] {
                for channel in [rgba.red, rgba.green, rgba.blue] {
                    XCTAssertTrue((0...255).contains(channel), "\(rgba)")
                }
                XCTAssertTrue((0.0...1.0).contains(rgba.alpha), "\(rgba)")
            }
        }
    }

    // MARK: - タイプ色

    /// design.md「タイプ色(自作パレット)」の表。キーは api/openapi.yaml の `PokeType` enum の英小文字 ID
    /// (normal, fire, ... fairy の 18 個)。日本語名との対応は design.md の表のとおり
    /// (ノーマル=normal、ほのお=fire、みず=water、でんき=electric、くさ=grass、こおり=ice、かくとう=fighting、
    /// どく=poison、じめん=ground、ひこう=flying、エスパー=psychic、むし=bug、いわ=rock、ゴースト=ghost、
    /// ドラゴン=dragon、あく=dark、はがね=steel、フェアリー=fairy)。タイプ色は不透明。
    private static let expectedTypeColors: [String: (Int, Int, Int)] = [
        "normal": (0x9F, 0xA1, 0x9F),
        "fire": (0xE8, 0x62, 0x2B),
        "water": (0x3B, 0x8F, 0xE0),
        "electric": (0xF2, 0xC2, 0x1B),
        "grass": (0x4C, 0xAF, 0x50),
        "ice": (0x5C, 0xC8, 0xD8),
        "fighting": (0xD0, 0x45, 0x3A),
        "poison": (0x9B, 0x51, 0xC6),
        "ground": (0xB8, 0x86, 0x3B),
        "flying": (0x7F, 0xA7, 0xE8),
        "psychic": (0xE8, 0x51, 0x7F),
        "bug": (0x93, 0xB2, 0x1E),
        "rock": (0xA9, 0x9A, 0x62),
        "ghost": (0x6B, 0x55, 0xA8),
        "dragon": (0x51, 0x60, 0xD8),
        "dark": (0x54, 0x47, 0x4A),
        "steel": (0x6E, 0x93, 0xA8),
        "fairy": (0xE6, 0x8A, 0xD6),
    ]

    func testTypeColorsMatchDesignDocForAll18Types() throws {
        XCTAssertEqual(Self.expectedTypeColors.count, 18, "テストの表自体が 18 タイプを持つこと")
        for (typeID, (red, green, blue)) in Self.expectedTypeColors.sorted(by: { $0.key < $1.key }) {
            let rgba = try XCTUnwrap(TypeColorToken.rgba(forTypeID: typeID), "\(typeID) の色が無い")
            assertRGBA(rgba, RGBA(red: red, green: green, blue: blue, alpha: 1.0), typeID)
        }
    }

    /// 欠けも余りも無いこと(18 タイプちょうど)。
    func testTypeColorKeysAreExactlyThe18OpenAPITypeIDs() {
        XCTAssertEqual(Set(TypeColorToken.allTypeIDs), Set(Self.expectedTypeColors.keys))
        XCTAssertEqual(TypeColorToken.allTypeIDs.count, 18, "重複が無いこと")
    }

    func testUnknownTypeIDHasNoColor() {
        // タイプ以外の ID で色が引けない(黙って既定色を返さない。画面側がエンブレムの代替を決める)。
        XCTAssertNil(TypeColorToken.rgba(forTypeID: "test-unknown-type"))
        XCTAssertNil(TypeColorToken.rgba(forTypeID: "Fire"), "ID は英小文字で完全一致")
    }

    // MARK: - 文字・形・余白

    /// design.md「文字」: 結果の%表示 28 / 見出し 17 / 本文 15 / 補足 12。
    func testTextStyleSizesMatchDesignDoc() {
        let cases: [(TextStyleToken, CGFloat)] = [
            (.resultPercent, 28),
            (.heading, 17),
            (.body, 15),
            (.caption, 12),
        ]
        for (style, size) in cases {
            XCTAssertEqual(style.size, size, "\(style)")
        }
        XCTAssertEqual(TextStyleToken.allCases.count, 4, "design.md に無いサイズを足さない")
    }

    /// design.md「形・余白」: 角丸 カード 20 / ボタン・チップ 999(ピル)/ 入力 12。
    func testCornerRadiiMatchDesignDoc() {
        XCTAssertEqual(RadiusToken.card, 20)
        XCTAssertEqual(RadiusToken.pill, 999)
        XCTAssertEqual(RadiusToken.input, 12)
    }

    /// design.md「形・余白」: 余白は 4 の倍数(4, 8, 12, 16, 24)。名前は 4 の何倍かを表す。
    func testSpacingScaleMatchesDesignDoc() {
        XCTAssertEqual(SpacingToken.x1, 4)
        XCTAssertEqual(SpacingToken.x2, 8)
        XCTAssertEqual(SpacingToken.x3, 12)
        XCTAssertEqual(SpacingToken.x4, 16)
        XCTAssertEqual(SpacingToken.x6, 24)
        XCTAssertEqual(SpacingToken.all, [4, 8, 12, 16, 24])
    }

    // MARK: - SwiftUI のアクセサ(コンパイルできることの確認)

    /// View から使う形(Color / Font)が用意されていること。値の比較は上の RGBA / size で行う。
    func testSwiftUIAccessorsExist() {
        let adaptive: Color = ColorToken.bgBase.color   // ライト/ダークで切り替わる Color
        let fixed: Color = ColorToken.danger.light.color
        let typeColor: Color? = TypeColorToken.color(forTypeID: "fire")
        let font: Font = TextStyleToken.resultPercent.font  // SF Pro Rounded(design.md「文字」)
        _ = (adaptive, fixed, font)
        XCTAssertNotNil(typeColor)
    }

    // MARK: - 補助

    private func assertRGBA(_ actual: RGBA, _ expected: RGBA, _ label: String,
                            file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(actual.red, expected.red, "\(label) red", file: file, line: line)
        XCTAssertEqual(actual.green, expected.green, "\(label) green", file: file, line: line)
        XCTAssertEqual(actual.blue, expected.blue, "\(label) blue", file: file, line: line)
        XCTAssertEqual(actual.alpha, expected.alpha, accuracy: 0.0001, "\(label) alpha", file: file, line: line)
    }
}
