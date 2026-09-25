import PokeCalcAPI
import XCTest

@testable import PokeCalcCore

/// 計算条件のドメイン型と日本語ラベル(issue #274。ADR-0501「issue #274」3章)。
///
/// 並び順(`allCases`)は画面のピル・トグルの並びで、Web レーンもこの順と文言に揃える
/// (docs/ai-shared/DECISIONS.md 2026-09-25「計算条件の入力 UI」)。値の集合は openapi と同期する。
final class CalcConditionsDomainTests: XCTestCase {

    // MARK: - 契約との同期

    func testWeatherMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(Weather.allCases.map(\.rawValue)),
                       Set(Components.Schemas.Weather.allCases.map(\.rawValue)))
    }

    func testTerrainMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(Terrain.allCases.map(\.rawValue)),
                       Set(Components.Schemas.Terrain.allCases.map(\.rawValue)))
    }

    /// `ScreenKind` の raw value は openapi `Screens` のプロパティ名(CodingKeys)と同じ。
    func testScreenKindMatchesOpenAPIScreensProperties() {
        XCTAssertEqual(ScreenKind.allCases.map(\.rawValue),
                       [Components.Schemas.Screens.CodingKeys.reflect.rawValue,
                        Components.Schemas.Screens.CodingKeys.lightScreen.rawValue,
                        Components.Schemas.Screens.CodingKeys.auroraVeil.rawValue])
    }

    // MARK: - 既定値

    func testDefaultFieldIsEmptyAndBulkRequestDefaultsToIt() {
        let field = FieldState()
        XCTAssertEqual(field.weather, .none)
        XCTAssertEqual(field.terrain, .none)
        XCTAssertEqual(field.attackerScreens, Screens(reflect: false, lightScreen: false, auroraVeil: false))
        XCTAssertEqual(field.defenderScreens, Screens(reflect: false, lightScreen: false, auroraVeil: false))

        let request = BulkCalcRequest(
            format: .single,
            attacker: Individual(speciesKey: "9001-000", natureId: "test-nature", sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)),
            defenderSpeciesKey: "9002-000", moveId: "test-move")
        XCTAssertEqual(request.field, FieldState(), "既存の呼び出し(field を渡さない)は何もない場")
        XCTAssertFalse(request.critical)
    }

    func testScreensKindAccessorsRoundTrip() {
        for kind in ScreenKind.allCases {
            let on = Screens().setting(kind, to: true)
            XCTAssertTrue(on.isOn(kind), "\(kind)")
            for other in ScreenKind.allCases where other != kind {
                XCTAssertFalse(on.isOn(other), "\(kind) は \(other) を変えない")
            }
            XCTAssertEqual(on.setting(kind, to: false), Screens())
        }
    }

    // MARK: - 並び順とラベル(3章の表)

    func testWeatherOrderAndLabels() {
        XCTAssertEqual(Weather.allCases, [.none, .sun, .rain, .sand, .snow])
        XCTAssertEqual(Weather.allCases.map(WeatherLabel.japaneseName(for:)),
                       ["なし", "はれ", "あめ", "すなあらし", "ゆき"])
    }

    func testTerrainOrderAndLabels() {
        XCTAssertEqual(Terrain.allCases, [.none, .electric, .grassy, .psychic, .misty])
        XCTAssertEqual(Terrain.allCases.map(TerrainLabel.japaneseName(for:)),
                       ["なし", "エレキフィールド", "グラスフィールド", "サイコフィールド", "ミストフィールド"])
    }

    func testScreenOrderAndLabels() {
        XCTAssertEqual(ScreenKind.allCases, [.reflect, .lightScreen, .auroraVeil])
        XCTAssertEqual(ScreenKind.allCases.map(ScreenKindLabel.japaneseName(for:)),
                       ["リフレクター", "ひかりのかべ", "オーロラベール"])
    }

    func testSectionAndToggleLabels() {
        XCTAssertEqual(CalcConditionLabels.sectionTitle, "詳細")
        XCTAssertEqual(CalcConditionLabels.critical, "急所")
        XCTAssertEqual(CalcConditionLabels.burn, "やけど")
        XCTAssertEqual(CalcConditionLabels.weatherTitle, "天候")
        XCTAssertEqual(CalcConditionLabels.terrainTitle, "フィールド")
        XCTAssertEqual(CalcConditionLabels.defenderScreensTitle, "防御側の壁")
        XCTAssertEqual(CalcConditionLabels.rankTitle, "攻撃側のランク")
        XCTAssertEqual(CalcConditionLabels.abilityTitle, "攻撃側の特性")
        XCTAssertEqual(CalcConditionLabels.abilityUnspecified, "指定なし")
    }

    /// ランクは「文字 + 符号付きの数」。0 は「±0」。文字は `AttackerPreset` と同じ(atk → A、spa → C)。
    func testRankLabel() {
        let cases: [(StatKey, Int, String)] = [
            (.atk, 0, "A ±0"),
            (.atk, 1, "A +1"),
            (.atk, 6, "A +6"),
            (.atk, -1, "A -1"),
            (.atk, -6, "A -6"),
            (.spa, 0, "C ±0"),
            (.spa, 2, "C +2"),
            (.spa, -6, "C -6"),
        ]
        for (stat, value, expected) in cases {
            XCTAssertEqual(RankLabel.text(stat: stat, value: value), expected, "\(stat) \(value)")
        }
    }
}
