package main

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		list []string
		want []string // 違反の文に含まれるべき文字列(空なら違反なし)
	}{
		{"ok", nil, nil},
		{"sibling_ok", nil, nil}, // 別サービスの adapters は(サービス間 import は別の規則)この検査の対象外
		{"domain_to_adapters", nil, []string{"internal/domain/bad.go", "internal/adapters"}},
		{"app_to_adapters", nil, []string{"internal/app/bad.go", "internal/adapters"}},
		{"domain_to_app", nil, []string{"internal/domain/bad.go", "internal/app"}},
		{"app_to_api", nil, []string{"internal/app/bad.go", "internal/api"}},
		{"shared_outer", nil, []string{"internal/app/bad.go", "外側の共有パッケージ"}},
		{"external", nil, []string{"internal/domain/bad.go", "internal/app/bad2.go", "HTTP・DB・NATS"}},
		{"new_unshaped", nil, []string{"services/svc", "最初からそろえて"}},
		{"unmigrated_pass", []string{"svc"}, nil},
		{"listed_but_shaped", []string{"svc"}, []string{"unmigrated", "消して"}},
		{"stale_entry", []string{"real", "ghost"}, []string{"\"ghost\"", "存在しない"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := check("testdata/"+tt.name, tt.list)
			if err != nil {
				t.Fatal(err)
			}
			all := strings.Join(got, "\n")
			if len(tt.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("違反なしのはずが: %s", all)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("違反が出るはずが 0 件")
			}
			for _, w := range tt.want {
				if !strings.Contains(all, w) {
					t.Errorf("%q を含む違反が無い:\n%s", w, all)
				}
			}
		})
	}
}

// 実リポジトリの未移行の一覧は、実在するサービスと一致している(archcheck が今の main で通る)。
func TestRealRepositoryPasses(t *testing.T) {
	got, err := check("../..", unmigrated)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("実リポジトリで違反: %v", got)
	}
}
