import Foundation
import XCTest

@testable import PokeCalcCore

// TeamMember / Team / TeamLimits / TeamValidator(P6-2c 構築ビルダー。ADR-0500 §4、ADR-0501「P6-2c」)。
//
// この時点では `TeamMember` / `Team` / `TeamLimits` / `TeamValidator` は実装されていない
// (spec-writer はテストだけを書く。実装は implementer の担当)。このファイルはコンパイルが通った時点で
// 型の形が決まったことを意味する。
//
// 決定した形(ADR-0501「P6-2c」参照):
// - `TeamMember`: id(既定は UUID)・speciesKey・nickname(任意)・moveIds(0..4・重複不可)・itemId(任意)・
//   abilityId(任意)・natureId(必須)・sp: StatBlock(既定 0)・teraType(任意)。`Equatable, Sendable, Codable`。
// - `Team`: id(既定は UUID)・name(必須。前後空白のみは無効)・members(0..6、追加順。並べ替えはこのタスクの範囲外)。
//   `Equatable, Sendable, Codable`。
// - `TeamLimits.maxMembers == 6`(requirements.md「構築ビルダー」の6体パーティ)。
//   `TeamLimits.maxMovesPerMember == 4`(ADR-0016 §1 と同じ規則。同一メンバー内の技 ID 重複は不可)。
// - `SPLimits.maxTotal == 66`(CLAUDE.md ドメイン規約「合計66」。既存の `SPLimits`(DomainTypes.swift)に
//   `maxPerStat` と並べて追加する。新しい型は作らない)。
// - `TeamValidator.firstViolation(in:) -> PokeCalcError?`: 純粋関数。最初に見つかった違反を1つだけ返す
//   (無ければ nil)。判定順(ADR-0016 §4 と同じ流儀。早いものが勝つ):
//     1. name(前後空白を落として空) → `PokeCalcError.Code.teamNameEmpty`
//     2. members.count > `TeamLimits.maxMembers` → `.teamTooManyMembers`
//     3. members を先頭から順に見て、各メンバーごとに:
//        a. moveIds.count > `TeamLimits.maxMovesPerMember` → `.teamTooManyMoves`
//        b. moveIds に重複がある → `.teamDuplicateMoves`
//        c. sp のいずれかのステータスが `SPLimits.maxPerStat` を超える → `.teamSPInvalid`
//        d. sp の合計が `SPLimits.maxTotal` を超える → `.teamSPInvalid`
//   `LocalTeamStore.save` はこの関数の結果をそのまま投げる(却下する。値を丸めて保存し直したりしない。
//   理由: SP の超過や重複技を黙って正規化すると、保存した内容が画面の見た目と食い違いうるため)。
final class TeamDomainTypesTests: XCTestCase {

    // MARK: - 定数

    func testTeamLimits() {
        XCTAssertEqual(TeamLimits.maxMembers, 6, "requirements.md 構築ビルダー: 6体のパーティ")
        XCTAssertEqual(TeamLimits.maxMovesPerMember, 4, "ADR-0016 §1 と同じ、メンバーごとの技は最大4つ")
    }

    func testSPTotalLimit() {
        XCTAssertEqual(SPLimits.maxTotal, 66, "CLAUDE.md ドメイン規約: 能力ポイントの合計は66")
        XCTAssertEqual(SPLimits.maxPerStat, 32, "既存の定数は変わらない")
    }

    // MARK: - TeamMember / Team の構築(既定値)

