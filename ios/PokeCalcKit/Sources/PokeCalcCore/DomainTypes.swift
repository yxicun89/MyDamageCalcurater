// PokeCalcCore: ドメインの型(ADR-0017 §3)。
//
// 画面(View)は生成型(PokeCalcAPI)を直接使わず、この型と `PokeCalcService` だけに依存する。
// 生成型 ↔ ドメインの写像は `APIPokeCalcService` の中の1か所に置く(このファイルは写像を持たない)。
// 逆算(Reverse*)は ADR-0010 §R の形(いまの api/openapi.yaml の `ReverseCandidate` とは別の形。
// 契約は P3-1 で追従する)。

// MARK: - Lv50・個体値31固定(CLAUDE.md ドメイン規約)

/// 計算対象のレベル。ポケモンチャンピオンズは常に Lv50 固定(個体値も 31 固定だが、
/// 個体値は API へ渡さない値なのでここには持たない)。
public let fixedLevel = 50

// MARK: - 契約と同期する enum(DomainTypesTests がテストで固定する)

/// シングル/ダブル(openapi `Format`)。
public enum Format: String, CaseIterable, Sendable, Hashable {
    case single
    case double
}

/// タイプ(openapi `PokeType`)。
public enum PokeType: String, CaseIterable, Sendable, Hashable {
    case normal, fire, water, electric, grass, ice, fighting, poison, ground
    case flying, psychic, bug, rock, ghost, dragon, dark, steel, fairy
}

/// ステータスキー(openapi `StatKey`。Showdown 規約)。
public enum StatKey: String, CaseIterable, Sendable, Hashable {
    case hp, atk, def, spa, spd, spe
}

/// 技の分類(openapi `MoveCategory`)。
public enum MoveCategory: String, CaseIterable, Sendable, Hashable {
    case physical, special, status
}

/// 状態異常(openapi `StatusCondition`)。
public enum StatusCondition: String, CaseIterable, Sendable, Hashable {
    case none, burn, paralysis, poison
    case badlyPoison = "badly_poison"
    case sleep, freeze
}

/// 防御側の代表調整(openapi `DefenderPreset`。ADR-0009)。
public enum DefenderPreset: String, CaseIterable, Sendable, Hashable {
    case none
    case hp
    case hbBoost = "hb_boost"
    case hb
    case hbFull = "hb_full"
    case hdBoost = "hd_boost"
    case hd
    case hdFull = "hd_full"
}

/// 逆算でどちら側の調整を推定するか(openapi `ReverseSide` と同じ値)。
public enum ReverseSide: String, CaseIterable, Sendable, Hashable {
    case defender, attacker
}

/// 逆算で探索する性格クラス(ADR-0010 §R1)。下降補正は探索しないので2つだけ。
/// `CaseIterable` の順序がそのまま順位(§R4 の定義順)に使われる。
public enum NatureClass: String, CaseIterable, Sendable, Hashable {
    case neutral, plus
}

// MARK: - 共用の値型

/// ランク補正(-6..+6)。HP は持たない(openapi `RankBlock`)。
public struct RankBlock: Equatable, Sendable {
    public var atk: Int
    public var def: Int
    public var spa: Int
    public var spd: Int
    public var spe: Int

    public init(atk: Int = 0, def: Int = 0, spa: Int = 0, spd: Int = 0, spe: Int = 0) {
        self.atk = atk
        self.def = def
        self.spa = spa
        self.spd = spd
        self.spe = spe
    }
}

/// 6ステータスの値。種族値・実数値・能力ポイント(SP)に共用(openapi `StatBlock`)。
public struct StatBlock: Equatable, Sendable {
    public var hp: Int
    public var atk: Int
    public var def: Int
    public var spa: Int
    public var spd: Int
    public var spe: Int

    public init(hp: Int, atk: Int, def: Int, spa: Int, spd: Int, spe: Int) {
        self.hp = hp
        self.atk = atk
        self.def = def
        self.spa = spa
        self.spd = spd
        self.spe = spe
    }
}

