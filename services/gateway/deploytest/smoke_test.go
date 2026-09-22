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
	"testing"
	"time"

	"example.com/pokecalc/services/calc/calctest"
	"example.com/pokecalc/services/gateway/deploytest"
	"example.com/pokecalc/services/gateway/internal/httpapi"
)

const smokeScript = "services/gateway/scripts/smoke.sh"

// stack は gateway の構成の変え方。
type stack struct {
	calcDown       bool // calc の上流を閉じたサーバにする(接続拒否 → 503)
	pokedexAnswers bool // pokedex の上流を 200 を返す偽物にする(スモークは 503 を期待するので落ちるべき)
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

	cfg := httpapi.Config{CalcURL: calcURL, UpstreamTimeout: 5 * time.Second}
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

// AC-S6: calc-svc と gateway がそろっていて pokedex が未設定なら成功する。
func TestSmokeScriptPassesAgainstGatewayAndCalc(t *testing.T) {
	out, err := runSmoke(t, startStack(t, stack{}), "3")
	if err != nil {
		t.Fatalf("smoke.sh が失敗: %v\n%s", err, out)
	}
	want := "api smoke: calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=503 balance=skipped"
	if !strings.Contains(out, want) {
		t.Errorf("smoke.sh の出力に %q が無い:\n%s", want, out)
	}
}

// buildDefaultGatewayHandler は startStack(t, stack{}) と同じ構成(calc-svc は実物、pokedex は未設定)の
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
	h, err := httpapi.NewHandler(httpapi.Config{CalcURL: mustURL(t, calcSrv.URL), UpstreamTimeout: 5 * time.Second})
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
// その値は case の 000/404/502/503 のどれにも一致せず、request_with_retry が「再試行不要な最終ステータス」
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

// AC-S6: 壊れた構成では非ゼロで終わり、ステータスを出す(確認が空振りしない)。
func TestSmokeScriptFailsOnBrokenStack(t *testing.T) {
	tests := []struct {
		name     string
		stack    stack
		wantText string
	}{
		{"calc に届かない(503)", stack{calcDown: true}, "HTTP 503"},
		{"pokedex が答える(503 の確認が効いている)", stack{pokedexAnswers: true}, "/api/pokedex/natures"},
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
