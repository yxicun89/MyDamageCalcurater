import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamEditMemberCard: 構築編集画面のメンバー1体分のカード(P6-2c・ADR-0501「P6-2c」5章)。
// `CalcScreenCards.swift` / `ReverseScreenCards.swift` の部品(`SpeciesHeaderMenuLabel` /
// `MenuLabelChip`)を再利用する。F-08(ADR-0522)で、外す・入れ替え・書き出しは枠(`TeamEditView`)側に移した。

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
    @State private var isSpeciesSearchPresented = false
    /// 非 nil のとき、その index の技スロットの検索シートが開いている(issue #68: シートは画面ごとに
    /// 1つだが、メンバー内では「いま編集中のスロット」を覚える必要がある。ADR-0501「issue #68」8章)。
    @State private var moveSearchSlot: MoveSlotTarget?

    init(viewModel: TeamEditViewModel, member: TeamMember) {
        self.viewModel = viewModel
        self.member = member
        _nicknameText = State(initialValue: member.nickname ?? "")
    }

    private var species: SpeciesSummary? {
        viewModel.speciesSummary(forKey: member.speciesKey)
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
                PopNoticeView(kind: .error, message: memberError.uiMessage, identifier: "memberError-\(member.id)")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(typeID: species?.types.first?.rawValue)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("memberCard-\(member.id)")
    }

    private var header: some View {
        HStack(spacing: SpacingToken.x2) {
            // issue #68: `Menu` ではなく検索シートで選ぶ(`CalcScreenCards` と同じ理由)。
            Button {
                isSpeciesSearchPresented = true
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("memberSpeciesPicker-\(member.id)")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel) { option in
                    Task { await viewModel.setMemberSpecies(id: member.id, speciesKey: option.key) }
                }
            }

        }
    }

    /// ニックネーム(任意項目。空なら未設定)。ADR-0501「P6-2c」§7 の未確認事項として保留していたが、
    /// `TeamMember.nickname` を使わないまま残すと coding-rules §3「使われていない機構を作らない」に
    /// 触れるため配線した(critic 指摘)。
    private var nicknameField: some View {
        TextField(
            TeamLabels.nicknameField,
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
        .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
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
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            let lock = viewModel.itemLock(forMember: member.id)
            Menu {
                Button(ItemDisplayName.noItemLabel) {
                    viewModel.setMemberItem(id: member.id, itemId: nil)
                }
                ForEach(viewModel.itemOptions(forMember: member.id), id: \.id) { item in
                    Button(item.nameJa) {
                        viewModel.setMemberItem(id: member.id, itemId: item.id)
                    }
                }
            } label: {
                MenuLabelChip(text: MegaItemText.fixedItemName(viewModel.itemLabel(for: member.itemId)))
            }
            // メガ種族は持ち物がメガストーンに固定される(操作不可。ADR-0509 §8)。
            .disabled(lock.disablesItemField)
            .accessibilityHint(lockReason(for: lock) ?? "")
            .accessibilityIdentifier("memberItemPicker-\(member.id)")
            if let reason = lockReason(for: lock) {
                Text(reason)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("memberItemLockReason-\(member.id)")
            }
            // 保存データを直したときの通知(保存すると反映される。ADR-0509 §4)。
            if let notice = viewModel.itemNotice(forMember: member.id) {
                Text(notice)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("memberItemNotice-\(member.id)")
            }
            // 持ち物の一覧が上限に達していても黙って切り捨てない(ADR-0501「issue #68 の残り」6章)。
            if viewModel.itemOptionsReachedLimit {
                Text(MasterSearchLabels.itemsTruncated)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("memberItemLimitHint-\(member.id)")
            }
        }
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
            Text(TeamLabels.movesHeading)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityAddTraits(.isHeader)
                .accessibilityIdentifier("memberMovesHeading-\(member.id)")
            ForEach(0..<TeamLimits.maxMovesPerMember, id: \.self) { index in
                moveSlot(index: index)
            }
        }
        // シートは1つだけ持ち、`moveSearchSlot`(いま編集中のスロット index)で内容を出し分ける
        // (issue #68・8章「シートは画面ごとに1つ」。4つのスロットボタンそれぞれに `.sheet` を付けると
        // 同じ状態を4つのシートが同時に監視してしまうため、ここにまとめる)。
        .sheet(item: $moveSearchSlot) { target in
            let selectedID = member.moveIds.indices.contains(target.index) ? member.moveIds[target.index] : nil
            // 他の枠で選んだ技は一覧に残して押せなくする(自分の枠の技は選べる。Web の技ピッカーと同じ)。
            let takenElsewhere = Set(member.moveIds).subtracting(selectedID.map { [$0] } ?? [])
            MoveSearchSheet(
                viewModel: viewModel, options: moveOptions,
                onSelect: { move in
                    // 自分の枠の技を選び直したときは何も変えない(重複の警告を出さない)。
                    if move.id != selectedID { viewModel.addMove(id: member.id, moveId: move.id) }
                },
                // 「(なし)」は空の枠でも出す(選ぶと枠が空く。空の枠では何も変わらない)。
                removeAction: {
                    if selectedID != nil { viewModel.removeMove(id: member.id, at: target.index) }
                },
                noneLabel: TeamLabels.moveNone, disabledIDs: takenElsewhere, groupsByType: true
            )
        }
    }

    /// issue #68: `Menu` ではなく検索シートで選ぶ(`CalcScreenView.moveSelector` と同じ理由)。
    /// 選択中の技の名前は `viewModel.move(forID:)`(一度でも見た技の辞書。先頭ページ・技検索の結果・
    /// `move(id:)` の応答が合流する)から引く(ADR-0501「issue #68 の残り」5章)。それでも解決できない
    /// (404・通信失敗)ときは、`BulkRowDisplay.itemLabel` の「マスタに無い ID はそのまま出す」規則に
    /// そろえて ID を出す(「技を選択」に見えてしまうと選択済みであることが伝わらないため)。
    private func moveSlot(index: Int) -> some View {
        let selectedID = member.moveIds.indices.contains(index) ? member.moveIds[index] : nil
        let label = selectedID.map { id in viewModel.move(forID: id)?.nameJa ?? id } ?? TeamLabels.moveNone
        return Button {
            moveSearchSlot = MoveSlotTarget(index: index)
        } label: {
            MenuLabelChip(text: label)
        }
        .accessibilityIdentifier("memberMoveSlot-\(member.id)-\(index)")
    }

    // MARK: - SP(能力ポイント。1ステータス最大32・合計66)

    private var spSection: some View {
        let total = member.sp.teamEditValues.reduce(0, +)
        return VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Text(TeamLabels.spSummary(total: total))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("memberSPSummary-\(member.id)")
            SPTotalBar(fraction: TeamSPControl.fillFraction(total: total))
                .accessibilityIdentifier("memberSPBar-\(member.id)")
            ForEach(StatKey.allCases, id: \.self) { stat in
                SPStatRow(
                    statName: StatKeyLabel.japaneseName(for: stat), value: member.sp.teamEditValue(for: stat), total: total,
                    memberID: member.id, statID: stat.rawValue
                ) { newValue in viewModel.setMemberSP(id: member.id, stat: stat, value: newValue) }
            }
        }
    }
}