    func testTeamMemberDefaults() {
        let member = TeamMember(speciesKey: "test-species-alpha", natureId: "test-nature-neutral")
        XCTAssertNotNil(UUID(uuidString: member.id), "id は既定で UUID 形式")
        XCTAssertNil(member.nickname)
        XCTAssertEqual(member.moveIds, [])
        XCTAssertNil(member.itemId)
        XCTAssertNil(member.abilityId)
        XCTAssertEqual(member.sp, StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
        XCTAssertNil(member.teraType)
    }

    func testTeamMemberExplicitValuesAreKept() {
        let member = TeamMember(
            id: "member-1", speciesKey: "test-species-alpha", nickname: "テストのニックネーム",
            moveIds: ["move-1", "move-2"], itemId: "item-1", abilityId: "ability-1",
            natureId: "nature-1", sp: StatBlock(hp: 4, atk: 32, def: 0, spa: 0, spd: 0, spe: 30),
            teraType: .fire
        )
        XCTAssertEqual(member.id, "member-1")
        XCTAssertEqual(member.nickname, "テストのニックネーム")
        XCTAssertEqual(member.moveIds, ["move-1", "move-2"])
        XCTAssertEqual(member.itemId, "item-1")
        XCTAssertEqual(member.abilityId, "ability-1")
        XCTAssertEqual(member.natureId, "nature-1")
        XCTAssertEqual(member.teraType, .fire)
    }

    func testTeamDefaults() {
        let team = Team(name: "テストチーム")
        XCTAssertNotNil(UUID(uuidString: team.id))
        XCTAssertEqual(team.name, "テストチーム")
        XCTAssertEqual(team.members, [])
    }

    // MARK: - Codable round-trip(LocalTeamStore が JSON で永続化するための前提)

    func testTeamRoundTripsThroughJSON() throws {
        let member = TeamMember(
            id: "member-1", speciesKey: "test-species-alpha", nickname: "テストのニックネーム",
            moveIds: ["move-1", "move-2", "move-3", "move-4"], itemId: "item-1", abilityId: "ability-1",
            natureId: "nature-1", sp: StatBlock(hp: 4, atk: 32, def: 0, spa: 0, spd: 0, spe: 30),
            teraType: .water
        )
        let team = Team(id: "team-1", name: "テストチーム", members: [member])
        let data = try JSONEncoder().encode(team)
        let decoded = try JSONDecoder().decode(Team.self, from: data)
        XCTAssertEqual(decoded, team)
    }

    /// 任意フィールドが nil のときも往復する(nickname / itemId / abilityId / teraType 無し、moveIds 空)。
    func testTeamWithOptionalFieldsNilRoundTrips() throws {
        let member = TeamMember(speciesKey: "test-species-alpha", natureId: "nature-1")
        let team = Team(name: "テストチーム2", members: [member])
        let data = try JSONEncoder().encode(team)
        let decoded = try JSONDecoder().decode(Team.self, from: data)
        XCTAssertEqual(decoded, team)
    }

    // MARK: - TeamValidator(テーブル駆動)

    private func member(
        moveIds: [String] = [], sp: StatBlock = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
    ) -> TeamMember {
        TeamMember(speciesKey: "test-species-alpha", moveIds: moveIds, natureId: "test-nature-neutral", sp: sp)
    }

    func testValidTeamHasNoViolation() {
        let team = Team(name: "テストチーム", members: [member(moveIds: ["m1", "m2"])])
        XCTAssertNil(TeamValidator.firstViolation(in: team))
    }

    func testEmptyMembersIsValid() {
        // 0体は構築の途中の状態として許す(メンバーを1体以上要求しない。requirements.md に下限の指定は無い)。
        XCTAssertNil(TeamValidator.firstViolation(in: Team(name: "テストチーム")))
    }

    func testNameViolations() throws {
        let cases = ["", "   ", "\n\t"]
        for name in cases {
            let error = try XCTUnwrap(TeamValidator.firstViolation(in: Team(name: name)), "name=\(name.debugDescription)")
            XCTAssertEqual(error.code, PokeCalcError.Code.teamNameEmpty, "name=\(name.debugDescription)")
        }
    }

    func testTooManyMembersViolation() throws {
        let members = (0..<(TeamLimits.maxMembers + 1)).map { _ in member() }
        let error = try XCTUnwrap(TeamValidator.firstViolation(in: Team(name: "テストチーム", members: members)))
        XCTAssertEqual(error.code, PokeCalcError.Code.teamTooManyMembers)
    }

    func testExactlyMaxMembersIsValid() {
        let members = (0..<TeamLimits.maxMembers).map { _ in member() }
        XCTAssertNil(TeamValidator.firstViolation(in: Team(name: "テストチーム", members: members)))
    }

    func testTooManyMovesViolation() throws {
        let tooMany = (0..<(TeamLimits.maxMovesPerMember + 1)).map { "move-\($0)" }
        let error = try XCTUnwrap(
            TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(moveIds: tooMany)]))
        )
        XCTAssertEqual(error.code, PokeCalcError.Code.teamTooManyMoves)
    }

    func testExactlyMaxMovesIsValid() {
        let ok = (0..<TeamLimits.maxMovesPerMember).map { "move-\($0)" }
        XCTAssertNil(TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(moveIds: ok)])))
    }

    func testDuplicateMovesViolation() throws {
        let error = try XCTUnwrap(
            TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(moveIds: ["move-a", "move-a"])]))
        )
        XCTAssertEqual(error.code, PokeCalcError.Code.teamDuplicateMoves)
    }

    func testSPPerStatOverLimitViolation() throws {
        let sp = StatBlock(hp: 0, atk: SPLimits.maxPerStat + 1, def: 0, spa: 0, spd: 0, spe: 0)
        let error = try XCTUnwrap(
            TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(sp: sp)]))
        )
        XCTAssertEqual(error.code, PokeCalcError.Code.teamSPInvalid)
    }

    func testSPAtPerStatLimitIsValid() {
        let sp = StatBlock(hp: 0, atk: SPLimits.maxPerStat, def: 0, spa: 0, spd: 0, spe: 0)
        XCTAssertNil(TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(sp: sp)])))
    }

    /// issue #101: 負の SP は `$0 > SPLimits.maxPerStat` だけの判定だと通ってしまう(上限だけを見ていて
    /// 0 未満を見ていないため)。`$0 < 0 || $0 > SPLimits.maxPerStat` で拒否する(既定案)。
    func testNegativeSPPerStatViolation() throws {
        let sp = StatBlock(hp: 0, atk: -1, def: 0, spa: 0, spd: 0, spe: 0)
        let error = try XCTUnwrap(
            TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(sp: sp)]))
        )
        XCTAssertEqual(error.code, PokeCalcError.Code.teamSPInvalid)
    }

    /// issue #101: 負の値は合計検査も素通りさせる(例: -1 + 32*2 + 3 = 66 と、上限内に収まって見える)。
    /// 単体の負数チェックが先に効くことを確かめる。
    func testNegativeSPStillViolatesEvenWhenTotalLooksWithinLimit() throws {
        let sp = StatBlock(hp: -1, atk: 32, def: 32, spa: 3, spd: 0, spe: 0)
        XCTAssertEqual(sp.total, 66, "合計だけ見ると上限ちょうどに見える")
        let error = try XCTUnwrap(
            TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(sp: sp)]))
        )
        XCTAssertEqual(error.code, PokeCalcError.Code.teamSPInvalid)
    }

    func testSPTotalOverLimitViolation() throws {
        // 各ステータス上限内でも合計が66を超える(32 * 3 = 96)。
        let sp = StatBlock(hp: 0, atk: 32, def: 32, spa: 32, spd: 0, spe: 0)
        let error = try XCTUnwrap(
            TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(sp: sp)]))
        )
        XCTAssertEqual(error.code, PokeCalcError.Code.teamSPInvalid)
    }

    func testSPAtTotalLimitIsValid() {
        // 32 + 32 + 2 = 66(ちょうど上限)。
        let sp = StatBlock(hp: 0, atk: 32, def: 32, spa: 2, spd: 0, spe: 0)
        XCTAssertNil(TeamValidator.firstViolation(in: Team(name: "テストチーム", members: [member(sp: sp)])))
    }

    /// 2体目の違反も見つかる(1体目だけを見ていないことの確認)。
    func testViolationInSecondMemberIsFound() throws {
        let team = Team(name: "テストチーム", members: [member(), member(moveIds: ["move-a", "move-a"])])
        let error = try XCTUnwrap(TeamValidator.firstViolation(in: team))
        XCTAssertEqual(error.code, PokeCalcError.Code.teamDuplicateMoves)
    }

    /// エラーコードは `client_` 接頭辞(PokeCalcError.swift の規約)。
    func testTeamErrorCodesUseClientPrefix() {
        let codes = [
            PokeCalcError.Code.teamNameEmpty,
            PokeCalcError.Code.teamTooManyMembers,
            PokeCalcError.Code.teamTooManyMoves,
            PokeCalcError.Code.teamDuplicateMoves,
            PokeCalcError.Code.teamSPInvalid,
            PokeCalcError.Code.teamStoreCorrupted,
        ]
        for code in codes {
            XCTAssertTrue(code.hasPrefix("client_"), code)
        }
        XCTAssertEqual(Set(codes).count, codes.count, "コードはすべて別の値")
    }
}
