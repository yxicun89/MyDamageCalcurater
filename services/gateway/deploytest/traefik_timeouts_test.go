package deploytest_test

// issue #299(ADR-0801): タイムアウトの連鎖は「内側 < 外側」。Traefik の既定は respondingTimeouts.writeTimeout = 0 と
// forwardingTimeouts.responseHeaderTimeout = 0(どちらも無期限)で、balance・speed・judge の Ingress は gateway を
// 通らず Traefik から直接届く。k3d の Traefik に有限の値を置き(deploy/k8s/overlays/local/traefik/)、
// 各サービスの writeTimeout より長いことをここで固定する。サービス側の値は各 cmd/main.go の定数から読み、
// 数値を二重管理しない。

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/gateway/deploytest"
	"gopkg.in/yaml.v3"
)

const traefikConfigPath = "deploy/k8s/overlays/local/traefik/helmchartconfig.yaml"

// timeoutServices は writeTimeout を持つサービス。direct は Ingress が gateway を通らず Traefik から直接届くサービス(と gateway)の main.go。
var timeoutServices = []struct {
	name, mainFile string
	direct         bool
}{
	{"gateway", "services/gateway/cmd/gateway/main.go", false},
	{"calc", "services/calc/cmd/calc/main.go", false},
	{"pokedex", "services/pokedex/cmd/pokedex/main.go", false},
	{"balance", "services/balance/cmd/api/main.go", true},
	{"speed", "services/speed/cmd/api/main.go", true},
	{"judge", "services/judge/cmd/api/main.go", true},
}

func durationConst(t *testing.T, file, name string) time.Duration {
	t.Helper()
	raw, err := os.ReadFile(deploytest.RepoPath(t, file))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*` + name + `\s*=\s*(\d+)\s*\*\s*time\.Second`).FindSubmatch(raw)
	if m == nil {
		t.Fatalf("%s に %s の定数が見つからない", file, name)
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	return time.Duration(n) * time.Second
}

// traefikArgs は HelmChartConfig の additionalArguments を key=value の map にして返す。
func traefikArgs(t *testing.T) map[string]time.Duration {
	t.Helper()
	raw, err := os.ReadFile(deploytest.RepoPath(t, traefikConfigPath))
	if err != nil {
		t.Fatalf("Traefik の設定が読めない(issue #299): %v", err)
	}
	var cfg struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			ValuesContent string `yaml:"valuesContent"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != "HelmChartConfig" || cfg.Metadata.Name != "traefik" || cfg.Metadata.Namespace != "kube-system" {
		t.Fatalf("%s は kube-system の traefik の HelmChartConfig であること: %+v", traefikConfigPath, cfg)
	}
	var values struct {
		AdditionalArguments []string `yaml:"additionalArguments"`
	}
	if err := yaml.Unmarshal([]byte(cfg.Spec.ValuesContent), &values); err != nil {
		t.Fatal(err)
	}
	out := map[string]time.Duration{}
	for _, arg := range values.AdditionalArguments {
		key, val, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !ok {
			continue
		}
		if d, err := time.ParseDuration(val); err == nil {
			out[key] = d
		}
	}
	return out
}

func TestTraefikTimeoutsAreFiniteAndOuterThanServices(t *testing.T) {
	args := traefikArgs(t)
	write, ok := args["entryPoints.web.transport.respondingTimeouts.writeTimeout"]
	if !ok || write <= 0 {
		t.Fatalf("entryPoints.web の respondingTimeouts.writeTimeout が有限でない(既定 0 = 無期限): %v", args)
	}
	header, ok := args["serversTransport.forwardingTimeouts.responseHeaderTimeout"]
	if !ok || header <= 0 {
		t.Fatalf("serversTransport の forwardingTimeouts.responseHeaderTimeout が有限でない(既定 0 = 無期限): %v", args)
	}

	var maxWrite, maxDirectWrite time.Duration
	for _, s := range timeoutServices {
		w := durationConst(t, s.mainFile, "writeTimeout")
		maxWrite = max(maxWrite, w)
		if s.direct {
			maxDirectWrite = max(maxDirectWrite, w)
		}
	}
	if write <= maxWrite {
		t.Errorf("Traefik の writeTimeout(%v)が最長のサービスの writeTimeout(%v)以下(内側 < 外側)", write, maxWrite)
	}
	// 直接届くサービスは、応答ヘッダを返すのが遅くても writeTimeout(15 秒)まで。それより長く待つ。
	if header <= maxDirectWrite {
		t.Errorf("Traefik の responseHeaderTimeout(%v)が直接届くサービスの writeTimeout(%v)以下", header, maxDirectWrite)
	}
	// gateway 経由は、gateway の上流の締め切り(既定 10 秒)より長く待つ。
	if gw := durationConst(t, "services/gateway/cmd/gateway/main.go", "defaultUpstreamTimeout"); header <= gw {
		t.Errorf("Traefik の responseHeaderTimeout(%v)が gateway の defaultUpstreamTimeout(%v)以下", header, gw)
	}
}