/// 計算に使う個体。SP と性格 ID から実数値を導出する(openapi `Individual`)。
public struct Individual: Equatable, Sendable {
    public var speciesKey: String
    public var level: Int
    public var natureId: String
    public var abilityId: String?
    public var itemId: String?
    public var moveId: String?
    /// 能力ポイント。各 0..32、合計 <= 66。
    public var sp: StatBlock
    public var ranks: RankBlock
    public var teraType: PokeType?
    public var status: StatusCondition

    public init(
        speciesKey: String,
        natureId: String,
        sp: StatBlock,
        level: Int = fixedLevel,
        abilityId: String? = nil,
        itemId: String? = nil,
        moveId: String? = nil,
        ranks: RankBlock = RankBlock(),
        teraType: PokeType? = nil,
        status: StatusCondition = .none
    ) {
        self.speciesKey = speciesKey
        self.level = level
        self.natureId = natureId
        self.abilityId = abilityId
        self.itemId = itemId
        self.moveId = moveId
        self.sp = sp
        self.ranks = ranks
        self.teraType = teraType
        self.status = status
    }
}

// MARK: - マスタ参照(pokedex)

public struct SpeciesSummary: Equatable, Sendable {
    public var key: String
    public var dexNo: Int
    public var form: Int
    public var nameJa: String
    public var types: [PokeType]

    public init(key: String, dexNo: Int, form: Int, nameJa: String, types: [PokeType]) {
        self.key = key
        self.dexNo = dexNo
        self.form = form
        self.nameJa = nameJa
        self.types = types
    }
}

public struct Ability: Equatable, Sendable {
    public var id: String
    public var nameJa: String

    public init(id: String, nameJa: String) {
        self.id = id
        self.nameJa = nameJa
    }
}

public struct SpeciesDetail: Equatable, Sendable {
    public var key: String
    public var dexNo: Int
    public var form: Int
    public var nameJa: String
    public var types: [PokeType]
    public var baseStats: StatBlock
    public var abilities: [Ability]
    /// 覚える技の ID 一覧。
    public var learnset: [String]

    public init(
        key: String, dexNo: Int, form: Int, nameJa: String, types: [PokeType],
        baseStats: StatBlock, abilities: [Ability], learnset: [String]
    ) {
        self.key = key
        self.dexNo = dexNo
        self.form = form
        self.nameJa = nameJa
        self.types = types
        self.baseStats = baseStats
        self.abilities = abilities
        self.learnset = learnset
    }
}

public struct Move: Equatable, Sendable {
    public var id: String
    public var nameJa: String
    public var type: PokeType
    public var category: MoveCategory
    /// 威力(0 は変化技/固定ダメージ)。
    public var power: Int
    public var priority: Int

    public init(id: String, nameJa: String, type: PokeType, category: MoveCategory, power: Int, priority: Int = 0) {
        self.id = id
        self.nameJa = nameJa
        self.type = type
        self.category = category
        self.power = power
        self.priority = priority
    }
}

public struct Item: Equatable, Sendable {
    public var id: String
    public var nameJa: String

    public init(id: String, nameJa: String) {
        self.id = id
        self.nameJa = nameJa
    }
}

public struct Nature: Equatable, Sendable {
    public var id: String
    public var nameJa: String
    /// 上昇補正(+10%)を受ける能力。無補正性格は nil。
    public var plus: StatKey?
    /// 下降補正(-10%)を受ける能力。無補正性格は nil。
    public var minus: StatKey?

    public init(id: String, nameJa: String, plus: StatKey? = nil, minus: StatKey? = nil) {
        self.id = id
        self.nameJa = nameJa
        self.plus = plus
        self.minus = minus
    }
}

// MARK: - 計算(calc)

/// 確定数/乱数n発(openapi `KOChance`)。
public struct KOChance: Equatable, Sendable {
    /// 最大ダメージで倒すのに必要な攻撃回数(0 = 倒せない)。
    public var hits: Int
    /// 最小ダメージでも hits 回で倒せるなら true(確定n発)。
    public var guaranteed: Bool
    /// engine の生値。画面には出さない(displayChancePercent を使う。openapi の説明を参照)。
    public var chancePercent: Double
    /// 画面に出す「hits 回で倒せる確率(%)」。0.1% 刻みで、常に表示できる値。
    public var displayChancePercent: Double

