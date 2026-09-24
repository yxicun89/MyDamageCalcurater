// アプリの最上位(P4-2)。ヘッダーと計算画面を出す。マスタは MasterSource(既定は架空の例データ)から、
// 計算は CalcEngine(既定は WASM 実装)から受け取り、テストでは差し替える(ADR-0300 §2・§3)。
// P4-5: ヘッダーに計算モード(オフライン = WASM / オンライン = API)の切り替えを置く(ADR-0301 §4)。
// engines(offline・online)を渡せ、選択中のモードの engine だけで計算する(自動フォールバックはしない)。
// 単一の engine を渡すと(既存の使い方のまま)両モードでその engine を使う。
// P4-10: タブの選択は URL(History API)と連動する(ADR-0300 §1: ルーターのライブラリは入れない)。
// 画面 ID・パス・タブの表示名・文書タイトルの対応は app/routes.ts の SCREEN_ROUTES を正とする。

import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from "react";
import "./App.css";
import { createApiEngine } from "./api/apiEngine";
import { createBalanceClient } from "./api/balanceClient";
import { apiBaseUrl } from "./api/config";
import { createClientIds, type ClientIds } from "./api/clientIds";
import { loadCalcMode, saveCalcMode, type CalcMode } from "./app/calcMode";
import {
  DEFAULT_SCREEN,
  SCREEN_ROUTES,
  documentTitle,
  pathForScreen,
  screenFromPath,
  screenLabel,
  type ScreenId,
} from "./app/routes";
import { browserWasmLoader } from "./engine/browserWasmLoader";
import type { CalcEngine } from "./engine/types";
import { createWasmEngine } from "./engine/wasmEngine";
import { appText } from "./i18n/ja";
import { createJudgeClient } from "./judge/judgeClient";
import { isSearchableMasterSource } from "./master/capabilities";
import { exampleMasterSource } from "./master/exampleSource";
import { createSpeedClient } from "./speed/speedClient";
import type { MasterData, MasterSource, MasterSources } from "./master/types";
import { SCREEN_COMPONENTS } from "./app/screens";

/** タブの定義順(ロービング tabIndex・矢印キーの移動順。WAI-ARIA Authoring Practices の Tabs パターン)。 */
const TAB_ORDER: readonly ScreenId[] = SCREEN_ROUTES.map((route) => route.id);

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
  /** マスタの取得口。省くと架空の例データ(ADR-0300 §3)。両モードで同じマスタを使う。 */
  readonly masterSource?: MasterSource;
  /**
   * P4-16(ADR-0304 §追記): 計算モードごとのマスタの取得口。masterSource より優先する。
   * 端末 ID・セッション ID は App が持つ(ADR-0301 §3)ので、オンラインのマスタを組み立てられるよう
   * ClientIds を受け取る関数で渡す。マウント時に1回だけ呼ぶ(以後この props の同一性は見ない)。
   * 省くと両モードとも masterSource(既定は架空の例データ)。本番の組み立ては main.tsx が渡す。
   */
  readonly masterSources?: (ids: ClientIds) => MasterSources;
}

/** masterSource.load() の結果(成功/失敗のどちらか)。読み込み中は state を持たず null のまま表す。 */
type MasterLoadResult =
  { readonly ok: true; readonly master: MasterData } | { readonly ok: false; readonly error: Error };

/**
 * masterLoad の state(P4-16、ADR-0304 §追記 A-6)。どの取得口(source)の結果かを持たせ、
 * 現在選ばれている取得口と一致しないとき(モードを切り替えた直後など)は読み込み中として扱う
 * (`useEffect` の中で setState せず、レンダー時にこの一致を見て導出する)。
 */
interface MasterLoadState {
  readonly source: MasterSource;
  readonly result: MasterLoadResult;
}

/**
 * アプリの最上位。ヘッダーと計算画面(CalcScreen)を出す。マスタを読み込むまでは「読み込み中」、
 * 読み込みに失敗したら role=alert で知らせる。
 */
