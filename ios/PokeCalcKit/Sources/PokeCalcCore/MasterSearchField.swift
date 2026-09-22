import Foundation

// MasterSearchField: 種族・技の検索欄1つ分の状態機械(issue #68。ADR-0501「issue #68 の受け入れ条件」
// 3章・4章・8章)。`CalcViewModel` / `ReverseViewModel` / `TeamEditViewModel` の6つの検索欄
// (種族×3・技×3)がどれも同じ規則(同期の文字反映・デバウンス・世代保護・上限判定・空クエリで
// 先頭ページへ戻す)を持つため、ここに1か所だけ実装する(「たまたま似ている」のではなく、
// ADR-0501「issue #68」10章が3画面共通の契約として定めているため。issue #113 が共有の基盤に
// 置き換えるまでの素朴な実装)。
//
// 「検索結果に何を出すか」より先の使い方(learnset との交差、一度でも見た値の辞書)は画面ごとに
// 違うので、ここには持たせない(このクラスの責務は「いま検索欄に何を出すか」だけ)。
//
// `public` にしない: この型はテスト対象の公開 API ではなく、各 ViewModel の実装の内側だけで使う
// (テストは ViewModel の公開メンバー経由でこの型の振る舞いを確かめる)。SwiftUI の `@Observable`
// によるビュー再描画の追跡対象にもしない(`ViewModel` 側が結果を自分の `@Observable` プロパティへ
// 明示的に写し取る。10章の実装メモ参照)。

@MainActor
final class MasterSearchField<Item: Sendable> {
    /// 検索欄の文字(前後空白を落とした値)。
    private(set) var query: String = ""
    /// いま出す候補(空クエリなら起動時の先頭ページ、それ以外は直近の検索結果)。
    private(set) var options: [Item] = []
    private(set) var isSearching = false
    /// 直近の候補が `MasterSearch.pageLimit` に達した。
    private(set) var reachedLimit = false

    private let debounce: Duration
    private let search: @Sendable (_ query: String, _ limit: Int) async throws -> [Item]
    private var firstPage: [Item] = []
    private var firstPageReachedLimit = false
    /// 「自分が最新の検索か」を確かめる世代トークン(`CalcViewModel.latestRequestToken` と同じ考え方)。
    private var generation = 0

    init(debounce: Duration, search: @escaping @Sendable (_ query: String, _ limit: Int) async throws -> [Item]) {
        self.debounce = debounce
        self.search = search
    }

    /// `load()` が読んだ起動時の先頭ページを記録し、いま出す候補にもする(起動直後はまだ検索していない)。
    func setFirstPage(_ items: [Item]) {
        firstPage = items
        firstPageReachedLimit = items.count >= MasterSearch.pageLimit
        options = items
        reachedLimit = firstPageReachedLimit
    }

    /// 検索欄の文字を同期的に反映する(4章「判断」: `TextField`/`.searchable` の `Binding` から
    /// 1フレーム遅れずに反映するため、同期の反映と非同期の検索を分ける)。語が変わったら true。
    @discardableResult
    func setQuery(_ text: String) -> Bool {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmed != query else { return false }
        query = trimmed
        return true
    }

    /// 世代トークンを進め、空クエリなら**ただちに**起動時の先頭ページへ戻す(API を呼ばない。4章)。
    /// 非空なら `debounce` だけ待ってから検索を呼び、**自分が最新の世代のときだけ**結果を反映する
    /// (呼ぶ「前」の確認が、打ち直した分の要求そのものを飛ばす。呼んだ「後」の確認が、追い越された
    /// 応答で上書きしない。8章・7章)。
    ///
    /// 反映した候補を返す(呼び出し元が「一度でも見た値」の辞書へ合流させるため)。追い越されて
    /// 反映しなかったときは nil。
    @discardableResult
    func run() async -> [Item]? {
        generation += 1
        let token = generation
        let currentQuery = query
        guard !currentQuery.isEmpty else {
            options = firstPage
            reachedLimit = firstPageReachedLimit
            isSearching = false
            return firstPage
        }
        isSearching = true
        try? await Task.sleep(for: debounce)
        guard token == generation else { return nil }
        do {
            let results = try await search(currentQuery, MasterSearch.pageLimit)
            guard token == generation else { return nil }
            options = results
            reachedLimit = results.count >= MasterSearch.pageLimit
            isSearching = false
            return results
        } catch {
            // 検索の失敗は致命的にしない(黙って前の候補を残し、読み込み中だけ解く。計算そのものの
            // 失敗(`calcBulk`/`species(key:)`)と違って、検索は空欄に戻す・打ち直すだけで復帰できる
            // ため `error` は立てない。ADR-0501「issue #68」実装メモ)。
            guard token == generation else { return nil }
            isSearching = false
            return nil
        }
    }
}
