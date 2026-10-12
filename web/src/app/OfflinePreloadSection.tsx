// ADR-0358 段 3: 「このアプリについて」の「オフライン用にポケモンのデータを保存」。
// 手順は master/cache/preloadSpecies.ts に任せ、ここは表示と操作(進捗・中断・フォーカス)だけを持つ。
// 実データは端末の IndexedDB だけに置く(ADR-0002)。サーバー負荷のため直列で引く(preloadSpecies)。

import { useEffect, useId, useRef, useState } from "react";
import { offlinePreloadText } from "../i18n/offlineScope";
import { preloadSpecies, type PreloadResult } from "../master/cache/preloadSpecies";
import type { SearchableMasterSource } from "../master/types";

export interface OfflinePreloadDeps {
  /** 結果を端末へ保存する取得口(createCachingMasterSource で包んだオンラインの取得口)。 */
  readonly source: SearchableMasterSource;
  readonly loadSavedKeys: () => Promise<ReadonlySet<string>>;
}

interface OfflinePreloadSectionProps {
  /** 計算する場所がサーバーか。false(オフライン)のときは押せず、理由を出す。 */
  readonly online: boolean;
  readonly preload: OfflinePreloadDeps;
  /** 保存が終わった(完了・中断・失敗)ときの合図。保存済み件数の表示を取り直す口。 */
  readonly onSaved?: () => void;
}

/** 読み上げの回数を抑える間隔(件)。連打で polite の読み上げが溜まらないようにする。 */
export const PROGRESS_ANNOUNCE_EVERY = 10;

type Phase =
  | { readonly kind: "idle" }
  | { readonly kind: "running"; readonly done: number; readonly total: number; readonly preparing: boolean }
  | { readonly kind: "finished"; readonly result: PreloadResult };

function finishedMessage(result: PreloadResult): string {
  if (result.status === "completed") {
    return offlinePreloadText.completed(result.savedCount);
  }
  if (result.status === "aborted") {
    return offlinePreloadText.aborted(result.savedCount);
  }
  return result.reason === "storage"
    ? offlinePreloadText.failedStorage(result.savedCount)
    : offlinePreloadText.failedServer(result.savedCount);
}

export function OfflinePreloadSection({ online, preload, onSaved }: OfflinePreloadSectionProps) {
  const [phase, setPhase] = useState<Phase>({ kind: "idle" });
  const abortRef = useRef<AbortController | null>(null);
  const startRef = useRef<HTMLButtonElement>(null);
  const stopRef = useRef<HTMLButtonElement>(null);
  const reasonId = useId();
  const running = phase.kind === "running";

  // 画面を離れたら以後の要求を送らない。
  useEffect(
    () => () => {
      abortRef.current?.abort();
    },
    [],
  );

  // 実行中は中断ボタン、終わったら開始ボタンへフォーカスを移す(押したボタンが消えてフォーカスを失わない)。
  useEffect(() => {
    if (phase.kind === "running" && phase.preparing) {
      stopRef.current?.focus();
    } else if (phase.kind === "finished") {
      startRef.current?.focus();
    }
  }, [phase]);

  function start(): void {
    if (abortRef.current !== null) {
      return;
    }
    const controller = new AbortController();
    abortRef.current = controller;
    setPhase({ kind: "running", done: 0, total: 0, preparing: true });
    let announced = 0;
    void preloadSpecies({
      source: preload.source,
      loadSavedKeys: preload.loadSavedKeys,
      signal: controller.signal,
      onProgress: (done, total) => {
        // 読み上げ(status の文)は間引く。最初と最後は必ず更新する。
        if (done === total || done - announced >= PROGRESS_ANNOUNCE_EVERY || announced === 0) {
          announced = done;
          setPhase({ kind: "running", done, total, preparing: false });
        }
      },
    }).then((result) => {
      abortRef.current = null;
      setPhase({ kind: "finished", result });
      onSaved?.();
    });
  }

  const failed = phase.kind === "finished" && phase.result.status === "failed";
  const resumable = phase.kind === "finished" && phase.result.status !== "completed";

  return (
    <section className="ui-card about__section">
      <h3 className="about__section-heading">{offlinePreloadText.sectionHeading}</h3>
      <p>{offlinePreloadText.explanation}</p>
      {running ? (
        <button
          ref={stopRef}
          type="button"
          className="ui-button ui-button--secondary"
          onClick={() => abortRef.current?.abort()}
        >
          {offlinePreloadText.stopButton}
        </button>
      ) : (
        <button
          ref={startRef}
          type="button"
          className="ui-button ui-button--primary"
          disabled={!online}
          aria-describedby={online ? undefined : reasonId}
          onClick={start}
        >
          {resumable ? offlinePreloadText.resumeButton : offlinePreloadText.startButton}
        </button>
      )}
      {!online && (
        <p id={reasonId} className="ui-notice ui-notice--info">
          {offlinePreloadText.offlineReason}
        </p>
      )}
      {/* 領域は常に置く(出現と同時の変更は読み上げられないことがある)。失敗は role=alert、進捗・完了・中断は status(polite)。 */}
      <p role="status" aria-live="polite" className="about__preload-status">
        {phase.kind === "running" &&
          (phase.preparing
            ? offlinePreloadText.preparing
            : offlinePreloadText.progress(phase.done, phase.total))}
        {phase.kind === "finished" && !failed && finishedMessage(phase.result)}
      </p>
      {phase.kind === "finished" && failed && (
        <p role="alert" className="ui-notice ui-notice--error">
          {finishedMessage(phase.result)}
        </p>
      )}
    </section>
  );
}
