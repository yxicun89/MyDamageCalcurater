package events

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"

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
	if streamConfig.Retention != jetstream.LimitsPolicy {
		t.Errorf("Retention = %v, want LimitsPolicy(ADR-0212 §4。Interest retention は使わない)", streamConfig.Retention)
	}
	if streamConfig.Storage != jetstream.FileStorage {
		t.Errorf("Storage = %v, want FileStorage", streamConfig.Storage)
	}
	if streamConfig.Discard != jetstream.DiscardOld {
		t.Errorf("Discard = %v, want DiscardOld", streamConfig.Discard)
	}
	if streamConfig.Replicas != 1 {
		t.Errorf("Replicas = %v, want 1", streamConfig.Replicas)
	}
}

// AC-N2: 接続先が TCP では繋がるが NATS の INFO を返さない(応答が返らない)場合でも、
// New は connectTimeout 程度で返る(calc-svc の起動をブロックしない)。既定の2秒
// (nats.go の Timeout オプション既定値)より十分短いことを確認する(critic レビューで
// 実測して判明した起動ブロックの回帰確認)。外部ネットワークに依存する固定 IP は使わず、
// ローカルの listener で「接続はできるが何も返さない」状態を再現する
// (nats.go は Opts.Timeout を TCP dial だけでなく最初の INFO の読み取りにも
// SetDeadline で適用するため、これでも connectTimeout の検証になる)。
func TestNewReturnsQuicklyForUnresponsiveHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener を作れない: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// わざと何も書かない・読まない(INFO を返さないことで接続を意図的にハングさせる)。
		time.Sleep(5 * time.Second)
	}()

	start := time.Now()
	p := New(fmt.Sprintf("nats://%s", ln.Addr().String()))
	elapsed := time.Since(start)
	if p == nil {
		t.Fatal("New が nil を返した(RetryOnFailedConnect(true) なので接続失敗時も nil にならないはず)")
	}
	p.nc.Close()
	if elapsed > time.Second {
		t.Errorf("New に %v かかった(connectTimeout=%v 程度で返るはず。起動をブロックしないこと。AC-N2)", elapsed, connectTimeout)
	}
}
