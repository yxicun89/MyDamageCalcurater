// NetworkPolicy の描画結果の検査(issue #240・ADR-0132)。AC-N1〜N4 は kubectl kustomize の描画を使うので、kubectl が無い
// 環境では skip になり、kustomization への組み込み(AC-N5)だけが走る。CI(.github/workflows/ci.yml は kubectl を入れる)と
// 開発機の make test-services で走ることを前提にする。
package deploytest

// pokecalc namespace の NetworkPolicy(ingress の default-deny + 許可リスト。ADR-0132・issue #240)の静的検査。
//
// `kubectl kustomize` の描画結果から NetworkPolicy・Pod テンプレート・Service を読み、
// 「送信元 Pod → 宛先 Pod:ポート」が通るかを NetworkPolicy の意味どおりに評価する(CNI は使わない)。
// 評価するのは ingress の podSelector / namespaceSelector / ports だけ。ipBlock・egress は今回の対象外(ADR-0132)。
//
// 許可表は実装の写しではなく、このテストが独立に持つ「あるべき通信」(coding-rules §2 の独立した検証の例外)。
// 宛先ポートは Service の port(80)ではなく Pod の containerPort(8080)。NetworkPolicy は DNAT 後に評価されるため。
//
//   AC-N1: base / local / cloud の描画に ingress の default-deny(podSelector: {})がある。egress は絞らない
//   AC-N2: 許可表の通信がすべて通る(ラベル・namespace・ポートが実際の Pod と一致している)
//   AC-N3: 拒否表の通信がすべて拒否される(web → pokedex・mysql、他 namespace → pokedex など)
//   AC-N4: どの Service も、少なくとも1つの許可された送信元から到達できる(許可の書き漏らしで Service が孤立しない)
//   AC-N5: base/kustomization.yaml が networkpolicy を読み込む

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	appNameLabel      = "app.kubernetes.io/name"
	namespaceNameKey  = "kubernetes.io/metadata.name"
	podPort           = 8080 // 各サービスの containerPort。Service の port 80 の targetPort
	gatewayMetrics    = 9090 // gateway のメトリクス専用 containerPort(issue #216)。他サービスは podPort で /metrics を出す
	mysqlPort         = 3306
	natsPort          = 4222
	tidbPort          = 4000
	appNamespace      = "pokecalc"
	renderLocalDir    = "deploy/k8s/overlays/local"
	renderCloudDir    = "deploy/k8s/overlays/cloud"
	renderBaseDir     = "deploy/k8s/base"
	networkPolicyDir  = "networkpolicy"
	renderTidbDir     = "deploy/k8s/overlays/local/tidb"
	renderBalanceDir  = "services/balance/deploy/k8s/overlays/local"
	renderSpeedDir    = "services/speed/deploy/k8s/overlays/local"
	renderJudgeDir    = "services/judge/deploy/k8s/overlays/local"
	recordMigrateFile = "deploy/k8s/base/record/job-migrate.yaml"
	teamMigrateFile   = "deploy/k8s/base/team/job-migrate.yaml"
)

// npPod は評価に使う Pod 1種類(Deployment 等の Pod テンプレート、または外部の送信元)。
type npPod struct {
	namespace string
	labels    map[string]string
	ports     map[string]int // containerPort の name → number
}

// npWorld は評価の世界。namespaces は namespace 名 → その namespace のラベル。
type npWorld struct {
	policies []npPolicy
	pods     map[string]npPod // キーは app.kubernetes.io/name(外部の送信元は専用の名前)
	services []npService
}

type npService struct {
	name       string
	selector   map[string]string
	targetPort string // 数値または名前
}

