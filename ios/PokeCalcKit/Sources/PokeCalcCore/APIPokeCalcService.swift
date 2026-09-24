import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import PokeCalcAPI

/// `PokeCalcService` を生成クライアント(`PokeCalcAPI.Client`)で実装する(ADR-0500 §3)。
///
/// 生成型(`Components.Schemas.*`) ↔ ドメイン(`PokeCalcCore` の型)の写像は、この1ファイルに
/// 閉じる。契約(`api/openapi.yaml`)が変わったら、直すのはここだけでよい。
/// 全操作に `X-Device-Id` / `X-Session-Id`(必須ヘッダー)を付ける。
public struct APIPokeCalcService: PokeCalcService {
    private let client: Client
    private let identity: ClientIdentity

    public init(client: Client, identity: ClientIdentity) {
        self.client = client
        self.identity = identity
    }

    /// `URLSession` を使う既定の transport で `Client` を組み立てる便利イニシャライザ。
    /// アプリ(View 層)は `PokeCalcAPI` / `OpenAPIURLSession` に直接依存せず、この1つの
    /// 初期化子だけで API 実装を作れる(ADR-0500 §1「View 以外はパッケージに置く」)。
    public init(baseURL: URL, identity: ClientIdentity) {
        self.init(client: Client(serverURL: baseURL, transport: URLSessionTransport()), identity: identity)
    }

    // MARK: - 検索・マスタ参照

