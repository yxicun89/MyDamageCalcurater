import PokeCalcCore
import PokeCalcDesign
import SwiftUI

/// ルート画面(P6-1)。モックで動いていることの表示と、登録された画面(`FeatureRegistry`。ADR-0507)への
/// 入口だけを持つ。画面を足すときはこのファイルを編集しない。
struct RootView: View {
    private let environment: AppEnvironment
    /// 構築(team)の永続化(P6-2c・ADR-0500 §4)。`RootView` が1回だけ作り、使う画面へ
    /// `FeatureContext` で渡す(`AppEnvironment` は起動時の設定から作るサービスの生成元なので、契約が別の
    /// `TeamStore` をそこに混ぜない)。
    @State private var teamStore: any TeamStore
    /// 画面への遷移を値ベースにし、起動時に自動で遷移させられるようにする(各 `AppFeature` の
    /// `openAtLaunchEnvironmentKey` 用。テスト・スクリーンショット撮影専用。ADR-0501「P6-2a」参照)。
    @State private var path = NavigationPath()

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
        NavigationStack(path: $path) {
            VStack(spacing: SpacingToken.x6) {
                statusBadge
                Spacer()
                if case .ready = environment {
                    VStack(spacing: SpacingToken.x4) {
                        ForEach(Self.rootButtons) { button in
                            NavigationLink(value: FeatureRoute(featureID: button.id)) {
                                PopLabel(title: button.title, systemImage: PopSymbol.forFeature(id: button.id))
                            }
                            .buttonStyle(PillButtonStyle(kind: .primary))
                            .accessibilityIdentifier(button.accessibilityIdentifier)
                        }
                    }
                }
                Spacer()
            }
            .padding(SpacingToken.x4)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .popScreenBackground()
            .toolbar {
                // `navigationTitle` はシステムフォントに固定されるため、design.md の
                // SF Pro Rounded を出すために principal 位置のカスタム View で置き換える。
                ToolbarItem(placement: .principal) {
                    Text("PokeCalc")
                        .font(TextStyleToken.title.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                }
                // P6-18(issue #328): 非公式の表示とデータの出典への控えめな入口。
                // `.principal` は上記の見出しで埋まっているため `.topBarTrailing` に置く。設定エラー時も出す。
                ToolbarItemGroup(placement: .topBarTrailing) {
                    ForEach(Self.toolbarIcons) { icon in
                        NavigationLink(value: FeatureRoute(featureID: icon.id)) {
                            Image(systemName: icon.systemImage)
                        }
                        .accessibilityIdentifier(icon.accessibilityIdentifier)
                        .accessibilityLabel(icon.accessibilityLabel)
                    }
                }
            }
            .navigationDestination(for: FeatureRoute.self) { route in
                destination(for: route)
            }
        }
        .task {
            let isReady = if case .ready = environment { true } else { false }
            // 従来の else-if と同じく、並び順で最初に指定された1画面だけを開く(設定エラー時は開かない)。
            if let id = FeatureCatalog.featureToOpenAtLaunch(
                FeatureRegistry.specs, environment: ProcessInfo.processInfo.environment, servicesReady: isReady)
            {
                path.append(FeatureRoute(featureID: id))
            }
        }
    }

    @ViewBuilder
    private func destination(for route: FeatureRoute) -> some View {
        if let feature = FeatureRegistry.feature(id: route.featureID) {
            switch environment {
            case .ready(let core, let services):
                feature.destination(
                    in: FeatureContext(core: core, services: services, teamStore: teamStore, path: $path)
                )
                .environment(\.imageCatalog, core.images)
            case .configurationError:
                if feature.availableWithoutServices {
                    feature.destinationWithoutServices()
                }
            }
        }
    }

    @ViewBuilder
    private var statusBadge: some View {
        switch environment {
        case .ready(let core, _):
            Text(core.backendDescription)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.tableZebra.color, in: Capsule())
                .accessibilityIdentifier("backendModeBadge")
        case .configurationError(let message):
            Text(message)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.danger.color)
                .multilineTextAlignment(.center)
                .accessibilityIdentifier("backendModeBadge")
        }
    }
}

/// ルートのピルの入口(`FeatureEntry.rootButton`)。
private struct RootButtonEntry: Identifiable {
    let id: String
    let title: String
    let accessibilityIdentifier: String
}

/// 右上のアイコンの入口(`FeatureEntry.toolbarIcon`)。
private struct ToolbarIconEntry: Identifiable {
    let id: String
    let systemImage: String
    let accessibilityLabel: String
    let accessibilityIdentifier: String
}

extension RootView {
    fileprivate static let rootButtons: [RootButtonEntry] = FeatureRegistry.features.compactMap { feature in
        guard case .rootButton(let title, let identifier) = feature.entry else { return nil }
        return RootButtonEntry(id: feature.id, title: title, accessibilityIdentifier: identifier)
    }

    fileprivate static let toolbarIcons: [ToolbarIconEntry] = FeatureRegistry.features.compactMap { feature in
        guard case .toolbarIcon(let systemImage, let label, let identifier) = feature.entry else { return nil }
        return ToolbarIconEntry(
            id: feature.id, systemImage: systemImage, accessibilityLabel: label, accessibilityIdentifier: identifier)
    }
}

#Preview {
    RootView(environment: .makeAtLaunch(environment: [AppConfiguration.useMockEnvironmentKey: "1"]))
}
