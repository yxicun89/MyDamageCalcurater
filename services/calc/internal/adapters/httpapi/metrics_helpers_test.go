package httpapi

// /metrics(Prometheus text format)をテストから読むための最小の道具(ADR-0406。P7-1)。
// 6サービス(gateway・pokedex・calc・balance・speed・judge)の httpapi に同じ内容を複製している
// (サービスごとにテストパッケージが違い、balance・speed・judge は独立 go.mod のため。ADR-0406 §2)。

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	metricsPath            = "/metrics"
	metricsRequestsTotal   = "http_requests_total"
	metricsDurationSeconds = "http_request_duration_seconds"
)

type metricsSample struct {
	name   string
	labels map[string]string
	value  float64
}

type metricsRequest struct {
	method string
	target string
	header http.Header
	body   string
}

var (
	metricsSampleLine = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+(\S+)`)
	metricsLabelPair  = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"`)
)

func metricsDo(h http.Handler, method, target string, header http.Header, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// metricsScrape は GET /metrics を(端末ID・セッションIDのヘッダ無しで)呼び、200・text/plain であることを確かめて本文を返す。
func metricsScrape(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := metricsDo(h, http.MethodGet, metricsPath, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200; body=%s", metricsPath, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("GET %s: Content-Type = %q, want text/plain で始まる(Prometheus text format)", metricsPath, ct)
	}
	return rec.Body.String()
}

func metricsParse(t *testing.T, body string) []metricsSample {
	t.Helper()
	var out []metricsSample
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := metricsSampleLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("text format として読めない行: %q", line)
		}
		v, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			t.Fatalf("値を読めない行: %q: %v", line, err)
		}
		labels := map[string]string{}
		for _, p := range metricsLabelPair.FindAllStringSubmatch(m[2], -1) {
			labels[p[1]] = p[2]
		}
		out = append(out, metricsSample{name: m[1], labels: labels, value: v})
	}
	return out
}

// metricsSum は name のサンプルのうち subset のラベルをすべて持つものの合計。
func metricsSum(samples []metricsSample, name string, subset map[string]string) float64 {
	total := 0.0
	for _, s := range samples {
		if s.name != name {
			continue
		}
		match := true
		for k, v := range subset {
			if s.labels[k] != v {
				match = false
				break
			}
		}
		if match {
			total += s.value
		}
	}
	return total
}

// assertMetricsFormat は2つの指標の # HELP・# TYPE 行があり、全行が text format として読めることを確かめる。
func assertMetricsFormat(t *testing.T, body string) {
	t.Helper()
	for _, want := range []string{
		"# HELP " + metricsRequestsTotal + " ",
		"# TYPE " + metricsRequestsTotal + " counter",
		"# HELP " + metricsDurationSeconds + " ",
		"# TYPE " + metricsDurationSeconds + " histogram",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics の本文に %q が無い; body=\n%s", want, body)
		}
	}
	metricsParse(t, body)
}

// assertCountedOnce は req を1回送ると http_requests_total{method,path=wantPath,status=実際の応答}
// と http_request_duration_seconds_count{method,path=wantPath} がちょうど1増えることを確かめる。
// 実際に来たパス(req.target)が wantPath と違う場合は、生のパスがどのラベルにも入らないことも確かめる。
func assertCountedOnce(t *testing.T, h http.Handler, req metricsRequest, wantPath string) *httptest.ResponseRecorder {
	t.Helper()
	before := metricsParse(t, metricsScrape(t, h))
	rec := metricsDo(h, req.method, req.target, req.header, req.body)
	after := metricsParse(t, metricsScrape(t, h))

	counter := map[string]string{"method": req.method, "path": wantPath, "status": strconv.Itoa(rec.Code)}
	if d := metricsSum(after, metricsRequestsTotal, counter) - metricsSum(before, metricsRequestsTotal, counter); d != 1 {
		t.Errorf("%s %s(応答 %d)の後 %s%v の増分 = %v, want 1", req.method, req.target, rec.Code, metricsRequestsTotal, counter, d)
	}
	histogram := map[string]string{"method": req.method, "path": wantPath}
	name := metricsDurationSeconds + "_count"
	if d := metricsSum(after, name, histogram) - metricsSum(before, name, histogram); d != 1 {
		t.Errorf("%s %s の後 %s%v の増分 = %v, want 1", req.method, req.target, name, histogram, d)
	}
	if rawPath := strings.SplitN(req.target, "?", 2)[0]; rawPath != wantPath {
		// 生のパスの最後のセグメント(未知の ID 等)がどのラベル値にも入っていないこと。
		rawSegment := rawPath[strings.LastIndex(rawPath, "/")+1:]
		for _, s := range after {
			for k, v := range s.labels {
				if rawSegment != "" && strings.Contains(v, rawSegment) {
					t.Errorf("%s{%s=%q}: 実際に来たパスの一部 %q がラベルに入っている(ルーティングパターン %q を使うこと)", s.name, k, v, rawSegment, wantPath)
				}
			}
		}
	}
	return rec
}

// assertMetricsNotSelfCounted は /metrics を何度呼んでも指標の合計が変わらず、path="/metrics" のサンプルが無いことを確かめる。
func assertMetricsNotSelfCounted(t *testing.T, h http.Handler) {
	t.Helper()
	before := metricsParse(t, metricsScrape(t, h))
	for range 3 {
		metricsScrape(t, h)
	}
	after := metricsParse(t, metricsScrape(t, h))
	for _, name := range []string{metricsRequestsTotal, metricsDurationSeconds + "_count"} {
		if b, a := metricsSum(before, name, nil), metricsSum(after, name, nil); a != b {
			t.Errorf("%s の合計: /metrics を呼ぶ前 = %v, 後 = %v(/metrics を数えてはいけない)", name, b, a)
		}
	}
	for _, s := range after {
		if s.labels["path"] == metricsPath {
			t.Errorf("%s に path=%q のサンプルがある(自己参照)", s.name, metricsPath)
		}
	}
}
