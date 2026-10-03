import XCTest

@testable import PokeCalcCore

/// 素早さ画面の文言(ADR-0503 §7)。Web の `speedScreenText`・`speedPresetText`(web/src/i18n/ja.ts)と同じ文言を固定する。
final class SpeedLabelsTests: XCTestCase {
    /// 契約の `ErrorCode` → 日本語。値は Web の `speedScreenText.errorByCode` と同じ。
    /// `not_found` は Web の表に無く、汎用の文言になる。
    static let expectedErrorMessages: [String: String] = [
        "invalid_request": "入力の形が正しくありません。値の範囲を確認してください",
        "missing_header": "端末の識別情報が送られていません",
        "invalid_header": "端末の識別情報の形が正しくありません",
        "unknown_pokemon": "このポケモンはマスタにありません",
        "request_too_large": "入力が大きすぎます",
        "master_unavailable": "ポケモンのマスタを読み込めません",
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
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: PokeCalcError.Code.transport), "素早さの API に接続できません")
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: PokeCalcError.Code.decode), "素早さの API に接続できません")
    }

    func testUnknownAndUnexpectedCodesFallBackToGenericMessage() {
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: PokeCalcError.Code.unexpectedStatus), "素早さの計算に失敗しました")
        XCTAssertEqual(SpeedLabels.errorMessage(forCode: "some_future_code"), "素早さの計算に失敗しました")
    }

    func testFailureShowsJapaneseMessageNeverTheServerMessage() {
        let failure = SpeedFailure(code: "master_unavailable")
        XCTAssertEqual(failure.message, "ポケモンのマスタを読み込めません")
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

    func testModeAndNatureNamesMatchWeb() {
        XCTAssertEqual(SpeedInputMode.allCases.map(SpeedLabels.mode), ["プリセット", "カスタム", "実数値"])
        XCTAssertEqual(SpeedNature.allCases.map(SpeedLabels.nature), ["下降", "補正なし", "上昇"])
    }

    func testFixedTextsMatchWeb() {
        XCTAssertEqual(SpeedLabels.openButton, "素早さを比べる")
        XCTAssertEqual(SpeedLabels.tableRegion, "素早さの表")
        XCTAssertEqual(SpeedLabels.selfRegion, "自分のポケモン")
        XCTAssertEqual(SpeedLabels.filterGroup, "表の絞り込み")
        XCTAssertEqual(SpeedLabels.filterMinimumNotice, "少なくとも1つは選ぶ必要があります")
        XCTAssertEqual(SpeedLabels.fieldGroup, "場の状態")
        XCTAssertEqual(SpeedLabels.tableTailwind, "追い風(相手側)")
        XCTAssertEqual(SpeedLabels.trickRoom, "トリックルーム")
        XCTAssertEqual(SpeedLabels.selfTailwind, "追い風(自分側)")
        XCTAssertEqual(SpeedLabels.paralysis, "まひ")
        XCTAssertEqual(SpeedLabels.scarf, "こだわりスカーフ")
        XCTAssertEqual(SpeedLabels.tie, "同速")
        XCTAssertEqual(SpeedLabels.noTie, "同速なし")
        XCTAssertEqual(SpeedLabels.selfTier, "自分と同速")
        XCTAssertEqual(SpeedLabels.selfBoundary, "ここに自分が入る")
        XCTAssertEqual(SpeedLabels.loading, "読み込み中")
        XCTAssertEqual(SpeedLabels.positionLoading, "位置を計算中")
        XCTAssertEqual(SpeedLabels.unselected, "未選択")
        XCTAssertEqual(SpeedLabels.sp, "素早さ SP")
        XCTAssertEqual(SpeedLabels.rank, "ランク")
        XCTAssertEqual(SpeedLabels.rawValue, "実数値")
    }

    func testParameterizedTextsMatchWeb() {
        XCTAssertEqual(SpeedLabels.tierSpeed(250), "素早さ 250")
        XCTAssertEqual(SpeedLabels.selfSpeed(301), "実数値 301")
        XCTAssertEqual(SpeedLabels.faster(12), "自分より速い 12行")
        XCTAssertEqual(SpeedLabels.slower(30), "自分より遅い 30行")
        XCTAssertEqual(SpeedLabels.movesBefore(5), "自分より先に動く 5行")
        XCTAssertEqual(SpeedLabels.movesAfter(3), "自分より後に動く 3行")
    }

    /// 範囲の数値は既存の定数から(文言に 32・±6 を直書きしない)。
    func testRangeMessagesUseSharedLimits() {
        XCTAssertEqual(SpeedLabels.spRange(max: SPLimits.maxPerStat), "能力ポイントは0〜32の整数で入力してください")
        XCTAssertEqual(
            SpeedLabels.rankRange(min: RankLimits.min, max: RankLimits.max), "ランクは-6〜+6の整数で入力してください")
        XCTAssertEqual(SpeedLabels.rawValueRange, "実数値は1以上の整数で入力してください")
    }
}
