import Foundation

// LatestTaskRunner: 「最新の1つの入力 Task」を保持する小さな状態機械(issue #113。ADR-0501
// 「issue #113 の受け入れ条件(iOS 側)」8章)。`ReverseViewModel` / `CalcViewModel` の入力操作の
// Task 管理で使う内部部品(`MasterSearchField` と同じ位置づけ。テスト対象の公開 API ではなく、
// テストは ViewModel の公開メンバー経由でこの型の振る舞いを確かめる)。

/// 保持している Task は常に1つ。`schedule` を呼ぶたびに先行 Task を cancel してから新しい Task を持つ。
@MainActor
final class LatestTaskRunner {
    private var currentTask: Task<Void, Never>?

    /// 保持中の Task を cancel する(画面破棄の `.onDisappear` から呼ぶ想定)。
    func cancel() {
        currentTask?.cancel()
        currentTask = nil
    }

    /// 先行 Task を cancel し、`debounce` だけ待ってから `operation` を呼ぶ(`debounce` が `.zero` なら
    /// 待たずにただちに呼ぶ)。呼ぶ「前」に `Task.isCancelled` を確認するので、debounce の待機中に
    /// 次の呼び出しで cancel された分は `operation` を呼ばずに終わる(=要求を出さない)。
    @discardableResult
    func schedule(debounce: Duration, operation: @escaping @MainActor @Sendable () async -> Void) -> Task<Void, Never> {
        currentTask?.cancel()
        let task = Task { @MainActor in
            if debounce > .zero {
                try? await Task.sleep(for: debounce)
            }
            guard !Task.isCancelled else { return }
            await operation()
        }
        currentTask = task
        return task
    }
}
