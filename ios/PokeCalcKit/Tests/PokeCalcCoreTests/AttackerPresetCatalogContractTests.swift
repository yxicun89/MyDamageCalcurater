import Foundation
import XCTest

@testable import PokeCalcCore

/// 攻撃側プリセットの契約テスト(issue #71。ADR-0114「Web・iOS への依頼」、ADR-0501「P6-12」)。
///
/// 正は `engine/presets/attacker.json`。複製せず、このファイルの位置(`#filePath`)からリポジトリの
/// 実ファイルを読む。macOS の `swift test` だけでなく、iOS シミュレータ(`make ios-test-unit`)の
/// テストプロセスもホストのファイルを読めることを確かめてある(ADR-0501「P6-12」4章)。
/// ファイルが無ければ失敗させる(スキップしない)。
///
/// 確かめること: キーと並び順、既定、技の分類ごとの関連ステータス、プリセットごとの SP と性格の規則
/// (`boost` = 関連ステータス上昇・`boostMinus` 下降、`neutral` = 無補正)。
final class AttackerPresetCatalogContractTests: XCTestCase {

    // MARK: - JSON の形(engine の読み込みと同じく、知らない項目があれば失敗させる)

    private struct Catalog: Decodable {
        let schemaVersion: Int
        let `default`: String
        let relevantStat: [String: String]
        let boostMinus: [String: String]
        let presets: [Entry]

        struct Entry: Decodable {
            let key: String
            let relevantSp: Int
            let nature: String
        }
    }

    /// このテストが理解している版。上がったら規則を読み直してからテストを直す(黙って通さない)。
    private static let knownSchemaVersion = 1
    private static let topLevelKeys: Set<String> = ["schemaVersion", "default", "relevantStat", "boostMinus", "presets"]
    private static let entryKeys: Set<String> = ["key", "relevantSp", "nature"]
    private static let natureBoost = "boost"
    private static let natureNeutral = "neutral"

    /// `ios/PokeCalcKit/Tests/PokeCalcCoreTests/<このファイル>` から5階層上がリポジトリのルート。
    private static var catalogURL: URL {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<5 { url.deleteLastPathComponent() }
        return url.appendingPathComponent("engine/presets/attacker.json")
    }

    private struct CatalogMissing: Error {}

    private func loadCatalog() throws -> (catalog: Catalog, raw: [String: Any]) {
        let url = Self.catalogURL
        guard FileManager.default.fileExists(atPath: url.path) else {
            XCTFail("攻撃側プリセットの正が見つからない: \(url.path)(ADR-0114)")
            throw CatalogMissing()
        }
        let data = try Data(contentsOf: url)
        let raw = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any], "最上位が object ではない")
        return (try JSONDecoder().decode(Catalog.self, from: data), raw)
    }

    func testCatalogHasOnlyKnownFieldsAndVersion() throws {
        let (catalog, raw) = try loadCatalog()
        XCTAssertEqual(catalog.schemaVersion, Self.knownSchemaVersion, "版が変わった。ADR-0114 の規則を読み直す")
        XCTAssertEqual(Set(raw.keys), Self.topLevelKeys)
        let entries = try XCTUnwrap(raw["presets"] as? [[String: Any]])
        XCTAssertFalse(entries.isEmpty)
        for entry in entries {
            XCTAssertEqual(Set(entry.keys), Self.entryKeys, "\(entry)")
        }
        let natures = Set(catalog.presets.map(\.nature))
        XCTAssertTrue(natures.isSubset(of: [Self.natureBoost, Self.natureNeutral]), "知らない nature: \(natures)")
    }

    // MARK: - キー・順序・既定

    /// `allCases` の順(= 画面のセグメントの並び)と `catalogKey` が JSON の `presets` と同じ。
    func testCasesMatchCatalogKeysInOrder() throws {
        let (catalog, _) = try loadCatalog()
        XCTAssertEqual(AttackerPreset.allCases.map(\.catalogKey), catalog.presets.map(\.key))
    }

    func testDefaultMatchesCatalog() throws {
        let (catalog, _) = try loadCatalog()
        XCTAssertEqual(AttackerPreset.defaultPreset.catalogKey, catalog.default)
    }

    // MARK: - 規則

    func testRelevantStatMatchesCatalogForEveryCategory() throws {
        let (catalog, _) = try loadCatalog()
        XCTAssertEqual(Set(catalog.relevantStat.keys), Set(MoveCategory.allCases.map(\.rawValue)),
                       "JSON の分類と MoveCategory が一致しない")
        for category in MoveCategory.allCases {
            XCTAssertEqual(AttackerPreset.relevantStat(for: category).rawValue, catalog.relevantStat[category.rawValue],
                           "\(category)")
        }
    }

    /// プリセット × 技の分類の全組み合わせで、`build` の SP と性格が JSON の規則どおり。
    /// 性格一覧は「関連ステータス上昇だが下降が違う」囮を先に置き、`boostMinus` まで見ていることを確かめる。
    func testBuildFollowsCatalogSpAndNatureRules() throws {
        let (catalog, _) = try loadCatalog()
        let natures = [
            Nature(id: "tn-atk-def", nameJa: "テスト攻撃上昇防御下降", plus: .atk, minus: .def),
            Nature(id: "tn-spa-def", nameJa: "テスト特攻上昇防御下降", plus: .spa, minus: .def),
            Nature(id: "tn-atk-spa", nameJa: "テスト攻撃上昇特攻下降", plus: .atk, minus: .spa),
            Nature(id: "tn-spa-atk", nameJa: "テスト特攻上昇攻撃下降", plus: .spa, minus: .atk),
            Nature(id: "tn-neutral", nameJa: "テスト無補正"),
        ]
        let entries = Dictionary(uniqueKeysWithValues: catalog.presets.map { ($0.key, $0) })
        for preset in AttackerPreset.allCases {
            let entry = try XCTUnwrap(entries[preset.catalogKey], "JSON に無いキー: \(preset.catalogKey)")
            for category in MoveCategory.allCases {
                let name = "\(preset) / \(category)"
                let relevant = try XCTUnwrap(catalog.relevantStat[category.rawValue].flatMap(StatKey.init(rawValue:)), name)
                let build = try AttackerPreset.build(preset, moveCategory: category, natures: natures)

                XCTAssertEqual(build.sp, Self.block(only: relevant, value: entry.relevantSp), name)

                let nature = try XCTUnwrap(natures.first { $0.id == build.natureId }, name)
                switch entry.nature {
                case Self.natureBoost:
                    let minus = try XCTUnwrap(catalog.boostMinus[relevant.rawValue].flatMap(StatKey.init(rawValue:)), name)
                    XCTAssertEqual(nature.plus, relevant, name)
                    XCTAssertEqual(nature.minus, minus, name)
                case Self.natureNeutral:
                    XCTAssertNil(nature.plus, name)
                    XCTAssertNil(nature.minus, name)
                default:
                    XCTFail("知らない nature: \(entry.nature)(\(name))")
                }
            }
        }
    }

    /// `stat` だけに `value`、他は 0。
    private static func block(only stat: StatKey, value: Int) -> StatBlock {
        var block = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        switch stat {
        case .hp: block.hp = value
        case .atk: block.atk = value
        case .def: block.def = value
        case .spa: block.spa = value
        case .spd: block.spd = value
        case .spe: block.spe = value
        }
        return block
    }
}
