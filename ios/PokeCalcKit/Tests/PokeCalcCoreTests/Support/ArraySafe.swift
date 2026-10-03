extension Array {
    /// 範囲外なら nil(実装前の足場で、空の結果に添字でアクセスしてテストプロセスごと落ちないようにする)。
    subscript(safe index: Int) -> Element? {
        indices.contains(index) ? self[index] : nil
    }
}
