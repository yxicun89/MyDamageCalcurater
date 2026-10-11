import XCTest

@testable import PokeCalcCore

// F-08(構築の作り直し。ADR-0522): 構築名を廃止し、既定名の構築は「構築 N」と表示する。
// N は既定名の構築だけを作成の古い順(= `TeamStore.list()` の追加順)に数えた 1 始まりの番号。
// 旧データで名前を持つ構築は保存された名前をそのまま出す。架空の名前だけを使う。
final class TeamNamingTests: XCTestCase {
    private func team(_ id: String, _ name: String) -> Team { Team(id: id, name: name) }

    func testDefaultNameIsTheSharedUnnamedValue() {
        XCTAssertEqual(TeamNaming.defaultName, "名称未設定")
    }

    func testUntitledLabelUsesNumber() {
        XCTAssertEqual(TeamNaming.untitled(3), "構築 3")
    }

    func testDefaultNamedTeamsAreNumberedInListOrder() {
        let names = TeamNaming.displayNames(for: [team("a", "名称未設定"), team("b", "名称未設定"), team("c", "名称未設定")])
        XCTAssertEqual(names, ["a": "構築 1", "b": "構築 2", "c": "構築 3"])
    }

    func testNamedLegacyTeamKeepsItsNameAndDoesNotTakeANumber() {
        let names = TeamNaming.displayNames(for: [team("a", "名称未設定"), team("b", "テストむかしのなまえ"), team("c", "名称未設定")])
        XCTAssertEqual(names["a"], "構築 1")
        XCTAssertEqual(names["b"], "テストむかしのなまえ")
        XCTAssertEqual(names["c"], "構築 2", "名前付きは数えない")
    }

    func testNumbersCloseUpAfterDeletion() {
        let names = TeamNaming.displayNames(for: [team("b", "名称未設定"), team("c", "名称未設定")])
        XCTAssertEqual(names, ["b": "構築 1", "c": "構築 2"])
    }

    func testEmptyListGivesEmptyDictionary() {
        XCTAssertEqual(TeamNaming.displayNames(for: []), [:])
    }

    func testDefaultNameNeverAppearsInDisplayNames() {
        let names = TeamNaming.displayNames(for: [team("a", "名称未設定")])
        XCTAssertFalse(names.values.contains(TeamNaming.defaultName))
    }
}

/// 旧データ(最終更新の無い保存)を読める。`updatedAt` は任意項目。
final class TeamCodableCompatTests: XCTestCase {
    func testLegacyJSONWithoutUpdatedAtDecodes() throws {
        let json = #"[{"id":"t1","name":"テストむかし","members":[]}]"#
        let teams = try JSONDecoder().decode([Team].self, from: Data(json.utf8))
        XCTAssertEqual(teams.first?.name, "テストむかし")
        XCTAssertNil(teams.first?.updatedAt)
    }

    func testUpdatedAtRoundTrips() throws {
        let date = Date(timeIntervalSince1970: 1_800_000_000)
        let data = try JSONEncoder().encode([Team(id: "t1", name: "名称未設定", updatedAt: date)])
        let back = try JSONDecoder().decode([Team].self, from: data)
        XCTAssertEqual(back.first?.updatedAt, date)
    }
}

final class TeamLabelsTests: XCTestCase {
    func testSlotLabelsUseSameWordsAsWeb() {
        XCTAssertEqual(TeamLabels.slotTitle(1), "1体目")
        XCTAssertEqual(TeamLabels.moveUp(2), "2体目を上へ")
        XCTAssertEqual(TeamLabels.moveDown(2), "2体目を下へ")
        XCTAssertEqual(TeamLabels.remove(6), "6体目を外す")
        XCTAssertEqual(TeamLabels.memberCount(3), "3/6体")
        XCTAssertEqual(TeamLabels.createButton, "新しい構築")
        XCTAssertEqual(TeamLabels.backToList, "一覧に戻る")
        XCTAssertEqual(TeamLabels.unsavedNotice, "保存していない変更があります")
        XCTAssertEqual(TeamLabels.leaveDiscard, "保存せずに戻る")
        XCTAssertEqual(TeamLabels.leaveCancel, "編集を続ける")
        XCTAssertEqual(TeamLabels.emptySlotHint, "ポケモンを選ぶと、技・持ち物・特性などを決められます")
    }

    func testUpdatedTextIsFallbackWhenUnknown() {
        XCTAssertEqual(TeamLabels.updatedText(nil), TeamLabels.updatedUnknown)
        XCTAssertTrue(TeamLabels.updatedText(Date(timeIntervalSince1970: 1_800_000_000)).hasPrefix("最終更新: "))
    }
}
