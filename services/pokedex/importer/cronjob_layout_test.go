package importer_test

// CronJob・イメージ・Makefile・up.sh の静的検査(ADR-0104 §2・§5〜§8)。ファイルを読むだけで、
// kubectl・docker・ネットワーク・DB は使わない(make test で走る)。kustomize の描画は make k8s-render で確かめる。

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var layoutRepoRoot = filepath.Join("..", "..", "..")

const (
	cronJobPath   = "deploy/k8s/base/pokedex/cronjob-import.yaml"
	cronJobName   = "pokedex-import"
	cachePVCName  = "pokedex-import-cache"
	overridesCM   = "pokedex-name-overrides"
	dockerfile    = "services/pokedex/Dockerfile"
	cronJobScript = "tools/importer/cronjob.sh"
)

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(layoutRepoRoot, rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return string(b)
}

// --- CronJob の型(必要な項目だけ) ---------------------------------------------------

type k8sEnvVar struct {
	Name      string `yaml:"name"`
	Value     string `yaml:"value"`
	ValueFrom *struct {
		SecretKeyRef *struct {
			Name string `yaml:"name"`
			Key  string `yaml:"key"`
		} `yaml:"secretKeyRef"`
	} `yaml:"valueFrom"`
}

type k8sContainer struct {
	Name            string      `yaml:"name"`
	Image           string      `yaml:"image"`
	ImagePullPolicy string      `yaml:"imagePullPolicy"`
	Command         []string    `yaml:"command"`
	Args            []string    `yaml:"args"`
	Env             []k8sEnvVar `yaml:"env"`
	VolumeMounts    []struct {
		Name      string `yaml:"name"`
		MountPath string `yaml:"mountPath"`
		ReadOnly  bool   `yaml:"readOnly"`
	} `yaml:"volumeMounts"`
	SecurityContext *struct {
		AllowPrivilegeEscalation *bool `yaml:"allowPrivilegeEscalation"`
		ReadOnlyRootFilesystem   *bool `yaml:"readOnlyRootFilesystem"`
		RunAsNonRoot             *bool `yaml:"runAsNonRoot"`
		Capabilities             *struct {
			Drop []string `yaml:"drop"`
		} `yaml:"capabilities"`
	} `yaml:"securityContext"`
}

