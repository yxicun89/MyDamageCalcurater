package master

// マスタ一式の読み込み(DecodeExport)と入手元(FileSource / HTTPSource)の受け入れテスト(ADR-0204 §2)。
// 上流の pokedex-svc は httptest の偽物で、本文は例のファイル(架空データ)。偽物の本文が契約から
// ずれないよう、contract_test.go が同じ本文を api/openapi.yaml の MasterExport に照らす。

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sharedmaster "example.com/pokecalc/services/internal/master"
)

// masterExportPath は pokedex-svc の内部 API のパス(api/openapi.yaml の getMasterExport)。
const masterExportPath = "/internal/pokedex/master"

func readExample(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(exampleMasterPath)
	if err != nil {
		t.Fatalf("例のファイルを読めない: %v", err)
	}
	return b
}

// AC-M3: 例のファイル(services/calc/testdata/master.example.json)は常に読めて Store になる(README の例が腐らない)。
func TestExampleExportLoads(t *testing.T) {
	export, err := DecodeExport(bytes.NewReader(readExample(t)))
	if err != nil {
		t.Fatalf("DecodeExport(example) = %v, want nil", err)
	}
	store, err := FromExport(export)
	if err != nil {
		t.Fatalf("FromExport(example) = %v, want nil", err)
	}
	if got := store.DataVersion(); got != "example-1" {
		t.Errorf("DataVersion() = %q, want example-1", got)
	}
	// 例の ID(calctest・smoke.sh・cmd/calc のテストが使う)が引けること。
	for _, key := range []string{"9001-000", "9002-000", "9002-001"} {
		if _, ok := store.Species(key); !ok {
			t.Errorf("Species(%s) が無い", key)
		}
	}
	if _, ok := store.Move("testbeam"); !ok {
		t.Error("Move(testbeam) が無い")
	}
	for _, id := range []string{"testatkup", "testneutrala"} {
		if _, ok := store.Nature(id); !ok {
			t.Errorf("Nature(%s) が無い", id)
		}
	}
}