    public func searchSpecies(query: String, limit: Int) async throws -> [SpeciesSummary] {
        let output = try await send {
            try await client.searchSpecies(.init(
                query: .init(q: query, limit: limit),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try ok.body.json.map(Self.domainSpeciesSummary)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func species(key: String) async throws -> SpeciesDetail {
        let output = try await send {
            try await client.getSpecies(.init(
                path: .init(key: key),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainSpeciesDetail(ok.body.json)
        case .notFound(let response):
            // 404 は他の操作の `Components.Responses._Error` と違い、getSpecies だけの inline body
            // (`#/paths/.../404` を `#/responses/Error` の参照ではなく直接定義しているため。ADR-0105 の
            // openapi 更新で追加。他の 404(natures 等)は `.default` 経由のまま)。
            throw try Self.domainErrorFromSchema(response.body.json)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func searchMoves(query: String, limit: Int) async throws -> [Move] {
        let output = try await send {
            try await client.searchMoves(.init(
                query: .init(q: query, limit: limit),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try ok.body.json.map(Self.domainMove)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func move(id: String) async throws -> Move {
        let output = try await send {
            try await client.getMove(.init(
                path: .init(key: id),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainMove(ok.body.json)
        case .notFound(let response):
            // 404 は species(key:) と同じ inline body(ADR-0501「issue #68 の残り」2章)。
            throw try Self.domainErrorFromSchema(response.body.json)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    /// `getMovesByIds`(`move(id:)` の複数版。ADR-0501「getMovesByIds による構築編集の技の一括解決」1章)。
    /// `ids` を重複除去し、空なら通信せず `[]` を返す。`RequestLimits.maxMoveBatchIds` 件ずつに分割して
    /// `client.getMovesByIds` を順に呼び、応答を渡した順に連結する(並行に投げる必要は無い。1章「実装者への
    /// 注意」)。404 は無い(マスタに無い ID は 200 の応答から黙って省かれるだけ)ので `.ok` 以外は
    /// 他の pokedex 操作と同じ写像(`domainErrorFromSchema` / `domainError`)。
    public func moves(ids: [String]) async throws -> [Move] {
        var seen = Set<String>()
        let dedupedIds = ids.filter { seen.insert($0).inserted }
        guard !dedupedIds.isEmpty else { return [] }

        var result: [Move] = []
        for start in stride(from: 0, to: dedupedIds.count, by: RequestLimits.maxMoveBatchIds) {
            let end = min(start + RequestLimits.maxMoveBatchIds, dedupedIds.count)
            let chunk = Array(dedupedIds[start..<end])
            let output = try await send {
                try await client.getMovesByIds(.init(
                    query: .init(ids: chunk),
                    headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
                ))
            }
            switch output {
            case .ok(let ok):
                result.append(contentsOf: try ok.body.json.map(Self.domainMove))
            case .serviceUnavailable(let response):
                throw try Self.domainErrorFromSchema(response.body.json)
            case .default(_, let error):
                throw try Self.domainError(error)
            }
        }
        return result
    }

    public func searchItems(query: String, limit: Int) async throws -> [Item] {
        let output = try await send {
            try await client.searchItems(.init(
                query: .init(q: query, limit: limit),
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try ok.body.json.map { Item(id: $0.id, nameJa: $0.nameJa) }
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func natures() async throws -> [Nature] {
        let output = try await send {
            try await client.listNatures(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID)
            ))
        }
        switch output {
        case .ok(let ok):
            return try ok.body.json.map(Self.domainNature)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    // MARK: - 計算

    public func calcDamage(_ request: CalcRequest) async throws -> CalcResult {
        let output = try await send {
            try await client.calcDamage(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedCalcRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainCalcResult(ok.body.json)
        case .badRequest(let error):
            throw try Self.domainError(error)
        case .internalServerError(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    public func calcBulk(_ request: BulkCalcRequest) async throws -> BulkCalcResult {
        let output = try await send {
            try await client.calcBulk(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedBulkCalcRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainBulkCalcResult(ok.body.json)
        case .badRequest(let error):
            throw try Self.domainError(error)
        case .internalServerError(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    /// 逆算(ADR-0010 §R・P3-1 で契約が確定)。要求・応答は1か所で写す(ADR-0500 §3)。
    public func reverse(_ request: ReverseRequest) async throws -> ReverseResult {
        let output = try await send {
            try await client.calcReverse(.init(
                headers: .init(xDeviceId: identity.deviceID, xSessionId: identity.sessionID),
                body: .json(Self.generatedReverseRequest(request))
            ))
        }
        switch output {
        case .ok(let ok):
            return try Self.domainReverseResult(ok.body.json)
        case .badRequest(let error):
            throw try Self.domainError(error)
        case .internalServerError(let error):
            throw try Self.domainError(error)
        case .serviceUnavailable(let response):
            throw try Self.domainErrorFromSchema(response.body.json)
        case .default(_, let error):
            throw try Self.domainError(error)
        }
    }

    // MARK: - 通信失敗 → PokeCalcError

    /// 契約どおりの応答を得られなかった失敗を `PokeCalcError` にする: 接続できない(`transport`)と
    /// 応答をデコードできない(`decode`)を区別する。キャンセルは包まずに投げ直す。
    /// 応答があってエラー body を持つケース(400/404/default)はここでは捕まえない
    /// (呼び出し側が `switch output` で明示的に写す)。
    private func send<T>(_ operation: () async throws -> T) async throws -> T {
        do {
            return try await operation()
        } catch is CancellationError {
            // タスクのキャンセルはサービスの失敗ではない。`PokeCalcError` に包まず、
            // 呼び出し側(Swift Concurrency)がキャンセルとして扱えるようにそのまま投げ直す。
            throw CancellationError()
        } catch {
            if Self.isCancellation(error) {
                throw CancellationError()
            }
            if Self.isDecodingFailure(error) {
                // 応答は受け取れたが期待した形にデコードできない。接続できない(`transport`)とは
                // 原因が違う(契約のずれ・サーバー側のバグ等)ので、別のコードで区別する。
                throw PokeCalcError(code: PokeCalcError.Code.decode, message: "\(error)")
            }
            throw PokeCalcError(code: PokeCalcError.Code.transport, message: "\(error)")
        }
    }

    /// `Client` は transport/デコードの失敗を `OpenAPIRuntime.ClientError` に包んで投げる。
    /// 元の原因(`underlyingError`)を見て、キャンセルとデコード失敗をここで区別する。
    private static func isCancellation(_ error: any Error) -> Bool {
        guard let clientError = error as? ClientError else { return false }
        if clientError.underlyingError is CancellationError { return true }
        // URLSession 経由のキャンセルは CancellationError ではなく URLError(.cancelled) として届く。
        if let urlError = clientError.underlyingError as? URLError, urlError.code == .cancelled { return true }
        return false
    }

    private static func isDecodingFailure(_ error: any Error) -> Bool {
        guard let clientError = error as? ClientError else { return false }
        return clientError.underlyingError is DecodingError
    }

    private static func domainError(_ response: Components.Responses._Error) throws -> PokeCalcError {
        try Self.domainErrorFromSchema(response.body.json)
    }

    /// 503 は5つの pokedex 操作では `Components.Responses._Error` ではなく、そのパスだけの
    /// inline body(`Output.ServiceUnavailable.Body.json: Components.Schemas._Error`)で届く
    /// (openapi の `#/responses/Error` を再利用していないため)。同じ形なので写像は共通にする。
    private static func domainErrorFromSchema(_ error: Components.Schemas._Error) -> PokeCalcError {
        PokeCalcError(code: error.code.rawValue, message: error.message)
    }

    // MARK: - 応答 → ドメイン

    private static func domainSpeciesSummary(_ summary: Components.Schemas.SpeciesSummary) -> SpeciesSummary {
        SpeciesSummary(
            key: summary.key, dexNo: summary.dexNo, form: summary.form, nameJa: summary.nameJa,
            types: summary.types.map(domainPokeType)
        )
    }

    private static func domainSpeciesDetail(_ detail: Components.Schemas.SpeciesDetail) -> SpeciesDetail {
        SpeciesDetail(
            key: detail.value1.key, dexNo: detail.value1.dexNo, form: detail.value1.form,
            nameJa: detail.value1.nameJa, types: detail.value1.types.map(domainPokeType),
            baseStats: domainStatBlock(detail.value2.baseStats),
            abilities: detail.value2.abilities.map { Ability(id: $0.id, nameJa: $0.nameJa) },
            learnset: detail.value2.learnset ?? []
        )
    }

    private static func domainMove(_ move: Components.Schemas.Move) -> Move {
        Move(
            id: move.id, nameJa: move.nameJa, type: domainPokeType(move._type),
            category: domainMoveCategory(move.category), power: move.power, priority: move.priority ?? 0
        )
    }

    private static func domainNature(_ nature: Components.Schemas.Nature) -> Nature {
        Nature(
            id: nature.id, nameJa: nature.nameJa,
            plus: nature.plus.map { domainStatKey($0.value1) },
            minus: nature.minus.map { domainStatKey($0.value1) }
        )
    }

    private static func domainStatBlock(_ block: Components.Schemas.StatBlock) -> StatBlock {
        StatBlock(hp: block.hp, atk: block.atk, def: block.def, spa: block.spa, spd: block.spd, spe: block.spe)
    }

    private static func domainKOChance(_ ko: Components.Schemas.KOChance) -> KOChance {
        KOChance(
            hits: ko.hits, guaranteed: ko.guaranteed,
            chancePercent: ko.chancePercent ?? 0, displayChancePercent: ko.displayChancePercent
        )
    }

    private static func domainCalcResult(_ result: Components.Schemas.CalcResult) -> CalcResult {
        CalcResult(
            rolls: result.rolls, minDamage: result.minDamage, maxDamage: result.maxDamage,
            minPercent: result.minPercent, maxPercent: result.maxPercent, defenderHP: result.defenderHP,
            effectiveness: result.effectiveness, stab: result.stab,
            category: domainMoveCategory(result.category.value1), ko: domainKOChance(result.ko)
        )
    }

    private static func domainBulkCalcResult(_ result: Components.Schemas.BulkCalcResult) -> BulkCalcResult {
        BulkCalcResult(defenderSpeciesKey: result.defenderSpeciesKey, rows: result.rows.map(domainBulkCalcRow))
    }

    private static func domainBulkCalcRow(_ row: Components.Schemas.BulkCalcRow) -> BulkCalcRow {
        BulkCalcRow(
            preset: domainDefenderPreset(row.preset), presetLabel: row.presetLabel,
            itemId: row.itemId, defender: domainBulkDefender(row.defender), result: domainCalcResult(row.result)
        )
    }

    private static func domainBulkDefender(_ defender: Components.Schemas.BulkDefender) -> BulkDefender {
        BulkDefender(
            sp: domainStatBlock(defender.sp.value1),
            nature: domainNatureModifier(defender.nature),
            natureId: defender.natureId,
            stats: domainStatBlock(defender.stats.value1)
        )
    }

    private static func domainNatureModifier(_ nature: Components.Schemas.NatureModifier) -> NatureModifier {
        NatureModifier(
            plus: nature.plus.map { domainStatKey($0.value1) },
            minus: nature.minus.map { domainStatKey($0.value1) }
        )
    }

    // MARK: - 逆算(ADR-0010 §R)応答 → ドメイン

    private static func domainReverseResult(_ result: Components.Schemas.ReverseResult) -> ReverseResult {
        ReverseResult(
            side: domainReverseSide(result.side),
            stat: domainStatKey(result.stat.value1),
            assumedHPSP: result.assumedHpSp,
            candidates: result.candidates.map(domainReverseCandidate),
            exactCount: result.exactCount
        )
    }

    private static func domainReverseCandidate(_ candidate: Components.Schemas.ReverseCandidate) -> ReverseCandidate {
        ReverseCandidate(
            natureClass: domainNatureClass(candidate.natureClass),
            nature: domainNatureModifier(candidate.nature),
            natureId: candidate.natureId,
            itemId: candidate.itemId,
            ranges: candidate.ranges.map { SPRange(min: $0.min, max: $0.max) },
            spCount: candidate.spCount,
            exact: candidate.exact,
            mismatch: candidate.mismatch,
            support: candidate.support,
            minPercent: candidate.minPercent,
            maxPercent: candidate.maxPercent
        )
    }

    private static func domainReverseSide(_ side: Components.Schemas.ReverseSide) -> ReverseSide {
        switch side {
        case .defender: return .defender
        case .attacker: return .attacker
        }
    }

    private static func domainNatureClass(_ natureClass: Components.Schemas.NatureClass) -> NatureClass {
        switch natureClass {
        case .neutral: return .neutral
        case .plus: return .plus
        }
    }

    // MARK: - ドメイン → 要求

    private static func generatedIndividual(_ individual: Individual) -> Components.Schemas.Individual {
        .init(
            speciesKey: individual.speciesKey,
            level: individual.level,
            natureId: individual.natureId,
            abilityId: individual.abilityId,
            itemId: individual.itemId,
            moveId: individual.moveId,
            sp: .init(value1: generatedStatBlock(individual.sp)),
            ranks: generatedRankBlock(individual.ranks),
            teraType: individual.teraType.map { .init(value1: generatedPokeType($0)) },
            status: generatedStatusCondition(individual.status)
        )
    }

    private static func generatedStatBlock(_ block: StatBlock) -> Components.Schemas.StatBlock {
        .init(hp: block.hp, atk: block.atk, def: block.def, spa: block.spa, spd: block.spd, spe: block.spe)
    }

    private static func generatedRankBlock(_ ranks: RankBlock) -> Components.Schemas.RankBlock {
        .init(atk: ranks.atk, def: ranks.def, spa: ranks.spa, spd: ranks.spd, spe: ranks.spe)
    }

    private static func generatedCalcRequest(_ request: CalcRequest) -> Components.Schemas.CalcRequest {
        .init(
            format: generatedFormat(request.format),
            attacker: generatedIndividual(request.attacker),
            defender: generatedIndividual(request.defender),
            moveId: request.moveId,
            options: .init(critical: request.critical)
        )
    }

    private static func generatedBulkCalcRequest(_ request: BulkCalcRequest) -> Components.Schemas.BulkCalcRequest {
        .init(
            format: generatedFormat(request.format),
            attacker: generatedIndividual(request.attacker),
            defenderSpeciesKey: request.defenderSpeciesKey,
            moveId: request.moveId,
            options: .init(critical: request.critical),
            // 省略(空配列を含む)は同じ意味(openapi の description)なので、空のときは
            // フィールド自体を送らない(nil のプロパティは JSON エンコード時に省かれる)。
            presets: request.presets.isEmpty ? nil : request.presets.map(generatedDefenderPreset),
            itemVariants: request.itemVariants.isEmpty ? nil : request.itemVariants
        )
    }

    /// openapi `ReverseRequest`(ADR-0010 §R): itemCandidates の省略と maxCandidates == 0 は既定値と
    /// 同じ意味なので、送るときは省く。
    private static func generatedReverseRequest(_ request: ReverseRequest) -> Components.Schemas.ReverseRequest {
        .init(
            format: generatedFormat(request.format),
            side: generatedReverseSide(request.side),
            known: .init(value1: generatedIndividual(request.known)),
            unknownSpeciesKey: .init(value1: request.unknownSpeciesKey),
            moveId: request.moveId,
            options: .init(critical: request.critical),
            itemCandidates: request.itemCandidates.isEmpty ? nil : request.itemCandidates,
            observations: request.observations.map(generatedObservation),
            maxCandidates: request.maxCandidates == 0 ? nil : request.maxCandidates
        )
    }

    private static func generatedObservation(_ observation: DamageObservation) -> Components.Schemas.Observation {
        switch observation {
        case .percent(let value): return .init(percent: value)
        case .percentTenths(let value): return .init(percentTenths: value)
        case .damage(let value): return .init(damage: value)
        }
    }

    private static func generatedReverseSide(_ side: ReverseSide) -> Components.Schemas.ReverseSide {
        switch side {
        case .defender: return .defender
        case .attacker: return .attacker
        }
    }

    // MARK: - enum の写像(契約とドメインの enum は同じ値集合。DomainTypesTests が同期を固定する)

    private static func domainPokeType(_ type: Components.Schemas.PokeType) -> PokeType {
        switch type {
        case .normal: return .normal
        case .fire: return .fire
        case .water: return .water
        case .electric: return .electric
        case .grass: return .grass
        case .ice: return .ice
        case .fighting: return .fighting
        case .poison: return .poison
        case .ground: return .ground
        case .flying: return .flying
        case .psychic: return .psychic
        case .bug: return .bug
        case .rock: return .rock
        case .ghost: return .ghost
        case .dragon: return .dragon
        case .dark: return .dark
        case .steel: return .steel
        case .fairy: return .fairy
        }
    }

    private static func generatedPokeType(_ type: PokeType) -> Components.Schemas.PokeType {
        switch type {
        case .normal: return .normal
        case .fire: return .fire
        case .water: return .water
        case .electric: return .electric
        case .grass: return .grass
        case .ice: return .ice
        case .fighting: return .fighting
        case .poison: return .poison
        case .ground: return .ground
        case .flying: return .flying
        case .psychic: return .psychic
        case .bug: return .bug
        case .rock: return .rock
        case .ghost: return .ghost
        case .dragon: return .dragon
        case .dark: return .dark
        case .steel: return .steel
        case .fairy: return .fairy
        }
    }

    private static func domainStatKey(_ key: Components.Schemas.StatKey) -> StatKey {
        switch key {
        case .hp: return .hp
        case .atk: return .atk
        case .def: return .def
        case .spa: return .spa
        case .spd: return .spd
        case .spe: return .spe
        }
    }

    private static func domainMoveCategory(_ category: Components.Schemas.MoveCategory) -> MoveCategory {
        switch category {
        case .physical: return .physical
        case .special: return .special
        case .status: return .status
        }
    }

    private static func generatedStatusCondition(_ status: StatusCondition) -> Components.Schemas.StatusCondition {
        switch status {
        case .none: return .none
        case .burn: return .burn
        case .paralysis: return .paralysis
        case .poison: return .poison
        case .badlyPoison: return .badlyPoison
        case .sleep: return .sleep
        case .freeze: return .freeze
        }
    }

    private static func generatedFormat(_ format: Format) -> Components.Schemas.Format {
        switch format {
        case .single: return .single
        case .double: return .double
        }
    }

    private static func domainDefenderPreset(_ preset: Components.Schemas.DefenderPreset) -> DefenderPreset {
        switch preset {
        case .none: return .none
        case .hp: return .hp
        case .hbBoost: return .hbBoost
        case .hb: return .hb
        case .hbFull: return .hbFull
        case .hdBoost: return .hdBoost
        case .hd: return .hd
        case .hdFull: return .hdFull
        }
    }

    private static func generatedDefenderPreset(_ preset: DefenderPreset) -> Components.Schemas.DefenderPreset {
        switch preset {
        case .none: return .none
        case .hp: return .hp
        case .hbBoost: return .hbBoost
        case .hb: return .hb
        case .hbFull: return .hbFull
        case .hdBoost: return .hdBoost
        case .hd: return .hd
        case .hdFull: return .hdFull
        }
    }
}
