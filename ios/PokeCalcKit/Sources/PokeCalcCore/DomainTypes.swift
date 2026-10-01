// PokeCalcCore: ドメインの型(ADR-0500 §3)。
//
// 画面(View)は生成型(PokeCalcAPI)を直接使わず、この型と `PokeCalcService` だけに依存する。
// 生成型 ↔ ドメインの写像は `APIPokeCalcService` の中の1か所に置く(このファイルは写像を持たない)。
// 逆算(Reverse*)は ADR-0010 §R の形。api/openapi.yaml も P3-1(ADR-0200)でこの形になった。

// MARK: - Lv50・個体値31固定(CLAUDE.md ドメイン規約)

/// 計算対象のレベル。ポケモンチャンピオンズは常に Lv50 固定(個体値も 31 固定だが、
/// 個体値は API へ渡さない値なのでここには持たない)。
public let fixedLevel = 50

/// 能力ポイント(SP)の上限。CLAUDE.md ドメイン規約: 1ステータスにつき最大32(合計は66まで)。
/// `AttackerPreset` / `KnownDefenderPreset` / `ReverseCandidateDisplay` が「振り切った」SP の判定に
/// 共通で使う(同じ値を複数箇所に書かない。coding-rules §2)。
public enum SPLimits {
    public static let maxPerStat = 32
    /// 合計の上限(CLAUDE.md ドメイン規約「合計66」。P6-2c `TeamValidator` が使う)。
    public static let maxTotal = 66
}

/// ランク補正の範囲(openapi `RankBlock` の contract: minimum -6 / maximum 6)。
/// `CalcViewModel.setAttackerRank` のクランプと View の ±6 での無効化が同じ値を参照する
/// (coding-rules §2「同じ定義を複数箇所に書かない」。issue #274)。
public enum RankLimits {
    public static let min = -6
    public static let max = 6
}

// MARK: - 契約と同期する enum(DomainTypesTests がテストで固定する)

/// シングル/ダブル(openapi `Format`)。
public enum Format: String, CaseIterable, Sendable, Hashable {
    case single
    case double
}

/// タイプ(openapi `PokeType`)。`Codable` は P6-2c `LocalTeamStore` が `Team` を JSON で
/// 永続化するために要る(自動合成は宣言と同じファイルでしか効かないため、ここで付ける。
/// ADR-0501「P6-2c」2章)。
public enum PokeType: String, CaseIterable, Sendable, Hashable, Codable {
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

/// 逆算で探索する性格クラス(openapi `NatureClass`。ADR-0010 §R1)。下降補正は探索しないので2つだけ。
/// `CaseIterable` の順序がそのまま順位(§R4 の定義順)に使われる。
public enum NatureClass: String, CaseIterable, Sendable, Hashable {
    case neutral, plus
}

// MARK: - 共用の値型

/// ランク補正(-6..+6)。HP は持たない(openapi `RankBlock`)。`Codable` は P6-2c の
/// 永続化のため(PokeType のコメントと同じ理由)。
public struct RankBlock: Equatable, Sendable, Codable {
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

/// 6ステータスの値。種族値・実数値・能力ポイント(SP)に共用(openapi `StatBlock`)。`Codable` は
/// P6-2c の永続化のため(PokeType のコメントと同じ理由)。
public struct StatBlock: Equatable, Sendable, Codable {
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

/// 性格補正の構造値(openapi `NatureModifier`)。`plus` が +10%、`minus` が -10% を受ける能力。
/// 無補正は両方 nil。HP を指すことはない。
public struct NatureModifier: Equatable, Sendable {
    public var plus: StatKey?
    public var minus: StatKey?

