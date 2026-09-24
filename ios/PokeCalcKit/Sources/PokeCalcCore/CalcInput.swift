import Foundation

// CalcInput: 入力変更時の古い計算要求の抑止・キャンセルで共有する契約値(issue #113。
// ADR-0501「issue #113 の受け入れ条件(iOS 側)」4章)。`MasterSearch.debounceInterval`(検索の250ms)
// とは別の定数にする(用途も値も違うため。同ADR 4章「判断」)。

/// 文字入力を計算へ渡すまでの trailing debounce の契約値。
public enum CalcInput {
    /// 観測欄の文字入力から `reverse` を呼ぶまでの trailing debounce(issue #113 の契約値)。
    public static let debounceInterval: Duration = .milliseconds(200)
}
