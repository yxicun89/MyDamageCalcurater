import { useCallback, useEffect, useRef, useState } from "react";
import type { Genre, Item, ItemEstimates, Listing, PriceHistory, Site, SiteEstimate } from "../api/types";
import { bandDots, bandLeft, bestSiteIds, hasBandRange } from "../lib/band";
import { ApiError, resolveImageUrl, type ApiClient } from "../lib/api";
import { CHART_BOX, buildChart, historyLabel } from "../lib/history";
import { itemQuery, resolveSiteLinks, type SiteLink } from "../lib/links";
import { officialChange, officialEvidence, officialLastAttempt, officialSummary } from "../lib/official";
import { formatAge, formatJstDate, formatYen, isHttpUrl, reasonLabel } from "../lib/price";
import { siteColor, siteMark } from "../lib/siteMark";
import { DEFAULT_GENRE_COLOR, UiIcon, genreColorProps, type UiIconName } from "./icons";
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

/** 状態の短い文。成功して更新中でもないときは空(価格は帯とタイルが形で伝える)。 */
function statusText(s: Summary): string {
  switch (s.kind) {
    case "loading":
      return "読み込み中…";
    case "offline":
      return "オフライン";
    case "error":
      return "価格情報を取得できませんでした";
    case "ok": {
      const parts = [s.estimates.refreshing && !s.gaveUp ? "更新中…" : "", s.notice ?? ""];
      return parts.filter((p) => p !== "").join(" ");
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

  const genreColor = genre?.color ?? DEFAULT_GENRE_COLOR;
  const siteEstimates = estimates?.sites ?? [];
  const fetchedAt = estimates?.summary_fetched_at;
  const dots = bandDots(
    links.map((l) => l.site),
    siteEstimates,
  );
  const best = bestSiteIds(
    siteEstimates,
    links.map((l) => l.site.id),
  );
  const low = estimates?.summary_low;
  const noPrice =
    estimates !== null &&
    (estimates.sites.length === 0 ||
      (low == null && !estimates.sites.every((e) => e.status === "no_result")));
  const noListing =
    estimates !== null &&
    low == null &&
    estimates.sites.length > 0 &&
    estimates.sites.every((e) => e.status === "no_result");

  if (listingsOpen) {
    return (
      <Modal label="詳細" className="sheet" onClose={onClose}>
        <div className="list-head">
          <button
            type="button"
            className="square-button"
            aria-label="戻る"
            onClick={() => {
              setListingsOpen(false);
            }}
          >
            <UiIcon name="back" />
          </button>
          <h2>あやしい出品 {String(suspiciousTotal)}</h2>
        </div>
        <Suspicious state={listings} sites={sites} />
      </Modal>
    );
  }

  return (
    <Modal label="詳細" className="sheet" onClose={onClose}>
      <div className="sheet-top">
        <button type="button" className="square-button round" aria-label="閉じる" onClick={onClose}>
          <UiIcon name="close" size={18} strokeWidth={2.4} />
        </button>
      </div>
      <div className="hero">
        <div className="photo" {...genreColorProps(genreColor)}>
          <img src={resolveImageUrl(item.image_url, baseUrl)} alt={item.name} />
        </div>
      </div>

      <div className="title-row">
        <span className="genre-dot" data-testid="genre-dot" {...genreColorProps(genreColor)} />
        <span className="item-name">{item.name}</span>
        {fetchedAt ? (
          <span className="when" data-testid="fetched-at">
            <UiIcon name="clock" size={12} strokeWidth={2.4} />
            {formatJstDate(fetchedAt)}
          </span>
        ) : null}
      </div>

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
          <UiIcon name="search" size={16} />
          <span className="query-text">{itemQuery(item, genre)}</span>
          <UiIcon name="edit" size={14} />
        </button>
      )}
      <ErrorText message={error} />

      {low != null && !noPrice ? (
        <div className="band-card" data-testid="price-band">
          <div className="band-numbers">
            <span className="band-low" data-testid="band-low">
              {formatYen(low)}
            </span>
            <span className="band-high" data-testid="band-high">
              〜{estimates?.summary_mid == null ? "" : ` ${formatYen(estimates.summary_mid)}`}
            </span>
          </div>
          <div className="band-track">
            <span className="band-line" />
            {hasBandRange(dots) ? <span className="band-range" data-testid="band-range" /> : null}
            {dots.map((d) => (
              <span
                key={d.site.id}
                className="price-dot"
                data-testid="price-dot"
                data-site-id={d.site.id}
                data-position={d.position}
                aria-label={d.site.name}
                style={{
                  left: bandLeft(dots, d),
                  background: siteColor(d.site.name),
                }}
              >
                {siteMark(d.site.name)}
              </span>
            ))}
          </div>
        </div>
      ) : null}
      {noListing ? (
        <div className="no-listing" data-testid="no-listing">
          <UiIcon name="search" size={28} />
          <span>出品ないかも</span>
        </div>
      ) : null}

      {noPrice ? (
        <div className="band-card empty" data-testid="band-empty">
          <UiIcon name="clock" size={16} />
        </div>
      ) : null}

      <p role="status" className="status">
        {statusText(summary)}
      </p>

      <ul className="site-tiles" aria-label="サイト" data-columns="3">
        {links.map((l) => (
          <li key={l.site.id}>
            <SiteTile link={l} est={siteEstimates.find((e) => e.site_id === l.site.id)} best={best} />
          </li>
        ))}
      </ul>

      {suspiciousTotal > 0 ? (
        <button
          type="button"
          className="warn-button"
          aria-expanded={false}
          onClick={() => {
            setListings({ kind: "loading" });
            setListingsOpen(true);
          }}
        >
          <UiIcon name="warn" size={16} />
          あやしい出品 {String(suspiciousTotal)}
        </button>
      ) : null}

      {item.watch_official ? <OfficialStatusView status={item.official_status} /> : null}

      <p>
        <button
          type="button"
          className="square-button"
          aria-label="更新"
          disabled={summary.kind === "offline" || busy}
          onClick={() => {
            setPosting(true);
            run(() => client.refreshEstimates(itemId), 0);
          }}
        >
          <UiIcon name="refresh" size={18} />
        </button>
      </p>

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

/** サイトのタイル。押すと検索結果を新しいタブで開く。状態は文ではなく形で伝える。 */
function SiteTile({ link, est, best }: { link: SiteLink; est: SiteEstimate | undefined; best: Set<number> }) {
  const { site } = link;
  const none = est?.status === "no_result";
  const failedWithoutPrice = est?.status === "failed" && est.low == null;
  const isBest = best.has(site.id);
  let price: string | null = null;
  if (none) price = "なし";
  else if (est?.low != null) price = formatYen(est.low);
  const title =
    est?.status === "failed" && est.low != null
      ? `最終取得: ${formatAge(est.fetched_at, new Date())}`
      : undefined;
  return (
    <a
      className="site-tile"
      href={link.url}
      target="_blank"
      rel="noopener"
      aria-label={site.name}
      title={title}
      data-dim={none ? "true" : undefined}
      data-failed={failedWithoutPrice ? "true" : undefined}
      data-best={isBest ? "true" : undefined}
      aria-current={isBest ? "true" : undefined}
    >
      <span className="site-mark" data-testid="site-mark" style={{ background: siteColor(site.name) }}>
        {siteMark(site.name)}
      </span>
      {price !== null && (
        <span className="site-price" data-testid="site-price">
          {price}
        </span>
      )}
    </a>
  );
}

/** 公式ページの販売状況(監視中の商品だけ。alert にしない)。 */
function OfficialStatusView({ status }: { status: Item["official_status"] }) {
  const lines = [
    officialSummary(status),
    officialEvidence(status),
    officialChange(status, new Date()),
    officialLastAttempt(status),
  ].filter((l): l is string => l != null);
  return (
    <section aria-label="公式の販売状況" className="official">
      {lines.map((l) => (
        <p key={l} className="site-meta">
          {l}
        </p>
      ))}
    </section>
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

const REASON_ICONS: Record<Listing["suspicious_reasons"][number], UiIconName> = {
  title_mismatch: "tag",
  too_cheap: "down",
  below_min: "warn",
};

function Suspicious({ state, sites }: { state: ListingsState; sites: Site[] }) {
  if (state.kind === "error") return <p className="site-meta">出品を取得できませんでした</p>;
  if (state.kind !== "ok") return <p className="site-meta">読み込み中…</p>;
  const rows = state.listings.filter((l) => l.suspicious_reasons.length > 0);
  return (
    <ul className="suspicious-list" aria-label="あやしい出品">
      {rows.map((l) => {
        const siteName = sites.find((s) => s.id === l.site_id)?.name ?? `サイト${String(l.site_id)}`;
        const body = (
          <>
            {l.image_url && isHttpUrl(l.image_url) ? <img src={l.image_url} alt={l.title} /> : null}
            <span className="listing-main">
              <span className="listing-price">{formatYen(l.price)}</span>
              <span className="listing-tags">
                {l.suspicious_reasons.map((r) => (
                  <span key={r} className="reason-tag" data-testid="reason-tag" data-reason={r}>
                    <UiIcon name={REASON_ICONS[r]} size={12} strokeWidth={2.4} />
                    {reasonLabel(r)}
                  </span>
                ))}
              </span>
            </span>
            <span
              className="site-mark small"
              data-testid="listing-site"
              aria-label={siteName}
              style={{ background: siteColor(siteName) }}
            >
              {siteMark(siteName)}
            </span>
          </>
        );
        return (
          <li key={l.id}>
            {isHttpUrl(l.url) ? (
              <a className="listing-row" href={l.url} target="_blank" rel="noopener">
                {body}
              </a>
            ) : (
              <div className="listing-row">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
