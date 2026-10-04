import XCTest

@testable import PokeCalcCore

/// `RecordScreenError`(code → 日本語。サーバーの英語 `message` と `code` を画面に出さない)と
/// `FavoriteLabel.normalize`・文言。ADR-0511。
final class RecordScreenErrorTests: XCTestCase {
    private func error(_ code: String, message: String = "English server message") -> RecordScreenError {
        RecordScreenError(PokeCalcError(code: code, message: message))
    }

    private static let allActions: [RecordAction] = [.loadFavorites, .addFavorite, .removeFavorite, .loadHistory]

    func testCodeMapping() {
        XCTAssertEqual(error(PokeCalcError.Code.transport), .transport)
        XCTAssertEqual(error(PokeCalcError.Code.decode), .unexpectedResponse)
        XCTAssertEqual(error(PokeCalcError.Code.unexpectedStatus), .unexpectedResponse)
        XCTAssertEqual(error("store_unavailable"), .storeUnavailable)
        XCTAssertEqual(error("upstream_unavailable"), .storeUnavailable)
        XCTAssertEqual(error("not_found"), .notFound)
        XCTAssertEqual(error("invalid_input"), .invalidInput)
        XCTAssertEqual(error("unknown_field"), .other(code: "unknown_field"))
        XCTAssertEqual(error("internal"), .other(code: "internal"))
        XCTAssertEqual(RecordScreenError(URLError(.badServerResponse)), .unexpectedResponse, "PokeCalcError 以外は想定外の応答")
    }

    func testEveryMessageIsNonEmptyJapaneseWithoutCodeOrServerMessage() {
        let errors: [RecordScreenError] = [
            .transport, .unexpectedResponse, .storeUnavailable, .notFound, .invalidInput,
            .other(code: "unknown_field"),
        ]
        for error in errors {
            for action in Self.allActions {
                let message = error.message(for: action)
                XCTAssertFalse(message.isEmpty, "\(error) / \(action)")
                XCTAssertFalse(message.contains("unknown_field"), "code を出さない: \(message)")
                XCTAssertFalse(message.contains("English"), "サーバーの message を出さない: \(message)")
                XCTAssertTrue(
                    message.unicodeScalars.contains { $0.value >= 0x3040 }, "日本語の文言: \(message)")
            }
        }
    }

    func testServerMessageNeverLeaksThroughMapping() {
        let mapped = error("some_new_code", message: "Secret English detail")
        for action in Self.allActions {
            XCTAssertFalse(mapped.message(for: action).contains("Secret"))
            XCTAssertFalse(mapped.message(for: action).contains("some_new_code"))
        }
    }

    func testMessagesPerKindAndAction() {
        XCTAssertEqual(RecordScreenError.transport.message(for: .loadFavorites), FavoritesLabels.transportFailure)
        XCTAssertEqual(RecordScreenError.transport.message(for: .addFavorite), FavoritesLabels.transportFailure)
        XCTAssertEqual(RecordScreenError.transport.message(for: .removeFavorite), FavoritesLabels.transportFailure)
        XCTAssertEqual(RecordScreenError.transport.message(for: .loadHistory), FavoritesLabels.transportFailure)

        XCTAssertEqual(RecordScreenError.storeUnavailable.message(for: .loadFavorites), FavoritesLabels.loadFavoritesUnavailable)
        XCTAssertEqual(RecordScreenError.storeUnavailable.message(for: .addFavorite), FavoritesLabels.addFavoriteUnavailable)
        XCTAssertEqual(RecordScreenError.storeUnavailable.message(for: .removeFavorite), FavoritesLabels.removeFavoriteUnavailable)
        XCTAssertEqual(RecordScreenError.storeUnavailable.message(for: .loadHistory), FavoritesLabels.loadHistoryUnavailable)

        XCTAssertEqual(RecordScreenError.invalidInput.message(for: .addFavorite), FavoritesLabels.addFavoriteInvalid)
        XCTAssertEqual(RecordScreenError.unexpectedResponse.message(for: .loadFavorites), FavoritesLabels.genericFailure)
        XCTAssertEqual(RecordScreenError.other(code: "x").message(for: .addFavorite), FavoritesLabels.genericFailure)
        XCTAssertEqual(RecordScreenError.notFound.message(for: .loadFavorites), FavoritesLabels.genericFailure)
    }

    func test503MessagesTellTheUserCalculationStillWorks() {
        for action in Self.allActions {
            XCTAssertTrue(
                RecordScreenError.storeUnavailable.message(for: action).contains("計算はそのまま使えます"),
                "絶対ルール5: 保存・取得できなくても計算は使えると案内する(\(action))")
        }
    }

    func testLimitMessageEmbedsContractLimit() {
        XCTAssertTrue(FavoritesLabels.addFavoriteInvalid.contains("\(RequestLimits.maxFavorites)"))
        XCTAssertTrue(FavoritesLabels.limitNote.contains("\(RequestLimits.maxFavorites)"))
    }

    // MARK: - ラベルの正規化

    func testLabelNormalize() {
        XCTAssertNil(FavoriteLabel.normalize(nil))
        XCTAssertNil(FavoriteLabel.normalize(""))
        XCTAssertNil(FavoriteLabel.normalize("   \n\t "))
        XCTAssertEqual(FavoriteLabel.normalize("  HB特化\n"), "HB特化")
        XCTAssertEqual(FavoriteLabel.normalize("A B"), "A B", "途中の空白は残す")
    }

    func testLabelNormalizeTruncatesByCodePoint() {
        let max = RequestLimits.maxFavoriteLabelLength
        let exact = String(repeating: "あ", count: max)
        XCTAssertEqual(FavoriteLabel.normalize(exact), exact)
        XCTAssertEqual(FavoriteLabel.normalize(exact + "い"), exact)
        // 絵文字(1コードポイント)も1と数える。
        let emoji = String(repeating: "😀", count: max + 3)
        XCTAssertEqual(FavoriteLabel.normalize(emoji)?.unicodeScalars.count, max)
    }
}
