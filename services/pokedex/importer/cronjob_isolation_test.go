package importer_test

// issue #301(D19)・ADR-0101 追記: 第三者のコード(Showdown の npm 依存と build)を実行する「取得段」と、
// DB の資格情報を持つ「投入段」を、同じ Pod の別コンテナに分ける。取得段は initContainer `fetch`
// (DSN・Secret を一切持たない)、投入段は既存の `import`。成果物は PVC(/app/data/generated)で渡す。
// ファイルを読むだけの静的検査(kubectl・docker・ネットワークは使わない)。

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const fetchContainerName = "fetch"

func fetchInitContainer(t *testing.T) k8sContainer {
	t.Helper()
	ics := loadCronJob(t).Spec.JobTemplate.Spec.Template.Spec.InitContainers
	if len(ics) != 1 || ics[0].Name != fetchContainerName {
		t.Fatalf("initContainers は %s の1つだけ(取得・build 段。第三者のコードを実行する): %+v", fetchContainerName, ics)
	}
	return ics[0]
}

// rawInitContainers は initContainers を未知のキーも含めて汎用の形で読む(envFrom・volumes の Secret など、
// 型に無いキーからの資格情報の混入も見逃さないため)。
func rawInitContainers(t *testing.T) []map[string]any {
	t.Helper()
	var doc struct {
		Spec struct {
			JobTemplate struct {
				Spec struct {
					Template struct {
						Spec struct {
							InitContainers []map[string]any `yaml:"initContainers"`
						} `yaml:"spec"`
					} `yaml:"template"`
				} `yaml:"spec"`
			} `yaml:"jobTemplate"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(readRepo(t, cronJobPath)), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Spec.JobTemplate.Spec.Template.Spec.InitContainers
}

// 取得段は DB の資格情報にも Secret にも触れない(#301 の受け入れ条件。fetch を行うコンテナに DSN が無い)。
func TestFetchInitContainerHasNoCredentials(t *testing.T) {
	_ = fetchInitContainer(t)
	raw := rawInitContainers(t)
	out, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bad := regexp.MustCompile(`(?i)dsn|secretKeyRef|secretRef|secret|mysql-auth|password|token|envFrom`).FindString(string(out)); bad != "" {
		t.Errorf("取得段(initContainer)に資格情報・Secret・envFrom に関わる記述がある(%q)。DB の DSN は投入段 import だけが持つ:\n%s", bad, out)
	}

	// 投入段の DSN は引き続き Secret mysql-auth から(既存の TestImportCronJobPodSecurityAndWiring も見る)。
	imp := importContainer(t, loadCronJob(t))
	var hasDSN bool
	for _, e := range imp.Env {
		if e.Name == "POKEDEX_DATABASE_DSN" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil &&
			e.ValueFrom.SecretKeyRef.Name == "mysql-auth" {
			hasDSN = true
		}
	}
	if !hasDSN {
		t.Error("投入段 import が POKEDEX_DATABASE_DSN(Secret mysql-auth)を持たない")
	}

	// Pod の volumes に Secret を足さない(足すと initContainer にもマウントできてしまう)。
	for _, v := range rawPodVolumes(t) {
		if _, ok := v["secret"]; ok {
			t.Errorf("CronJob の volumes に Secret を足さない(資格情報は import の env だけ): %v", v["name"])
		}
		if _, ok := v["projected"]; ok {
			t.Errorf("CronJob の volumes に projected(Secret を含みうる)を足さない: %v", v["name"])
		}
	}
}

// rawPodVolumes は Pod の volumes を未知のキーも含めて汎用の形で読む(YAML の構造で Secret の有無を見る)。
func rawPodVolumes(t *testing.T) []map[string]any {
	t.Helper()
	var doc struct {
		Spec struct {
			JobTemplate struct {
				Spec struct {
					Template struct {
						Spec struct {
							Volumes []map[string]any `yaml:"volumes"`
						} `yaml:"spec"`
					} `yaml:"template"`
				} `yaml:"spec"`
			} `yaml:"jobTemplate"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(readRepo(t, cronJobPath)), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Spec.JobTemplate.Spec.Template.Spec.Volumes) == 0 {
		t.Fatal("volumes を読めない")
	}
	return doc.Spec.JobTemplate.Spec.Template.Spec.Volumes
}

// 取得段は PVC と tmp だけをマウントする。名前の上書き ConfigMap(/app/data/local。実データ)は投入段だけが読む。
func TestFetchInitContainerMountsAndHardening(t *testing.T) {
	cj := loadCronJob(t)
	pod := cj.Spec.JobTemplate.Spec.Template.Spec
	f := fetchInitContainer(t)
	imp := importContainer(t, cj)

	if f.Image != imp.Image {
		t.Errorf("fetch と import は同じイメージ(同じ版・同じ up.sh の既定): fetch=%q import=%q", f.Image, imp.Image)
	}
	if f.ImagePullPolicy != "IfNotPresent" {
		t.Errorf("fetch の imagePullPolicy = %q, want IfNotPresent", f.ImagePullPolicy)
	}
	// cronjob.sh のフェーズ指定(ENTRYPOINT が cronjob.sh なので args だけで渡す)。
	if len(f.Command) != 0 || len(f.Args) != 1 || f.Args[0] != "fetch" {
		t.Errorf("fetch は ENTRYPOINT(cronjob.sh)に args: [fetch] を渡す(command は書かない): command=%v args=%v", f.Command, f.Args)
	}
	if len(imp.Command) != 0 || len(imp.Args) != 1 || imp.Args[0] != "import" {
		t.Errorf("import は ENTRYPOINT(cronjob.sh)に args: [import] を渡す(command は書かない): command=%v args=%v", imp.Command, imp.Args)
	}

	// npm・Node が読み取り専用ルートで動く env は取得段にも要る(import と同じ)。
	env := map[string]string{}
	for _, e := range f.Env {
		env[e.Name] = e.Value
	}
	if env["HOME"] != "/tmp" || env["npm_config_cache"] != "/tmp/npm-cache" {
		t.Errorf("fetch の env に HOME=/tmp・npm_config_cache=/tmp/npm-cache が要る: %v", env)
	}
	if v, ok := env["IMPORT_ALLOW_REMOVED"]; ok {
		t.Errorf("IMPORT_ALLOW_REMOVED は投入段(import)にだけ付く(取得段では意味を持たない): %q", v)
	}

	// マウント: PVC(同じ claim・同じパス)と tmp(emptyDir)だけ。
	volByName := map[string]int{}
	for i, v := range pod.Volumes {
		volByName[v.Name] = i
	}
	var pvcMounted, tmpMounted bool
	for _, m := range f.VolumeMounts {
		i, ok := volByName[m.Name]
		if !ok {
			t.Errorf("fetch が未定義の volume %q をマウントしている", m.Name)
			continue
		}
		v := pod.Volumes[i]
		switch {
		case v.PersistentVolumeClaim != nil:
			if v.PersistentVolumeClaim.ClaimName != cachePVCName || m.MountPath != "/app/data/generated" || m.ReadOnly {
				t.Errorf("fetch の PVC は %s を /app/data/generated に読み書きでマウントする: %+v", cachePVCName, m)
			}
			pvcMounted = true
		case v.EmptyDir != nil:
			if m.MountPath != "/tmp" {
				t.Errorf("fetch の emptyDir は /tmp: %+v", m)
			}
			tmpMounted = true
		default:
			t.Errorf("fetch がマウントしてよいのは PVC と /tmp の emptyDir だけ(ConfigMap・Secret は不可): %+v", m)
		}
	}
	if !pvcMounted || !tmpMounted {
		t.Errorf("fetch は PVC(/app/data/generated)と emptyDir(/tmp)をマウントする: pvc=%v tmp=%v", pvcMounted, tmpMounted)
	}
	// ロックと引き渡しのファイルは PVC 上にある。import も同じ PVC・同じパスを使う。
	var impPVC bool
	for _, m := range imp.VolumeMounts {
		if m.MountPath == "/app/data/generated" && pod.Volumes[volByName[m.Name]].PersistentVolumeClaim != nil {
			impPVC = true
		}
	}
	if !impPVC {
		t.Error("import も同じ PVC を /app/data/generated にマウントする(取得物とロック・引き渡しファイルの受け渡し)")
	}
	for _, c := range []k8sContainer{f, imp} {
		for _, e := range c.Env {
			if (e.Name == "IMPORT_LOCK_FILE" || e.Name == "IMPORT_HANDOFF_FILE") && !strings.HasPrefix(e.Value, "/app/data/generated/") {
				t.Errorf("%s: %s は PVC(/app/data/generated)の下に置く(2つのコンテナが同じファイルを見る): %q", c.Name, e.Name, e.Value)
			}
		}
	}

	// 第三者のコードを走らせる段こそ、投入段と同じ(以上の)強化をかける。
	sc := f.SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation ||
		sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem ||
		sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot ||
		sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("fetch は非 root・読み取り専用ルート・権限昇格なし・capabilities を全部落とす: %+v", sc)
	}
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Error("automountServiceAccountToken: false のまま(取得段が k8s API の資格情報を持たない)")
	}
	if pod.RestartPolicy != "Never" {
		t.Errorf("restartPolicy = %q, want Never", pod.RestartPolicy)
	}
}

