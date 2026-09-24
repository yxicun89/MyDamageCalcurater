package db

// db.Provision の入力検査と権限セットの定数(ADR-0110 §1・§4)。DB は使わない(make test で走る)。
// 接続前に弾くべき入力は、閉じたポート(127.0.0.1:1)を指す DSN を渡して ErrInvalidRoleGrant で
// 返ること(接続エラーでないこと)を確かめる。実際の権限境界は grants_mysql_test.go(make test-db)。

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// テスト用の架空の資格情報(実在の Secret の値ではない。英数字だけの固定値)。
const (
	fakeRootPass  = "rootpassrootpass0000"
	fakeRolePass  = "0123456789abcdef0123456789abcdef"
	fakeRolePass2 = "fedcba9876543210fedcba9876543210"
)

// unreachableDSN は閉じたポートを指す DSN を作る(検査を通れば接続で失敗する)。
func unreachableDSN(user, pass, dbName string) string {
	c := mysql.NewConfig()
	c.User = user
	c.Passwd = pass
	c.Net = "tcp"
	c.Addr = "127.0.0.1:1"
	c.DBName = dbName
	c.Timeout = time.Second
	return c.FormatDSN()
}

// AC-P0: 権限セットは ADR-0110 決定1 のちょうどの値(過不足なく)。
func TestRolePrivilegeConstants(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want []string
	}{
		{"ReaderPrivileges", ReaderPrivileges, []string{"SELECT"}},
		{"ImporterPrivileges", ImporterPrivileges, []string{"SELECT", "INSERT", "UPDATE", "DELETE"}},
		{"MigratorPrivileges", MigratorPrivileges, []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP", "INDEX", "REFERENCES"}},
		{"AppPrivileges", AppPrivileges, []string{"SELECT", "INSERT", "UPDATE", "DELETE"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]bool{}
			for _, p := range strings.Split(tc.got, ",") {
				got[strings.ToUpper(strings.TrimSpace(p))] = true
			}
			if len(got) != len(tc.want) {
				t.Errorf("%s = %q, want ちょうど %v", tc.name, tc.got, tc.want)
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("%s に %s が無い: %q", tc.name, w, tc.got)
				}
			}
			for _, forbidden := range []string{"ALL", "ALL PRIVILEGES", "GRANT OPTION", "CREATE USER", "SUPER", "FILE", "PROCESS", "RELOAD", "SHUTDOWN"} {
				if got[forbidden] {
					t.Errorf("%s にグローバル/管理権限 %s を含めない", tc.name, forbidden)
				}
			}
		})
	}
}

