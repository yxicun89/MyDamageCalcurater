// CalcViewModel+AttackerStats: 攻撃側の「攻撃」「特攻」2ブロック入力の操作と派生値(ADR-0518)。
//
// - 値が変わる操作は `beginInput()` → 入力の更新 → `recalculate(token:)` の形(calcBulk をちょうど1回)。
//   SP が不正・性格が解決できないときは計算せず、古い行・古い応答も出さない(`recalculate` が判定する)。
// - どの操作も構築の選択を外す(構築から来た特性も外す)。
// - 要求の組み立ては `AttackerStatRules.resolve`。構築の個体を呼んでいる間(`.team`)はその個体の値のまま。

extension CalcViewModel {
    /// 選んだ技が使うブロック(物理・変化 = 攻撃、特殊 = 特攻)。技が無ければ nil。
    public var usedAttackStat: AttackStat? {
        AttackerStatRules.attackStat(for: selectedMove?.category)
    }

    /// `stat` のブロックの値と一致するプリセット(カスタムなら nil)。
    public func attackerPreset(for stat: AttackStat) -> AttackerPreset? {
        AttackerStatRules.matchingPreset(attackerStatInputs[stat])
    }

    /// `stat` のブロックがどのプリセットとも一致しない(「カスタム」の印を出す)。
    public func isCustom(_ stat: AttackStat) -> Bool {
        attackerPreset(for: stat) == nil
    }

    /// 要求にできない理由(SP の不正・性格の解決失敗)。構築の個体を呼んでいる間は常に空
    /// (入力欄の値を使わないため)。選んだ技が無いときは性格の解決を見ない(SP の不正だけ)。
    public var attackerStatIssues: [AttackerStatIssue] {
        guard attackerBuildSource.teamSelection == nil else { return [] }
        let category = selectedMove?.category
        let resolution = AttackerStatRules.resolve(
            inputs: attackerStatInputs, natures: natureOptions, category: category ?? .physical)
        guard case .invalid(let issues) = resolution else { return [] }
        return category == nil ? issues.filter { $0 != .nature } : issues
    }

    /// `stat` のブロックの SP の入力が不正か(`attackerStatIssues` に `.sp(stat)` があるか。構築の個体を呼んでいる間は false)。
    public func isSPInvalid(_ stat: AttackStat) -> Bool {
        attackerStatIssues.contains(.sp(stat))
    }

    /// `stat` のブロックを `modifier` にできるか(もう一方が同じ向きなら false。補正なしは常に true)。
    public func isModifierSelectable(_ modifier: NatureChoice, for stat: AttackStat) -> Bool {
        AttackerStatRules.isModifierSelectable(inputs: attackerStatInputs, stat: stat, modifier: modifier)
    }

    /// `stat` のブロックにプリセットを入れられるか(もう一方が上昇のときの「特化」は false)。
    public func isPresetSelectable(_ preset: AttackerPreset, for stat: AttackStat) -> Bool {
        AttackerStatRules.isPresetSelectable(inputs: attackerStatInputs, stat: stat, preset: preset)
    }

    /// SP の文字列を入れる。文字列が変わらなければ何もしない(計算もしない)。変わったときは、入力が
    /// 要求にできるなら calcBulk 1回、できないなら計算せず結果の行を消す(`error` は立てない。理由は
    /// `isSPInvalid` で画面に出す)。不正な文字列も丸めずそのまま保つ。
    public func setAttackerSPText(_ text: String, for stat: AttackStat) async {
        guard text != attackerStatInputs[stat].spText else { return }
        let token = beginInput()
        releaseTeamSelection()
        attackerStatInputs[stat].spText = text
        await recalculate(token: token)
    }

    /// 性格補正を入れる。いまと同じ、または `isModifierSelectable` が false なら何もしない(計算もしない)。
    /// 数値は変えない。
    public func setAttackerNatureModifier(_ modifier: NatureChoice, for stat: AttackStat) async {
        guard modifier != attackerStatInputs[stat].modifier, isModifierSelectable(modifier, for: stat) else { return }
        let token = beginInput()
        releaseTeamSelection()
        attackerStatInputs[stat].modifier = modifier
        await recalculate(token: token)
    }

    /// 選んだ技が使う側のブロックにプリセットの値を入れる(ピルの行。技が無ければ攻撃)。
    public func selectAttackerPreset(_ preset: AttackerPreset) async {
        await selectAttackerPreset(preset, for: usedAttackStat ?? .atk)
    }

    /// `stat` のブロックにプリセットの値(`AttackerStatRules.presetInput`)を入れ、calcBulk 1回。
    /// `isPresetSelectable` が false なら何もしない(計算もしない)。もう一方のブロックは変えない。
    /// 同じ値のプリセットを押し直しても計算し直す(従来のピルの挙動のまま)。
    public func selectAttackerPreset(_ preset: AttackerPreset, for stat: AttackStat) async {
        guard isPresetSelectable(preset, for: stat) else { return }
        let value = AttackerStatRules.presetInput(preset)
        let token = beginInput()
        releaseTeamSelection()
        attackerStatInputs[stat] = value
        await recalculate(token: token)
    }

    /// 選択中の技が変化技(UI からは選べない安全網。保存データ・将来の入力経路)。true のとき計算要求を送らない。
    public var isStatusMoveSelected: Bool {
        selectedMove.map(CalcMoveRules.isStatusMove) ?? false
    }

    /// 攻撃側の種族がダメージを与える技を1つも覚えない(技欄は空、`AttackerStatLabels.noDamagingMovesNotice` を出し、計算しない)。
    /// 攻撃側の種族と learnset が読めた後だけ true になりうる(読み込み前・失敗時は false)。
    public var hasNoDamagingMoves: Bool {
        noDamagingMoves
    }

    /// 構築の個体の選択を外す(構築から来た特性も外す。プリセット同士の切り替えなら利用者が選んだ特性は残す)。
    private func releaseTeamSelection() {
        guard attackerBuildSource.teamSelection != nil else { return }
        attackerAbilityId = nil
        attackerBuildSource = .preset(AttackerPreset.defaultPreset)
    }
}