// 取得段の終了コード 3(ハッシュの不一致・容量不足)・2 は再試行しても同じなので Job ごと失敗させる。
// 1(ネットワークの一時失敗・ロック競合)は backoffLimit の再試行に任せる。
func TestPodFailurePolicyCoversFetchContainer(t *testing.T) {
	cj := loadCronJob(t)
	pfp := cj.Spec.JobTemplate.Spec.PodFailurePolicy
	if pfp == nil {
		t.Fatal("podFailurePolicy が無い")
	}
	byContainer := map[string]map[int]bool{}
	for _, r := range pfp.Rules {
		if r.Action != "FailJob" || r.OnExitCodes == nil || r.OnExitCodes.Operator != "In" {
			continue
		}
		name := r.OnExitCodes.ContainerName
		if byContainer[name] == nil {
			byContainer[name] = map[int]bool{}
		}
		for _, v := range r.OnExitCodes.Values {
			byContainer[name][v] = true
		}
	}
	for _, name := range []string{fetchContainerName, "import"} {
		codes := byContainer[name]
		if !codes[2] || !codes[3] || codes[1] {
			t.Errorf("podFailurePolicy: コンテナ %s の終了コード 2・3 を FailJob にし、1 は FailJob にしない(rules=%v)", name, byContainer)
		}
	}
}

