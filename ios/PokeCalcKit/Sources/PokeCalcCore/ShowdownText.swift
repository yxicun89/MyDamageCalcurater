import Foundation

// ShowdownText: 構築の「Showdown 風テキスト」の書式・解釈・組み立て(P6-20。ADR-0502・ADR-0501「P6-20」)。
//
// 日本語名で書く(実 Showdown とは互換にしない)。この足場は spec-writer が置いたもので、
// 純粋関数だけ(通信・時刻・乱数を持たない)。名前から ID への解決は `ShowdownTransfer.swift`。

/// 書式のキーワードと画面の文言を集約する(coding-rules §2。View・Parser・Serializer はここだけを参照する)。
public enum ShowdownTextLabels {
    // MARK: 書式(Showdown と同じ英語のキーワード。値は日本語名)

    public static let abilityKey = "Ability:"
    public static let natureKey = "Nature:"
    public static let spKey = "SP:"
    public static let teraTypeKey = "Tera Type:"
    /// 取り込めない形式の行頭(EV・個体値。`unsupportedStatLine` として報告する)。
    public static let evKey = "EVs:"
    public static let ivKey = "IVs:"
    /// 名前と持ち物の区切り(前後の空白を含む)。
    public static let itemSeparator = " @ "
    /// 技の行頭(後ろの空白を含む)。
    public static let movePrefix = "- "
    /// SP の各ステータスの区切り(前後の空白を含む)。
    public static let spSeparator = " / "

    /// SP 行のステータス略称(Showdown と同じ HP/Atk/Def/SpA/SpD/Spe)。
    public static func statAbbreviation(for stat: StatKey) -> String {
        switch stat {
        case .hp: return "HP"
        case .atk: return "Atk"
        case .def: return "Def"
        case .spa: return "SpA"
        case .spd: return "SpD"
        case .spe: return "Spe"
        }
    }

    // MARK: 画面の文言(ADR-0501「P6-20」3章の表)

    public static let transferButton = "テキストで書き出し・取り込み"
    public static let sheetTitle = "構築のテキスト"
    public static let exportMemberButton = "この1体を書き出す"
    public static let exportTeamButton = "全員を書き出す"
    public static let copyButton = "コピー"
    public static func exportUnresolvedNotice(count: Int) -> String { "\(count) 項目は名前を引けず省きました。" }
    public static func exportSkippedNotice(count: Int) -> String { "\(count) 体は種族を引けず書き出していません。" }
    public static let copiedNotice = "コピーしました。"
    public static let shareButton = "共有"
    public static let importSectionTitle = "テキストから取り込む"
    public static let importPlaceholder = "ここに貼り付け"
    public static let analyzeButton = "内容を確認"
    public static let rejectedTitle = "取り込めなかった行"
    public static let cancelButton = "やめる"
    public static let closeButton = "閉じる"
    public static let nothingImportable = "取り込めるポケモンがありません。"
    public static let emptyInput = "テキストを貼り付けてください。"
    /// 通信できず書き出し・取り込みの解決ができなかったときの案内(計算・保存済みの構築には影響しない旨)。
    public static let lookupFailure = "サーバーに届かず、名前を確認できませんでした。通信を確認してもう一度お試しください。保存済みの構築は変わりません。"

    /// 「取り込める N 体だけ追加」(取り込めなかった行がある場合の既定の選択肢)。
    public static func importValidOnlyButton(count: Int) -> String {
        "取り込める\(count)体だけ追加"
    }

    /// 「N 体を追加」(取り込めなかった行が無い場合)。
    public static func importAllButton(count: Int) -> String {
        "\(count)体を追加"
    }

    /// 追加後の通知「N 体を追加しました。保存すると反映されます。」(取り込みは保存を伴わない)。
    public static func importedNotice(count: Int) -> String {
        "\(count)体を追加しました。保存すると反映されます。"
    }

    /// 「取り込めなかった行」の1行の見出し(「3行目」)。
    public static func lineNumberLabel(_ lineNumber: Int) -> String {
        "\(lineNumber)行目"
    }