// 外部(pokecalc の外)の送信元。ラベルは実クラスタで確認した値(k3s 同梱 Traefik / kube-prometheus-stack)。
var (
	traefikPod = npPod{namespace: "kube-system", labels: map[string]string{
		appNameLabel: "traefik", "app.kubernetes.io/instance": "traefik-kube-system"}}
	prometheusPod = npPod{namespace: "observability", labels: map[string]string{
		appNameLabel: "prometheus", "app.kubernetes.io/instance": "kube-prometheus-stack-prometheus"}}
	grafanaPod = npPod{namespace: "observability", labels: map[string]string{
		appNameLabel: "grafana", "app.kubernetes.io/instance": "kube-prometheus-stack"}}
	corednsPod = npPod{namespace: "kube-system", labels: map[string]string{"k8s-app": "kube-dns"}}
	// 別 namespace に同じラベルの Pod を作られても通してはいけない(namespaceSelector の検査)。
	impostorPrometheusPod = npPod{namespace: "default", labels: prometheusPod.labels}
	impostorTraefikPod    = npPod{namespace: appNamespace, labels: traefikPod.labels}
	strayPod              = npPod{namespace: "default", labels: map[string]string{"run": "tmp"}}
	// tidb-operator が付ける標準ラベル(TiDB は pokecalc namespace に入る。ADR-0211)。実クラスタで TiDB 起動後に要確認。
	tidbServerPod = npPod{namespace: appNamespace, labels: map[string]string{
		"app.kubernetes.io/instance": "pokecalc-tidb", "app.kubernetes.io/component": "tidb",
		"app.kubernetes.io/managed-by": "tidb-operator"}}
	tidbPDPod = npPod{namespace: appNamespace, labels: map[string]string{
		"app.kubernetes.io/instance": "pokecalc-tidb", "app.kubernetes.io/component": "pd",
		"app.kubernetes.io/managed-by": "tidb-operator"}}
)

// ---- 描画の読み込み ----

func kustomizeRender(t *testing.T, dir string) []map[string]any {
	t.Helper()
	out, err := exec.Command("kubectl", "kustomize", RepoPath(t, dir)).Output()
	if err != nil {
		t.Fatalf("kubectl kustomize %s が失敗した: %v", dir, err)
	}
	return decodeAll(t, dir, out)
}

func decodeAll(t *testing.T, from string, raw []byte) []map[string]any {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return docs
		}
		if err != nil {
			t.Fatalf("%s の描画結果を解析できない: %v", from, err)
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
}

func requireKubectl(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl が無いので描画の検査を省略する(AC-N5 の静的検査は走っている)")
	}
}

