import XCTest

@testable import PokeCalcCore

/// 素早さ画面の文言(ADR-0503 §7)。Web の `speedScreenText`・`speedPresetText`(web/src/i18n/speed.ts)と同じ文言を固定する。
final class SpeedLabelsTests: XCTestCase {
    /// 契約の `ErrorCode` → 日本語。値は Web の `speedScreenText.errorByCode` と同じ。
    /// `not_found` は Web の表に無く、汎用の文言になる。
    static let expectedErrorMessages: [String: String] = [
        "invalid_request": "入力の形が正しくありません。値の範囲を確認してください",
        "missing_header": "端末の情報が送られていません。アプリを開き直してください",
        "invalid_header": "端末の情報が正しくありません。アプリを開き直してください",
        "unknown_pokemon": "このポケモンはデータにありません",
        "request_too_large": "入力が大きすぎます",
        "master_unavailable": "ポケモンのデータを読み込めません",
        "internal_error": "素早さの計算に失敗しました",
        "not_found": "素早さの計算に失敗しました",
        "overloaded": "素早さの計算に失敗しました",
    ]

    func testErrorMessagesByContractCode() {
        for (code, expected) in Self.expectedErrorMessages {
            XCTAssertEqual(SpeedLabels.errorMessage(forCode: code), expected, code)
        }
    }

    /// Web の `speed_unavailable`(通信できない・応答が読めない)に当たる iOS の失敗。
    func testTransportAndDecodeFailuresMeanTheServiceIsUnavailable() {
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: PokeCalcError.Code.transport), "素早さのサーバーに接続できません")
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: PokeCalcError.Code.decode), "素早さのサーバーに接続できません")
    }

    /// gateway が speed に届かないときの 503(契約外の code)は、「計算に失敗」ではなく接続できない旨にする。
    func testGatewayUpstreamUnavailableMeansTheServiceIsUnavailable() {
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: "upstream_unavailable"), "素早さのサーバーに接続できません")
    }

    func testUnknownAndUnexpectedCodesFallBackToGenericMessage() {
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: PokeCalcError.Code.unexpectedStatus), "素早さの計算に失敗しました")
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: "some_future_code"), "素早さの計算に失敗しました")
    }

    func testFailureShowsJapaneseMessageNeverTheServerMessage() {
        let failure = SpeedFailure(code: "master_unavailable")
        XCTAssertEqual(failure.message, "ポケモンのデータを読み込めません")
    }

    func testPresetNamesMatchWeb() {
        let expected: [SpeedPresetID: String] = [
            .uninvested: "無振り", .neutralMax: "準速", .max: "最速",
            .maxScarf: "最速スカーフ", .maxPlus1: "最速+1", .maxPlus2: "最速+2",
        ]
        XCTAssertEqual(Set(expected.keys), Set(SpeedPresetID.allCases), "全6行の名前を固定する")
        for (id, name) in expected {
            XCTAssertEqual(SpeedLabels.preset(id), name)
        }
        for minimal in SpeedMinimalPreset.allCases {
            let same = SpeedPresetID(rawValue: minimal.rawValue)
            XCTAssertEqual(SpeedLabels.preset(minimal), same.map { SpeedLabels.preset($0) }, "自分の調整名は表の同じ行と同じ語")
        }
    }

    func testNatureNamesMatchWeb() {
        XCTAssertEqual(SpeedNature.allCases.map(SpeedLabels.nature), ["下降", "補正なし", "上昇"])
    }

    func testFixedTextsMatchWeb() {
        XCTAssertEqual(SpeedLabels.openButton, "素早さを比べる")
        XCTAssertEqual(SpeedLabels.selfRegion, "自分のポケモン")
        XCTAssertEqual(SpeedLabels.fieldGroup, "場の状態")
        XCTAssertEqual(SpeedLabels.trickRoom, "トリックルーム")
        XCTAssertEqual(SpeedLabels.selfTailwind, "追い風(自分側)")
        XCTAssertEqual(SpeedLabels.scarf, "こだわりスカーフ")
        XCTAssertEqual(SpeedLabels.loading, "読み込み中")
        XCTAssertEqual(SpeedLabels.unselected, "未選択")
        XCTAssertEqual(SpeedLabels.sp, "素早さ SP")
        XCTAssertEqual(SpeedLabels.rank, "ランク")
        XCTAssertEqual(SpeedLabels.selfSpeed(301), "実数値 301")
    }
}
