// P4-2: ブラウザの読み込み口 browserWasmLoader(ADR-0300 §2)。
// `make wasm` が web/public/ に出す /wasm_exec.js を script 要素で読み、/engine.wasm を fetch して
// WebAssembly.instantiateStreaming で実体化する。本物のファイルは読まず、DOM と fetch を観察する。

import { afterEach, expect, test, vi } from "vitest";
import { browserWasmLoader } from "./browserWasmLoader";

// BASE_URL(vite の base、既定は "/")から組み立てる実装(browserWasmLoader.ts)と同じ式で作る。
// ハードコードした絶対パスと比較すると、サブパス配置への対応が壊れても検出できない。
const runtimeUrl = `${import.meta.env.BASE_URL}wasm_exec.js`;
const wasmUrl = `${import.meta.env.BASE_URL}engine.wasm`;

function runtimeScript(): HTMLScriptElement | null {
  return document.querySelector<HTMLScriptElement>(`script[src="${runtimeUrl}"]`);
}

afterEach(() => {
  document.querySelectorAll("script").forEach((script) => {
    script.remove();
  });
  Reflect.deleteProperty(globalThis, "Go");
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

test("loadRuntime は /wasm_exec.js の script 要素を足し、load で解決する", async () => {
  const loading = browserWasmLoader().loadRuntime();
  const script = runtimeScript();
  expect(script).not.toBeNull();
  script?.dispatchEvent(new Event("load"));
  await expect(loading).resolves.toBeUndefined();
});

test("loadRuntime は script の error で reject する", async () => {
  const loading = browserWasmLoader().loadRuntime();
  runtimeScript()?.dispatchEvent(new Event("error"));
  await expect(loading).rejects.toThrow();
});

test("globalThis.Go が既にあれば script を足さずに解決する", async () => {
  (globalThis as Record<string, unknown>).Go = () => undefined;
  await expect(browserWasmLoader().loadRuntime()).resolves.toBeUndefined();
  expect(runtimeScript()).toBeNull();
});

/** fetch の応答の最小限の fake。clone() は自分自身を返す(instantiate が実体化の前に複製するため)。 */
function fakeResponse(overrides: Partial<Response>): Response {
  const response = {} as Response;
  Object.assign(response, { clone: () => response }, overrides);
  return response;
}

test("instantiate は /engine.wasm を fetch し、instantiateStreaming に importObject と一緒に渡す", async () => {
  const response = fakeResponse({ ok: true, status: 200 });
  const fetchMock = vi.fn(() => Promise.resolve(response));
  vi.stubGlobal("fetch", fetchMock);
  const instance = { exports: {} } as WebAssembly.Instance;
  const streaming = vi.spyOn(WebAssembly, "instantiateStreaming").mockResolvedValue({ instance, module: {} });
  const importObject = { go: {} };

  await expect(browserWasmLoader().instantiate(importObject)).resolves.toBe(instance);
  expect(fetchMock).toHaveBeenCalledWith(wasmUrl);
  expect(streaming).toHaveBeenCalledTimes(1);
  const [source, imports] = streaming.mock.calls[0] ?? [];
  await expect(Promise.resolve(source)).resolves.toBe(response);
  expect(imports).toBe(importObject);
});

test("instantiate は /engine.wasm の取得が失敗(404 など)したら reject し、実体化しない", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(fakeResponse({ ok: false, status: 404 }))),
  );
  const streaming = vi.spyOn(WebAssembly, "instantiateStreaming");
  await expect(browserWasmLoader().instantiate({ go: {} })).rejects.toThrow();
  expect(streaming).not.toHaveBeenCalled();
});

test("instantiateStreaming が失敗(MIME 不一致など)したら、複製した応答の arrayBuffer から実体化し直す", async () => {
  const bytes = new ArrayBuffer(8);
  const response = fakeResponse({ ok: true, status: 200, arrayBuffer: () => Promise.resolve(bytes) });
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(response)),
  );
  const instance = { exports: {} } as WebAssembly.Instance;
  const streaming = vi
    .spyOn(WebAssembly, "instantiateStreaming")
    .mockRejectedValue(new Error("応答の Content-Type が application/wasm でない(fake)"));
  // WebAssembly.instantiate はオーバーロード(BufferSource 版 / Module 版で戻り値の形が違う)を持ち、
  // vi.spyOn は片方(Module 版、戻り値が Instance 単体)の型で解決してしまうため、実装が使う
  // BufferSource 版の戻り値({instance, module})を明示してキャストする。
  const instantiateMock = vi
    .spyOn(WebAssembly, "instantiate")
    .mockImplementation((() =>
      Promise.resolve({ instance, module: {} })) as unknown as typeof WebAssembly.instantiate);
  const importObject = { go: {} };

  await expect(browserWasmLoader().instantiate(importObject)).resolves.toBe(instance);
  expect(streaming).toHaveBeenCalledTimes(1);
  expect(instantiateMock).toHaveBeenCalledWith(bytes, importObject);
});
