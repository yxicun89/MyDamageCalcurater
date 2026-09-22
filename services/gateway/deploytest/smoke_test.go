package deploytest_test

// スモークスクリプト(services/gateway/scripts/smoke.sh)の検査(ADR-0203 §5。AC-S6)。
// k3d を使わず、同じプロセスの中で calc-svc の実物(calctest。架空マスタ)と gateway を httptest で起動し、
// スクリプトをそこへ向けて流す。成功するべき構成で成功し、壊れた構成で失敗する(確認が空振りしない)ことを確かめる。
// k3d 上での実行(`make api-smoke`)は implementer と人間の手動確認。

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/calc/calctest"
	"example.com/pokecalc/services/gateway/deploytest"
	"example.com/pokecalc/services/gateway/internal/httpapi"
)

const smokeScript = "services/gateway/scripts/smoke.sh"

// webMode は gateway の GATEWAY_WEB_URL の状態(ADR-0205)。ゼロ値は k3d の local overlay で Web レーンの
// Service がまだ無い状態(WebURL は設定済みだが接続できない → `/` は 503 upstream_unavailable)。
type webMode int

const (
	webNotDeployed webMode = iota // WebURL は閉じたサーバ(接続拒否 → 503)。スモークは成功するべき
	webServes                     // WebURL は 200 の HTML を返す偽の nginx。スモークは成功するべき
	webUnset                      // WebURL 未設定(`/` は 404)。スモークは落ちるべき
	webBroken                     // WebURL は 500 を返す偽物(200 でも 503 upstream_unavailable でもない)。落ちるべき
)

// stack は gateway の構成の変え方。
type stack struct {
	calcDown       bool    // calc の上流を閉じたサーバにする(接続拒否 → 503)
	pokedexAnswers bool    // pokedex の上流を 200 を返す偽物にする(スモークは 503 を期待するので落ちるべき)
	web            webMode // Web の上流の状態(ゼロ値は k3d で Web 未デプロイのとき)
}

// closedURL は「さっきまで待ち受けていたが今は閉じた」アドレス(接続拒否になる)。
func closedURL(t *testing.T) *url.URL {
	t.Helper()
	closed := httptest.NewServer(http.NotFoundHandler())
	u := mustURL(t, closed.URL)
	closed.Close()
	return u
}

// webURLFor は mode に応じた WebURL を返す(webUnset なら nil)。
func webURLFor(t *testing.T, mode webMode) *url.URL {
	t.Helper()
	respond := func(status int) *url.URL {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(status)
			_, _ = w.Write([]byte("<!doctype html><title>test</title>"))
		}))
		t.Cleanup(srv.Close)
		return mustURL(t, srv.URL)
	}
	switch mode {
	case webServes:
		return respond(http.StatusOK)
	case webBroken:
		return respond(http.StatusInternalServerError)
	case webUnset:
		return nil
	default:
		return closedURL(t)
	}
}

// startStack は calc-svc の実物と gateway を起動し、gateway の基底 URL を返す。
func startStack(t *testing.T, s stack) string {
	t.Helper()
	calcHandler, err := calctest.NewExampleHandler()
	if err != nil {
		t.Fatalf("calc-svc の実物を起動できない: %v", err)
	}
	calcSrv := httptest.NewServer(calcHandler)
	t.Cleanup(calcSrv.Close)
	calcURL := mustURL(t, calcSrv.URL)
	if s.calcDown {
		closed := httptest.NewServer(http.NotFoundHandler())
		calcURL = mustURL(t, closed.URL)
		closed.Close()
	}

	cfg := httpapi.Config{CalcURL: calcURL, UpstreamTimeout: 5 * time.Second, WebURL: webURLFor(t, s.web)}
	if s.pokedexAnswers {
		pokedex := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))
		t.Cleanup(pokedex.Close)
		cfg.PokedexURL = mustURL(t, pokedex.URL)
	}
	h, err := httpapi.NewHandler(cfg)
	if err != nil {
		t.Fatalf("gateway を作れない: %v", err)
	}
	gw := httptest.NewServer(h)
	t.Cleanup(gw.Close)
	return gw.URL
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("URL を解析できない %q: %v", raw, err)
	}
	return u
}

// runSmoke は smoke.sh を sh で流す。balance の確認は切る(kubectl を呼ばない。開発者のクラスタに触らない)。
func runSmoke(t *testing.T, apiURL, retries string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl が無いのでスモークスクリプトを流せない(スクリプトは curl を使う)")
	}
	cmd := exec.Command("sh", deploytest.RepoPath(t, smokeScript))
	cmd.Env = append(os.Environ(),
		"API_URL="+apiURL,
		"API_SMOKE_RETRIES="+retries,
		"API_SMOKE_BALANCE=off",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// AC-S6: スクリプトは POSIX sh(`#!/usr/bin/env sh`・`set -eu`)で、実行ビットがあり、構文が正しい。
func TestSmokeScriptIsPOSIXShell(t *testing.T) {
	path := deploytest.RepoPath(t, smokeScript)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s が無い: %v", smokeScript, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("%s に実行ビットが無い", smokeScript)
	}
	src := readRepoFile(t, smokeScript)
	if !strings.HasPrefix(src, "#!/usr/bin/env sh\n") {
		t.Errorf("%s の1行目が #!/usr/bin/env sh でない", smokeScript)
	}
	if !strings.Contains(src, "\nset -eu\n") {
		t.Errorf("%s に set -eu が無い", smokeScript)
	}
	if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Errorf("sh -n %s: %v\n%s", smokeScript, err, out)
	}
}

