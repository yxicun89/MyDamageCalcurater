import PokeCalcCore
import PokeCalcDesign
import SwiftUI
import UIKit

// TeamTextSections: 構築の Showdown 形式の書き出し・取り込み(P6-20・ADR-0501「P6-20」・ADR-0502)。
//
// F-08(ADR-0522)でシートをやめ、補助の入口として折りたたみの中に置く: 取り込みは一覧の下(新しい構築として作る)、
// 書き出しは編集画面の下。ロジックは持たない。`TeamTextTransferViewModel`(PokeCalcCore)の状態を描き、
// 操作をつなぐだけ。文言は `ShowdownTextLabels` / `TeamLabels` の1か所。`Menu` は使わない(identifier が UIKit に
// 渡らないため)。確認は alert/confirmationDialog ではなくカードで描く(P6-7)。

/// 折りたたみ(閉じた状態で始まる)。`DisclosureGroup` ではなく自前のボタンで開閉する(識別子を確実に付けるため)。
/// 開いている間だけ中身を出す。常時動くアニメーションは持たない。
struct TeamFold<Content: View>: View {
    let title: String
    let identifier: String
    @Binding var isOpen: Bool
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Button {
                isOpen.toggle()
            } label: {
                HStack(spacing: SpacingToken.x2) {
                    Text(title)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .multilineTextAlignment(.leading)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Image(systemName: isOpen ? "chevron.up" : "chevron.down")
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .accessibilityHidden(true)
                }
                .padding(SpacingToken.x3)
                .frame(minHeight: CalcScreenMetrics.minimumTapSide)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityValue(isOpen ? "開いています" : "閉じています")
            .accessibilityIdentifier("\(identifier)Toggle")
            if isOpen {
                VStack(alignment: .leading, spacing: SpacingToken.x3) {
                    content()
                }
                .padding(.horizontal, SpacingToken.x3)
                .padding(.bottom, SpacingToken.x3)
                // 子の識別子を上書きしないよう、まとまりとして公開する。
                .accessibilityElement(children: .contain)
                .accessibilityIdentifier("\(identifier)Content")
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard()
    }
}

// MARK: - 書き出し(編集画面の下の折りたたみ)

/// 開いている構築のいまの内容を書き出す。
struct TeamExportSection: View {
    @State private var model: TeamTextTransferViewModel
    let members: [TeamMember]
    @State private var didCopy = false

    init(service: any PokeCalcService, members: [TeamMember]) {
        _model = State(initialValue: TeamTextTransferViewModel(service: service))
        self.members = members
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(TeamLabels.exportHelp)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("teamExportHelp")
            TeamTextControls.actionButton(ShowdownTextLabels.exportTeamButton, identifier: "exportTeamTextButton") {
                Task {
                    didCopy = false
                    await model.prepareExport(members: members)
                }
            }
            .disabled(members.isEmpty)
            if model.isExporting {
                ProgressView()
            }
            if model.exportError != nil {
                TeamTextControls.noticeText(ShowdownTextLabels.lookupFailure, identifier: "exportFailureNotice", isError: true)
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
                    TeamTextControls.actionButton(ShowdownTextLabels.copyButton, identifier: "copyExportedTextButton") {
                        UIPasteboard.general.string = text
                        didCopy = true
                    }
                    ShareLink(item: text) {
                        TeamTextControls.buttonLabel(ShowdownTextLabels.shareButton)
                    }
                    .buttonStyle(PillButtonStyle(kind: .secondary))
                    .accessibilityIdentifier("shareExportedTextLink")
                    if model.exportUnresolvedCount > 0 {
                        TeamTextControls.noticeText(
                            ShowdownTextLabels.exportUnresolvedNotice(count: model.exportUnresolvedCount),
                            identifier: "exportUnresolvedNotice", isError: false)
                    }
                    if model.exportSkippedMemberCount > 0 {
                        TeamTextControls.noticeText(
                            ShowdownTextLabels.exportSkippedNotice(count: model.exportSkippedMemberCount),
                            identifier: "exportSkippedNotice", isError: false)
                    }
                    if didCopy {
                        TeamTextControls.noticeText(ShowdownTextLabels.copiedNotice, identifier: "copiedNotice", isError: false)
                    }
                }
                .padding(SpacingToken.x3)
                .frame(maxWidth: .infinity, alignment: .leading)
                .popInset()
            }
        }
    }
}

