import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { webPath } from "../test/paths";
import { baseConfig } from "../../vite.config";

const read = (p: string) => readFileSync(webPath(p), "utf8");

// AC-PWA-01
describe("manifest.webmanifest", () => {
  const m = () => JSON.parse(read("public/manifest.webmanifest")) as Record<string, unknown>;
  it("名前・standalone・start_url・scope", () => {
    expect(m()).toMatchObject({
      name: "欲しいもの",
      display: "standalone",
      start_url: "/wishlist/",
      scope: "/wishlist/",
    });
  });
  it("アイコンが 1 つ以上あり、src のファイルが public に存在する(外部 URL を使わない)", () => {
    const icons = m().icons as { src: string }[];
    expect(icons.length).toBeGreaterThan(0);
    for (const i of icons) {
      expect(i.src).not.toMatch(/^https?:/);
      expect(existsSync(webPath("public", i.src.replace(/^\/wishlist\//, "")))).toBe(true);
    }
  });
  it("iPhone 用 apple-touch-icon(PNG 180x180)がある", () => {
    const buf = readFileSync(webPath("public", "apple-touch-icon.png"));
    expect(buf.subarray(1, 4).toString()).toBe("PNG");
    expect(buf.readUInt32BE(16)).toBe(180);
    expect(buf.readUInt32BE(20)).toBe(180);
  });
});

// AC-PWA-02
describe("index.html", () => {
  it("viewport-fit=cover・manifest・apple-touch-icon・日本語タイトル", () => {
    const html = read("index.html");
    expect(html).toMatch(/<meta[^>]+name="viewport"[^>]+viewport-fit=cover/);
    expect(html).toMatch(/<link[^>]+rel="manifest"[^>]+href="[^"]*manifest\.webmanifest"/);
    expect(html).toMatch(/<link[^>]+rel="apple-touch-icon"/);
    expect(html).toContain("<title>欲しいもの</title>");
  });
  it("安全領域(env(safe-area-inset-*))の CSS がある", () => {
    const css = cssFiles().join("\n");
    expect(css).toContain("env(safe-area-inset-bottom");
    expect(css).toContain("env(safe-area-inset-top");
  });
  it("常時動くアニメーション(infinite)を入れない", () => {
    expect(cssFiles().join("\n")).not.toMatch(/infinite/);
  });
});

function cssFiles(): string[] {
  const out: string[] = [];
  const walk = (dir: string) => {
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, e.name);
      if (e.isDirectory()) walk(full);
      else if (e.name.endsWith(".css")) out.push(readFileSync(full, "utf8"));
    }
  };
  walk(webPath("src"));
  return out;
}

describe("vite.config", () => {
  it("base は /wishlist/", () => {
    expect(baseConfig.base).toBe("/wishlist/");
  });
});

// AC-DEP-01: 配信設定(イメージのビルド自体は implementer が確かめる)。
describe("Dockerfile / nginx.conf", () => {
  it("非 root・8080・/healthz・SPA フォールバック・sw.js と index.html は no-cache", () => {
    const docker = read("Dockerfile");
    const nginx = read("nginx.conf");
    expect(docker).toMatch(/^USER\s+(?!root\b)\S+/m);
    expect(docker).toContain("8080");
    expect(nginx).toContain("listen 8080");
    expect(nginx).toContain("/healthz");
    expect(nginx).toMatch(/try_files[^;]*\/index\.html/);
    expect(nginx).toMatch(/sw\.js[\s\S]*?no-cache/);
    expect(nginx).toMatch(/index\.html[\s\S]*?no-cache/);
  });
});
