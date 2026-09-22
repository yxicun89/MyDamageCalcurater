import Foundation

/// サーバー(P3)ができるまで画面を動かすための `PokeCalcService` 実装(ADR-0500 §4)。
///
/// 架空データ(`Resources/*.json`。名前はすべて「テスト」で始まる)を返すだけで、
/// **ダメージ計算はしない**(engine の式を Swift に写すと単一の正が2つできてしまう。
/// CLAUDE.md 絶対ルール2・coding-rules §2)。一括計算・逆算・1対1は、決め打ちの計算結果
/// フィクスチャを要求の形(プリセット・持ち物の組合せ)に合わせて返すだけにする。
public struct MockPokeCalcService: PokeCalcService {
    /// 探索する SP の範囲(CLAUDE.md ドメイン規約: SP は 1 ステータス最大 32)。
    private static let minSP = 0
    private static let maxSP = 32
    /// モックが返す `support`(観測を説明できるロールの延べ数)の決め打ち値。
    /// 実際の一致度は計算しない(このファイルの方針)ので、範囲内の妥当な値を固定で返す。
    private static let mockSupport = 16
    /// ADR-0010 §R1: 逆算は H を defender=32・attacker=0 で仮定する。
    private static let assumedDefenderHPSP = 32
    private static let assumedAttackerHPSP = 0
    /// `calc-results.json` の1対1計算用エントリのキー(プリセットに属さない)。
    private static let singleResultKey = "single"

    private let fixtures: MockFixtures

    public init() throws {
        fixtures = try MockFixtures.load()
    }

    /// フィクスチャ JSON の URL(架空データ検査用。`MockPokeCalcServiceTests` が生の JSON を
    /// 全走査するために使う)。
    public static func fixtureURLs() throws -> [URL] {
        try MockFixtures.fixtureURLs()
    }

    // MARK: - 検索・マスタ参照

    public func searchSpecies(query: String, limit: Int) async throws -> [SpeciesSummary] {
        try matchingByPrefix(fixtures.species, query: query, limit: limit, nameJa: { $0.nameJa })
            .map(Self.domainSpeciesSummary)
    }

    public func species(key: String) async throws -> SpeciesDetail {
        guard let entry = fixtures.species.first(where: { $0.key == key }) else {
            throw Self.notFoundError("種族", key)
        }
        return try Self.domainSpeciesDetail(entry)
    }

    public func searchMoves(query: String, limit: Int) async throws -> [Move] {
        try matchingByPrefix(fixtures.moves, query: query, limit: limit, nameJa: { $0.nameJa })
            .map(Self.domainMove)
    }

    public func searchItems(query: String, limit: Int) async throws -> [Item] {
        matchingByPrefix(fixtures.items, query: query, limit: limit, nameJa: { $0.nameJa })
            .map { Item(id: $0.id, nameJa: $0.nameJa) }
    }

    public func natures() async throws -> [Nature] {
        try fixtures.natures.map(Self.domainNature)
    }

    /// 日本語名の前方一致(空なら全件)。`limit` で切る(openapi の検索と同じ規則)。
    private func matchingByPrefix<T>(
        _ items: [T], query: String, limit: Int, nameJa: (T) -> String
    ) -> [T] {
        let matched = query.isEmpty ? items : items.filter { nameJa($0).hasPrefix(query) }
        return Array(matched.prefix(limit))
    }

    // MARK: - 計算

    public func calcDamage(_ request: CalcRequest) async throws -> CalcResult {
        guard fixtures.moves.contains(where: { $0.id == request.moveId }) else {
            throw Self.notFoundError("技", request.moveId)
        }
        guard fixtures.species.contains(where: { $0.key == request.attacker.speciesKey }) else {
            throw Self.notFoundError("種族", request.attacker.speciesKey)
        }
        guard fixtures.species.contains(where: { $0.key == request.defender.speciesKey }) else {
            throw Self.notFoundError("種族", request.defender.speciesKey)
        }
        return try cannedResult(forKey: Self.singleResultKey)
    }

