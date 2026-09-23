package deploytest

// 私設サービスの境界(ADR-0210。issue #148)の静的検査。
//
// ユーザー決定(DECISIONS.md 2026-09-23): private overlay(tailnet)経由のみ。
// public LoadBalancer / Ingress を作らない。
//
// base の gateway Ingress は host 指定なし・TLS なしの HTTP Ingress で、k3d 同梱の Traefik で
// localhost:8080 から受けるための local 専用の経路(ADR-0203 §3・ADR-0012)。これをクラウドの
// 一般的な Ingress Controller に渡すと公開ロードバランサーになりうるので、cloud overlay の描画結果には
// 残さない(ADR-0210 §2)。ここではそれを2層で確かめる:
//
//   AC-B1: resources / components を辿った「残るオブジェクト」に Ingress が無い・LoadBalancer / NodePort の
//          Service が無い(kubectl 不要。常に走る)
//   AC-B2: `kubectl kustomize` の描画結果に同じものが無い(kubectl があるときだけ。OverlayObjects が
//          再現しない「削除以外の patch」の穴を塞ぐ)
//   AC-B3: base/gateway/ingress.yaml のコメントが「local 専用・cloud では使わない」と書いてある
//
// AC-B4(local の到達経路は変わらない)は既存の cmd/gateway.TestManifestGatewayIngress が担う。

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// publicServiceTypes は「クラウドで公開 IP を作りうる」Service の type(ADR-0210 §2)。
var publicServiceTypes = []string{"LoadBalancer", "NodePort"}

// AC-B1: cloud overlay から到達するオブジェクトに Ingress が無く、Service は ClusterIP(既定)だけ。
func TestCloudOverlayHasNoPublicEntrypoint(t *testing.T) {
	objs := OverlayObjects(t, CloudOverlayDir)
	if len(objs) == 0 {
		t.Fatalf("%s から1つもオブジェクトを辿れない(検査が空振りしている)", CloudOverlayDir)
	}

	for _, o := range objs {
		if o.Kind == "Ingress" {
			t.Errorf("%s の描画結果に Ingress %q が残る(%s 由来)。"+
				"private overlay 経由のみの方針では公開の入口を作らない(ADR-0210 §2)",
				CloudOverlayDir, o.Name, o.File)
		}
		switch o.Kind {
		case "Service":
			var svc Service
			o.Decode(t, &svc)
			for _, bad := range publicServiceTypes {
				if strings.EqualFold(svc.Spec.Type, bad) {
					t.Errorf("%s の Service %q が type: %s(%s 由来)。公開 IP を作らない(ADR-0210 §2)",
						CloudOverlayDir, o.Name, svc.Spec.Type, o.File)
				}
			}
			if len(svc.Spec.ExternalIPs) > 0 {
				t.Errorf("%s の Service %q が externalIPs を設定している(%s 由来)。"+
					"type に関わらずノード外部 IP へ直接公開しない(ADR-0210 §2)", CloudOverlayDir, o.Name, o.File)
			}
		case "Deployment":
			var dep Deployment
			o.Decode(t, &dep)
			podSpec := dep.Spec.Template.Spec
			if podSpec.HostNetwork {
				t.Errorf("%s の Deployment %q が hostNetwork: true(%s 由来)。"+
					"ノードのネットワークを直接使わない(ADR-0210 §2)", CloudOverlayDir, o.Name, o.File)
			}
			for _, c := range podSpec.Containers {
				for _, p := range c.Ports {
					if p.HostPort != 0 {
						t.Errorf("%s の Deployment %q のコンテナ %q が hostPort: %d(%s 由来)。"+
							"ノードのポートへ直接バインドしない(ADR-0210 §2)",
							CloudOverlayDir, o.Name, c.Name, p.HostPort, o.File)
					}
				}
			}
		}
	}

	// OverlayObjects は削除以外の patch を適用しないので、patch で公開の入口を後付けしていないことを本文でも見る。
	ingressKind := regexp.MustCompile(`(?m)^\s*kind:\s*Ingress\s*$`)
	for rel, body := range OverlayFiles(t, CloudOverlayDir) {
		if !strings.HasSuffix(rel, ".yaml") && !strings.HasSuffix(rel, ".yml") {
			continue
		}
		if strings.HasSuffix(rel, "/kustomization.yaml") {
			continue // resources / patches の並びは上の到達オブジェクトの検査で見ている
		}
		if ingressKind.MatchString(body) && !strings.Contains(body, "$patch: delete") {
			t.Errorf("%s が Ingress を定義している(削除の patch ではない)。ADR-0210 §2", rel)
		}
		for _, bad := range publicServiceTypes {
			if regexp.MustCompile(`(?m)^\s*type:\s*` + bad + `\s*$`).MatchString(body) {
				t.Errorf("%s が type: %s を設定している。公開 IP を作らない(ADR-0210 §2)", rel, bad)
			}
		}
		if regexp.MustCompile(`(?m)^\s*externalIPs:\s*$`).MatchString(body) {
			t.Errorf("%s が externalIPs を設定している。type に関わらずノード外部 IP へ直接公開しない(ADR-0210 §2)", rel)
		}
		if regexp.MustCompile(`(?m)^\s*hostNetwork:\s*true\s*$`).MatchString(body) {
			t.Errorf("%s が hostNetwork: true を設定している。ノードのネットワークを直接使わない(ADR-0210 §2)", rel)
		}
		if regexp.MustCompile(`(?m)^\s*hostPort:\s*[1-9][0-9]*\s*$`).MatchString(body) {
			t.Errorf("%s が hostPort を設定している。ノードのポートへ直接バインドしない(ADR-0210 §2)", rel)
		}
	}
}

