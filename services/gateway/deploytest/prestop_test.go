package deploytest_test

// issue #324: 停止時に Endpoints から外れる前の新規接続を拒否しないよう、全 Go サービス・web に preStop の sleep を置き、
// terminationGracePeriodSeconds を preStop + shutdownTimeout より長くする(pokedex と同じ方式。ADR-0129 §4)。
// shutdownTimeout は各 cmd/main.go の定数から読み、数値を二重管理しない。

import (
	"regexp"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"
)

type preStopDeployment struct {
	Spec struct {
		Template struct {
			Spec struct {
				TerminationGracePeriodSeconds int `yaml:"terminationGracePeriodSeconds"`
				Containers                    []struct {
					Name      string `yaml:"name"`
					Lifecycle struct {
						PreStop struct {
							Sleep struct {
								Seconds int `yaml:"seconds"`
							} `yaml:"sleep"`
						} `yaml:"preStop"`
					} `yaml:"lifecycle"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// mainFile が空のサービスは shutdownTimeout を持たない(nginx)ので 0 として扱う。
var preStopServices = []struct{ name, manifest, mainFile string }{
	{"calc", "deploy/k8s/base/calc/deployment.yaml", "services/calc/cmd/calc/shutdown.go"},
	{"gateway", "deploy/k8s/base/gateway/deployment.yaml", "services/gateway/cmd/gateway/main.go"},
	{"judge", "services/judge/deploy/k8s/base/deployment.yaml", "services/judge/cmd/api/shutdown.go"},
	{"speed", "services/speed/deploy/k8s/base/deployment.yaml", "services/speed/cmd/api/shutdown.go"},
	{"balance", "services/balance/deploy/k8s/base/deployment.yaml", "services/balance/cmd/api/shutdown.go"},
	{"web", "deploy/k8s/base/web/deployment.yaml", ""},
	// ADR-0220 §1
	{"record", "deploy/k8s/base/record/deployment.yaml", "services/record/cmd/record/main.go"},
	{"team", "deploy/k8s/base/team/deployment.yaml", "services/team/cmd/team/main.go"},
}

var shutdownTimeoutRe = regexp.MustCompile(`shutdownTimeout\s*=\s*(\d+)\s*\*\s*time\.Second`)

func TestDeploymentsHavePreStopAndGracePeriod(t *testing.T) {
	for _, s := range preStopServices {
		t.Run(s.name, func(t *testing.T) {
			var d preStopDeployment
			if err := yaml.Unmarshal([]byte(readRepoFile(t, s.manifest)), &d); err != nil {
				t.Fatalf("%s を読めない: %v", s.manifest, err)
			}
			spec := d.Spec.Template.Spec
			if len(spec.Containers) != 1 {
				t.Fatalf("%s: コンテナは1つ(実際 %d)", s.manifest, len(spec.Containers))
			}
			preStop := spec.Containers[0].Lifecycle.PreStop.Sleep.Seconds
			if preStop <= 0 {
				t.Errorf("%s: lifecycle.preStop.sleep.seconds が無い(Endpoints から外れる前の新規接続を拒否してしまう)", s.manifest)
			}
			shutdown := 0
			if s.mainFile != "" {
				m := shutdownTimeoutRe.FindStringSubmatch(readRepoFile(t, s.mainFile))
				if m == nil {
					t.Fatalf("%s に shutdownTimeout の定数が見つからない", s.mainFile)
				}
				shutdown, _ = strconv.Atoi(m[1])
			}
			if grace := spec.TerminationGracePeriodSeconds; grace <= preStop+shutdown {
				t.Errorf("%s: terminationGracePeriodSeconds=%d は preStop(%d)+shutdownTimeout(%d) より長いこと", s.manifest, grace, preStop, shutdown)
			}
		})
	}
}

// issue #523: 処理中のリクエストを停止時に切らないよう、shutdownTimeout は writeTimeout 以上(ADR-0804)。
var writeTimeoutRe = regexp.MustCompile(`writeTimeout\s*=\s*(\d+)\s*\*\s*time\.Second`)

func TestShutdownTimeoutCoversWriteTimeout(t *testing.T) {
	for _, s := range []struct{ name, shutdownFile, mainFile string }{
		{"calc", "services/calc/cmd/calc/shutdown.go", "services/calc/cmd/calc/main.go"},
		{"judge", "services/judge/cmd/api/shutdown.go", "services/judge/cmd/api/main.go"},
		{"speed", "services/speed/cmd/api/shutdown.go", "services/speed/cmd/api/main.go"},
		{"balance", "services/balance/cmd/api/shutdown.go", "services/balance/cmd/api/main.go"},
	} {
		t.Run(s.name, func(t *testing.T) {
			sm := shutdownTimeoutRe.FindStringSubmatch(readRepoFile(t, s.shutdownFile))
			wm := writeTimeoutRe.FindStringSubmatch(readRepoFile(t, s.mainFile))
			if sm == nil || wm == nil {
				t.Fatalf("shutdownTimeout(%v)または writeTimeout(%v)の定数が見つからない", sm, wm)
			}
			shutdown, _ := strconv.Atoi(sm[1])
			write, _ := strconv.Atoi(wm[1])
			if shutdown < write {
				t.Errorf("shutdownTimeout(%ds)が writeTimeout(%ds)より短い", shutdown, write)
			}
		})
	}
}
