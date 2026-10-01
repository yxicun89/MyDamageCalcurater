package deploytest

// TiDB を使うサービス(record・team)の base の静的検査(ADR-0220 §1・§3。AC-K1〜K4)。
// AssertWorkload は env を平文の value に限る(API レーンは秘密を持たない)ので、DSN を Secret から受ける
// record・team はこちらを使う。pokedex の manifest_test と同じ観点(secretKeyRef・非 root・probe・resources)に、
// ConfigMap の envFrom・GOMEMLIMIT・preStop・CronJob を加える。

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TiDBService は AssertTiDBService に渡す、サービスごとの期待値。
type TiDBService struct {
	Service     string // Deployment・Service・コンテナの名前(例 "record")
	ImageRepo   string // 例 "pokecalc/record"
	AddrEnv     string // 待ち受けアドレスの環境変数名(cmd の定数)
	DefaultAddr string // 待ち受けアドレスの既定値(cmd の定数)
	DSNEnv      string // 例 "RECORD_APP_DSN"
	Secret      string // 例 "record-db-auth"
	SecretKey   string // 例 "record-app-dsn"(app ロール。ADR-0211 §4)
	// ForbiddenSecrets はこのサービスの Pod が参照してはいけない Secret(他サービスの Secret・tidb-root-auth)。
	ForbiddenSecrets []string
	ConfigMap        string // 保持日数の ConfigMap(例 "record-retention")
	CronJob          string // 失効ジョブ(例 "record-expire")
	// ShutdownTimeout は cmd/main.go の shutdownTimeout(grace > preStop + これ)。
	ShutdownTimeout time.Duration
}

// --- 検査に使う型(既存の Deployment 型は args・envFrom・lifecycle を持たないので別に持つ) ---

type secretKeyRef struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

type tidbEnvVar struct {
	Name      string  `yaml:"name"`
	Value     *string `yaml:"value"`
	ValueFrom *struct {
		SecretKeyRef    *secretKeyRef `yaml:"secretKeyRef"`
		ConfigMapKeyRef *secretKeyRef `yaml:"configMapKeyRef"`
	} `yaml:"valueFrom"`
}

type tidbContainer struct {
	Name            string          `yaml:"name"`
	Image           string          `yaml:"image"`
	ImagePullPolicy string          `yaml:"imagePullPolicy"`
	Args            []string        `yaml:"args"`
	Command         []string        `yaml:"command"`
	Ports           []ContainerPort `yaml:"ports"`
	Env             []tidbEnvVar    `yaml:"env"`
	EnvFrom         []struct {
		ConfigMapRef *struct {
			Name string `yaml:"name"`
		} `yaml:"configMapRef"`
		SecretRef *struct {
			Name string `yaml:"name"`
		} `yaml:"secretRef"`
	} `yaml:"envFrom"`
	ReadinessProbe  *Probe                   `yaml:"readinessProbe"`
	LivenessProbe   *Probe                   `yaml:"livenessProbe"`
	Resources       ResourceRequirements     `yaml:"resources"`
	SecurityContext ContainerSecurityContext `yaml:"securityContext"`
	Lifecycle       struct {
		PreStop struct {
			Sleep struct {
				Seconds int `yaml:"seconds"`
			} `yaml:"sleep"`
		} `yaml:"preStop"`
	} `yaml:"lifecycle"`
}

type tidbPodSpec struct {
	AutomountServiceAccountToken  *bool              `yaml:"automountServiceAccountToken"`
	TerminationGracePeriodSeconds int                `yaml:"terminationGracePeriodSeconds"`
	RestartPolicy                 string             `yaml:"restartPolicy"`
	HostNetwork                   bool               `yaml:"hostNetwork"`
	SecurityContext               PodSecurityContext `yaml:"securityContext"`
	InitContainers                []tidbContainer    `yaml:"initContainers"`
	Containers                    []tidbContainer    `yaml:"containers"`
}

