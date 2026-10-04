@testable import PokeCalcCore

/// 持ち物の役割とメガ固定(ADR-0509)の ViewModel テスト用の架空マスタ。
/// 名前は「テスト」で始める(実在の名前・数値を使わない)。メガストーンの `nameJa` は画面に出てはならない値にしてある。
enum StubMegaMaster {
    static let attackOnly = Item(id: "stub-item-attack-only", nameJa: "テストどうぐこうげき", roles: [.attacker], isMegaStone: false)
    static let defenseOnly = Item(id: "stub-item-defense-only", nameJa: "テストどうぐぼうぎょ", roles: [.defender], isMegaStone: false)
    static let both = Item(id: "stub-item-both", nameJa: "テストどうぐりょうほう", roles: [.attacker, .defender], isMegaStone: false)
    static let noRole = Item(id: "stub-item-no-role", nameJa: "テストどうぐこうかなし", roles: [], isMegaStone: false)
    static let stone = Item(id: "stub-item-mega-stone", nameJa: "テストSTONE-ENGLISH", roles: [], isMegaStone: true)
    static let otherStone = Item(id: "stub-item-mega-stone-2", nameJa: "テストSTONE-ENGLISH-2", roles: [], isMegaStone: true)

    /// 役割つきの持ち物(マスタの順)。
    static let items = [attackOnly, defenseOnly, both, noRole, stone, otherStone]

    /// `StubMaster.alpha` のメガ(learnset は同じ。基本種名あり)。
    static let megaAlpha = SpeciesDetail(
        key: "9101-001", dexNo: 9101, form: 1, nameJa: "テストメガアルファ", types: [.normal],
        baseStats: StatBlock(hp: 50, atk: 80, def: 60, spa: 50, spd: 60, spe: 70),
        abilities: [StubMaster.ability], learnset: StubMaster.alpha.learnset,
        isMega: true, requiredItemId: stone.id, baseSpeciesKey: StubMaster.alpha.key, baseSpeciesNameJa: StubMaster.alpha.nameJa
    )
    /// 基本種名が null のメガ(「メガストーン」だけを出す)。
    static let megaNoBaseName = SpeciesDetail(
        key: "9102-001", dexNo: 9102, form: 1, nameJa: "テストメガベータ", types: [.fire],
        baseStats: StatBlock(hp: 50, atk: 80, def: 60, spa: 50, spd: 60, spe: 70),
        abilities: [StubMaster.ability], learnset: StubMaster.beta.learnset,
        isMega: true, requiredItemId: otherStone.id, baseSpeciesKey: nil, baseSpeciesNameJa: nil
    )
    /// ストーンが持ち物マスタに無いメガ(missing)。
    static let megaMissingStone = SpeciesDetail(
        key: "9103-001", dexNo: 9103, form: 1, nameJa: "テストメガガンマ", types: [.water],
        baseStats: StatBlock(hp: 50, atk: 80, def: 60, spa: 50, spd: 60, spe: 70),
        abilities: [StubMaster.ability], learnset: StubMaster.gamma.learnset,
        isMega: true, requiredItemId: "stub-item-not-in-master", baseSpeciesKey: StubMaster.gamma.key, baseSpeciesNameJa: StubMaster.gamma.nameJa
    )

    /// 固定中の表示(`MegaItemText.stoneName` に基本種名を入れた値)。
    static let megaAlphaStoneName = "\(StubMaster.alpha.nameJa)のメガストーン"

    /// 既定の攻撃側 = alpha、防御側 = beta(先頭2件)のまま、メガ種族を後ろに足したマスタ。
    static func makeService(natures: [Nature] = [StubMaster.atkUpNature, StubMaster.neutralNature, StubMaster.spaUpNature]) -> StubPokeCalcService {
        StubMaster.makeService(
            species: [StubMaster.alpha, StubMaster.beta, StubMaster.gamma, StubMaster.statusOnly, megaAlpha, megaNoBaseName, megaMissingStone],
            items: items, natures: natures)
    }
}
