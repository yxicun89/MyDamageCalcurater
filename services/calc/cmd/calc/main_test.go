package main

// calc-svc の起動(設定・マスタの入手)の受け入れテスト(ADR-0200 AC-10 → ADR-0204 §3)。
// 例のマスタ(MasterExport の形の架空データ。相性表を含む)で起動できること、設定の不正・壊れたファイルでは
// 起動しないこと、URL 方式では上流(pokedex-svc の偽物)が準備できるまで 503 で待ち、準備できたら計算に答えること。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	exampleMasterPath   = "../../testdata/master.example.json"
	sharedTypeChartPath = "../../../../testdata/golden/typechart.json"
	masterExportPath    = "/internal/pokedex/master" // api/openapi.yaml の getMasterExport
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func fileEnv() map[string]string { return map[string]string{envMasterPath: exampleMasterPath} }

// withDefaults は loadConfig が環境変数に依らず埋める既定値(再試行の間隔・1回の取得のタイムアウト)を足す。
func withDefaults(c config) config {
	c.MasterRetry = retryPolicy{Initial: defaultMasterRetryInitial, Max: defaultMasterRetryMax}
	c.MasterFetchTimeout = defaultMasterFetchTimeout
	return c
}

// testRetry はテスト用の短い再試行間隔。
var testRetry = retryPolicy{Initial: 10 * time.Millisecond, Max: 40 * time.Millisecond}

// 環境変数の名前は運用(k8s の manifest・README・scripts/dev.sh)が依存するので固定する。
func TestEnvNames(t *testing.T) {
	got := []string{envAddr, envMasterURL, envMasterPath, envNatsURL, envTypeChartPath}
	want := []string{"CALC_ADDR", "CALC_MASTER_URL", "CALC_MASTER_PATH", "CALC_NATS_URL", "CALC_TYPECHART_PATH"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("環境変数名 = %q, want %q", got[i], want[i])
		}
	}
}

// 既定値の関係(再試行の間隔は正で Initial <= Max、取得のタイムアウトは正)。
func TestDefaultMasterFetchSettings(t *testing.T) {
	if defaultMasterRetryInitial <= 0 || defaultMasterRetryMax < defaultMasterRetryInitial {
		t.Errorf("再試行の既定値 Initial=%v Max=%v(0 < Initial <= Max であること)", defaultMasterRetryInitial, defaultMasterRetryMax)
	}
	if defaultMasterFetchTimeout <= 0 {
		t.Errorf("取得のタイムアウトの既定値 = %v(正であること)", defaultMasterFetchTimeout)
	}
}

