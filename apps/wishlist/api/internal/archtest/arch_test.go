package archtest

// ヘキサゴナル(ポートとアダプタ)への適合を機械で確かめる(apps/wishlist/docs/arch-spec.md の AC-A1〜A6)。
// go/parser で import 文だけを読む(ビルド・生成物は要らない)。規則の正は docs/plan/architecture-adoption.md §5.1。

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// moduleRoot はこのモジュール(apps/wishlist/api)の場所。テストはパッケージのディレクトリで動く。
const moduleRoot = "../.."

const specDoc = "apps/wishlist/docs/arch-spec.md"

// allowedTopLevel は internal/ の直下に置いてよいディレクトリ(AC-A2)。
var allowedTopLevel = []string{
	"adapters", // 外側(HTTP・DB・外部サイト・ファイル)
	"api",      // oapi-codegen の生成物(場所は今のまま。外側として扱う)
	"app",      // ユースケースとポート
	"archtest", // このテスト
	"domain",   // 純粋な核
	"testimg",  // テスト用の画像(_test.go からだけ使う)
}

// expectedPackages は移動後にあるべきパッケージ(本番の .go を 1 つ以上持つディレクトリ。AC-A3)。
var expectedPackages = []string{
	"internal/domain/item",
	"internal/domain/query",
	"internal/domain/deeplink",
	"internal/domain/estimate",
	"internal/domain/crawl",
	"internal/domain/page",
	"internal/app/items",
	"internal/app/items/itemtest",
	"internal/app/refresh",
	"internal/app/fromurl",
	"internal/adapters/httpapi",
	"internal/adapters/store",
	"internal/adapters/store/sqlcgen",
	"internal/adapters/memory",
	"internal/adapters/fetcher",
	"internal/adapters/chromium",
	"internal/adapters/official",
	"internal/adapters/ogp",
	"internal/adapters/storage",
	"internal/adapters/netguard",
	"internal/adapters/dbwait",
}

// forbiddenOuterLibs は domain・app の本番コードが import してはいけない外側のライブラリ(前方一致。AC-A5)。
var forbiddenOuterLibs = []string{
	"database/sql",
	"net/http",
	"os",
	"github.com/labstack/echo",
	"github.com/go-sql-driver/mysql",
	"github.com/golang-migrate/",
	"github.com/chromedp/",
	"github.com/PuerkitoBio/goquery",
	"github.com/google/uuid",
	"golang.org/x/net/html",
	"github.com/oapi-codegen/runtime",
}

// layer は import パス(モジュール内の相対)の置き場所。
type layer string

const (
	lDomain   layer = "domain"
	lApp      layer = "app"
	lAdapters layer = "adapters"
	lAPI      layer = "api(生成物)"
	lTestimg  layer = "testimg"
	lArchtest layer = "archtest"
	lCmd      layer = "cmd"
	lMigr     layer = "migrations"
	lLegacy   layer = "旧パッケージ"
	lOther    layer = "その他"
)

func layerOf(rel string) layer {
	switch {
	case rel == "internal/domain" || strings.HasPrefix(rel, "internal/domain/"):
		return lDomain
	case rel == "internal/app" || strings.HasPrefix(rel, "internal/app/"):
		return lApp
	case rel == "internal/adapters" || strings.HasPrefix(rel, "internal/adapters/"):
		return lAdapters
	case rel == "internal/api":
		return lAPI
	case rel == "internal/testimg":
		return lTestimg
	case rel == "internal/archtest":
		return lArchtest
	case rel == "cmd" || strings.HasPrefix(rel, "cmd/"):
		return lCmd
	case rel == "migrations" || strings.HasPrefix(rel, "migrations/"):
		return lMigr
	case strings.HasPrefix(rel, "internal/"):
		return lLegacy
	}
	return lOther
}

func modulePath(t *testing.T) string {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		t.Fatalf("go.mod を開けない: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(m)
		}
	}
	t.Fatal("go.mod に module 行が無い")
	return ""
}

