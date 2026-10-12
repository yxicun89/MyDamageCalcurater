import { useState } from "react";
import type { Genre, Item, ItemUpdate } from "../api/types";
import { resolveImageUrl, type ApiClient } from "../lib/api";
import { GenreButton } from "./GenreButton";
import { colorOf, genreColorProps } from "./icons";
import { ErrorText, Modal, errorMessage } from "./Modal";

interface Props {
  item: Item;
  genres: Genre[];
  client: ApiClient;
  baseUrl: string;
  onClose: () => void;
  onUpdated: (item: Item) => void;
}

/** 空欄は null(消す)。 */
const orNull = (s: string): string | null => (s.trim() === "" ? null : s.trim());

export function Edit({ item, genres, client, baseUrl, onClose, onUpdated }: Props) {
  const sorted = [...genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const [name, setName] = useState(item.name);
  const [option, setOption] = useState(item.option_text ?? "");
  const [override, setOverride] = useState(item.query_override ?? "");
  const [genreId, setGenreId] = useState(item.genre_id);
  const [minPrice, setMinPrice] = useState(item.min_price == null ? "" : String(item.min_price));
  const [watch, setWatch] = useState(item.watch_official ?? false);
  const hasSource = (item.source_url ?? "").trim() !== "";
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  // 変えた項目だけを送る。
  const buildPatch = (): ItemUpdate => {
    const patch: ItemUpdate = {};
    if (name.trim() !== item.name) patch.name = name.trim();
    if (orNull(option) !== (item.option_text ?? null)) patch.option_text = orNull(option);
    if (orNull(override) !== (item.query_override ?? null)) patch.query_override = orNull(override);
    if (genreId !== item.genre_id) patch.genre_id = genreId;
    const nextMin = minPrice.trim() === "" ? null : Number(minPrice);
    if (nextMin !== (item.min_price ?? null)) patch.min_price = nextMin;
    if (watch !== (item.watch_official ?? false)) patch.watch_official = watch;
    return patch;
  };

  const save = () =>
    run(async () => {
      const patch = buildPatch();
      if (Object.keys(patch).length === 0) {
        onClose();
        return;
      }
      onUpdated(await client.updateItem(item.id, patch));
      onClose();
    });

  const replaceImage = (file: File) =>
    run(async () => {
      onUpdated(await client.replaceItemImage(item.id, file));
    });

  return (
    <Modal label="編集" className="tall" onClose={onClose}>
      <h2>編集</h2>
      <div className="edit-head">
        <img
          className="edit-image"
          src={resolveImageUrl(item.image_url, baseUrl)}
          alt=""
          {...genreColorProps(colorOf(genres, genreId))}
        />
        <label className="field grow">
          <span>画像を差し替え</span>
          <input
            type="file"
            accept="image/*"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void replaceImage(f);
            }}
          />
        </label>
      </div>
      <div className="genre-pick" role="group" aria-label="ジャンル">
        {sorted.map((g) => (
          <GenreButton
            key={g.id}
            className="compact"
            name={g.name}
            icon={g.icon}
            color={g.color}
            glyphSize={24}
            selected={genreId === g.id}
            onClick={() => {
              setGenreId(g.id);
            }}
          />
        ))}
      </div>
      <label className="field">
        <span>名前</span>
        <input
          type="text"
          value={name}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>オプション</span>
        <input
          type="text"
          value={option}
          onChange={(e) => {
            setOption(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>検索ワード上書き</span>
        <input
          type="text"
          value={override}
          onChange={(e) => {
            setOverride(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>最低価格(円)</span>
        <input
          type="number"
          inputMode="numeric"
          min={0}
          value={minPrice}
          onChange={(e) => {
            setMinPrice(e.target.value);
          }}
        />
      </label>
      <label className="field">
        <span>
          <input
            type="checkbox"
            checked={watch}
            disabled={!hasSource}
            onChange={(e) => {
              setWatch(e.target.checked);
            }}
          />{" "}
          公式ページを監視する
        </span>
      </label>
      {hasSource ? null : <p className="site-meta">公式ページの URL が無いので監視できません</p>}
      <ErrorText message={error} />
      <div className="actions stretch">
        <button type="button" onClick={onClose}>
          キャンセル
        </button>
        <button
          type="button"
          className="primary"
          disabled={busy || name.trim() === ""}
          onClick={() => void save()}
        >
          保存
        </button>
      </div>
    </Modal>
  );
}
