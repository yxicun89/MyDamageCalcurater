import Foundation

@testable import PokeCalcCore

/// `DeviceDataDeletionViewModel` のテスト用 `DeviceDataService`。対象ごとに応答の台本を持ち、
/// 台本が尽きたら `.completed`(冪等な本物と同じ)。呼び出しを順に記録する。
actor StubDeviceDataService: DeviceDataService {
    enum Step: Sendable {
        case progress(DeletionProgress)
        case failure(PokeCalcError)
        /// キャンセルされるまで待つ(キャンセルで `CancellationError`)。
        case hang
    }

    private(set) var calls: [DeviceDataTarget] = []
    private var scripts: [DeviceDataTarget: [Step]]
    private var hook: (@Sendable (DeviceDataTarget, Int) async -> Void)?

    init(record: [Step] = [], team: [Step] = []) {
        scripts = [.record: record, .team: team]
    }

    /// 要求を処理する直前に呼ぶ(対象, その対象の何回目か〈1始まり〉)。途中の画面状態の観測用。
    func setHook(_ hook: @escaping @Sendable (DeviceDataTarget, Int) async -> Void) {
        self.hook = hook
    }

    func callCount(_ target: DeviceDataTarget) -> Int {
        calls.filter { $0 == target }.count
    }

    func deleteRecordDeviceData() async throws -> DeletionProgress {
        try await handle(.record)
    }

    func deleteTeamDeviceData() async throws -> DeletionProgress {
        try await handle(.team)
    }

    private func handle(_ target: DeviceDataTarget) async throws -> DeletionProgress {
        calls.append(target)
        let nth = callCount(target)
        if let hook { await hook(target, nth) }
        let step = nextStep(target)
        switch step {
        case .progress(let progress):
            return progress
        case .failure(let error):
            throw error
        case .hang:
            try await Task.sleep(for: .seconds(60))
            return .completed
        }
    }

    private func nextStep(_ target: DeviceDataTarget) -> Step {
        var script = scripts[target] ?? []
        guard !script.isEmpty else { return .progress(.completed) }
        let first = script.removeFirst()
        scripts[target] = script
        return first
    }
}
