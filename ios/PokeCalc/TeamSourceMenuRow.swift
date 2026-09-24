import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamSourceMenuRow: 「構築から選ぶ」の入口(P6-2d・ADR-0501「P6-2d」7章)。
//
// 計算画面(自分側)・逆算画面(いま表示している側)の両方で使う共通部品。既存の3択ピル行の
// 直下に置く独立した1行(4つ目のピルにしない。ADR 5章「判断」)。見た目は `CalcScreenView.moveSelector`
// と同じ部品(`Menu` + `glassCard`)を再利用し、新しい視覚言語は作らない。

/// 「構築から選ぶ」の入口の行。`identifierPrefix` は `attackerTeam` / `reverseTeam`
/// (ADR-0501「P6-2d」7章の identifier 契約: `<prefix>SourceButton` / `<prefix>Member-<teamID>-<memberID>` /
/// `<prefix>EmptyMessage`)。
struct TeamSourceMenuRow: View {
    let teamOptions: [TeamPickerGroup]
    let selection: TeamIndividualSelection?
    let identifierPrefix: String
    /// 同期クロージャ(issue #113。ADR-0501「issue #113 の受け入れ条件(iOS 側)」8章):
    /// Task の起動・保持は呼び出し側の画面が `scheduleLatest` で行う。
    let onSelect: (String, String) -> Void

    /// 選べる個体が1体も無い(構築が無い・`teamStore` が無い・読み込み失敗)。7章「判断」:
    /// 行ごと消さず無効にして案内文を添える。
    private var isEmpty: Bool { teamOptions.isEmpty }

    /// 未選択は `TeamLoadLabels.entryTitle`、選択中は「<構築名> · <表示名>」(7章の identifier 表)。
    private var labelText: String {
        guard let selection else { return TeamLoadLabels.entryTitle }
        guard let teamName = teamOptions.first(where: { $0.id == selection.teamID })?.name else {
            // 呼び出した構築がいまの `teamOptions` に無い(例: 直後に削除された)ときは、名前の
            // 結合をあきらめて表示名だけにする(画面を止めない。coding-rules §3)。
            return selection.displayName
        }
        return "\(teamName) · \(selection.displayName)"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Menu {
                ForEach(teamOptions) { group in
                    // 構築ごとにセクション見出し = 構築名(7章の identifier 表)。`Section` の中の
                    // `Button` も `accessibilityIdentifier` は UIKit メニューへ渡らない(入れ子 `Menu`
                    // でも同じ制約なので回避にならない。critic 指摘を受けて確認し、`Section` に戻した。
                    // XCUITest はラベル文字列で辿る)。この形ならメニューを開いた直後に全メンバーが
                    // 一覧され、構築ごとの入れ子をタップする手間が無い。
                    Section(group.name) {
                        ForEach(group.members) { member in
                            Button(member.displayName) {
                                onSelect(group.id, member.id)
                            }
                            .accessibilityIdentifier("\(identifierPrefix)Member-\(group.id)-\(member.id)")
                        }
                    }
                }
            } label: {
                HStack {
                    Text(labelText)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .lineLimit(1)
                        .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                    Spacer()
                    Image(systemName: "chevron.up.chevron.down")
                        .foregroundStyle(ColorToken.textSecondary.color)
                }
                .padding(SpacingToken.x3)
                .frame(maxWidth: .infinity)
                .glassCard(cornerRadius: RadiusToken.input)
            }
            .disabled(isEmpty)
            .accessibilityIdentifier("\(identifierPrefix)SourceButton")

            if isEmpty {
                Text(TeamLoadLabels.empty)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .padding(.horizontal, SpacingToken.x1)
                    .accessibilityIdentifier("\(identifierPrefix)EmptyMessage")
            }
        }
    }
}
