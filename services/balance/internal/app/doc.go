// Package app は balance のユースケース層の置き場(ADR-0811 S8)。
//
// balance の各エンドポイント(analyze・coverage・threats・recommendations・move-range)は、
// read model から引いた Provider を渡して domain の関数を 1 回呼ぶだけで、複数の手順を束ねる処理が無いため、今は何も置かない(app は薄い)。
// 入力の検証・締め切り・エラー整形は adapters/httpapi、read model の読み込みは adapters/master にある。
package app
