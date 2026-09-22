package main

// 投入の CLI のモードと終了コード(ADR-0104 §3・§4・§9)。DB は偽の Store、上流の検出はファイルで差し替える。
// ネットワーク・DB に触らない(make test で走る)。

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

type fakeStore struct {
	applied    []importer.SourceVersion
	appliedErr error
	applyErr   error
	applyCalls [][]importer.SourceVersion
}

func (f *fakeStore) AppliedVersions(context.Context) ([]importer.SourceVersion, error) {
	if f.appliedErr != nil {
		return nil, f.appliedErr
	}
	return append([]importer.SourceVersion(nil), f.applied...), nil
}

func (f *fakeStore) Apply(_ context.Context, _ importer.Output, versions []importer.SourceVersion, _ time.Time) error {
	f.applyCalls = append(f.applyCalls, append([]importer.SourceVersion(nil), versions...))
	return f.applyErr
}

type nopCloser struct{ closed *bool }

func (c nopCloser) Close() error { *c.closed = true; return nil }

// harness は run に渡す環境と、その観測結果。
type harness struct {
	store     *fakeStore
	openErr   error
	openCalls int
	openedDSN string
	closed    bool
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	env       map[string]string
	now       time.Time
}

func newHarness() *harness {
	return &harness{
		store: &fakeStore{},
		env:   map[string]string{"POKEDEX_DATABASE_DSN": "u:p@tcp(127.0.0.1:1)/pokedex_fake"},
		now:   time.Date(2026, 9, 26, 3, 30, 0, 0, time.UTC),
	}
}

func (h *harness) cliEnv() cliEnv {
	return cliEnv{
		Stdout: &h.stdout,
		Stderr: &h.stderr,
		Getenv: func(k string) string { return h.env[k] },
		OpenStore: func(dsn string) (importer.Store, io.Closer, error) {
			h.openCalls++
			h.openedDSN = dsn
			if h.openErr != nil {
				return nil, nil, h.openErr
			}
			return h.store, nopCloser{&h.closed}, nil
		},
		Now: func() time.Time { return h.now },
	}
}

func (h *harness) run(t *testing.T, args ...string) int {
	t.Helper()
	code := run(args, h.cliEnv())
	t.Logf("exit=%d\nstdout:\n%s\nstderr:\n%s", code, h.stdout.String(), h.stderr.String())
	return code
}

// pinnedVersions は data ディレクトリの固定版(LoadInput の結果)を返す。
func pinnedVersions(t *testing.T, data string) []importer.SourceVersion {
	t.Helper()
	_, vs, err := importer.LoadInput(data)
	if err != nil {
		t.Fatalf("LoadInput: %v", err)
	}
	return vs
}

func versionOf(vs []importer.SourceVersion, source string) string {
	for _, v := range vs {
		if v.Source == source {
			return v.Version
		}
	}
	return ""
}

