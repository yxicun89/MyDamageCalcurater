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
    /// 「よく使う相手」(P6-23。防御側・逆算の相手だけが渡す。nil なら出さない)。
    var frequentOpponents: FrequentOpponentsViewModel?
    let onSelect: (SpeciesSummary) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                frequentOpponentsSection
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
            .task { await frequentOpponents?.refresh() }
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

    /// 空クエリのときだけ先頭に出す「よく使う相手」。選んだときは通常の検索結果と同じ `onSelect`。
    @ViewBuilder
    private var frequentOpponentsSection: some View {
        let items = frequentOpponents?.visibleItems(forQuery: viewModel.speciesQuery) ?? []
        if !items.isEmpty {
            Section {
                ForEach(items, id: \.key) { option in
                    Button {
                        onSelect(option)
                        dismiss()
                    } label: {
                        MasterSearchRow.species(option)
                    }
                    .accessibilityIdentifier("frequentOpponentRow-\(option.key)")
                    .accessibilityLabel(option.nameJa)
                }
            } header: {
                Text(FrequentOpponentsLabels.sectionTitle)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityAddTraits(.isHeader)
                    .accessibilityIdentifier("frequentOpponentsSection")
            }
        }
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
    /// true なら技をタイプ別の群(見出し = タイプ名)で出す(G-01。並びはタイプ順だけ)。`options` は呼び出し元がタイプ順に並べて渡す。
    var groupsByType = false
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
                if groupsByType {
                    // タイプ順は見出し(タイプ名)つきの群で出す。
                    ForEach(MoveSort.typeGroups(options), id: \.type) { group in
                        Section(PokeTypeLabel.japaneseName(for: group.type)) {
                            ForEach(group.moves, id: \.id) { moveRow($0) }
                        }
                    }
                } else {
                    ForEach(options, id: \.id) { moveRow($0) }
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

    private func moveRow(_ move: Move) -> some View {
        Button {
            onSelect(move)
            dismiss()
        } label: {
            MasterSearchRow.move(move)
        }
        .accessibilityIdentifier("moveSearchResult-\(move.id)")
        .accessibilityLabel(move.nameJa)
        // 読み上げは従来どおり全情報(タイプ・分類・威力)を含める。見た目は名前+アイコンの1行(G-01)。
        .accessibilityValue(MoveRowLabels.detail(move))
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
            SpeciesImageView(speciesKey: option.key, name: option.nameJa, types: option.types)
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

    /// 技の1行(G-01): タイプのアイコン + 技名を主に、分類の小さなアイコンと威力を副に、縦に短い1行で出す。
    static func move(_ move: Move) -> some View {
        HStack(spacing: SpacingToken.x2) {
            MoveTypeEmblem(type: move.type)
            Text(move.nameJa)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
            Spacer(minLength: SpacingToken.x2)
            Image(systemName: MoveRowLabels.categorySymbol(move.category))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityLabel(MoveCategoryLabel.japaneseName(for: move.category))
            Text("\(move.power)")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityLabel("威力\(move.power)")
        }
        .frame(minHeight: CalcScreenMetrics.minimumTapSide)
    }
}

/// 技のタイプを示す丸いアイコン(タイプ色の円 + タイプ名の頭文字。画像を使わない)。
private struct MoveTypeEmblem: View {
    let type: PokeType
    private static let side: CGFloat = 28

    var body: some View {
        Circle()
            .fill(TypeColorToken.color(forTypeID: type.rawValue) ?? ColorToken.textSecondary.color)
            .frame(width: Self.side, height: Self.side)
            .overlay(
                Text(PokeTypeLabel.japaneseName(for: type).prefix(1))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(TypeColorToken.inkColor(forTypeID: type.rawValue) ?? .white)
            )
            .accessibilityHidden(true)
    }
}

private enum MoveRowLabels {
    static func categorySymbol(_ category: MoveCategory) -> String {
        switch category {
        case .physical: "figure.boxing"
        case .special: "sparkles"
        case .status: "circle.dashed"
        }
    }

    static func detail(_ move: Move) -> String {
        "\(PokeTypeLabel.japaneseName(for: move.type))、\(MoveCategoryLabel.japaneseName(for: move.category))、威力\(move.power)"
    }
}
