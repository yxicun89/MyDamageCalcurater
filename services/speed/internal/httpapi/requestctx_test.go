package httpapi

// X-Device-Id / X-Session-Id の検証(ADR-0606。gateway の headers.go の複製)の単体テスト。

import (
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/api"
)

func TestIsCanonicalUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"小文字", testDeviceID, true},
		{"大文字", "ABCDEF01-2345-4678-9ABC-DEF012345678", true},
		{"大文字小文字の混在", "abcDEF01-2345-4678-9abc-DEF012345678", true},
		{"nil UUID", "00000000-0000-0000-0000-000000000000", true},
		{"版7", "01890a5d-ac96-774b-bcce-b302099a8057", true},

		{"空", "", false},
		{"空白だけ", "  ", false},
		{"UUID でない文字列", "not-a-uuid", false},
		{"1桁足りない", testDeviceID[:35], false},
		{"1桁多い", testDeviceID + "1", false},
		{"16進でない文字", "1111111g-1111-1111-1111-111111111111", false},
		{"ハイフンの位置が違う", "111111111-111-1111-1111-111111111111", false},
		{"ハイフン無し32桁", "11111111111111111111111111111111", false},
		{"波括弧つき", "{" + testDeviceID + "}", false},
		{"urn:uuid: つき", "urn:uuid:" + testDeviceID, false},
		{"前後に空白", " " + testDeviceID[:34] + " ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isCanonicalUUID(tt.in); got != tt.want {
				t.Errorf("isCanonicalUUID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestHeaderStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		values      []string
		wantMissing bool
		wantInvalid bool
	}{
		{"無い", nil, true, false},
		{"空", []string{""}, true, false},
		{"正準形 UUID", []string{testDeviceID}, false, false},
		{"UUID でない", []string{"test-device"}, false, true},
		{"空白だけ(空ではない・UUID でない)", []string{"  "}, false, true},
		{"重複(同じ値)", []string{testDeviceID, testDeviceID}, false, true},
		{"重複(別の値)", []string{testDeviceID, testSessionID}, false, true},
		{"重複(空を含む)", []string{"", testDeviceID}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := http.Header{}
			for _, v := range tt.values {
				h.Add("X-Device-Id", v)
			}
			missing, invalid := headerStatus(h, "X-Device-Id")
			if missing != tt.wantMissing || invalid != tt.wantInvalid {
				t.Errorf("headerStatus = (missing %v, invalid %v), want (%v, %v)", missing, invalid, tt.wantMissing, tt.wantInvalid)
			}
		})
	}
}

func TestCheckAPIHeaders(t *testing.T) {
	t.Parallel()

	valid := func() http.Header {
		h := http.Header{}
		h.Set("X-Device-Id", testDeviceID)
		h.Set("X-Session-Id", testSessionID)
		return h
	}
	with := func(mutate func(http.Header)) http.Header {
		h := valid()
		mutate(h)
		return h
	}

	t.Run("両方が正準形 UUID なら nil", func(t *testing.T) {
		t.Parallel()
		if got := checkAPIHeaders(valid()); got != nil {
			t.Errorf("checkAPIHeaders = %+v, want nil", *got)
		}
	})

	tests := []struct {
		name   string
		header http.Header
		want   api.ErrorCode
	}{
		{"X-Device-Id なし", with(func(h http.Header) { h.Del("X-Device-Id") }), api.MissingHeader},
		{"X-Session-Id が空", with(func(h http.Header) { h.Set("X-Session-Id", "") }), api.MissingHeader},
		{"欠落と不正が同時なら missing_header", with(func(h http.Header) {
			h.Del("X-Device-Id")
			h.Set("X-Session-Id", "not-a-uuid")
		}), api.MissingHeader},
		{"不正と欠落が同時(逆の組み合わせ)でも missing_header", with(func(h http.Header) {
			h.Set("X-Device-Id", "not-a-uuid")
			h.Del("X-Session-Id")
		}), api.MissingHeader},
		{"重複と欠落が同時なら missing_header", with(func(h http.Header) {
			h.Add("X-Device-Id", testDeviceID)
			h.Del("X-Session-Id")
		}), api.MissingHeader},
		{"X-Device-Id が UUID でない", with(func(h http.Header) { h.Set("X-Device-Id", "not-a-uuid") }), api.InvalidHeader},
		{"X-Session-Id が urn:uuid: つき", with(func(h http.Header) { h.Set("X-Session-Id", "urn:uuid:"+testSessionID) }), api.InvalidHeader},
		{"X-Session-Id の重複", with(func(h http.Header) { h.Add("X-Session-Id", testSessionID) }), api.InvalidHeader},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := checkAPIHeaders(tt.header)
			if got == nil {
				t.Fatalf("checkAPIHeaders = nil, want code %q", tt.want)
			}
			if got.Code != tt.want {
				t.Errorf("code = %q, want %q", got.Code, tt.want)
			}
			// 文言はどちらのヘッダーの問題かを示す(TestTableChecksHeadersBeforeQuery 等と同じ前提)。
			if !strings.Contains(got.Message, "X-Device-Id") {
				t.Errorf("message = %q, want it to mention X-Device-Id", got.Message)
			}
		})
	}
}
