// テスト専用: Node で本物の engine.wasm(`make wasm` の成果物)を読む WasmLoader(ADR-0016 §8 の WASM 結合テスト)。
// ブラウザの browserWasmLoader と同じ2段階(wasm_exec.js の実行 → engine.wasm のインスタンス化)を、ファイルから行う。
// 前提のファイルが無ければスキップせず失敗させる(requireWasmArtifacts。CLAUDE.md、ADR-0016 §8)。

import { existsSync, readFileSync } from "node:fs";
import vm from "node:vm";
import type { WasmLoader } from "../engine/wasmEngine";
import { localPath } from "./localPath";

export const wasmExecPath = localPath("../../public/wasm_exec.js", import.meta.url);
export const wasmPath = localPath("../../public/engine.wasm", import.meta.url);

/** engine.wasm・wasm_exec.js が無ければ例外にする(このテストはスキップしない)。 */
export function requireWasmArtifacts(): void {
  for (const path of [wasmExecPath, wasmPath]) {
    if (!existsSync(path)) {
      throw new Error(`${path} が無い。先に \`make wasm\` を実行すること(このテストはスキップしない)`);
    }
  }
}

/** Node 用の読み込み口。 */
export function fileWasmLoader(): WasmLoader {
  return {
    loadRuntime() {
      new vm.Script(readFileSync(wasmExecPath, "utf8"), { filename: wasmExecPath }).runInThisContext();
      return Promise.resolve();
    },
    async instantiate(importObject) {
      const { instance } = await WebAssembly.instantiate(readFileSync(wasmPath), importObject);
      return instance;
    },
  };
}
