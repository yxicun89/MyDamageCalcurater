package events

import (
	"testing"
	"time"

	"example.com/pokecalc/services/internal/calcevents"
)

// AC-N1: CALC_NATS_URL が未設定(空文字)なら発行を丸ごと無効化する(ADR-0212 §6)。
func TestNewWithEmptyURLReturnsNil(t *testing.T) {
	if p := New(""); p != nil {
		t.Errorf("New(\"\") = %v, want nil", p)
	}
}

// New(不正な URL) は失敗しても panic せず nil を返す(ADR-0212 §5。起動をブロックしない)。
func TestNewWithInvalidURLReturnsNil(t *testing.T) {
	if p := New("not a url"); p != nil {
		t.Errorf("New(不正な URL) = %v, want nil", p)
	}
}

// nil の *Publisher に対する呼び出しは安全(発行しないだけ。ADR-0212 §6)。
func TestNilPublisherIsSafe(t *testing.T) {
	var p *Publisher
	p.Publish("device-1", "session-1", calcevents.OperationCalc, time.Now(), nil)
	p.Shutdown()
}

// streamRetryDelay は services/calc/cmd/calc/main.go の backoffDelay と同じ計算をする
// (Initial から倍々に伸ばし、Max で止める。桁あふれしない。ADR-0212 §5)。
func TestStreamRetryDelay(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{-1, createStreamRetryInitial},
		{0, createStreamRetryInitial},
		{1, createStreamRetryInitial * 2},
		{2, createStreamRetryInitial * 4},
		{10, createStreamRetryMax},
		{64, createStreamRetryMax},
		{1 << 20, createStreamRetryMax},
	}
	for _, tt := range tests {
		if got := streamRetryDelay(tt.attempt); got != tt.want {
			t.Errorf("streamRetryDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

// streamConfig はストリーム名・subject・保持設定が ADR-0212 §4 のとおりであることを固定する。
func TestStreamConfig(t *testing.T) {
	if streamConfig.Name != calcevents.StreamName {
		t.Errorf("Name = %q, want %q", streamConfig.Name, calcevents.StreamName)
	}
	wantSubjects := []string{"calc.events.*"}
	if len(streamConfig.Subjects) != 1 || streamConfig.Subjects[0] != wantSubjects[0] {
		t.Errorf("Subjects = %v, want %v", streamConfig.Subjects, wantSubjects)
	}
	if streamConfig.MaxAge != 7*24*time.Hour {
		t.Errorf("MaxAge = %v, want 7日", streamConfig.MaxAge)
	}
	if streamConfig.MaxBytes != streamMaxBytes {
		t.Errorf("MaxBytes = %v, want %v", streamConfig.MaxBytes, streamMaxBytes)
	}
}
