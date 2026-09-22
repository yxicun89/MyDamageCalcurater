import Foundation
import XCTest

@testable import PokeCalcCore

/// `DefenderPreset` を API の文字列(openapi の `DefenderPreset` enum)から作る。
/// テストは Swift の case 名ではなく契約の文字列で書く(case 名は実装が決めてよい)。
func presets(_ rawValues: [String], file: StaticString = #filePath, line: UInt = #line) throws -> [DefenderPreset] {
    try rawValues.map { raw in
        try XCTUnwrap(DefenderPreset(rawValue: raw), "未知のプリセット \(raw)", file: file, line: line)
    }
}

/// async な throw を検査し、`PokeCalcError` であることを確かめて返す。
func assertThrowsPokeCalcError<T>(
    _ label: String,
    file: StaticString = #filePath,
    line: UInt = #line,
    _ body: () async throws -> T
) async -> PokeCalcError? {
    do {
        _ = try await body()
        XCTFail("\(label): エラーにならなかった", file: file, line: line)
        return nil
    } catch let error as PokeCalcError {
        return error
    } catch {
        XCTFail("\(label): PokeCalcError ではない \(type(of: error)): \(error)", file: file, line: line)
        return nil
    }
}

/// 全 SP 0 の能力ポイント。
let zeroSP = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)

/// 一括計算の行を組み立てるテスト用の防御側(値に意味は無い。行の整形・ViewModel は defender を使わない)。
/// `BulkCalcRow.defender` が契約で必須になった(ADR-0200 §1)ので、行を作るテストはこれを渡す。
let testBulkDefender = BulkDefender(
    sp: zeroSP, nature: NatureModifier(), natureId: "test-nature-neutral",
    stats: StatBlock(hp: 100, atk: 50, def: 50, spa: 50, spd: 50, spe: 50)
)
