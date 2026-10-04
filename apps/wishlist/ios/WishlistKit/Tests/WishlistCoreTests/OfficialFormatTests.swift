import XCTest

@testable import WishlistCore

// AC-IOS-OFF-01〜04: 公式サイトの販売状況の表示(純関数。PWA の AC-OFF-01〜04 と同じ文言・規則。docs/phase4-spec.md 4-3)。
final class OfficialFormatTests: XCTestCase {
    /// JST 10/4 3:00
    private let checked = iso("2026-10-03T18:00:00Z")

    private func status(
        _ state: OfficialState = .preorder, evidence: [String] = ["予約受付中", "予約する"], changedAt: Date? = nil,
        previous: OfficialState? = nil, last: OfficialState? = nil, lastAt: Date? = nil
    ) -> OfficialStatus {
        OfficialStatus(
            status: state, evidence: evidence, checkedAt: checked, changedAt: changedAt, previousStatus: previous, lastResult: last ?? state,
            lastAttemptAt: lastAt ?? checked)
    }

    func testLabels() {
        let table: [(OfficialState, String)] = [
            (.available, "販売中"), (.preorder, "予約受付中"), (.soldout, "在庫切れ"), (.ended, "販売終了"),
            (.unknown, "判定できません"), (.ambiguous, "判定できません(複数の表示)"),
            (.blocked, "取得しません(robots.txt)"), (.failed, "取得できませんでした"),
        ]
        for (state, want) in table { XCTAssertEqual(OfficialFormat.label(state), want, state.rawValue) }
        XCTAssertEqual(OfficialState.allCases.count, 8)
    }

    func testSummary() {
        let table: [(String, OfficialStatus?, String)] = [
            ("判定済み", status(), "公式: 予約受付中(10/4 確認)"),
            ("unknown", status(.unknown, evidence: []), "公式: 判定できません(10/4 確認)"),
            ("ambiguous", status(.ambiguous), "公式: 判定できません(複数の表示)(10/4 確認)"),
            ("一度も判定できていない failed", status(.failed, evidence: []), "公式: 取得できませんでした(10/4)"),
            ("一度も判定できていない blocked", status(.blocked, evidence: []), "公式: 取得しません(robots.txt)(10/4)"),
            ("まだ確かめていない", nil, "公式: まだ確認していません"),
            ("日付は checkedAt", status(last: .failed, lastAt: iso("2026-10-05T18:00:00Z")), "公式: 予約受付中(10/4 確認)"),
        ]
        for (name, value, want) in table { XCTAssertEqual(OfficialFormat.summary(value), want, name) }
    }

    func testEvidence() {
        XCTAssertEqual(OfficialFormat.evidence(status()), "根拠: 予約受付中・予約する")
        XCTAssertEqual(OfficialFormat.evidence(status(.soldout, evidence: ["SOLD OUT"])), "根拠: SOLD OUT")
        XCTAssertNil(OfficialFormat.evidence(status(.unknown, evidence: [])))
        XCTAssertNil(OfficialFormat.evidence(nil))
    }

    func testChangeWithinSevenDays() {
        let changed = status(.ended, evidence: ["販売終了"], changedAt: iso("2026-10-02T18:00:00Z"), previous: .available)
        let table: [(String, OfficialStatus?, String, String?)] = [
            ("当日", changed, "2026-10-02T20:00:00Z", "10/3 に 販売中 → 販売終了"),
            ("ちょうど 7 日", changed, "2026-10-09T18:00:00Z", "10/3 に 販売中 → 販売終了"),
            ("7 日を 1 秒過ぎた", changed, "2026-10-09T18:00:01Z", nil),
            ("未来の changedAt(時計のずれ)", changed, "2026-10-01T00:00:00Z", "10/3 に 販売中 → 販売終了"),
            ("変化なし", status(), "2026-10-04T00:00:00Z", nil),
            (
                "判定できない状態への変化", status(.unknown, evidence: [], changedAt: iso("2026-10-02T18:00:00Z"), previous: .preorder),
                "2026-10-04T00:00:00Z", "10/3 に 予約受付中 → 判定できません"
            ),
            ("nil", nil, "2026-10-04T00:00:00Z", nil),
        ]
        for (name, value, now, want) in table { XCTAssertEqual(OfficialFormat.change(value, now: iso(now)), want, name) }
        XCTAssertEqual(OfficialFormat.changeDays, 7)
    }

    func testLastAttempt() {
        let later = iso("2026-10-04T18:00:00Z")
        XCTAssertEqual(OfficialFormat.lastAttempt(status(last: .failed, lastAt: later)), "最新の確認(10/5): 取得できませんでした")
        XCTAssertEqual(OfficialFormat.lastAttempt(status(last: .blocked, lastAt: later)), "最新の確認(10/5): 取得しません(robots.txt)")
        XCTAssertNil(OfficialFormat.lastAttempt(status()), "最後も判定できた")
        XCTAssertNil(OfficialFormat.lastAttempt(status(.failed, evidence: [])), "status 自体が failed")
        XCTAssertNil(OfficialFormat.lastAttempt(nil))
    }
}