type k8sCronJob struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Schedule                   string `yaml:"schedule"`
		TimeZone                   string `yaml:"timeZone"`
		ConcurrencyPolicy          string `yaml:"concurrencyPolicy"`
		StartingDeadlineSeconds    *int   `yaml:"startingDeadlineSeconds"`
		SuccessfulJobsHistoryLimit *int   `yaml:"successfulJobsHistoryLimit"`
		FailedJobsHistoryLimit     *int   `yaml:"failedJobsHistoryLimit"`
		Suspend                    *bool  `yaml:"suspend"`
		JobTemplate                struct {
			Spec struct {
				BackoffLimit            *int `yaml:"backoffLimit"`
				ActiveDeadlineSeconds   *int `yaml:"activeDeadlineSeconds"`
				TTLSecondsAfterFinished *int `yaml:"ttlSecondsAfterFinished"`
				PodFailurePolicy        *struct {
					Rules []struct {
						Action      string `yaml:"action"`
						OnExitCodes *struct {
							ContainerName string `yaml:"containerName"`
							Operator      string `yaml:"operator"`
							Values        []int  `yaml:"values"`
						} `yaml:"onExitCodes"`
					} `yaml:"rules"`
				} `yaml:"podFailurePolicy"`
				Template struct {
					Spec struct {
						RestartPolicy                string         `yaml:"restartPolicy"`
						AutomountServiceAccountToken *bool          `yaml:"automountServiceAccountToken"`
						InitContainers               []k8sContainer `yaml:"initContainers"`
						Containers                   []k8sContainer `yaml:"containers"`
						SecurityContext              *struct {
							RunAsNonRoot   *bool `yaml:"runAsNonRoot"`
							SeccompProfile *struct {
								Type string `yaml:"type"`
							} `yaml:"seccompProfile"`
						} `yaml:"securityContext"`
						Volumes []struct {
							Name                  string `yaml:"name"`
							PersistentVolumeClaim *struct {
								ClaimName string `yaml:"claimName"`
							} `yaml:"persistentVolumeClaim"`
							ConfigMap *struct {
								Name     string `yaml:"name"`
								Optional *bool  `yaml:"optional"`
							} `yaml:"configMap"`
							EmptyDir *struct{} `yaml:"emptyDir"`
						} `yaml:"volumes"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate"`
	} `yaml:"spec"`
}

func loadCronJob(t *testing.T) k8sCronJob {
	t.Helper()
	var cj k8sCronJob
	if err := yaml.Unmarshal([]byte(readRepo(t, cronJobPath)), &cj); err != nil {
		t.Fatalf("%s: %v", cronJobPath, err)
	}
	if cj.Kind != "CronJob" || cj.Metadata.Name != cronJobName {
		t.Fatalf("%s は CronJob %s であること: kind=%q name=%q", cronJobPath, cronJobName, cj.Kind, cj.Metadata.Name)
	}
	return cj
}

func importContainer(t *testing.T, cj k8sCronJob) k8sContainer {
	t.Helper()
	cs := cj.Spec.JobTemplate.Spec.Template.Spec.Containers
	if len(cs) != 1 || cs[0].Name != "import" {
		t.Fatalf("コンテナは import の1つだけ(取得→照合→投入を1つの Job で行う。ADR-0104 §2): %+v", cs)
	}
	return cs[0]
}

func intIn(p *int, lo, hi int) bool { return p != nil && *p >= lo && *p <= hi }

// --- AC1: スケジュールと Job の設定 --------------------------------------------------

func TestImportCronJobSchedule(t *testing.T) {
	cj := loadCronJob(t)
	s := cj.Spec
	fields := strings.Fields(s.Schedule)
	number := regexp.MustCompile(`^\d+$`)
	if len(fields) != 5 || !number.MatchString(fields[0]) || !number.MatchString(fields[1]) ||
		fields[2] != "*" || fields[3] != "*" || !regexp.MustCompile(`^[0-6]$`).MatchString(fields[4]) {
		t.Errorf("schedule は週1回(「分 時 * * 曜日」で分・時・曜日は単一の値): %q", s.Schedule)
	}
	if s.TimeZone != "Asia/Tokyo" {
		t.Errorf("timeZone = %q, want Asia/Tokyo", s.TimeZone)
	}
	if s.ConcurrencyPolicy != "Forbid" {
		t.Errorf("concurrencyPolicy = %q, want Forbid", s.ConcurrencyPolicy)
	}
	if !intIn(s.StartingDeadlineSeconds, 1, 7*24*3600-1) {
		t.Errorf("startingDeadlineSeconds は 0 より大きく7日未満(次の予定と重ねない): %v", s.StartingDeadlineSeconds)
	}
	if !intIn(s.SuccessfulJobsHistoryLimit, 1, 5) || !intIn(s.FailedJobsHistoryLimit, 1, 5) {
		t.Errorf("履歴の保持数は 1〜5: successful=%v failed=%v", s.SuccessfulJobsHistoryLimit, s.FailedJobsHistoryLimit)
	}
	if s.Suspend != nil && *s.Suspend {
		t.Error("base の CronJob を suspend しない(止めるのは cloud overlay だけ)")
	}
}

func TestImportCronJobJobSpec(t *testing.T) {
	cj := loadCronJob(t)
	js := cj.Spec.JobTemplate.Spec
	if !intIn(js.BackoffLimit, 0, 3) {
		t.Errorf("backoffLimit は 0〜3: %v", js.BackoffLimit)
	}
	if !intIn(js.ActiveDeadlineSeconds, 1, 6*3600) {
		t.Errorf("activeDeadlineSeconds は 1〜6時間: %v", js.ActiveDeadlineSeconds)
	}
	if js.TTLSecondsAfterFinished == nil || *js.TTLSecondsAfterFinished < 7*24*3600 {
		t.Errorf("ttlSecondsAfterFinished は7日以上(週1回の結果を次の実行まで残す): %v", js.TTLSecondsAfterFinished)
	}
	if got := js.Template.Spec.RestartPolicy; got != "Never" {
		t.Errorf("restartPolicy = %q, want Never(podFailurePolicy の前提)", got)
	}
	failJob := map[int]bool{}
	if js.PodFailurePolicy != nil {
		for _, r := range js.PodFailurePolicy.Rules {
			if r.Action == "FailJob" && r.OnExitCodes != nil && r.OnExitCodes.Operator == "In" &&
				(r.OnExitCodes.ContainerName == "" || r.OnExitCodes.ContainerName == "import") {
				for _, v := range r.OnExitCodes.Values {
					failJob[v] = true
				}
			}
		}
	}
	if !failJob[2] || !failJob[3] {
		t.Errorf("podFailurePolicy で終了コード 2・3(再試行しても同じ)を FailJob にすること: %v", failJob)
	}
	if failJob[1] {
		t.Error("終了コード 1(再試行で直りうる)を FailJob にしない")
	}
}

// --- AC2: DB の接続・セキュリティ・ボリュームの配線 --------------------------------------

func TestImportCronJobPodSecurityAndWiring(t *testing.T) {
	cj := loadCronJob(t)
	pod := cj.Spec.JobTemplate.Spec.Template.Spec
	c := importContainer(t, cj)

	if !regexp.MustCompile(`^pokecalc/pokedex-importer:\d+\.\d+\.\d+$`).MatchString(c.Image) {
		t.Errorf("image は pokecalc/pokedex-importer:<semver>(latest にしない): %q", c.Image)
	}
	if c.ImagePullPolicy != "IfNotPresent" {
		t.Errorf("imagePullPolicy = %q, want IfNotPresent(k3d に import したイメージを使う)", c.ImagePullPolicy)
	}
	for _, a := range append(append([]string(nil), c.Command...), c.Args...) {
		if a == "-force" || a == "--force" || a == "-dry-run" || a == "--dry-run" {
			t.Errorf("CronJob に %s を渡さない(版に変化が無ければ取り込まない)", a)
		}
	}

	// importer は DML だけの pokedex_importer の DSN を使う(ADR-0110 §6。root の pokedex-dsn は使わない)。
	var dsnFromSecret bool
	for _, e := range c.Env {
		if e.Name == "POKEDEX_DATABASE_DSN" {
			if e.Value != "" || e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil ||
				e.ValueFrom.SecretKeyRef.Name != "mysql-auth" || e.ValueFrom.SecretKeyRef.Key != "pokedex-importer-dsn" {
				t.Errorf("POKEDEX_DATABASE_DSN は Secret mysql-auth の pokedex-importer-dsn から渡す: %+v", e)
			} else {
				dsnFromSecret = true
			}
		}
		if regexp.MustCompile(`(?i)dsn|password|secret|token`).MatchString(e.Name) && e.Value != "" {
			t.Errorf("env %s に値を直接書かない", e.Name)
		}
	}
	if !dsnFromSecret {
		t.Error("POKEDEX_DATABASE_DSN が無い")
	}
	// root・他用途の資格情報をどのコンテナ(initContainer を含む)にも渡さない(ADR-0110 §2・§6)。
	for _, cc := range append(append([]k8sContainer(nil), pod.InitContainers...), pod.Containers...) {
		for _, e := range cc.Env {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				continue
			}
			switch e.ValueFrom.SecretKeyRef.Key {
			case "pokedex-dsn", "mysql-root-password", "pokedex-reader-dsn", "pokedex-migrator-dsn":
				t.Errorf("importer の %s/%s が %s を参照している(importer は pokedex-importer-dsn だけ)",
					cc.Name, e.Name, e.ValueFrom.SecretKeyRef.Key)
			}
		}
	}

	sc := c.SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation ||
		sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem ||
		sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot ||
		sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("コンテナは非 root・読み取り専用ルート・権限昇格なし・capabilities を全部落とす: %+v", sc)
	}
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Error("automountServiceAccountToken: false にする(k8s API を使わない)")
	}
	if pod.SecurityContext == nil || pod.SecurityContext.RunAsNonRoot == nil || !*pod.SecurityContext.RunAsNonRoot ||
		pod.SecurityContext.SeccompProfile == nil || pod.SecurityContext.SeccompProfile.Type != "RuntimeDefault" {
		t.Errorf("Pod は runAsNonRoot・seccomp RuntimeDefault: %+v", pod.SecurityContext)
	}

	mounts := map[string]string{} // mountPath → volume name
	readOnly := map[string]bool{}
	for _, m := range c.VolumeMounts {
		mounts[m.MountPath] = m.Name
		readOnly[m.MountPath] = m.ReadOnly
	}
	vol := func(path string) (name string, found bool) {
		name, found = mounts[path]
		return
	}
	var pvcOK, cmOK, tmpOK bool
	for _, v := range pod.Volumes {
		if name, ok := vol("/app/data/generated"); ok && v.Name == name && v.PersistentVolumeClaim != nil &&
			v.PersistentVolumeClaim.ClaimName == cachePVCName {
			pvcOK = true
		}
		if name, ok := vol("/app/data/local"); ok && v.Name == name && v.ConfigMap != nil &&
			v.ConfigMap.Name == overridesCM && v.ConfigMap.Optional != nil && *v.ConfigMap.Optional {
			cmOK = readOnly["/app/data/local"]
			if !cmOK {
				t.Error("/app/data/local(override)は読み取り専用でマウントする")
			}
		}
		if name, ok := vol("/tmp"); ok && v.Name == name && v.EmptyDir != nil {
			tmpOK = true
		}
	}
	if !pvcOK {
		t.Errorf("/app/data/generated に PVC %s をマウントする(取得のキャッシュと報告を残す)", cachePVCName)
	}
	if !cmOK {
		t.Errorf("/app/data/local に ConfigMap %s を optional: true でマウントする(無ければ override なし)", overridesCM)
	}
	if !tmpOK {
		t.Error("/tmp に emptyDir をマウントする(読み取り専用ルートで npm の HOME・キャッシュを置く)")
	}
}

