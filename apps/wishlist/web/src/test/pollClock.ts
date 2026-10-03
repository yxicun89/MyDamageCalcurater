import { act } from "@testing-library/react";
import { vi } from "vitest";

/**
 * ポーリング用の偽の時計。delay が 2000ms 以上(testing-library の waitFor の 1000ms は本物のまま通す)の `setTimeout` だけを横取りして手動で進める
 * (長押しの 500ms・RTL/user-event 内部の 0ms は本物のまま通す。RTL は vitest の偽タイマーだと固まるため、
 * vi.useFakeTimers は使わない)。`setInterval` は横取りしない = ポーリングは連鎖する `setTimeout` で書くこと。
 * 後始末は setup.ts の `vi.unstubAllGlobals()`。
 */
export function installPollClock() {
  const realSetTimeout = globalThis.setTimeout.bind(globalThis);
  const realClearTimeout = globalThis.clearTimeout.bind(globalThis);
  const pending = new Map<number, { run: () => void; at: number }>();
  let seq = 0;
  let now = 0;

  vi.stubGlobal("setTimeout", (handler: TimerHandler, ms?: number, ...args: unknown[]) => {
    if (typeof handler === "function" && (ms ?? 0) >= 2000) {
      seq -= 1; // 負の ID(本物のタイマー ID と衝突させない)
      const id = seq;
      pending.set(id, {
        run: () => void (handler as (...a: unknown[]) => unknown)(...args),
        at: now + (ms ?? 0),
      });
      return id as unknown as ReturnType<typeof setTimeout>;
    }
    return realSetTimeout(handler, ms, ...args);
  });
  vi.stubGlobal("clearTimeout", (id?: number | ReturnType<typeof setTimeout>) => {
    if (typeof id === "number" && pending.delete(id)) return;
    realClearTimeout(id);
  });

  return {
    /** 待機中の(2000ms 以上の)タイマーの数 */
    pendingCount: () => pending.size,
    /** ms 進め、期限が来たタイマーを古い順に実行する(実行の結果の再描画は act で包む) */
    advance: async (ms: number) => {
      const target = now + ms;
      for (;;) {
        const due = [...pending.entries()]
          .filter(([, t]) => t.at <= target)
          .sort((a, b) => a[1].at - b[1].at)[0];
        if (!due) break;
        pending.delete(due[0]);
        now = due[1].at;
        await act(async () => {
          due[1].run();
          await Promise.resolve();
        });
      }
      now = target;
    },
  };
}
