import PokeCalcCore
import PokeCalcDesign
import SwiftUI

/// ルート画面(P6-1・ADR-0531「ナビゲーション案 B」)。下部タブ(`AppTab`)と、タブ内の区切り・右上のアイコンを、
/// 登録された画面(`FeatureRegistry`。ADR-0507)から組む。画面を足すときはこのファイルを編集しない。
///
/// 入力を消さないための持ち方: 選択(`AppNavigationModel`)と各画面のスタックの path をここで持ち、画面は
/// 一度表示したら破棄しない(タブ内の非選択の区切りは透明・操作不可・支援技術から隠すだけ)。画面ごとに
/// `NavigationStack` を分けるので、画面の中で積んだ状態(構築の編集など)もタブ・区切りを移って戻ると残る。
struct RootView: View {
    private let environment: AppEnvironment
    /// 構築(team)の永続化(P6-2c・ADR-0500 §4)。`RootView` が1回だけ作り、使う画面へ
    /// `FeatureContext` で渡す(`AppEnvironment` は起動時の設定から作るサービスの生成元なので、契約が別の
    /// `TeamStore` をそこに混ぜない)。
    @State private var teamStore: any TeamStore
    /// 選択中のタブ・区切りと、訪れた画面(ADR-0531)。
    @State private var navigation = AppNavigationModel(specs: FeatureRegistry.specs)
    /// 画面(`AppFeature.id`)ごとの `NavigationStack` の path。値ベースにして、起動時の自動遷移
    /// (各 `AppFeature` の `openAtLaunchEnvironmentKey`。テスト・スクリーンショット撮影専用。ADR-0501「P6-2a」)にも使う。
    @State private var paths: [String: NavigationPath] = [:]
    /// 設定エラー時の 1 画面のスタックの path(「このアプリについて」だけを積む)。
    @State private var configurationErrorPath = NavigationPath()

    /// `environment` は既定でも作れる(#Preview 用)。実行時は `PokeCalcApp` が `@State` で
    /// 1回だけ作ったものを渡す(セッション ID を起動ごとに1つに保つため)。
    init(environment: AppEnvironment = .makeAtLaunch()) {
        self.environment = environment
        _teamStore = State(initialValue: Self.makeTeamStore())
    }

    /// `POKECALC_USE_MOCK=1`(XCUITest・スクリーンショット撮影)のときは専用の `UserDefaults` suite を
    /// 使い、起動のたびに空にする(前回の実行で作った構築が残らないようにする。モックは決定的で
    /// あるべきという方針[ADR-0501「P6-1」6章「モックの数値に依存しない」と同じ発想]を `LocalTeamStore`
    /// にも広げた実装判断。ADR-0501「P6-2c」「### 7 確認事項」に追記)。通常起動は `UserDefaults.standard`
    /// にそのまま保存する(構築は端末に残って良いデータ)。
    private static let teamsUITestSuiteName = "PokeCalcTeamsUITest"

    /// 専用 suite を消去してから返す。SwiftUI は `RootView.init` を(たとえば親の再描画のたびに)
    /// 何度も呼び直せるが、`@State(initialValue:)` の**式そのもの**は呼ばれるたびに評価される
    /// (捨てられるのは戻り値だけ)。init の中で直接 `removePersistentDomain` を呼ぶと、
    /// 起動後に作った構築が次の再描画で消えてしまう(`PokeCalcApp.swift` のコメントと同じ理由で
    /// 避けるべき副作用)。`static let` はプロセスで1回しか評価されないので、ここに副作用を
    /// 閉じ込めて「本当に起動1回だけ」を保証する。
    private static let mockTeamStoreDefaults: UserDefaults = {
        let defaults = UserDefaults(suiteName: teamsUITestSuiteName) ?? .standard
        defaults.removePersistentDomain(forName: teamsUITestSuiteName)
        return defaults
    }()

    private static func makeTeamStore() -> any TeamStore {
        guard ProcessInfo.processInfo.environment[AppConfiguration.useMockEnvironmentKey] == "1" else {
            return LocalTeamStore()
        }
        return LocalTeamStore(defaults: mockTeamStoreDefaults)
    }

