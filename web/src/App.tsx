// アプリの最上位(P4-2)。ヘッダーと計算画面を出す。マスタは MasterSource(既定は架空の例データ)から、
// 計算は CalcEngine(既定は WASM 実装)から受け取り、テストでは差し替える(ADR-0016 §2・§3)。

import { useEffect, useState } from "react";
import "./App.css";
import { browserWasmLoader } from "./engine/browserWasmLoader";
import type { CalcEngine } from "./engine/types";
import { createWasmEngine } from "./engine/wasmEngine";
import { appText } from "./i18n/ja";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource } from "./master/types";
import { CalcScreen } from "./screens/CalcScreen";

/** App の props(テストで engine・masterSource を差し替える。ADR-0016 §2・§3)。 */
export interface AppProps {
  /** 計算の差し替え口。省くと WASM 実装(ブラウザから wasm_exec.js / engine.wasm を読む)。 */
  readonly engine?: CalcEngine;
  /** マスタの取得口。省くと架空の例データ(ADR-0016 §3)。 */
  readonly masterSource?: MasterSource;
}

/** masterSource.load() の結果(成功/失敗のどちらか)。読み込み中は state を持たず null のまま表す。 */
type MasterLoad =
  { readonly ok: true; readonly master: MasterData } | { readonly ok: false; readonly error: Error };

/**
 * アプリの最上位。ヘッダーと計算画面(CalcScreen)を出す。マスタを読み込むまでは「読み込み中」、
 * 読み込みに失敗したら role=alert で知らせる。
 */
export function App({ engine, masterSource = exampleMasterSource }: AppProps) {
  // 既定の WASM エンジンはマウント時に1回だけ作る(呼び出しのたびに作り直すと、計算のたびに
  // 読み込み状態がリセットされる)。createWasmEngine 自体は engine.wasm を読まない(初回の計算まで遅延)。
  const [fallbackEngine] = useState<CalcEngine>(() => createWasmEngine(browserWasmLoader()));
  const resolvedEngine = engine ?? fallbackEngine;

  // setState は応答が届いたとき(.then のコールバック)だけで行う(react-hooks/set-state-in-effect)。
  // 読み込み中は load 未完了(null)のまま表す。
  const [masterLoad, setMasterLoad] = useState<MasterLoad | null>(null);

  useEffect(() => {
    let cancelled = false;
    masterSource.load().then(
      (master) => {
        if (!cancelled) {
          setMasterLoad({ ok: true, master });
        }
      },
      (error: unknown) => {
        if (!cancelled) {
          setMasterLoad({ ok: false, error: error instanceof Error ? error : new Error(String(error)) });
        }
      },
    );
    return () => {
      cancelled = true;
    };
  }, [masterSource]);

  return (
    <>
      {/* main の外に置く: main の内側だと header は banner ランドマークにならない(HTML-AAM)。 */}
      <header className="app-header">
        <h1>{appText.title}</h1>
      </header>
      <main className="app-main">
        {masterLoad === null && <p>{appText.loading}</p>}
        {masterLoad !== null && !masterLoad.ok && <p role="alert">{appText.masterLoadError}</p>}
        {masterLoad !== null && masterLoad.ok && (
          <CalcScreen engine={resolvedEngine} master={masterLoad.master} />
        )}
      </main>
    </>
  );
}
