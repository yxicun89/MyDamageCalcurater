// DisplayLabels: レギュレーションに依存しない、ポケモン全体で固定の日本語表示ラベル(P6-2a)。
//
// マスタ(pokedex)には無い表示専用の文言なので、`MockPokeCalcService.presetLabel` と同じ理由で
// コードに1か所持つ(マスタ扱いにしない。coding-rules §2)。`CalcViewModel.moveSummaryText` が
// 技の分類・相性のラベルを組み立てるのに使うため、View 層ではなく Core に置く。

/// 技の分類の日本語ラベル(openapi `MoveCategory` の3値)。
public enum MoveCategoryLabel {
    public static func japaneseName(for category: MoveCategory) -> String {
        switch category {
        case .physical: return "物理"
        case .special: return "特殊"
        case .status: return "変化"
        }
    }
}

/// タイプの日本語名(openapi `PokeType` の18値。docs/design.md のタイプ色パレットと同じ並び)。
public enum PokeTypeLabel {
    public static func japaneseName(for type: PokeType) -> String {
        switch type {
        case .normal: return "ノーマル"
        case .fire: return "ほのお"
        case .water: return "みず"
        case .electric: return "でんき"
        case .grass: return "くさ"
        case .ice: return "こおり"
        case .fighting: return "かくとう"
        case .poison: return "どく"
        case .ground: return "じめん"
        case .flying: return "ひこう"
        case .psychic: return "エスパー"
        case .bug: return "むし"
        case .rock: return "いわ"
        case .ghost: return "ゴースト"
        case .dragon: return "ドラゴン"
        case .dark: return "あく"
        case .steel: return "はがね"
        case .fairy: return "フェアリー"
        }
    }
}

/// ステータスキーの日本語名(openapi `StatKey` の6値。Showdown 規約の並び)。P6-2c 構築編集画面の
/// SP 入力ラベルで使う(他画面には無かったので新設。マスタに無い表示専用文言なのでコードに置く
/// 理由は `MoveCategoryLabel` と同じ)。
public enum StatKeyLabel {
    public static func japaneseName(for stat: StatKey) -> String {
        switch stat {
        case .hp: return "HP"
        case .atk: return "こうげき"
        case .def: return "ぼうぎょ"
        case .spa: return "とくこう"
        case .spd: return "とくぼう"
        case .spe: return "すばやさ"
        }
    }
}

/// タイプ相性(`CalcResult.effectiveness`。0, 0.25, 0.5, 1, 2, 4 のどれか)の日本語ラベル。
public enum EffectivenessLabel {
    /// 「ばつぐん」(等倍より効果が高い)かどうか。design.md「色を持つのはタイプだけ」: 技の要約行は
    /// ばつぐんのときだけ技のタイプ色を使うため、その判定を View から Core へ移してここに集約する。
    public static func isSuperEffective(_ effectiveness: Double) -> Bool {
        effectiveness >= 2
    }

    public static func text(_ effectiveness: Double) -> String {
        switch effectiveness {
        case 0: return "効果なし"
        case 0.25: return "いまひとつ(×0.25)"
        case 0.5: return "いまひとつ(×0.5)"
        case 1: return "等倍"
        case 2: return "ばつぐん(×2)"
        case 4: return "ばつぐん(×4)"
        default:
            // engine の契約上は上の6値のどれかだが、想定外の値でも画面を止めない(coding-rules §3)。
            return "×\(effectiveness)"
        }
    }
}

// MARK: - 計算画面の「詳細」(issue #274。ADR-0501「issue #274」)

/// 計算画面の「詳細」(折りたたみ)の見出し・トグルの日本語ラベル。Web レーンも同じ語を使う
/// (docs/ai-shared/DECISIONS.md 2026-09-25「計算条件の入力 UI」)。
public enum CalcConditionLabels {
    public static let sectionTitle = "詳細"
    public static let critical = "急所"
    public static let burn = "やけど"
    public static let weatherTitle = "天候"
    public static let terrainTitle = "フィールド"
    public static let defenderScreensTitle = "防御側の壁"
    public static let rankTitle = "攻撃側のランク"
    public static let abilityTitle = "攻撃側の特性"
    /// 特性を指定しない(`abilityId` を送らない)選択肢。
    public static let abilityUnspecified = "指定なし"
    /// ランクの −/+ ボタンの VoiceOver 読み上げ(画像だけのボタンなので明示する)。
    public static let rankDecrement = "ランクを下げる"
    public static let rankIncrement = "ランクを上げる"
}

/// 天候の日本語名(openapi `Weather` の5値)。
public enum WeatherLabel {
    public static func japaneseName(for weather: Weather) -> String {
        switch weather {
        case .none: return "なし"
        case .sun: return "はれ"
        case .rain: return "あめ"
        case .sand: return "すなあらし"
        case .snow: return "ゆき"
        }
    }
}

/// フィールドの日本語名(openapi `Terrain` の5値)。
public enum TerrainLabel {
    public static func japaneseName(for terrain: Terrain) -> String {
        switch terrain {
        case .none: return "なし"
        case .electric: return "エレキフィールド"
        case .grassy: return "グラスフィールド"
        case .psychic: return "サイコフィールド"
        case .misty: return "ミストフィールド"
        }
    }
}

/// 壁の日本語名(openapi `Screens` の3プロパティ)。
public enum ScreenKindLabel {
    public static func japaneseName(for kind: ScreenKind) -> String {
        switch kind {
        case .reflect: return "リフレクター"
        case .lightScreen: return "ひかりのかべ"
        case .auroraVeil: return "オーロラベール"
        }
    }
}

/// ランク補正の表示(「A +1」「C -2」「A ±0」)。文字は `AttackerPreset` と同じ対応(atk → A、spa → C)。
public enum RankLabel {
    public static func text(stat: StatKey, value: Int) -> String {
        let letter = AttackerPreset.statLetter(for: stat)
        let signedValue: String
        switch value {
        case 0: signedValue = "±0"
        case ..<0: signedValue = "\(value)"
        default: signedValue = "+\(value)"
        }
        return "\(letter) \(signedValue)"
    }
}