    var body: some View {
        Group {
            switch environment {
            case .ready(let core, let services):
                tabView(core: core, services: services)
            case .configurationError(let message):
                configurationErrorView(message: message)
            }
        }
        .task {
            let isReady = if case .ready = environment { true } else { false }
            // 従来の else-if と同じく、並び順で最初に指定された1画面だけを開く(設定エラー時は開かない)。
            // タブ内の画面はそのタブ・区切りを選び、それ以外(画面の中から積む画面)は今の画面のスタックへ積む。
            guard
                let id = FeatureCatalog.featureToOpenAtLaunch(
                    FeatureRegistry.specs, environment: ProcessInfo.processInfo.environment, servicesReady: isReady)
            else { return }
            if navigation.tab(of: id) != nil {
                navigation.select(featureID: id)
            } else if let current = navigation.currentFeatureID {
                paths[current, default: NavigationPath()].append(FeatureRoute(featureID: id))
            }
        }
    }

    private var tabSelection: Binding<AppTab> {
        Binding(get: { navigation.selectedTab }, set: { navigation.select(tab: $0) })
    }

    private func path(for featureID: String) -> Binding<NavigationPath> {
        Binding(
            get: { paths[featureID] ?? NavigationPath() },
            set: { paths[featureID] = $0 })
    }

    private func tabView(core: CoreServices, services: FeatureServices) -> some View {
        TabView(selection: tabSelection) {
            ForEach(navigation.tabs, id: \.self) { tab in
                Tab(tab.title, systemImage: PopSymbol.forTab(tab), value: tab) {
                    tabContent(tab, core: core, services: services)
                }
                .accessibilityIdentifier(tab.accessibilityIdentifier)
            }
        }
    }

    /// タブの中身: 訪れた画面をすべて重ねて保持し、選択中の1つだけを見せる。選んでいない画面は `RetainedScreenHost` が
    /// UIKit の `isHidden` で隠す(見えない・押せない。状態は残る)。区切りは選択中の画面にだけ描く(XCUITest は隠れた
    /// 画面の要素も木に残すため、区切りのボタンの識別子を 1 つに保つ。区切りは状態を持たないので描き直してよい)。
    private func tabContent(_ tab: AppTab, core: CoreServices, services: FeatureServices) -> some View {
        ZStack {
            ForEach(navigation.visitedFeatureIDs(in: tab), id: \.self) { featureID in
                let isVisible = navigation.selectedFeatureID(in: tab) == featureID
                RetainedScreenHost(
                    isVisible: isVisible,
                    content: featureStack(featureID, in: tab, showsSegments: isVisible, core: core, services: services))
                    .ignoresSafeArea()
            }
        }
    }

    @ViewBuilder
    private func featureStack(
        _ featureID: String, in tab: AppTab, showsSegments: Bool, core: CoreServices, services: FeatureServices
    ) -> some View {
        if let feature = FeatureRegistry.feature(id: featureID) {
            NavigationStack(path: path(for: featureID)) {
                VStack(spacing: 0) {
                    if showsSegments, Self.segments(in: tab, model: navigation).count > 1 {
                        TabSegmentsView(
                            tab: tab, segments: Self.segments(in: tab, model: navigation),
                            selectedID: navigation.selectedFeatureID(in: tab)
                        ) { navigation.select(featureID: $0) }
                    }
                    feature.destination(
                        in: FeatureContext(
                            core: core, services: services, teamStore: teamStore, path: path(for: featureID),
                            links: Self.links(in: tab)))
                        .frame(maxHeight: .infinity)
                }
                .popScreenBackground()
                .toolbar { toolbarIcons }
                .navigationDestination(for: FeatureRoute.self) { route in
                    destination(for: route, core: core, services: services, path: path(for: featureID))
                }
            }
            .environment(\.imageCatalog, core.images)
        }
    }

    @ViewBuilder
    private func destination(
        for route: FeatureRoute, core: CoreServices, services: FeatureServices, path: Binding<NavigationPath>
    ) -> some View {
        if let feature = FeatureRegistry.feature(id: route.featureID) {
            feature.destination(in: FeatureContext(core: core, services: services, teamStore: teamStore, path: path))
        }
    }

