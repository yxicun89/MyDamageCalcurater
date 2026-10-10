import Foundation
import XCTest

@testable import PokeCalcCore

/// お気に入り関連 ViewModel のテスト用 `FavoritesService`。操作ごとに台本を1つ消費する(尽きたら空・成功)。
/// `.manual` は応答を保留し、テストが `resolve…` で**任意の順序**で返す(古い応答が後から届く状況の再現)。
/// 保留中に Task cancel を受けたら `CancellationError` で終える。
actor StubFavoritesService: FavoritesService {
    enum Step<Value: Sendable>: Sendable {
        case result(Value)
        case failure(PokeCalcError)
        case manual
    }

    private(set) var listCalls = 0
    private(set) var addRequests: [(label: String?, individual: Individual)] = []
    /// `addRequests` と同じ順で、各要求の `calc`(F-09)。
    private(set) var addCalcs: [CalcHistoryCalc?] = []
    private(set) var removeRequests: [String] = []

    private var listScript: [Step<[Favorite]>]
    private var addScript: [Step<FavoriteSaveResult>]
    private var removeScript: [Step<Void>]
    private var pendingList: [Int: CheckedContinuation<[Favorite], any Error>] = [:]
    private var pendingAdd: [Int: CheckedContinuation<FavoriteSaveResult, any Error>] = [:]
    private var pendingRemove: [Int: CheckedContinuation<Void, any Error>] = [:]

    init(
        list: [Step<[Favorite]>] = [], add: [Step<FavoriteSaveResult>] = [], remove: [Step<Void>] = []
    ) {
        listScript = list
        addScript = add
        removeScript = remove
    }

    static func favorite(
        _ id: String, key: String = "9001-000", label: String? = nil, updated: TimeInterval = 1_790_000_000
    ) -> Favorite {
        Favorite(
            id: id, label: label,
            individual: Individual(
                speciesKey: key, natureId: "test-nature-neutral",
                sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0)),
            createdAt: Date(timeIntervalSince1970: 1_789_000_000), updatedAt: Date(timeIntervalSince1970: updated))
    }

    func favorites() async throws -> [Favorite] {
        let index = listCalls
        listCalls += 1
        switch listScript.isEmpty ? Step<[Favorite]>.result([]) : listScript.removeFirst() {
        case .result(let value): return value
        case .failure(let error): throw error
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { pendingList[index] = $0 }
            } onCancel: {
                Task { await self.cancelList(index) }
            }
        }
    }

    func addFavorite(label: String?, individual: Individual, calc: CalcHistoryCalc?) async throws -> FavoriteSaveResult {
        let index = addRequests.count
        addRequests.append((label, individual))
        addCalcs.append(calc)
        guard !addScript.isEmpty else { return .created(Self.favorite("1", key: individual.speciesKey)) }
        switch addScript.removeFirst() {
        case .result(let value): return value
        case .failure(let error): throw error
        case .manual:
            return try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { pendingAdd[index] = $0 }
            } onCancel: {
                Task { await self.cancelAdd(index) }
            }
        }
    }

    func removeFavorite(id: String) async throws {
        let index = removeRequests.count
        removeRequests.append(id)
        guard !removeScript.isEmpty else { return }
        switch removeScript.removeFirst() {
        case .result: return
        case .failure(let error): throw error
        case .manual:
            try await withTaskCancellationHandler {
                try await withCheckedThrowingContinuation { pendingRemove[index] = $0 }
            } onCancel: {
                Task { await self.cancelRemove(index) }
            }
        }
    }

    func resolveList(at index: Int, with result: Result<[Favorite], PokeCalcError>) {
        guard let continuation = pendingList.removeValue(forKey: index) else {
            return XCTFail("保留中の favorites が無い: index \(index)")
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    func resolveAdd(at index: Int, with result: Result<FavoriteSaveResult, PokeCalcError>) {
        guard let continuation = pendingAdd.removeValue(forKey: index) else {
            return XCTFail("保留中の addFavorite が無い: index \(index)")
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    func resolveRemove(at index: Int, with result: Result<Void, PokeCalcError>) {
        guard let continuation = pendingRemove.removeValue(forKey: index) else {
            return XCTFail("保留中の removeFavorite が無い: index \(index)")
        }
        continuation.resume(with: result.mapError { $0 as any Error })
    }

    private func cancelList(_ index: Int) { pendingList.removeValue(forKey: index)?.resume(throwing: CancellationError()) }
    private func cancelAdd(_ index: Int) { pendingAdd.removeValue(forKey: index)?.resume(throwing: CancellationError()) }
    private func cancelRemove(_ index: Int) { pendingRemove.removeValue(forKey: index)?.resume(throwing: CancellationError()) }

    /// 各操作が `count` 回以上呼ばれるまで待つ(1ms ポーリング。壁時計の固定待ちをしない)。
    func waitForList(calls count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        try await poll("favorites", file: file, line: line) { await self.listCalls >= count }
    }

    func waitForAdd(calls count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        try await poll("addFavorite", file: file, line: line) { await self.addRequests.count >= count }
    }

    func waitForRemove(calls count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        try await poll("removeFavorite", file: file, line: line) { await self.removeRequests.count >= count }
    }

    private func poll(
        _ name: String, file: StaticString, line: UInt, _ condition: () async -> Bool
    ) async throws {
        for _ in 0..<5000 {
            if await condition() { return }
            try await Task.sleep(for: .milliseconds(1))
        }
        XCTFail("\(name) が呼ばれなかった", file: file, line: line)
        throw PokeCalcError(code: "test_timeout", message: "\(name) の待ち合わせがタイムアウト")
    }
}
