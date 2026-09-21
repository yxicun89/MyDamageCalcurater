// P4-2: ブラウザの WasmLoader 実装(ADR-0016 §2)。`make wasm` が web/public/ に出す
// /wasm_exec.js を script 要素で読み、/engine.wasm を fetch して WebAssembly.instantiateStreaming で実体化する。

import type { WasmLoader } from "./wasmEngine";

const RUNTIME_PATH = "wasm_exec.js";
const WASM_PATH = "engine.wasm";

/**
 * デプロイ先のベースパス(vite の base、既定は "/")を踏まえた URL を作る。
 * サブパスへの配置(例: GitHub Pages のプロジェクトページ)でも /wasm_exec.js のような絶対パス
 * 決め打ちにならないよう、import.meta.env.BASE_URL から組み立てる。
 */
function assetUrl(path: string): string {
  return `${import.meta.env.BASE_URL}${path}`;
}

/** wasm_exec.js を script 要素で読み込む。globalThis.Go が既にあれば読み込まずに解決する。 */
function loadRuntime(): Promise<void> {
  if (globalThis.Go !== undefined) {
    return Promise.resolve();
  }
  const url = assetUrl(RUNTIME_PATH);
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = url;
    script.addEventListener("load", () => {
      resolve();
    });
    script.addEventListener("error", () => {
      reject(new Error(`${url} の読み込みに失敗した`));
    });
    document.head.append(script);
  });
}

/**
 * engine.wasm を fetch して実体化する。取得の失敗(404 など)は実体化せずに reject する。
 * WebAssembly.instantiateStreaming は応答の Content-Type が application/wasm でないと失敗することがある
 * (開発サーバーやプロキシが正しい MIME を返さない場合)。その場合は、応答の複製(body は一度しか
 * 読めないため、ストリーミングの前に複製しておく)から ArrayBuffer 経由で読み直す。
 */
async function instantiate(importObject: WebAssembly.Imports): Promise<WebAssembly.Instance> {
  const url = assetUrl(WASM_PATH);
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`${url} の取得に失敗した(status ${String(response.status)})`);
  }
  const fallbackResponse = response.clone();
  try {
    const { instance } = await WebAssembly.instantiateStreaming(response, importObject);
    return instance;
  } catch {
    const bytes = await fallbackResponse.arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, importObject);
    return instance;
  }
}

/** ブラウザ用の WasmLoader(createWasmEngine に渡す)。 */
export function browserWasmLoader(): WasmLoader {
  return { loadRuntime, instantiate };
}