    public init(hits: Int, guaranteed: Bool, chancePercent: Double, displayChancePercent: Double) {
        self.hits = hits
        self.guaranteed = guaranteed
        self.chancePercent = chancePercent
        self.displayChancePercent = displayChancePercent
    }
}

/// ダメージ計算の結果(openapi `CalcResult`)。
public struct CalcResult: Equatable, Sendable {
    /// 16 段階の乱数ダメージ(非減少)。
    public var rolls: [Int]
    public var minDamage: Int
    public var maxDamage: Int
    /// 最小ダメージの表示%(切り捨て。ADR-0010 §3)。
    public var minPercent: Double
    /// 最大ダメージの表示%(四捨五入。ADR-0010 §3)。100 超もそのまま返る。
    public var maxPercent: Double
    public var defenderHP: Int
    /// タイプ相性(0, 0.25, 0.5, 1, 2, 4)。
    public var effectiveness: Double
    /// タイプ一致。
    public var stab: Bool
    public var ko: KOChance

    public init(
        rolls: [Int], minDamage: Int, maxDamage: Int, minPercent: Double, maxPercent: Double,
        defenderHP: Int, effectiveness: Double, stab: Bool, ko: KOChance
    ) {
        self.rolls = rolls
        self.minDamage = minDamage
        self.maxDamage = maxDamage
        self.minPercent = minPercent
        self.maxPercent = maxPercent
        self.defenderHP = defenderHP
        self.effectiveness = effectiveness
        self.stab = stab
        self.ko = ko
    }
}

/// 1 vs 1 のダメージ計算の要求(openapi `CalcRequest`)。
/// 場(天候・フィールド・壁)は P6-1 では未使用のため持たない(coding-rules §3「使われていない
/// 汎用機構を作らない」)。要る画面が出てきたら足す。
public struct CalcRequest: Sendable {
    public var format: Format
    public var attacker: Individual
    public var defender: Individual
    /// 使用する技(attacker.moveId より優先)。
    public var moveId: String
    /// 急所。
    public var critical: Bool

    public init(format: Format, attacker: Individual, defender: Individual, moveId: String, critical: Bool = false) {
        self.format = format
        self.attacker = attacker
        self.defender = defender
        self.moveId = moveId
        self.critical = critical
    }
}

/// 防御側の代表調整すべてに対する一括計算の要求(openapi `BulkCalcRequest`)。
public struct BulkCalcRequest: Sendable {
    public var format: Format
    public var attacker: Individual
    public var defenderSpeciesKey: String
    public var moveId: String
    public var critical: Bool
    /// 省略(空を含む)は技の分類に応じた既定セット。指定したときはその順に行を返す。
    public var presets: [DefenderPreset]
    /// 差し替えて比較する持ち物 ID(省略時は素の1通り)。`nil` は「持ち物なし」。
    public var itemVariants: [String?]

    public init(
        format: Format, attacker: Individual, defenderSpeciesKey: String, moveId: String,
        critical: Bool = false, presets: [DefenderPreset] = [], itemVariants: [String?] = []
    ) {
        self.format = format
        self.attacker = attacker
        self.defenderSpeciesKey = defenderSpeciesKey
        self.moveId = moveId
        self.critical = critical
        self.presets = presets
        self.itemVariants = itemVariants
    }
}

public struct BulkCalcRow: Equatable, Sendable {
    public var preset: DefenderPreset
    /// 表示名(例 HB振り / HB特化 / H振り+B補正)。
    public var presetLabel: String
    public var itemId: String?
    public var result: CalcResult

    public init(preset: DefenderPreset, presetLabel: String, itemId: String? = nil, result: CalcResult) {
        self.preset = preset
        self.presetLabel = presetLabel
        self.itemId = itemId
        self.result = result
    }
}

public struct BulkCalcResult: Equatable, Sendable {
    public var defenderSpeciesKey: String
    public var rows: [BulkCalcRow]