export function App({ engine, engines, masterSource = exampleMasterSource, masterSources }: AppProps) {
  // 既定のオフライン(WASM)エンジンはマウント時に1回だけ作る(呼び出しのたびに作り直すと、計算のたびに
  // 読み込み状態がリセットされる)。createWasmEngine 自体は engine.wasm を読まない(初回の計算まで遅延)。
  const [fallbackOfflineEngine] = useState<CalcEngine>(() => createWasmEngine(browserWasmLoader()));
  // 端末 ID・セッション ID はマウント時に1回だけ作る(ADR-0301 §3: セッション ID はページを開くたびに新しく)。
  const [clientIds] = useState<ClientIds>(() => createClientIds());
  // P4-12a: balance API のクライアント(ADR-0303 §5)。計算と同じ基点 URL・端末 ID・セッション ID を使う。
  // createBalanceClient 自体は fetch しない(メンバーを選ぶまで呼ばれない。BalanceScreen.tsx)。
  const [balanceClient] = useState(() =>
    createBalanceClient({ baseUrl: apiBaseUrl(), fetch: globalThis.fetch.bind(globalThis), ids: clientIds }),
  );
  // SP3: speed API のクライアント(ADR-0604 §2・§3)。balance と同じ基点 URL・端末 ID・セッション ID を使う。
  // createSpeedClient 自体は fetch しない(素早さのタブを開くまで呼ばれない。speed/SpeedScreen.tsx)。
  const [speedClient] = useState(() =>
    createSpeedClient({ baseUrl: apiBaseUrl(), fetch: globalThis.fetch.bind(globalThis), ids: clientIds }),
  );
  // JD5: judge API のクライアント(ADR-0705 §1・§3)。同じ基点 URL・端末 ID・セッション ID を使う。
  // createJudgeClient 自体は fetch しない(判定のタブを開くだけでは呼ばれない。judge/JudgeScreen.tsx)。
  const [judgeClient] = useState(() =>
    createJudgeClient({ baseUrl: apiBaseUrl(), fetch: globalThis.fetch.bind(globalThis), ids: clientIds }),
  );

  // 計算モード(オフライン = WASM / オンライン = API)。既定はオフラインで、選択は localStorage に覚える
  // (ADR-0301 §4)。マウント時に一度だけ読み、以後はこの state が正(他タブでの変更は追わない)。
  // マスタの取得口(下)がモードで切り替わるため、mode はそれより前に置く。
  const [mode, setMode] = useState<CalcMode>(() => loadCalcMode());

  function selectMode(nextMode: CalcMode): void {
    setMode(nextMode);
    saveCalcMode(nextMode);
  }

  // P4-16(ADR-0304 §追記 A-6): モードごとのマスタの取得口。masterSources はマウント時に1回だけ呼ぶ
  // (以後この props の同一性は見ない。関数の再生成でマスタを読み直させない)。
  // 省略時は null のままにし、masterSource(単数)を両モードで使う既存の使い方を保つ
  // (モードを切り替えてもマスタを読み直さない)。
  const [modeMasterSources] = useState<MasterSources | null>(() =>
    masterSources === undefined ? null : masterSources(clientIds),
  );
  // 今選ばれている取得口。masterSources が無ければ常に masterSource(既定は架空の例データ)。
  const activeMasterSource: MasterSource =
    modeMasterSources === null ? masterSource : modeMasterSources[mode];

  // P4-16b(ADR-0304 A-10): 今の取得口が検索付きのときだけ、その search を画面へ渡す(省略は「検索できない」)。
  const activeMasterSearch = isSearchableMasterSource(activeMasterSource)
    ? activeMasterSource.search
    : undefined;

  // setState は応答が届いたとき(.then のコールバック)だけで行う(react-hooks/set-state-in-effect)。
  // どの取得口(source)の結果かを state に持たせ、現在の取得口(activeMasterSource)と一致しないときは
  // 読み込み中として扱う(ADR-0304 §追記 A-6。前のモードのマスタで画面を出さない)。
  const [masterLoad, setMasterLoad] = useState<MasterLoadState | null>(null);
  const currentMasterLoad: MasterLoadResult | null =
    masterLoad !== null && masterLoad.source === activeMasterSource ? masterLoad.result : null;

  useEffect(() => {
    let cancelled = false;
    activeMasterSource.load().then(
      (master) => {
        if (!cancelled) {
          setMasterLoad({ source: activeMasterSource, result: { ok: true, master } });
        }
      },
      (error: unknown) => {
        if (!cancelled) {
          setMasterLoad({
            source: activeMasterSource,
            result: { ok: false, error: error instanceof Error ? error : new Error(String(error)) },
          });
        }
      },
    );
    return () => {
      cancelled = true;
    };
  }, [activeMasterSource]);

  // 既定のオンライン(API)エンジンは、natures(性格の一覧)が要るのでマスタの読み込みが終わってから作る
  // (ADR-0301 §2・§4)。createApiEngine 自体は fetch しない(初回の計算まで遅延。ADR-0300 §2 と同じ考え方)。
  const fallbackOnlineEngine = useMemo<CalcEngine | null>(() => {
    if (currentMasterLoad === null || !currentMasterLoad.ok) {
      return null;
    }
    return createApiEngine({
      baseUrl: apiBaseUrl(),
      fetch: globalThis.fetch.bind(globalThis),
      master: { natures: currentMasterLoad.master.natures },
      ids: clientIds,
    });
  }, [currentMasterLoad, clientIds]);

  // モードごとに使う engine を決める(ADR-0301 §4)。engines > engine(両モードに使う) > 既定の順。
  // 既定のオンラインは、マスタ読み込み前は使われない(currentMasterLoad が ok になるまで下の画面を描画しない)
  // ので null のままでもよく、その間は offline のプレースホルダで埋める。
  const resolvedEngines: CalcEngines =
    engines ??
    (engine !== undefined
      ? { offline: engine, online: engine }
      : { offline: fallbackOfflineEngine, online: fallbackOnlineEngine ?? fallbackOfflineEngine });

  const resolvedEngine = mode === "online" ? resolvedEngines.online : resolvedEngines.offline;

  // 計算・逆算の切り替え(P4-4)。URL と連動する(P4-10)。パスは BASE_URL からの相対として読む。
  // 初期状態は現在の URL から決め、未知のパスは既定の画面(calc)にしておき(マウント効果が URL を
  // /calc に置き換える。画面自体はマスタ読み込み待ちの間 state だけ既定にしておけば齟齬はない)。
  const base = import.meta.env.BASE_URL;
  const [tab, setTab] = useState<ScreenId>(
    () => screenFromPath(window.location.pathname, base) ?? DEFAULT_SCREEN,
  );

  // タブ・タブパネルの id(WAI-ARIA Authoring Practices の Tabs パターン: tab の aria-controls が
  // panel の id を指し、panel の aria-labelledby が選択中の tab の id を指す)。パネルは1つだけ描画し
  // (非選択側の画面は DOM から外す。App.test.tsx が別タブの combobox が無いことを確かめる)、
  // 中身を選択中のタブで入れ替える。
  const idBase = useId();
  const tabElementId = (id: ScreenId): string => `${idBase}-tab-${id}`;
  const panelId = `${idBase}-panel`;

  // ロービング tabIndex(選択中のタブだけ 0、他は -1)のフォーカス移動先を、クリックでなく
  // キーボードで選んだときに参照する(WAI-ARIA Authoring Practices「automatic activation」)。
  const tabRefs = useRef<Partial<Record<ScreenId, HTMLButtonElement>>>({});

  /** タブの選択(クリック・キーボード)。URL を pushState する(同じタブの選び直しは履歴を積まない)。 */
  function navigateToTab(nextTab: ScreenId): void {
    if (nextTab === tab) {
      return;
    }
    window.history.pushState(null, "", pathForScreen(nextTab, base));
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
    navigateToTab(nextTab);
    tabRefs.current[nextTab]?.focus();
  }

  // マウント時: URL が既知のパスでなければ /calc に置き換える(push でなく replace。履歴を増やさない)。
  // マスタの読み込みを待たない(state の初期値は既に既定になっているので、ここは URL の見た目を直すだけ)。
  useEffect(() => {
    if (screenFromPath(window.location.pathname, base) === null) {
      window.history.replaceState(null, "", pathForScreen(DEFAULT_SCREEN, base));
    }
    // base は import.meta.env.BASE_URL(実行中は不変)なので依存配列に含めない。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ブラウザの戻る・進む(popstate)で URL に合わせてタブを切り替える。未知のパスへ戻ったときは
  // 計算タブを出し、URL も /calc に置き換える(push はしない)。アンマウントで購読を外す。
  useEffect(() => {
    function handlePopState(): void {
      const next = screenFromPath(window.location.pathname, base);
      if (next === null) {
        window.history.replaceState(null, "", pathForScreen(DEFAULT_SCREEN, base));
        setTab(DEFAULT_SCREEN);
        return;
      }
      setTab(next);
    }
    window.addEventListener("popstate", handlePopState);
    return () => {
      window.removeEventListener("popstate", handlePopState);
    };
    // base は import.meta.env.BASE_URL(実行中は不変)なので依存配列に含めない(購読を張り直さない)。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 文書のタイトル(「<画面名> | pokecalc」)。タブの切り替え・popstate に追従し、マスタ読み込み中も設定する。
  useEffect(() => {
    document.title = documentTitle(tab);
  }, [tab]);

  // 選ばれている画面のコンポーネント(app/screens.tsx の対応表から引く)。
  const ActiveScreen = SCREEN_COMPONENTS[tab];

  return (
    <>
      {/* main の外に置く: main の内側だと header は banner ランドマークにならない(HTML-AAM)。 */}
      <header className="app-header">
        <h1>{appText.title}</h1>
        <CalcModeSelector value={mode} onChange={selectMode} />
      </header>
      <main className="app-main">
        {currentMasterLoad === null && <p>{appText.loading}</p>}
        {currentMasterLoad !== null && !currentMasterLoad.ok && <p role="alert">{appText.masterLoadError}</p>}
        {currentMasterLoad !== null && currentMasterLoad.ok && (
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
                      navigateToTab(id);
                    }}
                    onKeyDown={(event) => {
                      handleTabKeyDown(event, index);
                    }}
                  >
                    {screenLabel(id)}
                  </button>
                );
              })}
            </div>
            <div role="tabpanel" id={panelId} aria-labelledby={tabElementId(tab)} className="app-tabs__panel">
              <ActiveScreen
                engine={resolvedEngine}
                master={currentMasterLoad.master}
                client={balanceClient}
                speedClient={speedClient}
                judgeClient={judgeClient}
                masterSearch={activeMasterSearch}
              />
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
