import Foundation
import XCTest

@testable import PokeCalcCore

// LocalTeamStore(P6-2c・ADR-0500 §4「TeamStore プロトコルと端末内の実装(UserDefaults に JSON)」)。
//
// 決定した形(ADR-0501「P6-2c」参照):
// - `TeamStore`(protocol, `Sendable`): `list() async throws -> [Team]` /
//   `get(id: String) async throws -> Team?` / `save(_ team: Team) async throws`(id が既存なら更新、
//   無ければ追加) / `delete(id: String) async throws`。並べ替え(reorder/move)はこのタスクの範囲外
//   (requirements.md に並べ替えの要求が無く、実装するとしても UI が無いと確かめられないため。
//   必要になったら follow-up で `TeamStore` に足す)。
// - `LocalTeamStore`(`actor`, `TeamStore` に準拠): `init(defaults: UserDefaults = .standard)`。
//   `ClientIdentity(defaults:)` と同じ「保存先を注入できる」形にし、テストは専用の UserDefaults suite を使う
//   (`ClientIdentityTests` と同じ手法)。
// - 保存キー: `LocalTeamStore.teamsDefaultsKey == "PokeCalcTeams"`(`ClientIdentity.deviceIDDefaultsKey`
//   `"PokeCalcDeviceID"` と衝突しない)。
// - 保存形式: **1つのキーの下に teams の配列を丸ごと JSON で保存する**(team ごとに別キーにしない)。
//   理由: 端末内のデータは最大6体 × 数チーム程度で小さく、複数キーにすると「どのチームがあるか」を知るための
//   索引キーが別途要り、索引と実体の不整合(索引にあるが実体が無い等)が起こりうる。1キーなら常に一貫する。
// - `save`: 保存前に `TeamValidator.firstViolation(in:)` を確かめ、違反があればその `PokeCalcError` を投げて
//   保存しない(値を丸めて保存し直したりしない。TeamDomainTypesTests のコメント参照)。
// - 壊れたデータ(UserDefaults の値が JSON として teams にデコードできない)は
//   `PokeCalcError(code: PokeCalcError.Code.teamStoreCorrupted)` を投げる(黙って空配列にしない。
//   データ破損に気付けるようにする)。
final class LocalTeamStoreTests: XCTestCase {

    private var suiteName = ""
    private var defaults = UserDefaults()

    override func setUpWithError() throws {
        suiteName = "pokecalc-tests-team-store-\(UUID().uuidString)"
        defaults = try XCTUnwrap(UserDefaults(suiteName: suiteName))
    }

    override func tearDown() {
        defaults.removePersistentDomain(forName: suiteName)
    }

    private func member(name species: String = "test-species-alpha") -> TeamMember {
        TeamMember(speciesKey: species, natureId: "test-nature-neutral")
    }

    // MARK: - キー

    func testDefaultsKeyDoesNotCollideWithClientIdentity() {
        XCTAssertEqual(LocalTeamStore.teamsDefaultsKey, "PokeCalcTeams")
        XCTAssertNotEqual(LocalTeamStore.teamsDefaultsKey, ClientIdentity.deviceIDDefaultsKey)
    }

    // MARK: - list / save / get / delete の基本往復

    func testListIsEmptyInitially() async throws {
        let store = LocalTeamStore(defaults: defaults)
        let teams = try await store.list()
        XCTAssertEqual(teams, [])
    }

    func testSaveThenListAndGet() async throws {
        let store = LocalTeamStore(defaults: defaults)
        let team = Team(id: "team-1", name: "テストチーム1", members: [member()])
        try await store.save(team)

        let listed = try await store.list()
        XCTAssertEqual(listed, [team])

        let fetched = try await store.get(id: "team-1")
        XCTAssertEqual(fetched, team)
    }

    func testGetUnknownIDReturnsNil() async throws {
        let store = LocalTeamStore(defaults: defaults)
        let fetched = try await store.get(id: "does-not-exist")
        XCTAssertNil(fetched)
    }

    /// 同じ id で save すると更新(元の並び順の位置を保つ。末尾に付け替えたりしない)。
    func testSaveWithExistingIDUpdatesInPlace() async throws {
        let store = LocalTeamStore(defaults: defaults)
        try await store.save(Team(id: "team-1", name: "テストチーム1"))
        try await store.save(Team(id: "team-2", name: "テストチーム2"))
        try await store.save(Team(id: "team-1", name: "テストチーム1改"))

        let listed = try await store.list()
        XCTAssertEqual(listed.map(\.id), ["team-1", "team-2"], "並び順は追加順のまま")
        XCTAssertEqual(listed.first(where: { $0.id == "team-1" })?.name, "テストチーム1改")
    }

