package importer_test

// cronjob.sh の相互排他(issue #106 / ADR-0109)。「取得(Node)→ 上流の検出(Node)→ 照合・投入(Go)」を
// 1本のロックで覆っているかを、実物の tools/importer/cronjob.sh を2プロセス同時に起動して確かめる。
// kubectl・docker・ネットワーク・DB は使わない。sh・flock(busybox 互換で可)があれば make test で走る。
//
// 現時点(spec-writer)では cronjob.sh はロックを持たないため、このテストは失敗する
// (2プロセスとも最後まで処理を進めてしまう = fetch-called-B / check-upstream-called-B / processed-B の
// マーカーが全部でき、TestCronJobLockRejectsConcurrentRun が失敗する)。実装は implementer が
// ADR-0109 のとおり cronjob.sh に行う。
//
// macOS の既定の /bin/sh に flock は入っていない(busybox/util-linux 由来)。ローカルで走らせるには
// 別途 flock を用意する必要があり、無い環境では requireShAndFlock が t.Skip する
// (services/gateway/deploytest/smoke_test.go の curl と同じ流儀)。

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// requireShAndFlock は sh・flock(busybox 互換で可)が無い環境ではスキップする。
func requireShAndFlock(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh が無いので cronjob.sh を流せない")
	}
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skip("flock が無いので排他のテストができない(busybox または util-linux の flock が要る。macOS は既定で無い)")
	}
}

// writeExecutable は実行可能なスクリプトファイルを書く(親ディレクトリも作る)。
func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// stubImportEnv は cronjob.sh を、実 Node・実 pokedex-import バイナリ・実データなしで流すための
// 一時環境(IMPORT_APP_DIR 配下のスタブ一式 + PATH に足す偽の node)を作る。
type stubImportEnv struct {
	markerDir string
	env       []string // IMPORT_TEST_ROLE 以外の環境変数一式
}

// stubStubSleep はスタブ pokedex-import が「処理中」を演じる時間。ロックが実際に
// 「処理中は取れない」ことを検証できるよう、瞬時に終わらせない(ADR-0109 のテスト設計)。
const stubStubSleep = 900 * time.Millisecond

func newStubImportEnv(t *testing.T) stubImportEnv {
	t.Helper()
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "app")
	binDir := filepath.Join(tmp, "bin")
	markerDir := filepath.Join(tmp, "marker")
	homeDir := filepath.Join(tmp, "home")
	for _, d := range []string{appDir, binDir, markerDir, homeDir, filepath.Join(appDir, "data", "generated")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// 偽の node: 引数のスクリプト名(fetch.mjs / check-upstream.mjs)ごとにマーカーを書くだけ。
	// 実際の Node.js が要らないので、このテストの関心(排他)以外の依存を持ち込まない。
	writeExecutable(t, filepath.Join(binDir, "node"), `#!/bin/sh
set -eu
role="${IMPORT_TEST_ROLE:-unknown}"
case "$1" in
  *fetch.mjs) : > "$MARKER_DIR/fetch-called-$role" ;;
  *check-upstream.mjs) : > "$MARKER_DIR/check-upstream-called-$role" ;;
  *) echo "stub node: unexpected script $1" >&2; exit 1 ;;
esac
`)

	// 偽の pokedex-import: 少し(既定 900ms)処理してからマーカーを書く。
	writeExecutable(t, filepath.Join(appDir, "pokedex-import"), `#!/bin/sh
set -eu
role="${IMPORT_TEST_ROLE:-unknown}"
sleep "${IMPORT_TEST_STUB_SLEEP_SECONDS:-0.9}"
: > "$MARKER_DIR/processed-$role"
`)

	writeExecutable(t, filepath.Join(appDir, "tools", "importer", "fetch.mjs"), "// stub: 中身は偽の node が読まないので何でもよい\n")
	writeExecutable(t, filepath.Join(appDir, "tools", "importer", "check-upstream.mjs"), "// stub\n")

	env := []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + homeDir,
		"npm_config_cache=" + filepath.Join(homeDir, "npm-cache"),
		// ADR-0109 §2 のテスト契約: cronjob.sh はこの2つの環境変数で /app とロックファイルの場所を
		// 上書きできること(本番の /app に書き込めないネイティブ Go のテストから差し替えるため)。
		"IMPORT_APP_DIR=" + appDir,
		"IMPORT_LOCK_FILE=" + filepath.Join(appDir, "data", "generated", ".import.lock"),
		"MARKER_DIR=" + markerDir,
	}
	return stubImportEnv{markerDir: markerDir, env: env}
}

func (s stubImportEnv) marker(name string) bool {
	_, err := os.Stat(filepath.Join(s.markerDir, name))
	return err == nil
}

// cronJobScriptPath は実物の tools/importer/cronjob.sh の絶対パス。
// layoutRepoRoot・cronJobScript は cronjob_layout_test.go(同パッケージ)で定義済み。
func cronJobScriptPath() string {
	return filepath.Join(layoutRepoRoot, cronJobScript)
}

