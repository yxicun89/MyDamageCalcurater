import Foundation

/// `MockPokeCalcService` が読む架空データ(ADR-0017 §4)。
///
/// JSON は `Resources/*.json` に置く(パッケージのリソースとして `Bundle.module` から読める)。
/// 名前はすべて「テスト」で始まる架空データで、実在のポケモン・技・持ち物・性格の名前や
/// 数値は使わない(ADR-0002)。この JSON の形は openapi の契約とは無関係(サーバーへ送らない、
/// アプリ内だけの固定フィクスチャ)。
struct MockFixtures {
    struct SpeciesEntry: Decodable {
        struct BaseStats: Decodable {
            let hp: Int, atk: Int, def: Int, spa: Int, spd: Int, spe: Int
        }
        struct AbilityEntry: Decodable {
            let id: String
            let nameJa: String
        }
        let key: String
        let dexNo: Int
        let form: Int
        let nameJa: String
        let types: [String]
        let baseStats: BaseStats
        let abilities: [AbilityEntry]
        let learnset: [String]
    }

    struct MoveEntry: Decodable {
        let id: String
        let nameJa: String
        let type: String
        let category: String
        let power: Int
        let priority: Int
    }

    struct ItemEntry: Decodable {
        let id: String
        let nameJa: String
    }

    struct NatureEntry: Decodable {
        let id: String
        let nameJa: String
        let plus: String?
        let minus: String?
    }

    struct CalcResultEntry: Decodable {
        struct KOEntry: Decodable {
            let hits: Int
            let guaranteed: Bool
            let chancePercent: Double
            let displayChancePercent: Double
        }
        let rolls: [Int]
        let minDamage: Int
        let maxDamage: Int
        let minPercent: Double
        let maxPercent: Double
        let defenderHP: Int
        let effectiveness: Double
        let stab: Bool
        let ko: KOEntry
    }

    let species: [SpeciesEntry]
    let moves: [MoveEntry]
    let items: [ItemEntry]
    let natures: [NatureEntry]
    /// preset の生値(openapi `DefenderPreset`)または `"single"`(1 vs 1 用)をキーにした
    /// 決め打ちの計算結果。モックはダメージを計算しないので、この JSON をそのまま返す
    /// (式は書かない)。ただし JSON の値そのものは手で自己矛盾なく作ってある:
    /// `minPercent` = `floor(minDamage*1000/defenderHP)/10`、`maxPercent` = 同じ式の四捨五入
    /// (ADR-0010 §3)、`ko.hits` = `ceil(defenderHP/maxDamage)`、`ko.guaranteed` =
    /// `minDamage*hits >= defenderHP`。`guaranteed` のときだけ `chancePercent=0` /
    /// `displayChancePercent=100.0`(同 §3.4)。`guaranteed=false` の `chancePercent` は
    /// engine の実式(複数発の乱数計算)ではなく、フィクスチャらしく見せるための概算値。
    let calcResultsByKey: [String: CalcResultEntry]

    /// `Resources/` 直下の JSON ファイル名(拡張子なし)。`fixtureURLs()` と `load()` の両方が使う。
    static let resourceNames = ["species", "moves", "items", "natures", "calc-results"]

    static func fixtureURLs() throws -> [URL] {
        try resourceNames.map { name in
            guard let url = Bundle.module.url(forResource: name, withExtension: "json") else {
                throw PokeCalcError(code: PokeCalcError.Code.fixtureMissing, message: "\(name).json がバンドルに無い")
            }
            return url
        }
    }

    static func load() throws -> MockFixtures {
        let decoder = JSONDecoder()
        func decode<T: Decodable>(_ type: T.Type, from url: URL) throws -> T {
            try decoder.decode(T.self, from: Data(contentsOf: url))
        }
        let urls = try fixtureURLs()
        var byName: [String: URL] = [:]
        for (name, url) in zip(resourceNames, urls) { byName[name] = url }
        guard let speciesURL = byName["species"], let movesURL = byName["moves"],
              let itemsURL = byName["items"], let naturesURL = byName["natures"],
              let calcResultsURL = byName["calc-results"]
        else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureMissing, message: "モックのリソース構成が不正")
        }
        return MockFixtures(
            species: try decode([SpeciesEntry].self, from: speciesURL),
            moves: try decode([MoveEntry].self, from: movesURL),
            items: try decode([ItemEntry].self, from: itemsURL),
            natures: try decode([NatureEntry].self, from: naturesURL),
            calcResultsByKey: try decode([String: CalcResultEntry].self, from: calcResultsURL)
        )
    }
}
