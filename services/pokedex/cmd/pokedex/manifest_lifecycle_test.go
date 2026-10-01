package main

// pokedex の Deployment の probe と停止の手順の静的検査(issue #107・#324 の pokedex 分・ADR-0129 §3・§4)。
// kubectl を使わず base の YAML を読む(manifest_test.go と同じ流儀)。
//
//   - readinessProbe は GET /readyz(DB に連動)、livenessProbe は GET /healthz(DB に連動させない)
//   - readinessProbe.timeoutSeconds は /readyz の DB 確認の締め切り(httpapi.DefaultReadinessTimeout)より長い
//     (kubelet が待ち切る前に、ハンドラが 503 を返して NotReady を伝える)
//   - lifecycle.preStop は sleep アクション(イメージに sleep コマンドが無いので exec にしない。k8s 1.34 で GA)
//   - terminationGracePeriodSeconds > preStop の sleep + shutdownTimeout

import (
	"testing"
	"time"

	"example.com/pokecalc/services/gateway/deploytest"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
)

// readyzPath は readiness の probe のパス(httpapi の GET /readyz)。
const readyzPath = "/readyz"

type lifecycleProbe struct {
	HTTPGet *struct {
		Path string `yaml:"path"`
		Port any    `yaml:"port"`
	} `yaml:"httpGet"`
	TimeoutSeconds   *int `yaml:"timeoutSeconds"`
	PeriodSeconds    *int `yaml:"periodSeconds"`
	FailureThreshold *int `yaml:"failureThreshold"`
}

// pokedexLifecycleDeployment は Deployment のうち、probe と停止の手順の部分。
type pokedexLifecycleDeployment struct {
	Spec struct {
		Template struct {
			Spec struct {
				TerminationGracePeriodSeconds *int64 `yaml:"terminationGracePeriodSeconds"`
				Containers                    []struct {
					Name           string          `yaml:"name"`
					ReadinessProbe *lifecycleProbe `yaml:"readinessProbe"`
					LivenessProbe  *lifecycleProbe `yaml:"livenessProbe"`
					Lifecycle      *struct {
						PreStop *struct {
							Sleep *struct {
								Seconds *int64 `yaml:"seconds"`
							} `yaml:"sleep"`
							Exec    any `yaml:"exec"`
							HTTPGet any `yaml:"httpGet"`
						} `yaml:"preStop"`
					} `yaml:"lifecycle"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func pokedexContainerLifecycle(t *testing.T) (pokedexLifecycleDeployment, int) {
	t.Helper()
	objs := deploytest.BaseObjects(t, pokedexService)
	var d pokedexLifecycleDeployment
	deploytest.Find(t, objs, "Deployment", pokedexService).Decode(t, &d)
	for i, c := range d.Spec.Template.Spec.Containers {
		if c.Name == pokedexService {
			return d, i
		}
	}
	t.Fatalf("コンテナ %s が無い", pokedexService)
	return d, -1
}

// AC-K7(#107): readinessProbe は /readyz、livenessProbe は /healthz(DB 障害で Pod を再起動ループにしない)。
// readiness の timeoutSeconds は httpapi.DefaultReadinessTimeout より長く、periodSeconds を明示する。
func TestPokedexProbesSplitReadinessAndLiveness(t *testing.T) {
	d, i := pokedexContainerLifecycle(t)
	c := d.Spec.Template.Spec.Containers[i]

	r := c.ReadinessProbe
	if r == nil || r.HTTPGet == nil || r.HTTPGet.Path != readyzPath {
		t.Fatalf("readinessProbe が httpGet %s でない", readyzPath)
	}
	if r.HTTPGet.Port != "http" {
		t.Errorf("readinessProbe の port = %v, want http(名前で指す)", r.HTTPGet.Port)
	}
	if r.PeriodSeconds == nil || *r.PeriodSeconds <= 0 {
		t.Error("readinessProbe.periodSeconds を明示すること")
	}
	if r.TimeoutSeconds == nil {
		t.Fatal("readinessProbe.timeoutSeconds が無い(既定の1秒は /readyz の DB 確認の締め切りより短い)")
	}
	if got := time.Duration(*r.TimeoutSeconds) * time.Second; got <= httpapi.DefaultReadinessTimeout {
		t.Errorf("readinessProbe.timeoutSeconds = %v, want httpapi.DefaultReadinessTimeout(%v)より長い", got, httpapi.DefaultReadinessTimeout)
	}

	l := c.LivenessProbe
	if l == nil || l.HTTPGet == nil || l.HTTPGet.Path != deploytest.HealthzPath {
		t.Fatalf("livenessProbe が httpGet %s でない(liveness は DB に連動させない)", deploytest.HealthzPath)
	}
}

// AC-K8(#324): 停止時は preStop の sleep で数秒受け付けを続け(Endpoints からの削除が伝わるまで)、
// その後 SIGTERM → Shutdown(shutdownTimeout)。terminationGracePeriodSeconds はその合計より長い。
// preStop は sleep アクション(exec の sleep はイメージに無い。httpGet でもない)。
func TestPokedexPreStopSleepAndGracePeriod(t *testing.T) {
	d, i := pokedexContainerLifecycle(t)
	c := d.Spec.Template.Spec.Containers[i]

	if c.Lifecycle == nil || c.Lifecycle.PreStop == nil {
		t.Fatal("lifecycle.preStop が無い(SIGTERM の直後にリスナーを閉じ、Endpoints の更新前の接続を拒否してしまう)")
	}
	ps := c.Lifecycle.PreStop
	if ps.Exec != nil || ps.HTTPGet != nil {
		t.Error("preStop は sleep アクションにする(イメージ distroless に sleep コマンドが無い)")
	}
	if ps.Sleep == nil || ps.Sleep.Seconds == nil || *ps.Sleep.Seconds <= 0 {
		t.Fatal("preStop.sleep.seconds が正の値でない")
	}
	preStop := time.Duration(*ps.Sleep.Seconds) * time.Second

	grace := d.Spec.Template.Spec.TerminationGracePeriodSeconds
	if grace == nil {
		t.Fatal("terminationGracePeriodSeconds が無い")
	}
	if got := time.Duration(*grace) * time.Second; got <= preStop+shutdownTimeout {
		t.Errorf("terminationGracePeriodSeconds = %v, want preStop(%v)+ shutdownTimeout(%v)より長い", got, preStop, shutdownTimeout)
	}
}
