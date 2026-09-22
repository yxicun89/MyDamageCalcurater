import Foundation
import PokeCalcAPI
import PokeCalcDesign
import XCTest

@testable import PokeCalcCore

/// ドメインの型(ADR-0500 §3)が契約(api/openapi.yaml の enum)と食い違わないことの同期テスト。
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

    /// 逆算のドメインは ADR-0010 §R の形。
    /// 期待値の変更理由: P3-1(ADR-0200)で契約が §R の形になったので、以前は「契約更新前なので生成型とは
    /// 突き合わせない」としていた `ReverseSide` / `NatureClass` も生成型と値集合を突き合わせる。
    func testReverseDomainEnumsFollowADR0010R() {
        XCTAssertEqual(Set(ReverseSide.allCases.map(\.rawValue)), ["defender", "attacker"])
        // §R1: 探索する性格クラスは「補正なし」「上昇」の2つだけ(下降は探索しない)
        XCTAssertEqual(NatureClass.allCases.map(\.rawValue), ["neutral", "plus"])
    }

    func testReverseSideMatchesOpenAPIEnum() {
        XCTAssertEqual(Set(ReverseSide.allCases.map(\.rawValue)),
                       Set(Components.Schemas.ReverseSide.allCases.map(\.rawValue)))
    }

    /// `NatureClass` は openapi の `NatureClass` と同じ値集合で、定義順(§R4 の順位に使う)も同じ。
    func testNatureClassMatchesOpenAPIEnumInOrder() {
        XCTAssertEqual(NatureClass.allCases.map(\.rawValue),
                       Components.Schemas.NatureClass.allCases.map(\.rawValue))
    }

    /// エラーコードの持ち方(ADR-0500 §3・P6-2 契約追従で決定):
    /// ドメインは openapi `ErrorCode` を enum に写さず、**rawValue の文字列のまま** `PokeCalcError.code` に運ぶ。
    /// 理由: (1) 同じ `code` にクライアント側のコード(`client_*`)も入るので、enum にすると2つの語彙を
    /// 1つの型に混ぜることになる。(2) 画面が分岐に使うのは一部だけで、残りは code と message を表示するだけ。
    /// 代わりに、ドメインが定数で持つサーバー語彙が契約に実在すること、クライアント側のコードが契約の語彙と
    /// 衝突しないことをここで固定する。
    func testErrorCodeConstantsAreConsistentWithOpenAPIErrorCode() {
        let contract = Set(Components.Schemas.ErrorCode.allCases.map(\.rawValue))
        // サーバーの語彙を真似た定数(モックが使う)は契約の ErrorCode にある値でなければならない
        let serverVocabulary = [PokeCalcError.Code.notFound, PokeCalcError.Code.invalidInput]
        for code in serverVocabulary {
            XCTAssertTrue(contract.contains(code), "\(code) が openapi ErrorCode に無い")
        }
        // クライアント側で作るコードは `client_` 接頭辞で、契約の語彙と衝突しない
        let clientCodes = [
            PokeCalcError.Code.transport, PokeCalcError.Code.decode,
            PokeCalcError.Code.natureUnavailable, PokeCalcError.Code.insufficientSpecies,
            PokeCalcError.Code.moveUnavailable, PokeCalcError.Code.selectedMoveMissing,
            PokeCalcError.Code.fixtureMissing, PokeCalcError.Code.fixtureInvalid,
        ]
        XCTAssertEqual(Set(clientCodes).count, clientCodes.count, "クライアント側のコードが重複している")
        for code in clientCodes {
            XCTAssertTrue(code.hasPrefix("client_"), code)
            XCTAssertFalse(contract.contains(code), "\(code) が openapi ErrorCode と衝突")
        }
        // 契約側も `client_` 接頭辞を使わない(将来サーバーに同名が増えたら気付けるように)
        XCTAssertTrue(contract.allSatisfy { !$0.hasPrefix("client_") })
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
        // (apiUnsupported は P3-1 で逆算が API 対応したため生成元が無くなり削除した。ADR-0500 §3)
        XCTAssertFalse(PokeCalcError.Code.transport.isEmpty)
        XCTAssertNotEqual(PokeCalcError.Code.transport, PokeCalcError.Code.decode)
    }
}
