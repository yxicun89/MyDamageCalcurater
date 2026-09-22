package main

// gateway の k8s マニフェストの静的検査(ADR-0203 §3。AC-S3・AC-S4)。kubectl を使わず YAML を読む。
// 環境変数名はこのパッケージの定数と突き合わせ、local overlay の値で loadConfig が通ることまで確かめる。

import (
	"slices"
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

const (
	gatewayService   = "gateway"
	gatewayImageRepo = "pokecalc/gateway"
	// calcServiceName は gateway が上流として指す calc-svc の Service 名(GATEWAY_CALC_URL=http://calc)。
	calcServiceName = "calc"
	// localViteOrigin は local overlay で CORS を許可するオリジン(Vite の開発サーバの既定)。
	localViteOrigin = "http://localhost:5173"
)

// AC-S3: base の Deployment・Service が ADR-0203 §3 の形(/healthz の probe・非 root・readOnlyRootFilesystem・80 番)。
func TestManifestGatewayWorkload(t *testing.T) {
	deploytest.AssertWorkload(t, deploytest.Workload{
		Service: gatewayService, ImageRepo: gatewayImageRepo, AddrEnv: envAddr, DefaultAddr: defaultAddr,
	})
}

// AC-S3: gateway の Ingress は traefik・path "/" Prefix・ホスト指定なしで gateway の Service を指す
// (balance の "/api/balance" は最長一致で balance の Ingress に届く。ADR-0012)。
func TestManifestGatewayIngress(t *testing.T) {
	var ing deploytest.Ingress
	deploytest.Find(t, deploytest.BaseObjects(t, gatewayService), "Ingress", gatewayService).Decode(t, &ing)
	if ing.Spec.IngressClassName != "traefik" {
		t.Errorf("ingressClassName = %q, want traefik", ing.Spec.IngressClassName)
	}
	if len(ing.Spec.Rules) != 1 {
		t.Fatalf("rules が %d 個(1個であること)", len(ing.Spec.Rules))
	}
	rule := ing.Spec.Rules[0]
	if rule.Host != "" {
		t.Errorf("host = %q, want 指定なし(k3d の localhost:8080 で受ける)", rule.Host)
	}
	if len(rule.HTTP.Paths) != 1 {
		t.Fatalf("paths が %d 個(1個であること)", len(rule.HTTP.Paths))
	}
	p := rule.HTTP.Paths[0]
	if p.Path != "/" || p.PathType != "Prefix" {
		t.Errorf("path = %q %q, want \"/\" Prefix", p.Path, p.PathType)
	}
	backend := p.Backend.Service
	if backend.Name != gatewayService {
		t.Errorf("backend.service.name = %q, want %q", backend.Name, gatewayService)
	}
	if backend.Port.Name != "http" && backend.Port.Number != deploytest.ServicePort {
		t.Errorf("backend.service.port = %+v, want 名前 http か %d", backend.Port, deploytest.ServicePort)
	}
}

// AC-S4: base の設定だけで起動できる(GATEWAY_CALC_URL は Service 名)。CORS の許可オリジンは local 専用なので base に置かない。
func TestManifestGatewayBaseConfig(t *testing.T) {
	d := deploytest.BaseDeployment(t, gatewayService)
	env := d.Container(t, gatewayService).EnvMap(t)
	if _, ok := env[envCORSAllowedOrigins]; ok {
		t.Errorf("base に %s がある(localhost の許可は local overlay だけに置く)", envCORSAllowedOrigins)
	}
	if _, err := loadConfig(lookupFrom(env)); err != nil {
		t.Fatalf("base の環境変数で loadConfig が失敗: %v(env=%v)", err, env)
	}
}

// AC-S4: local overlay(base + deploy/k8s/overlays/local/api)の環境変数で loadConfig が通り、
// calc は Service 名 calc の 80 番、pokedex・assets は未設定(503 / 404)、CORS は Vite の既定オリジンだけ。
func TestManifestGatewayLocalConfig(t *testing.T) {
	d := deploytest.LocalDeployment(t, gatewayService)
	env := d.Container(t, gatewayService).EnvMap(t)
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("local overlay の環境変数で loadConfig が失敗: %v(env=%v)", err, env)
	}
	calc := cfg.Gateway.CalcURL
	if calc.Scheme != "http" || calc.Hostname() != calcServiceName || (calc.Port() != "" && calc.Port() != "80") {
		t.Errorf("%s = %q, want http://%s(Service の %d 番)", envCalcURL, calc, calcServiceName, deploytest.ServicePort)
	}
	if calc.Path != "" && calc.Path != "/" {
		t.Errorf("%s にパスがある: %q", envCalcURL, calc)
	}
	// pokedex-svc(plan.md P2-3)を deploy/k8s に入れたら、この検査と smoke.sh の 503 の確認を一緒に変える(ADR-0203 §5)。
	if cfg.Gateway.PokedexURL != nil {
		t.Errorf("%s が設定されている(pokedex-svc はまだ無い。入れるならスモークの 503 の確認も変えること)", envPokedexURL)
	}
	if cfg.Gateway.AssetsURL != nil {
		t.Errorf("%s が設定されている(画像配信はまだ無い)", envAssetsURL)
	}
	if want := []string{localViteOrigin}; !slices.Equal(cfg.Gateway.CORSAllowedOrigins, want) {
		t.Errorf("CORS の許可オリジン = %q, want %q", cfg.Gateway.CORSAllowedOrigins, want)
	}
}

// AC-S4: local overlay で使うイメージは api-docker-build が作る pokecalc/gateway:local。
func TestManifestGatewayLocalImage(t *testing.T) {
	deploytest.AssertLocalImage(t, gatewayService, gatewayImageRepo)
}
