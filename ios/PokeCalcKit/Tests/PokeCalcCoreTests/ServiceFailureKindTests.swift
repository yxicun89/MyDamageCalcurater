import Foundation
import XCTest

@testable import PokeCalcCore

/// 失敗の分類(I-ios-19。ADR-0534 の方針): サーバーが使えない理由を `PokeCalcCore` の 1 か所
/// (`ServiceFailureKind`)で分け、全画面が同じ分類・同じ文言を使う。HTTP の番号とサーバーの英語は画面に出さない。
/// 期待値は docs/ai-shared/decisions/2026-10-11-ios-offline-and-api-errors.md の「3.4 直す案」から書く。
final class ServiceFailureKindTests: XCTestCase {

    private func error(_ code: String, status: Int? = nil) -> PokeCalcError {
        PokeCalcError(code: code, message: "English server message", statusCode: status)
    }

    // MARK: - 分類(テーブル駆動)

    func testClassification() {
        let cases: [(name: String, error: any Error, expected: ServiceFailureKind)] = [
            ("接続できない", error(PokeCalcError.Code.transport), .offline),
            ("時間切れ", error(PokeCalcError.Code.timeout), .timeout),
            ("デコード失敗", error(PokeCalcError.Code.decode), .unreadable),
            ("サーバーの internal_error", error("internal_error", status: 500), .serverFault),
            ("サーバーの master_unavailable", error("master_unavailable", status: 503), .serverFault),
            ("サーバーの upstream_unavailable", error("upstream_unavailable", status: 503), .serverFault),
            ("サーバーの store_unavailable", error("store_unavailable", status: 503), .serverFault),
            ("混み合い", error("overloaded", status: 503), .overloaded),
            ("契約外 502(HTML)", error(PokeCalcError.Code.unexpectedStatus, status: 502), .serverFault),
            ("契約外 504(HTML)", error(PokeCalcError.Code.unexpectedStatus, status: 504), .serverFault),
            ("契約外 503(HTML)", error(PokeCalcError.Code.unexpectedStatus, status: 503), .serverFault),
            ("経路の無い 404(HTML)", error(PokeCalcError.Code.unexpectedStatus, status: 404), .unsupported),
            ("501", error(PokeCalcError.Code.unexpectedStatus, status: 501), .unsupported),
            ("番号の無い契約外", error(PokeCalcError.Code.unexpectedStatus), .unreadable),
            ("契約外の 418", error(PokeCalcError.Code.unexpectedStatus, status: 418), .unreadable),
            ("入力の誤り(not_found は経路の 404 ではない)", error("not_found", status: 404), .other(code: "not_found")),
            ("入力の誤り(invalid_request)", error("invalid_request", status: 400), .other(code: "invalid_request")),
            ("端末の情報", error("missing_header", status: 400), .other(code: "missing_header")),
            ("クライアント側のコード", error(PokeCalcError.Code.natureUnavailable), .other(code: PokeCalcError.Code.natureUnavailable)),
            ("PokeCalcError 以外", URLError(.badServerResponse), .unreadable),
        ]
        for testCase in cases {
            XCTAssertEqual(ServiceFailureKind(testCase.error), testCase.expected, testCase.name)
        }
    }

    func testStatusCodeDefaultsToNilAndIsKept() {
        XCTAssertNil(PokeCalcError(code: "x", message: "y").statusCode)
        XCTAssertEqual(PokeCalcError(code: "x", message: "y", statusCode: 502).statusCode, 502)
        XCTAssertNotEqual(
            PokeCalcError(code: "x", message: "y", statusCode: 502), PokeCalcError(code: "x", message: "y"),
            "番号も同値性に含める(既存の == は code と message だけだったので、番号つきを区別したい)")
    }

    // MARK: - 文言

    private static let features = ["計算", "タイプバランス", "素早さ", "調整", "構築", "お気に入り"]

    private static let kinds: [ServiceFailureKind] = [.offline, .timeout, .serverFault, .unsupported, .unreadable, .overloaded]

    func testFixedMessages() {
        XCTAssertEqual(
            ServiceFailureKind.offline.message(feature: "計算"),
            "サーバーに接続できません。インターネットの接続を確かめて、もう一度お試しください")
        XCTAssertEqual(
            ServiceFailureKind.timeout.message(feature: "計算"),
            "サーバーから時間内に返事がありませんでした。しばらくしてからもう一度お試しください")
        XCTAssertEqual(
            ServiceFailureKind.serverFault.message(feature: "素早さ"),
            "素早さのサーバーで問題が起きています。しばらくしてからもう一度お試しください")
        XCTAssertEqual(
            ServiceFailureKind.unsupported.message(feature: "調整"),
            "調整は、いまつないでいるサーバーではまだ使えません")
        XCTAssertEqual(
            ServiceFailureKind.unreadable.message(feature: "計算"),
            "サーバーの返事を読み取れませんでした。アプリかサーバーの更新が必要かもしれません")
        XCTAssertEqual(
            ServiceFailureKind.overloaded.message(feature: "計算"),
            "サーバーが混み合っています。しばらくしてからもう一度お試しください")
        XCTAssertEqual(ServiceFailureText.notConfigured, "サーバーの接続先が設定されていません")
        XCTAssertEqual(ServiceFailureText.retryButtonLabel, "もう一度")
    }

    func testKindsHaveDistinctMessages() {
        let messages = Self.kinds.map { $0.message(feature: "計算") }
        XCTAssertEqual(Set(messages).count, messages.count, "原因ごとに別の文言にする(13 件の監査の目的)")
    }

    /// 番号(502 など)・サーバーの英語・コードは画面に出さない。禁止語(docs/glossary.md)も使わない。
    func testMessagesHaveNoDigitsEnglishOrForbiddenWords() {
        let forbidden = ["観測", "プリセット", "API", "WASM", "マスタ", "識別子", "リクエスト", "HTTP", "エラー("]
        var texts = ServiceFailureKindTests.features.flatMap { feature in
            Self.kinds.map { $0.message(feature: feature) }
        }
        texts.append(ServiceFailureKind.other(code: "some_new_code").message(feature: "計算"))
        texts += [ServiceFailureText.notConfigured, ServiceFailureText.retryButtonLabel]
        for text in texts {
            XCTAssertFalse(text.isEmpty)
            XCTAssertNil(text.range(of: "[0-9]", options: .regularExpression), "番号を出さない: \(text)")
            XCTAssertNil(text.range(of: "[A-Za-z]", options: .regularExpression), "英語・コードを出さない: \(text)")
            for word in forbidden {
                XCTAssertFalse(text.contains(word), "禁止語「\(word)」: \(text)")
            }
            XCTAssertFalse(text.contains("English"))
        }
    }

    func testOtherKindDoesNotLeakCodeOrServerMessage() {
        let text = ServiceFailureKind.other(code: "some_new_code").message(feature: "計算")
        XCTAssertFalse(text.contains("some_new_code"))
        XCTAssertFalse(text.isEmpty)
    }
}