// AC-S6: calc-svc と gateway がそろっていて pokedex が未設定なら成功する。内部 API(/internal/*)は 404(ADR-0204)。
func TestSmokeScriptPassesAgainstGatewayAndCalc(t *testing.T) {
	out, err := runSmoke(t, startStack(t, stack{}), "3")
	if err != nil {
		t.Fatalf("smoke.sh が失敗: %v\n%s", err, out)
	}
	want := "api smoke: calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=503 internal=404 balance=skipped"
	if !strings.Contains(out, want) {
		t.Errorf("smoke.sh の出力に %q が無い:\n%s", want, out)
	}
}

// AC-W9(ADR-0205): k3d で Web レーンの Service がまだ無い(GATEWAY_WEB_URL は設定済みで接続できない)とき、
// `/` の 503 upstream_unavailable を許容して成功し、出力の最後に web=503 を出す。内部 API の 404 の確認は維持する。
func TestSmokeScriptAcceptsWebNotDeployed(t *testing.T) {
	out, err := runSmoke(t, startStack(t, stack{web: webNotDeployed}), "3")
	if err != nil {
		t.Fatalf("smoke.sh が失敗(Web 未デプロイの 503 は許容するべき): %v\n%s", err, out)
	}
	want := "internal=404 balance=skipped web=503"
	if !strings.Contains(out, want) {
		t.Errorf("smoke.sh の出力に %q が無い:\n%s", want, out)
	}
}

// AC-W9(ADR-0205): Web がデプロイ済み(`/` が 200)なら成功し、出力の最後に web=200 を出す。
func TestSmokeScriptAcceptsWebDeployed(t *testing.T) {
	out, err := runSmoke(t, startStack(t, stack{web: webServes}), "3")
	if err != nil {
		t.Fatalf("smoke.sh が失敗: %v\n%s", err, out)
	}
	want := "internal=404 balance=skipped web=200"
	if !strings.Contains(out, want) {
		t.Errorf("smoke.sh の出力に %q が無い:\n%s", want, out)
	}
}

// buildDefaultGatewayHandler は startStack(t, stack{}) と同じ構成(calc-svc は実物、pokedex は未設定、
// WebURL は接続できない Web。k3d で Web 未デプロイのとき)の
// gateway ハンドラを作る。TestSmokeScriptRetriesThroughGatewayNotYetListening が listen の開始を自分で
// 遅らせるために、httptest.NewServer(自動で bind される)を使わずここでハンドラだけを組み立てる。
func buildDefaultGatewayHandler(t *testing.T) http.Handler {
	t.Helper()
	calcHandler, err := calctest.NewExampleHandler()
	if err != nil {
		t.Fatalf("calc-svc の実物を起動できない: %v", err)
	}
	calcSrv := httptest.NewServer(calcHandler)
	t.Cleanup(calcSrv.Close)
	h, err := httpapi.NewHandler(httpapi.Config{
		CalcURL: mustURL(t, calcSrv.URL), UpstreamTimeout: 5 * time.Second, WebURL: webURLFor(t, webNotDeployed),
	})
	if err != nil {
		t.Fatalf("gateway を作れない: %v", err)
	}
	return h
}

// AC-S6 回帰(critic 指摘): gateway がロールアウト直後で最初は接続拒否(まだ誰も listen していない)、
// 数百 ms 後に listen を始める場合でも、request_with_retry が正しく再試行して成功すること。
//
// 修正前は request() が `status=$(curl ... || printf '000')` としており、curl 自身が接続拒否時に
// 書き出す "000"(-w '%{http_code}')に、失敗時の `printf '000'` がさらに連結されて "000000" になっていた。
// その値は case の 000/502 のどれにも一致せず、request_with_retry が「再試行不要な最終ステータス」
// と誤認して即座に打ち切っていた(=このテストが無ければ壊れたまま気づけない)。
func TestSmokeScriptRetriesThroughGatewayNotYetListening(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl が無いのでスモークスクリプトを流せない(スクリプトは curl を使う)")
	}
	h := buildDefaultGatewayHandler(t)

	// ポート番号だけを予約してすぐ閉じる(この直後は誰も listen していないので接続拒否になる)。
	reserve, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ポートを予約できない: %v", err)
	}
	addr := reserve.Addr().String()
	if err := reserve.Close(); err != nil {
		t.Fatalf("予約したポートを閉じられない: %v", err)
	}

	srv := &http.Server{Addr: addr, Handler: h}
	t.Cleanup(func() { _ = srv.Close() })
	listenErr := make(chan error, 1)
	go func() {
		// ロールアウト直後(Pod がまだ Ready でない)を模して、しばらく接続拒否のままにする。
		time.Sleep(300 * time.Millisecond)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			listenErr <- err
			return
		}
		listenErr <- nil
		_ = srv.Serve(l)
	}()

	out, err := runSmoke(t, "http://"+addr, "10")
	if lerr := <-listenErr; lerr != nil {
		t.Fatalf("予約したポートで listen できない(テストの前提が崩れている): %v", lerr)
	}
	if err != nil {
		t.Fatalf("smoke.sh が失敗した(接続拒否からの再試行が壊れている): %v\n%s", err, out)
	}
}

