import { useCallback, useEffect, useRef, useState } from "react";
import type { Genre, Item, ItemEstimates, Listing, PriceHistory, Site, SiteEstimate } from "../api/types";
import { ApiError, resolveImageUrl, type ApiClient } from "../lib/api";
import { CHART_BOX, buildChart, historyLabel } from "../lib/history";
import { itemQuery, resolveSiteLinks } from "../lib/links";
import { formatAge, formatJstDate, formatRange, formatYen, isHttpUrl, reasonLabel } from "../lib/price";
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

const POLL_MS = 5000;
const MAX_POLLS = 6;

type Summary =
  | { kind: "loading" }
  | { kind: "ok"; estimates: ItemEstimates; gaveUp: boolean; notice: string | null }
  | { kind: "offline" }
  | { kind: "error" };

type ListingsState =
  { kind: "idle" } | { kind: "loading" } | { kind: "ok"; listings: Listing[] } | { kind: "error" };

function summaryText(s: Summary): string {
  switch (s.kind) {
    case "loading":
      return "読み込み中…";
    case "offline":
      return "オフライン";
    case "error":
      return "価格情報を取得できませんでした";
    case "ok": {
      const { sites, summary_low: low, summary_mid: mid, summary_fetched_at: at } = s.estimates;
      const refreshing = s.estimates.refreshing && !s.gaveUp ? " 更新中…" : "";
      const notice = s.notice ? ` ${s.notice}` : "";
      if (sites.length > 0 && low == null && sites.every((e) => e.status === "no_result"))
        return `出品ないかも${refreshing}${notice}`;
      if (sites.length === 0 || low == null) return `まだ価格情報はありません${refreshing}${notice}`;
      const when = at ? `(${formatJstDate(at)} 時点)` : "";
      return `だいたい ${formatRange(low, mid)} で買えそう${when}${refreshing}${notice}`;
    }
  }
}

/** サイト行の目安(estimates にあるサイトだけ) */
function SiteDetail({ est, now }: { est: SiteEstimate; now: Date }) {
  if (est.status === "no_result") return <span className="site-meta">出品ないかも</span>;
  if (est.low == null) return <span className="site-meta">取得できませんでした</span>;
  return (
    <>
      <span className="site-meta">
        {formatRange(est.low, est.mid)} / {String(est.count)}件 /{" "}
        {est.in_stock_count > 0 ? "在庫あり" : "在庫なし"}
      </span>
      {est.status === "failed" ? (
        <span className="site-meta">最終取得: {formatAge(est.fetched_at, now)}</span>
      ) : null}
    </>
  );
}