func TestImportCronJobInBaseAndSuspendedInCloud(t *testing.T) {
	kust := readRepo(t, "deploy/k8s/base/pokedex/kustomization.yaml")
	for _, want := range []string{"cronjob-import.yaml", "job-migrate.yaml"} {
		if !strings.Contains(kust, want) {
			t.Errorf("base/pokedex/kustomization.yaml の resources に %s が無い", want)
		}
	}

	// PVC は base/pokedex のどこかのファイルに(kustomization から参照される形で)ある。
	var pvcFile string
	entries, err := os.ReadDir(filepath.Join(layoutRepoRoot, "deploy/k8s/base/pokedex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s := readRepo(t, filepath.Join("deploy/k8s/base/pokedex", e.Name()))
		if regexp.MustCompile(`(?m)^kind:\s*PersistentVolumeClaim\s*$`).MatchString(s) &&
			regexp.MustCompile(`(?m)^\s+name:\s*`+cachePVCName+`\s*$`).MatchString(s) {
			pvcFile = e.Name()
			if !strings.Contains(s, "ReadWriteOnce") {
				t.Errorf("%s: PVC %s は ReadWriteOnce", e.Name(), cachePVCName)
			}
		}
	}
	if pvcFile == "" {
		t.Fatalf("deploy/k8s/base/pokedex に PVC %s が無い", cachePVCName)
	}
	if !strings.Contains(kust, pvcFile) {
		t.Errorf("base/pokedex/kustomization.yaml の resources に %s が無い", pvcFile)
	}

	// ConfigMap pokedex-name-overrides は Git に置かない(override は実データ。up.sh が作る)。
	walkRepoFiles(t, "deploy", func(rel, s string) {
		for _, doc := range strings.Split(s, "\n---") {
			if regexp.MustCompile(`(?m)^kind:\s*ConfigMap\s*$`).MatchString(doc) && strings.Contains(doc, overridesCM) {
				t.Errorf("%s: ConfigMap %s を manifest に書かない(Git 管理外の override。up.sh が作る)", rel, overridesCM)
			}
		}
	})

	// cloud overlay は CronJob を suspend する(MySQL・Secret・イメージの配布経路が無いため。ADR-0104 §8)。
	var cloud strings.Builder
	walkRepoFiles(t, "deploy/k8s/overlays/cloud", func(_, s string) { cloud.WriteString(s + "\n") })
	c := cloud.String()
	if !strings.Contains(c, cronJobName) || !regexp.MustCompile(`suspend:\s*true`).MatchString(c) {
		t.Error("cloud overlay に CronJob pokedex-import を suspend: true にするパッチが無い")
	}
	// local overlay は suspend しない。
	var local strings.Builder
	walkRepoFiles(t, "deploy/k8s/overlays/local", func(_, s string) { local.WriteString(s + "\n") })
	if regexp.MustCompile(`suspend:\s*true`).MatchString(local.String()) {
		t.Error("local overlay で CronJob を suspend しない")
	}
}

func walkRepoFiles(t *testing.T, dir string, fn func(rel, content string)) {
	t.Helper()
	root := filepath.Join(layoutRepoRoot, dir)
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(layoutRepoRoot, p)
		fn(rel, string(b))
		return nil
	})
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
}

