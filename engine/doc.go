// Package engine はポケモンのダメージ計算・実数値計算・逆算を行う純粋な Go
// パッケージ。DB・HTTP・ファイル・時刻・乱数などの外部 I/O を持ち込まない
// (CLAUDE.md 絶対ルール2)。calc-svc とブラウザ用 WASM の両方から利用する。
package engine

// EngineVersion は計算エンジンのスキーマ/ロジックの版。計算結果の互換性追跡に使う。
// ビルド版(services/internal/version.Version、-ldflags で差し込む)とは別物。
const EngineVersion = "0.0.0-dev"
