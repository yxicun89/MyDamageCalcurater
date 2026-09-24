// @vitest-environment node
// P4-11: Web をコンテナで動かすための成果物(web/Dockerfile・web/nginx.conf・deploy/k8s の web・web/Makefile)を
// 文字列として読み、要の設定が入っていることを確かめる。kubectl・docker が無くても `make test` で走る
// (実際に描画・起動して確かめるのは `make web-kustomize`・`make web-e2e-container`・`make web-k3d-smoke`)。
// docs/coding-rules.md §4 の k8s の規則(非 root・読み取り専用のルート・権限を落とす・probe・resources)と、
// ADR-0300 §1(SPA のフォールバック)・ADR-0011 §6(application/wasm)に対応する。

import { existsSync, readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { localPath } from "../test/localPath";

const repoRoot = localPath("../../../", import.meta.url);

/** リポジトリルートからの相対パスのファイルを読む。コメント行(# から始まる行)は除く。無ければ分かる形で失敗させる。 */
function readConfig(relative: string): string {
  const path = `${repoRoot}${relative}`;
  if (!existsSync(path)) {
    throw new Error(`${relative} が無い(P4-11 で作る)`);
  }
  return readFileSync(path, "utf8")
    .split("\n")
    .filter((line) => !line.trim().startsWith("#"))
    .join("\n");
}

/** YAML の `key: value` 行(インデントは問わない)が1つ以上あるか。 */
function hasEntry(yaml: string, key: string, value: string): boolean {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const escapedValue = value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`^\\s*(?:- )?${escapedKey}:\\s*["']?${escapedValue}["']?\\s*$`, "m").test(yaml);
}

/** YAML の並び(`- item`)の要素。インデントは問わない。 */
function listItems(yaml: string): string[] {
  return [...yaml.matchAll(/^\s*-\s+(\S+)\s*$/gm)].map((match) => match[1] ?? "");
}

/** `key:` に続く、より深くインデントされた塊(最初に見つかったもの)。無ければ空文字。 */
function blockOf(yaml: string, key: string): string {
  const lines = yaml.split("\n");
  const start = lines.findIndex((line) => line.trim() === `${key}:`);
  if (start < 0) {
    return "";
  }
  const indent = (lines[start] ?? "").search(/\S/);
  const body: string[] = [];
  for (const line of lines.slice(start + 1)) {
    if (line.trim() !== "" && line.search(/\S/) <= indent) {
      break;
    }
    body.push(line);
  }
  return body.join("\n");
}

describe("deploy/k8s/base/web/deployment.yaml", () => {
  const deployment = () => readConfig("deploy/k8s/base/web/deployment.yaml");

  test("Deployment web で、イメージは pokecalc/web(latest を使わない)", () => {
    const yaml = deployment();
    expect(hasEntry(yaml, "kind", "Deployment")).toBe(true);
    expect(hasEntry(yaml, "name", "web")).toBe(true);
    expect(hasEntry(yaml, "app.kubernetes.io/name", "web")).toBe(true);
    const image = /^\s*image:\s*(\S+)\s*$/m.exec(yaml)?.[1] ?? "";
    expect(image).toMatch(/^pokecalc\/web:[\w.-]+$/);
    expect(image).not.toMatch(/:latest$/);
  });

  test("コンテナは 8080(名前 http)で待ち受ける", () => {
    const yaml = deployment();
    expect(hasEntry(yaml, "containerPort", "8080")).toBe(true);
    expect(hasEntry(yaml, "name", "http")).toBe(true);
  });

  test("非 root(イメージの UID 101)・権限昇格なし・全 capability を落とす・読み取り専用のルート・seccomp", () => {
    const yaml = deployment();
    expect(hasEntry(yaml, "runAsNonRoot", "true")).toBe(true);
    expect(hasEntry(yaml, "runAsUser", "101")).toBe(true);
    expect(hasEntry(yaml, "allowPrivilegeEscalation", "false")).toBe(true);
    expect(hasEntry(yaml, "readOnlyRootFilesystem", "true")).toBe(true);
    expect(listItems(blockOf(yaml, "drop"))).toContain("ALL");
    expect(hasEntry(yaml, "type", "RuntimeDefault")).toBe(true);
    expect(hasEntry(yaml, "automountServiceAccountToken", "false")).toBe(true);
    expect(yaml).not.toMatch(/privileged:\s*true/);
  });

  test("読み取り専用のルートでも nginx が書ける場所(/tmp)を emptyDir で渡す", () => {
    const yaml = deployment();
    expect(hasEntry(yaml, "mountPath", "/tmp")).toBe(true);
    expect(yaml).toMatch(/^\s*emptyDir:/m);
  });

  test("readiness・liveness の probe はどちらも /healthz を http ポートで見る", () => {
    const yaml = deployment();
    for (const probe of ["readinessProbe", "livenessProbe"]) {
      const block = blockOf(yaml, probe);
      expect(block, probe).not.toBe("");
      expect(hasEntry(block, "path", "/healthz"), probe).toBe(true);
      expect(hasEntry(block, "port", "http"), probe).toBe(true);
    }
  });

  test("resources の requests・limits に cpu と memory を持つ", () => {
    const resources = blockOf(deployment(), "resources");
    for (const kind of ["requests", "limits"]) {
      const block = blockOf(resources, kind);
      expect(block, kind).toMatch(/^\s*cpu:\s*\S+/m);
      expect(block, kind).toMatch(/^\s*memory:\s*\S+/m);
    }
  });
});

