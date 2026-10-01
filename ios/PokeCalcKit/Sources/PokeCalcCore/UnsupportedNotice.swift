// UnsupportedNotice: 未対応の印(`UnsupportedMark`。ADR-0123)を画面に出す形へ整形する
// (P6-17。ADR-0501「P6-17」)。
//
// 置き場所の規則(`UnsupportedPlacement`): 結果の全行(逆算は全候補)に共通する印は結果の上に1回だけ
// (`summary`)、一部の行にだけある印はその行に(`rowNote`)出す。技・攻撃側の持ち物/特性の印は全行に
// 付くので上に1回、持ち物の比較で増えた行の防御側の持ち物の印は行ごとになる。
// 純粋な整形だけを持ち、View はここが作った文字列をそのまま描く(`BulkRowDisplay` と同じ方針)。

/// 印の `id` を日本語名へ引く辞書(マスタから作る)。見つからない ID は ID のまま出す
/// (黙って消さない。`BulkRowDisplay.itemLabel` と同じ方針)。
public struct UnsupportedMarkNames: Equatable, Sendable {
    public var moveNames: [String: String]
    public var itemNames: [String: String]
    public var abilityNames: [String: String]

    public init(moveNames: [String: String] = [:], itemNames: [String: String] = [:], abilityNames: [String: String] = [:]) {
        self.moveNames = moveNames
        self.itemNames = itemNames
        self.abilityNames = abilityNames
    }

    /// マスタの一覧から作る(同じ ID が複数あれば後勝ち)。
    public init(moves: [Move], items: [Item], abilities: [Ability]) {
        self.init(
            moveNames: Dictionary(moves.map { ($0.id, $0.nameJa) }, uniquingKeysWith: { _, latest in latest }),
            itemNames: Dictionary(items.map { ($0.id, $0.nameJa) }, uniquingKeysWith: { _, latest in latest }),
            abilityNames: Dictionary(abilities.map { ($0.id, $0.nameJa) }, uniquingKeysWith: { _, latest in latest })
        )
    }

    /// `target` で辞書を選び(技 → moveNames、持ち物 → itemNames、特性 → abilityNames)、無ければ `mark.id`。
    public func name(for mark: UnsupportedMark) -> String {
        let dictionary: [String: String]
        switch mark.target {
        case .move: dictionary = moveNames
        case .attackerItem, .defenderItem: dictionary = itemNames
        case .attackerAbility, .defenderAbility: dictionary = abilityNames
        }
        return dictionary[mark.id] ?? mark.id
    }
}

/// 印の置き場所(結果の上に1回 / 行ごと)を決める。
public struct UnsupportedPlacement: Equatable, Sendable {
    /// すべての行(候補)に付いている印。1行目の順を保つ。行が無ければ空。
    public let common: [UnsupportedMark]
    /// 各行から `common` を除いた残り(行の順・行の中の印の順を保つ)。`perEntry.count` は入力の行数と同じ。
    public let perEntry: [[UnsupportedMark]]

    /// 同じ行の中の重複した印は1つにまとめる(契約上は来ないが、同じ文言を2回出さない)。
    public init(_ lists: [[UnsupportedMark]]) {
        // 行ごとにまず重複を除く(順序は保つ)。
        let deduped: [[UnsupportedMark]] = lists.map { entry in
            var seen = Set<UnsupportedMark>()
            return entry.filter { seen.insert($0).inserted }
        }
        guard let first = deduped.first else {
            common = []
            perEntry = []
            return
        }
        let commonMarks = first.filter { mark in deduped.allSatisfy { $0.contains(mark) } }
        common = commonMarks
        let commonSet = Set(commonMarks)
        perEntry = deduped.map { entry in entry.filter { !commonSet.contains($0) } }
    }
}

/// 印の文言(結果の上の注記・行の注記)。印が無ければ nil(View は何も描かない)。
public enum UnsupportedNoticeText {
    /// 結果の上に1回だけ出す注記。`この結果は正確でない可能性があります(未対応: <印>、<印>)`。
    public static func summary(_ marks: [UnsupportedMark], names: UnsupportedMarkNames) -> String? {
        guard !marks.isEmpty else { return nil }
        return "この結果は正確でない可能性があります(未対応: \(joinedText(marks, names: names)))"
    }

    /// 行(候補カード)に出す短い注記。`未対応: <印>、<印>`。
    public static func rowNote(_ marks: [UnsupportedMark], names: UnsupportedMarkNames) -> String? {
        guard !marks.isEmpty else { return nil }
        return "未対応: \(joinedText(marks, names: names))"
    }

    private static func joinedText(_ marks: [UnsupportedMark], names: UnsupportedMarkNames) -> String {
        marks.map { UnsupportedMarkLabel.text(for: $0, name: names.name(for: $0)) }.joined(separator: "、")
    }
}

/// 一括計算の結果全体を画面向けに整形した値(行 + 結果の上の注記)。`CalcViewModel` が使う。
public struct BulkResultDisplay: Equatable, Sendable {
    public let rows: [BulkRowDisplay]
    /// 全行に共通する印の注記(無ければ nil)。
    public let unsupportedNotice: String?

    public init(result: BulkCalcResult, items: [Item], names: UnsupportedMarkNames) {
        let placement = UnsupportedPlacement(result.rows.map(\.result.unsupported))
        unsupportedNotice = UnsupportedNoticeText.summary(placement.common, names: names)
        rows = zip(result.rows, placement.perEntry).map { row, marks in
            BulkRowDisplay(row: row, items: items, unsupportedNote: UnsupportedNoticeText.rowNote(marks, names: names))
        }
    }
}