    /// 取り込めなかった理由の文言(全ケース非空・互いに異なる)。
    public static func message(for reason: ShowdownRejectionReason) -> String {
        switch reason {
        case .unrecognizedLine: return "解釈できない行です"
        case .unsupportedStatLine: return "努力値(EVs)・個体値(IVs)の形式には対応していません。能力ポイントは SP: で書いてください"
        case .duplicateField: return "同じ項目が2回書かれています"
        case .tooManyMoves: return "技は\(TeamLimits.maxMovesPerMember)つまでです"
        case .duplicateMove: return "同じ技が重複しています"
        case .spMalformed: return "SP の書き方が正しくありません"
        case .spOutOfRange: return "SP は1ステータスにつき\(SPLimits.maxPerStat)までです"
        case .spTotalExceeded: return "SP の合計は\(SPLimits.maxTotal)までです"
        case .speciesNotFound: return "ポケモンが見つかりません"
        case .moveNotFound: return "技が見つかりません"
        case .itemNotFound: return "持ち物が見つかりません"
        case .abilityNotFound: return "このポケモンの特性に見つかりません"
        case .natureNotFound: return "性格が見つかりません"
        case .teraTypeNotFound: return "テラスタイプが見つかりません"
        case .lookupFailed: return "通信できず確認できませんでした"
        case .memberLimitExceeded: return "構築は\(TeamLimits.maxMembers)体までです"
        }
    }
}

// MARK: - 名前の戦略(英語名への拡張点)

/// 名前の書き方と、取り込み時の検索語の作り方を差し替えられるようにする型。
/// 既定は日本語名(`JapaneseShowdownNaming`)。API に `nameEn`/`showdownId` が足されたら、
/// 英語名の実装を足して差し替えるだけで広げられる(ADR-0502 §4。`api/openapi.yaml` は触らない)。
public protocol ShowdownNaming: Sendable {
    func name(of species: SpeciesDetail) -> String
    func name(of move: Move) -> String
    func name(of item: Item) -> String
    func name(of ability: Ability) -> String
    func name(of nature: Nature) -> String
    func name(of type: PokeType) -> String
    /// 取り込んだ名前からマスタ検索(`searchSpecies`/`searchMoves`/`searchItems`)の検索語を作る。
    /// 検索結果は `name(of:)` と完全一致したものだけを採る。
    func searchQuery(forImportedName name: String) -> String
    /// 取り込んだ名前からタイプを引く(`name(of: PokeType)` の逆)。
    func type(forImportedName name: String) -> PokeType?
}

/// 既定の日本語名。名前は `nameJa`、タイプは `PokeTypeLabel.japaneseName`。検索語は名前そのまま。
public struct JapaneseShowdownNaming: ShowdownNaming {
    public init() {}
    public func name(of species: SpeciesDetail) -> String { species.nameJa }
    public func name(of move: Move) -> String { move.nameJa }
    public func name(of item: Item) -> String { item.nameJa }
    public func name(of ability: Ability) -> String { ability.nameJa }
    public func name(of nature: Nature) -> String { nature.nameJa }
    public func name(of type: PokeType) -> String { PokeTypeLabel.japaneseName(for: type) }
    public func searchQuery(forImportedName name: String) -> String { name }
    public func type(forImportedName name: String) -> PokeType? {
        PokeType.allCases.first { PokeTypeLabel.japaneseName(for: $0) == name }
    }
}

// MARK: - 解釈(Parser)

/// 取り込めなかった理由。パーサ・名前解決・追加枠の3段で共通。
public enum ShowdownRejectionReason: Equatable, Hashable, Sendable, CaseIterable {
    /// どの書式にも当てはまらない行。
    case unrecognizedLine
    /// `EVs:` / `IVs:` の行(努力値形式は対象外。SP を使う)。
    case unsupportedStatLine
    /// 同じ項目(Ability/Nature/SP/Tera Type/持ち物)が同じメンバーに2回ある(後の行を捨てる)。
    case duplicateField
    /// 技の行が `TeamLimits.maxMovesPerMember` を超えた(超えた行を捨てる)。
    case tooManyMoves
    /// 同じ技が重複している(後の行を捨てる)。
    case duplicateMove
    /// SP 行の書き方が正しくない(行全体を捨てる)。
    case spMalformed
    /// SP が 1 ステータスにつき `SPLimits.maxPerStat` を超える(行全体を捨てる)。
    case spOutOfRange
    /// SP の合計が `SPLimits.maxTotal` を超える(行全体を捨てる)。
    case spTotalExceeded
    /// ポケモン名がマスタに無い(そのメンバーごと捨てる。理由は先頭行に付く)。
    case speciesNotFound
    case moveNotFound
    case itemNotFound
    /// 特性が、そのポケモンの特性(`SpeciesDetail.abilities`)に無い。
    case abilityNotFound
    case natureNotFound
    case teraTypeNotFound
    /// 通信に失敗して確かめられなかった(「見つからない」とは別。通信を確認すれば取り込める)。
    case lookupFailed
    /// 構築の上限(`TeamLimits.maxMembers`)を超えるメンバー(ブロックごと捨てる。理由は先頭行に付く)。
    case memberLimitExceeded
}

