import PokeCalcCore
import PokeCalcDesign
import SwiftUI

/// ルート画面(P6-1)。モックで動いていることの表示と、計算画面(P6-2)への入口だけを持つ。
struct RootView: View {
    private let environment: AppEnvironment
    /// 構築(team)の永続化(P6-2c・ADR-0500 §4)。`RootView` が1回だけ作り、一覧・編集画面へ
    /// 渡す(`AppEnvironment` は計算/逆算が使う `PokeCalcService` の生成元なので、契約が別の
    /// `TeamStore` をそこに混ぜない)。
    @State private var teamStore: any TeamStore
    /// 計算画面への遷移を値ベースにし、起動時に自動で遷移させられるようにする(下記の
    /// `openCalcScreenAtLaunchEnvironmentKey` 用。テスト・スクリーンショット撮影専用)。
    @State private var path = NavigationPath()

    /// 起動時にいきなり計算画面を開かせる環境変数(XCUITest を介さずスクリーンショットを撮る用途。
    /// 通常の起動には影響しない。ADR-0501「P6-2a」参照)。
    static let openCalcScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH"
    private static let openCalcScreenAtLaunchValue = "1"
    /// 起動時にいきなり逆算画面を開かせる環境変数(同上。ADR-0501「P6-2b」参照)。
    static let openReverseScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_REVERSE_SCREEN_AT_LAUNCH"
    private static let openReverseScreenAtLaunchValue = "1"
    /// 起動時にいきなり構築一覧画面を開かせる環境変数(同上。ADR-0501「P6-2c」参照)。
    static let openTeamListScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_TEAM_LIST_SCREEN_AT_LAUNCH"
    private static let openTeamListScreenAtLaunchValue = "1"

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
                        NavigationLink(value: CalcScreenRoute()) {
                            Text("計算する")
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openCalcScreen")

                        NavigationLink(value: ReverseScreenRoute()) {
                            Text("逆算する")
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openReverseScreen")

                        NavigationLink(value: TeamListScreenRoute()) {
                            Text("構築")
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openTeamListScreen")
                    }
                }
                Spacer()
            }
            .padding(SpacingToken.x4)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(ColorToken.bgBase.color.ignoresSafeArea())
            .toolbar {
                // `navigationTitle` はシステムフォントに固定されるため、design.md の
                // SF Pro Rounded を出すために principal 位置のカスタム View で置き換える。
                ToolbarItem(placement: .principal) {
                    Text("PokeCalc")
                        .font(TextStyleToken.heading.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                }
                // P6-18(issue #328): 非公式の表示とデータの出典への控えめな入口。
                // `.principal` は上記の見出しで埋まっているため `.topBarTrailing` に置く。
                ToolbarItem(placement: .topBarTrailing) {
                    NavigationLink(value: AboutScreenRoute()) {
                        Image(systemName: "info.circle")
                    }
                    .accessibilityIdentifier("openAboutScreen")
                    .accessibilityLabel("このアプリについて")
                }
            }
            .navigationDestination(for: CalcScreenRoute.self) { _ in
                if case .ready(let service, let backendDescription) = environment {
                    CalcScreenView(service: service, teamStore: teamStore, backendDescription: backendDescription)
                }
            }
            .navigationDestination(for: ReverseScreenRoute.self) { _ in
                if case .ready(let service, let backendDescription) = environment {
                    ReverseScreenView(service: service, teamStore: teamStore, backendDescription: backendDescription)
                }
            }
            .navigationDestination(for: TeamListScreenRoute.self) { _ in
                if case .ready(let service, _) = environment {
                    TeamListView(store: teamStore, service: service, path: $path)
                }
            }
            .navigationDestination(for: AboutScreenRoute.self) { _ in
                AboutView()
            }
        }
        .task {
            let env = ProcessInfo.processInfo.environment
            guard case .ready = environment else { return }
            if env[Self.openCalcScreenAtLaunchEnvironmentKey] == Self.openCalcScreenAtLaunchValue {
                path.append(CalcScreenRoute())
            } else if env[Self.openReverseScreenAtLaunchEnvironmentKey] == Self.openReverseScreenAtLaunchValue {
                path.append(ReverseScreenRoute())
            } else if env[Self.openTeamListScreenAtLaunchEnvironmentKey] == Self.openTeamListScreenAtLaunchValue {
                path.append(TeamListScreenRoute())
            }
        }
    }

    @ViewBuilder
    private var statusBadge: some View {
        switch environment {
        case .ready(_, let description):
            Text(description)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.bgGlass.color, in: Capsule())
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

/// design.md「ピルのボタン」。無彩色トークンだけを使う(色を持つのはタイプだけ、という方針。
/// システムの既定色(青)に頼らない)。
struct PillButtonStyle: ButtonStyle {
    /// 押している間だけ少し薄くして、押せたことを示す(操作への反応だけの演出。design.md「動き」)。
    private static let pressedOpacity = 0.7

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(TextStyleToken.heading.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .padding(.horizontal, SpacingToken.x6)
            .padding(.vertical, SpacingToken.x3)
            .background(ColorToken.bgGlass.color, in: Capsule())
            .opacity(configuration.isPressed ? Self.pressedOpacity : 1.0)
    }
}

/// `NavigationPath` に積む計算画面の行き先(値だけで、状態は持たない)。
private struct CalcScreenRoute: Hashable {}

/// `NavigationPath` に積む逆算画面の行き先(値だけで、状態は持たない)。
private struct ReverseScreenRoute: Hashable {}

/// `NavigationPath` に積む構築一覧画面の行き先(値だけで、状態は持たない)。
private struct TeamListScreenRoute: Hashable {}

/// `NavigationPath` に積む「このアプリについて」画面の行き先(値だけで、状態は持たない。P6-18)。
private struct AboutScreenRoute: Hashable {}

#Preview {
    if let mock = try? MockPokeCalcService() {
        RootView(environment: .ready(service: mock, backendDescription: "モックデータで動作中"))
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
