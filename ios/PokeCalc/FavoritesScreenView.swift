import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// FavoritesScreenView: お気に入り・計算履歴画面(ADR-0511)。
//
// ロジックは持たない。`FavoritesViewModel`・`CalcHistoryViewModel`・`OpponentHistoryViewModel`(PokeCalcCore)の状態を描き、
// 操作を async メソッドへつなぐだけ。ここが失敗しても計算は使える(絶対ルール5)ので、失敗は画面の中の案内にとどめる。
// 計算履歴の行をタップすると、その行の計算を計算画面に復元して開く(ADR-0519)。

struct FavoritesScreenView: View {
    @State private var favorites: FavoritesViewModel
    @State private var calcHistory: CalcHistoryViewModel
    @State private var history: OpponentHistoryViewModel
    @State private var restoreTarget: CalcHistoryRestoreTarget?
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    /// 履歴の行の計算を復元した計算画面を作る(この画面は計算画面の部品を知らない)。
    private let calcScreen: (CalcHistoryCalc) -> AnyView

    init(
        favoritesService: (any FavoritesService)?, calcHistoryService: (any CalcHistoryService)?,
        frequentOpponentsService: any FrequentOpponentsService, resolver: any PokeCalcService,
        calcScreen: @escaping (CalcHistoryCalc) -> AnyView
    ) {
        _favorites = State(initialValue: FavoritesViewModel(service: favoritesService, resolver: resolver))
        _calcHistory = State(initialValue: CalcHistoryViewModel(service: calcHistoryService, resolver: resolver))
        _history = State(
            initialValue: OpponentHistoryViewModel(service: frequentOpponentsService, resolver: resolver))
        self.calcScreen = calcScreen
    }

