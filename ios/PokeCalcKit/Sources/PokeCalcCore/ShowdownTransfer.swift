import Foundation
import Observation

// ShowdownTransfer: 構築のテキスト書き出し・取り込みの通信を伴う部分(P6-20。ADR-0506・ADR-0501「P6-20」)。
// 計算は `PokeCalcService` のマスタ参照だけを使い、
// 失敗しても画面・保存済みの構築を壊さない(絶対ルール5)。

/// 名前解決で出す通信の歯止め。
public enum ShowdownTransferLimits {
    /// 同時に飛ばす `PokeCalcService` の呼び出しの上限。
    public static let maxConcurrentRequests = 4
    /// 検索1回で受け取る件数(`MasterSearch.pageLimit`)。完全一致だけを採る。
    public static let searchLimit = MasterSearch.pageLimit
}

/// 取り込みの解決結果。`members` はすべて `TeamValidator` を満たす(新しい ID を割り当て済み)。
public struct ShowdownImportPlan: Equatable, Sendable {
    public var members: [TeamMember]
    public var rejected: [ShowdownRejection]
    /// 通信に失敗した場合のエラー(`rejected` には `lookupFailed` の行が入る)。
    public var error: PokeCalcError?

    public init(members: [TeamMember] = [], rejected: [ShowdownRejection] = [], error: PokeCalcError? = nil) {
        self.members = members
        self.rejected = rejected
        self.error = error
    }
}

/// 書き出しの結果。通信に失敗しても投げない。
public struct ShowdownExportResult: Equatable, Sendable {
    /// 書き出したテキスト。1体も書けなければ nil。
    public var text: String?
    public var skippedMemberIds: [String]
    public var unresolvedIds: [String]
    public var error: PokeCalcError?

    public init(
        text: String? = nil, skippedMemberIds: [String] = [], unresolvedIds: [String] = [], error: PokeCalcError? = nil
    ) {
        self.text = text
        self.skippedMemberIds = skippedMemberIds
        self.unresolvedIds = unresolvedIds
        self.error = error
    }
}

/// 名前 ⇄ ID の解決(`PokeCalcService` のマスタ参照だけ)。
public struct ShowdownTransferService: Sendable {
    private let service: any PokeCalcService
    private let naming: any ShowdownNaming
    private let maxConcurrentRequests: Int

    public init(
        service: any PokeCalcService,
        naming: any ShowdownNaming = JapaneseShowdownNaming(),
        maxConcurrentRequests: Int = ShowdownTransferLimits.maxConcurrentRequests
    ) {
        self.service = service
        self.naming = naming
        self.maxConcurrentRequests = max(1, maxConcurrentRequests)
    }

    // MARK: 書き出し

    /// 書き出し。種族は `species(key:)`(特性名もここから)、技は `moves(ids:)` を1回、持ち物は
    /// `searchItems`(空クエリの先頭ページ)、性格は `natures()` を1回。同じ ID・キーは1回しか引かない。
    /// 種族を引けなかったメンバーは飛ばし `error` を立てる(他のメンバーは書く)。
    public func export(members: [TeamMember]) async -> ShowdownExportResult {
        var speciesKeys: [String] = []
        var moveIds: [String] = []
        for member in members {
            if !speciesKeys.contains(member.speciesKey) { speciesKeys.append(member.speciesKey) }
            for id in member.moveIds.prefix(TeamLimits.maxMovesPerMember) where !moveIds.contains(id) { moveIds.append(id) }
        }
        let needsItems = members.contains { $0.itemId != nil }

        var lookups: [ExportLookup] = speciesKeys.map(ExportLookup.species)
        lookups.append(.natures)
        if !moveIds.isEmpty { lookups.append(.moves(moveIds)) }
        if needsItems { lookups.append(.items) }

        let outcomes = await boundedMap(lookups) { lookup in await self.perform(lookup) }

        var names = ShowdownNames(typeNames: Dictionary(uniqueKeysWithValues: PokeType.allCases.map { ($0, naming.name(of: $0)) }))
        var firstError: PokeCalcError?
        func note(_ error: PokeCalcError) { if firstError == nil { firstError = error } }
        var resolvedMoves: [Move] = []
        for (lookup, outcome) in zip(lookups, outcomes) {
            switch (lookup, outcome) {
            case (.species(let key), .species(.success(let detail))?):
                if let detail {
                    names.speciesByKey[key] = naming.name(of: detail)
                    for ability in detail.abilities { names.abilitiesById[ability.id] = naming.name(of: ability) }
                }
            case (.natures, .natures(.success(let natures))?):
                for nature in natures { names.naturesById[nature.id] = naming.name(of: nature) }
            case (.moves, .moves(.success(let moves))?):
                resolvedMoves = moves
            case (.items, .items(.success(let items))?):
                for item in items { names.itemsById[item.id] = naming.name(of: item) }
            case (_, .species(.failure(let error))?), (_, .natures(.failure(let error))?),
                (_, .moves(.failure(let error))?), (_, .items(.failure(let error))?):
                note(error)
            default:
                note(Self.cancelledError)
            }
        }
        for move in resolvedMoves { names.movesById[move.id] = naming.name(of: move) }

        // `moves(ids:)` が返さなかった技は、先頭ページの検索から名前を補う(それでも引けなければ行を省く)。
        let missingMoveIds = moveIds.filter { names.movesById[$0] == nil }
        let movesSucceeded = zip(lookups, outcomes).contains { lookup, outcome in
            if case .moves = lookup, case .moves(.success)? = outcome { return true }
            return false
        }
        if movesSucceeded, !missingMoveIds.isEmpty, !Task.isCancelled {
            if let page = try? await service.searchMoves(query: "", limit: ShowdownTransferLimits.searchLimit) {
                for move in page where missingMoveIds.contains(move.id) { names.movesById[move.id] = naming.name(of: move) }
            }
        }

        let team = ShowdownTextSerializer.serialize(members: members, names: names)
        if !team.skippedMemberIds.isEmpty { note(firstError ?? Self.missingSpeciesError) }
        return ShowdownExportResult(
            text: team.text.isEmpty ? nil : team.text, skippedMemberIds: team.skippedMemberIds,
            unresolvedIds: team.unresolvedIds, error: firstError
        )
    }

