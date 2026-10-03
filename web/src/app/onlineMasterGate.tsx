// issue 276(ADR-0411): API 専用の画面(タイプバランス・判定)を、オンラインのマスタで包む。
// これらの画面が API に送る ID(ポケモン・特性・技)は、サーバーのマスタにあるものだけが通る。
// オフラインの架空の例データの ID を送ると 422 になるので、ダメージ計算の実行場所(ヘッダーの切替)に
// 関係なく、常にオンラインの取得口から読んだマスタを画面へ渡す。画面を開いたとき(mount)に1回だけ読む。
// 読めなければ、オフラインの架空データへ静かに落とさず、日本語の案内と「再試行」を出す。
// ADR-0323: 以前の高階コンポーネント withOnlineMaster(共有の ScreenProps 専用)を、各画面の Props の型を崩さずに
// 使える render-prop に置き換えた(画面の登録ファイルの render が children で画面を描く)。挙動は同じ。

import { useEffect, useState, type ReactNode } from "react";
import { appText } from "../i18n/ja";
import { isSearchableMasterSource } from "../master/capabilities";
import type { MasterData, MasterSource, MasterSpeciesSearch } from "../master/types";

type Load =
  | { readonly source: MasterSource; readonly status: "ok"; readonly master: MasterData }
  | { readonly source: MasterSource; readonly status: "error" };

interface OnlineMasterGateProps {
  /** オンラインのマスタの取得口(ScreenEnvironment.onlineMasterSource)。 */
  readonly onlineMasterSource: MasterSource;
  /** 読めたマスタと、取得口が検索付きのときだけその search を受け取って画面を描く。 */
  readonly children: (master: MasterData, masterSearch: MasterSpeciesSearch | undefined) => ReactNode;
}

export function OnlineMasterGate({ onlineMasterSource, children }: OnlineMasterGateProps) {
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
  return children(
    current.master,
    isSearchableMasterSource(onlineMasterSource) ? onlineMasterSource.search : undefined,
  );
}