    private var isAccessibilitySize: Bool { dynamicTypeSize >= .accessibility1 }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x6) {
                favoritesSection
                calcHistorySection
                historySection
            }
            .padding(SpacingToken.x4)
        }
        .popScreenBackground()
        .accessibilityIdentifier("favoritesScreen")
        .toolbar {
            ToolbarItem(placement: .principal) {
                Text(FavoritesLabels.screenTitle)
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
        }
        .navigationDestination(item: $restoreTarget) { target in
            calcScreen(target.calc)
        }
        .task { await favorites.load() }
        .task { await calcHistory.load() }
        .task { await history.load() }
    }

    // MARK: - お気に入り

    private var favoritesSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            sectionHeading(FavoritesLabels.favoritesSectionTitle, systemImage: PopSymbol.favorites)
            // 読み込み中は、見出しだけだと子が1つになり `.contain` の識別子が見出しに畳まれる(P6-25 で判明)ので、
            // 読み込み中の目印を常に子に足す。
            if favorites.loadState == .idle || favorites.loadState == .loading {
                ProgressView().popNotice(.loading).accessibilityIdentifier("favoritesLoading")
            }
            if let actionError = favorites.actionError {
                ErrorBannerView(message: actionError.message(for: .removeFavorite), identifier: "favoritesActionError")
            }
            if case .failed(let error) = favorites.loadState {
                ErrorBannerView(message: error.message(for: .loadFavorites), identifier: "favoritesError")
                retryButton(identifier: "favoritesRetryButton") { await favorites.load() }
            }
            if favorites.loadState == .loaded && favorites.items.isEmpty {
                PopNoticeView(kind: .empty, message: FavoritesLabels.emptyFavorites, identifier: "favoritesEmpty")
            }
            ForEach(Array(favorites.items.enumerated()), id: \.element.id) { index, row in
                favoriteRow(row, index: index)
            }
            if favorites.isAtLimit {
                captionText(FavoritesLabels.limitNote)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("favoritesSection")
    }

    private func favoriteRow(_ row: FavoriteRow, index: Int) -> some View {
        let layout = isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x2))
            : AnyLayout(HStackLayout(alignment: .center, spacing: SpacingToken.x3))
        return layout {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(row.title)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                if let subtitle = row.subtitle {
                    captionText(subtitle)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            Button {
                Task { await favorites.remove(id: row.id) }
            } label: {
                PopLabel(title: FavoritesLabels.removeButton, systemImage: PopSymbol.delete)
            }
            .buttonStyle(PillButtonStyle(kind: .danger))
            .disabled(favorites.removingIDs.contains(row.id))
            .accessibilityLabel("\(FavoritesLabels.removeButton) \(row.title)")
            .accessibilityIdentifier("favoriteDeleteButton-\(row.id)")
        }
        .padding(SpacingToken.x3)
        .popRow(index: index)
        .accessibilityElement(children: .contain)
        .accessibilityLabel([row.title, row.subtitle].compactMap { $0 }.joined(separator: " "))
        .accessibilityIdentifier("favoriteRow-\(row.id)")
    }

    // MARK: - 計算履歴(新しい順。ADR-0230・ADR-0519)

    private var calcHistorySection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            sectionHeading(CalcHistoryLabels.sectionTitle, systemImage: PopSymbol.history)
            // 読み込み中は見出しだけだと子が1つになり `.contain` の識別子が見出しに畳まれるので、目印を常に子に足す。
            if (calcHistory.loadState == .idle || calcHistory.loadState == .loading) && calcHistory.rows.isEmpty {
                ProgressView().accessibilityIdentifier("calcHistoryLoading")
            }
            if case .failed(let error) = calcHistory.loadState {
                ErrorBannerView(message: error.message(for: .loadHistory), identifier: "calcHistoryError")
                retryButton(identifier: "calcHistoryRetryButton") { await calcHistory.load() }
            }
            if calcHistory.isEmpty {
                PopNoticeView(kind: .empty, message: CalcHistoryLabels.emptyHistory, identifier: "calcHistoryEmpty")
            }
            ForEach(Array(calcHistory.rows.enumerated()), id: \.element.id) { index, row in
                calcHistoryRow(row, index: index)
            }
            if let moreError = calcHistory.loadMoreError {
                ErrorBannerView(message: moreError.message(for: .loadHistory), identifier: "calcHistoryMoreError")
            }
            if calcHistory.hasMore {
                if calcHistory.isLoadingMore {
                    ProgressView().accessibilityIdentifier("calcHistoryLoadingMore")
                } else {
                    Button { Task { await calcHistory.loadMore() } } label: { PopLabel(title: CalcHistoryLabels.loadMoreButton, systemImage: PopSymbol.reload) }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("calcHistoryLoadMoreButton")
                }
            }
            captionText(CalcHistoryLabels.note)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("calcHistorySection")
    }

    private func calcHistoryRow(_ row: CalcHistoryRow, index: Int) -> some View {
        let dateText = row.occurredText(now: Date(), calendar: .current)
        let detailLayout = isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x1))
            : AnyLayout(HStackLayout(alignment: .firstTextBaseline, spacing: SpacingToken.x3))
        return Button {
            restoreTarget = CalcHistoryRestoreTarget(id: row.id, calc: row.entry.calc)
        } label: {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(row.title)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                captionText(row.moveText)
                detailLayout {
                    Text(row.percentText)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .fixedSize(horizontal: false, vertical: true)
                    captionText(dateText)
                }
            }
            .padding(SpacingToken.x3)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .buttonStyle(PopRowButtonStyle(index: index))
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(row.title) \(row.moveText) \(row.percentText) \(dateText)")
        .accessibilityHint(CalcHistoryLabels.rowHint)
        .accessibilityAddTraits(.isButton)
        .accessibilityIdentifier("calcHistoryRow-\(row.id)")
    }

    // MARK: - よく計算する相手

    private var historySection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            sectionHeading(FavoritesLabels.historySectionTitle, systemImage: PopSymbol.calc)
            if case .failed(let error) = history.loadState {
                ErrorBannerView(message: error.message(for: .loadHistory), identifier: "opponentHistoryError")
                retryButton(identifier: "opponentHistoryRetryButton") { await history.load() }
            }
            if history.loadState == .loaded && history.items.isEmpty {
                PopNoticeView(kind: .empty, message: FavoritesLabels.emptyHistory, identifier: "opponentHistoryEmpty")
            }
            ForEach(Array(history.items.enumerated()), id: \.element.id) { index, row in
                historyRow(row, index: index)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("opponentHistorySection")
    }

    private func historyRow(_ row: OpponentHistoryRow, index: Int) -> some View {
        let countText = OpponentHistoryLabels.countText(row.count)
        let lastText = OpponentHistoryLabels.lastCalculatedText(
            row.lastCalculatedAt, now: Date(), calendar: .current)
        let layout = isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x1))
            : AnyLayout(HStackLayout(alignment: .center, spacing: SpacingToken.x3))
        return layout {
            Text(row.title)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
            VStack(alignment: isAccessibilitySize ? .leading : .trailing, spacing: SpacingToken.x1) {
                Text(countText)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                captionText(lastText)
            }
        }
        .padding(SpacingToken.x3)
        .popRow(index: index)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(row.title) \(countText) \(lastText)")
        .accessibilityIdentifier("opponentHistoryRow-\(row.speciesKey)")
    }

    // MARK: - 部品

    private func sectionHeading(_ text: String, systemImage: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            PopIcon(systemImage).foregroundStyle(ColorToken.brandPrimary.color)
            Text(text)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
        }
    }

    private func captionText(_ text: String) -> some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func retryButton(identifier: String, action: @escaping () async -> Void) -> some View {
        Button { Task { await action() } } label: { PopLabel(title: FavoritesLabels.retryButton, systemImage: PopSymbol.reload) }
            .buttonStyle(PillButtonStyle())
            .accessibilityIdentifier(identifier)
    }
}

/// 履歴の行から計算画面を開くための遷移先(`navigationDestination(item:)` 用)。同一性は行の ID。
struct CalcHistoryRestoreTarget: Identifiable, Hashable {
    let id: String
    let calc: CalcHistoryCalc

    static func == (lhs: CalcHistoryRestoreTarget, rhs: CalcHistoryRestoreTarget) -> Bool { lhs.id == rhs.id }
    func hash(into hasher: inout Hasher) { hasher.combine(id) }
}
