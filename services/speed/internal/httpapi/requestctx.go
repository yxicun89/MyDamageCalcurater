package httpapi

// X-Device-Id / X-Session-Id の検証(ADR-0606。services/gateway/internal/httpapi/headers.go
// の checkAPIHeaders・headerStatus・isCanonicalUUID・isHexDigit を、gateway 専用の生成型への
// 依存を除いて移植したもの。判定基準は一字一句 gateway と同じにする。gateway のコードは
// 変更しない(コピー元として読むだけ)。

import (
	"net/http"

	"example.com/pokecalc/services/speed/internal/api"
)

// uuidLength は正準形の UUID 文字列("8-4-4-4-12")の長さ。
const uuidLength = 36

// checkAPIHeaders は /api/speed/v1/* に課す X-Device-Id / X-Session-Id の検証。
// 欠落・空は missing_header、UUID でない値・同名ヘッダの重複は invalid_header。
// 欠落と不正が同時にあれば missing_header を優先する(ADR-0606)。
func checkAPIHeaders(h http.Header) *api.Error {
	deviceMissing, deviceInvalid := headerStatus(h, deviceIDHeader)
	sessionMissing, sessionInvalid := headerStatus(h, sessionIDHeader)

	if deviceMissing || sessionMissing {
		return &api.Error{
			Code:    api.MissingHeader,
			Message: "X-Device-Id / X-Session-Id が無い",
		}
	}
	if deviceInvalid || sessionInvalid {
		return &api.Error{
			Code:    api.InvalidHeader,
			Message: "X-Device-Id / X-Session-Id が UUID でない、または重複している",
		}
	}
	return nil
}

// headerStatus は1つのヘッダの状態を返す: missing(欠落・空)、invalid(UUID でない・重複)。
func headerStatus(h http.Header, name string) (missing, invalid bool) {
	values := h.Values(name)
	switch len(values) {
	case 0:
		return true, false
	case 1:
		if values[0] == "" {
			return true, false
		}
		return false, !isCanonicalUUID(values[0])
	default:
		// 同名ヘッダが複数個(値が同じでも別でも重複は invalid_header)。
		return false, true
	}
}

// isCanonicalUUID は正準形 8-4-4-4-12(16進・大文字小文字は問わない)だけを true にする。
// 波括弧・urn:uuid: 接頭辞・ハイフン無しの32桁は false(uuid.Parse は受け付けてしまうため使わない)。
func isCanonicalUUID(s string) bool {
	if len(s) != uuidLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			if !isHexDigit(s[i]) {
				return false
			}
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
