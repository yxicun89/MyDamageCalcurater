package main

// gateway の k8s マニフェストの静的検査(ADR-0203 §3。AC-S3・AC-S4)。kubectl を使わず YAML を読む。
// 環境変数名はこのパッケージの定数と突き合わせ、local overlay の値で loadConfig が通ることまで確かめる。

import (
	"net/url"
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
	// webServiceName は Web レーンが deploy/k8s/base/web に置く nginx の Service 名(80 番。ADR-0205)。
	webServiceName = "web"
	// pokedexServiceName は gateway が /api/pokedex/* を転送する pokedex-svc の Service 名
	// (deploy/k8s/base/pokedex。ADR-0105・ADR-0206)。
	pokedexServiceName = "pokedex"
)

// assertServiceURL は cfg の上流 URL が Service 名の 80 番(パス・クエリなし)であることを確かめる。
func assertServiceURL(t *testing.T, envName string, u *url.URL, service string) {
	t.Helper()
	if u == nil {
		t.Fatalf("%s が読まれていない(nil)", envName)
	}
	if u.Scheme != "http" || u.Hostname() != service || (u.Port() != "" && u.Port() != "80") {
		t.Errorf("%s = %q, want http://%s(Service の %d 番)", envName, u, service, deploytest.ServicePort)
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		t.Errorf("%s にパス・クエリがある: %q", envName, u)
	}
}

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

// AC-S4 / AC-P1(ADR-0206): base の設定だけで起動できる(GATEWAY_CALC_URL・GATEWAY_POKEDEX_URL はどちらも
// Service 名。Service 名はクラウドでも同じなので base に置く)。CORS の許可オリジンは local 専用なので base に置かない。
// 画像配信(GATEWAY_ASSETS_URL)はまだ無いので未設定(→ /assets/* は 404)。
func TestManifestGatewayBaseConfig(t *testing.T) {
	d := deploytest.BaseDeployment(t, gatewayService)
	env := d.Container(t, gatewayService).EnvMap(t)
	if _, ok := env[envCORSAllowedOrigins]; ok {
		t.Errorf("base に %s がある(localhost の許可は local overlay だけに置く)", envCORSAllowedOrigins)
	}
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("base の環境変数で loadConfig が失敗: %v(env=%v)", err, env)
	}
	assertServiceURL(t, envCalcURL, cfg.Gateway.CalcURL, calcServiceName)
	assertServiceURL(t, envPokedexURL, cfg.Gateway.PokedexURL, pokedexServiceName)
	if cfg.Gateway.AssetsURL != nil {
		t.Errorf("base に %s がある(画像配信はまだ無い)", envAssetsURL)
	}
}

// AC-S4 / AC-P2(ADR-0206): local overlay(base + deploy/k8s/overlays/local/api)の環境変数で loadConfig が通り、
// calc は Service 名 calc の 80 番、pokedex は Service 名 pokedex の 80 番(base の設定が Component の patch で
// 消えていないこと)、assets は未設定(404)、CORS は Vite の既定オリジンだけ。
func TestManifestGatewayLocalConfig(t *testing.T) {
	d := deploytest.LocalDeployment(t, gatewayService)
	env := d.Container(t, gatewayService).EnvMap(t)
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("local overlay の環境変数で loadConfig が失敗: %v(env=%v)", err, env)
	}
	assertServiceURL(t, envCalcURL, cfg.Gateway.CalcURL, calcServiceName)
	// ADR-0206: pokedex-svc(P2-3)が base に入ったので /api/pokedex/* は上流に届く(未設定の 503 ではなくなった)。
	assertServiceURL(t, envPokedexURL, cfg.Gateway.PokedexURL, pokedexServiceName)
	if cfg.Gateway.AssetsURL != nil {
		t.Errorf("%s が設定されている(画像配信はまだ無い)", envAssetsURL)
	}
	if want := []string{localViteOrigin}; !slices.Equal(cfg.Gateway.CORSAllowedOrigins, want) {
		t.Errorf("CORS の許可オリジン = %q, want %q", cfg.Gateway.CORSAllowedOrigins, want)
	}
}

// AC-W8(ADR-0205): GATEWAY_WEB_URL は base に置かず(クラウドの Web の置き方は未定)、local overlay だけが
// Web レーンの Service 名 web(80 番)を指す。loadConfig を通した値も http://web になる。
func TestManifestGatewayWebURLOnlyInLocal(t *testing.T) {
	baseDeployment := deploytest.BaseDeployment(t, gatewayService)
	base := baseDeployment.Container(t, gatewayService).EnvMap(t)
	if v, ok := base[envWebURL]; ok {
		t.Errorf("base に %s=%q がある(local overlay だけに置く)", envWebURL, v)
	}

	d := deploytest.LocalDeployment(t, gatewayService)
	env := d.Container(t, gatewayService).EnvMap(t)
	if got := env[envWebURL]; got != "http://"+webServiceName {
		t.Fatalf("local overlay の %s = %q, want %q", envWebURL, got, "http://"+webServiceName)
	}
	cfg, err := loadConfig(lookupFrom(env))
	if err != nil {
		t.Fatalf("local overlay の環境変数で loadConfig が失敗: %v(env=%v)", err, env)
	}
	web := cfg.Gateway.WebURL
	if web == nil {
		t.Fatalf("loadConfig が %s を読んでいない(WebURL が nil)", envWebURL)
	}
	if web.Scheme != "http" || web.Hostname() != webServiceName || (web.Port() != "" && web.Port() != "80") {
		t.Errorf("WebURL = %q, want http://%s(Service の %d 番)", web, webServiceName, deploytest.ServicePort)
	}
	if web.Path != "" && web.Path != "/" {
		t.Errorf("WebURL にパスがある: %q", web)
	}
}

// AC-S4: local overlay で使うイメージは api-docker-build が作る pokecalc/gateway:local。
func TestManifestGatewayLocalImage(t *testing.T) {
	deploytest.AssertLocalImage(t, gatewayService, gatewayImageRepo)
}
