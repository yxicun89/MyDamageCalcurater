package storetest_test

// storetest はテスト専用の偽の store.Querier と架空データ(ADR-0105 §7)。本番のコードから
// import してはいけない(パッケージ doc コメントの規約)。この静的検査はソースを解析するだけで、
// パッケージ自体には依存しない(import 検査の対象にならないように)。

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// storetestImportPath は禁止する import path(モジュール名は書かず末尾の一致で見る。
// vendoring や module rename があっても検査が壊れないように)。
const storetestImportSuffix = "services/pokedex/internal/storetest"

// TestStoretestIsNotImportedByProductionCode は services/pokedex 配下の *_test.go 以外の
// .go ファイルが storetest を import していないことを確かめる(ADR-0105 §1)。
func TestStoretestIsNotImportedByProductionCode(t *testing.T) {
	root := ".."                             // services/pokedex/internal
	pokedexRoot := filepath.Join(root, "..") // services/pokedex
	fset := token.NewFileSet()

	err := filepath.WalkDir(pokedexRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// storetest 自身のソース(storetest.go)は対象外(自己参照ではないので該当しないが、念のため除く)。
		if strings.Contains(filepath.ToSlash(path), storetestImportSuffix) {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(p, storetestImportSuffix) {
				t.Errorf("%s が storetest を import している(本番のコードから import しない。*_test.go だけが使える)", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
