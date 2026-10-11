// Package app はユースケースを置く層(ヘキサゴナル構成。apps/wishlist/docs/arch-spec.md)。
// 入力の検査・手順・永続化や取得のポート(インターフェース)をここで宣言する。ポートの実装は adapters が持つ。
// 配下のパッケージ(app/**)は domain と app だけを import する。DB・HTTP・ファイルなど外側のライブラリには触れない。
package app
