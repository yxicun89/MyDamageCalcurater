import { useEffect, useState } from "react";
import type { Genre, Item } from "../api/types";
import type { ApiClient } from "../lib/api";
import { isHttpUrl } from "../lib/deeplink";
import { GenreGlyph, UiIcon, genreColorProps } from "./icons";
import { ErrorText, Modal, errorMessage } from "./Modal";

interface Props {
  genres: Genre[];
  /** ホームで絞っているジャンル(下書きにジャンルが無いときの初期値) */
  initialGenreId: number | null;
  client: ApiClient;
  onClose: () => void;
  onCreated: (item: Item) => void;
}

type Step = "entry" | "url" | "confirm";

/** 選んだ写真のプレビュー(data URL)。読み終えるまでは null。 */
function usePreview(file: File | null): string | null {
  const [loaded, setLoaded] = useState<{ file: File; url: string } | null>(null);
  useEffect(() => {
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") setLoaded({ file, url: reader.result });
    };
    reader.readAsDataURL(file);
    return () => {
      reader.onload = null;
    };
  }, [file]);
  return loaded !== null && loaded.file === file ? loaded.url : null;
}

/** 追加のシート。入口(コピーした URL / URL を入力 / 写真)→ 確認(1 タップで追加)。 */
export function Register({ genres, initialGenreId, client, onClose, onCreated }: Props) {
  const sorted = [...genres].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
  const [step, setStep] = useState<Step>("entry");
  const [url, setUrl] = useState("");
  const [name, setName] = useState("");
  const [genreId, setGenreId] = useState<number | null>(initialGenreId);
  const [file, setFile] = useState<File | null>(null);
  const [draftImage, setDraftImage] = useState<string | null>(null);
  const [drafted, setDrafted] = useState(false);
  const [sourceUrl, setSourceUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const preview = usePreview(file);

  const canSubmit = !busy && name.trim() !== "" && genreId !== null && (file !== null || draftImage !== null);

  /** 成功したら true。失敗は alert にして false。 */
  const run = async (fn: () => Promise<void>): Promise<boolean> => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      return true;
    } catch (e) {
      setError(errorMessage(e));
      return false;
    } finally {
      setBusy(false);
    }
  };

  /** 推定するのはサーバー。genre_id は渡さず、返ってきたジャンルを選んでおく。 */
  const fetchDraft = (target: string) =>
    run(async () => {
      const d = await client.draftFromUrl(target);
      setName(d.name);
      setDraftImage(d.image_url ?? null);
      setSourceUrl(d.source_url);
      setDrafted(true);
      setGenreId(d.genre_id ?? initialGenreId);
      setStep("confirm");
    });

  /** 押したときにだけクリップボードを読む。読めない・URL でないときは入力欄へ。 */
  const pasteFromClipboard = async () => {
    let text = "";
    try {
      text = (await navigator.clipboard.readText()).trim();
    } catch {
      // 拒否・未対応は失敗扱いにせず、入力欄へ。
    }
    if (isHttpUrl(text)) {
      setUrl(text);
      // 取れなければ入力欄に URL を残して直せるようにする(alert は残す)。
      if (!(await fetchDraft(text))) setStep("url");
    } else {
      setStep("url");
    }
  };

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

  const cancel = (
    <button type="button" className="text-button" onClick={onClose}>
      キャンセル
    </button>
  );

  if (step === "entry") {
    return (
      <Modal label="追加" onClose={onClose}>
        <h2>追加</h2>
        <button
          type="button"
          className="big-button primary"
          disabled={busy}
          onClick={() => void pasteFromClipboard()}
        >
          <UiIcon name="clipboard" size={22} />
          コピーした URL を追加
        </button>
        <div className="entry-row">
          <button
            type="button"
            className="big-button"
            onClick={() => {
              setStep("url");
            }}
          >
            <UiIcon name="link" size={20} />
            URL を入力
          </button>
          <label className="big-button">
            <UiIcon name="camera" size={20} />
            写真
            <input
              type="file"
              accept="image/*"
              aria-label="写真"
              className="visually-hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (!f) return;
                setFile(f);
                setStep("confirm");
              }}
            />
          </label>
        </div>
        <ErrorText message={error} />
        <p className="hint">Safari の共有からも追加できます</p>
        <div className="actions center">{cancel}</div>
      </Modal>
    );
  }

  if (step === "url") {
    return (
      <Modal label="追加" onClose={onClose}>
        <h2>URL を入力</h2>
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
          <button
            type="button"
            className="primary"
            disabled={busy || url.trim() === ""}
            onClick={() => void fetchDraft(url.trim())}
          >
            取得
          </button>
        </div>
        <ErrorText message={error} />
        <div className="actions center">{cancel}</div>
      </Modal>
    );
  }

  const image = preview ?? draftImage;
  return (
    <Modal label="追加" onClose={onClose}>
      {image !== null ? (
        <img
          className="confirm-image"
          src={image}
          alt={name}
          {...genreColorProps(genres.find((g) => g.id === genreId)?.color ?? "#1d5fd1")}
        />
      ) : file === null ? (
        <div className="photo-pick">
          <label className="photo-frame">
            <UiIcon name="image" size={28} />
            写真を選ぶ
            <input
              type="file"
              accept="image/*"
              aria-label="写真を選ぶ"
              className="visually-hidden"
              onChange={(e) => {
                setFile(e.target.files?.[0] ?? null);
              }}
            />
          </label>
          {drafted && <p className="hint">このページの画像は取れませんでした</p>}
        </div>
      ) : (
        <div className="confirm-image placeholder" />
      )}
      <input
        type="text"
        className="confirm-name"
        aria-label="名前"
        value={name}
        onChange={(e) => {
          setName(e.target.value);
        }}
      />
      <div className="genre-pick" role="group" aria-label="ジャンル">
        {sorted.map((g) => (
          <button
            key={g.id}
            type="button"
            className="genre-button large"
            aria-label={g.name}
            aria-pressed={genreId === g.id}
            {...genreColorProps(g.color)}
            onClick={() => {
              setGenreId(g.id);
            }}
          >
            <GenreGlyph icon={g.icon} size={28} />
          </button>
        ))}
      </div>
      <ErrorText message={error} />
      <button
        type="button"
        className="big-button primary"
        disabled={!canSubmit}
        onClick={() => void submit()}
      >
        追加
      </button>
      <div className="actions center">{cancel}</div>
    </Modal>
  );
}
