// ComboHandoff: 素早さ画面 →「この相手で連続技を計算」→ 計算タブの連続技(I-ios-21。ADR-0542・ADR-0501「素早さ画面から連続技へ」)。
//
// spec-writer の骨組み(コンパイルを通すだけ。値は仮)。実装は implementer が ComboHandoffTests・AppNavigationComboOpenTests・
// CalcViewModelSpeedHandoffTests・ComboViewModelHandoffTests・ComboHandoffTextTests を通す形で書く。
// Web の対応: `web/src/app/comboOpenRequest.ts`(token)・`ComboSection.tsx` の openToken / openSpeed(I-web-32)。

/// 計算タブへ渡す「連続技を開く」要求。`token` はルート(`AppNavigationModel.openCombo`)が 1 ずつ進める。
/// 計算画面は `token` が変わったときだけ受け取る(タブを移っただけでは入れ直さない)。
public struct ComboOpenRequest: Equatable, Sendable {
    public let token: Int
    public let handoff: SpeedComboHandoff

    public init(token: Int, handoff: SpeedComboHandoff) {
        self.token = token
        self.handoff = handoff
    }
}

/// 素早さ画面の値を、連続技の「自分の素早さ」の入力へ写す規則(純粋)。
public enum ComboHandoffPlan {
    /// 自分の素早さの入力(素早さ画面の SP・性格補正・ランク)。
    public static func selfSpeedInput(for handoff: SpeedComboHandoff) -> ComboSelfSpeedInput {
        guard let speed = handoff.attackerSpeed else {
            return ComboSelfSpeedInput(sp: nil, nature: .keep, rank: handoff.attackerRank)
        }
        return ComboSelfSpeedInput(sp: speed.sp, nature: nature(speed.nature), rank: handoff.attackerRank)
    }

    /// 素早さ画面の性格補正(上昇 / 補正なし / 下降)→ 連続技の性格補正の選択肢。
    public static func nature(_ nature: SpeedNature) -> ComboSelfSpeedNature {
        switch nature {
        case .plus: return .plus
        case .neutral: return .neutral
        case .minus: return .minus
        }
    }

    /// 「こだわりスカーフは反映できない」の 1 行を出すか。
    public static func scarfIgnored(for handoff: SpeedComboHandoff) -> Bool {
        handoff.attackerScarf
    }
}
