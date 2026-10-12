// SpeedMatchupDisplay: matchup・reach の応答を画面に出す形に整える純粋関数と表示用の型(I-speed-4c。ADR-0610 §1・§4)。
//
// 計算はしない(SP の区間・族の区切りは応答のまま)。語の選び方(見出し・性格の行・「これ以上」の言い換え)だけを持つ。
// 規則の例は SpeedMatchupDisplayTests が固定する。
//
// spec-writer の足場: 型の形は本物。`SpeedMatchupDisplayBuilder` の関数は未実装で空配列を返す。

import Foundation

/// 性格 1 つぶんの行(例「補正なし  S31以上で抜ける」)。
public struct SpeedNatureRowDisplay: Equatable, Identifiable, Sendable {
    /// `SpeedNature.rawValue`。
    public let id: String
    public let nature: SpeedNature
    /// 「下降」「補正なし」「上昇」
    public let natureLabel: String
    /// 本文(`SpeedMatchupLabels.firstLine` / `trickRoomLine`)。
    public let text: String
    /// 上を取れる SP があるか(色だけに頼らず、文字の違いと一緒に使う)。
    public let isReachable: Bool
    /// 同速の SP の文(同速が無ければ nil)。
    public let tieText: String?

    public init(nature: SpeedNature, natureLabel: String, text: String, isReachable: Bool, tieText: String?) {
        self.id = nature.rawValue
        self.nature = nature
        self.natureLabel = natureLabel
        self.text = text
        self.isReachable = isReachable
        self.tieText = tieText
    }
}

/// 想定 1 つぶんのカード。
public struct SpeedMatchupCardDisplay: Equatable, Identifiable, Sendable {
    /// プリセットは `SpeedPresetID.rawValue`(例 `max-scarf`)、カスタムは `custom-<カスタムの中の通し番号>`。
    public let id: String
    /// 「最速」「カスタム(S20・上昇)」
    public let assumptionLabel: String
    /// 「相手の素早さ 200」
    public let opponentSpeedLabel: String
    /// 見出し(B の要約語。`SpeedMatchupLabels.headline` / `trickRoomHeadline`)。
    public let headline: String
    /// 性格ごとの行。下降は**トリックルーム中だけ**含める(ADR-0610 §6 Q5)。順は 下降・補正なし・上昇。
    public let rows: [SpeedNatureRowDisplay]
    /// 今の振りがあるときの行動順(「先に動く」など)。無ければ nil。
    public let orderBadge: String?
    public let order: SpeedTurnOrder?

    public init(
        id: String, assumptionLabel: String, opponentSpeedLabel: String, headline: String,
        rows: [SpeedNatureRowDisplay], orderBadge: String?, order: SpeedTurnOrder?
    ) {
        self.id = id
        self.assumptionLabel = assumptionLabel
        self.opponentSpeedLabel = opponentSpeedLabel
        self.headline = headline
        self.rows = rows
        self.orderBadge = orderBadge
        self.order = order
    }
}

/// 「これ以上」の 1 行(表の 1 プリセット)。
public struct SpeedReachRowDisplay: Equatable, Identifiable, Sendable {
    /// `SpeedPresetID.rawValue`。
    public let id: String
    /// 調整名(「準速」など)。
    public let presetLabel: String
    /// 「準速 117族まで抜ける」
    public let slowerText: String
    /// 「準速 118族と同速」。同速の族が無ければ nil。
    public let tieText: String?
    /// 「準速 119族以上に抜かれる」
    public let fasterText: String
    /// 「5体」
    public let slowerCountLabel: String
    public let tieCountLabel: String
    public let fasterCountLabel: String
    /// 「例: ○○」。無ければ nil。
    public let slowerExample: String?
    public let fasterExample: String?

