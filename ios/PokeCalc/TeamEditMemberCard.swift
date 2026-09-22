import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamEditMemberCard: 構築編集画面のメンバー1体分のカード(P6-2c・ADR-0501「P6-2c」5章)。
// `CalcScreenCards.swift` / `ReverseScreenCards.swift` の部品(`SpeciesHeaderMenuLabel` /
// `MenuLabelChip`)を再利用する。

/// メンバー1体のカード: 種族・ニックネーム・持ち物・特性・性格・テラスタイプ・技(最大4)・SP・エラー。
struct MemberCardView: View {
    let viewModel: TeamEditViewModel
    let member: TeamMember
    /// ニックネーム欄はローカルの `@State` を真とし、`viewModel.setMemberNickname(_:)` へ同期的に
    /// 反映する(`TeamEditView.nameField` の理由と同じ: ViewModel 側は空白除去・空文字→nil の
    /// 正規化を行うため、入力中の生の文字列をそのまま TextField に戻すとカーソル位置が揺れうる)。
    /// `ForEach(id: \.id)` がメンバーごとに安定したインスタンスを作るので、この `@State` は
    /// 選択中のメンバーと1対1で対応する。
    @State private var nicknameText: String

    init(viewModel: TeamEditViewModel, member: TeamMember) {
        self.viewModel = viewModel
        self.member = member
        _nicknameText = State(initialValue: member.nickname ?? "")
    }

    private var species: SpeciesSummary? {
        viewModel.speciesOptions.first(where: { $0.key == member.speciesKey })
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            header
            nicknameField
            Divider().overlay(ColorToken.borderHairline.color)
            selectorGrid
            movesSection
            spSection
            if let memberError = viewModel.memberErrors[member.id] {
                Text(memberError.uiMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("memberError-\(member.id)")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("memberCard-\(member.id)")
    }

    private var header: some View {
        HStack(spacing: SpacingToken.x2) {
            Menu {
                ForEach(viewModel.speciesOptions, id: \.key) { option in
                    Button(option.nameJa) {
                        Task { await viewModel.setMemberSpecies(id: member.id, speciesKey: option.key) }
                    }
                }
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("memberSpeciesPicker-\(member.id)")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")

            Button {
                viewModel.removeMember(id: member.id)
            } label: {
                Image(systemName: "trash")
                    .foregroundStyle(ColorToken.danger.color)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("memberDelete-\(member.id)")
            .accessibilityLabel("このメンバーを削除")
        }
    }

    /// ニックネーム(任意項目。空なら未設定)。ADR-0501「P6-2c」§7 の未確認事項として保留していたが、
    /// `TeamMember.nickname` を使わないまま残すと coding-rules §3「使われていない機構を作らない」に
    /// 触れるため配線した(critic 指摘)。
    private var nicknameField: some View {
        TextField(
            "ニックネーム(任意)",
            text: Binding(
                get: { nicknameText },
                set: { newValue in
                    nicknameText = newValue
                    viewModel.setMemberNickname(id: member.id, nickname: newValue)
                }
            )
        )
        .font(TextStyleToken.body.font)
        .foregroundStyle(ColorToken.textPrimary.color)
        .padding(.horizontal, SpacingToken.x2)
        .padding(.vertical, SpacingToken.x1)
        .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
        .accessibilityIdentifier("memberNickname-\(member.id)")
    }

    /// 持ち物・特性・性格・テラスタイプのセレクタ。design.md のチップ(角丸999)に揃える。
    private var selectorGrid: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            itemPicker
            abilityPicker
            naturePicker
            teraPicker
        }
    }

    private var itemPicker: some View {
        Menu {
            Button(BulkRowDisplay.itemLabel(itemId: nil, items: viewModel.itemOptions)) {
                viewModel.setMemberItem(id: member.id, itemId: nil)
            }
            ForEach(viewModel.itemOptions, id: \.id) { item in
                Button(item.nameJa) {
                    viewModel.setMemberItem(id: member.id, itemId: item.id)
                }
            }
        } label: {
            MenuLabelChip(text: "持ち物: " + BulkRowDisplay.itemLabel(itemId: member.itemId, items: viewModel.itemOptions))
        }
        .accessibilityIdentifier("memberItemPicker-\(member.id)")
    }

    private var abilityOptions: [Ability] {
        viewModel.abilityOptionsByMember[member.id] ?? []
    }

    private var abilityLabel: String {
        abilityOptions.first(where: { $0.id == member.abilityId })?.nameJa ?? "-"
    }

    private var abilityPicker: some View {
        Menu {
            ForEach(abilityOptions, id: \.id) { ability in
                Button(ability.nameJa) {
                    viewModel.setMemberAbility(id: member.id, abilityId: ability.id)
                }
            }
        } label: {
            MenuLabelChip(text: "特性: " + abilityLabel)
        }
        .accessibilityIdentifier("memberAbilityPicker-\(member.id)")
    }

    private var natureLabel: String {
        viewModel.natureOptions.first(where: { $0.id == member.natureId })?.nameJa ?? "-"
    }

    private var naturePicker: some View {
        Menu {
            ForEach(viewModel.natureOptions, id: \.id) { nature in
                Button(nature.nameJa) {
                    viewModel.setMemberNature(id: member.id, natureId: nature.id)
                }
            }
        } label: {
            MenuLabelChip(text: "性格: " + natureLabel)
        }
        .accessibilityIdentifier("memberNaturePicker-\(member.id)")
    }

    private var teraLabel: String {
        member.teraType.map(PokeTypeLabel.japaneseName) ?? "なし"
    }

    private var teraPicker: some View {
        Menu {
            Button("なし") {
                viewModel.setMemberTeraType(id: member.id, teraType: nil)
            }
            ForEach(PokeType.allCases, id: \.self) { type in
                Button(PokeTypeLabel.japaneseName(for: type)) {
                    viewModel.setMemberTeraType(id: member.id, teraType: type)
                }
            }
        } label: {
            MenuLabelChip(text: "テラスタイプ: " + teraLabel)
        }
        .accessibilityIdentifier("memberTeraPicker-\(member.id)")
    }

    // MARK: - 技(最大4スロット)

    private var moveOptions: [Move] {
        viewModel.moveOptionsByMember[member.id] ?? []
    }

    private var movesSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text("わざ")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            ForEach(0..<TeamLimits.maxMovesPerMember, id: \.self) { index in
                moveSlot(index: index)
            }
        }
    }

