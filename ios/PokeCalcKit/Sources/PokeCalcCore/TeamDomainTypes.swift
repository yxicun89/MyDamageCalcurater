import Foundation

// TeamDomainTypes: 構築ビルダーのドメインの型(P6-2c・ADR-0500 §4、ADR-0501「P6-2c」1章)。
//
// `TeamStore`(端末内保存)を介して JSON で永続化するため、`TeamMember` / `Team` は `Codable` に
// する(`DomainTypes.swift` 側で `StatBlock` / `RankBlock` / `PokeType` に `Codable` を足した理由も同じ)。

/// 構築ビルダーの制約値。requirements.md「構築ビルダー」の6体パーティ、ADR-0016 §1 balance TB2 と
/// 同じ「メンバーごとの技IDは最大4つ・同一メンバー内の重複は不可」を踏襲する。
public enum TeamLimits {
    public static let maxMembers = 6
    public static let maxMovesPerMember = 4
}

/// 構築の1体。ID はマスタ参照のためだけに持ち、マスタに実在するかどうかはこの型では確かめない
/// (`Individual` と同じ、ID をそのまま運ぶ値型。ADR-0501「P6-2c」1章)。
public struct TeamMember: Equatable, Sendable, Codable {
    public var id: String
    public var speciesKey: String
    /// 任意のニックネーム。requirements.md には無いが実機の構築には一般的にあり、低コストな
    /// 任意フィールドなので先に用意する(ADR-0501「P6-2c」1章の判断)。
    public var nickname: String?
    /// 0〜4。`TeamLimits.maxMovesPerMember` を超えない・同一メンバー内で重複しない
    /// (`TeamValidator` が確かめる。この型自体は不変条件を強制しない)。
    public var moveIds: [String]
    public var itemId: String?
    public var abilityId: String?
    public var natureId: String
    public var sp: StatBlock
    public var teraType: PokeType?

    public init(
        id: String = UUID().uuidString,
        speciesKey: String,
        nickname: String? = nil,
        moveIds: [String] = [],
        itemId: String? = nil,
        abilityId: String? = nil,
        natureId: String,
        sp: StatBlock = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0),
        teraType: PokeType? = nil
    ) {
        self.id = id
        self.speciesKey = speciesKey
        self.nickname = nickname
        self.moveIds = moveIds
        self.itemId = itemId
        self.abilityId = abilityId
        self.natureId = natureId
        self.sp = sp
        self.teraType = teraType
    }
}

/// 構築(パーティ)。`members` は追加順(並べ替えは範囲外。ADR-0501「P6-2c」1章「範囲外」)。
public struct Team: Equatable, Sendable, Codable {
    public var id: String
    public var name: String
    /// 0〜6。`TeamValidator` が上限を確かめる。
    public var members: [TeamMember]

    public init(id: String = UUID().uuidString, name: String, members: [TeamMember] = []) {
        self.id = id
        self.name = name
        self.members = members
    }
}

/// `Team` の不変条件を確かめる純粋関数。`TeamStore.save` はこの結果をそのまま投げて却下する
/// (無効な値を丸めて保存し直さない。保存した内容が画面の見た目と食い違うのを避けるため。
/// ADR-0501「P6-2c」1章)。
public enum TeamValidator {
    /// 最初に見つかった違反を1つだけ返す(無ければ nil)。判定順は ADR-0501「P6-2c」1章のとおり:
    /// name → メンバー数 → 各メンバーを先頭から(技数 → 技重複 → SP単体 → SP合計)。
    public static func firstViolation(in team: Team) -> PokeCalcError? {
        if team.name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return PokeCalcError(code: PokeCalcError.Code.teamNameEmpty, message: "チーム名を入力してください")
        }
        if team.members.count > TeamLimits.maxMembers {
            return PokeCalcError(
                code: PokeCalcError.Code.teamTooManyMembers,
                message: "パーティは\(TeamLimits.maxMembers)体までです"
            )
        }
        for member in team.members {
            if member.moveIds.count > TeamLimits.maxMovesPerMember {
                return PokeCalcError(
                    code: PokeCalcError.Code.teamTooManyMoves,
                    message: "技は\(TeamLimits.maxMovesPerMember)個までです"
                )
            }
            if Set(member.moveIds).count != member.moveIds.count {
                return PokeCalcError(code: PokeCalcError.Code.teamDuplicateMoves, message: "同じ技が重複しています")
            }
            if member.sp.values.contains(where: { $0 > SPLimits.maxPerStat }) {
                return PokeCalcError(
                    code: PokeCalcError.Code.teamSPInvalid,
                    message: "能力ポイントは1ステータスにつき\(SPLimits.maxPerStat)までです"
                )
            }
            if member.sp.total > SPLimits.maxTotal {
                return PokeCalcError(
                    code: PokeCalcError.Code.teamSPInvalid,
                    message: "能力ポイントの合計は\(SPLimits.maxTotal)までです"
                )
            }
        }
        return nil
    }
}

// MARK: - StatBlock のステータスキー越しのアクセス(TeamValidator / TeamEditViewModel が使う内部ヘルパー)

extension StatBlock {
    /// `StatKey` 順の値一覧(合計・上限判定に使う。`DomainTypes.swift` は変更しないので、
    /// このファイル内の internal extension として持つ)。
    var values: [Int] { [hp, atk, def, spa, spd, spe] }

    var total: Int { values.reduce(0, +) }

    func value(for stat: StatKey) -> Int {
        switch stat {
        case .hp: return hp
        case .atk: return atk
        case .def: return def
        case .spa: return spa
        case .spd: return spd
        case .spe: return spe
        }
    }

    mutating func setValue(_ value: Int, for stat: StatKey) {
        switch stat {
        case .hp: hp = value
        case .atk: atk = value
        case .def: def = value
        case .spa: spa = value
        case .spd: spd = value
        case .spe: spe = value
        }
    }
}
