// AttackerPreset: 自分側(攻撃側)の調整プリセット(ADR-0500 §6、docs/requirements.md「自分側のプリセット」)。
//
// 画面はこの型を使ってセグメントを並べ、選ばれたプリセットから `Individual.natureId` / `sp` を組み立てる。
// 性格 ID は一覧(マスタ)から規則で選ぶだけで、直書きしない(CLAUDE.md「ハードコードしない」)。

/// 自分側の調整プリセット。`CaseIterable` の順序がそのまま画面のセグメントの並び順になる。
public enum AttackerPreset: String, CaseIterable, Sendable, Hashable {
    /// 無振り: SP 0 + 無補正性格。
    case none
    /// A特化(特殊技なら特攻特化): 関連ステータス SP 32 + 上昇性格。
    case aFull
    /// A振り: 関連ステータス SP 32 + 無補正性格。
    case aMax

    /// `engine/presets/attacker.json` の `presets[].key`(ADR-0114。ADR-0501「P6-12」)。
    /// case 名(= raw value)は `accessibilityIdentifier`(`attackerPreset-aFull` 等)に使っているので変えず、
    /// JSON のキーとの対応はこの1か所にだけ書く(`AttackerPresetCatalogContractTests` が JSON と突き合わせる)。
    public var catalogKey: String {
        switch self {
        case .aFull: return "x_full"
        case .aMax: return "x"
        case .none: return "none"
        }
    }

    /// 既定の選択(`engine/presets/attacker.json` の `default`。ADR-0501「P6-12」3章)。
    /// `CalcViewModel` / `ReverseViewModel` の初期値と `load()` での再設定はこれを参照し、case を直書きしない。
    public static let defaultPreset: AttackerPreset = .none

    /// 画面に出す表示名(docs/requirements.md と同じ言葉)。技の分類によらず固定の文字列を返すため、
    /// 特殊技でも「A特化」のままになる不具合がある(issue #334)。呼び出し側は `label(for:)` に置き換える
    /// (`AttackerPresetTests.testCasesAreOrderedAndLabeledAsRequirements` が旧文言を固定しているため、
    /// この property 自体は残す。ADR-0501「P6-11」参照)。
    public var label: String {
        switch self {
        case .aFull: return "A特化"
        case .aMax: return "A振り"
        case .none: return "無振り"
        }
    }

    /// 画面に出す表示名。技の分類で「A」(物理・変化)/「C」(特殊)の文字を切り替える(issue #334)。
    /// Web(`web/src/i18n/ja.ts` の `attackerPresetText`)と同じ語を使う: A振り/C振り は「(無補正)」を付ける
    /// (requirements.md の「A振り(補正なし)」とは表記が異なるが、クライアント間で語を揃えるため
    /// Web の出荷済み表記に合わせる。ADR-0501「P6-11」の判断)。
    public func label(for moveCategory: MoveCategory) -> String {
        let letter = Self.statLetter(for: Self.relevantStat(for: moveCategory))
        switch self {
        case .aFull: return letter + Self.fullSuffix
        case .aMax: return letter + Self.xSuffix
        case .none: return "無振り"
        }
    }

    /// 関連ステータス(atk/spa)を画面用の1文字に変える(Web の `statLetterJa` と同じ対応)。
    private static func statLetter(for stat: StatKey) -> String {
        switch stat {
        case .atk: return "A"
        case .spa: return "C"
        default: return ""
        }
    }

    /// 特化の接尾辞(Web の `attackerPresetText.fullSuffix` と同じ語)。
    private static let fullSuffix = "特化"
    /// 振り(無補正)の接尾辞(Web の `attackerPresetText.xSuffix` と同じ語)。
    private static let xSuffix = "振り(無補正)"

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
            return AttackerBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: SPLimits.maxPerStat))
        case .aMax:
            let nature = try neutralNature(in: natures)
            return AttackerBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: SPLimits.maxPerStat))
        case .none:
            let nature = try neutralNature(in: natures)
            return AttackerBuild(natureId: nature.id, sp: statBlock(relevant: relevant, value: 0))
        }
    }

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
