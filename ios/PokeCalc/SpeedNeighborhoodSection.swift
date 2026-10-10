import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedNeighborhoodSection: 素早さ画面の「自分の周り」パネル(G-04。ADR-0527)。
// 【spec-writer の足場】描画は未実装(何も出さない)。SpeedScreenView への差し込みも実装者がやる。
//
// 実装者への契約(識別子・見た目の要件):
//  - 置き場所: 自分の結果カードの近く。表(`SpeedTableSection` の LazyVStack)のスクロール領域の外。常時表示・折りたたみなし。
//  - 識別子: 全体 `speedNeighborhood`(`.contain`)/ 先に動く側 `speedNeighborhoodBefore` / 後に動く側 `speedNeighborhoodAfter` /
//    自分(同速または境界)`speedNeighborhoodSelf` / 合計行 `speedNeighborhoodBeforeTotal`・`speedNeighborhoodAfterTotal` /
//    自分未決定の案内 `speedNeighborhoodEmpty`。
//  - 色だけに頼らない: 「自分」バッジ・↑ ↓ の文字。固定高にしない(Dynamic Type・AX5 で折り返す)。

struct SpeedNeighborhoodSection: View {
    let viewModel: SpeedViewModel

    var body: some View {
        EmptyView()
    }
}
