// ObservationInput: 逆算画面の観測(テンキー入力)の検証(P6-2b・ADR-0010 §R2)。
//
// 実機の HP バーはゲーム内表示のとおり整数%(与えたダメージ)/ 実点数(受けたダメージ)なので、
// 画面は 0.1% 入力(`DamageObservation.percentTenths`)を出さない(2026-09-21 のユーザー回答。ADR-0501「判断した点」)。
// 純粋関数だけを持ち、`ReverseViewModel` が呼び出す。

/// 観測1件の精度(`side` から決まる。ADR-0010 §R1: 与えたダメージ = 相手 HP の%、受けたダメージ = 自分 HP の実点数)。
public enum ObservationKind: Equatable, Sendable {
    /// 整数%(1〜100)。
    case percent
    /// HP の実点数(1 以上)。
    case damage

    public init(side: ReverseSide) {
        switch side {
        case .defender: self = .percent
        case .attacker: self = .damage
        }
    }
}

/// 観測入力の範囲。値は openapi `Observation`(`percent`: minimum 1 / maximum 100、`damage`: minimum 1)と
/// ADR-0010 §R2 に合わせる。文言(`ObservationFieldError.message(kind:)`)はここから作り、直書きしない。
public enum ObservationLimits {
    public static let percentRange = 1...100
    public static let minimumDamage = 1
}

/// 観測入力欄1つぶんの検証エラー。
public enum ObservationFieldError: Equatable, Sendable, Error {
    /// 空(前後の空白だけを含む場合も)。
    case empty
    /// ASCII の数字だけで構成されていない(小数点・符号・全角数字・数字以外の文字・空白混じりを含む)。
    case notANumber
    /// 数字だけだが範囲外(0・上限超・`Int` に収まらない)。
    case outOfRange

    /// 画面に出す文言。範囲の数値は `ObservationLimits` から組み立てる(文言と定数がずれないように)。
    public func message(kind: ObservationKind) -> String {
        switch self {
        case .empty:
            return "値を入力してください"
        case .notANumber:
            return "数字だけを入力してください"
        case .outOfRange:
            switch kind {
            case .percent:
                return "\(ObservationLimits.percentRange.lowerBound)〜\(ObservationLimits.percentRange.upperBound) の整数%を入力してください"
            case .damage:
                return "\(ObservationLimits.minimumDamage) 以上のダメージを入力してください"
            }
        }
    }
}

/// テンキー入力の文字列を観測値へ変換する(純粋関数)。
public enum ObservationParser {
    /// `text` の前後の空白を落とし、ASCII の数字だけを受ける(全角数字・小数点・符号・数字以外の混入は
    /// `.notANumber`)。範囲は `kind` で決まる(`ObservationLimits`)。
    public static func parse(_ text: String, kind: ObservationKind) -> Result<DamageObservation, ObservationFieldError> {
        let trimmed = text.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return .failure(.empty) }
        guard trimmed.allSatisfy({ $0.isASCII && $0.isNumber }) else { return .failure(.notANumber) }
        // 数字だけでも `Int` の範囲を超えることがある(例: 20桁の数字列)。オーバーフローは範囲外として扱う。
        guard let value = Int(trimmed) else { return .failure(.outOfRange) }
        switch kind {
        case .percent:
            guard ObservationLimits.percentRange.contains(value) else { return .failure(.outOfRange) }
            return .success(.percent(value))
        case .damage:
            guard value >= ObservationLimits.minimumDamage else { return .failure(.outOfRange) }
            return .success(.damage(value))
        }
    }
}