type tidbPodTemplate struct {
	Metadata struct {
		Labels map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec tidbPodSpec `yaml:"spec"`
}

type tidbDeployment struct {
	Spec struct {
		Replicas *int `yaml:"replicas"`
		Selector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		Template tidbPodTemplate `yaml:"template"`
	} `yaml:"spec"`
}

type tidbCronJob struct {
	Spec struct {
		Schedule                string `yaml:"schedule"`
		TimeZone                string `yaml:"timeZone"`
		ConcurrencyPolicy       string `yaml:"concurrencyPolicy"`
		StartingDeadlineSeconds int    `yaml:"startingDeadlineSeconds"`
		Suspend                 *bool  `yaml:"suspend"`
		JobTemplate             struct {
			Spec struct {
				BackoffLimit            *int              `yaml:"backoffLimit"`
				ActiveDeadlineSeconds   int               `yaml:"activeDeadlineSeconds"`
				TTLSecondsAfterFinished *int              `yaml:"ttlSecondsAfterFinished"`
				PodFailurePolicy        *podFailurePolicy `yaml:"podFailurePolicy"`
				Template                tidbPodTemplate   `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate"`
	} `yaml:"spec"`
}

type podFailurePolicy struct {
	Rules []struct {
		Action      string `yaml:"action"`
		OnExitCodes *struct {
			ContainerName string `yaml:"containerName"`
			Operator      string `yaml:"operator"`
			Values        []int  `yaml:"values"`
		} `yaml:"onExitCodes"`
	} `yaml:"rules"`
}

type configMap struct {
	Data map[string]string `yaml:"data"`
}

// SecretPlaceholder は secretKeyRef の値の代わりに env に入れる、DSN として解釈できる架空の値
// (loadConfig / loadExpireConfig を manifest の env で通すため。実際の資格情報ではない)。
const SecretPlaceholder = "placeholder:placeholder@tcp(tidb.invalid:4000)/placeholder?parseTime=true"

// TiDBServiceEnv は base の Deployment(または CronJob)のコンテナが受け取る環境変数を、
// envFrom の ConfigMap → env の順に合成して返す(Kustomize と k8s の優先順位どおり env が後勝ち)。
// secretKeyRef は SecretPlaceholder に置き換える。
type TiDBServiceEnv struct {
	Deployment map[string]string
	CronJob    map[string]string
}

// AssertTiDBService は base/<Service> の Deployment・Service・ConfigMap・CronJob を検査し、
// 合成した環境変数を返す(呼び出し側は自分の loadConfig / loadExpireConfig に通す)。
func AssertTiDBService(t *testing.T, w TiDBService) TiDBServiceEnv {
	t.Helper()
	dir := BaseDir + "/" + w.Service
	k := ReadKustomization(t, dir)
	for _, want := range []string{"deployment.yaml", "service.yaml", "configmap-retention.yaml", "cronjob-expire.yaml"} {
		if !slices.Contains(k.Resources, want) {
			t.Errorf("%s/kustomization.yaml の resources に %s が無い: %q", dir, want, k.Resources)
		}
	}
	if slices.Contains(k.Resources, "job-migrate.yaml") {
		t.Errorf("%s/kustomization.yaml の resources に job-migrate.yaml がある(up.sh が TidbInitializer の後に個別 apply する。ADR-0211 §3.2)", dir)
	}
	objs := BaseObjects(t, w.Service)

	var cm configMap
	Find(t, objs, "ConfigMap", w.ConfigMap).Decode(t, &cm)
	if len(cm.Data) == 0 {
		t.Errorf("ConfigMap %s の data が空", w.ConfigMap)
	}

	// --- Deployment ---
	var d tidbDeployment
	Find(t, objs, "Deployment", w.Service).Decode(t, &d)
	for k, v := range d.Spec.Selector.MatchLabels {
		if d.Spec.Template.Metadata.Labels[k] != v {
			t.Errorf("Deployment の selector %s=%s が Pod テンプレートに無い", k, v)
		}
	}
	if got := d.Spec.Template.Metadata.Labels["app.kubernetes.io/name"]; got != w.Service {
		t.Errorf("Pod のラベル app.kubernetes.io/name = %q, want %q(NetworkPolicy が名前で選ぶ)", got, w.Service)
	}
	pod := d.Spec.Template.Spec
	if len(pod.Containers) != 1 {
		t.Fatalf("Deployment のコンテナが %d 個(1個であること)", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.Name != w.Service {
		t.Errorf("コンテナ名 = %q, want %q", c.Name, w.Service)
	}
	assertTiDBImage(t, "Deployment", c, w.ImageRepo)
	if strings.Join(c.Args, " ") != "serve" || len(c.Command) != 0 {
		t.Errorf("Deployment の command/args = %v/%v, want args [serve] だけ(ENTRYPOINT はイメージのもの)", c.Command, c.Args)
	}
	if len(c.Ports) != 1 || c.Ports[0].Name != "http" || c.Ports[0].ContainerPort <= 0 || c.Ports[0].HostPort != 0 {
		t.Fatalf("ports = %+v, want 名前 http のポート1つ(hostPort なし)", c.Ports)
	}
	containerPort := c.Ports[0].ContainerPort
	depEnv := mergedEnv(t, "Deployment", c, w, cm)
	addr := w.DefaultAddr
	if v := depEnv[w.AddrEnv]; v != "" {
		addr = v
	}
	if _, port, _ := strings.Cut(addr, ":"); port != itoa(containerPort) {
		t.Errorf("待ち受けアドレス %q のポートが containerPort %d と違う", addr, containerPort)
	}
	for _, p := range []struct {
		name  string
		probe *Probe
		path  string
	}{
		// readiness は DB に届くかの /readyz、liveness は DB に触れない /healthz(pokedex と同じ。ADR-0129)。
		{"readinessProbe", c.ReadinessProbe, "/readyz"},
		{"livenessProbe", c.LivenessProbe, HealthzPath},
	} {
		if p.probe == nil || p.probe.HTTPGet == nil || p.probe.HTTPGet.Path != p.path {
			t.Errorf("%s が httpGet %s でない", p.name, p.path)
			continue
		}
		if port := p.probe.ProbePort(); port != "http" && port != itoa(containerPort) {
			t.Errorf("%s の port = %q, want http", p.name, port)
		}
	}
	assertTiDBPodHardening(t, "Deployment", pod, c)
	// GOMEMLIMIT(issue #298): limits.memory の 75% 以上・未満。
	assertGoMemLimit(t, c)
	// preStop と grace(ADR-0129 §4・issue #324)。
	preStop := c.Lifecycle.PreStop.Sleep.Seconds
	if preStop <= 0 {
		t.Error("Deployment に lifecycle.preStop.sleep.seconds が無い")
	}
	if grace := time.Duration(pod.TerminationGracePeriodSeconds) * time.Second; grace <= time.Duration(preStop)*time.Second+w.ShutdownTimeout {
		t.Errorf("terminationGracePeriodSeconds=%v は preStop(%ds)+shutdownTimeout(%v) より長いこと", grace, preStop, w.ShutdownTimeout)
	}

	// --- Service ---
	var svc Service
	Find(t, objs, "Service", w.Service).Decode(t, &svc)
	if svc.Spec.Type != "" && svc.Spec.Type != "ClusterIP" {
		t.Errorf("Service の type = %q, want ClusterIP(公開は gateway 経由)", svc.Spec.Type)
	}
	if len(svc.Spec.ExternalIPs) != 0 {
		t.Errorf("Service に externalIPs がある: %v", svc.Spec.ExternalIPs)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Name != "http" || svc.Spec.Ports[0].Port != ServicePort {
		t.Errorf("Service のポート = %+v, want http:%d", svc.Spec.Ports, ServicePort)
	} else if tp := svc.Spec.Ports[0].TargetPort.Value; tp != "http" && tp != itoa(containerPort) {
		t.Errorf("Service の targetPort = %q, want http", tp)
	}
	if len(svc.Spec.Selector) == 0 {
		t.Error("Service の selector が空")
	}
	for k, v := range svc.Spec.Selector {
		if d.Spec.Template.Metadata.Labels[k] != v {
			t.Errorf("Service の selector %s=%s が Pod のラベルに無い", k, v)
		}
	}
	for _, o := range objs {
		if o.Kind == "Ingress" {
			t.Errorf("base/%s に Ingress %s がある(公開は gateway 経由だけ)", w.Service, o.Name)
		}
	}

	// --- CronJob(失効ジョブ。ADR-0220 §3) ---
	var cj tidbCronJob
	Find(t, objs, "CronJob", w.CronJob).Decode(t, &cj)
	spec := cj.Spec
	if fields := strings.Fields(spec.Schedule); len(fields) != 5 || fields[2] != "*" || fields[3] != "*" || fields[4] != "*" ||
		strings.ContainsAny(fields[0]+fields[1], "*/,-") {
		t.Errorf("schedule = %q, want 日次(分・時が固定の1つ、日・月・曜日が *)", spec.Schedule)
	}
	if spec.TimeZone == "" {
		t.Error("CronJob の timeZone が無い(ノート PC の k3d が起動している時間帯を日本時間で書く)")
	}
	if spec.ConcurrencyPolicy != "Forbid" {
		t.Errorf("concurrencyPolicy = %q, want Forbid", spec.ConcurrencyPolicy)
	}
	if s := spec.StartingDeadlineSeconds; s <= 0 || s >= 24*60*60 {
		t.Errorf("startingDeadlineSeconds = %d, want 0 より大きく1日未満(次の予定と重ねない)", s)
	}
	if spec.Suspend != nil && *spec.Suspend {
		t.Error("base の CronJob が suspend されている(suspend は cloud overlay だけ)")
	}
	job := spec.JobTemplate.Spec
	if job.BackoffLimit == nil || job.ActiveDeadlineSeconds <= 0 || job.TTLSecondsAfterFinished == nil {
		t.Errorf("jobTemplate に backoffLimit・activeDeadlineSeconds・ttlSecondsAfterFinished が揃っていない: %+v", job)
	}
	if !podFailurePolicyFailsOnUsageError(job.PodFailurePolicy, w.Service) {
		t.Errorf("podFailurePolicy に「コンテナ %s の終了コード 2 で FailJob」が無い(使い方の誤りは再試行しない)", w.Service)
	}
	if got := job.Template.Metadata.Labels["app.kubernetes.io/name"]; got != w.CronJob {
		t.Errorf("CronJob の Pod のラベル app.kubernetes.io/name = %q, want %q(NetworkPolicy が名前で選ぶ)", got, w.CronJob)
	}
	jpod := job.Template.Spec
	if jpod.RestartPolicy != "Never" && jpod.RestartPolicy != "OnFailure" {
		t.Errorf("CronJob の restartPolicy = %q", jpod.RestartPolicy)
	}
	if len(jpod.Containers) != 1 {
		t.Fatalf("CronJob のコンテナが %d 個(1個であること)", len(jpod.Containers))
	}
	jc := jpod.Containers[0]
	if jc.Name != w.Service {
		t.Errorf("CronJob のコンテナ名 = %q, want %q(podFailurePolicy が名前で指す)", jc.Name, w.Service)
	}
	assertTiDBImage(t, "CronJob", jc, w.ImageRepo)
	if jc.Image != c.Image {
		t.Errorf("CronJob の image = %q, want Deployment と同じ %q(同じバイナリのサブコマンド)", jc.Image, c.Image)
	}
	if strings.Join(jc.Args, " ") != "expire" || len(jc.Command) != 0 {
		t.Errorf("CronJob の command/args = %v/%v, want args [expire] だけ", jc.Command, jc.Args)
	}
	if len(jc.Ports) != 0 {
		t.Errorf("CronJob のコンテナが ports を持つ: %+v(待ち受けない)", jc.Ports)
	}
	assertTiDBPodHardening(t, "CronJob", jpod, jc)
	cronEnv := mergedEnv(t, "CronJob", jc, w, cm)
	for name := range cronEnv {
		if strings.HasSuffix(name, "_NATS_URL") {
			t.Errorf("CronJob に %s がある(失効ジョブは NATS に触れない)", name)
		}
	}

	// --- 他サービスの Secret を参照しない(ADR-0211 §4) ---
	for _, o := range objs {
		raw := o.node
		var generic any
		if err := raw.Decode(&generic); err != nil {
			t.Fatal(err)
		}
		walkStrings(generic, func(s string) {
			if slices.Contains(w.ForbiddenSecrets, s) {
				t.Errorf("base/%s の %s/%s が %q を参照している(自分の Secret %s だけ)", w.Service, o.Kind, o.Name, s, w.Secret)
			}
		})
	}
	return TiDBServiceEnv{Deployment: depEnv, CronJob: cronEnv}
}

func assertTiDBImage(t *testing.T, where string, c tidbContainer, repo string) {
	t.Helper()
	if name, tag, _ := SplitImage(c.Image); name != repo || tag == "" || tag == "latest" {
		t.Errorf("%s の image = %q, want %s:<固定のタグ>", where, c.Image, repo)
	}
	if c.ImagePullPolicy != "IfNotPresent" && c.ImagePullPolicy != "Never" {
		t.Errorf("%s の imagePullPolicy = %q, want IfNotPresent か Never(k3d image import したイメージ)", where, c.ImagePullPolicy)
	}
}

func assertTiDBPodHardening(t *testing.T, where string, pod tidbPodSpec, c tidbContainer) {
	t.Helper()
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Errorf("%s: automountServiceAccountToken が false でない", where)
	}
	if pod.HostNetwork {
		t.Errorf("%s: hostNetwork が true", where)
	}
	if !isTrue(pod.SecurityContext.RunAsNonRoot) {
		t.Errorf("%s: Pod の runAsNonRoot: true が無い", where)
	}
	for _, uid := range []*int64{pod.SecurityContext.RunAsUser, c.SecurityContext.RunAsUser} {
		if uid != nil && *uid == 0 {
			t.Errorf("%s: runAsUser が 0(root)", where)
		}
	}
	if pod.SecurityContext.SeccompProfile.Type != "RuntimeDefault" {
		t.Errorf("%s: seccompProfile.type = %q, want RuntimeDefault", where, pod.SecurityContext.SeccompProfile.Type)
	}
	sc := c.SecurityContext
	if !isTrue(sc.ReadOnlyRootFilesystem) {
		t.Errorf("%s: readOnlyRootFilesystem: true が無い", where)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("%s: allowPrivilegeEscalation: false が無い", where)
	}
	if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("%s: capabilities.drop = %v, want [ALL]", where, sc.Capabilities.Drop)
	}
	for _, kind := range []string{"cpu", "memory"} {
		if c.Resources.Requests[kind] == "" || c.Resources.Limits[kind] == "" {
			t.Errorf("%s: resources の %s の requests / limits が無い", where, kind)
		}
	}
}

func assertGoMemLimit(t *testing.T, c tidbContainer) {
	t.Helper()
	limit, err := parseMiBValue(c.Resources.Limits["memory"])
	if err != nil {
		t.Errorf("limits.memory: %v", err)
		return
	}
	var values []string
	for _, e := range c.Env {
		if e.Name == "GOMEMLIMIT" && e.Value != nil {
			values = append(values, *e.Value)
		}
	}
	if len(values) != 1 {
		t.Errorf("env GOMEMLIMIT が %d 個(1個であること。issue #298)", len(values))
		return
	}
	soft, err := parseMiBValue(values[0])
	if err != nil {
		t.Errorf("GOMEMLIMIT: %v", err)
		return
	}
	if soft >= limit || soft*4 < limit*3 {
		t.Errorf("GOMEMLIMIT=%dMiB は limits.memory=%dMiB の 75%% 以上・未満であること", soft, limit)
	}
}

func parseMiBValue(s string) (int, error) {
	for _, suffix := range []string{"MiB", "Mi"} {
		if n, ok := strings.CutSuffix(s, suffix); ok {
			return strconv.Atoi(n)
		}
	}
	return 0, fmt.Errorf("MiB 単位(Mi か MiB)でない: %q", s)
}

// mergedEnv は envFrom(ConfigMap)→ env の順に合成する。DSN は w.Secret / w.SecretKey の secretKeyRef だけを許す。
func mergedEnv(t *testing.T, where string, c tidbContainer, w TiDBService, cm configMap) map[string]string {
	t.Helper()
	env := map[string]string{}
	usesConfigMap := false
	for _, ef := range c.EnvFrom {
		if ef.SecretRef != nil {
			t.Errorf("%s: envFrom で Secret %s を丸ごと読んでいる(必要なキーだけを secretKeyRef で渡す)", where, ef.SecretRef.Name)
		}
		if ef.ConfigMapRef != nil {
			if ef.ConfigMapRef.Name != w.ConfigMap {
				t.Errorf("%s: envFrom の ConfigMap = %q, want %q", where, ef.ConfigMapRef.Name, w.ConfigMap)
				continue
			}
			usesConfigMap = true
			for k, v := range cm.Data {
				env[k] = v
			}
		}
	}
	if !usesConfigMap {
		t.Errorf("%s: envFrom で ConfigMap %s を読んでいない(保持日数を Deployment と CronJob で二重に書かない)", where, w.ConfigMap)
	}
	dsnFromSecret := false
	for _, e := range c.Env {
		switch {
		case e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil:
			ref := e.ValueFrom.SecretKeyRef
			if e.Name == w.DSNEnv && ref.Name == w.Secret && ref.Key == w.SecretKey {
				dsnFromSecret = true
			} else {
				t.Errorf("%s: env %s が Secret %s/%s を参照している(%s だけを %s/%s から)", where, e.Name, ref.Name, ref.Key, w.DSNEnv, w.Secret, w.SecretKey)
			}
			env[e.Name] = SecretPlaceholder
		case e.Value != nil:
			if e.Name == w.DSNEnv {
				t.Errorf("%s: %s が平文の value(Secret %s から渡すこと)", where, w.DSNEnv, w.Secret)
			}
			env[e.Name] = *e.Value
		default:
			t.Errorf("%s: env %s は value か secretKeyRef で渡すこと", where, e.Name)
		}
	}
	if !dsnFromSecret {
		t.Errorf("%s: %s が secretKeyRef %s/%s でない", where, w.DSNEnv, w.Secret, w.SecretKey)
	}
	for k := range cm.Data {
		for _, e := range c.Env {
			if e.Name == k {
				t.Errorf("%s: %s が ConfigMap と env の両方にある(ConfigMap だけに置く)", where, k)
			}
		}
	}
	return env
}

func podFailurePolicyFailsOnUsageError(p *podFailurePolicy, container string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Rules {
		if r.Action == "FailJob" && r.OnExitCodes != nil && r.OnExitCodes.ContainerName == container &&
			r.OnExitCodes.Operator == "In" && slices.Contains(r.OnExitCodes.Values, 2) {
			return true
		}
	}
	return false
}

func walkStrings(v any, f func(string)) {
	switch x := v.(type) {
	case string:
		f(x)
	case map[string]any:
		for _, val := range x {
			walkStrings(val, f)
		}
	case []any:
		for _, val := range x {
			walkStrings(val, f)
		}
	}
}