    /// 設定エラー時: タブは出さず、エラーの文言と「このアプリについて」(設定エラー時も開ける画面)だけを出す。
    private func configurationErrorView(message: String) -> some View {
        NavigationStack(path: $configurationErrorPath) {
            Text(message)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.danger.color)
                .multilineTextAlignment(.center)
                .accessibilityIdentifier("backendModeBadge")
                .padding(SpacingToken.x4)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .popScreenBackground()
                .toolbar { toolbarIcons }
                .navigationDestination(for: FeatureRoute.self) { route in
                    if let feature = FeatureRegistry.feature(id: route.featureID), feature.availableWithoutServices {
                        feature.destinationWithoutServices()
                    }
                }
        }
    }

    /// 右上のアイコン(P6-18。issue #328: 非公式の表示とデータの出典への控えめな入口)。各タブの根と設定エラー時に出す。
    @ToolbarContentBuilder
    private var toolbarIcons: some ToolbarContent {
        ToolbarItemGroup(placement: .topBarTrailing) {
            ForEach(Self.toolbarIconEntries) { icon in
                NavigationLink(value: FeatureRoute(featureID: icon.id)) {
                    Image(systemName: icon.systemImage)
                }
                .accessibilityIdentifier(icon.accessibilityIdentifier)
                .accessibilityLabel(icon.accessibilityLabel)
            }
        }
    }
}

/// 画面(とその `NavigationStack`)を 1 つの `UIHostingController` に載せ、選んでいないときは UIKit の `isHidden` で隠す入れ物。
/// 隠しても破棄しないので、入力・積んだ画面が残る(ADR-0531)。
private struct RetainedScreenHost<Content: View>: UIViewControllerRepresentable {
    let isVisible: Bool
    let content: Content

    func makeUIViewController(context: Context) -> UIHostingController<Content> {
        let host = UIHostingController(rootView: content)
        // 背景は中の画面(`popScreenBackground`)が描く。入れ物は透明にする。
        host.view.backgroundColor = .clear
        host.view.isHidden = !isVisible
        return host
    }

    func updateUIViewController(_ host: UIHostingController<Content>, context: Context) {
        host.rootView = content
        host.view.isHidden = !isVisible
    }
}

/// 右上のアイコンの入口(`FeatureEntry.toolbarIcon`)。
private struct ToolbarIconEntry: Identifiable {
    let id: String
    let systemImage: String
    let accessibilityLabel: String
    let accessibilityIdentifier: String
}

extension RootView {
    /// タブ内の区切り(`FeatureEntry.tab` の title・識別子。並びはモデルの `order` 順)。
    fileprivate static func segments(in tab: AppTab, model: AppNavigationModel) -> [TabSegment] {
        model.segments(in: tab).compactMap { id in
            guard let feature = FeatureRegistry.feature(id: id),
                case .tab(_, let title, let identifier) = feature.entry
            else { return nil }
            return TabSegment(id: id, title: title, accessibilityIdentifier: identifier)
        }
    }

    /// タブの根の画面の中に置く行(`FeatureEntry.link(in:)`)。
    fileprivate static func links(in tab: AppTab) -> [FeatureLink] {
        FeatureRegistry.features.compactMap { feature in
            guard case .link(let linkTab, let title, let identifier) = feature.entry, linkTab == tab else { return nil }
            return FeatureLink(id: feature.id, title: title, accessibilityIdentifier: identifier)
        }
    }

    fileprivate static let toolbarIconEntries: [ToolbarIconEntry] = FeatureRegistry.features.compactMap { feature in
        guard case .toolbarIcon(let systemImage, let label, let identifier) = feature.entry else { return nil }
        return ToolbarIconEntry(
            id: feature.id, systemImage: systemImage, accessibilityLabel: label, accessibilityIdentifier: identifier)
    }
}

#Preview {
    RootView(environment: .makeAtLaunch(environment: [AppConfiguration.useMockEnvironmentKey: "1"]))
}
