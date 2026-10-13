import Foundation
import XCTest

@testable import PokeCalcCore

/// 連続技(CB-8。ADR-0535)のテスト用の架空データ。名前は「テスト」で始める(ADR-0002)。数値に意味は無い。
enum ComboStub {
    static let attackMove = Move(id: "stub-combo-attack", nameJa: "テストこうげきわざ", type: .fighting, category: .physical, power: 80)
    static let priorityMove = Move(
        id: "stub-combo-priority", nameJa: "テストせんせいわざ", type: .ghost, category: .physical, power: 40, priority: 1)
    static let specialMove = Move(id: "stub-combo-special", nameJa: "テストとくしゅわざ", type: .water, category: .special, power: 90)
    /// 自分の攻撃 +1・素早さ +1(確率 100% の効果)。
    static let boostMove = Move(
        id: "stub-combo-boost", nameJa: "テストまいわざ", type: .dragon, category: .status, power: 0,
        effect: MoveRankEffect(target: .user, stages: RankBlock(atk: 1, spe: 1)))
    /// 相手の素早さ -1(確率 100%)を持つ攻撃技。
    static let slowDownMove = Move(
        id: "stub-combo-slowdown", nameJa: "テストいわわざ", type: .rock, category: .physical, power: 60,
        effect: MoveRankEffect(target: .opponent, stages: RankBlock(spe: -1)))
    /// 自分の防御 -1・特防 -1 を持つ攻撃技(下降のチップ)。
    static let closeMove = Move(
        id: "stub-combo-close", nameJa: "テストかくとうわざ", type: .fighting, category: .physical, power: 120,
        effect: MoveRankEffect(target: .user, stages: RankBlock(def: -1, spd: -1)))
    /// ランクを変えない変化技(手に置けない)。
    static let plainStatusMove = Move(id: "stub-combo-plain-status", nameJa: "テストへんかのみ", type: .normal, category: .status, power: 0)
    /// stages がすべて 0 の効果(ランクを変えない扱い)。
    static let zeroEffectMove = Move(
        id: "stub-combo-zero-effect", nameJa: "テストこうかなし", type: .normal, category: .status, power: 0,
        effect: MoveRankEffect(target: .user, stages: RankBlock()))

    static let allMoves = [attackMove, priorityMove, specialMove, boostMove, slowDownMove, closeMove, plainStatusMove, zeroEffectMove]

