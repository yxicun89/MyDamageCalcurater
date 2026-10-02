// JudgeDomainTypes: 判定画面(P6-25。ADR-0504)のドメインの型と、画面が依存する境界 `JudgeService`。
//
// 契約は services/judge/api/openapi.yaml(生成物は PokeCalcJudgeAPI)。ここの型は生成型に依存しない
// (`PokeCalcService`・`SpeedService` のドメイン型と同じ方針。ADR-0500 §3)。生成型 ↔ ドメインの写像は `APIJudgeService` に閉じる。
// 素早さ・行動順・確定数はすべて judge-svc が決める。この層は応答を表示用に整えるだけで、勝敗の真偽値には丸めない
// (ADR-0700 §6-1・ADR-0704 §3)。
//
// spec-writer の足場: 型の形(名前・引数)は JudgeDomainTypesTests・APIJudgeServiceTests・JudgeViewModel*Tests が固定する。

import Foundation

/// 判定の要求の自分・相手候補の1体(openapi `Individual` / `DefenderCandidate` の技を除いた部分)。
/// 「送らないものは nil」にしてあり、API 実装はこの値をそのまま写す(省略の規則は `JudgeViewModel` が決める。ADR-0504 §5)。
public struct JudgeIndividual: Equatable, Sendable {
    public var speciesKey: String
    public var natureId: String
    public var sp: StatBlock
    /// nil なら送らない(5項目すべて 0 のとき。契約上は省略可で既定 0)。非 nil なら5項目すべてを送る(生成型の `RankBlock` は5項目を持つ)。
    public var ranks: RankBlock?
    /// nil なら欄ごと送らない(`null` を明示的に送らない)。
    public var abilityId: String?
    public var itemId: String?

    public init(
        speciesKey: String, natureId: String, sp: StatBlock, ranks: RankBlock? = nil,
        abilityId: String? = nil, itemId: String? = nil
    ) {
        self.speciesKey = speciesKey
        self.natureId = natureId
        self.sp = sp
        self.ranks = ranks
        self.abilityId = abilityId
        self.itemId = itemId
    }
}

/// 相手候補1体(openapi `DefenderCandidate`)。**候補ごとに、その候補が撃ち返す技 `moveId` を必ず持つ**(ADR-0704 §1)。
public struct JudgeDefender: Equatable, Sendable {
    public var individual: JudgeIndividual
    public var moveId: String

    public init(individual: JudgeIndividual, moveId: String) {
        self.individual = individual
        self.moveId = moveId
    }
}

/// 素早さの判定にだけ効く場の効果(openapi `SpeedField`。1リクエストに1つで、すべての候補に同じように適用される。ADR-0703 §5)。
public struct JudgeSpeedField: Equatable, Sendable {
    public var trickRoom: Bool
    public var attackerTailwind: Bool
    public var defenderTailwind: Bool

    /// すべて false(場の効果なし)。
    public var isDefault: Bool { !trickRoom && !attackerTailwind && !defenderTailwind }

    public init(trickRoom: Bool = false, attackerTailwind: Bool = false, defenderTailwind: Bool = false) {
        self.trickRoom = trickRoom
        self.attackerTailwind = attackerTailwind
        self.defenderTailwind = defenderTailwind
    }
}

/// `POST /api/judge/v1/outspeed-and-ko` の要求(openapi `OutspeedAndKoRequest`)。
/// `field`(天候・地形・壁)は iOS では送らない(Web の JD5 と同じ。ADR-0705 §6。ADR-0504 §3)。
public struct JudgeRequest: Equatable, Sendable {
    /// iOS の画面は常に `.single`(ユーザー決定でダブルは対象外。ADR-0504 §3)。契約上は必須なので型には持つ。
    public var format: Format
    public var attacker: JudgeIndividual
    /// 自分が使う技(1つ)。すべての候補に同じ技で判定する。
    public var moveId: String
    /// 1〜`RequestLimits.maxJudgeDefenders` 件。応答の `matchups` は同じ順序・同じ件数。
    public var defenders: [JudgeDefender]
    /// nil なら送らない(3つすべて false のとき。送らない要求の挙動は JD1 と同じ。ADR-0702)。非 nil なら3項目すべてを送る。
    public var speedField: JudgeSpeedField?

    public init(
        format: Format = .single, attacker: JudgeIndividual, moveId: String, defenders: [JudgeDefender],
        speedField: JudgeSpeedField? = nil
    ) {
        self.format = format
        self.attacker = attacker
        self.moveId = moveId
        self.defenders = defenders
        self.speedField = speedField
    }
}

