// Package version は各サービス共通のビルド情報を保持する。
package version

// Version はビルド時に -ldflags で差し込む。既定は開発版。
var Version = "0.0.0-dev"
