// AttackerPreset: 自分側(攻撃側)の調整プリセット(ADR-0500 §6、docs/requirements.md「自分側のプリセット」)。
//
// 画面はこの型を使ってセグメントを並べ、選ばれたプリセットから `Individual.natureId` / `sp` を組み立てる。
// 性格 ID は一覧(マスタ)から規則で選ぶだけで、直書きしない(CLAUDE.md「ハードコードしない」)。

/// 自分側の調整プリセット。`CaseIterable` の順序がそのまま画面のセグメントの並び順になる。
public enum AttackerPreset: String, CaseIterable, Sendable, Hashable {
    /// A特化(特殊技なら特攻特化): 関連ステータス SP 32 + 上昇性格。
    case aFull
    /// A振り: 関連ステータス SP 32 + 無補正性格。
    case aMax
    /// 無振り: SP 0 + 無補正性格。
    case none

    /// 画面に出す表示名(docs/requirements.md と同じ言葉)。
    public var label: String {
        switch self {
        case .aFull: return "A特化"
        case .aMax: return "A振り"
        case .none: return "無振り"
        }
    }

    /// 技の分類から、振る/参照する関連ステータスを決める(ADR-0010 §2 と同じ規則。
    /// 変化技はダメージを出さないので物理と同じ atk 扱いにし、`MockPokeCalcService.reverseStat` と揃える)。
    public static func relevantStat(for moveCategory: MoveCategory) -> StatKey {
        switch moveCategory {
        case .physical, .status: return .atk
        case .special: return .spa
        }
    }

    /// `preset` と技の分類から性格 ID と SP を組み立てる(ADR-0500 §6)。
    /// 該当する性格が `natures`(一覧の順序で探す)に無ければ `PokeCalcError(code: .natureUnavailable)`。
    public static func build(_ preset: AttackerPreset, moveCategory: MoveCategory, natures: [Nature]) throws -> AttackerBuild {
        let relevant = relevantStat(for: moveCategory)
        switch preset {
        case .aFull:
            let nature = try increasingNature(for: relevant, in: natures)
            return AttackerBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: maxStatSP))
        case .aMax:
            let nature = try neutralNature(in: natures)
            return AttackerBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: maxStatSP))
        case .none:
            let nature = try neutralNature(in: natures)
            return AttackerBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: 0))
        }
    }

    /// CLAUDE.md ドメイン規約: SP は 1 ステータス最大 32(A特化・A振りは関連ステータスに振り切る)。
    private static let maxStatSP = 32

    /// `plus` が `relevant`、`minus` がもう一方(atk/spa の逆側)の性格を一覧の最初から探す。
    private static func increasingNature(for relevant: StatKey, in natures: [Nature]) throws -> Nature {
        let counterpart: StatKey = relevant == .atk ? .spa : .atk
        guard let nature = natures.first(where: { $0.plus == relevant && $0.minus == counterpart }) else {
            throw PokeCalcError(
                code: PokeCalcError.Code.natureUnavailable,
                message: "上昇性格が見つからない(\(relevant) 上昇 / \(counterpart) 下降)"
            )
        }
        return nature
    }

    /// 無補正(`plus == nil`)の性格を一覧の最初から探す。
    private static func neutralNature(in natures: [Nature]) throws -> Nature {
        guard let nature = natures.first(where: { $0.plus == nil }) else {
            throw PokeCalcError(code: PokeCalcError.Code.natureUnavailable, message: "無補正の性格が見つからない")
        }
        return nature
    }

    /// `relevant`(atk か spa)だけに `value` を入れ、他は 0 の SP。
    private static func statBlock(relevant: StatKey, value: Int) -> StatBlock {
        var block = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        switch relevant {
        case .atk: block.atk = value
        case .spa: block.spa = value
        // relevantStat(for:) は atk か spa しか返さない。
        default: break
        }
        return block
    }
}

/// `AttackerPreset.build` の結果(性格 ID と SP)。
public struct AttackerBuild: Equatable, Sendable {
    public let natureId: String
    public let sp: StatBlock

    public init(natureId: String, sp: StatBlock) {
        self.natureId = natureId
        self.sp = sp
    }
}
