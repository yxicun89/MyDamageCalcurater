// BalanceViewModel: タイプバランス画面の状態と操作(P6-26。ADR-0505)。
//
// 方針(ADR-0505 §4):
//  - 入力は**構築(`TeamStore`)から選ぶ**(最大 6 体)。メンバー・特性・技の手入力は無い。選んだ構築から analyze・coverage の要求を作り、2 本を**独立に**呼ぶ
//    (片方の失敗・遅延がもう片方の表示を消さない。Web の ADR-0303 §9 と同じ)。
//  - 構築を選んだ時点で送る(1 回の選択 = 1 組の呼び出し。入力のたびに送る入力欄が無いので debounce は無い)。送信のたびに世代を進めて先行を cancel し、
//    最新の世代の応答だけ反映する(cancel を無視するサービスの古い応答も捨てる)。`CancellationError` は失敗にしない。
//  - 相性の計算・分類の判定し直しをしない(応答を `BalanceResultDisplayBuilder` で整えるだけ)。タイプ相性表を持たない。
//  - balance の失敗・構築の読み込みの失敗・マスタ(名前の引き当て)の失敗は互いに、また計算・構築に影響しない(絶対ルール 5)。`load()` は throw しない。
//
// 公開 API の形(名前・型・引数)は Balance*Tests 群が固定する。

import Foundation
import Observation

/// 表示する失敗。`message` は `BalanceLabels.errorMessage(forCode:)`(サーバーの英語 message は出さない)。
public struct BalanceFailure: Equatable, Sendable {
    public let code: String

    public var message: String { BalanceLabels.errorMessage(forCode: code) }

    public init(code: String) {
        self.code = code
    }
}

/// 1 つの解析(防御相性・攻撃範囲)の状態。2 つは互いに独立。
public enum BalanceSectionState<Value: Equatable & Sendable>: Equatable, Sendable {
    /// まだ何も選んでいない(または判定中に画面を離れて中断した)。
    case idle
    case loading
    case loaded(Value)
    case failed(BalanceFailure)
    /// 呼ばなかった。防御相性: 構築のメンバーが 0 体。攻撃範囲: 構築のメンバーが 0 体、または技を持つメンバーが 1 体もいない。
    case skipped
}

/// 構築の選択肢(メンバー 0 体の構築も出す。選ぶと「ポケモンがいません」の案内になり、API は呼ばない)。
public struct BalanceTeamOption: Identifiable, Equatable, Sendable {
    public let id: String
    public let name: String
    public let memberCount: Int

    public init(id: String, name: String, memberCount: Int) {
        self.id = id
        self.name = name
        self.memberCount = memberCount
    }
}

/// 構築から作る要求の組。`analyze` は メンバーが 1 体以上のとき、`coverage` は技を持つメンバーが 1 体以上いるときだけ非 nil(nil なら呼ばない)。
public struct BalanceRequests: Equatable, Sendable {
    public var analyze: BalanceAnalyzeRequest?
    public var coverage: BalanceCoverageRequest?

    public init(analyze: BalanceAnalyzeRequest?, coverage: BalanceCoverageRequest?) {
        self.analyze = analyze
        self.coverage = coverage
    }
}

/// 構築 → 要求の純粋関数(ADR-0505 §5)。
public enum BalanceRequestBuilder {
    /// 規則:
    ///  - メンバーは構築の順で先頭から `RequestLimits.maxBalanceMembers` 体まで(超えた分は送らない)。
    ///  - analyze: `pokemonId = speciesKey`・`abilityId` は nil か空文字なら nil(欄ごと送らない)。
    ///  - coverage: `moveIds` は構築の順のまま重複を除き(契約の uniqueItems)、先頭から `RequestLimits.maxBalanceMovesPerMember` 件まで。空配列のメンバーも載せる。
    ///    送るのは技を持つメンバーが 1 体以上いるときだけ(全員が空なら nil)。
    ///  - メンバーが 0 体なら両方 nil。
    public static func make(from team: Team) -> BalanceRequests {
        let members = Array(team.members.prefix(RequestLimits.maxBalanceMembers))
        guard !members.isEmpty else { return BalanceRequests(analyze: nil, coverage: nil) }
        let analyze = BalanceAnalyzeRequest(
            members: members.map { member in
                let ability = member.abilityId.flatMap { $0.isEmpty ? nil : $0 }
                return BalanceAnalyzeMember(pokemonId: member.speciesKey, abilityId: ability)
            })
        let coverageMembers = members.map { member in
            BalanceCoverageMember(pokemonId: member.speciesKey, moveIds: uniqueMoveIds(member.moveIds))
        }
        let hasAnyMove = coverageMembers.contains { !$0.moveIds.isEmpty }
        return BalanceRequests(analyze: analyze, coverage: hasAnyMove ? BalanceCoverageRequest(members: coverageMembers) : nil)
    }