describe("deploy/k8s/base/web/service.yaml", () => {
  test("Service web は 80 で受けて http(8080)へ渡す(gateway の転送先。DECISIONS.md 2026-09-22)", () => {
    const yaml = readConfig("deploy/k8s/base/web/service.yaml");
    expect(hasEntry(yaml, "kind", "Service")).toBe(true);
    expect(hasEntry(yaml, "name", "web")).toBe(true);
    expect(hasEntry(yaml, "port", "80")).toBe(true);
    expect(hasEntry(yaml, "targetPort", "http") || hasEntry(yaml, "targetPort", "8080")).toBe(true);
    expect(hasEntry(blockOf(yaml, "selector"), "app.kubernetes.io/name", "web")).toBe(true);
  });
});

describe("Kustomize の組み立て", () => {
  test("base/web は deployment.yaml と service.yaml を持つ", () => {
    const items = listItems(blockOf(readConfig("deploy/k8s/base/web/kustomization.yaml"), "resources"));
    expect(items).toEqual(expect.arrayContaining(["deployment.yaml", "service.yaml"]));
  });

  test("base の kustomization が web を含む", () => {
    expect(listItems(blockOf(readConfig("deploy/k8s/base/kustomization.yaml"), "resources"))).toContain(
      "web",
    );
  });

  test("local の Component(local/web)は pokecalc/web を local タグにする", () => {
    const yaml = readConfig("deploy/k8s/overlays/local/web/kustomization.yaml");
    expect(hasEntry(yaml, "kind", "Component")).toBe(true);
    const images = blockOf(yaml, "images");
    expect(hasEntry(images, "name", "pokecalc/web")).toBe(true);
    expect(hasEntry(images, "newTag", "local")).toBe(true);
  });

  test("local overlay は Component web を含む", () => {
    expect(
      listItems(blockOf(readConfig("deploy/k8s/overlays/local/kustomization.yaml"), "components")),
    ).toContain("web");
  });

  test("local-web overlay は base/web と local/web だけを pokecalc に適用する(他レーンのリソースに触らない)", () => {
    const yaml = readConfig("deploy/k8s/overlays/local-web/kustomization.yaml");
    expect(hasEntry(yaml, "namespace", "pokecalc")).toBe(true);
    expect(listItems(blockOf(yaml, "resources"))).toEqual(["../../base/web"]);
    expect(listItems(blockOf(yaml, "components"))).toEqual(["../local/web"]);
  });
});