// --- AC3: イメージ・.dockerignore・up.sh -----------------------------------------------

// dockerStages は Dockerfile を FROM ごとに分ける(stage 名 → 本文)。順序も返す。
func dockerStages(src string) (names []string, from map[string]string, body map[string]string) {
	from, body = map[string]string{}, map[string]string{}
	re := regexp.MustCompile(`(?i)^FROM\s+(\S+)(?:\s+AS\s+(\S+))?\s*$`)
	var cur string
	for i, line := range strings.Split(src, "\n") {
		if m := re.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			cur = m[2]
			if cur == "" {
				cur = "#" + strconv.Itoa(i)
			}
			names = append(names, cur)
			from[cur] = m[1]
			continue
		}
		if cur != "" {
			body[cur] += line + "\n"
		}
	}
	return names, from, body
}

func TestImporterImageDockerfile(t *testing.T) {
	src := readRepo(t, dockerfile)
	names, from, body := dockerStages(src)
	stageSet := map[string]bool{}
	for _, n := range names {
		stageSet[n] = true
	}
	pinned := regexp.MustCompile(`^[a-z0-9./-]+:\d+\.\d+\.\d+(-[a-z0-9.]+)?@sha256:[0-9a-f]{64}$`)
	var hasNode bool
	for _, n := range names {
		img := from[n]
		if img == "scratch" || stageSet[img] {
			continue
		}
		if !pinned.MatchString(img) {
			t.Errorf("FROM %s: 外部のベースイメージは完全な版のタグ+digest で固定する(ADR-0102)", img)
		}
		if strings.HasPrefix(img, "node:") {
			hasNode = true
		}
	}
	if !hasNode {
		t.Error("Node のベースイメージが無い(取得スクリプトは Node。ADR-0101 63行)")
	}
	imp, ok := body["importer"]
	if !ok {
		t.Fatalf("%s に importer ターゲット(FROM ... AS importer)が無い", dockerfile)
	}
	if !regexp.MustCompile(`go build[^\n]*\./pokedex/cmd/import\b`).MatchString(src) {
		t.Error("build ステージで ./pokedex/cmd/import を build していない")
	}
	checks := []struct{ re, msg string }{
		{`COPY\s+--from=build\s+\S*pokedex-import\S*\s`, "Go の pokedex-import を build ステージからコピーする"},
		{`COPY\s[^\n]*data/importer`, "data/importer(config・effects・regulations)をイメージに焼く"},
		{`COPY\s[^\n]*tools/importer`, "tools/importer の取得スクリプトを含める"},
		{`(?m)^USER\s+(node|1000)(:\S+)?\s*$`, "非 root(node / 1000)で動かす"},
		{`cronjob\.sh`, "ENTRYPOINT などで tools/importer/cronjob.sh を起動する"},
	}
	for _, c := range checks {
		if !regexp.MustCompile(c.re).MatchString(imp) {
			t.Errorf("importer ステージ: %s(/%s/ が無い)", c.msg, c.re)
		}
	}
	if !regexp.MustCompile(`npm\s+ci`).MatchString(src) {
		t.Error("tools/importer の依存を npm ci で入れる(package-lock.json の固定版)")
	}
	for _, line := range strings.Split(src, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(l), "COPY") || strings.HasPrefix(strings.ToUpper(l), "ADD") {
			if strings.Contains(l, "data/generated") || strings.Contains(l, "data/local") || regexp.MustCompile(`\sdata/?\s`).MatchString(l+" ") {
				t.Errorf("実データ(data/generated・data/local・data 全体)をイメージに入れない: %q", l)
			}
		}
	}
	if _, ok := body["migrate"]; !ok {
		t.Error("既存の migrate ターゲットを消さない")
	}
}

