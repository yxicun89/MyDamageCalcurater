import { isHttpUrl } from "./deeplink";

/** localStorage のキー "wishlist.settings" に JSON `{"apiBaseUrl": string | null, "token": string}` で保存する。 */
export interface Settings {
  /** null なら既定(defaultBaseUrl) */
  apiBaseUrl: string | null;
  token: string;
}

export const SETTINGS_KEY = "wishlist.settings";

const DEFAULTS: Settings = { apiBaseUrl: null, token: "" };

/** 保存が無い・壊れている・localStorage が使えないときは { apiBaseUrl: null, token: "" }。 */
export const loadSettings = (): Settings => {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY);
    if (raw === null) return { ...DEFAULTS };
    const v: unknown = JSON.parse(raw);
    if (typeof v !== "object" || v === null) return { ...DEFAULTS };
    const { apiBaseUrl, token } = v as Record<string, unknown>;
    if ((apiBaseUrl !== null && typeof apiBaseUrl !== "string") || typeof token !== "string") {
      return { ...DEFAULTS };
    }
    // http(s) 以外の API の URL は使わない(トークンを送る先なので)
    if (apiBaseUrl !== null && !isHttpUrl(apiBaseUrl)) return { apiBaseUrl: null, token };
    return { apiBaseUrl, token };
  } catch {
    return { ...DEFAULTS };
  }
};

/** 保存できたら true。localStorage が使えない(例外)ときは例外を投げず false。 */
export const saveSettings = (settings: Settings): boolean => {
  try {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
    return true;
  } catch {
    return false;
  }
};
