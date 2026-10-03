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
    /// 起動時にいきなり調整画面を開かせる環境変数(同上。ADR-0502 §1)。
    static let openAdjustScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_ADJUST_SCREEN_AT_LAUNCH"
    private static let openAdjustScreenAtLaunchValue = "1"
    /// 起動時にいきなり素早さ比較画面を開かせる環境変数(同上。ADR-0503 §7)。
    static let openSpeedScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH"
    private static let openSpeedScreenAtLaunchValue = "1"
    /// 起動時にいきなり判定画面を開かせる環境変数(同上。ADR-0504)。
    static let openJudgeScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH"
    private static let openJudgeScreenAtLaunchValue = "1"

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

                        NavigationLink(value: AdjustScreenRoute()) {
                            Text(AdjustText.screenTitle)
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openAdjustScreen")

                        NavigationLink(value: BalanceScreenRoute()) {
                            Text("タイプバランス")
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openBalanceScreen")

                        NavigationLink(value: SpeedScreenRoute()) {
                            Text(SpeedLabels.openButton)
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openSpeedScreen")

                        NavigationLink(value: JudgeScreenRoute()) {
                            Text(JudgeLabels.openButton)
                        }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("openJudgeScreen")
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
                if case .ready(let service, _, let backendDescription, _, _, let frequentOpponents, _, _) = environment {
                    CalcScreenView(
                        service: service, teamStore: teamStore, backendDescription: backendDescription,
                        frequentOpponentsService: frequentOpponents)
                }
            }
            .navigationDestination(for: ReverseScreenRoute.self) { _ in
                if case .ready(let service, _, let backendDescription, _, _, let frequentOpponents, _, _) = environment {
                    ReverseScreenView(
                        service: service, teamStore: teamStore, backendDescription: backendDescription,
                        frequentOpponentsService: frequentOpponents)
                }
            }
            .navigationDestination(for: TeamListScreenRoute.self) { _ in
                if case .ready(let service, _, _, _, _, _, _, _) = environment {
                    TeamListView(store: teamStore, service: service, path: $path)
                }
            }
            .navigationDestination(for: AdjustScreenRoute.self) { _ in
                if case .ready(let service, _, let backendDescription, let adjust, _, _, _, _) = environment {
                    AdjustScreenView(service: service, adjust: adjust, backendDescription: backendDescription)
                }
            }
            .navigationDestination(for: BalanceScreenRoute.self) { _ in
                if case .ready(let service, _, _, _, let balance, _, _, _) = environment {
                    BalanceScreenView(balance: balance, service: service, teamStore: teamStore)
                }
            }
            .navigationDestination(for: SpeedScreenRoute.self) { _ in
                if case .ready(_, _, _, _, _, _, let speed, _) = environment {
                    SpeedScreenView(service: speed)
                }
            }
            .navigationDestination(for: JudgeScreenRoute.self) { _ in
                if case .ready(let service, _, _, _, _, _, _, let judge) = environment {
                    JudgeScreenView(service: judge, master: service, teamStore: teamStore)
                }
            }
            .navigationDestination(for: AboutScreenRoute.self) { _ in
                if case .ready(_, let deviceData, _, _, _, _, _, _) = environment {
                    AboutView(deviceDataService: deviceData)
                } else {
                    AboutView()
                }
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
            } else if env[Self.openAdjustScreenAtLaunchEnvironmentKey] == Self.openAdjustScreenAtLaunchValue {
                path.append(AdjustScreenRoute())
            } else if env[Self.openSpeedScreenAtLaunchEnvironmentKey] == Self.openSpeedScreenAtLaunchValue {
                path.append(SpeedScreenRoute())
            } else if env[Self.openJudgeScreenAtLaunchEnvironmentKey] == Self.openJudgeScreenAtLaunchValue {
                path.append(JudgeScreenRoute())
            }
        }
    }

    @ViewBuilder
    private var statusBadge: some View {
        switch environment {
        case .ready(_, _, let description, _, _, _, _, _):
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

/// `NavigationPath` に積む調整画面の行き先(値だけで、状態は持たない。ADR-0502)。
private struct AdjustScreenRoute: Hashable {}

/// `NavigationPath` に積むタイプバランス画面の行き先(値だけで、状態は持たない。P6-21)。
private struct BalanceScreenRoute: Hashable {}

/// `NavigationPath` に積む素早さ比較画面の行き先(値だけで、状態は持たない。P6-24)。
private struct SpeedScreenRoute: Hashable {}

/// `NavigationPath` に積む判定画面の行き先(値だけで、状態は持たない。P6-25)。
private struct JudgeScreenRoute: Hashable {}

/// `NavigationPath` に積む「このアプリについて」画面の行き先(値だけで、状態は持たない。P6-18)。
private struct AboutScreenRoute: Hashable {}

#Preview {
    if let mock = try? MockPokeCalcService(), let adjust = try? MockAdjustService() {
        RootView(
            environment: .ready(
                service: mock, deviceData: MockDeviceDataService(), backendDescription: "モックデータで動作中", adjust: adjust,
                balance: UnavailableBalanceService(), frequentOpponents: MockFrequentOpponentsService(),
                speed: MockSpeedService(), judge: MockJudgeService()))
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
