import { useRef } from "react";
import type { Genre, Item } from "../api/types";
import { resolveImageUrl } from "../lib/api";

const LONG_PRESS_MS = 500;

interface TileProps {
  item: Item;
  baseUrl: string;
  onTap: () => void;
  onLongPress: () => void;
}

/** 画像だけのタイル。長押し(500ms)でメニュー、短いタップで詳細。 */
function Tile({ item, baseUrl, onTap, onLongPress }: TileProps) {
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
    </button>
  );
}

interface Props {
  items: Item[];
  genres: Genre[];
  baseUrl: string;
  genreFilter: number | null;
  loadError: string | null;
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
    <main className="home">
      <header className="topbar">
        <nav className="chips" aria-label="ジャンル">
          <button
            type="button"
            className="chip"
            aria-pressed={p.genreFilter === null}
            onClick={() => {
              p.onFilter(null);
            }}
          >
            すべて
          </button>
          {genres.map((g) => (
            <button
              key={g.id}
              type="button"
              className="chip"
              aria-pressed={p.genreFilter === g.id}
              onClick={() => {
                p.onFilter(g.id);
              }}
            >
              {g.name}
            </button>
          ))}
        </nav>
        <button type="button" className="icon-button" aria-label="設定" onClick={p.onSettings}>
          ⚙
        </button>
      </header>
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
            baseUrl={p.baseUrl}
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
        +
      </button>
    </main>
  );
}