// goFile は読んだ 1 ファイル。
type goFile struct {
	rel     string // モジュールからの相対パス(/ 区切り)
	dir     string // 置き場所のディレクトリ(モジュールからの相対)
	test    bool
	imports []string
}

// readFiles はモジュールの .go を読む。testdata・node_modules・生成物("Code generated ... DO NOT EDIT.")は除く。
func readFiles(t *testing.T) []goFile {
	t.Helper()
	var out []goFile
	fset := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == "testdata" || n == "node_modules" || (strings.HasPrefix(n, ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if ast.IsGenerated(f) {
			return nil
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		g := goFile{rel: rel, dir: filepath.ToSlash(filepath.Dir(rel)), test: strings.HasSuffix(path, "_test.go")}
		for _, im := range f.Imports {
			p, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			g.imports = append(g.imports, p)
		}
		out = append(out, g)
		return nil
	})
	if err != nil {
		t.Fatalf("ソースを読めない: %v", err)
	}
	return out
}

// AC-A1: internal/domain・internal/app・internal/adapters があり、それぞれに役割を書いた doc.go がある。
func TestLayout_LayerDocs(t *testing.T) {
	for _, d := range []string{"domain", "app", "adapters"} {
		p := filepath.Join(moduleRoot, "internal", d, "doc.go")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("internal/%s/doc.go が無い。層のフォルダを作り、doc.go にその層の役割(何を置き、何を import してよいか)を書く(%s AC-A1)", d, specDoc)
		}
	}
}

// AC-A2: internal/ の直下には層のフォルダ・生成物・テスト用のものだけを置く(旧パッケージを残さない)。
func TestLayout_NoLegacyPackages(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(moduleRoot, "internal"))
	if err != nil {
		t.Fatalf("internal/ を読めない: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() || slices.Contains(allowedTopLevel, e.Name()) {
			continue
		}
		t.Errorf("internal/%s が残っている。%s の「置き場所の表」に従って domain・app・adapters のどれかへ移す(internal/ の直下に置けるのは %v だけ。AC-A2)",
			e.Name(), specDoc, allowedTopLevel)
	}
}