// AC-M3: DecodeExport は厳格に読む。形の不正は ErrInvalidMaster(値の検証は FromExport)。
func TestDecodeExportRejectsInvalid(t *testing.T) {
	example := string(readExample(t))
	replaceOnce := func(t *testing.T, old, repl string) string {
		t.Helper()
		if !strings.Contains(example, old) {
			t.Fatalf("例のファイルに %q が無い(テストの前提が崩れた)", old)
		}
		return strings.Replace(example, old, repl, 1)
	}
	tests := []struct {
		name string
		raw  func(t *testing.T) string
	}{
		{"壊れた JSON", func(*testing.T) string { return `{"schemaVersion":1,` }},
		{"空", func(*testing.T) string { return `` }},
		{"JSON の後ろに余計なデータ", func(*testing.T) string { return example + " {}" }},
		{"未知のトップレベルフィールド", func(t *testing.T) string {
			return replaceOnce(t, `"schemaVersion": 1,`, `"schemaVersion": 1, "extra": true,`)
		}},
		{"未知の種族フィールド(nameEn は含めない契約)", func(t *testing.T) string {
			return replaceOnce(t, `"nameJa": "テストモン",`, `"nameJa": "テストモン", "nameEn": "Testmon",`)
		}},
		{"整数のフィールドに小数", func(t *testing.T) string {
			return replaceOnce(t, `"power": 80,`, `"power": 80.5,`)
		}},
		{"必須の typeChart が欠落(全て等倍にしない)", func(t *testing.T) string {
			start := strings.Index(example, `"typeChart": [`)
			end := strings.Index(example, `"species": [`)
			if start < 0 || end < start {
				t.Fatal("例のファイルの並びが想定と違う")
			}
			return example[:start] + example[end:]
		}},
		{"必須の natures が null", func(t *testing.T) string {
			start := strings.Index(example, `"natures": [`)
			if start < 0 {
				t.Fatal("例のファイルに natures が無い")
			}
			return example[:start] + `"natures": null` + "\n}\n"
		}},
		{"items の要素に effect が無い(critic 指摘: 欠落と null は区別する)", func(t *testing.T) string {
			return replaceOnce(t,
				`"id": "testplainitem", "nameJa": "テストのいし", "effect": null}`,
				`"id": "testplainitem", "nameJa": "テストのいし"}`)
		}},
		{"abilities の要素に effect が無い(critic 指摘: 欠落と null は区別する)", func(t *testing.T) string {
			return replaceOnce(t,
				`"id": "testplain", "nameJa": "テストとくせい", "effect": null}`,
				`"id": "testplain", "nameJa": "テストとくせい"}`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeExport(strings.NewReader(tt.raw(t)))
			if !errors.Is(err, ErrInvalidMaster) {
				t.Fatalf("DecodeExport err = %v, want ErrInvalidMaster", err)
			}
		})
	}
}

// AC-M3: 効果定義の数値は字面のまま共通マスタへ渡す。5324.0 を 5324 に丸めて通さない
// (float64 経由で読むと "5324" に戻って検査をすり抜けるため。ADR-0005 の 4096 基準の整数)。
func TestDecodeExportKeepsEffectLiterals(t *testing.T) {
	raw := strings.Replace(string(readExample(t)), `{"DamageMod": 5324}`, `{"DamageMod": 5324.0}`, 1)
	if raw == string(readExample(t)) {
		t.Fatal("例のファイルに testorb の効果が無い(テストの前提が崩れた)")
	}
	export, err := DecodeExport(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeExport = %v, want nil(効果の中身は FromExport が検証する)", err)
	}
	if _, err := FromExport(export); !errors.Is(err, sharedmaster.ErrInvalidEffect) || !errors.Is(err, ErrInvalidMaster) {
		t.Fatalf("FromExport err = %v, want ErrInvalidMaster かつ ErrInvalidEffect", err)
	}
}

// AC-M4: FileSource は JSON ファイルから読む。無い・壊れているはエラー。
func TestFileSource(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte(`{"schemaVersion":1,`), 0o600); err != nil {
		t.Fatal(err)
	}

	export, err := FileSource{Path: exampleMasterPath}.Fetch(context.Background())
	if err != nil {
		t.Fatalf("FileSource(example).Fetch = %v, want nil", err)
	}
	if export.DataVersion != "example-1" || len(export.Species) != 3 {
		t.Errorf("FileSource(example) = dataVersion %q・種族 %d 件, want example-1・3 件", export.DataVersion, len(export.Species))
	}

	if _, err := (FileSource{Path: filepath.Join(dir, "missing.json")}).Fetch(context.Background()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("FileSource(無いファイル) err = %v, want fs.ErrNotExist", err)
	}
	if _, err := (FileSource{Path: broken}).Fetch(context.Background()); !errors.Is(err, ErrInvalidMaster) {
		t.Errorf("FileSource(壊れたファイル) err = %v, want ErrInvalidMaster", err)
	}
}

// fakePokedex は pokedex-svc の内部 API の偽物。受けたリクエストを記録する。
type fakePokedex struct {
	mu       sync.Mutex
	requests []*http.Request
	handler  http.HandlerFunc
}

func newFakePokedex(t *testing.T, handler http.HandlerFunc) (*fakePokedex, *httptest.Server) {
	t.Helper()
	f := &fakePokedex{handler: handler}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.Clone(context.Background()))
		f.mu.Unlock()
		f.handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakePokedex) recorded() []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*http.Request(nil), f.requests...)
}

func respondJSON(status int, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}

// fetchTimeout はテストの HTTPSource の1回の取得のタイムアウト(遅い上流のケース以外は十分に長い)。
const fetchTimeout = 2 * time.Second

// AC-M4: NewHTTPSource はベース URL(http / https の絶対 URL)とタイムアウト(正)を検証する。
func TestNewHTTPSourceRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		timeout time.Duration
	}{
		{"URL が空", "", fetchTimeout},
		{"スキームが無い", "pokedex", fetchTimeout},
		{"http/https 以外", "ftp://pokedex", fetchTimeout},
		{"ホストが無い", "http://", fetchTimeout},
		{"クエリ付き", "http://pokedex?x=1", fetchTimeout},
		{"タイムアウトが 0", "http://pokedex", 0},
		{"タイムアウトが負", "http://pokedex", -time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if src, err := NewHTTPSource(tt.baseURL, tt.timeout); err == nil {
				t.Fatalf("NewHTTPSource(%q, %v) = %+v, nil; want エラー", tt.baseURL, tt.timeout, src)
			}
		})
	}
}