    public init(defenderSpeciesKey: String, rows: [BulkCalcRow]) {
        self.defenderSpeciesKey = defenderSpeciesKey
        self.rows = rows
    }
}

// MARK: - 逆算(ADR-0010 §R)

/// 観測(ADR-0010 §R2)。3つのうちちょうど1つの精度で持つ(値と精度の組み合わせ不整合を
/// 型で起こせなくするため enum にする。engine と違い note は今のところ画面から渡さない)。
public enum Observation: Equatable, Sendable {
    /// 整数%の観測(ゲーム内表示)。
    case percent(Int)
    /// 小数第1位の観測(0.1% 単位の整数)。
    case percentTenths(Int)
    /// HP の実点数。
    case damage(Int)
}

/// SP の範囲(両端を含む)。
public struct SPRange: Equatable, Sendable {
    public var min: Int
    public var max: Int

    public init(min: Int, max: Int) {
        self.min = min
        self.max = max
    }
}

/// 逆算候補(ADR-0010 §R3)。1候補 = (性格クラス, 持ち物)。
public struct ReverseCandidate: Equatable, Sendable {
    public var natureClass: NatureClass
    public var itemId: String?
    /// 昇順・互いに素・隣接しない(極大連続区間)。空にならない。
    public var ranges: [SPRange]
    /// `ranges` に含まれる SP の数。
    public var spCount: Int
    /// `mismatch == 0`。
    public var exact: Bool
    public var mismatch: Int
    public var support: Int
    /// `ranges` 全体での想定ダメージ幅(表示%)。
    public var minPercent: Double
    public var maxPercent: Double

    public init(
        natureClass: NatureClass, itemId: String?, ranges: [SPRange], spCount: Int,
        exact: Bool, mismatch: Int, support: Int, minPercent: Double, maxPercent: Double
    ) {
        self.natureClass = natureClass
        self.itemId = itemId
        self.ranges = ranges
        self.spCount = spCount
        self.exact = exact
        self.mismatch = mismatch
        self.support = support
        self.minPercent = minPercent
        self.maxPercent = maxPercent
    }
}

/// 逆算の結果(ADR-0010 §R3)。
public struct ReverseResult: Equatable, Sendable {
    public var side: ReverseSide
    /// 逆算する関連ステータス(側と技の分類で決まる)。
    public var stat: StatKey
    /// 仮定した既知でない側の H の SP(defender = 32、attacker = 0。ADR-0010 §R1)。
    public var assumedHPSP: Int
    /// `mismatch` 昇順(ADR-0010 §R4)。
    public var candidates: [ReverseCandidate]
    public var exactCount: Int

    public init(side: ReverseSide, stat: StatKey, assumedHPSP: Int, candidates: [ReverseCandidate], exactCount: Int) {
        self.side = side
        self.stat = stat
        self.assumedHPSP = assumedHPSP
        self.candidates = candidates
        self.exactCount = exactCount
    }
}

/// 逆算の要求。ADR-0017 §3: いまの openapi `ReverseRequest` は P3-1 で置き換わる決定済みの形
/// (`ReverseCandidate.matchScore` 等)なので、ドメインは先に ADR-0010 §R の形にしておく。
public struct ReverseRequest: Sendable {
    public var format: Format
    public var side: ReverseSide
    /// 既知側の個体。
    public var known: Individual
    /// 未知側の種族(SP・性格・持ち物は探索対象なので渡さない)。
    public var unknownSpeciesKey: String
    public var moveId: String
    /// 持ち物候補(`nil` は「持ち物なし」)。空は「持ち物なし」の1通りと同じ。
    public var itemCandidates: [String?]
    public var observations: [Observation]

    public init(
        format: Format, side: ReverseSide, known: Individual, unknownSpeciesKey: String, moveId: String,
        itemCandidates: [String?] = [], observations: [Observation]
    ) {
        self.format = format
        self.side = side
        self.known = known
        self.unknownSpeciesKey = unknownSpeciesKey
        self.moveId = moveId
        self.itemCandidates = itemCandidates
        self.observations = observations
    }
}
