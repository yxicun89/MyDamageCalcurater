import XCTest

@testable import PokeCalcCore

/// P6-7: 画面の固定文言(ADR-0209 §8 を iOS 向けに確定。ADR-0501「P6-7」2章)。
final class DeviceDataTextTests: XCTestCase {
    func testExplanationIsThreeSentencesInOrder() {
        XCTAssertEqual(DeviceDataText.explanation, [
            "アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた ID でサーバーに保存しています。",
            "ID が変わると(アプリを削除して入れ直したとき)、前のデータは開けなくなります。元に戻す方法はありません。",
            "開けなくなったデータは自動的に消えます。計算の履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です。",
        ])
    }

    func testExplanationDoesNotMentionWebOnlyWording() {
        for sentence in DeviceDataText.explanation {
            XCTAssertFalse(sentence.contains("ブラウザ"), sentence)
            XCTAssertFalse(sentence.contains("サイトデータ"), sentence)
        }
    }

    func testOperationTexts() {
        XCTAssertEqual(DeviceDataText.deleteButton, "この端末のデータを削除")
        XCTAssertEqual(DeviceDataText.confirmMessage, "履歴・お気に入り・構築をサーバーから削除します。元に戻せません。")
        XCTAssertEqual(DeviceDataText.deleting, "削除しています…")
        XCTAssertEqual(DeviceDataText.partialNotice, "まだ残っています。続けて削除します。")
        XCTAssertEqual(DeviceDataText.failure, "サーバーに届きませんでした。通信を確認してもう一度お試しください。")
        XCTAssertEqual(DeviceDataText.completed, "削除しました。")
    }

    func testAuxiliaryTexts() {
        XCTAssertFalse(DeviceDataText.confirmAction.isEmpty)
        XCTAssertFalse(DeviceDataText.cancelAction.isEmpty)
        XCTAssertFalse(DeviceDataText.retryButton.isEmpty)
        XCTAssertEqual(DeviceDataText.recordLabel, "履歴・お気に入り")
        XCTAssertEqual(DeviceDataText.teamLabel, "構築")
        XCTAssertEqual(DeviceDataText.partlyDeleted(label: DeviceDataText.teamLabel), "構築は削除済みです。")
    }
}