// Dockerfile: イメージ内で実行する npm ci も install スクリプトを走らせない(#222)。
func TestDockerfileNpmCiIgnoresScripts(t *testing.T) {
	src := readRepo(t, dockerfile)
	lines := regexp.MustCompile(`(?m)^\s*RUN\s[^\n]*\bnpm\s+ci\b[^\n]*$`).FindAllString(src, -1)
	if len(lines) == 0 {
		t.Fatal("Dockerfile に npm ci が無い")
	}
	for _, l := range lines {
		if !strings.Contains(l, "--ignore-scripts") {
			t.Errorf("Dockerfile の npm ci に --ignore-scripts が無い: %q", strings.TrimSpace(l))
		}
	}
}

// 取得段は cronjob.sh の fetch フェーズだけを呼ぶ。DB の資格情報で動く pokedex-import は投入フェーズだけ。
func TestCronJobScriptPhases(t *testing.T) {
	s := readRepo(t, cronJobScript)
	for _, want := range []string{"fetch)", "import)", "IMPORT_HANDOFF_FILE", "IMPORT_HANDOFF_TTL_SECONDS"} {
		if !strings.Contains(s, want) {
			t.Errorf("%s: %q が無い(フェーズ fetch|import と引き渡しファイル。ADR-0101 追記)", cronJobScript, want)
		}
	}
}