// AC-A3: 移動先のパッケージがそろっている(本番の .go を 1 つ以上持つ)。
func TestLayout_ExpectedPackages(t *testing.T) {
	for _, p := range expectedPackages {
		matches, _ := filepath.Glob(filepath.Join(moduleRoot, filepath.FromSlash(p), "*.go"))
		ok := false
		for _, m := range matches {
			if !strings.HasSuffix(m, "_test.go") {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("%s に本番の .go が無い。%s の「置き場所の表」の移動先へ移す(AC-A3)", p, specDoc)
		}
	}
}

// allowed は from(置き場所・テストか)が to を import してよいか(AC-A4)。
func allowed(from layer, test bool, to layer) bool {
	if to == lLegacy || to == lCmd || to == lOther || to == lArchtest {
		return false // 旧パッケージ・cmd・置き場所の決まっていないものは誰も import しない
	}
	if to == lTestimg && !test {
		return false // テスト用の画像は _test.go からだけ
	}
	switch from {
	case lDomain:
		return to == lDomain || to == lTestimg
	case lApp:
		if test {
			// app のテストは、アダプタ(memory・storage・fetcher など)を組み合わせて手順を確かめてよい
			return to == lDomain || to == lApp || to == lAdapters || to == lTestimg
		}
		return to == lDomain || to == lApp
	case lAdapters:
		return to == lDomain || to == lApp || to == lAdapters || to == lAPI || to == lMigr || to == lTestimg
	case lAPI, lTestimg, lArchtest:
		return false // 生成物・テスト用の部品・このテストはモジュール内を import しない
	case lCmd, lMigr:
		return true // cmd は組み立て役(何でも import してよい)。migrations は外側(MySQL のテストが domain を使う)
	}
	return false
}

func hint(from, to layer) string {
	switch {
	case to == lLegacy:
		return "旧パッケージを import している。移動先(arch-spec.md の置き場所の表)の import パスに直す"
	case from == lDomain && to == lApp:
		return "domain は app を import しない。domain が要る型・規則は domain に置き、手順だけを app に置く"
	case from == lDomain && to == lAdapters:
		return "domain はアダプタを import しない。DB・HTTP・外部サイトの処理は adapters へ移し、型と規則だけを domain に残す"
	case from == lApp && to == lAdapters:
		return "app はアダプタを import しない。要る機能は app にインターフェース(ポート)として宣言し、アダプタがそれを実装する。cmd で組み立てる"
	case to == lAPI:
		return "生成物(internal/api)を使えるのはアダプタ(httpapi)と cmd だけ。app・domain では自前の型を使う"
	case to == lTestimg:
		return "internal/testimg はテスト用。_test.go からだけ使う"
	case to == lCmd:
		return "cmd は組み立て役。他のパッケージから import しない"
	}
	return "向きは adapters → app → domain の一方向だけ"
}

// AC-A4: import の向き。本番コードは adapters → app → domain の一方向。
// _test.go は、app のテストだけアダプタを使ってよい(domain のテストは domain と testimg だけ)。
func TestImports_Direction(t *testing.T) {
	mod := modulePath(t)
	for _, f := range readFiles(t) {
		from := layerOf(f.dir)
		for _, im := range f.imports {
			rel, ok := strings.CutPrefix(im, mod+"/")
			if !ok {
				continue
			}
			to := layerOf(rel)
			if !allowed(from, f.test, to) {
				kind := "本番コード"
				if f.test {
					kind = "テスト"
				}
				t.Errorf("%s(%s・%s)が %s(%s)を import している。%s(%s AC-A4)", f.rel, from, kind, rel, to, hint(from, to), specDoc)
			}
		}
	}
}

// AC-A5: domain・app の本番コードは HTTP・DB・ファイル・ブラウザなど外側のライブラリを import しない。
func TestImports_NoOuterLibsInCore(t *testing.T) {
	for _, f := range readFiles(t) {
		from := layerOf(f.dir)
		if f.test || (from != lDomain && from != lApp) {
			continue
		}
		for _, im := range f.imports {
			for _, bad := range forbiddenOuterLibs {
				base := strings.TrimSuffix(bad, "/")
				if im == base || strings.HasPrefix(im, base+"/") {
					t.Errorf("%s(%s)が %s を import している。外側の処理は adapters に置き、app にはポート(インターフェース)だけを宣言する(%s AC-A5)", f.rel, from, im, specDoc)
				}
			}
		}
	}
}

// AC-A6: from-url の手順は app/fromurl にあり、httpapi の本番コードは ogp(下書きの組み立て)を直接使わない。
func TestImports_FromURLProcedureInApp(t *testing.T) {
	mod := modulePath(t)
	ogp := mod + "/internal/adapters/ogp"
	page := mod + "/internal/domain/page"
	var fromurlUsesPage bool
	for _, f := range readFiles(t) {
		if f.test {
			continue
		}
		if f.dir == "internal/adapters/httpapi" && slices.Contains(f.imports, ogp) {
			t.Errorf("%s が adapters/ogp を import している。from-url の手順(URL の検査・静的な下書き・名前の整形・共有拡張の値の採用・ジャンルの推定)は app/fromurl へ移し、httpapi は変換とエラー整形だけにする(%s AC-A6)", f.rel, specDoc)
		}
		if f.dir == "internal/app/fromurl" && slices.Contains(f.imports, page) {
			fromurlUsesPage = true
		}
	}
	if !fromurlUsesPage {
		t.Errorf("internal/app/fromurl が domain/page(URL の検査・StaticDraft・CleanTitle)を使っていない。from-url の手順を app/fromurl に置く(%s AC-A6)", specDoc)
	}
}
