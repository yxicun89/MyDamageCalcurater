export interface RegisterOptions {
  /** 既定は import.meta.env.BASE_URL */
  baseUrl?: string;
  /** 既定は import.meta.env.PROD(開発サーバーでは登録しない) */
  enabled?: boolean;
  /** 既定は navigator */
  nav?: Pick<Navigator, "serviceWorker">;
}

/** `${baseUrl}sw.js` を scope `${baseUrl}` で登録する。未対応・無効・登録失敗でも例外を投げない。 */
export const registerServiceWorker = async (options: RegisterOptions = {}): Promise<void> => {
  const baseUrl = options.baseUrl ?? import.meta.env.BASE_URL;
  const enabled = options.enabled ?? import.meta.env.PROD;
  const nav = options.nav ?? navigator;
  if (!enabled || !("serviceWorker" in nav)) return;
  try {
    await nav.serviceWorker.register(`${baseUrl}sw.js`, { scope: baseUrl });
  } catch {
    // 登録できなくてもアプリは動く(オフライン対応が効かないだけ)。
  }
};
