import Foundation
import XCTest

@testable import PokeCalcCore

/// `TeamListViewModel` / `TeamEditViewModel` のテスト用の `TeamStore`(P6-2c)。
/// `StubPokeCalcService.swift` と同じ流儀: `actor`、呼び出しをすべて記録し、失敗を注入できる。
/// `LocalTeamStore` の実装(UserDefaults・JSON)に依存せず、ViewModel のロジックだけを確かめる。
actor StubTeamStore: TeamStore {
    private(set) var teams: [Team]
    private(set) var saveCalls: [Team] = []
    private(set) var deleteCalls: [String] = []
    private(set) var listCallCount = 0

    private var saveError: PokeCalcError?
    private var listError: PokeCalcError?

    init(teams: [Team] = []) {
        self.teams = teams
    }

    func setSaveError(_ error: PokeCalcError?) {
        saveError = error
    }

    func setListError(_ error: PokeCalcError?) {
        listError = error
    }

    /// テストから直接状態を差し込む(`save` のバリデーションを経由せずに、あらかじめ不正な状態を
    /// 作っておきたいときに使う。通常は `save` 経由で組み立てる)。
    func seed(_ teams: [Team]) {
        self.teams = teams
    }

    func list() async throws -> [Team] {
        listCallCount += 1
        if let listError { throw listError }
        return teams
    }

    func get(id: String) async throws -> Team? {
        teams.first(where: { $0.id == id })
    }

    func save(_ team: Team) async throws {
        saveCalls.append(team)
        if let saveError { throw saveError }
        if let violation = TeamValidator.firstViolation(in: team) { throw violation }
        if let index = teams.firstIndex(where: { $0.id == team.id }) {
            teams[index] = team
        } else {
            teams.append(team)
        }
    }

    func delete(id: String) async throws {
        deleteCalls.append(id)
        teams.removeAll(where: { $0.id == id })
    }
}