// AC-P6: 危険な入力は接続する前に ErrInvalidRoleGrant で拒否する。エラー文にパスワードを含めない。
func TestProvisionRejectsUnsafeInputBeforeConnecting(t *testing.T) {
	root := unreachableDSN("root", fakeRootPass, "pokedex")
	role := func(user, pass, dbName, privs string) RoleGrant {
		return RoleGrant{DSN: unreachableDSN(user, pass, dbName), Privileges: privs}
	}
	cases := []struct {
		name  string
		root  string
		roles []RoleGrant
	}{
		{"ロールが空", root, nil},
		{"root DSN が解釈できない", "not a dsn", []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"root DSN に DB 名が無い", unreachableDSN("root", fakeRootPass, ""), []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"ロールの DSN が解釈できない", root, []RoleGrant{{DSN: "not a dsn", Privileges: ReaderPrivileges}}},
		{"パスワードに引用符", root, []RoleGrant{role("pokedex_reader", "abc'def0123456789", "pokedex", ReaderPrivileges)}},
		{"パスワードにバックスラッシュ", root, []RoleGrant{role("pokedex_reader", `abc\def0123456789`, "pokedex", ReaderPrivileges)}},
		{"パスワードに空白", root, []RoleGrant{role("pokedex_reader", "abc def0123456789", "pokedex", ReaderPrivileges)}},
		{"パスワードが空", root, []RoleGrant{role("pokedex_reader", "", "pokedex", ReaderPrivileges)}},
		{"パスワードが16文字未満", root, []RoleGrant{role("pokedex_reader", "abc123", "pokedex", ReaderPrivileges)}},
		{"ユーザー名に引用符", root, []RoleGrant{role("pokedex'reader", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"ユーザー名にバッククォート", root, []RoleGrant{role("pokedex`reader", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"ユーザー名が33文字以上", root, []RoleGrant{role(strings.Repeat("a", 33), fakeRolePass, "pokedex", ReaderPrivileges)}},
		// root 自身を対象にすると REVOKE ALL で root の権限を剥がしてしまう(mysql イメージには root@'%' がある)。
		{"ロールのユーザーが root DSN のユーザーと同じ", root, []RoleGrant{role("root", fakeRolePass, "pokedex", ReaderPrivileges)}},
		// root DSN のユーザーが "root" という名前とは限らない(admin 等の別名で運用されうる)。
		// cfg.User == rootCfg.User の分岐(cfg.User == "root" の分岐とは別経路)を単独で検証する。
		{"ロールのユーザーが root DSN のユーザーと同じ(root という名前でない場合)", unreachableDSN("admin", fakeRootPass, "pokedex"), []RoleGrant{role("admin", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"ロールのユーザーが root", unreachableDSN("admin", fakeRootPass, "pokedex"), []RoleGrant{role("root", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"ロールのユーザーが mysql 予約名", root, []RoleGrant{role("mysql.sys", fakeRolePass, "pokedex", ReaderPrivileges)}},
		{"ロールの DB 名が root DSN と違う", root, []RoleGrant{role("pokedex_reader", fakeRolePass, "other", ReaderPrivileges)}},
		{"同じユーザーが2回", root, []RoleGrant{
			role("pokedex_reader", fakeRolePass, "pokedex", ReaderPrivileges),
			role("pokedex_reader", fakeRolePass2, "pokedex", ImporterPrivileges),
		}},
		{"権限が空", root, []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", "")}},
		{"権限に ALL", root, []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", "ALL PRIVILEGES")}},
		{"権限に GRANT OPTION", root, []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", "SELECT, GRANT OPTION")}},
		{"権限に CREATE USER", root, []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", "SELECT, CREATE USER")}},
		{"権限に SQL を混ぜる", root, []RoleGrant{role("pokedex_reader", fakeRolePass, "pokedex", "SELECT ON *.* TO x; --")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Provision(tc.root, tc.roles)
			if !errors.Is(err, ErrInvalidRoleGrant) {
				t.Fatalf("Provision = %v, want ErrInvalidRoleGrant(接続する前に拒否する)", err)
			}
			msg := err.Error()
			for _, secret := range []string{fakeRootPass, fakeRolePass, fakeRolePass2, "abc'def", `abc\def`, "abc def"} {
				if strings.Contains(msg, secret) {
					t.Errorf("エラー文にパスワードを含めない: %q", msg)
				}
			}
		})
	}
}

// AC-P6: 正しい入力は検査を通り、接続の段階まで進む(閉じたポートなので接続エラーになる)。
func TestProvisionAcceptsValidInputAndReachesConnect(t *testing.T) {
	root := unreachableDSN("root", fakeRootPass, "pokedex")
	roles := []RoleGrant{
		{DSN: unreachableDSN("pokedex_reader", fakeRolePass, "pokedex"), Privileges: ReaderPrivileges},
		{DSN: unreachableDSN("pokedex_importer", fakeRolePass2, "pokedex"), Privileges: ImporterPrivileges},
		{DSN: unreachableDSN("pokedex_migrator", strings.ToUpper(fakeRolePass), "pokedex"), Privileges: MigratorPrivileges},
	}
	err := Provision(root, roles)
	if err == nil {
		t.Fatal("閉じたポートなのに成功した")
	}
	if errors.Is(err, ErrInvalidRoleGrant) {
		t.Fatalf("正しい入力を ErrInvalidRoleGrant で拒否した: %v", err)
	}
	for _, secret := range []string{fakeRootPass, fakeRolePass, fakeRolePass2, strings.ToUpper(fakeRolePass)} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("接続エラーの文にパスワードを含めない: %q", err.Error())
		}
	}
}