/// 確定数 / 乱数 n 発(openapi `KOChance` の judge 版)。engine の生値 `chancePercent` は返らないので持たない。
public struct JudgeKOChance: Equatable, Sendable {
    /// 最大ダメージで倒すのに必要な攻撃回数(0 = 倒せない)。
    public var hits: Int
    /// 最小ダメージでも hits 回で倒せるなら true(確定 n 発)。
    public var guaranteed: Bool
    /// 画面に出す「hits 回で倒せる確率(%)」。小数第1位。
    public var displayChancePercent: Double

    public init(hits: Int, guaranteed: Bool, displayChancePercent: Double) {
        self.hits = hits
        self.guaranteed = guaranteed
        self.displayChancePercent = displayChancePercent
    }
}

/// 相手候補1件ぶんの判定結果(openapi `Matchup`)。値は judge の応答のまま(丸めない。ADR-0700 §6-1)。
public struct JudgeMatchup: Equatable, Sendable {
    /// 要求の `defenders` の位置(0 始まり)。
    public var defenderIndex: Int
    /// 素早さの比較で自分が先に動く側か(トリックルーム反映済み。優先度は見ない)。
    public var outspeeds: Bool
    /// 同速(`outspeeds` と同時に true にならない)。
    public var speedTie: Bool
    public var attackerSpeed: Int
    public var defenderSpeed: Int
    public var attackerMovePriority: Int
    public var defenderMovePriority: Int
    /// 優先度まで含めた最終的な行動順。`turnOrderTie` のときは false。
    public var attackerMovesFirst: Bool
    /// 優先度も素早さも同じで、どちらが先に動くか決まらない。
    public var turnOrderTie: Bool
    /// 自分の技がこの候補に与える確定数(順方向)。
    public var attackerKo: JudgeKOChance
    /// この候補の技が自分に与える確定数(逆方向)。
    public var defenderKo: JudgeKOChance
    /// `attackerKo` に付いた「正確でない可能性がある」印(順方向。印が無ければ空)。`target` の attacker_* は自分・defender_* はこの候補(ADR-0708 §5)。
    public var attackerKoUnsupported: [UnsupportedMark]
    /// `defenderKo` に付いた印(逆方向。印が無ければ空)。**役割が入れ替わる**: `target` の attacker_* はこの候補・defender_* は自分(ADR-0708 §5)。
    public var defenderKoUnsupported: [UnsupportedMark]

    public init(
        defenderIndex: Int, outspeeds: Bool, speedTie: Bool, attackerSpeed: Int, defenderSpeed: Int,
        attackerMovePriority: Int, defenderMovePriority: Int, attackerMovesFirst: Bool, turnOrderTie: Bool,
        attackerKo: JudgeKOChance, defenderKo: JudgeKOChance,
        attackerKoUnsupported: [UnsupportedMark] = [], defenderKoUnsupported: [UnsupportedMark] = []
    ) {
        self.defenderIndex = defenderIndex
        self.outspeeds = outspeeds
        self.speedTie = speedTie
        self.attackerSpeed = attackerSpeed
        self.defenderSpeed = defenderSpeed
        self.attackerMovePriority = attackerMovePriority
        self.defenderMovePriority = defenderMovePriority
        self.attackerMovesFirst = attackerMovesFirst
        self.turnOrderTie = turnOrderTie
        self.attackerKo = attackerKo
        self.defenderKo = defenderKo
        self.attackerKoUnsupported = attackerKoUnsupported
        self.defenderKoUnsupported = defenderKoUnsupported
    }
}

/// `POST /api/judge/v1/outspeed-and-ko` の応答(openapi `OutspeedAndKoResponse`)。`matchups` は要求の `defenders` と同じ順序・同じ件数。
public struct JudgeResponse: Equatable, Sendable {
    public var matchups: [JudgeMatchup]

    public init(matchups: [JudgeMatchup]) {
        self.matchups = matchups
    }
}

/// 画面が依存する判定の境界(`PokeCalcService`・`SpeedService` とは別のプロトコル。絶対ルール5: 判定の失敗が
/// 計算・構築に影響しないよう、混ぜない)。実装は `APIJudgeService` と `MockJudgeService`。
/// 失敗は `PokeCalcError`(`code` はサーバーの語彙をそのまま運ぶ。通信失敗は `PokeCalcError.Code.transport`)。
public protocol JudgeService: Sendable {
    /// 相手候補ごとの「素早さで抜けるか・行動順・双方の確定数」。1回の呼び出しは上流を何度も叩くので遅いことがある
    /// (画面は送信ボタンを押したときだけ呼ぶ。ADR-0705 §7)。
    func outspeedAndKo(_ request: JudgeRequest) async throws -> JudgeResponse
}
