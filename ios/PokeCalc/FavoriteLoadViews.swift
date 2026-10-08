import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// FavoriteLoadViews: 計算画面でお気に入りを攻撃側・防御側に読み込む導線(ADR-0513)。
//
// ロジックは持たない。`FavoriteLoadPickerViewModel`・`CalcViewModel.favoriteLoadNotice`(PokeCalcCore)の状態を描き、
// 操作を `CalcViewModel.loadFavorite` につなぐだけ。取得が失敗しても計算は使える(絶対ルール5)ので、
// 失敗はシートの中の案内にとどめる。

/// 「構築から選ぶ」行の直下に置く入口2行(攻撃側・防御側)。見た目は `TeamSourceMenuRow` と同じ `glassCard`・幅いっぱい。
/// `Menu` ではなくシートで開く(お気に入りは最大 100 件)。
struct FavoriteLoadEntryRows: View {
    let onOpen: (FavoriteLoadSide) -> Void

    var body: some View {
        VStack(spacing: SpacingToken.x2) {
            entry(.attacker, identifier: "attackerFavoriteSourceButton")
            entry(.defender, identifier: "defenderFavoriteSourceButton")
        }
    }

    private func entry(_ side: FavoriteLoadSide, identifier: String) -> some View {
        Button {
            onOpen(side)
        } label: {
            HStack(spacing: SpacingToken.x2) {
                Text(FavoritesLabels.loadTitle(for: side))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                Image(systemName: "chevron.right")
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            .padding(SpacingToken.x3)
            .frame(maxWidth: .infinity)
            .glassCard(cornerRadius: RadiusToken.input)
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
    }
}

/// 読み込み結果の案内(全部読めたときは出ない)。次の入力で `CalcViewModel` が消す。
struct FavoriteLoadNoticeView: View {
    let notice: FavoriteLoadNotice

    var body: some View {
        Text(notice.text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, SpacingToken.x1)
            .accessibilityIdentifier("favoriteLoadNotice")
    }
}

/// お気に入りを選ぶシート。開くたびに作り直し、`.task` で取得を1回だけ行う。
struct FavoriteLoadSheet: View {
    let side: FavoriteLoadSide
    let onSelect: (Favorite) -> Void
    @State private var picker: FavoriteLoadPickerViewModel
    @Environment(\.dismiss) private var dismiss

    init(
        side: FavoriteLoadSide, service: (any FavoritesService)?, resolver: any PokeCalcService,
        onSelect: @escaping (Favorite) -> Void
    ) {
        self.side = side
        self.onSelect = onSelect
        _picker = State(initialValue: FavoriteLoadPickerViewModel(service: service, resolver: resolver))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x3) {
                HStack(alignment: .top, spacing: SpacingToken.x3) {
                    Text(FavoritesLabels.loadTitle(for: side))
                        .font(TextStyleToken.heading.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .accessibilityAddTraits(.isHeader)
                    Button(FavoritesLabels.loadCloseButton) { dismiss() }
                        .buttonStyle(PillButtonStyle())
                        .accessibilityIdentifier("favoriteLoadClose")
                }
                Text(FavoritesLabels.loadNote(for: side))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("favoriteLoadNote")
                content
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("favoriteLoadSheet")
        .task { await picker.load() }
    }

    @ViewBuilder
    private var content: some View {
        switch picker.loadState {
        case .idle, .loading:
            ProgressView()
                .frame(maxWidth: .infinity)
                .accessibilityLabel(FavoritesLabels.loadLoading)
                .accessibilityIdentifier("favoriteLoadLoading")
        case .failed(let error):
            ErrorBannerView(message: error.message(for: .loadFavorites), identifier: "favoriteLoadError")
            Button(FavoritesLabels.retryButton) { Task { await picker.load() } }
                .buttonStyle(PillButtonStyle())
                .accessibilityIdentifier("favoriteLoadRetry")
        case .loaded:
            if picker.isEmpty {
                Text(FavoritesLabels.emptyFavorites)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("favoriteLoadEmpty")
            }
            ForEach(picker.rows) { row in
                rowButton(row)
            }
        }
    }

    private func rowButton(_ row: FavoriteRow) -> some View {
        Button {
            onSelect(row.favorite)
            dismiss()
        } label: {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                Text(row.title)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                if let subtitle = row.subtitle {
                    Text(subtitle)
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .fixedSize(horizontal: false, vertical: true)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(SpacingToken.x3)
            .glassCard()
        }
        .buttonStyle(.plain)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel([row.title, row.subtitle].compactMap { $0 }.joined(separator: " "))
        .accessibilityAddTraits(.isButton)
        .accessibilityIdentifier("favoriteLoadRow-\(row.id)")
    }
}

/// `.sheet(item:)` 用(側ごとに別のシートを開く)。
struct FavoriteLoadTarget: Identifiable {
    let side: FavoriteLoadSide
    var id: String { side == .attacker ? "attacker" : "defender" }
}
