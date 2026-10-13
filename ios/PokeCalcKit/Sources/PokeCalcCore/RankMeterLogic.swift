// 能力ランクのメーター(13 段)の純粋ロジック。Web の `RankMeter`(ADR-0357)と段数・符号・読み上げをそろえる。
// 見た目と操作は `ios/PokeCalc/RankMeterView.swift`。ここは UI に依存しない値の計算と文言だけを持つ(ADR-0533)。

import Foundation

public enum RankMeterLogic {
    /// 段の数(左 6 = 下降・中央 = 0・右 6 = 上昇)。`RankLimits` から導く(直書きしない)。
    public static let segmentCount = RankLimits.max - RankLimits.min + 1
    /// 左から右へ並べた各段のランク値(-6...6)。
    public static let values: [Int] = Array(RankLimits.min...RankLimits.max)
    /// 中央(ランク 0)の段の添字。
    public static let zeroIndex = -RankLimits.min

    /// 段の見た目の区分(Web の `data-state`)。
    public enum SegmentState: Equatable, Sendable {
        case up, down, zero, off
    }

    /// 段の塗りの種類(見た目)。値の入った段は濃い塗り、入っていない段は向きごとの薄い色、中央は太い目印。
    public enum SegmentFill: Equatable, Sendable {
        case solidUp, solidDown, tintUp, tintDown, zeroMarker
    }

    /// 値の入っていない段の薄い塗り・枠の不透明度(Web の `color-mix` 16% / 70% と同じ)。
    public static let tintFillOpacity = 0.16
    public static let tintStrokeOpacity = 0.70

    public static func clamp(_ value: Int) -> Int {
        min(RankLimits.max, max(RankLimits.min, value))
    }

    public static func index(forValue value: Int) -> Int { clamp(value) - RankLimits.min }

    public static func value(forIndex index: Int) -> Int { clamp(index + RankLimits.min) }

    /// `value` を `delta` 動かし、両端で止める。
    public static func stepped(_ value: Int, by delta: Int) -> Int { clamp(value + delta) }

    public static func canIncrement(_ value: Int) -> Bool { value < RankLimits.max }
    public static func canDecrement(_ value: Int) -> Bool { value > RankLimits.min }

    /// トラック上の横位置 `x`(0...`width`)から、その位置の段のランク値。範囲外は両端に丸める。幅が 0 以下なら nil。
    public static func value(atX x: Double, width: Double) -> Int? {
        guard width > 0 else { return nil }
        let raw = Int((x / width * Double(segmentCount)).rounded(.down))
        return value(forIndex: raw)
    }

    /// 符号付きの表記(`+2` / `-3` / `±0`)。Web の `signedRank` と同じ。
    public static func signedText(_ value: Int) -> String {
        let value = clamp(value)
        if value > 0 { return "+\(value)" }
        if value < 0 { return "\(value)" }
        return "±0"
    }

    /// 段 `step` の区分。上昇中は 0 より右の `step <= value` だけ、下降中は 0 より左の `step >= value` だけ塗る。
    public static func segmentState(step: Int, value: Int) -> SegmentState {
        if step == 0 { return .zero }
        if value > 0, step > 0, step <= value { return .up }
        if value < 0, step < 0, step >= value { return .down }
        return .off
    }

    /// 段 `step` の塗りの種類。値で塗られた段は濃い、そうでない段は 0 より左が下降の薄色・右が上昇の薄色。
    public static func segmentFill(step: Int, value: Int) -> SegmentFill {
        switch segmentState(step: step, value: value) {
        case .zero: return .zeroMarker
        case .up: return .solidUp
        case .down: return .solidDown
        case .off: return step < 0 ? .tintDown : .tintUp
        }
    }

    /// 読み上げ用のステータス名(Web の `rankStatNameJa`。攻撃・防御・特攻・特防・素早さ)。HP にランクは無い。
    public static func statName(for stat: StatKey) -> String {
        switch stat {
        case .hp: return "HP"
        case .atk: return "攻撃"
        case .def: return "防御"
        case .spa: return "特攻"
        case .spd: return "特防"
        case .spe: return "素早さ"
        }
    }

    /// 読み上げ全文(「攻撃 +2」「攻撃 ±0」)。Web の `aria-valuetext` と同じ。
    public static func spokenText(statName: String, value: Int) -> String {
        "\(statName) \(signedText(value))"
    }

    /// − / + ボタンの名前(「<欄の名前>を上げる / 下げる」)。
    public static func increaseLabel(fieldName: String) -> String { "\(fieldName)を上げる" }
    public static func decreaseLabel(fieldName: String) -> String { "\(fieldName)を下げる" }

    // 識別子は欄(`field`)ごとに安定させる。UITest が欄を指す。
    public static func identifier(field: String) -> String { "rankMeter-\(field)" }
    public static func minusIdentifier(field: String) -> String { "rankMeterMinus-\(field)" }
    public static func plusIdentifier(field: String) -> String { "rankMeterPlus-\(field)" }

    /// ドラッグの移動量が水平優勢か(縦と同じ大きさは縦扱い。縦スクロールを優先する)。
    public static func isHorizontalDrag(dx: Double, dy: Double) -> Bool { abs(dx) > abs(dy) }
}
