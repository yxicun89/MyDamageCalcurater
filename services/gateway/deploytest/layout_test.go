package deploytest_test

// API レーンの配置物(Kustomize の組み込み・Dockerfile・Makefile・scripts/dev.sh)の静的検査(ADR-0203。AC-S2〜S5・AC-S7)。

import (
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

// apiServices は API レーンが k3d に載せるサービス(Deployment・Service・イメージの名前)。
var apiServices = []string{"calc", "gateway"}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(deploytest.RepoPath(t, rel))
	if err != nil {
		t.Fatalf("%s を読めない: %v", rel, err)
	}
	return string(raw)
}

// AC-S3: base の kustomization.yaml の resources に calc と gateway が1行ずつある(共有ファイル。既存の行は残す)。
func TestBaseKustomizationListsAPIServices(t *testing.T) {
	k := deploytest.ReadKustomization(t, deploytest.BaseDir)
	for _, want := range append([]string{"namespace.yaml", "pokedex"}, apiServices...) {
		if n := countOf(k.Resources, want); n != 1 {
			t.Errorf("%s/kustomization.yaml の resources に %q が %d 個(1個であること)。resources=%q",
				deploytest.BaseDir, want, n, k.Resources)
		}
	}
}

// AC-S4: local overlay は API の Component(api)を components で読み、既存の resources(base・mysql)を残す。
// Component は kind: Component で、base の Deployment に patch を当てる(resources では base のオブジェクトに patch できない)。
func TestLocalOverlayUsesAPIComponent(t *testing.T) {
	overlay := deploytest.ReadKustomization(t, deploytest.LocalOverlayDir)
	if overlay.Namespace != "pokecalc" {
		t.Errorf("local overlay の namespace = %q, want pokecalc", overlay.Namespace)
	}
	for _, want := range []string{"../../base", "mysql"} {
		if !slices.Contains(overlay.Resources, want) {
			t.Errorf("local overlay の resources から %q が消えている: %q", want, overlay.Resources)
		}
	}
	if n := countOf(overlay.Components, deploytest.LocalAPIComponentRef); n != 1 {
		t.Errorf("local overlay の components に %q が %d 個(1個であること): %q",
			deploytest.LocalAPIComponentRef, n, overlay.Components)
	}
	comp := deploytest.ReadKustomization(t, deploytest.LocalAPIComponentDir)
	if comp.Kind != "Component" {
		t.Errorf("%s/kustomization.yaml の kind = %q, want Component", deploytest.LocalAPIComponentDir, comp.Kind)
	}
	if len(comp.Resources) != 0 {
		t.Errorf("Component が resources を持つ: %q(base のオブジェクトを二重に作らない)", comp.Resources)
	}
}

// AC-S2: Dockerfile はリポジトリ直下をビルドコンテキストにし、golang の alpine を services/go.mod の Go の版と
// digest で固定し、CGO なしで静的にビルドし、最終段は scratch で非 root の数値 UID で動く。
func TestAPIDockerfiles(t *testing.T) {
	goVersion := regexp.MustCompile(`(?m)^go (\S+)$`).FindStringSubmatch(readRepoFile(t, "services/go.mod"))
	if goVersion == nil {
		t.Fatal("services/go.mod に go の版が無い")
	}
	builder := regexp.MustCompile(`^golang:` + regexp.QuoteMeta(goVersion[1]) + `-alpine@sha256:[0-9a-f]{64}(\s+AS\s+\S+)?$`)
	for _, svc := range apiServices {
		t.Run(svc, func(t *testing.T) {
			path := "services/" + svc + "/Dockerfile"
			src := readRepoFile(t, path)
			var froms []string
			var lines []string
			for _, line := range strings.Split(src, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				lines = append(lines, line)
				if rest, ok := strings.CutPrefix(line, "FROM "); ok {
					froms = append(froms, strings.TrimSpace(rest))
				}
			}
			if len(froms) < 2 {
				t.Fatalf("%s の FROM が %d 個(ビルド段と最終段の2段以上)", path, len(froms))
			}
			if !builder.MatchString(froms[0]) {
				t.Errorf("%s のビルド段 = %q, want golang:%s-alpine@sha256:<digest>", path, froms[0], goVersion[1])
			}
			if final := strings.Fields(froms[len(froms)-1])[0]; final != "scratch" {
				t.Errorf("%s の最終段 = %q, want scratch", path, final)
			}
			for _, want := range []string{"COPY engine ", "COPY services ", "CGO_ENABLED=0", "./" + svc + "/cmd/" + svc} {
				if !strings.Contains(src, want) {
					t.Errorf("%s に %q が無い(コンテキストはリポジトリ直下。services は ../engine を replace で参照する)", path, want)
				}
			}
			// 最終段の USER は数値の非 root(runAsNonRoot は数値 UID でないと検証できない)。
			lastFrom := 0
			for i, l := range lines {
				if strings.HasPrefix(l, "FROM ") {
					lastFrom = i
				}
			}
			user := ""
			hasEntrypoint := false
			for _, l := range lines[lastFrom:] {
				if rest, ok := strings.CutPrefix(l, "USER "); ok {
					user = strings.TrimSpace(rest)
				}
				if strings.HasPrefix(l, "ENTRYPOINT ") {
					hasEntrypoint = true
				}
			}
			if !regexp.MustCompile(`^[1-9][0-9]*(:[1-9][0-9]*)?$`).MatchString(user) {
				t.Errorf("%s の最終段の USER = %q, want 数値の非 root UID(例 65532:65532)", path, user)
			}
			if !hasEntrypoint {
				t.Errorf("%s の最終段に ENTRYPOINT が無い", path)
			}
		})
	}
}

