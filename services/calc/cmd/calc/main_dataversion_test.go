package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// 例のマスタ(testdata/master.example.json)の dataVersion。
const exampleDataVersion = "example-1"

// syncBuffer は slog の出力を並行に書き込まれても安全に集める。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func readyzDataVersion(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := get(h, "/readyz")
	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status      string `json:"status"`
		DataVersion string `json:"dataVersion"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("/readyz の本文が JSON でない: %v; body=%s", err, rec.Body.String())
	}
	if body.Status != "ok" {
		t.Errorf("/readyz status フィールド = %q, want ok", body.Status)
	}
	return body.DataVersion
}

// issue #281: ファイル方式は起動ログと /readyz に読み込んだ dataVersion を出す。
func TestFileModeReportsDataVersion(t *testing.T) {
	logs := captureLogs(t)
	cfg, err := loadConfig(lookupFrom(fileEnv()))
	if err != nil {
		t.Fatalf("loadConfig = %v", err)
	}
	h, _, err := newHandler(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newHandler = %v", err)
	}
	if got := readyzDataVersion(t, h); got != exampleDataVersion {
		t.Errorf("/readyz dataVersion = %q, want %q", got, exampleDataVersion)
	}
	if out := logs.String(); !strings.Contains(out, "dataVersion="+exampleDataVersion) {
		t.Errorf("起動ログに dataVersion が無い: %s", out)
	}
}

// issue #281: URL 方式は取得前の /readyz が 503 のまま(dataVersion を出さない)で、取得後に dataVersion を出し、ログにも残す。
func TestURLModeReportsDataVersionAfterFetch(t *testing.T) {
	logs := captureLogs(t)
	fake, srv := newFakePokedex(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h, _, err := newHandler(ctx, urlConfig(srv.URL))
	if err != nil {
		t.Fatalf("newHandler = %v", err)
	}
	if rec := get(h, "/readyz"); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "dataVersion") {
		t.Errorf("準備中の /readyz = %d %s, want 503 で dataVersion なし", rec.Code, rec.Body.String())
	}
	fake.ready.Store(true)
	waitFor(t, 5*time.Second, "/readyz が 200", func() bool { return get(h, "/readyz").Code == http.StatusOK })
	if got := readyzDataVersion(t, h); got != exampleDataVersion {
		t.Errorf("/readyz dataVersion = %q, want %q", got, exampleDataVersion)
	}
	waitFor(t, 5*time.Second, "起動ログの dataVersion", func() bool {
		return strings.Contains(logs.String(), "dataVersion="+exampleDataVersion)
	})
}
