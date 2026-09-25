// Command import は取得済みのスナップショット(data/)を読み、変換し、pokedex の DB に
// 冪等に投入する CLI(ADR-0101 §11・ADR-0104 §3・§10)。ネットワークには触らない
// (取得は tools/importer、上流の最新版の検出結果はファイルで受け取るだけ)。
//
//	import -data <dir> [-dry-run] [-force] [-typechart <path>] [-upstream <path>] [-upstream-max-age <duration>]
//
// 照合では、取り込む相性表を参照の相性表(既定は <data>/../testdata/golden/typechart.json)と比べ、
// 食い違いは Blocker にする(issue #280・ADR-0118)。
//
// DSN(go-sql-driver/mysql 形式)は環境変数 POKEDEX_DATABASE_DSN から読む(-dry-run のときは不要)。
//
// 終了コード(CronJob の podFailurePolicy が使う。ADR-0104 §3):
//
//	0 成功(投入した・取得元の版と変換結果が同じでスキップ・dry-run)
//	1 再試行で直りうる失敗(DB に接続できない・報告を書けない・その他の I/O)
//	2 使い方・設定の誤り(フラグの誤り・POKEDEX_DATABASE_DSN が無い)
//	3 人間の対応が要る(ErrBlocked・ErrKeyChanged・ErrInvalidInput・ErrInvalidData・
//	  master.ErrInvalidEffect・ErrSchemaNotReady・DB の制約違反)。DB は変えない
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

// cliEnv は run が使う環境(テストのため差し替え可能にする。ADR-0104 §10)。
type cliEnv struct {
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	OpenStore      func(dsn string) (importer.Store, io.Closer, error)
	Now            func() time.Time
}

func main() {
	os.Exit(run(os.Args[1:], productionEnv()))
}

func productionEnv() cliEnv {
	return cliEnv{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		OpenStore: openSQLStore,
		Now:       func() time.Time { return time.Now().UTC() },
	}
}

// openSQLStore は DSN から *sql.DB を開き、importer.Store として包む。db 自身が io.Closer。
func openSQLStore(dsn string) (importer.Store, io.Closer, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, nil, err
	}
	return importer.NewSQLStore(db), db, nil
}

func run(args []string, env cliEnv) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	dataDir := fs.String("data", "../data", "取得済みスナップショット・設定一式のディレクトリ")
	dryRun := fs.Bool("dry-run", false, "変換と報告だけ行い、DB には触らない")
	force := fs.Bool("force", false, "取得元の版と変換結果に変化が無くても投入する")
	typeChartPath := fs.String("typechart", "", "照合する参照の相性表(空なら <data>/../testdata/golden/typechart.json)")
	upstreamPath := fs.String("upstream", "", "上流の最新版の検出結果ファイル(空なら表示しない)")
	upstreamMaxAge := fs.Duration("upstream-max-age", 24*time.Hour, "checkedAt がこれより古い検出結果は unknown 扱いにする")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	in, versions, err := importer.LoadInput(*dataDir)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import:", err)
		return classifyErr(err)
	}

	refPath := *typeChartPath
	if refPath == "" {
		refPath = importer.ReferenceTypeChartDefaultPath(*dataDir)
	}
	in.ReferenceTypeChart, err = importer.LoadReferenceTypeChart(refPath)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import:", err)
		return classifyErr(err)
	}

	printUpstream(env, in.Config.Sources, *upstreamPath, *upstreamMaxAge, env.Now())

	out, reconciliation, err := importer.Reconcile(in)
	// Reconcile は ErrInvalidInput/ErrInvalidData/master.ErrInvalidEffect のとき Reconciliation を
	// ゼロ値で返す(ADR-0103 §1)。中身の無い報告で latest.json を上書きしないよう、
	// nil か ErrBlocked のときだけ書く。
	if err == nil || errors.Is(err, importer.ErrBlocked) {
		reportsDir := filepath.Join(*dataDir, "generated", "reports")
		if writeErr := importer.WriteReconciliation(reportsDir, reconciliation, env.Now()); writeErr != nil {
			fmt.Fprintln(env.Stderr, "import: 報告を書けない:", writeErr)
			return 1
		}
	}
	if err != nil {
		if errors.Is(err, importer.ErrBlocked) {
			fmt.Fprintf(env.Stderr, "import: 人間の裁定が必要な食い違いが %d 件ある(報告を参照)\n", len(reconciliation.Report.Blockers))
		} else {
			fmt.Fprintln(env.Stderr, "import:", err)
		}
		return classifyErr(err)
	}
	fmt.Fprintf(env.Stdout, "import: 照合完了(警告 %d 件)\n", len(reconciliation.Report.Warnings))
	if summary := strings.TrimSpace(importer.FormatSummary(reconciliation)); summary != "" {
		fmt.Fprintln(env.Stdout, summary)
	}

	// 変換結果の版(取得元が同じでも、変換ロジックやスキーマの変更で変わる。ADR-0122)。
	// 投入の判定は RunStore が同じ関数で行う。ここでは kubectl logs と dry-run で比べられるよう表示だけする。
	outVersion, err := importer.OutputVersion(out)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import:", err)
		return classifyErr(err)
	}
	fmt.Fprintf(env.Stdout, "import: 変換結果の版 %s=%s\n", outVersion.Source, outVersion.Version)

	if *dryRun {
		fmt.Fprintln(env.Stdout, "import: -dry-run のため DB には投入しない")
		return 0
	}

	dsn := env.Getenv("POKEDEX_DATABASE_DSN")
	if dsn == "" {
		fmt.Fprintln(env.Stderr, "import: POKEDEX_DATABASE_DSN が設定されていない")
		return 2
	}

	store, closer, err := env.OpenStore(dsn)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import: DB を開けない:", err)
		return 1
	}
	defer closer.Close() //nolint:errcheck // 投入の成否とは無関係

	applied, err := importer.RunStore(context.Background(), store, out, versions, env.Now(), *force)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import: 投入に失敗:", err)
		return classifyErr(err)
	}
	if applied {
		fmt.Fprintln(env.Stdout, "import: 投入完了")
	} else {
		fmt.Fprintln(env.Stdout, "import: 取得元の版と変換結果に変化が無いのでスキップ")
	}
	return 0
}

