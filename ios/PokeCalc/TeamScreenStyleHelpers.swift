import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamScreenStyleHelpers: 構築画面(一覧・編集)だけで使う見た目のヘルパーとエラー文言(P6-2c)。
//
// `TeamFieldError` / `TeamMemberFieldError`(PokeCalcCore の `TeamScreenError.swift`)はケースだけを
// 持つ enum で、文言を持たない(ADR-0501「P6-2c」3章)。`Team*.swift` は実装依頼の指示で変更しない
// ため、表示文言はこの View 層の extension で組み立てる(`TeamValidator`[Core]が同じ違反を判定する
// ときに使うメッセージと文面をそろえてある)。

extension TeamFieldError {
    /// 画面に出す文言。
    var uiMessage: String {
        switch self {
        case .emptyName:
            return "チーム名を入力してください"
        case .tooManyMembers:
            return "パーティは\(TeamLimits.maxMembers)体までです"
        }
    }
}

extension TeamMemberFieldError {
    /// 画面に出す文言。
    var uiMessage: String {
        switch self {
        case .duplicateMove:
            return "同じ技はもう一度選べません"
        case .tooManyMoves:
            return "技は\(TeamLimits.maxMovesPerMember)個までです"
        case .spPerStatExceeded:
            return "能力ポイントは1ステータスにつき\(SPLimits.maxPerStat)までです"
        case .spTotalExceeded:
            return "能力ポイントの合計は\(SPLimits.maxTotal)までです"
        }
    }
}

/// `StatBlock` は `PokeCalcCore` 内部(internal)の `value(for:)` / `setValue(for:)` しか持たない
/// (`TeamValidator` / `TeamEditViewModel` だけが使う想定のヘルパーのため。ADR-0501「P6-2c」1章)。
/// View 層(別モジュール)は公開フィールド(`hp`/`atk`/... )だけから読むので、ここに同じ形の
/// ヘルパーを持つ(Core 側を変更しない制約のため、重複は許容する)。
extension StatBlock {
    /// `StatKey` 順の値一覧。
    var teamEditValues: [Int] { [hp, atk, def, spa, spd, spe] }

    func teamEditValue(for stat: StatKey) -> Int {
        switch stat {
        case .hp: return hp
        case .atk: return atk
        case .def: return def
        case .spa: return spa
        case .spd: return spd
        case .spe: return spe
        }
    }
}
