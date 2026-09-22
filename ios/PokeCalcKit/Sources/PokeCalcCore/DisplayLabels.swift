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