describe("web/Dockerfile", () => {
  const dockerfile = () => readConfig("web/Dockerfile");
  const fromLines = () => dockerfile().match(/^FROM\s+.+$/gim) ?? [];

  test("すべての FROM が版と digest で固定されている(latest を使わない)", () => {
    const lines = fromLines();
    expect(lines.length).toBeGreaterThanOrEqual(3);
    for (const line of lines) {
      expect(line).toMatch(/^FROM\s+[\w./-]+:[\w.-]+@sha256:[0-9a-f]{64}(\s+AS\s+\w+)?\s*$/i);
      expect(line).not.toMatch(/:latest[@\s]/i);
    }
  });

  test("Go で engine.wasm を作り、Node で vite build し、nginx-unprivileged で配る", () => {
    const lines = fromLines();
    expect(lines[0]).toMatch(/golang:/);
    expect(lines.some((line) => /node:/.test(line))).toBe(true);
    expect(lines[lines.length - 1]).toMatch(/nginxinc\/nginx-unprivileged:/);
    const text = dockerfile();
    expect(text).toMatch(/GOOS=js/);
    expect(text).toMatch(/GOARCH=wasm/);
    expect(text).toMatch(/wasm_exec\.js/);
    expect(text).toMatch(/npm ci/);
    expect(text).toMatch(/typechart\.json/);
  });

  test("Node の版は web/package.json の engines と揃える", () => {
    const pkg = JSON.parse(readFileSync(`${repoRoot}web/package.json`, "utf8")) as {
      engines: { node: string };
    };
    const nodeLine = fromLines().find((line) => /node:/.test(line)) ?? "";
    expect(nodeLine).toContain(`node:${pkg.engines.node}-`);
  });

  test("最終段は nginx.conf を入れ、数値 UID 101 で 8080 を公開する(Deployment の runAsUser と一致)", () => {
    const text = dockerfile();
    const finalStage = text.slice(text.lastIndexOf("\nFROM "));
    expect(finalStage).toMatch(/COPY\s+.*nginx\.conf/);
    expect(finalStage).toMatch(/^USER\s+101(:101)?\s*$/m);
    expect(finalStage).toMatch(/^EXPOSE\s+8080\s*$/m);
  });

  test(".dockerignore が相性表(testdata/golden/typechart.json)をビルドコンテキストから外していない", () => {
    const ignored = readConfig(".dockerignore")
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line !== "" && !line.startsWith("!"));
    expect(ignored.some((line) => line.startsWith("testdata"))).toBe(false);
  });
});

describe("web/nginx.conf", () => {
  const nginx = () => readConfig("web/nginx.conf");

  test("8080 で待ち受け、/api を転送しない(gateway の持ち物)", () => {
    const conf = nginx();
    expect(conf).toMatch(/listen\s+8080/);
    expect(conf).not.toMatch(/proxy_pass/);
    expect(conf).toMatch(/location\s+(\^~\s+)?\/api/);
  });

  test("SPA のフォールバック(未知のパスは /index.html)", () => {
    expect(nginx()).toMatch(/try_files\s+\$uri[^;]*\/index\.html\s*;/);
  });

  test(".wasm を application/wasm で返し、gzip の対象に含める", () => {
    const conf = nginx();
    expect(conf).toMatch(/application\/wasm\s+wasm\s*;/);
    expect(conf).toMatch(/gzip\s+on\s*;/);
    expect(/gzip_types[^;]*application\/wasm/.exec(conf)).not.toBeNull();
  });

  test("/static/* は immutable で長く、index.html・engine.wasm・wasm_exec.js は no-cache", () => {
    const conf = nginx();
    expect(conf).toMatch(/location\s+(\^~\s+)?\/static\/[\s\S]*?immutable/);
    expect(conf).toMatch(/no-cache/);
    for (const file of ["index\\.html", "engine\\.wasm", "wasm_exec\\.js"]) {
      expect(conf, file).toMatch(new RegExp(file));
    }
  });

  test("/healthz を持つ(probe 用)", () => {
    expect(nginx()).toMatch(/location\s+(=\s+)?\/healthz/);
  });
});

describe("web/Makefile のコンテナ用ターゲット", () => {
  const makefile = () => readFileSync(`${repoRoot}web/Makefile`, "utf8");

  test.each([
    "web-docker-build",
    "web-k3d-deploy",
    "web-k3d-open",
    "web-k3d-smoke",
    "web-kustomize",
    "web-e2e-container",
  ])("%s を持つ", (target) => {
    expect(makefile()).toMatch(new RegExp(`^${target}:`, "m"));
  });

  test("web-docker-build はリポジトリルートをコンテキストに web/Dockerfile から pokecalc/web:local を作る", () => {
    expect(makefile()).toMatch(/docker build -f web\/Dockerfile -t pokecalc\/web:local \./);
  });

  test("web-k3d-deploy は local-web overlay だけを apply する(共有の local overlay を丸ごと apply しない)", () => {
    const text = makefile();
    expect(text).toMatch(/k3d image import pokecalc\/web:local/);
    expect(text).toMatch(/kubectl apply -k deploy\/k8s\/overlays\/local-web\b/);
    expect(text).not.toMatch(/kubectl apply -k deploy\/k8s\/overlays\/local\s*$/m);
    expect(text).toMatch(/rollout status deployment\/web/);
  });

  test("web-k3d-smoke は web/scripts/k3d-smoke.sh を使い、web-e2e-container は先にイメージを作る", () => {
    const text = makefile();
    expect(text).toMatch(/web\/scripts\/k3d-smoke\.sh/);
    expect(text).toMatch(/^web-e2e-container:.*web-docker-build/m);
  });
});
