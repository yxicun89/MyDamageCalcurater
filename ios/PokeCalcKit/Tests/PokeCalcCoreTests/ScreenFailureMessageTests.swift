import Foundation
import XCTest

@testable import PokeCalcCore

/// 全画面の失敗文言が `ServiceFailureKind` の分類に従うこと(I-ios-19。監査 3.3 の 2・3・4・5・6・7 件目)。
/// - 原因が分かる(接続できない / 時間切れ / サーバーの障害 / まだ使えない / 読み取れない)
/// - 5xx を「接続できません」にしない・形のずれを「接続できません」にしない
/// - HTTP の番号・コード・サーバーの英語を画面に出さない
final class ScreenFailureMessageTests: XCTestCase {

    private func error(_ code: String, status: Int? = nil) -> PokeCalcError {
        PokeCalcError(code: code, message: "English server message", statusCode: status)
    }

    /// 分類ごとの入力(画面に届くエラー)。
    private var inputs: [(kind: ServiceFailureKind, error: PokeCalcError)] {
        [
            (.offline, error(PokeCalcError.Code.transport)),
            (.timeout, error(PokeCalcError.Code.timeout)),
            (.serverFault, error("internal_error", status: 500)),
            (.serverFault, error(PokeCalcError.Code.unexpectedStatus, status: 502)),
            (.unsupported, error(PokeCalcError.Code.unexpectedStatus, status: 404)),
            (.unreadable, error(PokeCalcError.Code.decode)),
            (.overloaded, error("overloaded", status: 503)),
        ]
    }

    private func assertSafe(_ text: String, _ label: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertNil(text.range(of: "[0-9]", options: .regularExpression), "\(label): 番号を出さない: \(text)", file: file, line: line)
        XCTAssertFalse(text.contains("English"), "\(label): サーバーの英語を出さない: \(text)", file: file, line: line)
        XCTAssertFalse(text.contains("エラー("), "\(label): code を出さない: \(text)", file: file, line: line)
        XCTAssertFalse(text.contains("internal_error"), label, file: file, line: line)
    }

    func testCalcAndReverseScreenError() {
        for (kind, error) in inputs {
            let text = CalcScreenError(error).message
            XCTAssertEqual(text, kind.message(feature: "計算"), "\(kind) / \(error.code)")
            assertSafe(text, "計算 \(kind)")
        }
        // 画面別の入力の誤りの文言は残す。code と英語は出さない。
        let other = CalcScreenError(error("invalid_input", status: 400)).message
        assertSafe(other, "計算 invalid_input")
        XCTAssertFalse(other.isEmpty)
        let nature = CalcScreenError(error(PokeCalcError.Code.natureUnavailable)).message
        XCTAssertFalse(nature.contains("client_"), "内部のコードを出さない: \(nature)")
    }

    func testTeamScreenError() {
        for (kind, error) in inputs {
            let text = TeamScreenError(error).message
            XCTAssertEqual(text, kind.message(feature: "構築"), "\(kind) / \(error.code)")
            assertSafe(text, "構築 \(kind)")
        }
        let nature = TeamScreenError(error(PokeCalcError.Code.natureUnavailable)).message
        XCTAssertFalse(nature.contains("client_"), "内部のコードを出さない: \(nature)")
    }

    func testBalanceScreenError() {
        for (kind, error) in inputs {
            let text = BalanceScreenError(error).message
            XCTAssertEqual(text, kind.message(feature: "タイプバランス"), "\(kind) / \(error.code)")
            assertSafe(text, "タイプバランス \(kind)")
        }
        // 形のずれ・契約外の応答を「接続できません」にまとめない(13 件の 2・1)。
        XCTAssertNotEqual(
            BalanceScreenError(error(PokeCalcError.Code.decode)).message,
            BalanceScreenError(error(PokeCalcError.Code.transport)).message)
        XCTAssertNotEqual(
            BalanceScreenError(error(PokeCalcError.Code.unexpectedStatus, status: 502)).message,
            BalanceScreenError(error(PokeCalcError.Code.transport)).message)
        // 画面別の既存の入力の誤りの文言は変えない。
        XCTAssertEqual(BalanceErrorText.message(forCode: "invalid_request"), "入力の内容が正しくありません。見直してください")
        XCTAssertEqual(BalanceErrorText.message(forCode: "overloaded"), "サーバーが混み合っています。しばらくしてからもう一度お試しください")
    }

