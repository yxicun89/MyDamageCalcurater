package main

// issue #284: speed は gateway(/api/speed/*)経由だけで届く。Traefik から直接届く Ingress は持たない
// (ADR-0414)。k8s マニフェストの静的検査(kubectl を使わず、ファイルを読む)。

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var ingressKind = regexp.MustCompile(`(?m)^kind:\s*Ingress\s*$`)

func TestManifestHasNoIngress(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "k8s")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return err
		}
		if filepath.Base(p) == "ingress.yaml" {
			t.Errorf("%s がある(speed は gateway 経由。Ingress を置かない)", p)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if ingressKind.Match(b) {
			t.Errorf("%s に kind: Ingress がある", p)
		}
		if filepath.Base(p) == "kustomization.yaml" {
			for _, line := range strings.Split(string(b), "\n") {
				// コメントは対象外(NetworkPolicy の allow-mysql-ingress.yaml への言及があるため)。
				if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "ingress") {
					t.Errorf("%s が ingress を参照している: %q", p, line)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
