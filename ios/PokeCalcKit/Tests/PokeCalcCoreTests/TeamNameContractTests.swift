import Foundation
import PokeCalcAPI
import XCTest

/// ADR-0229(usability-round2 F-08「構築名はいらない」): 構築名は入力で省略でき、サーバーが既定名「名称未設定」を補う。
/// 応答の `Team.name` は従来どおり必ずある `String`(型を変えない)。生成型(`make ios-gen`)の形を固定する契約の網。
/// 架空の key だけを使う(実データは使わない。ADR-0002)。
final class TeamNameContractTests: XCTestCase {
    private func decoder() -> JSONDecoder {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }

    /// 作成・置換の本文は `name` を省略できる(`String?`)。
    func testTeamInputNameIsOptional() throws {
        let input = try decoder().decode(Components.Schemas.TeamInput.self, from: Data(#"{"members":[]}"#.utf8))
        let name: String? = input.name
        XCTAssertNil(name)
        let named = try decoder().decode(Components.Schemas.TeamInput.self, from: Data(#"{"name":"雨パ"}"#.utf8))
        XCTAssertEqual(named.name, "雨パ")
    }

    /// 応答の `Team.name` は `String`(省略されない)。サーバーが補った既定名もそのまま読める。
    func testTeamNameIsAlwaysString() throws {
        let json = #"""
        {"id":"11111111-2222-4333-8444-555555555555","name":"名称未設定","members":[],
         "createdAt":"2026-10-01T00:00:00Z","updatedAt":"2026-10-01T00:00:00Z"}
        """#
        let team = try decoder().decode(Components.Schemas.Team.self, from: Data(json.utf8))
        let name: String = team.name
        XCTAssertEqual(name, "名称未設定")
    }
}