    /// 攻撃側(覚える技にランクを変えない変化技と、マスタに無い技を含む)。
    static let attacker = SpeciesDetail(
        key: "9001-000", dexNo: 9001, form: 0, nameJa: "テストコンボいち", types: [.fighting],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-combo-ability", nameJa: "テストとくせいコンボ")],
        learnset: [attackMove.id, plainStatusMove.id, boostMove.id, priorityMove.id, closeMove.id, zeroEffectMove.id, "stub-combo-missing"])
    /// 防御側(相手の技の候補: 攻撃技だけ)。
    static let defender = SpeciesDetail(
        key: "9002-000", dexNo: 9002, form: 0, nameJa: "テストコンボに", types: [.water],
        baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50),
        abilities: [Ability(id: "stub-combo-ability", nameJa: "テストとくせいコンボ")],
        learnset: [specialMove.id, boostMove.id, slowDownMove.id])

    // 性格(ADR-0200 §2 の「(上昇, 下降) が一致する性格の ID 昇順の最初」を確かめるため、同じ組を 2 つ持たせる)。
    static let neutral = Nature(id: "test-nature-a-neutral", nameJa: "テストせいかく無補正")
    static let neutralLater = Nature(id: "test-nature-z-neutral", nameJa: "テストせいかく無補正2")
    static let atkUp = Nature(id: "test-nature-atk-spa", nameJa: "テストせいかくA上C下", plus: .atk, minus: .spa)
    static let spaUp = Nature(id: "test-nature-spa-atk", nameJa: "テストせいかくC上A下", plus: .spa, minus: .atk)
    static let speUpSpaDown = Nature(id: "test-nature-spe-spa", nameJa: "テストせいかくS上C下", plus: .spe, minus: .spa)
    static let speUpAtkDown = Nature(id: "test-nature-spe-atk", nameJa: "テストせいかくS上A下", plus: .spe, minus: .atk)
    static let atkUpSpeDown = Nature(id: "test-nature-atk-spe", nameJa: "テストせいかくA上S下", plus: .atk, minus: .spe)
    static let defUpSpeDown = Nature(id: "test-nature-def-spe", nameJa: "テストせいかくB上S下", plus: .def, minus: .spe)
    static let natures = [neutralLater, neutral, atkUp, spaUp, speUpSpaDown, speUpAtkDown, atkUpSpeDown, defUpSpeDown]

    static func individual(
        speciesKey: String = attacker.key, natureId: String = atkUp.id,
        sp: StatBlock = StatBlock(hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32), ranks: RankBlock = RankBlock()
    ) -> Individual {
        Individual(speciesKey: speciesKey, natureId: natureId, sp: sp, ranks: ranks)
    }

    static func defenderOption(_ key: String, label: String, natureId: String = neutral.id) -> ComboDefenderOption {
        ComboDefenderOption(
            key: key, label: label,
            individual: Individual(
                speciesKey: defender.key, natureId: natureId, sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 0, spe: 0)))
    }

    static let defenderA = defenderOption("none@-", label: "テスト無振り")
    static let defenderB = defenderOption("hb@-", label: "テストHB振り")

    static func context(
        attacker: Individual = individual(), mainMove: Move = attackMove, mainCritical: Bool = false,
        field: FieldState = FieldState(), defenders: [ComboDefenderOption] = [defenderA, defenderB],
        attackerLearnset: [String] = ComboStub.attacker.learnset
    ) -> ComboContext {
        ComboContext(
            attacker: attacker, mainMove: mainMove, mainCritical: mainCritical, field: field, defenders: defenders,
            attackerLearnset: attackerLearnset, defenderSpeciesKey: defender.key, natures: natures)
    }

    static func calcResult(minPercent: Double = 40.2, maxPercent: Double = 47.8, category: MoveCategory = .physical) -> CalcResult {
        CalcResult(
            rolls: Array(repeating: 40, count: 16), minDamage: 40, maxDamage: 48, minPercent: minPercent, maxPercent: maxPercent,
            defenderHP: 100, effectiveness: 1, stab: false, category: category,
            ko: KOChance(hits: 3, guaranteed: true, chancePercent: 100, displayChancePercent: 100))
    }

    static func line(_ selfSpeed: Int) -> ComboSpeedLine {
        ComboSpeedLine(
            selfSpeed: selfSpeed, selfFirst: ComboSpeedRange(min: 0, max: selfSpeed - 1),
            opponentFirst: ComboSpeedRange(min: selfSpeed + 1, max: 999))
    }

    static func step(
        _ move: Move, damage: CalcResult?, selfSpeed: Int = 132, order: ComboTurnOrder = .dependsOnOpponentSpeed,
        line: ComboSpeedLine? = nil
    ) -> ComboStepResult {
        ComboStepResult(
            moveId: move.id, selfStartRanks: RankBlock(), selfEndRanks: RankBlock(), opponentStartRanks: RankBlock(),
            opponentEndRanks: RankBlock(), damage: damage, selfSpeed: selfSpeed, opponentSpeed: 0, selfPriority: move.priority,
            opponentPriority: 0, order: order, line: line ?? ComboStub.line(selfSpeed))
    }

    static func ko(display: Double, guaranteed: Bool = false) -> ComboKOChance {
        ComboKOChance(chancePercent: display, guaranteed: guaranteed, displayChancePercent: display)
    }

    /// 2 手(攻撃 → 変化技)の既定の結果。
    static func result(
        steps: [ComboStepResult]? = nil, ko: ComboKOChance = ko(display: 51.6), minPercent: Double = 91.4, maxPercent: Double = 108.6,
        speed: ComboSpeedSummary = ComboSpeedSummary(firstSelfSpeed: 132, lastSelfSpeed: 198, opponentSpeed: 0, opponentSpeedKnown: false),
        opponentAttack: ComboOpponentAttack? = nil, unsupported: [ComboMark] = []
    ) -> ComboResult {
        let builtSteps = steps ?? [
            step(attackMove, damage: calcResult(), selfSpeed: 132),
            step(boostMove, damage: nil, selfSpeed: 198),
        ]
        return ComboResult(
            steps: builtSteps,
            total: ComboTotal(
                minDamage: 91, maxDamage: 109, minPercent: minPercent, maxPercent: maxPercent, defenderHP: 100, ko: ko,
                koByStep: Array(repeating: ko, count: builtSteps.count)),
            speed: speed, opponentAttack: opponentAttack, unsupported: unsupported)
    }

    static func opponentAttack(fastSurvive: Double, minActions: Int = 1, maxActions: Int = 2) -> ComboOpponentAttack {
        let action = ComboOpponentAction(step: 0, timing: .beforeSelf, damage: calcResult(minPercent: 25.0, maxPercent: 31.7), surviveChancePercent: fastSurvive)
        return ComboOpponentAttack(
            selfHp: 132, minActions: minActions, maxActions: maxActions,
            fast: ComboOpponentScenario(actions: Array(repeating: action, count: maxActions), surviveChancePercent: fastSurvive),
            slow: ComboOpponentScenario(actions: Array(repeating: action, count: minActions), surviveChancePercent: 100))
    }
}

/// `ComboViewModel` のテスト用の `ComboCalcService`。要求をすべて記録し、応答はクロージャで決める。
/// `hold()` 中は応答を保留し、テストが `release(at:)` で任意の順に返す(古い応答が後から届く状況の再現)。
actor StubComboService: ComboCalcService {
    private static let pollInterval: Duration = .milliseconds(5)
    private static let waitPollLimit = 2000

    private(set) var requests: [ComboRequest] = []
    private var responder: @Sendable (ComboRequest, Int) -> Result<ComboResult, PokeCalcError> = { _, _ in
        .success(ComboStub.result())
    }
    private var holding = false
    private var released: Set<Int> = []

    func setResponder(_ responder: @escaping @Sendable (ComboRequest, Int) -> Result<ComboResult, PokeCalcError>) {
        self.responder = responder
    }

    func hold() { holding = true }

    func release(at index: Int) { released.insert(index) }

    func calcCombo(_ request: ComboRequest) async throws -> ComboResult {
        let index = requests.count
        requests.append(request)
        if holding {
            while !released.contains(index) {
                try Task.checkCancellation()
                try await Task.sleep(for: Self.pollInterval)
            }
        }
        return try responder(request, index).get()
    }

    func waitForRequests(count: Int, file: StaticString = #filePath, line: UInt = #line) async throws {
        for _ in 0..<Self.waitPollLimit {
            if requests.count >= count { return }
            try await Task.sleep(for: Self.pollInterval)
        }
        XCTFail("calcCombo が \(count) 回呼ばれない(\(requests.count) 回)", file: file, line: line)
    }
}
