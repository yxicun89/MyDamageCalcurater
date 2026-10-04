package main

import "testing"

// P8-1(ADR-0808): 画像ディレクトリの環境変数名は運用(k8s の manifest・README)が依存するので固定する。
// 任意設定(未設定なら画像なし = /images/* は 404)。実装前は envImagesDir が無くコンパイルできず失敗する。
func TestImagesDirEnvName(t *testing.T) {
	if envImagesDir != "GATEWAY_IMAGES_DIR" {
		t.Fatalf("envImagesDir = %q, want GATEWAY_IMAGES_DIR", envImagesDir)
	}
}