/// パーサ内部で `Result` の失敗側に使う(理由をそのまま運ぶだけ)。
extension ShowdownRejectionReason: Error {}

/// 取り込めなかった1行(黙って捨てずに画面で伝える)。
public struct ShowdownRejection: Equatable, Sendable {
    /// 貼り付けたテキストの 1 始まりの行番号(空行も数える)。
    public var lineNumber: Int
    /// 前後の空白を除いた行の内容。
    public var text: String
    public var reason: ShowdownRejectionReason

    public init(lineNumber: Int, text: String, reason: ShowdownRejectionReason) {
        self.lineNumber = lineNumber
        self.text = text
        self.reason = reason
    }
}

/// 解釈しただけ(名前はまだ ID に解決していない)の1体。
public struct ShowdownParsedMember: Equatable, Sendable {
    public var speciesName: String
    public var nickname: String?
    public var itemName: String?
    public var abilityName: String?
    public var natureName: String?
    /// 書かれたステータスだけ(書かれていないものは 0 として扱う)。範囲・合計は検査済み。
    public var sp: [StatKey: Int]
    public var moveNames: [String]
    public var teraTypeName: String?
    /// 先頭行(`名前 @ 持ち物`)の行番号と内容。名前解決の失敗を先頭行に付けるため。
    public var headerLineNumber: Int
    public var headerText: String
    /// 項目ごとの行番号(名前解決の失敗をその行に付ける)。
    public var itemLine: Int?
    public var abilityLine: Int?
    public var natureLine: Int?
    public var teraTypeLine: Int?
    public var moveLines: [Int]

    public init(
        speciesName: String, nickname: String? = nil, itemName: String? = nil, abilityName: String? = nil,
        natureName: String? = nil, sp: [StatKey: Int] = [:], moveNames: [String] = [], teraTypeName: String? = nil,
        headerLineNumber: Int = 1, headerText: String = "", itemLine: Int? = nil, abilityLine: Int? = nil,
        natureLine: Int? = nil, teraTypeLine: Int? = nil, moveLines: [Int] = []
    ) {
        self.speciesName = speciesName
        self.nickname = nickname
        self.itemName = itemName
        self.abilityName = abilityName
        self.natureName = natureName
        self.sp = sp
        self.moveNames = moveNames
        self.teraTypeName = teraTypeName
        self.headerLineNumber = headerLineNumber
        self.headerText = headerText
        self.itemLine = itemLine
        self.abilityLine = abilityLine
        self.natureLine = natureLine
        self.teraTypeLine = teraTypeLine
        self.moveLines = moveLines
    }
}

public struct ShowdownParseResult: Equatable, Sendable {
    public var members: [ShowdownParsedMember]
    public var rejected: [ShowdownRejection]

    public init(members: [ShowdownParsedMember] = [], rejected: [ShowdownRejection] = []) {
        self.members = members
        self.rejected = rejected
    }
}

