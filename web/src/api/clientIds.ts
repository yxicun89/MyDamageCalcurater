// P4-5: 端末 ID とセッション ID(ADR-0301 §3、CLAUDE.md 技術規約「クライアントは端末ID・セッションIDを
// 全リクエストに付与」)。UUID は crypto.getRandomValues から v4 の形で作る
// (crypto.randomUUID は安全なコンテキストでしか使えず、LAN の HTTP で開くと落ちるため使わない)。
// 端末 ID は localStorage に保存して使い回し、セッション ID は呼び出しごと(ページを開くたび)に新しくする。
// localStorage が使えない環境(プライベートブラウズ等)でも失敗させず、端末 ID もページごとの値で代える。

import { defaultStorage, type StorageLike } from "../app/storage";

/** 全リクエストに付ける端末 ID・セッション ID(ADR-0301 §3)。 */
export interface ClientIds {
  readonly deviceId: string;
  readonly sessionId: string;
}

/** 端末 ID の保存先キー。 */
export const DEVICE_ID_STORAGE_KEY = "pokecalc.deviceId";

/** RFC 4122 の v4(version 4・variant 10xx)の小文字表記。保存済みの端末 ID の妥当性検査に使う。 */
const UUID_V4_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

/** UUID のバイト数(16バイト = 128ビット)。 */
const UUID_BYTE_LENGTH = 16;

/** crypto.getRandomValues の16バイトから RFC 4122 v4 の UUID を作る(crypto.randomUUID は使わない)。 */
export function generateUuidV4(): string {
  const bytes = new Uint8Array(UUID_BYTE_LENGTH);
  crypto.getRandomValues(bytes);
  // version 4(上位4ビットを 0100 にする)。
  bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40;
  // variant 10xx(上位2ビットを 10 にする)。
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/** 保存済みの端末 ID を読み、無い・壊れていれば新しく作って保存する。ストレージが無ければ都度作る。 */
function loadOrCreateDeviceId(storage: StorageLike | null): string {
  if (storage === null) {
    return generateUuidV4();
  }
  try {
    const stored = storage.getItem(DEVICE_ID_STORAGE_KEY);
    if (stored !== null && UUID_V4_PATTERN.test(stored)) {
      return stored;
    }
    const created = generateUuidV4();
    storage.setItem(DEVICE_ID_STORAGE_KEY, created);
    return created;
  } catch {
    return generateUuidV4();
  }
}

/**
 * 端末 ID(保存して使い回す)とセッション ID(呼び出しごとに新しく作る)を作る。
 * ストレージが使えなくても失敗させない(ADR-0301 §3)。
 */
export function createClientIds(storage: StorageLike | null = defaultStorage()): ClientIds {
  return {
    deviceId: loadOrCreateDeviceId(storage),
    sessionId: generateUuidV4(),
  };
}
