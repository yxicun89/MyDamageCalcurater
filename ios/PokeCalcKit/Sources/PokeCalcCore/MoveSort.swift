import Foundation

// MoveSort: 技の選択肢の並び(F-02・ADR-0523。Web の ADR-0335 と同じ語・同じ規則)。
//
// 習得順(既定。入力の順のまま)・五十音順・タイプ順。画面の表示順だけを変える純粋関数で、
// 選択中の技・計算の要求・結果には影響しない。タイプ順の「タイプ表の並び」は `PokeType` の宣言順
// (docs/design.md のタイプ色パレットと同じ並び)。

/// 並びの種類。`rawValue` は保存値(不正値は既定に戻す)。
public enum MoveSortOrder: String, CaseIterable, Sendable, Hashable {
    case learnset, kana, type

    /// 既定は習得順(従来の並び)。
    public static let `default`: MoveSortOrder = .learnset
}

public enum MoveSortLabels {
    public static let groupLabel = "技の並び"

    public static func label(for order: MoveSortOrder) -> String {
        switch order {
        case .learnset: "習得順"
        case .kana: "五十音順"
        case .type: "タイプ順"
        }
    }
}

/// タイプ順の1群(見出し = タイプ名)。
public struct MoveTypeGroup: Equatable, Sendable {
    public var type: PokeType
    public var moves: [Move]
}

public enum MoveSort {
    /// 並べ替えた新しい配列(入力は変えない)。
    public static func sorted(_ moves: [Move], by order: MoveSortOrder) -> [Move] {
        switch order {
        case .learnset: moves
        case .kana: moves.sorted(by: kanaPrecedes)
        case .type: typeGroups(moves).flatMap(\.moves)
        }
    }

    /// タイプ表の並びで群にし、群の中は五十音順。技のあるタイプだけ。
    public static func typeGroups(_ moves: [Move]) -> [MoveTypeGroup] {
        PokeType.allCases.compactMap { type in
            let inType = moves.filter { $0.type == type }.sorted(by: kanaPrecedes)
            return inType.isEmpty ? nil : MoveTypeGroup(type: type, moves: inType)
        }
    }

    /// 五十音順の比較。ひらがな/カタカナは同じ字として扱い(カタカナをひらがなへ寄せて比べる)、
    /// 日本語の照合器(濁点は同じ字の中で後・長音は照合器の扱い)で決める。同順位は技 ID の昇順。
    static func kanaPrecedes(_ lhs: Move, _ rhs: Move) -> Bool {
        let left = collationKey(lhs.nameJa)
        let right = collationKey(rhs.nameJa)
        switch left.compare(right, options: [], range: nil, locale: collationLocale) {
        case .orderedAscending: return true
        case .orderedDescending: return false
        case .orderedSame: return lhs.id < rhs.id
        }
    }

    private static let collationLocale = Locale(identifier: "ja")

    private static func collationKey(_ name: String) -> String {
        name.applyingTransform(.hiraganaToKatakana, reverse: true) ?? name
    }
}

/// 並びの記憶(UserDefaults の1キー。`LocalTeamStore` と同じく保存先を注入できる)。
/// 読めない・不正な値は既定(習得順)。
public struct MoveSortStore: Sendable {
    public static let defaultsKey = "pokecalc.moveSort"

    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public func load() -> MoveSortOrder {
        defaults.string(forKey: Self.defaultsKey).flatMap(MoveSortOrder.init(rawValue:)) ?? .default
    }

    public func save(_ order: MoveSortOrder) {
        defaults.set(order.rawValue, forKey: Self.defaultsKey)
    }
}
