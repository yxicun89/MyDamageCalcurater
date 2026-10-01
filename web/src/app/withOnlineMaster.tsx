// issue 276(ADR-0411): API 専用の画面(タイプバランス・判定)を、オンラインのマスタで包む。
// これらの画面が API に送る ID(ポケモン・特性・技)は、サーバーのマスタにあるものだけが通る。
// オフラインの架空の例データの ID を送ると 422 になるので、ダメージ計算の実行場所(ヘッダーの切替)に
// 関係なく、常にオンラインの取得口から読んだマスタを画面へ渡す。画面を開いたとき(mount)に1回だけ読む。
// 読めなければ、オフラインの架空データへ静かに落とさず、日本語の案内と「再試行」を出す。

import { useEffect, useState, type ComponentType } from "react";
import { appText } from "../i18n/ja";
import { isSearchableMasterSource } from "../master/capabilities";
import type { MasterData, MasterSource } from "../master/types";
import type { ScreenProps } from "./screens";

type Load =
  | { readonly source: MasterSource; readonly status: "ok"; readonly master: MasterData }
  | { readonly source: MasterSource; readonly status: "error" };

export function withOnlineMaster(Screen: ComponentType<ScreenProps>): ComponentType<ScreenProps> {
  function OnlineMasterScreen(props: ScreenProps) {
    const { onlineMasterSource } = props;
    const [load, setLoad] = useState<Load | null>(null);
    const [retryToken, setRetryToken] = useState(0);

    useEffect(() => {
      let cancelled = false;
      onlineMasterSource.load().then(
        (master) => {
          if (!cancelled) {
            setLoad({ source: onlineMasterSource, status: "ok", master });
          }
        },
        () => {
          if (!cancelled) {
            setLoad({ source: onlineMasterSource, status: "error" });
          }
        },
      );
      return () => {
        cancelled = true;
      };
      // retryToken は再実行のためだけの依存(値は使わない)。
    }, [onlineMasterSource, retryToken]);

    const current = load !== null && load.source === onlineMasterSource ? load : null;
    if (current === null) {
      return <p>{appText.loading}</p>;
    }
    if (current.status === "error") {
      return (
        <div role="alert" className="app-master-error">
          <p>{appText.onlineMasterLoadError}</p>
          <div className="app-master-error__actions">
            <button
              type="button"
              className="app-master-error__button"
              onClick={() => {
                setLoad(null);
                setRetryToken((token) => token + 1);
              }}
            >
              {appText.masterLoadRetryLabel}
            </button>
          </div>
        </div>
      );
    }
    return (
      <Screen
        {...props}
        master={current.master}
        masterSearch={isSearchableMasterSource(onlineMasterSource) ? onlineMasterSource.search : undefined}
      />
    );
  }
  return OnlineMasterScreen;
}
