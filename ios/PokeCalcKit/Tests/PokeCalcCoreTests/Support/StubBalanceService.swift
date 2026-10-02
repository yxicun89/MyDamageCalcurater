import Foundation
import XCTest

@testable import PokeCalcCore

/// タイプバランスのテスト用フィクスチャ(架空。数値・タイプに意味は無い。ADR-0002。相性表の写しも持たない)。
enum StubBalance {
    /// 防御の応答(要求と同じ数・同じ順)。メンバー i の攻撃タイプ 0(normal)は i で変わる(行の取り違えを検出する):
    /// i = 0 は 4 倍弱点 / 1 は弱点 / 2 は耐性 / それ以外は等倍。特性を持つメンバーの攻撃タイプ 1(fire)は特性で無効(source = ability・effect = immune)。
    /// 集計はこの応答から数えた値(分類の判定は応答の値のまま。テストは判定しない)。
    static func defense(for request: BalanceAnalyzeRequest) -> BalanceAnalyzeResponse {
        let members = request.members.enumerated().map { index, member in
            BalanceMemberDefense(
                pokemonId: member.pokemonId, abilityId: member.abilityId, types: [PokeType.allCases[index % PokeType.allCases.count]],
                defense: PokeType.allCases.enumerated().map { typeIndex, type in
                    if typeIndex == 0 {
                        switch index {
                        case 0: return BalanceDefenseEntry(attackType: type, multiplier: "4", category: .quadWeak)
                        case 1: return BalanceDefenseEntry(attackType: type, multiplier: "2", category: .weak)
                        case 2: return BalanceDefenseEntry(attackType: type, multiplier: "1/2", category: .resist)
                        default: break
                        }
                    }
                    if typeIndex == 1, member.abilityId != nil {
                        return BalanceDefenseEntry(attackType: type, multiplier: "0", category: .immune, source: .ability, effect: .immune)
                    }
                    return BalanceDefenseEntry(attackType: type, multiplier: "1", category: .neutral)
                })
        }
        let summary = PokeType.allCases.enumerated().map { typeIndex, type in
            let entries = members.map { $0.defense[typeIndex] }
            return BalanceTeamSummaryEntry(
                attackType: type,
                weak: entries.filter { $0.category == .weak || $0.category == .quadWeak }.count,
                quadWeak: entries.filter { $0.category == .quadWeak }.count,
                resist: entries.filter { $0.category == .resist || $0.category == .quadResist }.count,
                immune: entries.filter { $0.category == .immune }.count,
                neutral: entries.filter { $0.category == .neutral }.count)
        }
        return BalanceAnalyzeResponse(members: members, teamSummary: summary)
    }

    /// 攻撃範囲の応答。技が空のメンバーは attackTypes 空・全タイプ nil。技を持つメンバー i は attackTypes [fire]・防御タイプごとに i が偶数なら ×2(抜群)、奇数なら ×1/2。
    static func coverage(for request: BalanceCoverageRequest) -> BalanceCoverageResponse {
        let members = request.members.enumerated().map { index, member in
            BalanceMemberCoverage(
                pokemonId: member.pokemonId, moveIds: member.moveIds, attackTypes: member.moveIds.isEmpty ? [] : [.fire],
                coverage: PokeType.allCases.map { type in
                    if member.moveIds.isEmpty {
                        return BalanceDefenseCoverageEntry(defenseType: type, bestMultiplier: nil, effective: false, superEffective: false)
                    }
                    let isDouble = index % 2 == 0
                    return BalanceDefenseCoverageEntry(
                        defenseType: type, bestMultiplier: isDouble ? .double : .half, effective: isDouble, superEffective: isDouble)
                })
        }
        let team = PokeType.allCases.enumerated().map { typeIndex, type in
            let entries = members.map { $0.coverage[typeIndex] }
            let best: BalanceCoverageMultiplier? = entries.contains { $0.bestMultiplier == .double }
                ? .double : (entries.contains { $0.bestMultiplier == .half } ? .half : nil)
            return BalanceTeamCoverageEntry(
                defenseType: type, bestMultiplier: best, effectiveMembers: entries.filter(\.effective).count,
                superEffectiveMembers: entries.filter(\.superEffective).count)
        }
        return BalanceCoverageResponse(members: members, teamCoverage: team)
    }

    static let failure = PokeCalcError(code: "overloaded", message: "english message from server")
    static let otherFailure = PokeCalcError(code: "internal_error", message: "english message from server")