func writeUpstream(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "generated", "upstream", "latest.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const newerShowdown = "0123456789abcdef0123456789abcdef01234567"

func upstreamBody(checkedAt, showdown string) string {
	return `{"schemaVersion":1,"checkedAt":"` + checkedAt + `","sources":{"calc":"` + fixtureCalcVersion +
		`","showdown":"` + showdown + `","pokeapi":"` + fixturePokeAPIVersion + `"}}`
}

// --- 版に変化が無ければ取り込まない(ADR-0104 §1) -------------------------------------

func TestRunSkipsWhenDBHasPinnedVersions(t *testing.T) {
	data := copyFixtureData(t, 2)
	h := newHarness()
	h.store.applied = pinnedVersions(t, data)
	if code := h.run(t, "-data", data); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(h.store.applyCalls) != 0 {
		t.Errorf("DB が固定版と同じなのに Apply を %d 回呼んだ", len(h.store.applyCalls))
	}
	if h.openedDSN != h.env["POKEDEX_DATABASE_DSN"] {
		t.Errorf("OpenStore に渡った DSN = %q", h.openedDSN)
	}
	if !h.closed {
		t.Error("Store を閉じていない")
	}
}

func TestRunAppliesPinnedVersionsWhenDBDiffers(t *testing.T) {
	data := copyFixtureData(t, 2)
	h := newHarness()
	if code := h.run(t, "-data", data); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(h.store.applyCalls) != 1 {
		t.Fatalf("DB が空なのに Apply が %d 回(want 1)", len(h.store.applyCalls))
	}
	if needs, _ := importer.NeedsImport(h.store.applyCalls[0], pinnedVersions(t, data)); needs {
		t.Errorf("Apply に渡った版が固定版と違う: %+v", h.store.applyCalls[0])
	}
}

func TestRunPrintsSummary(t *testing.T) {
	data := copyFixtureData(t, 2)
	in, _, err := importer.LoadInput(data)
	if err != nil {
		t.Fatal(err)
	}
	_, rec, err := importer.Reconcile(in)
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness()
	if code := h.run(t, "-data", data, "-dry-run"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	summary := strings.TrimSpace(importer.FormatSummary(rec))
	if summary == "" || !strings.Contains(h.stdout.String(), summary) {
		t.Errorf("stdout に照合の要約(FormatSummary)が無い(kubectl logs で読むため)")
	}
}

// --- 上流の検出は報告だけ(ADR-0104 §1・§4) --------------------------------------------

func TestRunUpstreamDiffersIsReportOnly(t *testing.T) {
	data := copyFixtureData(t, 2)
	cfgPath := filepath.Join(data, "importer", "config.json")
	cfgBefore, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	up := writeUpstream(t, data, upstreamBody("2026-09-26T03:00:00Z", newerShowdown))

	h := newHarness()
	if code := h.run(t, "-data", data, "-upstream", up); code != 0 {
		t.Fatalf("上流に違う版があっても終了コードは 0(報告だけ): %d", code)
	}
	if len(h.store.applyCalls) != 1 {
		t.Fatalf("Apply が %d 回(want 1)", len(h.store.applyCalls))
	}
	if got := versionOf(h.store.applyCalls[0], "showdown"); got != fixtureShowdownVersion {
		t.Errorf("投入した showdown の版 = %q, want 固定版 %q(上流の版を取り込まない)", got, fixtureShowdownVersion)
	}
	cfgAfter, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cfgBefore, cfgAfter) {
		t.Error("config.json を書き換えた(版を上げるのは人の PR)")
	}
	out := h.stdout.String()
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "UPSTREAM") && strings.Contains(line, "showdown") &&
			strings.Contains(line, newerShowdown) && strings.Contains(line, fixtureShowdownVersion) {
			found = true
		}
	}
	if !found {
		t.Errorf("stdout に showdown の UPSTREAM の行(固定版と上流の版)が無い")
	}
}

func TestRunUpstreamShownEvenWhenSkipping(t *testing.T) {
	data := copyFixtureData(t, 2)
	up := writeUpstream(t, data, upstreamBody("2026-09-26T03:00:00Z", newerShowdown))
	h := newHarness()
	h.store.applied = pinnedVersions(t, data)
	if code := h.run(t, "-data", data, "-upstream", up); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(h.store.applyCalls) != 0 {
		t.Error("版が同じなのに上流の検出を理由に投入した")
	}
	if !strings.Contains(h.stdout.String(), newerShowdown) {
		t.Error("版が同じでスキップするときも上流の検出結果を表示すること")
	}
}

func TestRunUpstreamProblemsDoNotBlockImport(t *testing.T) {
	tests := []struct {
		name string
		body string // "" = ファイルを作らない
	}{
		{"ファイルが無い", ""},
		{"壊れている", `{"schemaVersion":1,`},
		{"古い(前回の結果の使い回し)", upstreamBody("2026-09-01T03:00:00Z", newerShowdown)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := copyFixtureData(t, 2)
			up := filepath.Join(data, "generated", "upstream", "latest.json")
			if tt.body != "" {
				up = writeUpstream(t, data, tt.body)
			}
			h := newHarness()
			if code := h.run(t, "-data", data, "-upstream", up); code != 0 {
				t.Fatalf("上流の検出の問題で終了コードが %d(0 のまま取り込みを続けること)", code)
			}
			if len(h.store.applyCalls) != 1 {
				t.Errorf("上流の検出の問題で投入を止めた(Apply %d 回)", len(h.store.applyCalls))
			}
			all := h.stdout.String() + h.stderr.String()
			if strings.Contains(all, "UPSTREAM") && strings.Contains(all, newerShowdown) {
				t.Error("読めない/古い検出結果を differs として表示した")
			}
		})
	}
}

