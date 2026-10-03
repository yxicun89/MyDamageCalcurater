package deploytest_test

// record-svc・team-svc の k3d 配線(ADR-0220 §1・§2。AC-K1・K4・K6・K7・K8)の静的検査。
// Deployment・Service・CronJob の形そのものは services/{record,team}/cmd/*/manifest_test.go(AssertTiDBService)が見る。
// ここはクラスタ全体の組み込み(base の kustomization・cloud overlay・up.sh・calc との独立・ServiceMonitor)を見る。

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

var tidbServices = []struct {
	name, image, cronJob string
}{
	{"record", "pokecalc/record", "record-expire"},
	{"team", "pokecalc/team", "team-expire"},
}

// AC-K6: base の kustomization.yaml の resources に record・team が1行ずつある。
func TestBaseKustomizationListsRecordAndTeam(t *testing.T) {
	k := deploytest.ReadKustomization(t, deploytest.BaseDir)
	for _, s := range tidbServices {
		if n := countOf(k.Resources, s.name); n != 1 {
			t.Errorf("%s/kustomization.yaml の resources に %q が %d 個(1個であること): %q", deploytest.BaseDir, s.name, n, k.Resources)
		}
	}
}

// AC-K6: up.sh が record・team の server イメージを build して k3d に import する。タグは base の Deployment と同じ。
func TestUpScriptBuildsRecordAndTeamServerImages(t *testing.T) {
	up := readRepoFile(t, "scripts/up.sh")
	for _, s := range tidbServices {
		t.Run(s.name, func(t *testing.T) {
			d := deploytest.BaseDeployment(t, s.name)
			image := d.Spec.Template.Spec.Containers[0].Image
			re := regexp.MustCompile(`docker build -f services/` + s.name + `/Dockerfile --target server -t "\$` +
				`([A-Z_]+)"`)
			m := re.FindStringSubmatch(up)
			if m == nil {
				t.Fatalf("scripts/up.sh に services/%s/Dockerfile の server ターゲットの build が無い", s.name)
			}
			def := regexp.MustCompile(`(?m)^` + m[1] + `="\$\{` + m[1] + `:-([^}]+)\}"`).FindStringSubmatch(up)
			if def == nil || def[1] != image {
				t.Errorf("scripts/up.sh の %s の既定 = %v, want Deployment の image %q", m[1], def, image)
			}
			if !strings.Contains(up, `k3d image import "$`+m[1]+`"`) {
				t.Errorf("scripts/up.sh が $%s を k3d image import していない", m[1])
			}
		})
	}
}

// AC-K4: cloud overlay は TiDB・Secret が無いので失効ジョブを suspend する(pokedex-import と同じ。ADR-0104 §8)。
func TestCloudOverlaySuspendsExpireCronJobs(t *testing.T) {
	const dir = "deploy/k8s/overlays/cloud"
	k := deploytest.ReadKustomization(t, dir)
	suspended := map[string]bool{}
	for _, p := range k.Patches {
		if p.Path == "" {
			continue
		}
		for _, o := range deploytest.ReadObjects(t, dir+"/"+p.Path) {
			if o.Kind != "CronJob" {
				continue
			}
			var cj struct {
				Spec struct {
					Suspend *bool `yaml:"suspend"`
				} `yaml:"spec"`
			}
			o.Decode(t, &cj)
			if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
				suspended[o.Name] = true
			}
		}
	}
	for _, s := range tidbServices {
		if !suspended[s.cronJob] {
			t.Errorf("cloud overlay が CronJob %s を suspend していない", s.cronJob)
		}
	}
}

