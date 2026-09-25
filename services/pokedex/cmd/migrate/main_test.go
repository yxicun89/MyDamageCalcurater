package main

// migrate CLI の up のプロビジョニング分岐(ADR-0110 §3・§7)。DB は偽の関数で差し替え、
// 実 DB に触らない(make test で走る)。実際の権限は services/pokedex/db/grants_mysql_test.go。
//
// run のシグネチャ(実装への契約):
//
//	type cliEnv struct {
//		Stdout, Stderr io.Writer
//		Getenv         func(string) string
//		Provision      func(rootDSN string, roles []db.RoleGrant) error
//		Up             func(dsn string) error
//		Version        func(dsn string) (version uint, dirty bool, ok bool, err error)
//		DownAll        func(dsn, confirmDatabase string) error
//		Force          func(dsn, confirmDatabase string, version int) error
//	}
//	func run(args []string, env cliEnv) int
//
// main は os.Getenv と db.Provision・db.Up・db.Version・db.DownAll・db.Force を渡す。

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/db"
)

// 架空の DSN(パスワードは出力に漏れていないかを探す目印)。
const (
	fakeProvisionDSN = "root:provPW0000000000000000@tcp(127.0.0.1:1)/pokedex_fake"
	fakeReaderDSN    = "pokedex_reader:readPW0000000000000000@tcp(127.0.0.1:1)/pokedex_fake"
	fakeImporterDSN  = "pokedex_importer:impoPW0000000000000000@tcp(127.0.0.1:1)/pokedex_fake"
	fakeMigratorDSN  = "pokedex_migrator:migrPW0000000000000000@tcp(127.0.0.1:1)/pokedex_fake"
)

var fakePWs = []string{"provPW0000000000000000", "readPW0000000000000000", "impoPW0000000000000000", "migrPW0000000000000000"}

type harness struct {
	env    map[string]string
	stdout bytes.Buffer
	stderr bytes.Buffer

	calls          []string // "provision" / "up" / "version" / "down" / "force" の呼ばれた順
	provisionRoot  string
	provisionRoles []db.RoleGrant
	// provisionRolesByCall は Provision の呼び出しごとのロール(Up の前後で2回呼ばれる)。
	provisionRolesByCall [][]db.RoleGrant
	upDSN                string
	provisionErr         error
	upErr                error
	forceDSN             string
	forceConfirm         string
	forceVersion         int
	forceErr             error
}

func newHarness(env map[string]string) *harness { return &harness{env: env} }

func (h *harness) cliEnv() cliEnv {
	return cliEnv{
		Stdout: &h.stdout,
		Stderr: &h.stderr,
		Getenv: func(k string) string { return h.env[k] },
		Provision: func(rootDSN string, roles []db.RoleGrant) error {
			h.calls = append(h.calls, "provision")
			h.provisionRoot = rootDSN
			h.provisionRoles = append([]db.RoleGrant(nil), roles...)
			h.provisionRolesByCall = append(h.provisionRolesByCall, h.provisionRoles)
			return h.provisionErr
		},
		Up: func(dsn string) error {
			h.calls = append(h.calls, "up")
			h.upDSN = dsn
			return h.upErr
		},
		Version: func(string) (uint, bool, bool, error) {
			h.calls = append(h.calls, "version")
			return 7, false, true, nil
		},
		DownAll: func(string, string) error {
			h.calls = append(h.calls, "down")
			return nil
		},
		Force: func(dsn, confirm string, version int) error {
			h.calls = append(h.calls, "force")
			h.forceDSN, h.forceConfirm, h.forceVersion = dsn, confirm, version
			return h.forceErr
		},
	}
}

// assertNoSecrets は stdout・stderr に DSN のパスワードが出ていないこと。
func (h *harness) assertNoSecrets(t *testing.T) {
	t.Helper()
	out := h.stdout.String() + h.stderr.String()
	for _, pw := range fakePWs {
		if strings.Contains(out, pw) {
			t.Errorf("出力にパスワードが出ている: %q", out)
		}
	}
}

