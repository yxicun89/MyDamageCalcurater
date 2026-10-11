import XCTest

@testable import PokeCalcCore

// F-13(ADR-0526): docs/glossary.md の「言い換える語(禁止語)」が画面の文言に残っていないことの検査(Web の
// web/src/i18n/glossary.test.ts に相当)。禁止語リストは用語集を実行時に読まず、このテストに持つ
// (iOS のテストが repo のドキュメントの位置に依存しないため。用語集を変えたらここも変える)。
//
// 対象は PokeCalcKit が持つ画面の文言(Labels 系の列挙型・エラー表・固定文)のうち、列挙できるもの。
// 判定画面(JudgeLabels)は F-07 で非表示で、判定レーンの担当なので用語集の例外にならって含めない。
// 「種族」は「すべての特性」の意味で使うときだけ禁止語(用語集)。ポケモンそのものを指す説明文の
// 「種族」は Web も残しているので、機械検査の対象にしない。
final class GlossaryTests: XCTestCase {

    /// 用語集「言い換える語(禁止語)」の1列目(「種族」と、単語としての「ID」は別に扱う)。
    private static let forbiddenWords = [
        "観測", "推定結果", "近い候補", "プリセット", "指数", "16n", "最小の振り方", "API", "WASM",
        "実行場所", "計算モード", "マスタ", "識別子", "識別情報", "リクエスト", "メガシンカ:", "メガシンカ：",
    ]

    /// 用語集「変えない語」のうち、iOS のどこかの画面の文言に残っているべき語(言い換えすぎの防止)。
    private static let keptWords = ["確定", "乱数", "性格", "特性", "持ち物", "能力ポイント", "実数値", "仮想敵", "メガストーン", "メガシンカ"]

    /// 画面に出る文言を、列挙できる範囲で集める。
    private static func screenStrings() -> [(source: String, text: String)] {
        var out: [(String, String)] = []
        func add(_ source: String, _ texts: [String]) { out += texts.map { (source, $0) } }

        add("AdjustText", [
            AdjustText.natureNotFound, AdjustText.indicesHeading, AdjustText.firepowerIndexLabel,
            AdjustText.physicalBulkLabel, AdjustText.specialBulkLabel, AdjustText.indexNote,
            AdjustText.hpLineHeading, AdjustText.next16nLabel, AdjustText.prev16nLabel,
            AdjustText.next16nMinus1Label, AdjustText.prev16nMinus1Label, AdjustText.maxIndexHeading,
            AdjustText.minSpHeading, AdjustText.minSpNotRequested, AdjustText.unavailable,
            AdjustText.errorFallback, AdjustText.speedMet, AdjustText.speedNotMet,
        ])
        add("AdjustText.errorMessages", Array(AdjustText.errorMessages.values))
        add("AdjustText.modeLabel", AdjustMode.allCases.map(AdjustText.modeLabel))
        add("AdjustText.hpLineKindLabel", HPLineKind.allCases.map(AdjustText.hpLineKindLabel))

        add("BulkRowDisplay.koText", [
            BulkRowDisplay.koText(KOChance(hits: 3, guaranteed: true, chancePercent: 100, displayChancePercent: 100)),
            BulkRowDisplay.koText(KOChance(hits: 4, guaranteed: false, chancePercent: 40, displayChancePercent: 40)),
        ])

        add("SpeedLabels.mode", SpeedInputMode.allCases.map(SpeedLabels.mode))
        add("SpeedLabels.nature", SpeedNature.allCases.map(SpeedLabels.nature))
        let speedCodes = [
            "invalid_request", "missing_header", "invalid_header", "unknown_pokemon", "request_too_large",
            "master_unavailable", "internal_error", PokeCalcError.Code.transport, PokeCalcError.Code.decode, "unknown_code",
        ]
        add("SpeedLabels.errorMessage", speedCodes.map(SpeedLabels.errorMessage(forCode:)))

        let balanceCodes = [
            "missing_header", "invalid_header", "missing_request_context", "invalid_request", "request_too_large",
            "unknown_pokemon", "unknown_move", "unknown_ability", "master_unavailable", "overloaded",
            "internal_error", BalanceErrorText.unavailableCode, "unknown_code",
        ]
        add("BalanceErrorText", balanceCodes.map(BalanceErrorText.message(forCode:)))

        add("DeviceDataText", DeviceDataText.explanation + [
            DeviceDataText.deleteButton, DeviceDataText.confirmMessage, DeviceDataText.failure,
            DeviceDataText.partialNotice, DeviceDataText.sectionTitle,
        ])
        add("RequestLimitLabels", [
            RequestLimitLabels.observationsReachedLimit, RequestLimitLabels.itemCandidatesReachedLimit,
            RequestLimitLabels.itemVariantsReachedLimit,
        ])
        add("MegaItemText", [
            MegaItemText.stoneName(baseSpeciesNameJa: "テストアルファ"), MegaItemText.stoneName(baseSpeciesNameJa: nil),
            MegaItemText.lockedReason, MegaItemText.missingReason, MegaItemText.compareDisabledReason,
            MegaItemText.clearedNotice, MegaItemText.correctedNotice("テスト石"),
        ])
        add("FavoriteLoad", [
            FavoritesLabels.loadSpeciesMissingNotice, FavoritesLabels.loadUnavailableNotice,
        ])
        add("ReverseCandidateDisplay", [
            ReverseCandidateDisplay.matchLabel(exact: true), ReverseCandidateDisplay.matchLabel(exact: false),
        ])
        add("BalanceScreenText", [
            BalanceScreenText.screenTitle, BalanceScreenText.emptyNotice, BalanceScreenText.threatsRegionLabel,
            BalanceScreenText.addThreatLabel, BalanceScreenText.threatsLoadingNotice, BalanceScreenText.tooManyThreats,
        ])
        return out
    }

    func testScreenStringsContainNoForbiddenWords() {
        for (source, text) in Self.screenStrings() {
            for word in Self.forbiddenWords {
                XCTAssertFalse(text.contains(word), "\(source) に禁止語「\(word)」: \(text)")
            }
            // 単語としての「ID」(アルファベットに挟まれない大文字 2 字)。
            XCTAssertNil(text.range(of: "(?<![A-Za-z])ID(?![A-Za-z])", options: .regularExpression), "\(source) に禁止語「ID」: \(text)")
            // 「のメガストーン」(「専用のメガストーン」は可)。
            XCTAssertFalse(text.replacingOccurrences(of: "専用のメガストーン", with: "").contains("のメガストーン"), "\(source): \(text)")
        }
    }

    func testScreenStringsAreNotEmpty() {
        XCTAssertGreaterThan(Self.screenStrings().count, 60, "列挙が空になっていないこと(検査が素通りしない)")
        XCTAssertTrue(Self.screenStrings().allSatisfy { !$0.text.isEmpty })
    }

    func testKeptWordsRemainSomewhereOnScreen() {
        let all = Self.screenStrings().map(\.text).joined(separator: "\n")
        for word in Self.keptWords {
            XCTAssertTrue(all.contains(word), "変えない語「\(word)」が画面の文言から消えている(言い換えすぎ)")
        }
    }

    func testReplacementWordsAreUsed() {
        XCTAssertEqual(MegaItemText.stoneName(baseSpeciesNameJa: "テストアルファ"), "テストアルファ専用のメガストーン")
        XCTAssertEqual(SpeedLabels.mode(.preset), "定番の振り方")
        XCTAssertTrue(DeviceDataText.explanation.joined().contains("番号"))
        XCTAssertEqual(AdjustText.unavailable, "調整のサーバーに接続できません")
    }
}