// AC-B2: 実際の描画結果(kubectl kustomize)にも公開の入口が無い。kubectl が無ければ skip
// (smoke_test.go が curl の無い環境で skip する前例に合わせる。ADR-0203 §5)。
func TestCloudOverlayRenderHasNoPublicEntrypoint(t *testing.T) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl が無いので描画の検査を省略する(AC-B1 の構造の検査は走っている)")
	}
	out, err := exec.Command("kubectl", "kustomize", RepoPath(t, CloudOverlayDir)).Output()
	if err != nil {
		t.Fatalf("kubectl kustomize %s が失敗した: %v", CloudOverlayDir, err)
	}
	rendered := string(out)
	if strings.TrimSpace(rendered) == "" {
		t.Fatalf("%s の描画結果が空(検査が空振りしている)", CloudOverlayDir)
	}

	if regexp.MustCompile(`(?m)^kind:\s*Ingress\s*$`).MatchString(rendered) {
		t.Errorf("kubectl kustomize %s の描画結果に Ingress がある。"+
			"private overlay 経由のみの方針では公開の入口を作らない(ADR-0210 §2)", CloudOverlayDir)
	}
	for _, bad := range publicServiceTypes {
		if regexp.MustCompile(`(?m)^\s*type:\s*` + bad + `\s*$`).MatchString(rendered) {
			t.Errorf("kubectl kustomize %s の描画結果に type: %s がある。公開 IP を作らない(ADR-0210 §2)",
				CloudOverlayDir, bad)
		}
	}
	if regexp.MustCompile(`(?m)^\s*externalIPs:\s*$`).MatchString(rendered) {
		t.Errorf("kubectl kustomize %s の描画結果に externalIPs がある。"+
			"type に関わらずノード外部 IP へ直接公開しない(ADR-0210 §2)", CloudOverlayDir)
	}
	if regexp.MustCompile(`(?m)^\s*hostNetwork:\s*true\s*$`).MatchString(rendered) {
		t.Errorf("kubectl kustomize %s の描画結果に hostNetwork: true がある。"+
			"ノードのネットワークを直接使わない(ADR-0210 §2)", CloudOverlayDir)
	}
	if regexp.MustCompile(`(?m)^\s*hostPort:\s*[1-9][0-9]*\s*$`).MatchString(rendered) {
		t.Errorf("kubectl kustomize %s の描画結果に hostPort がある。"+
			"ノードのポートへ直接バインドしない(ADR-0210 §2)", CloudOverlayDir)
	}
}

// AC-B3: base の gateway Ingress は local(k3d / Traefik)専用であることが、ファイルの先頭コメントから読める。
// 読んだ人が cloud にそのまま持ち出さないようにするための文書検査(ADR-0210 §2.3。
// web_docs_test.go / pokedex_docs_test.go と同じパターン)。
func TestBaseGatewayIngressIsDocumentedAsLocalOnly(t *testing.T) {
	const file = BaseDir + "/gateway/ingress.yaml"
	body := OverlayFiles(t, BaseDir+"/gateway")[file]
	if body == "" {
		t.Fatalf("%s を読めない", file)
	}

	// コメント(先頭の "#" の行)だけを見る。マニフェストの中身ではなく「読んだ人への説明」を固定する。
	var comments strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			comments.WriteString(line + "\n")
		}
	}
	got := comments.String()

	for _, want := range []string{"local", "cloud", "ADR-0210"} {
		if !strings.Contains(got, want) {
			t.Errorf("%s のコメントに %q が無い。この Ingress が local(k3d / Traefik)専用で "+
				"cloud overlay では使わないこと、根拠が ADR-0210 であることを書く(ADR-0210 §2.3)。コメント:\n%s",
				file, want, got)
		}
	}
}