func TestDockerignoreExcludesRealData(t *testing.T) {
	s := readRepo(t, ".dockerignore")
	lines := map[string]bool{}
	for _, l := range strings.Split(s, "\n") {
		lines[strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(l), "/"), "/")] = true
	}
	for _, want := range [][]string{{"data/generated"}, {"data/local"}, {"**/node_modules", "node_modules"}, {".env"}} {
		ok := false
		for _, w := range want {
			ok = ok || lines[w]
		}
		if !ok {
			t.Errorf(".dockerignore に %v が無い(第三者データ・手元の依存・秘密をビルドコンテキストに入れない)", want)
		}
	}
}

func TestUpScriptBuildsImporterAndOverrides(t *testing.T) {
	s := readRepo(t, "scripts/up.sh")
	if !regexp.MustCompile(`docker build[^\n]*--target importer`).MatchString(s) {
		t.Error("scripts/up.sh が importer イメージを build していない(--target importer)")
	}
	m := regexp.MustCompile(`POKEDEX_IMPORTER_IMAGE="\$\{POKEDEX_IMPORTER_IMAGE:-([^}]+)\}"`).FindStringSubmatch(s)
	if m == nil {
		t.Fatal(`scripts/up.sh に POKEDEX_IMPORTER_IMAGE="${POKEDEX_IMPORTER_IMAGE:-<image>}" が無い`)
	}
	if got := importContainer(t, loadCronJob(t)).Image; got != m[1] {
		t.Errorf("CronJob の image %q と up.sh の既定 %q が違う", got, m[1])
	}
	if !regexp.MustCompile(`k3d image import[^\n]*POKEDEX_IMPORTER_IMAGE`).MatchString(s) {
		t.Error("scripts/up.sh が importer イメージを k3d image import していない")
	}
	for _, want := range []string{overridesCM, "data/local/name_ja_overrides.json", "create configmap"} {
		if !strings.Contains(s, want) {
			t.Errorf("scripts/up.sh が override の ConfigMap を作っていない(%q が無い)", want)
		}
	}
	if !regexp.MustCompile(`\[\s+-f\s+"?[^\]]*name_ja_overrides\.json`).MatchString(s) {
		t.Error("override の ConfigMap はファイルがあるときだけ作る([ -f ... ])")
	}
	if regexp.MustCompile(`create\s+job\s+[^\n]*--from=cronjob`).MatchString(s) {
		t.Error("up.sh は CronJob を即時に流さない(初回の取得はネットワークが要る。人が make import-k8s で流す)")
	}
	for _, f := range []string{"scripts/up.sh", "Makefile"} {
		if regexp.MustCompile(`delete\s+(pvc|persistentvolumeclaim)`).MatchString(readRepo(t, f)) {
			t.Errorf("%s: PVC を消さない(データの削除は人間の確認)", f)
		}
	}
}

