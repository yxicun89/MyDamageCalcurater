package readmodel_test

// Export(`pokedex export`)が全 SELECT を1つの読み取り専用トランザクション(一貫したスナップショット)の
// 中で行うことのテスト(issue #220・ADR-0127)。実 MySQL での一貫性は
// services/pokedex/importer/snapshot_mysql_test.go(-tags mysql)が確かめる。

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/readtx"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// exportReads は Export が読むクエリ(全 SELECT)。
var exportReads = []string{
	"GetDefaultRegulation", "ListRegulationSpeciesKeys", "ListRegulationMoveIDs", "ListRegulationAbilityIDs",
	"ListTypes", "ListTypeChart", "ListSpecies", "ListAllSpeciesAbilities", "ListMoves", "ListAbilityEffects",
}

// 偽の Querier が Export の受け取る型(readtx.Beginner)を満たすこと(コンパイル時の確認)。
var _ readtx.Beginner = (*storetest.Querier)(nil)

// AC-S1: 正常系。BeginTx(ReadOnly)をちょうど1回開き、全 SELECT をその Tx の中で行い、最後に Commit する。
func TestExportReadsInOneReadOnlySnapshot(t *testing.T) {
	q := storetest.New()
	export(t, q)
	for _, p := range q.SnapshotViolations(exportReads) {
		t.Error(p)
	}
}

// AC-S2: 異常系。トランザクションを開けなければ失敗(原因を errors.Is で辿れる)し、Files はゼロ値。
// SELECT を1つも発行しない(autocommit に逃げない)。
func TestExportBeginTxFailure(t *testing.T) {
	q := storetest.New()
	q.ErrByMethod = map[string]error{storetest.MethodBeginTx: storetest.ErrDB}
	files, _, err := readmodel.Export(context.Background(), q)
	if !errors.Is(err, storetest.ErrDB) {
		t.Fatalf("err = %v, want storetest.ErrDB", err)
	}
	if !reflect.DeepEqual(files, readmodel.Files{}) {
		t.Errorf("失敗時に部分的な出力を返した")
	}
	for _, c := range q.Calls {
		if c.Method != storetest.MethodBeginTx {
			t.Errorf("BeginTx が失敗したのに %s を呼んだ", c.Method)
		}
	}
}

// AC-S3: 異常系。Tx の中の失敗は既存のエラー(ErrNoDefaultRegulation・ErrInvalidExport・DB の失敗)のまま返し、
// Tx を Rollback で閉じ、Commit しない。
func TestExportRollsBackOnFailure(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(q *storetest.Querier)
		wantErr error
	}{
		{"既定のレギュレーションが無い", func(q *storetest.Querier) { q.DefaultRegulation = nil }, readmodel.ErrNoDefaultRegulation},
		{"使用可能な種族が0件", func(q *storetest.Querier) {
			q.RegulationSpecies = map[string][]string{}
		}, readmodel.ErrInvalidExport},
		{"クエリが失敗", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListMoves": storetest.ErrDB}
		}, storetest.ErrDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			files, _, err := readmodel.Export(context.Background(), q)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(files, readmodel.Files{}) {
				t.Errorf("失敗時に部分的な出力を返した")
			}
			for _, p := range q.RollbackViolations() {
				t.Error(p)
			}
		})
	}
}

// AC-S4: 異常系。Commit の失敗も失敗として返す(Files はゼロ値。スナップショットを正しく閉じられなかった出力を書かない)。
func TestExportCommitFailure(t *testing.T) {
	q := storetest.New()
	q.ErrByMethod = map[string]error{storetest.MethodCommit: storetest.ErrDB}
	files, _, err := readmodel.Export(context.Background(), q)
	if !errors.Is(err, storetest.ErrDB) {
		t.Fatalf("err = %v, want storetest.ErrDB", err)
	}
	if !reflect.DeepEqual(files, readmodel.Files{}) {
		t.Errorf("失敗時に部分的な出力を返した")
	}
	if open := q.OpenTxCount(); open != 0 {
		t.Errorf("開いたままのトランザクションが %d 個ある", open)
	}
}
