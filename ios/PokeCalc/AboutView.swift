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
    /// 「データの扱い」セクションの削除先。nil(設定エラーで API が無い)ならセクションを出さない。
    private let deviceDataService: (any DeviceDataService)?

    init(deviceDataService: (any DeviceDataService)? = nil) {
        self.deviceDataService = deviceDataService
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                noticeCard
                if let deviceDataService {
                    DeviceDataSection(service: deviceDataService)
                }
                dataSourcesSection
            }
            .padding(SpacingToken.x4)
        }
        .popScreenBackground()
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
            .popCard()
            .accessibilityIdentifier("aboutUnofficialNotice")
    }

    private var dataSourcesSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: "データの出典", systemImage: PopSymbol.info)

            ForEach(Array(AboutText.dataSources.enumerated()), id: \.offset) { index, source in
                AboutDataSourceRow(source: source)
                    .accessibilityIdentifier("aboutDataSource-\(index)")
            }
        }
    }
}

/// 「データの扱い」セクション(P6-7・ADR-0501「P6-7」2章)。文言は `DeviceDataText`、状態は
/// `DeviceDataDeletionViewModel` に任せ、ここは描くだけ。削除は確認(alert)を必ず挟む。
private struct DeviceDataSection: View {
    @State private var viewModel: DeviceDataDeletionViewModel
    @State private var task: Task<Void, Never>?

    init(service: any DeviceDataService) {
        _viewModel = State(initialValue: DeviceDataDeletionViewModel(service: service))
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: DeviceDataText.sectionTitle, systemImage: PopSymbol.delete)

            ForEach(Array(DeviceDataText.explanation.enumerated()), id: \.offset) { index, sentence in
                Text(sentence)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("deviceDataExplanation-\(index)")
            }

            if viewModel.phase == .confirming {
                confirmationCard
            } else {
                Button { viewModel.requestDeletion() } label: { PopLabel(title: DeviceDataText.deleteButton, systemImage: PopSymbol.delete) }
                    .buttonStyle(PillButtonStyle(kind: .danger))
                    .disabled(viewModel.phase == .deleting)
                    .accessibilityIdentifier("deleteDeviceDataButton")
            }

            if let message = viewModel.statusMessage {
                Text(message)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("deleteDeviceDataStatus")
            }

            if viewModel.canRetry {
                Button(DeviceDataText.retryButton) {
                    task = Task { await viewModel.retry() }
                }
                .buttonStyle(PillButtonStyle())
                .accessibilityIdentifier("retryDeleteDeviceDataButton")
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        // 子要素の identifier を上書きしないよう、コンテナとして識別する(`aboutDataSource-*` と同じ理由)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("deviceDataSection")
        .onDisappear { task?.cancel() }
    }

    /// 削除の確認。システムの alert / confirmationDialog は XCUITest で同じ identifier のボタンが入れ子に
    /// 2つ見えて操作できないため、セクション内の確認として描く(確認なしに削除しない点は同じ。ADR-0501「P6-7」実装結果)。
    private var confirmationCard: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text(DeviceDataText.confirmMessage)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button(DeviceDataText.confirmAction) {
                task = Task { await viewModel.confirmDeletion() }
            }
            .buttonStyle(PillButtonStyle(kind: .danger))
            .accessibilityIdentifier("confirmDeleteDeviceDataButton")
            Button(DeviceDataText.cancelAction) { viewModel.cancelConfirmation() }
                .buttonStyle(PillButtonStyle())
                .accessibilityIdentifier("cancelDeleteDeviceDataButton")
        }
        .padding(SpacingToken.x3)
        .popCard()
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
        .popCard()
        // `CalcScreenResults.swift`(`calcResultRow-*`)と同じ理由: `popCard()` のコンテナが
        // 複数の `Text` を持つ場合、これが無いと同じ identifier の要素が複数見つかってしまう。
        .accessibilityElement(children: .contain)
    }
}

#Preview {
    NavigationStack {
        AboutView()
    }
}