// --- 失敗の扱い・終了コード(ADR-0104 §3・§9) ------------------------------------------

func TestRunBlockedExitsNeedsHumanWithoutTouchingDB(t *testing.T) {
	data := copyFixtureData(t, 3) // 裁定の件数が fixture と食い違う → 照合の Blocker
	h := newHarness()
	if code := h.run(t, "-data", data); code != 3 {
		t.Fatalf("exit = %d, want 3(人の対応が要る)", code)
	}
	if h.openCalls != 0 {
		t.Errorf("照合で止まったのに DB を開いた(%d 回)", h.openCalls)
	}
	if _, err := os.Stat(filepath.Join(data, "generated", "reports", "latest.json")); err != nil {
		t.Errorf("止まったときも報告を書くこと: %v", err)
	}
}

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(h *harness)
		args       []string
		want       int
		wantOpen   bool
		applyCalls int
	}{
		{name: "migrate が済んでいない", setup: func(h *harness) { h.store.appliedErr = importer.ErrSchemaNotReady }, want: 3, wantOpen: true},
		{name: "key が変わった(ErrKeyChanged)", setup: func(h *harness) { h.store.applyErr = importer.ErrKeyChanged }, want: 3, wantOpen: true, applyCalls: 1},
		{name: "DB に接続できない", setup: func(h *harness) { h.openErr = errors.New("dial tcp: connection refused") }, want: 1, wantOpen: true},
		{name: "版を読めない一時的な失敗", setup: func(h *harness) { h.store.appliedErr = errors.New("driver: bad connection") }, want: 1, wantOpen: true},
		{name: "DSN が無い", setup: func(h *harness) { delete(h.env, "POKEDEX_DATABASE_DSN") }, want: 2},
		{name: "未知のフラグ", args: []string{"-no-such-flag"}, want: 2},
		{name: "dry-run は DB を開かない", args: []string{"-dry-run"}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := copyFixtureData(t, 2)
			h := newHarness()
			if tt.setup != nil {
				tt.setup(h)
			}
			args := append([]string{"-data", data}, tt.args...)
			if code := h.run(t, args...); code != tt.want {
				t.Fatalf("exit = %d, want %d", code, tt.want)
			}
			if (h.openCalls > 0) != tt.wantOpen {
				t.Errorf("OpenStore の呼び出し %d 回(want 呼ぶ=%v)", h.openCalls, tt.wantOpen)
			}
			if len(h.store.applyCalls) != tt.applyCalls {
				t.Errorf("Apply の呼び出し %d 回, want %d", len(h.store.applyCalls), tt.applyCalls)
			}
		})
	}
}

func TestRunInvalidInputExitsNeedsHuman(t *testing.T) {
	data := copyFixtureData(t, 2)
	if err := os.Remove(filepath.Join(data, "importer", "effects.json")); err != nil {
		t.Fatal(err)
	}
	h := newHarness()
	if code := h.run(t, "-data", data); code != 3 {
		t.Fatalf("exit = %d, want 3(入力の不正は再試行しても同じ)", code)
	}
	if h.openCalls != 0 {
		t.Error("入力が不正なのに DB を開いた")
	}
}

func TestRunForceAppliesEvenWhenSame(t *testing.T) {
	data := copyFixtureData(t, 2)
	h := newHarness()
	h.store.applied = pinnedVersions(t, data)
	if code := h.run(t, "-data", data, "-force"); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if len(h.store.applyCalls) != 1 {
		t.Errorf("-force で Apply が %d 回(want 1)", len(h.store.applyCalls))
	}
}
