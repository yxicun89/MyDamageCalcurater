import XCTest

@testable import PokeCalcCore

/// 逆算画面の観測入力の検証(P6-2b)。観測はテンキーで数値入力する(DECISIONS.md 2026-09-22)。
///
/// - 与えたダメージ(side = defender)は相手 HP の減少%(整数 1〜100)→ `.percent`
/// - 受けたダメージ(side = attacker)は自分 HP の減少量(実点数 1 以上)→ `.damage`
/// - 0.1% 入力(`.percentTenths`)は画面に出さない(実機の HP 表示は整数%。ADR-0010 §R2 の 2026-09-21 ユーザー回答)
final class ObservationInputTests: XCTestCase {

    func testLimitsMatchContract() {
        // openapi `Observation.percent`(minimum 1 / maximum 100)・`Observation.damage`(minimum 1)、ADR-0010 §R2。
        XCTAssertEqual(ObservationLimits.percentRange, 1...100)
        XCTAssertEqual(ObservationLimits.minimumDamage, 1)
    }

    func testKindFollowsSide() {
        XCTAssertEqual(ObservationKind(side: .defender), .percent)
        XCTAssertEqual(ObservationKind(side: .attacker), .damage)
    }

    func testParsePercent() {
        let cases: [(input: String, expected: Result<DamageObservation, ObservationFieldError>)] = [
            ("12", .success(.percent(12))),
            (" 12 ", .success(.percent(12))),       // 前後の空白は落とす(貼り付け対策)
            ("1", .success(.percent(1))),           // 下限
            ("100", .success(.percent(100))),       // 上限(HP バーの頭打ち。ADR-0010 §R2)
            ("007", .success(.percent(7))),         // 先頭の 0 は数値として読む
            ("", .failure(.empty)),
            ("   ", .failure(.empty)),
            ("0", .failure(.outOfRange)),
            ("101", .failure(.outOfRange)),
            ("99999999999999999999", .failure(.outOfRange)), // 数字だけだが Int に収まらない
            ("abc", .failure(.notANumber)),
            ("12.5", .failure(.notANumber)),        // 小数は受けない(表示%との取り違え防止。ADR-0010 §R8)
            ("-5", .failure(.notANumber)),          // 符号はテンキーに無い
            ("+5", .failure(.notANumber)),
            ("１２", .failure(.notANumber)),          // 全角数字は受けない(ASCII の数字だけ)
            ("1 2", .failure(.notANumber)),
        ]
        for (input, expected) in cases {
            XCTAssertEqual(ObservationParser.parse(input, kind: .percent), expected, "入力 \"\(input)\"")
        }
    }

    func testParseDamage() {
        let cases: [(input: String, expected: Result<DamageObservation, ObservationFieldError>)] = [
            ("1", .success(.damage(1))),
            ("250", .success(.damage(250))),        // % と違い上限は無い(openapi に maximum が無い)
            ("0", .failure(.outOfRange)),
            ("", .failure(.empty)),
            ("x", .failure(.notANumber)),
            ("99999999999999999999", .failure(.outOfRange)),
        ]
        for (input, expected) in cases {
            XCTAssertEqual(ObservationParser.parse(input, kind: .damage), expected, "入力 \"\(input)\"")
        }
    }

    func testFieldErrorMessages() {
        // 範囲の数値は定数から作る(文言に直書きした値と定数がずれないこと)。
        XCTAssertEqual(ObservationFieldError.empty.message(kind: .percent), "値を入力してください")
        XCTAssertEqual(ObservationFieldError.notANumber.message(kind: .damage), "数字だけを入力してください")
        XCTAssertEqual(ObservationFieldError.outOfRange.message(kind: .percent), "1〜100 の整数%を入力してください")
        XCTAssertEqual(ObservationFieldError.outOfRange.message(kind: .damage), "1 以上のダメージを入力してください")
    }
}
