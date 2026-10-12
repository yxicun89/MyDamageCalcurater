import { useRef } from "react";
import type { Genre, Item } from "../api/types";
import { resolveImageUrl } from "../lib/api";
import { formatYen } from "../lib/price";
import { GenreButton } from "./GenreButton";
import { UiIcon, colorOf, genreColorProps } from "./icons";

const LONG_PRESS_MS = 500;

/** 価格の札。目安が無い・取得失敗(none / failed / summary なし)なら出さない。 */
function PriceTag({ summary }: { summary: Item["summary"] }) {
  if (summary?.status === "ok" && summary.low != null) {
    return (
      <span className="price-tag" data-testid="price-tag" data-kind="price">
        {formatYen(summary.low)}〜
      </span>
    );
  }
  if (summary?.status === "no_result") {
    return (
      <span className="price-tag" data-testid="price-tag" data-kind="none">
        出品なし
      </span>
    );
  }
  return null;
}

interface TileProps {
  item: Item;
  color: string;
  baseUrl: string;
  showPrice: boolean;
  onTap: () => void;
  onLongPress: () => void;
}

/** 画像と価格の札だけのタイル。長押し(500ms)でメニュー、短いタップで詳細。 */
function Tile({ item, color, baseUrl, showPrice, onTap, onLongPress }: TileProps) {
  const timer = useRef<number | undefined>(undefined);
  const fired = useRef(false);

  const clear = () => {
    window.clearTimeout(timer.current);
    timer.current = undefined;
  };

  return (
    <button
      type="button"
      className="tile"
      aria-label={item.name}
      {...genreColorProps(color)}
      onPointerDown={() => {
        fired.current = false;
        clear();
        timer.current = window.setTimeout(() => {
          fired.current = true;
          onLongPress();
        }, LONG_PRESS_MS);
      }}
      onPointerUp={clear}
      onPointerCancel={clear}
      onPointerLeave={clear}
      onClick={() => {
        // 長押しが成立したあとの click は無視する(メニューが出たあとにシートが開かないように)。
        if (fired.current) {
          fired.current = false;
          return;
        }
        onTap();
      }}
      // iOS の長押しメニュー(画像の保存など)を出さない。
      onContextMenu={(e) => {
        e.preventDefault();
      }}
    >
      <img src={resolveImageUrl(item.image_url, baseUrl)} alt={item.name} draggable={false} />
      {showPrice && <PriceTag summary={item.summary} />}
    </button>
  );
}

interface Props {
  items: Item[];
  genres: Genre[];
  baseUrl: string;
  genreFilter: number | null;
  loadError: string | null;
  offline: boolean;
  /** 長押しメニューが出ている間は背景をぼかす */
  blurred: boolean;
  onFilter: (genreId: number | null) => void;
  onOpen: (item: Item) => void;
  onMenu: (item: Item) => void;
  onAdd: () => void;
  onSettings: () => void;
}

export function Home(p: Props) {
  const genres = [...p.genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const shown = p.genreFilter === null ? p.items : p.items.filter((i) => i.genre_id === p.genreFilter);

  return (
    <main className="home" data-testid="home" data-blurred={p.blurred ? "true" : undefined}>
      <header className="home-head">
        <div className="home-title">
          <h1>欲しいもの</h1>
          <span className="count" data-testid="item-count">
            {String(shown.length)}
          </span>
        </div>
        {p.offline && (
          <span className="offline-badge" data-testid="offline-badge">
            <UiIcon name="offline" size={16} />
            オフライン
          </span>
        )}
        <button type="button" className="square-button" aria-label="設定" onClick={p.onSettings}>
          <UiIcon name="gear" />
        </button>
      </header>
      <nav className="genre-nav" aria-label="ジャンル">
        <GenreButton
          name="すべて"
          icon="all"
          selected={p.genreFilter === null}
          onClick={() => {
            p.onFilter(null);
          }}
        />
        {genres.map((g) => (
          <GenreButton
            key={g.id}
            name={g.name}
            icon={g.icon}
            color={g.color}
            selected={p.genreFilter === g.id}
            onClick={() => {
              p.onFilter(g.id);
            }}
          />
        ))}
      </nav>
      {p.loadError !== null && (
        <p role="alert" className="error">
          {p.loadError}
        </p>
      )}
      <div className="grid" data-testid="item-grid">
        {shown.map((item) => (
          <Tile
            key={item.id}
            item={item}
            color={colorOf(p.genres, item.genre_id)}
            baseUrl={p.baseUrl}
            showPrice={!p.offline}
            onTap={() => {
              p.onOpen(item);
            }}
            onLongPress={() => {
              p.onMenu(item);
            }}
          />
        ))}
      </div>
      <button type="button" className="fab" aria-label="追加" onClick={p.onAdd}>
        <UiIcon name="plus" size={28} strokeWidth={2.6} />
      </button>
    </main>
  );
}
