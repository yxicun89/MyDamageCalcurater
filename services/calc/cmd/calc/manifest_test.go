package main

// calc-svc の k8s マニフェストの静的検査(ADR-0203 §3。AC-S3・AC-S4。ADR-0204 §4。ADR-0206)。kubectl を使わず YAML を読む。
// マスタの入手元は pokedex-svc の内部 API(URL 方式。ADR-0204 §2)で、その設定は base に置く(ADR-0206 §1。
// Service 名 pokedex はどの環境でも同じで、local と local-api の2つの overlay が同じ Deployment を別内容で
// 描画しないようにするため)。ファイル方式は `make dev` とテストだけで使う。

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

const (
	calcService   = "calc"
	calcImageRepo = "pokecalc/calc"
	// calcReadinessPath は calc-svc の readiness の probe(マスタの取得前は 503。ADR-0204 §3)。
	calcReadinessPath = "/readyz"
	// pokedexServiceName は calc-svc がマスタを取りに行く pokedex-svc の Service 名(deploy/k8s/base/pokedex。ADR-0105)。
	pokedexServiceName = "pokedex"
	// calcMasterExampleConfigMap は、ファイル方式だったころに local の Component が作っていた ConfigMap(ADR-0206 で廃止)。
	calcMasterExampleConfigMap = "calc-master-example"
)

// AC-S3: base の Deployment・Service が ADR-0203 §3 の形(liveness は /healthz、readiness は /readyz・非 root・
// readOnlyRootFilesystem・80 番)。readiness を /readyz にするのは、URL 方式でマスタの取得前に Service の宛先へ
// 入れないため(ADR-0204 §3)。
func TestManifestCalcWorkload(t *testing.T) {
	deploytest.AssertWorkload(t, deploytest.Workload{
		Service: calcService, ImageRepo: calcImageRepo, AddrEnv: envAddr, DefaultAddr: defaultAddr,
		ReadinessPath: calcReadinessPath,
	})
}

// assertPokedexMasterURL は env がマスタの入手元として pokedex-svc の Service(http://pokedex の 80 番)だけを
// 指していることを確かめる(ADR-0206 §1)。ファイル方式の設定(local 専用の ConfigMap を要する CALC_MASTER_PATH)と
// 廃止した CALC_TYPECHART_PATH は、base にも overlay にも無い。
func assertPokedexMasterURL(t *testing.T, where string, env map[string]string) {
	t.Helper()
	for _, name := range []string{envMasterPath, envTypeChartPath} {
		if v, ok := env[name]; ok {
			t.Errorf("%s に %s=%q がある(マスタは pokedex-svc の内部 API から取る。ADR-0206)", where, name, v)
		}
	}
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("%s の環境変数で loadConfig が失敗: %v(env=%v)", where, err, env)
	}
	if cfg.MasterURL == "" {
		t.Fatalf("%s に %s が無い(pokedex-svc の Service を指すこと)", where, envMasterURL)
	}
	u, err := url.Parse(cfg.MasterURL)
	if err != nil {
		t.Fatalf("%s の %s を解析できない %q: %v", where, envMasterURL, cfg.MasterURL, err)
	}
	if u.Scheme != "http" || u.Hostname() != pokedexServiceName || (u.Port() != "" && u.Port() != "80") {
		t.Errorf("%s の %s = %q, want http://%s(Service の %d 番)", where, envMasterURL, cfg.MasterURL,
			pokedexServiceName, deploytest.ServicePort)
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		t.Errorf("%s の %s にパス・クエリがある: %q(内部 API のパスは calc-svc が付ける)", where, envMasterURL, cfg.MasterURL)
	}
}

// assertNoConfigMapMounts は Deployment が ConfigMap を1つもマウントしていないことを確かめる
// (URL 方式にしたので、例のマスタのファイルを読ませる必要が無い。ADR-0206 §1)。
func assertNoConfigMapMounts(t *testing.T, where string, d deploytest.Deployment) {
	t.Helper()
	if len(d.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("%s に volumes がある: %+v(マスタは pokedex-svc から取るのでマウントは要らない)", where, d.Spec.Template.Spec.Volumes)
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		if len(c.VolumeMounts) != 0 {
			t.Errorf("%s のコンテナ %s に volumeMounts がある: %+v", where, c.Name, c.VolumeMounts)
		}
	}
}

// AC-P3(ADR-0206。TestManifestCalcBaseHasNoLocalData からの移行): base だけでマスタの入手元がそろう
// (CALC_MASTER_URL=http://pokedex)。local 専用の ConfigMap を base から参照しないことは変わらず固定する
// (ファイル方式の設定・volumes が base に無い)。
func TestManifestCalcBaseUsesPokedexMaster(t *testing.T) {
	d := deploytest.BaseDeployment(t, calcService)
	env := d.Container(t, calcService).EnvMap(t)
	assertPokedexMasterURL(t, deploytest.BaseDir, env)
	assertNoConfigMapMounts(t, deploytest.BaseDir, d)
}

// AC-P4(ADR-0206。TestManifestCalcLocalDataFromOverlayCopies からの移行): local overlay の Component は calc に
// patch を当てず、base の URL 方式のままになる(local と local-api が同じ内容の Deployment calc を描画する)。
func TestManifestCalcLocalUsesPokedexMaster(t *testing.T) {
	d := deploytest.LocalDeployment(t, calcService)
	env := d.Container(t, calcService).EnvMap(t)
	assertPokedexMasterURL(t, deploytest.LocalAPIComponentDir, env)
	assertNoConfigMapMounts(t, deploytest.LocalAPIComponentDir, d)
}

// AC-P4(ADR-0206。ADR-0204 が typechart.example.json を廃止したときと同じ形): ファイル方式をやめたので、
// local の Component に例のマスタの ConfigMap もコピーも残っていない。元ファイル(単一の正)は `make dev`・
// calctest・契約テストが使うので残っていること。
func TestManifestCalcLocalHasNoMasterCopy(t *testing.T) {
	comp := deploytest.ReadKustomization(t, deploytest.LocalAPIComponentDir)
	for _, gen := range comp.ConfigMapGenerator {
		if gen.Name == calcMasterExampleConfigMap {
			t.Errorf("%s に ConfigMap %s の configMapGenerator が残っている(マスタは pokedex-svc から取る)",
				deploytest.LocalAPIComponentDir, calcMasterExampleConfigMap)
		}
	}
	stale := deploytest.RepoPath(t, deploytest.LocalAPIComponentDir+"/master.example.json")
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s が残っている(err=%v)。k3d はマスタを pokedex-svc から取る", stale, err)
	}
	for _, v := range deploytest.LocalDeployment(t, calcService).Spec.Template.Spec.Volumes {
		if v.ConfigMap != nil && v.ConfigMap.Name == calcMasterExampleConfigMap {
			t.Errorf("local の calc Deployment が ConfigMap %s をマウントしている", calcMasterExampleConfigMap)
		}
	}
	source := deploytest.RepoPath(t, "services/calc/testdata/master.example.json")
	if _, err := os.Stat(source); err != nil {
		t.Errorf("元ファイル %s が無い(%v)。`make dev`・calctest・スモークの fallback が使う", source, err)
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