    // MARK: 取り込み

    /// 取り込みの名前解決。完全一致(前後空白を除く)だけを採る。
    /// - 種族: `searchSpecies(query: naming.searchQuery(...))` の完全一致 → `species(key:)`(特性・性格の既定に使う)
    /// - 技・持ち物: `searchMoves`/`searchItems` の完全一致。性格: `natures()`。タイプ: `naming.type(forImportedName:)`
    /// - 性格の行が無ければ `natures()` の先頭(`TeamEditViewModel.addMember` と同じ)。性格の名前を解決できなければ
    ///   先頭にして `natureNotFound` を報告する
    /// - 種族が解決できなければそのメンバーを捨て、先頭行に `speciesNotFound`(通信失敗は `lookupFailed`)
    /// - 持ち物・特性・技・タイプが解決できなければ、その行だけを捨てて(該当の理由で報告し)メンバーは取り込む
    /// - `existingMemberCount + 取り込むメンバー数` が `TeamLimits.maxMembers` を超えるぶんは、超えたブロックの
    ///   先頭行を `memberLimitExceeded` で報告して取り込まない(先頭から順に枠を使う)
    /// - 同じ名前は1回しか検索しない。同時に飛ばす呼び出しは `maxConcurrentRequests` まで
    /// - 通信失敗でも投げない(`plan.error` を立てる)。キャンセルされたら以後の呼び出しをしない
    public func resolve(_ parsed: ShowdownParseResult, existingMemberCount: Int) async -> ShowdownImportPlan {
        var rejected = parsed.rejected
        let capacity = max(0, TeamLimits.maxMembers - existingMemberCount)
        if capacity == 0 {
            for member in parsed.members {
                rejected.append(Self.header(of: member, reason: .memberLimitExceeded))
            }
            return ShowdownImportPlan(members: [], rejected: Self.ordered(rejected), error: nil)
        }

        var speciesNames: [String] = []
        var moveNames: [String] = []
        var itemNames: [String] = []
        for member in parsed.members {
            if !speciesNames.contains(member.speciesName) { speciesNames.append(member.speciesName) }
            for name in member.moveNames where !moveNames.contains(name) { moveNames.append(name) }
            if let item = member.itemName, !itemNames.contains(item) { itemNames.append(item) }
        }
        var lookups: [ImportLookup] = [.natures]
        lookups += speciesNames.map(ImportLookup.species)
        lookups += moveNames.map(ImportLookup.move)
        lookups += itemNames.map(ImportLookup.item)
        let outcomes = await boundedMap(lookups) { lookup in await self.perform(lookup) }

        var natures: Result<[Nature], PokeCalcError> = .failure(Self.cancelledError)
        var speciesByName: [String: Result<SpeciesDetail?, PokeCalcError>] = [:]
        var movesByName: [String: Result<Move?, PokeCalcError>] = [:]
        var itemsByName: [String: Result<Item?, PokeCalcError>] = [:]
        for (lookup, outcome) in zip(lookups, outcomes) {
            switch (lookup, outcome) {
            case (.natures, .natures(let value)?): natures = value
            case (.species(let name), .species(let value)?): speciesByName[name] = value
            case (.move(let name), .move(let value)?): movesByName[name] = value
            case (.item(let name), .item(let value)?): itemsByName[name] = value
            default: break
            }
        }

        var firstError: PokeCalcError?
        func note(_ error: PokeCalcError) { if firstError == nil { firstError = error } }
        if case .failure(let error) = natures { note(error) }
        for name in speciesNames { if case .failure(let error)? = speciesByName[name] { note(error) } }
        for name in moveNames { if case .failure(let error)? = movesByName[name] { note(error) } }
        for name in itemNames { if case .failure(let error)? = itemsByName[name] { note(error) } }

        var members: [TeamMember] = []
        for parsedMember in parsed.members {
            var rows: [ShowdownRejection] = []
            func row(_ line: Int?, _ text: String, _ reason: ShowdownRejectionReason) {
                rows.append(ShowdownRejection(lineNumber: line ?? parsedMember.headerLineNumber, text: text, reason: reason))
            }
            guard case .success(let natureList) = natures else {
                rejected.append(Self.header(of: parsedMember, reason: .lookupFailed))
                continue
            }
            switch speciesByName[parsedMember.speciesName] {
            case .success(let detail?):
                if members.count >= capacity {
                    rejected.append(Self.header(of: parsedMember, reason: .memberLimitExceeded))
                    continue
                }
                var member = TeamMember(
                    speciesKey: detail.key, nickname: parsedMember.nickname, natureId: natureList.first?.id ?? "")
                for stat in StatKey.allCases { member.sp.setValue(parsedMember.sp[stat] ?? 0, for: stat) }

                if let name = parsedMember.itemName {
                    switch itemsByName[name] {
                    case .success(let item?): member.itemId = item.id
                    case .success(nil): row(parsedMember.itemLine, parsedMember.headerText, .itemNotFound)
                    default: row(parsedMember.itemLine, parsedMember.headerText, .lookupFailed)
                    }
                }
                if let name = parsedMember.abilityName {
                    let text = "\(ShowdownTextLabels.abilityKey) \(name)"
                    if let ability = detail.abilities.first(where: { naming.name(of: $0) == name }) {
                        member.abilityId = ability.id
                    } else {
                        row(parsedMember.abilityLine, text, .abilityNotFound)
                    }
                }
                if let name = parsedMember.teraTypeName {
                    if let type = naming.type(forImportedName: name) {
                        member.teraType = type
                    } else {
                        row(parsedMember.teraTypeLine, "\(ShowdownTextLabels.teraTypeKey) \(name)", .teraTypeNotFound)
                    }
                }
                if let name = parsedMember.natureName {
                    if let nature = natureList.first(where: { naming.name(of: $0) == name }) {
                        member.natureId = nature.id
                    } else {
                        row(parsedMember.natureLine, "\(ShowdownTextLabels.natureKey) \(name)", .natureNotFound)
                    }
                }
                for (index, name) in parsedMember.moveNames.enumerated() {
                    let line = index < parsedMember.moveLines.count ? parsedMember.moveLines[index] : nil
                    let text = ShowdownTextLabels.movePrefix + name
                    switch movesByName[name] {
                    case .success(let move?):
                        if member.moveIds.contains(move.id) {
                            row(line, text, .duplicateMove)
                        } else if member.moveIds.count < TeamLimits.maxMovesPerMember {
                            member.moveIds.append(move.id)
                        } else {
                            row(line, text, .tooManyMoves)
                        }
                    case .success(nil): row(line, text, .moveNotFound)
                    default: row(line, text, .lookupFailed)
                    }
                }
                members.append(member)
                rejected += rows
            case .success(nil):
                rejected.append(Self.header(of: parsedMember, reason: .speciesNotFound))
            default:
                rejected.append(Self.header(of: parsedMember, reason: .lookupFailed))
            }
        }
        return ShowdownImportPlan(members: members, rejected: Self.ordered(rejected), error: firstError)
    }