/** 詳細シート。画像・検索ワード(その場編集)・サマリ・サイト行。 */
export function Sheet({ item, genre, sites, client, baseUrl, onClose, onUpdated }: Props) {
  const [summary, setSummary] = useState<Summary>(() => ({ kind: navigator.onLine ? "loading" : "offline" }));
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [listingsOpen, setListingsOpen] = useState(false);
  const [listings, setListings] = useState<ListingsState>({ kind: "idle" });
  const [historyOpen, setHistoryOpen] = useState(false);
  const [history, setHistory] = useState<HistoryState>({ kind: "idle" });
  const historyRequested = useRef(false);
  const itemId = item.id;
  // 最後に始めた要求の世代。古い応答で新しい状態を上書きしない。
  const gen = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const alive = useRef(false);
  // 一度でも取得できた estimates(失敗しても表示し続ける)。
  const last = useRef<ItemEstimates | null>(null);
  const [posting, setPosting] = useState(false);
  const poll = useRef<(attempt: number) => void>(() => undefined);

  // 要求(GET か POST)を出し、応答を反映する。refreshing:true なら 5 秒後の再取得を予約する(連鎖する setTimeout)。
  const run = useCallback((request: () => Promise<ItemEstimates>, attempt: number) => {
    clearTimeout(timer.current);
    const g = ++gen.current;
    request().then(
      (estimates) => {
        if (!alive.current || g !== gen.current) return;
        setPosting(false);
        last.current = estimates;
        const more = estimates.refreshing && attempt < MAX_POLLS;
        setSummary({ kind: "ok", estimates, gaveUp: estimates.refreshing && !more, notice: null });
        if (more) {
          timer.current = setTimeout(() => {
            poll.current(attempt + 1);
          }, POLL_MS);
        }
      },
      (e: unknown) => {
        if (!alive.current || g !== gen.current) return;
        setPosting(false);
        const offline = e instanceof ApiError && e.code === "network";
        const prev = last.current;
        if (prev) {
          // 前回値を残し、失敗は短い文で添える(再取得は予約しない)。
          const notice = offline ? "オフライン(前回の値)" : "更新できませんでした";
          setSummary({ kind: "ok", estimates: prev, gaveUp: true, notice });
        } else {
          setSummary({ kind: offline ? "offline" : "error" });
        }
      },
    );
  }, []);

  useEffect(() => {
    alive.current = true;
    last.current = null;
    poll.current = (attempt) => {
      run(() => client.getEstimates(itemId), attempt);
    };
    if (navigator.onLine) run(() => client.getEstimates(itemId), 0); // オフラインなら初期値が「オフライン」
    return () => {
      alive.current = false;
      clearTimeout(timer.current);
    };
  }, [client, itemId, run]);

  const busy = posting || (summary.kind === "ok" && summary.estimates.refreshing && !summary.gaveUp);
  const estimates = summary.kind === "ok" ? summary.estimates : null;
  const suspiciousTotal = estimates?.sites.reduce((n, s) => n + s.suspicious_count, 0) ?? 0;

  // 参考外は開いたときだけ取る。
  useEffect(() => {
    if (!listingsOpen) return;
    let cancelled = false;
    client.listListings(itemId).then(
      (all) => {
        if (!cancelled) setListings({ kind: "ok", listings: all });
      },
      () => {
        if (!cancelled) setListings({ kind: "error" });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [client, itemId, listingsOpen]);

  // 価格の推移は開いたときに 1 回だけ取る(シートを開いている間は取り直さない)。
  useEffect(() => {
    if (!historyOpen || historyRequested.current) return;
    historyRequested.current = true;
    client.getPriceHistory(itemId).then(
      (h) => {
        setHistory({ kind: "ok", history: h });
      },
      (e: unknown) => {
        setHistory({ kind: e instanceof ApiError && e.code === "network" ? "offline" : "error" });
      },
    );
  }, [client, itemId, historyOpen]);

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

      <p>
        <button
          type="button"
          className="text-button"
          disabled={summary.kind === "offline" || busy}
          onClick={() => {
            setPosting(true);
            run(() => client.refreshEstimates(itemId), 0);
          }}
        >
          更新
        </button>
      </p>

      <ul className="site-list" aria-label="サイト">
        {links.map((l) => {
          const est = estimates?.sites.find((e) => e.site_id === l.site.id);
          return (
            <li key={l.site.id}>
              <a href={l.url} target="_blank" rel="noopener">
                <span className="site-main">
                  <span>{l.site.name}</span>
                  {est ? <SiteDetail est={est} now={new Date()} /> : null}
                </span>
                <span aria-hidden="true">›</span>
              </a>
            </li>
          );
        })}
      </ul>

      {suspiciousTotal > 0 ? (
        <details
          className="suspicious"
          open={listingsOpen}
          onToggle={(e) => {
            setListingsOpen(e.currentTarget.open);
          }}
        >
          <summary>参考外 {String(suspiciousTotal)}件</summary>
          {listingsOpen ? <Suspicious state={listings} /> : null}
        </details>
      ) : null}

      <details
        className="history"
        open={historyOpen}
        onToggle={(e) => {
          setHistoryOpen(e.currentTarget.open);
        }}
      >
        <summary>価格の推移</summary>
        {historyOpen ? <History state={history} sites={sites} /> : null}
      </details>
    </Modal>
  );
}

type HistoryState =
  { kind: "idle" } | { kind: "ok"; history: PriceHistory } | { kind: "offline" } | { kind: "error" };

function History({ state, sites }: { state: HistoryState; sites: Site[] }) {
  const [shown, setShown] = useState<number[]>([]);
  if (state.kind === "offline") return <p className="site-meta">オフライン</p>;
  if (state.kind === "error") return <p className="site-meta">価格の推移を取得できませんでした</p>;
  if (state.kind !== "ok") return <p className="site-meta">読み込み中…</p>;
  const h = state.history;
  if (h.overall.length < 2) return <p className="site-meta">推移はまだありません</p>;
  const name = (id: number) => sites.find((s) => s.id === id)?.name ?? `サイト${String(id)}`;
  const series = [
    { key: "overall", points: h.overall.map((o) => ({ day: o.day, value: o.low })) },
    ...h.sites.flatMap((s) =>
      shown.includes(s.site_id)
        ? [{ key: `site-${String(s.site_id)}`, points: s.points.map((p) => ({ day: p.day, value: p.low })) }]
        : [],
    ),
  ];
  const chart = buildChart(series, CHART_BOX);
  if (!chart) return <p className="site-meta">推移はまだありません</p>;
  const color = (key: string) => {
    if (key === "overall") return "var(--chart-overall)";
    const i = h.sites.findIndex((s) => `site-${String(s.site_id)}` === key);
    return `var(--chart-site-${String((i % 4) + 1)})`;
  };
  return (
    <>
      <svg
        className="history-chart"
        role="img"
        aria-label={historyLabel(h.overall)}
        viewBox={`0 0 ${String(CHART_BOX.width)} ${String(CHART_BOX.height)}`}
      >
        {chart.xLabels.map((l, i) => (
          <text key={`x${String(i)}`} x={l.x} y={CHART_BOX.height - 6} textAnchor="middle">
            {l.text}
          </text>
        ))}
        {chart.yLabels.map((l, i) => (
          <text key={`y${String(i)}`} x={CHART_BOX.left - 6} y={l.y + 3} textAnchor="end">
            {l.text}
          </text>
        ))}
        {chart.paths.map((p) => (
          <path key={p.key} data-series={p.key} d={p.d} stroke={color(p.key)} />
        ))}
        {chart.dots.map((d) => (
          <circle
            key={`${d.key}-${d.day}`}
            data-series={d.key}
            cx={d.x}
            cy={d.y}
            r={2.5}
            fill={color(d.key)}
          />
        ))}
      </svg>
      <div className="history-legend" role="group" aria-label="凡例">
        {h.sites.map((s) => {
          const on = shown.includes(s.site_id);
          return (
            <button
              key={s.site_id}
              type="button"
              aria-pressed={on}
              onClick={() => {
                setShown(on ? shown.filter((id) => id !== s.site_id) : [...shown, s.site_id]);
              }}
            >
              {name(s.site_id)}
            </button>
          );
        })}
      </div>
    </>
  );
}

function Suspicious({ state }: { state: ListingsState }) {
  if (state.kind === "error") return <p className="site-meta">参考外の出品を取得できませんでした</p>;
  if (state.kind !== "ok") return <p className="site-meta">読み込み中…</p>;
  const rows = state.listings.filter((l) => l.suspicious_reasons.length > 0);
  return (
    <ul className="suspicious-list" aria-label="参考外の出品">
      {rows.map((l) => (
        <li key={l.id}>
          {l.image_url && isHttpUrl(l.image_url) ? <img src={l.image_url} alt={l.title} /> : null}
          <div>
            <div>{l.title}</div>
            <div>{formatYen(l.price)}</div>
            <div className="reason">{l.suspicious_reasons.map(reasonLabel).join("・")}</div>
            {isHttpUrl(l.url) ? (
              <a href={l.url} target="_blank" rel="noopener">
                出品を開く
              </a>
            ) : null}
          </div>
        </li>
      ))}
    </ul>
  );
}
