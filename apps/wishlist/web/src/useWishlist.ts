import { useEffect, useMemo, useState } from "react";
import type { Genre, Item, Site } from "./api/types";
import { createApiClient, defaultBaseUrl, type ApiClient } from "./lib/api";
import { fetchWithCache, loadCache, saveCache } from "./lib/cache";

export interface Wishlist {
  client: ApiClient;
  baseUrl: string;
  items: Item[];
  genres: Genre[];
  sites: Site[];
  /** 取得に失敗し、保存済みも無いときのメッセージ */
  loadError: string | null;
  /** 一覧を取れず保存済みを使っている、またはブラウザがオフライン。価格の札は出さない */
  offline: boolean;
  setItems: (items: Item[]) => void;
  setGenres: (genres: Genre[]) => void;
  setSites: (sites: Site[]) => void;
}

/** 一覧・ジャンル・サイトを持つ。起動時は保存済みの値をすぐ出し(オフライン対応)、取得できたら置き換える。 */
export function useWishlist(): Wishlist {
  const baseUrl = defaultBaseUrl();
  const client = useMemo(() => createApiClient({ baseUrl }), [baseUrl]);
  const [items, setItemsState] = useState<Item[]>(() => loadCache("items") ?? []);
  const [genres, setGenresState] = useState<Genre[]>(() => loadCache("genres") ?? []);
  const [sites, setSitesState] = useState<Site[]>(() => loadCache("sites") ?? []);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const [browserOffline, setBrowserOffline] = useState(() => !navigator.onLine);
  // オンラインに戻ったら取り直す。
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const goOffline = () => {
      setBrowserOffline(true);
    };
    const goOnline = () => {
      setBrowserOffline(false);
      setReloadKey((k) => k + 1);
    };
    window.addEventListener("offline", goOffline);
    window.addEventListener("online", goOnline);
    return () => {
      window.removeEventListener("offline", goOffline);
      window.removeEventListener("online", goOnline);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      const results = await Promise.allSettled([
        fetchWithCache("items", () => client.listItems()).then((r) => {
          if (!cancelled) setItemsState(r.data);
          return r.stale;
        }),
        fetchWithCache("genres", () => client.listGenres()).then((r) => {
          if (!cancelled) setGenresState(r.data);
          return r.stale;
        }),
        fetchWithCache("sites", () => client.listSites()).then((r) => {
          if (!cancelled) setSitesState(r.data);
          return r.stale;
        }),
      ]);
      if (cancelled) return;
      setStale(results.some((r) => r.status === "fulfilled" && r.value));
      // 保存済みも無くて取得できなかった分だけがここに来る。白画面にせず、理由を出す。
      const failed = results.find((r) => r.status === "rejected");
      const reason = failed?.reason as unknown;
      setLoadError(failed ? (reason instanceof Error ? reason.message : "一覧を取得できませんでした") : null);
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, [client, reloadKey]);

  return {
    client,
    baseUrl,
    items,
    genres,
    sites,
    loadError,
    offline: stale || browserOffline,
    setItems: (next) => {
      setItemsState(next);
      saveCache("items", next);
    },
    setGenres: (next) => {
      setGenresState(next);
      saveCache("genres", next);
    },
    setSites: (next) => {
      setSitesState(next);
      saveCache("sites", next);
    },
  };
}
