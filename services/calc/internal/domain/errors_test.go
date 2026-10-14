package domain

// 失敗の語彙(Code)と engine の sentinel の写し(FromEngine)の単体テスト。
// 契約の ErrorCode との一致は adapters/httpapi の TestDomainCodesAreContractErrorCodes が固定する。

import (
	"errors"
	"fmt"
	"testing"

	"example.com/pokecalc/engine"
)

func codeOf(t *testing.T, err error) Code {
	t.Helper()
	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("err = %v(%T), want *domain.Error", err, err)
	}
	return de.Code
}

func TestAllCodesAreUnique(t *testing.T) {
	seen := map[Code]bool{}
	for _, c := range AllCodes() {
		if c == "" || seen[c] {
			t.Errorf("Code %q が空か重複している", c)
		}
		seen[c] = true
	}
}

func TestErrorfKeepsCodeAndMessage(t *testing.T) {
	err := Errorf(CodeUnknownMove, "技 %q が無い", "x")
	var de *Error
	if !errors.As(err, &de) || de.Code != CodeUnknownMove || de.Error() != `技 "x" が無い` || de.Cause != nil {
		t.Errorf("Errorf = %+v", de)
	}
}

// engine の sentinel は(包まれていても)決まった Code に写り、文言は元のエラーのまま(Cause は持たない)。
func TestFromEngineMapsSentinels(t *testing.T) {
	tests := []struct {
		err  error
		want Code
	}{
		{engine.ErrUnknownPreset, CodeUnknownPreset},
		{engine.ErrDuplicatePreset, CodeDuplicatePreset},
		{engine.ErrInvalidPreset, CodeInvalidPreset},
		{engine.ErrInvalidReverseSide, CodeInvalidReverseSide},
		{engine.ErrNoObservation, CodeNoObservation},
		{engine.ErrInvalidObservation, CodeInvalidObservation},
		{engine.ErrTypeChartMissing, CodeTypeChartMissing},
		{engine.ErrInvalidTypeChart, CodeInvalidTypeChart},
		{engine.ErrUnknownType, CodeUnknownType},
		{engine.ErrMoveDealsNoDamage, CodeInvalidInput},
		{engine.ErrInvalidDefenderOverride, CodeInvalidInput},
		{engine.ErrInvalidAbilityCandidates, CodeInvalidInput},
		{engine.ErrInvalidAdjustInput, CodeInvalidInput},
		{engine.ErrInvalidBattleState, CodeInvalidInput},
		{engine.ErrInvalidComboInput, CodeInvalidInput},
		{engine.ErrInvalidSpeedInput, CodeInvalidInput},
	}
	if len(tests) != len(engineSentinels) {
		t.Fatalf("テストの件数 %d と engineSentinels の件数 %d が違う(表に足したらここにも足す)", len(tests), len(engineSentinels))
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			wrapped := fmt.Errorf("詳細: %w", tt.err)
			got := FromEngine(wrapped)
			var de *Error
			if !errors.As(got, &de) {
				t.Fatalf("FromEngine = %v(%T)", got, got)
			}
			if de.Code != tt.want || de.Message != wrapped.Error() || de.Cause != nil {
				t.Errorf("FromEngine = {%q, %q, %v}, want {%q, %q, nil}", de.Code, de.Message, de.Cause, tt.want, wrapped.Error())
			}
		})
	}
}

// sentinel に無い失敗は固定文の internal にし、元のエラーは Cause にだけ残す(ADR-0200 AC-7)。
func TestFromEngineUnknownIsInternal(t *testing.T) {
	cause := errors.New("ランタイムの詳細")
	var de *Error
	if !errors.As(FromEngine(cause), &de) {
		t.Fatal("*domain.Error でない")
	}
	if de.Code != CodeInternal || de.Message != MessageInternal || !errors.Is(de.Cause, cause) {
		t.Errorf("FromEngine = {%q, %q, %v}, want {internal, 固定文, 元のエラー}", de.Code, de.Message, de.Cause)
	}
}
