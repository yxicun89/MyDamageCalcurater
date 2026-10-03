import PokeCalcCore
import SwiftUI

/// ルート画面から開く1つの画面(機能)の登録情報(ADR-0507 §1)。
///
/// 画面を足すときは、このプロトコルの実装を `Features/` に1ファイル書き、`FeatureRegistry` の配列に1行足す。
/// `RootView`・`AppEnvironment` は編集しない。
protocol AppFeature: Sendable {
    /// 画面の識別子。`ios/scripts/sim-run.sh` の `IOS_SCREEN` の画面名と同じ値にする。
    var id: String { get }
    /// 入口の並び順(小さい順)。
    var order: Int { get }
    /// 入口の形(ルートのピル/右上のアイコン)。
    var entry: FeatureEntry { get }
    /// 起動時にこの画面を開く環境変数(値が `FeatureRegistry.openAtLaunchValue` のとき開く。
    /// XCUITest を介さずスクリーンショットを撮る用途)。無ければ `nil`。
    var openAtLaunchEnvironmentKey: String? { get }

    /// この画面だけが使うサービスを登録する(モック/API)。複数の画面が共有するものは `CoreServices`。
    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws

    /// 遷移先の画面(サービスがそろっているとき)。
    @MainActor func destination(in context: FeatureContext) -> AnyView

    /// 設定エラーでサービスが無いときの遷移先。`nil` なら何も出さない(設定エラー時も入口が出る右上のアイコンの画面だけが使う)。
    @MainActor func destinationWithoutServices() -> AnyView?
}

extension AppFeature {
    var openAtLaunchEnvironmentKey: String? { nil }

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {}

    @MainActor func destinationWithoutServices() -> AnyView? { nil }
}

/// 入口の形。
enum FeatureEntry: Sendable {
    /// ルート画面のピルのボタン(サービスがそろっているときだけ出す)。
    case rootButton(title: String, accessibilityIdentifier: String)
    /// ナビゲーションバー右上のアイコン(設定エラー時も出す)。
    case toolbarIcon(systemImage: String, accessibilityLabel: String, accessibilityIdentifier: String)
}

/// 遷移先の画面を作るときに渡す、起動時の状態とルート画面が持つ共有の状態。
struct FeatureContext {
    let core: CoreServices
    let services: FeatureServices
    /// 構築の永続化(`RootView` が1回だけ作る。ADR-0500 §4)。
    let teamStore: any TeamStore
    /// ルートの `NavigationStack` の path(構築一覧のように、画面の中から先へ積む画面が使う)。
    let path: Binding<NavigationPath>
}

/// `NavigationPath` に積む画面の行き先(値だけで、状態は持たない)。
struct FeatureRoute: Hashable {
    let featureID: String
}
