package db

// ファイルを読むだけの静的テスト(ADR-0100 §1〜§5・§7・§9)。DB は使わない(make test で走る)。

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const repoRoot = "../../.."

var migrationName = regexp.MustCompile(`^(\d{6})_([a-z0-9]+(?:_[a-z0-9]+)*)\.(up|down)\.sql$`)

// migrationPairs は migrations/ のファイルを版ごとにまとめる。規則に合わない名前は失敗にする。
func migrationPairs(t *testing.T) (versions []int, up, down map[int]string) {
	t.Helper()
	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatalf("migrations/ が読めない: %v", err)
	}
	up, down = map[int]string{}, map[int]string{}
	titles := map[int]string{}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("migrations/ にディレクトリがある: %s", e.Name())
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("ファイル名が NNNNNN_<snake>.(up|down).sql でない: %s", e.Name())
			continue
		}
		v, _ := strconv.Atoi(m[1])
		if prev, ok := titles[v]; ok && prev != m[2] {
			t.Errorf("版 %d の up と down の題名が違う: %s / %s", v, prev, m[2])
		}
		titles[v] = m[2]
		target := up
		if m[3] == "down" {
			target = down
		}
		if _, dup := target[v]; dup {
			t.Errorf("版 %d の %s が重複している", v, m[3])
		}
		target[v] = filepath.Join("migrations", e.Name())
	}
	for v := range titles {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions, up, down
}

func TestMigrationsAreNumberedPairs(t *testing.T) {
	versions, up, down := migrationPairs(t)
	if len(versions) == 0 {
		t.Fatal("migration が1つも無い")
	}
	for i, v := range versions {
		if v != i+1 {
			t.Fatalf("版は 1 から隙間なく並ぶこと: %v", versions)
		}
		if up[v] == "" {
			t.Errorf("版 %d に up が無い", v)
		}
		if down[v] == "" {
			t.Errorf("版 %d に down が無い(down の無い migration を作らない)", v)
		}
	}
}

func readAll(t *testing.T, paths map[int]string) string {
	t.Helper()
	var b strings.Builder
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return strings.ToLower(b.String())
}

// requiredTables は ADR-0100 §3 のテーブル。
var requiredTables = []string{
	"types", "type_chart",
	"abilities", "items", "moves", "species", "species_abilities",
	"item_effects", "ability_effects", "move_effects", "learnsets",
	"regulations", "regulation_species", "regulation_moves", "regulation_items", "regulation_abilities",
	"data_versions",
}

func TestMigrationsCreateAndDropRequiredTables(t *testing.T) {
	_, up, down := migrationPairs(t)
	upSQL, downSQL := readAll(t, up), readAll(t, down)
	for _, table := range requiredTables {
		create := regexp.MustCompile(`create\s+table\s+(if\s+not\s+exists\s+)?` + "`?" + table + "`?" + `\s*\(`)
		if !create.MatchString(upSQL) {
			t.Errorf("up に CREATE TABLE %s が無い", table)
		}
		drop := regexp.MustCompile(`drop\s+table\s+(if\s+exists\s+)?` + "`?" + table + "`?" + `\s*;`)
		if !drop.MatchString(downSQL) {
			t.Errorf("down に DROP TABLE %s が無い", table)
		}
	}
}

// TestMigrationsHaveNoData は migrations に実データを入れないこと(ADR-0100 §2。タイプもマスタ = ADR-0013)。
func TestMigrationsHaveNoData(t *testing.T) {
	_, up, down := migrationPairs(t)
	for _, paths := range []map[int]string{up, down} {
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if regexp.MustCompile(`(?i)\b(insert|replace)\s+into\b`).Match(raw) {
				t.Errorf("%s: migrations に INSERT/REPLACE を書かない(データは importer が入れる)", p)
			}
		}
	}
}

