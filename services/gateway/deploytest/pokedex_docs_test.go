package deploytest_test

// calc・gateway を pokedex-svc につないだ変更の文書の検査(ADR-0206。AC-P10)。
// README は人が読むものなので、文面そのものは固定しない。入手元が変わったことが分かる最小限
// (環境変数の既定値・ファイル方式の位置づけ・初回の投入)だけを確かめる。

import (
	"strings"
	"testing"
)

// AC-P10: gateway の README の環境変数の表に GATEWAY_POKEDEX_URL の既定(Service 名 pokedex)があり、
// calc の README は k3d を URL 方式として書いている(ファイル方式は `make dev` とテスト用)。
// どちらかが、DB が空のクラスタでは初回の投入(`make import-k8s`)が要ることに触れている。ADR-0206 がある。
func TestPokedexWiringIsDocumented(t *testing.T) {
	gateway := readRepoFile(t, "services/gateway/README.md")
	row := ""
	for _, line := range strings.Split(gateway, "\n") {
		if strings.HasPrefix(line, "| `GATEWAY_POKEDEX_URL`") {
			row = line
			break
		}
	}
	switch {
	case row == "":
		t.Error("services/gateway/README.md の環境変数の表に `GATEWAY_POKEDEX_URL` の行が無い")
	case !strings.Contains(row, "http://pokedex"):
		t.Errorf("services/gateway/README.md の `GATEWAY_POKEDEX_URL` の行に base の既定 http://pokedex が無い: %s", row)
	}

	calc := readRepoFile(t, "services/calc/README.md")
	if !strings.Contains(calc, "CALC_MASTER_URL") {
		t.Error("services/calc/README.md に CALC_MASTER_URL が無い")
	}
	// ファイル方式は `make dev` とテストだけになった(k3d は URL 方式。ADR-0206 §1)。
	for _, stale := range []string{"k3d の local overlay", "k3d local overlay"} {
		if strings.Contains(calc, stale) {
			t.Errorf("services/calc/README.md に %q が残っている(k3d は pokedex-svc から取る)", stale)
		}
	}
	if !strings.Contains(calc, "make import-k8s") && !strings.Contains(gateway, "make import-k8s") {
		t.Error("calc / gateway のどちらの README も、DB が空のクラスタで初回の `make import-k8s` が要ることに触れていない")
	}

	adr := readRepoFile(t, "docs/adr/0206-wire-to-pokedex-svc.md")
	for _, want := range []string{"GATEWAY_POKEDEX_URL", "CALC_MASTER_URL"} {
		if !strings.Contains(adr, want) {
			t.Errorf("docs/adr/0206-wire-to-pokedex-svc.md に %s の記述が無い", want)
		}
	}
}
