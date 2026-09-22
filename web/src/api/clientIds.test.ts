// P4-5: 端末 ID とセッション ID(ADR-0301 §3、CLAUDE.md 技術規約)。
// UUID は crypto.getRandomValues から v4 の形で作る(crypto.randomUUID は安全なコンテキスト限定で、LAN の HTTP で落ちる)。
// 端末 ID は localStorage に保存して使い回し、セッション ID はページを開くたび(createClientIds の呼び出しごと)に新しくする。
// localStorage が使えない環境でも失敗させない(端末 ID もページごとの値で代える)。

import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { StorageLike } from "../app/storage";
import { DEVICE_ID_STORAGE_KEY, createClientIds, generateUuidV4 } from "./clientIds";

/** RFC 4122 の v4(version 4・variant 10xx)の小文字表記。 */
const uuidV4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

/** 読み書きのたびに例外を投げるストレージ(プライベートブラウズ・容量超過・無効化の代わり)。 */
const throwingStorage: StorageLike = {
  getItem() {
    throw new DOMException("denied", "SecurityError");
  },
  setItem() {
    throw new DOMException("denied", "QuotaExceededError");
  },
};

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

describe("generateUuidV4", () => {
  test("v4 の形(小文字・ハイフン区切り・version 4・variant 10xx)で、呼ぶたびに違う値", () => {
    const values = Array.from({ length: 50 }, () => generateUuidV4());
    for (const value of values) {
      expect(value).toMatch(uuidV4);
    }
    expect(new Set(values).size).toBe(values.length);
  });

  test("crypto.getRandomValues の16バイトから作り、crypto.randomUUID は呼ばない", () => {
    const randomUUID = vi.spyOn(crypto, "randomUUID");
    const getRandomValues = vi.spyOn(crypto, "getRandomValues");
    generateUuidV4();
    expect(randomUUID).not.toHaveBeenCalled();
    expect(getRandomValues).toHaveBeenCalled();
  });

  test.each([
    [0x00, "00000000-0000-4000-8000-000000000000"],
    [0xff, "ffffffff-ffff-4fff-bfff-ffffffffffff"],
  ])("乱数がすべて 0x%s なら version・variant のビットだけ立てた %s", (byte, expected) => {
    vi.spyOn(crypto, "getRandomValues").mockImplementation(
      <T extends ArrayBufferView | null>(array: T): T => {
        if (array instanceof Uint8Array) {
          array.fill(byte);
        }
        return array;
      },
    );
    expect(generateUuidV4()).toBe(expected);
  });
});

describe("createClientIds", () => {
  test("端末 ID を localStorage の pokecalc.deviceId に保存し、次の呼び出しでも同じ値を使う", () => {
    expect(DEVICE_ID_STORAGE_KEY).toBe("pokecalc.deviceId");
    const first = createClientIds();
    expect(first.deviceId).toMatch(uuidV4);
    expect(localStorage.getItem(DEVICE_ID_STORAGE_KEY)).toBe(first.deviceId);

    const second = createClientIds();
    expect(second.deviceId).toBe(first.deviceId);
  });

  test("保存済みの端末 ID があればそれを使う", () => {
    const stored = "33333333-3333-4333-9333-333333333333";
    localStorage.setItem(DEVICE_ID_STORAGE_KEY, stored);
    expect(createClientIds().deviceId).toBe(stored);
  });

  test("保存済みの値が UUID v4 の形でなければ作り直して保存し直す", () => {
    localStorage.setItem(DEVICE_ID_STORAGE_KEY, "not-a-uuid");
    const { deviceId } = createClientIds();
    expect(deviceId).toMatch(uuidV4);
    expect(localStorage.getItem(DEVICE_ID_STORAGE_KEY)).toBe(deviceId);
  });

  test("セッション ID は呼び出しごとに新しい UUID v4 で、端末 ID とも違う", () => {
    const first = createClientIds();
    const second = createClientIds();
    expect(first.sessionId).toMatch(uuidV4);
    expect(second.sessionId).toMatch(uuidV4);
    expect(second.sessionId).not.toBe(first.sessionId);
    expect(first.sessionId).not.toBe(first.deviceId);
  });

  test("セッション ID は保存しない(localStorage に書くのは端末 ID の1件だけ)", () => {
    createClientIds();
    expect(localStorage.length).toBe(1);
  });

  test("ストレージが例外を投げても失敗せず、UUID v4 の端末 ID・セッション ID を返す", () => {
    const ids = createClientIds(throwingStorage);
    expect(ids.deviceId).toMatch(uuidV4);
    expect(ids.sessionId).toMatch(uuidV4);
    expect(localStorage.getItem(DEVICE_ID_STORAGE_KEY)).toBeNull();
  });

  test("ストレージを注入すると、そのストレージに保存する", () => {
    const saved = new Map<string, string>();
    const storage: StorageLike = {
      getItem: (key) => saved.get(key) ?? null,
      setItem: (key, value) => {
        saved.set(key, value);
      },
    };
    const { deviceId } = createClientIds(storage);
    expect(saved.get(DEVICE_ID_STORAGE_KEY)).toBe(deviceId);
    expect(createClientIds(storage).deviceId).toBe(deviceId);
  });
});
