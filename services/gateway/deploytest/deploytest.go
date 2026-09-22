// Package deploytest は API レーン(calc-svc・gateway)の k8s マニフェストを静的に検査するテストの共通部品
// (ADR-0203 §3)。kubectl が無くても動くように、Kustomize の描画はせず YAML を gopkg.in/yaml.v3 で読む。
// 本番コードから使わない(テストからだけ使う。calctest と同じ扱い)。
//
// 読むのは次の2か所だけ(描画の全機能は再現しない):
//   - deploy/k8s/base/<サービス>/: kustomization.yaml の resources に並ぶファイル(Deployment・Service・Ingress)
//   - deploy/k8s/overlays/local/api/: local overlay 専用の Kustomize Component。configMapGenerator・images と、
//     patches に並ぶ strategic merge patch(Deployment)。
//
// patch の合成は「コンテナを name で突き合わせ、env は name で上書き・volumeMounts と volumes は追加」だけを行う。
package deploytest

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// リポジトリ直下からの相対パス(ADR-0203 §3、および「apply の分離」の追記)。
const (
	BaseDir              = "deploy/k8s/base"
	LocalOverlayDir      = "deploy/k8s/overlays/local"
	LocalAPIComponentDir = "deploy/k8s/overlays/local/api"
	// LocalAPIComponentRef は local overlay の kustomization.yaml の components に書く値。
	LocalAPIComponentRef = "api"
	// LocalAPIOnlyOverlayDir は API レーンだけ(calc・gateway)を k3d に載せる専用の overlay。
	// `api-k3d-deploy` は常にここだけを適用し、共有の LocalOverlayDir を丸ごとは apply しない
	// (pokedex-migrate Job の再実行・mysql の上書きを避ける。ADR-0203 追記「apply の分離」)。
	LocalAPIOnlyOverlayDir = "deploy/k8s/overlays/local-api"
)

// RepoRoot はリポジトリ直下の絶対パスを返す(このファイルの位置から解決するので、テストの作業ディレクトリに依存しない)。
func RepoRoot(t testing.TB) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("deploytest: 自身のソースの位置を得られない")
	}
	// services/gateway/deploytest/deploytest.go → リポジトリ直下
	return filepath.Clean(filepath.Join(filepath.Dir(self), "..", "..", ".."))
}

// RepoPath はリポジトリ直下からの相対パスを絶対パスにする。
func RepoPath(t testing.TB, rel string) string {
	t.Helper()
	return filepath.Join(RepoRoot(t), filepath.FromSlash(rel))
}

// Kustomization は kustomization.yaml のうち検査に使う部分。
type Kustomization struct {
	Kind               string               `yaml:"kind"`
	Namespace          string               `yaml:"namespace"`
	Resources          []string             `yaml:"resources"`
	Components         []string             `yaml:"components"`
	Patches            []KustomizePatch     `yaml:"patches"`
	ConfigMapGenerator []ConfigMapGenerator `yaml:"configMapGenerator"`
	Images             []KustomizeImage     `yaml:"images"`
}

// KustomizePatch は patches の1件(path で strategic merge patch のファイルを指すものだけを扱う)。
type KustomizePatch struct {
	Path  string `yaml:"path"`
	Patch string `yaml:"patch"`
}

// ConfigMapGenerator は configMapGenerator の1件。Files は "key=file" か "file"。
type ConfigMapGenerator struct {
	Name  string   `yaml:"name"`
	Files []string `yaml:"files"`
}

// KustomizeImage は images の1件。
type KustomizeImage struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
	Digest  string `yaml:"digest"`
}

// ReadKustomization は dir(リポジトリ直下からの相対)の kustomization.yaml を読む。
func ReadKustomization(t testing.TB, dir string) Kustomization {
	t.Helper()
	path := RepoPath(t, dir+"/kustomization.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s/kustomization.yaml を読めない: %v", dir, err)
	}
	var k Kustomization
	if err := yaml.Unmarshal(raw, &k); err != nil {
		t.Fatalf("%s/kustomization.yaml を解析できない: %v", dir, err)
	}
	return k
}

// Object は1つの k8s オブジェクト(kind と名前、元の YAML)。
type Object struct {
	File string // リポジトリ直下からの相対
	Kind string
	Name string
	node yaml.Node
}

