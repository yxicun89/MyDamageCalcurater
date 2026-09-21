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
