// Package calctest は、他のサービス(gateway)のテストが calc-svc の実物を架空マスタで起動するための補助
// (ADR-0202 §テスト)。calc-svc の httpapi / master は services/calc/internal にあり、Go の internal 規則で
// calc の外からは import できないため、この薄い入口だけを公開する。本番コードから使わない。
//
// マスタは services/calc/testdata/master.example.json(MasterExport の形の架空データ。相性表を含む。ADR-0204)を読む。
// パスはこのファイルの位置から解決するので、呼び出し側のテストの作業ディレクトリに依存しない。
package calctest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	MovePhysical    = "testbeam"
	NatureAtkUp     = "testatkup"
	NatureNeutral   = "testneutrala"
)

// NewExampleHandler は例のマスタで calc-svc の HTTP ハンドラ(httpapi.NewHandler)を作る。
func NewExampleHandler() (http.Handler, error) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("calctest: 自身のソースの位置を得られない")
	}
	calcDir := filepath.Dir(filepath.Dir(self)) // services/calc
	masterPath := filepath.Join(calcDir, "testdata", "master.example.json")

	export, err := master.FileSource{Path: masterPath}.Fetch(context.Background())
	if err != nil {
		return nil, fmt.Errorf("calctest: 例のマスタを読めない: %w", err)
	}
	store, err := master.FromExport(export)
	if err != nil {
		return nil, fmt.Errorf("calctest: 例のマスタの検証に失敗: %w", err)
	}
	return httpapi.NewHandler(store, nil), nil // publisher なし(テスト用途。ADR-0212)
}