// Decode はオブジェクトを型付きの構造体に読む。
func (o Object) Decode(t testing.TB, v any) {
	t.Helper()
	if err := o.node.Decode(v); err != nil {
		t.Fatalf("%s の %s/%s を解析できない: %v", o.File, o.Kind, o.Name, err)
	}
}

// ReadObjects は file(リポジトリ直下からの相対)の YAML の全ドキュメントを読む。
func ReadObjects(t testing.TB, file string) []Object {
	t.Helper()
	raw, err := os.ReadFile(RepoPath(t, file))
	if err != nil {
		t.Fatalf("%s を読めない: %v", file, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var objs []Object
	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("%s を解析できない: %v", file, err)
		}
		var head struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		if err := node.Decode(&head); err != nil {
			t.Fatalf("%s の kind / metadata.name を読めない: %v", file, err)
		}
		if head.Kind == "" {
			continue // 空のドキュメント
		}
		objs = append(objs, Object{File: file, Kind: head.Kind, Name: head.Metadata.Name, node: node})
	}
	return objs
}

// BaseObjects は deploy/k8s/base/<service> の kustomization.yaml の resources(ファイル)を全て読む。
func BaseObjects(t testing.TB, service string) []Object {
	t.Helper()
	dir := BaseDir + "/" + service
	k := ReadKustomization(t, dir)
	if len(k.Resources) == 0 {
		t.Fatalf("%s/kustomization.yaml の resources が空", dir)
	}
	var objs []Object
	for _, r := range k.Resources {
		objs = append(objs, ReadObjects(t, dir+"/"+r)...)
	}
	return objs
}