// --- AC4: CronJob の手順 -------------------------------------------------------------

func TestCronJobScriptOrder(t *testing.T) {
	s := readRepo(t, cronJobScript)
	if !regexp.MustCompile(`(?m)^set -eu`).MatchString(s) {
		t.Errorf("%s: set -eu で途中の失敗を拾う", cronJobScript)
	}
	idx := func(re string) int {
		loc := regexp.MustCompile(re).FindStringIndex(s)
		if loc == nil {
			return -1
		}
		return loc[0]
	}
	fetch := idx(`node\s+\S*fetch\.mjs`)
	upstream := idx(`node\s+\S*check-upstream\.mjs`)
	imp := idx(`pokedex-import\b[^\n]*-data`)
	if fetch < 0 || upstream < 0 || imp < 0 {
		t.Fatalf("%s: fetch.mjs(%d)・check-upstream.mjs(%d)・pokedex-import -data(%d)のどれかが無い", cronJobScript, fetch, upstream, imp)
	}
	if !(fetch < upstream && upstream < imp) {
		t.Errorf("%s: 順序は 取得 → 上流の検出 → 照合・投入", cronJobScript)
	}
	upLine := s[upstream:]
	if i := strings.IndexByte(upLine, '\n'); i >= 0 {
		upLine = upLine[:i]
	}
	if !strings.Contains(upLine, "||") {
		t.Errorf("%s: 上流の検出の失敗で止めない(|| で警告にする): %q", cronJobScript, upLine)
	}
	if !regexp.MustCompile(`pokedex-import\b[^\n]*-upstream`).MatchString(s) {
		t.Errorf("%s: pokedex-import に -upstream を渡す", cronJobScript)
	}
	for _, bad := range []string{"-force", "-dry-run", "migrate"} {
		if strings.Contains(s, bad) {
			t.Errorf("%s: %s を含めない", cronJobScript, bad)
		}
	}
}

