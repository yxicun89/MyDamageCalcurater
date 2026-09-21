// アンビエント型宣言(P4-2)。
// - "@typechart": vite.config.ts の別名(testdata/golden/typechart.json、ADR-0016 §3)。
// - Go / pokecalc / pokecalcReady: wasm_exec.js(Go ランタイム)と engine/cmd/wasm が
//   登録するグローバル(ADR-0011 §2)。値は web/src/engine/wasmEngine.ts だけが読む。
//
// この2つを1ファイルにまとめる理由: import/export を持たせるとこのファイルが「モジュール」扱いになり、
// トップレベルの `declare module "@typechart"` がこのファイル内だけのローカルな拡張になってしまう
// (グローバルなアンビエント宣言として認識されない)。import/export を持たない「スクリプト」のままにして、
// トップレベルの宣言をすべてグローバルにする。

declare module "@typechart" {
  const data: unknown;
  export default data;
}

/** engine/cmd/wasm が globalThis.pokecalc に登録する3関数(ADR-0011 §2)。引数・戻り値は JSON 文字列。 */
interface PokecalcApi {
  calc(requestJSON: string): string;
  calcBulk(requestJSON: string): string;
  calcReverse(requestJSON: string): string;
}

/** wasm_exec.js が生成する Go インスタンス(実際に使うのは importObject と run だけ)。 */
interface GoInstance {
  readonly importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

/** wasm_exec.js が定義する Go コンストラクタ。 */
interface GoConstructor {
  new (): GoInstance;
}

// globalThis への値の宣言には var が要る(let/const はアンビエント宣言として認識されない)。
/* eslint-disable no-var */
declare var Go: GoConstructor | undefined;
declare var pokecalc: PokecalcApi | undefined;
declare var pokecalcReady: boolean | undefined;
/* eslint-enable no-var */
