package main

// 締め切りの大小関係(issue #323・#299・#324 の pokedex 分・ADR-0129 §2・§3)。値を2箇所に書かず、
// main.go の定数と httpapi の Default* を不等式で比べる(どちらか一方だけを変えたら検知する)。
//
//	httpapi.DefaultReadinessTimeout ≤ httpapi.DefaultRequestTimeout < writeTimeout ≤ … (外側)
//	shutdownTimeout ≥ httpapi.DefaultRequestTimeout(処理中の要求は締め切りまでに終わり、Shutdown がそれを待てる)

import (
	"testing"

	"example.com/pokecalc/services/pokedex/internal/httpapi"
)

// AC-T1(#323・#299): 要求の締め切りは WriteTimeout より短い(WriteTimeout はハンドラを止めないので、
// 先に締め切りで 503 を書き終える)。readiness の締め切りは要求の締め切り以下。
func TestRequestDeadlineIsShorterThanWriteTimeout(t *testing.T) {
	if httpapi.DefaultRequestTimeout <= 0 {
		t.Fatalf("httpapi.DefaultRequestTimeout = %v, want 正の値", httpapi.DefaultRequestTimeout)
	}
	if httpapi.DefaultRequestTimeout >= writeTimeout {
		t.Errorf("httpapi.DefaultRequestTimeout(%v)が writeTimeout(%v)以上(締め切りの 503 を書く前に WriteTimeout が来て空応答になる)",
			httpapi.DefaultRequestTimeout, writeTimeout)
	}
	if httpapi.DefaultReadinessTimeout <= 0 || httpapi.DefaultReadinessTimeout > httpapi.DefaultRequestTimeout {
		t.Errorf("httpapi.DefaultReadinessTimeout = %v, want 0 < x ≤ DefaultRequestTimeout(%v)",
			httpapi.DefaultReadinessTimeout, httpapi.DefaultRequestTimeout)
	}
}

// AC-T2(#324): shutdownTimeout は要求の締め切り以上(停止の合図の後も、処理中の要求は締め切り内に終わり、
// Shutdown がそれを待ち切ってから終わる。途中で切って空応答にしない)。
func TestShutdownTimeoutCoversRequestDeadline(t *testing.T) {
	if shutdownTimeout < httpapi.DefaultRequestTimeout {
		t.Errorf("shutdownTimeout(%v)が httpapi.DefaultRequestTimeout(%v)より短い", shutdownTimeout, httpapi.DefaultRequestTimeout)
	}
}