// --- AC(issue #106 / ADR-0109): cronjob.sh の相互排他 ---------------------------------

// TestCronJobScriptLocksBeforeFetch は cronjob.sh が flock で非ブロッキングに排他し、
// ロックが取れなければ fetch.mjs(取得キャッシュへの書き込み)より前に諦めて終わることを固定する。
// 実行時の動作(2プロセス同時起動で片方だけが処理を進めること)は cronjob_lock_test.go の統合テストが見る。
// 現時点(spec-writer)では cronjob.sh はロックを持たないため、このテストは失敗する。
func TestCronJobScriptLocksBeforeFetch(t *testing.T) {
	s := readRepo(t, cronJobScript)

	if !regexp.MustCompile(`\bflock\b`).MatchString(s) {
		t.Fatalf("%s: flock で排他していない(issue #106 / ADR-0109)", cronJobScript)
	}
	if !regexp.MustCompile(`flock\s+-n\b`).MatchString(s) {
		t.Errorf("%s: flock は非ブロッキング(-n)で使うこと(他のプロセスの処理が終わるまで待たず、諦めて exit 1)", cronJobScript)
	}

	idx := func(re string) int {
		loc := regexp.MustCompile(re).FindStringIndex(s)
		if loc == nil {
			return -1
		}
		return loc[0]
	}
	lock := idx(`flock\s+-n`)
	fetch := idx(`node\s+\S*fetch\.mjs`)
	if lock < 0 {
		t.Fatalf("%s: flock -n が見つからない", cronJobScript)
	}
	if fetch < 0 {
		t.Fatalf("%s: fetch.mjs の呼び出しが見つからない", cronJobScript)
	}
	if !(lock < fetch) {
		t.Errorf("%s: ロック取得は fetch.mjs(取得キャッシュへの書き込み)より前であること", cronJobScript)
	}

	if !strings.Contains(s, "IMPORT_LOCK_FILE") {
		t.Errorf("%s: ロックファイルの場所を環境変数 IMPORT_LOCK_FILE で上書きできること(テストから差し替えるため。ADR-0109 §2)", cronJobScript)
	}
	if !strings.Contains(s, "IMPORT_APP_DIR") {
		t.Errorf("%s: /app のパスを環境変数 IMPORT_APP_DIR(既定 /app)で上書きできること(テストから差し替えるため。ADR-0109 §2)", cronJobScript)
	}
	if !regexp.MustCompile(`exit\s+1\b`).MatchString(s) {
		t.Errorf("%s: ロック取得の失敗は終了コード1(再試行で直りうる失敗。ADR-0104 §3・ADR-0109 §4)", cronJobScript)
	}
	// stale lock 対策: mkdir でロックを表現する方式(ADR-0109 で却下)を使っていないことを固定する。
	if regexp.MustCompile(`mkdir\s+[^\n]*\.lock`).MatchString(s) {
		t.Errorf("%s: mkdir でロックを表現しない(プロセスが SIGKILL されると stale lock が残る。flock を使う。ADR-0109 §3)", cronJobScript)
	}
}

