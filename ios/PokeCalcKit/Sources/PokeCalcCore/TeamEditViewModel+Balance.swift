// 構築編集の下書きを「タイプバランス」の入力にする口(ADR-0421 §3・ADR-0422 §2・ADR-0532)。
//
// 構築側はバランスの内部(呼び出し・待ち・古い応答の無視)を知らない。渡すのは下書きのメンバーだけ
// (`BalanceViewModel.setParty`)。おすすめのタイルを押したときは既存の `addMember(speciesKey:)`(最初の空き枠に入る)を使う。

extension TeamEditViewModel {
    /// 種族を選んだ枠のメンバーを枠の順に(空き枠は入れない)。保存前の下書きのまま。
    public var balanceParty: [TeamMember] {
        team.members.filter { !$0.speciesKey.isEmpty }
    }

    /// 空き枠が 1 つでもあるか(おすすめのタイルを押せるか)。
    public var hasEmptySlot: Bool {
        slotIDs.contains { $0 == nil }
    }
}