    public init(plus: StatKey? = nil, minus: StatKey? = nil) {
        self.plus = plus
        self.minus = minus
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

extension SpeciesSummary {
    /// `SpeciesDetail`(`species(key:)` の応答)から一覧表示に要る値だけを写す(issue #68。
    /// ADR-0501「issue #68」5章: 「一度でも見た種族」の辞書には検索結果だけでなく
    /// `species(key:)` の応答も入れるため、3画面の ViewModel が共通で使う変換)。
    public init(detail: SpeciesDetail) {
        self.init(key: detail.key, dexNo: detail.dexNo, form: detail.form, nameJa: detail.nameJa, types: detail.types)
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
    /// 使った技の分類(openapi `CalcResult.category`。必須)。
    public var category: MoveCategory
    public var ko: KOChance
    /// 「正確でない可能性がある」印(openapi `CalcResult.unsupported`。必須・印なしは空。ADR-0123)。
    /// 数値は印があっても通常の式のまま(サーバーが拒否しない)。表示は `UnsupportedNotice.swift`。
    public var unsupported: [UnsupportedMark]

    /// `unsupported` は既定で空(P6-17 より前に書かれた呼び出し側・テストをそのまま通すため)。
    public init(
        rolls: [Int], minDamage: Int, maxDamage: Int, minPercent: Double, maxPercent: Double,
        defenderHP: Int, effectiveness: Double, stab: Bool, category: MoveCategory, ko: KOChance,
        unsupported: [UnsupportedMark] = []
    ) {
        self.rolls = rolls
        self.minDamage = minDamage
        self.maxDamage = maxDamage
        self.minPercent = minPercent
        self.maxPercent = maxPercent
        self.defenderHP = defenderHP
        self.effectiveness = effectiveness
        self.stab = stab
        self.category = category
        self.ko = ko
        self.unsupported = unsupported
    }
}

// MARK: - 未対応の印(ADR-0123・ADR-0501「P6-17」)

/// 印の対象(openapi `UnsupportedMark.target`)。値の集合は `UnsupportedMarkDomainTests` が契約と照合する。
public enum UnsupportedTarget: String, CaseIterable, Sendable, Hashable {
    case move
    case attackerItem = "attacker_item"
    case attackerAbility = "attacker_ability"
    case defenderItem = "defender_item"
    case defenderAbility = "defender_ability"
}

/// 印の理由(openapi `UnsupportedMark.reason`)。技は機構(ADR-0121 の13種)か `zero_power`、
/// 持ち物・特性は `unsupported_effect`。値の集合は `UnsupportedMarkDomainTests` が契約と照合する。
public enum UnsupportedReason: String, CaseIterable, Sendable, Hashable {
    case altDefenseStat = "alt_defense_stat"
    case altOffenseStat = "alt_offense_stat"
    case alwaysCrit = "always_crit"
    case effectivenessChange = "effectiveness_change"
    case fieldSpecific = "field_specific"
    case fixedDamage = "fixed_damage"
    case ignoreDefenseRanks = "ignore_defense_ranks"
    case moveSpecific = "move_specific"
    case multiHit = "multi_hit"
    case ohko
    case priorityChange = "priority_change"
    case typeChange = "type_change"
    case variablePower = "variable_power"
    case zeroPower = "zero_power"
    case unsupportedEffect = "unsupported_effect"
}

/// 「この結果は正確でない可能性がある」印1つ(openapi `UnsupportedMark`。ADR-0123 §2)。
/// `id` は技・持ち物・特性の ID(`target` で決まる)。
public struct UnsupportedMark: Equatable, Hashable, Sendable {
    public var target: UnsupportedTarget
    public var reason: UnsupportedReason
    public var id: String

    public init(target: UnsupportedTarget, reason: UnsupportedReason, id: String) {
        self.target = target
        self.reason = reason
        self.id = id
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

// MARK: - 場の状態(issue #274。ADR-0501「issue #274」)

/// 天候(openapi `Weather`)。`allCases` の順が画面のピルの並び(左から)。
public enum Weather: String, CaseIterable, Sendable, Hashable {
    case none, sun, rain, sand, snow
}

/// フィールド(openapi `Terrain`)。`allCases` の順が画面のピルの並び(左から。ゲームの並びに合わせ、
/// openapi の enum の順とは違う。値の集合は同じ)。
public enum Terrain: String, CaseIterable, Sendable, Hashable {
    case none, electric, grassy, psychic, misty
}

/// 壁の種類(openapi `Screens` のプロパティ名)。`allCases` の順が画面のトグルの並び。
public enum ScreenKind: String, CaseIterable, Sendable, Hashable {
    case reflect, lightScreen, auroraVeil
}

/// 片側の壁(openapi `Screens`)。既定はすべて false。
public struct Screens: Equatable, Sendable {
    public var reflect: Bool
    public var lightScreen: Bool
    public var auroraVeil: Bool

    public init(reflect: Bool = false, lightScreen: Bool = false, auroraVeil: Bool = false) {
        self.reflect = reflect
        self.lightScreen = lightScreen
        self.auroraVeil = auroraVeil
    }

    /// `kind` の壁が張られているか。
    public func isOn(_ kind: ScreenKind) -> Bool {
        switch kind {
        case .reflect: return reflect
        case .lightScreen: return lightScreen
        case .auroraVeil: return auroraVeil
        }
    }

    /// `kind` の壁を `isOn` にした値を返す。
    public func setting(_ kind: ScreenKind, to isOn: Bool) -> Screens {
        var copy = self
        switch kind {
        case .reflect: copy.reflect = isOn
        case .lightScreen: copy.lightScreen = isOn
        case .auroraVeil: copy.auroraVeil = isOn
        }
        return copy
    }
}

/// 場の状態(openapi `FieldState`)。既定(`FieldState()`)は「何もない場」で、要求では省略と同じ意味。
public struct FieldState: Equatable, Sendable {
    public var weather: Weather
    public var terrain: Terrain
    /// 攻撃側の場の壁(シングルのダメージには効かない。画面からは変えない。ADR-0501「issue #274」)。
    public var attackerScreens: Screens
    public var defenderScreens: Screens

    public init(
        weather: Weather = .none, terrain: Terrain = .none,
        attackerScreens: Screens = Screens(), defenderScreens: Screens = Screens()
    ) {
        self.weather = weather
        self.terrain = terrain
        self.attackerScreens = attackerScreens
        self.defenderScreens = defenderScreens
    }
}

/// 防御側の代表調整すべてに対する一括計算の要求(openapi `BulkCalcRequest`)。
public struct BulkCalcRequest: Sendable {
    public var format: Format
    public var attacker: Individual
    public var defenderSpeciesKey: String
    public var moveId: String
    /// 場(天候・フィールド・壁)。既定は何もない場(issue #274)。
    public var field: FieldState
    public var critical: Bool
    /// 省略(空を含む)は技の分類に応じた既定セット。指定したときはその順に行を返す。
    public var presets: [DefenderPreset]
    /// 差し替えて比較する持ち物 ID(省略時は素の1通り)。`nil` は「持ち物なし」。
    public var itemVariants: [String?]

    public init(
        format: Format, attacker: Individual, defenderSpeciesKey: String, moveId: String,
        field: FieldState = FieldState(),
        critical: Bool = false, presets: [DefenderPreset] = [], itemVariants: [String?] = []
    ) {
        self.format = format
        self.attacker = attacker
        self.defenderSpeciesKey = defenderSpeciesKey
        self.moveId = moveId
        self.field = field
        self.critical = critical
        self.presets = presets
        self.itemVariants = itemVariants
    }
}

/// 一括計算の1行で使った防御側の調整(openapi `BulkDefender`。ADR-0200 §1)。
public struct BulkDefender: Equatable, Sendable {
    /// 能力ポイント。
    public var sp: StatBlock
    public var nature: NatureModifier
    /// `nature` と一致するマスタの性格 ID。該当する性格がマスタに無ければ nil(ADR-0200 §2)。
    public var natureId: String?
    /// 実数値(Lv50・個体値31)。クライアントでは計算しない(サーバーの値をそのまま運ぶ)。
    public var stats: StatBlock

    public init(sp: StatBlock, nature: NatureModifier, natureId: String?, stats: StatBlock) {
        self.sp = sp
        self.nature = nature
        self.natureId = natureId
        self.stats = stats
    }
}

public struct BulkCalcRow: Equatable, Sendable {
    public var preset: DefenderPreset
    /// 表示名(例 HB振り / HB特化 / H振り+B補正)。
    public var presetLabel: String
    /// この行の防御側の持ち物(持ち物なしは nil)。
    public var itemId: String?
    /// この行で使った防御側の調整(openapi `BulkCalcRow.defender`。必須)。
    public var defender: BulkDefender
    public var result: CalcResult

    public init(
        preset: DefenderPreset, presetLabel: String, itemId: String? = nil,
        defender: BulkDefender, result: CalcResult
    ) {
        self.preset = preset
        self.presetLabel = presetLabel
        self.itemId = itemId
        self.defender = defender
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
/// 名前は `DamageObservation`(`Observation` ではなく): SwiftUI の `@Observable` マクロが展開する
/// コードは `Observation.ObservationRegistrar`(Apple の Observation フレームワーク)を参照するため、
/// 同名の型がこのモジュールにあると解決が衝突する(P6-2a・CalcViewModel の `@Observable` 化で判明)。
public enum DamageObservation: Equatable, Sendable {
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

/// 逆算候補(openapi `ReverseCandidate`。ADR-0010 §R3)。1候補 = (性格クラス, 持ち物)。
public struct ReverseCandidate: Equatable, Sendable {
    public var natureClass: NatureClass
    /// 性格クラスの代表の補正(neutral = 両方 nil、plus = 関連ステータス +10%。ADR-0010 §R)。
    public var nature: NatureModifier
    /// `nature` と一致するマスタの性格 ID(`BulkDefender.natureId` と同じ規則)。該当なしは nil。
    public var natureId: String?
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
    /// 「正確でない可能性がある」印(openapi `ReverseCandidate.unsupported`。必須・印なしは空。
    /// SP によらず候補ごとに決まる。ADR-0123 §2)。
    public var unsupported: [UnsupportedMark]

    /// `unsupported` は既定で空(`CalcResult` と同じ理由)。
    public init(
        natureClass: NatureClass, nature: NatureModifier, natureId: String?, itemId: String?,
        ranges: [SPRange], spCount: Int,
        exact: Bool, mismatch: Int, support: Int, minPercent: Double, maxPercent: Double,
        unsupported: [UnsupportedMark] = []
    ) {
        self.natureClass = natureClass
        self.nature = nature
        self.natureId = natureId
        self.itemId = itemId
        self.ranges = ranges
        self.spCount = spCount
        self.exact = exact
        self.mismatch = mismatch
        self.support = support
        self.minPercent = minPercent
        self.maxPercent = maxPercent
        self.unsupported = unsupported
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

/// 逆算の要求(openapi `ReverseRequest`。ADR-0010 §R)。
/// 場(field)は `CalcRequest` と同じ理由(P6 の画面で未使用)で持たない。
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
    public var observations: [DamageObservation]
    /// 急所(openapi `options.critical`)。
    public var critical: Bool
    /// 返す候補数の上限。0 は無制限(openapi の既定値)。
    public var maxCandidates: Int

    public init(
        format: Format, side: ReverseSide, known: Individual, unknownSpeciesKey: String, moveId: String,
        itemCandidates: [String?] = [], observations: [DamageObservation],
        critical: Bool = false, maxCandidates: Int = 0
    ) {
        self.format = format
        self.side = side
        self.known = known
        self.unknownSpeciesKey = unknownSpeciesKey
        self.moveId = moveId
        self.itemCandidates = itemCandidates
        self.observations = observations
        self.critical = critical
        self.maxCandidates = maxCandidates
    }
}
