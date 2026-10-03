import { useEffect, useState } from "react";
import type { Genre, Item, ItemEstimates, Site } from "../api/types";
import { ApiError, resolveImageUrl, type ApiClient } from "../lib/api";
import { itemQuery, resolveSiteLinks } from "../lib/links";
import { ErrorText, Modal, errorMessage } from "./Modal";

interface Props {
  item: Item;
  genre: Genre | undefined;
  sites: Site[];
  client: ApiClient;
  baseUrl: string;
  onClose: () => void;
  onUpdated: (item: Item) => void;
}

const yen = (n: number) => `¥${n.toLocaleString("ja-JP")}`;

type Summary =
  { kind: "loading" } | { kind: "ok"; estimates: ItemEstimates } | { kind: "offline" } | { kind: "error" };

function summaryText(s: Summary): string {
  switch (s.kind) {
    case "loading":
      return "読み込み中…";
    case "offline":
      return "オフライン";
    case "error":
      return "価格情報を取得できませんでした";
    case "ok": {
      const { sites, summary_low: low, summary_mid: mid } = s.estimates;
      if (sites.length === 0 || low == null) return "まだ価格情報はありません";
      return `だいたい ${yen(low)}${mid == null ? "" : `〜${yen(mid)}`} で買えそう`;
    }
  }
}

/** 詳細シート。画像・検索ワード(その場編集)・サマリ・サイト行。 */
export function Sheet({ item, genre, sites, client, baseUrl, onClose, onUpdated }: Props) {
  const [summary, setSummary] = useState<Summary>(() => ({ kind: navigator.onLine ? "loading" : "offline" }));
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const itemId = item.id;

  useEffect(() => {
    let cancelled = false;
    if (!navigator.onLine) return; // 初期値が「オフライン」
    client.getEstimates(itemId).then(
      (estimates) => {
        if (!cancelled) setSummary({ kind: "ok", estimates });
      },
      (e: unknown) => {
        if (cancelled) return;
        setSummary({ kind: e instanceof ApiError && e.code === "network" ? "offline" : "error" });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [client, itemId]);

  // 編集中はリンクも入力値で作り直す(一時的な値。保存するまで商品は変えない)。
  const linkItem = editing ? { ...item, query_override: draft } : item;
  const links = resolveSiteLinks(linkItem, genre, sites);

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      const value = draft.trim() === "" ? null : draft;
      onUpdated(await client.updateItem(item.id, { query_override: value }));
      setEditing(false);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal label="詳細" onClose={onClose}>
      <div className="sheet-head">
        <button type="button" className="text-button" onClick={onClose}>
          閉じる
        </button>
      </div>
      <img className="sheet-image" src={resolveImageUrl(item.image_url, baseUrl)} alt={item.name} />

      {editing ? (
        <div className="query-edit">
          <input
            type="text"
            aria-label="検索ワード"
            value={draft}
            onChange={(e) => {
              setDraft(e.target.value);
            }}
          />
          <button type="button" className="primary" disabled={saving} onClick={() => void save()}>
            保存
          </button>
        </div>
      ) : (
        <button
          type="button"
          className="query"
          aria-label="検索ワードを編集"
          onClick={() => {
            setDraft(itemQuery(item, genre));
            setError(null);
            setEditing(true);
          }}
        >
          {itemQuery(item, genre)}
        </button>
      )}
      <ErrorText message={error} />

      <p role="status" className="summary">
        {summaryText(summary)}
      </p>

      <ul className="site-list">
        {links.map((l) => (
          <li key={l.site.id}>
            <a href={l.url} target="_blank" rel="noopener">
              <span>{l.site.name}</span>
              <span aria-hidden="true">›</span>
            </a>
          </li>
        ))}
      </ul>
    </Modal>
  );
}
