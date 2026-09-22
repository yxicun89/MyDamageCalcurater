// TeamMemberConverter: `TeamMember` → `Individual` の下ごしらえ(P6-2c)。
//
// requirements.md「自分側のプリセット: 構築から個体を呼び出す」に備えた純粋関数。
// **この変換関数を `CalcViewModel` / `ReverseViewModel` の「構築から呼び出す」ボタンとして実際に
// 配線するのはこのタスク(P6-2c)の範囲外**(ADR-0501「P6-2c」4章)。配線は別タスクとして
// `docs/plan.md` の follow-up に残す。

/// `TeamMember` を計算対象の `Individual` へ変換する。
public enum TeamMemberConverter {
    /// `speciesKey` / `natureId` / `sp` / `itemId` / `abilityId` / `teraType` はそのまま写す
    /// (`Individual` も ID をそのまま運ぶ値型なので、マスタ照合はここでは行わない)。
    /// `moveId` は `member.moveIds` から選ぶ(下記 `selectMoveId` 参照)。
    public static func makeIndividual(from member: TeamMember, moves: [Move]) -> Individual {
        Individual(
            speciesKey: member.speciesKey,
            natureId: member.natureId,
            sp: member.sp,
            abilityId: member.abilityId,
            itemId: member.itemId,
            moveId: selectMoveId(from: member.moveIds, moves: moves),
            teraType: member.teraType
        )
    }

    /// `moveIds` から「最初の変化技でない技(`moves` で判定)、無ければ `moveIds` の最初、空なら nil」を
    /// 選ぶ(`CalcViewModel.reselectMove` の既定技の選び方と同じ規則)。`moves` に無い ID は変化技かどうか
    /// 判定できないのでダメージ技扱いにせず飛ばす(全部そうなら `moveIds` の最初にフォールバック)。
    private static func selectMoveId(from moveIds: [String], moves: [Move]) -> String? {
        guard !moveIds.isEmpty else { return nil }
        let firstDamaging = moveIds.first { id in
            guard let move = moves.first(where: { $0.id == id }) else { return false }
            return move.category != .status
        }
        return firstDamaging ?? moveIds.first
    }
}
