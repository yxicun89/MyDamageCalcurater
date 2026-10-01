package deploytest_test

// issue #298: 64Mi の limit を持つ Go サービスは GOMEMLIMIT で GC を OOMKill の前に働かせる。
// calc・gateway は deploy/k8s/base/<サービス>/ に、balance・judge・speed は services/<サービス>/deploy/k8s/base/ に
// Deployment を持つ(置き場所が違うだけで、守る不変条件は同じ)。値は limits.memory との関係で検査し、数値は直書きしない:
// limits.memory の 75% 以上 かつ limits.memory 未満(calc の 56MiB / 64Mi = 87.5% が基準)。

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var goMemLimitDeployments = map[string]string{
	"calc":    "deploy/k8s/base/calc/deployment.yaml",
	"gateway": "deploy/k8s/base/gateway/deployment.yaml",
	"balance": "services/balance/deploy/k8s/base/deployment.yaml",
	"judge":   "services/judge/deploy/k8s/base/deployment.yaml",
	"speed":   "services/speed/deploy/k8s/base/deployment.yaml",
	// ADR-0220 §1
	"record": "deploy/k8s/base/record/deployment.yaml",
	"team":   "deploy/k8s/base/team/deployment.yaml",
}

type memLimitDeployment struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name string `yaml:"name"`
					Env  []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
					Resources struct {
						Limits struct {
							Memory string `yaml:"memory"`
						} `yaml:"limits"`
					} `yaml:"resources"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// parseMiB は "64Mi" / "56MiB" のような MiB 単位の値を MiB で返す(この2つの書き方だけを許す)。
func parseMiB(s string) (int, error) {
	for _, suffix := range []string{"MiB", "Mi"} {
		if n, ok := strings.CutSuffix(s, suffix); ok {
			return strconv.Atoi(n)
		}
	}
	return 0, fmt.Errorf("MiB 単位(Mi か MiB)でない: %q", s)
}

func TestGoServicesSetGOMEMLIMITBelowMemoryLimit(t *testing.T) {
	for service, rel := range goMemLimitDeployments {
		t.Run(service, func(t *testing.T) {
			var d memLimitDeployment
			if err := yaml.Unmarshal([]byte(readRepoFile(t, rel)), &d); err != nil {
				t.Fatalf("%s を読めない: %v", rel, err)
			}
			if len(d.Spec.Template.Spec.Containers) != 1 {
				t.Fatalf("%s: コンテナは1つ(実際 %d)", rel, len(d.Spec.Template.Spec.Containers))
			}
			c := d.Spec.Template.Spec.Containers[0]
			limit, err := parseMiB(c.Resources.Limits.Memory)
			if err != nil {
				t.Fatalf("%s の limits.memory: %v", rel, err)
			}
			var values []string
			for _, e := range c.Env {
				if e.Name == "GOMEMLIMIT" {
					values = append(values, e.Value)
				}
			}
			if len(values) != 1 {
				t.Fatalf("%s: env GOMEMLIMIT が %d 個(1個であること。GC が OOMKill の前に働くように)", rel, len(values))
			}
			soft, err := parseMiB(values[0])
			if err != nil {
				t.Fatalf("%s の GOMEMLIMIT: %v", rel, err)
			}
			if soft >= limit || soft*4 < limit*3 {
				t.Errorf("%s: GOMEMLIMIT=%dMiB は limits.memory=%dMiB の 75%% 以上・未満であること", rel, soft, limit)
			}
		})
	}
}
