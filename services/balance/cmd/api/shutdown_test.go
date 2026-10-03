package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// issue #324: 停止時は処理中のリクエストを writeTimeout まで待つ(それより前に切らない)。
func TestShutdownTimeoutCoversWriteTimeout(t *testing.T) {
	if shutdownTimeout < writeTimeout {
		t.Errorf("shutdownTimeout(%v)が writeTimeout(%v)より短い", shutdownTimeout, writeTimeout)
	}
}

// 処理が終わらないとき、gracefulShutdown は残りの接続を閉じて error を返す(呼び出し側は異常終了させない)。
func TestGracefulShutdownClosesLeftoverConnections(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	defer ts.Close()
	defer close(release)

	errCh := make(chan error, 1)
	go func() {
		resp, err := http.Get(ts.URL)
		if err == nil {
			resp.Body.Close()
		}
		errCh <- err
	}()
	<-started

	if err := gracefulShutdown(ts.Config, 50*time.Millisecond); err == nil {
		t.Error("締め切りを超えたのに error が無い")
	}
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("残った接続が閉じられていない")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("締め切り後も接続が残っている")
	}
}

// 処理が時間内に終われば error なしで、処理中のリクエストは完了する。
func TestGracefulShutdownWaitsForInflight(t *testing.T) {
	started := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		time.Sleep(100 * time.Millisecond)
	}))
	defer ts.Close()
	done := make(chan error, 1)
	go func() {
		resp, err := http.Get(ts.URL)
		if err == nil {
			resp.Body.Close()
		}
		done <- err
	}()
	<-started
	if err := gracefulShutdown(ts.Config, 2*time.Second); err != nil {
		t.Errorf("gracefulShutdown = %v, want nil", err)
	}
	if err := <-done; err != nil {
		t.Errorf("処理中のリクエストが切られた: %v", err)
	}
}