    /// 接続先が無い構成(モック・未設定)のタイプバランスは「接続できません」ではなく「接続先が設定されていません」。
    func testUnavailableBalanceServiceSaysNotConfigured() async {
        do {
            _ = try await UnavailableBalanceService().moveRange(moveIds: [])
            XCTFail("失敗するはず")
        } catch {
            XCTAssertEqual(BalanceScreenError(error).message, ServiceFailureText.notConfigured)
        }
    }

    func testSpeedFailure() {
        for (kind, error) in inputs {
            let failure = SpeedFailure(error)
            XCTAssertEqual(failure.message, kind.message(feature: "素早さ"), "\(kind) / \(error.code)")
            assertSafe(failure.message, "素早さ \(kind)")
        }
        // サーバーの障害が汎用の「計算に失敗しました」に埋もれない(13 件の 3・4)。
        XCTAssertNotEqual(SpeedFailure(error("internal_error", status: 500)).message, SpeedLabels.errorMessage(forCode: "unknown_code"))
        // PokeCalcError 以外の失敗は「読み取れない」。
        XCTAssertEqual(SpeedFailure(URLError(.badServerResponse)).message, ServiceFailureKind.unreadable.message(feature: "素早さ"))
        // 既存の code 起点の初期化と文言は残す(入力の誤りの文言)。
        XCTAssertEqual(SpeedFailure(code: "unknown_pokemon").message, SpeedLabels.errorMessage(forCode: "unknown_pokemon"))
    }

    func testAdjustTextErrorMessage() {
        for (kind, error) in inputs {
            let text = AdjustText.errorMessage(for: error)
            XCTAssertEqual(text, kind.message(feature: "調整"), "\(kind) / \(error.code)")
            assertSafe(text, "調整 \(kind)")
        }
        // 形のずれを「接続できません」にしない(13 件の 2)。
        XCTAssertNotEqual(
            AdjustText.errorMessage(for: error(PokeCalcError.Code.decode)),
            AdjustText.errorMessage(for: error(PokeCalcError.Code.transport)))
        // 入力の誤りの not_found は経路の 404 と区別する(13 件の 7)。
        XCTAssertNotEqual(
            AdjustText.errorMessage(for: error("not_found", status: 404)),
            AdjustText.errorMessage(for: error(PokeCalcError.Code.unexpectedStatus, status: 404)))
    }

    func testRecordScreenErrorDistinguishesCauses() {
        let actions: [RecordAction] = [.loadFavorites, .addFavorite, .removeFavorite, .loadHistory]
        for action in actions {
            let offline = RecordScreenError(error(PokeCalcError.Code.transport)).message(for: action)
            let timeout = RecordScreenError(error(PokeCalcError.Code.timeout)).message(for: action)
            let fault = RecordScreenError(error("internal_error", status: 500)).message(for: action)
            let gateway = RecordScreenError(error(PokeCalcError.Code.unexpectedStatus, status: 502)).message(for: action)
            let unsupported = RecordScreenError(error(PokeCalcError.Code.unexpectedStatus, status: 404)).message(for: action)
            let unreadable = RecordScreenError(error(PokeCalcError.Code.decode)).message(for: action)
            XCTAssertEqual(Set([offline, timeout, fault, unsupported, unreadable]).count, 5, "\(action): 原因ごとに別の文言")
            XCTAssertTrue(offline.contains("サーバーに接続できません"), "\(action): UI テストが見る語")
            XCTAssertEqual(fault, gateway, "\(action): 500 の JSON も契約外の 502 も「サーバーの障害」")
            for text in [offline, timeout, fault, unsupported, unreadable] {
                assertSafe(text, "お気に入り・履歴 \(action)")
            }
        }
        // 既存の個別文言(保存先が使えない)は残す。
        XCTAssertEqual(RecordScreenError(error("store_unavailable", status: 503)), .storeUnavailable)
    }
}