// AC-S5: services/gateway/Makefile に api- 接頭辞のターゲットがあり、ルートの Makefile は include の1行だけで読む。
func TestAPIMakefile(t *testing.T) {
	const path = "services/gateway/Makefile"
	src := readRepoFile(t, path)

	targets := map[string]string{} // ターゲット名 → レシピ
	var current string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "\t") {
			if current != "" {
				targets[current] += line + "\n"
			}
			continue
		}
		current = ""
		m := regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*:([^=]|$)`).FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(m[1], ".") {
			continue
		}
		if !strings.HasPrefix(m[1], "api-") {
			t.Errorf("%s のターゲット %q に api- 接頭辞が無い(レーン間でターゲット名を衝突させない)", path, m[1])
		}
		current = m[1]
		targets[current] += line + "\n"
	}

	wants := map[string][]string{
		"api-docker-build": {"services/calc/Dockerfile", "services/gateway/Dockerfile"},
		"api-k3d-deploy":   {"k3d image import", "kubectl apply -k", "rollout status"},
		"api-smoke":        {"scripts/smoke.sh"},
		"api-kustomize":    {"kubectl kustomize"},
	}
	for target, fragments := range wants {
		recipe, ok := targets[target]
		if !ok {
			t.Errorf("%s にターゲット %s が無い", path, target)
			continue
		}
		if !regexp.MustCompile(`(?m)^\.PHONY:.*\b` + regexp.QuoteMeta(target) + `\b`).MatchString(src) {
			t.Errorf("%s の .PHONY に %s が無い", path, target)
		}
		for _, f := range fragments {
			if !strings.Contains(expandVars(src, recipe), f) {
				t.Errorf("%s の %s に %q が無い", path, target, f)
			}
		}
	}
	// イメージ名は Kustomize の local overlay と同じ(AssertLocalImage)。
	for _, img := range []string{"pokecalc/calc", "pokecalc/gateway"} {
		if !strings.Contains(src, img) {
			t.Errorf("%s にイメージ名 %s が無い", path, img)
		}
	}
	for _, dir := range []string{deploytest.BaseDir, deploytest.LocalOverlayDir} {
		if !strings.Contains(expandVars(src, targets["api-kustomize"]), dir) {
			t.Errorf("%s の api-kustomize が %s を描画していない", path, dir)
		}
	}

	root := readRepoFile(t, "Makefile")
	if n := len(regexp.MustCompile(`(?m)^include services/gateway/Makefile\s*$`).FindAllString(root, -1)); n != 1 {
		t.Errorf("ルートの Makefile の `include services/gateway/Makefile` が %d 行(1行であること)", n)
	}
}

// expandVars はレシピ中の $(VAR) を、同じ Makefile の `VAR ?= 値` / `VAR := 値` / `VAR = 値` で1段だけ展開する
// (イメージ名やディレクトリを変数に置いても検査できるように)。
func expandVars(makefile, recipe string) string {
	vars := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^([A-Za-z0-9_]+)\s*[?:]?=\s*(.*)$`).FindAllStringSubmatch(makefile, -1) {
		vars[m[1]] = strings.TrimSpace(m[2])
	}
	out := recipe
	for i := 0; i < 3; i++ { // 変数が変数を参照する分を数段まで
		out = regexp.MustCompile(`\$\(([A-Za-z0-9_]+)\)`).ReplaceAllStringFunc(out, func(ref string) string {
			name := ref[2 : len(ref)-1]
			if v, ok := vars[name]; ok {
				return v
			}
			return ref
		})
	}
	return out
}

// AC-S7: scripts/dev.sh は占位ではなく、例のマスタで calc-svc と gateway を起動し、Ctrl-C で両方を止める。
// 実際の起動は手動の確認(`make dev` の後に API_URL を渡して smoke.sh を流す。ADR-0203 §6)。
func TestDevScript(t *testing.T) {
	const path = "scripts/dev.sh"
	src := readRepoFile(t, path)
	if strings.Contains(src, "P3 で") {
		t.Errorf("%s がまだ占位のまま", path)
	}
	for _, want := range []string{
		"./calc/cmd/calc", "./gateway/cmd/gateway",
		"CALC_MASTER_PATH", "CALC_TYPECHART_PATH", "master.example.json", "testdata/golden/typechart.json",
		"GATEWAY_CALC_URL", "DEV_CALC_PORT", "DEV_GATEWAY_PORT", "trap",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%s に %q が無い", path, want)
		}
	}
	if out, err := exec.Command("bash", "-n", deploytest.RepoPath(t, path)).CombinedOutput(); err != nil {
		t.Errorf("bash -n %s: %v\n%s", path, err, out)
	}
}

func countOf(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}
