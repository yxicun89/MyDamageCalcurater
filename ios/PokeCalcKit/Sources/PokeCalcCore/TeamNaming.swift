import Foundation

// TeamNaming / TeamLabels: 構築の作り直し(F-08。ADR-0522)の表示名と画面の文言。
//
// 構築名は廃止した。保存する名前は既定名 `TeamNaming.defaultName`(Web・team-svc の既定名と同じ値。ADR-0229)。
// 一覧の表示名は、既定名なら「構築 N」、それ以外(旧データで名前を持つもの)は保存された名前。
// 文言の語は Web(ADR-0332 §6)と同じにする(usability-round2 §5)。

public enum TeamNaming {
    /// 保存する既定名。画面には出さない(表示は `untitled(_:)`)。
    public static let defaultName = "名称未設定"

    public static func untitled(_ number: Int) -> String { "構築 \(number)" }

    /// 構築 id → 表示名。`teams` は作成の古い順(`TeamStore.list()` の追加順)で渡す。
    /// N は既定名の構築だけを渡された順に数えた 1 始まりの番号(名前付きの旧データは数えない)。
    public static func displayNames(for teams: [Team]) -> [String: String] {
        var result: [String: String] = [:]
        var number = 0
        for team in teams {
            if team.name == defaultName {
                number += 1
                result[team.id] = untitled(number)
            } else {
                result[team.id] = team.name
            }
        }
        return result
    }
}

public enum TeamLabels {
    // 一覧
    public static let createButton = "新しい構築"
    public static let emptyList = "まだ構築がありません。「新しい構築」を押すと、ポケモンを6体まで選んで構築を作れます"
    public static let openButton = "開く"
    public static let deleteButton = "削除"
    public static func openHint(name: String) -> String { "「\(name)」を開く" }
    public static func deleteHint(name: String) -> String { "「\(name)」を削除" }
    public static func deleteConfirmNotice(name: String) -> String { "「\(name)」を削除します。よろしいですか" }
    public static let deleteConfirmButton = "削除する"
    public static let deleteCancelButton = "やめる"
    public static func memberCount(_ count: Int) -> String { "\(count)/\(TeamLimits.maxMembers)体" }
    public static let updatedUnknown = "最終更新: -"
    public static func updatedText(_ date: Date?) -> String {
        guard let date else { return updatedUnknown }
        return "最終更新: " + date.formatted(
            Date.FormatStyle(date: .numeric, time: .shortened).locale(Locale(identifier: "ja_JP")))
    }

    // 編集
    public static let backToList = "一覧に戻る"
    public static let save = "保存"
    public static let savedNotice = "保存しました"
    public static let unsavedNotice = "保存していない変更があります"
    public static let leaveConfirmNotice = "保存していない変更があります。保存せずに一覧に戻りますか"
    public static let leaveDiscard = "保存せずに戻る"
    public static let leaveCancel = "編集を続ける"
    public static let pokemonField = "ポケモン"
    public static let emptySlotHint = "ポケモンを選ぶと、技・持ち物・特性などを決められます"
    public static func slotTitle(_ number: Int) -> String { "\(number)体目" }
    public static func moveUp(_ number: Int) -> String { "\(number)体目を上へ" }
    public static func moveDown(_ number: Int) -> String { "\(number)体目を下へ" }
    public static func remove(_ number: Int) -> String { "\(number)体目を外す" }

    // Showdown 形式(補助)
    public static let importFold = "Showdown 形式で取り込む"
    public static let exportFold = "Showdown 形式で書き出す"
    public static let importHelp =
        "Pokémon Showdown などで作った構築のテキストを貼り付けると、新しい構築として取り込めます。ポケモン・持ち物・特性・技は日本語の名前で書き、ポケモンごとに空の行で区切ります"
    public static let importExampleLabel = "入力の例(1体分)"
    /// 入力例。実在のポケモン名・技名を書かないひな形(ADR-0002)。書式はこのアプリのパーサに合わせる
    /// (能力ポイントは `SP:`。ADR-0502)。
    public static let importExample = [
        "ポケモンの名前 @ 持ち物の名前",
        "Ability: 特性の名前",
        "Nature: 性格の名前",
        "SP: 32 Atk / 32 Spe",
        "- 1つ目の技の名前",
        "- 2つ目の技の名前",
        "- 3つ目の技の名前",
    ].joined(separator: "\n")
    public static let exportHelp = "いまの内容を Showdown 形式のテキストにします。コピーして他のアプリに貼り付けられます"
    public static func importCreatedNotice(count: Int) -> String { "\(count)体の構築を作りました" }
}