/// テキスト → `ShowdownParseResult`。純粋関数(通信しない)。
public enum ShowdownTextParser {
    /// 規則(ADR-0501「P6-20」2章):
    /// - 改行は LF/CRLF/CR。空行(空白だけの行)でメンバーを区切る。行頭末の空白(全角を含む)は無視する
    /// - ブロックの先頭行は `名前` / `名前 @ 持ち物` / `ニックネーム (名前)` / `ニックネーム (名前) @ 持ち物`
    /// - 2行目以降は `Ability:` `Nature:` `SP:` `Tera Type:` `- 技名`(順不同)。それ以外は `unrecognizedLine`、
    ///   `EVs:` `IVs:` は `unsupportedStatLine`
    /// - メンバーは `TeamLimits.maxMembers` まで。超えたブロックは先頭行だけを `memberLimitExceeded` で報告する
    /// - 黙って捨てない: 採らなかった行は必ず `rejected` に行番号つきで入る
    public static func parse(_ text: String) -> ShowdownParseResult {
        var result = ShowdownParseResult()
        var block: [Line] = []
        var blockCount = 0

        func flush() {
            guard !block.isEmpty else { return }
            if blockCount < TeamLimits.maxMembers {
                parseBlock(block, into: &result)
            } else if let header = block.first {
                result.rejected.append(
                    ShowdownRejection(lineNumber: header.number, text: header.text, reason: .memberLimitExceeded))
            }
            blockCount += 1
            block = []
        }

        for line in lines(of: text) {
            if line.text.isEmpty {
                flush()
            } else {
                block.append(line)
            }
        }
        flush()
        return result
    }

    private struct Line {
        var number: Int
        /// 前後の空白(全角を含む)を除いた内容。
        var text: String
    }

    /// 改行(LF/CRLF/CR)で分け、1 始まりの行番号と空白を除いた内容を付ける。
    private static func lines(of text: String) -> [Line] {
        let unified = text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        return unified.components(separatedBy: "\n").enumerated().map { index, raw in
            Line(number: index + 1, text: raw.trimmingCharacters(in: .whitespacesAndNewlines))
        }
    }

    private static func parseBlock(_ lines: [Line], into result: inout ShowdownParseResult) {
        guard let header = lines.first, let head = parseHeader(header) else {
            for line in lines {
                result.rejected.append(ShowdownRejection(lineNumber: line.number, text: line.text, reason: .unrecognizedLine))
            }
            return
        }
        var member = head
        var spAccepted = false

        func reject(_ line: Line, _ reason: ShowdownRejectionReason) {
            result.rejected.append(ShowdownRejection(lineNumber: line.number, text: line.text, reason: reason))
        }

        for line in lines.dropFirst() {
            let text = line.text
            if text.hasPrefix(ShowdownTextLabels.evKey) || text.hasPrefix(ShowdownTextLabels.ivKey) {
                reject(line, .unsupportedStatLine)
            } else if let value = value(of: text, after: ShowdownTextLabels.abilityKey) {
                if value.isEmpty {
                    reject(line, .unrecognizedLine)
                } else if member.abilityName != nil {
                    reject(line, .duplicateField)
                } else {
                    member.abilityName = value
                    member.abilityLine = line.number
                }
            } else if let value = value(of: text, after: ShowdownTextLabels.natureKey) {
                if value.isEmpty {
                    reject(line, .unrecognizedLine)
                } else if member.natureName != nil {
                    reject(line, .duplicateField)
                } else {
                    member.natureName = value
                    member.natureLine = line.number
                }
            } else if let value = value(of: text, after: ShowdownTextLabels.teraTypeKey) {
                if value.isEmpty {
                    reject(line, .unrecognizedLine)
                } else if member.teraTypeName != nil {
                    reject(line, .duplicateField)
                } else {
                    member.teraTypeName = value
                    member.teraTypeLine = line.number
                }
            } else if let value = value(of: text, after: ShowdownTextLabels.spKey) {
                if spAccepted {
                    reject(line, .duplicateField)
                } else {
                    switch parseSP(value) {
                    case .success(let sp):
                        member.sp = sp
                        spAccepted = true
                    case .failure(let reason):
                        reject(line, reason)
                    }
                }
            } else if text.hasPrefix(ShowdownTextLabels.movePrefix) || text.hasPrefix("-\u{3000}") {
                let name = String(text.dropFirst(ShowdownTextLabels.movePrefix.count))
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                if name.isEmpty {
                    reject(line, .unrecognizedLine)
                } else if member.moveNames.contains(name) {
                    reject(line, .duplicateMove)
                } else if member.moveNames.count >= TeamLimits.maxMovesPerMember {
                    reject(line, .tooManyMoves)
                } else {
                    member.moveNames.append(name)
                    member.moveLines.append(line.number)
                }
            } else {
                reject(line, .unrecognizedLine)
            }
        }
        result.members.append(member)
    }