    public init(
        id: String, presetLabel: String, slowerText: String, tieText: String?, fasterText: String,
        slowerCountLabel: String, tieCountLabel: String, fasterCountLabel: String,
        slowerExample: String?, fasterExample: String?
    ) {
        self.id = id
        self.presetLabel = presetLabel
        self.slowerText = slowerText
        self.tieText = tieText
        self.fasterText = fasterText
        self.slowerCountLabel = slowerCountLabel
        self.tieCountLabel = tieCountLabel
        self.fasterCountLabel = fasterCountLabel
        self.slowerExample = slowerExample
        self.fasterExample = fasterExample
    }
}

public enum SpeedMatchupDisplayBuilder {
    /// 応答の `lines`(要求の想定の順)をカードにする。`trickRoom` は応答の `trickRoom` を使う。
    public static func cards(_ matchup: SpeedMatchup) -> [SpeedMatchupCardDisplay] {
        var customIndex = 0
        return matchup.lines.map { line in
            let id: String
            let label: String
            switch line.assumption {
            case .preset(let preset):
                id = preset.rawValue
                label = SpeedLabels.preset(preset)
            case .custom(let custom):
                id = "custom-\(customIndex)"
                customIndex += 1
                label = SpeedMatchupLabels.customAssumption(custom)
            }
            let trickRoom = matchup.trickRoom
            // 下降の行はトリックルーム中だけ(ADR-0610 §6 Q5)。順は 下降・補正なし・上昇。
            let rows = line.natures
                .filter { trickRoom || $0.nature != .minus }
                .map { natureRow($0, trickRoom: trickRoom) }
            return SpeedMatchupCardDisplay(
                id: id, assumptionLabel: label, opponentSpeedLabel: SpeedMatchupLabels.opponentSpeed(line.opponentSpeed),
                headline: trickRoom
                    ? SpeedMatchupLabels.trickRoomHeadline(natures: line.natures)
                    : SpeedMatchupLabels.headline(natures: line.natures),
                rows: rows, orderBadge: line.current.map(SpeedMatchupLabels.orderBadge), order: line.current)
        }
    }

    private static func natureRow(_ line: SpeedNatureLine, trickRoom: Bool) -> SpeedNatureRowDisplay {
        SpeedNatureRowDisplay(
            nature: line.nature, natureLabel: SpeedLabels.nature(line.nature),
            text: trickRoom ? SpeedMatchupLabels.trickRoomLine(line.movesFirst) : SpeedMatchupLabels.firstLine(line.movesFirst),
            isReachable: line.movesFirst != nil, tieText: line.tie.map(SpeedMatchupLabels.tieLine))
    }

    /// 応答の `lines`(6 件)を行にする。`trickRoom`(今の場の状態)で「抜ける」の言い換えが変わる
    /// (reach の応答はトリックルームに依存しないので、言い換えだけ画面側で切り替える。ADR-0610 §1)。
    public static func reachRows(_ reach: SpeedReach, trickRoom: Bool) -> [SpeedReachRowDisplay] {
        reach.lines.map { line in
            let preset = SpeedLabels.preset(line.preset)
            return SpeedReachRowDisplay(
                id: line.preset.rawValue, presetLabel: preset,
                slowerText: SpeedMatchupLabels.reachSlower(preset: preset, range: line.slower, trickRoom: trickRoom),
                tieText: line.tie.map { SpeedMatchupLabels.reachTie(preset: preset, range: $0) },
                fasterText: SpeedMatchupLabels.reachFaster(preset: preset, range: line.faster, trickRoom: trickRoom),
                slowerCountLabel: SpeedMatchupLabels.count(line.slowerCount),
                tieCountLabel: SpeedMatchupLabels.count(line.tieCount),
                fasterCountLabel: SpeedMatchupLabels.count(line.fasterCount),
                slowerExample: line.fastestSlower.map { SpeedMatchupLabels.example($0.nameJa) },
                fasterExample: line.slowestFaster.map { SpeedMatchupLabels.example($0.nameJa) })
        }
    }
}