    public func calcBulk(_ request: BulkCalcRequest) async throws -> BulkCalcResult {
        guard let move = fixtures.moves.first(where: { $0.id == request.moveId }) else {
            throw Self.notFoundError("技", request.moveId)
        }
        guard fixtures.species.contains(where: { $0.key == request.attacker.speciesKey }) else {
            throw Self.notFoundError("種族", request.attacker.speciesKey)
        }
        guard fixtures.species.contains(where: { $0.key == request.defenderSpeciesKey }) else {
            throw Self.notFoundError("種族", request.defenderSpeciesKey)
        }
        let category = try Self.domainMoveCategory(move.category)
        let presets = request.presets.isEmpty ? Self.defaultPresets(for: category) : request.presets
        // 省略時は「素の1通り」(openapi `BulkCalcRequest.itemVariants` の description)。
        let itemVariants = request.itemVariants.isEmpty ? [String?.none] : request.itemVariants

        // 行の順序は「プリセット優先」(presets × itemVariants。plan.md P3-1)。
        var rows: [BulkCalcRow] = []
        for preset in presets {
            let result = try cannedResult(forKey: preset.rawValue)
            let label = Self.presetLabel(preset)
            for itemId in itemVariants {
                rows.append(BulkCalcRow(preset: preset, presetLabel: label, itemId: itemId, result: result))
            }
        }
        return BulkCalcResult(defenderSpeciesKey: request.defenderSpeciesKey, rows: rows)
    }

    /// openapi `BulkCalcRequest.presets` の description / ADR-0009: 省略(空を含む)時の既定セット。
    private static func defaultPresets(for category: MoveCategory) -> [DefenderPreset] {
        switch category {
        case .physical: return [.none, .hp, .hbBoost, .hb, .hbFull]
        case .special: return [.none, .hp, .hdBoost, .hd, .hdFull]
        case .status: return [.none, .hp]
        }
    }

    /// 防御側プリセットの表示名(ADR-0009)。UI 文言なのでマスタ扱いではなく、ここに1か所で持つ。
    /// 辞書 + `??` ではなく網羅 switch にして、プリセットが増えたときにコンパイルエラーで気付けるようにする。
    private static func presetLabel(_ preset: DefenderPreset) -> String {
        switch preset {
        case .none: return "無振り"
        case .hp: return "H振り"
        case .hbBoost: return "H振り+B補正"
        case .hb: return "HB振り"
        case .hbFull: return "HB特化"
        case .hdBoost: return "H振り+D補正"
        case .hd: return "HD振り"
        case .hdFull: return "HD特化"
        }
    }

    // MARK: - 逆算(ADR-0010 §R)

    public func reverse(_ request: ReverseRequest) async throws -> ReverseResult {
        // openapi `ReverseRequest.observations` は minItems 1。
        guard !request.observations.isEmpty else {
            throw PokeCalcError(code: PokeCalcError.Code.invalidInput, message: "observations が空")
        }
        guard fixtures.species.contains(where: { $0.key == request.unknownSpeciesKey }) else {
            throw Self.notFoundError("種族", request.unknownSpeciesKey)
        }
        guard let move = fixtures.moves.first(where: { $0.id == request.moveId }) else {
            throw Self.notFoundError("技", request.moveId)
        }
        let category = try Self.domainMoveCategory(move.category)
        let stat = Self.reverseStat(side: request.side, category: category)
        let assumedHPSP = request.side == .defender ? Self.assumedDefenderHPSP : Self.assumedAttackerHPSP
        let result = try cannedResult(forKey: Self.singleResultKey)

        // §R1: 候補 = (性格クラス, 持ち物)。空の持ち物候補は「持ち物なし」の1通り。
        let items = request.itemCandidates.isEmpty ? [String?.none] : request.itemCandidates
        var candidates: [ReverseCandidate] = []
        for natureClass in NatureClass.allCases {
            for itemId in items {
                candidates.append(ReverseCandidate(
                    natureClass: natureClass, itemId: itemId,
                    ranges: [SPRange(min: Self.minSP, max: Self.maxSP)],
                    spCount: Self.maxSP - Self.minSP + 1,
                    exact: true, mismatch: 0, support: Self.mockSupport,
                    minPercent: result.minPercent, maxPercent: result.maxPercent
                ))
            }
        }
        return ReverseResult(
            side: request.side, stat: stat, assumedHPSP: assumedHPSP,
            candidates: candidates, exactCount: candidates.filter(\.exact).count
        )
    }

