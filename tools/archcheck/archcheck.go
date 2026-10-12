// Package main は Go サービスの import の向きを検査する(ADR-0811)。
//
// 規則は 1 種類だけ: adapters → app → domain(内側は外側を import しない)。
//   - internal/domain は、同じサービスの app・adapters・api、外側の共有パッケージ、
//     echo・DB ドライバ・NATS を import しない。
//   - internal/app は、同じサービスの adapters・api、外側の共有パッケージ、
//     echo・DB ドライバ・NATS を import しない。
//   - internal/adapters は制限なし。
//
// 検査の対象は、internal/{domain,app,adapters} がそろった「移行済み」のサービスだけ。
// まだそろっていないサービスは unmigrated(未移行の一覧)に載せ、一覧のサービスは検査しない。
// 一覧に無いサービスで形がそろっていなければ失敗する(新しいサービスは最初から形をそろえる)。
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// unmigrated は、まだ形(domain・app・adapters)をそろえていないサービスの一覧。
// 段階の PR(docs/plan/architecture-adoption.md §6)でサービスごとに 1 行ずつ消す。空になったら仕組みごと消す。
var unmigrated = []string{
	"balance",
	"judge",
	"record",
	"speed",
	"team",
}

// layers は、形をそろえるときに必ず置く 3 つのフォルダ。
var layers = []string{"domain", "app", "adapters"}

// outerShared は、外側として扱う共有パッケージ(services/internal 以下)。master は内側なので含めない。
var outerShared = []string{"api", "calcevents", "dbmigrate", "httpguard", "httpmetrics", "reqlog", "version"}

// outerExternal は、内側(domain・app)から使ってはいけない外部のパッケージ(前方一致)。
var outerExternal = []string{
	"github.com/labstack/echo", // echo/v4 など版付きも "/" 区切りで一致する
	"net/http",
	"database/sql",
	"github.com/go-sql-driver/",
	"github.com/jackc/",
	"github.com/nats-io/",
}

var sharedRe = regexp.MustCompile(`(^|/)services/internal/(` + strings.Join(outerShared, "|") + `)(/|$)`)

// check は root(services の 1 つ上。testdata では擬似リポジトリ)の services/ を検査し、違反の一覧を返す。
func check(root string, list []string) ([]string, error) {
	svcRoot := filepath.Join(root, "services")
	entries, err := os.ReadDir(svcRoot)
	if err != nil {
		return nil, fmt.Errorf("services を読めません: %w", err)
	}
	inList := map[string]bool{}
	for _, s := range list {
		inList[s] = true
	}
	var violations []string
	exist := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "internal" {
			continue
		}
		svc := e.Name()
		exist[svc] = true
		shaped := isShaped(filepath.Join(svcRoot, svc))
		switch {
		case inList[svc] && shaped:
			violations = append(violations, fmt.Sprintf("services/%s: internal/domain・app・adapters がそろっています。tools/archcheck の unmigrated(未移行の一覧)から %q を消してください", svc, svc))
		case inList[svc]:
			// 未移行: 検査しない。
		case !shaped:
			violations = append(violations, fmt.Sprintf("services/%s: 新しいサービスは internal/domain・app・adapters を最初からそろえてください(中身が無いフォルダも doc.go に役割を書いて置く。docs/coding-rules.md「Go のサービスの形」)", svc))
		default:
			v, err := checkService(root, svc)
			if err != nil {
				return nil, err
			}
			violations = append(violations, v...)
		}
	}
	for _, s := range list {
		if !exist[s] {
			violations = append(violations, fmt.Sprintf("tools/archcheck の unmigrated に、存在しないサービス %q があります。消してください", s))
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func isShaped(svcDir string) bool {
	for _, l := range layers {
		if st, err := os.Stat(filepath.Join(svcDir, "internal", l)); err != nil || !st.IsDir() {
			return false
		}
	}
	return true
}

func checkService(root, svc string) ([]string, error) {
	var violations []string
	base := filepath.Join(root, "services", svc, "internal")
	own := func(layer string) *regexp.Regexp {
		return regexp.MustCompile(`(^|/)` + regexp.QuoteMeta(svc) + `/internal/` + layer + `(/|$)`)
	}
	forbidden := map[string][]string{
		"domain": {"app", "adapters", "api"},
		"app":    {"adapters", "api"},
	}
	for layer, targets := range forbidden {
		var res []*regexp.Regexp
		var names []string
		for _, t := range targets {
			res = append(res, own(t))
			names = append(names, t)
		}
		dir := filepath.Join(base, layer)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if perr != nil {
				return fmt.Errorf("%s を解析できません: %w", path, perr)
			}
			rel, _ := filepath.Rel(root, path)
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if why := reason(p, res, names, layer); why != "" {
					violations = append(violations, fmt.Sprintf("%s: %q を import しています。%s", filepath.ToSlash(rel), p, why))
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return violations, nil
}

func reason(p string, res []*regexp.Regexp, names []string, layer string) string {
	for i, re := range res {
		if re.MatchString(p) {
			return fmt.Sprintf("internal/%s は同じサービスの internal/%s を import できません(向きは adapters → app → domain)。必要なものはポート(インターフェース)を %s 側で宣言し、%s 側から実装を渡してください", layer, names[i], layer, names[i])
		}
	}
	if sharedRe.MatchString(p) {
		return fmt.Sprintf("internal/%s は外側の共有パッケージを import できません。adapters か cmd に移し、ポートを経由してください", layer)
	}
	for _, x := range outerExternal {
		if p == x || strings.HasPrefix(p, x+"/") || (strings.HasSuffix(x, "/") && strings.HasPrefix(p, x)) {
			return fmt.Sprintf("internal/%s は HTTP・DB・NATS の部品を import できません。adapters に移してください", layer)
		}
	}
	return ""
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	violations, err := check(root, unmigrated)
	if err != nil {
		fmt.Fprintln(os.Stderr, "archcheck:", err)
		os.Exit(2)
	}
	for _, v := range violations {
		fmt.Fprintln(os.Stderr, "archcheck:", v)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "archcheck: 違反 %d 件(規則: docs/coding-rules.md「Go のサービスの形」・ADR-0811)\n", len(violations))
		os.Exit(1)
	}
	fmt.Printf("archcheck: ok(未移行 %d サービス: %s)\n", len(unmigrated), strings.Join(unmigrated, ", "))
}
