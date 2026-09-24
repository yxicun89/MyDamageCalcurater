package db

// docs/runbooks/data.md の静的検査(issue #112 / ADR-0112 決定6)。DB は使わない(make test で走る)。
// repoRoot・readRepoFile は layout_test.go(同じパッケージ)のものを使う。

import (
	"regexp"
	"strings"
	"testing"
)

// AC6: replica を増やすときの接続予算の注記(replicas × MaxOpenConns + importer/migrate の
// 接続予算 < DB の max_connections)が docs/runbooks/data.md にある。
func TestRunbookDataNotesReplicaConnectionBudget(t *testing.T) {
	s := readRepoFile(t, "docs/runbooks/data.md")
	for _, want := range []string{"MaxOpenConns", "max_connections"} {
		if !strings.Contains(s, want) {
			t.Errorf("docs/runbooks/data.md に %q が無い(replica を増やすときの接続予算の注記)", want)
		}
	}
	if !regexp.MustCompile(`(?i)replica`).MatchString(s) {
		t.Error("docs/runbooks/data.md に replica についての言及が無い")
	}
	if !regexp.MustCompile(`(?i)import|migrat`).MatchString(s) {
		t.Error("docs/runbooks/data.md に importer/migrate の接続予算についての言及が無い")
	}
}
