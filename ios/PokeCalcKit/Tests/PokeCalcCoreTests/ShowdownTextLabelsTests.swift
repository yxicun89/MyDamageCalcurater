import XCTest

@testable import PokeCalcCore

// P6-20: Showdown 風テキストの書式キーワードと画面の文言(ADR-0501「P6-20」2章・3章の表)。
// 文言は Core の `ShowdownTextLabels` 1か所に集約し、View はそれを描くだけ。
final class ShowdownTextLabelsTests: XCTestCase {
    func testFormatKeywordsAreTheFixedStrings() {
        XCTAssertEqual(ShowdownTextLabels.abilityKey, "Ability:")
        XCTAssertEqual(ShowdownTextLabels.natureKey, "Nature:")
        XCTAssertEqual(ShowdownTextLabels.spKey, "SP:")
        XCTAssertEqual(ShowdownTextLabels.teraTypeKey, "Tera Type:")
        XCTAssertEqual(ShowdownTextLabels.evKey, "EVs:")
        XCTAssertEqual(ShowdownTextLabels.ivKey, "IVs:")
        XCTAssertEqual(ShowdownTextLabels.itemSeparator, " @ ")
        XCTAssertEqual(ShowdownTextLabels.movePrefix, "- ")
        XCTAssertEqual(ShowdownTextLabels.spSeparator, " / ")
    }

    func testStatAbbreviationsMatchShowdownAndAreDistinct() {
        let expected: [StatKey: String] = [.hp: "HP", .atk: "Atk", .def: "Def", .spa: "SpA", .spd: "SpD", .spe: "Spe"]
        for stat in StatKey.allCases {
            XCTAssertEqual(ShowdownTextLabels.statAbbreviation(for: stat), expected[stat], "\(stat)")
        }
        XCTAssertEqual(Set(StatKey.allCases.map(ShowdownTextLabels.statAbbreviation(for:))).count, StatKey.allCases.count)
    }

    func testScreenLabelsAreTheFixedStrings() {
        XCTAssertEqual(ShowdownTextLabels.transferButton, "テキストで書き出し・取り込み")
        XCTAssertEqual(ShowdownTextLabels.sheetTitle, "構築のテキスト")
        XCTAssertEqual(ShowdownTextLabels.exportMemberButton, "この1体を書き出す")
        XCTAssertEqual(ShowdownTextLabels.exportTeamButton, "全員を書き出す")
        XCTAssertEqual(ShowdownTextLabels.copyButton, "コピー")
        XCTAssertEqual(ShowdownTextLabels.copiedNotice, "コピーしました。")
        XCTAssertEqual(ShowdownTextLabels.shareButton, "共有")
        XCTAssertEqual(ShowdownTextLabels.importSectionTitle, "テキストから取り込む")
        XCTAssertEqual(ShowdownTextLabels.importPlaceholder, "ここに貼り付け")
        XCTAssertEqual(ShowdownTextLabels.analyzeButton, "内容を確認")
        XCTAssertEqual(ShowdownTextLabels.rejectedTitle, "取り込めなかった行")
        XCTAssertEqual(ShowdownTextLabels.cancelButton, "やめる")
        XCTAssertEqual(ShowdownTextLabels.closeButton, "閉じる")
        XCTAssertEqual(ShowdownTextLabels.nothingImportable, "取り込めるポケモンがありません。")
        XCTAssertEqual(ShowdownTextLabels.emptyInput, "テキストを貼り付けてください。")
        XCTAssertEqual(
            ShowdownTextLabels.lookupFailure,
            "サーバーに届かず、名前を確認できませんでした。通信を確認してもう一度お試しください。保存済みの構築は変わりません。"
        )
        XCTAssertEqual(ShowdownTextLabels.importValidOnlyButton(count: 2), "取り込める2体だけ追加")
        XCTAssertEqual(ShowdownTextLabels.importAllButton(count: 3), "3体を追加")
        XCTAssertEqual(ShowdownTextLabels.importedNotice(count: 1), "1体を追加しました。保存すると反映されます。")
        XCTAssertEqual(ShowdownTextLabels.lineNumberLabel(7), "7行目")
    }

    /// 上限の数字は定数から埋める(直書きしない。値は SPLimits/TeamLimits と一致する)。
    func testRejectionMessagesTable() {
        let expected: [ShowdownRejectionReason: String] = [
            .unrecognizedLine: "解釈できない行です",
            .unsupportedStatLine: "努力値(EVs)・個体値(IVs)の形式には対応していません。能力ポイントは SP: で書いてください",
            .duplicateField: "同じ項目が2回書かれています",
            .tooManyMoves: "技は\(TeamLimits.maxMovesPerMember)つまでです",
            .duplicateMove: "同じ技が重複しています",
            .spMalformed: "SP の書き方が正しくありません",
            .spOutOfRange: "SP は1ステータスにつき\(SPLimits.maxPerStat)までです",
            .spTotalExceeded: "SP の合計は\(SPLimits.maxTotal)までです",
            .speciesNotFound: "ポケモンが見つかりません",
            .moveNotFound: "技が見つかりません",
            .itemNotFound: "持ち物が見つかりません",
            .abilityNotFound: "このポケモンの特性に見つかりません",
            .natureNotFound: "性格が見つかりません",
            .teraTypeNotFound: "テラスタイプが見つかりません",
            .lookupFailed: "通信できず確認できませんでした",
            .memberLimitExceeded: "構築は\(TeamLimits.maxMembers)体までです",
        ]
        XCTAssertEqual(Set(expected.keys), Set(ShowdownRejectionReason.allCases), "表が全ケースを覆う")
        for reason in ShowdownRejectionReason.allCases {
            XCTAssertEqual(ShowdownTextLabels.message(for: reason), expected[reason], "\(reason)")
        }
        XCTAssertEqual(Set(ShowdownRejectionReason.allCases.map(ShowdownTextLabels.message(for:))).count,
                       ShowdownRejectionReason.allCases.count, "理由ごとに文言が違う")
    }
}
