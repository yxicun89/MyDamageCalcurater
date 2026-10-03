import { useState } from "react";
import type { Genre, Item, ItemUpdate } from "../api/types";
import type { ApiClient } from "../lib/api";
import { ErrorText, Modal, errorMessage } from "./Modal";

interface Props {
  item: Item;
  genres: Genre[];
  client: ApiClient;
  onClose: () => void;
  onUpdated: (item: Item) => void;
}

/** 空欄は null(消す)。 */
const orNull = (s: string): string | null => (s.trim() === "" ? null : s.trim());

export function Edit({ item, genres, client, onClose, onUpdated }: Props) {
  const sorted = [...genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const [name, setName] = useState(item.name);
  const [option, setOption] = useState(item.option_text ?? "");
  const [override, setOverride] = useState(item.query_override ?? "");
  const [genreId, setGenreId] = useState(item.genre_id);
  const [minPrice, setMinPrice] = useState(item.min_price == null ? "" : String(item.min_price));
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
    <Modal label="編集" onClose={onClose}>
      <h2>編集</h2>
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
        <span>ジャンル</span>
        <select
          value={genreId}
          onChange={(e) => {
            setGenreId(Number(e.target.value));
          }}
        >
          {sorted.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </select>
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
      <ErrorText message={error} />
      <div className="actions">
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
