import XCTest

@testable import PokeCalcCore

/// 読み込みの文言(ADR-0513 §7。`FavoritesLabels` の追加分)。既存の固定文言は変えない。
final class FavoriteLoadLabelsTests: XCTestCase {

    func testTitlesAndNotesPerSide() {
        XCTAssertEqual(FavoritesLabels.loadTitle(for: .attacker), "お気に入りから攻撃側を選ぶ")
        XCTAssertEqual(FavoritesLabels.loadTitle(for: .defender), "お気に入りから防御側を選ぶ")
        XCTAssertEqual(
            FavoritesLabels.loadNote(for: .attacker), "攻撃側には、種族・性格・能力ポイント・特性・持ち物を読み込みます。技は変わりません。")
        XCTAssertEqual(
            FavoritesLabels.loadNote(for: .defender), "防御側には、種族と特性だけを読み込みます。性格・能力ポイント・持ち物は使いません。")
        XCTAssertEqual(FavoritesLabels.loadCloseButton, "閉じる")
    }

    func testDroppedNamesCoverEveryCase() {
        XCTAssertEqual(FavoriteLoadDropped.allCases, [.nature, .item, .ability], "案内の並びは宣言順")
        XCTAssertEqual(FavoritesLabels.loadDroppedName(.nature), "性格と能力ポイント")
        XCTAssertEqual(FavoritesLabels.loadDroppedName(.item), "持ち物")
        XCTAssertEqual(FavoritesLabels.loadDroppedName(.ability), "特性")
    }

    func testNoticeTexts() {
        XCTAssertEqual(
            FavoriteLoadNotice.partial([.item, .ability]).text,
            "一部は読み込めませんでした(持ち物、特性)。読み込めた分だけ設定しました。")
        XCTAssertEqual(
            FavoriteLoadNotice.partial([.nature]).text, "一部は読み込めませんでした(性格と能力ポイント)。読み込めた分だけ設定しました。")
        XCTAssertEqual(
            FavoriteLoadNotice.speciesMissing.text, "このお気に入りのポケモンはいまのデータに無いため、読み込めませんでした。何も変えていません。")
        XCTAssertEqual(FavoriteLoadNotice.unavailable.text, "お気に入りを読み込めませんでした。何も変えていません。もう一度お試しください。")
    }

    func testNoticeTextsAreJapaneseOnlyAndNeverLeakCodes() {
        let texts = [
            FavoriteLoadNotice.partial(FavoriteLoadDropped.allCases).text,
            FavoriteLoadNotice.speciesMissing.text, FavoriteLoadNotice.unavailable.text,
            FavoritesLabels.loadNote(for: .attacker), FavoritesLabels.loadNote(for: .defender),
            FavoritesLabels.loadTitle(for: .attacker), FavoritesLabels.loadTitle(for: .defender),
            FavoritesLabels.loadLoading,
        ]
        for text in texts {
            XCTAssertFalse(text.isEmpty)
            XCTAssertNil(text.range(of: "[A-Za-z_]{3,}", options: .regularExpression), "英字の code を出さない: \(text)")
        }
    }

    func testSheetFailureAndEmptyReuseExistingRecordLabels() {
        // シートの失敗・空は ADR-0511 の文言をそのまま使う(新しい言い回しを増やさない)。
        XCTAssertEqual(
            RecordScreenError.storeUnavailable.message(for: .loadFavorites), FavoritesLabels.loadFavoritesUnavailable)
        XCTAssertEqual(RecordScreenError.transport.message(for: .loadFavorites), FavoritesLabels.transportFailure)
        XCTAssertTrue(FavoritesLabels.loadFavoritesUnavailable.contains("計算はそのまま使えます"))
    }

    func testExistingFixedLabelsAreUnchanged() {
        XCTAssertEqual(FavoritesLabels.pinAttackerButton, "攻撃側をお気に入りに追加")
        XCTAssertEqual(FavoritesLabels.pinDefenderButton, "防御側をお気に入りに追加")
        XCTAssertEqual(FavoritesLabels.emptyFavorites, "お気に入りはまだありません。計算画面の「お気に入りに追加」から追加できます。")
        XCTAssertEqual(FavoritesLabels.retryButton, "再読み込み")
        XCTAssertEqual(FavoritesLabels.unknownSpecies, "不明なポケモン")
        XCTAssertEqual(TeamLoadLabels.entryTitle, "構築から選ぶ")
    }

    func testSourceTeamIDIsStable() {
        XCTAssertEqual(FavoriteLoad.sourceTeamID, "favorite")
    }
}
