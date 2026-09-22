import Foundation
import XCTest

@testable import PokeCalcCore

/// `TeamListViewModel` / `TeamEditViewModel` のテスト用の `TeamStore`(P6-2c)。
/// `StubPokeCalcService.swift` と同じ流儀: `actor`、呼び出しをすべて記録し、失敗を注入できる。
/// `LocalTeamStore` の実装(UserDefaults・JSON)に依存せず、ViewModel のロジックだけを確かめる。
actor StubTeamStore: TeamStore {
    /// `list()` の応答の返し方(P6-2d。`StubPokeCalcService.SpeciesMode` と同じ流儀)。
    enum ListMode {
        /// 呼ばれたらすぐ返す(既定。P6-2c のテストの前提を変えない)。
        case immediate
        /// 応答を保留する。テストが `resolveList(at:with:)` で**任意の順序**に返す
        /// (古い一覧の応答が後から届く状況を再現する。P6-2d 6章の世代の保護)。
        case manual
    }

    /// 保留の待ち合わせで条件がそろうまで待つ上限(1ms × 回数)。`StubPokeCalcService` と同じ値。
    private static let waitPollLimit = 10000

    private(set) var teams: [Team]
    private(set) var saveCalls: [Team] = []
    private(set) var deleteCalls: [String] = []
    private(set) var listCallCount = 0

    private var saveError: PokeCalcError?
    private var listError: PokeCalcError?
    private var listMode: ListMode = .immediate
    private var pendingList: [Int: CheckedContinuation<[Team], any Error>] = [:]

    init(teams: [Team] = []) {
        self.teams = teams
    }

    func setSaveError(_ error: PokeCalcError?) {
        saveError = error
    }

    func setListError(_ error: PokeCalcError?) {
        listError = error
    }

    func setListMode(_ mode: ListMode) {
        listMode = mode
    }

    /// 保留中の `index` 番目(0 始まり、`list()` が呼ばれた順)の応答を返す。
    func resolveList(at index: Int, with result: Result<[Team], PokeCalcError>) {
        guard let continuation = pendingList.removeValue(forKey: index) else {
            XCTFail("保留中の list が無い: index \(index)")
            return
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    /// `list()` が `count` 回以上呼ばれるまで待つ。
    func waitForListCalls(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if listCallCount >= count { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("list が \(count) 回呼ばれなかった(\(listCallCount) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "list の待ち合わせがタイムアウト")
    }

    /// テストから直接状態を差し込む(`save` のバリデーションを経由せずに、あらかじめ不正な状態を
    /// 作っておきたいときに使う。通常は `save` 経由で組み立てる)。
    func seed(_ teams: [Team]) {
        self.teams = teams
    }

    func list() async throws -> [Team] {
        let index = listCallCount
        listCallCount += 1
        if let listError { throw listError }
        switch listMode {
        case .immediate:
            return teams
        case .manual:
            return try await withCheckedThrowingContinuation { continuation in
                pendingList[index] = continuation
            }
        }
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

// MARK: - 架空の構築(P6-2d「構築から個体を呼び出す」用。名前はすべて「テスト」で始める)

/// `CalcViewModelTeamIndividualTests` / `ReverseViewModelTeamIndividualTests` の架空の構築。
/// 種族・技・持ち物・性格は `StubMaster`(`Support/StubPokeCalcService.swift`)のものを使い、
/// **どの `AttackerPreset` / `KnownDefenderPreset` とも一致しない** SP と性格を持たせる
/// (プリセットから作り直していないことを、値そのもので見分けられるようにするため)。
enum StubTeams {
    /// プリセットが作りうる形(関連ステータスだけに 0 か 32)のどれとも一致しない手詰めの SP。
    /// 各値は `SPLimits.maxPerStat` 以下、合計は `SPLimits.maxTotal` 以下。
    static let customSP = StatBlock(hp: 12, atk: 20, def: 4, spa: 0, spd: 8, spe: 6)

    /// `StubMaster.alpha` の learnset にある技(= 呼び出した技がそのまま残る個体)。
    /// 性格は特攻上昇で、物理技のときの A特化(攻撃上昇)とも無補正とも違う。
    static let namedMember = TeamMember(
        id: "stub-member-named", speciesKey: StubMaster.alpha.key, nickname: "テストこたいニックネーム",
        moveIds: [StubMaster.specialMove.id], itemId: StubMaster.itemB.id, abilityId: StubMaster.ability.id,
        natureId: StubMaster.spaUpNature.id, sp: customSP, teraType: .water
    )
    /// 技を1つも登録していない個体(`moveIds` は 0〜4 なので構築ビルダー上ふつうに作れる)。
    /// ニックネームが無いので表示名は種族名になる。
    static let noMoveMember = TeamMember(
        id: "stub-member-no-move", speciesKey: StubMaster.alpha.key,
        moveIds: [], itemId: nil, abilityId: nil,
        natureId: StubMaster.spaUpNature.id, sp: customSP
    )
    /// `StubMaster.alpha` が覚えない技だけを持つ個体(learnset とマスタの突き合わせで落ちる)。
    static let unlearnedMoveMember = TeamMember(
        id: "stub-member-unlearned-move", speciesKey: StubMaster.alpha.key, nickname: "テストこたいわざちがい",
        moveIds: [StubMaster.physicalMove.id], natureId: StubMaster.spaUpNature.id, sp: customSP
    )
    /// 変化技だけを持つ個体。計算画面では変化技も選択肢にあるのでそのまま選ばれ、
    /// 逆算画面では選択肢がダメージ技だけなので落ちる(画面ごとの規則の違いを見分ける)。
    static let statusMoveMember = TeamMember(
        id: "stub-member-status-move", speciesKey: StubMaster.alpha.key, nickname: "テストこたいへんかわざ",
        moveIds: [StubMaster.statusMove.id], natureId: StubMaster.spaUpNature.id, sp: customSP
    )

    static let teamAlpha = Team(id: "stub-team-alpha", name: "テストこうちくアルファ", members: [namedMember, noMoveMember])
    static let teamBeta = Team(
        id: "stub-team-beta", name: "テストこうちくベータ", members: [unlearnedMoveMember, statusMoveMember]
    )
    /// メンバーが0体の構築(`teamOptions` に出さない。ADR-0501「P6-2d」6章)。
    static let emptyTeam = Team(id: "stub-team-empty", name: "テストこうちくからっぽ", members: [])

    static let all = [teamAlpha, teamBeta, emptyTeam]

    static func makeStore(teams: [Team] = all) -> StubTeamStore {
        StubTeamStore(teams: teams)
    }
}
