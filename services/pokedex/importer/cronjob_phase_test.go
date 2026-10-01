package importer_test

// issue #301(D19)・ADR-0101 追記: cronjob.sh を「取得段(initContainer fetch)」と「投入段(import)」の
// 2つのフェーズに分けたときの動作を、実物の tools/importer/cronjob.sh と偽の node・pokedex-import で確かめる。
// 契約:
//   - 引数 `fetch`: flock -n → 容量の確認(prune.mjs check)→ fetch.mjs → check-upstream.mjs → 引き渡しファイルを書く。
//     pokedex-import は呼ばない(DSN を持たないコンテナで動く)。
//   - 引数 `import`: flock -n → 引き渡しファイルの所有者が自分であることを確認 → pokedex-import → prune.mjs prune
//     → 引き渡しファイルを消す(成功・失敗のどちらでも)。fetch.mjs・check-upstream.mjs は呼ばない。
//   - 引数なし(従来どおり。make dev・既存のロックのテスト): 1つのプロセスで全工程。ロックを持ったまま通す。
//   - 引き渡しファイル(IMPORT_HANDOFF_FILE。既定 <APP_DIR>/data/generated/.import.handoff)は2行
//     `owner=<Pod 名>` と `expires=<epoch 秒>`。所有者の Pod 名は環境変数 HOSTNAME。有効期限は
//     IMPORT_HANDOFF_TTL_SECONDS(既定 7200 = activeDeadlineSeconds の2倍)。
//     flock は2つのコンテナの間(initContainer の終了から main の開始まで)で解放されるため、その隙間を
//     この引き渡しファイルで埋め、手動 Job と定期 Job の同時実行を防ぐ。有効期限が切れた引き渡しは
//     SIGKILL で残った stale として無視してよい(恒久的な stale lock を残さない。ADR-0109 §3 と同じ要請)。
//   - 他の Pod の有効な引き渡しがあるとき、どのフェーズ(引数なしを含む)も fetch.mjs・pokedex-import を呼ばず、
//     終了コード1(再試行で直りうる失敗。ADR-0109 §4)で諦める。
//   - 終了コードはそのまま通す(fetch.mjs の 3 = ハッシュ不一致、pokedex-import の 3 = ID 消滅)。不明なフェーズは 2。
//   - IMPORT_ALLOW_REMOVED(ADR-0131)は import フェーズの pokedex-import に -allow-removed として渡す(意味は不変)。

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type phaseEnv struct {
	stubImportEnv
	handoff string
	orderLn string
}

// newPhaseEnv は newStubImportEnv の偽の node・pokedex-import を、呼び出し順を order.log に残す版へ差し替える。
func newPhaseEnv(t *testing.T) phaseEnv {
	t.Helper()
	s := newStubImportEnv(t)
	appDir := ""
	for _, e := range s.env {
		if strings.HasPrefix(e, "IMPORT_APP_DIR=") {
			appDir = strings.TrimPrefix(e, "IMPORT_APP_DIR=")
		}
	}
	binDir := ""
	for _, e := range s.env {
		if strings.HasPrefix(e, "PATH=") {
			binDir = strings.SplitN(strings.TrimPrefix(e, "PATH="), string(os.PathListSeparator), 2)[0]
		}
	}
	writeExecutable(t, filepath.Join(binDir, "node"), `#!/bin/sh
set -eu
script="$(basename "$1")"
case "$script" in
  prune.mjs) echo "prune-$2" >> "$MARKER_DIR/order.log" ;;
  fetch.mjs) echo "fetch" >> "$MARKER_DIR/order.log"; sleep "${IMPORT_TEST_FETCH_SLEEP_SECONDS:-0}"; exit "${IMPORT_TEST_FETCH_EXIT:-0}" ;;
  check-upstream.mjs) echo "check-upstream" >> "$MARKER_DIR/order.log" ;;
  *) echo "stub node: unexpected script $1" >&2; exit 1 ;;
esac
`)
	writeExecutable(t, filepath.Join(appDir, "pokedex-import"), `#!/bin/sh
set -eu
echo "pokedex-import $*" >> "$MARKER_DIR/order.log"
sleep "${IMPORT_TEST_STUB_SLEEP_SECONDS:-0}"
exit "${IMPORT_TEST_IMPORT_EXIT:-0}"
`)
	return phaseEnv{
		stubImportEnv: s,
		handoff:       filepath.Join(appDir, "data", "generated", ".import.handoff"),
		orderLn:       filepath.Join(s.markerDir, "order.log"),
	}
}

// run は cronjob.sh を pod(HOSTNAME)・phase("" なら引数なし)で流して、終了コードと出力を返す。
func (p phaseEnv) run(t *testing.T, pod, phase string, extraEnv ...string) (int, string) {
	t.Helper()
	return runCmd(p.cmd(pod, phase, extraEnv...))
}

func (p phaseEnv) cmd(pod, phase string, extraEnv ...string) *exec.Cmd {
	args := []string{cronJobScriptPath()}
	if phase != "" {
		args = append(args, phase)
	}
	cmd := exec.Command("sh", args...)
	cmd.Env = append(append([]string(nil), p.env...), "HOSTNAME="+pod)
	cmd.Env = append(cmd.Env, extraEnv...)
	return cmd
}

