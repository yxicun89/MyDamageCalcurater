// KnownDefenderPreset: 逆算画面「受けたダメージ」(side = attacker)で、自分(既知の防御側)を
// 作るプリセット(P6-2b、ADR-0501「P6-2b の受け入れ条件」2、「判断した点」)。
//
// ADR-0009 の一括計算プリセットのカタログから、H の有無で迷わない3つだけを選ぶ:
// 無振り(`none`)/ HB(HD)振り(`max`。ADR-0009 の `hb`/`hd`)/ HB(HD)特化(`full`。`hb_full`/`hd_full`)。
// `hp` や `*_boost`(H だけ・B(D) 無し)は選択肢を増やすだけなので入れない。
// `AttackerPreset`(与えたダメージの自分)と対になる型で、性格 ID を直書きしない同じ規則に従う。

/// 自分(既知の防御側)の調整プリセット。`CaseIterable` の順序がそのまま画面のセグメントの並び順になる。
public enum KnownDefenderPreset: String, CaseIterable, Sendable, Hashable {
    /// 無振り: SP 0 + 無補正性格。
    case none
    /// HB(HD)振り: H32 + 関連ステータス SP 32 + 無補正性格。
    case max
    /// HB(HD)特化: H32 + 関連ステータス SP 32 + 上昇性格(ADR-0009: `hb_full` = +def/-atk、`hd_full` = +spd/-atk)。
    case full

    /// 画面に出す表示名。相手の技の分類(関連ステータスが def か spd か)で「HB」「HD」が切り替わる。
    public func label(for moveCategory: MoveCategory) -> String {
        switch self {
        case .none:
            return "無振り"
        case .max:
            return Self.relevantStat(for: moveCategory) == .spd ? "HD振り" : "HB振り"
        case .full:
            return Self.relevantStat(for: moveCategory) == .spd ? "HD特化" : "HB特化"
        }
    }

    /// 相手の技の分類から、参照する関連ステータスを決める(物理・変化 = def、特殊 = spd)。
    public static func relevantStat(for moveCategory: MoveCategory) -> StatKey {
        switch moveCategory {
        case .physical, .status: return .def
        case .special: return .spd
        }
    }

    /// `preset` と相手の技の分類から性格 ID と SP を組み立てる。該当する性格が `natures`
    /// (一覧の順序で探す)に無ければ `PokeCalcError(code: .natureUnavailable)`。
    public static func build(_ preset: KnownDefenderPreset, moveCategory: MoveCategory, natures: [Nature]) throws -> DefenderBuild {
        let relevant = relevantStat(for: moveCategory)
        switch preset {
        case .none:
            let nature = try neutralNature(in: natures)
            return DefenderBuild(natureId: nature.id, sp: statBlock(relevant: nil, value: 0))
        case .max:
            let nature = try neutralNature(in: natures)
            return DefenderBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: SPLimits.maxPerStat))
        case .full:
            let nature = try increasingNature(for: relevant, in: natures)
            return DefenderBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: SPLimits.maxPerStat))
        }
    }

    /// `plus` が `relevant`、`minus` が atk の性格を一覧の最初から探す(ADR-0009: HB特化 = +def/-atk、HD特化 = +spd/-atk)。
    private static func increasingNature(for relevant: StatKey, in natures: [Nature]) throws -> Nature {
        guard let nature = natures.first(where: { $0.plus == relevant && $0.minus == .atk }) else {
            throw PokeCalcError(
                code: PokeCalcError.Code.natureUnavailable,
                message: "上昇性格が見つからない(\(relevant) 上昇 / atk 下降)"
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

    /// `relevant`(def か spd。nil は無振り)があれば H とその関連ステータスに `value` を入れ、他は 0 の SP。
    private static func statBlock(relevant: StatKey?, value: Int) -> StatBlock {
        var block = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        guard let relevant else { return block }
        block.hp = SPLimits.maxPerStat
        switch relevant {
        case .def: block.def = value
        case .spd: block.spd = value
        // relevantStat(for:) は def か spd しか返さない。
        default: break
        }
        return block
    }
}

/// `KnownDefenderPreset.build` / `AttackerPreset.build` の結果(性格 ID と SP)。
/// `AttackerBuild` と同じ形だが、既知側が攻撃側か防御側かで型を分けて意味の取り違えを防ぐ。
public struct DefenderBuild: Equatable, Sendable {
    public let natureId: String
    public let sp: StatBlock

    public init(natureId: String, sp: StatBlock) {
        self.natureId = natureId
        self.sp = sp
    }
}