// Find は kind と名前が一致するオブジェクトを1つ返す(無い・複数なら失敗)。
func Find(t testing.TB, objs []Object, kind, name string) Object {
	t.Helper()
	var found []Object
	for _, o := range objs {
		if o.Kind == kind && o.Name == name {
			found = append(found, o)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s/%s が %d 個ある(1個であること)", kind, name, len(found))
	}
	return found[0]
}

// --- k8s の型(検査に使う部分だけ) -------------------------------------------------

// Deployment は apps/v1 Deployment の一部。
type Deployment struct {
	Spec struct {
		Selector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Template struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec PodSpec `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// PodSpec は Pod の spec の一部。
type PodSpec struct {
	AutomountServiceAccountToken *bool              `yaml:"automountServiceAccountToken"`
	SecurityContext              PodSecurityContext `yaml:"securityContext"`
	Containers                   []Container        `yaml:"containers"`
	Volumes                      []Volume           `yaml:"volumes"`
}

// PodSecurityContext は Pod の securityContext の一部。
type PodSecurityContext struct {
	RunAsNonRoot   *bool  `yaml:"runAsNonRoot"`
	RunAsUser      *int64 `yaml:"runAsUser"`
	SeccompProfile struct {
		Type string `yaml:"type"`
	} `yaml:"seccompProfile"`
}

// Container はコンテナの一部。
type Container struct {
	Name            string                   `yaml:"name"`
	Image           string                   `yaml:"image"`
	ImagePullPolicy string                   `yaml:"imagePullPolicy"`
	Ports           []ContainerPort          `yaml:"ports"`
	Env             []EnvVar                 `yaml:"env"`
	ReadinessProbe  *Probe                   `yaml:"readinessProbe"`
	LivenessProbe   *Probe                   `yaml:"livenessProbe"`
	Resources       ResourceRequirements     `yaml:"resources"`
	SecurityContext ContainerSecurityContext `yaml:"securityContext"`
	VolumeMounts    []VolumeMount            `yaml:"volumeMounts"`
}

// ContainerPort はコンテナのポート。
type ContainerPort struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
}

// EnvVar は環境変数(valueFrom は使っているかどうかだけ見る)。
type EnvVar struct {
	Name      string    `yaml:"name"`
	Value     *string   `yaml:"value"`
	ValueFrom yaml.Node `yaml:"valueFrom"`
}

// Probe は probe の一部(httpGet だけ)。
type Probe struct {
	HTTPGet *struct {
		Path string    `yaml:"path"`
		Port yaml.Node `yaml:"port"` // 名前(文字列)か番号
	} `yaml:"httpGet"`
}

// ProbePort は httpGet.port を文字列で返す(名前か番号)。
func (p Probe) ProbePort() string {
	if p.HTTPGet == nil {
		return ""
	}
	return p.HTTPGet.Port.Value
}

// ResourceRequirements は resources。
type ResourceRequirements struct {
	Requests map[string]string `yaml:"requests"`
	Limits   map[string]string `yaml:"limits"`
}

// ContainerSecurityContext はコンテナの securityContext の一部。
type ContainerSecurityContext struct {
	AllowPrivilegeEscalation *bool  `yaml:"allowPrivilegeEscalation"`
	ReadOnlyRootFilesystem   *bool  `yaml:"readOnlyRootFilesystem"`
	RunAsNonRoot             *bool  `yaml:"runAsNonRoot"`
	RunAsUser                *int64 `yaml:"runAsUser"`
	Capabilities             struct {
		Drop []string `yaml:"drop"`
	} `yaml:"capabilities"`
}

// VolumeMount はボリュームのマウント。
type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	SubPath   string `yaml:"subPath"`
	ReadOnly  bool   `yaml:"readOnly"`
}

// Volume は Pod のボリューム(configMap だけを扱う)。
type Volume struct {
	Name      string `yaml:"name"`
	ConfigMap *struct {
		Name  string `yaml:"name"`
		Items []struct {
			Key  string `yaml:"key"`
			Path string `yaml:"path"`
		} `yaml:"items"`
	} `yaml:"configMap"`
}

// Service は v1 Service の一部。
type Service struct {
	Spec struct {
		Selector map[string]string `yaml:"selector"`
		Ports    []struct {
			Name       string    `yaml:"name"`
			Port       int       `yaml:"port"`
			TargetPort yaml.Node `yaml:"targetPort"`
		} `yaml:"ports"`
	} `yaml:"spec"`
}

// Ingress は networking.k8s.io/v1 Ingress の一部。
type Ingress struct {
	Spec struct {
		IngressClassName string `yaml:"ingressClassName"`
		Rules            []struct {
			Host string `yaml:"host"`
			HTTP struct {
				Paths []struct {
					Path     string `yaml:"path"`
					PathType string `yaml:"pathType"`
					Backend  struct {
						Service struct {
							Name string `yaml:"name"`
							Port struct {
								Name   string `yaml:"name"`
								Number int    `yaml:"number"`
							} `yaml:"port"`
						} `yaml:"service"`
					} `yaml:"backend"`
				} `yaml:"paths"`
			} `yaml:"http"`
		} `yaml:"rules"`
	} `yaml:"spec"`
}

// Container はコンテナを名前で返す(無ければ失敗)。
func (d *Deployment) Container(t testing.TB, name string) *Container {
	t.Helper()
	for i := range d.Spec.Template.Spec.Containers {
		if d.Spec.Template.Spec.Containers[i].Name == name {
			return &d.Spec.Template.Spec.Containers[i]
		}
	}
	t.Fatalf("コンテナ %q が無い", name)
	return nil
}

// EnvMap はコンテナの環境変数を name → value にする。valueFrom を使うもの・値の無いものは失敗にする
// (API レーンの設定は ConfigMap の値ではなく平文の value で渡す。秘密を持たないため)。
func (c *Container) EnvMap(t testing.TB) map[string]string {
	t.Helper()
	env := make(map[string]string, len(c.Env))
	for _, e := range c.Env {
		if e.ValueFrom.Kind != 0 {
			t.Errorf("環境変数 %s が valueFrom を使っている(平文の value で渡すこと)", e.Name)
			continue
		}
		if e.Value == nil {
			t.Errorf("環境変数 %s に value が無い", e.Name)
			continue
		}
		if _, dup := env[e.Name]; dup {
			t.Errorf("環境変数 %s が重複している", e.Name)
		}
		env[e.Name] = *e.Value
	}
	return env
}

// BaseDeployment は deploy/k8s/base/<service> の Deployment(名前はサービス名)を読む。
func BaseDeployment(t testing.TB, service string) Deployment {
	t.Helper()
	var d Deployment
	Find(t, BaseObjects(t, service), "Deployment", service).Decode(t, &d)
	return d
}

// LocalDeployment は base の Deployment に local の API Component の patch と images を合成したものを返す。
func LocalDeployment(t testing.TB, service string) Deployment {
	t.Helper()
	d := BaseDeployment(t, service)
	comp := ReadKustomization(t, LocalAPIComponentDir)
	for _, p := range comp.Patches {
		if p.Path == "" {
			t.Fatalf("%s の patches は path でファイルを指すこと(インラインの patch は検査できない)", LocalAPIComponentDir)
		}
		for _, o := range ReadObjects(t, LocalAPIComponentDir+"/"+p.Path) {
			if o.Kind != "Deployment" || o.Name != service {
				continue
			}
			var patch Deployment
			o.Decode(t, &patch)
			assertPatchFieldsSupported(t, LocalAPIComponentDir+"/"+p.Path, o)
			mergeDeployment(&d, patch)
		}
	}
	for i := range d.Spec.Template.Spec.Containers {
		c := &d.Spec.Template.Spec.Containers[i]
		c.Image = applyImageOverride(c.Image, comp.Images)
	}
	return d
}

// mergeDeployment は strategic merge patch のうち、API レーンが使う部分(env・volumeMounts・volumes)を合成する。
func mergeDeployment(dst *Deployment, patch Deployment) {
	pod := &dst.Spec.Template.Spec
	pod.Volumes = append(pod.Volumes, patch.Spec.Template.Spec.Volumes...)
	for _, pc := range patch.Spec.Template.Spec.Containers {
		for i := range pod.Containers {
			c := &pod.Containers[i]
			if c.Name != pc.Name {
				continue
			}
			for _, e := range pc.Env {
				replaced := false
				for j := range c.Env {
					if c.Env[j].Name == e.Name {
						c.Env[j] = e
						replaced = true
					}
				}
				if !replaced {
					c.Env = append(c.Env, e)
				}
			}
			c.VolumeMounts = append(c.VolumeMounts, pc.VolumeMounts...)
			if pc.Image != "" {
				c.Image = pc.Image
			}
		}
	}
}

// patchAllowedFields は、strategic merge patch(Deployment)のうち mergeDeployment が実際に合成する
// フィールドを、パス(ドット区切り。配列は "[]")ごとに列挙する。ここに無いフィールドを patch に書いても
// mergeDeployment は静かに無視する(実物の kustomize は適用するが、この静的検査は再現しない)ため、
// 気づかないまま乖離することを防ぐ(任意項目。critic 指摘)。
var patchAllowedFields = map[string][]string{
	"":                                      {"apiVersion", "kind", "metadata", "spec"},
	"metadata":                              {"name"},
	"spec":                                  {"template"},
	"spec.template":                         {"spec"},
	"spec.template.spec":                    {"containers", "volumes"},
	"spec.template.spec.containers[]":       {"name", "env", "volumeMounts", "image"},
	"spec.template.spec.containers[].env[]": {"name", "value", "valueFrom"},
	"spec.template.spec.containers[].volumeMounts[]": {"name", "mountPath", "subPath", "readOnly"},
	"spec.template.spec.volumes[]":                   {"name", "configMap"},
	"spec.template.spec.volumes[].configMap":         {"name", "items"},
	"spec.template.spec.volumes[].configMap.items[]": {"key", "path"},
}

// assertPatchFieldsSupported は o(strategic merge patch の Deployment)が patchAllowedFields の
// 範囲だけを使っていることを確かめる。範囲外のフィールドがあれば、この静的検査が実物の kustomize と
// 乖離する(patch は適用されるのに、テストはそれを見ない)ので、テストを失敗させる。
func assertPatchFieldsSupported(t testing.TB, file string, o Object) {
	t.Helper()
	var generic any
	if err := o.node.Decode(&generic); err != nil {
		t.Fatalf("%s を解析できない: %v", file, err)
	}
	walkPatchFields(t, file, "", generic)
}

func walkPatchFields(t testing.TB, file, path string, node any) {
	t.Helper()
	switch v := node.(type) {
	case map[string]any:
		allowed, ok := patchAllowedFields[path]
		if !ok {
			t.Fatalf("%s: mergeDeployment が検査しない場所 %q が patch にある(mergeDeployment・patchAllowedFields を拡張すること)",
				file, path)
			return
		}
		for key, val := range v {
			if !slices.Contains(allowed, key) {
				t.Fatalf("%s: %s に mergeDeployment が合成しないフィールド %q がある"+
					"(patch はこの範囲だけを使うこと。ADR-0203 §3)", file, path, key)
				continue
			}
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			walkPatchFields(t, file, childPath, val)
		}
	case []any:
		childPath := path + "[]"
		for _, item := range v {
			walkPatchFields(t, file, childPath, item)
		}
	}
}

// SplitImage はイメージ参照を名前・タグ・digest に分ける。
func SplitImage(ref string) (name, tag, digest string) {
	name = ref
	if n, d, ok := strings.Cut(name, "@"); ok {
		name, digest = n, d
	}
	// タグの ":" はレジストリのポートの ":" と区別する(最後の "/" の後ろだけを見る)。
	slash := strings.LastIndex(name, "/")
	if colon := strings.LastIndex(name, ":"); colon > slash {
		name, tag = name[:colon], name[colon+1:]
	}
	return name, tag, digest
}

func applyImageOverride(ref string, images []KustomizeImage) string {
	name, tag, digest := SplitImage(ref)
	for _, img := range images {
		if img.Name != name {
			continue
		}
		if img.NewName != "" {
			name = img.NewName
		}
		if img.NewTag != "" {
			tag = img.NewTag
		}
		if img.Digest != "" {
			digest = img.Digest
		}
	}
	out := name
	if tag != "" {
		out += ":" + tag
	}
	if digest != "" {
		out += "@" + digest
	}
	return out
}

// Workload は AssertWorkload に渡す、サービスごとの期待値。
type Workload struct {
	Service     string // Deployment・Service・コンテナの名前(例 "calc")
	ImageRepo   string // イメージ名(タグ抜き。例 "pokecalc/calc")
	AddrEnv     string // 待ち受けアドレスの環境変数名(cmd の定数)
	DefaultAddr string // 待ち受けアドレスの既定値(cmd の定数)
	// ReadinessPath は readinessProbe のパス。空なら HealthzPath(liveness と同じ)。
	// calc-svc はマスタの取得前に Service の宛先へ入らないよう /readyz を使う(ADR-0204 §3)。
	ReadinessPath string
}

// ServicePort は Service の公開ポート。gateway は上流を http://<Service 名> で指すので 80 に固定する(ADR-0203 §3)。
const ServicePort = 80

// HealthzPath は liveness の probe のパス(calc-svc・gateway の運用エンドポイント。ADR-0200 / ADR-0202)。
// readiness も、Workload.ReadinessPath が空ならこのパス。
const HealthzPath = "/healthz"

// AssertWorkload は base の Deployment と Service が ADR-0203 §3 の形であることを確かめる:
// 名前・ラベル・イメージ・ポート・probe(liveness は /healthz、readiness は Workload.ReadinessPath か /healthz)・
// resources・非 root・readOnlyRootFilesystem・Service の 80 番。
func AssertWorkload(t *testing.T, w Workload) {
	t.Helper()
	objs := BaseObjects(t, w.Service)
	var d Deployment
	Find(t, objs, "Deployment", w.Service).Decode(t, &d)
	pod := d.Spec.Template.Spec

	if len(d.Spec.Selector.MatchLabels) == 0 {
		t.Error("Deployment の selector.matchLabels が空")
	}
	for k, v := range d.Spec.Selector.MatchLabels {
		if d.Spec.Template.Metadata.Labels[k] != v {
			t.Errorf("selector のラベル %s=%s が Pod テンプレートに無い", k, v)
		}
	}
	if len(pod.Containers) != 1 {
		t.Fatalf("コンテナが %d 個(1個であること)", len(pod.Containers))
	}
	c := d.Container(t, w.Service)

	if name, _, _ := SplitImage(c.Image); name != w.ImageRepo {
		t.Errorf("image = %q, want %s:<tag>", c.Image, w.ImageRepo)
	}
	if c.ImagePullPolicy != "IfNotPresent" && c.ImagePullPolicy != "Never" {
		t.Errorf("imagePullPolicy = %q, want IfNotPresent か Never(k3d image import したイメージを使う)", c.ImagePullPolicy)
	}

	if len(c.Ports) != 1 || c.Ports[0].Name != "http" || c.Ports[0].ContainerPort <= 0 {
		t.Fatalf("ports = %+v, want 名前 http のポート1つ", c.Ports)
	}
	containerPort := c.Ports[0].ContainerPort
	addr := w.DefaultAddr
	if env := c.EnvMap(t); env[w.AddrEnv] != "" {
		addr = env[w.AddrEnv]
	}
	if _, port, _ := strings.Cut(addr, ":"); port != itoa(containerPort) {
		t.Errorf("待ち受けアドレス %q(%s か既定値)のポートが containerPort %d と違う", addr, w.AddrEnv, containerPort)
	}

	readinessPath := w.ReadinessPath
	if readinessPath == "" {
		readinessPath = HealthzPath
	}
	probes := []struct {
		name     string
		probe    *Probe
		wantPath string
	}{
		{"readinessProbe", c.ReadinessProbe, readinessPath},
		{"livenessProbe", c.LivenessProbe, HealthzPath},
	}
	for _, pr := range probes {
		name, p := pr.name, pr.probe
		if p == nil || p.HTTPGet == nil {
			t.Errorf("%s の httpGet が無い", name)
			continue
		}
		if p.HTTPGet.Path != pr.wantPath {
			t.Errorf("%s.httpGet.path = %q, want %s", name, p.HTTPGet.Path, pr.wantPath)
		}
		if port := p.ProbePort(); port != "http" && port != itoa(containerPort) {
			t.Errorf("%s.httpGet.port = %q, want http か %d", name, port, containerPort)
		}
	}

	for _, kind := range []string{"cpu", "memory"} {
		if c.Resources.Requests[kind] == "" {
			t.Errorf("resources.requests.%s が無い", kind)
		}
		if c.Resources.Limits[kind] == "" {
			t.Errorf("resources.limits.%s が無い", kind)
		}
	}

	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Error("automountServiceAccountToken が false でない(API を呼ばないので token を持たせない)")
	}
	if !isTrue(pod.SecurityContext.RunAsNonRoot) && !isTrue(c.SecurityContext.RunAsNonRoot) {
		t.Error("runAsNonRoot: true が無い(Pod かコンテナの securityContext)")
	}
	for _, uid := range []*int64{pod.SecurityContext.RunAsUser, c.SecurityContext.RunAsUser} {
		if uid != nil && *uid == 0 {
			t.Error("runAsUser が 0(root)")
		}
	}
	if pod.SecurityContext.SeccompProfile.Type != "RuntimeDefault" {
		t.Errorf("seccompProfile.type = %q, want RuntimeDefault", pod.SecurityContext.SeccompProfile.Type)
	}
	if !isTrue(c.SecurityContext.ReadOnlyRootFilesystem) {
		t.Error("readOnlyRootFilesystem: true が無い")
	}
	if c.SecurityContext.AllowPrivilegeEscalation == nil || *c.SecurityContext.AllowPrivilegeEscalation {
		t.Error("allowPrivilegeEscalation: false が無い")
	}
	if len(c.SecurityContext.Capabilities.Drop) != 1 || c.SecurityContext.Capabilities.Drop[0] != "ALL" {
		t.Errorf("capabilities.drop = %v, want [ALL]", c.SecurityContext.Capabilities.Drop)
	}

	var svc Service
	Find(t, objs, "Service", w.Service).Decode(t, &svc)
	if len(svc.Spec.Selector) == 0 {
		t.Error("Service の selector が空")
	}
	for k, v := range svc.Spec.Selector {
		if d.Spec.Template.Metadata.Labels[k] != v {
			t.Errorf("Service の selector %s=%s が Pod テンプレートのラベルに無い", k, v)
		}
	}
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("Service のポートが %d 個(1個であること)", len(svc.Spec.Ports))
	}
	sp := svc.Spec.Ports[0]
	if sp.Name != "http" || sp.Port != ServicePort {
		t.Errorf("Service のポート = %s:%d, want http:%d", sp.Name, sp.Port, ServicePort)
	}
	if tp := sp.TargetPort.Value; tp != "http" && tp != itoa(containerPort) {
		t.Errorf("Service の targetPort = %q, want http か %d", tp, containerPort)
	}
}

// AssertLocalImage は local で使うイメージが <repo>:local であることを確かめる(api-docker-build が作る名前)。
func AssertLocalImage(t *testing.T, service, imageRepo string) {
	t.Helper()
	d := LocalDeployment(t, service)
	c := d.Container(t, service)
	if want := imageRepo + ":local"; c.Image != want {
		t.Errorf("local overlay での image = %q, want %q(make api-docker-build が作る名前)", c.Image, want)
	}
}

func isTrue(b *bool) bool { return b != nil && *b }

func itoa(n int) string { return strconv.Itoa(n) }
