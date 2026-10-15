import XCTest

@testable import PokeCalcCore

/// I-ios-21(ADR-0542 §4〜§5・ADR-0501「素早さ画面から連続技へ」AC-h4・AC-h5・AC-h8): 引き継ぎの文言。
/// スカーフの 1 行は Web の `web/src/i18n/combo.ts` の `selfSpeed.scarfIgnored` と同じ語。「説明」の奥の箇条と読み上げは iOS の文言
/// (Web は「説明」を持たない。読み上げは iOS がフォーカスの移動と一緒に出す)。docs/glossary.md の禁止語を含まない。
final class ComboHandoffTextTests: XCTestCase {

    /// docs/glossary.md「言い換える語(禁止語)」(GlossaryTests と同じ列。「種族」はここでは語として使わない)。
    private static let forbiddenWords = [
        "観測", "推定結果", "近い候補", "プリセット", "指数", "16n", "最小の振り方", "API", "WASM", "実行場所", "計算モード", "マスタ",
        "識別子", "識別情報", "リクエスト", "メガシンカ:", "メガシンカ：", "種族", "ID",
    ]

    func testScarfLineMatchesWeb() {
        XCTAssertEqual(ComboText.handoffScarfIgnored, "素早さ画面で指定したこだわりスカーフは、連続技の計算では反映できません。")
    }

    func testHelpLinesSayWhatIsAndIsNotCarried() {
        XCTAssertEqual(
            ComboText.handoffHelpLines,
            [
                "自分と相手のポケモン、素早さのSP・性格補正・ランクを引き継ぎます",
                "こだわりスカーフとまひは、連続技に指定する項目が無いので引き継ぎません",
                "追い風とトリックルームは、この節の「素早さの場」で選びます",
            ])
        for line in ComboText.handoffHelpLines {
            XCTAssertEqual(line.components(separatedBy: "。").count, 1, "1 行 1 文(箇条。句点で文を足さない): \(line)")
        }
    }

    func testHelpMentionsTheWordsUsedOnTheScreen() {
        let joined = ComboText.handoffHelpLines.joined()
        XCTAssertTrue(joined.contains(ComboText.speedFieldToggle), "節の「素早さの場」と同じ語で案内する")
        XCTAssertTrue(joined.contains("こだわりスカーフ"))
    }

    func testAnnouncement() {
        XCTAssertEqual(ComboText.handoffAnnouncement, "素早さ画面の自分と相手で連続技を開きました")
    }

    func testLinkLabelStays() {
        XCTAssertEqual(SpeedMatchupLabels.comboLink, "この相手で連続技を計算", "素早さ画面のボタンの語(Web の comboLinkLabel と同じ)は変えない")
    }

    func testNoForbiddenWords() {
        let texts = [ComboText.handoffScarfIgnored, ComboText.handoffAnnouncement, SpeedMatchupLabels.comboLink]
            + ComboText.handoffHelpLines
        for text in texts {
            XCTAssertFalse(text.isEmpty)
            for word in Self.forbiddenWords {
                XCTAssertFalse(text.contains(word), "「\(text)」に禁止語「\(word)」がある")
            }
        }
    }
}
