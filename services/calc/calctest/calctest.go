// Package calctest は、他のサービス(gateway)のテストが calc-svc の実物を架空マスタで起動するための補助
// (ADR-0020 §テスト)。calc-svc の httpapi / master は services/calc/internal にあり、Go の internal 規則で
// calc の外からは import できないため、この薄い入口だけを公開する。本番コードから使わない。
//
// マスタは services/calc/testdata/master.example.json(架空データ)、相性表は testdata/golden/typechart.json
// (数値と英語 ID のみ。ADR-0015 と同じ扱い)を読む。パスはこのファイルの位置から解決するので、
// 呼び出し側のテストの作業ディレクトリに依存しない。
package calctest

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"example.com/pokecalc/services/calc/internal/httpapi"
	"example.com/pokecalc/services/calc/internal/master"
)

// 例のマスタに含まれる架空の ID(services/calc/testdata/master.example.json)。
// 呼び出し側がリクエストを組み立てるときに使う。
const (
	SpeciesAttacker = "9001-000" // テストモン(normal)
	SpeciesDefender = "9002-000" // テストガード(water/steel)
	MovePhysical    = "test-beam"
	NatureAtkUp     = "test-atk-up"
	NatureNeutral   = "test-neutral-a"
)

// NewExampleHandler は例のマスタと共有の相性表で calc-svc の HTTP ハンドラ(httpapi.NewHandler)を作る。
func NewExampleHandler() (http.Handler, error) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("calctest: 自身のソースの位置を得られない")
	}
	calcDir := filepath.Dir(filepath.Dir(self)) // services/calc
	masterPath := filepath.Join(calcDir, "testdata", "master.example.json")
	chartPath := filepath.Join(calcDir, "..", "..", "testdata", "golden", "typechart.json")

	mf, err := os.Open(masterPath)
	if err != nil {
		return nil, fmt.Errorf("calctest: 例のマスタを開けない: %w", err)
	}
	defer mf.Close()
	snapshot, err := master.LoadSnapshot(mf)
	if err != nil {
		return nil, fmt.Errorf("calctest: 例のマスタを読めない: %w", err)
	}
	tf, err := os.Open(chartPath)
	if err != nil {
		return nil, fmt.Errorf("calctest: 相性表を開けない: %w", err)
	}
	defer tf.Close()
	chart, err := master.LoadTypeChart(tf)
	if err != nil {
		return nil, fmt.Errorf("calctest: 相性表を読めない: %w", err)
	}
	store, err := master.New(snapshot, chart)
	if err != nil {
		return nil, fmt.Errorf("calctest: マスタの整合性検査に失敗: %w", err)
	}
	return httpapi.NewHandler(store), nil
}