    private func moveSlot(index: Int) -> some View {
        let selectedID = member.moveIds.indices.contains(index) ? member.moveIds[index] : nil
        let selectedMove = selectedID.flatMap { id in moveOptions.first(where: { $0.id == id }) }
        let remainingOptions = moveOptions.filter { !member.moveIds.contains($0.id) }
        return Menu {
            if selectedID != nil {
                Button("外す", role: .destructive) {
                    viewModel.removeMove(id: member.id, at: index)
                }
            }
            ForEach(remainingOptions, id: \.id) { move in
                Button(move.nameJa) {
                    viewModel.addMove(id: member.id, moveId: move.id)
                }
            }
        } label: {
            MenuLabelChip(text: selectedMove?.nameJa ?? "技を選択")
        }
        .accessibilityIdentifier("memberMoveSlot-\(member.id)-\(index)")
    }

    // MARK: - SP(能力ポイント。1ステータス最大32・合計66)

    private var spSection: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text("能力ポイント(合計 \(member.sp.teamEditValues.reduce(0, +))/\(SPLimits.maxTotal))")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            ForEach(StatKey.allCases, id: \.self) { stat in
                spStepper(for: stat)
            }
        }
    }

    private func spStepper(for stat: StatKey) -> some View {
        let value = member.sp.teamEditValue(for: stat)
        return HStack(spacing: SpacingToken.x2) {
            // 「とくこう」「ぼうぎょ」など4文字のラベルがあるので、固定幅で切ると折り返る
            // (実機スクリーンショットで確認した不具合)。`.fixedSize()` で1行の自然な幅を確保し、
            // `frame(minWidth:)` は短いラベル(HP)の見た目をそろえる下限としてだけ使う。
            Text(StatKeyLabel.japaneseName(for: stat))
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .lineLimit(1)
                .fixedSize()
                .frame(minWidth: 64, alignment: .leading)
            Stepper(
                value: Binding(
                    get: { value },
                    set: { newValue in viewModel.setMemberSP(id: member.id, stat: stat, value: newValue) }
                ),
                in: 0...SPLimits.maxPerStat
            ) {
                Text("\(value)")
                    .font(TextStyleToken.body.font.monospacedDigit())
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .frame(minWidth: 28, alignment: .trailing)
            }
        }
        .accessibilityIdentifier("memberSP-\(member.id)-\(stat.rawValue)")
    }
}
