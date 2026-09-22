package httpapi

// critic 指摘 R2: engine/wasmapi の Code* 語彙と、engine の sentinel → ErrorCode の写像が
// wasmapi と同じ文字列であることを固定する(ADR-0200: 「同じ失敗は HTTP と WASM で同じ code」)。
// httpapi は engine/wasmapi に依存しない構成を保つ(ADR-0200 §3 の補足)ので、定数どうしを
// 1つずつ突き合わせるだけで、wasmapi パッケージを import すること自体はしない。

import (
	"errors"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
	"example.com/pokecalc/services/internal/api"
)

// wasmCodes は engine/wasmapi.Code* 定数の全量(手で列挙。新しい code を足したらここにも足すこと)。
var wasmCodes = []string{
	wasmapi.CodeInvalidJSON,
	wasmapi.CodeUnknownField,
	wasmapi.CodeInvalidEnum,
	wasmapi.CodeInvalidInput,
	wasmapi.CodeUnknownPreset,
	wasmapi.CodeDuplicatePreset,
	wasmapi.CodeInvalidPreset,
	wasmapi.CodeInvalidReverseSide,
	wasmapi.CodeNoObservation,
	wasmapi.CodeInvalidObservation,
	wasmapi.CodeTypeChartMissing,
	wasmapi.CodeInvalidTypeChart,
	wasmapi.CodeUnknownType,
	wasmapi.CodeInternal,
}

// R2: wasmapi の全 Code* が契約の ErrorCode enum に含まれる。
func TestWasmCodesAreValidErrorCodes(t *testing.T) {
	for _, code := range wasmCodes {
		t.Run(code, func(t *testing.T) {
			if !api.ErrorCode(code).Valid() {
				t.Errorf("api.ErrorCode(%q).Valid() = false, want true", code)
			}
		})
	}
}

// R2: engine の sentinel → httpapi の ErrorCode の写像が wasmapi.Code* と同じ文字列であること。
func TestErrorCodeVocabularyMatchesWasm(t *testing.T) {
	tests := []struct {
		sentinel error
		wantCode string
	}{
		{engine.ErrUnknownPreset, wasmapi.CodeUnknownPreset},
		{engine.ErrDuplicatePreset, wasmapi.CodeDuplicatePreset},
		{engine.ErrInvalidPreset, wasmapi.CodeInvalidPreset},
		{engine.ErrInvalidReverseSide, wasmapi.CodeInvalidReverseSide},
		{engine.ErrNoObservation, wasmapi.CodeNoObservation},
		{engine.ErrInvalidObservation, wasmapi.CodeInvalidObservation},
		{engine.ErrTypeChartMissing, wasmapi.CodeTypeChartMissing},
		{engine.ErrInvalidTypeChart, wasmapi.CodeInvalidTypeChart},
		{engine.ErrUnknownType, wasmapi.CodeUnknownType},
	}
	for _, tt := range tests {
		t.Run(tt.wantCode, func(t *testing.T) {
			got, ok := errFromEngine(tt.sentinel).(*httpError)
			if !ok {
				t.Fatalf("errFromEngine(%v) の型が *httpError でない", tt.sentinel)
			}
			if string(got.code) != tt.wantCode {
				t.Errorf("code = %q, want %q(wasmapi と不一致)", got.code, tt.wantCode)
			}
		})
	}

	// sentinel に無い想定外のエラーは、wasmapi の CodeInternal と同じ文字列にする。
	t.Run(wasmapi.CodeInternal, func(t *testing.T) {
		got, ok := errFromEngine(errors.New("想定外")).(*httpError)
		if !ok {
			t.Fatalf("errFromEngine の型が *httpError でない")
		}
		if string(got.code) != wasmapi.CodeInternal {
			t.Errorf("code = %q, want %q", got.code, wasmapi.CodeInternal)
		}
	})
}
