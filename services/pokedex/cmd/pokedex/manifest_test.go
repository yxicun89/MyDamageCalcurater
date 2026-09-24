package main

// pokedex-svc の k8s マニフェスト・イメージ・起動スクリプトの静的検査(ADR-0105 §6)。kubectl・docker を使わず YAML とファイルを読む。
// 要点: Deployment と Service(ClusterIP・80 番)はクラスタ内だけ。どの Ingress も pokedex を指さない
// (公開の /api/pokedex/* は gateway 経由。内部 API /internal/pokedex/master は gateway でも 404。ADR-0204)。
// DSN は Secret mysql-auth の pokedex-reader-dsn(SELECT 専用の pokedex_reader。ADR-0110)から渡し、平文でマニフェストに書かない。

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/gateway/deploytest"
)

const (
	pokedexService   = "pokedex"
	pokedexImageRepo = "pokecalc/pokedex"
	mysqlSecret      = "mysql-auth"
	mysqlSecretDSN   = "pokedex-reader-dsn" // ADR-0110 §6: 公開 API は SELECT 専用ユーザー
)

// pokedexDeployment は Deployment のうち、この検査に要る部分(deploytest.Deployment は valueFrom を読まないため)。
type pokedexDeployment struct {
	Spec struct {
		Selector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Template struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec struct {
				AutomountServiceAccountToken  *bool  `yaml:"automountServiceAccountToken"`
				TerminationGracePeriodSeconds *int64 `yaml:"terminationGracePeriodSeconds"`
				SecurityContext               struct {
					RunAsNonRoot   *bool `yaml:"runAsNonRoot"`
					SeccompProfile struct {
						Type string `yaml:"type"`
					} `yaml:"seccompProfile"`
				} `yaml:"securityContext"`
				Containers []struct {
					Name            string   `yaml:"name"`
					Image           string   `yaml:"image"`
					ImagePullPolicy string   `yaml:"imagePullPolicy"`
					Args            []string `yaml:"args"`
					Ports           []struct {
						Name          string `yaml:"name"`
						ContainerPort int    `yaml:"containerPort"`
					} `yaml:"ports"`
					Env []struct {
						Name      string  `yaml:"name"`
						Value     *string `yaml:"value"`
						ValueFrom *struct {
							SecretKeyRef *struct {
								Name string `yaml:"name"`
								Key  string `yaml:"key"`
							} `yaml:"secretKeyRef"`
						} `yaml:"valueFrom"`
					} `yaml:"env"`
					ReadinessProbe *deploytest.Probe `yaml:"readinessProbe"`
					LivenessProbe  *deploytest.Probe `yaml:"livenessProbe"`
					Resources      struct {
						Requests map[string]string `yaml:"requests"`
						Limits   map[string]string `yaml:"limits"`
					} `yaml:"resources"`
					SecurityContext deploytest.ContainerSecurityContext `yaml:"securityContext"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type serviceSpec struct {
	Spec struct {
		Type     string            `yaml:"type"`
		Selector map[string]string `yaml:"selector"`
		Ports    []struct {
			Name       string `yaml:"name"`
			Port       int    `yaml:"port"`
			TargetPort any    `yaml:"targetPort"`
			NodePort   int    `yaml:"nodePort"`
		} `yaml:"ports"`
	} `yaml:"spec"`
}

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(deploytest.RepoPath(t, rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return string(raw)
}

// AC-K1: base/pokedex に Deployment pokedex がある(serve・DSN は Secret・probe は /healthz・非 root・読み取り専用)。
func TestManifestPokedexDeployment(t *testing.T) {
	objs := deploytest.BaseObjects(t, pokedexService)
	var d pokedexDeployment
	deploytest.Find(t, objs, "Deployment", pokedexService).Decode(t, &d)

	for k, v := range d.Spec.Selector.MatchLabels {
		if d.Spec.Template.Metadata.Labels[k] != v {
			t.Errorf("selector のラベル %s=%s が Pod テンプレートに無い", k, v)
		}
	}
	pod := d.Spec.Template.Spec
	if len(pod.Containers) != 1 {
		t.Fatalf("コンテナが %d 個(1個であること)", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.Name != pokedexService {
		t.Errorf("コンテナ名 = %q, want %s", c.Name, pokedexService)
	}
	if name, _, _ := deploytest.SplitImage(c.Image); name != pokedexImageRepo {
		t.Errorf("image = %q, want %s:<tag>", c.Image, pokedexImageRepo)
	}
	if c.ImagePullPolicy != "IfNotPresent" && c.ImagePullPolicy != "Never" {
		t.Errorf("imagePullPolicy = %q(k3d image import したイメージを使う)", c.ImagePullPolicy)
	}
	if strings.Join(c.Args, " ") != "serve" {
		t.Errorf("args = %v, want [serve](export は Job / make から別に流す)", c.Args)
	}
	if len(c.Ports) != 1 || c.Ports[0].Name != "http" {
		t.Fatalf("ports = %+v, want 名前 http のポート1つ", c.Ports)
	}
	addr := defaultAddr
	var dsnFromSecret bool
	for _, e := range c.Env {
		switch e.Name {
		case envAddr:
			if e.Value != nil && *e.Value != "" {
				addr = *e.Value
			}
		case envDatabaseDSN:
			if e.Value != nil {
				t.Errorf("%s が平文の value で書かれている(Secret %s から渡すこと)", envDatabaseDSN, mysqlSecret)
			}
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil &&
				e.ValueFrom.SecretKeyRef.Name == mysqlSecret && e.ValueFrom.SecretKeyRef.Key == mysqlSecretDSN {
				dsnFromSecret = true
			}
		}
	}
	if !dsnFromSecret {
		t.Errorf("%s が secretKeyRef %s/%s でない", envDatabaseDSN, mysqlSecret, mysqlSecretDSN)
	}
	if _, port, _ := strings.Cut(addr, ":"); port != itoa(c.Ports[0].ContainerPort) {
		t.Errorf("待ち受けアドレス %q のポートが containerPort %d と違う", addr, c.Ports[0].ContainerPort)
	}
	for name, p := range map[string]*deploytest.Probe{"readinessProbe": c.ReadinessProbe, "livenessProbe": c.LivenessProbe} {
		if p == nil || p.HTTPGet == nil || p.HTTPGet.Path != deploytest.HealthzPath {
			t.Errorf("%s が httpGet %s でない", name, deploytest.HealthzPath)
		}
	}
	for _, kind := range []string{"cpu", "memory"} {
		if c.Resources.Requests[kind] == "" || c.Resources.Limits[kind] == "" {
			t.Errorf("resources の %s の requests / limits が無い", kind)
		}
	}
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Error("automountServiceAccountToken が false でない")
	}
	if pod.SecurityContext.RunAsNonRoot == nil || !*pod.SecurityContext.RunAsNonRoot {
		t.Error("Pod の runAsNonRoot: true が無い")
	}
	if pod.SecurityContext.SeccompProfile.Type != "RuntimeDefault" {
		t.Errorf("seccompProfile.type = %q", pod.SecurityContext.SeccompProfile.Type)
	}
	sc := c.SecurityContext
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Error("readOnlyRootFilesystem: true が無い")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Error("allowPrivilegeEscalation: false が無い")
	}
	if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("capabilities.drop = %v, want [ALL]", sc.Capabilities.Drop)
	}
}

// AC-K6(issue #109 / ADR-0111 決定3): terminationGracePeriodSeconds を明示し、main.go の
// shutdownTimeout 定数より長い。値を2箇所にハードコードする代わりに不等式で比較することで、
// どちらか一方だけを変更したときに検知できるようにする(main.go の shutdownTimeout 定数と
// 手で同期する必要はない。このテストが直接参照する)。
func TestPokedexTerminationGracePeriodExceedsShutdownTimeout(t *testing.T) {
	objs := deploytest.BaseObjects(t, pokedexService)
	var d pokedexDeployment
	deploytest.Find(t, objs, "Deployment", pokedexService).Decode(t, &d)

	grace := d.Spec.Template.Spec.TerminationGracePeriodSeconds
	if grace == nil {
		t.Fatal("terminationGracePeriodSeconds が無い(既定の30秒に暗黙で頼らず明示すること。ADR-0111 決定3)")
	}
	if *grace <= 0 {
		t.Fatalf("terminationGracePeriodSeconds = %d, want 正の値", *grace)
	}
	got := time.Duration(*grace) * time.Second
	if got <= shutdownTimeout {
		t.Errorf("terminationGracePeriodSeconds = %v, want shutdownTimeout(%v)より長い(main.go の shutdownTimeout 定数と比較)",
			got, shutdownTimeout)
	}
}

// AC-K2: Service pokedex はクラスタ内だけ(ClusterIP・80 番・nodePort なし)。calc-svc は http://pokedex で内部 API を呼ぶ(ADR-0204 §5)。
func TestManifestPokedexServiceIsInternal(t *testing.T) {
	objs := deploytest.BaseObjects(t, pokedexService)
	var d pokedexDeployment
	deploytest.Find(t, objs, "Deployment", pokedexService).Decode(t, &d)
	var svc serviceSpec
	deploytest.Find(t, objs, "Service", pokedexService).Decode(t, &svc)
	if svc.Spec.Type != "" && svc.Spec.Type != "ClusterIP" {
		t.Errorf("Service の type = %q, want ClusterIP(クラスタの外に出さない)", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Name != "http" || svc.Spec.Ports[0].Port != deploytest.ServicePort || svc.Spec.Ports[0].NodePort != 0 {
		t.Errorf("Service のポート = %+v, want http:%d(nodePort なし)", svc.Spec.Ports, deploytest.ServicePort)
	}
	if len(svc.Spec.Selector) == 0 {
		t.Error("Service の selector が空")
	}
	for k, v := range svc.Spec.Selector {
		if d.Spec.Template.Metadata.Labels[k] != v {
			t.Errorf("Service の selector %s=%s が Pod のラベルに無い", k, v)
		}
	}
}

// k8sManifests はリポジトリの k8s の YAML(deploy/k8s と services/*/deploy)を全部返す(リポジトリ直下からの相対)。
func k8sManifests(t *testing.T) []string {
	t.Helper()
	root := deploytest.RepoRoot(t)
	var files []string
	roots := []string{"deploy/k8s"}
	if m, _ := filepath.Glob(filepath.Join(root, "services", "*", "deploy")); m != nil {
		for _, d := range m {
			rel, _ := filepath.Rel(root, d)
			roots = append(roots, rel)
		}
	}
	for _, r := range roots {
		err := filepath.WalkDir(filepath.Join(root, r), func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
				rel, _ := filepath.Rel(root, path)
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s を走査できない: %v", r, err)
		}
	}
	return files
}

// AC-K3: どの Ingress も pokedex の Service を指さない。pokedex の名前の Service を NodePort / LoadBalancer にしない。
// マニフェストのどこにも DSN の平文(user:pass@tcp(...))が無い。
func TestPokedexIsNotExposed(t *testing.T) {
	files := k8sManifests(t)
	if len(files) == 0 {
		t.Fatal("k8s の YAML が1つも見つからない(検査が空振りしている)")
	}
	dsnLiteral := regexp.MustCompile(`[^\s:]+:[^\s@]*@tcp\(`)
	var ingresses int
	for _, f := range files {
		if dsnLiteral.MatchString(readRepo(t, f)) {
			t.Errorf("%s に DSN の平文がある(Secret %s から渡すこと)", f, mysqlSecret)
		}
		if strings.Contains(f, "kustomization") {
			continue
		}
		for _, o := range deploytest.ReadObjects(t, f) {
			switch o.Kind {
			case "Ingress":
				ingresses++
				var ing deploytest.Ingress
				o.Decode(t, &ing)
				for _, r := range ing.Spec.Rules {
					for _, p := range r.HTTP.Paths {
						if p.Backend.Service.Name == pokedexService {
							t.Errorf("%s の Ingress %s が pokedex を指している(path %s)。pokedex はクラスタ内だけ", f, o.Name, p.Path)
						}
						if strings.HasPrefix(p.Path, "/internal") {
							t.Errorf("%s の Ingress %s が /internal を公開している", f, o.Name)
						}
					}
				}
			case "Service":
				if o.Name != pokedexService {
					continue
				}
				var svc serviceSpec
				o.Decode(t, &svc)
				if svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer" {
					t.Errorf("%s の Service pokedex が %s(クラスタ内だけにする)", f, svc.Spec.Type)
				}
			}
		}
	}
	if ingresses == 0 {
		t.Error("Ingress が1つも見つからない(gateway の Ingress があるはず。検査が空振りしている)")
	}
}

// AC-K4: base の kustomization に Deployment・Service を足す(既存の migrate Job・CronJob・PVC は残す)。
func TestBaseKustomizationListsServiceResources(t *testing.T) {
	k := deploytest.ReadKustomization(t, deploytest.BaseDir+"/"+pokedexService)
	got := strings.Join(k.Resources, ",")
	for _, want := range []string{"deployment.yaml", "service.yaml", "job-migrate.yaml", "cronjob-import.yaml", "pvc-import-cache.yaml"} {
		if !strings.Contains(got, want) {
			t.Errorf("base/pokedex の resources に %s が無い: %v", want, k.Resources)
		}
	}
}

// AC-K5: イメージは services/pokedex/Dockerfile の server ターゲット(./pokedex/cmd/pokedex。非 root。既定の引数は serve)。
// up.sh がそれを build して k3d に import する。
func TestServerImageAndUpScript(t *testing.T) {
	df := readRepo(t, "services/pokedex/Dockerfile")
	for _, re := range []string{
		`(?m)^FROM\s+\S+\s+AS\s+server\s*$`,
		`go build[^\n]*-o\s+/out/pokedex\s+\./pokedex/cmd/pokedex`,
		`(?s)AS server.*USER 65532:65532`,
		`(?s)AS server.*ENTRYPOINT \["/pokedex"\]`,
		`(?s)AS server.*CMD \["serve"\]`,
	} {
		if !regexp.MustCompile(re).MatchString(df) {
			t.Errorf("Dockerfile に /%s/ が無い", re)
		}
	}
	up := readRepo(t, "scripts/up.sh")
	if !regexp.MustCompile(`--target server`).MatchString(up) {
		t.Error("scripts/up.sh が server ターゲットを build していない")
	}
	if !strings.Contains(up, "pokecalc/pokedex:") {
		t.Error("scripts/up.sh に pokecalc/pokedex のイメージが無い(k3d image import)")
	}
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}
