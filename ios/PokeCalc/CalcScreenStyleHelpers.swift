import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcScreenStyleHelpers: 計算画面だけで使う見た目のヘルパー(P6-2a)。
//
// タイプ・技の分類・相性の日本語ラベルは `PokeCalcCore`(`DisplayLabels.swift`)にある
// (`CalcViewModel.moveSummaryText` が Core 側で組み立てるため。View はそれをそのまま描く)。

/// 計算画面のあちこちで使う見た目の定数を1か所にまとめる(批評「任意」対応: 0.7 の縮小率・
/// 1pt のヘアライン枠線が複数ファイルに同じ値でばらばらに書かれていた。coding-rules §2
/// 「同じ定義を複数箇所に書かない」)。
enum CalcScreenMetrics {
    /// 1行に収めるためのテキストの最小縮小率。design.md には数値指定が無いため実装側で決める。
    static let compactMinimumScaleFactor: CGFloat = 0.7
    /// カード・チップ・ダメージバーのトラックに使うヘアライン枠線の太さ。
    static let hairlineBorderWidth: CGFloat = 1
    /// 件数上限に達して選べなくなったチップの減光(issue #110。ADR-0501「issue #110」9章)。
    static let disabledChipOpacity: Double = 0.4
    /// 「詳細」の攻撃側のランク表示(「A +1」等)の最小幅。値が変わって桁数が増減しても
    /// ±ボタンの位置がずれないようにする(issue #274。ADR-0501「issue #274」)。
    static let rankValueMinWidth: CGFloat = 48
    /// 攻撃側の「攻撃」「特攻」ブロックの入力・選択肢の最小の高さ(design.md のタップ範囲 36pt 以上。ADR-0518)。
    static let minimumTapSide: CGFloat = 36
}
