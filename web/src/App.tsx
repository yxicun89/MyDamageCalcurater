// アプリの最上位(P4-2)。ヘッダーと計算画面を出す。マスタは MasterSource(既定は架空の例データ)から、
// 計算は CalcEngine(既定は WASM 実装)から受け取り、テストでは差し替える(ADR-0300 §2・§3)。
// P4-5: ヘッダーに計算モード(オフライン = WASM / オンライン = API)の切り替えを置く(ADR-0301 §4)。
// engines(offline・online)を渡せ、選択中のモードの engine だけで計算する(自動フォールバックはしない)。
// 単一の engine を渡すと(既存の使い方のまま)両モードでその engine を使う。

import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from "react";
import "./App.css";
import { createApiEngine } from "./api/apiEngine";
import { apiBaseUrl } from "./api/config";
import { createClientIds, type ClientIds } from "./api/clientIds";
import { loadCalcMode, saveCalcMode, type CalcMode } from "./app/calcMode";
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

/** 計算モードの定義順(ラジオの表示順。既定のオフラインを先に出す)。 */
const CALC_MODE_ORDER: readonly CalcMode[] = ["offline", "online"];

/** 計算モードの表示名(appText の語をそのまま使う)。 */
function calcModeLabel(mode: CalcMode): string {
  return mode === "offline" ? appText.calcModeOfflineLabel : appText.calcModeOnlineLabel;
}

/** オフライン(WASM)・オンライン(API)、それぞれの計算の差し替え口(ADR-0301 §4)。 */
export interface CalcEngines {
  readonly offline: CalcEngine;
  readonly online: CalcEngine;
}

/** App の props(テストで engine・masterSource を差し替える。ADR-0300 §2・§3、ADR-0301 §4)。 */
export interface AppProps {
  /**
   * 計算の差し替え口。渡すと計算モードによらず常にこれを使う(engines を渡さないときの簡便な指定。
   * 既存の使い方のまま)。省くと既定の WASM 実装(オフライン)・API 実装(オンライン)を使う。
   */
  readonly engine?: CalcEngine;
  /** モードごとの計算の差し替え口。engine より優先する。 */
  readonly engines?: CalcEngines;
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
export function App({ engine, engines, masterSource = exampleMasterSource }: AppProps) {
  // 既定のオフライン(WASM)エンジンはマウント時に1回だけ作る(呼び出しのたびに作り直すと、計算のたびに
  // 読み込み状態がリセットされる)。createWasmEngine 自体は engine.wasm を読まない(初回の計算まで遅延)。
  const [fallbackOfflineEngine] = useState<CalcEngine>(() => createWasmEngine(browserWasmLoader()));
  // 端末 ID・セッション ID はマウント時に1回だけ作る(ADR-0301 §3: セッション ID はページを開くたびに新しく)。
  const [clientIds] = useState<ClientIds>(() => createClientIds());

  // setState は応答が届いたとき(.then のコールバック)だけで行う(react-hooks/set-state-in-effect)。
  // 読み込み中は load 未完了(null)のまま表す。
  const [masterLoad, setMasterLoad] = useState<MasterLoad | null>(null);

  // 既定のオンライン(API)エンジンは、natures(性格の一覧)が要るのでマスタの読み込みが終わってから作る
  // (ADR-0301 §2・§4)。createApiEngine 自体は fetch しない(初回の計算まで遅延。ADR-0300 §2 と同じ考え方)。
  const fallbackOnlineEngine = useMemo<CalcEngine | null>(() => {
    if (masterLoad === null || !masterLoad.ok) {
      return null;
    }
    return createApiEngine({
      baseUrl: apiBaseUrl(),
      fetch: globalThis.fetch.bind(globalThis),
      master: { natures: masterLoad.master.natures },
      ids: clientIds,
    });
  }, [masterLoad, clientIds]);

  // モードごとに使う engine を決める(ADR-0301 §4)。engines > engine(両モードに使う) > 既定の順。
  // 既定のオンラインは、マスタ読み込み前は使われない(masterLoad が ok になるまで下の画面を描画しない)ので
  // null のままでもよく、その間は offline のプレースホルダで埋める。
  const resolvedEngines: CalcEngines =
    engines ??
    (engine !== undefined
      ? { offline: engine, online: engine }
      : { offline: fallbackOfflineEngine, online: fallbackOnlineEngine ?? fallbackOfflineEngine });

  // 計算モード(オフライン = WASM / オンライン = API)。既定はオフラインで、選択は localStorage に覚える
  // (ADR-0301 §4)。マウント時に一度だけ読み、以後はこの state が正(他タブでの変更は追わない)。
  const [mode, setMode] = useState<CalcMode>(() => loadCalcMode());
  const resolvedEngine = mode === "online" ? resolvedEngines.online : resolvedEngines.offline;

  function selectMode(nextMode: CalcMode): void {
    setMode(nextMode);
    saveCalcMode(nextMode);
  }

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
        <CalcModeSelector value={mode} onChange={selectMode} />
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

interface CalcModeSelectorProps {
  readonly value: CalcMode;
  readonly onChange: (mode: CalcMode) => void;
}

/**
 * 計算モード(オフライン = WASM / オンライン = API)のラジオグループ(ヘッダー、ADR-0301 §4)。
 * CalcScreen.tsx の AttackerPresetSelector と同じ、ピル型ラジオグループの作法。
 */
function CalcModeSelector({ value, onChange }: CalcModeSelectorProps) {
  // ラジオの name は画面内で一意にする(同じ部品を複数置いてもグループが混ざらないように)。
  const groupName = useId();
  return (
    <div role="radiogroup" aria-label={appText.calcModeGroupLabel} className="app-mode">
      {CALC_MODE_ORDER.map((mode) => {
        const selected = mode === value;
        return (
          <label key={mode} className={`app-mode__option${selected ? " app-mode__option--selected" : ""}`}>
            <input
              type="radio"
              name={groupName}
              className="app-mode__input"
              checked={selected}
              onChange={() => {
                onChange(mode);
              }}
            />
            {calcModeLabel(mode)}
          </label>
        );
      })}
    </div>
  );
}
