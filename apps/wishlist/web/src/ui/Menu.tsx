import { useEffect } from "react";
import type { Item } from "../api/types";
import { resolveImageUrl } from "../lib/api";
import { isHttpUrl } from "../lib/deeplink";
import { genreColorProps } from "./icons";

interface Props {
  item: Item;
  color: string;
  baseUrl: string;
  onEdit: () => void;
  onDelete: () => void;
  onClose: () => void;
}

/** 長押しメニュー。押した商品の画像を出し、背景のタップか Escape で閉じる。 */
export function Menu({ item, color, baseUrl, onEdit, onDelete, onClose }: Props) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  const source = item.source_url ?? "";

  return (
    <div className="menu-overlay" onClick={onClose}>
      <div className="menu-stack">
        <div className="menu-preview" data-testid="menu-preview" {...genreColorProps(color)}>
          <img src={resolveImageUrl(item.image_url, baseUrl)} alt={item.name} draggable={false} />
        </div>
        <div className="menu-list" role="menu">
          <button type="button" role="menuitem" onClick={onEdit}>
            編集
          </button>
          {isHttpUrl(source) && (
            <a role="menuitem" href={source} target="_blank" rel="noopener" onClick={onClose}>
              元のページを開く
            </a>
          )}
          <button type="button" role="menuitem" className="danger" onClick={onDelete}>
            削除
          </button>
        </div>
      </div>
    </div>
  );
}
