package deploytest

// overlay.go は「ある overlay を描画したら、どのオブジェクトが残るか」を kubectl 無しで辿る(ADR-0210 §5-1)。
// ADR-0203 §3 の方針(Kustomize を描画せず YAML を直接読む)を、overlay の resources / components の
// 再帰と `$patch: delete`(リソースそのものの削除)まで広げたもの。
//
// 再現するのはこれだけ:
//   - resources / components に並ぶエントリを、ディレクトリ(kustomization.yaml を持つ)なら再帰的に辿り、
//     ファイルなら YAML の全ドキュメントを読む
//   - patches のうち `$patch: delete` を持つもの(apiVersion / kind / metadata.name で対象を指す)を、
//     その kustomization が集めたオブジェクトから取り除く
//
// 再現しないもの(限界。ADR-0210 §5 はこの穴を本文検索と `kubectl kustomize` の検査で塞ぐ):
//   - 削除以外の patch(strategic merge / JSON 6902)。つまり patch でフィールドを足す・変える効果は見えない
//   - configMapGenerator / secretGenerator / images / namespace / 名前の接頭辞
//
// 本番コードから使わない(テストからだけ使う)。

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"bytes"

	"gopkg.in/yaml.v3"
)

// CloudOverlayDir はクラウド向け overlay(ADR-0210 §2)。
const CloudOverlayDir = "deploy/k8s/overlays/cloud"

// maxOverlayDepth は resources / components の再帰の上限(循環参照で止まらなくならないように)。
const maxOverlayDepth = 8

// OverlayObjects は dir(リポジトリ直下からの相対)の kustomization.yaml を起点に、
// resources / components を再帰的に辿って集めたオブジェクトから、`$patch: delete` で消えるものを除いて返す。
func OverlayObjects(t testing.TB, dir string) []Object {
	t.Helper()
	return resolveKustomizeDir(t, dir, 0)
}

func resolveKustomizeDir(t testing.TB, dir string, depth int) []Object {
	t.Helper()
	if depth > maxOverlayDepth {
		t.Fatalf("%s: resources / components の入れ子が深すぎる(%d 段。循環参照の疑い)", dir, depth)
	}
	k := ReadKustomization(t, dir)

	var objs []Object
	for _, entry := range append(append([]string{}, k.Resources...), k.Components...) {
		rel := path.Join(dir, entry)
		if hasKustomization(t, rel) {
			objs = append(objs, resolveKustomizeDir(t, rel, depth+1)...)
			continue
		}
		objs = append(objs, ReadObjects(t, rel)...)
	}

	for _, p := range k.Patches {
		for _, target := range deleteTargets(t, dir, p) {
			var kept []Object
			for _, o := range objs {
				if o.Kind == target.Kind && o.Name == target.Name {
					continue
				}
				kept = append(kept, o)
			}
			if len(kept) == len(objs) {
				t.Errorf("%s: `$patch: delete` の対象 %s/%s が resources に無い(削除の patch が何も消していない)",
					dir, target.Kind, target.Name)
			}
			objs = kept
		}
	}
	return objs
}

// hasKustomization は rel(リポジトリ直下からの相対)が kustomization.yaml を持つディレクトリかを返す。
func hasKustomization(t testing.TB, rel string) bool {
	t.Helper()
	info, err := os.Stat(RepoPath(t, rel+"/kustomization.yaml"))
	return err == nil && !info.IsDir()
}

// deleteTarget は `$patch: delete` が指すリソース。
type deleteTarget struct {
	Kind string
	Name string
}

// deleteTargets は patch 1件(path のファイル、またはインラインの patch)のうち、
// `$patch: delete` を持つドキュメントの対象を返す。
func deleteTargets(t testing.TB, dir string, p KustomizePatch) []deleteTarget {
	t.Helper()
	raw := p.Patch
	if p.Path != "" {
		b, err := os.ReadFile(RepoPath(t, path.Join(dir, p.Path)))
		if err != nil {
			t.Fatalf("%s/%s を読めない: %v", dir, p.Path, err)
		}
		raw = string(b)
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var targets []deleteTarget
	dec := yaml.NewDecoder(bytes.NewReader([]byte(raw)))
	for {
		var head struct {
			Patch    string `yaml:"$patch"`
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		err := dec.Decode(&head)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("%s の patch を解析できない: %v", dir, err)
		}
		if head.Patch != "delete" {
			continue
		}
		if head.Kind == "" || head.Metadata.Name == "" {
			t.Fatalf("%s: `$patch: delete` に kind / metadata.name が無い(対象を特定できない)", dir)
		}
		targets = append(targets, deleteTarget{Kind: head.Kind, Name: head.Metadata.Name})
	}
	return targets
}

// OverlayFiles は dir(リポジトリ直下からの相対)配下の全ファイルを「相対パス → 本文」で返す。
// OverlayObjects が再現しない patch(削除以外)の効果を本文検索で見るために使う(ADR-0210 §5-1)。
func OverlayFiles(t testing.TB, dir string) map[string]string {
	t.Helper()
	root := RepoPath(t, dir)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("%s を読めない: %v", dir, err)
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			for rel, body := range OverlayFiles(t, dir+"/"+e.Name()) {
				files[rel] = body
			}
			continue
		}
		b, err := os.ReadFile(path.Join(root, e.Name()))
		if err != nil {
			t.Fatalf("%s/%s を読めない: %v", dir, e.Name(), err)
		}
		files[dir+"/"+e.Name()] = string(b)
	}
	return files
}
