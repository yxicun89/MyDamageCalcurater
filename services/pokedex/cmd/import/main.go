// Command import は取得済みのスナップショット(data/)を読み、変換し、pokedex の DB に
// 冪等に投入する CLI(ADR-0101 §11)。ネットワークには触らない(取得は tools/importer)。
//
//	import -data <dir> [-dry-run] [-force]
//
// DSN(go-sql-driver/mysql 形式)は環境変数 POKEDEX_DATABASE_DSN から読む(-dry-run のときは不要)。
// ErrBlocked(人間の裁定が必要な食い違い)のときは報告を書いて非0で終わる。
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/pokedex/importer"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	dataDir := fs.String("data", "../data", "取得済みスナップショット・設定一式のディレクトリ")
	dryRun := fs.Bool("dry-run", false, "変換と報告だけ行い、DB には触らない")
	force := fs.Bool("force", false, "取得元の版に変化が無くても投入する")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	in, versions, err := importer.LoadInput(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		return 1
	}

	out, reconciliation, err := importer.Reconcile(in)
	// Reconcile は ErrInvalidInput/ErrInvalidData/ErrInvalidEffect のとき Reconciliation をゼロ値で返す
	// (ADR-0103 §1)。中身の無い報告で latest.json を上書きしないよう、nil か ErrBlocked のときだけ書く。
	if err == nil || errors.Is(err, importer.ErrBlocked) {
		reportsDir := filepath.Join(*dataDir, "generated", "reports")
		if writeErr := importer.WriteReconciliation(reportsDir, reconciliation, time.Now().UTC()); writeErr != nil {
			fmt.Fprintln(os.Stderr, "import: 報告を書けない:", writeErr)
			return 1
		}
	}
	if err != nil {
		if errors.Is(err, importer.ErrBlocked) {
			fmt.Fprintf(os.Stderr, "import: 人間の裁定が必要な食い違いが %d 件ある(報告を参照)\n", len(reconciliation.Report.Blockers))
		} else {
			fmt.Fprintln(os.Stderr, "import:", err)
		}
		return 1
	}
	fmt.Printf("import: 照合完了(警告 %d 件)\n", len(reconciliation.Report.Warnings))

	if *dryRun {
		fmt.Println("import: -dry-run のため DB には投入しない")
		return 0
	}

	dsn := os.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "import: POKEDEX_DATABASE_DSN が設定されていない")
		return 1
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import: DB を開けない:", err)
		return 1
	}
	defer db.Close()

	applied, err := importer.Run(context.Background(), db, out, versions, time.Now().UTC(), *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import: 投入に失敗:", err)
		return 1
	}
	if applied {
		fmt.Println("import: 投入完了")
	} else {
		fmt.Println("import: 版に変化が無いのでスキップ")
	}
	return 0
}