func fullProvisionEnv() map[string]string {
	return map[string]string{
		"POKEDEX_PROVISION_DSN": fakeProvisionDSN,
		"POKEDEX_READER_DSN":    fakeReaderDSN,
		"POKEDEX_IMPORTER_DSN":  fakeImporterDSN,
		"POKEDEX_DATABASE_DSN":  fakeMigratorDSN,
	}
}

// AC-5: POKEDEX_PROVISION_DSN が無ければプロビジョニングせず、POKEDEX_DATABASE_DSN で直接 Up する(後方互換)。
func TestUpWithoutProvisionDSNSkipsProvisioning(t *testing.T) {
	h := newHarness(map[string]string{"POKEDEX_DATABASE_DSN": fakeProvisionDSN})
	if code := run([]string{"up"}, h.cliEnv()); code != 0 {
		t.Fatalf("exit = %d, want 0(stderr=%q)", code, h.stderr.String())
	}
	if !reflect.DeepEqual(h.calls, []string{"up"}) {
		t.Errorf("呼び出し = %v, want [up](プロビジョニングしない)", h.calls)
	}
	if h.upDSN != fakeProvisionDSN {
		t.Error("Up に POKEDEX_DATABASE_DSN を渡していない")
	}
	h.assertNoSecrets(t)
}

// AC-5: POKEDEX_PROVISION_DSN があれば、root で3ロールをプロビジョニングしてから migrator で Up する。
// issue #312・ADR-0125: importer は表単位(schema_migrations を除く)の権限なので、Up で増えた表に
// 追随させるため、Up の後に importer だけもう一度プロビジョニングする。
func TestUpWithProvisionDSNProvisionsThenMigrates(t *testing.T) {
	h := newHarness(fullProvisionEnv())
	if code := run([]string{"up"}, h.cliEnv()); code != 0 {
		t.Fatalf("exit = %d, want 0(stderr=%q)", code, h.stderr.String())
	}
	if !reflect.DeepEqual(h.calls, []string{"provision", "up", "provision"}) {
		t.Fatalf("呼び出し = %v, want [provision up provision](プロビジョニングが先、Up の後に表単位のロールを付け直す)", h.calls)
	}
	if h.provisionRoot != fakeProvisionDSN {
		t.Error("Provision の root DSN が POKEDEX_PROVISION_DSN でない")
	}
	if len(h.provisionRolesByCall) != 2 {
		t.Fatalf("Provision の呼び出し回数 = %d, want 2", len(h.provisionRolesByCall))
	}
	importer := db.RoleGrant{DSN: fakeImporterDSN, Privileges: db.ImporterPrivileges, Scope: db.ScopeDataTables}
	if !reflect.DeepEqual(h.provisionRolesByCall[1], []db.RoleGrant{importer}) {
		t.Error("Up の後の Provision は importer(表単位)だけを渡すこと")
	}
	want := []db.RoleGrant{
		{DSN: fakeReaderDSN, Privileges: db.ReaderPrivileges},
		importer,
		{DSN: fakeMigratorDSN, Privileges: db.MigratorPrivileges},
	}
	h.provisionRoles = h.provisionRolesByCall[0]
	if !reflect.DeepEqual(h.provisionRoles, want) {
		// DSN を表示しない(パスワードを含むため)。権限だけを並べる。
		var got []string
		for _, r := range h.provisionRoles {
			got = append(got, r.Privileges)
		}
		t.Errorf("Provision のロール(権限)= %q, want reader・importer・migrator の順で DSN と権限の組が一致", got)
	}
	if h.upDSN != fakeMigratorDSN {
		t.Error("Up は migrator(POKEDEX_DATABASE_DSN)で行うこと。root(POKEDEX_PROVISION_DSN)で migrate しない")
	}
	h.assertNoSecrets(t)
}

