// P4-5: localStorage 相当の最小インターフェース(ADR-0301 §3・§4)。
// 実装(app/calcMode.ts・api/clientIds.ts)は既定で globalThis.localStorage を使うが、
// テストや「使えない環境(プライベートブラウズ等)」向けに差し替えられるようにする。

/** 読み書きに使う最小限のストレージ(localStorage のサブセット)。呼び出しは例外を投げてよい。 */
export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

/**
 * 既定のストレージ(globalThis.localStorage)。存在しない(Node 等)・アクセスで例外になる環境
 * (プライベートブラウズ・サイトデータのブロック)では null を返し、呼び出し側は保存しないで動く。
 */
export function defaultStorage(): StorageLike | null {
  try {
    // 型の上では常にあるが、Node などの環境では undefined になるため、あり得る値として扱う。
    const storage = (globalThis as { localStorage?: Storage }).localStorage;
    return storage ?? null;
  } catch {
    return null;
  }
}