/// SP の合計のバー(装飾。数値は上の合計が持つ)。満ち具合は `TeamSPControl.fillFraction`。
private struct SPTotalBar: View {
    let fraction: Double
    private static let height: CGFloat = 8

    var body: some View {
        Capsule()
            .fill(ColorToken.tableZebra.color)
            .frame(height: Self.height)
            .overlay(alignment: .leading) {
                GeometryReader { proxy in
                    Capsule()
                        .fill(ColorToken.brandPrimary.color)
                        .frame(width: proxy.size.width * fraction)
                }
            }
            .accessibilityElement(children: .ignore)
    }
}

/// SP の 1 行: 能力名・− ボタン・数値欄・+ ボタン。数値欄は打った値をそのまま ViewModel へ渡し(範囲外・合計超えは拒否されて
/// 欄が元の値へ戻る)、ボタンは上限・下限で押せなくなる。
private struct SPStatRow: View {
    let statName: String
    let value: Int
    let total: Int
    let memberID: String
    let statID: String
    /// ViewModel へ値を渡す(受け付けたかを返す)。
    let apply: (Int) -> Bool

    @State private var text: String
    @FocusState private var isFocused: Bool
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @ScaledMetric private var fieldWidth: CGFloat = 64

    init(statName: String, value: Int, total: Int, memberID: String, statID: String, apply: @escaping (Int) -> Bool) {
        self.statName = statName
        self.value = value
        self.total = total
        self.memberID = memberID
        self.statID = statID
        self.apply = apply
        _text = State(initialValue: String(value))
    }

