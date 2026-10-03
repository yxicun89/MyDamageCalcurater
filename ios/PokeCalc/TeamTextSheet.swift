import PokeCalcCore
import PokeCalcDesign
import SwiftUI
import UIKit

// TeamTextSheet: 構築のテキスト書き出し・取り込みのシート(P6-20・ADR-0501「P6-20」・ADR-0502)。
//
// ロジックは持たない。`TeamTextTransferViewModel`(PokeCalcCore)の状態を描き、操作をつなぐだけ。
// 文言は `ShowdownTextLabels` の1か所。`Menu` は使わない(identifier が UIKit に渡らないため)。確認は
// alert/confirmationDialog ではなくカードで描く(P6-7。iOS 26 系の XCUITest が入れ子2つを返すため)。

/// シートを開く依頼。`exportMembers` が非 nil なら開いた直後にその体を書き出す(メンバーカードの入口)。
struct TeamTextRequest: Identifiable {
    let id = UUID()
    let exportMembers: [TeamMember]?
}

struct TeamTextSheet: View {
    @State private var model: TeamTextTransferViewModel
    let members: [TeamMember]
    let initialExportMembers: [TeamMember]?
    /// 追加を確定したとき、取り込める体を渡す(保存はしない。呼び出し側が `importMembers` へ渡す)。
    let onImport: ([TeamMember]) -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var didCopy = false
    @State private var showsEmptyNotice = false