func runCmd(cmd *exec.Cmd) (int, string) {
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), string(out)
	}
	return -1, err.Error() + string(out)
}

func (p phaseEnv) lines() []string {
	b, err := os.ReadFile(p.orderLn)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func (p phaseEnv) handoffExists() bool {
	_, err := os.Stat(p.handoff)
	return err == nil
}

func (p phaseEnv) handoffOwner(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(p.handoff)
	if err != nil {
		t.Fatalf("引き渡しファイルを読めない: %v", err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "owner="); ok {
			return v
		}
	}
	t.Fatalf("引き渡しファイルに owner= が無い: %q", b)
	return ""
}

func wantLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("呼び出し順 = %q, want %q", got, want)
	}
}

func TestPhaseFetchRunsFetchStepsOnlyAndHandsOff(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	code, out := p.run(t, "pod-a", "fetch")
	if code != 0 {
		t.Fatalf("fetch フェーズは成功するべき: code=%d out=%s", code, out)
	}
	// 容量の確認(download の前)→ 取得 → 上流の検出。DB に触る pokedex-import は呼ばない。
	wantLines(t, p.lines(), "prune-check", "fetch", "check-upstream")
	if !p.handoffExists() || p.handoffOwner(t) != "pod-a" {
		t.Error("fetch フェーズは成功したら引き渡しファイルに owner=自分(HOSTNAME)を書く")
	}
}

func TestPhaseImportRunsApplyAndPruneAndReleasesHandoff(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	if code, out := p.run(t, "pod-a", "fetch"); code != 0 {
		t.Fatalf("前提の fetch フェーズが失敗: %d %s", code, out)
	}
	code, out := p.run(t, "pod-a", "import")
	if code != 0 {
		t.Fatalf("import フェーズは成功するべき: code=%d out=%s", code, out)
	}
	got := p.lines()
	if len(got) != 5 || !strings.HasPrefix(got[3], "pokedex-import ") || got[4] != "prune-prune" {
		t.Errorf("import フェーズは pokedex-import → prune prune の順で、fetch.mjs・check-upstream・prune check を呼ばない: %q", got)
	}
	if !strings.Contains(got[3], "-upstream") || strings.Contains(got[3], "-allow-removed") {
		t.Errorf("pokedex-import に -upstream を渡し、IMPORT_ALLOW_REMOVED が無ければ -allow-removed を付けない: %q", got[3])
	}
	if p.handoffExists() {
		t.Error("import フェーズが終わったら引き渡しファイルを消す(次の実行を妨げない)")
	}
}

// ADR-0131: ID の消滅を人が承認する手動 Job の IMPORT_ALLOW_REMOVED は import フェーズだけが使う。
func TestPhaseImportPassesAllowRemoved(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	p.run(t, "pod-a", "fetch")
	code, out := p.run(t, "pod-a", "import", "IMPORT_ALLOW_REMOVED=species:9002-002,move:teststrike")
	if code != 0 {
		t.Fatalf("code=%d out=%s", code, out)
	}
	if got := p.lines(); len(got) < 4 || !strings.Contains(got[3], "-allow-removed species:9002-002,move:teststrike") {
		t.Errorf("import フェーズの pokedex-import に -allow-removed を渡す: %q", got)
	}
}

func TestPhaseImportFailureKeepsExitCodeAndReleasesHandoff(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	p.run(t, "pod-a", "fetch")
	code, out := p.run(t, "pod-a", "import", "IMPORT_TEST_IMPORT_EXIT=3")
	if code != 3 {
		t.Errorf("pokedex-import の終了コード 3(ID の消滅)をそのまま返す: code=%d out=%s", code, out)
	}
	for _, l := range p.lines() {
		if l == "prune-prune" {
			t.Error("pokedex-import が失敗したら prune に進まない")
		}
	}
	if p.handoffExists() {
		t.Error("失敗でも引き渡しファイルを消す(再試行・手動 Job が有効期限まで待たされない)")
	}
}

func TestPhaseFetchFailureKeepsExitCodeAndWritesNoHandoff(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	code, out := p.run(t, "pod-a", "fetch", "IMPORT_TEST_FETCH_EXIT=3")
	if code != 3 {
		t.Errorf("fetch.mjs の終了コード 3(期待ハッシュの不一致)をそのまま返す: code=%d out=%s", code, out)
	}
	if p.handoffExists() {
		t.Error("取得に失敗したら引き渡しファイルを書かない")
	}
	for _, l := range p.lines() {
		if l == "check-upstream" {
			t.Error("fetch.mjs が失敗したら check-upstream に進まない")
		}
	}
}