    /// ADR-0010 §2: 関連ステータスは側と技の分類で決まる(変化技は物理と同じ扱い)。
    private static func reverseStat(side: ReverseSide, category: MoveCategory) -> StatKey {
        switch (side, category) {
        case (.defender, .special): return .spd
        case (.defender, .physical), (.defender, .status): return .def
        case (.attacker, .special): return .spa
        case (.attacker, .physical), (.attacker, .status): return .atk
        }
    }

    // MARK: - 決め打ちの計算結果

    private func cannedResult(forKey key: String) throws -> CalcResult {
        guard let entry = fixtures.calcResultsByKey[key] else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureMissing, message: "計算結果フィクスチャが無い: \(key)")
        }
        return CalcResult(
            rolls: entry.rolls, minDamage: entry.minDamage, maxDamage: entry.maxDamage,
            minPercent: entry.minPercent, maxPercent: entry.maxPercent, defenderHP: entry.defenderHP,
            effectiveness: entry.effectiveness, stab: entry.stab,
            ko: KOChance(
                hits: entry.ko.hits, guaranteed: entry.ko.guaranteed,
                chancePercent: entry.ko.chancePercent, displayChancePercent: entry.ko.displayChancePercent
            )
        )
    }

    // MARK: - フィクスチャ → ドメイン

    private static func notFoundError(_ kind: String, _ id: String) -> PokeCalcError {
        PokeCalcError(code: PokeCalcError.Code.notFound, message: "\(kind)が見つからない: \(id)")
    }

    private static func domainPokeType(_ raw: String) throws -> PokeType {
        guard let type = PokeType(rawValue: raw) else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureInvalid, message: "未知のタイプ: \(raw)")
        }
        return type
    }

    private static func domainMoveCategory(_ raw: String) throws -> MoveCategory {
        guard let category = MoveCategory(rawValue: raw) else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureInvalid, message: "未知の分類: \(raw)")
        }
        return category
    }

    private static func domainStatKey(_ raw: String) throws -> StatKey {
        guard let key = StatKey(rawValue: raw) else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureInvalid, message: "未知のステータス: \(raw)")
        }
        return key
    }

    private static func domainSpeciesSummary(_ entry: MockFixtures.SpeciesEntry) throws -> SpeciesSummary {
        SpeciesSummary(
            key: entry.key, dexNo: entry.dexNo, form: entry.form, nameJa: entry.nameJa,
            types: try entry.types.map(domainPokeType)
        )
    }

    private static func domainSpeciesDetail(_ entry: MockFixtures.SpeciesEntry) throws -> SpeciesDetail {
        SpeciesDetail(
            key: entry.key, dexNo: entry.dexNo, form: entry.form, nameJa: entry.nameJa,
            types: try entry.types.map(domainPokeType),
            baseStats: StatBlock(
                hp: entry.baseStats.hp, atk: entry.baseStats.atk, def: entry.baseStats.def,
                spa: entry.baseStats.spa, spd: entry.baseStats.spd, spe: entry.baseStats.spe
            ),
            abilities: entry.abilities.map { Ability(id: $0.id, nameJa: $0.nameJa) },
            learnset: entry.learnset
        )
    }

    private static func domainMove(_ entry: MockFixtures.MoveEntry) throws -> Move {
        Move(
            id: entry.id, nameJa: entry.nameJa, type: try domainPokeType(entry.type),
            category: try domainMoveCategory(entry.category), power: entry.power, priority: entry.priority
        )
    }

    private static func domainNature(_ entry: MockFixtures.NatureEntry) throws -> Nature {
        Nature(
            id: entry.id, nameJa: entry.nameJa,
            plus: try entry.plus.map(domainStatKey),
            minus: try entry.minus.map(domainStatKey)
        )
    }
}
