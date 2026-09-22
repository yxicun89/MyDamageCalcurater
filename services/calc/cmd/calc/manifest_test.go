package main

// calc-svc の k8s マニフェストの静的検査(ADR-0203 §3。AC-S3・AC-S4。ADR-0204 §4)。kubectl を使わず YAML を読む。
// 例のマスタ(MasterExport の形。相性表を含む)は local overlay 専用の Component(deploy/k8s/overlays/local/api)が
// configMapGenerator で読ませる(ファイル方式。pokedex-svc がデプロイされたら URL 方式に切り替える。ADR-0204)。
// Kustomize の load restrictor のため Component の下にコピーを置くので、元ファイルとのバイト一致を固定する
// (services/balance の前例と同じ)。

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

const (
	calcService   = "calc"
	calcImageRepo = "pokecalc/calc"
	// calcReadinessPath は calc-svc の readiness の probe(マスタの取得前は 503。ADR-0204 §3)。
	calcReadinessPath = "/readyz"
)

// 元ファイル(単一の正。リポジトリ直下からの相対)。
var calcLocalDataSources = map[string]string{
	envMasterPath: "services/calc/testdata/master.example.json", // 架空データ + 相性表(ADR-0002・ADR-0204)
}

// AC-S3: base の Deployment・Service が ADR-0203 §3 の形(liveness は /healthz、readiness は /readyz・非 root・
// readOnlyRootFilesystem・80 番)。readiness を /readyz にするのは、URL 方式でマスタの取得前に Service の宛先へ
// 入れないため(ADR-0204 §3)。
func TestManifestCalcWorkload(t *testing.T) {
	deploytest.AssertWorkload(t, deploytest.Workload{
		Service: calcService, ImageRepo: calcImageRepo, AddrEnv: envAddr, DefaultAddr: defaultAddr,
		ReadinessPath: calcReadinessPath,
	})
}

// AC-S4: base はマスタの場所を持たない(local 専用の ConfigMap を base から参照しない。クラウドは別の渡し方)。
// 廃止した CALC_TYPECHART_PATH も無い(ADR-0204)。
func TestManifestCalcBaseHasNoLocalData(t *testing.T) {
	d := deploytest.BaseDeployment(t, calcService)
	env := d.Container(t, calcService).EnvMap(t)
	for _, name := range []string{envMasterPath, envMasterURL, envTypeChartPath} {
		if _, ok := env[name]; ok {
			t.Errorf("base に %s がある(local overlay の Component で設定すること)", name)
		}
	}
	if len(d.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("base に volumes がある: %+v(例のマスタのマウントは local overlay の Component だけ)", d.Spec.Template.Spec.Volumes)
	}
}

// AC-S4: local overlay はファイル方式(ADR-0204 §4): CALC_MASTER_PATH が、Component の configMapGenerator が
// Component の下のコピーから作る ConfigMap の key を指し、コピーは元ファイルとバイト一致し、読み込んで起動できる。
// CALC_MASTER_URL(pokedex-svc のデプロイ後に切り替える)と、廃止した CALC_TYPECHART_PATH は無い。
func TestManifestCalcLocalDataFromOverlayCopies(t *testing.T) {
	d := deploytest.LocalDeployment(t, calcService)
	c := d.Container(t, calcService)
	env := c.EnvMap(t)
	for _, name := range []string{envMasterURL, envTypeChartPath} {
		if _, ok := env[name]; ok {
			t.Errorf("local overlay に %s がある(local はファイル方式。ADR-0204 §4)", name)
		}
	}
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

	// コピーそのもので起動できる(契約違反・写像のエラーなら newHandler が失敗する)。
	cfg.MasterPath = copies[envMasterPath]
	if _, err := newHandler(context.Background(), cfg); err != nil {
		t.Fatalf("Component の下のコピーで newHandler が失敗: %v", err)
	}
}

// AC-S4: 相性表は MasterExport に含まれるので、local overlay の Component は相性表の ConfigMap もコピーも持たない
// (ADR-0204 §4。古いコピーが残って「単一の正」がぶれないように)。
func TestManifestCalcLocalHasNoTypeChartCopy(t *testing.T) {
	comp := deploytest.ReadKustomization(t, deploytest.LocalAPIComponentDir)
	for _, gen := range comp.ConfigMapGenerator {
		if gen.Name == "calc-typechart" {
			t.Errorf("%s に ConfigMap calc-typechart の configMapGenerator が残っている", deploytest.LocalAPIComponentDir)
		}
	}
	stale := deploytest.RepoPath(t, deploytest.LocalAPIComponentDir+"/typechart.example.json")
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s が残っている(err=%v)。相性表は master.example.json に含まれる", stale, err)
	}
	d := deploytest.LocalDeployment(t, calcService)
	for _, v := range d.Spec.Template.Spec.Volumes {
		if v.ConfigMap != nil && v.ConfigMap.Name == "calc-typechart" {
			t.Errorf("local の calc Deployment が ConfigMap calc-typechart をマウントしている")
		}
	}
}

// AC-S4: local overlay で使うイメージは api-docker-build が作る pokecalc/calc:local。
func TestManifestCalcLocalImage(t *testing.T) {
	deploytest.AssertLocalImage(t, calcService, calcImageRepo)
}