// 引き渡し中(fetch が終わり import の開始を待つ間・import の実行中)は、別の Pod は何も始められない。
func TestPhaseHandoffBlocksOtherPods(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	if code, out := p.run(t, "pod-a", "fetch"); code != 0 {
		t.Fatalf("前提: %d %s", code, out)
	}
	before := len(p.lines())
	for _, phase := range []string{"fetch", "import", ""} {
		code, out := p.run(t, "pod-b", phase)
		if code != 1 {
			t.Errorf("A の引き渡し中、B のフェーズ %q は終了コード1で諦める: code=%d out=%s", phase, code, out)
		}
	}
	if got := len(p.lines()); got != before {
		t.Errorf("B は fetch.mjs・pokedex-import などを1つも呼ばない: 追加された呼び出し=%q", p.lines()[before:])
	}
	if p.handoffOwner(t) != "pod-a" {
		t.Error("B は A の引き渡しファイルを書き換えない")
	}
	// A はそのまま import を続けられる。
	if code, out := p.run(t, "pod-a", "import"); code != 0 {
		t.Errorf("A の import は続けられる: %d %s", code, out)
	}
	// A が終われば B は取得を始められる。
	if code, out := p.run(t, "pod-b", "fetch"); code != 0 {
		t.Errorf("A の終了後、B の fetch は成功する(stale を残さない): %d %s", code, out)
	}
}

func TestPhaseImportRequiresOwnHandoff(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	// 引き渡しが無い(fetch が走っていない・消えた)なら投入しない。
	if code, out := p.run(t, "pod-a", "import"); code != 1 {
		t.Errorf("引き渡しファイルが無いとき import は終了コード1で諦める: code=%d out=%s", code, out)
	}
	// 他の Pod の引き渡しのときも投入しない。
	p.run(t, "pod-a", "fetch")
	if code, out := p.run(t, "pod-b", "import"); code != 1 {
		t.Errorf("他の Pod の引き渡しのとき import は終了コード1で諦める: code=%d out=%s", code, out)
	}
	for _, l := range p.lines() {
		if strings.HasPrefix(l, "pokedex-import") {
			t.Errorf("pokedex-import を呼んではいけない: %q", p.lines())
		}
	}
}

// 有効期限切れの引き渡し(Pod が SIGKILL された等)は stale として無視し、取得を始められる。
func TestPhaseExpiredHandoffIsIgnored(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	past := strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	if err := os.WriteFile(p.handoff, []byte("owner=pod-dead\nexpires="+past+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := p.run(t, "pod-b", "fetch"); code != 0 {
		t.Fatalf("期限切れの引き渡しは無視して取得できる: code=%d out=%s", code, out)
	}
	if p.handoffOwner(t) != "pod-b" {
		t.Error("期限切れを上書きして owner=自分にする")
	}
	// 期限内なら無視しない。
	future := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	if err := os.WriteFile(p.handoff, []byte("owner=pod-live\nexpires="+future+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _ := p.run(t, "pod-b", "fetch"); code != 1 {
		t.Error("期限内の他の Pod の引き渡しは尊重する(終了コード1)")
	}
}

// import フェーズの実行中に手動 Job の fetch が始まっても、fetch.mjs(取得キャッシュへの書き込み)を呼ばない。
func TestPhaseConcurrentFetchDuringImportIsRejected(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	if code, out := p.run(t, "pod-a", "fetch"); code != 0 {
		t.Fatalf("前提: %d %s", code, out)
	}
	a := p.cmd("pod-a", "import", "IMPORT_TEST_STUB_SLEEP_SECONDS=0.9")
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	code, out := p.run(t, "pod-b", "fetch")
	if code != 1 {
		t.Errorf("A の投入中、B の fetch は終了コード1: code=%d out=%s", code, out)
	}
	if time.Since(start) > 600*time.Millisecond {
		t.Error("B は待たずに即座に諦める(flock -n)")
	}
	if err := a.Wait(); err != nil {
		t.Fatalf("A の import は成功する: %v", err)
	}
	fetches := 0
	for _, l := range p.lines() {
		if l == "fetch" {
			fetches++
		}
	}
	if fetches != 1 {
		t.Errorf("fetch.mjs は A の1回だけ(B は呼ばない): %d 回", fetches)
	}
}

func TestPhaseUnknownIsUsageError(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	if code, out := p.run(t, "pod-a", "bogus"); code != 2 {
		t.Errorf("不明なフェーズは終了コード2(使い方の誤り): code=%d out=%s", code, out)
	}
	if len(p.lines()) != 0 {
		t.Errorf("何も呼ばない: %q", p.lines())
	}
}

// 引数なし(従来)は1プロセスで全工程を通し、引き渡しファイルを残さない。
func TestPhaseDefaultRunsEverythingWithoutHandoff(t *testing.T) {
	requireShAndFlock(t)
	p := newPhaseEnv(t)
	if code, out := p.run(t, "pod-a", ""); code != 0 {
		t.Fatalf("引数なしは従来どおり成功: %d %s", code, out)
	}
	got := p.lines()
	if len(got) != 5 || got[0] != "prune-check" || got[1] != "fetch" || got[2] != "check-upstream" ||
		!strings.HasPrefix(got[3], "pokedex-import ") || got[4] != "prune-prune" {
		t.Errorf("引数なしの順序は 容量確認 → 取得 → 上流の検出 → 投入 → prune: %q", got)
	}
	if p.handoffExists() {
		t.Error("引数なしでは引き渡しファイルを残さない")
	}
}
