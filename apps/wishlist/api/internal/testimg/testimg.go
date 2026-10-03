// Package testimg はテスト用の小さな画像のバイト列を作る(実在の画像を Git に置かない)。
package testimg

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
)

func img() image.Image {
	m := image.NewRGBA(image.Rect(0, 0, 2, 2))
	m.Set(0, 0, color.RGBA{R: 255, A: 255})
	return m
}

// PNG は 2x2 の PNG。
func PNG() []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img()); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// JPEG は 2x2 の JPEG。
func JPEG() []byte {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img(), nil); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// GIF は 2x2 の GIF。
func GIF() []byte {
	var b bytes.Buffer
	if err := gif.Encode(&b, img(), nil); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// WebP は内容判定(http.DetectContentType)で image/webp になる最小のヘッダー付きバイト列。
func WebP() []byte {
	b := []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
	return append(b, make([]byte, 32)...)
}

// SVG は受け付けてはいけない画像(スクリプトを含みうる)。
func SVG() []byte {
	return []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
}

// PaddedPNG は PNG の後ろを 0 で埋めて合計 size バイトにする(サイズ上限の検査用)。
func PaddedPNG(size int) []byte {
	b := PNG()
	if size < len(b) {
		panic("testimg: size too small")
	}
	return append(b, make([]byte, size-len(b))...)
}