    /// 構築の順のまま重複を除き、先頭から `maxBalanceMovesPerMember` 件まで。
    private static func uniqueMoveIds(_ moveIds: [String]) -> [String] {
        var seen = Set<String>()
        let unique = moveIds.filter { seen.insert($0).inserted }
        return Array(unique.prefix(RequestLimits.maxBalanceMovesPerMember))
    }
}

@MainActor
@Observable
public final class BalanceViewModel {
    // MARK: - 構築(入力)

    public private(set) var teamOptions: [BalanceTeamOption] = []
    /// 構築の一覧を読めなかった(`TeamStore.list()` の失敗)。balance・計算には影響しない。
    public private(set) var teamLoadFailed = false
    public private(set) var selectedTeamID: String?

    // MARK: - 結果(2 つは独立)

    public private(set) var defenseState: BalanceSectionState<BalanceDefenseDisplay> = .idle
    public private(set) var coverageState: BalanceSectionState<BalanceCoverageDisplay> = .idle

    private let service: any BalanceService
    private let master: any PokeCalcService
    private let teamStore: (any TeamStore)?

    private var teams: [Team] = []
    /// 解析の世代。選択・再解析・画面を離れるたびに進め、古い応答・古い失敗を捨てる(cancel を無視するサービスの対策)。
    private var generation = 0
    private var analyzeTask: Task<Void, Never>?
    private var coverageTask: Task<Void, Never>?
    private var labelsTask: Task<[BalanceMemberLabel], Never>?

    /// `master` は名前の引き当て(ニックネーム → 種族名 → speciesKey。特性名)だけに使う。失敗しても解析は止まらない(名前が ID に落ちるだけ)。
    /// `teamStore` が nil なら構築は常に空。
    public init(service: any BalanceService, master: any PokeCalcService, teamStore: (any TeamStore)?) {
        self.service = service
        self.master = master
        self.teamStore = teamStore
    }

    /// 構築の一覧を読む(throw しない)。失敗は `teamLoadFailed`。
    public func load() async {
        guard let teamStore else {
            teams = []
            teamOptions = []
            teamLoadFailed = false
            return
        }
        do {
            teams = try await teamStore.list()
            teamLoadFailed = false
        } catch {
            teams = []
            teamLoadFailed = true
        }
        teamOptions = teams.map { BalanceTeamOption(id: $0.id, name: $0.name, memberCount: $0.members.count) }
    }

    /// 構築を選び、analyze・coverage を独立に呼ぶ(同期で状態を `.loading`/`.skipped` にし、世代を進めて先行を cancel して予約する)。
    /// 一覧に無い ID は何もしない。
    public func selectTeam(id: String) {
        guard let team = teams.first(where: { $0.id == id }) else { return }
        selectedTeamID = id
        supersedePendingWork()
        let generation = self.generation
        let requests = BalanceRequestBuilder.make(from: team)

        defenseState = requests.analyze == nil ? .skipped : .loading
        coverageState = requests.coverage == nil ? .skipped : .loading

        let members = Array(team.members.prefix(RequestLimits.maxBalanceMembers))
        let master = self.master
        let labels = Task { await Self.resolveLabels(members: members, master: master) }
        labelsTask = labels

        if let request = requests.analyze {
            analyzeTask = Task { [weak self] in
                await self?.runAnalyze(request, labels: labels, generation: generation)
            }
        }
        if let request = requests.coverage {
            coverageTask = Task { [weak self] in
                await self?.runCoverage(request, labels: labels, generation: generation)
            }
        }
    }

