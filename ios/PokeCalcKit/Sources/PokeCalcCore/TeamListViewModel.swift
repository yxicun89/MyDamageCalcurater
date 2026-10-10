import Foundation
import Observation

// TeamListViewModel: 構築一覧画面の状態(P6-2c・ADR-0500 §1「画面のロジックは ViewModel で
// XCTest に固定し、View は描くだけ」)。

/// 構築一覧画面の状態。`TeamStore` だけに依存する(ADR-0500 §4)。ADR-0501「P6-2c」3章の指定どおり
/// `@MainActor`(`CalcViewModel`/`ReverseViewModel` と同じ)。テストクラス(`TeamListViewModelTests`)も
/// `CalcViewModelTests`/`ReverseViewModelTests` と同じく `@MainActor` を付けて呼び出し側もそろえる。
@MainActor
@Observable
public final class TeamListViewModel {
    private let store: any TeamStore
    private let service: (any PokeCalcService)?
    private let now: @Sendable () -> Date
    /// 一覧のアイコン用に一度見た種族(タイプ色のエンブレムの色に使う)。引けなければ画像・色は出さない。
    private var speciesByKey: [String: SpeciesSummary] = [:]

    public private(set) var teams: [Team] = []
    /// 構築 id → 表示名(`TeamNaming`。既定名なら「構築 N」)。
    public private(set) var displayNames: [String: String] = [:]
    /// 削除の確認中の構築 id(2 段階の 1 段階目。nil なら確認していない)。
    public private(set) var pendingDeleteID: String?
    public private(set) var isLoading = false
    public private(set) var error: TeamScreenError?

    public init(store: any TeamStore, service: (any PokeCalcService)? = nil, now: @escaping @Sendable () -> Date = { Date() }) {
        self.store = store
        self.service = service
        self.now = now
    }

    public func displayName(for id: String) -> String {
        displayNames[id] ?? TeamNaming.untitled(1)
    }

    public func speciesSummary(forKey key: String) -> SpeciesSummary? { speciesByKey[key] }

    /// `store.list()` を呼んで `teams` を最新化する。`CalcViewModel.load()` と違い、
    /// 何度呼び直してもよい(一覧画面は編集・削除の画面から戻るたびに最新化が要るため。
    /// ADR-0501「P6-2c」3章)。
    public func load() async {
        isLoading = true
        do {
            teams = try await store.list()
            error = nil
        } catch {
            teams = []
            self.error = TeamScreenError(error)
        }
        displayNames = TeamNaming.displayNames(for: teams)
        isLoading = false
        await loadSpeciesForIcons()
    }

    /// アイコン用の種族を一度だけ引く(先頭ページ。失敗しても一覧は読めた状態のまま・`error` は立てない)。
    private func loadSpeciesForIcons() async {
        guard speciesByKey.isEmpty, let service else { return }
        guard let page = try? await service.searchSpecies(query: "", limit: MasterSearch.pageLimit) else { return }
        for item in page { speciesByKey[item.key] = item }
    }

    /// 名前なしで新しい構築を作る(`name` は既定で `TeamNaming.defaultName`)。前後空白を落として空なら保存せず
    /// `teamNameEmpty` の `error` を立てて nil を返す。保存して `load()` で最新化してから返す(失敗したら nil)。
    public func createTeam(name: String = TeamNaming.defaultName) async -> Team? {
        await createTeam(name: name, members: [])
    }

    /// 取り込んだメンバーで新しい構築を作る(既定名)。7 体以上は保存せず nil(`error`)。
    public func createTeam(members: [TeamMember]) async -> Team? {
        await createTeam(name: TeamNaming.defaultName, members: members)
    }

    private func createTeam(name: String, members: [TeamMember]) async -> Team? {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedName.isEmpty else {
            error = .service(code: PokeCalcError.Code.teamNameEmpty, message: "チーム名を入力してください")
            return nil
        }
        let team = Team(name: trimmedName, members: members, updatedAt: now())
        do {
            try await store.save(team)
        } catch {
            self.error = TeamScreenError(error)
            return nil
        }
        await load()
        return team
    }

    // MARK: - 削除(2 段階)

    public func requestDelete(id: String) { pendingDeleteID = id }

    public func cancelDelete() { pendingDeleteID = nil }

    /// 確認中の構築を削除する(確認していなければ何もしない)。
    public func confirmDelete() async {
        guard let id = pendingDeleteID else { return }
        pendingDeleteID = nil
        await deleteTeam(id: id)
    }

    /// `store.delete(id:)` を呼び、成功でも失敗でも `load()` して最新化する。delete 自体が
    /// 失敗したときは、その後の `load()` が成功しても失敗を伝える(黙って消える)。
    public func deleteTeam(id: String) async {
        var deleteFailure: (any Error)?
        do {
            try await store.delete(id: id)
        } catch {
            deleteFailure = error
        }
        await load()
        if let deleteFailure {
            self.error = TeamScreenError(deleteFailure)
        }
    }
}
