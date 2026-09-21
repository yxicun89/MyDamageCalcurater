import PokeCalcCore
import PokeCalcDesign
import SwiftUI

/// ルート画面(P6-1)。モックで動いていることの表示と、計算画面(P6-2)への入口だけを持つ。
struct RootView: View {
    private let environment: AppEnvironment

    /// `environment` は既定でも作れる(#Preview 用)。実行時は `PokeCalcApp` が `@State` で
    /// 1回だけ作ったものを渡す(セッション ID を起動ごとに1つに保つため)。
    init(environment: AppEnvironment = .makeAtLaunch()) {
        self.environment = environment
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: SpacingToken.x6) {
                statusBadge
                Spacer()
                if case .ready = environment {
                    NavigationLink {
                        CalcPlaceholderView()
                    } label: {
                        Text("計算する")
                    }
                    .buttonStyle(PillButtonStyle())
                    .accessibilityIdentifier("openCalcScreen")
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

/// 計算画面(P6-2)のプレースホルダ。P6-1 では画面遷移がつながることだけを確かめる。
struct CalcPlaceholderView: View {
    var body: some View {
        Text("計算画面(準備中)")
            .font(TextStyleToken.heading.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .background(ColorToken.bgBase.color.ignoresSafeArea())
            .accessibilityIdentifier("calcScreen")
    }
}

#Preview {
    if let mock = try? MockPokeCalcService() {
        RootView(environment: .ready(service: mock, backendDescription: "モックデータで動作中"))
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