    // MARK: - 架空の構築(種族・技・特性は `StubMaster`)

    /// ニックネームあり・特性あり・技 2 つ(物理・特殊)。
    static let memberA = TeamMember(
        id: "bal-member-a", speciesKey: StubMaster.alpha.key, nickname: "テストバランスニックA",
        moveIds: [StubMaster.physicalMove.id, StubMaster.specialMove.id], abilityId: StubMaster.ability.id,
        natureId: StubMaster.neutralNature.id)
    /// ニックネームなし・特性なし・技なし(表示名は種族名)。
    static let memberB = TeamMember(
        id: "bal-member-b", speciesKey: StubMaster.beta.key, moveIds: [], natureId: StubMaster.neutralNature.id)
    /// memberA と同じ種族の別個体(重複した種族の取り違えを検出する)。技は 1 つ・特性は空文字(送らない)。
    static let memberC = TeamMember(
        id: "bal-member-c", speciesKey: StubMaster.alpha.key, moveIds: [StubMaster.statusMove.id], abilityId: "",
        natureId: StubMaster.neutralNature.id)
    /// マスタに無い種族(名前は speciesKey に落ちる)。技 4 つ。
    static let memberUnknown = TeamMember(
        id: "bal-member-unknown", speciesKey: "9999-000", nickname: "   ",
        moveIds: [StubMaster.physicalMove.id, StubMaster.specialMove.id, StubMaster.statusMove.id, StubMaster.alphaOnlyMove.id],
        natureId: StubMaster.neutralNature.id)

    /// A・B(技を持つメンバーと持たないメンバーの混在)。
    static let teamMixed = Team(id: "bal-team-mixed", name: "テストバランスこうちくA", members: [memberA, memberB])
    /// A・B・C・Unknown(同じ種族の重複・マスタに無い種族を含む 4 体)。
    static let teamFour = Team(id: "bal-team-four", name: "テストバランスこうちく4", members: [memberA, memberB, memberC, memberUnknown])
    /// 技を持つメンバーが 1 体もいない(coverage は呼ばない)。
    static let teamNoMoves = Team(id: "bal-team-no-moves", name: "テストバランスわざなし", members: [memberB])
    /// メンバー 0 体(どちらも呼ばない)。
    static let teamEmpty = Team(id: "bal-team-empty", name: "テストバランスからっぽ", members: [])
    /// 6 体(上限ちょうど)。
    static let teamSix = Team(
        id: "bal-team-six", name: "テストバランス6体", members: (0..<6).map { index in
            TeamMember(
                id: "bal-six-\(index)", speciesKey: StubMaster.alpha.key, moveIds: index % 2 == 0 ? [StubMaster.physicalMove.id] : [],
                natureId: StubMaster.neutralNature.id)
        })
    /// 7 体(`TeamValidator` は通らない壊れた保存データ。先頭 6 体だけ送る)。
    static let teamSeven = Team(
        id: "bal-team-seven", name: "テストバランス7体", members: (0..<7).map { index in
            TeamMember(
                id: "bal-seven-\(index)", speciesKey: StubMaster.alpha.key, moveIds: [StubMaster.physicalMove.id],
                natureId: StubMaster.neutralNature.id)
        })

    static let allTeams = [teamMixed, teamFour, teamNoMoves, teamEmpty, teamSix]
}

