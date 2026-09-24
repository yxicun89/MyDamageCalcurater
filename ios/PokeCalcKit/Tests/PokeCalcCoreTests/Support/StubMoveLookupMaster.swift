import Foundation

@testable import PokeCalcCore

// MARK: - 選択中の技を ID で解決する(issue #68 の残り)ための架空マスタ

/// `StubBulkMaster`(先頭ページ `MasterSearch.pageLimit` 件 + その外)に、**先頭ページの外の技しか覚えない種族を
/// 種族一覧の先頭に置いた**サービスを作る。起動時の既定(攻撃側/自分 = 種族一覧の最初)がその種族になるので、
/// issue #68 の再現手順「検索は先頭200件の技だけを返す + learnset は201件目の技だけ」がそのまま `load()` で起きる。
///
/// 追加の技(`hiddenStatusMove(_:)` / `hiddenDamagingMove`)も技一覧の**末尾**(先頭ページの外)に置く。
/// 名前はすべて「テスト」で始める(架空データ。ADR-0002)。
enum StubMoveLookupMaster {
    /// 先頭ページの外にある変化技(`move(id:)` の上限の確認用。index ごとに別の技)。
    static func hiddenStatusMove(_ index: Int) -> Move {
        Move(id: "stub-hidden-status-\(index)", nameJa: "テストかくれへんか\(index)", type: .normal, category: .status, power: 0)
    }

    /// 先頭ページの外にある特殊技(`StubBulkMaster.hiddenMove` は物理なので分類で見分けられるように別に置く)。
    static let hiddenDamagingMove = Move(
        id: "stub-hidden-damaging", nameJa: "テストかくれとくしゅ", type: .fire, category: .special, power: 60
    )

    /// 先頭ページの外の変化技の数(`MasterSearch.maxMoveLookupsPerSelection` より1つ多い。
    /// 上限で打ち切ることを、上限ちょうどの呼び出し回数で確かめるため)。
    static var hiddenStatusCount: Int { MasterSearch.maxMoveLookupsPerSelection + 1 }

    /// learnset を指定して、種族一覧の先頭に置く種族を作る(先頭ページの中の種族とも key が重ならない)。
    static func leadSpecies(learnset: [String]) -> SpeciesDetail {
        SpeciesDetail(
            key: "8997-000", dexNo: 8997, form: 0, nameJa: "テストせんとうしゅぞく", types: [.water],
            baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
            abilities: [StubMaster.ability],
            learnset: learnset
        )
    }

    /// learnset が先頭ページの外の技1つだけ(issue #68 の再現手順そのもの)。
    static var onlyHiddenMoveLead: SpeciesDetail { leadSpecies(learnset: [StubBulkMaster.hiddenMove.id]) }

    /// learnset が「先頭ページの外の変化技1つ → 先頭ページの外のダメージ技」(既定の技の規則
    /// 「learnset の順で最初のダメージ技」を ID 解決でも守ることの確認用)。
    static var statusThenDamagingLead: SpeciesDetail {
        leadSpecies(learnset: [hiddenStatusMove(0).id, hiddenDamagingMove.id])
    }

    /// learnset が「先頭ページの外の変化技を上限+1個 → ダメージ技」(上限で打ち切ることの確認用)。
    static var manyStatusMovesLead: SpeciesDetail {
        leadSpecies(learnset: (0..<hiddenStatusCount).map { hiddenStatusMove($0).id } + [hiddenDamagingMove.id])
    }

    /// `lead` を種族一覧の先頭に置き、その後ろに `StubBulkMaster` の先頭ページ・`mixedSpecies`・`hiddenSpecies` を続ける。
    /// 技は `StubBulkMaster` の先頭ページ + `hiddenMove` + この型の追加の技(すべて先頭ページの外)。
    static func makeService(
        lead: SpeciesDetail,
        items: [Item] = [StubMaster.itemA, StubMaster.itemB],
        natures: [Nature] = [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature]
    ) -> StubPokeCalcService {
        let extraMoves = (0..<hiddenStatusCount).map(hiddenStatusMove) + [hiddenDamagingMove]
        return StubPokeCalcService(
            species: [lead] + StubBulkMaster.pageSpeciesList + [StubBulkMaster.mixedSpecies, StubBulkMaster.hiddenSpecies],
            moves: StubBulkMaster.pageMoveList + [StubBulkMaster.hiddenMove] + extraMoves,
            items: items,
            natures: natures
        )
    }

    /// `MasterSearch.pageLimit` 件ちょうどの架空の持ち物(持ち物の一覧が上限に達したことの確認用)。
    static var pageItemList: [Item] {
        (0..<MasterSearch.pageLimit).map { Item(id: "stub-page-item-\($0)", nameJa: "テストページどうぐ\($0)") }
    }
}
