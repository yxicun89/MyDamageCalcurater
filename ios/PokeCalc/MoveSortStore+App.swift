import Foundation
import PokeCalcCore

extension MoveSortStore {
    /// `POKECALC_USE_MOCK=1`(XCUITest・スクリーンショット)のときは専用の suite を起動ごとに空にする
    /// (前のテストで選んだ並びが次のテストに残らないように。`RootView` の構築の保存先と同じ流儀)。
    /// 通常起動は `UserDefaults.standard`(並びは端末に残ってよい)。
    private static let uiTestSuiteName = "PokeCalcMoveSortUITest"

    /// `static let` はプロセスで1回しか評価されないので、消去は起動1回だけ。
    private static let mockDefaults: UserDefaults = {
        let defaults = UserDefaults(suiteName: uiTestSuiteName) ?? .standard
        defaults.removePersistentDomain(forName: uiTestSuiteName)
        return defaults
    }()

    static func forApp() -> MoveSortStore {
        guard ProcessInfo.processInfo.environment[AppConfiguration.useMockEnvironmentKey] == "1" else {
            return MoveSortStore()
        }
        return MoveSortStore(defaults: mockDefaults)
    }
}