/// `BalanceViewModel` のテスト用 `BalanceService`。呼び出しを操作ごとに記録し、応答はクロージャで決める。
/// `hold(.analyze)` などで、`release` されるまでその操作の全ての呼び出しが返らない(古い応答・cancel・独立性の検証用)。
actor StubBalanceService: BalanceService {
    enum Operation: Hashable, Sendable { case analyze, coverage }

    private(set) var analyzeRequests: [BalanceAnalyzeRequest] = []
    private(set) var coverageRequests: [BalanceCoverageRequest] = []
    /// cancel された呼び出しの番号(操作ごと。0 始まり)。
    private(set) var cancelled: [Operation: [Int]] = [:]

    private var analyzeResponder: @Sendable (BalanceAnalyzeRequest, Int) -> Result<BalanceAnalyzeResponse, PokeCalcError> = { request, _ in
        .success(StubBalance.defense(for: request))
    }
    private var coverageResponder: @Sendable (BalanceCoverageRequest, Int) -> Result<BalanceCoverageResponse, PokeCalcError> = { request, _ in
        .success(StubBalance.coverage(for: request))
    }
    private var holding: Set<Operation> = []
    private var released: Set<String> = []
    /// true なら hold 中に cancel されても返らず、release で(古い要求でも)応答を返す。
    private var ignoresCancellation = false

    private static let pollInterval: Duration = .milliseconds(1)
    private static let pollLimit = 3000

    func setAnalyzeResponder(_ responder: @escaping @Sendable (BalanceAnalyzeRequest, Int) -> Result<BalanceAnalyzeResponse, PokeCalcError>) {
        analyzeResponder = responder
    }

    func setCoverageResponder(_ responder: @escaping @Sendable (BalanceCoverageRequest, Int) -> Result<BalanceCoverageResponse, PokeCalcError>) {
        coverageResponder = responder
    }

    func hold(_ operation: Operation, ignoringCancellation: Bool = false) {
        holding.insert(operation)
        ignoresCancellation = ignoringCancellation
    }

    func release(_ operation: Operation, at index: Int) {
        released.insert("\(operation)-\(index)")
    }

    func waitForCalls(_ operation: Operation, count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.pollLimit {
            if callCount(operation) >= count { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("\(operation) が \(count) 回呼ばれなかった(\(callCount(operation)) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    func waitForCancellation(_ operation: Operation, at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.pollLimit {
            if cancelled[operation, default: []].contains(index) { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("\(operation) の \(index) 番目の呼び出しが cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    func callCount(_ operation: Operation) -> Int {
        switch operation {
        case .analyze: return analyzeRequests.count
        case .coverage: return coverageRequests.count
        }
    }

    private func gate(_ operation: Operation, index: Int) async throws {
        guard holding.contains(operation) else { return }
        let key = "\(operation)-\(index)"
        while !released.contains(key) {
            if ignoresCancellation {
                await Task.yield()
                try? await Task.sleep(for: Self.pollInterval)
                continue
            }
            do {
                try await Task.sleep(for: Self.pollInterval)
            } catch {
                cancelled[operation, default: []].append(index)
                throw CancellationError()
            }
        }
        if !ignoresCancellation, Task.isCancelled {
            cancelled[operation, default: []].append(index)
            throw CancellationError()
        }
    }

    func analyzeTeamBalance(_ request: BalanceAnalyzeRequest) async throws -> BalanceAnalyzeResponse {
        let index = analyzeRequests.count
        analyzeRequests.append(request)
        try await gate(.analyze, index: index)
        return try analyzeResponder(request, index).get()
    }

    func analyzeTeamCoverage(_ request: BalanceCoverageRequest) async throws -> BalanceCoverageResponse {
        let index = coverageRequests.count
        coverageRequests.append(request)
        try await gate(.coverage, index: index)
        return try coverageResponder(request, index).get()
    }
}

/// `BalanceViewModel` のテストの共通の組み立て(マスタは `StubMaster`・構築は `StubBalance`)。
@MainActor
enum BalanceHarness {
    static func makeMaster() -> StubPokeCalcService {
        StubMaster.makeService()
    }

    static func makeViewModel(
        service: StubBalanceService = StubBalanceService(), master: StubPokeCalcService = makeMaster(),
        store: (any TeamStore)? = StubTeamStore(teams: StubBalance.allTeams)
    ) -> BalanceViewModel {
        BalanceViewModel(service: service, master: master, teamStore: store)
    }

    /// `load()` 済みの ViewModel。
    static func makeLoaded(
        service: StubBalanceService = StubBalanceService(), master: StubPokeCalcService = makeMaster(),
        store: (any TeamStore)? = StubTeamStore(teams: StubBalance.allTeams)
    ) async -> BalanceViewModel {
        let viewModel = makeViewModel(service: service, master: master, store: store)
        await viewModel.load()
        return viewModel
    }

    static func defenseDisplay(_ viewModel: BalanceViewModel, file: StaticString = #filePath, line: UInt = #line) -> BalanceDefenseDisplay? {
        if case .loaded(let display) = viewModel.defenseState { return display }
        XCTFail("防御相性が loaded ではない: \(viewModel.defenseState)", file: file, line: line)
        return nil
    }

    static func coverageDisplay(_ viewModel: BalanceViewModel, file: StaticString = #filePath, line: UInt = #line) -> BalanceCoverageDisplay? {
        if case .loaded(let display) = viewModel.coverageState { return display }
        XCTFail("攻撃範囲が loaded ではない: \(viewModel.coverageState)", file: file, line: line)
        return nil
    }
}