// TestMigrationsDeclareKeyConstraints は ADR-0100 §3 の要の制約が up にあることを確かめる
// (効くことの確認は -tags mysql のテスト)。
func TestMigrationsDeclareKeyConstraints(t *testing.T) {
	_, up, _ := migrationPairs(t)
	upSQL := readAll(t, up)
	cases := []struct {
		name string
		re   string
	}{
		{"倍率コード 0/1/2/4", `code\s+in\s*\(\s*0\s*,\s*1\s*,\s*2\s*,\s*4\s*\)`},
		{"ID 列は ascii_bin", `ascii_bin`},
		{"メガの整合(is_mega と required_item_id)", `is_mega\s*=\s*1[^;]*required_item_id\s+is\s+not\s+null`},
		{"既定のレギュレーションは高々1件(生成列)", `default_marker[^,]*generated\s+always\s+as|default_marker[^,]*\bas\s*\(`},
		{"効果定義は JSON 列", `effect\s+json\b`},
		{"特性スロット 1..4", `slot\s+in\s*\(\s*1\s*,\s*2\s*,\s*3\s*,\s*4\s*\)`},
		{"checksum は sha256 の16進", `\[0-9a-f\]\{64\}`},
		{"species の外部キー(タイプ)", `foreign\s+key\s*\(\s*` + "`?" + `type1` + "`?" + `\s*\)\s*references\s+` + "`?" + `types`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !regexp.MustCompile(tc.re).MatchString(upSQL) {
				t.Fatalf("up に見当たらない: /%s/", tc.re)
			}
		})
	}
}

// TestMigrationsAreEmbedded は migrate が使う embed.FS が migrations/ と同じ集合であること。
func TestMigrationsAreEmbedded(t *testing.T) {
	_, up, down := migrationPairs(t)
	var onDisk []string
	for _, m := range []map[int]string{up, down} {
		for _, p := range m {
			onDisk = append(onDisk, filepath.Base(p))
		}
	}
	sort.Strings(onDisk)
	embedded, err := fs.Glob(Migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := range embedded {
		embedded[i] = filepath.Base(embedded[i])
	}
	sort.Strings(embedded)
	if strings.Join(embedded, ",") != strings.Join(onDisk, ",") {
		t.Fatalf("embed の集合 %v と migrations/ の集合 %v が違う", embedded, onDisk)
	}
}

// TestDownAllRequiresConfirmation は DB 名の確認が無い down を接続前に拒否すること(ADR-0100 §5)。
// 到達できないアドレスを使い、接続を試みたら別のエラーになって落ちるようにしている。
func TestDownAllRequiresConfirmation(t *testing.T) {
	const dsn = "testuser:testpass@tcp(127.0.0.1:1)/pokedex_test"
	// DB 名を持たない DSN(スキーム全体への接続になり、全データベースが対象になり得る)。
	const dsnWithoutDBName = "testuser:testpass@tcp(127.0.0.1:1)/"
	cases := []struct {
		name    string
		dsn     string
		confirm string
	}{
		{name: "確認なし", confirm: ""},
		{name: "別の DB 名", confirm: "pokedex"},
		{name: "大文字小文字違い", confirm: "POKEDEX_TEST"},
		{name: "前方一致", confirm: "pokedex_tes"},
		// DSN 側に DB 名が無いと「確認なし(空文字)」同士が一致してしまいかねない。
		// 空文字同士の一致でも確認したことにしない(ADR-0100 §5)。
		{name: "DSN に DB 名が無い×確認なし", dsn: dsnWithoutDBName, confirm: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.dsn
			if d == "" {
				d = dsn
			}
			if err := DownAll(d, tc.confirm); !errors.Is(err, ErrDownNotConfirmed) {
				t.Fatalf("ErrDownNotConfirmed にならない: %v", err)
			}
		})
	}
}