    /// 構築の一覧を読み直し、選択中の構築を(最新の内容で)もう一度解析する。選択中の構築が消えていたら選択を外して `.idle` に戻す。
    public func reanalyze() async {
        await load()
        guard let selectedTeamID else { return }
        if teams.contains(where: { $0.id == selectedTeamID }) {
            selectTeam(id: selectedTeamID)
        } else {
            self.selectedTeamID = nil
            supersedePendingWork()
            defenseState = .idle
            coverageState = .idle
        }
    }

    /// 最新の予約済みの解析(2 本とも)が終わるまで待つ(テスト用)。
    public func settle() async {
        let tasks = [analyzeTask, coverageTask]
        for task in tasks { await task?.value }
    }

    /// 画面を離れるとき。進行中の解析を cancel し、`.loading` のものは `.idle` に戻す(結果・失敗はそのまま)。
    public func cancelPendingWork() {
        supersedePendingWork()
        if case .loading = defenseState { defenseState = .idle }
        if case .loading = coverageState { coverageState = .idle }
    }

    // MARK: - 内部

    /// 世代を進めて先行を cancel する。
    private func supersedePendingWork() {
        generation += 1
        analyzeTask?.cancel()
        coverageTask?.cancel()
        labelsTask?.cancel()
        analyzeTask = nil
        coverageTask = nil
        labelsTask = nil
    }

    private func runAnalyze(_ request: BalanceAnalyzeRequest, labels: Task<[BalanceMemberLabel], Never>, generation: Int) async {
        do {
            let response = try await service.analyzeTeamBalance(request)
            let names = await labels.value
            guard generation == self.generation else { return }
            defenseState = .loaded(BalanceResultDisplayBuilder.defense(response, labels: names))
        } catch is CancellationError {
            if generation == self.generation, case .loading = defenseState { defenseState = .idle }
        } catch {
            guard generation == self.generation else { return }
            defenseState = .failed(Self.failure(error))
        }
    }

    private func runCoverage(_ request: BalanceCoverageRequest, labels: Task<[BalanceMemberLabel], Never>, generation: Int) async {
        do {
            let response = try await service.analyzeTeamCoverage(request)
            let names = await labels.value
            guard generation == self.generation else { return }
            coverageState = .loaded(BalanceResultDisplayBuilder.coverage(response, labels: names))
        } catch is CancellationError {
            if generation == self.generation, case .loading = coverageState { coverageState = .idle }
        } catch {
            guard generation == self.generation else { return }
            coverageState = .failed(Self.failure(error))
        }
    }

    private static func failure(_ error: any Error) -> BalanceFailure {
        BalanceFailure(code: (error as? PokeCalcError)?.code ?? PokeCalcError.Code.transport)
    }

    /// 名前の引き当て(ニックネーム → 種族名 → speciesKey。特性名は種族の特性候補から。見つからなければ ID)。失敗しても握りつぶして ID に落とす。
    private nonisolated static func resolveLabels(members: [TeamMember], master: any PokeCalcService) async -> [BalanceMemberLabel] {
        var speciesByKey: [String: SpeciesDetail?] = [:]
        var labels: [BalanceMemberLabel] = []
        for member in members {
            if speciesByKey[member.speciesKey] == nil {
                speciesByKey[member.speciesKey] = .some(try? await master.species(key: member.speciesKey))
            }
            let species = speciesByKey[member.speciesKey] ?? nil
            let nickname = member.nickname?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            let name = nickname.isEmpty ? (species?.nameJa ?? member.speciesKey) : nickname
            var abilityName: String?
            if let abilityId = member.abilityId, !abilityId.isEmpty {
                abilityName = species?.abilities.first { $0.id == abilityId }?.nameJa ?? abilityId
            }
            labels.append(BalanceMemberLabel(name: name, abilityName: abilityName))
        }
        return labels
    }
}