// 汎用の map 辿り。無ければ nil。
func dig(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func asString(v any) string { s, _ := v.(string); return s }

func asStringMap(v any) map[string]string {
	m, _ := v.(map[string]any)
	out := map[string]string{}
	for k, val := range m {
		out[k] = asString(val)
	}
	return out
}

func asList(v any) []any { l, _ := v.([]any); return l }

// podTemplateOf は Deployment / StatefulSet / Job / CronJob の Pod テンプレートを返す。
func podTemplateOf(doc map[string]any) (map[string]any, bool) {
	switch asString(doc["kind"]) {
	case "Deployment", "StatefulSet", "Job":
		tpl, ok := dig(doc, "spec", "template").(map[string]any)
		return tpl, ok
	case "CronJob":
		tpl, ok := dig(doc, "spec", "jobTemplate", "spec", "template").(map[string]any)
		return tpl, ok
	}
	return nil, false
}

// loadWorld は描画結果から評価の世界を作る。policies は policyDirs の描画だけから集める
// (許可の置き場所は pokecalc namespace の base に一本化するため)。
func loadWorld(t *testing.T) npWorld {
	t.Helper()
	w := npWorld{pods: map[string]npPod{}}

	for _, doc := range kustomizeRender(t, renderLocalDir) {
		if asString(doc["kind"]) == "NetworkPolicy" {
			w.policies = append(w.policies, parsePolicy(t, doc))
		}
	}
	// Pod とサービスは、各レーンの local overlay(gitops overlay と同じ base)から集める。
	for _, dir := range []string{renderLocalDir, renderBalanceDir, renderSpeedDir, renderJudgeDir, renderTidbDir} {
		for _, doc := range kustomizeRenderDocs(t, dir) {
			w.addDoc(doc)
		}
	}
	for _, file := range []string{recordMigrateFile, teamMigrateFile} {
		for _, o := range ReadObjects(t, file) {
			var doc map[string]any
			o.Decode(t, &doc)
			w.addDoc(doc)
		}
	}
	return w
}

func kustomizeRenderDocs(t *testing.T, dir string) []map[string]any {
	t.Helper()
	if strings.HasSuffix(dir, "/tidb") {
		// TidbCluster は CRD でテンプレートを持たない。TiDB は固定の npPod で扱う。
		return nil
	}
	return kustomizeRender(t, dir)
}

func (w *npWorld) addDoc(doc map[string]any) {
	if tpl, ok := podTemplateOf(doc); ok {
		labels := asStringMap(dig(tpl, "metadata", "labels"))
		name := labels[appNameLabel]
		if name == "" {
			return
		}
		pod := npPod{namespace: appNamespace, labels: labels, ports: map[string]int{}}
		for _, c := range asList(dig(tpl, "spec", "containers")) {
			for _, p := range asList(dig(c, "ports")) {
				n, _ := dig(p, "containerPort").(int)
				pod.ports[asString(dig(p, "name"))] = n
			}
		}
		w.pods[name] = pod
	}
	if asString(doc["kind"]) == "Service" {
		for _, p := range asList(dig(doc, "spec", "ports")) {
			tp := dig(p, "targetPort")
			target := ""
			switch v := tp.(type) {
			case int:
				target = strconv.Itoa(v)
			case string:
				target = v
			}
			w.services = append(w.services, npService{
				name:       asString(dig(doc, "metadata", "name")),
				selector:   asStringMap(dig(doc, "spec", "selector")),
				targetPort: target,
			})
		}
	}
}

// ---- NetworkPolicy の評価 ----

type npSelector struct {
	matchLabels map[string]string
	exprs       []npExpr
	present     bool
}

type npExpr struct {
	key, op string
	values  []string
}

type npPeer struct {
	podSelector       *npSelector
	namespaceSelector *npSelector
}

type npPortRule struct {
	port     string // 数値または名前。空は全ポート
	protocol string
}

type npIngressRule struct {
	from  []npPeer // 空なら全送信元
	ports []npPortRule
}

type npPolicy struct {
	name        string
	namespace   string
	podSelector npSelector
	types       []string
	ingress     []npIngressRule
}

func parseSelector(v any) *npSelector {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	s := &npSelector{matchLabels: asStringMap(m["matchLabels"]), present: true}
	for _, e := range asList(m["matchExpressions"]) {
		ex := npExpr{key: asString(dig(e, "key")), op: asString(dig(e, "operator"))}
		for _, val := range asList(dig(e, "values")) {
			ex.values = append(ex.values, asString(val))
		}
		s.exprs = append(s.exprs, ex)
	}
	return s
}

func parsePolicy(t *testing.T, doc map[string]any) npPolicy {
	t.Helper()
	p := npPolicy{
		name:      asString(dig(doc, "metadata", "name")),
		namespace: asString(dig(doc, "metadata", "namespace")),
	}
	if s := parseSelector(dig(doc, "spec", "podSelector")); s != nil {
		p.podSelector = *s
	}
	for _, ty := range asList(dig(doc, "spec", "policyTypes")) {
		p.types = append(p.types, asString(ty))
	}
	for _, r := range asList(dig(doc, "spec", "ingress")) {
		var rule npIngressRule
		for _, f := range asList(dig(r, "from")) {
			rule.from = append(rule.from, npPeer{
				podSelector:       parseSelector(dig(f, "podSelector")),
				namespaceSelector: parseSelector(dig(f, "namespaceSelector")),
			})
			if dig(f, "ipBlock") != nil {
				t.Errorf("NetworkPolicy %s: ipBlock は使わない(Pod・namespace のラベルで許可する。ADR-0132)", p.name)
			}
		}
		for _, pr := range asList(dig(r, "ports")) {
			rule.ports = append(rule.ports, npPortRule{
				port:     portString(dig(pr, "port")),
				protocol: asString(dig(pr, "protocol")),
			})
		}
		p.ingress = append(p.ingress, rule)
	}
	return p
}

func portString(v any) string {
	switch p := v.(type) {
	case int:
		return strconv.Itoa(p)
	case string:
		return p
	}
	return ""
}

func (s npSelector) matches(labels map[string]string) bool {
	for k, v := range s.matchLabels {
		if got, ok := labels[k]; !ok || got != v {
			return false
		}
	}
	for _, e := range s.exprs {
		got, has := labels[e.key]
		in := false
		for _, v := range e.values {
			if has && got == v {
				in = true
			}
		}
		switch e.op {
		case "In":
			if !in {
				return false
			}
		case "NotIn":
			if in {
				return false
			}
		case "Exists":
			if !has {
				return false
			}
		case "DoesNotExist":
			if has {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// namespaceLabels は namespace のラベル。Kubernetes が自動で付ける kubernetes.io/metadata.name だけを前提にする
// (ほかのラベルに頼ると namespace を作り直したときに壊れるため。ADR-0132)。
func namespaceLabels(ns string) map[string]string {
	return map[string]string{namespaceNameKey: ns}
}

func (p npPeer) matches(src npPod, policyNamespace string) bool {
	if p.namespaceSelector != nil {
		if !p.namespaceSelector.matches(namespaceLabels(src.namespace)) {
			return false
		}
	} else if src.namespace != policyNamespace {
		return false // podSelector だけの peer は、ポリシーと同じ namespace の Pod だけ
	}
	if p.podSelector != nil && !p.podSelector.matches(src.labels) {
		return false
	}
	return true
}

func (r npPortRule) matches(dst npPod, port int) bool {
	if r.protocol != "" && r.protocol != "TCP" {
		return false
	}
	if r.port == "" {
		return true
	}
	if n, err := strconv.Atoi(r.port); err == nil {
		return n == port
	}
	return dst.ports[r.port] == port // 名前付きポート
}

// allowed は src から dst の port(Pod 側のポート)へ ingress が通るかを返す。
func (w npWorld) allowed(src, dst npPod, port int) bool {
	if dst.namespace != appNamespace {
		return true // この世界で制御するのは pokecalc の Pod だけ
	}
	selected := false
	for _, pol := range w.policies {
		if !hasType(pol.types, "Ingress") || !pol.podSelector.matches(dst.labels) {
			continue
		}
		selected = true
		for _, rule := range pol.ingress {
			if ruleAllows(rule, src, dst, port, appNamespace) {
				return true
			}
		}
	}
	return !selected // どのポリシーにも選ばれない Pod は全許可(default-deny が無い状態)
}

func hasType(types []string, want string) bool {
	for _, ty := range types {
		if ty == want {
			return true
		}
	}
	return false
}

func ruleAllows(rule npIngressRule, src, dst npPod, port int, ns string) bool {
	fromOK := len(rule.from) == 0
	for _, peer := range rule.from {
		if peer.matches(src, ns) {
			fromOK = true
		}
	}
	if !fromOK {
		return false
	}
	if len(rule.ports) == 0 {
		return true
	}
	for _, pr := range rule.ports {
		if pr.matches(dst, port) {
			return true
		}
	}
	return false
}

// ---- 許可表・拒否表 ----

type npFlow struct {
	why  string
	from func(npWorld) npPod
	to   func(npWorld) npPod
	port int
}

func app(name string) func(npWorld) npPod { return func(w npWorld) npPod { return w.pods[name] } }
func ext(p npPod) func(npWorld) npPod     { return func(npWorld) npPod { return p } }

// 8080 で受ける pokecalc のサービス(balance・speed・judge は各レーンの Pod だが同じ namespace に入る)。
var httpServices = []string{"gateway", "calc", "pokedex", "balance", "speed", "judge"}

func allowedFlows() []npFlow {
	var f []npFlow
	add := func(why string, from, to func(npWorld) npPod, port int) {
		f = append(f, npFlow{why, from, to, port})
	}
	// 入口: Traefik(kube-system)→ Ingress の向き先。web・calc・pokedex は Ingress の向き先ではない。
	for _, n := range []string{"gateway", "balance", "speed", "judge"} {
		add("Traefik → "+n+"(Ingress)", ext(traefikPod), app(n), podPort)
	}
	// gateway の上流。balance・speed・judge は issue #284 の配線(GATEWAY_*_URL)が入ると必要になる。
	for _, n := range []string{"calc", "pokedex", "web", "balance", "speed", "judge"} {
		add("gateway → "+n, app("gateway"), app(n), podPort)
	}
	add("calc → pokedex(内部 API /internal/pokedex/master)", app("calc"), app("pokedex"), podPort)
	add("calc → nats", app("calc"), app("nats"), natsPort)
	add("judge → pokedex", app("judge"), app("pokedex"), podPort)
	add("judge → calc", app("judge"), app("calc"), podPort)
	// DB
	for _, n := range []string{"pokedex", "pokedex-migrate", "pokedex-import"} {
		add(n+" → mysql", app(n), app("mysql"), mysqlPort)
	}
	for _, n := range []string{"record-migrate", "team-migrate"} {
		add(n+" → tidb", app(n), ext(tidbServerPod), tidbPort)
	}
	add("tidb 内部(tidb → pd)", ext(tidbServerPod), ext(tidbPDPod), 2379)
	add("tidb 内部(pd → tidb)", ext(tidbPDPod), ext(tidbServerPod), 10080)
	// 監視: Prometheus(observability)が ServiceMonitor 対象の /metrics を取る
	for _, n := range httpServices {
		port := podPort
		if n == "gateway" {
			port = gatewayMetrics
		}
		add("Prometheus → "+n+" /metrics", ext(prometheusPod), app(n), port)
	}
	return f
}

func deniedFlows() []npFlow {
	var f []npFlow
	add := func(why string, from, to func(npWorld) npPod, port int) {
		f = append(f, npFlow{why, from, to, port})
	}
	// issue #240 の再現手順: web Pod から pokedex の /internal へ届かない(ほかの上流にも届かない)
	add("web → pokedex(/internal)", app("web"), app("pokedex"), podPort)
	add("web → calc", app("web"), app("calc"), podPort)
	add("web → mysql", app("web"), app("mysql"), mysqlPort)
	add("web → nats", app("web"), app("nats"), natsPort)
	// 最小権限: 必要な相手だけ
	add("calc → mysql", app("calc"), app("mysql"), mysqlPort)
	add("gateway → mysql", app("gateway"), app("mysql"), mysqlPort)
	add("judge → mysql", app("judge"), app("mysql"), mysqlPort)
	add("balance → pokedex", app("balance"), app("pokedex"), podPort)
	add("speed → pokedex", app("speed"), app("pokedex"), podPort)
	add("pokedex → calc", app("pokedex"), app("calc"), podPort)
	add("pokedex-import → pokedex(/internal)", app("pokedex-import"), app("pokedex"), podPort)
	add("pokedex-migrate → pokedex", app("pokedex-migrate"), app("pokedex"), podPort)
	add("gateway → nats", app("gateway"), app("nats"), natsPort)
	add("record-migrate → mysql", app("record-migrate"), app("mysql"), mysqlPort)
	add("pokedex → tidb", app("pokedex"), ext(tidbServerPod), tidbPort)
	// 他 namespace・外部
	add("default namespace の一時 Pod → pokedex(/internal)", ext(strayPod), app("pokedex"), podPort)
	add("default namespace の一時 Pod → calc(/metrics)", ext(strayPod), app("calc"), podPort)
	add("default namespace の一時 Pod → mysql", ext(strayPod), app("mysql"), mysqlPort)
	add("default namespace の一時 Pod → gateway", ext(strayPod), app("gateway"), podPort)
	add("kube-dns → pokedex", ext(corednsPod), app("pokedex"), podPort)
	add("observability の Grafana → calc(監視対象は Prometheus だけ)", ext(grafanaPod), app("calc"), podPort)
	add("observability の Prometheus → gateway の公開ポート(/metrics は専用ポートだけ。issue #216)", ext(prometheusPod), app("gateway"), podPort)
	add("Traefik → gateway のメトリクス専用ポート(公開入口から /metrics に届かない。issue #216)", ext(traefikPod), app("gateway"), gatewayMetrics)
	add("default namespace の一時 Pod → gateway のメトリクス専用ポート", ext(strayPod), app("gateway"), gatewayMetrics)
	add("observability の Prometheus → mysql(監視対象外のポート)", ext(prometheusPod), app("mysql"), mysqlPort)
	add("observability の Prometheus → web(ServiceMonitor が無い)", ext(prometheusPod), app("web"), podPort)
	add("default namespace の偽 Prometheus → calc(namespace まで見る)", ext(impostorPrometheusPod), app("calc"), podPort)
	add("pokecalc 内の偽 Traefik → gateway(namespace まで見る)", ext(impostorTraefikPod), app("gateway"), podPort)
	add("Traefik → pokedex(Ingress の向き先ではない)", ext(traefikPod), app("pokedex"), podPort)
	add("Traefik → mysql", ext(traefikPod), app("mysql"), mysqlPort)
	// ポートは Pod 側。Service の port 80 では通さない(DNAT 後に評価されるため)
	add("gateway → calc の 80 番(Service port。Pod は 8080 だけ)", app("gateway"), app("calc"), 80)
	add("gateway → mysql の 8080 番", app("gateway"), app("mysql"), podPort)
	return f
}

// ---- テスト ----

func TestNetworkPolicyBaseKustomizationIncludesNetworkPolicy(t *testing.T) { // AC-N5
	k := ReadKustomization(t, renderBaseDir)
	found := false
	for _, r := range k.Resources {
		if r == networkPolicyDir {
			found = true
		}
	}
	if !found {
		t.Errorf("%s/kustomization.yaml の resources に %q が無い(NetworkPolicy が描画に入らない)", renderBaseDir, networkPolicyDir)
	}
	// ディレクトリ側にも kustomization.yaml が要る(ReadKustomization は無ければ失敗する)。
	nk := ReadKustomization(t, renderBaseDir+"/"+networkPolicyDir)
	if len(nk.Resources) == 0 {
		t.Errorf("%s/%s/kustomization.yaml に resources が無い", renderBaseDir, networkPolicyDir)
	}
}

// AC-N1: default-deny(ingress)が base・local・cloud のどの描画にもある。
func TestNetworkPolicyDefaultDenyIngress(t *testing.T) {
	requireKubectl(t)
	for _, dir := range []string{renderBaseDir, renderLocalDir, renderCloudDir} {
		var denies []npPolicy
		for _, doc := range kustomizeRender(t, dir) {
			if asString(doc["kind"]) != "NetworkPolicy" {
				continue
			}
			p := parsePolicy(t, doc)
			if hasType(p.types, "Egress") {
				t.Errorf("%s: NetworkPolicy %s が Egress を含む。今回は ingress だけ(egress は kube-dns 等の許可が要るため別タスク。ADR-0132)", dir, p.name)
			}
			if len(p.podSelector.matchLabels) == 0 && len(p.podSelector.exprs) == 0 &&
				hasType(p.types, "Ingress") && len(p.ingress) == 0 {
				denies = append(denies, p)
			}
		}
		if len(denies) != 1 {
			t.Errorf("%s の描画に default-deny(podSelector: {}・policyTypes: [Ingress]・ingress 無し)が %d 本ある(1本だけにする)", dir, len(denies))
		}
	}
}

func TestNetworkPolicyAllowedFlows(t *testing.T) { // AC-N2
	requireKubectl(t)
	w := loadWorld(t)
	if len(w.policies) == 0 {
		t.Fatal("local overlay の描画に NetworkPolicy が1つも無い")
	}
	for _, p := range w.policies {
		if p.namespace != appNamespace {
			t.Errorf("NetworkPolicy %s の namespace が %q(%s にする)", p.name, p.namespace, appNamespace)
		}
	}
	for _, fl := range allowedFlows() {
		src, dst := fl.from(w), fl.to(w)
		if len(src.labels) == 0 || len(dst.labels) == 0 {
			t.Errorf("%s: 描画に送信元または宛先の Pod が見つからない(ラベル不一致か世界の組み立ての誤り)", fl.why)
			continue
		}
		if !w.allowed(src, dst, fl.port) {
			t.Errorf("通るべき通信が拒否される: %s(:%d)", fl.why, fl.port)
		}
	}
}

func TestNetworkPolicyDeniedFlows(t *testing.T) { // AC-N3
	requireKubectl(t)
	w := loadWorld(t)
	for _, fl := range deniedFlows() {
		src, dst := fl.from(w), fl.to(w)
		if len(src.labels) == 0 || len(dst.labels) == 0 {
			t.Errorf("%s: 描画に送信元または宛先の Pod が見つからない", fl.why)
			continue
		}
		if w.allowed(src, dst, fl.port) {
			t.Errorf("拒否されるべき通信が通る: %s(:%d)", fl.why, fl.port)
		}
	}
}

// AC-N4: Service の targetPort は、許可表のどれかの送信元から届く。許可の書き漏らしで孤立した Service を見つける。
func TestNetworkPolicyNoServiceIsUnreachable(t *testing.T) {
	requireKubectl(t)
	w := loadWorld(t)
	var sources []npPod
	for _, fl := range allowedFlows() {
		sources = append(sources, fl.from(w))
	}
	if len(w.services) == 0 {
		t.Fatal("描画に Service が1つも無い(検査が空振りしている)")
	}
	for _, svc := range w.services {
		dst, ok := w.pods[svc.selector[appNameLabel]]
		if !ok {
			continue // TidbCluster 等、テンプレートを持たないもの
		}
		port, err := strconv.Atoi(svc.targetPort)
		if err != nil {
			port = dst.ports[svc.targetPort]
		}
		reachable := false
		for _, src := range sources {
			if w.allowed(src, dst, port) {
				reachable = true
			}
		}
		if !reachable {
			t.Errorf("Service %s(targetPort %s → %d)へ許可された送信元から到達できない。許可の書き漏らし", svc.name, svc.targetPort, port)
		}
	}
}