    // MARK: 内部

    private static var cancelledError: PokeCalcError {
        PokeCalcError(code: PokeCalcError.Code.transport, message: "キャンセルされました")
    }

    private static var missingSpeciesError: PokeCalcError {
        PokeCalcError(code: PokeCalcError.Code.notFound, message: "種族をマスタから引けませんでした")
    }

    private static func header(of member: ShowdownParsedMember, reason: ShowdownRejectionReason) -> ShowdownRejection {
        ShowdownRejection(lineNumber: member.headerLineNumber, text: member.headerText, reason: reason)
    }

    /// 行番号順(同じ行は先に入れた順)。
    private static func ordered(_ rows: [ShowdownRejection]) -> [ShowdownRejection] {
        rows.enumerated().sorted { ($0.element.lineNumber, $0.offset) < ($1.element.lineNumber, $1.offset) }.map(\.element)
    }

    private static func failure(_ error: any Error) -> PokeCalcError {
        (error as? PokeCalcError) ?? PokeCalcError(code: PokeCalcError.Code.transport, message: String(describing: error))
    }

    private enum ExportLookup: Sendable {
        case species(String)
        case natures
        case moves([String])
        case items
    }

    private enum ImportLookup: Sendable {
        case natures
        case species(String)
        case move(String)
        case item(String)
    }

