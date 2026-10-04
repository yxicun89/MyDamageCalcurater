import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// FavoritePinSection: 計算画面の「お気に入りに追加」(ADR-0511)。攻撃側・防御側の個体をそのまま1件ずつ追加する。
// 失敗しても計算は使える(絶対ルール5)ので、結果は1行の案内にとどめる。

struct FavoritePinSection: View {
    let calc: CalcViewModel
    let pin: FavoritePinViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            pinButton(FavoritesLabels.pinAttackerButton, identifier: "pinAttackerFavoriteButton") {
                try calc.attackerIndividualForFavorite()
            }
            pinButton(FavoritesLabels.pinDefenderButton, identifier: "pinDefenderFavoriteButton") {
                try calc.defenderIndividualForFavorite()
            }
            if let text = statusText {
                Text(text)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(isFailure ? ColorToken.danger.color : ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("favoritePinStatus")
            }
        }
        // 表示中の個体が変わったら、前の追加結果を消す。
        .onChange(of: calc.attackerSpeciesKey) { pin.reset() }
        .onChange(of: calc.defenderSpeciesKey) { pin.reset() }
    }

    private func pinButton(_ title: String, identifier: String, individual: @escaping () throws -> Individual)
        -> some View
    {
        Button(title) {
            guard let target = try? individual() else { return }
            Task { await pin.pin(target) }
        }
        .buttonStyle(PillButtonStyle())
        .disabled(pin.status == .saving)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityIdentifier(identifier)
    }

    private var isFailure: Bool {
        if case .failed = pin.status { return true }
        return false
    }

    private var statusText: String? {
        switch pin.status {
        case .idle: nil
        case .saving: FavoritesLabels.saving
        case .pinned: FavoritesLabels.pinned
        case .alreadyPinned: FavoritesLabels.alreadyPinned
        case .failed(let error): error.message(for: .addFavorite)
        }
    }
}