// AC-5: POKEDEX_PROVISION_DSN があるのにロールの DSN が欠けていれば、何もせず終了コード 2(設定の誤り)。
func TestUpWithProvisionDSNRequiresAllRoleDSNs(t *testing.T) {
	for _, missing := range []string{"POKEDEX_READER_DSN", "POKEDEX_IMPORTER_DSN"} {
		t.Run(missing, func(t *testing.T) {
			env := fullProvisionEnv()
			delete(env, missing)
			h := newHarness(env)
			if code := run([]string{"up"}, h.cliEnv()); code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			if len(h.calls) != 0 {
				t.Errorf("呼び出し = %v, want なし(途中までプロビジョニングしない)", h.calls)
			}
			if !strings.Contains(h.stderr.String(), missing) {
				t.Errorf("stderr に欠けている環境変数名 %s が無い: %q", missing, h.stderr.String())
			}
			h.assertNoSecrets(t)
		})
	}
}

// AC-5: POKEDEX_DATABASE_DSN が無ければ(PROVISION の有無によらず)何もせず失敗する(既存の挙動を保つ)。
func TestUpRequiresDatabaseDSN(t *testing.T) {
	env := fullProvisionEnv()
	delete(env, "POKEDEX_DATABASE_DSN")
	h := newHarness(env)
	if code := run([]string{"up"}, h.cliEnv()); code == 0 {
		t.Error("POKEDEX_DATABASE_DSN が無いのに成功した")
	}
	if len(h.calls) != 0 {
		t.Errorf("呼び出し = %v, want なし", h.calls)
	}
	h.assertNoSecrets(t)
}

// AC-5: プロビジョニングが失敗したら Up せず終了コード 1。エラー文にパスワードを出さない。
func TestUpStopsWhenProvisionFails(t *testing.T) {
	h := newHarness(fullProvisionEnv())
	h.provisionErr = errors.New("provision: 接続できない")
	if code := run([]string{"up"}, h.cliEnv()); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !reflect.DeepEqual(h.calls, []string{"provision"}) {
		t.Errorf("呼び出し = %v, want [provision](失敗したら Up しない)", h.calls)
	}
	if h.stderr.Len() == 0 {
		t.Error("失敗の理由を stderr に出していない")
	}
	h.assertNoSecrets(t)
}

// AC-5: Up が失敗したら終了コード 1。
func TestUpFailureExitsOne(t *testing.T) {
	h := newHarness(fullProvisionEnv())
	h.upErr = errors.New("migrate up: 失敗")
	if code := run([]string{"up"}, h.cliEnv()); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	h.assertNoSecrets(t)
}

// AC-5: プロビジョニングは up だけ。version は POKEDEX_PROVISION_DSN があってもプロビジョニングしない。
func TestVersionNeverProvisions(t *testing.T) {
	h := newHarness(fullProvisionEnv())
	if code := run([]string{"version"}, h.cliEnv()); code != 0 {
		t.Fatalf("exit = %d, want 0(stderr=%q)", code, h.stderr.String())
	}
	if !reflect.DeepEqual(h.calls, []string{"version"}) {
		t.Errorf("呼び出し = %v, want [version]", h.calls)
	}
	h.assertNoSecrets(t)
}

