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
    /// 起動時にこの画面を開く環境変数(値が `FeatureCatalog.openAtLaunchValue` のとき開く。
    /// XCUITest を介さずスクリーンショットを撮る用途)。無ければ `nil`。
    var openAtLaunchEnvironmentKey: String? { get }
    /// `destination(in:)` が `FeatureServices` から引くサービス。起動時にすべて登録されていることを検証し、
    /// 足りなければ設定エラーにする(登録漏れ・登録と取り出しの型違いで空白の画面を出さない)。
    var requiredServices: [ServiceKey] { get }
    /// 設定エラーでサービスが無いときも開けるか(`destinationWithoutServices()` を実装した画面だけ `true`)。
    var availableWithoutServices: Bool { get }

    /// この画面だけが使うサービスを登録する(モック/API)。複数の画面が共有するものは `CoreServices`。
    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws

    /// 遷移先の画面(サービスがそろっているとき)。サービスが引けないときは `unavailableView()` を返す。
    @MainActor func destination(in context: FeatureContext) -> AnyView

    /// 設定エラーでサービスが無いときの遷移先。**入口が `.toolbarIcon` の画面では必須**(設定エラー時も入口が出る
    /// ため)。実装したら `availableWithoutServices` を `true` にする(`.toolbarIcon` で `false` なら起動時に設定エラー)。
    @MainActor func destinationWithoutServices() -> AnyView
}

extension AppFeature {
    var openAtLaunchEnvironmentKey: String? { nil }
    var requiredServices: [ServiceKey] { [] }
    var availableWithoutServices: Bool { false }

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {}

    @MainActor func destinationWithoutServices() -> AnyView { unavailableView() }

    /// サービスが引けず画面を出せないときの表示(空白にしない)。
    @MainActor func unavailableView() -> AnyView {
        AnyView(
            ContentUnavailableView(
                FeatureRegistryText.unavailableTitle, systemImage: "exclamationmark.triangle",
                description: Text(FeatureRegistryText.unavailableDescription)
            )
            .accessibilityIdentifier("featureUnavailable"))
    }

    /// 検証・起動時の選択に使う、View に依存しない登録情報。
    var spec: FeatureSpec {
        let isToolbarEntry = if case .toolbarIcon = entry { true } else { false }
        return FeatureSpec(
            id: id, order: order, openAtLaunchEnvironmentKey: openAtLaunchEnvironmentKey,
            requiredServices: requiredServices, isToolbarEntry: isToolbarEntry,
            availableWithoutServices: availableWithoutServices)
    }
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