// AC-K7(絶対ルール5): calc は record・team・TiDB を知らない。保存が落ちても計算は成功する。
func TestCalcDoesNotDependOnRecordOrTeam(t *testing.T) {
	d := deploytest.BaseDeployment(t, "calc")
	env := d.Container(t, "calc").EnvMap(t) // valueFrom(Secret)を使っていれば EnvMap が失敗にする
	for name, value := range env {
		for _, bad := range []string{"RECORD", "TEAM", "TIDB"} {
			if strings.Contains(strings.ToUpper(name), bad) {
				t.Errorf("calc に環境変数 %s がある(calc は record/team/TiDB に依存しない)", name)
			}
		}
		for _, bad := range []string{"//record", "//team", "tidb", ":4000"} {
			if strings.Contains(value, bad) {
				t.Errorf("calc の %s=%q が record/team/TiDB を指している", name, value)
			}
		}
	}
	// gateway の readiness は上流(record・team)に連動しない(record が落ちても /api/calc は通る)。
	g := deploytest.BaseDeployment(t, "gateway")
	if p := g.Container(t, "gateway").ReadinessProbe; p == nil || p.HTTPGet == nil || p.HTTPGet.Path != deploytest.HealthzPath {
		t.Errorf("gateway の readinessProbe = %+v, want %s(上流の状態に連動しない)", p, deploytest.HealthzPath)
	}
}

// AC-K8: record・team の ServiceMonitor が observability にあり、kustomization に並ぶ(ADR-0406 §4 と同じ形)。
func TestRecordAndTeamServiceMonitors(t *testing.T) {
	const dir = "deploy/k8s/base/observability"
	k := deploytest.ReadKustomization(t, dir)
	for _, s := range tidbServices {
		file := "servicemonitors/" + s.name + ".yaml"
		if !slices.Contains(k.Resources, file) {
			t.Errorf("%s/kustomization.yaml の resources に %s が無い", dir, file)
			continue
		}
		var sm struct {
			Metadata struct {
				Namespace string `yaml:"namespace"`
			} `yaml:"metadata"`
			Spec struct {
				NamespaceSelector struct {
					MatchNames []string `yaml:"matchNames"`
				} `yaml:"namespaceSelector"`
				Selector struct {
					MatchLabels map[string]string `yaml:"matchLabels"`
				} `yaml:"selector"`
				Endpoints []struct {
					Port string `yaml:"port"`
					Path string `yaml:"path"`
				} `yaml:"endpoints"`
			} `yaml:"spec"`
		}
		deploytest.Find(t, deploytest.ReadObjects(t, dir+"/"+file), "ServiceMonitor", s.name).Decode(t, &sm)
		if sm.Metadata.Namespace != "observability" || !slices.Equal(sm.Spec.NamespaceSelector.MatchNames, []string{"pokecalc"}) {
			t.Errorf("%s: namespace / namespaceSelector = %q / %q, want observability / [pokecalc]", file, sm.Metadata.Namespace, sm.Spec.NamespaceSelector.MatchNames)
		}
		if sm.Spec.Selector.MatchLabels["app.kubernetes.io/name"] != s.name {
			t.Errorf("%s: selector = %v, want app.kubernetes.io/name=%s", file, sm.Spec.Selector.MatchLabels, s.name)
		}
		if len(sm.Spec.Endpoints) != 1 || sm.Spec.Endpoints[0].Port != "http" || sm.Spec.Endpoints[0].Path != "/metrics" {
			t.Errorf("%s: endpoints = %+v, want port http・path /metrics の1つ", file, sm.Spec.Endpoints)
		}
	}
}

// ADR-0220 §2(承認待ち): local overlay も失効ジョブを suspend する。実データへ初めて向ける承認(ADR-0209)が済むまで、
// 自動で実データを消さない。承認後にこの patch を外す。
func TestLocalOverlaySuspendsExpireCronJobs(t *testing.T) {
	k := deploytest.ReadKustomization(t, deploytest.LocalOverlayDir)
	suspended := map[string]bool{}
	for _, p := range k.Patches {
		if p.Path == "" {
			continue
		}
		for _, o := range deploytest.ReadObjects(t, deploytest.LocalOverlayDir+"/"+p.Path) {
			if o.Kind != "CronJob" {
				continue
			}
			var cj struct {
				Spec struct {
					Suspend *bool `yaml:"suspend"`
				} `yaml:"spec"`
			}
			o.Decode(t, &cj)
			if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
				suspended[o.Name] = true
			}
		}
	}
	for _, s := range tidbServices {
		if !suspended[s.cronJob] {
			t.Errorf("local overlay が CronJob %s を suspend していない(承認までは自動で実データを消さない)", s.cronJob)
		}
	}
}