// issue #221: force はフラグ(-version・-confirm)を検査してから Force に渡す。欠け・負数は
// Force を呼ばずに終了コード 2。Force の拒否(DB 名の不一致・存在しない版・dirty でない)は終了コード 1。
func TestForceFlags(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		forceErr  error
		wantCode  int
		wantCall  bool
		wantInErr string
	}{
		{"正常", []string{"force", "-version", "2", "-confirm", "pokedex_fake"}, nil, 0, true, ""},
		{"0 は未適用に戻す", []string{"force", "-version", "0", "-confirm", "pokedex_fake"}, nil, 0, true, ""},
		{"-version が無い", []string{"force", "-confirm", "pokedex_fake"}, nil, 2, false, "-version"},
		{"-version が負", []string{"force", "-version", "-1", "-confirm", "pokedex_fake"}, nil, 2, false, "-version"},
		{"-version が数でない", []string{"force", "-version", "x", "-confirm", "pokedex_fake"}, nil, 2, false, ""},
		{"-confirm が無い", []string{"force", "-version", "2"}, nil, 2, false, "-confirm"},
		{"余分な引数", []string{"force", "-version", "2", "-confirm", "pokedex_fake", "extra"}, nil, 2, false, ""},
		{"DB 名の不一致", []string{"force", "-version", "2", "-confirm", "other"}, db.ErrForceNotConfirmed, 1, true, ""},
		{"存在しない版", []string{"force", "-version", "99", "-confirm", "pokedex_fake"}, db.ErrForceUnknownVersion, 1, true, ""},
		{"dirty でない", []string{"force", "-version", "2", "-confirm", "pokedex_fake"}, db.ErrForceNotDirty, 1, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(map[string]string{"POKEDEX_DATABASE_DSN": fakeMigratorDSN, "POKEDEX_PROVISION_DSN": fakeProvisionDSN})
			h.forceErr = tc.forceErr
			code := run(tc.args, h.cliEnv())
			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d(stderr=%q)", code, tc.wantCode, h.stderr.String())
			}
			called := reflect.DeepEqual(h.calls, []string{"force"})
			if called != tc.wantCall {
				t.Errorf("呼び出し = %v, want Force を呼ぶ=%v(プロビジョニングもしない)", h.calls, tc.wantCall)
			}
			if tc.wantCall && tc.forceErr == nil {
				if h.forceDSN != fakeMigratorDSN {
					t.Error("Force に POKEDEX_DATABASE_DSN を渡していない")
				}
				if h.forceConfirm != "pokedex_fake" {
					t.Errorf("Force の確認用 DB 名 = %q", h.forceConfirm)
				}
			}
			if tc.wantInErr != "" && !strings.Contains(h.stderr.String(), tc.wantInErr) {
				t.Errorf("stderr に %q が無い: %q", tc.wantInErr, h.stderr.String())
			}
			if tc.forceErr != nil && h.stderr.Len() == 0 {
				t.Error("拒否の理由を stderr に出していない")
			}
			h.assertNoSecrets(t)
		})
	}
}

// issue #221: force の版は -version の値をそのまま渡す。
func TestForcePassesVersion(t *testing.T) {
	h := newHarness(map[string]string{"POKEDEX_DATABASE_DSN": fakeMigratorDSN})
	if code := run([]string{"force", "-version", "5", "-confirm", "pokedex_fake"}, h.cliEnv()); code != 0 {
		t.Fatalf("exit = %d(stderr=%q)", code, h.stderr.String())
	}
	if h.forceVersion != 5 {
		t.Errorf("Force の版 = %d, want 5", h.forceVersion)
	}
}

// POKEDEX_DATABASE_DSN が無ければ force も何もせず失敗する。
func TestForceRequiresDatabaseDSN(t *testing.T) {
	h := newHarness(map[string]string{})
	if code := run([]string{"force", "-version", "2", "-confirm", "pokedex_fake"}, h.cliEnv()); code == 0 {
		t.Error("POKEDEX_DATABASE_DSN が無いのに成功した")
	}
	if len(h.calls) != 0 {
		t.Errorf("呼び出し = %v, want なし", h.calls)
	}
}

// issue #312: Up の後の importer の付け直しが失敗したら終了コード 1(新しい表に書けない importer を黙って残さない)。
func TestUpFailsWhenRegrantAfterUpFails(t *testing.T) {
	h := newHarness(fullProvisionEnv())
	env := h.cliEnv()
	calls := 0
	env.Provision = func(rootDSN string, roles []db.RoleGrant) error {
		h.calls = append(h.calls, "provision")
		calls++
		if calls == 2 {
			return errors.New("provision: 接続できない")
		}
		return nil
	}
	if code := run([]string{"up"}, env); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !reflect.DeepEqual(h.calls, []string{"provision", "up", "provision"}) {
		t.Errorf("呼び出し = %v", h.calls)
	}
	if h.stderr.Len() == 0 {
		t.Error("失敗の理由を stderr に出していない")
	}
	h.assertNoSecrets(t)
}