    private enum Outcome: Sendable {
        case species(Result<SpeciesDetail?, PokeCalcError>)
        case natures(Result<[Nature], PokeCalcError>)
        case moves(Result<[Move], PokeCalcError>)
        case move(Result<Move?, PokeCalcError>)
        case items(Result<[Item], PokeCalcError>)
        case item(Result<Item?, PokeCalcError>)
    }

    private func perform(_ lookup: ExportLookup) async -> Outcome {
        do {
            switch lookup {
            case .species(let key): return .species(.success(try await service.species(key: key)))
            case .natures: return .natures(.success(try await service.natures()))
            case .moves(let ids): return .moves(.success(try await service.moves(ids: ids)))
            case .items:
                return .items(.success(try await service.searchItems(query: "", limit: ShowdownTransferLimits.searchLimit)))
            }
        } catch {
            let failure = Self.failure(error)
            switch lookup {
            case .species: return .species(.failure(failure))
            case .natures: return .natures(.failure(failure))
            case .moves: return .moves(.failure(failure))
            case .items: return .items(.failure(failure))
            }
        }
    }

    private func perform(_ lookup: ImportLookup) async -> Outcome {
        switch lookup {
        case .natures:
            do { return .natures(.success(try await service.natures())) } catch { return .natures(.failure(Self.failure(error))) }
        case .species(let name):
            do {
                let query = naming.searchQuery(forImportedName: name)
                let found = try await service.searchSpecies(query: query, limit: ShowdownTransferLimits.searchLimit)
                guard let match = found.first(where: { naming.name(of: Self.placeholderDetail(for: $0)) == name }) else {
                    return .species(.success(nil))
                }
                return .species(.success(try await service.species(key: match.key)))
            } catch {
                return .species(.failure(Self.failure(error)))
            }
        case .move(let name):
            do {
                let found = try await service.searchMoves(
                    query: naming.searchQuery(forImportedName: name), limit: ShowdownTransferLimits.searchLimit)
                return .move(.success(found.first { naming.name(of: $0) == name }))
            } catch {
                return .move(.failure(Self.failure(error)))
            }
        case .item(let name):
            do {
                let found = try await service.searchItems(
                    query: naming.searchQuery(forImportedName: name), limit: ShowdownTransferLimits.searchLimit)
                return .item(.success(found.first { naming.name(of: $0) == name }))
            } catch {
                return .item(.failure(Self.failure(error)))
            }
        }
    }

    /// 検索結果(`SpeciesSummary`)に `ShowdownNaming.name(of: SpeciesDetail)` を当てるための入れ物
    /// (名前に使う項目だけを写す。能力値・特性・learnset は使わない)。
    private static func placeholderDetail(for summary: SpeciesSummary) -> SpeciesDetail {
        SpeciesDetail(
            key: summary.key, dexNo: summary.dexNo, form: summary.form, nameJa: summary.nameJa, types: summary.types,
            baseStats: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0), abilities: [], learnset: []
        )
    }

    /// 同時実行数を `maxConcurrentRequests` までに絞って、入力の順に結果を返す。
    /// キャンセルされたら以後は始めない(始めなかった分は nil)。
    private func boundedMap<Input: Sendable>(
        _ inputs: [Input], _ body: @escaping @Sendable (Input) async -> Outcome
    ) async -> [Outcome?] {
        let limit = maxConcurrentRequests
        return await withTaskGroup(of: (Int, Outcome).self) { group in
            var results = [Outcome?](repeating: nil, count: inputs.count)
            var next = 0
            while next < min(limit, inputs.count) {
                let index = next
                next += 1
                group.addTask { (index, await body(inputs[index])) }
            }
            while let (index, outcome) = await group.next() {
                results[index] = outcome
                if next < inputs.count, !Task.isCancelled {
                    let following = next
                    next += 1
                    group.addTask { (following, await body(inputs[following])) }
                }
            }
            return results
        }
    }
}

