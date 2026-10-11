import Foundation

// MoveSort: 技の選択肢の並び(G-01・ADR-0527。F-02 / ADR-0523 の切り替えを廃止し、タイプ順だけにした)。
//
// 画面の表示順だけを変える純粋関数で、選択中の技・計算の要求・結果には影響しない。
// 「タイプ表の並び」は `PokeType` の宣言順(docs/design.md のタイプ色パレットと同じ並び)。

/// タイプ順の1群(見出し = タイプ名)。
public struct MoveTypeGroup: Equatable, Sendable {
    public var type: PokeType
    public var moves: [Move]
}

public enum MoveSort {
    /// タイプ順に並べた新しい配列(入力は変えない)。群の中は五十音順。
    public static func byType(_ moves: [Move]) -> [Move] {
        typeGroups(moves).flatMap(\.moves)
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