// perPathFlaky は「メソッド + パス」ごとに最初の1回だけ 502 Bad Gateway を返し、そのメソッド+パスへの
// 以降のリクエストは next にそのまま委ねる。ロールアウト直後、Traefik がまだ終了中の Pod に振り分けて
// 特定のリクエストだけ一時的に 502 を返す状況を模す。
//
// critic 指摘: 以前は「全体で最初の n 回」に 502 を返す実装だった。すべてのリクエストは最初に
// POST /api/calc(step1)を叩くので、n=2 だとその再試行だけで 502 を使い切ってしまい、step2 以降が
// 一度も 502 に遭遇しない。その結果、request_with_retry を最初の1件(POST /api/calc)にしか使っていない
// 修正前の smoke.sh でもこのテストは(空振りで)成功してしまい、回帰を検出できていなかった。
// メソッド+パスごとに初回だけ落とすことで、step2 以降の異なるパス(bulk・reverse・pokedex・internal・`/`)も
// それぞれ少なくとも1回は 502 に遭遇し、各リクエストで再試行しているかを検証できる。
type perPathFlaky struct {
	next http.Handler

	mu   sync.Mutex
	seen map[string]bool
}

func newPerPathFlaky(next http.Handler) *perPathFlaky {
	return &perPathFlaky{next: next, seen: map[string]bool{}}
}

func (f *perPathFlaky) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	first := !f.seen[key]
	f.seen[key] = true
	f.mu.Unlock()
	if first {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	f.next.ServeHTTP(w, r)
}

// AC-S6 回帰(critic 指摘): ロールアウト直後、前段(Traefik を模した perPathFlaky)がそれぞれのパスへの
// 最初のリクエストだけ 502 を返しても、smoke.sh の各リクエストが 000/502 を再試行する
// (request_with_retry をすべてのリクエストに使うようにした)ので成功する。
func TestSmokeScriptRetriesThroughTransientBadGateway(t *testing.T) {
	h := buildDefaultGatewayHandler(t)
	srv := httptest.NewServer(newPerPathFlaky(h))
	t.Cleanup(srv.Close)

	out, err := runSmoke(t, srv.URL, "5")
	if err != nil {
		t.Fatalf("smoke.sh が失敗(一時的な 502 は再試行して乗り越えるべき): %v\n%s", err, out)
	}
}

// AC-S6 回帰: 502 が再試行の上限を超えて続く場合は smoke.sh も失敗する(再試行が無限にならず、
// 本物の障害を見逃さないこと)。
func TestSmokeScriptFailsOnPersistentBadGateway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	out, err := runSmoke(t, srv.URL, "2")
	if err == nil {
		t.Fatalf("smoke.sh が成功した(502 が続くなら失敗するべき):\n%s", out)
	}
	if !strings.Contains(out, "HTTP 502") {
		t.Errorf("smoke.sh の出力に %q が無い:\n%s", "HTTP 502", out)
	}
}

// AC-S6: 壊れた構成では非ゼロで終わり、ステータスを出す(確認が空振りしない)。
func TestSmokeScriptFailsOnBrokenStack(t *testing.T) {
	tests := []struct {
		name     string
		stack    stack
		wantText string
	}{
		{"calc に届かない(503)", stack{calcDown: true}, "HTTP 503"},
		{"pokedex が答える(503 の確認が効いている)", stack{pokedexAnswers: true}, "/api/pokedex/natures"},
		// ADR-0205: gateway が `/` を 404 にする(GATEWAY_WEB_URL 未設定)なら、Web を後ろに置けていないので失敗。
		{"gateway が / を 404 にする(GATEWAY_WEB_URL 未設定)", stack{web: webUnset}, "api smoke: GET / "},
		{"Web が 500 を返す(200 でも 503 upstream_unavailable でもない)", stack{web: webBroken}, "api smoke: GET / "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runSmoke(t, startStack(t, tt.stack), "1")
			if err == nil {
				t.Fatalf("smoke.sh が成功した(失敗するべき):\n%s", out)
			}
			if !strings.Contains(out, tt.wantText) {
				t.Errorf("smoke.sh の出力に %q が無い:\n%s", tt.wantText, out)
			}
		})
	}
}
