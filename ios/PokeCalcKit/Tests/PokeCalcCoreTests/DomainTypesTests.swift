import Foundation
import PokeCalcAPI
import PokeCalcDesign
import XCTest

@testable import PokeCalcCore

/// ドメインの型(ADR-0017 §3)が契約(api/openapi.yaml の enum)と食い違わないことの同期テスト。
/// 画面は生成型を直接使わないので、enum の値の集合がずれると写像で黙って落ちる。ここで固定する。
final class DomainTypesTests: XCTestCase {

    /// `PokeType` の rawValue の集合 = openapi の `PokeType` enum(生成型の allCases)。
    func testPokeTypeMatchesOpenAPIEnum() {
        let domain = Set(PokeType.allCases.map(\.rawValue))
        let contract = Set(Components.Schemas.PokeType.allCases.map(\.rawValue))
        XCTAssertEqual(domain, contract)
        XCTAssertEqual(PokeType.allCases.count, 18)
    }

    /// 全タイプに design.md のタイプ色がある(タイプ色は PokeType の rawValue で引く)。
    func testEveryPokeTypeHasATypeColor() {
        for type in PokeType.allCases {
            XCTAssertNotNil(TypeColorToken.rgba(forTypeID: type.rawValue), "\(type.rawValue) の色が無い")
        }
    }

    func testStatKeyMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(StatKey.allCases.map(\.rawValue)),
                       Set(Components.Schemas.StatKey.allCases.map(\.rawValue)))
    }

    func testMoveCategoryMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(MoveCategory.allCases.map(\.rawValue)),
                       Set(Components.Schemas.MoveCategory.allCases.map(\.rawValue)))
    }

    /// `Format`(シングル/ダブル)は openapi の enum と同じ値集合。
    func testFormatMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(Format.allCases.map(\.rawValue)),
                       Set(Components.Schemas.Format.allCases.map(\.rawValue)))
    }

    /// `StatusCondition` は openapi の enum と同じ値集合(`badlyPoison` → `badly_poison` の
    /// スネークケースの取り違えをここで固定する)。
    func testStatusConditionMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(StatusCondition.allCases.map(\.rawValue)),
                       Set(Components.Schemas.StatusCondition.allCases.map(\.rawValue)))
    }

    /// `DefenderPreset` は openapi の enum と同じ8個(ADR-0009 §1)。
    func testDefenderPresetMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(DefenderPreset.allCases.map(\.rawValue)),
                       Set(Components.Schemas.DefenderPreset.allCases.map(\.rawValue)))
    }

    /// 逆算のドメインは ADR-0010 §R の形(契約更新前なので生成型とは突き合わせない)。
    func testReverseDomainEnumsFollowADR0010R() {
        XCTAssertEqual(Set(ReverseSide.allCases.map(\.rawValue)), ["defender", "attacker"])
        // §R1: 探索する性格クラスは「補正なし」「上昇」の2つだけ(下降は探索しない)
        XCTAssertEqual(NatureClass.allCases.map(\.rawValue), ["neutral", "plus"])
    }

    /// 2つの実装が同じ境界(`PokeCalcService`)を満たす。画面はこの型だけに依存する。
    func testBothImplementationsConformToPokeCalcService() throws {
        let url = try XCTUnwrap(URL(string: "https://pokecalc.example.invalid"))
        let client = Client(serverURL: url, transport: RecordingTransport(json: nil))
        let identity = ClientIdentity(deviceID: UUID().uuidString, sessionID: UUID().uuidString)
        let api: any PokeCalcService = APIPokeCalcService(client: client, identity: identity)
        let mock: any PokeCalcService = try MockPokeCalcService()
        _ = (api, mock)
    }

    /// `PokeCalcError` は OpenAPI の `Error.code` をそのまま運ぶ。
    func testPokeCalcErrorKeepsCodeAndMessage() {
        let error = PokeCalcError(code: "invalid_input", message: "テスト用のエラー")
        XCTAssertEqual(error.code, "invalid_input")
        XCTAssertEqual(error.message, "テスト用のエラー")
        // クライアント側で作るコードはサーバーの語彙と衝突しない名前で1か所に定義する
        XCTAssertFalse(PokeCalcError.Code.apiUnsupported.isEmpty)
        XCTAssertFalse(PokeCalcError.Code.transport.isEmpty)
        XCTAssertNotEqual(PokeCalcError.Code.apiUnsupported, PokeCalcError.Code.transport)
    }
}
