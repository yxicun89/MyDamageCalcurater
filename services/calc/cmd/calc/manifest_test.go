package main

// calc-svc の k8s マニフェストの静的検査(ADR-0203 §3。AC-S3・AC-S4)。kubectl を使わず YAML を読む。
// 例のマスタと相性表は local overlay 専用の Component(deploy/k8s/overlays/local/api)が configMapGenerator で
// 読ませる。Kustomize の load restrictor のため Component の下にコピーを置くので、元ファイルとのバイト一致を固定する
// (services/balance の前例と同じ)。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

const (
	calcService   = "calc"
	calcImageRepo = "pokecalc/calc"
)

// 元ファイル(単一の正。リポジトリ直下からの相対)。
var calcLocalDataSources = map[string]string{
	envMasterPath:    "services/calc/testdata/master.example.json", // 架空データ(ADR-0002)
	envTypeChartPath: "testdata/golden/typechart.json",             // 数値と英語 ID のみ(ADR-0015)
}

// AC-S3: base の Deployment・Service が ADR-0203 §3 の形(/healthz の probe・非 root・readOnlyRootFilesystem・80 番)。
func TestManifestCalcWorkload(t *testing.T) {
	deploytest.AssertWorkload(t, deploytest.Workload{
		Service: calcService, ImageRepo: calcImageRepo, AddrEnv: envAddr, DefaultAddr: defaultAddr,
	})
}

// AC-S4: base はマスタと相性表の場所を持たない(local 専用の ConfigMap を base から参照しない。クラウドは別の渡し方)。
func TestManifestCalcBaseHasNoLocalData(t *testing.T) {
	d := deploytest.BaseDeployment(t, calcService)
	env := d.Container(t, calcService).EnvMap(t)
	for name := range calcLocalDataSources {
		if _, ok := env[name]; ok {
			t.Errorf("base に %s がある(local overlay の Component で設定すること)", name)
		}
	}
	if len(d.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("base に volumes がある: %+v(例のマスタのマウントは local overlay の Component だけ)", d.Spec.Template.Spec.Volumes)
	}
}

// AC-S4: local overlay では CALC_MASTER_PATH / CALC_TYPECHART_PATH が、Component の configMapGenerator が
// Component の下のコピーから作る ConfigMap の key を指し、コピーは元ファイルとバイト一致し、読み込んで起動できる。
func TestManifestCalcLocalDataFromOverlayCopies(t *testing.T) {
	d := deploytest.LocalDeployment(t, calcService)
	c := d.Container(t, calcService)
	env := c.EnvMap(t)
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("local overlay の環境変数で loadConfig が失敗: %v(env=%v)", err, env)
	}

	comp := deploytest.ReadKustomization(t, deploytest.LocalAPIComponentDir)
	copies := map[string]string{} // 環境変数名 → Component の下のコピー(絶対パス)
	for name, source := range calcLocalDataSources {
		path := env[name]
		if !filepath.IsAbs(path) {
			t.Errorf("%s = %q, want コンテナ内の絶対パス", name, path)
			continue
		}
		cmName, key, ok := deploytest.MountedConfigMapFile(d, c, path)
		if !ok {
			t.Errorf("%s = %q が ConfigMap のマウントから来ていない", name, path)
			continue
		}
		var gen *deploytest.ConfigMapGenerator
		for i := range comp.ConfigMapGenerator {
			if comp.ConfigMapGenerator[i].Name == cmName {
				gen = &comp.ConfigMapGenerator[i]
			}
		}
		if gen == nil {
			t.Errorf("ConfigMap %q が %s の configMapGenerator に無い", cmName, deploytest.LocalAPIComponentDir)
			continue
		}
		file, ok := gen.SourceFor(key)
		if !ok {
			t.Errorf("configMapGenerator %q に key %q が無い", cmName, key)
			continue
		}
		if strings.Contains(file, "/") || strings.Contains(file, "..") {
			t.Errorf("configMapGenerator %q の %q は Component の直下のファイルを指すこと(load restrictor)", cmName, file)
			continue
		}
		copyPath := deploytest.RepoPath(t, deploytest.LocalAPIComponentDir+"/"+file)
		want, err := os.ReadFile(deploytest.RepoPath(t, source))
		if err != nil {
			t.Fatalf("元ファイル %s を読めない: %v", source, err)
		}
		got, err := os.ReadFile(copyPath)
		if err != nil {
			t.Errorf("%s の %s を読めない: %v", deploytest.LocalAPIComponentDir, file, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s/%s が元ファイル %s と一致しない(元ファイルが正。コピーし直すこと)", deploytest.LocalAPIComponentDir, file, source)
		}
		copies[name] = copyPath
	}
	for _, m := range c.VolumeMounts {
		if !m.ReadOnly {
			t.Errorf("volumeMount %q が readOnly でない", m.Name)
		}
	}
	if t.Failed() {
		return
	}

	// コピーそのもので起動できる(スキーマ違反・相性表との不整合なら newHandler が失敗する)。
	cfg.MasterPath = copies[envMasterPath]
	cfg.TypeChartPath = copies[envTypeChartPath]
	if _, err := newHandler(cfg); err != nil {
		t.Fatalf("Component の下のコピーで newHandler が失敗: %v", err)
	}
}

// AC-S4: local overlay で使うイメージは api-docker-build が作る pokecalc/calc:local。
func TestManifestCalcLocalImage(t *testing.T) {
	deploytest.AssertLocalImage(t, calcService, calcImageRepo)
}