    /// `キー: 値` の行なら、値(前後の空白を除く)を返す。
    private static func value(of text: String, after key: String) -> String? {
        guard text.hasPrefix(key) else { return nil }
        return String(text.dropFirst(key.count)).trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// 先頭行 `名前` / `名前 @ 持ち物` / `ニック (名前)` / `ニック (名前) @ 持ち物`。名前が空なら nil。
    private static func parseHeader(_ line: Line) -> ShowdownParsedMember? {
        var head = line.text
        var item: String?
        let separator = ShowdownTextLabels.itemSeparator
        let bareSeparator = separator.trimmingCharacters(in: .whitespaces)
        if let range = head.range(of: separator) {
            let value = head[range.upperBound...].trimmingCharacters(in: .whitespacesAndNewlines)
            item = value.isEmpty ? nil : value
            head = head[..<range.lowerBound].trimmingCharacters(in: .whitespacesAndNewlines)
        } else if head.hasSuffix(" " + bareSeparator) {
            head = String(head.dropLast(bareSeparator.count)).trimmingCharacters(in: .whitespacesAndNewlines)
        }
        var species = head
        var nickname: String?
        if head.hasSuffix(")"), let open = head.range(of: " (", options: .backwards) {
            let nick = head[..<open.lowerBound].trimmingCharacters(in: .whitespacesAndNewlines)
            let inner = head[open.upperBound...].dropLast().trimmingCharacters(in: .whitespacesAndNewlines)
            if !nick.isEmpty, !inner.isEmpty {
                nickname = nick
                species = inner
            }
        }
        guard !species.isEmpty else { return nil }
        return ShowdownParsedMember(
            speciesName: species, nickname: nickname, itemName: item, headerLineNumber: line.number,
            headerText: line.text, itemLine: item == nil ? nil : line.number)
    }

    /// `32 Atk / 20 Spe`。書き方の誤り・重複・負数は `spMalformed`、範囲外は `spOutOfRange`、合計超過は
    /// `spTotalExceeded`(どれも行全体を捨てる)。
    private static func parseSP(_ value: String) -> Result<[StatKey: Int], ShowdownRejectionReason> {
        var sp: [StatKey: Int] = [:]
        let separator = ShowdownTextLabels.spSeparator.trimmingCharacters(in: .whitespaces)
        for term in value.components(separatedBy: separator) {
            let parts = term.split(whereSeparator: { $0.isWhitespace }).map(String.init)
            guard parts.count == 2,
                parts[0].allSatisfy({ $0.isASCII && $0.isNumber }),
                let stat = StatKey.allCases.first(where: { ShowdownTextLabels.statAbbreviation(for: $0) == parts[1] }),
                sp[stat] == nil
            else {
                return .failure(.spMalformed)
            }
            sp[stat] = Int(parts[0]) ?? Int.max
        }
        if sp.values.contains(where: { $0 > SPLimits.maxPerStat }) { return .failure(.spOutOfRange) }
        if sp.values.reduce(0, +) > SPLimits.maxTotal { return .failure(.spTotalExceeded) }
        return .success(sp)
    }
}

// MARK: - 組み立て(Serializer)

/// 書き出しに使う ID → 表示名の対応(`ShowdownNaming` で作った名前)。
public struct ShowdownNames: Equatable, Sendable {
    public var speciesByKey: [String: String]
    public var itemsById: [String: String]
    public var abilitiesById: [String: String]
    public var naturesById: [String: String]
    public var movesById: [String: String]
    /// タイプの表示名(`ShowdownNaming.name(of: PokeType)`)。
    public var typeNames: [PokeType: String]

    public init(
        speciesByKey: [String: String] = [:], itemsById: [String: String] = [:],
        abilitiesById: [String: String] = [:], naturesById: [String: String] = [:],
        movesById: [String: String] = [:], typeNames: [PokeType: String] = [:]
    ) {
        self.speciesByKey = speciesByKey
        self.itemsById = itemsById
        self.abilitiesById = abilitiesById
        self.naturesById = naturesById
        self.movesById = movesById
        self.typeNames = typeNames
    }
}

/// 1体の書き出し結果。
public struct ShowdownMemberText: Equatable, Sendable {
    public var text: String
    /// 名前を引けずに書かなかった ID(持ち物・特性・性格・技・タイプのどれか)。
    public var unresolvedIds: [String]