    init(
        service: any PokeCalcService, members: [TeamMember], initialExportMembers: [TeamMember]?,
        onImport: @escaping ([TeamMember]) -> Void
    ) {
        _model = State(initialValue: TeamTextTransferViewModel(service: service))
        self.members = members
        self.initialExportMembers = initialExportMembers
        self.onImport = onImport
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                titleRow
                exportSection
                importSection
                reviewSection
            }
            .padding(SpacingToken.x4)
        }
        .scrollDismissesKeyboard(.interactively)
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("teamTextSheet")
        .task {
            if let initialExportMembers { await export(initialExportMembers) }
        }
    }

    // MARK: - 見出し

    private var titleRow: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            Text(ShowdownTextLabels.sheetTitle)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button(ShowdownTextLabels.closeButton) { dismiss() }
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .accessibilityIdentifier("closeTeamTextSheetButton")
        }
    }

    // MARK: - 書き出し

    private var exportSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            actionButton(ShowdownTextLabels.exportTeamButton, identifier: "exportTeamTextButton") {
                Task { await export(members) }
            }
            if model.isExporting {
                ProgressView()
            }
            if model.exportError != nil {
                noticeText(ShowdownTextLabels.lookupFailure, identifier: "exportFailureNotice", isError: true)
            }
            if let text = model.exportText {
                VStack(alignment: .leading, spacing: SpacingToken.x2) {
                    Text(text)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .textSelection(.enabled)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .accessibilityIdentifier("exportedText")
                    actionButton(ShowdownTextLabels.copyButton, identifier: "copyExportedTextButton") {
                        UIPasteboard.general.string = text
                        didCopy = true
                    }
                    ShareLink(item: text) {
                        buttonLabel(ShowdownTextLabels.shareButton)
                    }
                    .accessibilityIdentifier("shareExportedTextLink")
                    if model.exportUnresolvedCount > 0 {
                        noticeText(
                            ShowdownTextLabels.exportUnresolvedNotice(count: model.exportUnresolvedCount),
                            identifier: "exportUnresolvedNotice", isError: false)
                    }
                    if model.exportSkippedMemberCount > 0 {
                        noticeText(
                            ShowdownTextLabels.exportSkippedNotice(count: model.exportSkippedMemberCount),
                            identifier: "exportSkippedNotice", isError: false)
                    }
                    if didCopy {
                        noticeText(ShowdownTextLabels.copiedNotice, identifier: "copiedNotice", isError: false)
                    }
                }
                .padding(SpacingToken.x3)
                .frame(maxWidth: .infinity, alignment: .leading)
                .glassCard()
            }
        }
    }

    private func export(_ target: [TeamMember]) async {
        didCopy = false
        await model.prepareExport(members: target)
    }

    // MARK: - 取り込み(貼り付け)

    private var importSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(ShowdownTextLabels.importSectionTitle)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
            ZStack(alignment: .topLeading) {
                TextEditor(
                    text: Binding(
                        get: { model.pastedText },
                        set: { newValue in
                            model.setPastedText(newValue)
                            showsEmptyNotice = false
                        }
                    )
                )
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .scrollContentBackground(.hidden)
                .scrollDismissesKeyboard(.interactively)
                .frame(minHeight: Self.editorMinHeight)
                .accessibilityIdentifier("importTextEditor")
                if model.pastedText.isEmpty {
                    Text(ShowdownTextLabels.importPlaceholder)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .padding(.top, SpacingToken.x2)
                        .padding(.leading, SpacingToken.x1)
                        .allowsHitTesting(false)
                        .accessibilityHidden(true)
                }
            }
            .padding(SpacingToken.x2)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            actionButton(ShowdownTextLabels.analyzeButton, identifier: "analyzeImportTextButton") {
                Task { await analyze() }
            }
            if showsEmptyNotice {
                noticeText(ShowdownTextLabels.emptyInput, identifier: "importEmptyNotice", isError: false)
            }
        }
    }

    /// テキスト欄の最小の高さ(数行ぶん。値の意味は「貼り付け欄が潰れない下限」)。
    private static let editorMinHeight: CGFloat = 120

    private func analyze() async {
        showsEmptyNotice = model.pastedText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        await model.analyze(existingMemberCount: existingMemberCount)
    }

    private var existingMemberCount: Int { members.count }

    // MARK: - 確認(取り込めなかった行と、取り込める分だけ追加するかの選択)

    @ViewBuilder
    private var reviewSection: some View {
        if model.phase == .reviewing {
            VStack(alignment: .leading, spacing: SpacingToken.x3) {
                if model.importError != nil {
                    noticeText(ShowdownTextLabels.lookupFailure, identifier: "importFailureNotice", isError: true)
                }
                if !model.rejected.isEmpty {
                    rejectedCard
                }
                if model.importableCount == 0, model.importError == nil {
                    noticeText(ShowdownTextLabels.nothingImportable, identifier: "importNothingNotice", isError: false)
                }
                if model.canConfirm {
                    let label = model.needsDecision
                        ? ShowdownTextLabels.importValidOnlyButton(count: model.importableCount)
                        : ShowdownTextLabels.importAllButton(count: model.importableCount)
                    actionButton(label, identifier: "confirmImportValidButton") {
                        onImport(model.confirm())
                        dismiss()
                    }
                }
                actionButton(ShowdownTextLabels.cancelButton, identifier: "cancelImportButton") {
                    model.cancel()
                    showsEmptyNotice = false
                }
            }
        }
    }

    private var rejectedCard: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(ShowdownTextLabels.rejectedTitle)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityHidden(true)
            ForEach(Array(model.rejected.enumerated()), id: \.offset) { _, rejection in
                rejectedLine(rejection)
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityLabel(ShowdownTextLabels.rejectedTitle)
        .accessibilityIdentifier("importRejectedList")
    }

    private func rejectedLine(_ rejection: ShowdownRejection) -> some View {
        let lineLabel = ShowdownTextLabels.lineNumberLabel(rejection.lineNumber)
        let message = ShowdownTextLabels.message(for: rejection.reason)
        return VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(lineLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            Text(rejection.text)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
            Text(message)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.danger.color)
        }
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(lineLabel) \(rejection.text) \(message)")
        .accessibilityIdentifier("importRejectedLine-\(rejection.lineNumber)")
    }

    // MARK: - 部品

    private func buttonLabel(_ title: String) -> some View {
        Text(title)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x3)
            .frame(maxWidth: .infinity)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
    }

    private func actionButton(_ title: String, identifier: String, action: @escaping () -> Void) -> some View {
        Button(action: action) { buttonLabel(title) }
            .buttonStyle(.plain)
            .accessibilityIdentifier(identifier)
    }

    private func noticeText(_ text: String, identifier: String, isError: Bool) -> some View {
        Text(text)
            .font(TextStyleToken.body.font)
            .foregroundStyle(isError ? ColorToken.danger.color : ColorToken.textSecondary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(identifier)
    }
}
