import Foundation

// TeamBuildSource: 構築から個体を呼び出す(P6-2d・ADR-0501「P6-2d」)。
//
// 「自分側」の SP・性格などの出どころを、定型プリセット(`AttackerPreset` / `KnownDefenderPreset`)と
// 構築から呼び出した個体のスナップショットの排他的な直和にする(1章)。画面(`CalcViewModel` /
// `ReverseViewModel`)がこの型を状態として持ち、要求の組み立ては呼び出した個体をそのまま使う
// (プリセットに丸め直さない)。

/// 自分側の SP・性格などの出どころ。`.preset` と `.team` は型として排他(1章「判断」)。
public enum BuildSource<Preset: Equatable & Sendable>: Equatable, Sendable {
    case preset(Preset)
    case team(TeamIndividualSelection)

    /// `.preset` のときだけ非 nil。
    public var preset: Preset? {
        if case .preset(let value) = self { return value }
        return nil
    }

    /// `.team` のときだけ非 nil。
    public var teamSelection: TeamIndividualSelection? {
        if case .team(let value) = self { return value }
        return nil
    }
}

/// 計算画面の攻撃側・逆算「与えたダメージ」の自分の出どころ。
public typealias AttackerBuildSource = BuildSource<AttackerPreset>
/// 逆算「受けたダメージ」の自分(既知の防御側)の出どころ。
public typealias KnownDefenderBuildSource = BuildSource<KnownDefenderPreset>

/// 構築から呼び出した個体のスナップショット(1章「判断: ID だけを持たず選んだ時点の
/// `Individual` を写して持つ」)。呼び出したあとに構築ビルダー側でその個体を編集しても
/// この値は追従しない(呼び出し直すと反映される)。
public struct TeamIndividualSelection: Equatable, Sendable {
    public let teamID: String
    public let memberID: String
    /// 画面に出す名前(ニックネーム、無ければ種族名、それも無ければ speciesKey)。
    public let displayName: String
    /// 選んだ時点の `TeamMemberConverter.makeIndividual(from:moves:)` の結果。
    public let individual: Individual

    public init(teamID: String, memberID: String, displayName: String, individual: Individual) {
        self.teamID = teamID
        self.memberID = memberID
        self.displayName = displayName
        self.individual = individual
    }
}

extension TeamIndividualSelection {
    /// スナップショットの性格・SP・特性・テラスタイプと、画面がいま持っている種族・持ち物を
    /// 組み合わせて要求用の `Individual` を作る(2章の表: 種族・持ち物・技は画面の状態が正)。
    /// `moveId` は要求本体の `moveId` が正なので常に nil にそろえる。
    public func individualForRequest(speciesKey: String, itemId: String?) -> Individual {
        Individual(
            speciesKey: speciesKey,
            natureId: individual.natureId,
            sp: individual.sp,
            abilityId: individual.abilityId,
            itemId: itemId,
            teraType: individual.teraType
        )
    }
}

// MARK: - 構築の一覧を画面に出す形(6章)

/// 構築1体分の選択肢。
public struct TeamMemberOption: Identifiable, Equatable, Sendable {
    public let id: String
    /// nickname(前後空白を落として空でなければ)→ 種族名(マスタ)→ speciesKey。
    public let displayName: String
    public let speciesKey: String

    public init(id: String, displayName: String, speciesKey: String) {
        self.id = id
        self.displayName = displayName
        self.speciesKey = speciesKey
    }
}

/// 構築1つ分の選択肢のまとまり。
public struct TeamPickerGroup: Identifiable, Equatable, Sendable {
    public let id: String
    public let name: String
    public let members: [TeamMemberOption]

    public init(id: String, name: String, members: [TeamMemberOption]) {
        self.id = id
        self.name = name
        self.members = members
    }
}

/// `Team` の一覧から画面のピッカーの選択肢を作る純粋関数(6章「判断: 表示名の組み立ては
/// Core の純粋関数に置く」)。
public enum TeamIndividualOptions {
    /// メンバーが0体の構築は出さない(選べるものが無いため)。構築・メンバーの並び順はそのまま保つ。
    public static func groups(from teams: [Team], species: [SpeciesSummary]) -> [TeamPickerGroup] {
        teams.compactMap { team in
            guard !team.members.isEmpty else { return nil }
            let members = team.members.map { member in
                TeamMemberOption(
                    id: member.id,
                    displayName: displayName(for: member, species: species),
                    speciesKey: member.speciesKey
                )
            }
            return TeamPickerGroup(id: team.id, name: team.name, members: members)
        }
    }

    private static func displayName(for member: TeamMember, species: [SpeciesSummary]) -> String {
        if let nickname = member.nickname?.trimmingCharacters(in: .whitespacesAndNewlines), !nickname.isEmpty {
            return nickname
        }
        if let match = species.first(where: { $0.key == member.speciesKey }) {
            return match.nameJa
        }
        return member.speciesKey
    }
}

/// 画面に出す文言(`DisplayLabels.swift` と同じ理由でマスタに無い表示専用の語彙を Core に1か所置く)。
public enum TeamLoadLabels {
    public static let entryTitle = "構築から選ぶ"
    public static let empty = "まだ構築がありません"
}

// MARK: - CalcViewModel / ReverseViewModel 共通の下ごしらえ(内部専用)

/// 画面に見えている選択肢(`teamOptions`)と、読み込み済みの構築本体から `TeamIndividualSelection` を
/// 組み立てる。`teamOptions` に無い teamID/memberID は nil(未知の ID・メンバー0体の構築を無視する
/// 規則の実体。3章「判断」)。
enum TeamIndividualSelectionBuilder {
    static func make(
        teamID: String, memberID: String,
        teamOptions: [TeamPickerGroup], teams: [Team], moves: [Move]
    ) -> TeamIndividualSelection? {
        guard let option = teamOptions.first(where: { $0.id == teamID })?.members.first(where: { $0.id == memberID }),
              let team = teams.first(where: { $0.id == teamID }),
              let member = team.members.first(where: { $0.id == memberID })
        else { return nil }
        let individual = TeamMemberConverter.makeIndividual(from: member, moves: moves)
        return TeamIndividualSelection(teamID: teamID, memberID: memberID, displayName: option.displayName, individual: individual)
    }
}

/// `TeamStore.list()` を呼び、種族マスタと組み合わせて選択肢を作る。失敗しても空を返す
/// (呼び出し元の計算画面・逆算画面を壊さない。5章「判断」)。
enum TeamListFetcher {
    static func fetchGroups(from teamStore: any TeamStore, species: [SpeciesSummary]) async -> (teams: [Team], groups: [TeamPickerGroup]) {
        do {
            let teams = try await teamStore.list()
            return (teams, TeamIndividualOptions.groups(from: teams, species: species))
        } catch {
            return ([], [])
        }
    }
}
