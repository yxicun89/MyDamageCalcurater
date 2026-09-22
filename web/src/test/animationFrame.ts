// テスト専用: requestAnimationFrame / cancelAnimationFrame の fake(P4-9 ホロの間引き)。
// jsdom の requestAnimationFrame は実時間で進むため、フレームを出すタイミングをテストから決められるようにする。
// installFakeAnimationFrame() で window に差し込み、afterEach で vi.unstubAllGlobals() する。
// 予約されたコールバックは flush() を呼ぶまで実行しない(1回の flush = 1フレーム)。

import { vi, type Mock } from "vitest";

export interface FakeAnimationFrame {
  /** requestAnimationFrame の spy(呼ばれた回数 = 予約したフレームの数)。 */
  readonly request: Mock<(callback: FrameRequestCallback) => number>;
  /** cancelAnimationFrame の spy(引数は取り消した予約の id)。 */
  readonly cancel: Mock<(handle: number) => void>;
  /** まだ実行も取り消しもされていない予約の id。 */
  pendingIds(): number[];
  /** 1フレーム進める: 今ある予約を全部実行する(実行中に増えた予約は次のフレームに回す)。 */
  flush(): void;
}

export function installFakeAnimationFrame(): FakeAnimationFrame {
  let nextId = 1;
  let now = 0;
  const pending = new Map<number, FrameRequestCallback>();
  const request = vi.fn((callback: FrameRequestCallback): number => {
    const id = nextId;
    nextId += 1;
    pending.set(id, callback);
    return id;
  });
  const cancel = vi.fn((handle: number): void => {
    pending.delete(handle);
  });
  vi.stubGlobal("requestAnimationFrame", request);
  vi.stubGlobal("cancelAnimationFrame", cancel);
  return {
    request,
    cancel,
    pendingIds: () => [...pending.keys()],
    flush: () => {
      const callbacks = [...pending.values()];
      pending.clear();
      now += 16;
      for (const callback of callbacks) {
        callback(now);
      }
    },
  };
}
