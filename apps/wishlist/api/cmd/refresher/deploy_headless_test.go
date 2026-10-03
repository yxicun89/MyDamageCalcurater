package main

// refresher 専用イメージ(Chromium 入り)と、それを使う CronJob の受け入れ条件(AC-K4〜K7)。YAML・Dockerfile・Makefile を文字列で確かめる。
// docker build が通ることは実装者が手元で確かめる(テストでは docker を使わない)。

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	refresherDockerfile = "../../Dockerfile.refresher"
	localOverlay        = "../../../deploy/k8s/overlays/local/kustomization.yaml"
	wishlistMakefile    = "../../../Makefile"
)

// AC-K4: Dockerfile.refresher。chromedp/headless-shell は数字だけのバージョンタグ+ダイジェストで固定(latest・タグだけは不可)。
// ビルド用の golang も api の Dockerfile と同じくダイジェスト固定。/wishlist-refresher だけを入れる(api のバイナリは入れない)。
// 非 root(65532)で動く。
func TestDockerfileRefresherImage(t *testing.T) {
	d := readFile(t, refresherDockerfile)
	patterns := map[string]string{
		"headless-shell のダイジェスト固定": `(?m)^FROM chromedp/headless-shell:\d+(\.\d+)+@sha256:[0-9a-f]{64}(\s|$)`,
		"golang のダイジェスト固定":         `(?m)^FROM golang:[0-9.]+-alpine@sha256:[0-9a-f]{64} AS build\s*$`,
		"refresher のビルド":           `go build .*-o /out/wishlist-refresher \./cmd/refresher`,
		"refresher のコピー":           `COPY --from=build /out/wishlist-refresher /wishlist-refresher`,
		"非 root":                   `(?m)^USER 65532(:65532)?\s*$`,
	}
	for name, p := range patterns {
		if !regexp.MustCompile(p).MatchString(d) {
			t.Errorf("Dockerfile.refresher に %s(/%s/)が無い", name, p)
		}
	}
	if strings.Contains(d, "/wishlist-api") || strings.Contains(d, "./cmd/api") {
		t.Error("Dockerfile.refresher に api のバイナリが入っている(refresher 専用)")
	}
	if regexp.MustCompile(`headless-shell:(latest|\d+(\.\d+)*)\s*$`).MatchString(d) {
		t.Error("headless-shell がダイジェストなしのタグだけ")
	}
}

// AC-K5: Chromium は refresher 専用(ユーザー決定 2026-10-03)。api の Dockerfile には載せない。
func TestAPIDockerfileHasNoChromium(t *testing.T) {
	var code []string // コメント行は除く
	for _, l := range strings.Split(readFile(t, "../../Dockerfile"), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "#") {
			code = append(code, l)
		}
	}
	d := strings.ToLower(strings.Join(code, "\n"))
	for _, bad := range []string{"headless-shell", "chromium", "chrome"} {
		if strings.Contains(d, bad) {
			t.Errorf("api の Dockerfile に %q がある(Chromium は refresher 専用)", bad)
		}
	}
}

// resourceBlock は `<name>:`(requests / limits)の直下のインデントされた行(次の同じ深さの行まで)。無ければ空。
func resourceBlock(c, name string) string {
	lines := strings.Split(c, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) != name+":" {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		var out []string
		for _, n := range lines[i+1:] {
			if strings.TrimSpace(n) == "" || len(n)-len(strings.TrimLeft(n, " ")) <= indent {
				break
			}
			out = append(out, n)
		}
		return strings.Join(out, "\n")
	}
	return ""
}

// AC-K6: CronJob は refresher 専用イメージを使い、Chromium の場所を環境変数で渡す(空なら headless は取らない)。
// /dev/shm は emptyDir(medium: Memory、sizeLimit 付き)。メモリは requests 256Mi・limits 1Gi(既定案)。
// 読み取り専用のルート・非 root・capabilities drop ALL は従来どおり(AC-K2)。
func TestRefresherCronJobHeadless(t *testing.T) {
	c := readFile(t, filepath.Join(baseDir, "refresher-cronjob.yaml"))
	if strings.Contains(c, "wishlist/api:") {
		t.Error("CronJob が api のイメージを使っている")
	}
	if b := envBlock(c, "WISHLIST_CHROMIUM_PATH"); !regexp.MustCompile(`value: "?/\S+`).MatchString(b) {
		t.Errorf("WISHLIST_CHROMIUM_PATH が絶対パスの value でない: %q", b)
	}
	if !regexp.MustCompile(`mountPath: /dev/shm\b`).MatchString(c) {
		t.Error("/dev/shm をマウントしていない")
	}
	if !regexp.MustCompile(`(?s)emptyDir:\s*\n\s*medium: Memory\s*\n\s*sizeLimit: \S+`).MatchString(c) {
		t.Error("emptyDir(medium: Memory、sizeLimit 付き)の共有メモリが無い")
	}
	if r := resourceBlock(c, "requests"); !regexp.MustCompile(`memory: 256Mi\b`).MatchString(r) {
		t.Errorf("requests の memory が 256Mi でない: %q", r)
	}
	if l := resourceBlock(c, "limits"); !regexp.MustCompile(`memory: 1Gi\b`).MatchString(l) {
		t.Errorf("limits の memory が 1Gi でない: %q", l)
	}
}

// AC-K7: ローカルの overlay と Makefile が refresher のイメージ(wishlist/refresher:local)を扱う。
//   - overlay の images に wishlist/refresher(newTag: local)
//   - wishlist-docker-build が Dockerfile.refresher で build し、wishlist-k3d-deploy が k3d image import に含める
func TestRefresherImageWiring(t *testing.T) {
	o := readFile(t, localOverlay)
	if !regexp.MustCompile(`(?s)name: wishlist/refresher\s*\n\s*newTag: local`).MatchString(o) {
		t.Error("overlays/local の images に wishlist/refresher(newTag: local)が無い")
	}
	m := readFile(t, wishlistMakefile)
	if !regexp.MustCompile(`(?m)^WISHLIST_REFRESHER_IMAGE \?= wishlist/refresher:local\s*$`).MatchString(m) {
		t.Error("Makefile に WISHLIST_REFRESHER_IMAGE ?= wishlist/refresher:local が無い")
	}
	target := func(name string) string {
		mm := regexp.MustCompile(`(?ms)^` + name + `:.*?(?:\n\n|\n#|\z)`).FindString(m)
		return mm
	}
	if b := target("wishlist-docker-build"); !regexp.MustCompile(`docker build .*-f \$\(WISHLIST_API_DIR\)/Dockerfile\.refresher .*\$\(WISHLIST_REFRESHER_IMAGE\)|docker build .*\$\(WISHLIST_REFRESHER_IMAGE\) .*-f \$\(WISHLIST_API_DIR\)/Dockerfile\.refresher`).MatchString(b) {
		t.Errorf("wishlist-docker-build が Dockerfile.refresher を build しない:\n%s", b)
	}
	if b := target("wishlist-k3d-deploy"); !regexp.MustCompile(`k3d image import [^\n]*\$\(WISHLIST_REFRESHER_IMAGE\)`).MatchString(b) {
		t.Errorf("wishlist-k3d-deploy の k3d image import に refresher のイメージが無い:\n%s", b)
	}
}