func TestSqlcConfig(t *testing.T) {
	raw, err := os.ReadFile("sqlc.yaml")
	if err != nil {
		t.Fatalf("sqlc.yaml が無い: %v", err)
	}
	var cfg struct {
		Version string `yaml:"version"`
		SQL     []struct {
			Engine  string `yaml:"engine"`
			Schema  string `yaml:"schema"`
			Queries string `yaml:"queries"`
			Gen     struct {
				Go struct {
					Package string `yaml:"package"`
					Out     string `yaml:"out"`
				} `yaml:"go"`
			} `yaml:"gen"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("sqlc.yaml: %v", err)
	}
	if cfg.Version != "2" || len(cfg.SQL) != 1 {
		t.Fatalf("version \"2\" で sql が1件: %+v", cfg)
	}
	s := cfg.SQL[0]
	checks := map[string][2]string{
		"engine":         {s.Engine, "mysql"},
		"schema":         {strings.TrimSuffix(s.Schema, "/"), "migrations"},
		"queries":        {strings.TrimSuffix(s.Queries, "/"), "query"},
		"gen.go.package": {s.Gen.Go.Package, "store"},
		"gen.go.out":     {strings.TrimSuffix(s.Gen.Go.Out, "/"), "../internal/store"},
	}
	for k, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", k, v[0], v[1])
		}
	}
	queries, _ := filepath.Glob("query/*.sql")
	if len(queries) == 0 {
		t.Error("query/*.sql が無い")
	}
	generated, _ := filepath.Glob("../internal/store/*.go")
	found := false
	for _, g := range generated {
		b, err := os.ReadFile(g)
		if err == nil && strings.Contains(string(b), "Code generated by sqlc") {
			found = true
		}
	}
	if !found {
		t.Error("services/pokedex/internal/store に sqlc の生成物が無い(make gen の結果をコミットする)")
	}
}

// --- Makefile・k8s(ADR-0100 §5・§9) ------------------------------------------

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return string(b)
}

// makeTargets は Makefile の「ターゲット: 依存」行とレシピを集める(include は見ない)。
func makeTargets(src string) map[string]string {
	targets := map[string]string{}
	var current string
	head := regexp.MustCompile(`^([a-zA-Z0-9_-]+):([^=].*)?$`)
	for _, line := range strings.Split(src, "\n") {
		if m := head.FindStringSubmatch(line); m != nil {
			current = m[1]
			targets[current] += m[2] + "\n"
			continue
		}
		if strings.HasPrefix(line, "\t") && current != "" {
			targets[current] += line + "\n"
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, ".PHONY") || strings.HasPrefix(line, "#") {
			continue
		}
		current = ""
	}
	return targets
}

func TestMakefileTargets(t *testing.T) {
	targets := makeTargets(readRepoFile(t, "Makefile"))
	for _, name := range []string{"migrate-up", "migrate-down", "migrate-version", "test-db", "gen-sql", "db-local-up"} {
		if _, ok := targets[name]; !ok {
			t.Errorf("Makefile にターゲット %s が無い", name)
		}
	}
	if !regexp.MustCompile(`\bgen-sql\b`).MatchString(firstLine(targets["gen"])) {
		t.Errorf("gen が gen-sql に依存していない: %q", firstLine(targets["gen"]))
	}
	if !strings.Contains(targets["test-db"], "-tags mysql") || !strings.Contains(targets["test-db"], "POKEDEX_TEST_DSN") {
		t.Errorf("test-db は -tags mysql で走り、POKEDEX_TEST_DSN が無ければ失敗すること: %q", targets["test-db"])
	}
	if !strings.Contains(targets["migrate-down"], "CONFIRM_DESTROY") {
		t.Errorf("migrate-down は CONFIRM_DESTROY を要求すること: %q", targets["migrate-down"])
	}
	for name, body := range targets {
		if name == "migrate-down" {
			continue
		}
		if strings.Contains(body, "migrate-down") || regexp.MustCompile(`cmd/migrate\s+down`).MatchString(body) {
			t.Errorf("ターゲット %s が down を呼んでいる(down は人間の確認つきの migrate-down だけ)", name)
		}
	}
	for _, name := range []string{"test", "test-services"} {
		if strings.Contains(targets[name], "mysql") || strings.Contains(targets[name], "test-db") {
			t.Errorf("%s に DB が要るテストを混ぜない: %q", name, targets[name])
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// TestNoAutomaticDown は scripts・k8s・pokedex の Dockerfile/Makefile に down を流す経路が
// 無いこと(CLAUDE.md: DB のデータ削除は人間の確認)。
func TestNoAutomaticDown(t *testing.T) {
	re := regexp.MustCompile(`migrate-down|cmd/migrate\s+down|["']down["']\s*,\s*["']-confirm|\bmigrate\b[^\n]*\bdown\b`)
	check := func(p string) {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if loc := re.FindIndex(b); loc != nil {
			t.Errorf("%s: down を流す記述がある: %q", p, b[loc[0]:loc[1]])
		}
	}
	for _, dir := range []string{"scripts", "deploy"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			check(p)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	check(filepath.Join(repoRoot, "services/pokedex/Dockerfile"))
	makefiles, err := filepath.Glob(filepath.Join(repoRoot, "services/*/Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range makefiles {
		check(p)
	}
}

// TestUpScriptBuildsAndImportsMigrateImage は scripts/up.sh が pokedex-migrate イメージを
// build し、k3d に import してから Job を流すこと(ADR-0100 §9: 「make up が k3d に import
// する」)。
func TestUpScriptBuildsAndImportsMigrateImage(t *testing.T) {
	s := readRepoFile(t, "scripts/up.sh")
	if !regexp.MustCompile(`docker build[^\n]*--target migrate`).MatchString(s) {
		t.Error("scripts/up.sh が pokedex-migrate イメージを build していない(--target migrate)")
	}
	if !strings.Contains(s, "k3d image import") {
		t.Error("scripts/up.sh が k3d image import を実行していない")
	}
	if !strings.Contains(s, "pokedex-migrate") {
		t.Error("scripts/up.sh が pokedex-migrate イメージ/Job を参照していない")
	}
}

// TestUpScriptCreatesMigrateJobOnlyOnce は pokedex-migrate Job が overlay 経由の1か所だけで
// 作られ、pokecalc namespace に入ること、削除が apply より前にあることを検査する
// (再レビュー: base/pokedex を単独 apply すると namespace が付かず default にも Job が
// でき、MySQL Ready 後の delete が実行中の正しい Job を消しかねなかった)。
func TestUpScriptCreatesMigrateJobOnlyOnce(t *testing.T) {
	s := readRepoFile(t, "scripts/up.sh")

	if regexp.MustCompile(`apply\s+-k\s+deploy/k8s/base/pokedex\b`).MatchString(s) {
		t.Error("scripts/up.sh が deploy/k8s/base/pokedex を単独で apply している" +
			"(namespace が付かず default に Job ができる。overlay 経由の1か所だけにする)")
	}
	applyOverlay := regexp.MustCompile(`apply\s+-k\s+deploy/k8s/overlays/local\b`)
	if n := len(applyOverlay.FindAllString(s, -1)); n != 1 {
		t.Errorf("overlays/local の apply が %d 回ある(1回だけであること)", n)
	}

	overlayKustomization := readRepoFile(t, "deploy/k8s/overlays/local/kustomization.yaml")
	if !regexp.MustCompile(`(?m)^namespace:\s*pokecalc\s*$`).MatchString(overlayKustomization) {
		t.Error("deploy/k8s/overlays/local/kustomization.yaml に namespace: pokecalc が無い" +
			"(Job が pokecalc namespace に入らない)")
	}

	deleteIdx := regexp.MustCompile(`delete\s+job\s+pokedex-migrate`).FindStringIndex(s)
	applyIdx := applyOverlay.FindStringIndex(s)
	if deleteIdx == nil {
		t.Fatal("scripts/up.sh に pokedex-migrate Job の削除が無い")
	}
	if applyIdx == nil {
		t.Fatal("scripts/up.sh に overlays/local の apply が無い")
	}
	if deleteIdx[0] > applyIdx[0] {
		t.Error("Job の削除が overlay の apply より後にある(実行中かもしれない正しい Job を消してしまう)")
	}
}

// TestUpScriptWaitsForMigrateJobCompletion は up.sh が pokedex-migrate Job の完了を
// 確かめ、失敗時は up.sh 自体を失敗させること(kubectl wait はデフォルトで非ゼロ終了する)。
func TestUpScriptWaitsForMigrateJobCompletion(t *testing.T) {
	s := readRepoFile(t, "scripts/up.sh")
	if !regexp.MustCompile(`wait\s+--for=condition=complete\s+job/pokedex-migrate`).MatchString(s) {
		t.Error("scripts/up.sh が pokedex-migrate Job の完了を待っていない" +
			"(kubectl wait --for=condition=complete)")
	}
}

func TestLocalMySQLManifests(t *testing.T) {
	dir := filepath.Join(repoRoot, "deploy/k8s/overlays/local/mysql")
	var all strings.Builder
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		all.Write(b)
		all.WriteByte('\n')
		return err
	})
	if err != nil {
		t.Fatalf("deploy/k8s/overlays/local/mysql が読めない: %v", err)
	}
	s := all.String()
	for _, want := range []string{"kind: StatefulSet", "image: mysql:9.7.2@sha256:", "volumeClaimTemplates", "kind: Service", "MYSQL_DATABASE"} {
		if !strings.Contains(s, want) {
			t.Errorf("MySQL の manifest に %q が無い", want)
		}
	}
	// pokedex-dsn(scripts/up.sh が作る secret)は DB 名 pokedex を指す。MYSQL_DATABASE が
	// 無いと初回起動時に DB が作られず、migrate が「Unknown database」で失敗し続ける。
	if !strings.Contains(s, "value: pokedex") {
		t.Error("MySQL の manifest の MYSQL_DATABASE が pokedex を指していない(scripts/up.sh の pokedex-dsn と食い違う)")
	}
	if !strings.Contains(readRepoFile(t, "deploy/k8s/overlays/local/kustomization.yaml"), "mysql") {
		t.Error("overlays/local/kustomization.yaml が mysql を参照していない")
	}
	if !strings.Contains(readRepoFile(t, "scripts/up.sh"), "mysql-auth") {
		t.Error("scripts/up.sh が Secret mysql-auth を作っていない(Secret はコミットせず up.sh が作る)")
	}
}

// TestNoCommittedSecrets は deploy/ に値入りの Secret を置かないこと(ADR-0100 §9)。
func TestNoCommittedSecrets(t *testing.T) {
	err := filepath.WalkDir(filepath.Join(repoRoot, "deploy"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, doc := range strings.Split(string(b), "\n---") {
			if regexp.MustCompile(`(?m)^kind:\s*Secret\s*$`).MatchString(doc) &&
				regexp.MustCompile(`(?m)^(data|stringData):`).MatchString(doc) {
				t.Errorf("%s: 値入りの Secret をコミットしない", p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- 架空データの example(ADR-0100 §7) --------------------------------------

func TestExampleSeedIsFictional(t *testing.T) {
	raw, err := os.ReadFile("testdata/example_seed.sql")
	if err != nil {
		t.Fatalf("example が無い: %v", err)
	}
	japanese := regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)
	literal := regexp.MustCompile(`'([^']*)'`)
	for _, m := range literal.FindAllStringSubmatch(string(raw), -1) {
		if japanese.MatchString(m[1]) && !strings.HasPrefix(m[1], "テスト") {
			t.Errorf("日本語の値は「テスト」で始める: %q", m[1])
		}
	}
	for _, m := range regexp.MustCompile(`'(\d{4})-(\d{3})'`).FindAllStringSubmatch(string(raw), -1) {
		if n, _ := strconv.Atoi(m[1]); n < 9001 {
			t.Errorf("図鑑番号は 9001 以降(実在の番号を使わない): %s-%s", m[1], m[2])
		}
	}
}