// --- AC8: Makefile -------------------------------------------------------------------

// layoutMakeTargets は Makefile の「ターゲット: 依存」行とレシピを集める(include は見ない)。
func layoutMakeTargets(src string) map[string]string {
	targets := map[string]string{}
	var current string
	head := regexp.MustCompile(`^([a-zA-Z0-9_-]+):([^=].*)?$`)
	for _, line := range strings.Split(src, "\n") {
		if m := head.FindStringSubmatch(line); m != nil {
			current = m[1]
			targets[current] += m[2] + "\n"
			continue
		}
		if strings.HasPrefix(line, "\t") && current != "" {
			targets[current] += line + "\n"
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, ".PHONY") || strings.HasPrefix(line, "#") {
			continue
		}
		current = ""
	}
	return targets
}

func TestMakefileImportTargets(t *testing.T) {
	targets := layoutMakeTargets(readRepo(t, "Makefile"))
	for _, name := range []string{"import", "import-dry-run", "import-fetch", "import-k8s", "import-check-upstream", "k8s-render"} {
		if _, ok := targets[name]; !ok {
			t.Errorf("Makefile にターゲット %s が無い", name)
		}
	}
	k := targets["import-k8s"]
	if !regexp.MustCompile(`create\s+job\s+[^\n]*--from=cronjob/` + cronJobName).MatchString(k) {
		t.Errorf("import-k8s は kubectl create job --from=cronjob/%s で CronJob を1回流す: %q", cronJobName, k)
	}
	if !strings.Contains(k, "pokecalc") {
		t.Errorf("import-k8s は namespace pokecalc に作る: %q", k)
	}
	if !strings.Contains(k, "current-context") || !strings.Contains(k, "k3d-$(CLUSTER)") {
		t.Errorf("import-k8s は kubectl の context が k3d-$(CLUSTER) であることを確かめる(別クラスタで流さない): %q", k)
	}
	if !strings.Contains(targets["import-check-upstream"], "check-upstream.mjs") {
		t.Errorf("import-check-upstream は tools/importer/check-upstream.mjs を実行する: %q", targets["import-check-upstream"])
	}
	r := targets["k8s-render"]
	if !strings.Contains(r, "kustomize") || !strings.Contains(r, "deploy/k8s/overlays/local") || !strings.Contains(r, "deploy/k8s/overlays/cloud") {
		t.Errorf("k8s-render は local と cloud の overlay を kustomize で描画する: %q", r)
	}
	lint := targets["lint"]
	if !strings.Contains(lint, "k8s-render") {
		t.Errorf("lint から k8s-render を呼ぶ: %q", lint)
	}
	if !regexp.MustCompile(`sh -n[^\n]*tools/importer/|tools/importer/\*\.sh`).MatchString(lint) {
		t.Errorf("lint で tools/importer/*.sh の構文を検査する: %q", lint)
	}
	for _, name := range []string{"test", "test-services"} {
		for _, bad := range []string{"import", "kubectl", "npm", "test-db"} {
			if strings.Contains(targets[name], bad) {
				t.Errorf("%s にネットワーク・DB・クラスタが要るもの(%s)を混ぜない: %q", name, bad, targets[name])
			}
		}
	}
}

func TestCheckUpstreamScriptExists(t *testing.T) {
	s := readRepo(t, "tools/importer/check-upstream.mjs")
	if !strings.Contains(s, "upstream/latest.json") {
		t.Error("check-upstream.mjs は data/generated/upstream/latest.json に結果を書く")
	}
	if !strings.Contains(s, "config.json") {
		t.Error("check-upstream.mjs は比較対象の source を data/importer/config.json から読む")
	}
	if regexp.MustCompile(`writeFileSync\([^)]*config\.json`).MatchString(s) {
		t.Error("check-upstream.mjs は config.json を書き換えない(版を上げるのは人の PR)")
	}
}