// newCronJobCmd は実物の cronjob.sh を role("A"・"B" など)として起動する *exec.Cmd を作る(まだ開始しない)。
func newCronJobCmd(s stubImportEnv, role string) *exec.Cmd {
	cmd := exec.Command("sh", cronJobScriptPath())
	cmd.Env = append(append([]string(nil), s.env...), "IMPORT_TEST_ROLE="+role)
	return cmd
}

// TestCronJobLockRejectsConcurrentRun は、手動実行の Job と CronJob の定期 Job が同時に
// cronjob.sh を起動しても、片方(先着)だけが最後まで処理を進め、もう片方は
// fetch.mjs すら呼ばずに終了コード1で終わることを確かめる(issue #106 の受け入れ条件・ADR-0109)。
func TestCronJobLockRejectsConcurrentRun(t *testing.T) {
	requireShAndFlock(t)
	s := newStubImportEnv(t)

	cmdA := newCronJobCmd(s, "A")
	var outA bytes.Buffer
	cmdA.Stdout, cmdA.Stderr = &outA, &outA
	if err := cmdA.Start(); err != nil {
		t.Fatalf("A(1つ目のプロセス)の起動に失敗: %v", err)
	}

	// A がロックを取ってから(偽の node は瞬時に終わるので数十msで十分)B を起動する。
	// 偽の pokedex-import は 900ms 処理するので、この程度の遅延では A はまだロックを保持している。
	time.Sleep(300 * time.Millisecond)

	cmdB := newCronJobCmd(s, "B")
	startB := time.Now()
	outB, errB := cmdB.CombinedOutput()
	durationB := time.Since(startB)

	if errB == nil {
		t.Fatalf("B(2つ目のプロセス)はロックを取れず終了コード1で終わるべき(実際は成功): 出力=%s", outB)
	}
	exitErr, ok := errB.(*exec.ExitError)
	if !ok {
		t.Fatalf("B の起動自体に失敗した(exec のエラー。sh/cronjob.sh を確認): %v", errB)
	}
	if got := exitErr.ExitCode(); got != 1 {
		t.Errorf("B の終了コード = %d, want 1(ADR-0104 §3「再試行で直りうる失敗」・ADR-0109 §4): 出力=%s", got, outB)
	}
	// ロックが取れなければ即座に諦めるべきで、偽の pokedex-import の sleep(900ms)を待ってはいけない。
	if durationB >= stubStubSleep-200*time.Millisecond {
		t.Errorf("B は非ブロッキングで即座に諦めるべき(実際は %s かかった。flock -n を使っているか確認)", durationB)
	}

	if s.marker("fetch-called-B") {
		t.Error("B は fetch.mjs を呼んではいけない(ロックを取れなかったら取得キャッシュに触る前に諦める。issue #106 の受け入れ条件)")
	}
	if s.marker("check-upstream-called-B") {
		t.Error("B は check-upstream.mjs を呼んではいけない")
	}
	if s.marker("processed-B") {
		t.Error("B は pokedex-import を呼んではいけない(DB へ投入しない)")
	}

	if err := cmdA.Wait(); err != nil {
		t.Fatalf("A(1つ目のプロセス)は最後まで成功するべき: %v, 出力=%s", err, outA.String())
	}
	if !s.marker("fetch-called-A") {
		t.Error("A は fetch.mjs まで進むべき")
	}
	if !s.marker("check-upstream-called-A") {
		t.Error("A は check-upstream.mjs まで進むべき")
	}
	if !s.marker("processed-A") {
		t.Error("A は pokedex-import まで進むべき")
	}
}

// TestCronJobLockReleasedAfterExit は、1つ目のプロセスが終わった後、ロックファイルの中身・存在が
// stale lock として残らず、次のプロセスが正常にロックを取得できることを確かめる
// (issue #106 の受け入れ条件「恒久的な stale lock を残さない」・ADR-0109 §3)。
func TestCronJobLockReleasedAfterExit(t *testing.T) {
	requireShAndFlock(t)
	s := newStubImportEnv(t)

	cmdA := newCronJobCmd(s, "A")
	outA, errA := cmdA.CombinedOutput()
	if errA != nil {
		t.Fatalf("A(1回目)は成功するべき: %v, 出力=%s", errA, outA)
	}
	if !s.marker("processed-A") {
		t.Fatal("A(1回目)が pokedex-import まで進んでいない(前提が崩れている。テストの構成を見直す)")
	}

	// A が終わった直後に B を起動する。ロックファイル自体は PVC 上に残るが、
	// その中身・存在が次の取得を妨げてはいけない。
	cmdB := newCronJobCmd(s, "B")
	outB, errB := cmdB.CombinedOutput()
	if errB != nil {
		t.Fatalf("B(A の終了後の2回目)はロックを取得できて成功するべき(stale lock になっている): %v, 出力=%s", errB, outB)
	}
	if !s.marker("fetch-called-B") || !s.marker("check-upstream-called-B") || !s.marker("processed-B") {
		t.Error("B は A の終了後、fetch → check-upstream → pokedex-import まで最後まで進められるべき")
	}
}