// --- DB 資格情報の最小権限(ADR-0110 §5・§6。issue #104) ---------------------------

// Secret mysql-auth のキー(ADR-0110 §3・§5)。値そのものはテストに書かない。
const (
	secretMySQLAuth      = "mysql-auth"
	keyRootPW            = "mysql-root-password"
	keyProvisionDSN      = "pokedex-dsn" // root。migrate Job のプロビジョニング専用
	keyReaderDSN         = "pokedex-reader-dsn"
	keyImporterDSN       = "pokedex-importer-dsn"
	keyMigratorDSN       = "pokedex-migrator-dsn"
	pokedexDeployment    = "deploy/k8s/base/pokedex/deployment.yaml"
	pokedexMigrateJob    = "deploy/k8s/base/pokedex/job-migrate.yaml"
	pokedexImportCronJob = "deploy/k8s/base/pokedex/cronjob-import.yaml"
)

type credEnvVar struct {
	Name      string  `yaml:"name"`
	Value     *string `yaml:"value"`
	ValueFrom *struct {
		SecretKeyRef *struct {
			Name string `yaml:"name"`
			Key  string `yaml:"key"`
		} `yaml:"secretKeyRef"`
	} `yaml:"valueFrom"`
}

type credContainer struct {
	Name    string       `yaml:"name"`
	Command []string     `yaml:"command"`
	Args    []string     `yaml:"args"`
	Env     []credEnvVar `yaml:"env"`
	EnvFrom []any        `yaml:"envFrom"`
}

