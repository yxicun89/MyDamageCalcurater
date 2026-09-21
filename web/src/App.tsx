// アプリの最上位(P4-2)。ヘッダーと計算画面を出す。マスタは MasterSource(既定は架空の例データ)から、
// 計算は CalcEngine(既定は WASM 実装)から受け取り、テストでは差し替える(ADR-0300 §2・§3)。

import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import "./App.css";
import { browserWasmLoader } from "./engine/browserWasmLoader";
import type { CalcEngine } from "./engine/types";
import { createWasmEngine } from "./engine/wasmEngine";
import { appText } from "./i18n/ja";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource } from "./master/types";
import { CalcScreen } from "./screens/CalcScreen";
import { ReverseScreen } from "./screens/ReverseScreen";

/** 計算・逆算の切り替えタブ(P4-4、ADR-0300 §7)。既定は計算。 */
type ScreenTab = "calc" | "reverse";

/** タブの定義順(ロービング tabIndex・矢印キーの移動順。WAI-ARIA Authoring Practices の Tabs パターン)。 */
const TAB_ORDER: readonly ScreenTab[] = ["calc", "reverse"];

/** App の props(テストで engine・masterSource を差し替える。ADR-0300 §2・§3)。 */
export interface AppProps {
  /** 計算の差し替え口。省くと WASM 実装(ブラウザから wasm_exec.js / engine.wasm を読む)。 */
  readonly engine?: CalcEngine;
  /** マスタの取得口。省くと架空の例データ(ADR-0300 §3)。 */
  readonly masterSource?: MasterSource;
}

/** masterSource.load() の結果(成功/失敗のどちらか)。読み込み中は state を持たず null のまま表す。 */
type MasterLoad =
  { readonly ok: true; readonly master: MasterData } | { readonly ok: false; readonly error: Error };

/** タブの表示名(appText の語をそのまま使う)。 */
function tabLabel(tabId: ScreenTab): string {
  return tabId === "calc" ? appText.calcTabLabel : appText.reverseTabLabel;
}

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
  // 計算・逆算の切り替え(P4-4)。既定は計算。どちらの画面も同じ engine・master を使う。
  const [tab, setTab] = useState<ScreenTab>("calc");

  // タブ・タブパネルの id(WAI-ARIA Authoring Practices の Tabs パターン: tab の aria-controls が
  // panel の id を指し、panel の aria-labelledby が選択中の tab の id を指す)。パネルは1つだけ描画し
  // (非選択側の画面は DOM から外す。App.test.tsx が別タブの combobox が無いことを確かめる)、
  // 中身を選択中のタブで入れ替える。
  const idBase = useId();
  const tabElementId = (id: ScreenTab): string => `${idBase}-tab-${id}`;
  const panelId = `${idBase}-panel`;

  // ロービング tabIndex(選択中のタブだけ 0、他は -1)のフォーカス移動先を、クリックでなく
  // キーボードで選んだときに参照する(WAI-ARIA Authoring Practices「automatic activation」)。
  const tabRefs = useRef<Partial<Record<ScreenTab, HTMLButtonElement>>>({});

  function selectTab(nextTab: ScreenTab): void {
    setTab(nextTab);
  }

  /** ArrowLeft/ArrowRight/Home/End でタブの選択とフォーカスを移動する(automatic activation)。 */
  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number): void {
    let nextIndex: number;
    switch (event.key) {
      case "ArrowRight":
        nextIndex = (index + 1) % TAB_ORDER.length;
        break;
      case "ArrowLeft":
        nextIndex = (index - 1 + TAB_ORDER.length) % TAB_ORDER.length;
        break;
      case "Home":
        nextIndex = 0;
        break;
      case "End":
        nextIndex = TAB_ORDER.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    const nextTab = TAB_ORDER[nextIndex];
    if (nextTab === undefined) {
      return;
    }
    selectTab(nextTab);
    tabRefs.current[nextTab]?.focus();
  }

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
          <div className="app-tabs">
            <div role="tablist" aria-label={appText.tabsLabel} className="app-tabs__list">
              {TAB_ORDER.map((id, index) => {
                const selected = tab === id;
                return (
                  <button
                    key={id}
                    type="button"
                    role="tab"
                    id={tabElementId(id)}
                    aria-selected={selected}
                    aria-controls={panelId}
                    // ロービング tabIndex(WAI-ARIA Authoring Practices): 選択中のタブだけ Tab キーで
                    // 止まり、他のタブは矢印キー(handleTabKeyDown)でだけ選ぶ。
                    tabIndex={selected ? 0 : -1}
                    ref={(element) => {
                      tabRefs.current[id] = element ?? undefined;
                    }}
                    className="app-tabs__tab"
                    onClick={() => {
                      selectTab(id);
                    }}
                    onKeyDown={(event) => {
                      handleTabKeyDown(event, index);
                    }}
                  >
                    {tabLabel(id)}
                  </button>
                );
              })}
            </div>
            <div role="tabpanel" id={panelId} aria-labelledby={tabElementId(tab)} className="app-tabs__panel">
              {tab === "calc" ? (
                <CalcScreen engine={resolvedEngine} master={masterLoad.master} />
              ) : (
                <ReverseScreen engine={resolvedEngine} master={masterLoad.master} />
              )}
            </div>
          </div>
        )}
      </main>
    </>
  );
}
