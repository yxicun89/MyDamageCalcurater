import Foundation
import XCTest

@testable import PokeCalcCore

/// 素早さのテスト用フィクスチャ(架空。名前は「テスト」で始める。ADR-0002)。数値に意味は無い。
enum StubSpeed {
    static let pokemonA = SpeedPokemon(pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100)
    static let pokemonB = SpeedPokemon(pokemonId: "9002-000", nameJa: "テストミズガメ", types: ["water"], baseSpeed: 40)
    static let pokemonC = SpeedPokemon(pokemonId: "9003-000", nameJa: "テストクサネコ", types: ["grass"], baseSpeed: 80)
    static let pokemonD = SpeedPokemon(pokemonId: "9004-000", nameJa: "テストデンキネズミ", types: ["electric"], baseSpeed: 130)

    static let pokemonList = SpeedPokemonList(
        regulationId: "example", pokemon: [pokemonA, pokemonB, pokemonC, pokemonD])

    static func entry(_ pokemon: SpeedPokemon, _ preset: SpeedPresetID) -> SpeedTableEntry {
        SpeedTableEntry(
            pokemonId: pokemon.pokemonId, nameJa: pokemon.nameJa, types: pokemon.types,
            baseSpeed: pokemon.baseSpeed, preset: preset)
    }

    /// 速い順の表(300 は同速の2行)。トリックルームのときは `reversed()` を使う。
    static let tiersDescending: [SpeedTier] = [
        SpeedTier(speed: 300, entries: [entry(pokemonA, .maxScarf), entry(pokemonA, .maxPlus1)]),
        SpeedTier(speed: 250, entries: [entry(pokemonD, .max)]),
        SpeedTier(speed: 200, entries: [entry(pokemonC, .neutralMax)]),
        SpeedTier(speed: 100, entries: [entry(pokemonB, .uninvested)]),
    ]

    static let allPresets = SpeedPresetID.allCases

    static func table(tiers: [SpeedTier] = tiersDescending, presets: [SpeedPresetID] = allPresets) -> SpeedTable {
        SpeedTable(regulationId: "example", presets: presets, tiers: tiers)
    }

    static func position(
        speed: Int, pokemon: SpeedPokemon? = nil, faster: Int = 0, slower: Int = 0, tie: [SpeedTableEntry] = []
    ) -> SpeedPosition {
        SpeedPosition(speed: speed, pokemon: pokemon, faster: faster, slower: slower, tie: tie)
    }

    static let failure = PokeCalcError(code: "master_unavailable", message: "test")
}

/// `SpeedViewModel` のテスト用 `SpeedService`。呼び出しを記録し、応答はクロージャで決める。
/// `hold` した種類の呼び出しは `release` されるまで返らない(古い応答・cancel の検証用)。
/// 位置の既定の応答は「呼び出しの順番 × 100 の実数値」(どの要求への応答か区別できる)。
actor StubSpeedService: SpeedService {
    enum Kind: Hashable, Sendable {
        case pokemon
        case table
        case position
    }

    struct TableCall: Equatable, Sendable {
        let presets: [SpeedPresetID]?
        let field: SpeedTableField
    }

    private(set) var pokemonCalls = 0
    private(set) var tableCalls: [TableCall] = []
    private(set) var positionCalls: [SpeedPositionRequest] = []
    /// cancel された呼び出しの番号(0 始まり。種類ごと)。
    private(set) var cancelled: [Kind: [Int]] = [:]

    private var pokemonResult: Result<SpeedPokemonList, PokeCalcError> = .success(StubSpeed.pokemonList)
    private var tableResponder: @Sendable ([SpeedPresetID]?, SpeedTableField, Int) -> Result<SpeedTable, PokeCalcError> = { _, _, _ in
        .success(StubSpeed.table())
    }
    private var positionResponder: @Sendable (SpeedPositionRequest, Int) -> Result<SpeedPosition, PokeCalcError> = { _, index in
        .success(StubSpeed.position(speed: 100 * (index + 1)))
    }

    private var held: Set<Kind> = []
    private var released: [Kind: Set<Int>] = [:]
    /// true なら hold 中に cancel されても返らず、release で(古い要求でも)応答を返す。
    private var ignoresCancellation = false

    private static let pollInterval: Duration = .milliseconds(1)
    private static let pollLimit = 3000

    // MARK: - 設定

    func setPokemonResult(_ result: Result<SpeedPokemonList, PokeCalcError>) { pokemonResult = result }

    func setTableResponder(_ responder: @escaping @Sendable ([SpeedPresetID]?, SpeedTableField, Int) -> Result<SpeedTable, PokeCalcError>) {
        tableResponder = responder
    }

    func setPositionResponder(_ responder: @escaping @Sendable (SpeedPositionRequest, Int) -> Result<SpeedPosition, PokeCalcError>) {
        positionResponder = responder
    }

    func hold(_ kinds: Set<Kind>, ignoringCancellation: Bool = false) {
        held = kinds
        ignoresCancellation = ignoringCancellation
    }

    func release(_ kind: Kind, at index: Int) {
        released[kind, default: []].insert(index)
    }

    func cancelledIndices(_ kind: Kind) -> [Int] { cancelled[kind] ?? [] }

    func waitForCalls(_ kind: Kind, count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.pollLimit {
            if callCount(kind) >= count { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("\(kind) が \(count) 回呼ばれなかった(\(callCount(kind)) 回)", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    func waitForCancellation(_ kind: Kind, at index: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.pollLimit {
            if (cancelled[kind] ?? []).contains(index) { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("\(kind) の \(index) 番目が cancel されなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "待ち合わせがタイムアウト")
    }

    private func callCount(_ kind: Kind) -> Int {
        switch kind {
        case .pokemon: return pokemonCalls
        case .table: return tableCalls.count
        case .position: return positionCalls.count
        }
    }

    // MARK: - SpeedService

    func pokemon() async throws -> SpeedPokemonList {
        pokemonCalls += 1
        try await holdIfNeeded(.pokemon, index: pokemonCalls - 1)
        return try pokemonResult.get()
    }

    func table(presets: [SpeedPresetID]?, field: SpeedTableField) async throws -> SpeedTable {
        let index = tableCalls.count
        tableCalls.append(TableCall(presets: presets, field: field))
        try await holdIfNeeded(.table, index: index)
        return try tableResponder(presets, field, index).get()
    }

    func position(_ request: SpeedPositionRequest) async throws -> SpeedPosition {
        let index = positionCalls.count
        positionCalls.append(request)
        try await holdIfNeeded(.position, index: index)
        return try positionResponder(request, index).get()
    }

    private func holdIfNeeded(_ kind: Kind, index: Int) async throws {
        guard held.contains(kind) else { return }
        while !(released[kind] ?? []).contains(index) {
            if ignoresCancellation {
                await Task.yield()
                try? await Task.sleep(for: Self.pollInterval)
                continue
            }
            do {
                try await Task.sleep(for: Self.pollInterval)
            } catch {
                cancelled[kind, default: []].append(index)
                throw CancellationError()
            }
        }
        if !ignoresCancellation, Task.isCancelled {
            cancelled[kind, default: []].append(index)
            throw CancellationError()
        }
    }
}