/// 書き出し・取り込みのシートの状態(`TeamEditView` が持つ。`@MainActor @Observable`)。
@MainActor
@Observable
public final class TeamTextTransferViewModel {
    public enum Phase: Equatable, Sendable {
        case idle
        case analyzing
        /// 解釈を終えた。`plan` と `parseRejected` を見せて、追加するかを選ばせる。
        case reviewing
    }

    public private(set) var phase: Phase = .idle
    public private(set) var pastedText: String = ""
    /// 書き出したテキスト(書き出し前・失敗時は nil)。
    public private(set) var exportText: String?
    public private(set) var isExporting = false
    public private(set) var exportError: PokeCalcError?
    /// 書き出しで名前を引けず省いた技・持ち物の数(注意の表示用)。
    public private(set) var exportUnresolvedCount = 0
    /// 書き出しで種族を引けず飛ばしたメンバーの数(注意の表示用)。
    public private(set) var exportSkippedMemberCount = 0
    /// 取り込めなかった行(パーサの分と名前解決の分を行番号順に並べたもの)。
    public private(set) var rejected: [ShowdownRejection] = []
    /// 取り込める体数。
    public private(set) var importableCount = 0
    public private(set) var importError: PokeCalcError?

    private let transfer: ShowdownTransferService
    private var plan: ShowdownImportPlan?

    public init(
        service: any PokeCalcService,
        naming: any ShowdownNaming = JapaneseShowdownNaming(),
        maxConcurrentRequests: Int = ShowdownTransferLimits.maxConcurrentRequests
    ) {
        transfer = ShowdownTransferService(service: service, naming: naming, maxConcurrentRequests: maxConcurrentRequests)
    }

    /// 1体または複数体を書き出す(メンバー単位は `[member]`)。直前の `exportText`/`exportError` は置き換える。
    public func prepareExport(members: [TeamMember]) async {
        isExporting = true
        let result = await transfer.export(members: members)
        exportText = result.text
        exportError = result.error
        exportUnresolvedCount = result.unresolvedIds.count
        exportSkippedMemberCount = result.skippedMemberIds.count
        isExporting = false
    }

    /// 貼り付けた文字列を反映する。変わったら解釈の結果(`phase == .reviewing`)を捨てて `idle` に戻す
    /// (見せている一覧と文字列が食い違わないように)。
    public func setPastedText(_ text: String) {
        guard text != pastedText else { return }
        pastedText = text
        discardReview()
    }

    /// 解釈 → 名前解決。空(空白だけ)なら通信せず `idle` のまま。`existingMemberCount` は編集中の構築の現在の体数。
    public func analyze(existingMemberCount: Int) async {
        // 二重実行しない(解析中の再タップで通信を重ねない)。
        guard phase != .analyzing else { return }
        let text = pastedText
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            discardReview()
            return
        }
        phase = .analyzing
        let result = await transfer.resolve(ShowdownTextParser.parse(text), existingMemberCount: existingMemberCount)
        // 解析中に貼り付けが変わっていたら、古い結果を見せない(`setPastedText` が idle に戻している)。
        guard text == pastedText else { return }
        plan = result
        rejected = result.rejected
        importableCount = result.members.count
        importError = result.error
        phase = .reviewing
    }

    /// 取り込める体があるか(`reviewing` で `importableCount > 0`)。
    public var canConfirm: Bool { phase == .reviewing && importableCount > 0 }
    /// 取り込めなかった行があり、取り込める体もある(「取り込める分だけ追加するか」を選ばせる)。
    public var needsDecision: Bool { canConfirm && !rejected.isEmpty }

    /// 追加を確定する。`canConfirm` のときだけ取り込める体を返し(`idle` に戻し、貼り付けも空にする)、
    /// それ以外は空配列(状態は変えない)。呼び出し側が `TeamEditViewModel.importMembers` に渡す。
    public func confirm() -> [TeamMember] {
        guard canConfirm, let members = plan?.members else { return [] }
        pastedText = ""
        discardReview()
        return members
    }

    /// やめる。`idle` に戻し、解釈の結果と貼り付けを捨てる。書き出しの状態は残す。
    public func cancel() {
        pastedText = ""
        discardReview()
    }

    private func discardReview() {
        phase = .idle
        plan = nil
        rejected = []
        importableCount = 0
        importError = nil
    }
}
