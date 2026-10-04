import Foundation

/// 表記揺れの辞書(フェーズ4-1)の、設定画面での 1 行 = 1 グループの読み書き(PWA の web/src/lib/aliases.ts と同じ規則。
/// docs/phase4-spec.md AC-IOS-ALI-01)。
public enum AliasLines {
    /// 1 行を語の配列にする。区切りは半角カンマ・全角カンマ・読点。各語の前後の空白(全角を含む)を除き、空の語は捨てる。語の中の空白は残す
    public static func parseLine(_ line: String) -> [String] {
        line.split(whereSeparator: { $0 == "," || $0 == "，" || $0 == "、" })
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
    }

    /// グループを 1 行の表示にする(`, ` でつなぐ)
    public static func format(_ group: [String]) -> String {
        group.joined(separator: ", ")
    }

    /// 行の配列をグループの配列にする。空の行は除く。`invalidRow` は 1 語だけの最初の行(1 始まり)。無ければ nil
    public static func parseGroups(_ lines: [String]) -> (groups: [[String]], invalidRow: Int?) {
        var groups: [[String]] = []
        var invalidRow: Int?
        for (index, line) in lines.enumerated() {
            let words = parseLine(line)
            if words.isEmpty { continue }
            if words.count == 1, invalidRow == nil { invalidRow = index + 1 }
            groups.append(words)
        }
        return (groups, invalidRow)
    }

    /// 1 語だけの行があったときの文言(PWA と同じ)。`別名グループ2は2語以上をカンマで区切って入力してください`
    public static func invalidRowMessage(_ row: Int) -> String {
        "別名グループ\(row)は2語以上をカンマで区切って入力してください"
    }
}