// AC-M4: HTTPSource は GET {base}/internal/pokedex/master を呼び、200 の本文を MasterExport として読む。
// 端末ID/セッションID は付けない(内部 API。ADR-0204)。ベース URL の末尾の / やパスの接頭辞も扱える。
func TestHTTPSourceFetchesExport(t *testing.T) {
	body := readExample(t)
	tests := []struct {
		name     string
		suffix   string // httptest のベース URL の後ろに付ける
		wantPath string
	}{
		{"ベース URL そのまま", "", masterExportPath},
		{"末尾に /", "/", masterExportPath},
		{"パスの接頭辞", "/pokedex", "/pokedex" + masterExportPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, srv := newFakePokedex(t, respondJSON(http.StatusOK, body))
			src, err := NewHTTPSource(srv.URL+tt.suffix, fetchTimeout)
			if err != nil {
				t.Fatalf("NewHTTPSource = %v", err)
			}
			export, err := src.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch = %v, want nil", err)
			}
			if export.DataVersion != "example-1" || len(export.Species) != 3 || len(export.Natures) != 6 {
				t.Errorf("Fetch = dataVersion %q・種族 %d・性格 %d, want example-1・3・6", export.DataVersion, len(export.Species), len(export.Natures))
			}
			reqs := fake.recorded()
			if len(reqs) != 1 {
				t.Fatalf("上流へのリクエスト数 = %d, want 1", len(reqs))
			}
			if reqs[0].Method != http.MethodGet || reqs[0].URL.Path != tt.wantPath {
				t.Errorf("リクエスト = %s %s, want GET %s", reqs[0].Method, reqs[0].URL.Path, tt.wantPath)
			}
			if reqs[0].Header.Get("X-Device-Id") != "" || reqs[0].Header.Get("X-Session-Id") != "" {
				t.Errorf("内部 API に端末ID/セッションID を付けた: %v", reqs[0].Header)
			}
		})
	}
}

// AC-M4: 200 以外・壊れた本文・未知のフィールド・接続できない・タイムアウトはエラー(部分的な結果を使わない)。
func TestHTTPSourceFailures(t *testing.T) {
	example := readExample(t)
	unavailable := []byte(`{"code":"master_unavailable","message":"マスタが未投入"}`)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{"503 master_unavailable", respondJSON(http.StatusServiceUnavailable, unavailable), ErrMasterUnavailable},
		{"404", respondJSON(http.StatusNotFound, []byte(`{"code":"not_found","message":"無い"}`)), ErrMasterUnavailable},
		{"500", respondJSON(http.StatusInternalServerError, []byte(`{"code":"internal","message":"内部"}`)), ErrMasterUnavailable},
		{"200 でも本文が 200 以外の形(Error)", respondJSON(http.StatusOK, unavailable), ErrInvalidMaster},
		{"200 で壊れた JSON", respondJSON(http.StatusOK, []byte(`{"schemaVersion":1,`)), ErrInvalidMaster},
		{"200 で未知のフィールド", respondJSON(http.StatusOK,
			bytes.Replace(example, []byte(`"schemaVersion": 1,`), []byte(`"schemaVersion": 1, "extra": 1,`), 1)), ErrInvalidMaster},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, srv := newFakePokedex(t, tt.handler)
			src, err := NewHTTPSource(srv.URL, fetchTimeout)
			if err != nil {
				t.Fatalf("NewHTTPSource = %v", err)
			}
			if _, err := src.Fetch(context.Background()); !errors.Is(err, tt.want) {
				t.Fatalf("Fetch err = %v, want %v", err, tt.want)
			}
		})
	}
}

// AC-M4: 接続できない上流は ErrMasterUnavailable。
func TestHTTPSourceConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close() // 閉じたポートに向ける
	src, err := NewHTTPSource(base, fetchTimeout)
	if err != nil {
		t.Fatalf("NewHTTPSource = %v", err)
	}
	if _, err := src.Fetch(context.Background()); !errors.Is(err, ErrMasterUnavailable) {
		t.Fatalf("Fetch err = %v, want ErrMasterUnavailable", err)
	}
}

// AC-M4: 応答しない上流はタイムアウトで ErrMasterUnavailable(待ち続けない)。
func TestHTTPSourceTimeout(t *testing.T) {
	release := make(chan struct{})
	_, srv := newFakePokedex(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) }) // srv.Close より先に走る(Cleanup は後入れ先出し)

	const shortTimeout = 100 * time.Millisecond
	src, err := NewHTTPSource(srv.URL, shortTimeout)
	if err != nil {
		t.Fatalf("NewHTTPSource = %v", err)
	}
	start := time.Now()
	_, err = src.Fetch(context.Background())
	if !errors.Is(err, ErrMasterUnavailable) {
		t.Fatalf("Fetch err = %v, want ErrMasterUnavailable", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Fetch が %v かかった(タイムアウト %v で打ち切ること)", elapsed, shortTimeout)
	}
}

