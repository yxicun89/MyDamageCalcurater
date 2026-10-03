package main

// refresher のビルドと k8s マニフェストの受け入れ条件(AC-K*)。YAML の文字列で確かめる
// (kustomize のビルドが通ることは `make wishlist-kustomize` で確かめる)。

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const baseDir = "../../../deploy/k8s/base"

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s を読めない: %v", path, err)
	}
	return string(b)
}

// envBlock は `- name: <name>` から次の `- name:` または `securityContext:` 等の直前までを返す(見つからなければ空)。
func envBlock(doc, name string) string {
	i := strings.Index(doc, "- name: "+name)
	if i < 0 {
		return ""
	}
	rest := doc[i+len("- name: "+name):]
	if j := strings.Index(rest, "- name:"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// AC-K1: イメージは api と refresher の両方のバイナリを含む。
func TestDockerfileBuildsRefresher(t *testing.T) {
	d := readFile(t, "../../Dockerfile")
	for _, want := range []string{"./cmd/api", "./cmd/refresher", "/wishlist-refresher"} {
		if !strings.Contains(d, want) {
			t.Errorf("Dockerfile に %q が無い", want)
		}
	}
}

// AC-K2: CronJob wishlist-refresher。毎日 03:00 JST、多重起動しない、refresher 専用イメージ(wishlist/refresher。AC-K4)の /wishlist-refresher を起動、
// securityContext は Deployment と同じ、DSN と appid(optional)を Secret から渡す。base の kustomization に載る。
func TestRefresherCronJob(t *testing.T) {
	k := readFile(t, filepath.Join(baseDir, "kustomization.yaml"))
	if !strings.Contains(k, "refresher-cronjob.yaml") {
		t.Error("kustomization.yaml に refresher-cronjob.yaml が無い")
	}
	c := readFile(t, filepath.Join(baseDir, "refresher-cronjob.yaml"))
	patterns := map[string]string{
		"kind":               `(?m)^kind: CronJob$`,
		"name":               `(?m)^  name: wishlist-refresher$`,
		"label":              `app\.kubernetes\.io/name: wishlist-refresher`,
		"schedule":           `(?m)^\s+schedule: "?0 3 \* \* \*"?\s*$`,
		"timeZone":           `(?m)^\s+timeZone: "?Asia/Tokyo"?\s*$`,
		"concurrencyPolicy":  `(?m)^\s+concurrencyPolicy: Forbid\s*$`,
		"image":              `(?m)^\s+image: wishlist/refresher:`, // Chromium 入りの専用イメージ(AC-K4。api のイメージには Chromium を載せない)
		"command":            `/wishlist-refresher`,
		"automount":          `automountServiceAccountToken: false`,
		"noPrivEsc":          `allowPrivilegeEscalation: false`,
		"dropAll":            `(?s)drop:\s*\n\s*- ALL`,
		"readOnlyRoot":       `readOnlyRootFilesystem: true`,
		"runAsNonRoot":       `runAsNonRoot: true`,
		"runAsUser":          `runAsUser: 65532`,
		"runAsGroup":         `runAsGroup: 65532`,
		"seccomp":            `(?s)seccompProfile:\s*\n\s*type: RuntimeDefault`,
		"resources.limits":   `(?s)limits:\s*\n\s*cpu:`,
		"resources.requests": `(?s)requests:\s*\n\s*cpu:`,
	}
	for name, p := range patterns {
		if !regexp.MustCompile(p).MatchString(c) {
			t.Errorf("CronJob に %s(/%s/)が無い", name, p)
		}
	}
	if b := envBlock(c, "WISHLIST_DATABASE_DSN"); !strings.Contains(b, "name: wishlist-api") || !strings.Contains(b, "key: database-dsn") {
		t.Errorf("WISHLIST_DATABASE_DSN が Secret wishlist-api の database-dsn でない: %q", b)
	}
	checkYahooEnv(t, "CronJob", c)
}

// AC-K3: api の Deployment にも WISHLIST_YAHOO_APPID を optional の secretKeyRef で渡す(無くても起動する)。
func TestAPIDeploymentYahooAppID(t *testing.T) {
	checkYahooEnv(t, "Deployment", readFile(t, filepath.Join(baseDir, "api-deployment.yaml")))
}

func checkYahooEnv(t *testing.T, what, doc string) {
	t.Helper()
	b := envBlock(doc, "WISHLIST_YAHOO_APPID")
	for _, want := range []string{"secretKeyRef:", "name: wishlist-api", "key: yahoo-appid", "optional: true"} {
		if !strings.Contains(b, want) {
			t.Errorf("%s の WISHLIST_YAHOO_APPID に %q が無い: %q", what, want, b)
		}
	}
}