    func testSaveWithNewIDAppends() async throws {
        let store = LocalTeamStore(defaults: defaults)
        try await store.save(Team(id: "team-1", name: "テストチーム1"))
        try await store.save(Team(id: "team-2", name: "テストチーム2"))
        let listed = try await store.list()
        XCTAssertEqual(listed.map(\.id), ["team-1", "team-2"])
    }

    func testDeleteRemovesTeam() async throws {
        let store = LocalTeamStore(defaults: defaults)
        try await store.save(Team(id: "team-1", name: "テストチーム1"))
        try await store.save(Team(id: "team-2", name: "テストチーム2"))
        try await store.delete(id: "team-1")
        let listed = try await store.list()
        XCTAssertEqual(listed.map(\.id), ["team-2"])
    }

    /// 無い id を delete しても失敗しない(冪等)。
    func testDeleteUnknownIDDoesNotThrow() async throws {
        let store = LocalTeamStore(defaults: defaults)
        try await store.delete(id: "does-not-exist")
    }

    // MARK: - 別の保存先は別のデータ(端末内保存なので、他の suite を巻き込まない)

    func testDifferentStoresAreIndependent() async throws {
        let otherSuite = "pokecalc-tests-team-store-other-\(UUID().uuidString)"
        let otherDefaults = try XCTUnwrap(UserDefaults(suiteName: otherSuite))
        defer { otherDefaults.removePersistentDomain(forName: otherSuite) }

        let store = LocalTeamStore(defaults: defaults)
        let other = LocalTeamStore(defaults: otherDefaults)
        try await store.save(Team(id: "team-1", name: "テストチーム1"))

        let storeCount = try await store.list().count
        let otherCount = try await other.list().count
        XCTAssertEqual(storeCount, 1)
        XCTAssertEqual(otherCount, 0)
    }

    // MARK: - 永続化(同じ suite を指す新しいインスタンスが読める。UserDefaults 経由の JSON を確かめる)

    func testDataPersistsAcrossInstances() async throws {
        let first = LocalTeamStore(defaults: defaults)
        let team = Team(
            id: "team-1", name: "テストチーム1",
            members: [
                TeamMember(
                    id: "member-1", speciesKey: "test-species-alpha", nickname: "テストのニックネーム",
                    moveIds: ["move-1", "move-2"], itemId: "item-1", abilityId: "ability-1",
                    natureId: "nature-1", sp: StatBlock(hp: 4, atk: 32, def: 0, spa: 0, spd: 0, spe: 30),
                    teraType: .dragon
                )
            ]
        )
        try await first.save(team)

        let second = LocalTeamStore(defaults: defaults)
        let listed = try await second.list()
        XCTAssertEqual(listed, [team])
    }

    /// UserDefaults に生の JSON 文字列として入っていても読める(実装が具体的にどう保存するにせよ、
    /// `save` → 新しいインスタンスの `list` で往復することだけを外部から確認する。上のテストと合わせて、
    /// 「1キーの下に配列を丸ごと JSON で持つ」という決定を、保存形式を直接覗かずに固定する)。
    func testCorruptedStoredValueThrowsTeamStoreCorrupted() async throws {
        defaults.set("not valid json", forKey: LocalTeamStore.teamsDefaultsKey)
        let store = LocalTeamStore(defaults: defaults)
        let error = await assertThrowsPokeCalcError("list() with corrupted data") {
            try await store.list()
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.teamStoreCorrupted)
    }

    // MARK: - save のバリデーション(TeamValidator を通す)

    func testSaveRejectsInvalidTeamAndDoesNotPersist() async throws {
        let store = LocalTeamStore(defaults: defaults)
        let invalid = Team(id: "team-1", name: "") // 空の名前
        let error = await assertThrowsPokeCalcError("save invalid team") {
            try await store.save(invalid)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.teamNameEmpty)

        let listed = try await store.list()
        XCTAssertEqual(listed, [], "無効なチームは保存されない")
    }

    func testSaveRejectsTeamWithDuplicateMoves() async throws {
        let store = LocalTeamStore(defaults: defaults)
        let invalidMember = TeamMember(
            speciesKey: "test-species-alpha", moveIds: ["move-a", "move-a"], natureId: "test-nature-neutral"
        )
        let error = await assertThrowsPokeCalcError("save duplicate moves") {
            try await store.save(Team(name: "テストチーム", members: [invalidMember]))
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.teamDuplicateMoves)
    }
}