    public init(text: String, unresolvedIds: [String] = []) {
        self.text = text
        self.unresolvedIds = unresolvedIds
    }
}

public enum ShowdownTextSerializer {
    /// 1体を書き出す。種族名が引けなければ nil(先頭行を書けない)。
    /// 行の順: 先頭行 → `Ability:` → `Tera Type:` → `SP:` → `Nature:` → `- 技`(1体につき行末の改行なし)。
    /// 値が無い・名前を引けない項目の行は書かない。SP は 0 のステータスを省き、全部 0 なら `SP:` 行ごと省く。
    /// ニックネームがあれば `ニックネーム (種族名)` の形。技は `TeamLimits.maxMovesPerMember` まで。
    public static func serialize(member: TeamMember, names: ShowdownNames) -> ShowdownMemberText? {
        guard let speciesName = names.speciesByKey[member.speciesKey] else { return nil }
        var unresolved: [String] = []
        var lines: [String] = []

        var header = speciesName
        if let nickname = member.nickname, !nickname.isEmpty { header = "\(nickname) (\(speciesName))" }
        if let itemId = member.itemId {
            if let itemName = names.itemsById[itemId] {
                header += ShowdownTextLabels.itemSeparator + itemName
            } else {
                unresolved.append(itemId)
            }
        }
        lines.append(header)

        if let abilityId = member.abilityId {
            if let name = names.abilitiesById[abilityId] {
                lines.append("\(ShowdownTextLabels.abilityKey) \(name)")
            } else {
                unresolved.append(abilityId)
            }
        }
        if let tera = member.teraType, let name = names.typeNames[tera] {
            lines.append("\(ShowdownTextLabels.teraTypeKey) \(name)")
        }
        let spTerms = StatKey.allCases.compactMap { stat -> String? in
            let value = member.sp.value(for: stat)
            return value > 0 ? "\(value) \(ShowdownTextLabels.statAbbreviation(for: stat))" : nil
        }
        if !spTerms.isEmpty {
            lines.append("\(ShowdownTextLabels.spKey) " + spTerms.joined(separator: ShowdownTextLabels.spSeparator))
        }
        if !member.natureId.isEmpty {
            if let name = names.naturesById[member.natureId] {
                lines.append("\(ShowdownTextLabels.natureKey) \(name)")
            } else {
                unresolved.append(member.natureId)
            }
        }
        for moveId in member.moveIds.prefix(TeamLimits.maxMovesPerMember) {
            if let name = names.movesById[moveId] {
                lines.append(ShowdownTextLabels.movePrefix + name)
            } else {
                unresolved.append(moveId)
            }
        }
        return ShowdownMemberText(text: lines.joined(separator: "\n"), unresolvedIds: unresolved)
    }

    /// 複数体を書き出す。1体ずつの書き出しを空行1つ(`"\n\n"`)で区切り、末尾に改行を付けない。
    /// 種族名を引けないメンバーは飛ばし `skippedMemberIds` に入れる。
    public static func serialize(members: [TeamMember], names: ShowdownNames) -> ShowdownTeamText {
        var texts: [String] = []
        var skipped: [String] = []
        var unresolved: [String] = []
        for member in members {
            guard let out = serialize(member: member, names: names) else {
                skipped.append(member.id)
                continue
            }
            texts.append(out.text)
            for id in out.unresolvedIds where !unresolved.contains(id) { unresolved.append(id) }
        }
        return ShowdownTeamText(text: texts.joined(separator: "\n\n"), skippedMemberIds: skipped, unresolvedIds: unresolved)
    }
}

public struct ShowdownTeamText: Equatable, Sendable {
    public var text: String
    public var skippedMemberIds: [String]
    public var unresolvedIds: [String]

    public init(text: String, skippedMemberIds: [String] = [], unresolvedIds: [String] = []) {
        self.text = text
        self.skippedMemberIds = skippedMemberIds
        self.unresolvedIds = unresolvedIds
    }
}
