import Foundation
import HTTPTypes
import OpenAPIRuntime

// JudgeSpeedValueMiddleware: 判定応答の素早さの欄(`*SpeedApplied`・`*SpeedIgnored`)の未知の値で decode を落とさない(ADR-0512 §3)。
//
// 200 の JSON から4配列を「生の文字列」として呼び出しごとの入れ物(`@TaskLocal`)へ退避し、生成型には空配列で渡す。
// ミドルウェア自身は状態を持たない(生成の `Client` は Sendable で共有されるため。並行呼び出しで混ざらない)。
// 4配列が「文字列の配列」でないとき(欄なし・null・非配列・非文字列要素・JSON でない)は本文に触れず、生成の decode に落とさせる。

/// 1回の呼び出しの退避先。`matchups` の位置 → 4配列の生の文字列。
final class JudgeSpeedValueCapture: @unchecked Sendable {
    struct Values: Sendable {
        var attackerApplied: [String]
        var defenderApplied: [String]
        var attackerIgnored: [String]
        var defenderIgnored: [String]
    }

    @TaskLocal static var current: JudgeSpeedValueCapture?

    private let lock = NSLock()
    private var storage: [Int: Values] = [:]

    func store(_ values: [Int: Values]) {
        lock.lock()
        defer { lock.unlock() }
        storage = values
    }

    func values(at position: Int) -> Values? {
        lock.lock()
        defer { lock.unlock() }
        return storage[position]
    }
}

struct JudgeSpeedValueMiddleware: ClientMiddleware {
    /// 応答本文を読む上限(判定の応答は小さい)。
    private static let bodyLimit = 8 * 1024 * 1024
    private static let keys = ["attackerSpeedApplied", "defenderSpeedApplied", "attackerSpeedIgnored", "defenderSpeedIgnored"]

    func intercept(
        _ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String,
        next: @Sendable (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        let (response, responseBody) = try await next(request, body, baseURL)
        guard let capture = JudgeSpeedValueCapture.current, response.status.code == 200, let responseBody else {
            return (response, responseBody)
        }
        let data = try await Data(collecting: responseBody, upTo: Self.bodyLimit)
        guard let rewritten = Self.rewrite(data, capture: capture) else {
            return (response, HTTPBody(data))
        }
        var fields = response.headerFields
        fields[.contentLength] = String(rewritten.count)
        var changed = response
        changed.headerFields = fields
        return (changed, HTTPBody(rewritten))
    }

    /// 退避して空配列にした本文。触らない(元の本文のまま)なら nil。
    private static func rewrite(_ data: Data, capture: JudgeSpeedValueCapture) -> Data? {
        guard var root = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
            var matchups = root["matchups"] as? [Any]
        else { return nil }
        var stored: [Int: JudgeSpeedValueCapture.Values] = [:]
        for (position, element) in matchups.enumerated() {
            guard var matchup = element as? [String: Any] else { continue }
            var lists: [[String]] = []
            for key in keys {
                guard let list = matchup[key] as? [Any] else { break }
                let strings = list.compactMap { $0 as? String }
                guard strings.count == list.count else { break }
                lists.append(strings)
            }
            guard lists.count == keys.count else { continue }
            stored[position] = .init(
                attackerApplied: lists[0], defenderApplied: lists[1], attackerIgnored: lists[2], defenderIgnored: lists[3])
            for key in keys { matchup[key] = [String]() }
            matchups[position] = matchup
        }
        guard !stored.isEmpty else { return nil }
        root["matchups"] = matchups
        guard let output = try? JSONSerialization.data(withJSONObject: root) else { return nil }
        capture.store(stored)
        return output
    }
}
