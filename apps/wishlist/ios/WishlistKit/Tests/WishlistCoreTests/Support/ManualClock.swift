import Foundation
import Synchronization
import XCTest

@testable import WishlistCore

/// 偽の `sleep`。要求された時間を記録し、テストが `advance()` するまで待たせる(キャンセルされたら `CancellationError`)。
/// 実時間で待たないので、5 秒ごとのポーリングを即座に進められる。
final class ManualSleeper: Sendable {
    private struct Waiter {
        let id: Int
        let continuation: CheckedContinuation<Void, any Error>
    }
    private struct State {
        var durations: [Duration] = []
        var waiters: [Waiter] = []
        var nextID = 0
        var cancelledBeforeWait: Set<Int> = []
    }

    private let state = Mutex(State())

    var sleep: ItemDetailViewModel.Sleep {
        { [self] duration in try await wait(duration) }
    }

    /// これまでに要求された時間(順)
    var requested: [Duration] { state.withLock { $0.durations } }
    /// いま待っている数
    var pendingCount: Int { state.withLock { $0.waiters.count } }

    private func wait(_ duration: Duration) async throws {
        let id = state.withLock { s -> Int in
            s.nextID += 1
            s.durations.append(duration)
            return s.nextID
        }
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, any Error>) in
                let cancelled = state.withLock { s -> Bool in
                    if s.cancelledBeforeWait.contains(id) { return true }
                    s.waiters.append(Waiter(id: id, continuation: continuation))
                    return false
                }
                if cancelled { continuation.resume(throwing: CancellationError()) }
            }
        } onCancel: {
            let continuation = state.withLock { s -> CheckedContinuation<Void, any Error>? in
                if let index = s.waiters.firstIndex(where: { $0.id == id }) { return s.waiters.remove(at: index).continuation }
                s.cancelledBeforeWait.insert(id)
                return nil
            }
            continuation?.resume(throwing: CancellationError())
        }
    }

    /// 待っている最も古い sleep を 1 つ終わらせる。待っているものが無ければ false。
    @discardableResult
    func advance() -> Bool {
        let waiter = state.withLock { s in s.waiters.isEmpty ? nil : s.waiters.removeFirst() }
        waiter?.continuation.resume()
        return waiter != nil
    }

    /// `pendingCount >= count` になるまで待つ(実時間で最大 5 秒。超えたら失敗)。
    func waitUntilPending(_ count: Int = 1, file: StaticString = #filePath, line: UInt = #line) async {
        let deadline = ContinuousClock.now + .seconds(5)
        while pendingCount < count {
            if ContinuousClock.now > deadline {
                XCTFail("sleep が \(count) 件待ちにならない(いま \(pendingCount) 件)", file: file, line: line)
                return
            }
            try? await Task.sleep(for: .milliseconds(1))
        }
    }
}

/// 応答の保留用。`pass()` は `open()` されるまで待つ。`waitUntilArrived` で「保留に入った」ことを待てる。
final class Gate: Sendable {
    private struct State {
        var isOpen = false
        var arrived = 0
        var waiters: [CheckedContinuation<Void, Never>] = []
    }
    private let state = Mutex(State())

    func pass() async {
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            let resumeNow = state.withLock { s -> Bool in
                s.arrived += 1
                if s.isOpen { return true }
                s.waiters.append(continuation)
                return false
            }
            if resumeNow { continuation.resume() }
        }
    }

    func open() {
        let waiters = state.withLock { s -> [CheckedContinuation<Void, Never>] in
            s.isOpen = true
            defer { s.waiters = [] }
            return s.waiters
        }
        waiters.forEach { $0.resume() }
    }

    var arrived: Int { state.withLock { $0.arrived } }

    func waitUntilArrived(_ count: Int = 1, file: StaticString = #filePath, line: UInt = #line) async {
        let deadline = ContinuousClock.now + .seconds(5)
        while arrived < count {
            if ContinuousClock.now > deadline {
                XCTFail("保留に \(count) 件入らない(いま \(arrived) 件)", file: file, line: line)
                return
            }
            try? await Task.sleep(for: .milliseconds(1))
        }
    }
}

/// ISO 8601 の文字列 → Date(テストの時刻を読みやすく書く)
func iso(_ text: String) -> Date {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime]
    guard let date = formatter.date(from: text) else { preconditionFailure("bad date: \(text)") }
    return date
}