    var body: some View {
        let layout = dynamicTypeSize.isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: SpacingToken.x1))
            : AnyLayout(HStackLayout(alignment: .center, spacing: SpacingToken.x2))
        layout {
            Text(statName)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
            if !dynamicTypeSize.isAccessibilitySize { Spacer(minLength: 0) }
            HStack(spacing: SpacingToken.x1) {
                PopIconButton(
                    systemImage: PopSymbol.minus, label: TeamLabels.spDecrease(statName: statName),
                    identifier: "memberSPDown-\(memberID)-\(statID)", isEnabled: TeamSPControl.canDecrease(value: value)
                ) { if apply(value - 1) { text = String(value - 1) } }
                field
                PopIconButton(
                    systemImage: PopSymbol.add, label: TeamLabels.spIncrease(statName: statName),
                    identifier: "memberSPUp-\(memberID)-\(statID)",
                    isEnabled: TeamSPControl.canIncrease(value: value, total: total)
                ) { if apply(value + 1) { text = String(value + 1) } }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("memberSP-\(memberID)-\(statID)")
        // ボタンや外部の変更で値が変わったら欄に映す(打っている最中は欄を書き換えない。打った文字が失われるため)。
        .onChange(of: value) { _, newValue in
            if !isFocused { text = String(newValue) }
        }
        // 打ち終わったら、受け付けられなかった入力を元の値へ戻す。
        .onChange(of: isFocused) { _, focused in
            if !focused { text = String(value) }
        }
        .toolbar {
            ToolbarItemGroup(placement: .keyboard) {
                if isFocused {
                    Spacer()
                    Button(TeamLabels.keyboardDone) { isFocused = false }
                }
            }
        }
    }

    private var fieldIdentifier: String { "memberSPField-\(memberID)-\(statID)" }

    private var field: some View {
        TextField(
            TeamLabels.spField(statName: statName),
            text: Binding(
                get: { text },
                set: { newValue in
                    let digits = newValue.filter(\.isNumber)
                    text = digits
                    if let number = Int(digits) { _ = apply(number) }
                }
            )
        )
        .keyboardType(.numberPad)
        .focused($isFocused)
        .multilineTextAlignment(.center)
        .font(TextStyleToken.body.font.monospacedDigit())
        .foregroundStyle(ColorToken.textPrimary.color)
        .frame(width: fieldWidth, height: CalcScreenMetrics.minimumTapSide)
        .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
        .accessibilityLabel(TeamLabels.spField(statName: statName))
        .accessibilityIdentifier(fieldIdentifier)
        // 数値欄は触れたら全体を選択する(打った数字が今の値に置き換わる。カーソルが数字の手前に入って「320」にならない)。
        .onReceive(NotificationCenter.default.publisher(for: UITextField.textDidBeginEditingNotification)) { note in
            guard let field = note.object as? UITextField, field.accessibilityIdentifier == fieldIdentifier else { return }
            DispatchQueue.main.async { field.selectAll(nil) }
        }
    }
}

/// `.sheet(item:)` に渡すための、開いている技スロットの index(issue #68)。
private struct MoveSlotTarget: Identifiable, Equatable {
    let index: Int
    var id: Int { index }
}
