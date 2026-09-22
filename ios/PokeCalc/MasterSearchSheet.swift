import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// MasterSearchSheet: 種族・技の検索シート(issue #68。ADR-0501「issue #68 の受け入れ条件」1章・8章)。
//
// `Menu` の中にインラインの検索欄は置かない(`Menu` の中身はシステムのメニュー表示に渡されるため
// `TextField` が実質的に機能しない。1章「判断」)。かわりに、入口(ヘッダー/チップ)のタップで
// `.searchable()` を付けた `List` を `.sheet` で出す。3画面(計算・逆算・構築編集)はどれも同じ形を
// 使うので、`MasterSpeciesSearchProviding` / `MasterMoveSearchProviding`(PokeCalcCore)を介して
// 1つの部品を共用する(「たまたま似ている」のではなく、ADR-0501「issue #68」10章が3画面共通の
// 契約として定めているため)。

/// 種族の検索シート。3画面のどの種族セレクタ(攻撃側・防御側・自分・相手・メンバー・新規メンバー)からも
/// これを開く(画面ごとに検索の状態は1つ。8章「シートは画面ごとに1つ」)。
struct SpeciesSearchSheet<ViewModel: MasterSpeciesSearchProviding>: View {
    let viewModel: ViewModel
    let onSelect: (SpeciesSummary) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                hintRow
                ForEach(viewModel.speciesOptions, id: \.key) { option in
                    Button {
                        onSelect(option)
                        dismiss()
                    } label: {
                        MasterSearchRow.species(option)
                    }
                    .accessibilityIdentifier("speciesSearchResult-\(option.key)")
                    // タイプバッジの文言が自動連結されて種族名だけで引けなくなるのを避ける
                    // (XCUITest は種族名でも探せるようにする)。
                    .accessibilityLabel(option.nameJa)
                }
            }
            .listStyle(.plain)
            .searchable(
                text: Binding(get: { viewModel.speciesQuery }, set: { viewModel.setSpeciesQuery($0) }),
                prompt: MasterSearchLabels.prompt
            )
            .accessibilityIdentifier("speciesSearchField")
            .task(id: viewModel.speciesQuery) { await viewModel.runSpeciesSearch() }
            .navigationTitle("ポケモンを検索")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("閉じる") { dismiss() }
                        .accessibilityIdentifier("speciesSearchCancelButton")
                }
                ToolbarItem(placement: .principal) {
                    if viewModel.isSearchingSpecies {
                        ProgressView()
                            .accessibilityIdentifier("speciesSearchLoadingIndicator")
                    }
                }
            }
        }
        .accessibilityIdentifier("speciesSearchSheet")
    }

    @ViewBuilder
    private var hintRow: some View {
        if let hint = MasterSearchRow.hint(
            query: viewModel.speciesQuery, isEmpty: viewModel.speciesOptions.isEmpty, reachedLimit: viewModel.speciesSearchReachedLimit
        ) {
            Text(hint)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityIdentifier("speciesSearchHint")
        }
    }
}

/// 技の検索シート。`options` は呼び出し元が渡す(Calc/Reverse は `moveOptions`、Team はメンバーごとの
/// `moveOptionsByMember[id]`。10章「候補一覧は画面ごとに形が違う」)。
struct MoveSearchSheet<ViewModel: MasterMoveSearchProviding>: View {
    let viewModel: ViewModel
    let options: [Move]
    let onSelect: (Move) -> Void
    /// 非 nil なら「外す」行を先頭に出す(構築編集の技スロット専用。すでに選ばれている技を外す)。
    var removeAction: (() -> Void)? = nil
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                if let removeAction {
                    Button(role: .destructive) {
                        removeAction()
                        dismiss()
                    } label: {
                        Text("外す")
                    }
                    .accessibilityIdentifier("moveSearchRemoveButton")
                }
                hintRow
                ForEach(options, id: \.id) { move in
                    Button {
                        onSelect(move)
                        dismiss()
                    } label: {
                        MasterSearchRow.move(move)
                    }
                    .accessibilityIdentifier("moveSearchResult-\(move.id)")
                    .accessibilityLabel(move.nameJa)
                }
            }
            .listStyle(.plain)
            .searchable(
                text: Binding(get: { viewModel.moveQuery }, set: { viewModel.setMoveQuery($0) }),
                prompt: MasterSearchLabels.prompt
            )
            .accessibilityIdentifier("moveSearchField")
            .task(id: viewModel.moveQuery) { await viewModel.runMoveSearch() }
            .navigationTitle("わざを検索")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("閉じる") { dismiss() }
                        .accessibilityIdentifier("moveSearchCancelButton")
                }
                ToolbarItem(placement: .principal) {
                    if viewModel.isSearchingMoves {
                        ProgressView()
                            .accessibilityIdentifier("moveSearchLoadingIndicator")
                    }
                }
            }
        }
        .accessibilityIdentifier("moveSearchSheet")
    }

    @ViewBuilder
    private var hintRow: some View {
        if let hint = MasterSearchRow.hint(query: viewModel.moveQuery, isEmpty: options.isEmpty, reachedLimit: viewModel.moveSearchReachedLimit) {
            Text(hint)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityIdentifier("moveSearchHint")
        }
    }
}

/// 検索シートの行・案内文言の共通部品(種族シート・技シートの両方から使う)。
private enum MasterSearchRow {
    /// 案内文言(prompt/truncated/noMatch のいずれか。一致していれば何も出さない。ADR 4章・8章)。
    static func hint(query: String, isEmpty: Bool, reachedLimit: Bool) -> String? {
        if query.isEmpty { return MasterSearchLabels.prompt }
        if isEmpty { return MasterSearchLabels.noMatch }
        if reachedLimit { return MasterSearchLabels.truncated }
        return nil
    }

    static func species(_ option: SpeciesSummary) -> some View {
        HStack(spacing: SpacingToken.x2) {
            SpeciesEmblemView(name: option.nameJa, types: option.types)
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(option.nameJa)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                HStack(spacing: SpacingToken.x1) {
                    ForEach(option.types, id: \.self) { TypeBadgeView(type: $0) }
                }
            }
        }
    }

    static func move(_ move: Move) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(move.nameJa)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
            Text("威力\(move.power) / \(MoveCategoryLabel.japaneseName(for: move.category))")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
    }
}