// AC-B1: CALC_MASTER_URL と CALC_MASTER_PATH のちょうど1つ。CALC_TYPECHART_PATH は廃止(設定されていたら起動しない)。
func TestLoadConfig(t *testing.T) {
	with := func(base map[string]string, kv ...string) map[string]string {
		e := map[string]string{}
		for k, v := range base {
			e[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			e[kv[i]] = kv[i+1]
		}
		return e
	}
	tests := []struct {
		name    string
		env     map[string]string
		want    config
		wantErr bool
	}{
		{"ファイル方式・既定の待ち受けアドレス", fileEnv(), withDefaults(config{Addr: ":8080", MasterPath: exampleMasterPath}), false},
		{"CALC_ADDR が空なら既定", with(fileEnv(), envAddr, ""), withDefaults(config{Addr: ":8080", MasterPath: exampleMasterPath}), false},
		{"CALC_ADDR の指定", with(fileEnv(), envAddr, "127.0.0.1:9090"),
			withDefaults(config{Addr: "127.0.0.1:9090", MasterPath: exampleMasterPath}), false},
		{"URL 方式", map[string]string{envMasterURL: "http://pokedex"}, withDefaults(config{Addr: ":8080", MasterURL: "http://pokedex"}), false},
		{"URL 方式(https・ポート・パス)", map[string]string{envMasterURL: "https://pokedex.example:8443/base"},
			withDefaults(config{Addr: ":8080", MasterURL: "https://pokedex.example:8443/base"}), false},
		{"URL があり CALC_MASTER_PATH が空なら URL 方式", map[string]string{envMasterURL: "http://pokedex", envMasterPath: ""},
			withDefaults(config{Addr: ":8080", MasterURL: "http://pokedex"}), false},
		{"CALC_TYPECHART_PATH が空なら未設定と同じ", with(fileEnv(), envTypeChartPath, ""),
			withDefaults(config{Addr: ":8080", MasterPath: exampleMasterPath}), false},
		{"CALC_NATS_URL の指定(ADR-0212)", with(fileEnv(), envNatsURL, "nats://127.0.0.1:4222"),
			withDefaults(config{Addr: ":8080", MasterPath: exampleMasterPath, NatsURL: "nats://127.0.0.1:4222"}), false},
		{"CALC_NATS_URL が空なら未設定と同じ(発行を無効化)", with(fileEnv(), envNatsURL, ""),
			withDefaults(config{Addr: ":8080", MasterPath: exampleMasterPath}), false},

		{"どちらも無い", map[string]string{}, config{}, true},
		{"どちらも空", map[string]string{envMasterURL: "", envMasterPath: ""}, config{}, true},
		{"両方ある", map[string]string{envMasterURL: "http://pokedex", envMasterPath: exampleMasterPath}, config{}, true},
		{"URL にスキームが無い", map[string]string{envMasterURL: "pokedex"}, config{}, true},
		{"URL が http/https 以外", map[string]string{envMasterURL: "ftp://pokedex"}, config{}, true},
		{"URL にホストが無い", map[string]string{envMasterURL: "http://"}, config{}, true},
		{"廃止した CALC_TYPECHART_PATH がある(ファイル方式)", with(fileEnv(), envTypeChartPath, sharedTypeChartPath), config{}, true},
		{"廃止した CALC_TYPECHART_PATH がある(URL 方式)",
			map[string]string{envMasterURL: "http://pokedex", envTypeChartPath: sharedTypeChartPath}, config{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(lookupFrom(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("loadConfig = %+v, nil; want エラー", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("loadConfig = %+v, %v; want %+v, nil", got, err, tt.want)
			}
		})
	}
}

// AC-B2: 再試行の間隔は Initial から倍々に伸び、Max で止まる(桁あふれしない)。
func TestBackoffDelay(t *testing.T) {
	p := retryPolicy{Initial: 100 * time.Millisecond, Max: time.Second}
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{-1, 100 * time.Millisecond},
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, time.Second},
		{10, time.Second},
		{64, time.Second},
		{1 << 20, time.Second},
	}
	for _, tt := range tests {
		if got := backoffDelay(tt.attempt, p); got != tt.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

// --- リクエストの補助 ------------------------------------------------------------

// 例のマスタの ID(services/calc/testdata/master.example.json。calctest・smoke.sh と同じ)。
const exampleCalcBody = `{"format":"single",` +
	`"attacker":{"speciesKey":"9001-000","natureId":"testatkup","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}},` +
	`"defender":{"speciesKey":"9002-000","natureId":"testneutrala","sp":{"hp":32,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}},` +
	`"moveId":"testbeam"}`

var exampleBodies = map[string]string{
	"/api/calc": exampleCalcBody,
	"/api/calc/bulk": `{"format":"single",` +
		`"attacker":{"speciesKey":"9001-000","natureId":"testatkup","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}},` +
		`"defenderSpeciesKey":"9002-000","moveId":"testbeam"}`,
	"/api/calc/reverse": `{"format":"single","side":"defender",` +
		`"known":{"speciesKey":"9001-000","natureId":"testatkup","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}},` +
		`"unknownSpeciesKey":"9002-000","moveId":"testbeam","observations":[{"percent":18}]}`,
}

func postCalc(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-Session-Id", "00000000-0000-4000-8000-000000000002")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("エラー本文が JSON でない: %v; body=%s", err, rec.Body.String())
	}
	return e.Code
}

func assertRolls(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/calc status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var res struct {
		Rolls []int `json:"rolls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || len(res.Rolls) != 16 || res.Rolls[15] <= 0 {
		t.Fatalf("計算結果が不正: %v; body=%s", err, rec.Body.String())
	}
}

// waitFor は cond が真になるまで(最大 timeout)待つ。
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s が %v 以内に成り立たない", what, timeout)
}

// --- ファイル方式 ----------------------------------------------------------------

// AC-B3: 例のマスタ(ファイル方式)で起動したハンドラが、/healthz・/readyz・計算に答える(端から端まで)。
func TestNewHandlerServesExampleMaster(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(fileEnv()))
	if err != nil {
		t.Fatalf("loadConfig = %v", err)
	}
	h, _, err := newHandler(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newHandler = %v", err)
	}
	if rec := get(h, "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want 200", rec.Code)
	}
	if rec := get(h, "/readyz"); rec.Code != http.StatusOK {
		t.Fatalf("/readyz status = %d, want 200(ファイル方式は起動時に読み込み済み); body=%s", rec.Code, rec.Body.String())
	}
	assertRolls(t, postCalc(t, h, "/api/calc", exampleCalcBody))
}

// AC-B3: ファイル方式の起動前の失敗: ファイルが無い・壊れている・形が違う(旧スナップショット・相性表 JSON)なら
// newHandler も run もエラー(非ゼロ終了の元)。設定の不正は loadConfig の段で落ちる。
func TestStartupFailsOnBadFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	broken := write("broken.json", `{"schemaVersion":1,`)
	wrongSchema := write("wrong.json", `{"schemaVersion":1,"unknown":true}`)
	// ADR-0200 の暫定スナップショット形式(ADR-0204 で廃止)。dataVersion・types・typeChart が無い。
	oldSnapshot := write("old.json", `{"schemaVersion":1,"species":[],"moves":[],"items":[],"abilities":[],"natures":[]}`)
	missing := filepath.Join(dir, "missing.json")

	tests := []struct {
		name          string
		env           map[string]string
		wantConfigErr bool // true: 設定の段で落ちる / false: 設定は通り、ファイルの読み込みで落ちる
	}{
		{"マスタが無い", map[string]string{envMasterPath: missing}, false},
		{"マスタが壊れている", map[string]string{envMasterPath: broken}, false},
		{"マスタが契約違反", map[string]string{envMasterPath: wrongSchema}, false},
		{"旧形式のスナップショット", map[string]string{envMasterPath: oldSnapshot}, false},
		{"相性表の JSON をマスタとして渡した", map[string]string{envMasterPath: sharedTypeChartPath}, false},
		{"マスタの設定が無い", map[string]string{}, true},
		{"廃止した CALC_TYPECHART_PATH がある", map[string]string{envMasterPath: exampleMasterPath, envTypeChartPath: sharedTypeChartPath}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(lookupFrom(tt.env))
			if tt.wantConfigErr {
				if err == nil {
					t.Fatalf("loadConfig = %+v, nil; want エラー", cfg)
				}
			} else {
				if err != nil {
					t.Fatalf("loadConfig = %v; want nil(設定はそろっている)", err)
				}
				if h, _, err := newHandler(context.Background(), cfg); err == nil || h != nil {
					t.Errorf("newHandler = %v, %v; want nil, エラー", h, err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			env := map[string]string{envAddr: "127.0.0.1:0"}
			for k, v := range tt.env {
				env[k] = v
			}
			if err := run(ctx, lookupFrom(env)); err == nil {
				t.Error("run = nil, want 起動エラー")
			}
		})
	}
}

// 正常な設定なら run は待ち受け、ctx の終了で止まって nil を返す(ファイル方式)。
func TestRunStopsOnContextCancel(t *testing.T) {
	env := fileEnv()
	env[envAddr] = "127.0.0.1:0"
	assertRunsUntilCanceled(t, env)
}

func assertRunsUntilCanceled(t *testing.T, env map[string]string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, lookupFrom(env)) }()

	// 起動直後の失敗(設定・マスタ)はすぐ返るので、少し待ってまだ動いていることを確かめる。
	select {
	case err := <-done:
		cancel()
		t.Fatalf("run が ctx の終了前に返った: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run = %v, want nil(正常な停止)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run が ctx の終了後 5 秒以内に止まらない")
	}
}

// --- URL 方式 --------------------------------------------------------------------

// fakePokedex は pokedex-svc の内部 API の偽物。ready が false の間は 503 master_unavailable、
// true になったら例のマスタを返す。呼ばれた回数を数える。
type fakePokedex struct {
	ready atomic.Bool
	calls atomic.Int32
	paths chan string
}

func newFakePokedex(t *testing.T) (*fakePokedex, *httptest.Server) {
	t.Helper()
	body, err := os.ReadFile(exampleMasterPath)
	if err != nil {
		t.Fatalf("例のマスタを読めない: %v", err)
	}
	f := &fakePokedex{paths: make(chan string, 1024)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		select {
		case f.paths <- r.Method + " " + r.URL.Path:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		if !f.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"master_unavailable","message":"マスタが未投入"}`))
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func urlConfig(base string) config {
	return config{Addr: "127.0.0.1:0", MasterURL: base, MasterRetry: testRetry, MasterFetchTimeout: 2 * time.Second}
}

// AC-B4: URL 方式は上流が準備できていなくても起動を止めない(newHandler はすぐ返る)。準備中は
// /healthz 200・/readyz 503・calc の3操作 503 master_unavailable。上流は GET /internal/pokedex/master を再試行で呼ばれる。
// 上流が準備できたら /readyz 200・計算 200 に変わり、その後は再取得しない(マスタの更新は再起動で反映。ADR-0204)。
func TestURLModeBecomesReadyAfterUpstream(t *testing.T) {
	fake, srv := newFakePokedex(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	h, _, err := newHandler(ctx, urlConfig(srv.URL))
	if err != nil {
		t.Fatalf("newHandler(URL 方式) = %v, want nil(上流が準備中でも起動する)", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("newHandler が %v かかった(取得を待たずに返すこと)", elapsed)
	}

	if rec := get(h, "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("準備中の /healthz status = %d, want 200", rec.Code)
	}
	if rec := get(h, "/readyz"); rec.Code != http.StatusServiceUnavailable || errorCode(t, rec) != "master_unavailable" {
		t.Errorf("準備中の /readyz = %d %s, want 503 master_unavailable", rec.Code, rec.Body.String())
	}
	for path, body := range exampleBodies {
		rec := postCalc(t, h, path, body)
		if rec.Code != http.StatusServiceUnavailable || errorCode(t, rec) != "master_unavailable" {
			t.Errorf("準備中の %s = %d %s, want 503 master_unavailable", path, rec.Code, rec.Body.String())
		}
	}

	// 再試行している(1回で諦めない)。
	waitFor(t, 5*time.Second, "上流への再試行(3回以上)", func() bool { return fake.calls.Load() >= 3 })
	if got := <-fake.paths; got != http.MethodGet+" "+masterExportPath {
		t.Errorf("上流へのリクエスト = %q, want GET %s", got, masterExportPath)
	}

	fake.ready.Store(true)
	waitFor(t, 5*time.Second, "/readyz が 200", func() bool { return get(h, "/readyz").Code == http.StatusOK })
	assertRolls(t, postCalc(t, h, "/api/calc", exampleCalcBody))
	for path, body := range exampleBodies {
		if rec := postCalc(t, h, path, body); rec.Code != http.StatusOK {
			t.Errorf("準備後の %s status = %d, want 200; body=%s", path, rec.Code, rec.Body.String())
		}
	}

	// 取得後は再取得しない。
	after := fake.calls.Load()
	time.Sleep(10 * testRetry.Max)
	if got := fake.calls.Load(); got != after {
		t.Errorf("取得後も上流を呼んだ: %d → %d 回", after, got)
	}
}

// AC-B4: 取得の失敗(503・壊れた本文・検証エラー)はどれも再試行し、正しいマスタが返ったら準備済みになる。
func TestURLModeRetriesOnInvalidExport(t *testing.T) {
	example, err := os.ReadFile(exampleMasterPath)
	if err != nil {
		t.Fatal(err)
	}
	responses := [][]byte{
		[]byte(`{"schemaVersion":1,`),                                          // 壊れた JSON
		bytes.Replace(example, []byte(`"testbeam"`), []byte(`"test-beam"`), 1), // 検証エラー(技 ID の形式)
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1)) - 1
		w.Header().Set("Content-Type", "application/json")
		if n < len(responses) {
			_, _ = w.Write(responses[n])
			return
		}
		_, _ = w.Write(example)
	}))
	t.Cleanup(srv.Close)
	if !strings.Contains(string(responses[1]), `"test-beam"`) {
		t.Fatal("例のマスタに testbeam が無い(テストの前提が崩れた)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, _, err := newHandler(ctx, urlConfig(srv.URL))
	if err != nil {
		t.Fatalf("newHandler = %v", err)
	}
	waitFor(t, 5*time.Second, "/readyz が 200", func() bool { return get(h, "/readyz").Code == http.StatusOK })
	if got := calls.Load(); got < 3 {
		t.Errorf("上流を呼んだ回数 = %d, want 3 以上(失敗を再試行して準備済みになる)", got)
	}
	assertRolls(t, postCalc(t, h, "/api/calc", exampleCalcBody))
}

// AC-B5: ctx が終わったら再試行を止める(上流を呼び続けない)。準備済みにはならない。
func TestURLModeStopsRetryingOnCancel(t *testing.T) {
	fake, srv := newFakePokedex(t) // ready にしない(ずっと 503)
	ctx, cancel := context.WithCancel(context.Background())
	h, _, err := newHandler(ctx, urlConfig(srv.URL))
	if err != nil {
		cancel()
		t.Fatalf("newHandler = %v", err)
	}
	waitFor(t, 5*time.Second, "上流への再試行(3回以上)", func() bool { return fake.calls.Load() >= 3 })
	cancel()
	time.Sleep(5 * testRetry.Max) // 進行中の1回が終わるのを待つ
	stopped := fake.calls.Load()
	time.Sleep(10 * testRetry.Max)
	if got := fake.calls.Load(); got != stopped {
		t.Errorf("ctx の終了後も上流を呼び続けた: %d → %d 回", stopped, got)
	}
	if rec := get(h, "/readyz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz status = %d, want 503(準備済みになっていない)", rec.Code)
	}
}

// AC-B5: URL 方式の run は上流に届かなくても起動エラーにせず待ち受け、ctx の終了で nil を返す。
func TestRunURLModeStartsWithoutUpstream(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close() // 接続できない上流
	assertRunsUntilCanceled(t, map[string]string{envAddr: "127.0.0.1:0", envMasterURL: base})
}