// MARK: - 取り込み(一覧の下の折りたたみ。新しい構築として作る)

struct TeamImportSection: View {
    @State private var model: TeamTextTransferViewModel
    /// 追加を確定したとき、取り込める体を渡す(呼び出し側が新しい構築として保存する)。
    let onImport: ([TeamMember]) -> Void
    @State private var showsEmptyNotice = false

    init(service: any PokeCalcService, onImport: @escaping ([TeamMember]) -> Void) {
        _model = State(initialValue: TeamTextTransferViewModel(service: service))
        self.onImport = onImport
    }

    /// テキスト欄の最小の高さ(数行ぶん。値の意味は「貼り付け欄が潰れない下限」)。
    private static let editorMinHeight: CGFloat = 120

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            helpAndExample
            importEditor
            reviewSection
        }
    }

    // 説明と入力例(何を入れればよいかが分かるように)
    private var helpAndExample: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(TeamLabels.importHelp)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("teamImportHelp")
            Text(TeamLabels.importExampleLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
            Text(TeamLabels.importExample)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(SpacingToken.x3)
                .popInset()
                .accessibilityIdentifier("teamImportExample")
        }
    }

    private var importEditor: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
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
            .popInset()
            TeamTextControls.actionButton(ShowdownTextLabels.analyzeButton, identifier: "analyzeImportTextButton") {
                Task { await analyze() }
            }
            if showsEmptyNotice {
                TeamTextControls.noticeText(ShowdownTextLabels.emptyInput, identifier: "importEmptyNotice", isError: false)
            }
        }
    }

    private func analyze() async {
        showsEmptyNotice = model.pastedText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        // 取り込みは常に新しい構築を作るので、既存の体数は 0(1 つの構築は 6 体まで)。
        await model.analyze(existingMemberCount: 0)
    }

    // MARK: 確認(取り込めなかった行と、取り込める分だけ追加するかの選択)

    @ViewBuilder
    private var reviewSection: some View {
        if model.phase == .reviewing {
            VStack(alignment: .leading, spacing: SpacingToken.x3) {
                if model.importError != nil {
                    TeamTextControls.noticeText(ShowdownTextLabels.lookupFailure, identifier: "importFailureNotice", isError: true)
                }
                if !model.rejected.isEmpty {
                    rejectedCard
                }
                if model.importableCount == 0, model.importError == nil {
                    TeamTextControls.noticeText(ShowdownTextLabels.nothingImportable, identifier: "importNothingNotice", isError: false)
                }
                if model.canConfirm {
                    let label = model.needsDecision
                        ? ShowdownTextLabels.importValidOnlyButton(count: model.importableCount)
                        : ShowdownTextLabels.importAllButton(count: model.importableCount)
                    TeamTextControls.actionButton(label, identifier: "confirmImportValidButton") {
                        onImport(model.confirm())
                    }
                }
                TeamTextControls.actionButton(ShowdownTextLabels.cancelButton, identifier: "cancelImportButton") {
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
        .popCard()
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
}

// MARK: - 部品

enum TeamTextControls {
    static func buttonLabel(_ title: String) -> some View {
        Text(title)
            .font(TextStyleToken.body.font)
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity)
    }

    static func actionButton(_ title: String, identifier: String, action: @escaping () -> Void) -> some View {
        Button(action: action) { buttonLabel(title) }
            .buttonStyle(PillButtonStyle(kind: .secondary))
            .accessibilityIdentifier(identifier)
    }

    static func noticeText(_ text: String, identifier: String, isError: Bool) -> some View {
        PopNoticeView(kind: isError ? .error : .info, message: text, identifier: identifier)
    }
}