type credPodSpec struct {
	InitContainers []credContainer `yaml:"initContainers"`
	Containers     []credContainer `yaml:"containers"`
	Volumes        []struct {
		Name   string `yaml:"name"`
		Secret *struct {
			SecretName string `yaml:"secretName"`
		} `yaml:"secret"`
	} `yaml:"volumes"`
}

// credWorkload は Deployment・Job・CronJob の Pod テンプレートを同じ形で読む。
type credWorkload struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Template struct {
			Spec credPodSpec `yaml:"spec"`
		} `yaml:"template"`
		JobTemplate struct {
			Spec struct {
				Template struct {
					Spec credPodSpec `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate"`
	} `yaml:"spec"`
}

func loadCredPod(t *testing.T, rel string) credPodSpec {
	t.Helper()
	var w credWorkload
	if err := yaml.Unmarshal([]byte(readRepoFile(t, rel)), &w); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	if w.Kind == "CronJob" {
		return w.Spec.JobTemplate.Spec.Template.Spec
	}
	return w.Spec.Template.Spec
}

// secretEnv はコンテナの env を「名前 → mysql-auth のキー」にする。値の直書き・他 Secret・envFrom は失敗にする。
func secretEnv(t *testing.T, where string, c credContainer) map[string]string {
	t.Helper()
	if len(c.EnvFrom) != 0 {
		t.Errorf("%s/%s: envFrom で Secret を丸ごと渡さない(必要なキーだけを secretKeyRef で渡す)", where, c.Name)
	}
	out := map[string]string{}
	credName := regexp.MustCompile(`(?i)dsn|password|pwd|secret|token`)
	for _, e := range c.Env {
		if credName.MatchString(e.Name) && e.Value != nil {
			t.Errorf("%s/%s: env %s に値を直接書かない", where, c.Name, e.Name)
		}
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		if e.ValueFrom.SecretKeyRef.Name != secretMySQLAuth {
			t.Errorf("%s/%s: env %s が Secret %s 以外(%s)を参照している", where, c.Name, e.Name, secretMySQLAuth, e.ValueFrom.SecretKeyRef.Name)
		}
		out[e.Name] = e.ValueFrom.SecretKeyRef.Key
	}
	return out
}

// AC-7: pokedex-svc(公開 API)は reader の DSN だけを使い、root の資格情報を一切参照しない。
func TestPokedexDeploymentUsesReaderDSNOnly(t *testing.T) {
	pod := loadCredPod(t, pokedexDeployment)
	if len(pod.Containers) == 0 {
		t.Fatalf("%s にコンテナが無い", pokedexDeployment)
	}
	for _, v := range pod.Volumes {
		if v.Secret != nil && v.Secret.SecretName == secretMySQLAuth {
			t.Errorf("%s: Secret %s をボリュームで丸ごとマウントしない", pokedexDeployment, secretMySQLAuth)
		}
	}
	for _, c := range append(append([]credContainer(nil), pod.InitContainers...), pod.Containers...) {
		env := secretEnv(t, pokedexDeployment, c)
		for name, key := range env {
			if key == keyProvisionDSN || key == keyRootPW || key == keyImporterDSN || key == keyMigratorDSN {
				t.Errorf("%s/%s: env %s が %s を参照している(pokedex-svc は %s だけ)", pokedexDeployment, c.Name, name, key, keyReaderDSN)
			}
		}
	}
	env := secretEnv(t, pokedexDeployment, pod.Containers[0])
	if got := env["POKEDEX_DATABASE_DSN"]; got != keyReaderDSN {
		t.Errorf("%s: POKEDEX_DATABASE_DSN のキー = %q, want %s", pokedexDeployment, got, keyReaderDSN)
	}
}

// AC-7: migrate Job の主コンテナは4つの DSN をそれぞれ決まったキーから受け取る。
// wait-for-mysql の initContainer は root パスワード(MYSQL_PWD)のまま変えない(鶏卵。ADR-0110 §6)。
func TestMigrateJobCredentialWiring(t *testing.T) {
	pod := loadCredPod(t, pokedexMigrateJob)
	var migrate *credContainer
	for i := range pod.Containers {
		if pod.Containers[i].Name == "migrate" {
			migrate = &pod.Containers[i]
		}
	}
	if migrate == nil {
		t.Fatalf("%s に migrate コンテナが無い", pokedexMigrateJob)
	}
	got := secretEnv(t, pokedexMigrateJob, *migrate)
	want := map[string]string{
		"POKEDEX_PROVISION_DSN": keyProvisionDSN,
		"POKEDEX_DATABASE_DSN":  keyMigratorDSN,
		"POKEDEX_READER_DSN":    keyReaderDSN,
		"POKEDEX_IMPORTER_DSN":  keyImporterDSN,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: migrate の env(名前→キー)= %v, want %v", pokedexMigrateJob, got, want)
	}
	for _, a := range append(append([]string(nil), migrate.Command...), migrate.Args...) {
		if strings.Contains(strings.ToLower(a), "dsn") || strings.Contains(a, "@tcp(") {
			t.Errorf("%s: DSN をコマンドライン引数で渡さない: %q", pokedexMigrateJob, a)
		}
	}

	var waitOK bool
	for _, c := range pod.InitContainers {
		env := secretEnv(t, pokedexMigrateJob, c)
		if c.Name == "wait-for-mysql" && env["MYSQL_PWD"] == keyRootPW {
			waitOK = true
		}
		for name, key := range env {
			if key == keyProvisionDSN || key == keyReaderDSN || key == keyImporterDSN || key == keyMigratorDSN {
				t.Errorf("%s/%s: initContainer に DSN(%s ← %s)を渡さない", pokedexMigrateJob, c.Name, name, key)
			}
		}
	}
	if !waitOK {
		t.Errorf("%s: wait-for-mysql が MYSQL_PWD ← %s のまま無い(3ユーザーは migrate が作るまで存在しない)", pokedexMigrateJob, keyRootPW)
	}
}

// AC-7: root の資格情報(pokedex-dsn・mysql-root-password)を参照するのは migrate Job と
// local の MySQL(StatefulSet の初期化)だけ。reader/importer/migrator のキーもそれぞれの用途だけが参照する。
func TestCredentialKeysReferencedOnlyByTheirOwners(t *testing.T) {
	owners := map[string][]string{
		keyProvisionDSN: {pokedexMigrateJob},
		keyRootPW:       {pokedexMigrateJob, "deploy/k8s/overlays/local/mysql/"},
		keyReaderDSN:    {pokedexDeployment, pokedexMigrateJob},
		keyImporterDSN:  {pokedexImportCronJob, pokedexMigrateJob},
		keyMigratorDSN:  {pokedexMigrateJob},
	}
	keyRef := regexp.MustCompile(`(?m)^\s*key:\s*["']?([a-z0-9-]+)["']?\s*$`)
	seen := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(repoRoot, "deploy"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, p)
		rel = filepath.ToSlash(rel)
		for _, m := range keyRef.FindAllStringSubmatch(string(b), -1) {
			allowed, tracked := owners[m[1]]
			if !tracked {
				continue
			}
			seen[m[1]+" "+rel] = true
			ok := false
			for _, a := range allowed {
				if rel == a || (strings.HasSuffix(a, "/") && strings.HasPrefix(rel, a)) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s が Secret キー %s を参照している(参照してよいのは %v だけ)", rel, m[1], allowed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, need := range []string{
		keyReaderDSN + " " + pokedexDeployment,
		keyImporterDSN + " " + pokedexImportCronJob,
		keyMigratorDSN + " " + pokedexMigrateJob,
		keyProvisionDSN + " " + pokedexMigrateJob,
	} {
		if !seen[need] {
			k, f, _ := strings.Cut(need, " ")
			t.Errorf("%s が Secret キー %s を参照していない", f, k)
		}
	}
}

// upScriptSecretBlock は scripts/up.sh のうち mysql-auth を扱う部分(最初の言及から pokedex-migrate の build まで)。
func upScriptSecretBlock(t *testing.T) string {
	t.Helper()
	s := readRepoFile(t, "scripts/up.sh")
	start := strings.Index(s, secretMySQLAuth)
	end := regexp.MustCompile(`docker build[^\n]*--target migrate`).FindStringIndex(s)
	if start < 0 || end == nil || end[0] < start {
		t.Fatal("scripts/up.sh に mysql-auth の作成 → pokedex-migrate の build の順の記述が無い")
	}
	return s[start:end[0]]
}

// AC-6: up.sh は新規クラスタで4つの DSN キー(と root パスワード)を作る。3ユーザーのパスワードは
// root と別に openssl rand -hex で作る(英数字だけ。db.Provision の入力検査の前提)。
func TestUpScriptCreatesRoleDSNKeys(t *testing.T) {
	block := upScriptSecretBlock(t)
	for _, key := range []string{keyRootPW, keyProvisionDSN, keyReaderDSN, keyImporterDSN, keyMigratorDSN} {
		if !strings.Contains(block, key) {
			t.Errorf("scripts/up.sh の mysql-auth の作成に %s が無い", key)
		}
	}
	for _, user := range []string{"pokedex_reader", "pokedex_importer", "pokedex_migrator"} {
		if !regexp.MustCompile(`\b` + user + `:`).MatchString(block) {
			t.Errorf("scripts/up.sh が %s の DSN(%s:<password>@...)を作っていない", user, user)
		}
	}
	if n := len(regexp.MustCompile(`openssl rand -hex (\d+)`).FindAllString(block, -1)); n < 2 {
		// 1 か所の関数/ループで4つ作る書き方も許すが、root と同じ値を使い回していないかは下で見る。
		if !regexp.MustCompile(`(?s)(for|while)\b.*openssl rand -hex`).MatchString(block) &&
			!regexp.MustCompile(`(?s)\w+\s*\(\)\s*\{[^}]*openssl rand -hex`).MatchString(block) {
			t.Errorf("scripts/up.sh が用途別のパスワードを openssl rand -hex で作っていない(%d 回)", n)
		}
	}
	for _, m := range regexp.MustCompile(`openssl rand -hex (\d+)`).FindAllStringSubmatch(block, -1) {
		if n, _ := strconv.Atoi(m[1]); n < 16 {
			t.Errorf("openssl rand -hex %s は短い(16 バイト以上。db.Provision は 16 文字以上を要求する)", m[1])
		}
	}
	for _, user := range []string{"pokedex_reader", "pokedex_importer", "pokedex_migrator"} {
		if regexp.MustCompile(`\b` + user + `:\$\{?root_pw`).MatchString(block) {
			t.Errorf("scripts/up.sh が %s のパスワードに root のパスワードを使い回している", user)
		}
	}
}

// AC-6: 既存の Secret は作り直さず、無いキーだけを kubectl patch --type=merge で足す(既存キーの値は変えない)。
func TestUpScriptPatchesMissingKeysOnExistingSecret(t *testing.T) {
	block := upScriptSecretBlock(t)
	if regexp.MustCompile(`delete\s+secret\s+mysql-auth`).MatchString(block) {
		t.Error("scripts/up.sh が mysql-auth を消している(既存 PVC の root パスワードと食い違う。作り直さない)")
	}
	patch := regexp.MustCompile(`patch\s+secret\s+mysql-auth[^\n]*`)
	if !patch.MatchString(block) {
		t.Fatal("scripts/up.sh に既存 Secret への kubectl patch secret mysql-auth が無い")
	}
	if !regexp.MustCompile(`patch\s+secret\s+mysql-auth[^\n]*--type[= ]merge`).MatchString(block) {
		t.Error("kubectl patch は --type=merge(JSON merge patch。指定したキーだけ足す)で行う")
	}
	for _, key := range []string{keyReaderDSN, keyImporterDSN, keyMigratorDSN} {
		if !regexp.MustCompile(`jsonpath=[^\n]*\{\.data(\.|\['|\[")`+regexp.QuoteMeta(key)).MatchString(block) &&
			!regexp.MustCompile(`jsonpath=[^\n]*\{\.data\.\$\{?\w+\}?\}`).MatchString(block) {
			t.Errorf("scripts/up.sh が既存 Secret に %s があるかを jsonpath で確かめていない(あれば上書きしない)", key)
		}
	}
	// patch の中身に root のキーを含めない(既存の値を変えない)。
	for _, line := range strings.Split(block, "\n") {
		if strings.Contains(line, "patch") && (strings.Contains(line, keyRootPW) || regexp.MustCompile(`"`+keyProvisionDSN+`"`).MatchString(line)) {
			t.Errorf("kubectl patch で root のキーを書き換えない: %q", strings.TrimSpace(line))
		}
	}
}

// AC-8: up.sh は Secret の値をログ・コマンドライン引数に出さない。値は標準入力かファイル経由で kubectl に渡す。
func TestUpScriptKeepsSecretValuesOutOfArgvAndLogs(t *testing.T) {
	s := readRepoFile(t, "scripts/up.sh")
	block := upScriptSecretBlock(t)
	if regexp.MustCompile(`(?m)^\s*set\s+-[a-z]*x`).MatchString(s) {
		t.Error("scripts/up.sh で set -x しない(コマンドを実行前に表示し、Secret の値がログに出る)")
	}
	if regexp.MustCompile(`--from-literal=["']?\$\{?(\w*(pw|pass|dsn)\w*)`).MatchString(block) ||
		regexp.MustCompile(`--from-literal=["']?[^\n]*\$\{?\w*(pw|pass)\w*`).MatchString(block) {
		t.Error("mysql-auth の値を --from-literal(コマンドライン引数)で渡さない(ps で見える。--from-env-file・--from-file・標準入力の apply を使う)")
	}
	if regexp.MustCompile(`patch\s+secret\s+mysql-auth[^\n]*\s(-p|--patch)[ =]`).MatchString(block) {
		t.Error("kubectl patch の中身を -p/--patch(コマンドライン引数)で渡さない(--patch-file を使う)")
	}
	echoSecret := regexp.MustCompile(`(?i)\b(echo|printf)\b[^\n]*\$\{?\w*(pw|pass|dsn|b64|base64)\w*`)
	for _, line := range strings.Split(block, "\n") {
		if !echoSecret.MatchString(line) {
			continue
		}
		// ファイル/標準入力へ流す(> や |)なら表示ではない。
		if regexp.MustCompile(`[>|]`).MatchString(line) {
			continue
		}
		t.Errorf("Secret の値を表示している: %q", strings.TrimSpace(line))
	}
	if strings.Contains(block, "mktemp") && !strings.Contains(block, "trap") {
		t.Error("Secret の値を一時ファイルに書くなら trap で必ず消す")
	}
}
