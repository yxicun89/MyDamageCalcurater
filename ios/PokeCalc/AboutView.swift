import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AboutView: 「このアプリについて」画面(P6-18・issue #328・ADR-0501「P6-18」1章)。
//
// 文言(非公式の注記・データの出典一覧)はすべて `PokeCalcCore.AboutText` に持つ(このファイルは
// それを描くだけ)。ロジックを持たない静的な画面のため専用の ViewModel は置かない
// (`TeamListView` のようにストアを介した非同期の読み込みが無いため)。

/// 「このアプリについて」画面。非公式の注記とデータの出典一覧を表示する。
struct AboutView: View {
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                noticeCard
                dataSourcesSection
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("aboutScreen")
        .toolbar {
            ToolbarItem(placement: .principal) {
                Text("このアプリについて")
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
        }
    }

    /// 非公式であることの注記(ADR-0501「P6-18」2章)。長文でも折り返す(`lineLimit` を付けない。
    /// P6-14/P6-17 と同じ方針で AX5 でも横にはみ出させない)。
    private var noticeCard: some View {
        Text(AboutText.unofficialNotice)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(SpacingToken.x3)
            .glassCard()
            .accessibilityIdentifier("aboutUnofficialNotice")
    }

    private var dataSourcesSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text("データの出典")
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)

            ForEach(Array(AboutText.dataSources.enumerated()), id: \.offset) { index, source in
                AboutDataSourceRow(source: source)
                    .accessibilityIdentifier("aboutDataSource-\(index)")
            }
        }
    }
}

/// データの出典1件の行(用途と出典・ライセンス)。
private struct AboutDataSourceRow: View {
    let source: AboutText.DataSource

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(source.title)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
            Text(source.detail)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(SpacingToken.x3)
        .glassCard()
        // `CalcScreenResults.swift`(`calcResultRow-*`)と同じ理由: `glassCard()` のコンテナが
        // 複数の `Text` を持つ場合、これが無いと同じ identifier の要素が複数見つかってしまう。
        .accessibilityElement(children: .contain)
    }
}

#Preview {
    NavigationStack {
        AboutView()
    }
}
