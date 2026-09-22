import Foundation

// LocalTeamStore: `TeamStore` の端末内実装(P6-2c・ADR-0500 §4)。
//
// 1つの UserDefaults キーの下に `[Team]` を丸ごと JSON で保存する(team ごとに別キーにしない。
// 索引キーと実体の不整合を起こしうる複数キー方式より一貫する。ADR-0501「P6-2c」2章の判断)。

/// UserDefaults に JSON で保存する `TeamStore`(`ClientIdentity(defaults:)` と同じ、保存先を
/// 注入できる形。テストは専用の UserDefaults suite を使う)。
public actor LocalTeamStore: TeamStore {
    /// `ClientIdentity.deviceIDDefaultsKey`(`"PokeCalcDeviceID"`)と衝突しないキー。
    public static let teamsDefaultsKey = "PokeCalcTeams"

    /// `UserDefaults` は Swift Concurrency の `Sendable` に適合しない(SDK が明示的に unavailable と
    /// している)が、Apple のドキュメントどおりスレッドセーフなクラスなので `nonisolated(unsafe)` で
    /// 安全に共有する(`ClientIdentity` が構造体で同じ値をそのまま保持するのと同じ前提)。
    private nonisolated(unsafe) let defaults: UserDefaults

    // TODO(未解決・作業中断): `UserDefaults`(非 Sendable)をアクターの init へ渡す箇所で
    // 「sending 'self.defaults' risks causing data races」が取れていない。`nonisolated(unsafe)` を
    // プロパティに付けても解消しない(呼び出し側の region-based sending チェックは宣言型が
    // Sendable かどうかだけで決まり、内部の isolation 属性では変わらないため)。次の一手候補:
    // 1) `UserDefaults` を `@unchecked Sendable` の小さな箱型で包んでから actor へ渡す、
    // 2) `actor` をやめて内部ロック付きの `final class ... : TeamStore, @unchecked Sendable` にする
    //   (ADR-0501「P6-2c」2章は `actor` を明示しているので、2) を選ぶなら ADR 追記が要る)。
    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public func list() async throws -> [Team] {
        try loadTeams()
    }

    public func get(id: String) async throws -> Team? {
        try loadTeams().first(where: { $0.id == id })
    }

    public func save(_ team: Team) async throws {
        if let violation = TeamValidator.firstViolation(in: team) {
            throw violation
        }
        var teams = try loadTeams()
        if let index = teams.firstIndex(where: { $0.id == team.id }) {
            teams[index] = team
        } else {
            teams.append(team)
        }
        try persist(teams)
    }

    public func delete(id: String) async throws {
        var teams = try loadTeams()
        teams.removeAll(where: { $0.id == id })
        try persist(teams)
    }

    // MARK: - 内部

    /// 保存が無ければ空配列(黙って空にする「未保存」は正常な状態)。値はあるが JSON として
    /// `[Team]` にデコードできないときだけ `teamStoreCorrupted` を投げる(データ破損に気付けるように)。
    private func loadTeams() throws -> [Team] {
        guard let stored = defaults.object(forKey: Self.teamsDefaultsKey) else { return [] }
        let data: Data
        switch stored {
        case let value as Data:
            data = value
        case let value as String:
            // `UserDefaults.set(_:forKey:)` で文字列として書き込まれた値(テストの壊れたデータ、
            // または将来の互換性)も同じ経路で「壊れている」と判定できるよう Data 化してから試す。
            guard let converted = value.data(using: .utf8) else {
                throw PokeCalcError(code: PokeCalcError.Code.teamStoreCorrupted, message: "保存された構築データの形式が不正です")
            }
            data = converted
        default:
            throw PokeCalcError(code: PokeCalcError.Code.teamStoreCorrupted, message: "保存された構築データの形式が不正です")
        }
        do {
            return try JSONDecoder().decode([Team].self, from: data)
        } catch {
            throw PokeCalcError(code: PokeCalcError.Code.teamStoreCorrupted, message: "保存された構築データを読み込めません")
        }
    }

    private func persist(_ teams: [Team]) throws {
        let data = try JSONEncoder().encode(teams)
        defaults.set(data, forKey: Self.teamsDefaultsKey)
    }
}