// printUpstream は上流の最新版の検出結果(path)を読み、pinned(config.json の sources)と比べて
// 表示する。ファイルが無い・壊れている・古いときは警告だけ出して続ける(取り込みを止めない。
// ADR-0104 §3・§6)。path が空なら何もしない。
func printUpstream(env cliEnv, pinned map[string]string, path string, maxAge time.Duration, now time.Time) {
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import: 上流の検出結果を読めない(無視して続ける):", err)
		return
	}
	latest, err := importer.DecodeUpstreamLatest(raw)
	if err != nil {
		fmt.Fprintln(env.Stderr, "import: 上流の検出結果の形式が不正(無視して続ける):", err)
		return
	}
	statuses := importer.CompareUpstream(pinned, latest, now, maxAge)
	if out := importer.FormatUpstream(statuses); out != "" {
		fmt.Fprintln(env.Stdout, out)
	}
}

// dataErrorCodes は投入(Apply)で MySQL が返す、データ自体が制約に合わないことを示すエラー番号。
// 同じ入力で再試行しても同じ結果になるので、終了コード 3(人間の対応が要る)に分類する(#310)。
// トランザクションは巻き戻るので DB は変わらない。
var dataErrorCodes = map[uint16]bool{
	1062: true, // ER_DUP_ENTRY(重複)
	1264: true, // ER_WARN_DATA_OUT_OF_RANGE(列の型の範囲外。strict モード)
	1406: true, // ER_DATA_TOO_LONG(列の長さを超える。strict モード)
	1452: true, // ER_NO_REFERENCED_ROW_2(外部キー違反)
	3819: true, // ER_CHECK_CONSTRAINT_VIOLATED(CHECK 違反)
}

// classifyErr は ADR-0104 §3 の終了コード 3(人間の対応が要る。再試行しても同じ。DB は変えない)
// に当たるかを判定する。それ以外は 1(再試行で直りうる失敗)。
func classifyErr(err error) int {
	for _, sentinel := range []error{
		importer.ErrBlocked,
		importer.ErrInvalidInput,
		importer.ErrInvalidData,
		master.ErrInvalidEffect,
		importer.ErrKeyChanged,
		importer.ErrSchemaNotReady,
	} {
		if errors.Is(err, sentinel) {
			return 3
		}
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) && dataErrorCodes[myErr.Number] {
		return 3
	}
	return 1
}
