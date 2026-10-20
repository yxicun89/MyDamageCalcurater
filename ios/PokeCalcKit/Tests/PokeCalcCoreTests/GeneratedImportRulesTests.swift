import Foundation
import XCTest

/// ヘキサゴナル採用 I0(ADR-0811・docs/plan/architecture-adoption.md §5.3): `PokeCalcCore` の中で、
/// 生成モジュール(PokeCalcAPI ほか)を import してよいのは `API*.swift`(アダプタ)だけ。
/// ViewModel・`*DomainTypes.swift`・モックなどは Service プロトコルと自分の型だけを使う。
/// ソースの import 行を読んで確かめる(Xcode 不要。`swift test` で macOS でも走る)。
/// 生成モジュール名は ios/scripts/openapi-targets.sh の「名前|仕様|…」の先頭欄から導く(生成対象を足せば検査も広がる)。
/// 既知の違反は無い(例外の一覧は空)。違反が出たら、直すか、ここへファイル名を理由つきで足す(足すのは最後の手段)。
final class GeneratedImportRulesTests: XCTestCase {

    /// 既知の違反の凍結一覧(ファイル名)。今は空。
    private static let frozenViolations: Set<String> = []

    private var ios: URL {
        // このファイル: ios/PokeCalcKit/Tests/PokeCalcCoreTests/<this>.swift → ios/
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().deletingLastPathComponent()
    }

    private func generatedModules() throws -> [String] {
        let script = try String(
            contentsOf: ios.appendingPathComponent("scripts/openapi-targets.sh"), encoding: .utf8)
        // `  "PokeCalcAPI|api/openapi.yaml|…"` の行の先頭欄
        return script.split(separator: "\n").compactMap { line -> String? in
            let t = line.trimmingCharacters(in: .whitespaces)
            guard t.hasPrefix("\""), t.contains("|") else { return nil }
            return String(t.dropFirst().prefix { $0 != "|" })
        }
    }

    func testGeneratedModulesAreImportedOnlyByAPIFiles() throws {
        let modules = try generatedModules()
        XCTAssertTrue(modules.contains("PokeCalcAPI"), "openapi-targets.sh から生成モジュール名を読めていない")

        let dir = ios.appendingPathComponent("PokeCalcKit/Sources/PokeCalcCore")
        let names = try FileManager.default.contentsOfDirectory(atPath: dir.path)
            .filter { $0.hasSuffix(".swift") }
        XCTAssertTrue(names.contains { $0.hasPrefix("API") }, "API*.swift が見つからない(検査の対象がずれている)")

        var violations: Set<String> = []
        for name in names where !name.hasPrefix("API") {
            let text = try String(contentsOf: dir.appendingPathComponent(name), encoding: .utf8)
            for line in text.split(separator: "\n") {
                let words = line.split(whereSeparator: { $0 == " " || $0 == "\t" }).map(String.init)
                guard let i = words.firstIndex(of: "import"), i + 1 < words.count else { continue }
                // `import Mod` / `import struct Mod.Type` / `@preconcurrency import Mod`
                let target = words[(i + 1)...].last { !$0.hasPrefix("//") } ?? ""
                let module = target.split(separator: ".").first.map(String.init) ?? target
                if modules.contains(module) { violations.insert(name) }
            }
        }
        XCTAssertEqual(
            violations.sorted(), Self.frozenViolations.sorted(),
            "API*.swift 以外が生成モジュールを import している。Service プロトコルと自分の型を使うこと")
    }
}