// AC-M4: 呼び出し側の ctx が終わったら ctx のエラーで返る(起動時の再試行を止められる)。
func TestHTTPSourceHonorsContext(t *testing.T) {
	release := make(chan struct{})
	_, srv := newFakePokedex(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })

	src, err := NewHTTPSource(srv.URL, 10*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPSource = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	_, err = src.Fetch(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Fetch が ctx の終了後も %v 待った", elapsed)
	}
}

// critic 指摘: 本文の途中で接続が切れる上流は、JSON として不正なのではなく取得できていないだけなので
// ErrMasterUnavailable(ErrInvalidMaster にしない)。Content-Length を実際より大きく宣言してから
// 少しだけ書いて接続を切り、クライアント側に読み切れないことを気付かせる。
func TestHTTPSourceBodyStopsMidStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			// ハンドラはサーバの goroutine で動くので Fatal ではなく Error + return にする。
			t.Error("ResponseWriter が http.Hijacker を実装していない")
			return
		}
		conn, rw, err := hj.Hijack()
		if err != nil {
			t.Errorf("Hijack = %v", err)
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000000\r\n\r\n")
		_, _ = rw.WriteString(`{"schemaVersion":1,`)
		_ = rw.Flush()
		// ここで接続を閉じる(defer conn.Close)。宣言した Content-Length に満たないので、
		// クライアントは本文を読み切れず io.ErrUnexpectedEOF 相当のエラーになる。
	}))
	t.Cleanup(srv.Close)

	src, err := NewHTTPSource(srv.URL, fetchTimeout)
	if err != nil {
		t.Fatalf("NewHTTPSource = %v", err)
	}
	if _, err := src.Fetch(context.Background()); !errors.Is(err, ErrMasterUnavailable) {
		t.Fatalf("Fetch err = %v, want ErrMasterUnavailable(本文が途中で切れた)", err)
	}
}

// critic 指摘: 本文を読んでいる最中に呼び出し側の ctx が終わったら、ErrMasterUnavailable ではなく
// ctx のエラーで返る(タイムアウト到達前の応答ヘッダ受信後、本文のストリーミング中の cancel を再現する)。
func TestHTTPSourceContextEndsWhileReadingBody(t *testing.T) {
	headerSent := make(chan struct{})
	release := make(chan struct{})
	_, srv := newFakePokedex(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"schemaVersion":1,`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(headerSent)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })

	src, err := NewHTTPSource(srv.URL, 10*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPSource = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var fetchErr error
	go func() {
		_, fetchErr = src.Fetch(ctx)
		close(done)
	}()

	<-headerSent
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Fetch が ctx の終了後も 5 秒以内に返らない")
	}
	if !errors.Is(fetchErr, context.Canceled) {
		t.Fatalf("Fetch err = %v, want context.Canceled", fetchErr)
	}
}

// critic 指摘: 本文が上限(maxMasterExportBytes)を超えたら ErrInvalidMaster(内容の不正として扱う)。
func TestHTTPSourceRejectsOversizedBody(t *testing.T) {
	huge := bytes.Repeat([]byte(" "), maxMasterExportBytes+1)
	_, srv := newFakePokedex(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(huge)
	})

	src, err := NewHTTPSource(srv.URL, 10*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPSource = %v", err)
	}
	if _, err := src.Fetch(context.Background()); !errors.Is(err, ErrInvalidMaster) {
		t.Fatalf("Fetch err = %v, want ErrInvalidMaster(本文が上限を超える)", err)
	}
}

// critic 指摘: DecodeExport 自身も本文の上限を超えたら ErrInvalidMaster にする(HTTPSource 経由に限らない)。
func TestDecodeExportRejectsOversizedBody(t *testing.T) {
	huge := bytes.Repeat([]byte(" "), maxMasterExportBytes+1)
	if _, err := DecodeExport(bytes.NewReader(huge)); !errors.Is(err, ErrInvalidMaster) {
		t.Fatalf("DecodeExport(oversized) err = %v, want ErrInvalidMaster", err)
	}
}
