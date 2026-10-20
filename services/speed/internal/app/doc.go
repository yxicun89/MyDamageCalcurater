// Package app は speed サービスのユースケース層の置き場(ADR-0811)。
//
// speed のハンドラは、入力を検証して domain の関数(表・位置・最低ライン・届く範囲)を 1 回呼ぶだけで、
// 複数の手順を束ねる処理や外部のポートが無いため、今は何も置かない(balance と同じ。architecture-adoption §8 Q7)。
// HTTP の入口は adapters/httpapi、read model の読み込みは adapters/master にある。
package app
