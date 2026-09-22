import PokeCalcCore
import PokeCalcDesign
import SwiftUI

/// ルート画面(P6-1)。モックで動いていることの表示と、計算画面(P6-2)への入口だけを持つ。
struct RootView: View {
    private let environment: AppEnvironment
    /// 計算画面への遷移を値ベースにし、起動時に自動で遷移させられるようにする(下記の
    /// `openCalcScreenAtLaunchEnvironmentKey` 用。テスト・スクリーンショット撮影専用)。
    @State private var path = NavigationPath()

    /// 起動時にいきなり計算画面を開かせる環境変数(XCUITest を介さずスクリーンショットを撮る用途。
    /// 通常の起動には影響しない。README「P6-2a」参照)。
    static let openCalcScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH"
    private static let openCalcScreenAtLaunchValue = "1"
    /// 起動時にいきなり逆算画面を開かせる環境変数(同上。README「P6-2b」参照)。
    static let openReverseScreenAtLaunchEnvironmentKey = "POKECALC_OPEN_REVERSE_SCREEN_AT_LAUNCH"
    private static let openReverseScreenAtLaunchValue = "1"

    /// `environment` は既定でも作れる(#Preview 用)。実行時は `PokeCalcApp` が `@State` で
    /// 1回だけ作ったものを渡す(セッション ID を起動ごとに1つに保つため)。
    init(environment: AppEnvironment = .makeAtLaunch()) {
        self.environment = environment
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
            }
            .navigationDestination(for: CalcScreenRoute.self) { _ in
                if case .ready(let service, let backendDescription) = environment {
                    CalcScreenView(service: service, backendDescription: backendDescription)
                }
            }
            .navigationDestination(for: ReverseScreenRoute.self) { _ in
                if case .ready(let service, let backendDescription) = environment {
                    ReverseScreenView(service: service, backendDescription: backendDescription)
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

#Preview {
    if let mock = try? MockPokeCalcService() {
        RootView(environment: .ready(service: mock, backendDescription: "モックデータで動作中"))
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
