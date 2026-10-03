import { useEffect, useMemo, useState } from "react";
import type { Genre, Item, Site } from "./api/types";
import { createApiClient, defaultBaseUrl, type ApiClient } from "./lib/api";
import { fetchWithCache, loadCache, saveCache } from "./lib/cache";
import type { Settings } from "./lib/settings";

export interface Wishlist {
  client: ApiClient;
  baseUrl: string;
  items: Item[];
  genres: Genre[];
  sites: Site[];
  /** 取得に失敗し、保存済みも無いときのメッセージ */
  loadError: string | null;
  setItems: (items: Item[]) => void;
  setGenres: (genres: Genre[]) => void;
  setSites: (sites: Site[]) => void;
}

/** 一覧・ジャンル・サイトを持つ。起動時は保存済みの値をすぐ出し(オフライン対応)、取得できたら置き換える。 */
export function useWishlist(settings: Settings): Wishlist {
  const baseUrl = settings.apiBaseUrl ?? defaultBaseUrl();
  const { token } = settings;
  const client = useMemo(() => createApiClient({ baseUrl, token }), [baseUrl, token]);
  const [items, setItemsState] = useState<Item[]>(() => loadCache("items") ?? []);
  const [genres, setGenresState] = useState<Genre[]>(() => loadCache("genres") ?? []);
  const [sites, setSitesState] = useState<Site[]>(() => loadCache("sites") ?? []);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    if (token === "") return; // トークン未設定では API を呼ばない。
    let cancelled = false;
    const run = async () => {
      const results = await Promise.allSettled([
        fetchWithCache("items", () => client.listItems()).then((r) => {
          if (!cancelled) setItemsState(r.data);
        }),
        fetchWithCache("genres", () => client.listGenres()).then((r) => {
          if (!cancelled) setGenresState(r.data);
        }),
        fetchWithCache("sites", () => client.listSites()).then((r) => {
          if (!cancelled) setSitesState(r.data);
        }),
      ]);
      // 保存済みも無くて取得できなかった分だけがここに来る。白画面にせず、理由を出す。
      const failed = results.find((r) => r.status === "rejected");
      if (cancelled) return;
      const reason = failed?.reason as unknown;
      setLoadError(failed ? (reason instanceof Error ? reason.message : "一覧を取得できませんでした") : null);
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, [client, token]);

  return {
    client,
    baseUrl,
    items,
    genres,
    sites,
    loadError,
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
