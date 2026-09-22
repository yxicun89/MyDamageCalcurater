import Observation

// TeamListViewModel: 構築一覧画面の状態(P6-2c・ADR-0500 §1「画面のロジックは ViewModel で
// XCTest に固定し、View は描くだけ」)。

/// 構築一覧画面の状態。`TeamStore` だけに依存する(ADR-0500 §4)。
///
/// **判断(`@MainActor` を付けない)**: `CalcViewModel`(P6-2a)は `@MainActor` だが、
/// `TeamListViewModelTests` / `TeamEditViewModelTests` のテストクラスは `@MainActor` を付けずに
/// 書かれていて、`viewModel.error` 等のプロパティを `await` なしで直接参照する(XCTAssert の
/// 引数は暗黙の autoclosure で、呼び出し元が `@MainActor` でないと MainActor 隔離プロパティを
/// `await` なしで読めない。実測: `@MainActor` を付けると `swift build --build-tests` がテスト
/// ファイル側のこの参照でコンパイルエラーになる)。テストを変更せずに通すため(CLAUDE.md 絶対
/// ルール6)、構築の2つの ViewModel は `@MainActor` を付けない。呼び出し元(View)は既存の
/// `@State` プロパティ経由でメインスレッドから使う限り実害は無い(ADR-0501「P6-2c」実装メモに追記)。
@Observable
public final class TeamListViewModel {
    private let store: any TeamStore

    public private(set) var teams: [Team] = []
    public private(set) var isLoading = false
    public private(set) var error: TeamScreenError?

    public init(store: any TeamStore) {
        self.store = store
    }

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
        isLoading = false
    }

    /// 前後空白を落として空なら保存せず `teamNameEmpty` の `error` を立てて nil を返す。
    /// 空でなければ新しい `Team` を保存し、`load()` して最新化してから返す(失敗したら nil)。
    public func createTeam(name: String) async -> Team? {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedName.isEmpty else {
            error = .service(code: PokeCalcError.Code.teamNameEmpty, message: "チーム名を入力してください")
            return nil
        }
        let team = Team(name: trimmedName)
        do {
            try await store.save(team)
        } catch {
            self.error = TeamScreenError(error)
            return nil
        }
        await load()
        return team
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
