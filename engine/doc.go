// Package engine はポケモンのダメージ計算・実数値計算・逆算を行う純粋な Go
// パッケージ。DB・HTTP・ファイル・時刻・乱数などの外部 I/O を持ち込まない
// (CLAUDE.md 絶対ルール2)。calc-svc とブラウザ用 WASM の両方から利用する。
package engine

// Version はエンジンのスキーマ/ロジックのバージョン。計算結果の互換性追跡に使う。
const Version = "0.0.0-dev"
