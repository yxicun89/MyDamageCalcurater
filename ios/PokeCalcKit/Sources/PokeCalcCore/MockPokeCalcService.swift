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

    public func move(id: String) async throws -> Move {
        guard let entry = fixtures.moves.first(where: { $0.id == id }) else {
            throw Self.notFoundError("技", id)
        }
        return try Self.domainMove(entry)
    }

    /// `getMovesByIds` のモック版(ADR-0501「getMovesByIds による構築編集の技の一括解決」1章)。
    /// `ids` を重複除去した順で `fixtures.moves` から引き、無い ID は省く(`move(id:)` と違い
    /// `notFoundError` にしない。まとめ取りは部分一致をエラーにしない、という契約の規則どおり)。
    public func moves(ids: [String]) async throws -> [Move] {
        var seen = Set<String>()
        return try ids
            .filter { seen.insert($0).inserted }
            .compactMap { id in fixtures.moves.first(where: { $0.id == id }) }
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
        guard let move = fixtures.moves.first(where: { $0.id == request.moveId }) else {
            throw Self.notFoundError("技", request.moveId)
        }
        guard fixtures.species.contains(where: { $0.key == request.attacker.speciesKey }) else {
            throw Self.notFoundError("種族", request.attacker.speciesKey)
        }
        guard fixtures.species.contains(where: { $0.key == request.defender.speciesKey }) else {
            throw Self.notFoundError("種族", request.defender.speciesKey)
        }
        let category = try Self.domainMoveCategory(move.category)
        var result = try cannedResult(forKey: Self.singleResultKey, category: category)
        result.unsupported = try Self.moveMarks(move, category: category)
            + [itemMark(itemId: request.attacker.itemId, target: .attackerItem)].compactMap { $0 }
            + [itemMark(itemId: request.defender.itemId, target: .defenderItem)].compactMap { $0 }
        return result
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
        let domainNatures = try fixtures.natures.map(Self.domainNature)
        // 技・攻撃側の持ち物の印は全行共通(ADR-0501「P6-17」4章)。
        let commonMarks = try Self.moveMarks(move, category: category)
            + [itemMark(itemId: request.attacker.itemId, target: .attackerItem)].compactMap { $0 }

        // 行の順序は「プリセット優先」(presets × itemVariants。plan.md P3-1)。
        var rows: [BulkCalcRow] = []
        for preset in presets {
            let cannedRow = try cannedResult(forKey: preset.rawValue, category: category)
            let stats = try cannedDefenderStats(forKey: preset.rawValue)
            let defender = Self.defenderForRow(preset: preset, natures: domainNatures, stats: stats)
            let label = Self.presetLabel(preset)
            for itemId in itemVariants {
                var result = cannedRow
                result.unsupported = commonMarks + [itemMark(itemId: itemId, target: .defenderItem)].compactMap { $0 }
                rows.append(BulkCalcRow(preset: preset, presetLabel: label, itemId: itemId, defender: defender, result: result))
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

    /// 防御側プリセットの SP・性格補正のカタログ(ADR-0009)。UI 文言ではなくデータの規則なので、
    /// `presetLabel` と同じ理由でここに1か所持つ(辞書 + `??` ではなく網羅 switch)。
    private static func defenderSPAndNature(_ preset: DefenderPreset) -> (sp: StatBlock, nature: NatureModifier) {
        let full = 32
        let zero = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        switch preset {
        case .none:
            return (zero, NatureModifier())
        case .hp:
            return (StatBlock(hp: full, atk: 0, def: 0, spa: 0, spd: 0, spe: 0), NatureModifier())
        case .hbBoost:
            return (StatBlock(hp: full, atk: 0, def: 0, spa: 0, spd: 0, spe: 0), NatureModifier(plus: .def, minus: .atk))
        case .hb:
            return (StatBlock(hp: full, atk: 0, def: full, spa: 0, spd: 0, spe: 0), NatureModifier())
        case .hbFull:
            return (StatBlock(hp: full, atk: 0, def: full, spa: 0, spd: 0, spe: 0), NatureModifier(plus: .def, minus: .atk))
        case .hdBoost:
            return (StatBlock(hp: full, atk: 0, def: 0, spa: 0, spd: 0, spe: 0), NatureModifier(plus: .spd, minus: .atk))
        case .hd:
            return (StatBlock(hp: full, atk: 0, def: 0, spa: 0, spd: full, spe: 0), NatureModifier())
        case .hdFull:
            return (StatBlock(hp: full, atk: 0, def: 0, spa: 0, spd: full, spe: 0), NatureModifier(plus: .spd, minus: .atk))
        }
    }

    /// 一括計算の1行の `defender`(openapi `BulkCalcRow.defender`。必須。ADR-0200 §1)。
    /// SP・性格補正はカタログどおり、natureId は補正が一致するモックの性格を ID 昇順の最初で選ぶ
    /// (ADR-0200 §2)、実数値(stats)はフィクスチャの値をそのまま使う(式は書かない。coding-rules §2)。
    private static func defenderForRow(preset: DefenderPreset, natures: [Nature], stats: StatBlock) -> BulkDefender {
        let (sp, nature) = defenderSPAndNature(preset)
        return BulkDefender(sp: sp, nature: nature, natureId: Self.natureID(for: nature, in: natures), stats: stats)
    }

    /// ADR-0200 §2 の natureId の規則: (plus, minus) が一致するマスタの性格のうち ID 昇順の最初。無ければ nil。
    private static func natureID(for nature: NatureModifier, in natures: [Nature]) -> String? {
        natures
            .filter { $0.plus == nature.plus && $0.minus == nature.minus }
            .map(\.id)
            .sorted()
            .first
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
        let result = try cannedResult(forKey: Self.singleResultKey, category: category)
        let domainNatures = try fixtures.natures.map(Self.domainNature)
        // 技の印は全候補共通。持ち物は side で既知側・候補側それぞれの target を決める
        // (side=defender: 既知=攻撃側・候補=防御側、side=attacker: 既知=防御側・候補=攻撃側。ADR-0501「P6-17」4章)。
        let moveMarksList = try Self.moveMarks(move, category: category)
        let knownTarget: UnsupportedTarget = request.side == .defender ? .attackerItem : .defenderItem
        let candidateTarget: UnsupportedTarget = request.side == .defender ? .defenderItem : .attackerItem
        let knownItemMark = itemMark(itemId: request.known.itemId, target: knownTarget)

        // §R1: 候補 = (性格クラス, 持ち物)。空の持ち物候補は「持ち物なし」の1通り。
        let items = request.itemCandidates.isEmpty ? [String?.none] : request.itemCandidates
        var candidates: [ReverseCandidate] = []
        for natureClass in NatureClass.allCases {
            let nature = Self.representativeNature(for: natureClass, stat: stat)
            let natureId = Self.natureID(for: nature, in: domainNatures)
            for itemId in items {
                let candidateItemMark = itemMark(itemId: itemId, target: candidateTarget)
                var attackerItemMark: UnsupportedMark?
                var defenderItemMark: UnsupportedMark?
                if knownTarget == .attackerItem { attackerItemMark = knownItemMark } else { defenderItemMark = knownItemMark }
                if candidateTarget == .attackerItem { attackerItemMark = candidateItemMark } else { defenderItemMark = candidateItemMark }
                candidates.append(ReverseCandidate(
                    natureClass: natureClass, nature: nature, natureId: natureId, itemId: itemId,
                    ranges: [SPRange(min: Self.minSP, max: Self.maxSP)],
                    spCount: Self.maxSP - Self.minSP + 1,
                    exact: true, mismatch: 0, support: Self.mockSupport,
                    minPercent: result.minPercent, maxPercent: result.maxPercent,
                    unsupported: moveMarksList + [attackerItemMark, defenderItemMark].compactMap { $0 }
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

    /// ADR-0010 §R: 性格クラスの代表補正。neutral は無補正、plus は「関連ステータス +10% / atk -10%」
    /// (関連ステータスが atk 自身のときだけ spa を下降にする。ADR-0010 §R1、engine の natureForClass と同じ規則)。
    private static func representativeNature(for natureClass: NatureClass, stat: StatKey) -> NatureModifier {
        switch natureClass {
        case .neutral:
            return NatureModifier()
        case .plus:
            let minus: StatKey = stat == .atk ? .spa : .atk
            return NatureModifier(plus: stat, minus: minus)
        }
    }

    // MARK: - 未対応の印(ADR-0123。ADR-0501「P6-17」4章)

    /// 技の `mechanisms` から `target: move` の印を作る(昇順)。変化技には付けない。
    /// 未知の値は `fixtureInvalid`(フィクスチャの不整合。ADR-0123 の機構13種のどれかのはず)。
    private static func moveMarks(_ move: MockFixtures.MoveEntry, category: MoveCategory) throws -> [UnsupportedMark] {
        guard category != .status, let mechanisms = move.mechanisms else { return [] }
        return try mechanisms.sorted().map { mechanism in
            guard let reason = UnsupportedReason(rawValue: mechanism) else {
                throw PokeCalcError(code: PokeCalcError.Code.fixtureInvalid, message: "未知の mechanism: \(mechanism)")
            }
            return UnsupportedMark(target: .move, reason: reason, id: move.id)
        }
    }

    /// `itemId` がフィクスチャで `unsupportedEffect: true` なら `target` の印、無ければ nil
    /// (`itemId` が nil、またはフィクスチャに無い ID のときも nil。持ち物の実在チェックは calc の入力検証の役割ではない)。
    private func itemMark(itemId: String?, target: UnsupportedTarget) -> UnsupportedMark? {
        guard let itemId, let entry = fixtures.items.first(where: { $0.id == itemId }), entry.unsupportedEffect == true else {
            return nil
        }
        return UnsupportedMark(target: target, reason: .unsupportedEffect, id: itemId)
    }

    // MARK: - 決め打ちの計算結果

    private func cannedResultEntry(forKey key: String) throws -> MockFixtures.CalcResultEntry {
        guard let entry = fixtures.calcResultsByKey[key] else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureMissing, message: "計算結果フィクスチャが無い: \(key)")
        }
        return entry
    }

    /// openapi `BulkCalcRow.defender.stats`(実数値)。フィクスチャの値をそのまま使う(式は書かない)。
    private func cannedDefenderStats(forKey key: String) throws -> StatBlock {
        let entry = try cannedResultEntry(forKey: key)
        guard let stats = entry.stats else {
            throw PokeCalcError(code: PokeCalcError.Code.fixtureMissing, message: "実数値フィクスチャが無い: \(key)")
        }
        return StatBlock(hp: stats.hp, atk: stats.atk, def: stats.def, spa: stats.spa, spd: stats.spd, spe: stats.spe)
    }

    private func cannedResult(forKey key: String, category: MoveCategory) throws -> CalcResult {
        let entry = try cannedResultEntry(forKey: key)
        return CalcResult(
            rolls: entry.rolls, minDamage: entry.minDamage, maxDamage: entry.maxDamage,
            minPercent: entry.minPercent, maxPercent: entry.maxPercent, defenderHP: entry.defenderHP,
            effectiveness: entry.effectiveness, stab: entry.stab, category: category,
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
