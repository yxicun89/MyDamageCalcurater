package httpapi_test

// GET /internal/pokedex/master が全 SELECT を1つの読み取り専用トランザクション(一貫したスナップショット)の
// 中で行うことのテスト(issue #220・ADR-0127)。import の全置換の commit が SELECT の間に入っても、
// data_versions と各テーブルが同じ commit 由来になるようにするため。実 MySQL での一貫性は
// services/pokedex/importer/snapshot_mysql_test.go(-tags mysql)が確かめる。

import (
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/readtx"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// masterReads は内部 API が読むクエリ(buildMasterExport の全 SELECT)。
var masterReads = []string{
	"ListDataVersions", "ListTypes", "ListTypeChart", "ListSpecies", "ListAllSpeciesAbilities",
	"ListMoves", "ListMoveEffects", "ListMoveMechanisms", "ListItems", "ListItemEffects",
	"ListAbilities", "ListAbilityEffects", "ListNatures",
}

// 偽の Querier がハンドラの受け取る型(readtx.DB)を満たすこと(コンパイル時の確認)。
var _ readtx.DB = (*storetest.Querier)(nil)

// AC-S1: 正常系。BeginTx(ReadOnly)をちょうど1回開き、全 SELECT をその Tx の中で行い、最後に Commit する。
func TestMasterExportReadsInOneReadOnlySnapshot(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	for _, p := range q.SnapshotViolations(masterReads) {
		t.Error(p)
	}
}

// AC-S2: 異常系。トランザクションを開けなければ 503 master_unavailable(既存の unavailable と同じ写し方)。
// DB の内部エラーの文言を出さず、SELECT を1つも発行しない(autocommit に逃げない)。
func TestMasterExportBeginTxFailureIsUnavailable(t *testing.T) {
	q := storetest.New()
	q.ErrByMethod = map[string]error{storetest.MethodBeginTx: storetest.ErrDB}
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	if strings.Contains(rec.Body.String(), storetest.ErrDB.Error()) {
		t.Errorf("DB の内部エラーの文言を応答に出している: %s", rec.Body.String())
	}
	for _, c := range q.Calls {
		if c.Method != storetest.MethodBeginTx {
			t.Errorf("BeginTx が失敗したのに %s を呼んだ", c.Method)
		}
	}
	for _, p := range q.RollbackViolations() {
		t.Error(p)
	}
}

// AC-S3: 異常系。Tx の中の失敗(クエリの失敗・未投入・効果の JSON が壊れている)は 503 master_unavailable で、
// Tx を Rollback で閉じ、Commit しない(開いたままの Tx を残さない)。
func TestMasterExportRollsBackOnFailure(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *storetest.Querier)
	}{
		{"クエリが失敗", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListMoves": storetest.ErrDB}
		}},
		{"data_versions が空(未投入)", func(q *storetest.Querier) { q.DataVersions = nil }},
		{"natures が空", func(q *storetest.Querier) { q.Natures = nil }},
		{"効果の JSON が壊れている", func(q *storetest.Querier) {
			q.ItemEffects[0].Effect = []byte(`{"DamageMod":`)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, masterPath, false)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			for _, p := range q.RollbackViolations() {
				t.Error(p)
			}
		})
	}
}

// AC-S4: 異常系。Commit の失敗も 503 master_unavailable(スナップショットを正しく閉じられなかった応答を 200 で返さない)。
func TestMasterExportCommitFailureIsUnavailable(t *testing.T) {
	q := storetest.New()
	q.ErrByMethod = map[string]error{storetest.MethodCommit: storetest.ErrDB}
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	if open := q.OpenTxCount(); open != 0 {
		t.Errorf("開いたままのトランザクションが %d 個ある", open)
	}
}

// AC-S5: 1リクエストごとに新しいスナップショットを開く(Tx を使い回さない。前の応答の Tx は閉じている)。
func TestMasterExportOpensSnapshotPerRequest(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	for i := 0; i < 2; i++ {
		if rec := do(t, h, http.MethodGet, masterPath, false); rec.Code != http.StatusOK {
			t.Fatalf("%d 回目: status = %d, want 200", i+1, rec.Code)
		}
	}
	if begins := len(q.CallsOf(storetest.MethodBeginTx)); begins != 2 {
		t.Errorf("BeginTx の回数 = %d, want 2(1リクエストに1回)", begins)
	}
	if open := q.OpenTxCount(); open != 0 {
		t.Errorf("開いたままのトランザクションが %d 個ある", open)
	}
}
