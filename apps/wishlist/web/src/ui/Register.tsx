import { useState } from "react";
import type { Genre, Item } from "../api/types";
import type { ApiClient } from "../lib/api";
import { ErrorText, Modal, errorMessage } from "./Modal";

interface Props {
  genres: Genre[];
  /** チップで選んでいるジャンル(登録の初期値) */
  initialGenreId: number | null;
  client: ApiClient;
  onClose: () => void;
  onCreated: (item: Item) => void;
}

export function Register({ genres, initialGenreId, client, onClose, onCreated }: Props) {
  const sorted = [...genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const [genreId, setGenreId] = useState<number | null>(initialGenreId ?? sorted[0]?.id ?? null);
  const [file, setFile] = useState<File | null>(null);
  const [url, setUrl] = useState("");
  const [name, setName] = useState("");
  const [draftImage, setDraftImage] = useState<string | null>(null);
  const [sourceUrl, setSourceUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSubmit = !busy && name.trim() !== "" && genreId !== null && (file !== null || draftImage !== null);

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

  const fetchDraft = () =>
    run(async () => {
      const d = await client.draftFromUrl(url.trim());
      setName(d.name);
      setDraftImage(d.image_url ?? null);
      setSourceUrl(d.source_url);
      if (d.genre_id != null) setGenreId(d.genre_id);
    });

  const submit = () =>
    run(async () => {
      if (genreId === null) return;
      const fields = {
        genre_id: genreId,
        name: name.trim(),
        ...(sourceUrl ? { source_url: sourceUrl } : {}),
      };
      if (file) onCreated(await client.createItemWithImage(fields, file));
      else if (draftImage)
        onCreated(await client.createItemFromImageUrl({ ...fields, image_url: draftImage }));
    });

  return (
    <Modal label="登録" onClose={onClose}>
      <h2>欲しいものを登録</h2>
      <label className="field">
        <span>写真</span>
        <input
          type="file"
          accept="image/*"
          onChange={(e) => {
            setFile(e.target.files?.[0] ?? null);
          }}
        />
      </label>
      <div className="field-row">
        <label className="field grow">
          <span>URL</span>
          <input
            type="url"
            inputMode="url"
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
            }}
          />
        </label>
        <button type="button" disabled={busy || url.trim() === ""} onClick={() => void fetchDraft()}>
          取得
        </button>
      </div>
      {draftImage !== null && file === null && <img className="draft-image" src={draftImage} alt="" />}
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
        <span>ジャンル</span>
        <select
          value={genreId ?? ""}
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
      <ErrorText message={error} />
      <div className="actions">
        <button type="button" onClick={onClose}>
          キャンセル
        </button>
        <button type="button" className="primary" disabled={!canSubmit} onClick={() => void submit()}>
          登録
        </button>
      </div>
    </Modal>
  );
}
